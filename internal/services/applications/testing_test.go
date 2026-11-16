package services

import (
	"context"
	"errors"
	cc "fairchild-admin-panel-be/internal/constants"
	"fairchild-admin-panel-be/internal/interfaces"
	models "fairchild-admin-panel-be/internal/models/applications"
	audit "fairchild-admin-panel-be/internal/models/audit"
	"testing"
)

type reviewRepo struct {
	interfaces.ApplicationsRepositoryInterface
	id       int64
	reviewer int64
	audit    *audit.AuditLog
	err      error
}

func (r *reviewRepo) FindLoanApplicationByID(_ context.Context, id int64) (*models.LoanApplication, error) {
	if r.err != nil {
		return nil, r.err
	}
	if id != 42 {
		return nil, nil
	}
	return &models.LoanApplication{}, nil
}
func (r *reviewRepo) FindLoanApplicant(context.Context, int64) (*models.LoanApplicant, error) {
	return nil, nil
}
func (r *reviewRepo) ListMemberLoanApplications(context.Context, string, int64) ([]models.LoanApplication, error) {
	return nil, nil
}
func (r *reviewRepo) ListLoanApplicationNotes(context.Context, int64) ([]models.LoanApplicationNote, error) {
	return nil, nil
}
func (r *reviewRepo) ReviewLoanApplication(_ context.Context, id int64, _ *models.ReviewDecisionRequest, reviewer int64, a *audit.AuditLog) error {
	r.id, r.reviewer, r.audit = id, reviewer, a
	return r.err
}
func TestApplicationReviewAndErrors(t *testing.T) {
	ctx := context.Background()
	r := &reviewRepo{}
	s := NewApplicationsService(r, nil, nil)
	actor := audit.Actor{UserPanelID: 7, Name: "Reviewer", IP: "192.0.2.1"}
	if _, err := s.ReviewLoanApplication(ctx, "LA-00042", &models.ReviewDecisionRequest{Status: cc.APPLICATION_STATUS_APPROVED}, actor); err != nil {
		t.Fatal(err)
	}
	if r.id != 42 || r.reviewer != 7 || r.audit == nil || r.audit.Action != cc.AUDIT_APPROVED_LOAN_APPLICATION {
		t.Fatalf("bad audited review: %+v", r)
	}
	if _, err := s.GetLoanApplication(ctx, "43"); !errors.Is(err, cc.ErrApplicationNotFound) {
		t.Fatal(err)
	}
	failure := errors.New("repository unavailable")
	r.err = failure
	if _, err := s.GetLoanApplication(ctx, "42"); !errors.Is(err, failure) {
		t.Fatal(err)
	}
}
func TestSecurityApplicationIDs(t *testing.T) {
	s := NewApplicationsService(nil, nil, nil)
	for _, id := range []string{"x", "-1", "0", "42 OR 1=1", "LA-99999999999999999999999999"} {
		if _, err := s.GetLoanApplication(context.Background(), id); !errors.Is(err, cc.ErrApplicationNotFound) {
			t.Fatalf("%q: %v", id, err)
		}
	}
}
func BenchmarkApplications(b *testing.B) {
	s := NewApplicationsService(&reviewRepo{}, nil, nil)
	ctx := context.Background()
	b.ReportAllocs()
	for b.Loop() {
		if _, err := s.GetLoanApplication(ctx, "LA-00042"); err != nil {
			b.Fatal(err)
		}
	}
}

// fakeSender records what it was asked to send.
type fakeSender struct{ to, subject, text string }

func (f *fakeSender) SendEmail(_ context.Context, to, subject, _ string) error {
	f.to, f.subject = to, subject
	return nil
}
func (f *fakeSender) SendSMS(_ context.Context, to, text string) error {
	f.to, f.text = to, text
	return nil
}

// messageRepo serves one application whose applicant has the given contacts.
type messageRepo struct {
	reviewRepo
	applicant *models.LoanApplicant
	audits    []*audit.AuditLog
}

