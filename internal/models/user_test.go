package models

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dronm/modelbind"
)

func TestUserUpdateConstructionSiteIDsPresence(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		body        string
		wantPresent bool
		wantIDs     []int
		wantNil     bool
	}{
		{
			name:        "omitted preserves assignments",
			body:        `{"name":"Updated user"}`,
			wantPresent: false,
		},
		{
			name:        "empty clears assignments",
			body:        `{"construction_site_ids":[]}`,
			wantPresent: true,
			wantIDs:     []int{},
		},
		{
			name:        "values replace assignments",
			body:        `{"construction_site_ids":[3,7]}`,
			wantPresent: true,
			wantIDs:     []int{3, 7},
		},
		{
			name:        "null remains distinguishable from empty",
			body:        `{"construction_site_ids":null}`,
			wantPresent: true,
			wantNil:     true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest("PATCH", "/users/1", strings.NewReader(test.body))
			request.Header.Set("Content-Type", "application/json")

			input, err := modelbind.DecodeRequestInput[*UserUpdate](request)
			if err != nil {
				t.Fatalf("decode user update: %v", err)
			}
			if got := input.IsPresent("construction_site_ids"); got != test.wantPresent {
				t.Fatalf("construction_site_ids present = %v, want %v", got, test.wantPresent)
			}
			if test.wantPresent {
				if got := input.Model.ConstructionSiteIDs == nil; got != test.wantNil {
					t.Fatalf("construction_site_ids nil = %v, want %v", got, test.wantNil)
				}
				if len(input.Model.ConstructionSiteIDs) != len(test.wantIDs) {
					t.Fatalf(
						"construction_site_ids = %v, want %v",
						input.Model.ConstructionSiteIDs,
						test.wantIDs,
					)
				}
				for index, want := range test.wantIDs {
					if got := input.Model.ConstructionSiteIDs[index]; got != want {
						t.Fatalf(
							"construction_site_ids[%d] = %d, want %d",
							index,
							got,
							want,
						)
					}
				}
			}
		})
	}
}
