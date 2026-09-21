package diadoc

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type State struct {
	BoxID                   string
	AccessToken             string
	AccessTokenExpiresAt    time.Time
	RefreshToken            string
	AfterIndexKey           string
	EventTimestampFromTicks int64
	Enabled                 bool
	EventTimestampFrom      time.Time
	LastSyncStartedAt       time.Time
	LastSyncFinishedAt      time.Time
	LastSyncError           string
	Version                 int64
}

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func (s *Store) Load(ctx context.Context) (State, error) {
	var result State
	var accessTokenExpiresAtUnix int64
	var eventTimestampFrom *time.Time
	var lastSyncStartedAt *time.Time
	var lastSyncFinishedAt *time.Time
	err := s.pool.QueryRow(ctx, `
		SELECT
			COALESCE(box_id, ''),
			COALESCE(access_token, ''),
			COALESCE(EXTRACT(EPOCH FROM access_token_expires_at)::bigint, 0),
			COALESCE(refresh_token, ''),
			COALESCE(after_index_key, ''),
			COALESCE(event_timestamp_from_ticks, 0),
			enabled,
			event_timestamp_from,
			last_sync_started_at,
			last_sync_finished_at,
			COALESCE(last_sync_error, ''),
			version
		FROM integration_diadoc.state
		WHERE id = 1
	`).Scan(
		&result.BoxID,
		&result.AccessToken,
		&accessTokenExpiresAtUnix,
		&result.RefreshToken,
		&result.AfterIndexKey,
		&result.EventTimestampFromTicks,
		&result.Enabled,
		&eventTimestampFrom,
		&lastSyncStartedAt,
		&lastSyncFinishedAt,
		&result.LastSyncError,
		&result.Version,
	)
	if err == nil {
		if accessTokenExpiresAtUnix > 0 {
			result.AccessTokenExpiresAt = time.Unix(accessTokenExpiresAtUnix, 0)
		}
		if eventTimestampFrom != nil {
			result.EventTimestampFrom = *eventTimestampFrom
		}
		if lastSyncStartedAt != nil {
			result.LastSyncStartedAt = *lastSyncStartedAt
		}
		if lastSyncFinishedAt != nil {
			result.LastSyncFinishedAt = *lastSyncFinishedAt
		}
		return result, nil
	}
	if err != pgx.ErrNoRows {
		return State{}, fmt.Errorf("load Diadoc integration state: %w", err)
	}
	return State{}, nil
}

func (s *Store) Save(ctx context.Context, state State) error {
	result, err := s.pool.Exec(ctx, `
		UPDATE integration_diadoc.state
		SET
			box_id = NULLIF($1, ''),
			access_token = NULLIF($2, ''),
			access_token_expires_at = $3,
			refresh_token = NULLIF($4, ''),
			after_index_key = NULLIF($5, ''),
			event_timestamp_from_ticks = $6,
			event_timestamp_from = COALESCE($7, event_timestamp_from),
			updated_at = now()
		WHERE id = 1
	`,
		state.BoxID,
		state.AccessToken,
		nullableTime(state.AccessTokenExpiresAt),
		state.RefreshToken,
		state.AfterIndexKey,
		state.EventTimestampFromTicks,
		nullableTime(state.EventTimestampFrom),
	)
	if err != nil {
		return fmt.Errorf("save Diadoc integration state: %w", err)
	}
	if result.RowsAffected() != 1 {
		return fmt.Errorf("save Diadoc integration state: singleton row is missing")
	}
	return nil
}

func (s *Store) MarkSyncStarted(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE integration_diadoc.state
		SET
			last_sync_started_at = now(),
			last_sync_error = NULL,
			updated_at = now()
		WHERE id = 1
	`)
	if err != nil {
		return fmt.Errorf("mark Diadoc synchronization started: %w", err)
	}
	return nil
}

func (s *Store) MarkSyncFinished(ctx context.Context, syncErr error) error {
	var errorText any
	if syncErr != nil {
		errorText = syncErr.Error()
	}
	_, err := s.pool.Exec(ctx, `
		UPDATE integration_diadoc.state
		SET
			last_sync_finished_at = now(),
			last_sync_error = $1,
			updated_at = now()
		WHERE id = 1
	`, errorText)
	if err != nil {
		return fmt.Errorf("mark Diadoc synchronization finished: %w", err)
	}
	return nil
}

func nullableTime(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return value
}
