package httpapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/dronm/skstruktura/internal/models"
	"github.com/dronm/webapp"
)

const materialDocumentMaxBodySize = 2 << 20

func materialDocumentRoutes(api *webapp.Group) {
	api.POST(
		"/material-receipts",
		webapp.WithName("materialReceipt.create"),
		webapp.WithPermission("materialReceipt.create"),
		webapp.WithService("MaterialReceipt", "Create"),
		webapp.WithBinder(documentJSONBinder[models.MaterialReceiptDocument]()),
		webapp.WithSuccessCode(http.StatusCreated),
	)
	api.GET(
		"/material-receipts/{id}",
		webapp.WithName("materialReceipt.detail"),
		webapp.WithPermission("materialReceipt.detail"),
		webapp.WithService("MaterialReceipt", "DocumentDetail"),
		webapp.WithBinder(webapp.PathValueBinder[int]("id")),
	)
	api.PUT(
		"/material-receipts/{id}",
		webapp.WithName("materialReceipt.update"),
		webapp.WithPermission("materialReceipt.update"),
		webapp.WithService("MaterialReceipt", "Update"),
		webapp.WithBinder(materialReceiptDocumentUpdateBinder()),
	)

	api.POST(
		"/material-consumptions",
		webapp.WithName("materialConsumption.create"),
		webapp.WithPermission("materialConsumption.create"),
		webapp.WithService("MaterialConsumption", "Create"),
		webapp.WithBinder(documentJSONBinder[models.MaterialConsumptionDocument]()),
		webapp.WithSuccessCode(http.StatusCreated),
	)
	api.GET(
		"/material-consumptions/{id}",
		webapp.WithName("materialConsumption.detail"),
		webapp.WithPermission("materialConsumption.detail"),
		webapp.WithService("MaterialConsumption", "DocumentDetail"),
		webapp.WithBinder(webapp.PathValueBinder[int]("id")),
	)
	api.PUT(
		"/material-consumptions/{id}",
		webapp.WithName("materialConsumption.update"),
		webapp.WithPermission("materialConsumption.update"),
		webapp.WithService("MaterialConsumption", "Update"),
		webapp.WithBinder(materialConsumptionDocumentUpdateBinder()),
	)

	api.POST(
		"/material-transfers",
		webapp.WithName("materialTransfer.create"),
		webapp.WithPermission("materialTransfer.create"),
		webapp.WithService("MaterialTransfer", "Create"),
		webapp.WithBinder(documentJSONBinder[models.MaterialTransferDocument]()),
		webapp.WithSuccessCode(http.StatusCreated),
	)
	api.GET(
		"/material-transfers/{id}",
		webapp.WithName("materialTransfer.detail"),
		webapp.WithPermission("materialTransfer.detail"),
		webapp.WithService("MaterialTransfer", "DocumentDetail"),
		webapp.WithBinder(webapp.PathValueBinder[int]("id")),
	)
	api.PUT(
		"/material-transfers/{id}",
		webapp.WithName("materialTransfer.update"),
		webapp.WithPermission("materialTransfer.update"),
		webapp.WithService("MaterialTransfer", "Update"),
		webapp.WithBinder(materialTransferDocumentUpdateBinder()),
	)
}

func removeReplacedMaterialDocumentRoutes(routes []webapp.Route) []webapp.Route {
	replaced := map[string]struct{}{
		"materialReceipt.create":     {},
		"materialReceipt.detail":     {},
		"materialReceipt.update":     {},
		"materialConsumption.create": {},
		"materialConsumption.detail": {},
		"materialConsumption.update": {},
		"materialTransfer.create":    {},
		"materialTransfer.detail":    {},
		"materialTransfer.update":    {},
	}

	filtered := routes[:0]
	for _, route := range routes {
		if _, ok := replaced[route.Name]; ok {
			continue
		}
		if strings.HasPrefix(route.Name, "materialReceiptItem.") ||
			strings.HasPrefix(route.Name, "materialConsumptionItem.") ||
			strings.HasPrefix(route.Name, "materialTransferItem.") ||
			strings.HasPrefix(route.Pattern, "/api/material-receipt-items") ||
			strings.HasPrefix(route.Pattern, "/api/material-consumption-items") ||
			strings.HasPrefix(route.Pattern, "/api/material-transfer-items") {
			continue
		}
		filtered = append(filtered, route)
	}

	return filtered
}

func documentJSONBinder[T any]() webapp.Binder {
	return func(r *http.Request) (any, error) {
		return decodeDocumentJSON[T](r)
	}
}

func materialReceiptDocumentUpdateBinder() webapp.Binder {
	return func(r *http.Request) (any, error) {
		id, err := positiveDocumentPathID(r)
		if err != nil {
			return nil, err
		}
		document, err := decodeDocumentJSON[models.MaterialReceiptDocument](r)
		if err != nil {
			return nil, err
		}
		return models.UpdateMaterialReceiptDocumentRequest{ID: id, Document: document}, nil
	}
}

func materialConsumptionDocumentUpdateBinder() webapp.Binder {
	return func(r *http.Request) (any, error) {
		id, err := positiveDocumentPathID(r)
		if err != nil {
			return nil, err
		}
		document, err := decodeDocumentJSON[models.MaterialConsumptionDocument](r)
		if err != nil {
			return nil, err
		}
		return models.UpdateMaterialConsumptionDocumentRequest{ID: id, Document: document}, nil
	}
}

func materialTransferDocumentUpdateBinder() webapp.Binder {
	return func(r *http.Request) (any, error) {
		id, err := positiveDocumentPathID(r)
		if err != nil {
			return nil, err
		}
		document, err := decodeDocumentJSON[models.MaterialTransferDocument](r)
		if err != nil {
			return nil, err
		}
		return models.UpdateMaterialTransferDocumentRequest{ID: id, Document: document}, nil
	}
}

func decodeDocumentJSON[T any](r *http.Request) (*T, error) {
	if r == nil || r.Body == nil {
		return nil, webapp.BadRequest("document request body is required", nil)
	}

	data, err := io.ReadAll(io.LimitReader(r.Body, materialDocumentMaxBodySize+1))
	if err != nil {
		return nil, webapp.BadRequest("read document request body", nil)
	}
	if len(data) == 0 {
		return nil, webapp.BadRequest("document request body is required", nil)
	}
	if len(data) > materialDocumentMaxBodySize {
		return nil, webapp.BadRequest("document request body is too large", nil)
	}

	var document T
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&document); err != nil {
		return nil, webapp.BadRequest(
			"invalid document JSON",
			map[string]any{"error": err.Error()},
		)
	}
	if err := ensureJSONEnd(decoder); err != nil {
		return nil, webapp.BadRequest("invalid document JSON", map[string]any{"error": err.Error()})
	}

	return &document, nil
}

func ensureJSONEnd(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); err == io.EOF {
		return nil
	} else if err != nil {
		return err
	}
	return fmt.Errorf("request body contains more than one JSON value")
}

func positiveDocumentPathID(r *http.Request) (int, error) {
	value := r.PathValue("id")
	id, err := strconv.Atoi(value)
	if err != nil || id <= 0 {
		return 0, webapp.BadRequest("document id should be a positive integer", map[string]any{"id": value})
	}
	return id, nil
}
