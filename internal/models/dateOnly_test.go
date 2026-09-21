package models

import (
	"encoding/json"
	"testing"
)

func TestDateOnlyJSONRoundTrip(t *testing.T) {
	t.Parallel()

	var value DateOnly
	if err := json.Unmarshal([]byte(`"2026-09-21"`), &value); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if got := value.String(); got != "2026-09-21" {
		t.Fatalf("DateOnly.String() = %q, want %q", got, "2026-09-21")
	}

	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if got := string(encoded); got != `"2026-09-21"` {
		t.Fatalf("Marshal() = %s, want %s", got, `"2026-09-21"`)
	}
}

func TestDateOnlyRejectsTimestampsAndInvalidDates(t *testing.T) {
	t.Parallel()

	for _, input := range []string{
		`""`,
		`"2026-09-21T00:00:00Z"`,
		`"2026-02-30"`,
		`"21.09.2026"`,
		`42`,
	} {
		input := input
		t.Run(input, func(t *testing.T) {
			t.Parallel()

			var value DateOnly
			if err := json.Unmarshal([]byte(input), &value); err == nil {
				t.Fatalf("Unmarshal(%s) error = nil, want an invalid date error", input)
			}
		})
	}
}

func TestNullableDateOnlyAcceptsNull(t *testing.T) {
	t.Parallel()

	var payload struct {
		RequiredDate *DateOnly `json:"required_date"`
	}
	if err := json.Unmarshal([]byte(`{"required_date":null}`), &payload); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if payload.RequiredDate != nil {
		t.Fatalf("RequiredDate = %q, want nil", payload.RequiredDate.String())
	}
}
