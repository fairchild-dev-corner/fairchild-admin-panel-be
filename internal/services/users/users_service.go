package services

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	cc "fairchild-admin-panel-be/internal/constants"
	interfaces "fairchild-admin-panel-be/internal/interfaces"
	audit_models "fairchild-admin-panel-be/internal/models/audit"
	auth_models "fairchild-admin-panel-be/internal/models/auth"
	caps "fairchild-admin-panel-be/internal/models/capabilities"
	models "fairchild-admin-panel-be/internal/models/users"
	auth_service "fairchild-admin-panel-be/internal/services/auth"
	"fairchild-admin-panel-be/internal/utils"

	"github.com/google/uuid"
)

// Usernames are matched after auth_service.NormalizeUsername (lowercased).
var usernamePattern = regexp.MustCompile(`^[a-z0-9._-]{3,50}$`)

type UsersService struct {
	repo interfaces.UsersRepositoryInterface
}

func NewUsersService(repo interfaces.UsersRepositoryInterface) *UsersService {
	return &UsersService{repo: repo}
}

func (s *UsersService) List(ctx context.Context) ([]models.PanelUser, error) {
	return s.repo.ListUsers(ctx)
}

// Create adds an active panel account with the password the admin chose,
// under the same rules as cmd/create_panel_user. by may only give a role
// below their own (users.CanAssign). A manager, supervisor or staff account
// starts with its role's preset capabilities.
func (s *UsersService) Create(ctx context.Context, by *auth_models.UserPanel, req *models.CreatePanelUserRequest, actor audit_models.Actor) (*models.PanelUser, error) {
	if !models.CanAssign(by.Role, req.Role) {
		return nil, cc.ErrRoleNotAssignable
	}
	username := auth_service.NormalizeUsername(req.Username)
	if !usernamePattern.MatchString(username) {
		return nil, cc.ErrInvalidUsername
	}
	firstName, lastName := strings.TrimSpace(req.FirstName), strings.TrimSpace(req.LastName)
	if firstName == "" || lastName == "" {
		return nil, cc.ErrInvalidPayload
	}
	if utf8.RuneCountInString(req.Password) < auth_service.MinPasswordLength {
		return nil, cc.ErrPasswordTooShort
	}

	hash, err := auth_service.HashPassword(req.Password)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}
	return s.repo.CreateUser(ctx, &auth_models.UserPanel{
		UUID:         uuid.NewString(),
		Username:     username,
		PasswordHash: hash,
		FirstName:    firstName,
		LastName:     lastName,
		Email:        utils.NullIfEmpty(strings.TrimSpace(req.Email)),
		Role:         req.Role,
		Status:       cc.USER_STATUS_ACTIVE,
	}, caps.Preset(req.Role), actor.Log(cc.AUDIT_ADDED_PANEL_USER, fmt.Sprintf("%s (%s)", username, req.Role)))
}

// manageable finds the account by may change: one below their role, never
// their own.
func (s *UsersService) manageable(ctx context.Context, by *auth_models.UserPanel, id string) (*models.PanelUser, error) {
	target, err := s.repo.FindUser(ctx, id)
	if err != nil {
		return nil, err
	}
	if !models.CanManage(by.ID, by.Role, target.InternalID, target.Role) {
		return nil, cc.ErrCannotManagePanelUser
	}
	return target, nil
}

// UpdateRole moves an account within the hierarchy. by must outrank both
// its current and its new role. Its assigned capabilities are kept (they
// only apply below admin).
func (s *UsersService) UpdateRole(ctx context.Context, by *auth_models.UserPanel, id string, req *models.UpdateRoleRequest, actor audit_models.Actor) (*models.PanelUser, error) {
	target, err := s.manageable(ctx, by, id)
	if err != nil {
		return nil, err
	}
	if !models.CanAssign(by.Role, req.Role) {
		return nil, cc.ErrRoleNotAssignable
	}
	if target.Role == req.Role {
		return target, nil
	}
	audit := actor.Log(cc.AUDIT_CHANGED_PANEL_USER_ROLE, fmt.Sprintf("%s: %s to %s", target.Username, target.Role, req.Role))
	return s.repo.UpdateRole(ctx, target.InternalID, req.Role, audit)
}

// UpdateStatus suspends (and signs out everywhere) or reactivates an
// account by outranks.
func (s *UsersService) UpdateStatus(ctx context.Context, by *auth_models.UserPanel, id string, req *models.UpdateStatusRequest, actor audit_models.Actor) (*models.PanelUser, error) {
	target, err := s.manageable(ctx, by, id)
	if err != nil {
		return nil, err
	}
	if target.Status == req.Status {
		return target, nil
	}
	action := cc.AUDIT_REACTIVATED_PANEL_USER
	if req.Status == cc.USER_STATUS_SUSPENDED {
		action = cc.AUDIT_SUSPENDED_PANEL_USER
	}
	return s.repo.UpdateStatus(ctx, target.InternalID, req.Status, actor.Log(action, target.Username))
}
