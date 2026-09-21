//go:build integration

package apitest

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"
)

func TestMaterialBalanceReport(t *testing.T) {
	c := NewClient(t)
	c.Login(t)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	supplierINN := suffix[len(suffix)-12:]
	measureUnitID := createAPIObject(t, c, "/api/measure-unit", map[string]any{
		"name":      "balance-unit-" + suffix,
		"name_full": "Material balance report unit " + suffix,
		"is_active": true,
	})
	t.Cleanup(func() { c.DeleteIgnore(t, fmt.Sprintf("/api/measure-unit/%d", measureUnitID)) })

	materialTypeAID := createAPIObject(t, c, "/api/material-types", map[string]any{
		"name":      "A-balance-type-" + suffix,
		"code":      "BAL-A-" + suffix,
		"is_active": true,
	})
	t.Cleanup(func() { c.DeleteIgnore(t, fmt.Sprintf("/api/material-types/%d", materialTypeAID)) })
	materialTypeBID := createAPIObject(t, c, "/api/material-types", map[string]any{
		"name":      "B-balance-type-" + suffix,
		"code":      "BAL-B-" + suffix,
		"is_active": true,
	})
	t.Cleanup(func() { c.DeleteIgnore(t, fmt.Sprintf("/api/material-types/%d", materialTypeBID)) })

	createMaterial := func(name string, materialTypeID int) int {
		id := createAPIObject(t, c, "/api/material", map[string]any{
			"name":             name + suffix,
			"name_full":        name + suffix,
			"measure_unit_id":  measureUnitID,
			"material_type_id": materialTypeID,
			"is_active":        true,
		})
		t.Cleanup(func() { c.DeleteIgnore(t, fmt.Sprintf("/api/material/%d", id)) })
		return id
	}
	materialAHighID := createMaterial("balance-a-high-", materialTypeAID)
	materialALowID := createMaterial("balance-a-low-", materialTypeAID)
	materialAZeroID := createMaterial("balance-a-zero-", materialTypeAID)
	materialBID := createMaterial("balance-b-", materialTypeBID)
	inactiveMaterialID := createAPIObject(t, c, "/api/material", map[string]any{
		"name":             "balance-inactive-" + suffix,
		"name_full":        "balance-inactive-" + suffix,
		"measure_unit_id":  measureUnitID,
		"material_type_id": materialTypeAID,
		"is_active":        false,
	})
	t.Cleanup(func() { c.DeleteIgnore(t, fmt.Sprintf("/api/material/%d", inactiveMaterialID)) })

	siteID := createAPIObject(t, c, "/api/construction-sites", map[string]any{
		"name":      "balance-site-" + suffix,
		"is_active": true,
	})
	t.Cleanup(func() { c.DeleteIgnore(t, fmt.Sprintf("/api/construction-sites/%d", siteID)) })
	otherSiteID := createAPIObject(t, c, "/api/construction-sites", map[string]any{
		"name":      "balance-other-site-" + suffix,
		"is_active": true,
	})
	t.Cleanup(func() { c.DeleteIgnore(t, fmt.Sprintf("/api/construction-sites/%d", otherSiteID)) })
	inactiveSiteID := createAPIObject(t, c, "/api/construction-sites", map[string]any{
		"name":      "balance-inactive-site-" + suffix,
		"is_active": false,
	})
	t.Cleanup(func() { c.DeleteIgnore(t, fmt.Sprintf("/api/construction-sites/%d", inactiveSiteID)) })

	managerName := "balance-manager-" + suffix
	managerPassword := "integration-test-password"
	managerID := createAPIObject(t, c, "/api/users", map[string]any{
		"name":                  managerName,
		"role_id":               "construction_site_manager",
		"pwd":                   managerPassword,
		"construction_site_ids": []int{siteID, inactiveSiteID},
	})
	t.Cleanup(func() { c.DeleteIgnore(t, fmt.Sprintf("/api/users/%d", managerID)) })

	supplierID := createAPIObject(t, c, "/api/supplier", map[string]any{
		"name":      "balance-supplier-" + suffix,
		"name_full": "Material balance report supplier " + suffix,
		"inn":       supplierINN,
		"kpp":       "",
		"is_active": true,
	})
	t.Cleanup(func() { c.DeleteIgnore(t, fmt.Sprintf("/api/supplier/%d", supplierID)) })

	receiptID := createAPIObject(t, c, "/api/material-receipts", map[string]any{
		"date":                 "2026-09-21T00:00:00Z",
		"construction_site_id": siteID,
		"supplier_id":          supplierID,
		"number":               "BAL-" + suffix,
		"items": []map[string]any{
			{
				"material_id":     materialALowID,
				"measure_unit_id": measureUnitID,
				"quant":           5,
				"price":           1,
				"amount":          5,
			},
			{
				"material_id":     materialAHighID,
				"measure_unit_id": measureUnitID,
				"quant":           10,
				"price":           1,
				"amount":          10,
			},
			{
				"material_id":     materialBID,
				"measure_unit_id": measureUnitID,
				"quant":           7,
				"price":           1,
				"amount":          7,
			},
		},
	})
	t.Cleanup(func() { c.DeleteIgnore(t, fmt.Sprintf("/api/material-receipts/%d", receiptID)) })

	sitesResponse := c.DoJSON(
		t,
		http.MethodGet,
		"/api/reports/material-balance/sites",
		nil,
		http.StatusOK,
	)
	sites, ok := sitesResponse["rows"].([]any)
	if !ok {
		t.Fatalf("construction site rows type = %T", sitesResponse["rows"])
	}
	foundSite := false
	for _, rawSite := range sites {
		site, ok := rawSite.(map[string]any)
		if ok && intJSON(t, site, "id") == siteID {
			foundSite = true
			break
		}
	}
	if !foundSite {
		t.Fatalf("construction site %d is missing from material balance selector", siteID)
	}
	for _, rawSite := range sites {
		site, ok := rawSite.(map[string]any)
		if ok && intJSON(t, site, "id") == inactiveSiteID {
			t.Fatalf("inactive construction site %d is present in material balance selector", inactiveSiteID)
		}
	}

	managerAliasSitesResponse := c.DoJSON(
		t,
		http.MethodGet,
		"/api/construction-manager/sites",
		nil,
		http.StatusOK,
	)
	managerAliasSites, ok := managerAliasSitesResponse["rows"].([]any)
	if !ok {
		t.Fatalf("construction manager site rows type = %T", managerAliasSitesResponse["rows"])
	}
	foundSite = false
	for _, rawSite := range managerAliasSites {
		site, ok := rawSite.(map[string]any)
		if !ok {
			continue
		}
		siteIDValue := intJSON(t, site, "id")
		if siteIDValue == siteID {
			foundSite = true
		}
		if siteIDValue == inactiveSiteID {
			t.Fatalf("inactive construction site %d is present in construction manager selector", inactiveSiteID)
		}
	}
	if !foundSite {
		t.Fatalf("construction site %d is missing from construction manager selector", siteID)
	}

	values := url.Values{"construction_site_id": {fmt.Sprint(siteID)}}
	response := c.DoJSON(
		t,
		http.MethodGet,
		"/api/reports/material-balance?"+values.Encode(),
		nil,
		http.StatusOK,
	)
	rawRows, ok := response["rows"].([]any)
	if !ok {
		t.Fatalf("material balance rows type = %T", response["rows"])
	}
	if len(rawRows) != 3 {
		t.Fatalf("material balance row count = %d, want 3", len(rawRows))
	}

	wantMaterialIDs := []int{materialAHighID, materialALowID, materialBID}
	wantBalances := []float64{10, 5, 7}
	for index, rawRow := range rawRows {
		row, ok := rawRow.(map[string]any)
		if !ok {
			t.Fatalf("row %d type = %T", index, rawRow)
		}
		if got := intJSON(t, row, "material_id"); got != wantMaterialIDs[index] {
			t.Fatalf("row %d material_id = %d, want %d", index, got, wantMaterialIDs[index])
		}
		requireJSONNumber(t, row, "balance", wantBalances[index])
	}

	materialsResponse := c.DoJSON(
		t,
		http.MethodGet,
		"/api/construction-manager/materials?"+values.Encode()+"&count=5000",
		nil,
		http.StatusOK,
	)
	materialRows, ok := materialsResponse["rows"].([]any)
	if !ok {
		t.Fatalf("construction manager material rows type = %T", materialsResponse["rows"])
	}
	wantMaterialBalances := map[int]float64{
		materialAHighID: 10,
		materialALowID:  5,
		materialAZeroID: 0,
		materialBID:     7,
	}
	foundMaterialIDs := make(map[int]bool, len(wantMaterialBalances))
	for _, rawRow := range materialRows {
		row, ok := rawRow.(map[string]any)
		if !ok {
			t.Fatalf("construction manager material row type = %T", rawRow)
		}
		materialID := intJSON(t, row, "material_id")
		if materialID == inactiveMaterialID {
			t.Fatalf("inactive material %d is present in construction manager materials", inactiveMaterialID)
		}
		wantBalance, tracked := wantMaterialBalances[materialID]
		if !tracked {
			continue
		}
		if got := intJSON(t, row, "construction_site_id"); got != siteID {
			t.Fatalf("material %d construction_site_id = %d, want %d", materialID, got, siteID)
		}
		requireJSONNumber(t, row, "balance", wantBalance)
		foundMaterialIDs[materialID] = true
	}
	for materialID := range wantMaterialBalances {
		if !foundMaterialIDs[materialID] {
			t.Errorf("material %d is missing from construction manager materials", materialID)
		}
	}

	managerClient := NewClient(t)
	managerClient.User = managerName
	managerClient.Pwd = managerPassword
	managerClient.Login(t)

	managerSitesResponse := managerClient.DoJSON(
		t,
		http.MethodGet,
		"/api/reports/material-balance/sites",
		nil,
		http.StatusOK,
	)
	managerSites, ok := managerSitesResponse["rows"].([]any)
	if !ok {
		t.Fatalf("manager construction site rows type = %T", managerSitesResponse["rows"])
	}
	if len(managerSites) != 1 {
		t.Fatalf("manager construction site row count = %d, want 1", len(managerSites))
	}
	managerSite, ok := managerSites[0].(map[string]any)
	if !ok || intJSON(t, managerSite, "id") != siteID {
		t.Fatalf("manager construction site = %#v, want id %d", managerSites[0], siteID)
	}
	managerAliasSitesResponse = managerClient.DoJSON(
		t,
		http.MethodGet,
		"/api/construction-manager/sites",
		nil,
		http.StatusOK,
	)
	managerAliasSites, ok = managerAliasSitesResponse["rows"].([]any)
	if !ok {
		t.Fatalf("manager alias construction site rows type = %T", managerAliasSitesResponse["rows"])
	}
	if len(managerAliasSites) != 1 {
		t.Fatalf("manager alias construction site row count = %d, want 1", len(managerAliasSites))
	}
	managerSite, ok = managerAliasSites[0].(map[string]any)
	if !ok || intJSON(t, managerSite, "id") != siteID {
		t.Fatalf("manager alias construction site = %#v, want id %d", managerAliasSites[0], siteID)
	}

	managerClient.DoJSON(
		t,
		http.MethodGet,
		"/api/reports/material-balance?construction_site_id="+fmt.Sprint(siteID),
		nil,
		http.StatusOK,
	)
	managerClient.DoJSON(
		t,
		http.MethodGet,
		"/api/reports/material-balance?construction_site_id="+fmt.Sprint(otherSiteID),
		nil,
		http.StatusForbidden,
	)
	managerClient.DoJSON(
		t,
		http.MethodGet,
		"/api/construction-manager/materials?construction_site_id="+fmt.Sprint(siteID),
		nil,
		http.StatusOK,
	)
	managerClient.DoJSON(
		t,
		http.MethodGet,
		"/api/construction-manager/materials?construction_site_id="+fmt.Sprint(otherSiteID),
		nil,
		http.StatusForbidden,
	)
	managerClient.DoJSON(
		t,
		http.MethodGet,
		"/api/construction-manager/materials?construction_site_id="+fmt.Sprint(inactiveSiteID),
		nil,
		http.StatusForbidden,
	)
}
