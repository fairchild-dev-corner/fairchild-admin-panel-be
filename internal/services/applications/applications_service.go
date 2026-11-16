package services

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	cc "fairchild-admin-panel-be/internal/constants"
	interfaces "fairchild-admin-panel-be/internal/interfaces"
	models "fairchild-admin-panel-be/internal/models/applications"
	audit_models "fairchild-admin-panel-be/internal/models/audit"
	"fairchild-admin-panel-be/internal/services/messaging"
	"fairchild-admin-panel-be/internal/utils"
)

// sendTimeout bounds one provider call so a hung SMTP/Movider connection
// doesn't hold the request open.
const sendTimeout = 20 * time.Second

type ApplicationsService struct {
	repo  interfaces.ApplicationsRepositoryInterface
	email messaging.EmailSender // nil = emailing applicants disabled
	sms   messaging.SMSSender   // nil = texting applicants disabled
}

func NewApplicationsService(repo interfaces.ApplicationsRepositoryInterface, email messaging.EmailSender, sms messaging.SMSSender) *ApplicationsService {
	return &ApplicationsService{repo: repo, email: email, sms: sms}
}

// MessageLoanApplicant texts or emails the applicant of a loan application,
// at their portal account's mobile number or email address, and audits it.
// The message itself isn't stored.
func (s *ApplicationsService) MessageLoanApplicant(ctx context.Context, rawID string, req *models.MessageApplicantRequest, actor audit_models.Actor) (*models.MessageApplicantResult, error) {
	app, err := s.GetLoanApplication(ctx, rawID)
	if err != nil {
		return nil, err
	}
	body := strings.TrimSpace(req.Body)
	if body == "" {
		return nil, cc.ErrMessageEmpty
	}
	p := app.Applicant
	if p == nil {
		p = &models.LoanApplicant{}
	}

	sendCtx, cancel := context.WithTimeout(ctx, sendTimeout)
	defer cancel()

	var recipient, action string
	switch req.Channel {
	case models.MessageChannelEmail:
		if s.email == nil {
			return nil, cc.ErrMessageEmailUnavailable
		}
		recipient = strings.TrimSpace(p.Email)
		if utils.Validate.Var(recipient, "required,email") != nil {
			return nil, cc.ErrApplicantNoEmail
		}
		subject := strings.TrimSpace(req.Subject)
		if subject == "" {
			subject = "About your loan application " + app.ID
		}
		html, err := messaging.RenderAnnouncement(messaging.AnnouncementData{Subject: subject, Body: body})
		if err != nil {
			return nil, err
		}
		if err := s.email.SendEmail(sendCtx, recipient, subject, html); err != nil {
			slog.Error("loan applicant email failed", "application", app.ID, "err", err)
			return nil, cc.ErrMessageSendFailed
		}
		action = cc.AUDIT_EMAILED_LOAN_APPLICANT

	case models.MessageChannelSMS:
		if s.sms == nil {
			return nil, cc.ErrMessageSMSUnavailable
		}
		if utf8.RuneCountInString(body) > models.MaxSMSMessageLength {
			return nil, cc.ErrMessageTooLongForSMS
		}
		to, ok := messaging.NormalizeMobileNumber(p.MobileNumber)
		if !ok {
			return nil, cc.ErrApplicantNoMobile
		}
		if err := s.sms.SendSMS(sendCtx, to, fmt.Sprintf("%s: %s", messaging.BrandName, body)); err != nil {
			slog.Error("loan applicant SMS failed", "application", app.ID, "err", err)
			return nil, cc.ErrMessageSendFailed
		}
		recipient, action = to, cc.AUDIT_TEXTED_LOAN_APPLICANT

	default:
		return nil, cc.ErrInvalidPayload
	}

	// The member already has the message; use a fresh context so a client
	// disconnect can't stop it being audited.
	recordCtx, cancelRecord := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancelRecord()
	if err := s.repo.RecordApplicantMessage(recordCtx, actor.Log(action, app.ID)); err != nil {
		slog.Error("loan applicant message sent but not audited", "application", app.ID, "channel", req.Channel, "err", err)
	}
	return &models.MessageApplicantResult{Channel: req.Channel, Recipient: recipient}, nil
}

func (s *ApplicationsService) ListLoanApplications(ctx context.Context, status string) ([]models.LoanApplication, error) {
	return s.repo.ListLoanApplications(ctx, status)
}

