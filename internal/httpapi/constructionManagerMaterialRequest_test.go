package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dronm/skstruktura/internal/models"
)

func TestConstructionManagerMaterialRequestBinder(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/construction-manager/material-requests?construction_site_id=42&from=10&count=25",
		nil,
	)

	bound, err := constructionManagerMaterialRequestBinder()(req)
	if err != nil {
		t.Fatalf("constructionManagerMaterialRequestBinder() error = %v", err)
	}
	input, ok := bound.(models.ConstructionManagerMaterialRequestInput)
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

func TestConstructionManagerMaterialRequestBinderRequiresSite(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/construction-manager/material-requests?from=0&count=25",
		nil,
	)

	if _, err := constructionManagerMaterialRequestBinder()(req); err == nil {
		t.Fatal("constructionManagerMaterialRequestBinder() error = nil, want required site error")
	}
}

func TestConstructionManagerMaterialRequestRoutes(t *testing.T) {
	t.Parallel()

	want := map[string]struct {
		pattern     string
		permission  string
		serviceFunc string
	}{
		"constructionManager.materialRequests": {
			pattern:     "/api/construction-manager/material-requests",
			permission:  "materialRequest.list",
			serviceFunc: "ConstructionManagerList",
		},
		"constructionManager.materialRequest.detail": {
			pattern:     "/api/construction-manager/material-requests/{id}",
			permission:  "materialRequest.detail",
			serviceFunc: "ConstructionManagerDetail",
		},
	}
	found := make(map[string]int, len(want))
	for _, route := range BuildRoutes() {
		expected, ok := want[route.Name]
		if !ok {
			continue
		}
		found[route.Name]++
		if route.Method != http.MethodGet ||
			route.Pattern != expected.pattern ||
			route.Permission != expected.permission ||
			route.ServiceName != "MaterialRequest" ||
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
