package services

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	cc "fairchild-admin-panel-be/internal/constants"
	interfaces "fairchild-admin-panel-be/internal/interfaces"
	app_models "fairchild-admin-panel-be/internal/models/applications"
	audit_models "fairchild-admin-panel-be/internal/models/audit"
	models "fairchild-admin-panel-be/internal/models/blasts"
	config_models "fairchild-admin-panel-be/internal/models/config"
	member_models "fairchild-admin-panel-be/internal/models/members"
	portal_models "fairchild-admin-panel-be/internal/models/portal"
	"fairchild-admin-panel-be/internal/services/messaging"
	"fairchild-admin-panel-be/internal/utils"
)

const (
	// batchSize is how many messages a worker claims at a time.
	batchSize = 50
	// idlePoll is how often an idle worker looks for a new blast (a new blast
	// on this instance wakes it straight away).
	idlePoll = 15 * time.Second
	// sendTimeout bounds one provider call.
	sendTimeout = 20 * time.Second
	// failuresShown is how many failed messages a blast's detail lists.
	failuresShown = 100
)

type BlastsService struct {
	repo     interfaces.BlastsRepositoryInterface
	branches interfaces.BranchResolverInterface
	email    messaging.EmailSender // nil = no email blasts
	sms      messaging.SMSSender   // nil = no SMS blasts
	conf     config_models.BlastConfig
	// allowed holds BLAST_TEST_RECIPIENTS, normalised like recipients.
	allowed map[string]bool
	wake    chan struct{}
}

// NewBlastsService starts the background worker that sends queued blasts,
// including any left sending when the server last stopped.
func NewBlastsService(
	repo interfaces.BlastsRepositoryInterface,
	branches interfaces.BranchResolverInterface,
	email messaging.EmailSender,
	sms messaging.SMSSender,
	conf config_models.BlastConfig,
) *BlastsService {
	s := &BlastsService{
		repo: repo, branches: branches, email: email, sms: sms, conf: conf,
		allowed: map[string]bool{}, wake: make(chan struct{}, 1),
	}
	for _, r := range conf.TestRecipients {
		if addr, ok := normaliseAddress(r); ok {
			s.allowed[addr] = true
		}
	}
	go s.work()
	return s
}

// normaliseAddress lowercases an email or turns a mobile number into E.164.
func normaliseAddress(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if strings.Contains(raw, "@") {
		addr := strings.ToLower(raw)
		return addr, utils.Validate.Var(addr, "email") == nil
	}
	return messaging.NormalizeMobileNumber(raw)
}

func (s *BlastsService) channelStatus() models.ChannelStatus {
	return models.ChannelStatus{Email: s.email != nil, SMS: s.sms != nil}
}

func usesEmail(channel string) bool {
	return channel == models.ChannelEmail || channel == models.ChannelBoth
}
func usesSMS(channel string) bool {
	return channel == models.ChannelSMS || channel == models.ChannelBoth
}

func invalid(msg string) error { return &cc.BlastInputError{Msg: msg} }

// checkAudience validates the channel and filters and fills in defaults.
func (s *BlastsService) checkAudience(req *models.AudienceRequest) error {
	if !slices.Contains(models.Channels, req.Channel) {
		return invalid("Pick email, SMS, or both")
	}
	if usesEmail(req.Channel) && s.email == nil {
		return cc.ErrBlastEmailUnavailable
	}
	if usesSMS(req.Channel) && s.sms == nil {
		return cc.ErrBlastSMSUnavailable
	}
	req.Statuses = dedupe(req.Statuses)
	req.Branches = dedupe(req.Branches)
	if len(req.Statuses) == 0 {
		return invalid("Pick at least one account status")
	}
	for _, st := range req.Statuses {
		if !slices.Contains(portal_models.Statuses, st) {
			return invalid("Account status must be one of: active, pending_review, inactive, suspended")
		}
	}
	for _, b := range req.Branches {
		if !slices.Contains(member_models.Branches, b) {
			return invalid("Branch must be one of: main, knowles, tiic")
		}
	}
	return nil
}

func dedupe(values []string) []string {
	out := []string{}
	for _, v := range values {
		if v = strings.TrimSpace(v); v != "" && !slices.Contains(out, v) {
			out = append(out, v)
		}
	}
	return out
}

