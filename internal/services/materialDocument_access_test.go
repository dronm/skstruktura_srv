package services

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/dronm/ds/v4"
	"github.com/dronm/modelbind"
	"github.com/dronm/skstruktura/internal/models"
	"github.com/dronm/webapp"
)

func TestConstructionManagerDocumentAccessQueries(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		fragments []string
		check     func(context.Context, ds.Querier, models.UserLogin, int) error
	}{
		{
			name: "consumption assigned site",
			fragments: []string{
				"public.material_consumptions",
				"site.is_active",
				"public.user_construction_sites",
			},
			check: requireConstructionManagerMaterialConsumptionAccess,
		},
		{
			name: "transfer assigned source or destination",
			fragments: []string{
				"public.material_transfers",
				"transfer.source_construction_site_id",
				"transfer.destination_construction_site_id",
				"site.is_active",
				"public.user_construction_sites",
			},
			check: requireConstructionManagerMaterialTransferAccess,
		},
		{
			name: "request assigned site",
			fragments: []string{
				"public.material_requests",
				"site.is_active",
				"public.user_construction_sites",
			},
			check: requireAssignedSiteMaterialRequestAccess,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			querier := &documentAccessQuerierStub{
				row: documentAccessRowStub{available: true},
			}
			user := models.UserLogin{ID: 17, RoleID: models.RoleIDConstructionSiteManager}
			if err := test.check(context.Background(), querier, user, 42); err != nil {
				t.Fatalf("access check error = %v", err)
			}
			for _, fragment := range test.fragments {
				if !strings.Contains(querier.query, fragment) {
					t.Errorf("access query does not contain %q:\n%s", fragment, querier.query)
				}
			}
			wantArgs := []any{42, false, 17}
			if !reflect.DeepEqual(querier.args, wantArgs) {
				t.Errorf("access query args = %#v, want %#v", querier.args, wantArgs)
			}
		})
	}
}

func TestConstructionManagerDocumentAccessReturnsNotFoundWhenOutOfScope(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		check func(context.Context, ds.Querier, models.UserLogin, int) error
	}{
		{name: "consumption", check: requireConstructionManagerMaterialConsumptionAccess},
		{name: "transfer", check: requireConstructionManagerMaterialTransferAccess},
		{name: "request", check: requireAssignedSiteMaterialRequestAccess},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := test.check(
				context.Background(),
				&documentAccessQuerierStub{},
				models.UserLogin{ID: 17, RoleID: models.RoleIDConstructionSiteManager},
				42,
			)
			assertDocumentAccessHTTPStatus(t, err, 404)
		})
	}
}

func TestConstructionManagerDocumentAccessPassesAdminBypass(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		check func(context.Context, ds.Querier, models.UserLogin, int) error
	}{
		{name: "consumption", check: requireConstructionManagerMaterialConsumptionAccess},
		{name: "transfer", check: requireConstructionManagerMaterialTransferAccess},
		{name: "request", check: requireAssignedSiteMaterialRequestAccess},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			querier := &documentAccessQuerierStub{
				row: documentAccessRowStub{available: true},
			}
			user := models.UserLogin{ID: 1, RoleID: models.RoleIDAdmin}
			if err := test.check(context.Background(), querier, user, 42); err != nil {
				t.Fatalf("access check error = %v", err)
			}
			wantArgs := []any{42, true, 1}
			if !reflect.DeepEqual(querier.args, wantArgs) {
				t.Errorf("access query args = %#v, want %#v", querier.args, wantArgs)
			}
		})
	}
}

func TestAssignedSiteMaterialRequestAccessSupportsSupplyManagerAndAdminBypass(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		user          models.UserLogin
		wantAdminFlag bool
	}{
		{
			name: "supply manager uses assignment scope",
			user: models.UserLogin{ID: 31, RoleID: models.RoleIDSupplyManager},
		},
		{
			name:          "admin bypass",
			user:          models.UserLogin{ID: 1, RoleID: models.RoleIDAdmin},
			wantAdminFlag: true,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			querier := &documentAccessQuerierStub{
				row: documentAccessRowStub{available: true},
			}
			if err := requireAssignedSiteMaterialRequestAccess(
				context.Background(),
				querier,
				test.user,
				73,
			); err != nil {
				t.Fatalf("access check error = %v", err)
			}
			wantArgs := []any{73, test.wantAdminFlag, test.user.ID}
			if !reflect.DeepEqual(querier.args, wantArgs) {
				t.Errorf("access query args = %#v, want %#v", querier.args, wantArgs)
			}
		})
	}
}

