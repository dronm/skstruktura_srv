//go:build integration

package apitest

import (
	"fmt"
	"math"
	"net/http"
	"net/url"
	"testing"
	"time"
)

func TestMaterialActionReport(t *testing.T) {
	c := NewClient(t)
	c.Login(t)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	supplierINN := suffix[len(suffix)-12:]
	measureUnitID := createAPIObject(t, c, "/api/measure-unit", map[string]any{
		"name":      "report-unit-" + suffix,
		"name_full": "Material action report unit " + suffix,
		"is_active": true,
	})
	t.Cleanup(func() { c.DeleteIgnore(t, fmt.Sprintf("/api/measure-unit/%d", measureUnitID)) })

	materialTypeID := createAPIObject(t, c, "/api/material-types", map[string]any{
		"name":      "report-material-type-" + suffix,
		"code":      "MT-" + suffix,
		"is_active": true,
	})
	t.Cleanup(func() { c.DeleteIgnore(t, fmt.Sprintf("/api/material-types/%d", materialTypeID)) })

	materialID := createAPIObject(t, c, "/api/material", map[string]any{
		"name":             "report-material-" + suffix,
		"name_full":        "Material action report material " + suffix,
		"measure_unit_id":  measureUnitID,
		"material_type_id": materialTypeID,
		"is_active":        true,
	})
	t.Cleanup(func() { c.DeleteIgnore(t, fmt.Sprintf("/api/material/%d", materialID)) })

	siteID := createAPIObject(t, c, "/api/construction-sites", map[string]any{
		"name":      "report-site-" + suffix,
		"is_active": true,
	})
	t.Cleanup(func() { c.DeleteIgnore(t, fmt.Sprintf("/api/construction-sites/%d", siteID)) })

	supplierID := createAPIObject(t, c, "/api/supplier", map[string]any{
		"name":      "report-supplier-" + suffix,
		"name_full": "Material action report supplier " + suffix,
		"inn":       supplierINN,
		"kpp":       "",
		"is_active": true,
	})
	t.Cleanup(func() { c.DeleteIgnore(t, fmt.Sprintf("/api/supplier/%d", supplierID)) })

	receiptID := createAPIObject(t, c, "/api/material-receipts", map[string]any{
		"date":                 "2026-08-19T00:00:00Z",
		"construction_site_id": siteID,
		"supplier_id":          supplierID,
		"number":               "R-" + suffix,
		"items": []map[string]any{
			{
				"material_id":     materialID,
				"measure_unit_id": measureUnitID,
				"quant":           10,
				"price":           2,
				"amount":          20,
			},
		},
	})
	t.Cleanup(func() { c.DeleteIgnore(t, fmt.Sprintf("/api/material-receipts/%d", receiptID)) })

	consumptionID := createAPIObject(t, c, "/api/material-consumptions", map[string]any{
		"date":                 "2026-08-20T12:30:00Z",
		"construction_site_id": siteID,
		"items": []map[string]any{
			{
				"material_id":     materialID,
				"measure_unit_id": measureUnitID,
				"quant":           3,
			},
		},
	})
	t.Cleanup(func() { c.DeleteIgnore(t, fmt.Sprintf("/api/material-consumptions/%d", consumptionID)) })

	baseValues := url.Values{
		"date_from":             {"2026-08-20T12:30:00Z"},
		"date_to":               {"2026-08-20T12:30:00Z"},
		"construction_site_ids": {fmt.Sprint(siteID)},
		"material_ids":          {fmt.Sprint(materialID)},
	}

	siteValues := cloneReportValues(baseValues)
	siteValues.Set("level", "construction_site")
	siteResponse := c.DoJSON(
		t,
		http.MethodGet,
		"/api/reports/material-actions?"+siteValues.Encode(),
		nil,
		http.StatusOK,
	)
	siteRow := requireSingleReportRow(t, siteResponse)
	requireReportAmounts(t, siteRow, 10, 0, 3, 7)
	if got := intJSON(t, siteRow, "construction_site_id"); got != siteID {
		t.Fatalf("construction_site_id = %d, want %d", got, siteID)
	}
	if hasChildren, _ := siteRow["has_children"].(bool); !hasChildren {
		t.Fatal("construction-site row should have children")
	}

	materialValues := cloneReportValues(baseValues)
	materialValues.Set("level", "material")
	materialValues.Set("parent_construction_site_id", fmt.Sprint(siteID))
	materialResponse := c.DoJSON(
		t,
		http.MethodGet,
		"/api/reports/material-actions?"+materialValues.Encode(),
		nil,
		http.StatusOK,
	)
	materialRow := requireSingleReportRow(t, materialResponse)
	requireReportAmounts(t, materialRow, 10, 0, 3, 7)
	if got := intJSON(t, materialRow, "material_id"); got != materialID {
		t.Fatalf("material_id = %d, want %d", got, materialID)
	}

	documentValues := cloneReportValues(baseValues)
	documentValues.Set("level", "document")
	documentValues.Set("parent_construction_site_id", fmt.Sprint(siteID))
	documentValues.Set("parent_material_id", fmt.Sprint(materialID))
	documentResponse := c.DoJSON(
		t,
		http.MethodGet,
		"/api/reports/material-actions?"+documentValues.Encode(),
		nil,
		http.StatusOK,
	)
	documentRow := requireSingleReportRow(t, documentResponse)
	if got, _ := documentRow["recorder_type"].(string); got != "MaterialConsumption" {
		t.Fatalf("recorder_type = %q, want MaterialConsumption", got)
	}
	if got, _ := documentRow["document_date"].(string); got != "2026-08-20T12:30:00Z" {
		t.Fatalf("document_date = %q, want exact timestamp", got)
	}
	if documentRow["balance_start"] != nil || documentRow["balance_end"] != nil {
		t.Fatal("document rows should have null opening and closing balances")
	}
	requireJSONNumber(t, documentRow, "income", 0)
	requireJSONNumber(t, documentRow, "outcome", 3)

	materialList := c.DoJSON(t, http.MethodGet, "/api/material?count=1000", nil, http.StatusOK)
	listRow := findCollectionRowByID(t, materialList, materialID)
	if got := intJSON(t, listRow, "material_type_id"); got != materialTypeID {
		t.Fatalf("material_type_id = %d, want %d", got, materialTypeID)
	}
	materialTypeRaw := requireObjectField(t, listRow, "material_type")
	materialType, ok := materialTypeRaw.(map[string]any)
	if !ok {
		t.Fatalf("material_type has unexpected type %T", materialTypeRaw)
	}
	materialTypeKeysRaw := requireObjectField(t, materialType, "keys")
	materialTypeKeys, ok := materialTypeKeysRaw.(map[string]any)
	if !ok {
		t.Fatalf("material_type keys have unexpected type %T", materialTypeKeysRaw)
	}
	if got := intJSON(t, materialTypeKeys, "id"); got != materialTypeID {
		t.Fatalf("material_type reference id = %d, want %d", got, materialTypeID)
	}
	balances, ok := listRow["balances"].([]any)
	if !ok {
		t.Fatalf("material balances has unexpected type %T", listRow["balances"])
	}
	foundBalance := false
	for _, balanceRaw := range balances {
		balance, ok := balanceRaw.(map[string]any)
		if !ok {
			continue
		}
		if intJSON(t, balance, "construction_site_id") != siteID {
			continue
		}
		requireJSONNumber(t, balance, "quant", 7)
		foundBalance = true
		break
	}
	if !foundBalance {
		t.Fatalf("active site %d is missing from material balances", siteID)
	}
}

