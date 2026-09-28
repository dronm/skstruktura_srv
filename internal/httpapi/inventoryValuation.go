package httpapi

import (
	"net/http"

	"github.com/dronm/skstruktura/internal/models"
	"github.com/dronm/webapp"
)

func inventoryValuationRoutes(api *webapp.Group) {
	api.GET(
		"/inventory-valuation",
		webapp.WithName("inventoryValuation.state"),
		webapp.WithPermission("inventoryValuation.view"),
		webapp.WithService("InventoryValuation", "State"),
	)

	api.PUT(
		"/inventory-valuation/settings",
		webapp.WithName("inventoryValuation.settings"),
		webapp.WithPermission("inventoryValuation.manage"),
		webapp.WithService("InventoryValuation", "UpdateSettings"),
		webapp.WithBinder(documentJSONBinder[models.InventoryValuationSettingsUpdate]()),
	)

	api.POST(
		"/inventory-valuation/recalculate",
		webapp.WithName("inventoryValuation.recalculate"),
		webapp.WithPermission("inventoryValuation.manage"),
		webapp.WithService("InventoryValuation", "Recalculate"),
	)

	api.POST(
		"/inventory-valuation/close",
		webapp.WithName("inventoryValuation.close"),
		webapp.WithPermission("inventoryValuation.manage"),
		webapp.WithService("InventoryValuation", "Close"),
		webapp.WithBinder(documentJSONBinder[models.InventoryValuationCloseRequest]()),
	)

	api.POST(
		"/inventory-valuation/reopen",
		webapp.WithName("inventoryValuation.reopen"),
		webapp.WithPermission("inventoryValuation.manage"),
		webapp.WithService("InventoryValuation", "Reopen"),
		webapp.WithSuccessCode(http.StatusOK),
	)
}
