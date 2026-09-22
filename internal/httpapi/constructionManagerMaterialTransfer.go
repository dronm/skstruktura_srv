package httpapi

import (
	"net/http"

	"github.com/dronm/modelbind"
	"github.com/dronm/skstruktura/internal/models"
	"github.com/dronm/webapp"
)

func constructionManagerMaterialTransferRoutes(api *webapp.Group) {
	api.POST(
		"/construction-manager/material-transfers",
		webapp.WithName("constructionManager.materialTransfer.create"),
		webapp.WithPermission("constructionManager.materialTransfer.create"),
		webapp.WithService("MaterialTransfer", "ConstructionManagerCreate"),
		webapp.WithBinder(constructionManagerMaterialTransferDocumentBinder()),
		webapp.WithSuccessCode(http.StatusCreated),
	)

	api.GET(
		"/construction-manager/material-transfers",
		webapp.WithName("constructionManager.materialTransfer.list"),
		webapp.WithPermission("constructionManager.materialTransfer.list"),
		webapp.WithService("MaterialTransfer", "ConstructionManagerList"),
		webapp.WithBinder(constructionManagerMaterialTransferBinder()),
	)

	api.GET(
		"/construction-manager/material-transfers/{id}",
		webapp.WithName("constructionManager.materialTransfer.detail"),
		webapp.WithPermission("constructionManager.materialTransfer.list"),
		webapp.WithService("MaterialTransfer", "ConstructionManagerDetail"),
		webapp.WithBinder(webapp.PathValueBinder[int]("id")),
	)

	api.GET(
		"/construction-manager/transfer-destinations",
		webapp.WithName("constructionManager.materialTransfer.destinations"),
		webapp.WithPermission("constructionManager.materialTransfer.create"),
		webapp.WithService("MaterialTransfer", "ConstructionManagerDestinations"),
	)
}

func constructionManagerMaterialTransferDocumentBinder() webapp.Binder {
	return documentJSONBinder[models.MaterialTransferDocument]()
}

func constructionManagerMaterialTransferBinder() webapp.Binder {
	return func(r *http.Request) (any, error) {
		input, err := modelbind.DecodeURLValuesInput[*models.ConstructionManagerMaterialTransferQuery](r.URL.Query())
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

		return models.ConstructionManagerMaterialTransferInput{
			Query:  input.Model,
			Params: params,
		}, nil
	}
}
