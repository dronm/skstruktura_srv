package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/dronm/ds/v4"
	"github.com/dronm/modelbind"
	"github.com/dronm/session"
	"github.com/dronm/skstruktura/internal/apperrors"
	"github.com/dronm/skstruktura/internal/models"
	"github.com/dronm/webapp"
)

const materialActionReportMaxPageSize = 1000

type MaterialActionReportService struct {
	DB      ds.Provider
	Session session.Session
	QueryID string
}

func NewMaterialActionReportService(ctx webapp.ServiceContext) any {
	return &MaterialActionReportService{
		DB:      ctx.DB,
		Session: ctx.Session,
		QueryID: ctx.QueryID,
	}
}

func RegisterMaterialActionReportService() {
	webapp.MustRegisterService(
		"MaterialActionReport",
		&MaterialActionReportService{},
		NewMaterialActionReportService,
	)
}

func (s *MaterialActionReportService) List(
	ctx context.Context,
	input models.MaterialActionReportInput,
) (models.MaterialActionReportResponse, error) {
	if s.Session == nil {
		return models.MaterialActionReportResponse{}, apperrors.SessionRequired()
	}
	if s.DB == nil {
		return models.MaterialActionReportResponse{}, webapp.Internal("database is not initialized", nil)
	}

	query, params, err := validateMaterialActionReportInput(input)
	if err != nil {
		return models.MaterialActionReportResponse{}, err
	}

	poolConn, connID, err := s.DB.GetPrimary(ctx)
	if err != nil {
		return models.MaterialActionReportResponse{}, fmt.Errorf("get primary connection for material action report: %w", err)
	}
	defer s.DB.Release(poolConn, connID)

	result := models.MaterialActionReportResponse{
		Rows:        make([]*models.MaterialActionReportRow, 0),
		GeneratedAt: time.Now().UTC(),
	}

	switch query.Level {
	case models.MaterialActionReportLevelConstructionSite:
		result.Rows, result.Total, err = fetchMaterialActionReportSites(
			ctx,
			poolConn.Conn(),
			query,
			params,
		)
	case models.MaterialActionReportLevelMaterial:
		result.Rows, result.Total, err = fetchMaterialActionReportMaterials(
			ctx,
			poolConn.Conn(),
			query,
			params,
		)
	case models.MaterialActionReportLevelDocument:
		result.Rows, result.Total, err = fetchMaterialActionReportDocuments(
			ctx,
			poolConn.Conn(),
			query,
			params,
		)
	}
	if err != nil {
		return models.MaterialActionReportResponse{}, err
	}

	return result, nil
}

