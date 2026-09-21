package services

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/dronm/ds/v4"
	"github.com/dronm/modelbind"
	"github.com/dronm/session"
	"github.com/dronm/skstruktura/internal/apperrors"
	"github.com/dronm/skstruktura/internal/integrations/diadoc"
	"github.com/dronm/skstruktura/internal/models"
	"github.com/dronm/webapp"
)

const (
	diadocDocumentDefaultPageSize = 50
	diadocDocumentMaxPageSize     = 500
)

type DiadocImportService struct {
	DB         ds.Provider
	Session    session.Session
	Manager    *diadoc.Manager
	Authorizer *Authorizer
}

func RegisterDiadocImportService(manager *diadoc.Manager, permissions *PermissionService) {
	webapp.MustRegisterService(
		"DiadocImport",
		&DiadocImportService{},
		func(ctx webapp.ServiceContext) any {
			return &DiadocImportService{
				DB:         ctx.DB,
				Session:    ctx.Session,
				Manager:    manager,
				Authorizer: NewAuthorizer(permissions),
			}
		},
	)
}

func (s *DiadocImportService) require() (models.UserLogin, error) {
	if s.Session == nil {
		return models.UserLogin{}, apperrors.SessionRequired()
	}
	if s.DB == nil {
		return models.UserLogin{}, webapp.Internal("database is not initialized", nil)
	}
	var user models.UserLogin
	if err := s.Session.Get("user", &user); err != nil {
		return models.UserLogin{}, apperrors.SessionRequired()
	}
	return user, nil
}

func (s *DiadocImportService) List(
	ctx context.Context,
	input models.DiadocDocumentListInput,
) (models.DiadocDocumentListResponse, error) {
	if _, err := s.require(); err != nil {
		return models.DiadocDocumentListResponse{}, err
	}
	query, params, err := validateDiadocDocumentListInput(input)
	if err != nil {
		return models.DiadocDocumentListResponse{}, err
	}

	poolConn, connID, err := s.DB.GetPrimary(ctx)
	if err != nil {
		return models.DiadocDocumentListResponse{}, fmt.Errorf("get primary connection for Diadoc buffer: %w", err)
	}
	defer s.DB.Release(poolConn, connID)

	return fetchDiadocDocumentList(ctx, poolConn.Conn(), query, params)
}

func (s *DiadocImportService) Detail(
	ctx context.Context,
	id int64,
) (*models.DiadocDocumentDetail, error) {
	if _, err := s.require(); err != nil {
		return nil, err
	}
	if id <= 0 {
		return nil, webapp.BadRequest("Diadoc document id should be positive", nil)
	}

	poolConn, connID, err := s.DB.GetPrimary(ctx)
	if err != nil {
		return nil, fmt.Errorf("get primary connection for Diadoc document: %w", err)
	}
	defer s.DB.Release(poolConn, connID)

	document, err := fetchDiadocDocumentDetail(ctx, poolConn.Conn(), id)
	if errors.Is(err, ds.ErrNoRows) {
		return nil, webapp.NotFound("Diadoc document not found", map[string]any{"id": id})
	}
	if err != nil {
		return nil, fmt.Errorf("fetch Diadoc document: %w", err)
	}
	return document, nil
}

