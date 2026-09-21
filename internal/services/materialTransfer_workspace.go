package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/dronm/ds/v4"
	"github.com/dronm/modelbind"
	"github.com/dronm/skstruktura/internal/apperrors"
	"github.com/dronm/skstruktura/internal/models"
	"github.com/dronm/webapp"
	wmodels "github.com/dronm/webapp/models"
)

const (
	constructionManagerMaterialTransferCreatePermission = "constructionManager.materialTransfer.create"
	constructionManagerMaterialTransferListPermission   = "constructionManager.materialTransfer.list"

	constructionManagerMaterialTransferDefaultPageSize = 30
	constructionManagerMaterialTransferMaxPageSize     = 100
)

func (s *MaterialTransferService) ConstructionManagerCreate(
	ctx context.Context,
	document *models.MaterialTransferDocument,
) (*models.MaterialTransferDocument, error) {
	user, err := s.constructionManagerMaterialTransferUser()
	if err != nil {
		return nil, err
	}
	if err := authorizeConstructionManagerMaterialTransferRole(
		user,
		constructionManagerMaterialTransferCreatePermission,
	); err != nil {
		return nil, err
	}
	if err := s.requireDB(); err != nil {
		return nil, err
	}
	if err := validateMaterialTransferDocument(document, true, 0); err != nil {
		return nil, err
	}

	var result *models.MaterialTransferDocument
	if err := withPrimaryTransaction(ctx, s.DB, func(tx ds.Tx) error {
		available, err := materialBalanceSiteAvailable(
			ctx,
			tx,
			user,
			document.SourceConstructionSiteID,
		)
		if err != nil {
			return err
		}
		if !available {
			return constructionManagerMaterialTransferSiteForbidden(document.SourceConstructionSiteID)
		}
		if err := validateConstructionManagerMaterialTransferDestination(
			ctx,
			tx,
			document.DestinationConstructionSiteID,
		); err != nil {
			return err
		}
		if err := validateConstructionManagerMaterialTransferCatalog(ctx, tx, document.Items); err != nil {
			return err
		}

		result, err = createMaterialTransferDocument(ctx, tx, document)
		return err
	}); err != nil {
		return nil, fmt.Errorf("create construction manager material transfer: %w", err)
	}

	return result, nil
}

func (s *MaterialTransferService) ConstructionManagerList(
	ctx context.Context,
	input models.ConstructionManagerMaterialTransferInput,
) (wmodels.CollectionResponse[*models.MaterialTransferList], error) {
	user, err := s.constructionManagerMaterialTransferUser()
	if err != nil {
		return wmodels.CollectionResponse[*models.MaterialTransferList]{}, err
	}
	if err := authorizeConstructionManagerMaterialTransferRole(
		user,
		constructionManagerMaterialTransferListPermission,
	); err != nil {
		return wmodels.CollectionResponse[*models.MaterialTransferList]{}, err
	}
	if err := s.requireDB(); err != nil {
		return wmodels.CollectionResponse[*models.MaterialTransferList]{}, err
	}

	query, params, err := validateConstructionManagerMaterialTransferInput(input)
	if err != nil {
		return wmodels.CollectionResponse[*models.MaterialTransferList]{}, err
	}

	poolConn, connID, err := s.DB.GetPrimary(ctx)
	if err != nil {
		return wmodels.CollectionResponse[*models.MaterialTransferList]{}, fmt.Errorf(
			"get primary connection for construction manager material transfers: %w",
			err,
		)
	}
	defer s.DB.Release(poolConn, connID)

	db := poolConn.Conn()
	available, err := materialBalanceSiteAvailable(ctx, db, user, query.ConstructionSiteID)
	if err != nil {
		return wmodels.CollectionResponse[*models.MaterialTransferList]{}, err
	}
	if !available {
		return wmodels.CollectionResponse[*models.MaterialTransferList]{},
			constructionManagerMaterialTransferSiteForbidden(query.ConstructionSiteID)
	}

	total := 0
	if err := db.QueryRow(ctx, `
		SELECT COUNT(*)::integer
		FROM public.material_transfers_list
		WHERE source_construction_site_id = $1
			OR destination_construction_site_id = $1
	`, query.ConstructionSiteID).Scan(&total); err != nil {
		return wmodels.CollectionResponse[*models.MaterialTransferList]{}, fmt.Errorf(
			"count construction manager material transfers: %w",
			err,
		)
	}

	rows, err := db.Query(ctx, `
		SELECT
			id,
			date,
			source_construction_site_id,
			destination_construction_site_id,
			comment,
			version,
			source_construction_site,
			destination_construction_site
		FROM public.material_transfers_list
		WHERE source_construction_site_id = $1
			OR destination_construction_site_id = $1
		ORDER BY date DESC, id DESC
		LIMIT $2 OFFSET $3
	`, query.ConstructionSiteID, int(params.Count), int(params.From))
	if err != nil {
		return wmodels.CollectionResponse[*models.MaterialTransferList]{}, fmt.Errorf(
			"select construction manager material transfers: %w",
			err,
		)
	}
	defer rows.Close()

	result := wmodels.CollectionResponse[*models.MaterialTransferList]{
		Rows: make([]*models.MaterialTransferList, 0),
		Agg:  &wmodels.TotCount{TotCount: total},
	}
	for rows.Next() {
		row := &models.MaterialTransferList{}
		if err := rows.Scan(
			&row.ID,
			&row.Date,
			&row.SourceConstructionSiteID,
			&row.DestinationConstructionSiteID,
			&row.Comment,
			&row.Version,
			&row.SourceConstructionSite,
			&row.DestinationConstructionSite,
		); err != nil {
			return wmodels.CollectionResponse[*models.MaterialTransferList]{}, fmt.Errorf(
				"scan construction manager material transfer: %w",
				err,
			)
		}
		result.Rows = append(result.Rows, row)
	}
	if err := rows.Err(); err != nil {
		return wmodels.CollectionResponse[*models.MaterialTransferList]{}, fmt.Errorf(
			"iterate construction manager material transfers: %w",
			err,
		)
	}

	return result, nil
}