func validateMaterialActionReportInput(
	input models.MaterialActionReportInput,
) (*models.MaterialActionReportQuery, modelbind.CollectionParams, error) {
	if input.Query == nil {
		return nil, modelbind.CollectionParams{}, webapp.BadRequest("material action report query is required", nil)
	}

	query := *input.Query
	query.ConstructionSiteIDs = normalizePositiveIDs(query.ConstructionSiteIDs)
	query.MaterialIDs = normalizePositiveIDs(query.MaterialIDs)

	if query.DateFrom.IsZero() || query.DateTo.IsZero() {
		return nil, modelbind.CollectionParams{}, webapp.BadRequest("date_from and date_to are required", nil)
	}
	if query.DateFrom.After(query.DateTo) {
		return nil, modelbind.CollectionParams{}, webapp.BadRequest("date_from should not be after date_to", nil)
	}
	if containsNonPositiveID(input.Query.ConstructionSiteIDs) {
		return nil, modelbind.CollectionParams{}, webapp.BadRequest("construction_site_ids should contain only positive values", nil)
	}
	if containsNonPositiveID(input.Query.MaterialIDs) {
		return nil, modelbind.CollectionParams{}, webapp.BadRequest("material_ids should contain only positive values", nil)
	}
	if len(input.Params.Filter) > 0 {
		return nil, modelbind.CollectionParams{}, webapp.BadRequest(
			"generic collection filters are not supported; use the typed report filters",
			nil,
		)
	}
	if input.Params.From < 0 {
		return nil, modelbind.CollectionParams{}, webapp.BadRequest("from should not be negative", nil)
	}
	if input.Params.Count < 0 {
		return nil, modelbind.CollectionParams{}, webapp.BadRequest("count should not be negative", nil)
	}

	switch query.Level {
	case models.MaterialActionReportLevelConstructionSite:
		if query.ParentConstructionSiteID != nil || query.ParentMaterialID != nil {
			return nil, modelbind.CollectionParams{}, webapp.BadRequest(
				"parent ids are not allowed at construction_site level",
				nil,
			)
		}
	case models.MaterialActionReportLevelMaterial:
		if err := validateReportParent(query.ParentConstructionSiteID, "parent_construction_site_id"); err != nil {
			return nil, modelbind.CollectionParams{}, err
		}
		if query.ParentMaterialID != nil {
			return nil, modelbind.CollectionParams{}, webapp.BadRequest(
				"parent_material_id is not allowed at material level",
				nil,
			)
		}
		if len(query.ConstructionSiteIDs) > 0 && !containsID(query.ConstructionSiteIDs, *query.ParentConstructionSiteID) {
			return nil, modelbind.CollectionParams{}, webapp.BadRequest(
				"parent_construction_site_id is outside construction_site_ids",
				nil,
			)
		}
	case models.MaterialActionReportLevelDocument:
		if err := validateReportParent(query.ParentConstructionSiteID, "parent_construction_site_id"); err != nil {
			return nil, modelbind.CollectionParams{}, err
		}
		if err := validateReportParent(query.ParentMaterialID, "parent_material_id"); err != nil {
			return nil, modelbind.CollectionParams{}, err
		}
		if len(query.ConstructionSiteIDs) > 0 && !containsID(query.ConstructionSiteIDs, *query.ParentConstructionSiteID) {
			return nil, modelbind.CollectionParams{}, webapp.BadRequest(
				"parent_construction_site_id is outside construction_site_ids",
				nil,
			)
		}
		if len(query.MaterialIDs) > 0 && !containsID(query.MaterialIDs, *query.ParentMaterialID) {
			return nil, modelbind.CollectionParams{}, webapp.BadRequest(
				"parent_material_id is outside material_ids",
				nil,
			)
		}
	default:
		return nil, modelbind.CollectionParams{}, webapp.BadRequest(
			"level should be construction_site, material, or document",
			nil,
		)
	}

	params := input.Params
	if params.Count == 0 {
		if query.Level == models.MaterialActionReportLevelDocument {
			params.Count = 100
		} else {
			params.Count = 200
		}
	}
	if params.Count > materialActionReportMaxPageSize {
		params.Count = materialActionReportMaxPageSize
	}
	if _, err := materialActionReportOrderBy(query.Level, params.Sorter); err != nil {
		return nil, modelbind.CollectionParams{}, err
	}

	return &query, params, nil
}

