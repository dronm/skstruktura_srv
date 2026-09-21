package services

import (
	"context"
	"fmt"
	"time"

	"github.com/dronm/ds/v4"
	"github.com/dronm/skstruktura/internal/models"
	"github.com/dronm/webapp"
)

const dotNetUnixEpochTicksForDiadoc = int64(621355968000000000)

func (s *DiadocImportService) State(ctx context.Context) (models.DiadocState, error) {
	if _, err := s.require(); err != nil {
		return models.DiadocState{}, err
	}
	poolConn, connID, err := s.DB.GetPrimary(ctx)
	if err != nil {
		return models.DiadocState{}, err
	}
	defer s.DB.Release(poolConn, connID)

	result := models.DiadocState{BufferCounts: make(map[string]int64)}
	var lastSyncStartedAt, lastSyncFinishedAt *time.Time
	var hasRefreshToken, hasAccessToken bool
	if err := poolConn.Conn().QueryRow(ctx, `
		SELECT
			COALESCE(box_id, ''),
			enabled,
			event_timestamp_from,
			COALESCE(after_index_key, '') <> '',
			last_sync_started_at,
			last_sync_finished_at,
			COALESCE(last_sync_error, ''),
			version,
			COALESCE(refresh_token, '') <> '',
			COALESCE(access_token, '') <> ''
		FROM integration_diadoc.state
		WHERE id = 1
	`).Scan(
		&result.BoxID,
		&result.Enabled,
		&result.EventTimestampFrom,
		&result.HasCursor,
		&lastSyncStartedAt,
		&lastSyncFinishedAt,
		&result.LastSyncError,
		&result.Version,
		&hasRefreshToken,
		&hasAccessToken,
	); err != nil {
		return models.DiadocState{}, fmt.Errorf("select Diadoc state: %w", err)
	}
	result.LastSyncStartedAt = lastSyncStartedAt
	result.LastSyncFinishedAt = lastSyncFinishedAt
	result.Authorized = hasRefreshToken || hasAccessToken
	result.Configured = s.Manager != nil

	rows, err := poolConn.Conn().Query(ctx, `
		SELECT status, count(*)::bigint
		FROM integration_diadoc.documents
		GROUP BY status
	`)
	if err != nil {
		return models.DiadocState{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var status string
		var count int64
		if err := rows.Scan(&status, &count); err != nil {
			return models.DiadocState{}, err
		}
		result.BufferCounts[status] = count
	}
	if err := rows.Err(); err != nil {
		return models.DiadocState{}, err
	}
	return result, nil
}

func (s *DiadocImportService) UpdateState(
	ctx context.Context,
	request *models.DiadocStateUpdateRequest,
) (models.DiadocState, error) {
	user, err := s.require()
	if err != nil {
		return models.DiadocState{}, err
	}
	if request == nil || request.Version <= 0 {
		return models.DiadocState{}, webapp.BadRequest("valid Diadoc state version is required", nil)
	}
	var unlock func()
	if s.Manager != nil {
		unlock = s.Manager.LockSynchronization()
		defer unlock()
	}

	err = withPrimaryTransaction(ctx, s.DB, func(tx ds.Tx) error {
		if err := setAuditActor(ctx, tx, user); err != nil {
			return err
		}
		var currentVersion int64
		if err := tx.QueryRow(ctx, `
			SELECT version
			FROM integration_diadoc.state
			WHERE id = 1
			FOR UPDATE
		`).Scan(&currentVersion); err != nil {
			return err
		}
		if currentVersion != request.Version {
			return webapp.Conflict(
				"Diadoc state was changed by another request",
				map[string]any{"expected_version": request.Version, "current_version": currentVersion},
			)
		}
		_, err := tx.Exec(ctx, `
			UPDATE integration_diadoc.state
			SET
				enabled = $1,
				version = version + 1,
				updated_at = now()
			WHERE id = 1
		`, request.Enabled)
		return err
	})
	if err != nil {
		return models.DiadocState{}, fmt.Errorf("update Diadoc state: %w", err)
	}
	if s.Manager != nil {
		if err := s.Manager.ReloadState(ctx); err != nil {
			return models.DiadocState{}, err
		}
		if request.Enabled {
			s.Manager.PollNow()
		}
	}
	return s.State(ctx)
}

func (s *DiadocImportService) Replay(
	ctx context.Context,
	request *models.DiadocReplayRequest,
) (models.DiadocState, error) {
	user, err := s.require()
	if err != nil {
		return models.DiadocState{}, err
	}
	if request == nil || request.Version <= 0 || request.EventTimestampFrom.IsZero() {
		return models.DiadocState{}, webapp.BadRequest("version and event_timestamp_from are required", nil)
	}
	if request.EventTimestampFrom.After(time.Now().Add(time.Minute)) {
		return models.DiadocState{}, webapp.BadRequest("event_timestamp_from should not be in the future", nil)
	}
	var unlock func()
	if s.Manager != nil {
		unlock = s.Manager.LockSynchronization()
		defer unlock()
	}

	err = withPrimaryTransaction(ctx, s.DB, func(tx ds.Tx) error {
		if err := setAuditActor(ctx, tx, user); err != nil {
			return err
		}
		var currentVersion int64
		if err := tx.QueryRow(ctx, `
			SELECT version
			FROM integration_diadoc.state
			WHERE id = 1
			FOR UPDATE
		`).Scan(&currentVersion); err != nil {
			return err
		}
		if currentVersion != request.Version {
			return webapp.Conflict(
				"Diadoc state was changed by another request",
				map[string]any{"expected_version": request.Version, "current_version": currentVersion},
			)
		}
		ticks := dotNetTicksForDiadoc(request.EventTimestampFrom)
		if _, err := tx.Exec(ctx, `
			UPDATE integration_diadoc.state
			SET
				event_timestamp_from = $1,
				event_timestamp_from_ticks = $2,
				after_index_key = NULL,
				cursor_reset_at = now(),
				cursor_reset_by = $3,
				version = version + 1,
				updated_at = now()
			WHERE id = 1
		`, request.EventTimestampFrom, ticks, user.Name); err != nil {
			return err
		}
		if request.RestoreIgnored {
			if _, err := tx.Exec(ctx, `
				UPDATE integration_diadoc.documents
				SET
					status = 'needs_matching',
					ignored_reason = NULL,
					ignored_at = NULL,
					ignored_by = NULL,
					updated_at = now(),
					version = version + 1
				WHERE status = 'ignored'
					AND COALESCE(event_timestamp, created_at) >= $1
			`, request.EventTimestampFrom); err != nil {
				return err
			}
		}
		if request.RetryFailed {
			if _, err := tx.Exec(ctx, `
				UPDATE integration_diadoc.documents
				SET
					status = 'received',
					last_error = NULL,
					updated_at = now(),
					version = version + 1
				WHERE status = 'failed'
					AND COALESCE(event_timestamp, created_at) >= $1
			`, request.EventTimestampFrom); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return models.DiadocState{}, fmt.Errorf("reset Diadoc event cursor: %w", err)
	}
	if s.Manager != nil {
		if err := s.Manager.ReloadState(ctx); err != nil {
			return models.DiadocState{}, err
		}
		s.Manager.PollNow()
	}
	return s.State(ctx)
}

func dotNetTicksForDiadoc(value time.Time) int64 {
	value = value.UTC()
	return dotNetUnixEpochTicksForDiadoc + value.Unix()*10_000_000 + int64(value.Nanosecond()/100)
}
