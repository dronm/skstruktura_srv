package httpapi

import (
	"net/http"

	"github.com/dronm/modelbind"
	"github.com/dronm/skstruktura/internal/models"
	"github.com/dronm/webapp"
)

func constructionManagerMaterialConsumptionRoutes(api *webapp.Group) {
	api.POST(
		"/construction-manager/material-consumptions",
		webapp.WithName("constructionManager.materialConsumption.create"),
		webapp.WithPermission("constructionManager.materialConsumption.create"),
		webapp.WithService("MaterialConsumption", "ConstructionManagerCreate"),
		webapp.WithBinder(constructionManagerMaterialConsumptionDocumentBinder()),
		webapp.WithSuccessCode(http.StatusCreated),
	)

	api.GET(
		"/construction-manager/material-consumptions",
		webapp.WithName("constructionManager.materialConsumption.list"),
		webapp.WithPermission("constructionManager.materialConsumption.list"),
		webapp.WithService("MaterialConsumption", "ConstructionManagerList"),
		webapp.WithBinder(constructionManagerMaterialConsumptionBinder()),
	)

	api.GET(
		"/construction-manager/material-consumptions/{id}",
		webapp.WithName("constructionManager.materialConsumption.detail"),
		webapp.WithPermission("constructionManager.materialConsumption.list"),
		webapp.WithService("MaterialConsumption", "ConstructionManagerDetail"),
		webapp.WithBinder(webapp.PathValueBinder[int]("id")),
	)
}

func constructionManagerMaterialConsumptionDocumentBinder() webapp.Binder {
	return documentJSONBinder[models.MaterialConsumptionDocument]()
}

func constructionManagerMaterialConsumptionBinder() webapp.Binder {
	return func(r *http.Request) (any, error) {
		input, err := modelbind.DecodeURLValuesInput[*models.ConstructionManagerMaterialConsumptionQuery](r.URL.Query())
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

		return models.ConstructionManagerMaterialConsumptionInput{
			Query:  input.Model,
			Params: params,
		}, nil
	}
}
