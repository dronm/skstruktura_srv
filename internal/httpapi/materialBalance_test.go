package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dronm/skstruktura/internal/models"
)

func TestMaterialBalanceBinder(t *testing.T) {
	t.Parallel()

	paths := []string{
		"/api/reports/material-balance?construction_site_id=42&from=10&count=25",
		"/api/construction-manager/materials?construction_site_id=42&from=10&count=25",
	}
	for _, path := range paths {
		path := path
		t.Run(path, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, path, nil)
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
		})
	}
}

func TestMaterialBalanceRoutes(t *testing.T) {
	t.Parallel()

	want := map[string]struct {
		pattern     string
		permission  string
		serviceFunc string
	}{
		"materialBalance.list": {
			pattern:     "/api/reports/material-balance",
			permission:  "materialBalance.list",
			serviceFunc: "List",
		},
		"materialBalance.constructionSites": {
			pattern:     "/api/reports/material-balance/sites",
			permission:  "materialBalance.list",
			serviceFunc: "ConstructionSites",
		},
		"constructionManager.sites": {
			pattern:     "/api/construction-manager/sites",
			permission:  "materialBalance.list",
			serviceFunc: "ConstructionSites",
		},
		"constructionManager.materials": {
			pattern:     "/api/construction-manager/materials",
			permission:  "materialRequest.create",
			serviceFunc: "Materials",
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
			route.ServiceName != "MaterialBalance" ||
			route.ServiceFunc != expected.serviceFunc {
			t.Errorf(
				"route %s = %s %s permission %q -> %s.%s, want GET %s permission %q -> MaterialBalance.%s",
				route.Name,
				route.Method,
				route.Pattern,
				route.Permission,
				route.ServiceName,
				route.ServiceFunc,
				expected.pattern,
				expected.permission,
				expected.serviceFunc,
			)
		}
		if route.Name == "constructionManager.materials" && route.Binder == nil {
			t.Errorf("route %s has no binder", route.Name)
		}
	}
	for name := range want {
		if found[name] != 1 {
			t.Errorf("route %s count = %d, want 1", name, found[name])
		}
	}
}
