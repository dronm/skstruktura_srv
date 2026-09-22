//go:build integration

package apitest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"
)

func TestMaterialRequestLifecycle(t *testing.T) {
	c := NewClient(t)
	c.Login(t)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	measureUnitID := createAPIObject(t, c, "/api/measure-unit", map[string]any{
		"name":      "request-unit-" + suffix,
		"name_full": "Material request unit " + suffix,
		"is_active": true,
	})
	t.Cleanup(func() {
		c.DeleteIgnore(t, fmt.Sprintf("/api/measure-unit/%d", measureUnitID))
	})

	otherMeasureUnitID := createAPIObject(t, c, "/api/measure-unit", map[string]any{
		"name":      "request-other-unit-" + suffix,
		"name_full": "Other material request unit " + suffix,
		"is_active": true,
	})
	t.Cleanup(func() {
		c.DeleteIgnore(t, fmt.Sprintf("/api/measure-unit/%d", otherMeasureUnitID))
	})

	materialTypeID := createAPIObject(t, c, "/api/material-types", map[string]any{
		"name":      "request-material-type-" + suffix,
		"code":      "REQUEST-MT-" + suffix,
		"is_active": true,
	})
	t.Cleanup(func() {
		c.DeleteIgnore(t, fmt.Sprintf("/api/material-types/%d", materialTypeID))
	})

	materialID := createAPIObject(t, c, "/api/material", map[string]any{
		"name":             "request-material-" + suffix,
		"name_full":        "Material request material " + suffix,
		"measure_unit_id":  measureUnitID,
		"material_type_id": materialTypeID,
		"is_active":        true,
	})
	t.Cleanup(func() {
		c.DeleteIgnore(t, fmt.Sprintf("/api/material/%d", materialID))
	})

	siteID := createAPIObject(t, c, "/api/construction-sites", map[string]any{
		"name":      "request-site-" + suffix,
		"is_active": true,
	})
	t.Cleanup(func() {
		c.DeleteIgnore(t, fmt.Sprintf("/api/construction-sites/%d", siteID))
	})

	managerID := createAPIObject(t, c, "/api/users", map[string]any{
		"name":                  "request-manager-" + suffix,
		"role_id":               "construction_site_manager",
		"pwd":                   "integration-test-password",
		"construction_site_ids": []int{siteID},
	})
	t.Cleanup(func() {
		c.DeleteIgnore(t, fmt.Sprintf("/api/users/%d", managerID))
	})

	importanceID := createAPIObject(t, c, "/api/order-importances", map[string]any{
		"name":       "request-importance-" + suffix,
		"sort_order": 100,
		"is_active":  true,
	})
	t.Cleanup(func() {
		c.DeleteIgnore(t, fmt.Sprintf("/api/order-importances/%d", importanceID))
	})

	requestBody := map[string]any{
		"date":                    "2026-09-21T08:30:00+05:00",
		"construction_site_id":    siteID,
		"construction_manager_id": managerID,
		"comment":                 nil,
		"items": []map[string]any{
			{
				"material_id":         materialID,
				"measure_unit_id":     measureUnitID,
				"quant":               2.5,
				"supplier_id":         nil,
				"required_date":       nil,
				"order_importance_id": importanceID,
			},
		},
	}

	invalidUnitBody := cloneMaterialRequestBody(t, requestBody)
	invalidItems := invalidUnitBody["items"].([]any)
	invalidItems[0].(map[string]any)["measure_unit_id"] = float64(otherMeasureUnitID)
	c.DoJSON(
		t,
		http.MethodPost,
		"/api/material-requests",
		invalidUnitBody,
		http.StatusBadRequest,
	)

	created := c.DoJSON(
		t,
		http.MethodPost,
		"/api/material-requests",
		requestBody,
		http.StatusCreated,
	)
	requestID := intJSON(t, created, "id")
	t.Cleanup(func() {
		deleteMaterialRequestFixture(t, requestID, suffix)
	})
	if version := intJSON(t, created, "version"); version != 1 {
		t.Fatalf("created version = %d, want 1", version)
	}
	if statusID := intJSON(t, created, "status_id"); statusID != 1 {
		t.Fatalf("created header status_id = %d, want draft status id 1", statusID)
	}
	createdItems := materialDocumentItems(t, created)
	if len(createdItems) != 1 {
		t.Fatalf("created item count = %d, want 1", len(createdItems))
	}
	createdItem := createdItems[0]
	if statusID := intJSON(t, createdItem, "status_id"); statusID != 1 {
		t.Fatalf("created status_id = %d, want draft status id 1", statusID)
	}
	if createdItem["supplier_id"] != nil {
		t.Fatalf("created supplier_id = %#v, want null", createdItem["supplier_id"])
	}
	if createdItem["required_date"] != nil {
		t.Fatalf("created required_date = %#v, want null", createdItem["required_date"])
	}

	submitted := c.DoJSON(
		t,
		http.MethodPost,
		fmt.Sprintf("/api/material-requests/%d/submit", requestID),
		map[string]any{"version": 1},
		http.StatusOK,
	)
	if version := intJSON(t, submitted, "version"); version != 2 {
		t.Fatalf("submitted version = %d, want 2", version)
	}
	if statusID := intJSON(t, submitted, "status_id"); statusID != 2 {
		t.Fatalf("submitted header status_id = %d, want submitted status id 2", statusID)
	}
	submittedItems := materialDocumentItems(t, submitted)
	if len(submittedItems) != 1 {
		t.Fatalf("submitted item count = %d, want 1", len(submittedItems))
	}
	if statusID := intJSON(t, submittedItems[0], "status_id"); statusID != 2 {
		t.Fatalf("submitted status_id = %d, want submitted status id 2", statusID)
	}

	c.DoJSON(
		t,
		http.MethodDelete,
		fmt.Sprintf("/api/material-requests/%d", requestID),
		nil,
		http.StatusConflict,
	)

	c.DoJSON(
		t,
		http.MethodPost,
		fmt.Sprintf("/api/material-requests/%d/submit", requestID),
		map[string]any{"version": 1},
		http.StatusConflict,
	)
	c.DoJSON(
		t,
		http.MethodPost,
		fmt.Sprintf("/api/material-requests/%d/submit", requestID),
		map[string]any{"version": 2},
		http.StatusConflict,
	)
}

func cloneMaterialRequestBody(t *testing.T, body map[string]any) map[string]any {
	t.Helper()

	data, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	return result
}
