package services

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	cc "fairchild-admin-panel-be/internal/constants"
	audit_models "fairchild-admin-panel-be/internal/models/audit"
	auth_models "fairchild-admin-panel-be/internal/models/auth"
	models "fairchild-admin-panel-be/internal/models/capabilities"
	users "fairchild-admin-panel-be/internal/models/users"
)

// fakeCapabilitiesRepo is an in-memory CapabilitiesRepositoryInterface.
type fakeCapabilitiesRepo struct {
	keys   map[int64][]string
	finds  int
	saves  int
	audits []*audit_models.AuditLog
}

func (r *fakeCapabilitiesRepo) FindUserCapabilities(_ context.Context, id int64) ([]string, error) {
	r.finds++
	return slices.Clone(r.keys[id]), nil
}

func (r *fakeCapabilitiesRepo) SaveUserCapabilities(_ context.Context, id int64, add, remove []string, _ int64, audit *audit_models.AuditLog) error {
	r.saves++
	r.keys[id] = slices.DeleteFunc(r.keys[id], func(k string) bool { return slices.Contains(remove, k) })
	r.keys[id] = append(r.keys[id], add...)
	r.audits = append(r.audits, audit)
	return nil
}

// fakeUsersRepo serves FindUser only.
type fakeUsersRepo struct{ byUUID map[string]users.PanelUser }

func (r *fakeUsersRepo) FindUser(_ context.Context, id string) (*users.PanelUser, error) {
	u, ok := r.byUUID[id]
	if !ok {
		return nil, cc.ErrPanelUserNotFound
	}
	return &u, nil
}
func (r *fakeUsersRepo) ListUsers(context.Context) ([]users.PanelUser, error) { return nil, nil }
func (r *fakeUsersRepo) CreateUser(context.Context, *auth_models.UserPanel, []string, *audit_models.AuditLog) (*users.PanelUser, error) {
	return nil, nil
}
func (r *fakeUsersRepo) UpdateRole(context.Context, int64, string, *audit_models.AuditLog) (*users.PanelUser, error) {
	return nil, nil
}
func (r *fakeUsersRepo) UpdateStatus(context.Context, int64, string, *audit_models.AuditLog) (*users.PanelUser, error) {
	return nil, nil
}

// Accounts by users_panel.id: 1 super admin, 2 admin, 3 manager, 4 staff.
func newTestService(keys map[int64][]string) (*CapabilitiesService, *fakeCapabilitiesRepo) {
	repo := &fakeCapabilitiesRepo{keys: keys}
	ur := &fakeUsersRepo{byUUID: map[string]users.PanelUser{
		"u-1": {InternalID: 1, ID: "u-1", Username: "root", Role: cc.ROLE_SUPER_ADMIN},
		"u-2": {InternalID: 2, ID: "u-2", Username: "boss", Role: cc.ROLE_ADMIN},
		"u-3": {InternalID: 3, ID: "u-3", Username: "mgr", Role: cc.ROLE_MANAGER},
		"u-4": {InternalID: 4, ID: "u-4", Username: "jdoe", Role: cc.ROLE_STAFF},
	}}
	return NewCapabilitiesService(repo, ur), repo
}

func panelUser(id int64, role string) *auth_models.UserPanel {
	return &auth_models.UserPanel{ID: id, Role: role}
}

func allows(t *testing.T, s *CapabilitiesService, u *auth_models.UserPanel, key string) bool {
	t.Helper()
	ok, err := s.Allows(context.Background(), u, key)
	if err != nil {
		t.Fatalf("Allows(%s, %s): %v", u.Role, key, err)
	}
	return ok
}

func TestAllowsByRoleAndAssignment(t *testing.T) {
	s, _ := newTestService(map[int64][]string{
		3: {models.LocksView, models.LocksUnlock},
		4: {models.LocksUnlock, models.MetricsView}, // unlock without its view key
	})

	for _, c := range models.Catalog {
		for _, role := range []string{cc.ROLE_SUPER_ADMIN, cc.ROLE_ADMIN} {
			if !allows(t, s, panelUser(9, role), c.Key) {
				t.Errorf("%s should have %s", role, c.Key)
			}
		}
		if allows(t, s, panelUser(4, "member"), c.Key) {
			t.Errorf("an unknown role should have nothing, got %s", c.Key)
		}
	}

	manager, staff := panelUser(3, cc.ROLE_MANAGER), panelUser(4, cc.ROLE_STAFF)
	if !allows(t, s, manager, models.LocksUnlock) || !allows(t, s, manager, models.LocksView) {
		t.Error("manager should have their assigned locks capabilities")
	}
	if allows(t, s, manager, models.MetricsView) {
		t.Error("manager wasn't assigned metrics")
	}
	if allows(t, s, staff, models.LocksUnlock) {
		t.Error("an action needs its page's view capability too")
	}
	if !allows(t, s, staff, models.MetricsView) {
		t.Error("staff should have metrics")
	}
	// Same role, different capabilities: a supervisor with id 3's grants.
	if !allows(t, s, panelUser(3, cc.ROLE_SUPERVISOR), models.LocksUnlock) {
		t.Error("capabilities follow the account, not the role")
	}
}

