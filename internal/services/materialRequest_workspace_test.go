package services

import (
	"testing"

	"github.com/dronm/modelbind"
	"github.com/dronm/skstruktura/internal/models"
	"github.com/dronm/webapp"
)

func TestValidateConstructionManagerMaterialRequestInput(t *testing.T) {
	t.Parallel()

	query, params, err := validateConstructionManagerMaterialRequestInput(
		models.ConstructionManagerMaterialRequestInput{
			Query: &models.ConstructionManagerMaterialRequestQuery{ConstructionSiteID: 42},
			Params: modelbind.CollectionParams{
				From:  10,
				Count: constructionManagerMaterialRequestMaxPageSize + 1,
			},
		},
	)
	if err != nil {
		t.Fatalf("validateConstructionManagerMaterialRequestInput() error = %v", err)
	}
	if query.ConstructionSiteID != 42 {
		t.Fatalf("construction site id = %d, want 42", query.ConstructionSiteID)
	}
	if params.From != 10 || params.Count != constructionManagerMaterialRequestMaxPageSize {
		t.Fatalf("collection params = %#v", params)
	}
}

func TestValidateConstructionManagerMaterialRequestInputUsesDefaultPageSize(t *testing.T) {
	t.Parallel()

	_, params, err := validateConstructionManagerMaterialRequestInput(
		models.ConstructionManagerMaterialRequestInput{
			Query: &models.ConstructionManagerMaterialRequestQuery{ConstructionSiteID: 42},
		},
	)
	if err != nil {
		t.Fatalf("validateConstructionManagerMaterialRequestInput() error = %v", err)
	}
	if params.Count != constructionManagerMaterialRequestDefaultPageSize {
		t.Fatalf(
			"collection count = %d, want %d",
			params.Count,
			constructionManagerMaterialRequestDefaultPageSize,
		)
	}
}

func TestValidateConstructionManagerMaterialRequestInputRejectsInvalidValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input models.ConstructionManagerMaterialRequestInput
	}{
		{name: "missing query"},
		{
			name: "invalid construction site",
			input: models.ConstructionManagerMaterialRequestInput{
				Query: &models.ConstructionManagerMaterialRequestQuery{},
			},
		},
		{
			name: "generic filter",
			input: models.ConstructionManagerMaterialRequestInput{
				Query: &models.ConstructionManagerMaterialRequestQuery{ConstructionSiteID: 1},
				Params: modelbind.CollectionParams{
					Filter: []modelbind.CollectionFilter{{}},
				},
			},
		},
		{
			name: "custom sorting",
			input: models.ConstructionManagerMaterialRequestInput{
				Query: &models.ConstructionManagerMaterialRequestQuery{ConstructionSiteID: 1},
				Params: modelbind.CollectionParams{
					Sorter: []modelbind.CollectionSorter{{Field: "date"}},
				},
			},
		},
		{
			name: "negative from",
			input: models.ConstructionManagerMaterialRequestInput{
				Query:  &models.ConstructionManagerMaterialRequestQuery{ConstructionSiteID: 1},
				Params: modelbind.CollectionParams{From: -1},
			},
		},
		{
			name: "negative count",
			input: models.ConstructionManagerMaterialRequestInput{
				Query:  &models.ConstructionManagerMaterialRequestQuery{ConstructionSiteID: 1},
				Params: modelbind.CollectionParams{Count: -1},
			},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if _, _, err := validateConstructionManagerMaterialRequestInput(test.input); err == nil {
				t.Fatal("validateConstructionManagerMaterialRequestInput() error = nil, want error")
			}
		})
	}
}

func TestAuthorizeConstructionManagerMaterialRequestRole(t *testing.T) {
	t.Parallel()

	for _, roleID := range []models.RoleID{
		models.RoleIDAdmin,
		models.RoleIDConstructionSiteManager,
	} {
		if err := authorizeConstructionManagerMaterialRequestRole(models.UserLogin{RoleID: roleID}); err != nil {
			t.Errorf("role %q error = %v", roleID, err)
		}
	}

	for _, roleID := range []models.RoleID{
		models.RoleIDAccountant,
		models.RoleIDSupplyManager,
		models.RoleID("unknown"),
	} {
		err := authorizeConstructionManagerMaterialRequestRole(models.UserLogin{RoleID: roleID})
		appErr, ok := err.(*webapp.AppError)
		if !ok || appErr.StatusCode() != 403 {
			t.Errorf("role %q error = %#v, want HTTP 403", roleID, err)
		}
	}
}
