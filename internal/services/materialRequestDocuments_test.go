package services

import (
	"testing"
	"time"

	"github.com/dronm/skstruktura/internal/models"
)

func TestValidateMaterialRequestDocumentAllowsNullableLineFields(t *testing.T) {
	t.Parallel()

	document := validMaterialRequestDocument()
	document.Items = append(document.Items, &models.MaterialRequestDocumentItem{
		LineNum:           77,
		MaterialID:        31,
		MeasureUnitID:     41,
		Quant:             1.5,
		SupplierID:        nil,
		RequiredDate:      nil,
		OrderImportanceID: 51,
	})

	if err := validateMaterialRequestDocument(document, true, 0); err != nil {
		t.Fatalf("validateMaterialRequestDocument() error = %v", err)
	}
	for index, item := range document.Items {
		if want := index + 1; item.LineNum != want {
			t.Errorf("items[%d].LineNum = %d, want %d", index, item.LineNum, want)
		}
	}
}

func TestValidateMaterialRequestDocumentAcceptsDateOnly(t *testing.T) {
	t.Parallel()

	document := validMaterialRequestDocument()
	requiredDate := models.DateOnly("2026-10-15")
	document.Items[0].RequiredDate = &requiredDate

	if err := validateMaterialRequestDocument(document, true, 0); err != nil {
		t.Fatalf("validateMaterialRequestDocument() error = %v", err)
	}
}

func TestValidateMaterialRequestDocumentRejectsInvalidFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*models.MaterialRequestDocument)
	}{
		{
			name: "missing construction site",
			mutate: func(document *models.MaterialRequestDocument) {
				document.ConstructionSiteID = 0
			},
		},
		{
			name: "missing construction manager",
			mutate: func(document *models.MaterialRequestDocument) {
				document.ConstructionManagerID = 0
			},
		},
		{
			name: "missing items",
			mutate: func(document *models.MaterialRequestDocument) {
				document.Items = nil
			},
		},
		{
			name: "non-positive quantity",
			mutate: func(document *models.MaterialRequestDocument) {
				document.Items[0].Quant = 0
			},
		},
		{
			name: "missing importance",
			mutate: func(document *models.MaterialRequestDocument) {
				document.Items[0].OrderImportanceID = 0
			},
		},
		{
			name: "zero supplier",
			mutate: func(document *models.MaterialRequestDocument) {
				zero := 0
				document.Items[0].SupplierID = &zero
			},
		},
		{
			name: "invalid required date",
			mutate: func(document *models.MaterialRequestDocument) {
				invalidDate := models.DateOnly("2026-02-30")
				document.Items[0].RequiredDate = &invalidDate
			},
		},
		{
			name: "negative create status",
			mutate: func(document *models.MaterialRequestDocument) {
				document.Items[0].StatusID = -1
			},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			document := validMaterialRequestDocument()
			test.mutate(document)
			if err := validateMaterialRequestDocument(document, true, 0); err == nil {
				t.Fatal("validateMaterialRequestDocument() error = nil, want validation error")
			}
		})
	}
}

func TestValidateMaterialRequestDocumentUpdateRequiresStatusAndVersion(t *testing.T) {
	t.Parallel()

	document := validMaterialRequestDocument()
	document.ID = 17
	document.Version = 3
	document.Items[0].ID = 23
	document.Items[0].StatusID = 1
	if err := validateMaterialRequestDocument(document, false, 17); err != nil {
		t.Fatalf("valid update error = %v", err)
	}

	document.Items[0].StatusID = 0
	if err := validateMaterialRequestDocument(document, false, 17); err == nil {
		t.Fatal("update with an empty item status was accepted")
	}

	document.Items[0].StatusID = 1
	document.Version = 0
	if err := validateMaterialRequestDocument(document, false, 17); err == nil {
		t.Fatal("update without a version was accepted")
	}
}

func validMaterialRequestDocument() *models.MaterialRequestDocument {
	return &models.MaterialRequestDocument{
		Date:                  time.Date(2026, time.September, 21, 8, 30, 0, 0, time.UTC),
		ConstructionSiteID:    10,
		ConstructionManagerID: 20,
		Items: []*models.MaterialRequestDocumentItem{
			{
				LineNum:           99,
				MaterialID:        30,
				MeasureUnitID:     40,
				Quant:             2,
				OrderImportanceID: 50,
			},
		},
	}
}
