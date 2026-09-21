package services

import (
	"context"
	"errors"
	"testing"

	"github.com/dronm/ds/v4"
	"github.com/dronm/modelbind"
	"github.com/dronm/skstruktura/internal/models"
	"github.com/dronm/webapp"
)

func TestValidateConstructionManagerMaterialConsumptionInput(t *testing.T) {
	t.Parallel()

	query, params, err := validateConstructionManagerMaterialConsumptionInput(
		models.ConstructionManagerMaterialConsumptionInput{
			Query: &models.ConstructionManagerMaterialConsumptionQuery{ConstructionSiteID: 42},
			Params: modelbind.CollectionParams{
				From:  10,
				Count: constructionManagerMaterialConsumptionMaxPageSize + 1,
			},
		},
	)
	if err != nil {
		t.Fatalf("validateConstructionManagerMaterialConsumptionInput() error = %v", err)
	}
	if query.ConstructionSiteID != 42 {
		t.Fatalf("construction site id = %d, want 42", query.ConstructionSiteID)
	}
	if params.From != 10 || params.Count != constructionManagerMaterialConsumptionMaxPageSize {
		t.Fatalf("collection params = %#v", params)
	}
}

func TestValidateConstructionManagerMaterialConsumptionInputUsesDefaultPageSize(t *testing.T) {
	t.Parallel()

	_, params, err := validateConstructionManagerMaterialConsumptionInput(
		models.ConstructionManagerMaterialConsumptionInput{
			Query: &models.ConstructionManagerMaterialConsumptionQuery{ConstructionSiteID: 42},
		},
	)
	if err != nil {
		t.Fatalf("validateConstructionManagerMaterialConsumptionInput() error = %v", err)
	}
	if params.Count != constructionManagerMaterialConsumptionDefaultPageSize {
		t.Fatalf(
			"collection count = %d, want %d",
			params.Count,
			constructionManagerMaterialConsumptionDefaultPageSize,
		)
	}
}

func TestValidateConstructionManagerMaterialConsumptionInputRejectsInvalidValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input models.ConstructionManagerMaterialConsumptionInput
	}{
		{name: "missing query"},
		{
			name: "invalid construction site",
			input: models.ConstructionManagerMaterialConsumptionInput{
				Query: &models.ConstructionManagerMaterialConsumptionQuery{},
			},
		},
		{
			name: "generic filter",
			input: models.ConstructionManagerMaterialConsumptionInput{
				Query: &models.ConstructionManagerMaterialConsumptionQuery{ConstructionSiteID: 1},
				Params: modelbind.CollectionParams{
					Filter: []modelbind.CollectionFilter{{}},
				},
			},
		},
		{
			name: "custom sorting",
			input: models.ConstructionManagerMaterialConsumptionInput{
				Query: &models.ConstructionManagerMaterialConsumptionQuery{ConstructionSiteID: 1},
				Params: modelbind.CollectionParams{
					Sorter: []modelbind.CollectionSorter{{Field: "date"}},
				},
			},
		},
		{
			name: "negative from",
			input: models.ConstructionManagerMaterialConsumptionInput{
				Query:  &models.ConstructionManagerMaterialConsumptionQuery{ConstructionSiteID: 1},
				Params: modelbind.CollectionParams{From: -1},
			},
		},
		{
			name: "negative count",
			input: models.ConstructionManagerMaterialConsumptionInput{
				Query:  &models.ConstructionManagerMaterialConsumptionQuery{ConstructionSiteID: 1},
				Params: modelbind.CollectionParams{Count: -1},
			},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if _, _, err := validateConstructionManagerMaterialConsumptionInput(test.input); err == nil {
				t.Fatal("validateConstructionManagerMaterialConsumptionInput() error = nil, want error")
			}
		})
	}
}

