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

func TestSupplyManagerMaterialRequestBinder(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/supply-manager/material-requests?construction_site_id=42&date_from=2026-09-01T00%3A00%3A00Z&date_to=2026-09-30T23%3A59%3A59Z&material_search=%20cement%20&order_importance_id=3&from=10&count=25",
		nil,
	)

	bound, err := supplyManagerMaterialRequestBinder()(req)
	if err != nil {
		t.Fatalf("supplyManagerMaterialRequestBinder() error = %v", err)
	}
	input, ok := bound.(models.SupplyManagerMaterialRequestInput)
	if !ok {
		t.Fatalf("binder result type = %T", bound)
	}
	if input.Query == nil {
		t.Fatal("binder query is nil")
	}
	if input.Query.ConstructionSiteID == nil || *input.Query.ConstructionSiteID != 42 {
		t.Fatalf("construction_site_id = %#v", input.Query.ConstructionSiteID)
	}
	if input.Query.DateFrom == nil || input.Query.DateTo == nil {
		t.Fatalf("date range = %#v - %#v", input.Query.DateFrom, input.Query.DateTo)
	}
	if got, want := input.Query.DateFrom.Format(time.RFC3339), "2026-09-01T00:00:00Z"; got != want {
		t.Fatalf("date_from = %q, want %q", got, want)
	}
	if got, want := input.Query.DateTo.Format(time.RFC3339), "2026-09-30T23:59:59Z"; got != want {
		t.Fatalf("date_to = %q, want %q", got, want)
	}
	if input.Query.MaterialSearch == nil || *input.Query.MaterialSearch != "cement" {
		t.Fatalf("material_search = %#v", input.Query.MaterialSearch)
	}
	if input.Query.OrderImportanceID == nil || *input.Query.OrderImportanceID != 3 {
		t.Fatalf("order_importance_id = %#v", input.Query.OrderImportanceID)
	}
	if input.Params.From != 10 || input.Params.Count != 25 {
		t.Fatalf("collection params = %#v", input.Params)
	}
}

func TestSupplyManagerMaterialRequestBinderAllowsEmptyFilters(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/api/supply-manager/material-requests", nil)
	bound, err := supplyManagerMaterialRequestBinder()(req)
	if err != nil {
		t.Fatalf("supplyManagerMaterialRequestBinder() error = %v", err)
	}
	input, ok := bound.(models.SupplyManagerMaterialRequestInput)
	if !ok {
		t.Fatalf("binder result type = %T", bound)
	}
	if input.Query == nil {
		t.Fatal("binder query is nil")
	}
	if input.Query.ConstructionSiteID != nil || input.Query.DateFrom != nil ||
		input.Query.DateTo != nil || input.Query.MaterialSearch != nil ||
		input.Query.OrderImportanceID != nil {
		t.Fatalf("binder query = %#v, want empty optional filters", input.Query)
	}
}

func TestSupplyManagerMaterialRequestBinderRejectsInvalidFilters(t *testing.T) {
	t.Parallel()

	tests := []string{
		"?construction_site_id=0",
		"?order_importance_id=0",
		"?date_from=2026-09-01",
		"?date_from=2026-09-02T00%3A00%3A00Z&date_to=2026-09-01T00%3A00%3A00Z",
	}
	for _, query := range tests {
		query := query
		t.Run(query, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(
				http.MethodGet,
				"/api/supply-manager/material-requests"+query,
				nil,
			)
			if _, err := supplyManagerMaterialRequestBinder()(req); err == nil {
				t.Fatal("supplyManagerMaterialRequestBinder() error = nil, want invalid filter error")
			}
		})
	}
}

func TestSupplyManagerCreateAssignmentBinder(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/supply-manager/material-request-supplier-assignments",
		strings.NewReader(`{
			"date":"2026-09-22T08:30:00Z",
			"comment":"September supplier batch",
			"requests":[{"id":11,"version":4},{"id":12,"version":2}],
			"items":[
				{"material_request_item_id":101,"supplier_id":7},
				{"material_request_item_id":102,"supplier_id":8}
			]
		}`),
	)
	req.Header.Set("Content-Type", "application/json")

	bound, err := supplyManagerCreateAssignmentBinder()(req)
	if err != nil {
		t.Fatalf("supplyManagerCreateAssignmentBinder() error = %v", err)
	}
	input, ok := bound.(*models.SupplyManagerCreateAssignmentRequest)
	if !ok {
		t.Fatalf("binder result type = %T", bound)
	}
	if got, want := input.Date.Format(time.RFC3339), "2026-09-22T08:30:00Z"; got != want {
		t.Fatalf("date = %q, want %q", got, want)
	}
	if input.Comment == nil || *input.Comment != "September supplier batch" {
		t.Fatalf("comment = %#v", input.Comment)
	}
	if len(input.Requests) != 2 || input.Requests[0].ID != 11 || input.Requests[0].Version != 4 {
		t.Fatalf("requests = %#v", input.Requests)
	}
	if len(input.Items) != 2 || input.Items[1].MaterialRequestItemID != 102 ||
		input.Items[1].SupplierID != 8 {
		t.Fatalf("items = %#v", input.Items)
	}
}

