package services

import (
	"strings"
	"testing"
	"time"

	"github.com/dronm/modelbind"
	"github.com/dronm/skstruktura/internal/models"
)

func TestValidateMaterialActionReportInput(t *testing.T) {
	dateFrom := time.Date(2026, time.August, 19, 8, 15, 0, 0, time.FixedZone("UTC+3", 3*60*60))
	dateTo := time.Date(2026, time.August, 21, 18, 45, 30, 0, time.FixedZone("UTC+3", 3*60*60))
	siteID := 10

	query, params, err := validateMaterialActionReportInput(models.MaterialActionReportInput{
		Query: &models.MaterialActionReportQuery{
			DateFrom:                 dateFrom,
			DateTo:                   dateTo,
			ConstructionSiteIDs:      []int{10, 10, 20},
			MaterialIDs:              []int{30, 30},
			Level:                    models.MaterialActionReportLevelMaterial,
			ParentConstructionSiteID: &siteID,
		},
	})
	if err != nil {
		t.Fatalf("validateMaterialActionReportInput() error = %v", err)
	}
	if !query.DateFrom.Equal(dateFrom) || !query.DateTo.Equal(dateTo) {
		t.Fatalf("report boundaries changed: from=%s to=%s", query.DateFrom, query.DateTo)
	}
	if got, want := query.ConstructionSiteIDs, []int{10, 20}; !equalIntSlices(got, want) {
		t.Fatalf("construction site ids = %v, want %v", got, want)
	}
	if got, want := query.MaterialIDs, []int{30}; !equalIntSlices(got, want) {
		t.Fatalf("material ids = %v, want %v", got, want)
	}
	if params.Count != 200 {
		t.Fatalf("default count = %d, want 200", params.Count)
	}
}

func TestValidateMaterialActionReportInputRejectsInvalidHierarchy(t *testing.T) {
	date := time.Date(2026, time.August, 21, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name  string
		query *models.MaterialActionReportQuery
	}{
		{
			name: "reversed dates",
			query: &models.MaterialActionReportQuery{
				DateFrom: date.AddDate(0, 0, 1),
				DateTo:   date,
				Level:    models.MaterialActionReportLevelConstructionSite,
			},
		},
		{
			name: "material without site parent",
			query: &models.MaterialActionReportQuery{
				DateFrom: date,
				DateTo:   date,
				Level:    models.MaterialActionReportLevelMaterial,
			},
		},
		{
			name: "document without material parent",
			query: &models.MaterialActionReportQuery{
				DateFrom:                 date,
				DateTo:                   date,
				Level:                    models.MaterialActionReportLevelDocument,
				ParentConstructionSiteID: intPtr(1),
			},
		},
		{
			name: "non-positive filter",
			query: &models.MaterialActionReportQuery{
				DateFrom:            date,
				DateTo:              date,
				Level:               models.MaterialActionReportLevelConstructionSite,
				ConstructionSiteIDs: []int{0},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, _, err := validateMaterialActionReportInput(models.MaterialActionReportInput{Query: test.query}); err == nil {
				t.Fatal("validateMaterialActionReportInput() error = nil, want error")
			}
		})
	}
}

func TestMaterialActionReportOrderByWhitelistsFields(t *testing.T) {
	orderBy, err := materialActionReportOrderBy(
		models.MaterialActionReportLevelDocument,
		[]modelbind.CollectionSorter{{Field: "outcome", Direct: modelbind.SortParDesc}},
	)
	if err != nil {
		t.Fatalf("materialActionReportOrderBy() error = %v", err)
	}
	if !strings.HasPrefix(orderBy, "outcome DESC") {
		t.Fatalf("order by = %q, want outcome DESC first", orderBy)
	}

	if _, err := materialActionReportOrderBy(
		models.MaterialActionReportLevelDocument,
		[]modelbind.CollectionSorter{{Field: "recorder_id; DROP TABLE materials", Direct: modelbind.SortParAsc}},
	); err == nil {
		t.Fatal("materialActionReportOrderBy() accepted unsupported field")
	}
}

func TestMaterialDocumentCaption(t *testing.T) {
	receiptNumber := " R-42 "
	emptyNumber := "  "
	tests := []struct {
		name           string
		recorderType   string
		recorderID     int64
		documentNumber *string
		want           string
	}{
		{
			name:           "receipt number",
			recorderType:   materialReceiptRecorderType,
			recorderID:     7,
			documentNumber: &receiptNumber,
			want:           "Поступление материалов №R-42 19/08/26 14:05:06",
		},
		{
			name:         "consumption id fallback",
			recorderType: materialConsumptionRecorderType,
			recorderID:   8,
			want:         "Списание материалов №8 19/08/26 14:05:06",
		},
		{
			name:           "transfer empty number fallback",
			recorderType:   materialTransferRecorderType,
			recorderID:     9,
			documentNumber: &emptyNumber,
			want:           "Перемещение материалов №9 19/08/26 14:05:06",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := materialDocumentCaption(
				test.recorderType,
				test.recorderID,
				test.documentNumber,
				"19/08/26 14:05:06",
			)
			if got != test.want {
				t.Fatalf("materialDocumentCaption() = %q, want %q", got, test.want)
			}
		})
	}
}

func equalIntSlices(left, right []int) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
