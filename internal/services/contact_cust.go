package services

import (
	"context"
	"fmt"
	"strings"

	"github.com/dronm/skstruktura/internal/models"
)

const contactAutocompleteDefaultLimit = 20
const contactAutocompleteMaxLimit = 50

func (s *ContactService) Autocomplete(
	ctx context.Context,
	input models.ContactAutocompleteInput,
) ([]*models.Ref, error) {
	if err := s.requireSession(); err != nil {
		return nil, err
	}
	if err := s.requireDB(); err != nil {
		return nil, err
	}

	words := contactSearchWords(input.Query)
	limit := input.Limit
	if limit <= 0 {
		limit = contactAutocompleteDefaultLimit
	}
	if limit > contactAutocompleteMaxLimit {
		limit = contactAutocompleteMaxLimit
	}

	poolConn, connID, err := s.DB.GetPrimary(ctx)
	if err != nil {
		return nil, fmt.Errorf("get primary connection for contact autocomplete: %w", err)
	}
	defer s.DB.Release(poolConn, connID)

	query := `
		SELECT public.contacts_ref(contact)
		FROM public.contacts AS contact
		WHERE contact.is_active
	`
	args := make([]any, 0, len(words)+1)
	if len(words) > 0 {
		conditions := make([]string, 0, len(words))
		for _, word := range words {
			args = append(args, "%"+word+"%")
			conditions = append(
				conditions,
				fmt.Sprintf("contact.search ILIKE $%d", len(args)),
			)
		}
		query += "\n\t\tAND (" + strings.Join(conditions, " OR ") + ")"
	}

	args = append(args, limit)
	query += fmt.Sprintf(`
		ORDER BY contact.name, contact.id
		LIMIT $%d
	`, len(args))

	rows, err := poolConn.Conn().Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("select contacts for autocomplete: %w", err)
	}
	defer rows.Close()

	result := make([]*models.Ref, 0)
	for rows.Next() {
		var item *models.Ref
		if err := rows.Scan(&item); err != nil {
			return nil, fmt.Errorf("scan contact autocomplete item: %w", err)
		}
		if item != nil {
			result = append(result, item)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate contact autocomplete items: %w", err)
	}

	return result, nil
}

func contactSearchWords(query string) []string {
	fields := strings.Fields(query)
	if len(fields) == 0 {
		return nil
	}

	result := make([]string, 0, len(fields))
	seen := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field == "" {
			continue
		}
		key := strings.ToLower(field)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, field)
	}

	return result
}
