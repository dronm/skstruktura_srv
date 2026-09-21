package services

import (
	"testing"

	"github.com/dronm/modelbind"
	"github.com/dronm/skstruktura/internal/models"
)

func TestValidateMaterialBalanceInput(t *testing.T) {
	t.Parallel()

	query, params, err := validateMaterialBalanceInput(models.MaterialBalanceInput{
		Query: &models.MaterialBalanceQuery{ConstructionSiteID: 42},
		Params: modelbind.CollectionParams{
			From:  10,
			Count: materialBalanceMaxPageSize + 1,
		},
	})
	if err != nil {
		t.Fatalf("validateMaterialBalanceInput() error = %v", err)
	}
	if query.ConstructionSiteID != 42 {
		t.Fatalf("construction site id = %d, want 42", query.ConstructionSiteID)
	}
	if params.From != 10 || params.Count != materialBalanceMaxPageSize {
		t.Fatalf("collection params = %#v", params)
	}
}

func TestValidateMaterialBalanceInputUsesDefaultPageSize(t *testing.T) {
	t.Parallel()

	_, params, err := validateMaterialBalanceInput(models.MaterialBalanceInput{
		Query: &models.MaterialBalanceQuery{ConstructionSiteID: 42},
	})
	if err != nil {
		t.Fatalf("validateMaterialBalanceInput() error = %v", err)
	}
	if params.Count != materialBalanceDefaultPageSize {
		t.Fatalf("collection count = %d, want %d", params.Count, materialBalanceDefaultPageSize)
	}
}

func TestValidateMaterialBalanceInputRejectsInvalidValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input models.MaterialBalanceInput
	}{
		{
			name: "missing query",
		},
		{
			name: "invalid construction site",
			input: models.MaterialBalanceInput{
				Query: &models.MaterialBalanceQuery{},
			},
		},
		{
			name: "custom sorting",
			input: models.MaterialBalanceInput{
				Query: &models.MaterialBalanceQuery{ConstructionSiteID: 1},
				Params: modelbind.CollectionParams{
					Sorter: []modelbind.CollectionSorter{{Field: "balance"}},
				},
			},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if _, _, err := validateMaterialBalanceInput(test.input); err == nil {
				t.Fatal("validateMaterialBalanceInput() error = nil, want error")
			}
		})
	}
}
