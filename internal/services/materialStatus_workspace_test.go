package services

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/dronm/ds/v4"
	"github.com/dronm/modelbind"
	"github.com/dronm/skstruktura/internal/models"
	"github.com/dronm/webapp"
)

func TestValidateConstructionManagerMaterialStatusInput(t *testing.T) {
	t.Parallel()

	materialTypeID := 7
	query, params, err := validateConstructionManagerMaterialStatusInput(
		models.ConstructionManagerMaterialStatusInput{
			Query: &models.ConstructionManagerMaterialStatusQuery{
				ConstructionSiteID: 42,
				MaterialTypeID:     &materialTypeID,
			},
			Params: modelbind.CollectionParams{
				From:  10,
				Count: constructionManagerMaterialStatusMaxPageSize + 1,
			},
		},
	)
	if err != nil {
		t.Fatalf("validateConstructionManagerMaterialStatusInput() error = %v", err)
	}
	if query.ConstructionSiteID != 42 || query.MaterialTypeID == nil || *query.MaterialTypeID != 7 {
		t.Fatalf("query = %#v", query)
	}
	if params.From != 10 || params.Count != constructionManagerMaterialStatusMaxPageSize {
		t.Fatalf("params = %#v", params)
	}
}

func TestValidateConstructionManagerMaterialStatusInputUsesDefaultPageSize(t *testing.T) {
	t.Parallel()

	_, params, err := validateConstructionManagerMaterialStatusInput(
		models.ConstructionManagerMaterialStatusInput{
			Query: &models.ConstructionManagerMaterialStatusQuery{ConstructionSiteID: 42},
		},
	)
	if err != nil {
		t.Fatalf("validateConstructionManagerMaterialStatusInput() error = %v", err)
	}
	if params.Count != constructionManagerMaterialStatusDefaultPageSize {
		t.Fatalf("count = %d, want %d", params.Count, constructionManagerMaterialStatusDefaultPageSize)
	}
}

func TestValidateConstructionManagerMaterialStatusInputRejectsInvalidValues(t *testing.T) {
	t.Parallel()

	zero := 0
	tests := []struct {
		name  string
		input models.ConstructionManagerMaterialStatusInput
	}{
		{name: "missing query"},
		{
			name: "invalid construction site",
			input: models.ConstructionManagerMaterialStatusInput{
				Query: &models.ConstructionManagerMaterialStatusQuery{},
			},
		},
		{
			name: "invalid material type",
			input: models.ConstructionManagerMaterialStatusInput{
				Query: &models.ConstructionManagerMaterialStatusQuery{
					ConstructionSiteID: 1,
					MaterialTypeID:     &zero,
				},
			},
		},
		{
			name: "generic filter",
			input: models.ConstructionManagerMaterialStatusInput{
				Query: &models.ConstructionManagerMaterialStatusQuery{ConstructionSiteID: 1},
				Params: modelbind.CollectionParams{
					Filter: []modelbind.CollectionFilter{{}},
				},
			},
		},
		{
			name: "custom sorting",
			input: models.ConstructionManagerMaterialStatusInput{
				Query: &models.ConstructionManagerMaterialStatusQuery{ConstructionSiteID: 1},
				Params: modelbind.CollectionParams{
					Sorter: []modelbind.CollectionSorter{{Field: "created_at"}},
				},
			},
		},
		{
			name: "negative from",
			input: models.ConstructionManagerMaterialStatusInput{
				Query:  &models.ConstructionManagerMaterialStatusQuery{ConstructionSiteID: 1},
				Params: modelbind.CollectionParams{From: -1},
			},
		},
		{
			name: "negative count",
			input: models.ConstructionManagerMaterialStatusInput{
				Query:  &models.ConstructionManagerMaterialStatusQuery{ConstructionSiteID: 1},
				Params: modelbind.CollectionParams{Count: -1},
			},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if _, _, err := validateConstructionManagerMaterialStatusInput(test.input); err == nil {
				t.Fatal("validateConstructionManagerMaterialStatusInput() error = nil, want error")
			}
		})
	}
}

