package services

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	cc "fairchild-admin-panel-be/internal/constants"
	interfaces "fairchild-admin-panel-be/internal/interfaces"
	models "fairchild-admin-panel-be/internal/models/auth"
	"fairchild-admin-panel-be/internal/utils"
)

const (
	// Lockout: this many failed logins for one username, or from one IP,
	// within lockoutWindow blocks further attempts until the window rolls.
	maxFailedLogins = 5
	lockoutWindow   = 15 * time.Minute
)

// ClientInfo is request metadata recorded on sessions and login attempts.
type ClientInfo struct {
	IP        string
	UserAgent string
}

type AuthService struct {
	repo   interfaces.AuthRepositoryInterface
	tokens *TokenService
	// dummyHash is checked against when the username doesn't exist, so an
	// unknown username takes as long to reject as a wrong password.
	dummyHash string
}

func NewAuthService(repo interfaces.AuthRepositoryInterface, tokens *TokenService) (*AuthService, error) {
	dummy, err := HashPassword("timing-equalizer-not-a-real-password")
	if err != nil {
		return nil, fmt.Errorf("init dummy password hash: %w", err)
	}
	return &AuthService{repo: repo, tokens: tokens, dummyHash: dummy}, nil
}

// NormalizeUsername is applied on both account creation and login.
func NormalizeUsername(username string) string {
	return strings.ToLower(strings.TrimSpace(username))
}

// Login verifies username + password and issues an access token and a
// refresh session. There is no OTP step for panel accounts.
func (s *AuthService) Login(ctx context.Context, req *models.LoginRequest, client ClientInfo) (*models.IssuedTokens, error) {
	username := NormalizeUsername(req.Username)

	byUsername, byIP, err := s.repo.CountRecentFailedLogins(ctx, username, client.IP, lockoutWindow)
	if err != nil {
		return nil, err
	}
	if byUsername >= maxFailedLogins || byIP >= maxFailedLogins {
		return nil, cc.ErrAccountTemporarilyLocked
	}

	user, err := s.repo.FindUserByUsername(ctx, username)
	if err != nil {
		return nil, err
	}

	if user == nil {
		_ = CheckPassword(s.dummyHash, req.Password)
		s.recordAttempt(ctx, nil, username, client, false, "unknown username")
		return nil, cc.ErrInvalidCredentials
	}

	if err := CheckPassword(user.PasswordHash, req.Password); err != nil {
		s.recordAttempt(ctx, &user.ID, username, client, false, "wrong password")
		return nil, cc.ErrInvalidCredentials
	}

	if user.Status != cc.USER_STATUS_ACTIVE {
		s.recordAttempt(ctx, &user.ID, username, client, false, "account "+user.Status)
		return nil, cc.ErrAccountSuspended
	}

	tokens, err := s.issueTokens(ctx, user, client)
	if err != nil {
		return nil, err
	}

	s.recordAttempt(ctx, &user.ID, username, client, true, "")
	if err := s.repo.TouchLastLogin(ctx, user.ID); err != nil {
		slog.Warn("failed to update last_login_at", "user_panel_id", user.ID, "error", err)
	}

	return tokens, nil
}

// ConfirmPassword re-checks a signed-in panel user's own password before a
// sensitive action (e.g. turning the election on or off). It shares the
// login lockout: a wrong password is recorded as a failed login, so repeated
// guesses lock the account the same way.
func (s *AuthService) ConfirmPassword(ctx context.Context, userID int64, password, ip, userAgent string) error {
	user, err := s.repo.FindUserByID(ctx, userID)
	if err != nil {
		return err
	}
	if user == nil || user.Status != cc.USER_STATUS_ACTIVE {
		return cc.ErrPasswordIncorrect
	}
	client := ClientInfo{IP: ip, UserAgent: userAgent}

	byUsername, byIP, err := s.repo.CountRecentFailedLogins(ctx, user.Username, client.IP, lockoutWindow)
	if err != nil {
		return err
	}
	if byUsername >= maxFailedLogins || byIP >= maxFailedLogins {
		return cc.ErrAccountTemporarilyLocked
	}

	if err := CheckPassword(user.PasswordHash, password); err != nil {
		s.recordAttempt(ctx, &user.ID, user.Username, client, false, "wrong password (confirmation)")
		return cc.ErrPasswordIncorrect
	}
	return nil
}

