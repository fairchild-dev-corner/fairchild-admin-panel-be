package models

// Statuses lists the tabs: messages with no reply yet, and answered ones.
var Statuses = []string{"unreplied", "replied"}

// Reply channels (contact_message_replies.channel).
const (
	ChannelEmail = "email"
	ChannelSMS   = "sms"
)

// Body limits: email up to the column size; SMS up to three segments.
const (
	MaxEmailReplyLength = 2000
	MaxSMSReplyLength   = 480
)

// ContactAccount is the portal account (fairchild_be `users`) with the same
// email as the message - where a text reply is sent.
type ContactAccount struct {
	ID           string `json:"id"`
	MemberNo     string `json:"memberNo,omitempty"`
	FirstName    string `json:"firstName"`
	LastName     string `json:"lastName"`
	MobileNumber string `json:"mobileNumber,omitempty"`
}

// ContactReply is one contact_message_replies row. SentAt is RFC 3339.
type ContactReply struct {
	ID        string `json:"id"`
	Channel   string `json:"channel"`
	Recipient string `json:"recipient"`
	Body      string `json:"body"`
	SentBy    string `json:"sentBy"`
	SentAt    string `json:"sentAt"`
}

// ContactMessage matches ContactMessage in the admin panel
// (src/models/admin.ts): one fairchild_be `contact_messages` row from the
// public Contact Us form, plus the panel's replies (oldest first).
type ContactMessage struct {
	ID         string          `json:"id"`
	Name       string          `json:"name"`
	Email      string          `json:"email"`
	Message    string          `json:"message"`
	ReceivedAt string          `json:"receivedAt"`
	Account    *ContactAccount `json:"account,omitempty"`
	Replies    []ContactReply  `json:"replies"`
}

// ContactListQuery is GET /admin/contact-messages?status=&q=&page=&pageSize=.
type ContactListQuery struct {
	Status   string
	Search   string
	Page     int
	PageSize int
}

// ContactChannels says which reply channels are configured on the server.
type ContactChannels struct {
	Email bool `json:"email"`
	SMS   bool `json:"sms"`
}

// ContactMessagePage matches ContactMessagePage in the panel. StatusCounts
// honours the search.
type ContactMessagePage struct {
	Items        []ContactMessage `json:"items"`
	Total        int              `json:"total"`
	Page         int              `json:"page"`
	PageSize     int              `json:"pageSize"`
	StatusCounts map[string]int   `json:"statusCounts"`
	Channels     ContactChannels  `json:"channels"`
}

// ReplyRequest is the body of POST /admin/contact-messages/:id/replies.
type ReplyRequest struct {
	Channel string `json:"channel" validate:"required,oneof=email sms"`
	Body    string `json:"body" validate:"required,max=2000"`
}
