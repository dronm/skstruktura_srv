package services

import (
	"testing"
	"time"

	"github.com/dronm/skstruktura/internal/models"
	"github.com/dronm/webapp"
)

func TestValidateMaterialReceiptDocumentNormalizesLineNumbers(t *testing.T) {
	document := &models.MaterialReceiptDocument{
		Date:               time.Date(2026, time.August, 22, 0, 0, 0, 0, time.UTC),
		ConstructionSiteID: intPtr(10),
		SupplierID:         20,
		Number:             "  R-1  ",
		Items: []*models.MaterialReceiptDocumentItem{
			{LineNum: 99, MaterialID: 30, MeasureUnitID: 40, Quant: 2, Price: 3, Amount: 6},
			{LineNum: 55, MaterialID: 31, MeasureUnitID: 40, Quant: 1, Price: 4, Amount: 4},
		},
	}

	if err := validateMaterialReceiptDocument(document, true, 0); err != nil {
		t.Fatalf("validateMaterialReceiptDocument() error = %v", err)
	}
	if document.Number != "R-1" {
		t.Fatalf("number = %q, want R-1", document.Number)
	}
	if document.Items[0].LineNum != 1 || document.Items[1].LineNum != 2 {
		t.Fatalf("line numbers = [%d %d], want [1 2]", document.Items[0].LineNum, document.Items[1].LineNum)
	}
}

func TestValidateMaterialDocumentUpdateIdentity(t *testing.T) {
	document := validMaterialConsumptionDocument()
	document.ID = 7
	document.Version = 3
	document.Items[0].ID = 11

	if err := validateMaterialConsumptionDocument(document, false, 7); err != nil {
		t.Fatalf("valid update error = %v", err)
	}

	document.Items = append(document.Items, &models.MaterialConsumptionDocumentItem{
		ID:            11,
		MaterialID:    31,
		MeasureUnitID: 40,
		Quant:         1,
	})
	if err := validateMaterialConsumptionDocument(document, false, 7); err == nil {
		t.Fatal("duplicate item id was accepted")
	}
}

func TestValidateMaterialDocumentRejectsServerGeneratedCreateValues(t *testing.T) {
	document := validMaterialConsumptionDocument()
	document.Version = 1

	err := validateMaterialConsumptionDocument(document, true, 0)
	if err == nil {
		t.Fatal("create version was accepted")
	}
	if appErr, ok := err.(*webapp.AppError); !ok || appErr.StatusCode() != 400 {
		t.Fatalf("error = %#v, want HTTP 400", err)
	}
}

func TestValidateMaterialTransferRejectsSameSite(t *testing.T) {
	document := &models.MaterialTransferDocument{
		Date:                          time.Date(2026, time.August, 22, 0, 0, 0, 0, time.UTC),
		SourceConstructionSiteID:      10,
		DestinationConstructionSiteID: 10,
		Items: []*models.MaterialTransferDocumentItem{
			{MaterialID: 30, MeasureUnitID: 40, Quant: 2},
		},
	}

	if err := validateMaterialTransferDocument(document, true, 0); err == nil {
		t.Fatal("transfer with the same source and destination was accepted")
	}
}

func TestValidateMaterialDocumentRequiresItems(t *testing.T) {
	document := validMaterialConsumptionDocument()
	document.Items = nil

	if err := validateMaterialConsumptionDocument(document, true, 0); err == nil {
		t.Fatal("document without items was accepted")
	}
}

func validMaterialConsumptionDocument() *models.MaterialConsumptionDocument {
	return &models.MaterialConsumptionDocument{
		Date:               time.Date(2026, time.August, 22, 0, 0, 0, 0, time.UTC),
		ConstructionSiteID: 10,
		Items: []*models.MaterialConsumptionDocumentItem{
			{MaterialID: 30, MeasureUnitID: 40, Quant: 2},
		},
	}
}

func TestValidateMaterialReceiptSites(t *testing.T) {
	tests := []struct {
		name      string
		header    *int
		sites     []*int
		wantError bool
	}{
		{name: "inherit header", header: intPtr(10), sites: []*int{nil, nil}},
		{name: "override and inherit", header: intPtr(10), sites: []*int{intPtr(20), nil}},
		{name: "all lines have sites", sites: []*int{intPtr(10), intPtr(20)}},
		{name: "missing site on one line", sites: []*int{intPtr(10), nil}, wantError: true},
		{name: "missing both sites", sites: []*int{nil}, wantError: true},
		{name: "zero header", header: intPtr(0), sites: []*int{intPtr(10)}, wantError: true},
		{name: "negative line site", header: intPtr(10), sites: []*int{intPtr(-1)}, wantError: true},
		{name: "zero line site cannot inherit", header: intPtr(10), sites: []*int{intPtr(0)}, wantError: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			document := &models.MaterialReceiptDocument{
				Date:               time.Date(2026, time.September, 9, 8, 0, 0, 0, time.UTC),
				ConstructionSiteID: test.header,
				SupplierID:         1,
				Number:             "R-sites",
			}
			for _, site := range test.sites {
				document.Items = append(document.Items, &models.MaterialReceiptDocumentItem{
					MaterialID:         1,
					MeasureUnitID:      1,
					ConstructionSiteID: site,
					Quant:              1,
				})
			}
			err := validateMaterialReceiptDocument(document, true, 0)
			if (err != nil) != test.wantError {
				t.Fatalf("validation error = %v, wantError = %v", err, test.wantError)
			}
			if !test.wantError {
				// Keep null as inheritance; copying the header onto items would
				// prevent a later header edit from moving inherited lines.
				for i, site := range test.sites {
					if document.Items[i].ConstructionSiteID != site {
						t.Fatalf("line %d override changed during validation", i)
					}
				}
			}
		})
	}
}