func fetchMaterialActionReportSites(
	ctx context.Context,
	db ds.Querier,
	query *models.MaterialActionReportQuery,
	params modelbind.CollectionParams,
) ([]*models.MaterialActionReportRow, int64, error) {
	orderBy, err := materialActionReportOrderBy(query.Level, params.Sorter)
	if err != nil {
		return nil, 0, err
	}

	rows, err := db.Query(ctx, `
		WITH per_material AS (
			SELECT report.*
			FROM public.material_actions_report_totals($1::timestamptz, $2::timestamptz, $3, $4) AS report
		), grouped AS (
			SELECT
				site.id AS construction_site_id,
				site.name AS construction_site_name,
				SUM(per_material.balance_start)::double precision AS balance_start,
				SUM(per_material.income)::double precision AS income,
				SUM(per_material.outcome)::double precision AS outcome,
				SUM(per_material.balance_end)::double precision AS balance_end,
				COUNT(*)::bigint AS child_count
			FROM per_material
			JOIN public.construction_sites AS site
				ON site.id = per_material.construction_site_id
			GROUP BY site.id, site.name
		)
		SELECT
			grouped.*,
			COUNT(*) OVER()::bigint AS total_count
		FROM grouped
		ORDER BY `+orderBy+`
		LIMIT $5 OFFSET $6
	`,
		query.DateFrom,
		query.DateTo,
		query.ConstructionSiteIDs,
		query.MaterialIDs,
		int(params.Count),
		int(params.From),
	)
	if err != nil {
		return nil, 0, fmt.Errorf("select construction-site material action totals: %w", err)
	}
	defer rows.Close()

	result := make([]*models.MaterialActionReportRow, 0)
	total := int64(0)
	for rows.Next() {
		var row models.MaterialActionReportRow
		var siteName string
		var balanceStart, income, outcome, balanceEnd float64
		if err := rows.Scan(
			&row.ConstructionSiteID,
			&siteName,
			&balanceStart,
			&income,
			&outcome,
			&balanceEnd,
			&row.ChildCount,
			&total,
		); err != nil {
			return nil, 0, fmt.Errorf("scan construction-site material action total: %w", err)
		}

		row.Key = fmt.Sprintf("construction-site:%d", row.ConstructionSiteID)
		row.RowType = models.MaterialActionReportLevelConstructionSite
		row.ConstructionSite = reportRef(row.ConstructionSiteID, siteName)
		row.Caption = siteName
		row.BalanceStart = float64Ptr(balanceStart)
		row.Income = float64Ptr(income)
		row.Outcome = float64Ptr(outcome)
		row.BalanceEnd = float64Ptr(balanceEnd)
		row.HasChildren = row.ChildCount > 0
		result = append(result, &row)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate construction-site material action totals: %w", err)
	}

	return result, total, nil
}

func fetchMaterialActionReportMaterials(
	ctx context.Context,
	db ds.Querier,
	query *models.MaterialActionReportQuery,
	params modelbind.CollectionParams,
) ([]*models.MaterialActionReportRow, int64, error) {
	orderBy, err := materialActionReportOrderBy(query.Level, params.Sorter)
	if err != nil {
		return nil, 0, err
	}

	rows, err := db.Query(ctx, `
		WITH report_data AS (
			SELECT
				report.construction_site_id,
				site.name AS construction_site_name,
				report.material_id,
				material.name AS material_name,
				report.balance_start::double precision AS balance_start,
				report.income::double precision AS income,
				report.outcome::double precision AS outcome,
				report.balance_end::double precision AS balance_end,
				report.document_count
			FROM public.material_actions_report_totals($1::timestamptz, $2::timestamptz, $3, $4) AS report
			JOIN public.construction_sites AS site ON site.id = report.construction_site_id
			JOIN public.materials AS material ON material.id = report.material_id
		)
		SELECT
			report_data.*,
			COUNT(*) OVER()::bigint AS total_count
		FROM report_data
		ORDER BY `+orderBy+`
		LIMIT $5 OFFSET $6
	`,
		query.DateFrom,
		query.DateTo,
		[]int{*query.ParentConstructionSiteID},
		query.MaterialIDs,
		int(params.Count),
		int(params.From),
	)
	if err != nil {
		return nil, 0, fmt.Errorf("select material action totals: %w", err)
	}
	defer rows.Close()

	result := make([]*models.MaterialActionReportRow, 0)
	total := int64(0)
	for rows.Next() {
		var row models.MaterialActionReportRow
		var siteName, materialName string
		var materialID int
		var balanceStart, income, outcome, balanceEnd float64
		if err := rows.Scan(
			&row.ConstructionSiteID,
			&siteName,
			&materialID,
			&materialName,
			&balanceStart,
			&income,
			&outcome,
			&balanceEnd,
			&row.ChildCount,
			&total,
		); err != nil {
			return nil, 0, fmt.Errorf("scan material action total: %w", err)
		}

		row.Key = fmt.Sprintf("construction-site:%d/material:%d", row.ConstructionSiteID, materialID)
		row.RowType = models.MaterialActionReportLevelMaterial
		row.ConstructionSite = reportRef(row.ConstructionSiteID, siteName)
		row.MaterialID = intPtr(materialID)
		row.Material = reportRef(materialID, materialName)
		row.Caption = materialName
		row.BalanceStart = float64Ptr(balanceStart)
		row.Income = float64Ptr(income)
		row.Outcome = float64Ptr(outcome)
		row.BalanceEnd = float64Ptr(balanceEnd)
		row.HasChildren = row.ChildCount > 0
		result = append(result, &row)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate material action totals: %w", err)
	}

	return result, total, nil
}

