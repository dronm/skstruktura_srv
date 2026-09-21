package services

import (
	"context"
	"fmt"
	"time"

	"github.com/dronm/ds/v4"
	"github.com/dronm/modelbind"
	"github.com/dronm/session"
	"github.com/dronm/skstruktura/internal/apperrors"
	"github.com/dronm/skstruktura/internal/models"
	"github.com/dronm/webapp"
)

const (
	materialBalanceDefaultPageSize = 1000
	materialBalanceMaxPageSize     = 5000
)

type MaterialBalanceService struct {
	DB      ds.Provider
	Session session.Session
	QueryID string
}

func NewMaterialBalanceService(ctx webapp.ServiceContext) any {
	return &MaterialBalanceService{
		DB:      ctx.DB,
		Session: ctx.Session,
		QueryID: ctx.QueryID,
	}
}

func RegisterMaterialBalanceService() {
	webapp.MustRegisterService(
		"MaterialBalance",
		&MaterialBalanceService{},
		NewMaterialBalanceService,
	)
}

func (s *MaterialBalanceService) List(
	ctx context.Context,
	input models.MaterialBalanceInput,
) (models.MaterialBalanceResponse, error) {
	user, err := s.currentUser()
	if err != nil {
		return models.MaterialBalanceResponse{}, err
	}
	if s.DB == nil {
		return models.MaterialBalanceResponse{}, webapp.Internal("database is not initialized", nil)
	}

	query, params, err := validateMaterialBalanceInput(input)
	if err != nil {
		return models.MaterialBalanceResponse{}, err
	}
	poolConn, connID, err := s.DB.GetPrimary(ctx)
	if err != nil {
		return models.MaterialBalanceResponse{}, fmt.Errorf("get primary connection for material balance: %w", err)
	}
	defer s.DB.Release(poolConn, connID)

	db := poolConn.Conn()
	available, err := materialBalanceSiteAvailable(
		ctx,
		db,
		user,
		query.ConstructionSiteID,
	)
	if err != nil {
		return models.MaterialBalanceResponse{}, err
	}
	if !available {
		return models.MaterialBalanceResponse{}, webapp.Forbidden(
			"construction site is not available to the current user",
			map[string]any{
				"code":                 apperrors.CodeForbidden,
				"construction_site_id": query.ConstructionSiteID,
			},
		)
	}

	result := models.MaterialBalanceResponse{
		Rows:        make([]*models.MaterialBalanceRow, 0),
		GeneratedAt: time.Now().UTC(),
	}
	if err := db.QueryRow(ctx, `
		SELECT COUNT(*)::bigint
		FROM public.material_balances_list
		WHERE construction_site_id = $1
	`, query.ConstructionSiteID).Scan(&result.Total); err != nil {
		return models.MaterialBalanceResponse{}, fmt.Errorf("count material balances: %w", err)
	}

	rows, err := db.Query(ctx, `
		SELECT
			id,
			construction_site_id,
			construction_site,
			material_type_id,
			material_type,
			material_id,
			material,
			measure_unit_id,
			measure_unit,
			balance::double precision
		FROM public.material_balances_list
		WHERE construction_site_id = $1
		ORDER BY
			lower(material_type_name),
			material_type_id,
			balance DESC,
			lower(material_name),
			material_id
		LIMIT $2 OFFSET $3
	`, query.ConstructionSiteID, int(params.Count), int(params.From))
	if err != nil {
		return models.MaterialBalanceResponse{}, fmt.Errorf("select material balances: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		row := &models.MaterialBalanceRow{}
		if err := rows.Scan(
			&row.ID,
			&row.ConstructionSiteID,
			&row.ConstructionSite,
			&row.MaterialTypeID,
			&row.MaterialType,
			&row.MaterialID,
			&row.Material,
			&row.MeasureUnitID,
			&row.MeasureUnit,
			&row.Balance,
		); err != nil {
			return models.MaterialBalanceResponse{}, fmt.Errorf("scan material balance: %w", err)
		}
		result.Rows = append(result.Rows, row)
	}
	if err := rows.Err(); err != nil {
		return models.MaterialBalanceResponse{}, fmt.Errorf("iterate material balances: %w", err)
	}

	return result, nil
}

func (s *MaterialBalanceService) ConstructionSites(
	ctx context.Context,
) (models.MaterialBalanceConstructionSitesResponse, error) {
	user, err := s.currentUser()
	if err != nil {
		return models.MaterialBalanceConstructionSitesResponse{}, err
	}
	if s.DB == nil {
		return models.MaterialBalanceConstructionSitesResponse{}, webapp.Internal("database is not initialized", nil)
	}
	poolConn, connID, err := s.DB.GetPrimary(ctx)
	if err != nil {
		return models.MaterialBalanceConstructionSitesResponse{}, fmt.Errorf(
			"get primary connection for material balance construction sites: %w",
			err,
		)
	}
	defer s.DB.Release(poolConn, connID)

	query := `
		SELECT site.id, site.name
		FROM public.construction_sites AS site
		WHERE true
	`
	args := make([]any, 0, 1)
	if user.RoleID == models.RoleIDConstructionSiteManager {
		query += `
			AND EXISTS (
				SELECT 1
				FROM public.user_construction_sites AS assignment
				WHERE assignment.user_id = $1
					AND assignment.construction_site_id = site.id
			)
		`
		args = append(args, user.ID)
	}
	query += `
		ORDER BY lower(site.name), site.id
	`

	rows, err := poolConn.Conn().Query(ctx, query, args...)
	if err != nil {
		return models.MaterialBalanceConstructionSitesResponse{}, fmt.Errorf(
			"select material balance construction sites: %w",
			err,
		)
	}
	defer rows.Close()

	result := models.MaterialBalanceConstructionSitesResponse{
		Rows: make([]*models.MaterialBalanceConstructionSite, 0),
	}
	for rows.Next() {
		item := &models.MaterialBalanceConstructionSite{}
		if err := rows.Scan(&item.ID, &item.Name); err != nil {
			return models.MaterialBalanceConstructionSitesResponse{}, fmt.Errorf(
				"scan material balance construction site: %w",
				err,
			)
		}
		result.Rows = append(result.Rows, item)
	}
	if err := rows.Err(); err != nil {
		return models.MaterialBalanceConstructionSitesResponse{}, fmt.Errorf(
			"iterate material balance construction sites: %w",
			err,
		)
	}
	result.Total = int64(len(result.Rows))

	return result, nil
}

func (s *MaterialBalanceService) currentUser() (models.UserLogin, error) {
	if s.Session == nil {
		return models.UserLogin{}, apperrors.SessionRequired()
	}

	user := models.UserLogin{}
	if err := s.Session.Get("user", &user); err != nil || user.ID <= 0 {
		return models.UserLogin{}, apperrors.SessionRequired()
	}

	return user, nil
}

func validateMaterialBalanceInput(
	input models.MaterialBalanceInput,
) (*models.MaterialBalanceQuery, modelbind.CollectionParams, error) {
	if input.Query == nil {
		return nil, modelbind.CollectionParams{}, webapp.BadRequest("material balance query is required", nil)
	}
	if input.Query.ConstructionSiteID <= 0 {
		return nil, modelbind.CollectionParams{}, webapp.BadRequest("construction_site_id should be positive", nil)
	}
	if len(input.Params.Filter) > 0 {
		return nil, modelbind.CollectionParams{}, webapp.BadRequest(
			"generic collection filters are not supported by the material balance report",
			nil,
		)
	}
	if len(input.Params.Sorter) > 0 {
		return nil, modelbind.CollectionParams{}, webapp.BadRequest(
			"the material balance report uses fixed material-type and balance sorting",
			nil,
		)
	}
	if input.Params.From < 0 {
		return nil, modelbind.CollectionParams{}, webapp.BadRequest("from should not be negative", nil)
	}
	if input.Params.Count < 0 {
		return nil, modelbind.CollectionParams{}, webapp.BadRequest("count should not be negative", nil)
	}

	params := input.Params
	if params.Count == 0 {
		params.Count = materialBalanceDefaultPageSize
	}
	if params.Count > materialBalanceMaxPageSize {
		params.Count = materialBalanceMaxPageSize
	}

	query := *input.Query
	return &query, params, nil
}

func materialBalanceSiteAvailable(
	ctx context.Context,
	db ds.Querier,
	user models.UserLogin,
	constructionSiteID int,
) (bool, error) {
	query := `
		SELECT EXISTS (
			SELECT 1
			FROM public.construction_sites AS site
			WHERE site.id = $1
	`
	args := []any{constructionSiteID}
	if user.RoleID == models.RoleIDConstructionSiteManager {
		query += `
				AND EXISTS (
					SELECT 1
					FROM public.user_construction_sites AS assignment
					WHERE assignment.user_id = $2
						AND assignment.construction_site_id = site.id
				)
		`
		args = append(args, user.ID)
	}
	query += `
		)
	`

	var available bool
	if err := db.QueryRow(ctx, query, args...).Scan(&available); err != nil {
		return false, fmt.Errorf("check material balance construction site: %w", err)
	}
	return available, nil
}
