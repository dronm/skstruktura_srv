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
	constructionManagerMaterialConsumptionCreatePermission = "constructionManager.materialConsumption.create"
	constructionManagerMaterialConsumptionListPermission   = "constructionManager.materialConsumption.list"

	constructionManagerMaterialConsumptionDefaultPageSize = 30
	constructionManagerMaterialConsumptionMaxPageSize     = 100
)

func (s *MaterialConsumptionService) ConstructionManagerCreate(
	ctx context.Context,
	document *models.MaterialConsumptionDocument,
) (*models.MaterialConsumptionDocument, error) {
	user, err := s.constructionManagerMaterialConsumptionUser()
	if err != nil {
		return nil, err
	}
	if err := authorizeConstructionManagerMaterialConsumptionRole(
		user,
		constructionManagerMaterialConsumptionCreatePermission,
	); err != nil {
		return nil, err
	}
	if err := s.requireDB(); err != nil {
		return nil, err
	}
	if err := validateMaterialConsumptionDocument(document, true, 0); err != nil {
		return nil, err
	}

	var result *models.MaterialConsumptionDocument
	if err := withPrimaryTransaction(ctx, s.DB, func(tx ds.Tx) error {
		available, err := materialBalanceSiteAvailable(ctx, tx, user, document.ConstructionSiteID)
		if err != nil {
			return err
		}
		if !available {
			return constructionManagerMaterialConsumptionSiteForbidden(document.ConstructionSiteID)
		}
		if err := validateConstructionManagerMaterialConsumptionCatalog(ctx, tx, document.Items); err != nil {
			return err
		}

		result, err = createMaterialConsumptionDocument(ctx, tx, document)
		return err
	}); err != nil {
		return nil, fmt.Errorf("create construction manager material consumption: %w", err)
	}

	return result, nil
}

func (s *MaterialConsumptionService) ConstructionManagerList(
	ctx context.Context,
	input models.ConstructionManagerMaterialConsumptionInput,
) (wmodels.CollectionResponse[*models.MaterialConsumptionList], error) {
	user, err := s.constructionManagerMaterialConsumptionUser()
	if err != nil {
		return wmodels.CollectionResponse[*models.MaterialConsumptionList]{}, err
	}
	if err := authorizeConstructionManagerMaterialConsumptionRole(
		user,
		constructionManagerMaterialConsumptionListPermission,
	); err != nil {
		return wmodels.CollectionResponse[*models.MaterialConsumptionList]{}, err
	}
	if err := s.requireDB(); err != nil {
		return wmodels.CollectionResponse[*models.MaterialConsumptionList]{}, err
	}

	query, params, err := validateConstructionManagerMaterialConsumptionInput(input)
	if err != nil {
		return wmodels.CollectionResponse[*models.MaterialConsumptionList]{}, err
	}

	poolConn, connID, err := s.DB.GetPrimary(ctx)
	if err != nil {
		return wmodels.CollectionResponse[*models.MaterialConsumptionList]{}, fmt.Errorf(
			"get primary connection for construction manager material consumptions: %w",
			err,
		)
	}
	defer s.DB.Release(poolConn, connID)

	db := poolConn.Conn()
	available, err := materialBalanceSiteAvailable(ctx, db, user, query.ConstructionSiteID)
	if err != nil {
		return wmodels.CollectionResponse[*models.MaterialConsumptionList]{}, err
	}
	if !available {
		return wmodels.CollectionResponse[*models.MaterialConsumptionList]{},
			constructionManagerMaterialConsumptionSiteForbidden(query.ConstructionSiteID)
	}

	total := 0
	if err := db.QueryRow(ctx, `
		SELECT COUNT(*)::integer
		FROM public.material_consumptions_list
		WHERE construction_site_id = $1
	`, query.ConstructionSiteID).Scan(&total); err != nil {
		return wmodels.CollectionResponse[*models.MaterialConsumptionList]{}, fmt.Errorf(
			"count construction manager material consumptions: %w",
			err,
		)
	}

	rows, err := db.Query(ctx, `
		SELECT
			id,
			date,
			construction_site_id,
			comment,
			version,
			construction_site
		FROM public.material_consumptions_list
		WHERE construction_site_id = $1
		ORDER BY date DESC, id DESC
		LIMIT $2 OFFSET $3
	`, query.ConstructionSiteID, int(params.Count), int(params.From))
	if err != nil {
		return wmodels.CollectionResponse[*models.MaterialConsumptionList]{}, fmt.Errorf(
			"select construction manager material consumptions: %w",
			err,
		)
	}
	defer rows.Close()

	result := wmodels.CollectionResponse[*models.MaterialConsumptionList]{
		Rows: make([]*models.MaterialConsumptionList, 0),
		Agg:  &wmodels.TotCount{TotCount: total},
	}
	for rows.Next() {
		row := &models.MaterialConsumptionList{}
		if err := rows.Scan(
			&row.ID,
			&row.Date,
			&row.ConstructionSiteID,
			&row.Comment,
			&row.Version,
			&row.ConstructionSite,
		); err != nil {
			return wmodels.CollectionResponse[*models.MaterialConsumptionList]{}, fmt.Errorf(
				"scan construction manager material consumption: %w",
				err,
			)
		}
		result.Rows = append(result.Rows, row)
	}
	if err := rows.Err(); err != nil {
		return wmodels.CollectionResponse[*models.MaterialConsumptionList]{}, fmt.Errorf(
			"iterate construction manager material consumptions: %w",
			err,
		)
	}

	return result, nil
}

func (s *MaterialConsumptionService) constructionManagerMaterialConsumptionUser() (models.UserLogin, error) {
	if s.Session == nil {
		return models.UserLogin{}, apperrors.SessionRequired()
	}

	user := models.UserLogin{}
	if err := s.Session.Get("user", &user); err != nil || user.ID <= 0 {
		return models.UserLogin{}, apperrors.SessionRequired()
	}

	return user, nil
}

func authorizeConstructionManagerMaterialConsumptionRole(user models.UserLogin, permission string) error {
	switch user.RoleID {
	case models.RoleIDAdmin, models.RoleIDConstructionSiteManager:
		return nil
	default:
		return apperrors.Forbidden(permission)
	}
}

func validateConstructionManagerMaterialConsumptionInput(
	input models.ConstructionManagerMaterialConsumptionInput,
) (*models.ConstructionManagerMaterialConsumptionQuery, modelbind.CollectionParams, error) {
	if input.Query == nil {
		return nil, modelbind.CollectionParams{}, webapp.BadRequest(
			"construction manager material consumption query is required",
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
			"generic filters and sorting are not supported for construction manager material consumptions",
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
		params.Count = constructionManagerMaterialConsumptionDefaultPageSize
	}
	if params.Count > constructionManagerMaterialConsumptionMaxPageSize {
		params.Count = constructionManagerMaterialConsumptionMaxPageSize
	}

	query := *input.Query
	return &query, params, nil
}

func validateConstructionManagerMaterialConsumptionCatalog(
	ctx context.Context,
	tx ds.Querier,
	items []*models.MaterialConsumptionDocumentItem,
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

func constructionManagerMaterialConsumptionSiteForbidden(constructionSiteID int) error {
	return webapp.Forbidden(
		"construction site is not available to the current user",
		map[string]any{
			"code":                 apperrors.CodeForbidden,
			"construction_site_id": constructionSiteID,
		},
	)
}
