package httpapi

import (
	"net/http"
	"strings"
	"time"

	"github.com/dronm/modelbind"
	"github.com/dronm/skstruktura/internal/models"
	"github.com/dronm/webapp"
)

const (
	supplyManagerAssignmentCreatePermission = "materialRequestSupplierAssignment.create"
	supplyManagerAssignmentListPermission   = "materialRequestSupplierAssignment.list"
	supplyManagerAssignmentDetailPermission = "materialRequestSupplierAssignment.detail"
)

func supplyManagerWorkspaceRoutes(api *webapp.Group) {
	api.GET(
		"/supply-manager/sites",
		webapp.WithName("supplyManager.sites"),
		webapp.WithPermission(supplyManagerAssignmentCreatePermission),
		webapp.WithService("MaterialRequestSupplierAssignment", "SupplyManagerSites"),
	)

	api.GET(
		"/supply-manager/material-requests",
		webapp.WithName("supplyManager.materialRequests"),
		webapp.WithPermission(supplyManagerAssignmentCreatePermission),
		webapp.WithService("MaterialRequestSupplierAssignment", "SupplyManagerMaterialRequests"),
		webapp.WithBinder(supplyManagerMaterialRequestBinder()),
	)

	api.GET(
		"/supply-manager/material-requests/{id}",
		webapp.WithName("supplyManager.materialRequest.detail"),
		webapp.WithPermission(supplyManagerAssignmentCreatePermission),
		webapp.WithService("MaterialRequestSupplierAssignment", "SupplyManagerMaterialRequestDetail"),
		webapp.WithBinder(webapp.PathValueBinder[int]("id")),
	)

	api.POST(
		"/supply-manager/material-request-supplier-assignments",
		webapp.WithName("supplyManager.materialRequestSupplierAssignment.create"),
		webapp.WithPermission(supplyManagerAssignmentCreatePermission),
		webapp.WithService("MaterialRequestSupplierAssignment", "SupplyManagerCreate"),
		webapp.WithBinder(supplyManagerCreateAssignmentBinder()),
		webapp.WithSuccessCode(http.StatusCreated),
	)

	api.GET(
		"/supply-manager/material-request-supplier-assignments",
		webapp.WithName("supplyManager.materialRequestSupplierAssignment.list"),
		webapp.WithPermission(supplyManagerAssignmentListPermission),
		webapp.WithService("MaterialRequestSupplierAssignment", "SupplyManagerHistory"),
		webapp.WithBinder(supplyManagerAssignmentHistoryBinder()),
	)

	api.GET(
		"/supply-manager/material-request-supplier-assignments/{id}",
		webapp.WithName("supplyManager.materialRequestSupplierAssignment.detail"),
		webapp.WithPermission(supplyManagerAssignmentDetailPermission),
		webapp.WithService("MaterialRequestSupplierAssignment", "SupplyManagerDetail"),
		webapp.WithBinder(webapp.PathValueBinder[int]("id")),
	)
}

func supplyManagerMaterialRequestBinder() webapp.Binder {
	return func(r *http.Request) (any, error) {
		values := r.URL.Query()
		dateFrom, err := optionalSupplyManagerTimestamp(values.Get("date_from"), "date_from")
		if err != nil {
			return nil, err
		}
		dateTo, err := optionalSupplyManagerTimestamp(values.Get("date_to"), "date_to")
		if err != nil {
			return nil, err
		}

		modelValues := r.URL.Query()
		modelValues.Del("date_from")
		modelValues.Del("date_to")
		input, err := modelbind.DecodeURLValuesInput[*models.SupplyManagerMaterialRequestQuery](modelValues)
		if err != nil {
			return nil, err
		}
		input.Model.DateFrom = dateFrom
		input.Model.DateTo = dateTo
		if err := input.Validate(true); err != nil {
			return nil, err
		}
		if input.Model.ConstructionSiteID != nil && *input.Model.ConstructionSiteID <= 0 {
			return nil, webapp.BadRequest("construction_site_id should be positive", nil)
		}
		if input.Model.OrderImportanceID != nil && *input.Model.OrderImportanceID <= 0 {
			return nil, webapp.BadRequest("order_importance_id should be positive", nil)
		}
		if dateFrom != nil && dateTo != nil && dateFrom.After(*dateTo) {
			return nil, webapp.BadRequest("date_from should not be after date_to", nil)
		}
		if input.Model.MaterialSearch != nil {
			trimmed := strings.TrimSpace(*input.Model.MaterialSearch)
			input.Model.MaterialSearch = &trimmed
		}

		params, err := webapp.ParseCollectionParams(r)
		if err != nil {
			return nil, err
		}

		return models.SupplyManagerMaterialRequestInput{
			Query:  input.Model,
			Params: params,
		}, nil
	}
}

func supplyManagerCreateAssignmentBinder() webapp.Binder {
	return documentJSONBinder[models.SupplyManagerCreateAssignmentRequest]()
}

func supplyManagerAssignmentHistoryBinder() webapp.Binder {
	return func(r *http.Request) (any, error) {
		input, err := modelbind.DecodeURLValuesInput[*models.SupplyManagerAssignmentHistoryQuery](r.URL.Query())
		if err != nil {
			return nil, err
		}
		if err := input.Validate(true); err != nil {
			return nil, err
		}
		if input.Model.ConstructionSiteID != nil && *input.Model.ConstructionSiteID <= 0 {
			return nil, webapp.BadRequest("construction_site_id should be positive", nil)
		}

		params, err := webapp.ParseCollectionParams(r)
		if err != nil {
			return nil, err
		}

		return models.SupplyManagerAssignmentHistoryInput{
			Query:  input.Model,
			Params: params,
		}, nil
	}
}

func optionalSupplyManagerTimestamp(value string, field string) (*time.Time, error) {
	if value == "" {
		return nil, nil
	}

	result, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return nil, webapp.BadRequest(field+" should be an RFC 3339 timestamp", err)
	}
	return &result, nil
}

func removeGeneratedSupplyManagerAssignmentRoutes(routes []webapp.Route) []webapp.Route {
	filtered := routes[:0]
	for _, route := range routes {
		if strings.HasPrefix(route.Name, "materialRequestSupplierAssignment.") ||
			strings.HasPrefix(route.Pattern, "/api/material-request-supplier-assignments") {
			continue
		}
		filtered = append(filtered, route)
	}

	return filtered
}
