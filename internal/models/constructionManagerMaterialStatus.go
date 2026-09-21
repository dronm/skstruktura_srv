package models

import (
	"time"

	"github.com/dronm/modelbind"
)

type ConstructionManagerMaterialStatusQuery struct {
	ConstructionSiteID int  `json:"construction_site_id" required:"true"`
	MaterialTypeID     *int `json:"material_type_id"`
}

type ConstructionManagerMaterialStatusInput struct {
	Query  *ConstructionManagerMaterialStatusQuery
	Params modelbind.CollectionParams
}

type ConstructionManagerMaterialStatusCurrentRow struct {
	MaterialID       int                `json:"material_id"`
	MaterialName     string             `json:"material_name"`
	MaterialTypeID   int                `json:"material_type_id"`
	MaterialTypeName string             `json:"material_type_name"`
	Status           MaterialStatusType `json:"status"`
	StatusRecordID   *int               `json:"status_record_id"`
	StatusChangedAt  *time.Time         `json:"status_changed_at"`
}

type ConstructionManagerMaterialStatusHistoryRow struct {
	ID                 int                `json:"id"`
	CreatedAt          time.Time          `json:"created_at"`
	ConstructionSiteID int                `json:"construction_site_id"`
	MaterialID         int                `json:"material_id"`
	MaterialName       string             `json:"material_name"`
	MaterialTypeID     int                `json:"material_type_id"`
	MaterialTypeName   string             `json:"material_type_name"`
	Status             MaterialStatusType `json:"status"`
}

type ConstructionManagerMaterialStatusChangeRequest struct {
	ConstructionSiteID     int                `json:"construction_site_id" required:"true"`
	MaterialID             int                `json:"material_id" required:"true"`
	CreatedAt              time.Time          `json:"created_at" required:"true"`
	ExpectedStatusRecordID *int               `json:"expected_status_record_id"`
	ExpectedStatus         MaterialStatusType `json:"expected_status" required:"true" enum:"material_status_type"`
	TargetStatus           MaterialStatusType `json:"target_status" required:"true" enum:"material_status_type"`
}