// checkContent validates the draft for the channels it goes out on.
func checkContent(channel string, subject, emailBody, smsBody *string) error {
	*subject, *emailBody, *smsBody = strings.TrimSpace(*subject), strings.TrimSpace(*emailBody), strings.TrimSpace(*smsBody)
	if usesEmail(channel) {
		if *subject == "" || utf8.RuneCountInString(*subject) > models.MaxSubjectLength {
			return invalid(fmt.Sprintf("Enter an email subject (up to %d characters)", models.MaxSubjectLength))
		}
		if *emailBody == "" || utf8.RuneCountInString(*emailBody) > models.MaxEmailBodyLength {
			return invalid(fmt.Sprintf("Enter the email message (up to %d characters)", models.MaxEmailBodyLength))
		}
	} else {
		*subject, *emailBody = "", ""
	}
	if usesSMS(channel) {
		if *smsBody == "" || utf8.RuneCountInString(*smsBody) > models.MaxSMSLength {
			return invalid(fmt.Sprintf("Enter the text message (up to %d characters)", models.MaxSMSLength))
		}
	} else {
		*smsBody = ""
	}
	return nil
}

// audience returns the members matching the filters, plus how many match
// each branch (before the branch filter, for the picker).
func (s *BlastsService) audience(ctx context.Context, req models.AudienceRequest) ([]models.AudienceMember, map[string]int, error) {
	all, err := s.repo.ListAudience(ctx, req.Statuses)
	if err != nil {
		return nil, nil, err
	}
	branchOf, err := s.branches.BranchResolver(ctx)
	if err != nil {
		return nil, nil, err
	}

	counts := make(map[string]int, len(member_models.Branches))
	for _, b := range member_models.Branches {
		counts[b] = 0
	}
	members := all[:0:0]
	for _, m := range all {
		branch, onLedger := branchOf(m.MemberNo, m.ClientID)
		if onLedger {
			counts[branch]++
		}
		if len(req.Branches) == 0 || (onLedger && slices.Contains(req.Branches, branch)) {
			members = append(members, m)
		}
	}
	return members, counts, nil
}

// recipients turns members into one message per member per channel. An
// address shared by several accounts (a guardian's number, say) gets one
// message. Members with no usable address for any channel are counted.
func (s *BlastsService) recipients(members []models.AudienceMember, channel string) ([]models.NewRecipient, int) {
	seen := map[string]bool{}
	var out []models.NewRecipient
	noContact := 0
	for _, m := range members {
		reachable := false
		add := func(ch, raw string) {
			addr, ok := normaliseAddress(raw)
			if raw == "" || !ok || (ch == models.ChannelEmail) != strings.Contains(addr, "@") {
				return
			}
			reachable = true
			if seen[ch+addr] {
				return
			}
			seen[ch+addr] = true
			out = append(out, models.NewRecipient{
				UserID: m.UserID, MemberNo: m.MemberNo, Name: m.Name, Channel: ch, Recipient: addr,
				Skip: s.conf.TestMode && !s.allowed[addr],
			})
		}
		if usesEmail(channel) {
			add(models.ChannelEmail, m.Email)
		}
		if usesSMS(channel) {
			add(models.ChannelSMS, m.Mobile)
		}
		if !reachable {
			noContact++
		}
	}
	return out, noContact
}

func (s *BlastsService) Preview(ctx context.Context, req models.AudienceRequest) (*models.AudiencePreview, error) {
	if err := s.checkAudience(&req); err != nil {
		return nil, err
	}
	members, branchCounts, err := s.audience(ctx, req)
	if err != nil {
		return nil, err
	}
	recipients, noContact := s.recipients(members, req.Channel)

	p := &models.AudiencePreview{
		Members: len(members), NoContact: noContact, BranchCounts: branchCounts, TestMode: s.conf.TestMode,
	}
	for _, r := range recipients {
		if r.Channel == models.ChannelEmail {
			p.Email++
		} else {
			p.SMS++
		}
		if !r.Skip {
			p.Deliverable++
		}
	}
	return p, nil
}

