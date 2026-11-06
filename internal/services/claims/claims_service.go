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
	models "fairchild-admin-panel-be/internal/models/claims"
	locks_models "fairchild-admin-panel-be/internal/models/locks"
	claims_repo "fairchild-admin-panel-be/internal/repositories/claims"
	"fairchild-admin-panel-be/internal/services/messaging"
	"fairchild-admin-panel-be/internal/utils"
)

// notifyTimeout bounds each provider call so a hung SMTP/Movider connection
// doesn't hold the approve request open.
const notifyTimeout = 20 * time.Second

type ClaimsService struct {
	repo  interfaces.ClaimsRepositoryInterface
	email messaging.EmailSender // nil = no approval emails
	sms   messaging.SMSSender   // nil = no approval texts
}

func NewClaimsService(repo interfaces.ClaimsRepositoryInterface, email messaging.EmailSender, sms messaging.SMSSender) *ClaimsService {
	return &ClaimsService{repo: repo, email: email, sms: sms}
}

func (s *ClaimsService) List(ctx context.Context, q models.ClaimListQuery) (*models.ClaimPage, error) {
	q.Search = strings.TrimSpace(q.Search)
	counts, err := s.repo.CountByStatus(ctx, q)
	if err != nil {
		return nil, err
	}

	page := &models.ClaimPage{
		Total:        utils.TotalFor(counts, q.Status),
		Page:         q.Page,
		PageSize:     q.PageSize,
		StatusCounts: counts,
	}
	if lastPage := max(1, (page.Total+q.PageSize-1)/q.PageSize); page.Page > lastPage {
		page.Page = lastPage
	}
	q.Page = page.Page

	if page.Items, err = s.repo.List(ctx, q); err != nil {
		return nil, err
	}
	return page, nil
}

func (s *ClaimsService) Get(ctx context.Context, rawID string) (*models.ClaimRequest, error) {
	id, ok := app_models.ParseID(cc.ACCOUNT_CLAIM_ID_PREFIX, rawID)
	if !ok {
		return nil, cc.ErrClaimNotFound
	}
	c, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if c == nil {
		return nil, cc.ErrClaimNotFound
	}
	return c, nil
}

// Approve applies the claim: the matching portal account's email and mobile
// number become the ones the member asked for. Both must be valid, since the
// member will sign in and reset their password with them. The member is then
// told by email and text on those new details. A failed message doesn't undo
// the approval; the result says which channels went out.
func (s *ClaimsService) Approve(ctx context.Context, rawID string, actor audit_models.Actor) (*models.ApproveResult, error) {
	c, err := s.Get(ctx, rawID)
	if err != nil {
		return nil, err
	}
	if c.Status != cc.APPLICATION_STATUS_PENDING {
		return nil, cc.ErrClaimAlreadyReviewed
	}
	if c.Account == nil {
		return nil, cc.ErrClaimNoPortalAccount
	}

	email := strings.TrimSpace(c.ProposedEmail)
	mobile := utils.CompactMobile(c.ProposedMobileNumber)
	if utils.Validate.Var(email, "required,email,max=255") != nil || !utils.IsValidPhMobile(mobile) {
		return nil, cc.ErrClaimInvalidContact
	}

	id, _ := app_models.ParseID(cc.ACCOUNT_CLAIM_ID_PREFIX, rawID)
	if err := s.repo.Approve(ctx, id, email, mobile, actor.Log(cc.AUDIT_APPROVED_ACCOUNT_CLAIM, claims_repo.FormatClaimID(id))); err != nil {
		return nil, err
	}

	// The approval is done; a client disconnect mustn't stop the member
	// hearing about it.
	ctx = context.WithoutCancel(ctx)
	approved, err := s.Get(ctx, rawID)
	if err != nil {
		return nil, err
	}
	firstName := ""
	if approved.Account != nil {
		firstName = approved.Account.FirstName
	}
	return &models.ApproveResult{
		Claim: approved,
		Notifications: map[string]string{
			"email": s.emailApproval(ctx, approved, firstName, email, mobile),
			"sms":   s.textApproval(ctx, approved, firstName, mobile),
		},
	}, nil
}

func (s *ClaimsService) emailApproval(ctx context.Context, c *models.ClaimRequest, firstName, email, mobile string) string {
	if s.email == nil {
		return locks_models.NotifyUnavailable
	}
	name := messaging.FriendlyFirstName(firstName)
	if name == "" {
		name = "Member"
	}
	html, err := messaging.RenderClaimApproved(messaging.ClaimApprovedData{
		Name:     name,
		MemberNo: c.MemberNo,
		Email:    email,
		Mobile:   mobile,
	})
	if err != nil {
		slog.Error("rendering claim approval email failed", "claim", c.ID, "err", err)
		return locks_models.NotifyFailed
	}

	sendCtx, cancel := context.WithTimeout(ctx, notifyTimeout)
	defer cancel()
	if err := s.email.SendEmail(sendCtx, email, messaging.ClaimApprovedSubject, html); err != nil {
		slog.Error("claim approval email failed", "claim", c.ID, "err", err)
		return locks_models.NotifyFailed
	}
	return locks_models.NotifySent
}

