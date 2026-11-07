package services

import (
	"context"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	cc "fairchild-admin-panel-be/internal/constants"
	interfaces "fairchild-admin-panel-be/internal/interfaces"
	audit_models "fairchild-admin-panel-be/internal/models/audit"
	auth_models "fairchild-admin-panel-be/internal/models/auth"
	models "fairchild-admin-panel-be/internal/models/capabilities"
	users "fairchild-admin-panel-be/internal/models/users"
)

// cacheTTL bounds how long another instance of this service keeps using an
// account's capabilities after an admin changes them. A save on this
// instance takes effect immediately.
const cacheTTL = 10 * time.Second

// auditTargetMax is admin_audit_logs.target's length.
const auditTargetMax = 100

type cached struct {
	keys     map[string]bool
	loadedAt time.Time
}

type CapabilitiesService struct {
	repo  interfaces.CapabilitiesRepositoryInterface
	users interfaces.UsersRepositoryInterface

	mu    sync.Mutex
	cache map[int64]cached // by users_panel.id
}

func NewCapabilitiesService(repo interfaces.CapabilitiesRepositoryInterface, users interfaces.UsersRepositoryInterface) *CapabilitiesService {
	return &CapabilitiesService{repo: repo, users: users, cache: map[int64]cached{}}
}

func (s *CapabilitiesService) assigned(ctx context.Context, userPanelID int64) (map[string]bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if c, ok := s.cache[userPanelID]; ok && time.Since(c.loadedAt) < cacheTTL {
		return c.keys, nil
	}
	keys, err := s.repo.FindUserCapabilities(ctx, userPanelID)
	if err != nil {
		return nil, err
	}
	set := make(map[string]bool, len(keys))
	for _, k := range keys {
		set[k] = true
	}
	s.cache[userPanelID] = cached{keys: set, loadedAt: time.Now()}
	return set, nil
}

func (s *CapabilitiesService) forget(userPanelID int64) {
	s.mu.Lock()
	delete(s.cache, userPanelID)
	s.mu.Unlock()
}

// Allows reports whether user may use capability key. Super admins and
// admins may use everything. Managers, supervisors and staff need key, and
// its page's view key, assigned to their account.
func (s *CapabilitiesService) Allows(ctx context.Context, user *auth_models.UserPanel, key string) (bool, error) {
	if users.IsAdmin(user.Role) {
		return true, nil
	}
	if !users.HasAssignedCapabilities(user.Role) {
		return false, nil
	}
	keys, err := s.assigned(ctx, user.ID)
	if err != nil {
		return false, err
	}
	return keys[key] && keys[models.ViewKey(key)], nil
}

// allowedKeys lists, in catalog order, the capabilities an account with role
// and assigned keys can actually use.
func allowedKeys(role string, assigned map[string]bool) []string {
	keys := []string{}
	for _, c := range models.Catalog {
		if users.IsAdmin(role) || (users.HasAssignedCapabilities(role) && assigned[c.Key] && assigned[models.ViewKey(c.Key)]) {
			keys = append(keys, c.Key)
		}
	}
	return keys
}

// Mine lists the capabilities the signed-in user has.
func (s *CapabilitiesService) Mine(ctx context.Context, user *auth_models.UserPanel) (*models.MyCapabilities, error) {
	assigned := map[string]bool{}
	if users.HasAssignedCapabilities(user.Role) {
		var err error
		if assigned, err = s.assigned(ctx, user.ID); err != nil {
			return nil, err
		}
	}
	return &models.MyCapabilities{Role: user.Role, Capabilities: allowedKeys(user.Role, assigned)}, nil
}

// Catalog is every capability with its role preset.
func (s *CapabilitiesService) Catalog() *models.CatalogResponse {
	return &models.CatalogResponse{Items: models.Catalog}
}

// ForUser is one account's capabilities: every key for an admin.
func (s *CapabilitiesService) ForUser(ctx context.Context, id string) (*models.UserCapabilities, error) {
	target, err := s.users.FindUser(ctx, id)
	if err != nil {
		return nil, err
	}
	keys, err := s.repo.FindUserCapabilities(ctx, target.InternalID)
	if err != nil {
		return nil, err
	}
	assigned := map[string]bool{}
	for _, k := range keys {
		assigned[k] = true
	}
	return &models.UserCapabilities{User: *target, Capabilities: allowedKeys(target.Role, assigned)}, nil
}

// UpdateForUser replaces a manager's, supervisor's or staff account's
// capabilities. by must outrank the account (users.CanManage). Actions
// without their page's view key are dropped. Only changes are written, the
// audit entry names them, and nothing is written when nothing changed.
func (s *CapabilitiesService) UpdateForUser(ctx context.Context, by *auth_models.UserPanel, actor audit_models.Actor, id string, req *models.UpdateUserCapabilitiesRequest) (*models.UserCapabilities, error) {
	for _, key := range req.Capabilities {
		if _, ok := models.Find(key); !ok {
			return nil, cc.ErrUnknownCapability
		}
	}
	target, err := s.users.FindUser(ctx, id)
	if err != nil {
		return nil, err
	}
	if !users.CanManage(by.ID, by.Role, target.InternalID, target.Role) {
		return nil, cc.ErrCannotManagePanelUser
	}
	if !users.HasAssignedCapabilities(target.Role) {
		return nil, cc.ErrCapabilitiesNotEditable
	}

	current, err := s.repo.FindUserCapabilities(ctx, target.InternalID)
	if err != nil {
		return nil, err
	}
	var add, remove, granted, removed []string
	for _, c := range models.Catalog {
		want := slices.Contains(req.Capabilities, c.Key) && slices.Contains(req.Capabilities, models.ViewKey(c.Key))
		has := slices.Contains(current, c.Key)
		name := c.Page + ": " + c.Label
		switch {
		case want && !has:
			add = append(add, c.Key)
			granted = append(granted, name)
		case !want && has:
			remove = append(remove, c.Key)
			removed = append(removed, name)
		}
	}

	if len(add) > 0 || len(remove) > 0 {
		audit := actor.Log(cc.AUDIT_UPDATED_USER_CAPABILITIES, auditTarget(target.Username, granted, removed))
		if err := s.repo.SaveUserCapabilities(ctx, target.InternalID, add, remove, by.ID, audit); err != nil {
			return nil, err
		}
		s.forget(target.InternalID)
	}
	return s.ForUser(ctx, id)
}

// auditTarget summarises a change for the audit log, e.g.
// "jdoe: +Locked Accounts: Unlock; -Metrics: View page", cut to fit.
func auditTarget(username string, granted, removed []string) string {
	var parts []string
	for _, g := range granted {
		parts = append(parts, "+"+g)
	}
	for _, r := range removed {
		parts = append(parts, "-"+r)
	}
	target := username + ": " + strings.Join(parts, "; ")
	if utf8.RuneCountInString(target) <= auditTargetMax {
		return target
	}
	return string([]rune(target)[:auditTargetMax-1]) + "…"
}
