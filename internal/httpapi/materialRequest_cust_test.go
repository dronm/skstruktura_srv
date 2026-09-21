package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dronm/skstruktura/internal/models"
	"github.com/dronm/webapp"
)

func TestMaterialRequestRoutesReplaceGeneratedAggregateEndpoints(t *testing.T) {
	t.Parallel()

	routes := BuildRoutes()
	if err := webapp.ValidateRoutes(routes); err != nil {
		t.Fatalf("ValidateRoutes() error = %v", err)
	}

	want := map[string]struct {
		method      string
		pattern     string
		serviceFunc string
	}{
		"materialRequest.create": {
			method:      http.MethodPost,
			pattern:     "/api/material-requests",
			serviceFunc: "Create",
		},
		"materialRequest.detail": {
			method:      http.MethodGet,
			pattern:     "/api/material-requests/{id}",
			serviceFunc: "DocumentDetail",
		},
		"materialRequest.update": {
			method:      http.MethodPut,
			pattern:     "/api/material-requests/{id}",
			serviceFunc: "Update",
		},
		"materialRequest.submit": {
			method:      http.MethodPost,
			pattern:     "/api/material-requests/{id}/submit",
			serviceFunc: "Submit",
		},
	}

	found := make(map[string]int, len(want))
	for _, route := range routes {
		if strings.HasPrefix(route.Name, "materialRequestItem.") ||
			strings.HasPrefix(route.Pattern, "/api/material-request-items") {
			t.Errorf("standalone material request item route is exposed: %s %s", route.Name, route.Pattern)
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

func TestMaterialRequestDocumentBinderSupportsNullableDateOnlyFields(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodPost, "/api/material-requests", strings.NewReader(`{
		"date":"2026-09-21T08:30:00+05:00",
		"construction_site_id":10,
		"construction_manager_id":20,
		"comment":null,
		"items":[{
			"material_id":30,
			"measure_unit_id":40,
			"quant":2.5,
			"supplier_id":null,
			"required_date":"2026-10-15",
			"order_importance_id":50
		}]
	}`))

	bound, err := documentJSONBinder[models.MaterialRequestDocument]()(req)
	if err != nil {
		t.Fatalf("document binder error = %v", err)
	}
	document, ok := bound.(*models.MaterialRequestDocument)
	if !ok {
		t.Fatalf("binder result type = %T", bound)
	}
	if document.Comment != nil || len(document.Items) != 1 {
		t.Fatalf("binder result = %#v", document)
	}
	item := document.Items[0]
	if item.SupplierID != nil {
		t.Fatalf("SupplierID = %v, want nil", item.SupplierID)
	}
	if item.RequiredDate == nil || item.RequiredDate.String() != "2026-10-15" {
		t.Fatalf("RequiredDate = %v, want 2026-10-15", item.RequiredDate)
	}
	if item.StatusID != 0 {
		t.Fatalf("StatusID = %d, want zero before the service assigns draft", item.StatusID)
	}
}

func TestMaterialRequestDocumentBinderRejectsTimestampAsRequiredDate(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodPost, "/api/material-requests", strings.NewReader(`{
		"date":"2026-09-21T08:30:00Z",
		"construction_site_id":10,
		"construction_manager_id":20,
		"items":[{
			"material_id":30,
			"measure_unit_id":40,
			"quant":2.5,
			"required_date":"2026-10-15T00:00:00Z",
			"order_importance_id":50
		}]
	}`))

	if _, err := documentJSONBinder[models.MaterialRequestDocument]()(req); err == nil {
		t.Fatal("timestamp required_date was accepted")
	}
}

func TestMaterialRequestSubmitBinderRequiresVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		body      string
		wantError bool
	}{
		{name: "valid", body: `{"version":3}`},
		{name: "zero", body: `{"version":0}`, wantError: true},
		{name: "missing", body: `{}`, wantError: true},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(
				http.MethodPost,
				"/api/material-requests/42/submit",
				strings.NewReader(test.body),
			)
			req.SetPathValue("id", "42")

			bound, err := materialRequestSubmitBinder()(req)
			if (err != nil) != test.wantError {
				t.Fatalf("binder error = %v, wantError = %v", err, test.wantError)
			}
			if test.wantError {
				return
			}
			input, ok := bound.(models.SubmitMaterialRequestInput)
			if !ok {
				t.Fatalf("binder result type = %T", bound)
			}
			if input.ID != 42 || input.Request == nil || input.Request.Version != 3 {
				t.Fatalf("binder result = %#v", input)
			}
		})
	}
}
