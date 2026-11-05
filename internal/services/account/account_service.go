package services

import (
	"context"
	"strings"
	"unicode/utf8"

	cc "fairchild-admin-panel-be/internal/constants"
	interfaces "fairchild-admin-panel-be/internal/interfaces"
	models "fairchild-admin-panel-be/internal/models/account"
	audit_models "fairchild-admin-panel-be/internal/models/audit"
	auth_service "fairchild-admin-panel-be/internal/services/auth"
)

const (
	signInLimit   = 10
	activityLimit = 15
)

type AccountService struct {
	repo      interfaces.AccountRepositoryInterface
	passwords interfaces.PasswordConfirmerInterface
}

func NewAccountService(repo interfaces.AccountRepositoryInterface, passwords interfaces.PasswordConfirmerInterface) *AccountService {
	return &AccountService{repo: repo, passwords: passwords}
}

// Get builds the signed-in user's Account page: their profile, where they're
// still signed in, recent sign-ins and their recent actions.
func (s *AccountService) Get(ctx context.Context, userPanelID int64) (*models.Account, error) {
	a, err := s.repo.FindAccount(ctx, userPanelID)
	if err != nil {
		return nil, err
	}
	if a == nil {
		return nil, cc.ErrInvalidAccessToken
	}
	if a.Sessions, err = s.repo.ListActiveSessions(ctx, userPanelID); err != nil {
		return nil, err
	}
	if a.SignIns, err = s.repo.ListSignIns(ctx, userPanelID, a.Username, signInLimit); err != nil {
		return nil, err
	}
	if a.Activity, a.ActivityCount, err = s.repo.ListActivity(ctx, userPanelID, activityLimit); err != nil {
		return nil, err
	}
	return a, nil
}

// UpdateProfile changes the user's own name and email once their current
// password checks out (a wrong one counts toward the login lockout).
func (s *AccountService) UpdateProfile(ctx context.Context, req *models.UpdateProfileRequest, actor audit_models.Actor, userAgent string) (*models.Account, error) {
	if err := s.passwords.ConfirmPassword(ctx, actor.UserPanelID, req.CurrentPassword, actor.IP, userAgent); err != nil {
		return nil, err
	}
	first, last := strings.TrimSpace(req.FirstName), strings.TrimSpace(req.LastName)
	email := strings.ToLower(strings.TrimSpace(req.Email))
	if first == "" || last == "" {
		return nil, cc.ErrInvalidPayload
	}
	if err := s.repo.UpdateProfile(ctx, actor.UserPanelID, first, last, email, actor.Log(cc.AUDIT_UPDATED_OWN_PROFILE, "")); err != nil {
		return nil, err
	}
	return s.Get(ctx, actor.UserPanelID)
}

// ChangePassword sets a new password once the current one checks out, and
// signs the user out everywhere - they sign in again with the new one.
func (s *AccountService) ChangePassword(ctx context.Context, req *models.ChangePasswordRequest, actor audit_models.Actor, userAgent string) error {
	if utf8.RuneCountInString(req.NewPassword) < auth_service.MinPasswordLength {
		return cc.ErrPasswordTooShort
	}
	if req.NewPassword == req.CurrentPassword {
		return cc.ErrPasswordUnchanged
	}
	if err := s.passwords.ConfirmPassword(ctx, actor.UserPanelID, req.CurrentPassword, actor.IP, userAgent); err != nil {
		return err
	}
	hash, err := auth_service.HashPassword(req.NewPassword)
	if err != nil {
		return err
	}
	return s.repo.ChangePassword(ctx, actor.UserPanelID, hash, actor.Log(cc.AUDIT_CHANGED_OWN_PASSWORD, ""))
}