func TestSupplyManagerIncomingRequestDetailUsesCollectionEligibilityPredicate(t *testing.T) {
	t.Parallel()

	querier := &documentAccessQuerierStub{
		row: documentAccessRowStub{available: true},
	}
	user := models.UserLogin{ID: 31, RoleID: models.RoleIDSupplyManager}
	if err := requireSupplyManagerIncomingMaterialRequestAccess(
		context.Background(),
		querier,
		user,
		73,
	); err != nil {
		t.Fatalf("incoming request access check error = %v", err)
	}
	if !strings.Contains(querier.query, supplyManagerIncomingMaterialRequestPredicate) {
		t.Fatalf("detail query does not use the shared incoming predicate:\n%s", querier.query)
	}
	for _, fragment := range []string{
		"request_status.code = $8",
		"FROM public.material_request_items AS request_item",
		"item_status.code <> $8",
		"FROM public.material_request_supplier_assignment_items AS assignment_item",
		"FROM public.user_construction_sites AS assignment",
		"request.id = $9",
	} {
		if !strings.Contains(querier.query, fragment) {
			t.Errorf("detail query does not contain %q:\n%s", fragment, querier.query)
		}
	}
	wantArgs := []any{
		false,
		31,
		(*int)(nil),
		(*time.Time)(nil),
		(*time.Time)(nil),
		(*string)(nil),
		(*int)(nil),
		models.MaterialRequestStatusCodeNew,
		73,
	}
	if !reflect.DeepEqual(querier.args, wantArgs) {
		t.Errorf("detail query args = %#v, want %#v", querier.args, wantArgs)
	}
}

func TestSupplyManagerIncomingCollectionQueriesUseSharedEligibilityPredicate(t *testing.T) {
	t.Parallel()

	querier := &incomingRequestQueryCapture{}
	requestIDs, total, err := fetchSupplyManagerMaterialRequestIDs(
		context.Background(),
		querier,
		models.UserLogin{ID: 31, RoleID: models.RoleIDSupplyManager},
		&models.SupplyManagerMaterialRequestQuery{},
		modelbind.CollectionParams{Count: 30},
	)
	if err != nil {
		t.Fatalf("fetch incoming request ids error = %v", err)
	}
	if total != 0 || len(requestIDs) != 0 {
		t.Fatalf("result = ids %#v, total %d; want empty", requestIDs, total)
	}
	for name, query := range map[string]string{
		"count":  querier.countQuery,
		"select": querier.selectQuery,
	} {
		if !strings.Contains(query, supplyManagerIncomingMaterialRequestPredicate) {
			t.Errorf("%s query does not use the shared incoming predicate:\n%s", name, query)
		}
	}
}

func TestDocumentAccessPropagatesDatabaseError(t *testing.T) {
	t.Parallel()

	databaseError := errors.New("database unavailable")
	err := requireAssignedSiteMaterialRequestAccess(
		context.Background(),
		&documentAccessQuerierStub{row: documentAccessRowStub{err: databaseError}},
		models.UserLogin{ID: 17, RoleID: models.RoleIDConstructionSiteManager},
		42,
	)
	if !errors.Is(err, databaseError) {
		t.Fatalf("access check error = %v, want wrapped database error", err)
	}
}

func TestGenericMaterialRequestDocumentDetailUsesScopedRoleAuthorization(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		roleID     models.RoleID
		wantStatus int
	}{
		{name: "accountant", roleID: models.RoleIDAccountant, wantStatus: 403},
		{name: "supply manager", roleID: models.RoleIDSupplyManager, wantStatus: 403},
		{name: "unknown role", roleID: models.RoleID("unknown"), wantStatus: 403},
		{name: "construction manager reaches scoped service", roleID: models.RoleIDConstructionSiteManager, wantStatus: 500},
		{name: "admin reaches scoped service", roleID: models.RoleIDAdmin, wantStatus: 500},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			service := &MaterialRequestService{
				Session: &materialConsumptionSessionStub{
					user: models.UserLogin{ID: 17, RoleID: test.roleID},
				},
			}
			_, err := service.DocumentDetail(context.Background(), 42)
			assertDocumentAccessHTTPStatus(t, err, test.wantStatus)
		})
	}
}

