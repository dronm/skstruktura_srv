package httpapi

import (
	"net/http"
	"time"

	"github.com/dronm/modelbind"
	"github.com/dronm/skstruktura/internal/models"
	"github.com/dronm/webapp"
)

func materialActionReportRoutes(api *webapp.Group) {
	api.GET(
		"/reports/material-actions",
		webapp.WithName("materialActionReport.list"),
		webapp.WithPermission("materialActionReport.list"),
		webapp.WithService("MaterialActionReport", "List"),
		webapp.WithBinder(materialActionReportBinder()),
	)
}

func materialActionReportBinder() webapp.Binder {
	return func(r *http.Request) (any, error) {
		values := r.URL.Query()
		dateFrom, err := parseMaterialActionReportTimestamp(values.Get("date_from"), "date_from")
		if err != nil {
			return nil, err
		}
		dateTo, err := parseMaterialActionReportTimestamp(values.Get("date_to"), "date_to")
		if err != nil {
			return nil, err
		}

		modelValues := r.URL.Query()
		modelValues.Del("date_from")
		modelValues.Del("date_to")
		input, err := modelbind.DecodeURLValuesInput[*models.MaterialActionReportQuery](modelValues)
		if err != nil {
			return nil, err
		}
		input.Model.DateFrom = dateFrom
		input.Model.DateTo = dateTo
		if err := input.Validate(true); err != nil {
			return nil, err
		}

		params, err := webapp.ParseCollectionParams(r)
		if err != nil {
			return nil, err
		}

		return models.MaterialActionReportInput{
			Query:  input.Model,
			Params: params,
		}, nil
	}
}

func parseMaterialActionReportTimestamp(value string, field string) (time.Time, error) {
	if value == "" {
		return time.Time{}, webapp.BadRequest(field+" is required", nil)
	}

	result, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, webapp.BadRequest(field+" should be an RFC 3339 timestamp", err)
	}
	return result, nil
}