func (s *MaterialTransferService) ConstructionManagerDestinations(
	ctx context.Context,
) (models.MaterialBalanceConstructionSitesResponse, error) {
	user, err := s.constructionManagerMaterialTransferUser()
	if err != nil {
		return models.MaterialBalanceConstructionSitesResponse{}, err
	}
	if err := authorizeConstructionManagerMaterialTransferRole(
		user,
		constructionManagerMaterialTransferCreatePermission,
	); err != nil {
		return models.MaterialBalanceConstructionSitesResponse{}, err
	}
	if err := s.requireDB(); err != nil {
		return models.MaterialBalanceConstructionSitesResponse{}, err
	}

	poolConn, connID, err := s.DB.GetPrimary(ctx)
	if err != nil {
		return models.MaterialBalanceConstructionSitesResponse{}, fmt.Errorf(
			"get primary connection for material transfer destinations: %w",
			err,
		)
	}
	defer s.DB.Release(poolConn, connID)

	rows, err := poolConn.Conn().Query(ctx, `
		SELECT site.id, site.name
		FROM public.construction_sites AS site
		WHERE site.is_active
		ORDER BY lower(site.name), site.id
	`)
	if err != nil {
		return models.MaterialBalanceConstructionSitesResponse{}, fmt.Errorf(
			"select material transfer destinations: %w",
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
				"scan material transfer destination: %w",
				err,
			)
		}
		result.Rows = append(result.Rows, item)
	}
	if err := rows.Err(); err != nil {
		return models.MaterialBalanceConstructionSitesResponse{}, fmt.Errorf(
			"iterate material transfer destinations: %w",
			err,
		)
	}
	result.Total = int64(len(result.Rows))

	return result, nil
}

func (s *MaterialTransferService) constructionManagerMaterialTransferUser() (models.UserLogin, error) {
	if s.Session == nil {
		return models.UserLogin{}, apperrors.SessionRequired()
	}

	user := models.UserLogin{}
	if err := s.Session.Get("user", &user); err != nil || user.ID <= 0 {
		return models.UserLogin{}, apperrors.SessionRequired()
	}

	return user, nil
}

func authorizeConstructionManagerMaterialTransferRole(user models.UserLogin, permission string) error {
	switch user.RoleID {
	case models.RoleIDAdmin, models.RoleIDConstructionSiteManager:
		return nil
	default:
		return apperrors.Forbidden(permission)
	}
}