func fetchMaterialActionReportDocuments(
	ctx context.Context,
	db ds.Querier,
	query *models.MaterialActionReportQuery,
	params modelbind.CollectionParams,
) ([]*models.MaterialActionReportRow, int64, error) {
	orderBy, err := materialActionReportOrderBy(query.Level, params.Sorter)
	if err != nil {
		return nil, 0, err
	}

	rows, err := db.Query(ctx, `
		WITH report_data AS (
			SELECT
				report.construction_site_id,
				site.name AS construction_site_name,
				report.material_id,
				material.name AS material_name,
				report.recorder_type,
				report.recorder_id,
				report.document_date,
				to_char(
					report.document_date AT TIME ZONE public.register_business_timezone(),
					'DD/MM/YY HH24:MI:SS'
				) AS document_date_caption,
				report.document_number,
				report.income::double precision AS income,
				report.outcome::double precision AS outcome
			FROM public.material_actions_report_documents($1::timestamptz, $2::timestamptz, $3, $4) AS report
			JOIN public.construction_sites AS site ON site.id = report.construction_site_id
			JOIN public.materials AS material ON material.id = report.material_id
		)
		SELECT
			report_data.*,
			COUNT(*) OVER()::bigint AS total_count
		FROM report_data
		ORDER BY `+orderBy+`
		LIMIT $5 OFFSET $6
	`,
		query.DateFrom,
		query.DateTo,
		[]int{*query.ParentConstructionSiteID},
		[]int{*query.ParentMaterialID},
		int(params.Count),
		int(params.From),
	)
	if err != nil {
		return nil, 0, fmt.Errorf("select material action documents: %w", err)
	}
	defer rows.Close()

	result := make([]*models.MaterialActionReportRow, 0)
	total := int64(0)
	for rows.Next() {
		var row models.MaterialActionReportRow
		var siteName, materialName, recorderType, documentDateCaption string
		var materialID int
		var recorderID int64
		var documentDate time.Time
		var documentNumber *string
		var income, outcome float64
		if err := rows.Scan(
			&row.ConstructionSiteID,
			&siteName,
			&materialID,
			&materialName,
			&recorderType,
			&recorderID,
			&documentDate,
			&documentDateCaption,
			&documentNumber,
			&income,
			&outcome,
			&total,
		); err != nil {
			return nil, 0, fmt.Errorf("scan material action document: %w", err)
		}

		dateText := documentDate.UTC().Format(time.RFC3339Nano)
		row.Key = fmt.Sprintf(
			"construction-site:%d/material:%d/document:%s:%d",
			row.ConstructionSiteID,
			materialID,
			recorderType,
			recorderID,
		)
		row.RowType = models.MaterialActionReportLevelDocument
		row.ConstructionSite = reportRef(row.ConstructionSiteID, siteName)
		row.MaterialID = intPtr(materialID)
		row.Material = reportRef(materialID, materialName)
		row.RecorderType = stringPtr(recorderType)
		row.RecorderID = int64Ptr(recorderID)
		row.DocumentDate = &dateText
		row.DocumentNumber = documentNumber
		row.Caption = materialDocumentCaption(
			recorderType,
			recorderID,
			documentNumber,
			documentDateCaption,
		)
		row.Income = float64Ptr(income)
		row.Outcome = float64Ptr(outcome)
		result = append(result, &row)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate material action documents: %w", err)
	}

	return result, total, nil
}