func TestScopedPrintDetailMethodsRequireSessions(t *testing.T) {
	t.Parallel()

	calls := map[string]func() error{
		"consumption": func() error {
			_, err := (&MaterialConsumptionService{}).ConstructionManagerDetail(context.Background(), 1)
			return err
		},
		"transfer": func() error {
			_, err := (&MaterialTransferService{}).ConstructionManagerDetail(context.Background(), 1)
			return err
		},
		"request": func() error {
			_, err := (&MaterialRequestService{}).ConstructionManagerDetail(context.Background(), 1)
			return err
		},
		"supply request": func() error {
			_, err := (&MaterialRequestSupplierAssignmentService{}).SupplyManagerMaterialRequestDetail(
				context.Background(),
				1,
			)
			return err
		},
	}

	for name, call := range calls {
		name, call := name, call
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			assertDocumentAccessHTTPStatus(t, call(), 401)
		})
	}
}

func TestScopedPrintDetailMethodsRejectInvalidIDsBeforeDatabaseAccess(t *testing.T) {
	t.Parallel()

	constructionManagerSession := &materialConsumptionSessionStub{
		user: models.UserLogin{ID: 17, RoleID: models.RoleIDConstructionSiteManager},
	}
	supplyManagerSession := &materialConsumptionSessionStub{
		user: models.UserLogin{ID: 31, RoleID: models.RoleIDSupplyManager},
	}
	calls := map[string]func() error{
		"consumption": func() error {
			_, err := (&MaterialConsumptionService{Session: constructionManagerSession}).ConstructionManagerDetail(
				context.Background(),
				0,
			)
			return err
		},
		"transfer": func() error {
			_, err := (&MaterialTransferService{Session: constructionManagerSession}).ConstructionManagerDetail(
				context.Background(),
				-1,
			)
			return err
		},
		"request": func() error {
			_, err := (&MaterialRequestService{Session: constructionManagerSession}).ConstructionManagerDetail(
				context.Background(),
				0,
			)
			return err
		},
		"supply request": func() error {
			_, err := (&MaterialRequestSupplierAssignmentService{Session: supplyManagerSession}).SupplyManagerMaterialRequestDetail(
				context.Background(),
				-1,
			)
			return err
		},
	}

	for name, call := range calls {
		name, call := name, call
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			assertDocumentAccessHTTPStatus(t, call(), 400)
		})
	}
}

func TestScopedPrintDetailMethodsReturnNotFoundForOutOfScopeDocuments(t *testing.T) {
	t.Parallel()

	constructionManagerSession := &materialConsumptionSessionStub{
		user: models.UserLogin{ID: 17, RoleID: models.RoleIDConstructionSiteManager},
	}
	supplyManagerSession := &materialConsumptionSessionStub{
		user: models.UserLogin{ID: 31, RoleID: models.RoleIDSupplyManager},
	}
	newProvider := func() ds.Provider {
		return &documentAccessProviderStub{
			conn: &documentAccessConnStub{
				documentAccessQuerierStub: documentAccessQuerierStub{
					row: documentAccessRowStub{available: false},
				},
			},
		}
	}
	calls := map[string]func() error{
		"consumption": func() error {
			_, err := (&MaterialConsumptionService{
				Session: constructionManagerSession,
				DB:      newProvider(),
			}).ConstructionManagerDetail(context.Background(), 42)
			return err
		},
		"transfer": func() error {
			_, err := (&MaterialTransferService{
				Session: constructionManagerSession,
				DB:      newProvider(),
			}).ConstructionManagerDetail(context.Background(), 42)
			return err
		},
		"request": func() error {
			_, err := (&MaterialRequestService{
				Session: constructionManagerSession,
				DB:      newProvider(),
			}).ConstructionManagerDetail(context.Background(), 42)
			return err
		},
		"generic request": func() error {
			_, err := (&MaterialRequestService{
				Session: constructionManagerSession,
				DB:      newProvider(),
			}).DocumentDetail(context.Background(), 42)
			return err
		},
		"supply request": func() error {
			_, err := (&MaterialRequestSupplierAssignmentService{
				Session: supplyManagerSession,
				DB:      newProvider(),
			}).SupplyManagerMaterialRequestDetail(context.Background(), 42)
			return err
		},
	}

	for name, call := range calls {
		name, call := name, call
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			assertDocumentAccessHTTPStatus(t, call(), 404)
		})
	}
}

