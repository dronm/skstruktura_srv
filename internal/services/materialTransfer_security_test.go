package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dronm/modelbind"
	"github.com/dronm/skstruktura/internal/models"
)

func TestMaterialTransferGenericServiceGuardsAllowOnlyAdmin(t *testing.T) {
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

			currentSession := &materialTransferSessionStub{
				user: models.UserLogin{ID: 7, RoleID: test.roleID},
			}
			guards := map[string]func() error{
				"document": (&MaterialTransferService{Session: currentSession}).requireSession,
				"item":     (&MaterialTransferItemService{Session: currentSession}).requireSession,
			}
			for name, guard := range guards {
				err := guard()
				if test.wantStatus == 0 {
					if err != nil {
						t.Errorf("%s guard error = %v", name, err)
					}
					continue
				}
				assertMaterialTransferHTTPStatus(t, name+" guard", err, test.wantStatus)
			}
		})
	}
}

func TestMaterialTransferGenericServiceGuardsRequireValidSessionUser(t *testing.T) {
	t.Parallel()

	guards := map[string]func() error{
		"missing document session": (&MaterialTransferService{}).requireSession,
		"missing item session":     (&MaterialTransferItemService{}).requireSession,
		"missing document user": (&MaterialTransferService{
			Session: &materialTransferSessionStub{},
		}).requireSession,
		"missing item user": (&MaterialTransferItemService{
			Session: &materialTransferSessionStub{},
		}).requireSession,
	}
	for name, guard := range guards {
		assertMaterialTransferHTTPStatus(t, name, guard(), 401)
	}
}

func TestMaterialTransferGenericMethodsRejectConstructionManager(t *testing.T) {
	t.Parallel()

	currentSession := &materialTransferSessionStub{
		user: models.UserLogin{ID: 7, RoleID: models.RoleIDConstructionSiteManager},
	}
	documentService := &MaterialTransferService{Session: currentSession}
	itemService := &MaterialTransferItemService{Session: currentSession}
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
			_, err := documentService.Update(ctx, models.UpdateMaterialTransferDocumentRequest{})
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
		assertMaterialTransferHTTPStatus(t, name, call(), 403)
	}
}

type materialTransferSessionStub struct {
	user   models.UserLogin
	getErr error
}

func (s *materialTransferSessionStub) Set(string, any) error { return nil }

func (s *materialTransferSessionStub) Put(string, any) error { return nil }

func (s *materialTransferSessionStub) Get(key string, value any) error {
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

func (s *materialTransferSessionStub) GetBool(string) bool     { return false }
func (s *materialTransferSessionStub) GetString(string) string { return "" }
func (s *materialTransferSessionStub) GetInt(string) int64     { return 0 }
func (s *materialTransferSessionStub) GetFloat(string) float64 { return 0 }
func (s *materialTransferSessionStub) Delete(string) error     { return nil }
func (s *materialTransferSessionStub) SessionID() string       { return "material-transfer-test" }
func (s *materialTransferSessionStub) Flush() error            { return nil }
func (s *materialTransferSessionStub) TimeCreated() time.Time  { return time.Time{} }
func (s *materialTransferSessionStub) TimeAccessed() time.Time { return time.Time{} }
