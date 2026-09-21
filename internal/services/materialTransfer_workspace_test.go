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

func TestValidateConstructionManagerMaterialTransferInput(t *testing.T) {
	t.Parallel()

	query, params, err := validateConstructionManagerMaterialTransferInput(
		models.ConstructionManagerMaterialTransferInput{
			Query: &models.ConstructionManagerMaterialTransferQuery{ConstructionSiteID: 42},
			Params: modelbind.CollectionParams{
				From:  10,
				Count: constructionManagerMaterialTransferMaxPageSize + 1,
			},
		},
	)
	if err != nil {
		t.Fatalf("validateConstructionManagerMaterialTransferInput() error = %v", err)
	}
	if query.ConstructionSiteID != 42 {
		t.Fatalf("construction site id = %d, want 42", query.ConstructionSiteID)
	}
	if params.From != 10 || params.Count != constructionManagerMaterialTransferMaxPageSize {
		t.Fatalf("collection params = %#v", params)
	}
}

func TestValidateConstructionManagerMaterialTransferInputUsesDefaultPageSize(t *testing.T) {
	t.Parallel()

	_, params, err := validateConstructionManagerMaterialTransferInput(
		models.ConstructionManagerMaterialTransferInput{
			Query: &models.ConstructionManagerMaterialTransferQuery{ConstructionSiteID: 42},
		},
	)
	if err != nil {
		t.Fatalf("validateConstructionManagerMaterialTransferInput() error = %v", err)
	}
	if params.Count != constructionManagerMaterialTransferDefaultPageSize {
		t.Fatalf(
			"collection count = %d, want %d",
			params.Count,
			constructionManagerMaterialTransferDefaultPageSize,
		)
	}
}

func TestValidateConstructionManagerMaterialTransferInputRejectsInvalidValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input models.ConstructionManagerMaterialTransferInput
	}{
		{name: "missing query"},
		{
			name: "invalid construction site",
			input: models.ConstructionManagerMaterialTransferInput{
				Query: &models.ConstructionManagerMaterialTransferQuery{},
			},
		},
		{
			name: "generic filter",
			input: models.ConstructionManagerMaterialTransferInput{
				Query: &models.ConstructionManagerMaterialTransferQuery{ConstructionSiteID: 1},
				Params: modelbind.CollectionParams{
					Filter: []modelbind.CollectionFilter{{}},
				},
			},
		},
		{
			name: "custom sorting",
			input: models.ConstructionManagerMaterialTransferInput{
				Query: &models.ConstructionManagerMaterialTransferQuery{ConstructionSiteID: 1},
				Params: modelbind.CollectionParams{
					Sorter: []modelbind.CollectionSorter{{Field: "date"}},
				},
			},
		},
		{
			name: "negative from",
			input: models.ConstructionManagerMaterialTransferInput{
				Query:  &models.ConstructionManagerMaterialTransferQuery{ConstructionSiteID: 1},
				Params: modelbind.CollectionParams{From: -1},
			},
		},
		{
			name: "negative count",
			input: models.ConstructionManagerMaterialTransferInput{
				Query:  &models.ConstructionManagerMaterialTransferQuery{ConstructionSiteID: 1},
				Params: modelbind.CollectionParams{Count: -1},
			},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if _, _, err := validateConstructionManagerMaterialTransferInput(test.input); err == nil {
				t.Fatal("validateConstructionManagerMaterialTransferInput() error = nil, want error")
			}
		})
	}
}

func TestAuthorizeConstructionManagerMaterialTransferRole(t *testing.T) {
	t.Parallel()

	for _, roleID := range []models.RoleID{
		models.RoleIDAdmin,
		models.RoleIDConstructionSiteManager,
	} {
		for _, permission := range []string{
			constructionManagerMaterialTransferCreatePermission,
			constructionManagerMaterialTransferListPermission,
		} {
			if err := authorizeConstructionManagerMaterialTransferRole(
				models.UserLogin{RoleID: roleID},
				permission,
			); err != nil {
				t.Errorf("role %q permission %q error = %v", roleID, permission, err)
			}
		}
	}

	for _, roleID := range []models.RoleID{
		models.RoleIDAccountant,
		models.RoleIDSupplier,
		models.RoleID("unknown"),
	} {
		err := authorizeConstructionManagerMaterialTransferRole(
			models.UserLogin{RoleID: roleID},
			constructionManagerMaterialTransferListPermission,
		)
		assertMaterialTransferHTTPStatus(t, "role authorization", err, 403)
	}
}