func (r *messageRepo) FindLoanApplicationByID(context.Context, int64) (*models.LoanApplication, error) {
	return &models.LoanApplication{ID: "LA-00042", MemberID: "000044"}, nil
}
func (r *messageRepo) FindLoanApplicant(context.Context, int64) (*models.LoanApplicant, error) {
	return r.applicant, nil
}
func (r *messageRepo) RecordApplicantMessage(_ context.Context, a *audit.AuditLog) error {
	r.audits = append(r.audits, a)
	return nil
}

func TestMessageLoanApplicant(t *testing.T) {
	ctx := context.Background()
	actor := audit.Actor{UserPanelID: 7, Name: "Officer"}
	repo := &messageRepo{applicant: &models.LoanApplicant{Email: "member@example.com", MobileNumber: "09171234567"}}
	email, sms := &fakeSender{}, &fakeSender{}
	s := NewApplicationsService(repo, email, sms)

	if _, err := s.MessageLoanApplicant(ctx, "LA-00042", &models.MessageApplicantRequest{Channel: "email", Body: "Hi"}, actor); err != nil {
		t.Fatal(err)
	}
	if email.to != "member@example.com" || email.subject != "About your loan application LA-00042" {
		t.Errorf("email = %+v", email)
	}

	res, err := s.MessageLoanApplicant(ctx, "LA-00042", &models.MessageApplicantRequest{Channel: "sms", Body: "Please visit the branch"}, actor)
	if err != nil {
		t.Fatal(err)
	}
	if res.Recipient != sms.to || sms.text != "FCCMPC: Please visit the branch" {
		t.Errorf("sms = %+v, result = %+v", sms, res)
	}
	if len(repo.audits) != 2 || repo.audits[0].Action != cc.AUDIT_EMAILED_LOAN_APPLICANT ||
		repo.audits[1].Action != cc.AUDIT_TEXTED_LOAN_APPLICANT || repo.audits[1].Target != "LA-00042" {
		t.Errorf("audits = %+v", repo.audits)
	}
}

func TestMessageLoanApplicantRefused(t *testing.T) {
	ctx := context.Background()
	actor := audit.Actor{UserPanelID: 7}
	noContacts := NewApplicationsService(&messageRepo{}, &fakeSender{}, &fakeSender{})
	unconfigured := NewApplicationsService(&messageRepo{applicant: &models.LoanApplicant{Email: "a@b.co"}}, nil, nil)

	for name, c := range map[string]struct {
		s    *ApplicationsService
		req  models.MessageApplicantRequest
		want error
	}{
		"no email":   {noContacts, models.MessageApplicantRequest{Channel: "email", Body: "x"}, cc.ErrApplicantNoEmail},
		"no mobile":  {noContacts, models.MessageApplicantRequest{Channel: "sms", Body: "x"}, cc.ErrApplicantNoMobile},
		"blank":      {noContacts, models.MessageApplicantRequest{Channel: "sms", Body: "  "}, cc.ErrMessageEmpty},
		"no smtp":    {unconfigured, models.MessageApplicantRequest{Channel: "email", Body: "x"}, cc.ErrMessageEmailUnavailable},
		"no movider": {unconfigured, models.MessageApplicantRequest{Channel: "sms", Body: "x"}, cc.ErrMessageSMSUnavailable},
	} {
		if _, err := c.s.MessageLoanApplicant(ctx, "LA-00042", &c.req, actor); !errors.Is(err, c.want) {
			t.Errorf("%s: err = %v, want %v", name, err, c.want)
		}
	}
}

// noteRepo stores notes for application 42.
type noteRepo struct {
	reviewRepo
	notes []models.LoanApplicationNote
	audit *audit.AuditLog
}

func (r *noteRepo) InsertLoanApplicationNote(_ context.Context, id, authorID int64, author, body string, a *audit.AuditLog) (*models.LoanApplicationNote, error) {
	n := models.LoanApplicationNote{ID: "1", Author: author, Body: body}
	r.notes, r.audit = append(r.notes, n), a
	return &n, nil
}