func (s *ClaimsService) textApproval(ctx context.Context, c *models.ClaimRequest, firstName, mobile string) string {
	if s.sms == nil {
		return locks_models.NotifyUnavailable
	}
	to, ok := messaging.NormalizeMobileNumber(mobile)
	if !ok {
		return locks_models.NotifyNoAddress
	}

	sendCtx, cancel := context.WithTimeout(ctx, notifyTimeout)
	defer cancel()
	if err := s.sms.SendSMS(sendCtx, to, messaging.ClaimApprovedSMS(firstName)); err != nil {
		slog.Error("claim approval SMS failed", "claim", c.ID, "err", err)
		return locks_models.NotifyFailed
	}
	return locks_models.NotifySent
}

// Reject turns the claim down with an optional reason. When the admin asks
// (req.Notify), the member is then told by email and text on the details
// already on their portal account - never the claimed ones, which may not be
// theirs. A failed message doesn't undo the rejection; the result says which
// channels went out.
func (s *ClaimsService) Reject(ctx context.Context, rawID string, req *models.RejectClaimRequest, actor audit_models.Actor) (*models.RejectResult, error) {
	id, ok := app_models.ParseID(cc.ACCOUNT_CLAIM_ID_PREFIX, rawID)
	if !ok {
		return nil, cc.ErrClaimNotFound
	}
	reason := strings.TrimSpace(req.Reason)
	if err := s.repo.Reject(ctx, id, reason, actor.Log(cc.AUDIT_REJECTED_ACCOUNT_CLAIM, claims_repo.FormatClaimID(id))); err != nil {
		return nil, err
	}

	// As in Approve: the rejection is done, so notify even if the client left.
	ctx = context.WithoutCancel(ctx)
	rejected, err := s.Get(ctx, rawID)
	if err != nil {
		return nil, err
	}
	if !req.Notify {
		return &models.RejectResult{Claim: rejected}, nil
	}
	var account models.ClaimAccount
	if rejected.Account != nil {
		account = *rejected.Account
	}
	return &models.RejectResult{
		Claim: rejected,
		Notifications: map[string]string{
			"email": s.emailRejection(ctx, rejected, account),
			"sms":   s.textRejection(ctx, rejected, account),
		},
	}, nil
}

func (s *ClaimsService) emailRejection(ctx context.Context, c *models.ClaimRequest, account models.ClaimAccount) string {
	if s.email == nil {
		return locks_models.NotifyUnavailable
	}
	if strings.TrimSpace(account.Email) == "" {
		return locks_models.NotifyNoAddress
	}
	name := messaging.FriendlyFirstName(account.FirstName)
	if name == "" {
		name = "Member"
	}
	html, err := messaging.RenderClaimRejected(messaging.ClaimRejectedData{
		Name:     name,
		MemberNo: c.MemberNo,
		Reason:   c.RejectionReason,
	})
	if err != nil {
		slog.Error("rendering claim rejection email failed", "claim", c.ID, "err", err)
		return locks_models.NotifyFailed
	}

	sendCtx, cancel := context.WithTimeout(ctx, notifyTimeout)
	defer cancel()
	if err := s.email.SendEmail(sendCtx, strings.TrimSpace(account.Email), messaging.ClaimRejectedSubject, html); err != nil {
		slog.Error("claim rejection email failed", "claim", c.ID, "err", err)
		return locks_models.NotifyFailed
	}
	return locks_models.NotifySent
}

func (s *ClaimsService) textRejection(ctx context.Context, c *models.ClaimRequest, account models.ClaimAccount) string {
	if s.sms == nil {
		return locks_models.NotifyUnavailable
	}
	to, ok := messaging.NormalizeMobileNumber(account.MobileNumber)
	if !ok {
		return locks_models.NotifyNoAddress
	}

	sendCtx, cancel := context.WithTimeout(ctx, notifyTimeout)
	defer cancel()
	if err := s.sms.SendSMS(sendCtx, to, messaging.ClaimRejectedSMS(account.FirstName, c.RejectionReason)); err != nil {
		slog.Error("claim rejection SMS failed", "claim", c.ID, "err", err)
		return locks_models.NotifyFailed
	}
	return locks_models.NotifySent
}

// Delete removes an approved or rejected claim for good. A pending claim has
// to be reviewed first.
func (s *ClaimsService) Delete(ctx context.Context, rawID string, actor audit_models.Actor) error {
	id, ok := app_models.ParseID(cc.ACCOUNT_CLAIM_ID_PREFIX, rawID)
	if !ok {
		return cc.ErrClaimNotFound
	}
	return s.repo.Delete(ctx, id, actor.Log(cc.AUDIT_DELETED_ACCOUNT_CLAIM, claims_repo.FormatClaimID(id)))
}
