package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/dronm/ds/v4"
	"github.com/dronm/skstruktura/internal/models"
	"github.com/dronm/webapp"
)

type materialReceiptItemReadKey struct {
	ID int `json:"id" primaryKey:"true" required:"true"`
}

func (k materialReceiptItemReadKey) Relation() string {
	return "public.material_receipt_items_list"
}

func (s *MaterialReceiptItemService) Detail(
	ctx context.Context,
	id int,
) (*models.MaterialReceiptItemList, error) {
	if err := s.requireSession(); err != nil {
		return nil, err
	}
	if err := s.requireDB(); err != nil {
		return nil, err
	}
	if id <= 0 {
		return nil, webapp.BadRequest("material receipt item id is required", nil)
	}

	result, err := webapp.FetchModel(
		ctx,
		s.DB,
		materialReceiptItemReadKey{
			ID: id,
		},
		&models.MaterialReceiptItemList{},
	)
	if err != nil {
		if errors.Is(err, ds.ErrNoRows) {
			return nil, webapp.NotFound(
				"material receipt item not found",
				map[string]any{
					"id": id,
				},
			)
		}
		return nil, fmt.Errorf("fetch material receipt item: %w", err)
	}

	return result, nil
}
