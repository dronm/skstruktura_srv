package services

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/dronm/modelbind"
	"github.com/dronm/modelbind/types"
	"github.com/dronm/skstruktura/internal/apperrors"
	"github.com/dronm/skstruktura/internal/config"
	"github.com/dronm/skstruktura/internal/models"

	"github.com/dronm/ds/v4"
	"github.com/dronm/session"
	"github.com/dronm/webapp"
	wmodels "github.com/dronm/webapp/models"
)

const userLoginQuery = "USER_LOGIN_QUERY"

type UserService struct {
	DB      ds.Provider
	Session session.Session
	QueryID string
	SessCfg config.SessionConfig
	MaxCfg  config.MAXConfig
}

func NewUserService(ctx webapp.ServiceContext, sessCfg config.SessionConfig, maxCfg config.MAXConfig) any {
	return &UserService{
		DB:      ctx.DB,
		Session: ctx.Session,
		QueryID: ctx.QueryID,
		SessCfg: sessCfg,
		MaxCfg:  maxCfg,
	}
}

func RegisterUserService(sessCfg config.SessionConfig, maxCfg config.MAXConfig) {
	webapp.MustRegisterService(
		"User",
		&UserService{},
		func(ctx webapp.ServiceContext) any {
			return NewUserService(ctx, sessCfg, maxCfg)
		},
		webapp.WithCRUDNotifications(),
	)
}

func (s *UserService) Create(ctx context.Context, input modelbind.ModelInput[*models.User]) (map[string]any, error) {
	if s.Session == nil {
		return nil, apperrors.SessionRequired()
	}
	if err := s.requireDB(); err != nil {
		return nil, err
	}
	if input.Model == nil {
		return nil, webapp.BadRequest("user input is required", nil)
	}
	if _, err := validateUserConstructionSiteIDs(
		input.Model.ConstructionSiteIDs,
		input.IsPresent("construction_site_ids"),
	); err != nil {
		return nil, err
	}

	input.Model.Pwd = hashUserPassword(input.Model.Pwd)

	if err := withPrimaryTransaction(ctx, s.DB, func(tx ds.Tx) error {
		if err := tx.QueryRow(ctx, `
			INSERT INTO public.users (
				name,
				role_id,
				pwd
			)
			VALUES ($1, $2, $3)
			RETURNING id
		`, input.Model.Name, input.Model.RoleID, input.Model.Pwd).Scan(&input.Model.ID); err != nil {
			return err
		}

		return syncUserConstructionSites(
			ctx,
			tx,
			input.Model.ID,
			input.Model.ConstructionSiteIDs,
		)
	}); err != nil {
		return nil, fmt.Errorf("insert user: %w", err)
	}

	return map[string]any{"id": input.Model.ID}, nil
}