func validateDiadocDocumentListInput(
	input models.DiadocDocumentListInput,
) (models.DiadocDocumentListQuery, modelbind.CollectionParams, error) {
	query := input.Query
	query.Status = strings.TrimSpace(query.Status)
	query.Search = strings.TrimSpace(query.Search)
	if query.Status == "" {
		query.Status = "active"
	}
	validStatuses := map[string]struct{}{
		"active":         {},
		"received":       {},
		"needs_matching": {},
		"ready":          {},
		"imported":       {},
		"ignored":        {},
		"failed":         {},
		"revoked":        {},
		"superseded":     {},
		"all":            {},
	}
	if _, ok := validStatuses[query.Status]; !ok {
		return query, input.Params, webapp.BadRequest("unsupported Diadoc document status", nil)
	}
	if query.DateFrom != nil && query.DateTo != nil && query.DateFrom.After(*query.DateTo) {
		return query, input.Params, webapp.BadRequest("date_from should not be after date_to", nil)
	}
	if query.SupplierID != nil && *query.SupplierID <= 0 {
		return query, input.Params, webapp.BadRequest("supplier_id should be positive", nil)
	}
	if query.ConstructionSiteID != nil && *query.ConstructionSiteID <= 0 {
		return query, input.Params, webapp.BadRequest("construction_site_id should be positive", nil)
	}
	params := input.Params
	if len(params.Filter) > 0 || len(params.Sorter) > 0 {
		return query, params, webapp.BadRequest("generic Diadoc buffer filters and sorting are not supported", nil)
	}
	if params.From < 0 || params.Count < 0 {
		return query, params, webapp.BadRequest("pagination values should not be negative", nil)
	}
	if params.Count == 0 {
		params.Count = diadocDocumentDefaultPageSize
	}
	if params.Count > diadocDocumentMaxPageSize {
		params.Count = diadocDocumentMaxPageSize
	}
	return query, params, nil
}

