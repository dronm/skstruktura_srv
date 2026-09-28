package services

import (
	"context"
	"fmt"
	"time"

	"github.com/dronm/ds/v4"
	"github.com/dronm/session"
	"github.com/dronm/skstruktura/internal/apperrors"
	"github.com/dronm/skstruktura/internal/models"
	"github.com/dronm/webapp"
)

type InventoryValuationService struct {
	DB      ds.Provider
	Session session.Session
	QueryID string
}

func NewInventoryValuationService(ctx webapp.ServiceContext) any {
	return &InventoryValuationService{
		DB:      ctx.DB,
		Session: ctx.Session,
		QueryID: ctx.QueryID,
	}
}

func RegisterInventoryValuationService() {
	webapp.MustRegisterService(
		"InventoryValuation",
		&InventoryValuationService{},
		NewInventoryValuationService,
	)
}

func (s *InventoryValuationService) State(ctx context.Context) (models.InventoryValuationState, error) {
	user, err := s.currentUser()
	if err != nil {
		return models.InventoryValuationState{}, err
	}
	if !inventoryValuationCanView(user.RoleID) {
		return models.InventoryValuationState{}, webapp.Forbidden("inventory valuation is not available for the current role", nil)
	}
	if s.DB == nil {
		return models.InventoryValuationState{}, webapp.Internal("database is not initialized", nil)
	}

	poolConn, connID, err := s.DB.GetPrimary(ctx)
	if err != nil {
		return models.InventoryValuationState{}, fmt.Errorf("get inventory valuation connection: %w", err)
	}
	defer s.DB.Release(poolConn, connID)

	return fetchInventoryValuationState(ctx, poolConn.Conn(), inventoryValuationCanManage(user.RoleID))
}

func (s *InventoryValuationService) UpdateSettings(
	ctx context.Context,
	input *models.InventoryValuationSettingsUpdate,
) (models.InventoryValuationState, error) {
	_, err := s.requireManager()
	if err != nil {
		return models.InventoryValuationState{}, err
	}
	if input == nil || (input.AllowNegativeOpenPeriod == nil && input.ExcludeVATFromCost == nil) {
		return models.InventoryValuationState{}, webapp.BadRequest("at least one inventory valuation setting is required", nil)
	}

	var result models.InventoryValuationState
	err = withPrimaryTransaction(ctx, s.DB, func(tx ds.Tx) error {
		if _, err := tx.Exec(ctx, `
			UPDATE public.register_settings
			SET
				materials_allow_negative_open_period = COALESCE($1, materials_allow_negative_open_period),
				materials_exclude_vat_from_cost = COALESCE($2, materials_exclude_vat_from_cost)
			WHERE id = 1
		`, input.AllowNegativeOpenPeriod, input.ExcludeVATFromCost); err != nil {
			return err
		}

		if input.ExcludeVATFromCost != nil {
			if err := rebuildAllMaterialRegisterActions(ctx, tx); err != nil {
				return err
			}
		} else if err := revalueMaterialRegister(ctx, tx); err != nil {
			return err
		}

		var err error
		result, err = fetchInventoryValuationState(ctx, tx, true)
		return err
	})
	if err != nil {
		return models.InventoryValuationState{}, fmt.Errorf("update inventory valuation settings: %w", err)
	}

	return result, nil
}

func (s *InventoryValuationService) Recalculate(ctx context.Context) (models.InventoryValuationState, error) {
	_, err := s.requireManager()
	if err != nil {
		return models.InventoryValuationState{}, err
	}

	var result models.InventoryValuationState
	err = withPrimaryTransaction(ctx, s.DB, func(tx ds.Tx) error {
		if err := revalueMaterialRegister(ctx, tx); err != nil {
			return err
		}
		var err error
		result, err = fetchInventoryValuationState(ctx, tx, true)
		return err
	})
	if err != nil {
		return models.InventoryValuationState{}, fmt.Errorf("recalculate inventory valuation: %w", err)
	}
	return result, nil
}

func (s *InventoryValuationService) Close(
	ctx context.Context,
	input *models.InventoryValuationCloseRequest,
) (models.InventoryValuationState, error) {
	user, err := s.requireManager()
	if err != nil {
		return models.InventoryValuationState{}, err
	}
	if input == nil || input.ClosedThrough == "" {
		return models.InventoryValuationState{}, webapp.BadRequest("closed_through is required", nil)
	}
	closedThrough, err := time.Parse("2006-01-02", input.ClosedThrough)
	if err != nil {
		return models.InventoryValuationState{}, webapp.BadRequest("closed_through must use YYYY-MM-DD format", nil)
	}

	var result models.InventoryValuationState
	err = withPrimaryTransaction(ctx, s.DB, func(tx ds.Tx) error {
		if _, err := tx.Exec(ctx, "SELECT public.materials_close_period($1::date, $2)", closedThrough.Format("2006-01-02"), user.ID); err != nil {
			return err
		}
		var err error
		result, err = fetchInventoryValuationState(ctx, tx, true)
		return err
	})
	if err != nil {
		return models.InventoryValuationState{}, fmt.Errorf("close material valuation period: %w", err)
	}
	return result, nil
}

