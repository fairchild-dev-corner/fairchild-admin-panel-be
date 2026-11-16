package services

import (
	"context"
	"errors"
	cc "fairchild-admin-panel-be/internal/constants"
	"fairchild-admin-panel-be/internal/interfaces"
	audit "fairchild-admin-panel-be/internal/models/audit"
	auth "fairchild-admin-panel-be/internal/models/auth"
	models "fairchild-admin-panel-be/internal/models/users"
	"testing"
)

type managementRepo struct {
	interfaces.UsersRepositoryInterface
	target models.PanelUser
	writes int
}

func (r *managementRepo) FindUser(context.Context, string) (*models.PanelUser, error) {
	return &r.target, nil
}
func (r *managementRepo) UpdateRole(_ context.Context, _ int64, role string, a *audit.AuditLog) (*models.PanelUser, error) {
	r.writes++
	r.target.Role = role
	return &r.target, nil
}
func TestSecurityUsersCannotEscalateOrManageSelf(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		name, actorRole, targetRole string
		actorID, targetID           int64
	}{
		{"self", cc.ROLE_SUPER_ADMIN, cc.ROLE_SUPER_ADMIN, 1, 1},
		{"peer", cc.ROLE_ADMIN, cc.ROLE_ADMIN, 1, 2},
		{"higher", cc.ROLE_ADMIN, cc.ROLE_SUPER_ADMIN, 1, 2},
		{"staff", cc.ROLE_STAFF, cc.ROLE_STAFF, 1, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := &managementRepo{target: models.PanelUser{InternalID: tc.targetID, Role: tc.targetRole}}
			_, err := NewUsersService(r).UpdateRole(ctx, &auth.UserPanel{ID: tc.actorID, Role: tc.actorRole}, "target", &models.UpdateRoleRequest{Role: cc.ROLE_STAFF}, audit.Actor{})
			if !errors.Is(err, cc.ErrCannotManagePanelUser) || r.writes != 0 {
				t.Fatalf("err=%v writes=%d", err, r.writes)
			}
		})
	}
	r := &managementRepo{target: models.PanelUser{InternalID: 2, Role: cc.ROLE_STAFF}}
	_, err := NewUsersService(r).UpdateRole(ctx, &auth.UserPanel{ID: 1, Role: cc.ROLE_ADMIN}, "target", &models.UpdateRoleRequest{Role: cc.ROLE_SUPER_ADMIN}, audit.Actor{})
	if !errors.Is(err, cc.ErrRoleNotAssignable) || r.writes != 0 {
		t.Fatal("role escalation allowed", err)
	}
}
func TestUserCreateValidationBeforePersistence(t *testing.T) {
	s := NewUsersService(nil)
	by := &auth.UserPanel{Role: cc.ROLE_ADMIN}
	for _, tc := range []struct {
		req  models.CreatePanelUserRequest
		want error
	}{
		{models.CreatePanelUserRequest{Role: cc.ROLE_ADMIN}, cc.ErrRoleNotAssignable},
		{models.CreatePanelUserRequest{Role: cc.ROLE_STAFF, Username: "bad name"}, cc.ErrInvalidUsername},
		{models.CreatePanelUserRequest{Role: cc.ROLE_STAFF, Username: "staff", FirstName: " ", LastName: "Test"}, cc.ErrInvalidPayload},
		{models.CreatePanelUserRequest{Role: cc.ROLE_STAFF, Username: "staff", FirstName: "A", LastName: "B", Password: "short"}, cc.ErrPasswordTooShort},
	} {
		if _, err := s.Create(context.Background(), by, &tc.req, audit.Actor{}); !errors.Is(err, tc.want) {
			t.Fatalf("got %v want %v", err, tc.want)
		}
	}
}
func BenchmarkUsers(b *testing.B) {
	r := &managementRepo{target: models.PanelUser{InternalID: 2, Role: cc.ROLE_STAFF}}
	s := NewUsersService(r)
	by := &auth.UserPanel{ID: 1, Role: cc.ROLE_ADMIN}
	req := &models.UpdateRoleRequest{Role: cc.ROLE_STAFF}
	b.ReportAllocs()
	for b.Loop() {
		if _, err := s.UpdateRole(context.Background(), by, "target", req, audit.Actor{}); err != nil {
			b.Fatal(err)
		}
	}
}
