package services

import (
	"errors"
	"testing"
	"time"

	"github.com/dronm/skstruktura/internal/models"
	"github.com/dronm/webapp"
)

func TestValidateSupplyManagerCreateAssignmentRequest(t *testing.T) {
	t.Parallel()

	comment := "  supplier batch  "
	request := &models.SupplyManagerCreateAssignmentRequest{
		Date:    time.Date(2026, time.September, 22, 8, 30, 0, 0, time.UTC),
		Comment: &comment,
		Requests: []*models.SupplyManagerAssignmentRequestRef{
			{ID: 12, Version: 4},
			{ID: 7, Version: 2},
		},
		Items: []*models.SupplyManagerAssignmentItemInput{
			{MaterialRequestItemID: 101, SupplierID: 31},
			{MaterialRequestItemID: 102, SupplierID: 32},
		},
	}

	validated, err := validateSupplyManagerCreateAssignmentRequest(request)
	if err != nil {
		t.Fatalf("validateSupplyManagerCreateAssignmentRequest() error = %v", err)
	}
	if len(validated.requestIDs) != 2 || validated.requestIDs[0] != 7 || validated.requestIDs[1] != 12 {
		t.Fatalf("request ids = %#v, want [7 12]", validated.requestIDs)
	}
	if validated.requestVersions[12] != 4 || validated.requestVersions[7] != 2 {
		t.Fatalf("request versions = %#v", validated.requestVersions)
	}
	if len(validated.itemIDs) != 2 || validated.itemIDs[0] != 101 || validated.supplierIDs[1] != 32 {
		t.Fatalf("validated items = %#v / %#v", validated.itemIDs, validated.supplierIDs)
	}
	if request.Comment == nil || *request.Comment != "supplier batch" {
		t.Fatalf("trimmed comment = %#v", request.Comment)
	}
}

func TestValidateSupplyManagerCreateAssignmentRequestRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	valid := func() *models.SupplyManagerCreateAssignmentRequest {
		return &models.SupplyManagerCreateAssignmentRequest{
			Date: time.Date(2026, time.September, 22, 8, 30, 0, 0, time.UTC),
			Requests: []*models.SupplyManagerAssignmentRequestRef{
				{ID: 7, Version: 2},
			},
			Items: []*models.SupplyManagerAssignmentItemInput{
				{MaterialRequestItemID: 101, SupplierID: 31},
			},
		}
	}

	tests := []struct {
		name    string
		request func() *models.SupplyManagerCreateAssignmentRequest
	}{
		{name: "nil request", request: func() *models.SupplyManagerCreateAssignmentRequest { return nil }},
		{name: "missing date", request: func() *models.SupplyManagerCreateAssignmentRequest {
			request := valid()
			request.Date = time.Time{}
			return request
		}},
		{name: "missing requests", request: func() *models.SupplyManagerCreateAssignmentRequest {
			request := valid()
			request.Requests = nil
			return request
		}},
		{name: "duplicate request", request: func() *models.SupplyManagerCreateAssignmentRequest {
			request := valid()
			request.Requests = append(request.Requests, &models.SupplyManagerAssignmentRequestRef{ID: 7, Version: 2})
			return request
		}},
		{name: "missing items", request: func() *models.SupplyManagerCreateAssignmentRequest {
			request := valid()
			request.Items = nil
			return request
		}},
		{name: "duplicate item", request: func() *models.SupplyManagerCreateAssignmentRequest {
			request := valid()
			request.Items = append(request.Items, &models.SupplyManagerAssignmentItemInput{
				MaterialRequestItemID: 101,
				SupplierID:            32,
			})
			return request
		}},
		{name: "invalid supplier", request: func() *models.SupplyManagerCreateAssignmentRequest {
			request := valid()
			request.Items[0].SupplierID = 0
			return request
		}},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := validateSupplyManagerCreateAssignmentRequest(test.request())
			var appErr *webapp.AppError
			if !errors.As(err, &appErr) || appErr.StatusCode() != 400 {
				t.Fatalf("error = %v, want HTTP 400", err)
			}
		})
	}
}
