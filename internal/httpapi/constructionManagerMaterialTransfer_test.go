package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dronm/skstruktura/internal/models"
)

func TestConstructionManagerMaterialTransferBinder(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/construction-manager/material-transfers?construction_site_id=42&from=10&count=25",
		nil,
	)

	bound, err := constructionManagerMaterialTransferBinder()(req)
	if err != nil {
		t.Fatalf("constructionManagerMaterialTransferBinder() error = %v", err)
	}
	input, ok := bound.(models.ConstructionManagerMaterialTransferInput)
	if !ok {
		t.Fatalf("binder result type = %T", bound)
	}
	if input.Query == nil || input.Query.ConstructionSiteID != 42 {
		t.Fatalf("binder query = %#v", input.Query)
	}
	if input.Params.From != 10 || input.Params.Count != 25 {
		t.Fatalf("binder collection params = %#v", input.Params)
	}
}

func TestConstructionManagerMaterialTransferBinderRequiresSite(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/construction-manager/material-transfers?from=0&count=25",
		nil,
	)

	if _, err := constructionManagerMaterialTransferBinder()(req); err == nil {
		t.Fatal("constructionManagerMaterialTransferBinder() error = nil, want required site error")
	}
}

func TestConstructionManagerMaterialTransferDocumentBinder(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/construction-manager/material-transfers",
		strings.NewReader(`{
			"date":"2026-09-21T00:00:00Z",
			"source_construction_site_id":42,
			"destination_construction_site_id":43,
			"comment":"test",
			"items":[{"material_id":7,"measure_unit_id":3,"quant":2}]
		}`),
	)
	req.Header.Set("Content-Type", "application/json")

	bound, err := constructionManagerMaterialTransferDocumentBinder()(req)
	if err != nil {
		t.Fatalf("constructionManagerMaterialTransferDocumentBinder() error = %v", err)
	}
	document, ok := bound.(*models.MaterialTransferDocument)
	if !ok {
		t.Fatalf("binder result type = %T", bound)
	}
	if document.SourceConstructionSiteID != 42 || document.DestinationConstructionSiteID != 43 {
		t.Fatalf("binder document = %#v", document)
	}
	if len(document.Items) != 1 || document.Items[0].MaterialID != 7 ||
		document.Items[0].MeasureUnitID != 3 || document.Items[0].Quant != 2 {
		t.Fatalf("binder document items = %#v", document.Items)
	}
}

func TestConstructionManagerMaterialTransferRoutes(t *testing.T) {
	t.Parallel()

	want := map[string]struct {
		method      string
		pattern     string
		permission  string
		serviceFunc string
		binder      bool
	}{
		"constructionManager.materialTransfer.create": {
			method:      http.MethodPost,
			pattern:     "/api/construction-manager/material-transfers",
			permission:  "constructionManager.materialTransfer.create",
			serviceFunc: "ConstructionManagerCreate",
			binder:      true,
		},
		"constructionManager.materialTransfer.list": {
			method:      http.MethodGet,
			pattern:     "/api/construction-manager/material-transfers",
			permission:  "constructionManager.materialTransfer.list",
			serviceFunc: "ConstructionManagerList",
			binder:      true,
		},
		"constructionManager.materialTransfer.detail": {
			method:      http.MethodGet,
			pattern:     "/api/construction-manager/material-transfers/{id}",
			permission:  "constructionManager.materialTransfer.list",
			serviceFunc: "ConstructionManagerDetail",
			binder:      true,
		},
		"constructionManager.materialTransfer.destinations": {
			method:      http.MethodGet,
			pattern:     "/api/construction-manager/transfer-destinations",
			permission:  "constructionManager.materialTransfer.create",
			serviceFunc: "ConstructionManagerDestinations",
		},
	}
	found := make(map[string]int, len(want))
	for _, route := range BuildRoutes() {
		expected, ok := want[route.Name]
		if !ok {
			continue
		}
		found[route.Name]++
		if route.Method != expected.method ||
			route.Pattern != expected.pattern ||
			route.Permission != expected.permission ||
			route.ServiceName != "MaterialTransfer" ||
			route.ServiceFunc != expected.serviceFunc ||
			(route.Binder != nil) != expected.binder {
			t.Errorf(
				"route %s = %s %s permission %q -> %s.%s binder=%t",
				route.Name,
				route.Method,
				route.Pattern,
				route.Permission,
				route.ServiceName,
				route.ServiceFunc,
				route.Binder != nil,
			)
		}
	}
	for name := range want {
		if found[name] != 1 {
			t.Errorf("route %s count = %d, want 1", name, found[name])
		}
	}
}
