package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dronm/skstruktura/internal/models"
	"github.com/dronm/webapp"
)

func TestMaterialDocumentRoutesReplaceGeneratedEndpoints(t *testing.T) {
	routes := BuildRoutes()
	if err := webapp.ValidateRoutes(routes); err != nil {
		t.Fatalf("ValidateRoutes() error = %v", err)
	}

	want := map[string]struct {
		method      string
		pattern     string
		serviceFunc string
	}{
		"materialReceipt.create":     {http.MethodPost, "/api/material-receipts", "Create"},
		"materialReceipt.detail":     {http.MethodGet, "/api/material-receipts/{id}", "DocumentDetail"},
		"materialReceipt.update":     {http.MethodPut, "/api/material-receipts/{id}", "Update"},
		"materialConsumption.create": {http.MethodPost, "/api/material-consumptions", "Create"},
		"materialConsumption.detail": {http.MethodGet, "/api/material-consumptions/{id}", "DocumentDetail"},
		"materialConsumption.update": {http.MethodPut, "/api/material-consumptions/{id}", "Update"},
		"materialTransfer.create":    {http.MethodPost, "/api/material-transfers", "Create"},
		"materialTransfer.detail":    {http.MethodGet, "/api/material-transfers/{id}", "DocumentDetail"},
		"materialTransfer.update":    {http.MethodPut, "/api/material-transfers/{id}", "Update"},
	}

	found := make(map[string]int, len(want))
	for _, route := range routes {
		if strings.HasPrefix(route.Name, "materialReceiptItem.") ||
			strings.HasPrefix(route.Name, "materialConsumptionItem.") ||
			strings.HasPrefix(route.Name, "materialTransferItem.") {
			t.Errorf("standalone item route is still exposed: %s", route.Name)
		}

		expected, ok := want[route.Name]
		if !ok {
			continue
		}
		found[route.Name]++
		if route.Method != expected.method ||
			route.Pattern != expected.pattern ||
			route.ServiceFunc != expected.serviceFunc {
			t.Errorf(
				"route %s = %s %s -> %s, want %s %s -> %s",
				route.Name,
				route.Method,
				route.Pattern,
				route.ServiceFunc,
				expected.method,
				expected.pattern,
				expected.serviceFunc,
			)
		}
	}

	for name := range want {
		if found[name] != 1 {
			t.Errorf("route %s count = %d, want 1", name, found[name])
		}
	}
}

func TestMaterialReceiptDocumentBinder(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/material-receipts", strings.NewReader(`{
		"date":"2026-08-22T08:54:23+03:00",
		"construction_site_id":10,
		"supplier_id":20,
		"number":"R-1",
		"items":[{
			"material_id":30,
			"measure_unit_id":40,
			"quant":2.5,
			"price":3,
			"amount":7.5,
			"vat_percent":20,
			"vat_amount":1.25
		}]
	}`))

	bound, err := documentJSONBinder[models.MaterialReceiptDocument]()(req)
	if err != nil {
		t.Fatalf("document binder error = %v", err)
	}
	document, ok := bound.(*models.MaterialReceiptDocument)
	if !ok {
		t.Fatalf("binder result type = %T", bound)
	}
	if document.Number != "R-1" || len(document.Items) != 1 || document.Items[0].Quant != 2.5 {
		t.Fatalf("binder result = %#v", document)
	}
	if document.Items[0].VatPercent != 20 || document.Items[0].VatAmount != 1.25 {
		t.Fatalf("receipt VAT fields = %#v", document.Items[0])
	}
	if got := document.Date.Format(time.RFC3339); got != "2026-08-22T08:54:23+03:00" {
		t.Fatalf("document date = %s, want timestamp with time", got)
	}
}

func TestMaterialDocumentUpdateBinderUsesPathID(t *testing.T) {
	req := httptest.NewRequest(http.MethodPut, "/api/material-transfers/42", strings.NewReader(`{
		"version":3,
		"date":"2026-08-22T00:00:00Z",
		"source_construction_site_id":10,
		"destination_construction_site_id":20,
		"items":[{
			"material_id":30,
			"measure_unit_id":40,
			"quant":2
		}]
	}`))
	req.SetPathValue("id", "42")

	bound, err := materialTransferDocumentUpdateBinder()(req)
	if err != nil {
		t.Fatalf("update binder error = %v", err)
	}
	input, ok := bound.(models.UpdateMaterialTransferDocumentRequest)
	if !ok {
		t.Fatalf("binder result type = %T", bound)
	}
	if input.ID != 42 || input.Document.Version != 3 {
		t.Fatalf("binder result = %#v", input)
	}
}

func TestMaterialDocumentBinderRejectsUnknownAndTrailingJSON(t *testing.T) {
	tests := []string{
		`{"date":"2026-08-22T00:00:00Z","unknown":true}`,
		`{} {}`,
	}
	for _, body := range tests {
		req := httptest.NewRequest(http.MethodPost, "/api/material-consumptions", strings.NewReader(body))
		if _, err := documentJSONBinder[models.MaterialConsumptionDocument]()(req); err == nil {
			t.Fatalf("body %q was accepted", body)
		}
	}
}

func TestMaterialReceiptDocumentBinderWithLineSite(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/material-receipts", strings.NewReader(`{
		"date":"2026-09-09T08:00:00Z",
		"construction_site_id":null,
		"supplier_id":20,
		"number":"R-sites",
		"items":[{
			"material_id":30,
			"measure_unit_id":40,
			"construction_site_id":10,
			"quant":2,
			"price":3,
			"amount":6
		}]
	}`))
	bound, err := documentJSONBinder[models.MaterialReceiptDocument]()(req)
	if err != nil {
		t.Fatal(err)
	}
	document := bound.(*models.MaterialReceiptDocument)
	if document.ConstructionSiteID != nil || len(document.Items) != 1 ||
		document.Items[0].ConstructionSiteID == nil || *document.Items[0].ConstructionSiteID != 10 {
		t.Fatalf("line-site receipt was not decoded correctly: %#v", document)
	}
}