func TestSupplyManagerCreateAssignmentBinderRejectsUnknownField(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/supply-manager/material-request-supplier-assignments",
		strings.NewReader(`{
			"date":"2026-09-22T08:30:00Z",
			"comment":null,
			"requests":[{"id":11,"version":4}],
			"items":[{"material_request_item_id":101,"supplier_id":7,"quant":1}]
		}`),
	)
	req.Header.Set("Content-Type", "application/json")

	if _, err := supplyManagerCreateAssignmentBinder()(req); err == nil {
		t.Fatal("supplyManagerCreateAssignmentBinder() error = nil, want unknown field error")
	}
}

func TestSupplyManagerAssignmentHistoryBinder(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/supply-manager/material-request-supplier-assignments?construction_site_id=42&from=20&count=10",
		nil,
	)
	bound, err := supplyManagerAssignmentHistoryBinder()(req)
	if err != nil {
		t.Fatalf("supplyManagerAssignmentHistoryBinder() error = %v", err)
	}
	input, ok := bound.(models.SupplyManagerAssignmentHistoryInput)
	if !ok {
		t.Fatalf("binder result type = %T", bound)
	}
	if input.Query == nil || input.Query.ConstructionSiteID == nil ||
		*input.Query.ConstructionSiteID != 42 {
		t.Fatalf("binder query = %#v", input.Query)
	}
	if input.Params.From != 20 || input.Params.Count != 10 {
		t.Fatalf("collection params = %#v", input.Params)
	}
}

func TestSupplyManagerAssignmentHistoryBinderRejectsInvalidSite(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/supply-manager/material-request-supplier-assignments?construction_site_id=0",
		nil,
	)
	if _, err := supplyManagerAssignmentHistoryBinder()(req); err == nil {
		t.Fatal("supplyManagerAssignmentHistoryBinder() error = nil, want invalid site error")
	}
}

func TestSupplyManagerWorkspaceRoutes(t *testing.T) {
	t.Parallel()

	want := map[string]struct {
		method      string
		pattern     string
		permission  string
		serviceFunc string
		hasBinder   bool
		successCode int
	}{
		"supplyManager.sites": {
			method:      http.MethodGet,
			pattern:     "/api/supply-manager/sites",
			permission:  supplyManagerAssignmentCreatePermission,
			serviceFunc: "SupplyManagerSites",
		},
		"supplyManager.materialRequests": {
			method:      http.MethodGet,
			pattern:     "/api/supply-manager/material-requests",
			permission:  supplyManagerAssignmentCreatePermission,
			serviceFunc: "SupplyManagerMaterialRequests",
			hasBinder:   true,
		},
		"supplyManager.materialRequest.detail": {
			method:      http.MethodGet,
			pattern:     "/api/supply-manager/material-requests/{id}",
			permission:  supplyManagerAssignmentCreatePermission,
			serviceFunc: "SupplyManagerMaterialRequestDetail",
			hasBinder:   true,
		},
		"supplyManager.materialRequestSupplierAssignment.create": {
			method:      http.MethodPost,
			pattern:     "/api/supply-manager/material-request-supplier-assignments",
			permission:  supplyManagerAssignmentCreatePermission,
			serviceFunc: "SupplyManagerCreate",
			hasBinder:   true,
			successCode: http.StatusCreated,
		},
		"supplyManager.materialRequestSupplierAssignment.list": {
			method:      http.MethodGet,
			pattern:     "/api/supply-manager/material-request-supplier-assignments",
			permission:  supplyManagerAssignmentListPermission,
			serviceFunc: "SupplyManagerHistory",
			hasBinder:   true,
		},
		"supplyManager.materialRequestSupplierAssignment.detail": {
			method:      http.MethodGet,
			pattern:     "/api/supply-manager/material-request-supplier-assignments/{id}",
			permission:  supplyManagerAssignmentDetailPermission,
			serviceFunc: "SupplyManagerDetail",
			hasBinder:   true,
		},
	}
	found := make(map[string]int, len(want))
	for _, route := range BuildRoutes() {
		if strings.HasPrefix(route.Name, "materialRequestSupplierAssignment.") ||
			strings.HasPrefix(route.Pattern, "/api/material-request-supplier-assignments") {
			t.Errorf("unscoped generated assignment route remains: %s %s", route.Method, route.Pattern)
		}
		expected, ok := want[route.Name]
		if !ok {
			continue
		}
		found[route.Name]++
		if route.Method != expected.method ||
			route.Pattern != expected.pattern ||
			route.Permission != expected.permission ||
			route.ServiceName != "MaterialRequestSupplierAssignment" ||
			route.ServiceFunc != expected.serviceFunc ||
			(route.Binder != nil) != expected.hasBinder ||
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

func TestRemoveGeneratedSupplyManagerAssignmentRoutes(t *testing.T) {
	t.Parallel()

	routes := []webapp.Route{
		{Name: "materialRequestSupplierAssignment.list", Pattern: "/api/material-request-supplier-assignments"},
		{Name: "materialRequestSupplierAssignmentItem.detail", Pattern: "/api/material-request-supplier-assignment-items/{id}"},
		{Name: "unrelated.list", Pattern: "/api/unrelated"},
	}

	filtered := removeGeneratedSupplyManagerAssignmentRoutes(routes)
	if len(filtered) != 2 ||
		filtered[0].Name != "materialRequestSupplierAssignmentItem.detail" ||
		filtered[1].Name != "unrelated.list" {
		t.Fatalf("filtered routes = %#v", filtered)
	}
}
