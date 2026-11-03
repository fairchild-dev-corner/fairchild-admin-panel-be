package services

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"testing"
	"time"

	cc "fairchild-admin-panel-be/internal/constants"
	models "fairchild-admin-panel-be/internal/models/auth"
)

// fakeAuthRepo is an in-memory AuthRepositoryInterface.
type fakeAuthRepo struct {
	mu       sync.Mutex
	users    map[int64]*models.UserPanel
	sessions map[string]*models.AuthSession
	revoked  map[string]bool
	attempts []models.LoginAttempt
}

func newFakeAuthRepo(users ...*models.UserPanel) *fakeAuthRepo {
	r := &fakeAuthRepo{
		users:    map[int64]*models.UserPanel{},
		sessions: map[string]*models.AuthSession{},
		revoked:  map[string]bool{},
	}
	for _, u := range users {
		r.users[u.ID] = u
	}
	return r
}

func (r *fakeAuthRepo) FindUserByUsername(_ context.Context, username string) (*models.UserPanel, error) {
	for _, u := range r.users {
		if u.Username == username {
			return u, nil
		}
	}
	return nil, nil
}

func (r *fakeAuthRepo) FindUserByID(_ context.Context, id int64) (*models.UserPanel, error) {
	return r.users[id], nil
}

func (r *fakeAuthRepo) CreateUser(context.Context, *models.UserPanel) (int64, error) {
	return 0, errors.New("not implemented")
}

func (r *fakeAuthRepo) TouchLastLogin(context.Context, int64) error { return nil }

func (r *fakeAuthRepo) CreateSession(_ context.Context, s *models.AuthSession) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sessions[s.RefreshTokenHash] = s
	return nil
}

func (r *fakeAuthRepo) FindActiveSessionByTokenHash(_ context.Context, hash string) (*models.AuthSession, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s := r.sessions[hash]
	if s == nil || r.revoked[hash] || time.Now().After(s.ExpiresAt) {
		return nil, nil
	}
	return s, nil
}

func (r *fakeAuthRepo) RevokeSession(_ context.Context, hash string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.sessions[hash] == nil || r.revoked[hash] {
		return false, nil
	}
	r.revoked[hash] = true
	return true, nil
}

func (r *fakeAuthRepo) RecordLoginAttempt(_ context.Context, a *models.LoginAttempt) error {
	r.attempts = append(r.attempts, *a)
	return nil
}

func (r *fakeAuthRepo) CountRecentFailedLogins(_ context.Context, username, ip string, _ time.Duration) (int, int, error) {
	var byUser, byIP int
	for _, a := range r.attempts {
		if a.Success {
			continue
		}
		if a.UsernameAttempted == username {
			byUser++
		}
		if a.IPAddress == ip {
			byIP++
		}
	}
	return byUser, byIP, nil
}

func newTestUser(t *testing.T, id int64, username, password, status string) *models.UserPanel {
	t.Helper()
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	return &models.UserPanel{
		ID: id, UUID: "uuid", Username: username, PasswordHash: hash,
		FirstName: "Test", LastName: "User", Email: sql.NullString{},
		Role: cc.ROLE_ADMIN, Status: status,
	}
}

func newTestAuthService(t *testing.T, repo *fakeAuthRepo) *AuthService {
	t.Helper()
	svc, err := NewAuthService(repo, newTestTokenService())
	if err != nil {
		t.Fatal(err)
	}
	return svc
}

var client = ClientInfo{IP: "10.0.0.1", UserAgent: "test"}

func TestLoginSucceedsWithCaseInsensitiveUsername(t *testing.T) {
	repo := newFakeAuthRepo(newTestUser(t, 1, "jdoe", "a-long-password", cc.USER_STATUS_ACTIVE))
	svc := newTestAuthService(t, repo)

	tokens, err := svc.Login(context.Background(), &models.LoginRequest{Username: "  JDoe ", Password: "a-long-password"}, client)
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}
	if tokens.Response.User.Username != "jdoe" || tokens.Response.User.Role != cc.ROLE_ADMIN || tokens.RefreshToken == "" {
		t.Fatalf("unexpected tokens: %+v", tokens.Response)
	}

	user, err := svc.Authenticate(context.Background(), tokens.Response.AccessToken)
	if err != nil || user.ID != 1 {
		t.Fatalf("access token didn't authenticate: %v", err)
	}
}

