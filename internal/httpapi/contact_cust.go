package httpapi

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/dronm/skstruktura/internal/models"
	"github.com/dronm/webapp"
)

func contactCustomRoutes(api *webapp.Group) {
	api.GET(
		"/contacts/autocomplete",
		webapp.WithName("contact.autocomplete"),
		webapp.WithPermission("contact.list"),
		webapp.WithService("Contact", "Autocomplete"),
		webapp.WithBinder(contactAutocompleteBinder()),
	)
}

func contactAutocompleteBinder() webapp.Binder {
	return func(r *http.Request) (any, error) {
		limit := 20
		if value := strings.TrimSpace(r.URL.Query().Get("limit")); value != "" {
			parsed, err := strconv.Atoi(value)
			if err != nil {
				return nil, fmt.Errorf("contact autocomplete limit should be integer: %w", err)
			}
			limit = parsed
		}

		query := strings.TrimSpace(r.URL.Query().Get("q"))
		if query == "" {
			query = strings.TrimSpace(r.URL.Query().Get("query"))
		}

		return models.ContactAutocompleteInput{
			Query: query,
			Limit: limit,
		}, nil
	}
}
