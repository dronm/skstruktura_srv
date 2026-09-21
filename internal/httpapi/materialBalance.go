package httpapi

import (
	"net/http"

	"github.com/dronm/modelbind"
	"github.com/dronm/skstruktura/internal/models"
	"github.com/dronm/webapp"
)

func materialBalanceRoutes(api *webapp.Group) {
	api.GET(
		"/reports/material-balance",
		webapp.WithName("materialBalance.list"),
		webapp.WithPermission("materialBalance.list"),
		webapp.WithService("MaterialBalance", "List"),
		webapp.WithBinder(materialBalanceBinder()),
	)

	api.GET(
		"/reports/material-balance/sites",
		webapp.WithName("materialBalance.constructionSites"),
		webapp.WithPermission("materialBalance.list"),
		webapp.WithService("MaterialBalance", "ConstructionSites"),
	)
}

func materialBalanceBinder() webapp.Binder {
	return func(r *http.Request) (any, error) {
		input, err := modelbind.DecodeURLValuesInput[*models.MaterialBalanceQuery](r.URL.Query())
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

		return models.MaterialBalanceInput{
			Query:  input.Model,
			Params: params,
		}, nil
	}
}
