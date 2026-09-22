package services

import (
	"testing"
	"time"

	"github.com/dronm/modelbind"
	"github.com/dronm/skstruktura/internal/models"
	"github.com/dronm/webapp"
)

func TestAuthorizeSupplyManagerRole(t *testing.T) {
	t.Parallel()

	for _, roleID := range []models.RoleID{
		models.RoleIDAdmin,
		models.RoleIDSupplyManager,
	} {
		if err := authorizeSupplyManagerRole(
			models.UserLogin{RoleID: roleID},
			supplyManagerAssignmentCreatePermission,
		); err != nil {
			t.Errorf("role %q error = %v", roleID, err)
		}
	}

	for _, roleID := range []models.RoleID{
		models.RoleIDConstructionSiteManager,
		models.RoleIDAccountant,
		models.RoleID("unknown"),
	} {
		err := authorizeSupplyManagerRole(
			models.UserLogin{RoleID: roleID},
			supplyManagerAssignmentCreatePermission,
		)
		appErr, ok := err.(*webapp.AppError)
		if !ok || appErr.StatusCode() != 403 {
			t.Errorf("role %q error = %#v, want HTTP 403", roleID, err)
		}
	}
}

func TestValidateSupplyManagerMaterialRequestInput(t *testing.T) {
	t.Parallel()

	siteID := 42
	importanceID := 3
	materialSearch := "  cement  "
	dateFrom := time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC)
	dateTo := time.Date(2026, time.September, 30, 23, 59, 59, 0, time.UTC)
	query, params, err := validateSupplyManagerMaterialRequestInput(
		models.SupplyManagerMaterialRequestInput{
			Query: &models.SupplyManagerMaterialRequestQuery{
				ConstructionSiteID: &siteID,
				DateFrom:           &dateFrom,
				DateTo:             &dateTo,
				MaterialSearch:     &materialSearch,
				OrderImportanceID:  &importanceID,
			},
			Params: modelbind.CollectionParams{
				From:  10,
				Count: supplyManagerMaterialRequestMaxPageSize + 1,
			},
		},
	)
	if err != nil {
		t.Fatalf("validateSupplyManagerMaterialRequestInput() error = %v", err)
	}
	if query.MaterialSearch == nil || *query.MaterialSearch != "cement" {
		t.Fatalf("material search = %#v, want trimmed value", query.MaterialSearch)
	}
	if params.From != 10 || params.Count != supplyManagerMaterialRequestMaxPageSize {
		t.Fatalf("collection params = %#v", params)
	}
}

func TestValidateSupplyManagerMaterialRequestInputUsesDefaults(t *testing.T) {
	t.Parallel()

	emptySearch := "  "
	query, params, err := validateSupplyManagerMaterialRequestInput(
		models.SupplyManagerMaterialRequestInput{
			Query: &models.SupplyManagerMaterialRequestQuery{MaterialSearch: &emptySearch},
		},
	)
	if err != nil {
		t.Fatalf("validateSupplyManagerMaterialRequestInput() error = %v", err)
	}
	if query.MaterialSearch != nil {
		t.Fatalf("material search = %#v, want nil", query.MaterialSearch)
	}
	if params.Count != supplyManagerMaterialRequestDefaultPageSize {
		t.Fatalf("collection count = %d, want %d", params.Count, supplyManagerMaterialRequestDefaultPageSize)
	}
}

func TestValidateSupplyManagerMaterialRequestInputRejectsInvalidValues(t *testing.T) {
	t.Parallel()

	zero := 0
	date := time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC)
	later := date.AddDate(0, 0, 1)
	tests := []struct {
		name  string
		input models.SupplyManagerMaterialRequestInput
	}{
		{name: "missing query"},
		{
			name: "invalid site",
			input: models.SupplyManagerMaterialRequestInput{
				Query: &models.SupplyManagerMaterialRequestQuery{ConstructionSiteID: &zero},
			},
		},
		{
			name: "invalid importance",
			input: models.SupplyManagerMaterialRequestInput{
				Query: &models.SupplyManagerMaterialRequestQuery{OrderImportanceID: &zero},
			},
		},
		{
			name: "reversed dates",
			input: models.SupplyManagerMaterialRequestInput{
				Query: &models.SupplyManagerMaterialRequestQuery{DateFrom: &later, DateTo: &date},
			},
		},
		{
			name: "generic filter",
			input: models.SupplyManagerMaterialRequestInput{
				Query:  &models.SupplyManagerMaterialRequestQuery{},
				Params: modelbind.CollectionParams{Filter: []modelbind.CollectionFilter{{}}},
			},
		},
		{
			name: "generic sorter",
			input: models.SupplyManagerMaterialRequestInput{
				Query: &models.SupplyManagerMaterialRequestQuery{},
				Params: modelbind.CollectionParams{
					Sorter: []modelbind.CollectionSorter{{Field: "date"}},
				},
			},
		},
		{
			name: "negative from",
			input: models.SupplyManagerMaterialRequestInput{
				Query:  &models.SupplyManagerMaterialRequestQuery{},
				Params: modelbind.CollectionParams{From: -1},
			},
		},
		{
			name: "negative count",
			input: models.SupplyManagerMaterialRequestInput{
				Query:  &models.SupplyManagerMaterialRequestQuery{},
				Params: modelbind.CollectionParams{Count: -1},
			},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if _, _, err := validateSupplyManagerMaterialRequestInput(test.input); err == nil {
				t.Fatal("validateSupplyManagerMaterialRequestInput() error = nil, want error")
			}
		})
	}
}

func TestValidateSupplyManagerAssignmentHistoryInput(t *testing.T) {
	t.Parallel()

	query, params, err := validateSupplyManagerAssignmentHistoryInput(
		models.SupplyManagerAssignmentHistoryInput{
			Query: &models.SupplyManagerAssignmentHistoryQuery{},
			Params: modelbind.CollectionParams{
				From:  5,
				Count: supplyManagerAssignmentMaxPageSize + 1,
			},
		},
	)
	if err != nil {
		t.Fatalf("validateSupplyManagerAssignmentHistoryInput() error = %v", err)
	}
	if query == nil {
		t.Fatal("validated history query is nil")
	}
	if params.From != 5 || params.Count != supplyManagerAssignmentMaxPageSize {
		t.Fatalf("collection params = %#v", params)
	}
}

func TestValidateSupplyManagerAssignmentHistoryInputRejectsInvalidValues(t *testing.T) {
	t.Parallel()

	zero := 0
	tests := []models.SupplyManagerAssignmentHistoryInput{
		{},
		{Query: &models.SupplyManagerAssignmentHistoryQuery{ConstructionSiteID: &zero}},
		{
			Query:  &models.SupplyManagerAssignmentHistoryQuery{},
			Params: modelbind.CollectionParams{From: -1},
		},
		{
			Query:  &models.SupplyManagerAssignmentHistoryQuery{},
			Params: modelbind.CollectionParams{Count: -1},
		},
	}

	for index, input := range tests {
		if _, _, err := validateSupplyManagerAssignmentHistoryInput(input); err == nil {
			t.Errorf("case %d error = nil, want error", index)
		}
	}
}
