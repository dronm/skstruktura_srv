package httpapi

import (
	"net/http"

	"github.com/dronm/modelbind"
	"github.com/dronm/skstruktura/internal/models"
	"github.com/dronm/webapp"
)

func constructionManagerMaterialStatusRoutes(api *webapp.Group) {
	api.GET(
		"/construction-manager/material-statuses/current",
		webapp.WithName("constructionManager.materialStatus.current"),
		webapp.WithPermission("constructionManager.materialStatus.list"),
		webapp.WithService("MaterialStatus", "ConstructionManagerCurrent"),
		webapp.WithBinder(constructionManagerMaterialStatusQueryBinder()),
	)

	api.POST(
		"/construction-manager/material-statuses",
		webapp.WithName("constructionManager.materialStatus.create"),
		webapp.WithPermission("constructionManager.materialStatus.create"),
		webapp.WithService("MaterialStatus", "ConstructionManagerCreate"),
		webapp.WithBinder(constructionManagerMaterialStatusChangeBinder()),
		webapp.WithSuccessCode(http.StatusCreated),
	)

	api.GET(
		"/construction-manager/material-statuses/history",
		webapp.WithName("constructionManager.materialStatus.history"),
		webapp.WithPermission("constructionManager.materialStatus.list"),
		webapp.WithService("MaterialStatus", "ConstructionManagerHistory"),
		webapp.WithBinder(constructionManagerMaterialStatusQueryBinder()),
	)
}

func constructionManagerMaterialStatusQueryBinder() webapp.Binder {
	return func(r *http.Request) (any, error) {
		input, err := modelbind.DecodeURLValuesInput[*models.ConstructionManagerMaterialStatusQuery](r.URL.Query())
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

		return models.ConstructionManagerMaterialStatusInput{
			Query:  input.Model,
			Params: params,
		}, nil
	}
}

func constructionManagerMaterialStatusChangeBinder() webapp.Binder {
	return documentJSONBinder[models.ConstructionManagerMaterialStatusChangeRequest]()
}
