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

func TestConstructionManagerMaterialRequestRoute(t *testing.T) {
	t.Parallel()

	found := 0
	for _, route := range BuildRoutes() {
		if route.Name != "constructionManager.materialRequests" {
			continue
		}
		found++
		if route.Method != http.MethodGet ||
			route.Pattern != "/api/construction-manager/material-requests" ||
			route.Permission != "materialRequest.list" ||
			route.ServiceName != "MaterialRequest" ||
			route.ServiceFunc != "ConstructionManagerList" ||
			route.Binder == nil {
			t.Errorf(
				"route = %s %s permission %q -> %s.%s binder=%t",
				route.Method,
				route.Pattern,
				route.Permission,
				route.ServiceName,
				route.ServiceFunc,
				route.Binder != nil,
			)
		}
	}
	if found != 1 {
		t.Fatalf("constructionManager.materialRequests route count = %d, want 1", found)
	}
}