func TestAuthorizeConstructionManagerMaterialStatusRole(t *testing.T) {
	t.Parallel()

	for _, roleID := range []models.RoleID{models.RoleIDAdmin, models.RoleIDConstructionSiteManager} {
		for _, permission := range []string{
			constructionManagerMaterialStatusListPermission,
			constructionManagerMaterialStatusCreatePermission,
		} {
			if err := authorizeConstructionManagerMaterialStatusRole(
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
		err := authorizeConstructionManagerMaterialStatusRole(
			models.UserLogin{RoleID: roleID},
			constructionManagerMaterialStatusListPermission,
		)
		assertMaterialStatusHTTPStatus(t, "role authorization", err, 403)
	}
}

func TestValidateConstructionManagerMaterialStatusChange(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.September, 21, 10, 0, 0, 0, time.UTC)
	valid := func() *models.ConstructionManagerMaterialStatusChangeRequest {
		return &models.ConstructionManagerMaterialStatusChangeRequest{
			ConstructionSiteID: 42,
			MaterialID:         17,
			CreatedAt:          now,
			ExpectedStatus:     models.MaterialStatusTypeAtWork,
			TargetStatus:       models.MaterialStatusTypeOnMaintenance,
		}
	}

	if err := validateConstructionManagerMaterialStatusChange(valid(), now); err != nil {
		t.Fatalf("valid change error = %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*models.ConstructionManagerMaterialStatusChangeRequest)
	}{
		{name: "invalid site", mutate: func(v *models.ConstructionManagerMaterialStatusChangeRequest) { v.ConstructionSiteID = 0 }},
		{name: "invalid material", mutate: func(v *models.ConstructionManagerMaterialStatusChangeRequest) { v.MaterialID = 0 }},
		{name: "missing date", mutate: func(v *models.ConstructionManagerMaterialStatusChangeRequest) { v.CreatedAt = time.Time{} }},
		{name: "future date", mutate: func(v *models.ConstructionManagerMaterialStatusChangeRequest) { v.CreatedAt = now.Add(6 * time.Minute) }},
		{name: "invalid expected status", mutate: func(v *models.ConstructionManagerMaterialStatusChangeRequest) { v.ExpectedStatus = "invalid" }},
		{name: "invalid target status", mutate: func(v *models.ConstructionManagerMaterialStatusChangeRequest) { v.TargetStatus = "invalid" }},
		{name: "not reverse", mutate: func(v *models.ConstructionManagerMaterialStatusChangeRequest) {
			v.TargetStatus = models.MaterialStatusTypeAtWork
		}},
		{name: "invalid expected record id", mutate: func(v *models.ConstructionManagerMaterialStatusChangeRequest) {
			id := 0
			v.ExpectedStatusRecordID = &id
		}},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			request := valid()
			test.mutate(request)
			assertMaterialStatusHTTPStatus(
				t,
				test.name,
				validateConstructionManagerMaterialStatusChange(request, now),
				400,
			)
		})
	}
}

func TestValidateConstructionManagerMaterialStatusTransition(t *testing.T) {
	t.Parallel()

	latestID := 9
	requestID := 9
	latestTime := time.Date(2026, time.September, 21, 9, 0, 0, 0, time.UTC)
	request := &models.ConstructionManagerMaterialStatusChangeRequest{
		MaterialID:             17,
		CreatedAt:              latestTime,
		ExpectedStatusRecordID: &requestID,
		ExpectedStatus:         models.MaterialStatusTypeOnMaintenance,
		TargetStatus:           models.MaterialStatusTypeAtWork,
	}
	latest := constructionManagerMaterialStatusLatest{
		ID:        &latestID,
		CreatedAt: &latestTime,
		Status:    models.MaterialStatusTypeOnMaintenance,
	}
	if err := validateConstructionManagerMaterialStatusTransition(request, latest); err != nil {
		t.Fatalf("valid transition error = %v", err)
	}

	staleID := *request
	wrongID := 8
	staleID.ExpectedStatusRecordID = &wrongID
	assertMaterialStatusHTTPStatus(
		t,
		"stale record id",
		validateConstructionManagerMaterialStatusTransition(&staleID, latest),
		409,
	)

	staleStatus := *request
	staleStatus.ExpectedStatus = models.MaterialStatusTypeAtWork
	assertMaterialStatusHTTPStatus(
		t,
		"stale status",
		validateConstructionManagerMaterialStatusTransition(&staleStatus, latest),
		409,
	)

	notReverse := *request
	notReverse.TargetStatus = models.MaterialStatusTypeOnMaintenance
	assertMaterialStatusHTTPStatus(
		t,
		"not reverse",
		validateConstructionManagerMaterialStatusTransition(&notReverse, latest),
		400,
	)

	backdated := *request
	backdated.CreatedAt = latestTime.Add(-time.Nanosecond)
	assertMaterialStatusHTTPStatus(
		t,
		"backdated",
		validateConstructionManagerMaterialStatusTransition(&backdated, latest),
		400,
	)
}

