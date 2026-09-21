package models

import (
	"time"

	"github.com/dronm/modelbind"
)

type DiadocDocumentListQuery struct {
	Status             string
	Search             string
	DateFrom           *time.Time
	DateTo             *time.Time
	SupplierID         *int
	ConstructionSiteID *int
}

type DiadocDocumentListInput struct {
	Query  DiadocDocumentListQuery
	Params modelbind.CollectionParams
}

type DiadocDocumentListRow struct {
	ID                 int64      `json:"id"`
	Version            int64      `json:"version"`
	Status             string     `json:"status"`
	DocumentNumber     string     `json:"document_number"`
	DocumentDate       *time.Time `json:"document_date,omitempty"`
	SenderName         string     `json:"sender_name"`
	SenderINN          string     `json:"sender_inn"`
	SenderKPP          string     `json:"sender_kpp"`
	SupplierID         *int       `json:"supplier_id,omitempty"`
	Supplier           *Ref       `json:"supplier,omitempty"`
	ConstructionSiteID *int       `json:"construction_site_id,omitempty"`
	ConstructionSite   *Ref       `json:"construction_site,omitempty"`
	ReceiptNumber      string     `json:"receipt_number"`
	ReceiptDate        *time.Time `json:"receipt_date,omitempty"`
	LineCount          int        `json:"line_count"`
	AmountWithoutVAT   string     `json:"amount_without_vat"`
	VATAmount          string     `json:"vat_amount"`
	AmountWithVAT      string     `json:"amount_with_vat"`
	MissingCount       int        `json:"missing_count"`
	LastError          string     `json:"last_error,omitempty"`
	MaterialReceiptID  *int       `json:"material_receipt_id,omitempty"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

type DiadocDocumentListResponse struct {
	Rows  []*DiadocDocumentListRow `json:"rows"`
	Total int64                    `json:"total"`
}

type DiadocReadinessIssue struct {
	Code   string `json:"code"`
	Field  string `json:"field"`
	ItemID *int64 `json:"item_id,omitempty"`
}

type DiadocReadiness struct {
	Ready    bool                    `json:"ready"`
	Missing  []*DiadocReadinessIssue `json:"missing"`
	Warnings []string                `json:"warnings"`
}

type DiadocDocumentItem struct {
	ConstructionSiteID     *int     `json:"construction_site_id,omitempty"`
	ConstructionSite       *Ref     `json:"construction_site,omitempty"`
	ID                     int64    `json:"id"`
	LineNum                int      `json:"line_num"`
	SourceProductCode      string   `json:"source_product_code,omitempty"`
	SourceArticle          string   `json:"source_article,omitempty"`
	SourceGTIN             string   `json:"source_gtin,omitempty"`
	SourceName             string   `json:"source_name"`
	SourceOKEI             string   `json:"source_okei_code,omitempty"`
	SourceUnitName         string   `json:"source_unit_name,omitempty"`
	SourceQuant            string   `json:"source_quant"`
	SourcePrice            string   `json:"source_price"`
	SourceAmountWithoutVAT string   `json:"source_amount_without_vat"`
	SourceVATPercent       string   `json:"source_vat_percent"`
	SourceVATAmount        string   `json:"source_vat_amount"`
	SourceAmountWithVAT    string   `json:"source_amount_with_vat"`
	MaterialID             *int     `json:"material_id,omitempty"`
	Material               *Ref     `json:"material,omitempty"`
	MeasureUnitID          *int     `json:"measure_unit_id,omitempty"`
	MeasureUnit            *Ref     `json:"measure_unit,omitempty"`
	ConversionFactor       *string  `json:"conversion_factor,omitempty"`
	ImportQuant            *string  `json:"import_quant,omitempty"`
	ImportPrice            *string  `json:"import_price,omitempty"`
	ImportAmount           *string  `json:"import_amount,omitempty"`
	ImportVATPercent       *string  `json:"import_vat_percent,omitempty"`
	ImportVATAmount        *string  `json:"import_vat_amount,omitempty"`
	IsExcluded             bool     `json:"is_excluded"`
	MappingSource          string   `json:"mapping_source,omitempty"`
	LastError              string   `json:"last_error,omitempty"`
	Issues                 []string `json:"issues"`
}

type DiadocTotals struct {
	LineCount        int    `json:"line_count"`
	AmountWithoutVAT string `json:"amount_without_vat"`
	VATAmount        string `json:"vat_amount"`
	AmountWithVAT    string `json:"amount_with_vat"`
}

type DiadocDocumentTotals struct {
	Document DiadocTotals `json:"document"`
	Import   DiadocTotals `json:"import"`
	Excluded DiadocTotals `json:"excluded"`
}

type DiadocDocumentDetail struct {
	ID                 int64                 `json:"id"`
	Version            int64                 `json:"version"`
	Status             string                `json:"status"`
	MessageID          string                `json:"message_id"`
	EntityID           string                `json:"entity_id"`
	DocumentNumber     string                `json:"document_number"`
	DocumentDate       *time.Time            `json:"document_date,omitempty"`
	DocumentFunction   string                `json:"document_function,omitempty"`
	DocumentVersion    string                `json:"document_version,omitempty"`
	SenderBoxID        string                `json:"sender_box_id,omitempty"`
	SenderName         string                `json:"sender_name"`
	SenderINN          string                `json:"sender_inn"`
	SenderKPP          string                `json:"sender_kpp"`
	SupplierID         *int                  `json:"supplier_id,omitempty"`
	Supplier           *Ref                  `json:"supplier,omitempty"`
	ConstructionSiteID *int                  `json:"construction_site_id,omitempty"`
	ConstructionSite   *Ref                  `json:"construction_site,omitempty"`
	ReceiptNumber      string                `json:"receipt_number"`
	ReceiptDate        *time.Time            `json:"receipt_date,omitempty"`
	ReceiptComment     string                `json:"receipt_comment"`
	AmountWithoutVAT   string                `json:"amount_without_vat"`
	VATAmount          string                `json:"vat_amount"`
	AmountWithVAT      string                `json:"amount_with_vat"`
	Totals             DiadocDocumentTotals  `json:"totals"`
	MaterialReceiptID  *int                  `json:"material_receipt_id,omitempty"`
	IgnoredReason      string                `json:"ignored_reason,omitempty"`
	IgnoredAt          *time.Time            `json:"ignored_at,omitempty"`
	IgnoredBy          string                `json:"ignored_by,omitempty"`
	ImportedAt         *time.Time            `json:"imported_at,omitempty"`
	ImportedBy         string                `json:"imported_by,omitempty"`
	LastError          string                `json:"last_error,omitempty"`
	Readiness          DiadocReadiness       `json:"readiness"`
	Items              []*DiadocDocumentItem `json:"items"`
	CreatedAt          time.Time             `json:"created_at"`
	UpdatedAt          time.Time             `json:"updated_at"`
}

type DiadocResolutionItem struct {
	ConstructionSiteID    *int   `json:"construction_site_id"`
	ID                    int64  `json:"id"`
	MaterialID            int    `json:"material_id"`
	ConversionFactor      string `json:"conversion_factor"`
	RememberMaterialMatch bool   `json:"remember_material_match"`
}

type DiadocResolutionRequest struct {
	Version               int64                   `json:"version"`
	SupplierID            int                     `json:"supplier_id"`
	ConstructionSiteID    *int                    `json:"construction_site_id"`
	ReceiptNumber         string                  `json:"receipt_number"`
	ReceiptDate           *time.Time              `json:"receipt_date"`
	ReceiptComment        string                  `json:"receipt_comment"`
	RememberSupplierMatch bool                    `json:"remember_supplier_match"`
	Items                 []*DiadocResolutionItem `json:"items"`
}

type DiadocResolutionInput struct {
	ID      int64
	Request *DiadocResolutionRequest
}

type DiadocVersionRequest struct {
	Version int64 `json:"version"`
}

type DiadocVersionInput struct {
	ID      int64
	Request *DiadocVersionRequest
}

type DiadocItemVersionInput struct {
	ID      int64
	ItemID  int64
	Request *DiadocVersionRequest
}

type DiadocIgnoreRequest struct {
	Version int64  `json:"version"`
	Reason  string `json:"reason"`
}

type DiadocIgnoreInput struct {
	ID      int64
	Request *DiadocIgnoreRequest
}

type DiadocImportResponse struct {
	DocumentID        int64  `json:"document_id"`
	MaterialReceiptID int    `json:"material_receipt_id"`
	Status            string `json:"status"`
}

type DiadocState struct {
	Configured         bool             `json:"configured"`
	Authorized         bool             `json:"authorized"`
	BoxID              string           `json:"box_id,omitempty"`
	Enabled            bool             `json:"enabled"`
	EventTimestampFrom time.Time        `json:"event_timestamp_from"`
	HasCursor          bool             `json:"has_cursor"`
	LastSyncStartedAt  *time.Time       `json:"last_sync_started_at,omitempty"`
	LastSyncFinishedAt *time.Time       `json:"last_sync_finished_at,omitempty"`
	LastSyncError      string           `json:"last_sync_error,omitempty"`
	Version            int64            `json:"version"`
	BufferCounts       map[string]int64 `json:"buffer_counts"`
}

type DiadocStateUpdateRequest struct {
	Version int64 `json:"version"`
	Enabled bool  `json:"enabled"`
}

type DiadocReplayRequest struct {
	Version            int64     `json:"version"`
	EventTimestampFrom time.Time `json:"event_timestamp_from"`
	RestoreIgnored     bool      `json:"restore_ignored"`
	RetryFailed        bool      `json:"retry_failed"`
}
