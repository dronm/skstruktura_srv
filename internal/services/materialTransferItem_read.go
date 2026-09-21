package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/dronm/ds/v4"
	"github.com/dronm/skstruktura/internal/models"
	"github.com/dronm/webapp"
)

type materialTransferItemReadKey struct {
	ID int `json:"id" primaryKey:"true" required:"true"`
}

func (k materialTransferItemReadKey) Relation() string {
	return "public.material_transfer_items_list"
}

func (s *MaterialTransferItemService) Detail(
	ctx context.Context,
	id int,
) (*models.MaterialTransferItemList, error) {
	if err := s.requireSession(); err != nil {
		return nil, err
	}
	if err := s.requireDB(); err != nil {
		return nil, err
	}
	if id <= 0 {
		return nil, webapp.BadRequest("material transfer item id is required", nil)
	}

	result, err := webapp.FetchModel(
		ctx,
		s.DB,
		materialTransferItemReadKey{
			ID: id,
		},
		&models.MaterialTransferItemList{},
	)
	if err != nil {
		if errors.Is(err, ds.ErrNoRows) {
			return nil, webapp.NotFound(
				"material transfer item not found",
				map[string]any{
					"id": id,
				},
			)
		}
		return nil, fmt.Errorf("fetch material transfer item: %w", err)
	}

	return result, nil
}
