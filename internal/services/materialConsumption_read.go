package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/dronm/ds/v4"
	"github.com/dronm/skstruktura/internal/models"
	"github.com/dronm/webapp"
)

type materialConsumptionReadKey struct {
	ID int `json:"id" primaryKey:"true" required:"true"`
}

func (k materialConsumptionReadKey) Relation() string {
	return "public.material_consumptions_list"
}

func (s *MaterialConsumptionService) Detail(
	ctx context.Context,
	id int,
) (*models.MaterialConsumptionList, error) {
	if err := s.requireSession(); err != nil {
		return nil, err
	}
	if err := s.requireDB(); err != nil {
		return nil, err
	}
	if id <= 0 {
		return nil, webapp.BadRequest("material consumption id is required", nil)
	}

	result, err := webapp.FetchModel(
		ctx,
		s.DB,
		materialConsumptionReadKey{
			ID: id,
		},
		&models.MaterialConsumptionList{},
	)
	if err != nil {
		if errors.Is(err, ds.ErrNoRows) {
			return nil, webapp.NotFound(
				"material consumption not found",
				map[string]any{
					"id": id,
				},
			)
		}
		return nil, fmt.Errorf("fetch material consumption: %w", err)
	}

	return result, nil
}
