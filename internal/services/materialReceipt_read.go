package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/dronm/ds/v4"
	"github.com/dronm/skstruktura/internal/models"
	"github.com/dronm/webapp"
)

type materialReceiptReadKey struct {
	ID int `json:"id" primaryKey:"true" required:"true"`
}

func (k materialReceiptReadKey) Relation() string {
	return "public.material_receipts_list"
}

func (s *MaterialReceiptService) Detail(
	ctx context.Context,
	id int,
) (*models.MaterialReceiptList, error) {
	if err := s.requireSession(); err != nil {
		return nil, err
	}
	if err := s.requireDB(); err != nil {
		return nil, err
	}
	if id <= 0 {
		return nil, webapp.BadRequest("material receipt id is required", nil)
	}

	result, err := webapp.FetchModel(
		ctx,
		s.DB,
		materialReceiptReadKey{
			ID: id,
		},
		&models.MaterialReceiptList{},
	)
	if err != nil {
		if errors.Is(err, ds.ErrNoRows) {
			return nil, webapp.NotFound(
				"material receipt not found",
				map[string]any{
					"id": id,
				},
			)
		}
		return nil, fmt.Errorf("fetch material receipt: %w", err)
	}

	return result, nil
}
