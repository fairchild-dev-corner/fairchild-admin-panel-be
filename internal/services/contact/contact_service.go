package services

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	cc "fairchild-admin-panel-be/internal/constants"
	interfaces "fairchild-admin-panel-be/internal/interfaces"
	app_models "fairchild-admin-panel-be/internal/models/applications"
	audit_models "fairchild-admin-panel-be/internal/models/audit"
	models "fairchild-admin-panel-be/internal/models/contact"
	"fairchild-admin-panel-be/internal/services/messaging"
	"fairchild-admin-panel-be/internal/utils"
)

// sendTimeout bounds one provider call so a hung SMTP/Movider connection
// doesn't hold the request open.
const sendTimeout = 20 * time.Second

type ContactService struct {
	repo  interfaces.ContactRepositoryInterface
	email messaging.EmailSender // nil = email replies disabled
	sms   messaging.SMSSender   // nil = text replies disabled
}

func NewContactService(repo interfaces.ContactRepositoryInterface, email messaging.EmailSender, sms messaging.SMSSender) *ContactService {
	return &ContactService{repo: repo, email: email, sms: sms}
}

func (s *ContactService) channels() models.ContactChannels {
	return models.ContactChannels{Email: s.email != nil, SMS: s.sms != nil}
}

func (s *ContactService) List(ctx context.Context, q models.ContactListQuery) (*models.ContactMessagePage, error) {
	q.Search = strings.TrimSpace(q.Search)
	counts, err := s.repo.CountByStatus(ctx, q.Search)
	if err != nil {
		return nil, err
	}

	page := &models.ContactMessagePage{
		Total:        utils.TotalFor(counts, q.Status),
		Page:         q.Page,
		PageSize:     q.PageSize,
		StatusCounts: counts,
		Channels:     s.channels(),
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

func (s *ContactService) Get(ctx context.Context, rawID string) (*models.ContactMessage, error) {
	id, ok := app_models.ParseID(cc.CONTACT_MESSAGE_ID_PREFIX, rawID)
	if !ok {
		return nil, cc.ErrContactMessageNotFound
	}
	m, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if m == nil {
		return nil, cc.ErrContactMessageNotFound
	}
	return m, nil
}

// Reply sends the admin's reply by email (to the address on the message) or
// text (to the mobile number of the portal account with that email), then
// records it. Nothing is recorded when the provider refuses the message.
func (s *ContactService) Reply(ctx context.Context, rawID string, req *models.ReplyRequest, actor audit_models.Actor) (*models.ContactMessage, error) {
	m, err := s.Get(ctx, rawID)
	if err != nil {
		return nil, err
	}
	body := strings.TrimSpace(req.Body)
	if body == "" {
		return nil, cc.ErrReplyEmpty
	}

	sendCtx, cancel := context.WithTimeout(ctx, sendTimeout)
	defer cancel()

	var recipient, action string
	switch req.Channel {
	case models.ChannelEmail:
		if s.email == nil {
			return nil, cc.ErrReplyEmailUnavailable
		}
		recipient = strings.TrimSpace(m.Email)
		if utils.Validate.Var(recipient, "required,email") != nil {
			return nil, cc.ErrReplyInvalidEmail
		}
		received, _ := time.Parse(time.RFC3339, m.ReceivedAt)
		html, err := messaging.RenderContactReply(messaging.ContactReplyData{
			Name:       m.Name,
			Body:       body,
			ReceivedAt: received.In(utils.ManilaTZ).Format("January 2, 2006 3:04 PM"),
			Original:   m.Message,
		})
		if err != nil {
			return nil, err
		}
		if err := s.email.SendEmail(sendCtx, recipient, messaging.ContactReplySubject, html); err != nil {
			slog.Error("contact reply email failed", "message", m.ID, "err", err)
			return nil, cc.ErrReplySendFailed
		}
		action = cc.AUDIT_EMAILED_CONTACT_REPLY

	case models.ChannelSMS:
		if s.sms == nil {
			return nil, cc.ErrReplySMSUnavailable
		}
		if utf8.RuneCountInString(body) > models.MaxSMSReplyLength {
			return nil, cc.ErrReplyTooLongForSMS
		}
		if m.Account == nil {
			return nil, cc.ErrReplyNoMobile
		}
		to, ok := messaging.NormalizeMobileNumber(m.Account.MobileNumber)
		if !ok {
			return nil, cc.ErrReplyNoMobile
		}
		if err := s.sms.SendSMS(sendCtx, to, fmt.Sprintf("%s: %s", messaging.BrandName, body)); err != nil {
			slog.Error("contact reply SMS failed", "message", m.ID, "err", err)
			return nil, cc.ErrReplySendFailed
		}
		recipient, action = to, cc.AUDIT_TEXTED_CONTACT_REPLY

	default:
		return nil, cc.ErrInvalidPayload
	}

	id, _ := app_models.ParseID(cc.CONTACT_MESSAGE_ID_PREFIX, rawID)
	// The member already has the reply; use a fresh context so a client
	// disconnect can't stop it being recorded.
	recordCtx, cancelRecord := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancelRecord()
	if err := s.repo.InsertReply(recordCtx, id, req.Channel, recipient, body, actor.Log(action, m.ID)); err != nil {
		slog.Error("contact reply sent but not recorded", "message", m.ID, "channel", req.Channel, "err", err)
		return nil, cc.ErrReplySentNotRecorded
	}
	return s.Get(ctx, rawID)
}