func (s *InventoryValuationService) Reopen(ctx context.Context) (models.InventoryValuationState, error) {
	user, err := s.requireManager()
	if err != nil {
		return models.InventoryValuationState{}, err
	}

	var result models.InventoryValuationState
	err = withPrimaryTransaction(ctx, s.DB, func(tx ds.Tx) error {
		if _, err := tx.Exec(ctx, "SELECT public.materials_reopen_period($1)", user.ID); err != nil {
			return err
		}
		if err := revalueMaterialRegister(ctx, tx); err != nil {
			return err
		}
		var err error
		result, err = fetchInventoryValuationState(ctx, tx, true)
		return err
	})
	if err != nil {
		return models.InventoryValuationState{}, fmt.Errorf("reopen material valuation period: %w", err)
	}
	return result, nil
}

func (s *InventoryValuationService) currentUser() (models.UserLogin, error) {
	if s.Session == nil {
		return models.UserLogin{}, apperrors.SessionRequired()
	}
	var user models.UserLogin
	if err := s.Session.Get("user", &user); err != nil || user.ID <= 0 {
		return models.UserLogin{}, apperrors.SessionRequired()
	}
	return user, nil
}

func (s *InventoryValuationService) requireManager() (models.UserLogin, error) {
	user, err := s.currentUser()
	if err != nil {
		return models.UserLogin{}, err
	}
	if !inventoryValuationCanManage(user.RoleID) {
		return models.UserLogin{}, webapp.Forbidden("inventory valuation management is not available for the current role", nil)
	}
	if s.DB == nil {
		return models.UserLogin{}, webapp.Internal("database is not initialized", nil)
	}
	return user, nil
}

func inventoryValuationCanView(roleID models.RoleID) bool {
	switch roleID {
	case models.RoleIDAdmin, models.RoleIDAccountant, models.RoleIDConstructionSiteManager:
		return true
	default:
		return false
	}
}

func inventoryValuationCanManage(roleID models.RoleID) bool {
	return roleID == models.RoleIDAdmin || roleID == models.RoleIDAccountant
}

func fetchInventoryValuationState(
	ctx context.Context,
	db ds.Querier,
	canManage bool,
) (models.InventoryValuationState, error) {
	result := models.InventoryValuationState{
		CanManage:          canManage,
		Closures:           make([]*models.InventoryValuationClosure, 0),
		LastRecalculatedAt: time.Now().UTC(),
	}

	var closedThrough *time.Time
	if err := db.QueryRow(ctx, `
		SELECT
			materials_allow_negative_open_period,
			materials_exclude_vat_from_cost,
			materials_closed_through,
			NOT EXISTS (SELECT 1 FROM public.ra_materials LIMIT 1)
		FROM public.register_settings
		WHERE id = 1
	`).Scan(
		&result.Settings.AllowNegativeOpenPeriod,
		&result.Settings.ExcludeVATFromCost,
		&closedThrough,
		&result.Settings.VATBasisEditable,
	); err != nil {
		return models.InventoryValuationState{}, fmt.Errorf("read inventory valuation settings: %w", err)
	}
	result.Settings.ClosedThrough = closedThrough

	if err := db.QueryRow(ctx, `
		SELECT
			COALESCE(SUM(pending_count), 0)::bigint,
			COUNT(*) FILTER (WHERE quant < 0)::bigint,
			CASE
				WHEN bool_or(pending_count > 0 OR amount IS NULL) THEN NULL
				ELSE COALESCE(SUM(amount), 0)::double precision
			END
		FROM public.rg_materials_current
	`).Scan(&result.PendingCount, &result.NegativeCount, &result.CurrentAmount); err != nil {
		return models.InventoryValuationState{}, fmt.Errorf("read inventory valuation totals: %w", err)
	}

	rows, err := db.Query(ctx, `
		SELECT id, closed_through, closed_at, closed_by, reopened_at, reopened_by
		FROM public.material_valuation_closures
		ORDER BY id DESC
		LIMIT 20
	`)
	if err != nil {
		return models.InventoryValuationState{}, fmt.Errorf("read inventory valuation closure history: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		item := &models.InventoryValuationClosure{}
		if err := rows.Scan(
			&item.ID,
			&item.ClosedThrough,
			&item.ClosedAt,
			&item.ClosedBy,
			&item.ReopenedAt,
			&item.ReopenedBy,
		); err != nil {
			return models.InventoryValuationState{}, fmt.Errorf("scan inventory valuation closure history: %w", err)
		}
		result.Closures = append(result.Closures, item)
	}
	if err := rows.Err(); err != nil {
		return models.InventoryValuationState{}, fmt.Errorf("iterate inventory valuation closure history: %w", err)
	}

	return result, nil
}

func rebuildAllMaterialRegisterActions(ctx context.Context, tx ds.Querier) error {
	if _, err := tx.Exec(ctx, "TRUNCATE TABLE public.ra_materials RESTART IDENTITY"); err != nil {
		return fmt.Errorf("reset material register actions: %w", err)
	}

	recorderQueries := []struct {
		recorderType string
		query        string
	}{
		{materialReceiptRecorderType, "SELECT id FROM public.material_receipts ORDER BY date, id"},
		{materialConsumptionRecorderType, "SELECT id FROM public.material_consumptions ORDER BY date, id"},
		{materialTransferRecorderType, "SELECT id FROM public.material_transfers ORDER BY date, id"},
	}

	for _, item := range recorderQueries {
		rows, err := tx.Query(ctx, item.query)
		if err != nil {
			return fmt.Errorf("list %s recorders: %w", item.recorderType, err)
		}

		ids := make([]int, 0)
		for rows.Next() {
			var id int
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return err
			}
			ids = append(ids, id)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}

		for _, id := range ids {
			if err := rewriteMaterialRegisterActions(ctx, tx, item.recorderType, id); err != nil {
				return err
			}
		}
	}

	return revalueMaterialRegister(ctx, tx)
}
