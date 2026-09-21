package models

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"
)

// DateOnly is a calendar date transported as YYYY-MM-DD. It deliberately has
// no time zone: converting a requested delivery date to an instant would make
// the visible day depend on the client or server location.
type DateOnly string

func (d *DateOnly) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, []byte("null")) {
		return nil
	}

	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return fmt.Errorf("date should be a YYYY-MM-DD string: %w", err)
	}
	if _, err := time.Parse(time.DateOnly, value); err != nil {
		return fmt.Errorf("date should use YYYY-MM-DD format: %w", err)
	}

	*d = DateOnly(value)
	return nil
}

func (d DateOnly) String() string {
	return string(d)
}