func TestAddLoanApplicationNote(t *testing.T) {
	ctx := context.Background()
	repo := &noteRepo{}
	s := NewApplicationsService(repo, nil, nil)
	actor := audit.Actor{UserPanelID: 7, Name: "Loans Officer"}

	note, err := s.AddLoanApplicationNote(ctx, "LA-00042", &models.AddLoanNoteRequest{Body: "  Called the member, **docs** to follow  "}, actor)
	if err != nil {
		t.Fatal(err)
	}
	if note.Author != "Loans Officer" || note.Body != "Called the member, **docs** to follow" {
		t.Errorf("note = %+v", note)
	}
	if repo.audit == nil || repo.audit.Action != cc.AUDIT_ADDED_LOAN_APPLICATION_NOTE || repo.audit.Target != "LA-00042" {
		t.Errorf("audit = %+v", repo.audit)
	}

	if _, err := s.AddLoanApplicationNote(ctx, "LA-00042", &models.AddLoanNoteRequest{Body: "   "}, actor); !errors.Is(err, cc.ErrNoteEmpty) {
		t.Errorf("blank note err = %v", err)
	}
	if _, err := s.AddLoanApplicationNote(ctx, "LA-00043", &models.AddLoanNoteRequest{Body: "x"}, actor); !errors.Is(err, cc.ErrApplicationNotFound) {
		t.Errorf("missing application err = %v", err)
	}
}

// ownedNoteRepo has note 5 on application 42, written by panel user 7.
type ownedNoteRepo struct {
	noteRepo
	deleted bool
}

func (r *ownedNoteRepo) FindLoanApplicationNote(_ context.Context, id, noteID int64) (*models.LoanApplicationNote, error) {
	if id != 42 || noteID != 5 {
		return nil, nil
	}
	return &models.LoanApplicationNote{ID: "5", AuthorPanelID: 7, Author: "Loans Officer", Body: "old"}, nil
}
func (r *ownedNoteRepo) UpdateLoanApplicationNote(_ context.Context, _, _ int64, body string, a *audit.AuditLog) (*models.LoanApplicationNote, error) {
	r.audit = a
	return &models.LoanApplicationNote{ID: "5", Body: body}, nil
}
func (r *ownedNoteRepo) DeleteLoanApplicationNote(_ context.Context, _, _ int64, a *audit.AuditLog) error {
	r.deleted, r.audit = true, a
	return nil
}

func TestEditAndDeleteLoanApplicationNote(t *testing.T) {
	ctx := context.Background()
	author := audit.Actor{UserPanelID: 7, Name: "Loans Officer"}
	other := audit.Actor{UserPanelID: 8, Name: "Someone Else"}
	edit := &models.AddLoanNoteRequest{Body: " new text "}

	repo := &ownedNoteRepo{}
	s := NewApplicationsService(repo, nil, nil)

	if _, err := s.EditLoanApplicationNote(ctx, "LA-00042", "5", edit, other); !errors.Is(err, cc.ErrNoteEditForbidden) {
		t.Errorf("edit by someone else: err = %v", err)
	}
	note, err := s.EditLoanApplicationNote(ctx, "LA-00042", "5", edit, author)
	if err != nil || note.Body != "new text" || repo.audit.Action != cc.AUDIT_EDITED_LOAN_APPLICATION_NOTE {
		t.Errorf("edit by author: note = %+v, err = %v, audit = %+v", note, err, repo.audit)
	}
	if _, err := s.EditLoanApplicationNote(ctx, "LA-00042", "99", edit, author); !errors.Is(err, cc.ErrNoteNotFound) {
		t.Errorf("edit missing note: err = %v", err)
	}

	if err := s.DeleteLoanApplicationNote(ctx, "LA-00042", "5", other, false); !errors.Is(err, cc.ErrNoteDeleteForbidden) || repo.deleted {
		t.Errorf("delete by someone else: err = %v, deleted = %v", err, repo.deleted)
	}
	if err := s.DeleteLoanApplicationNote(ctx, "LA-00042", "5", other, true); err != nil || !repo.deleted {
		t.Errorf("delete by an admin: err = %v, deleted = %v", err, repo.deleted)
	}
	if repo.audit.Action != cc.AUDIT_DELETED_LOAN_APPLICATION_NOTE || repo.audit.UserPanelID != 8 {
		t.Errorf("delete audit = %+v", repo.audit)
	}
}
