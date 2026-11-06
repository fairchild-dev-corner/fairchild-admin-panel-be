package services

import (
	"context"
	"log/slog"
	"strings"
	"time"

	cc "fairchild-admin-panel-be/internal/constants"
	interfaces "fairchild-admin-panel-be/internal/interfaces"
	app_models "fairchild-admin-panel-be/internal/models/applications"
	audit_models "fairchild-admin-panel-be/internal/models/audit"
	models "fairchild-admin-panel-be/internal/models/locks"
	"fairchild-admin-panel-be/internal/services/messaging"
	"fairchild-admin-panel-be/internal/utils"
)

// notifyTimeout bounds each provider call so a hung SMTP/Movider connection
// doesn't hold the unlock request open.
const notifyTimeout = 20 * time.Second

type LocksService struct {
	repo  interfaces.LocksRepositoryInterface
	email messaging.EmailSender // nil = no unlock emails
	sms   messaging.SMSSender   // nil = no unlock texts
}

func NewLocksService(repo interfaces.LocksRepositoryInterface, email messaging.EmailSender, sms messaging.SMSSender) *LocksService {
	return &LocksService{repo: repo, email: email, sms: sms}
}

func (s *LocksService) List(ctx context.Context, q models.LockedAccountListQuery) (*models.LockedAccountPage, error) {
	q.Search = strings.TrimSpace(q.Search)
	counts, err := s.repo.CountByStatus(ctx, q)
	if err != nil {
		return nil, err
	}

	page := &models.LockedAccountPage{Total: utils.TotalFor(counts, q.Status), Page: q.Page, PageSize: q.PageSize, StatusCounts: counts}
	if lastPage := max(1, (page.Total+q.PageSize-1)/q.PageSize); page.Page > lastPage {
		page.Page = lastPage
	}
	q.Page = page.Page

	if page.Items, err = s.repo.List(ctx, q); err != nil {
		return nil, err
	}
	return page, nil
}

// Unlock lets the member sign in again straight away instead of waiting for
// the lock to expire, then tells them by email and text. Only a member ID
// with a portal account is messaged. A failed message doesn't undo the
// unlock; the result says which channels went out.
func (s *LocksService) Unlock(ctx context.Context, rawID string, actor audit_models.Actor) (*models.UnlockResult, error) {
	id, ok := app_models.ParseID(cc.LOCKED_ACCOUNT_ID_PREFIX, rawID)
	if !ok {
		return nil, cc.ErrLockedAccountNotFound
	}
	// The repository fills in the audit target (the member number).
	if err := s.repo.Unlock(ctx, id, actor.Log(cc.AUDIT_UNLOCKED_MEMBER_ACCOUNT, "")); err != nil {
		return nil, err
	}

	// The unlock is done; a client disconnect mustn't stop the member hearing
	// about it.
	ctx = context.WithoutCancel(ctx)
	lock, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	result := &models.UnlockResult{Lock: lock, Notifications: map[string]string{}}
	if lock == nil || lock.Account == nil {
		return result, nil
	}
	result.Notifications["email"] = s.emailUnlock(ctx, lock)
	result.Notifications["sms"] = s.textUnlock(ctx, lock)
	return result, nil
}

func (s *LocksService) emailUnlock(ctx context.Context, lock *models.LockedAccount) string {
	if s.email == nil {
		return models.NotifyUnavailable
	}
	to := strings.TrimSpace(lock.Account.Email)
	if utils.Validate.Var(to, "required,email") != nil {
		return models.NotifyNoAddress
	}

	name := strings.TrimSpace(lock.Account.FirstName)
	if name == "" {
		name = "Member"
	}
	unlocked, _ := time.Parse(time.RFC3339, lock.UnlockedAt)
	html, err := messaging.RenderAccountUnlocked(messaging.AccountUnlockedData{
		Name:       name,
		MemberNo:   lock.MemberNo,
		UnlockedAt: unlocked.In(utils.ManilaTZ).Format("January 2, 2006 3:04 PM"),
	})
	if err != nil {
		slog.Error("rendering unlock email failed", "lock", lock.ID, "err", err)
		return models.NotifyFailed
	}

	sendCtx, cancel := context.WithTimeout(ctx, notifyTimeout)
	defer cancel()
	if err := s.email.SendEmail(sendCtx, to, messaging.AccountUnlockedSubject, html); err != nil {
		slog.Error("unlock email failed", "lock", lock.ID, "err", err)
		return models.NotifyFailed
	}
	return models.NotifySent
}

func (s *LocksService) textUnlock(ctx context.Context, lock *models.LockedAccount) string {
	if s.sms == nil {
		return models.NotifyUnavailable
	}
	to, ok := messaging.NormalizeMobileNumber(lock.Account.NotifyMobile)
	if !ok {
		return models.NotifyNoAddress
	}

	sendCtx, cancel := context.WithTimeout(ctx, notifyTimeout)
	defer cancel()
	if err := s.sms.SendSMS(sendCtx, to, messaging.AccountUnlockedSMS(lock.Account.FirstName)); err != nil {
		slog.Error("unlock SMS failed", "lock", lock.ID, "err", err)
		return models.NotifyFailed
	}
	return models.NotifySent
}