// Create queues a blast to everyone in the audience and wakes the worker.
func (s *BlastsService) Create(ctx context.Context, req *models.CreateBlastRequest, actor audit_models.Actor) (*models.BlastDetail, error) {
	if err := s.checkAudience(&req.AudienceRequest); err != nil {
		return nil, err
	}
	if err := checkContent(req.Channel, &req.Subject, &req.EmailBody, &req.SMSBody); err != nil {
		return nil, err
	}
	members, _, err := s.audience(ctx, req.AudienceRequest)
	if err != nil {
		return nil, err
	}
	recipients, _ := s.recipients(members, req.Channel)
	if len(recipients) == 0 {
		return nil, cc.ErrBlastNoRecipients
	}

	blast := &models.Blast{
		Channel: req.Channel, Subject: req.Subject, EmailBody: req.EmailBody, SMSBody: req.SMSBody,
		Audience: models.BlastAudience{Statuses: req.Statuses, Branches: req.Branches},
		TestMode: s.conf.TestMode, CreatedBy: actor.Name,
	}
	id, err := s.repo.Create(ctx, blast, actor.UserPanelID, recipients, actor.Log(cc.AUDIT_STARTED_BLAST, ""))
	if err != nil {
		return nil, err
	}
	select {
	case s.wake <- struct{}{}:
	default:
	}
	return s.get(ctx, id)
}

func (s *BlastsService) List(ctx context.Context, page, pageSize int) (*models.BlastPage, error) {
	items, total, err := s.repo.List(ctx, page, pageSize)
	if err != nil {
		return nil, err
	}
	return &models.BlastPage{
		Items: items, Total: total, Page: page, PageSize: pageSize,
		TestMode: s.conf.TestMode, Channels: s.channelStatus(),
	}, nil
}

func (s *BlastsService) Get(ctx context.Context, rawID string) (*models.BlastDetail, error) {
	id, ok := app_models.ParseID(cc.MESSAGE_BLAST_ID_PREFIX, rawID)
	if !ok {
		return nil, cc.ErrBlastNotFound
	}
	return s.get(ctx, id)
}

func (s *BlastsService) get(ctx context.Context, id int64) (*models.BlastDetail, error) {
	b, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if b == nil {
		return nil, cc.ErrBlastNotFound
	}
	failures, err := s.repo.Failures(ctx, id, failuresShown)
	if err != nil {
		return nil, err
	}
	return &models.BlastDetail{Blast: *b, Failures: failures}, nil
}

func (s *BlastsService) Cancel(ctx context.Context, rawID string, actor audit_models.Actor) (*models.BlastDetail, error) {
	id, ok := app_models.ParseID(cc.MESSAGE_BLAST_ID_PREFIX, rawID)
	if !ok {
		return nil, cc.ErrBlastNotFound
	}
	if err := s.repo.Cancel(ctx, id, actor.Log(cc.AUDIT_CANCELLED_BLAST, "")); err != nil {
		return nil, err
	}
	return s.get(ctx, id)
}

// SendTest sends the draft once to the addresses the admin typed, with
// {name} filled in from the admin's own name.
func (s *BlastsService) SendTest(ctx context.Context, req *models.TestBlastRequest, actor audit_models.Actor) (models.TestBlastResult, error) {
	audience := models.AudienceRequest{Channel: req.Channel, Statuses: []string{portal_models.Statuses[0]}}
	if err := s.checkAudience(&audience); err != nil {
		return nil, err
	}
	if err := checkContent(req.Channel, &req.Subject, &req.EmailBody, &req.SMSBody); err != nil {
		return nil, err
	}
	email, emailOK := normaliseAddress(req.Email)
	mobile, mobileOK := normaliseAddress(req.Mobile)
	if (usesEmail(req.Channel) && (!emailOK || !strings.Contains(email, "@"))) ||
		(usesSMS(req.Channel) && (!mobileOK || strings.Contains(mobile, "@"))) {
		return nil, cc.ErrBlastTestAddress
	}

	job := models.BlastJob{Subject: req.Subject, EmailBody: req.EmailBody, SMSBody: req.SMSBody}
	result := models.TestBlastResult{}
	var targets []string
	if usesEmail(req.Channel) {
		result[models.ChannelEmail] = models.RecipientSent
		if err := s.send(ctx, job, models.ChannelEmail, email, actor.Name); err != nil {
			slog.Error("test blast email failed", "err", err)
			result[models.ChannelEmail] = models.RecipientFailed
		}
		targets = append(targets, email)
	}
	if usesSMS(req.Channel) {
		result[models.ChannelSMS] = models.RecipientSent
		if err := s.send(ctx, job, models.ChannelSMS, mobile, actor.Name); err != nil {
			slog.Error("test blast SMS failed", "err", err)
			result[models.ChannelSMS] = models.RecipientFailed
		}
		targets = append(targets, mobile)
	}

	if err := s.repo.InsertAudit(context.WithoutCancel(ctx), actor.Log(cc.AUDIT_SENT_TEST_BLAST, strings.Join(targets, ", "))); err != nil {
		slog.Error("test blast sent but not audited", "err", err)
	}
	for _, outcome := range result {
		if outcome == models.RecipientSent {
			return result, nil
		}
	}
	return nil, cc.ErrBlastTestSendFailed
}

