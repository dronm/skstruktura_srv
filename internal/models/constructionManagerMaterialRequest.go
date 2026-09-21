package models

import "github.com/dronm/modelbind"

type ConstructionManagerMaterialRequestQuery struct {
	ConstructionSiteID int `json:"construction_site_id" required:"true"`
}

type ConstructionManagerMaterialRequestInput struct {
	Query  *ConstructionManagerMaterialRequestQuery
	Params modelbind.CollectionParams
}
