package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dronm/modelbind"
	"github.com/dronm/skstruktura/internal/models"
	"github.com/dronm/webapp"
)

func TestMaterialStatusGenericServiceGuardAllowsOnlyAdmin(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		roleID     models.RoleID
		wantStatus int
	}{
		{name: "admin", roleID: models.RoleIDAdmin},
		{name: "construction site manager", roleID: models.RoleIDConstructionSiteManager, wantStatus: 403},
		{name: "accountant", roleID: models.RoleIDAccountant, wantStatus: 403},
		{name: "supply manager", roleID: models.RoleIDSupplyManager, wantStatus: 403},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			err := (&MaterialStatusService{
				Session: &materialStatusSessionStub{
					user: models.UserLogin{ID: 7, RoleID: test.roleID},
				},
			}).requireSession()
			if test.wantStatus == 0 {
				if err != nil {
					t.Fatalf("guard error = %v", err)
				}
				return
			}
			assertMaterialStatusHTTPStatus(t, "guard", err, test.wantStatus)
		})
	}
}

func TestMaterialStatusGenericServiceGuardRequiresValidSessionUser(t *testing.T) {
	t.Parallel()

	assertMaterialStatusHTTPStatus(t, "missing session", (&MaterialStatusService{}).requireSession(), 401)
	assertMaterialStatusHTTPStatus(
		t,
		"missing user",
		(&MaterialStatusService{Session: &materialStatusSessionStub{}}).requireSession(),
		401,
	)
}

func TestMaterialStatusGenericMethodsRejectConstructionManager(t *testing.T) {
	t.Parallel()

	service := &MaterialStatusService{
		Session: &materialStatusSessionStub{
			user: models.UserLogin{ID: 7, RoleID: models.RoleIDConstructionSiteManager},
		},
	}
	ctx := context.Background()
	calls := map[string]func() error{
		"Create": func() error {
			_, err := service.Create(ctx, modelbind.ModelInput[*models.MaterialStatus]{})
			return err
		},
		"List": func() error {
			_, err := service.List(ctx, modelbind.CollectionParams{})
			return err
		},
		"Detail": func() error {
			_, err := service.Detail(ctx, 1)
			return err
		},
		"Update": func() error {
			_, err := service.Update(ctx, webapp.UpdateByKeysInput[*models.MaterialStatusKey, *models.MaterialStatus]{})
			return err
		},
		"Delete": func() error {
			_, err := service.Delete(ctx, 1)
			return err
		},
	}
	for name, call := range calls {
		assertMaterialStatusHTTPStatus(t, name, call(), 403)
	}
}

func TestGenericMaterialStatusCreateRequiresConstructionSite(t *testing.T) {
	t.Parallel()

	assertMaterialStatusHTTPStatus(
		t,
		"missing site",
		validateGenericMaterialStatusCreateInput(modelbind.ModelInput[*models.MaterialStatus]{
			Model: &models.MaterialStatus{},
		}),
		400,
	)

	siteID := 42
	if err := validateGenericMaterialStatusCreateInput(modelbind.ModelInput[*models.MaterialStatus]{
		Model: &models.MaterialStatus{ConstructionSiteID: &siteID},
	}); err != nil {
		t.Fatalf("create input with site error = %v", err)
	}
}

func TestGenericMaterialStatusUpdateRejectsExplicitNullSiteButAllowsOmittedSite(t *testing.T) {
	t.Parallel()

	present := modelbind.NewAbsentFieldSet()
	assertMaterialStatusHTTPStatus(
		t,
		"explicit null site",
		validateGenericMaterialStatusUpdateInput(modelbind.ModelInput[*models.MaterialStatus]{
			Model:        &models.MaterialStatus{},
			AbsentFields: present,
		}),
		400,
	)

	omitted := modelbind.NewAbsentFieldSet()
	omitted.SetAbsent("construction_site_id")
	if err := validateGenericMaterialStatusUpdateInput(modelbind.ModelInput[*models.MaterialStatus]{
		Model:        &models.MaterialStatus{},
		AbsentFields: omitted,
	}); err != nil {
		t.Fatalf("update input with omitted site error = %v", err)
	}
}

type materialStatusSessionStub struct {
	user   models.UserLogin
	getErr error
}

func (s *materialStatusSessionStub) Set(string, any) error { return nil }

func (s *materialStatusSessionStub) Put(string, any) error { return nil }

func (s *materialStatusSessionStub) Get(key string, value any) error {
	if s.getErr != nil {
		return s.getErr
	}
	if key != "user" {
		return errors.New("unexpected session key")
	}
	user, ok := value.(*models.UserLogin)
	if !ok {
		return errors.New("unexpected session value type")
	}
	*user = s.user
	return nil
}

func (s *materialStatusSessionStub) GetBool(string) bool     { return false }
func (s *materialStatusSessionStub) GetString(string) string { return "" }
func (s *materialStatusSessionStub) GetInt(string) int64     { return 0 }
func (s *materialStatusSessionStub) GetFloat(string) float64 { return 0 }
func (s *materialStatusSessionStub) Delete(string) error     { return nil }
func (s *materialStatusSessionStub) SessionID() string       { return "material-status-test" }
func (s *materialStatusSessionStub) Flush() error            { return nil }
func (s *materialStatusSessionStub) TimeCreated() time.Time  { return time.Time{} }
func (s *materialStatusSessionStub) TimeAccessed() time.Time { return time.Time{} }
