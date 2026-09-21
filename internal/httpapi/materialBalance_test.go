package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dronm/skstruktura/internal/models"
)

func TestMaterialBalanceBinder(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/reports/material-balance?construction_site_id=42&from=10&count=25",
		nil,
	)

	bound, err := materialBalanceBinder()(req)
	if err != nil {
		t.Fatalf("materialBalanceBinder() error = %v", err)
	}
	input, ok := bound.(models.MaterialBalanceInput)
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

func TestMaterialBalanceRoutes(t *testing.T) {
	t.Parallel()

	want := map[string]string{
		"materialBalance.list":              "/api/reports/material-balance",
		"materialBalance.constructionSites": "/api/reports/material-balance/sites",
	}
	found := make(map[string]int, len(want))
	for _, route := range BuildRoutes() {
		pattern, ok := want[route.Name]
		if !ok {
			continue
		}
		found[route.Name]++
		if route.Method != http.MethodGet || route.Pattern != pattern {
			t.Errorf("route %s = %s %s, want GET %s", route.Name, route.Method, route.Pattern, pattern)
		}
	}
	for name := range want {
		if found[name] != 1 {
			t.Errorf("route %s count = %d, want 1", name, found[name])
		}
	}
}