// personalise fills {name} with the member's first name.
func personalise(text, fullName string) string {
	name := messaging.FriendlyFirstName(fullName)
	if name == "" {
		name = "Member"
	}
	return strings.ReplaceAll(text, models.NamePlaceholder, name)
}

func (s *BlastsService) send(ctx context.Context, job models.BlastJob, channel, to, name string) error {
	ctx, cancel := context.WithTimeout(ctx, sendTimeout)
	defer cancel()
	if channel == models.ChannelEmail {
		html, err := messaging.RenderAnnouncement(messaging.AnnouncementData{
			Subject: personalise(job.Subject, name),
			Body:    personalise(job.EmailBody, name),
		})
		if err != nil {
			return err
		}
		return s.email.SendEmail(ctx, to, personalise(job.Subject, name), html)
	}
	return s.sms.SendSMS(ctx, to, personalise(job.SMSBody, name))
}

// work sends queued blasts forever: oldest blast first, a batch of messages
// at a time, each channel paced to its per-minute rate.
func (s *BlastsService) work() {
	ctx := context.Background()
	gap := map[string]time.Duration{
		models.ChannelEmail: time.Minute / time.Duration(s.conf.EmailsPerMin),
		models.ChannelSMS:   time.Minute / time.Duration(s.conf.SMSPerMin),
	}
	last := map[string]time.Time{}

	for {
		busy, err := s.workOnce(ctx, gap, last)
		if err != nil {
			slog.Error("blast worker", "err", err)
			time.Sleep(idlePoll)
			continue
		}
		if !busy {
			select {
			case <-s.wake:
			case <-time.After(idlePoll):
			}
		}
	}
}

// workOnce sends one batch of the oldest sending blast. busy is false when
// there was nothing to send.
func (s *BlastsService) workOnce(ctx context.Context, gap map[string]time.Duration, last map[string]time.Time) (busy bool, err error) {
	job, err := s.repo.NextSendingBlast(ctx)
	if err != nil || job == nil {
		return false, err
	}
	batch, err := s.repo.ClaimBatch(ctx, job.ID, batchSize)
	if err != nil {
		return false, err
	}
	if len(batch) == 0 {
		// Everything is sent, or another instance is sending the rest.
		return false, s.repo.FinishIfDone(ctx, job.ID)
	}

	for _, m := range batch {
		if wait := time.Until(last[m.Channel].Add(gap[m.Channel])); wait > 0 {
			time.Sleep(wait)
		}
		// Checked after the wait, right before sending, so a cancel during
		// the wait stops this message. Cancel has already marked it skipped.
		status, err := s.repo.BlastStatus(ctx, job.ID)
		if err != nil {
			return true, err
		}
		if status != models.StatusSending {
			continue
		}
		last[m.Channel] = time.Now()

		outcome, errMsg := models.RecipientSent, ""
		if err := s.sendQueued(ctx, *job, m); err != nil {
			slog.Error("blast message failed", "blast", job.ID, "recipient", m.ID, "channel", m.Channel, "err", err)
			outcome, errMsg = models.RecipientFailed, err.Error()
		}
		if err := s.repo.MarkResult(ctx, m.ID, outcome, errMsg); err != nil {
			return true, err
		}
	}
	return true, s.repo.FinishIfDone(ctx, job.ID)
}

func (s *BlastsService) sendQueued(ctx context.Context, job models.BlastJob, m models.QueuedMessage) error {
	if (m.Channel == models.ChannelEmail && s.email == nil) || (m.Channel == models.ChannelSMS && s.sms == nil) {
		return fmt.Errorf("%s isn't set up on the server", m.Channel)
	}
	return s.send(ctx, job, m.Channel, m.Recipient, m.Name)
}
