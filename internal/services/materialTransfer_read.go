package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/dronm/ds/v4"
	"github.com/dronm/skstruktura/internal/models"
	"github.com/dronm/webapp"
)

type materialTransferReadKey struct {
	ID int `json:"id" primaryKey:"true" required:"true"`
}

func (k materialTransferReadKey) Relation() string {
	return "public.material_transfers_list"
}

func (s *MaterialTransferService) Detail(
	ctx context.Context,
	id int,
) (*models.MaterialTransferList, error) {
	if err := s.requireSession(); err != nil {
		return nil, err
	}
	if err := s.requireDB(); err != nil {
		return nil, err
	}
	if id <= 0 {
		return nil, webapp.BadRequest("material transfer id is required", nil)
	}

	result, err := webapp.FetchModel(
		ctx,
		s.DB,
		materialTransferReadKey{
			ID: id,
		},
		&models.MaterialTransferList{},
	)
	if err != nil {
		if errors.Is(err, ds.ErrNoRows) {
			return nil, webapp.NotFound(
				"material transfer not found",
				map[string]any{
					"id": id,
				},
			)
		}
		return nil, fmt.Errorf("fetch material transfer: %w", err)
	}

	return result, nil
}
