package services

import (
	"context"
	"errors"
	cc "fairchild-admin-panel-be/internal/constants"
	"fairchild-admin-panel-be/internal/interfaces"
	models "fairchild-admin-panel-be/internal/models/account"
	audit "fairchild-admin-panel-be/internal/models/audit"
	"testing"
)

type denyPassword struct {
	calls int
	id    int64
}

func (p *denyPassword) ConfirmPassword(_ context.Context, id int64, _, _, _ string) error {
	p.calls++
	p.id = id
	return cc.ErrInvalidAccessToken
}

type missingAccount struct {
	interfaces.AccountRepositoryInterface
}

func (missingAccount) FindAccount(context.Context, int64) (*models.Account, error) { return nil, nil }

func TestSecurityAccountPasswordRequired(t *testing.T) {
	p := &denyPassword{}
	s := NewAccountService(nil, p)
	actor := audit.Actor{UserPanelID: 42}
	if _, err := s.UpdateProfile(context.Background(), &models.UpdateProfileRequest{FirstName: "A", LastName: "B"}, actor, "test"); !errors.Is(err, cc.ErrInvalidAccessToken) {
		t.Fatal(err)
	}
	if err := s.ChangePassword(context.Background(), &models.ChangePasswordRequest{CurrentPassword: "old password", NewPassword: "new password long enough"}, actor, "test"); !errors.Is(err, cc.ErrInvalidAccessToken) {
		t.Fatal(err)
	}
	if p.calls != 2 || p.id != 42 {
		t.Fatal("password check did not use authenticated actor")
	}
}
func TestAccountPasswordPolicyAndMissingAccount(t *testing.T) {
	s := NewAccountService(missingAccount{}, nil)
	if _, err := s.Get(context.Background(), 42); !errors.Is(err, cc.ErrInvalidAccessToken) {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		req  models.ChangePasswordRequest
		want error
	}{
		{models.ChangePasswordRequest{NewPassword: "short"}, cc.ErrPasswordTooShort},
		{models.ChangePasswordRequest{CurrentPassword: "same password 123", NewPassword: "same password 123"}, cc.ErrPasswordUnchanged},
	} {
		if err := s.ChangePassword(context.Background(), &tc.req, audit.Actor{}, ""); !errors.Is(err, tc.want) {
			t.Fatal(err)
		}
	}
}
func BenchmarkAccount(b *testing.B) {
	s := NewAccountService(missingAccount{}, nil)
	b.ReportAllocs()
	for b.Loop() {
		if _, err := s.Get(context.Background(), 42); !errors.Is(err, cc.ErrInvalidAccessToken) {
			b.Fatal(err)
		}
	}
}
