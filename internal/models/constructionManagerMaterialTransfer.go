package models

import "github.com/dronm/modelbind"

type ConstructionManagerMaterialTransferQuery struct {
	ConstructionSiteID int `json:"construction_site_id" required:"true"`
}

type ConstructionManagerMaterialTransferInput struct {
	Query  *ConstructionManagerMaterialTransferQuery
	Params modelbind.CollectionParams
}
