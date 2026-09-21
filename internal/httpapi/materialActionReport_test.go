package httpapi

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dronm/skstruktura/internal/models"
)

func TestMaterialActionReportBinder(t *testing.T) {
	req := httptest.NewRequest(
		"GET",
		"/api/reports/material-actions?date_from=2026-08-19T08%3A15%3A00%2B03%3A00&date_to=2026-08-21T18%3A45%3A30%2B03%3A00&level=material&construction_site_ids=1,2&material_ids=4&material_ids=5&parent_construction_site_id=1&from=10&count=25",
		nil,
	)

	bound, err := materialActionReportBinder()(req)
	if err != nil {
		t.Fatalf("materialActionReportBinder() error = %v", err)
	}
	input, ok := bound.(models.MaterialActionReportInput)
	if !ok {
		t.Fatalf("binder result type = %T", bound)
	}
	if input.Query == nil {
		t.Fatal("binder result query is nil")
	}
	if got, want := input.Query.DateFrom.Format(time.RFC3339), "2026-08-19T08:15:00+03:00"; got != want {
		t.Fatalf("date_from = %q, want %q", got, want)
	}
	if got, want := input.Query.DateTo.Format(time.RFC3339), "2026-08-21T18:45:30+03:00"; got != want {
		t.Fatalf("date_to = %q, want %q", got, want)
	}
	if got, want := input.Query.Level, models.MaterialActionReportLevelMaterial; got != want {
		t.Fatalf("level = %q, want %q", got, want)
	}
	if len(input.Query.ConstructionSiteIDs) != 2 || input.Query.ConstructionSiteIDs[1] != 2 {
		t.Fatalf("construction site ids = %v, want [1 2]", input.Query.ConstructionSiteIDs)
	}
	if len(input.Query.MaterialIDs) != 2 || input.Query.MaterialIDs[1] != 5 {
		t.Fatalf("material ids = %v, want [4 5]", input.Query.MaterialIDs)
	}
	if input.Params.From != 10 || input.Params.Count != 25 {
		t.Fatalf("collection params = %#v, want from 10 count 25", input.Params)
	}
}

func TestMaterialActionReportBinderRejectsDateWithoutTime(t *testing.T) {
	req := httptest.NewRequest(
		"GET",
		"/api/reports/material-actions?date_from=2026-08-19&date_to=2026-08-21T18%3A45%3A30Z&level=construction_site",
		nil,
	)

	if _, err := materialActionReportBinder()(req); err == nil {
		t.Fatal("materialActionReportBinder() error = nil, want invalid timestamp error")
	}
}