// GetLoanApplication accepts "LA-01040" or "1040"; a malformed id is
// reported as not found. It adds the applicant's portal account and their
// other applications for the detail page.
func (s *ApplicationsService) GetLoanApplication(ctx context.Context, rawID string) (*models.LoanApplication, error) {
	id, ok := models.ParseID(cc.LOAN_APPLICATION_ID_PREFIX, rawID)
	if !ok {
		return nil, cc.ErrApplicationNotFound
	}
	app, err := s.repo.FindLoanApplicationByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if app == nil {
		return nil, cc.ErrApplicationNotFound
	}
	if app.Applicant, err = s.repo.FindLoanApplicant(ctx, id); err != nil {
		return nil, err
	}
	if app.History, err = s.repo.ListMemberLoanApplications(ctx, app.MemberID, id); err != nil {
		return nil, err
	}
	app.MessageChannels = &models.MessageChannels{Email: s.email != nil, SMS: s.sms != nil}
	if app.Notes, err = s.repo.ListLoanApplicationNotes(ctx, id); err != nil {
		return nil, err
	}
	return app, nil
}

// AddLoanApplicationNote adds a staff note to a loan application, written
// by actor, and audits it.
func (s *ApplicationsService) AddLoanApplicationNote(ctx context.Context, rawID string, req *models.AddLoanNoteRequest, actor audit_models.Actor) (*models.LoanApplicationNote, error) {
	id, ok := models.ParseID(cc.LOAN_APPLICATION_ID_PREFIX, rawID)
	if !ok {
		return nil, cc.ErrApplicationNotFound
	}
	body := strings.TrimSpace(req.Body)
	if body == "" {
		return nil, cc.ErrNoteEmpty
	}
	if utf8.RuneCountInString(body) > models.MaxNoteLength {
		return nil, cc.ErrInvalidPayload
	}
	app, err := s.repo.FindLoanApplicationByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if app == nil {
		return nil, cc.ErrApplicationNotFound
	}
	audit := actor.Log(cc.AUDIT_ADDED_LOAN_APPLICATION_NOTE, models.FormatID(cc.LOAN_APPLICATION_ID_PREFIX, id))
	return s.repo.InsertLoanApplicationNote(ctx, id, actor.UserPanelID, actor.Name, body, audit)
}

// findNote resolves rawID/rawNoteID to an existing note on that application.
func (s *ApplicationsService) findNote(ctx context.Context, rawID, rawNoteID string) (int64, *models.LoanApplicationNote, error) {
	id, ok := models.ParseID(cc.LOAN_APPLICATION_ID_PREFIX, rawID)
	if !ok {
		return 0, nil, cc.ErrApplicationNotFound
	}
	noteID, err := strconv.ParseInt(rawNoteID, 10, 64)
	if err != nil || noteID <= 0 {
		return 0, nil, cc.ErrNoteNotFound
	}
	note, err := s.repo.FindLoanApplicationNote(ctx, id, noteID)
	if err != nil {
		return 0, nil, err
	}
	if note == nil {
		return 0, nil, cc.ErrNoteNotFound
	}
	return id, note, nil
}

// EditLoanApplicationNote replaces a note's text. Only its author may.
func (s *ApplicationsService) EditLoanApplicationNote(ctx context.Context, rawID, rawNoteID string, req *models.AddLoanNoteRequest, actor audit_models.Actor) (*models.LoanApplicationNote, error) {
	body := strings.TrimSpace(req.Body)
	if body == "" {
		return nil, cc.ErrNoteEmpty
	}
	if utf8.RuneCountInString(body) > models.MaxNoteLength {
		return nil, cc.ErrInvalidPayload
	}
	id, note, err := s.findNote(ctx, rawID, rawNoteID)
	if err != nil {
		return nil, err
	}
	if note.AuthorPanelID == 0 || note.AuthorPanelID != actor.UserPanelID {
		return nil, cc.ErrNoteEditForbidden
	}
	noteID, _ := strconv.ParseInt(note.ID, 10, 64)
	audit := actor.Log(cc.AUDIT_EDITED_LOAN_APPLICATION_NOTE, models.FormatID(cc.LOAN_APPLICATION_ID_PREFIX, id))
	return s.repo.UpdateLoanApplicationNote(ctx, id, noteID, body, audit)
}

// DeleteLoanApplicationNote removes a note. Its author may, and so may an
// admin (isAdmin: super admin or admin).
func (s *ApplicationsService) DeleteLoanApplicationNote(ctx context.Context, rawID, rawNoteID string, actor audit_models.Actor, isAdmin bool) error {
	id, note, err := s.findNote(ctx, rawID, rawNoteID)
	if err != nil {
		return err
	}
	mine := note.AuthorPanelID != 0 && note.AuthorPanelID == actor.UserPanelID
	if !mine && !isAdmin {
		return cc.ErrNoteDeleteForbidden
	}
	noteID, _ := strconv.ParseInt(note.ID, 10, 64)
	audit := actor.Log(cc.AUDIT_DELETED_LOAN_APPLICATION_NOTE, models.FormatID(cc.LOAN_APPLICATION_ID_PREFIX, id))
	return s.repo.DeleteLoanApplicationNote(ctx, id, noteID, audit)
}

