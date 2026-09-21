package main

import (
	"github.com/dronm/skstruktura/internal/config"
	"github.com/dronm/skstruktura/internal/services"

	"github.com/dronm/webapp"
)

// registerServices registers all availvable services.
// Can add permChecker to a service if it needs some specific
// permission logic.
func registerServices(
	sessCfg config.SessionConfig,
	maxCfg config.MAXConfig,
	permChecker webapp.PermissionChecker,
	mainMenuCache services.MainMenuCache,
) {
	services.RegisterMainMenuService(mainMenuCache)
	services.RegisterApplicationRouteService(mainMenuCache)
	services.RegisterMaterialActionReportService()
	services.RegisterObjectHistoryService()

	services.RegisterGeneratedServices()

	services.RegisterProgAboutService()

	services.RegisterUserService(sessCfg, maxCfg)

}
