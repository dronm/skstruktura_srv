package models

import "time"

// MaterialReceiptDocument is the complete material receipt aggregate exchanged
// with the API. Item parent ids are derived by the service and are deliberately
// not part of the transport contract.
type MaterialReceiptDocument struct {
	ID                 int                            `json:"id"`
	Version            int64                          `json:"version"`
	Date               time.Time                      `json:"date"`
	ConstructionSiteID *int                           `json:"construction_site_id"`
	SupplierID         int                            `json:"supplier_id"`
	Number             string                         `json:"number"`
	Comment            *string                        `json:"comment"`
	Items              []*MaterialReceiptDocumentItem `json:"items"`
	ConstructionSite   *Ref                           `json:"construction_site"`
	Supplier           *Ref                           `json:"supplier"`
}

type MaterialReceiptDocumentItem struct {
	ID                 int     `json:"id"`
	LineNum            int     `json:"line_num"`
	MaterialID         int     `json:"material_id"`
	MeasureUnitID      int     `json:"measure_unit_id"`
	Quant              float64 `json:"quant"`
	Price              float64 `json:"price"`
	Amount             float64 `json:"amount"`
	VatPercent         float64 `json:"vat_percent"`
	VatAmount          float64 `json:"vat_amount"`
	Material           *Ref    `json:"material"`
	MeasureUnit        *Ref    `json:"measure_unit"`
	ConstructionSiteID *int    `json:"construction_site_id"`
	ConstructionSite   *Ref    `json:"construction_site"`
}

type UpdateMaterialReceiptDocumentRequest struct {
	ID       int
	Document *MaterialReceiptDocument
}

// MaterialConsumptionDocument is the complete material consumption aggregate.
type MaterialConsumptionDocument struct {
	ID                 int                                `json:"id"`
	Version            int64                              `json:"version"`
	Date               time.Time                          `json:"date"`
	ConstructionSiteID int                                `json:"construction_site_id"`
	Comment            *string                            `json:"comment"`
	Items              []*MaterialConsumptionDocumentItem `json:"items"`
	ConstructionSite   *Ref                               `json:"construction_site"`
}

type MaterialConsumptionDocumentItem struct {
	ID            int     `json:"id"`
	LineNum       int     `json:"line_num"`
	MaterialID    int     `json:"material_id"`
	MeasureUnitID int     `json:"measure_unit_id"`
	Quant         float64 `json:"quant"`
	Material      *Ref    `json:"material"`
	MeasureUnit   *Ref    `json:"measure_unit"`
}

type UpdateMaterialConsumptionDocumentRequest struct {
	ID       int
	Document *MaterialConsumptionDocument
}

// MaterialTransferDocument is the complete material transfer aggregate.
type MaterialTransferDocument struct {
	ID                            int                             `json:"id"`
	Version                       int64                           `json:"version"`
	Date                          time.Time                       `json:"date"`
	SourceConstructionSiteID      int                             `json:"source_construction_site_id"`
	DestinationConstructionSiteID int                             `json:"destination_construction_site_id"`
	Comment                       *string                         `json:"comment"`
	Items                         []*MaterialTransferDocumentItem `json:"items"`
	SourceConstructionSite        *Ref                            `json:"source_construction_site"`
	DestinationConstructionSite   *Ref                            `json:"destination_construction_site"`
}

type MaterialTransferDocumentItem struct {
	ID            int     `json:"id"`
	LineNum       int     `json:"line_num"`
	MaterialID    int     `json:"material_id"`
	MeasureUnitID int     `json:"measure_unit_id"`
	Quant         float64 `json:"quant"`
	Material      *Ref    `json:"material"`
	MeasureUnit   *Ref    `json:"measure_unit"`
}

type UpdateMaterialTransferDocumentRequest struct {
	ID       int
	Document *MaterialTransferDocument
}

const (
	MaterialRequestStatusCodeDraft      = "draft"
	MaterialRequestStatusCodeSubmitted  = "submitted"
	MaterialRequestStatusCodeInProgress = "in_progress"
	MaterialRequestStatusCodeCompleted  = "completed"
	MaterialRequestStatusCodeCancelled  = "cancelled"
)

// MaterialRequestDocument is a request from a construction-site manager for
// materials. Unlike stock movement documents, saving this aggregate does not
// post anything to the material register.
type MaterialRequestDocument struct {
	ID                    int                            `json:"id"`
	Version               int64                          `json:"version"`
	Date                  time.Time                      `json:"date"`
	ConstructionSiteID    int                            `json:"construction_site_id"`
	ConstructionManagerID int                            `json:"construction_manager_id"`
	Comment               *string                        `json:"comment"`
	Items                 []*MaterialRequestDocumentItem `json:"items"`
	ConstructionSite      *Ref                           `json:"construction_site"`
	ConstructionManager   *Ref                           `json:"construction_manager"`
}

type MaterialRequestDocumentItem struct {
	ID                int       `json:"id"`
	LineNum           int       `json:"line_num"`
	MaterialID        int       `json:"material_id"`
	MeasureUnitID     int       `json:"measure_unit_id"`
	Quant             float64   `json:"quant"`
	SupplierID        *int      `json:"supplier_id"`
	RequiredDate      *DateOnly `json:"required_date"`
	OrderImportanceID int       `json:"order_importance_id"`
	StatusID          int       `json:"status_id"`
	Material          *Ref      `json:"material"`
	MeasureUnit       *Ref      `json:"measure_unit"`
	Supplier          *Ref      `json:"supplier"`
	OrderImportance   *Ref      `json:"order_importance"`
	Status            *Ref      `json:"status"`
}

type UpdateMaterialRequestDocumentRequest struct {
	ID       int
	Document *MaterialRequestDocument
}

type MaterialRequestVersionRequest struct {
	Version int64 `json:"version"`
}

type SubmitMaterialRequestInput struct {
	ID      int
	Request *MaterialRequestVersionRequest
}
