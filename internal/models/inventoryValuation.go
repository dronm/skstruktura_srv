package models

import "time"

type InventoryValuationSettings struct {
	AllowNegativeOpenPeriod bool       `json:"allow_negative_open_period"`
	ExcludeVATFromCost      bool       `json:"exclude_vat_from_cost"`
	ClosedThrough           *time.Time `json:"closed_through"`
	VATBasisEditable        bool       `json:"vat_basis_editable"`
}

type InventoryValuationClosure struct {
	ID            int64      `json:"id"`
	ClosedThrough time.Time  `json:"closed_through"`
	ClosedAt      time.Time  `json:"closed_at"`
	ClosedBy      *int       `json:"closed_by"`
	ReopenedAt    *time.Time `json:"reopened_at"`
	ReopenedBy    *int       `json:"reopened_by"`
}

type InventoryValuationState struct {
	Settings           InventoryValuationSettings   `json:"settings"`
	PendingCount       int64                        `json:"pending_count"`
	NegativeCount      int64                        `json:"negative_count"`
	CurrentAmount      *float64                     `json:"current_amount"`
	CanManage          bool                         `json:"can_manage"`
	Closures           []*InventoryValuationClosure `json:"closures"`
	LastRecalculatedAt time.Time                    `json:"last_recalculated_at"`
}

type InventoryValuationSettingsUpdate struct {
	AllowNegativeOpenPeriod *bool `json:"allow_negative_open_period"`
	ExcludeVATFromCost      *bool `json:"exclude_vat_from_cost"`
}

type InventoryValuationCloseRequest struct {
	ClosedThrough string `json:"closed_through"`
}