func TestValidateConstructionManagerMaterialTransferDestination(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		row        materialTransferDestinationRow
		wantStatus int
		wantError  bool
	}{
		{name: "active destination", row: materialTransferDestinationRow{active: true}},
		{name: "missing or inactive destination", wantStatus: 400},
		{name: "database error", row: materialTransferDestinationRow{err: errors.New("db error")}, wantError: true},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			err := validateConstructionManagerMaterialTransferDestination(
				context.Background(),
				&materialTransferDestinationQuerier{row: test.row},
				42,
			)
			if test.wantStatus != 0 {
				assertMaterialTransferHTTPStatus(t, "destination validation", err, test.wantStatus)
				return
			}
			if test.wantError {
				if err == nil {
					t.Fatal("destination validation error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("destination validation error = %v", err)
			}
		})
	}
}

func TestValidateConstructionManagerMaterialTransferCatalog(t *testing.T) {
	t.Parallel()

	expectedUnitID := 7
	tests := []struct {
		name           string
		row            materialTransferCatalogRow
		wantStatusCode int
	}{
		{name: "valid catalog entries", row: materialTransferCatalogRow{err: ds.ErrNoRows}},
		{
			name:           "missing material",
			row:            materialTransferCatalogRow{index: 0, materialID: 11, measureUnitID: 7},
			wantStatusCode: 400,
		},
		{
			name: "inactive material",
			row: materialTransferCatalogRow{
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
			row: materialTransferCatalogRow{
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

			err := validateConstructionManagerMaterialTransferCatalog(
				context.Background(),
				&materialTransferCatalogQuerier{row: test.row},
				[]*models.MaterialTransferDocumentItem{
					{MaterialID: 11, MeasureUnitID: 7, Quant: 1},
				},
			)
			if test.wantStatusCode == 0 {
				if err != nil {
					t.Fatalf("catalog validation error = %v", err)
				}
				return
			}
			assertMaterialTransferHTTPStatus(t, "catalog validation", err, test.wantStatusCode)
		})
	}
}

type materialTransferDestinationQuerier struct {
	row materialTransferDestinationRow
}

func (q *materialTransferDestinationQuerier) Exec(context.Context, string, ...any) (ds.ExecResult, error) {
	return nil, errors.New("unexpected Exec call")
}

func (q *materialTransferDestinationQuerier) Query(context.Context, string, ...any) (ds.Rows, error) {
	return nil, errors.New("unexpected Query call")
}

func (q *materialTransferDestinationQuerier) QueryRow(context.Context, string, ...any) ds.Row {
	return q.row
}

type materialTransferDestinationRow struct {
	active bool
	err    error
}

func (r materialTransferDestinationRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(dest) != 1 {
		return errors.New("unexpected destination validation destination count")
	}
	*(dest[0].(*bool)) = r.active
	return nil
}

type materialTransferCatalogQuerier struct {
	row materialTransferCatalogRow
}

func (q *materialTransferCatalogQuerier) Exec(context.Context, string, ...any) (ds.ExecResult, error) {
	return nil, errors.New("unexpected Exec call")
}

func (q *materialTransferCatalogQuerier) Query(context.Context, string, ...any) (ds.Rows, error) {
	return nil, errors.New("unexpected Query call")
}

func (q *materialTransferCatalogQuerier) QueryRow(context.Context, string, ...any) ds.Row {
	return q.row
}

type materialTransferCatalogRow struct {
	index                 int
	materialID            int
	measureUnitID         int
	expectedMeasureUnitID *int
	materialExists        bool
	materialActive        bool
	err                   error
}

func (r materialTransferCatalogRow) Scan(dest ...any) error {
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

func assertMaterialTransferHTTPStatus(t *testing.T, name string, err error, want int) {
	t.Helper()

	var appErr *webapp.AppError
	if !errors.As(err, &appErr) || appErr.StatusCode() != want {
		t.Errorf("%s error = %#v, want HTTP %d", name, err, want)
	}
}
