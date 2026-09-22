package models

import (
	"time"

	"github.com/dronm/modelbind"
)

// SupplyManagerMaterialRequestQuery contains the optional filters used by the
// supply-manager incoming request workspace.
type SupplyManagerMaterialRequestQuery struct {
	ConstructionSiteID *int       `json:"construction_site_id"`
	DateFrom           *time.Time `json:"date_from"`
	DateTo             *time.Time `json:"date_to"`
	MaterialSearch     *string    `json:"material_search"`
	OrderImportanceID  *int       `json:"order_importance_id"`
}

type SupplyManagerMaterialRequestInput struct {
	Query  *SupplyManagerMaterialRequestQuery
	Params modelbind.CollectionParams
}

type SupplyManagerSite struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type SupplyManagerSitesResponse struct {
	Rows  []*SupplyManagerSite `json:"rows"`
	Total int64                `json:"total"`
}

// SupplyManagerAssignmentRequestRef supplies the optimistic version expected
// for one complete material request included in an assignment submission.
type SupplyManagerAssignmentRequestRef struct {
	ID      int   `json:"id" required:"true"`
	Version int64 `json:"version" required:"true"`
}

// SupplyManagerAssignmentItemInput assigns exactly one supplier to one
// material request line. Quantities are not accepted here because request lines
// cannot be divided between suppliers.
type SupplyManagerAssignmentItemInput struct {
	MaterialRequestItemID int `json:"material_request_item_id" required:"true"`
	SupplierID            int `json:"supplier_id" required:"true"`
}

type SupplyManagerCreateAssignmentRequest struct {
	Date     time.Time                            `json:"date" required:"true"`
	Comment  *string                              `json:"comment"`
	Requests []*SupplyManagerAssignmentRequestRef `json:"requests" required:"true"`
	Items    []*SupplyManagerAssignmentItemInput  `json:"items" required:"true"`
}

type SupplyManagerAssignmentHistoryQuery struct {
	ConstructionSiteID *int `json:"construction_site_id"`
}

type SupplyManagerAssignmentHistoryInput struct {
	Query  *SupplyManagerAssignmentHistoryQuery
	Params modelbind.CollectionParams
}

// SupplyManagerAssignmentHistoryRow is one immutable assignment document with
// its lines nested for the workspace history collection.
type SupplyManagerAssignmentHistoryRow struct {
	ID              int                                   `json:"id"`
	Date            time.Time                             `json:"date"`
	SupplyManagerID int                                   `json:"supply_manager_id"`
	Comment         *string                               `json:"comment"`
	Version         int64                                 `json:"version"`
	SupplyManager   *Ref                                  `json:"supply_manager"`
	Items           []*SupplyManagerAssignmentHistoryItem `json:"items"`
}

type SupplyManagerAssignmentHistoryItem struct {
	ID                                  int       `json:"id"`
	LineNum                             int       `json:"line_num"`
	MaterialRequestSupplierAssignmentID int       `json:"material_request_supplier_assignment_id"`
	MaterialRequestItemID               int       `json:"material_request_item_id"`
	MaterialRequestID                   int       `json:"material_request_id"`
	RequestDate                         time.Time `json:"request_date"`
	ConstructionSiteID                  int       `json:"construction_site_id"`
	MaterialID                          int       `json:"material_id"`
	MeasureUnitID                       int       `json:"measure_unit_id"`
	Quant                               float64   `json:"quant"`
	SupplierID                          int       `json:"supplier_id"`
	RequiredDate                        *DateOnly `json:"required_date"`
	OrderImportanceID                   int       `json:"order_importance_id"`
	ConstructionSite                    *Ref      `json:"construction_site"`
	Material                            *Ref      `json:"material"`
	MeasureUnit                         *Ref      `json:"measure_unit"`
	Supplier                            *Ref      `json:"supplier"`
	OrderImportance                     *Ref      `json:"order_importance"`
}