func materialActionReportOrderBy(
	level models.MaterialActionReportLevel,
	sorters []modelbind.CollectionSorter,
) (string, error) {
	fields := map[string]string{}
	defaults := []string{}
	tieBreakers := []string{}

	switch level {
	case models.MaterialActionReportLevelConstructionSite:
		fields = map[string]string{
			"construction_site":    "lower(construction_site_name)",
			"construction_site_id": "construction_site_id",
			"caption":              "lower(construction_site_name)",
			"balance_start":        "balance_start",
			"income":               "income",
			"outcome":              "outcome",
			"balance_end":          "balance_end",
		}
		defaults = []string{"lower(construction_site_name) ASC", "construction_site_id ASC"}
		tieBreakers = []string{"construction_site_id ASC"}
	case models.MaterialActionReportLevelMaterial:
		fields = map[string]string{
			"material":      "lower(material_name)",
			"material_id":   "material_id",
			"caption":       "lower(material_name)",
			"balance_start": "balance_start",
			"income":        "income",
			"outcome":       "outcome",
			"balance_end":   "balance_end",
		}
		defaults = []string{"lower(material_name) ASC", "material_id ASC"}
		tieBreakers = []string{"material_id ASC"}
	case models.MaterialActionReportLevelDocument:
		fields = map[string]string{
			"document_date":   "document_date",
			"document_number": "document_number",
			"recorder_type":   "recorder_type",
			"recorder_id":     "recorder_id",
			"income":          "income",
			"outcome":         "outcome",
		}
		defaults = []string{"document_date ASC", "recorder_type ASC", "recorder_id ASC"}
		tieBreakers = []string{"document_date ASC", "recorder_type ASC", "recorder_id ASC"}
	default:
		return "", webapp.BadRequest("unsupported material action report level", nil)
	}

	if len(sorters) == 0 {
		return strings.Join(defaults, ", "), nil
	}

	clauses := make([]string, 0, len(sorters)+len(tieBreakers))
	for _, sorter := range sorters {
		field, ok := fields[sorter.Field]
		if !ok {
			return "", webapp.BadRequest(
				fmt.Sprintf("unsupported sorter field %q for %s level", sorter.Field, level),
				nil,
			)
		}

		direction := ""
		switch sorter.Direct {
		case modelbind.SortParAsc:
			direction = "ASC"
		case modelbind.SortParDesc:
			direction = "DESC"
		default:
			return "", webapp.BadRequest(
				fmt.Sprintf("unsupported sorter direction %q", sorter.Direct),
				nil,
			)
		}
		clauses = append(clauses, field+" "+direction)
	}
	clauses = append(clauses, tieBreakers...)

	return strings.Join(clauses, ", "), nil
}

func materialDocumentCaption(
	recorderType string,
	recorderID int64,
	documentNumber *string,
	documentDate string,
) string {
	title := recorderType
	switch recorderType {
	case materialReceiptRecorderType:
		title = "Поступление материалов"
	case materialConsumptionRecorderType:
		title = "Списание материалов"
	case materialTransferRecorderType:
		title = "Перемещение материалов"
	}

	number := fmt.Sprintf("%d", recorderID)
	if documentNumber != nil && strings.TrimSpace(*documentNumber) != "" {
		number = strings.TrimSpace(*documentNumber)
	}

	return fmt.Sprintf("%s №%s %s", title, number, documentDate)
}

func normalizePositiveIDs(ids []int) []int {
	seen := make(map[int]struct{}, len(ids))
	result := make([]int, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result
}

func containsNonPositiveID(ids []int) bool {
	for _, id := range ids {
		if id <= 0 {
			return true
		}
	}
	return false
}

func containsID(ids []int, wanted int) bool {
	for _, id := range ids {
		if id == wanted {
			return true
		}
	}
	return false
}

func validateReportParent(id *int, field string) error {
	if id == nil || *id <= 0 {
		return webapp.BadRequest(field+" should be a positive integer", nil)
	}
	return nil
}

func reportRef(id int, description string) *models.Ref {
	ref := &models.Ref{Descr: description}
	ref.Keys.ID = id
	return ref
}

func intPtr(value int) *int             { return &value }
func int64Ptr(value int64) *int64       { return &value }
func float64Ptr(value float64) *float64 { return &value }
func stringPtr(value string) *string    { return &value }
