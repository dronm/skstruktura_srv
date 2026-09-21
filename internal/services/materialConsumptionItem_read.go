package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/dronm/ds/v4"
	"github.com/dronm/skstruktura/internal/models"
	"github.com/dronm/webapp"
)

type materialConsumptionItemReadKey struct {
	ID int `json:"id" primaryKey:"true" required:"true"`
}

func (k materialConsumptionItemReadKey) Relation() string {
	return "public.material_consumption_items_list"
}

func (s *MaterialConsumptionItemService) Detail(
	ctx context.Context,
	id int,
) (*models.MaterialConsumptionItemList, error) {
	if err := s.requireSession(); err != nil {
		return nil, err
	}
	if err := s.requireDB(); err != nil {
		return nil, err
	}
	if id <= 0 {
		return nil, webapp.BadRequest("material consumption item id is required", nil)
	}

	result, err := webapp.FetchModel(
		ctx,
		s.DB,
		materialConsumptionItemReadKey{
			ID: id,
		},
		&models.MaterialConsumptionItemList{},
	)
	if err != nil {
		if errors.Is(err, ds.ErrNoRows) {
			return nil, webapp.NotFound(
				"material consumption item not found",
				map[string]any{
					"id": id,
				},
			)
		}
		return nil, fmt.Errorf("fetch material consumption item: %w", err)
	}

	return result, nil
}
