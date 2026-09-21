package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dronm/skstruktura/internal/integrations/diadoc"
	"github.com/dronm/skstruktura/internal/models"
	"github.com/dronm/webapp"
)

func TestDiadocImportRoutes(t *testing.T) {
	routes := BuildRoutes(&diadoc.Manager{})
	if err := webapp.ValidateRoutes(routes); err != nil {
		t.Fatalf("ValidateRoutes() error = %v", err)
	}
	want := map[string]string{
		"diadocDocument.list":        "/api/diadoc/documents",
		"diadocDocument.detail":      "/api/diadoc/documents/{id}",
		"diadocDocument.resolve":     "/api/diadoc/documents/{id}/resolution",
		"diadocDocument.itemExclude": "/api/diadoc/documents/{id}/items/{itemId}/exclude",
		"diadocDocument.itemRestore": "/api/diadoc/documents/{id}/items/{itemId}/restore",
		"diadocDocument.ignore":      "/api/diadoc/documents/{id}",
		"diadocDocument.import":      "/api/diadoc/documents/{id}/import",
		"diadocState.view":           "/api/diadoc/state",
		"diadocState.update":         "/api/diadoc/state",
		"diadocState.replay":         "/api/diadoc/state/replay",
		"diadoc.sync":                "/api/diadoc/sync",
	}
	found := make(map[string]bool)
	for _, route := range routes {
		if pattern, ok := want[route.Name]; ok {
			found[route.Name] = true
			if route.Pattern != pattern {
				t.Errorf("route %s pattern = %s, want %s", route.Name, route.Pattern, pattern)
			}
		}
	}
	for name := range want {
		if !found[name] {
			t.Errorf("route %s is missing", name)
		}
	}
}

func TestDiadocResolutionBinder(t *testing.T) {
	request := httptest.NewRequest(
		http.MethodPut,
		"/api/diadoc/documents/42/resolution",
		strings.NewReader(`{
			"version":3,
			"supplier_id":10,
			"construction_site_id":20,
			"receipt_number":"R-10",
			"receipt_date":"2026-08-28T00:00:00Z",
			"receipt_comment":"Imported receipt",
			"items":[{"id":30,"material_id":40,"conversion_factor":"1"}]
		}`),
	)
	request.SetPathValue("id", "42")

	bound, err := diadocResolutionBinder()(request)
	if err != nil {
		t.Fatalf("diadocResolutionBinder() error = %v", err)
	}
	input, ok := bound.(models.DiadocResolutionInput)
	if !ok {
		t.Fatalf("binder result type = %T", bound)
	}
	if input.ID != 42 ||
		input.Request.Version != 3 ||
		input.Request.ReceiptNumber != "R-10" ||
		input.Request.ReceiptDate == nil ||
		len(input.Request.Items) != 1 {
		t.Fatalf("binder result = %#v", input)
	}
}

func TestDiadocResolutionBinderWithItemSite(t *testing.T) {
	request := httptest.NewRequest(http.MethodPut, "/api/diadoc/documents/42/resolution", strings.NewReader(`{
		"version":3,"supplier_id":10,"construction_site_id":null,
		"receipt_number":"R-10","receipt_date":"2026-08-28T00:00:00Z",
		"items":[{"id":30,"material_id":40,"construction_site_id":25,"conversion_factor":"1"}]
	}`))
	request.SetPathValue("id", "42")
	bound, err := diadocResolutionBinder()(request)
	if err != nil {
		t.Fatal(err)
	}
	input := bound.(models.DiadocResolutionInput)
	if input.Request.ConstructionSiteID != nil {
		t.Fatal("nullable header site was not preserved")
	}
	if len(input.Request.Items) != 1 || input.Request.Items[0].ConstructionSiteID == nil || *input.Request.Items[0].ConstructionSiteID != 25 {
		t.Fatalf("item construction site was not bound: %#v", input.Request.Items)
	}
}

func TestDiadocItemVersionBinder(t *testing.T) {
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/diadoc/documents/42/items/30/exclude",
		strings.NewReader(`{"version":3}`),
	)
	request.SetPathValue("id", "42")
	request.SetPathValue("itemId", "30")

	bound, err := diadocItemVersionBinder()(request)
	if err != nil {
		t.Fatalf("diadocItemVersionBinder() error = %v", err)
	}
	input, ok := bound.(models.DiadocItemVersionInput)
	if !ok {
		t.Fatalf("binder result type = %T", bound)
	}
	if input.ID != 42 || input.ItemID != 30 || input.Request.Version != 3 {
		t.Fatalf("binder result = %#v", input)
	}
}
