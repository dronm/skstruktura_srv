package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dronm/skstruktura/internal/models"
)

func TestConstructionManagerMaterialConsumptionBinder(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/construction-manager/material-consumptions?construction_site_id=42&from=10&count=25",
		nil,
	)

	bound, err := constructionManagerMaterialConsumptionBinder()(req)
	if err != nil {
		t.Fatalf("constructionManagerMaterialConsumptionBinder() error = %v", err)
	}
	input, ok := bound.(models.ConstructionManagerMaterialConsumptionInput)
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

func TestConstructionManagerMaterialConsumptionBinderRequiresSite(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/construction-manager/material-consumptions?from=0&count=25",
		nil,
	)

	if _, err := constructionManagerMaterialConsumptionBinder()(req); err == nil {
		t.Fatal("constructionManagerMaterialConsumptionBinder() error = nil, want required site error")
	}
}

func TestConstructionManagerMaterialConsumptionDocumentBinder(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/construction-manager/material-consumptions",
		strings.NewReader(`{
			"date":"2026-09-21T00:00:00Z",
			"construction_site_id":42,
			"comment":null,
			"items":[{"material_id":7,"measure_unit_id":3,"quant":2}]
		}`),
	)
	req.Header.Set("Content-Type", "application/json")

	bound, err := constructionManagerMaterialConsumptionDocumentBinder()(req)
	if err != nil {
		t.Fatalf("constructionManagerMaterialConsumptionDocumentBinder() error = %v", err)
	}
	document, ok := bound.(*models.MaterialConsumptionDocument)
	if !ok {
		t.Fatalf("binder result type = %T", bound)
	}
	if document.ConstructionSiteID != 42 || len(document.Items) != 1 {
		t.Fatalf("binder document = %#v", document)
	}
	if document.Items[0].MaterialID != 7 || document.Items[0].MeasureUnitID != 3 || document.Items[0].Quant != 2 {
		t.Fatalf("binder document item = %#v", document.Items[0])
	}
}

func TestConstructionManagerMaterialConsumptionRoutes(t *testing.T) {
	t.Parallel()

	want := map[string]struct {
		method      string
		permission  string
		serviceFunc string
	}{
		"constructionManager.materialConsumption.create": {
			method:      http.MethodPost,
			permission:  "constructionManager.materialConsumption.create",
			serviceFunc: "ConstructionManagerCreate",
		},
		"constructionManager.materialConsumption.list": {
			method:      http.MethodGet,
			permission:  "constructionManager.materialConsumption.list",
			serviceFunc: "ConstructionManagerList",
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
			route.Pattern != "/api/construction-manager/material-consumptions" ||
			route.Permission != expected.permission ||
			route.ServiceName != "MaterialConsumption" ||
			route.ServiceFunc != expected.serviceFunc ||
			route.Binder == nil {
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