func assertDocumentAccessHTTPStatus(t *testing.T, err error, want int) {
	t.Helper()

	var appErr *webapp.AppError
	if !errors.As(err, &appErr) || appErr.StatusCode() != want {
		t.Fatalf("error = %#v, want HTTP %d", err, want)
	}
}

type documentAccessQuerierStub struct {
	query string
	args  []any
	row   documentAccessRowStub
}

func (q *documentAccessQuerierStub) Exec(context.Context, string, ...any) (ds.ExecResult, error) {
	return nil, errors.New("unexpected Exec call")
}

func (q *documentAccessQuerierStub) Query(context.Context, string, ...any) (ds.Rows, error) {
	return nil, errors.New("unexpected Query call")
}

func (q *documentAccessQuerierStub) QueryRow(_ context.Context, query string, args ...any) ds.Row {
	q.query = query
	q.args = args
	return q.row
}

type documentAccessRowStub struct {
	available bool
	err       error
}

func (r documentAccessRowStub) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(dest) != 1 {
		return errors.New("unexpected document access destination count")
	}
	available, ok := dest[0].(*bool)
	if !ok {
		return errors.New("unexpected document access destination type")
	}
	*available = r.available
	return nil
}

type documentAccessConnStub struct {
	documentAccessQuerierStub
}

func (c *documentAccessConnStub) Prepare(
	context.Context,
	string,
	string,
) (ds.PreparedStatement, error) {
	return nil, errors.New("unexpected Prepare call")
}

func (c *documentAccessConnStub) Begin(context.Context) (ds.Tx, error) {
	return nil, errors.New("unexpected Begin call")
}

type documentAccessPoolConnStub struct {
	conn ds.Conn
}

func (c *documentAccessPoolConnStub) Conn() ds.Conn {
	return c.conn
}

func (c *documentAccessPoolConnStub) Release() {}

type documentAccessProviderStub struct {
	conn ds.Conn
}

func (p *documentAccessProviderStub) GetPrimary(context.Context) (ds.PoolConn, ds.ServerID, error) {
	return &documentAccessPoolConnStub{conn: p.conn}, ds.ServerID("primary"), nil
}

func (p *documentAccessProviderStub) GetSecondary(
	context.Context,
	string,
) (ds.PoolConn, ds.ServerID, error) {
	return nil, "", errors.New("unexpected GetSecondary call")
}

func (p *documentAccessProviderStub) Release(ds.PoolConn, ds.ServerID) {}

func (p *documentAccessProviderStub) Close() error {
	return nil
}

type incomingRequestQueryCapture struct {
	countQuery  string
	selectQuery string
}

func (q *incomingRequestQueryCapture) Exec(context.Context, string, ...any) (ds.ExecResult, error) {
	return nil, errors.New("unexpected Exec call")
}

func (q *incomingRequestQueryCapture) Query(
	_ context.Context,
	query string,
	_ ...any,
) (ds.Rows, error) {
	q.selectQuery = query
	return incomingRequestEmptyRows{}, nil
}

func (q *incomingRequestQueryCapture) QueryRow(
	_ context.Context,
	query string,
	_ ...any,
) ds.Row {
	q.countQuery = query
	return incomingRequestCountRow{}
}

type incomingRequestCountRow struct{}

func (incomingRequestCountRow) Scan(dest ...any) error {
	if len(dest) != 1 {
		return errors.New("unexpected incoming request count destination count")
	}
	count, ok := dest[0].(*int)
	if !ok {
		return errors.New("unexpected incoming request count destination type")
	}
	*count = 0
	return nil
}

type incomingRequestEmptyRows struct{}

func (incomingRequestEmptyRows) Close() error      { return nil }
func (incomingRequestEmptyRows) Err() error        { return nil }
func (incomingRequestEmptyRows) Next() bool        { return false }
func (incomingRequestEmptyRows) Scan(...any) error { return errors.New("unexpected Scan call") }