func (s *UserService) Update(
	ctx context.Context,
	input webapp.UpdateByKeysInput[*models.UserKey, *models.UserUpdate],
) (wmodels.RowsAffectedResponse, error) {
	if s.Session == nil {
		return wmodels.RowsAffectedResponse{}, apperrors.SessionRequired()
	}
	if err := s.requireDB(); err != nil {
		return wmodels.RowsAffectedResponse{}, err
	}

	if input.Keys == nil {
		return wmodels.RowsAffectedResponse{}, webapp.BadRequest("user key is required", nil)
	}

	if input.Keys.ID <= 0 {
		return wmodels.RowsAffectedResponse{}, webapp.BadRequest("user id is required", nil)
	}
	if input.Input.Model == nil {
		return wmodels.RowsAffectedResponse{}, webapp.BadRequest("user input is required", nil)
	}

	namePresent := input.Input.IsPresent("name")
	rolePresent := input.Input.IsPresent("role_id")
	constructionSiteIDsPresent := input.Input.IsPresent("construction_site_ids")
	if !namePresent && !rolePresent && !constructionSiteIDsPresent {
		return wmodels.RowsAffectedResponse{}, webapp.BadRequest("user update is empty", nil)
	}
	if constructionSiteIDsPresent {
		if _, err := validateUserConstructionSiteIDs(
			input.Input.Model.ConstructionSiteIDs,
			true,
		); err != nil {
			return wmodels.RowsAffectedResponse{}, err
		}
	}

	err := withPrimaryTransaction(ctx, s.DB, func(tx ds.Tx) error {
		var lockedUserID int
		if err := tx.QueryRow(ctx, `
			SELECT id
			FROM public.users
			WHERE id = $1
			FOR UPDATE
		`, input.Keys.ID).Scan(&lockedUserID); err != nil {
			if errors.Is(err, ds.ErrNoRows) {
				return webapp.NotFound("user not found", map[string]any{
					"id": input.Keys.ID,
				})
			}
			return err
		}

		assignments := make([]string, 0, 2)
		args := []any{input.Keys.ID}
		if namePresent {
			args = append(args, input.Input.Model.Name)
			assignments = append(assignments, fmt.Sprintf("name = $%d", len(args)))
		}
		if rolePresent {
			args = append(args, input.Input.Model.RoleID)
			assignments = append(assignments, fmt.Sprintf("role_id = $%d", len(args)))
		}
		if len(assignments) > 0 {
			query := "UPDATE public.users SET " + strings.Join(assignments, ", ") + " WHERE id = $1"
			if _, err := tx.Exec(ctx, query, args...); err != nil {
				return err
			}
		}

		if constructionSiteIDsPresent {
			if err := syncUserConstructionSites(
				ctx,
				tx,
				input.Keys.ID,
				input.Input.Model.ConstructionSiteIDs,
			); err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return wmodels.RowsAffectedResponse{}, fmt.Errorf("update user %d: %w", input.Keys.ID, err)
	}

	return wmodels.RowsAffectedResponse{
		RowsAffected: 1,
	}, nil
}

func (s *UserService) Delete(ctx context.Context, id int) (wmodels.RowsAffectedResponse, error) {
	if s.Session == nil {
		return wmodels.RowsAffectedResponse{}, apperrors.SessionRequired()
	}
	if err := s.requireDB(); err != nil {
		return wmodels.RowsAffectedResponse{}, err
	}
	if id <= 0 {
		return wmodels.RowsAffectedResponse{}, webapp.BadRequest("user id is required", nil)
	}

	rowsAffected, err := webapp.DeleteModel(
		ctx,
		s.DB,
		[]types.DBModel{models.UserKey{ID: id}},
		nil,
	)
	if err != nil {
		return wmodels.RowsAffectedResponse{}, fmt.Errorf("delete user %d: %w", id, err)
	}
	if rowsAffected == 0 {
		return wmodels.RowsAffectedResponse{}, webapp.NotFound("user not found", map[string]any{"id": id})
	}

	return wmodels.RowsAffectedResponse{RowsAffected: rowsAffected}, nil
}

func (s *UserService) Detail(ctx context.Context, id int) (*models.UserDetail, error) {
	if s.Session == nil {
		return nil, apperrors.SessionRequired()
	}
	if err := s.requireDB(); err != nil {
		return nil, err
	}
	if id <= 0 {
		return nil, webapp.BadRequest("user id is required", nil)
	}

	poolConn, connID, err := s.DB.GetPrimary(ctx)
	if err != nil {
		return nil, fmt.Errorf("get primary connection for user detail: %w", err)
	}
	defer s.DB.Release(poolConn, connID)

	result, err := fetchUserDetail(ctx, poolConn.Conn(), id)
	if err != nil {
		if errors.Is(err, ds.ErrNoRows) {
			return nil, webapp.NotFound("user not found", map[string]any{"id": id})
		}
		return nil, fmt.Errorf("fetch user %d: %w", id, err)
	}

	return result, nil
}

func (s *UserService) List(ctx context.Context, params modelbind.CollectionParams) (wmodels.CollectionResponse[*models.UserList], error) {
	if s.Session == nil {
		return wmodels.CollectionResponse[*models.UserList]{}, apperrors.SessionRequired()
	}
	if err := s.requireDB(); err != nil {
		return wmodels.CollectionResponse[*models.UserList]{}, err
	}

	rows, total, err := webapp.FetchCollectionModel(ctx, s.DB, &models.UserList{}, params)
	if err != nil {
		return wmodels.CollectionResponse[*models.UserList]{}, fmt.Errorf("fetch user collection: %w", err)
	}

	return wmodels.CollectionResponse[*models.UserList]{
		Rows: rows,
		Agg:  total,
	}, nil
}

func fetchUserDetail(ctx context.Context, db ds.Querier, id int) (*models.UserDetail, error) {
	result := &models.UserDetail{}
	var constructionSiteIDs []int32
	if err := db.QueryRow(ctx, `
		SELECT
			u.id,
			u.name,
			u.role_id,
			COALESCE(
				array_agg(
					assignment.construction_site_id
					ORDER BY assignment.construction_site_id
				) FILTER (WHERE assignment.construction_site_id IS NOT NULL),
				ARRAY[]::integer[]
			) AS construction_site_ids
		FROM public.users AS u
		LEFT JOIN public.user_construction_sites AS assignment
			ON assignment.user_id = u.id
		WHERE u.id = $1
		GROUP BY u.id, u.name, u.role_id
	`, id).Scan(
		&result.ID,
		&result.Name,
		&result.RoleID,
		&constructionSiteIDs,
	); err != nil {
		return nil, err
	}

	result.ConstructionSiteIDs = make([]int, len(constructionSiteIDs))
	for index, constructionSiteID := range constructionSiteIDs {
		result.ConstructionSiteIDs[index] = int(constructionSiteID)
	}

	return result, nil
}

func validateUserConstructionSiteIDs(
	constructionSiteIDs []int,
	present bool,
) (map[int]struct{}, error) {
	if present && constructionSiteIDs == nil {
		return nil, webapp.BadRequest(
			"construction_site_ids should be an array",
			map[string]any{"field": "construction_site_ids"},
		)
	}

	return userConstructionSiteIDSet(constructionSiteIDs)
}

func userConstructionSiteIDSet(constructionSiteIDs []int) (map[int]struct{}, error) {
	result := make(map[int]struct{}, len(constructionSiteIDs))
	for index, constructionSiteID := range constructionSiteIDs {
		if constructionSiteID <= 0 {
			return nil, webapp.BadRequest(
				fmt.Sprintf("construction_site_ids[%d] should be positive", index),
				map[string]any{
					"field": "construction_site_ids",
					"index": index,
				},
			)
		}
		if _, exists := result[constructionSiteID]; exists {
			return nil, webapp.BadRequest(
				fmt.Sprintf("construction_site_ids[%d] is duplicated", index),
				map[string]any{
					"field":                "construction_site_ids",
					"index":                index,
					"construction_site_id": constructionSiteID,
				},
			)
		}
		result[constructionSiteID] = struct{}{}
	}

	return result, nil
}

func syncUserConstructionSites(
	ctx context.Context,
	db ds.Querier,
	userID int,
	constructionSiteIDs []int,
) error {
	desired, err := userConstructionSiteIDSet(constructionSiteIDs)
	if err != nil {
		return err
	}

	rows, err := db.Query(ctx, `
		SELECT construction_site_id
		FROM public.user_construction_sites
		WHERE user_id = $1
		ORDER BY construction_site_id
		FOR UPDATE
	`, userID)
	if err != nil {
		return fmt.Errorf("select construction sites for user %d: %w", userID, err)
	}

	existing := make(map[int]struct{})
	for rows.Next() {
		var constructionSiteID int
		if err := rows.Scan(&constructionSiteID); err != nil {
			_ = rows.Close()
			return fmt.Errorf("scan construction site for user %d: %w", userID, err)
		}
		existing[constructionSiteID] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return fmt.Errorf("iterate construction sites for user %d: %w", userID, err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close construction sites for user %d: %w", userID, err)
	}

	removeIDs, addIDs := userConstructionSiteChanges(existing, desired)
	for _, constructionSiteID := range removeIDs {
		if _, err := db.Exec(ctx, `
			DELETE FROM public.user_construction_sites
			WHERE user_id = $1
				AND construction_site_id = $2
		`, userID, constructionSiteID); err != nil {
			return fmt.Errorf(
				"delete construction site %d from user %d: %w",
				constructionSiteID,
				userID,
				err,
			)
		}
	}

	for _, constructionSiteID := range addIDs {
		if _, err := db.Exec(ctx, `
			INSERT INTO public.user_construction_sites (
				user_id,
				construction_site_id
			)
			VALUES ($1, $2)
		`, userID, constructionSiteID); err != nil {
			return fmt.Errorf(
				"add construction site %d to user %d: %w",
				constructionSiteID,
				userID,
				err,
			)
		}
	}

	return nil
}

func userConstructionSiteChanges(
	existing map[int]struct{},
	desired map[int]struct{},
) (removeIDs []int, addIDs []int) {
	for constructionSiteID := range existing {
		if _, keep := desired[constructionSiteID]; !keep {
			removeIDs = append(removeIDs, constructionSiteID)
		}
	}
	for constructionSiteID := range desired {
		if _, keep := existing[constructionSiteID]; !keep {
			addIDs = append(addIDs, constructionSiteID)
		}
	}

	sort.Ints(removeIDs)
	sort.Ints(addIDs)
	return removeIDs, addIDs
}

func (s *UserService) CurrentProfile(ctx context.Context) (*models.UserProfile, error) {
	currentUser, err := s.currentSessionUser()
	if err != nil {
		return nil, err
	}
	if err := s.requireDB(); err != nil {
		return nil, err
	}

	result, err := webapp.FetchModel(
		ctx,
		s.DB,
		models.UserKey{ID: currentUser.ID},
		&models.UserProfile{},
	)
	if err != nil {
		if errors.Is(err, ds.ErrNoRows) {
			return nil, webapp.NotFound("user not found", map[string]any{
				"id": currentUser.ID,
			})
		}
		return nil, fmt.Errorf("fetch current user profile: %w", err)
	}

	return result, nil
}

func (s *UserService) UpdateCurrentProfile(
	ctx context.Context,
	input models.UserProfileUpdate,
) (*models.UserProfile, error) {
	currentUser, err := s.currentSessionUser()
	if err != nil {
		return nil, err
	}
	if err := s.requireDB(); err != nil {
		return nil, err
	}

	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, webapp.BadRequest("user name is required", nil)
	}

	poolConn, connID, err := s.DB.GetPrimary(ctx)
	if err != nil {
		return nil, fmt.Errorf("get database connection: %w", err)
	}
	defer s.DB.Release(poolConn, connID)

	result := models.UserProfile{}
	err = poolConn.Conn().QueryRow(ctx, `
		UPDATE public.users
		SET name = $1
		WHERE id = $2
		RETURNING id, name
	`, name, currentUser.ID).Scan(
		&result.ID,
		&result.Name,
	)
	if err != nil {
		if errors.Is(err, ds.ErrNoRows) {
			return nil, webapp.NotFound("user not found", map[string]any{
				"id": currentUser.ID,
			})
		}
		return nil, fmt.Errorf("update current user profile: %w", err)
	}

	currentUser.Name = result.Name
	if err := s.Session.Set("user", currentUser); err != nil {
		return nil, fmt.Errorf("update session user: %w", err)
	}
	if err := s.Session.Flush(); err != nil {
		return nil, fmt.Errorf("flush session user: %w", err)
	}

	return &result, nil
}

func (s *UserService) ChangeCurrentPassword(
	ctx context.Context,
	input models.UserPasswordChange,
) (wmodels.RowsAffectedResponse, error) {
	currentUser, err := s.currentSessionUser()
	if err != nil {
		return wmodels.RowsAffectedResponse{}, err
	}
	if err := s.requireDB(); err != nil {
		return wmodels.RowsAffectedResponse{}, err
	}

	poolConn, connID, err := s.DB.GetPrimary(ctx)
	if err != nil {
		return wmodels.RowsAffectedResponse{}, fmt.Errorf("get database connection: %w", err)
	}
	defer s.DB.Release(poolConn, connID)

	tag, err := poolConn.Conn().Exec(ctx, `
		UPDATE public.users
		SET pwd = $1
		WHERE id = $2
	`, hashUserPassword(input.NewPassword), currentUser.ID)
	if err != nil {
		return wmodels.RowsAffectedResponse{}, fmt.Errorf("change current user password: %w", err)
	}

	rowsAffected := tag.RowsAffected()
	if rowsAffected == 0 {
		return wmodels.RowsAffectedResponse{}, webapp.NotFound("user not found", map[string]any{
			"id": currentUser.ID,
		})
	}

	return wmodels.RowsAffectedResponse{
		RowsAffected: rowsAffected,
	}, nil
}

func (s *UserService) Login(
	ctx context.Context,
	input models.UserLoginInput,
) (models.UserLoginResponse, error) {
	if s.Session == nil {
		return models.UserLoginResponse{}, apperrors.SessionRequired()
	}
	if err := s.requireDB(); err != nil {
		return models.UserLoginResponse{}, err
	}

	poolConn, connID, err := s.DB.GetPrimary(ctx)
	if err != nil {
		return models.UserLoginResponse{}, err
	}
	defer s.DB.Release(poolConn, connID)
	conn := poolConn.Conn()

	userRow := models.UserLogin{}
	if _, err := conn.Prepare(ctx,
		userLoginQuery,
		`SELECT
			u.id,
			u.name,
			u.role_id,
			u.create_dt,
			u.pwd,
			COALESCE(u.banned, false),
			(
				SELECT string_agg(b.hash, ',')
				FROM public.login_device_bans AS b
				WHERE b.user_id = u.id
			) AS ban_hash
		FROM public.users AS u
		WHERE u.name = $1 AND u.pwd = md5($2)`,
	); err != nil {
		return models.UserLoginResponse{}, err
	}

	err = conn.QueryRow(ctx, userLoginQuery, input.Model.Name, input.Model.Pwd).Scan(
		&userRow.ID,
		&userRow.Name,
		&userRow.RoleID,
		&userRow.CreateDt,
		&userRow.Pwd,
		&userRow.Banned,
		&userRow.BanHash,
	)
	if err == ds.ErrNoRows {
		// no user with this name &&  pwd
		return models.UserLoginResponse{}, apperrors.InvalidCredentials()
	} else if err != nil {
		return models.UserLoginResponse{}, err
	}

	slog.Debug("UserService.Login, calling loginUser()")
	if err := loginUser(ctx, s.Session, input.UserInf, conn, &userRow); err != nil {
		return models.UserLoginResponse{}, err
	}

	tokenRefresh := ""
	tokenExpires := time.Time{}
	if s.SessCfg.MaxLifeTime > 0 {
		tokenExpires = time.Now().Add(time.Duration(s.SessCfg.MaxLifeTime) * time.Second)
	}

	return models.UserLoginResponse{
		User: &userRow,
		Auth: &wmodels.Auth{
			Token:        s.Session.SessionID(),
			TokenRefresh: tokenRefresh,
			Expires:      tokenExpires,
		},
	}, nil
}

func (s *UserService) Logout(ctx context.Context) error {
	userLogin := models.UserLogin{}
	if err := s.Session.Get("user", &userLogin); err != nil {
		return fmt.Errorf("Session.Get() failed: %v", err)
	}

	poolConn, connID, err := s.DB.GetPrimary(ctx)
	if err != nil {
		return fmt.Errorf("GetPrimary() failed: %v", err)
	}
	defer s.DB.Release(poolConn, connID)
	conn := poolConn.Conn()

	if _, err := conn.Prepare(ctx,
		"user_logout",
		`UPDATE logins 
		SET date_time_out = now() 
		WHERE id = $1`,
	); err != nil {
		return fmt.Errorf("conn.Prepare() failed: %v", err)
	}

	if _, err := conn.Exec(ctx, "BEGIN"); err != nil {
		return fmt.Errorf("conn.Exec() BEGIN failed: %v", err)
	}
	if _, err := conn.Exec(ctx, "user_logout", userLogin.LoginID); err != nil {
		_, _ = conn.Exec(ctx, "ROLLBACK")
		return fmt.Errorf("conn.Exec() failed: %v", err)
	}
	if err := s.Session.Delete(s.Session.SessionID()); err != nil {
		_, _ = conn.Exec(ctx, "ROLLBACK")
		return fmt.Errorf("Session.Delete() failed: %v", err)
	}

	if _, err := conn.Exec(ctx, "COMMIT"); err != nil {
		return fmt.Errorf("conn.Exec() COMMIT failed: %v", err)
	}

	return nil
}

func (s *UserService) currentSessionUser() (models.UserLogin, error) {
	if s.Session == nil {
		return models.UserLogin{}, apperrors.SessionRequired()
	}

	user := models.UserLogin{}
	if err := s.Session.Get("user", &user); err != nil || user.ID <= 0 {
		return models.UserLogin{}, apperrors.SessionRequired()
	}

	return user, nil
}

func (s *UserService) requireDB() error {
	if s.DB == nil {
		return webapp.Internal("database is not initialized", nil)
	}

	return nil
}

func hashUserPassword(password string) string {
	sum := md5.Sum([]byte(password))
	return hex.EncodeToString(sum[:])
}