// ReviewLoanApplication records the decision (plus audit entry) and returns
// the updated application.
func (s *ApplicationsService) ReviewLoanApplication(ctx context.Context, rawID string, decision *models.ReviewDecisionRequest, actor audit_models.Actor) (*models.LoanApplication, error) {
	id, ok := models.ParseID(cc.LOAN_APPLICATION_ID_PREFIX, rawID)
	if !ok {
		return nil, cc.ErrApplicationNotFound
	}

	action := cc.AUDIT_REJECTED_LOAN_APPLICATION
	if decision.Status == cc.APPLICATION_STATUS_APPROVED {
		action = cc.AUDIT_APPROVED_LOAN_APPLICATION
	}

	audit := actor.Log(action, models.FormatID(cc.LOAN_APPLICATION_ID_PREFIX, id))
	if err := s.repo.ReviewLoanApplication(ctx, id, decision, actor.UserPanelID, audit); err != nil {
		return nil, err
	}

	return s.GetLoanApplication(ctx, rawID)
}

func (s *ApplicationsService) ListMembershipApplications(ctx context.Context, status, search string) ([]models.MembershipApplication, error) {
	return s.repo.ListMembershipApplications(ctx, status, strings.TrimSpace(search))
}

func (s *ApplicationsService) GetMembershipApplication(ctx context.Context, rawID string) (*models.MembershipApplication, error) {
	id, ok := models.ParseID(cc.MEMBERSHIP_APPLICATION_ID_PREFIX, rawID)
	if !ok {
		return nil, cc.ErrApplicationNotFound
	}
	app, err := s.repo.FindMembershipApplicationByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if app == nil {
		return nil, cc.ErrApplicationNotFound
	}
	return app, nil
}

// CreateMembershipApplication saves a walk-in/phone request as a pending
// application. It applies the public form's rules that struct tags can't:
// a valid PH mobile, no future birth date, and the coop-only fields.
func (s *ApplicationsService) CreateMembershipApplication(ctx context.Context, req *models.NewMembershipApplicationRequest, actor audit_models.Actor) (*models.MembershipApplication, error) {
	req.FirstName = strings.TrimSpace(req.FirstName)
	req.LastName = strings.TrimSpace(req.LastName)
	req.MiddleName = strings.TrimSpace(req.MiddleName)
	req.Suffix = strings.TrimSpace(req.Suffix)
	req.Email = strings.TrimSpace(req.Email)
	req.MobileNumber = utils.CompactMobile(req.MobileNumber)
	req.TypeOfWork = strings.TrimSpace(req.TypeOfWork)

	if req.FirstName == "" || req.LastName == "" {
		return nil, cc.ErrInvalidPayload
	}
	if utils.IsAfterToday(req.DateOfBirth) {
		return nil, cc.ErrDateOfBirthInFuture
	}
	if !utils.IsValidPhMobile(req.MobileNumber) {
		return nil, cc.ErrInvalidMobileNumber
	}
	if req.MembershipType == cc.MEMBERSHIP_TYPE_COOP {
		if req.Email == "" || req.SourceOfIncome == "" || req.TypeOfWork == "" {
			return nil, cc.ErrCoopApplicantIncomplete
		}
	} else {
		// Laboratory (minor) applicants don't declare income.
		req.SourceOfIncome, req.TypeOfWork = "", ""
	}

	audit := actor.Log(cc.AUDIT_ADDED_MEMBERSHIP_APPLICATION, "")
	id, err := s.repo.CreateMembershipApplication(ctx, req, audit)
	if err != nil {
		return nil, err
	}
	return s.GetMembershipApplication(ctx, models.FormatID(cc.MEMBERSHIP_APPLICATION_ID_PREFIX, id))
}

func (s *ApplicationsService) ReviewMembershipApplication(ctx context.Context, rawID string, decision *models.ReviewDecisionRequest, actor audit_models.Actor) (*models.MembershipApplication, error) {
	id, ok := models.ParseID(cc.MEMBERSHIP_APPLICATION_ID_PREFIX, rawID)
	if !ok {
		return nil, cc.ErrApplicationNotFound
	}

	action := cc.AUDIT_REJECTED_MEMBERSHIP_APPLICATION
	if decision.Status == cc.APPLICATION_STATUS_APPROVED {
		action = cc.AUDIT_APPROVED_MEMBERSHIP_APPLICATION
	}

	audit := actor.Log(action, models.FormatID(cc.MEMBERSHIP_APPLICATION_ID_PREFIX, id))
	if err := s.repo.ReviewMembershipApplication(ctx, id, decision, actor.UserPanelID, audit); err != nil {
		return nil, err
	}

	return s.GetMembershipApplication(ctx, rawID)
}
