package models

// Blast channels.
const (
	ChannelEmail = "email"
	ChannelSMS   = "sms"
	ChannelBoth  = "both"
)

// Channels lists the channels a blast can go out on.
var Channels = []string{ChannelEmail, ChannelSMS, ChannelBoth}

// Blast statuses: sending until every recipient has an outcome, or until
// an admin cancels it.
const (
	StatusSending   = "sending"
	StatusCompleted = "completed"
	StatusCancelled = "cancelled"
)

// Recipient statuses. "sending" is a worker's claim on a pending row.
const (
	RecipientPending = "pending"
	RecipientSending = "sending"
	RecipientSent    = "sent"
	RecipientFailed  = "failed"
	RecipientSkipped = "skipped"
)

// Limits on what an admin can write.
const (
	MaxSubjectLength   = 200
	MaxEmailBodyLength = 5000
	MaxSMSLength       = 480
)

// NamePlaceholder in a subject, email or text is replaced with each
// member's first name.
const NamePlaceholder = "{name}"

// BlastAudience is who a blast goes to: live portal accounts with one of
// these statuses, on one of these branches (none = every branch, including
// accounts on no ledger).
type BlastAudience struct {
	Statuses []string `json:"statuses"`
	Branches []string `json:"branches"`
}

// BlastCounts tallies recipients (messages, one per member per channel) by
// outcome. Pending includes ones a worker is sending right now.
type BlastCounts struct {
	Total   int `json:"total"`
	Pending int `json:"pending"`
	Sent    int `json:"sent"`
	Failed  int `json:"failed"`
	Skipped int `json:"skipped"`
}

func (c *BlastCounts) Add(status string, n int) {
	c.Total += n
	switch status {
	case RecipientSent:
		c.Sent += n
	case RecipientFailed:
		c.Failed += n
	case RecipientSkipped:
		c.Skipped += n
	default:
		c.Pending += n
	}
}

// Blast matches Blast in the admin panel (src/models/admin.ts). Timestamps
// are RFC 3339. ByChannel has an entry for each channel the blast uses.
type Blast struct {
	ID         string                 `json:"id"`
	Channel    string                 `json:"channel"`
	Subject    string                 `json:"subject,omitempty"`
	EmailBody  string                 `json:"emailBody,omitempty"`
	SMSBody    string                 `json:"smsBody,omitempty"`
	Audience   BlastAudience          `json:"audience"`
	Status     string                 `json:"status"`
	TestMode   bool                   `json:"testMode"`
	CreatedBy  string                 `json:"createdBy"`
	CreatedAt  string                 `json:"createdAt"`
	FinishedAt string                 `json:"finishedAt,omitempty"`
	Counts     BlastCounts            `json:"counts"`
	ByChannel  map[string]BlastCounts `json:"byChannel"`
}

// BlastFailure is one message that failed or was skipped, for the detail
// view.
type BlastFailure struct {
	MemberNo  string `json:"memberNo,omitempty"`
	Name      string `json:"name"`
	Channel   string `json:"channel"`
	Recipient string `json:"recipient"`
	Status    string `json:"status"`
	Error     string `json:"error,omitempty"`
}

// BlastDetail is GET /admin/blasts/:id: the blast plus its most recent
// failed messages.
type BlastDetail struct {
	Blast
	Failures []BlastFailure `json:"failures"`
}

// BlastPage matches BlastPage in the panel, newest first.
type BlastPage struct {
	Items    []Blast `json:"items"`
	Total    int     `json:"total"`
	Page     int     `json:"page"`
	PageSize int     `json:"pageSize"`
	// Whether blasts only reach BLAST_TEST_RECIPIENTS on this server.
	TestMode bool          `json:"testMode"`
	Channels ChannelStatus `json:"channels"`
}

// ChannelStatus is which providers the server has credentials for.
type ChannelStatus struct {
	Email bool `json:"email"`
	SMS   bool `json:"sms"`
}

// AudienceRequest is the body of POST /admin/blasts/preview.
type AudienceRequest struct {
	Channel  string   `json:"channel" validate:"required"`
	Statuses []string `json:"statuses"`
	Branches []string `json:"branches"`
}

// AudiencePreview says how many messages a blast would send. In test mode
// Deliverable counts only the allow-listed addresses among them.
type AudiencePreview struct {
	Members      int            `json:"members"`
	Email        int            `json:"email"`
	SMS          int            `json:"sms"`
	NoContact    int            `json:"noContact"`
	BranchCounts map[string]int `json:"branchCounts"`
	TestMode     bool           `json:"testMode"`
	Deliverable  int            `json:"deliverable"`
}

// CreateBlastRequest is the body of POST /admin/blasts.
type CreateBlastRequest struct {
	AudienceRequest
	Subject   string `json:"subject"`
	EmailBody string `json:"emailBody"`
	SMSBody   string `json:"smsBody"`
}

// TestBlastRequest is the body of POST /admin/blasts/test: the draft, sent
// once to an address the admin types (normally their own). It ignores test
// mode - the admin chose the address.
type TestBlastRequest struct {
	Channel   string `json:"channel" validate:"required"`
	Subject   string `json:"subject"`
	EmailBody string `json:"emailBody"`
	SMSBody   string `json:"smsBody"`
	Email     string `json:"email"`
	Mobile    string `json:"mobile"`
}

// TestBlastResult is keyed by channel: "sent" or "failed".
type TestBlastResult map[string]string

// AudienceMember is one portal account a blast could go to.
type AudienceMember struct {
	UserID   int64
	MemberNo string
	ClientID string
	Status   string
	Name     string
	Email    string
	Mobile   string // the account's own, or a Young Saver's guardian's
}

// NewRecipient is a message to queue when a blast is created.
type NewRecipient struct {
	UserID    int64
	MemberNo  string
	Name      string
	Channel   string
	Recipient string
	// Outside test mode's allow-list: recorded as skipped, never sent.
	Skip bool
}

// BlastJob is what the worker needs to send a blast's messages.
type BlastJob struct {
	ID        int64
	Subject   string
	EmailBody string
	SMSBody   string
}

// QueuedMessage is one claimed recipient.
type QueuedMessage struct {
	ID        int64
	Channel   string
	Recipient string
	Name      string
}