// Refresh redeems a refresh token (rotate-on-use): the presented session is
// revoked and a brand-new pair is issued. The user's status is re-checked,
// so suspending an account stops it at the next refresh at the latest.
func (s *AuthService) Refresh(ctx context.Context, refreshToken string, client ClientInfo) (*models.IssuedTokens, error) {
	if refreshToken == "" {
		return nil, cc.ErrInvalidRefreshToken
	}
	hash := utils.MustStringSHA256(refreshToken)

	session, err := s.repo.FindActiveSessionByTokenHash(ctx, hash)
	if err != nil {
		return nil, err
	}
	if session == nil {
		return nil, cc.ErrInvalidRefreshToken
	}

	revoked, err := s.repo.RevokeSession(ctx, hash)
	if err != nil {
		return nil, err
	}
	if !revoked {
		// Another request rotated this token first.
		return nil, cc.ErrInvalidRefreshToken
	}

	user, err := s.repo.FindUserByID(ctx, session.UserPanelID)
	if err != nil {
		return nil, err
	}
	if user == nil || user.Status != cc.USER_STATUS_ACTIVE {
		return nil, cc.ErrInvalidRefreshToken
	}

	return s.issueTokens(ctx, user, client)
}

// Logout revokes the session behind the refresh token, if any. Idempotent.
func (s *AuthService) Logout(ctx context.Context, refreshToken string) error {
	if refreshToken == "" {
		return nil
	}
	_, err := s.repo.RevokeSession(ctx, utils.MustStringSHA256(refreshToken))
	return err
}

// Authenticate validates an access token and loads the panel user it
// belongs to, rejecting users who are no longer active. Used by
// middlewares.RequirePanelAuth on every /admin request, so suspending a
// user or changing their role takes effect immediately rather than when
// their access token expires.
func (s *AuthService) Authenticate(ctx context.Context, accessToken string) (*models.UserPanel, error) {
	claims, err := s.tokens.ParseAccessToken(accessToken)
	if err != nil {
		return nil, err
	}

	user, err := s.repo.FindUserByID(ctx, claims.UserPanelID)
	if err != nil {
		return nil, err
	}
	if user == nil || user.Status != cc.USER_STATUS_ACTIVE {
		return nil, cc.ErrInvalidAccessToken
	}
	return user, nil
}

func (s *AuthService) issueTokens(ctx context.Context, user *models.UserPanel, client ClientInfo) (*models.IssuedTokens, error) {
	accessToken, err := s.tokens.GenerateAccessToken(user.ID, user.Username, user.Role)
	if err != nil {
		return nil, fmt.Errorf("sign access token: %w", err)
	}

	refreshToken, err := GenerateOpaqueToken()
	if err != nil {
		return nil, fmt.Errorf("generate refresh token: %w", err)
	}

	refreshTTL := s.tokens.RefreshTokenTTL()
	if err := s.repo.CreateSession(ctx, &models.AuthSession{
		UserPanelID:      user.ID,
		RefreshTokenHash: utils.MustStringSHA256(refreshToken),
		UserAgent:        client.UserAgent,
		IPAddress:        client.IP,
		ExpiresAt:        time.Now().Add(refreshTTL),
	}); err != nil {
		return nil, err
	}

	return &models.IssuedTokens{
		Response: models.AuthTokenResponse{
			AccessToken: accessToken,
			TokenType:   "Bearer",
			ExpiresIn:   int64(s.tokens.AccessTokenTTL().Seconds()),
			User:        models.NewUserPanelResponse(user),
		},
		RefreshToken: refreshToken,
		RefreshTTL:   int64(refreshTTL.Seconds()),
	}, nil
}

// recordAttempt is best-effort: a failed audit insert is logged but never
// turns a valid login into an error. (A failed insert on a *bad* password
// does weaken the lockout for that attempt, which is why it's logged loudly.)
func (s *AuthService) recordAttempt(ctx context.Context, userID *int64, username string, client ClientInfo, success bool, reason string) {
	err := s.repo.RecordLoginAttempt(ctx, &models.LoginAttempt{
		UserPanelID:       userID,
		UsernameAttempted: username,
		IPAddress:         client.IP,
		UserAgent:         client.UserAgent,
		Success:           success,
		Reason:            reason,
	})
	if err != nil {
		slog.Error("failed to record login attempt", "username", username, "error", err)
	}
}
