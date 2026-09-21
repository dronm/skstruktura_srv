package models

import (
	"time"

	"github.com/dronm/modelbind"
)

type MaterialActionReportLevel string

const (
	MaterialActionReportLevelConstructionSite MaterialActionReportLevel = "construction_site"
	MaterialActionReportLevelMaterial         MaterialActionReportLevel = "material"
	MaterialActionReportLevelDocument         MaterialActionReportLevel = "document"
)

type MaterialActionReportQuery struct {
	DateFrom                 time.Time                 `json:"date_from"`
	DateTo                   time.Time                 `json:"date_to"`
	ConstructionSiteIDs      []int                     `json:"construction_site_ids"`
	MaterialIDs              []int                     `json:"material_ids"`
	Level                    MaterialActionReportLevel `json:"level" required:"true"`
	ParentConstructionSiteID *int                      `json:"parent_construction_site_id"`
	ParentMaterialID         *int                      `json:"parent_material_id"`
}

type MaterialActionReportInput struct {
	Query  *MaterialActionReportQuery
	Params modelbind.CollectionParams
}

type MaterialActionReportRow struct {
	Key                string                    `json:"key"`
	RowType            MaterialActionReportLevel `json:"row_type"`
	ConstructionSiteID int                       `json:"construction_site_id"`
	ConstructionSite   *Ref                      `json:"construction_site"`
	MaterialID         *int                      `json:"material_id,omitempty"`
	Material           *Ref                      `json:"material,omitempty"`
	RecorderType       *string                   `json:"recorder_type,omitempty"`
	RecorderID         *int64                    `json:"recorder_id,omitempty"`
	DocumentDate       *string                   `json:"document_date,omitempty"`
	DocumentNumber     *string                   `json:"document_number,omitempty"`
	Caption            string                    `json:"caption"`
	BalanceStart       *float64                  `json:"balance_start"`
	Income             *float64                  `json:"income"`
	Outcome            *float64                  `json:"outcome"`
	BalanceEnd         *float64                  `json:"balance_end"`
	HasChildren        bool                      `json:"has_children"`
	ChildCount         int64                     `json:"child_count"`
}

type MaterialActionReportResponse struct {
	Rows        []*MaterialActionReportRow `json:"rows"`
	Total       int64                      `json:"total"`
	GeneratedAt time.Time                  `json:"generated_at"`
}