func TestAuthorizeConstructionManagerMaterialConsumptionRole(t *testing.T) {
	t.Parallel()

	for _, roleID := range []models.RoleID{
		models.RoleIDAdmin,
		models.RoleIDConstructionSiteManager,
	} {
		if err := authorizeConstructionManagerMaterialConsumptionRole(
			models.UserLogin{RoleID: roleID},
			constructionManagerMaterialConsumptionCreatePermission,
		); err != nil {
			t.Errorf("role %q error = %v", roleID, err)
		}
	}

	for _, roleID := range []models.RoleID{
		models.RoleIDAccountant,
		models.RoleIDSupplier,
		models.RoleID("unknown"),
	} {
		err := authorizeConstructionManagerMaterialConsumptionRole(
			models.UserLogin{RoleID: roleID},
			constructionManagerMaterialConsumptionListPermission,
		)
		appErr, ok := err.(*webapp.AppError)
		if !ok || appErr.StatusCode() != 403 {
			t.Errorf("role %q error = %#v, want HTTP 403", roleID, err)
		}
	}
}

func TestValidateConstructionManagerMaterialConsumptionCatalog(t *testing.T) {
	t.Parallel()

	expectedUnitID := 7
	tests := []struct {
		name           string
		row            materialConsumptionCatalogRow
		wantStatusCode int
	}{
		{
			name: "valid catalog entries",
			row:  materialConsumptionCatalogRow{err: ds.ErrNoRows},
		},
		{
			name: "missing material",
			row: materialConsumptionCatalogRow{
				index:         0,
				materialID:    11,
				measureUnitID: 7,
			},
			wantStatusCode: 400,
		},
		{
			name: "inactive material",
			row: materialConsumptionCatalogRow{
				index:                 0,
				materialID:            11,
				measureUnitID:         7,
				expectedMeasureUnitID: &expectedUnitID,
				materialExists:        true,
			},
			wantStatusCode: 400,
		},
		{
			name: "mismatched unit",
			row: materialConsumptionCatalogRow{
				index:                 0,
				materialID:            11,
				measureUnitID:         8,
				expectedMeasureUnitID: &expectedUnitID,
				materialExists:        true,
				materialActive:        true,
			},
			wantStatusCode: 400,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			querier := &materialConsumptionCatalogQuerier{row: test.row}
			err := validateConstructionManagerMaterialConsumptionCatalog(
				context.Background(),
				querier,
				[]*models.MaterialConsumptionDocumentItem{
					{MaterialID: 11, MeasureUnitID: 7, Quant: 1},
				},
			)
			if test.wantStatusCode == 0 {
				if err != nil {
					t.Fatalf("catalog validation error = %v", err)
				}
				return
			}

			appErr, ok := err.(*webapp.AppError)
			if !ok || appErr.StatusCode() != test.wantStatusCode {
				t.Fatalf("catalog validation error = %#v, want HTTP %d", err, test.wantStatusCode)
			}
		})
	}
}

type materialConsumptionCatalogQuerier struct {
	row materialConsumptionCatalogRow
}

func (q *materialConsumptionCatalogQuerier) Exec(context.Context, string, ...any) (ds.ExecResult, error) {
	return nil, errors.New("unexpected Exec call")
}

func (q *materialConsumptionCatalogQuerier) Query(context.Context, string, ...any) (ds.Rows, error) {
	return nil, errors.New("unexpected Query call")
}

func (q *materialConsumptionCatalogQuerier) QueryRow(context.Context, string, ...any) ds.Row {
	return q.row
}

type materialConsumptionCatalogRow struct {
	index                 int
	materialID            int
	measureUnitID         int
	expectedMeasureUnitID *int
	materialExists        bool
	materialActive        bool
	err                   error
}

func (r materialConsumptionCatalogRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(dest) != 6 {
		return errors.New("unexpected catalog validation destination count")
	}

	*(dest[0].(*int)) = r.index
	*(dest[1].(*int)) = r.materialID
	*(dest[2].(*int)) = r.measureUnitID
	*(dest[3].(**int)) = r.expectedMeasureUnitID
	*(dest[4].(*bool)) = r.materialExists
	*(dest[5].(*bool)) = r.materialActive
	return nil
}