func fetchDiadocDocumentList(
	ctx context.Context,
	db ds.Querier,
	query models.DiadocDocumentListQuery,
	params modelbind.CollectionParams,
) (models.DiadocDocumentListResponse, error) {
	conditions := make([]string, 0, 8)
	args := make([]any, 0, 8)
	addArg := func(value any) string {
		args = append(args, value)
		return fmt.Sprintf("$%d", len(args))
	}

	switch query.Status {
	case "active":
		conditions = append(conditions, "document.status IN ('received', 'needs_matching', 'ready', 'failed')")
	case "all":
	default:
		conditions = append(conditions, "document.status = "+addArg(query.Status))
	}
	if query.Search != "" {
		placeholder := addArg("%" + query.Search + "%")
		conditions = append(conditions, `(
			document.document_number ILIKE `+placeholder+`
			OR document.sender_name ILIKE `+placeholder+`
			OR document.sender_inn ILIKE `+placeholder+`
		)`)
	}
	if query.DateFrom != nil {
		conditions = append(conditions, "document.document_date >= "+addArg(*query.DateFrom)+"::date")
	}
	if query.DateTo != nil {
		conditions = append(conditions, "document.document_date <= "+addArg(*query.DateTo)+"::date")
	}
	if query.SupplierID != nil {
		conditions = append(conditions, "document.supplier_id = "+addArg(*query.SupplierID))
	}
	if query.ConstructionSiteID != nil {
		conditions = append(conditions, "document.construction_site_id = "+addArg(*query.ConstructionSiteID))
	}
	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}
	limit := addArg(int(params.Count))
	offset := addArg(int(params.From))

	rows, err := db.Query(ctx, `
		SELECT
			document.id,
			document.version,
			document.status,
			COALESCE(document.document_number, ''),
			document.document_date,
			COALESCE(document.sender_name, ''),
			COALESCE(document.sender_inn, ''),
			COALESCE(document.sender_kpp, ''),
			document.supplier_id,
			COALESCE(supplier.name, ''),
			document.construction_site_id,
			COALESCE(site.name, ''),
			COALESCE(document.receipt_number, ''),
			document.receipt_date,
			document.line_count,
			document.amount_without_vat::text,
			document.vat_amount::text,
			document.amount_with_vat::text,
			(
				CASE WHEN document.supplier_id IS NULL OR NOT COALESCE(supplier.is_active, false) THEN 1 ELSE 0 END
				+ CASE WHEN document.receipt_date IS NULL THEN 1 ELSE 0 END
				+ CASE WHEN COALESCE(document.receipt_number, '') = '' THEN 1 ELSE 0 END
				+ CASE WHEN NOT EXISTS (
					SELECT 1
					FROM integration_diadoc.document_items AS included_item
					WHERE included_item.document_id = document.id
						AND NOT included_item.is_excluded
				) THEN 1 ELSE 0 END
				+ (
					SELECT count(*)::integer
					FROM integration_diadoc.document_items AS item
					LEFT JOIN public.materials AS item_material ON item_material.id = item.material_id
					LEFT JOIN public.measure_units AS item_unit ON item_unit.id = item.measure_unit_id
					WHERE item.document_id = document.id
						AND NOT item.is_excluded
						AND (
							item.material_id IS NULL
							OR NOT EXISTS (
								SELECT 1 FROM public.construction_sites AS effective_site
								WHERE effective_site.id = COALESCE(item.construction_site_id, document.construction_site_id)
									AND effective_site.is_active
							)
							OR NOT COALESCE(item_material.is_active, false)
							OR item.measure_unit_id IS NULL
							OR NOT COALESCE(item_unit.is_active, false)
							OR item_material.measure_unit_id <> item.measure_unit_id
							OR item.conversion_factor IS NULL
							OR item.conversion_factor <= 0
							OR item.import_quant IS NULL
							OR item.import_quant <= 0
							OR item.import_price IS NULL
							OR item.import_price < 0
							OR item.import_amount IS DISTINCT FROM item.source_amount_with_vat
							OR item.import_vat_percent IS DISTINCT FROM item.source_vat_percent
							OR item.import_vat_amount IS DISTINCT FROM item.source_vat_amount
						)
				)
			)::integer AS missing_count,
			COALESCE(document.last_error, ''),
			document.material_receipt_id,
			document.updated_at,
			COUNT(*) OVER()::bigint
		FROM integration_diadoc.documents AS document
		LEFT JOIN public.suppliers AS supplier ON supplier.id = document.supplier_id
		LEFT JOIN public.construction_sites AS site ON site.id = document.construction_site_id
		`+where+`
		ORDER BY document.document_date DESC NULLS LAST, document.id DESC
		LIMIT `+limit+` OFFSET `+offset,
		args...,
	)
	if err != nil {
		return models.DiadocDocumentListResponse{}, fmt.Errorf("select Diadoc buffer: %w", err)
	}
	defer rows.Close()

	result := models.DiadocDocumentListResponse{Rows: make([]*models.DiadocDocumentListRow, 0)}
	for rows.Next() {
		row := &models.DiadocDocumentListRow{}
		var supplierName, siteName string
		if err := rows.Scan(
			&row.ID,
			&row.Version,
			&row.Status,
			&row.DocumentNumber,
			&row.DocumentDate,
			&row.SenderName,
			&row.SenderINN,
			&row.SenderKPP,
			&row.SupplierID,
			&supplierName,
			&row.ConstructionSiteID,
			&siteName,
			&row.ReceiptNumber,
			&row.ReceiptDate,
			&row.LineCount,
			&row.AmountWithoutVAT,
			&row.VATAmount,
			&row.AmountWithVAT,
			&row.MissingCount,
			&row.LastError,
			&row.MaterialReceiptID,
			&row.UpdatedAt,
			&result.Total,
		); err != nil {
			return models.DiadocDocumentListResponse{}, err
		}
		if row.SupplierID != nil {
			row.Supplier = reportRef(*row.SupplierID, supplierName)
		}
		if row.ConstructionSiteID != nil {
			row.ConstructionSite = reportRef(*row.ConstructionSiteID, siteName)
		}
		result.Rows = append(result.Rows, row)
	}
	if err := rows.Err(); err != nil {
		return models.DiadocDocumentListResponse{}, err
	}
	return result, nil
}

func positiveDecimal(value string) bool {
	result, ok := new(big.Rat).SetString(strings.TrimSpace(value))
	return ok && result.Sign() > 0
}

func setAuditActor(ctx context.Context, tx ds.Querier, user models.UserLogin) error {
	if _, err := tx.Exec(ctx, `
		SELECT
			set_config('app.user_name', $1, true),
			set_config('app.user_role', $2, true)
	`, user.Name, string(user.RoleID)); err != nil {
		return fmt.Errorf("set audit actor: %w", err)
	}
	return nil
}
