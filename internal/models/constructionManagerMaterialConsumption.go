package models

import "github.com/dronm/modelbind"

type ConstructionManagerMaterialConsumptionQuery struct {
	ConstructionSiteID int `json:"construction_site_id" required:"true"`
}

type ConstructionManagerMaterialConsumptionInput struct {
	Query  *ConstructionManagerMaterialConsumptionQuery
	Params modelbind.CollectionParams
}
