//go:build integration

package apitest

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

func TestMaterialDocumentAggregateCreateAndReplace(t *testing.T) {
	c := NewClient(t)
	c.Login(t)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	supplierINN := suffix[len(suffix)-12:]
	measureUnitID := createAPIObject(t, c, "/api/measure-unit", map[string]any{
		"name":      "document-unit-" + suffix,
		"name_full": "Material document unit " + suffix,
		"is_active": true,
	})
	t.Cleanup(func() { c.DeleteIgnore(t, fmt.Sprintf("/api/measure-unit/%d", measureUnitID)) })

	materialTypeID := createAPIObject(t, c, "/api/material-types", map[string]any{
		"name":      "document-material-type-" + suffix,
		"code":      "DOC-MT-" + suffix,
		"is_active": true,
	})
	t.Cleanup(func() {
		c.DeleteIgnore(t, fmt.Sprintf("/api/material-types/%d", materialTypeID))
	})

	materialID := createAPIObject(t, c, "/api/material", map[string]any{
		"name":             "document-material-" + suffix,
		"name_full":        "Material document material " + suffix,
		"measure_unit_id":  measureUnitID,
		"material_type_id": materialTypeID,
		"is_active":        true,
	})
	t.Cleanup(func() { c.DeleteIgnore(t, fmt.Sprintf("/api/material/%d", materialID)) })

	siteID := createAPIObject(t, c, "/api/construction-sites", map[string]any{
		"name":      "document-site-" + suffix,
		"is_active": true,
	})
	t.Cleanup(func() { c.DeleteIgnore(t, fmt.Sprintf("/api/construction-sites/%d", siteID)) })

	supplierID := createAPIObject(t, c, "/api/supplier", map[string]any{
		"name":      "document-supplier-" + suffix,
		"name_full": "Material document supplier " + suffix,
		"inn":       supplierINN,
		"kpp":       "",
		"is_active": true,
	})
	t.Cleanup(func() { c.DeleteIgnore(t, fmt.Sprintf("/api/supplier/%d", supplierID)) })

	created := c.DoJSON(t, http.MethodPost, "/api/material-receipts", map[string]any{
		"date":                 "2026-08-22T00:00:00Z",
		"construction_site_id": siteID,
		"supplier_id":          supplierID,
		"number":               "DOC-" + suffix,
		"items": []map[string]any{
			{
				"material_id":     materialID,
				"measure_unit_id": measureUnitID,
				"quant":           3,
				"price":           2,
				"amount":          6,
				"vat_percent":     20,
				"vat_amount":      1,
			},
			{
				"material_id":     materialID,
				"measure_unit_id": measureUnitID,
				"quant":           4,
				"price":           2,
				"amount":          8,
			},
		},
	}, http.StatusCreated)
	receiptID := intJSON(t, created, "id")
	t.Cleanup(func() { c.DeleteIgnore(t, fmt.Sprintf("/api/material-receipts/%d", receiptID)) })
	if version := intJSON(t, created, "version"); version != 1 {
		t.Fatalf("created version = %d, want 1", version)
	}
	createdItems := materialDocumentItems(t, created)
	if len(createdItems) != 2 {
		t.Fatalf("created item count = %d, want 2", len(createdItems))
	}
	firstItemID := intJSON(t, createdItems[0], "id")
	secondItemID := intJSON(t, createdItems[1], "id")
	requireJSONNumber(t, createdItems[0], "vat_percent", 20)
	requireJSONNumber(t, createdItems[0], "vat_amount", 1)

	updated := c.DoJSON(t, http.MethodPut, fmt.Sprintf("/api/material-receipts/%d", receiptID), map[string]any{
		"id":                   receiptID,
		"version":              1,
		"date":                 "2026-08-22T00:00:00Z",
		"construction_site_id": siteID,
		"supplier_id":          supplierID,
		"number":               "DOC-" + suffix + "-UPDATED",
		"items": []map[string]any{
			{
				"id":              secondItemID,
				"material_id":     materialID,
				"measure_unit_id": measureUnitID,
				"quant":           5,
				"price":           2,
				"amount":          10,
				"vat_percent":     20,
				"vat_amount":      1.67,
			},
			{
				"material_id":     materialID,
				"measure_unit_id": measureUnitID,
				"quant":           1,
				"price":           2,
				"amount":          2,
			},
		},
	}, http.StatusOK)
	if version := intJSON(t, updated, "version"); version != 2 {
		t.Fatalf("updated version = %d, want 2", version)
	}
	updatedItems := materialDocumentItems(t, updated)
	if len(updatedItems) != 2 {
		t.Fatalf("updated item count = %d, want 2", len(updatedItems))
	}
	if got := intJSON(t, updatedItems[0], "id"); got != secondItemID {
		t.Fatalf("retained item id = %d, want %d", got, secondItemID)
	}
	if got := intJSON(t, updatedItems[0], "line_num"); got != 1 {
		t.Fatalf("first line number = %d, want 1", got)
	}
	if got := intJSON(t, updatedItems[1], "line_num"); got != 2 {
		t.Fatalf("second line number = %d, want 2", got)
	}
	if got := intJSON(t, updatedItems[1], "id"); got == 0 || got == firstItemID || got == secondItemID {
		t.Fatalf("new item id = %d, want a new positive id", got)
	}

	detail := c.DoJSON(
		t,
		http.MethodGet,
		fmt.Sprintf("/api/material-receipts/%d", receiptID),
		nil,
		http.StatusOK,
	)
	if intJSON(t, detail, "version") != 2 || len(materialDocumentItems(t, detail)) != 2 {
		t.Fatalf("detail does not contain the updated aggregate: %v", detail)
	}

	c.DoJSON(t, http.MethodPut, fmt.Sprintf("/api/material-receipts/%d", receiptID), map[string]any{
		"version":              1,
		"date":                 "2026-08-22T00:00:00Z",
		"construction_site_id": siteID,
		"supplier_id":          supplierID,
		"number":               "STALE",
		"items": []map[string]any{
			{
				"id":              secondItemID,
				"material_id":     materialID,
				"measure_unit_id": measureUnitID,
				"quant":           5,
				"price":           2,
				"amount":          10,
			},
		},
	}, http.StatusConflict)

	materialList := c.DoJSON(t, http.MethodGet, "/api/material?count=1000", nil, http.StatusOK)
	materialRow := findCollectionRowByID(t, materialList, materialID)
	requireMaterialSiteBalance(t, materialRow, siteID, 6)

	destinationSiteID := createAPIObject(t, c, "/api/construction-sites", map[string]any{
		"name":      "document-destination-site-" + suffix,
		"is_active": true,
	})
	t.Cleanup(func() {
		c.DeleteIgnore(t, fmt.Sprintf("/api/construction-sites/%d", destinationSiteID))
	})

	transfer := c.DoJSON(t, http.MethodPost, "/api/material-transfers", map[string]any{
		"date":                             "2026-08-22T00:00:00Z",
		"source_construction_site_id":      siteID,
		"destination_construction_site_id": destinationSiteID,
		"items": []map[string]any{
			{
				"material_id":     materialID,
				"measure_unit_id": measureUnitID,
				"quant":           2,
			},
		},
	}, http.StatusCreated)
	transferID := intJSON(t, transfer, "id")
	t.Cleanup(func() { c.DeleteIgnore(t, fmt.Sprintf("/api/material-transfers/%d", transferID)) })
	if intJSON(t, transfer, "version") != 1 || len(materialDocumentItems(t, transfer)) != 1 {
		t.Fatalf("transfer response does not contain the complete aggregate: %v", transfer)
	}

	materialList = c.DoJSON(t, http.MethodGet, "/api/material?count=1000", nil, http.StatusOK)
	materialRow = findCollectionRowByID(t, materialList, materialID)
	requireMaterialSiteBalance(t, materialRow, siteID, 4)
	requireMaterialSiteBalance(t, materialRow, destinationSiteID, 2)
}

func materialDocumentItems(t *testing.T, document map[string]any) []map[string]any {
	t.Helper()
	rawItems := requireObjectField(t, document, "items")
	items, ok := rawItems.([]any)
	if !ok {
		t.Fatalf("items has unexpected type %T", rawItems)
	}
	result := make([]map[string]any, 0, len(items))
	for index, rawItem := range items {
		item, ok := rawItem.(map[string]any)
		if !ok {
			t.Fatalf("items[%d] has unexpected type %T", index, rawItem)
		}
		result = append(result, item)
	}
	return result
}

func requireMaterialSiteBalance(t *testing.T, material map[string]any, siteID int, want float64) {
	t.Helper()
	rawBalances := requireObjectField(t, material, "balances")
	balances, ok := rawBalances.([]any)
	if !ok {
		t.Fatalf("balances has unexpected type %T", rawBalances)
	}
	for _, rawBalance := range balances {
		balance, ok := rawBalance.(map[string]any)
		if !ok || intJSON(t, balance, "construction_site_id") != siteID {
			continue
		}
		requireJSONNumber(t, balance, "quant", want)
		return
	}
	t.Fatalf("balance for construction site %d was not found", siteID)
}
