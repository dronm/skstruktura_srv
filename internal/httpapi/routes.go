// Package httpapi
package httpapi

import (
	"net/http"

	"github.com/dronm/skstruktura/internal/integrations/diadoc"
	"github.com/dronm/webapp"
)

func BuildRoutes(diadocManagers ...*diadoc.Manager) []webapp.Route {
	routes := make([]webapp.Route, 0)
	api := webapp.NewGroup("/api", &routes)

	api.GET(
		"/health",
		webapp.WithHandler(func(w http.ResponseWriter, r *http.Request) {
			_ = webapp.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
		}),
	)

	applicationRouteRoutes(&api)
	contactCustomRoutes(&api)

	registerGeneratedRoutes(&api)
	routes = removeReplacedMaterialDocumentRoutes(routes)
	routes = removeGeneratedSupplyManagerAssignmentRoutes(routes)
	materialDocumentRoutes(&api)

	mainMenuRoutes(&api)

	materialActionReportRoutes(&api)
	materialBalanceRoutes(&api)
	constructionManagerMaterialRequestRoutes(&api)
	constructionManagerMaterialConsumptionRoutes(&api)
	constructionManagerMaterialTransferRoutes(&api)
	constructionManagerMaterialStatusRoutes(&api)
	supplyManagerWorkspaceRoutes(&api)
	objectHistoryRoutes(&api)

	progAboutRoutes(&api)

	userRoutes(&api)
	if len(diadocManagers) > 0 && diadocManagers[0] != nil {
		diadocRoutes(&api, diadocManagers[0])
	}

	return routes
}
