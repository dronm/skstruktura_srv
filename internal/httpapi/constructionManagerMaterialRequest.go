package httpapi

import (
	"net/http"

	"github.com/dronm/modelbind"
	"github.com/dronm/skstruktura/internal/models"
	"github.com/dronm/webapp"
)

func constructionManagerMaterialRequestRoutes(api *webapp.Group) {
	api.GET(
		"/construction-manager/material-requests",
		webapp.WithName("constructionManager.materialRequests"),
		webapp.WithPermission("materialRequest.list"),
		webapp.WithService("MaterialRequest", "ConstructionManagerList"),
		webapp.WithBinder(constructionManagerMaterialRequestBinder()),
	)
}

func constructionManagerMaterialRequestBinder() webapp.Binder {
	return func(r *http.Request) (any, error) {
		input, err := modelbind.DecodeURLValuesInput[*models.ConstructionManagerMaterialRequestQuery](r.URL.Query())
		if err != nil {
			return nil, err
		}
		if err := input.Validate(true); err != nil {
			return nil, err
		}

		params, err := webapp.ParseCollectionParams(r)
		if err != nil {
			return nil, err
		}

		return models.ConstructionManagerMaterialRequestInput{
			Query:  input.Model,
			Params: params,
		}, nil
	}
}