func TestLockConstructionManagerMaterialStatusMaterialUsesRequiredLocks(t *testing.T) {
	t.Parallel()

	querier := &materialStatusQuerierStub{
		row: materialStatusRowStub{
			values: []any{"Compressor SN-17", 7, "Equipment"},
		},
	}
	material, err := lockConstructionManagerMaterialStatusMaterial(
		context.Background(),
		querier,
		42,
		17,
	)
	if err != nil {
		t.Fatalf("lock material error = %v", err)
	}
	if material.Name != "Compressor SN-17" || material.MaterialTypeID != 7 ||
		material.MaterialTypeName != "Equipment" {
		t.Fatalf("locked material = %#v", material)
	}
	if !strings.Contains(querier.query, "FOR UPDATE OF material, balance") {
		t.Fatalf("lock query does not lock material and balance: %s", querier.query)
	}
}

func TestLockConstructionManagerMaterialStatusMaterialRejectsUnavailableMaterial(t *testing.T) {
	t.Parallel()

	_, err := lockConstructionManagerMaterialStatusMaterial(
		context.Background(),
		&materialStatusQuerierStub{row: materialStatusRowStub{err: ds.ErrNoRows}},
		42,
		17,
	)
	assertMaterialStatusHTTPStatus(t, "unavailable material", err, 400)
}

func TestFetchConstructionManagerMaterialStatusLatestDefaultsAndStaysGlobal(t *testing.T) {
	t.Parallel()

	querier := &materialStatusQuerierStub{row: materialStatusRowStub{err: ds.ErrNoRows}}
	latest, err := fetchConstructionManagerMaterialStatusLatest(context.Background(), querier, 17)
	if err != nil {
		t.Fatalf("fetch latest error = %v", err)
	}
	if latest.ID != nil || latest.CreatedAt != nil || latest.Status != models.MaterialStatusTypeAtWork {
		t.Fatalf("latest default = %#v", latest)
	}
	if strings.Contains(querier.query, "construction_site_id") {
		t.Fatalf("latest global query unexpectedly filters by construction site: %s", querier.query)
	}
	if !strings.Contains(querier.query, "created_at DESC, status_row.id DESC") {
		t.Fatalf("latest query lacks stable newest-first order: %s", querier.query)
	}
}

func TestValidateConstructionManagerMaterialStatusHistoricalBalance(t *testing.T) {
	t.Parallel()

	request := &models.ConstructionManagerMaterialStatusChangeRequest{
		ConstructionSiteID: 42,
		MaterialID:         17,
		CreatedAt:          time.Date(2026, time.September, 21, 9, 0, 0, 0, time.UTC),
	}
	if err := validateConstructionManagerMaterialStatusHistoricalBalance(
		context.Background(),
		&materialStatusQuerierStub{row: materialStatusRowStub{values: []any{1.0}}},
		request,
	); err != nil {
		t.Fatalf("positive historical balance error = %v", err)
	}
	err := validateConstructionManagerMaterialStatusHistoricalBalance(
		context.Background(),
		&materialStatusQuerierStub{row: materialStatusRowStub{values: []any{0.0}}},
		request,
	)
	assertMaterialStatusHTTPStatus(t, "missing historical balance", err, 400)
}

type materialStatusQuerierStub struct {
	query string
	row   materialStatusRowStub
}

func (q *materialStatusQuerierStub) Exec(context.Context, string, ...any) (ds.ExecResult, error) {
	return nil, errors.New("unexpected Exec call")
}

func (q *materialStatusQuerierStub) Query(context.Context, string, ...any) (ds.Rows, error) {
	return nil, errors.New("unexpected Query call")
}

func (q *materialStatusQuerierStub) QueryRow(_ context.Context, query string, _ ...any) ds.Row {
	q.query = query
	return q.row
}

type materialStatusRowStub struct {
	values []any
	err    error
}

func (r materialStatusRowStub) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(dest) != len(r.values) {
		return errors.New("unexpected scan destination count")
	}
	for index, value := range r.values {
		switch target := dest[index].(type) {
		case *string:
			*target = value.(string)
		case *int:
			*target = value.(int)
		case *float64:
			*target = value.(float64)
		case *time.Time:
			*target = value.(time.Time)
		case *models.MaterialStatusType:
			*target = value.(models.MaterialStatusType)
		default:
			return errors.New("unexpected scan destination type")
		}
	}
	return nil
}

func assertMaterialStatusHTTPStatus(t *testing.T, name string, err error, want int) {
	t.Helper()

	var appErr *webapp.AppError
	if !errors.As(err, &appErr) || appErr.StatusCode() != want {
		t.Errorf("%s error = %#v, want HTTP %d", name, err, want)
	}
}
