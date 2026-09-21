package services

import (
	"reflect"
	"testing"

	"github.com/dronm/webapp"
)

func TestHashUserPassword(t *testing.T) {
	t.Parallel()

	got := hashUserPassword("123456")
	const want = "e10adc3949ba59abbe56e057f20f883e"
	if got != want {
		t.Fatalf("hashUserPassword() = %q, want %q", got, want)
	}
}

func TestUserConstructionSiteIDSet(t *testing.T) {
	t.Parallel()

	got, err := userConstructionSiteIDSet([]int{7, 2, 11})
	if err != nil {
		t.Fatalf("userConstructionSiteIDSet() error = %v", err)
	}
	for _, constructionSiteID := range []int{2, 7, 11} {
		if _, ok := got[constructionSiteID]; !ok {
			t.Fatalf("result does not contain construction site %d", constructionSiteID)
		}
	}

	for _, test := range []struct {
		name string
		ids  []int
	}{
		{name: "zero", ids: []int{0}},
		{name: "negative", ids: []int{-1}},
		{name: "duplicate", ids: []int{3, 3}},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := userConstructionSiteIDSet(test.ids)
			if err == nil {
				t.Fatal("invalid construction site ids were accepted")
			}
			appErr, ok := err.(*webapp.AppError)
			if !ok || appErr.StatusCode() != 400 {
				t.Fatalf("error = %#v, want HTTP 400 application error", err)
			}
		})
	}
}

func TestValidateUserConstructionSiteIDsRejectsExplicitNull(t *testing.T) {
	t.Parallel()

	if _, err := validateUserConstructionSiteIDs(nil, false); err != nil {
		t.Fatalf("omitted construction_site_ids error = %v", err)
	}
	if _, err := validateUserConstructionSiteIDs([]int{}, true); err != nil {
		t.Fatalf("empty construction_site_ids error = %v", err)
	}

	_, err := validateUserConstructionSiteIDs(nil, true)
	if err == nil {
		t.Fatal("explicit null construction_site_ids was accepted")
	}
	appErr, ok := err.(*webapp.AppError)
	if !ok || appErr.StatusCode() != 400 {
		t.Fatalf("error = %#v, want HTTP 400 application error", err)
	}
}

func TestUserConstructionSiteChangesPreservesExistingMemberships(t *testing.T) {
	t.Parallel()

	existing := map[int]struct{}{1: {}, 2: {}, 4: {}}
	desired := map[int]struct{}{2: {}, 3: {}, 4: {}, 5: {}}

	removeIDs, addIDs := userConstructionSiteChanges(existing, desired)
	if !reflect.DeepEqual(removeIDs, []int{1}) {
		t.Fatalf("remove ids = %v, want [1]", removeIDs)
	}
	if !reflect.DeepEqual(addIDs, []int{3, 5}) {
		t.Fatalf("add ids = %v, want [3 5]", addIDs)
	}
}
