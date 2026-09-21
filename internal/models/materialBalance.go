package models

import (
	"time"

	"github.com/dronm/modelbind"
)

type MaterialBalanceQuery struct {
	ConstructionSiteID int `json:"construction_site_id" required:"true"`
}

type MaterialBalanceInput struct {
	Query  *MaterialBalanceQuery
	Params modelbind.CollectionParams
}

type MaterialBalanceRow struct {
	ID                 int     `json:"id"`
	ConstructionSiteID int     `json:"construction_site_id"`
	ConstructionSite   *Ref    `json:"construction_site"`
	MaterialTypeID     int     `json:"material_type_id"`
	MaterialType       *Ref    `json:"material_type"`
	MaterialID         int     `json:"material_id"`
	Material           *Ref    `json:"material"`
	MeasureUnitID      int     `json:"measure_unit_id"`
	MeasureUnit        *Ref    `json:"measure_unit"`
	Balance            float64 `json:"balance"`
}

type MaterialBalanceResponse struct {
	Rows        []*MaterialBalanceRow `json:"rows"`
	Total       int64                 `json:"total"`
	GeneratedAt time.Time             `json:"generated_at"`
}

type MaterialBalanceConstructionSite struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type MaterialBalanceConstructionSitesResponse struct {
	Rows  []*MaterialBalanceConstructionSite `json:"rows"`
	Total int64                              `json:"total"`
}