func validateConstructionManagerMaterialTransferInput(
	input models.ConstructionManagerMaterialTransferInput,
) (*models.ConstructionManagerMaterialTransferQuery, modelbind.CollectionParams, error) {
	if input.Query == nil {
		return nil, modelbind.CollectionParams{}, webapp.BadRequest(
			"construction manager material transfer query is required",
			nil,
		)
	}
	if input.Query.ConstructionSiteID <= 0 {
		return nil, modelbind.CollectionParams{}, webapp.BadRequest(
			"construction_site_id should be positive",
			nil,
		)
	}

	params := input.Params
	if len(params.Filter) > 0 || len(params.Sorter) > 0 {
		return nil, modelbind.CollectionParams{}, webapp.BadRequest(
			"generic filters and sorting are not supported for construction manager material transfers",
			nil,
		)
	}
	if params.From < 0 {
		return nil, modelbind.CollectionParams{}, webapp.BadRequest("from should not be negative", nil)
	}
	if params.Count < 0 {
		return nil, modelbind.CollectionParams{}, webapp.BadRequest("count should not be negative", nil)
	}
	if params.Count == 0 {
		params.Count = constructionManagerMaterialTransferDefaultPageSize
	}
	if params.Count > constructionManagerMaterialTransferMaxPageSize {
		params.Count = constructionManagerMaterialTransferMaxPageSize
	}

	query := *input.Query
	return &query, params, nil
}

func validateConstructionManagerMaterialTransferDestination(
	ctx context.Context,
	tx ds.Querier,
	destinationConstructionSiteID int,
) error {
	var active bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM public.construction_sites AS site
			WHERE site.id = $1
				AND site.is_active
		)
	`, destinationConstructionSiteID).Scan(&active); err != nil {
		return err
	}
	if !active {
		return webapp.BadRequest(
			"destination_construction_site_id should reference an active construction site",
			map[string]any{"destination_construction_site_id": destinationConstructionSiteID},
		)
	}
	return nil
}

func validateConstructionManagerMaterialTransferCatalog(
	ctx context.Context,
	tx ds.Querier,
	items []*models.MaterialTransferDocumentItem,
) error {
	materialIDs := make([]int, len(items))
	measureUnitIDs := make([]int, len(items))
	for index, item := range items {
		materialIDs[index] = item.MaterialID
		measureUnitIDs[index] = item.MeasureUnitID
	}

	var index, materialID, measureUnitID int
	var expectedMeasureUnitID *int
	var materialExists, materialActive bool
	err := tx.QueryRow(ctx, `
		WITH submitted AS (
			SELECT
				input.material_id,
				input.measure_unit_id,
				input.ordinality::integer AS item_index
			FROM unnest(
				$1::integer[],
				$2::integer[]
			) WITH ORDINALITY AS input(material_id, measure_unit_id, ordinality)
		)
		SELECT
			submitted.item_index - 1,
			submitted.material_id,
			submitted.measure_unit_id,
			material.measure_unit_id,
			(material.id IS NOT NULL),
			(material.is_active IS TRUE)
		FROM submitted
		LEFT JOIN public.materials AS material
			ON material.id = submitted.material_id
		WHERE material.id IS NULL
			OR material.is_active IS NOT TRUE
			OR material.measure_unit_id <> submitted.measure_unit_id
		ORDER BY submitted.item_index
		LIMIT 1
	`, materialIDs, measureUnitIDs).Scan(
		&index,
		&materialID,
		&measureUnitID,
		&expectedMeasureUnitID,
		&materialExists,
		&materialActive,
	)
	if errors.Is(err, ds.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if !materialExists {
		return invalidMaterialDocumentItem(
			index,
			fmt.Sprintf("material_id %d does not reference a material", materialID),
		)
	}
	if !materialActive {
		return invalidMaterialDocumentItem(index, fmt.Sprintf("material_id %d is inactive", materialID))
	}
	return webapp.BadRequest(
		fmt.Sprintf("items[%d]: measure_unit_id does not match the selected material", index),
		map[string]any{
			"item_index":               index,
			"material_id":              materialID,
			"measure_unit_id":          measureUnitID,
			"expected_measure_unit_id": expectedMeasureUnitID,
		},
	)
}

func constructionManagerMaterialTransferSiteForbidden(constructionSiteID int) error {
	return webapp.Forbidden(
		"construction site is not available to the current user",
		map[string]any{
			"code":                 apperrors.CodeForbidden,
			"construction_site_id": constructionSiteID,
		},
	)
}