func TestLoginUnknownUserAndWrongPasswordLookTheSame(t *testing.T) {
	repo := newFakeAuthRepo(newTestUser(t, 1, "jdoe", "a-long-password", cc.USER_STATUS_ACTIVE))
	svc := newTestAuthService(t, repo)

	_, errUnknown := svc.Login(context.Background(), &models.LoginRequest{Username: "nobody", Password: "x"}, client)
	_, errWrong := svc.Login(context.Background(), &models.LoginRequest{Username: "jdoe", Password: "x"}, client)

	if !errors.Is(errUnknown, cc.ErrInvalidCredentials) || !errors.Is(errWrong, cc.ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials for both, got %v / %v", errUnknown, errWrong)
	}
}

func TestLoginLocksOutAfterRepeatedFailures(t *testing.T) {
	repo := newFakeAuthRepo(newTestUser(t, 1, "jdoe", "a-long-password", cc.USER_STATUS_ACTIVE))
	svc := newTestAuthService(t, repo)

	for i := 0; i < maxFailedLogins; i++ {
		svc.Login(context.Background(), &models.LoginRequest{Username: "jdoe", Password: "wrong"}, client)
	}

	// Even the right password is refused while locked.
	_, err := svc.Login(context.Background(), &models.LoginRequest{Username: "jdoe", Password: "a-long-password"}, client)
	if !errors.Is(err, cc.ErrAccountTemporarilyLocked) {
		t.Fatalf("expected lockout, got %v", err)
	}
}

func TestLoginRejectsSuspendedAccountOnlyAfterPasswordCheck(t *testing.T) {
	repo := newFakeAuthRepo(newTestUser(t, 1, "jdoe", "a-long-password", cc.USER_STATUS_SUSPENDED))
	svc := newTestAuthService(t, repo)

	_, err := svc.Login(context.Background(), &models.LoginRequest{Username: "jdoe", Password: "wrong"}, client)
	if !errors.Is(err, cc.ErrInvalidCredentials) {
		t.Fatalf("wrong password on suspended account should look like bad credentials, got %v", err)
	}

	_, err = svc.Login(context.Background(), &models.LoginRequest{Username: "jdoe", Password: "a-long-password"}, client)
	if !errors.Is(err, cc.ErrAccountSuspended) {
		t.Fatalf("expected ErrAccountSuspended, got %v", err)
	}
}

func TestRefreshRotatesAndRejectsReuse(t *testing.T) {
	repo := newFakeAuthRepo(newTestUser(t, 1, "jdoe", "a-long-password", cc.USER_STATUS_ACTIVE))
	svc := newTestAuthService(t, repo)
	ctx := context.Background()

	first, err := svc.Login(ctx, &models.LoginRequest{Username: "jdoe", Password: "a-long-password"}, client)
	if err != nil {
		t.Fatal(err)
	}

	second, err := svc.Refresh(ctx, first.RefreshToken, client)
	if err != nil {
		t.Fatalf("refresh failed: %v", err)
	}
	if second.RefreshToken == first.RefreshToken {
		t.Fatal("refresh token was not rotated")
	}

	if _, err := svc.Refresh(ctx, first.RefreshToken, client); !errors.Is(err, cc.ErrInvalidRefreshToken) {
		t.Fatalf("reused refresh token accepted: %v", err)
	}
	if _, err := svc.Refresh(ctx, "", client); !errors.Is(err, cc.ErrInvalidRefreshToken) {
		t.Fatalf("empty refresh token accepted: %v", err)
	}
}

func TestSuspendingUserBlocksRefreshAndAccessImmediately(t *testing.T) {
	user := newTestUser(t, 1, "jdoe", "a-long-password", cc.USER_STATUS_ACTIVE)
	repo := newFakeAuthRepo(user)
	svc := newTestAuthService(t, repo)
	ctx := context.Background()

	tokens, err := svc.Login(ctx, &models.LoginRequest{Username: "jdoe", Password: "a-long-password"}, client)
	if err != nil {
		t.Fatal(err)
	}

	user.Status = cc.USER_STATUS_SUSPENDED

	if _, err := svc.Authenticate(ctx, tokens.Response.AccessToken); !errors.Is(err, cc.ErrInvalidAccessToken) {
		t.Fatalf("suspended user's access token still works: %v", err)
	}
	if _, err := svc.Refresh(ctx, tokens.RefreshToken, client); !errors.Is(err, cc.ErrInvalidRefreshToken) {
		t.Fatalf("suspended user could refresh: %v", err)
	}
}
