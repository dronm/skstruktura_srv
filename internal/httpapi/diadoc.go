package httpapi

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/dronm/skstruktura/internal/integrations/diadoc"
	"github.com/dronm/skstruktura/internal/models"
	"github.com/dronm/webapp"
)

func diadocRoutes(api *webapp.Group, manager *diadoc.Manager) {
	api.GET(
		"/diadoc/auth/start",
		webapp.WithName("diadoc.auth.start"),
		webapp.WithPermission("diadoc.manage"),
		webapp.WithHandler(manager.AuthStartHandler),
	)
	api.GET(
		"/diadoc/auth/callback",
		webapp.WithName("diadoc.auth.callback"),
		webapp.WithHandler(manager.AuthCallbackHandler),
	)
	api.GET(
		"/diadoc/status",
		webapp.WithName("diadoc.status"),
		webapp.WithPermission("diadoc.manage"),
		webapp.WithHandler(manager.StatusHandler),
	)
	api.GET(
		"/diadoc/documents",
		webapp.WithName("diadocDocument.list"),
		webapp.WithPermission("diadocDocument.list"),
		webapp.WithService("DiadocImport", "List"),
		webapp.WithBinder(diadocDocumentListBinder()),
	)
	api.GET(
		"/diadoc/documents/{id}",
		webapp.WithName("diadocDocument.detail"),
		webapp.WithPermission("diadocDocument.detail"),
		webapp.WithService("DiadocImport", "Detail"),
		webapp.WithBinder(webapp.PathValueBinder[int64]("id")),
	)
	api.PUT(
		"/diadoc/documents/{id}/resolution",
		webapp.WithName("diadocDocument.resolve"),
		webapp.WithPermission("diadocDocument.resolve"),
		webapp.WithService("DiadocImport", "Resolve"),
		webapp.WithBinder(diadocResolutionBinder()),
	)
	api.POST(
		"/diadoc/documents/{id}/items/{itemId}/exclude",
		webapp.WithName("diadocDocument.itemExclude"),
		webapp.WithPermission("diadocDocument.resolve"),
		webapp.WithService("DiadocImport", "ExcludeItem"),
		webapp.WithBinder(diadocItemVersionBinder()),
	)
	api.POST(
		"/diadoc/documents/{id}/items/{itemId}/restore",
		webapp.WithName("diadocDocument.itemRestore"),
		webapp.WithPermission("diadocDocument.resolve"),
		webapp.WithService("DiadocImport", "RestoreItem"),
		webapp.WithBinder(diadocItemVersionBinder()),
	)
	api.DELETE(
		"/diadoc/documents/{id}",
		webapp.WithName("diadocDocument.ignore"),
		webapp.WithPermission("diadocDocument.ignore"),
		webapp.WithService("DiadocImport", "Ignore"),
		webapp.WithBinder(diadocIgnoreBinder()),
	)
	api.POST(
		"/diadoc/documents/{id}/restore",
		webapp.WithName("diadocDocument.restore"),
		webapp.WithPermission("diadocDocument.ignore"),
		webapp.WithService("DiadocImport", "Restore"),
		webapp.WithBinder(diadocVersionBinder()),
	)
	api.POST(
		"/diadoc/documents/{id}/retry",
		webapp.WithName("diadocDocument.retry"),
		webapp.WithPermission("diadocDocument.resolve"),
		webapp.WithService("DiadocImport", "Retry"),
		webapp.WithBinder(diadocVersionBinder()),
	)
	api.POST(
		"/diadoc/documents/{id}/import",
		webapp.WithName("diadocDocument.import"),
		webapp.WithPermission("diadocDocument.import"),
		webapp.WithService("DiadocImport", "Import"),
		webapp.WithBinder(diadocVersionBinder()),
		webapp.WithSuccessCode(http.StatusCreated),
	)
	api.GET(
		"/diadoc/state",
		webapp.WithName("diadocState.view"),
		webapp.WithPermission("diadocState.view"),
		webapp.WithService("DiadocImport", "State"),
	)
	api.PATCH(
		"/diadoc/state",
		webapp.WithName("diadocState.update"),
		webapp.WithPermission("diadocState.update"),
		webapp.WithService("DiadocImport", "UpdateState"),
		webapp.WithBinder(documentJSONBinder[models.DiadocStateUpdateRequest]()),
	)
	api.POST(
		"/diadoc/state/replay",
		webapp.WithName("diadocState.replay"),
		webapp.WithPermission("diadocState.update"),
		webapp.WithService("DiadocImport", "Replay"),
		webapp.WithBinder(documentJSONBinder[models.DiadocReplayRequest]()),
	)
	api.POST(
		"/diadoc/sync",
		webapp.WithName("diadoc.sync"),
		webapp.WithPermission("diadoc.sync"),
		webapp.WithHandler(manager.SyncHandler),
	)
}

