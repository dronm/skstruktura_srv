//go:build integration

package apitest

import (
	"fmt"
	"net/http"
	"reflect"
	"testing"
)

func TestUserListAndDetail(t *testing.T) {
	c := NewClient(t)
	c.Login(t)

	collection := c.DoJSON(
		t,
		http.MethodGet,
		"/api/users",
		nil,
		http.StatusOK,
	)
	rowsRaw := requireObjectField(t, collection, "rows")
	rows, ok := rowsRaw.([]any)
	if !ok {
		t.Fatalf("users.rows has unexpected type %T", rowsRaw)
	}
	if len(rows) == 0 {
		t.Fatal("users collection is empty")
	}

	for _, rowRaw := range rows {
		row, ok := rowRaw.(map[string]any)
		if !ok {
			t.Fatalf("users row has unexpected type %T", rowRaw)
		}
		if _, exposed := row["pwd"]; exposed {
			t.Fatal("users collection must not expose pwd")
		}
	}

	detail := c.DoJSON(
		t,
		http.MethodGet,
		"/api/users/1",
		nil,
		http.StatusOK,
	)
	if _, exposed := detail["pwd"]; exposed {
		t.Fatal("user detail must not expose pwd")
	}
	if _, ok := detail["name"]; !ok {
		t.Fatal("user detail is missing name")
	}
	if _, ok := detail["role_id"]; !ok {
		t.Fatal("user detail is missing role_id")
	}
	constructionSiteIDsRaw, ok := detail["construction_site_ids"]
	if !ok {
		t.Fatal("user detail is missing construction_site_ids")
	}
	constructionSiteIDs, ok := constructionSiteIDsRaw.([]any)
	if !ok {
		t.Fatalf(
			"user detail construction_site_ids has unexpected type %T",
			constructionSiteIDsRaw,
		)
	}
	for index, constructionSiteIDRaw := range constructionSiteIDs {
		constructionSiteID, ok := constructionSiteIDRaw.(float64)
		if !ok || constructionSiteID <= 0 {
			t.Fatalf(
				"user detail construction_site_ids[%d] = %#v, want a positive number",
				index,
				constructionSiteIDRaw,
			)
		}
	}
}

func TestUserConstructionSiteAggregate(t *testing.T) {
	c := NewClient(t)
	c.Login(t)

	constructionSiteBody := c.DoJSON(
		t,
		http.MethodPost,
		"/api/construction-sites",
		map[string]any{
			"name":      TestID("user_site"),
			"is_active": true,
		},
		http.StatusCreated,
	)
	constructionSiteID := IntFromJSON(t, constructionSiteBody, "id")

	userID := 0
	t.Cleanup(func() {
		if userID > 0 {
			c.DeleteIgnore(t, fmt.Sprintf("/api/users/%d", userID))
		}
		c.DeleteIgnore(t, fmt.Sprintf("/api/construction-sites/%d", constructionSiteID))
	})

	userBody := c.DoJSON(
		t,
		http.MethodPost,
		"/api/users",
		map[string]any{
			"name":                  TestID("site_manager"),
			"role_id":               "construction_site_manager",
			"pwd":                   "integration-test-password",
			"construction_site_ids": []int{constructionSiteID},
		},
		http.StatusCreated,
	)
	userID = IntFromJSON(t, userBody, "id")
	detailPath := fmt.Sprintf("/api/users/%d", userID)

	assertUserConstructionSiteIDs(t, c.DoJSON(
		t,
		http.MethodGet,
		detailPath,
		nil,
		http.StatusOK,
	), []int{constructionSiteID})

	// Omitting the aggregate field from PATCH must preserve the assignment.
	c.DoJSON(
		t,
		http.MethodPatch,
		detailPath,
		map[string]any{"name": TestID("site_manager_updated")},
		http.StatusOK,
	)
	assertUserConstructionSiteIDs(t, c.DoJSON(
		t,
		http.MethodGet,
		detailPath,
		nil,
		http.StatusOK,
	), []int{constructionSiteID})

	// JSON null is not the same operation as submitting an empty array.
	c.DoJSON(
		t,
		http.MethodPatch,
		detailPath,
		map[string]any{"construction_site_ids": nil},
		http.StatusBadRequest,
	)

	// An explicit empty array clears all assignments.
	c.DoJSON(
		t,
		http.MethodPatch,
		detailPath,
		map[string]any{"construction_site_ids": []int{}},
		http.StatusOK,
	)
	assertUserConstructionSiteIDs(t, c.DoJSON(
		t,
		http.MethodGet,
		detailPath,
		nil,
		http.StatusOK,
	), []int{})
}

func assertUserConstructionSiteIDs(t *testing.T, detail map[string]any, want []int) {
	t.Helper()

	raw, ok := detail["construction_site_ids"].([]any)
	if !ok {
		t.Fatalf(
			"construction_site_ids has unexpected type %T",
			detail["construction_site_ids"],
		)
	}
	got := make([]int, len(raw))
	for index, value := range raw {
		number, ok := value.(float64)
		if !ok {
			t.Fatalf("construction_site_ids[%d] has unexpected type %T", index, value)
		}
		got[index] = int(number)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("construction_site_ids = %v, want %v", got, want)
	}
}

func TestCurrentUserProfileAndPassword(t *testing.T) {
	c := NewClient(t)
	c.Login(t)

	profile := c.DoJSON(
		t,
		http.MethodGet,
		"/api/users/me",
		nil,
		http.StatusOK,
	)
	if got, _ := profile["name"].(string); got != c.User {
		t.Fatalf("current profile name = %q, want %q", got, c.User)
	}
	if _, exposed := profile["role_id"]; exposed {
		t.Fatal("current profile must not expose an editable role_id")
	}
	if _, exposed := profile["pwd"]; exposed {
		t.Fatal("current profile must not expose pwd")
	}

	updated := c.DoJSON(
		t,
		http.MethodPatch,
		"/api/users/me",
		map[string]any{
			"name": c.User,
		},
		http.StatusOK,
	)
	if got, _ := updated["name"].(string); got != c.User {
		t.Fatalf("updated current profile name = %q, want %q", got, c.User)
	}

	c.DoJSON(
		t,
		http.MethodPatch,
		"/api/users/me/password",
		map[string]any{
			"new_password": c.Pwd,
		},
		http.StatusOK,
	)

	// A fresh session must still be able to authenticate with the password just
	// written by the self-service endpoint.
	fresh := NewClient(t)
	fresh.Login(t)
}

func TestCurrentUserProfileRequiresAuthentication(t *testing.T) {
	c := NewClient(t)

	c.DoJSON(
		t,
		http.MethodGet,
		"/api/users/me",
		nil,
		http.StatusUnauthorized,
	)
	c.DoJSON(
		t,
		http.MethodPatch,
		"/api/users/me/password",
		map[string]any{
			"new_password": "not-used",
		},
		http.StatusUnauthorized,
	)
}
