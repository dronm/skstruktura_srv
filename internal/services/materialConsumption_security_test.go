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

func TestMaterialConsumptionGenericServiceGuardsAllowOnlyAdmin(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		roleID     models.RoleID
		wantStatus int
	}{
		{name: "admin", roleID: models.RoleIDAdmin},
		{name: "construction site manager", roleID: models.RoleIDConstructionSiteManager, wantStatus: 403},
		{name: "accountant", roleID: models.RoleIDAccountant, wantStatus: 403},
		{name: "supplier", roleID: models.RoleIDSupplier, wantStatus: 403},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			currentSession := &materialConsumptionSessionStub{
				user: models.UserLogin{ID: 7, RoleID: test.roleID},
			}
			guards := map[string]func() error{
				"document": (&MaterialConsumptionService{Session: currentSession}).requireSession,
				"item":     (&MaterialConsumptionItemService{Session: currentSession}).requireSession,
			}
			for name, guard := range guards {
				err := guard()
				if test.wantStatus == 0 {
					if err != nil {
						t.Errorf("%s guard error = %v", name, err)
					}
					continue
				}
				assertMaterialConsumptionHTTPStatus(t, name+" guard", err, test.wantStatus)
			}
		})
	}
}

func TestMaterialConsumptionGenericServiceGuardsRequireValidSessionUser(t *testing.T) {
	t.Parallel()

	guards := map[string]func() error{
		"missing document session": (&MaterialConsumptionService{}).requireSession,
		"missing item session":     (&MaterialConsumptionItemService{}).requireSession,
		"missing document user": (&MaterialConsumptionService{
			Session: &materialConsumptionSessionStub{},
		}).requireSession,
		"missing item user": (&MaterialConsumptionItemService{
			Session: &materialConsumptionSessionStub{},
		}).requireSession,
	}
	for name, guard := range guards {
		assertMaterialConsumptionHTTPStatus(t, name, guard(), 401)
	}
}

func TestMaterialConsumptionGenericMethodsRejectConstructionManager(t *testing.T) {
	t.Parallel()

	currentSession := &materialConsumptionSessionStub{
		user: models.UserLogin{ID: 7, RoleID: models.RoleIDConstructionSiteManager},
	}
	documentService := &MaterialConsumptionService{Session: currentSession}
	itemService := &MaterialConsumptionItemService{Session: currentSession}
	ctx := context.Background()

	calls := map[string]func() error{
		"Create": func() error {
			_, err := documentService.Create(ctx, nil)
			return err
		},
		"List": func() error {
			_, err := documentService.List(ctx, modelbind.CollectionParams{})
			return err
		},
		"Detail": func() error {
			_, err := documentService.Detail(ctx, 1)
			return err
		},
		"DocumentDetail": func() error {
			_, err := documentService.DocumentDetail(ctx, 1)
			return err
		},
		"Update": func() error {
			_, err := documentService.Update(ctx, models.UpdateMaterialConsumptionDocumentRequest{})
			return err
		},
		"Delete": func() error {
			_, err := documentService.Delete(ctx, 1)
			return err
		},
		"ItemList": func() error {
			_, err := itemService.List(ctx, modelbind.CollectionParams{})
			return err
		},
		"ItemDetail": func() error {
			_, err := itemService.Detail(ctx, 1)
			return err
		},
	}

	for name, call := range calls {
		assertMaterialConsumptionHTTPStatus(t, name, call(), 403)
	}
}

func assertMaterialConsumptionHTTPStatus(t *testing.T, name string, err error, want int) {
	t.Helper()

	var appErr *webapp.AppError
	if !errors.As(err, &appErr) || appErr.StatusCode() != want {
		t.Errorf("%s error = %#v, want HTTP %d", name, err, want)
	}
}

type materialConsumptionSessionStub struct {
	user   models.UserLogin
	getErr error
}

func (s *materialConsumptionSessionStub) Set(string, any) error {
	return nil
}

func (s *materialConsumptionSessionStub) Put(string, any) error {
	return nil
}

func (s *materialConsumptionSessionStub) Get(key string, value any) error {
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

func (s *materialConsumptionSessionStub) GetBool(string) bool {
	return false
}

func (s *materialConsumptionSessionStub) GetString(string) string {
	return ""
}

func (s *materialConsumptionSessionStub) GetInt(string) int64 {
	return 0
}

func (s *materialConsumptionSessionStub) GetFloat(string) float64 {
	return 0
}

func (s *materialConsumptionSessionStub) Delete(string) error {
	return nil
}

func (s *materialConsumptionSessionStub) SessionID() string {
	return "material-consumption-test"
}

func (s *materialConsumptionSessionStub) Flush() error {
	return nil
}

func (s *materialConsumptionSessionStub) TimeCreated() time.Time {
	return time.Time{}
}

func (s *materialConsumptionSessionStub) TimeAccessed() time.Time {
	return time.Time{}
}