func diadocDocumentListBinder() webapp.Binder {
	return func(r *http.Request) (any, error) {
		values := r.URL.Query()
		query := models.DiadocDocumentListQuery{
			Status: strings.TrimSpace(values.Get("status")),
			Search: strings.TrimSpace(values.Get("search")),
		}
		var err error
		if query.DateFrom, err = optionalDiadocDate(values.Get("date_from"), "date_from"); err != nil {
			return nil, err
		}
		if query.DateTo, err = optionalDiadocDate(values.Get("date_to"), "date_to"); err != nil {
			return nil, err
		}
		if query.SupplierID, err = optionalPositiveInt(values.Get("supplier_id"), "supplier_id"); err != nil {
			return nil, err
		}
		if query.ConstructionSiteID, err = optionalPositiveInt(values.Get("construction_site_id"), "construction_site_id"); err != nil {
			return nil, err
		}
		params, err := webapp.ParseCollectionParams(r)
		if err != nil {
			return nil, err
		}
		return models.DiadocDocumentListInput{Query: query, Params: params}, nil
	}
}

func diadocResolutionBinder() webapp.Binder {
	return func(r *http.Request) (any, error) {
		id, err := positiveDiadocPathID(r)
		if err != nil {
			return nil, err
		}
		request, err := decodeDocumentJSON[models.DiadocResolutionRequest](r)
		if err != nil {
			return nil, err
		}
		return models.DiadocResolutionInput{ID: id, Request: request}, nil
	}
}

func diadocIgnoreBinder() webapp.Binder {
	return func(r *http.Request) (any, error) {
		id, err := positiveDiadocPathID(r)
		if err != nil {
			return nil, err
		}
		request, err := decodeDocumentJSON[models.DiadocIgnoreRequest](r)
		if err != nil {
			return nil, err
		}
		return models.DiadocIgnoreInput{ID: id, Request: request}, nil
	}
}

func diadocVersionBinder() webapp.Binder {
	return func(r *http.Request) (any, error) {
		id, err := positiveDiadocPathID(r)
		if err != nil {
			return nil, err
		}
		request, err := decodeDocumentJSON[models.DiadocVersionRequest](r)
		if err != nil {
			return nil, err
		}
		return models.DiadocVersionInput{ID: id, Request: request}, nil
	}
}

func diadocItemVersionBinder() webapp.Binder {
	return func(r *http.Request) (any, error) {
		id, err := positiveDiadocPathID(r)
		if err != nil {
			return nil, err
		}
		itemIDValue := r.PathValue("itemId")
		itemID, err := strconv.ParseInt(itemIDValue, 10, 64)
		if err != nil || itemID <= 0 {
			return nil, webapp.BadRequest("Diadoc document item id should be a positive integer", map[string]any{"item_id": itemIDValue})
		}
		request, err := decodeDocumentJSON[models.DiadocVersionRequest](r)
		if err != nil {
			return nil, err
		}
		return models.DiadocItemVersionInput{ID: id, ItemID: itemID, Request: request}, nil
	}
}

func positiveDiadocPathID(r *http.Request) (int64, error) {
	value := r.PathValue("id")
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id <= 0 {
		return 0, webapp.BadRequest("Diadoc document id should be a positive integer", map[string]any{"id": value})
	}
	return id, nil
}

func optionalDiadocDate(value string, field string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	result, err := time.Parse("2006-01-02", value)
	if err != nil {
		return nil, webapp.BadRequest(field+" should use YYYY-MM-DD format", nil)
	}
	return &result, nil
}

func optionalPositiveInt(value string, field string) (*int, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	result, err := strconv.Atoi(value)
	if err != nil || result <= 0 {
		return nil, webapp.BadRequest(field+" should be a positive integer", nil)
	}
	return &result, nil
}
