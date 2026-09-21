package models

// ContactAutocompleteInput contains parameters for contact reference lookup.
type ContactAutocompleteInput struct {
	Query string
	Limit int
}