func TestMineListsUsableKeys(t *testing.T) {
	s, _ := newTestService(map[int64][]string{4: {models.LocksUnlock, models.MetricsView}})
	mine, err := s.Mine(context.Background(), panelUser(4, cc.ROLE_STAFF))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(mine.Capabilities, []string{models.MetricsView}) {
		t.Errorf("staff = %v", mine.Capabilities)
	}
	mine, _ = s.Mine(context.Background(), panelUser(1, cc.ROLE_SUPER_ADMIN))
	if len(mine.Capabilities) != len(models.Catalog) {
		t.Errorf("super admin has %d of %d", len(mine.Capabilities), len(models.Catalog))
	}
}

func TestUpdateForUser(t *testing.T) {
	ctx := context.Background()
	s, repo := newTestService(map[int64][]string{4: {models.MetricsView}})
	admin := panelUser(2, cc.ROLE_ADMIN)
	actor := audit_models.Actor{UserPanelID: 2, Name: "Boss"}

	// Warm the cache, then change: the save must take effect at once.
	if allows(t, s, panelUser(4, cc.ROLE_STAFF), models.LocksView) {
		t.Fatal("staff shouldn't have locks yet")
	}
	res, err := s.UpdateForUser(ctx, admin, actor, "u-4", &models.UpdateUserCapabilitiesRequest{
		Capabilities: []string{models.LocksView, models.LocksUnlock, models.BlastsSend}, // blasts.send without blasts.view
	})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(res.Capabilities, []string{models.LocksView, models.LocksUnlock}) {
		t.Errorf("saved %v", res.Capabilities)
	}
	if !allows(t, s, panelUser(4, cc.ROLE_STAFF), models.LocksUnlock) {
		t.Error("the save should clear the cache")
	}
	if repo.saves != 1 || len(repo.audits) != 1 {
		t.Fatalf("saves %d, audits %d", repo.saves, len(repo.audits))
	}
	if a := repo.audits[0]; a.Action != cc.AUDIT_UPDATED_USER_CAPABILITIES || !strings.HasPrefix(a.Target, "jdoe: +Locked Accounts: View page") || !strings.Contains(a.Target, "-Metrics: View page") {
		t.Errorf("audit %q %q", a.Action, a.Target)
	}

	// Same set again: nothing written.
	if _, err := s.UpdateForUser(ctx, admin, actor, "u-4", &models.UpdateUserCapabilitiesRequest{
		Capabilities: []string{models.LocksView, models.LocksUnlock},
	}); err != nil || repo.saves != 1 {
		t.Errorf("unchanged save: err %v, saves %d", err, repo.saves)
	}
}

func TestUpdateForUserRefusals(t *testing.T) {
	ctx := context.Background()
	s, repo := newTestService(map[int64][]string{})
	req := &models.UpdateUserCapabilitiesRequest{Capabilities: []string{models.LocksView}}
	tests := []struct {
		name string
		by   *auth_models.UserPanel
		id   string
		req  *models.UpdateUserCapabilitiesRequest
		want error
	}{
		{"unknown key", panelUser(2, cc.ROLE_ADMIN), "u-4", &models.UpdateUserCapabilitiesRequest{Capabilities: []string{"no.such"}}, cc.ErrUnknownCapability},
		{"unknown user", panelUser(2, cc.ROLE_ADMIN), "u-9", req, cc.ErrPanelUserNotFound},
		{"admin can't change an admin", panelUser(5, cc.ROLE_ADMIN), "u-2", req, cc.ErrCannotManagePanelUser},
		{"nobody changes their own", panelUser(3, cc.ROLE_SUPER_ADMIN), "u-3", req, cc.ErrCannotManagePanelUser},
		{"admins have everything", panelUser(1, cc.ROLE_SUPER_ADMIN), "u-2", req, cc.ErrCapabilitiesNotEditable},
		{"manager can't assign", panelUser(3, cc.ROLE_MANAGER), "u-4", req, cc.ErrCannotManagePanelUser},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := s.UpdateForUser(ctx, tt.by, audit_models.Actor{}, tt.id, tt.req)
			if !errors.Is(err, tt.want) {
				t.Errorf("err %v, want %v", err, tt.want)
			}
		})
	}
	if repo.saves != 0 {
		t.Errorf("a refused update saved %d times", repo.saves)
	}
}

func TestPresetsGrowWithRank(t *testing.T) {
	staff, supervisor, manager := models.Preset(cc.ROLE_STAFF), models.Preset(cc.ROLE_SUPERVISOR), models.Preset(cc.ROLE_MANAGER)
	for _, k := range staff {
		if !slices.Contains(supervisor, k) {
			t.Errorf("supervisor preset lacks staff's %s", k)
		}
	}
	for _, k := range supervisor {
		if !slices.Contains(manager, k) {
			t.Errorf("manager preset lacks supervisor's %s", k)
		}
	}
	if len(staff) >= len(supervisor) || len(supervisor) >= len(manager) {
		t.Errorf("presets should grow: %d, %d, %d", len(staff), len(supervisor), len(manager))
	}
	for _, role := range []string{cc.ROLE_STAFF, cc.ROLE_SUPERVISOR, cc.ROLE_MANAGER} {
		p := models.Preset(role)
		for _, k := range p {
			if !slices.Contains(p, models.ViewKey(k)) {
				t.Errorf("%s preset has %s without its page", role, k)
			}
		}
		if slices.Contains(p, models.MaintenanceManage) || slices.Contains(p, models.ElectionControl) {
			t.Errorf("%s preset includes a by-hand capability", role)
		}
	}
	if len(models.Preset(cc.ROLE_ADMIN)) != 0 || len(models.Preset(cc.ROLE_SUPER_ADMIN)) != 0 {
		t.Error("admins have no preset")
	}
}
