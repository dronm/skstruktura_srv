package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dronm/skstruktura/internal/models"
)

func TestConstructionManagerMaterialStatusQueryBinder(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/construction-manager/material-statuses/current?construction_site_id=42&material_type_id=7&from=10&count=25",
		nil,
	)

	bound, err := constructionManagerMaterialStatusQueryBinder()(req)
	if err != nil {
		t.Fatalf("constructionManagerMaterialStatusQueryBinder() error = %v", err)
	}
	input, ok := bound.(models.ConstructionManagerMaterialStatusInput)
	if !ok {
		t.Fatalf("binder result type = %T", bound)
	}
	if input.Query == nil || input.Query.ConstructionSiteID != 42 ||
		input.Query.MaterialTypeID == nil || *input.Query.MaterialTypeID != 7 {
		t.Fatalf("binder query = %#v", input.Query)
	}
	if input.Params.From != 10 || input.Params.Count != 25 {
		t.Fatalf("binder collection params = %#v", input.Params)
	}
}

func TestConstructionManagerMaterialStatusQueryBinderRequiresSite(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/construction-manager/material-statuses/history?from=0&count=25",
		nil,
	)
	if _, err := constructionManagerMaterialStatusQueryBinder()(req); err == nil {
		t.Fatal("constructionManagerMaterialStatusQueryBinder() error = nil, want required site error")
	}
}

func TestConstructionManagerMaterialStatusChangeBinder(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/construction-manager/material-statuses",
		strings.NewReader(`{
			"construction_site_id":42,
			"material_id":17,
			"created_at":"2026-09-21T09:30:00Z",
			"expected_status_record_id":null,
			"expected_status":"at_work",
			"target_status":"on_maintenance"
		}`),
	)
	req.Header.Set("Content-Type", "application/json")

	bound, err := constructionManagerMaterialStatusChangeBinder()(req)
	if err != nil {
		t.Fatalf("constructionManagerMaterialStatusChangeBinder() error = %v", err)
	}
	request, ok := bound.(*models.ConstructionManagerMaterialStatusChangeRequest)
	if !ok {
		t.Fatalf("binder result type = %T", bound)
	}
	wantTime := time.Date(2026, time.September, 21, 9, 30, 0, 0, time.UTC)
	if request.ConstructionSiteID != 42 || request.MaterialID != 17 ||
		!request.CreatedAt.Equal(wantTime) || request.ExpectedStatusRecordID != nil ||
		request.ExpectedStatus != models.MaterialStatusTypeAtWork ||
		request.TargetStatus != models.MaterialStatusTypeOnMaintenance {
		t.Fatalf("binder request = %#v", request)
	}
}

func TestConstructionManagerMaterialStatusRoutes(t *testing.T) {
	t.Parallel()

	want := map[string]struct {
		method      string
		pattern     string
		permission  string
		serviceFunc string
		successCode int
	}{
		"constructionManager.materialStatus.current": {
			method:      http.MethodGet,
			pattern:     "/api/construction-manager/material-statuses/current",
			permission:  "constructionManager.materialStatus.list",
			serviceFunc: "ConstructionManagerCurrent",
		},
		"constructionManager.materialStatus.create": {
			method:      http.MethodPost,
			pattern:     "/api/construction-manager/material-statuses",
			permission:  "constructionManager.materialStatus.create",
			serviceFunc: "ConstructionManagerCreate",
			successCode: http.StatusCreated,
		},
		"constructionManager.materialStatus.history": {
			method:      http.MethodGet,
			pattern:     "/api/construction-manager/material-statuses/history",
			permission:  "constructionManager.materialStatus.list",
			serviceFunc: "ConstructionManagerHistory",
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
			route.ServiceName != "MaterialStatus" ||
			route.ServiceFunc != expected.serviceFunc ||
			route.Binder == nil ||
			route.SuccessCode != expected.successCode {
			t.Errorf(
				"route %s = %s %s permission %q -> %s.%s binder=%t success=%d",
				route.Name,
				route.Method,
				route.Pattern,
				route.Permission,
				route.ServiceName,
				route.ServiceFunc,
				route.Binder != nil,
				route.SuccessCode,
			)
		}
	}
	for name := range want {
		if found[name] != 1 {
			t.Errorf("route %s count = %d, want 1", name, found[name])
		}
	}
}