func createAPIObject(t *testing.T, c *Client, path string, body map[string]any) int {
	t.Helper()
	created := c.DoJSON(t, http.MethodPost, path, body, http.StatusCreated)
	return intJSON(t, created, "id")
}

func intJSON(t *testing.T, object map[string]any, field string) int {
	t.Helper()
	value, ok := object[field].(float64)
	if !ok {
		t.Fatalf("field %q has unexpected type %T", field, object[field])
	}
	return int(value)
}

func cloneReportValues(values url.Values) url.Values {
	result := make(url.Values, len(values))
	for key, item := range values {
		result[key] = append([]string(nil), item...)
	}
	return result
}

func requireSingleReportRow(t *testing.T, response map[string]any) map[string]any {
	t.Helper()
	rowsRaw := requireObjectField(t, response, "rows")
	rows, ok := rowsRaw.([]any)
	if !ok {
		t.Fatalf("report rows has unexpected type %T", rowsRaw)
	}
	if len(rows) != 1 {
		t.Fatalf("report row count = %d, want 1", len(rows))
	}
	row, ok := rows[0].(map[string]any)
	if !ok {
		t.Fatalf("report row has unexpected type %T", rows[0])
	}
	return row
}

func requireReportAmounts(
	t *testing.T,
	row map[string]any,
	balanceStart float64,
	income float64,
	outcome float64,
	balanceEnd float64,
) {
	t.Helper()
	requireJSONNumber(t, row, "balance_start", balanceStart)
	requireJSONNumber(t, row, "income", income)
	requireJSONNumber(t, row, "outcome", outcome)
	requireJSONNumber(t, row, "balance_end", balanceEnd)
}

func requireJSONNumber(t *testing.T, object map[string]any, field string, want float64) {
	t.Helper()
	got, ok := object[field].(float64)
	if !ok {
		t.Fatalf("field %q has unexpected type %T", field, object[field])
	}
	if math.Abs(got-want) > 0.00005 {
		t.Fatalf("field %q = %.4f, want %.4f", field, got, want)
	}
}

func findCollectionRowByID(t *testing.T, collection map[string]any, id int) map[string]any {
	t.Helper()
	rowsRaw := requireObjectField(t, collection, "rows")
	rows, ok := rowsRaw.([]any)
	if !ok {
		t.Fatalf("collection rows has unexpected type %T", rowsRaw)
	}
	for _, rowRaw := range rows {
		row, ok := rowRaw.(map[string]any)
		if !ok {
			continue
		}
		if intJSON(t, row, "id") == id {
			return row
		}
	}
	t.Fatalf("collection row with id %d was not found", id)
	return nil
}
