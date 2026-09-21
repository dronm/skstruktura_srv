package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/dronm/ds/v4"
	"github.com/dronm/skstruktura/internal/models"
	"github.com/dronm/webapp"
)

type materialReadKey struct {
	ID int `json:"id" primaryKey:"true" required:"true"`
}

func (k materialReadKey) Relation() string {
	return "public.materials_list"
}

func (s *MaterialService) Detail(
	ctx context.Context,
	id int,
) (*models.MaterialList, error) {
	if err := s.requireSession(); err != nil {
		return nil, err
	}
	if err := s.requireDB(); err != nil {
		return nil, err
	}
	if id <= 0 {
		return nil, webapp.BadRequest("material id is required", nil)
	}

	result, err := webapp.FetchModel(
		ctx,
		s.DB,
		materialReadKey{
			ID: id,
		},
		&models.MaterialList{},
	)
	if err != nil {
		if errors.Is(err, ds.ErrNoRows) {
			return nil, webapp.NotFound(
				"material not found",
				map[string]any{
					"id": id,
				},
			)
		}
		return nil, fmt.Errorf("fetch material: %w", err)
	}

	return result, nil
}
