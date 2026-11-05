package models

import "time"

// Statuses lists the Locked Accounts tabs, in order: locks still blocking
// login, and locks that were lifted (by an admin) or ran out.
var Statuses = []string{"locked", "unlocked"}

// How an unlocked lock was lifted.
const (
	UnlockedViaPanel   = "panel"   // an admin unlocked it here
	UnlockedViaExpired = "expired" // it ran out on its own
	UnlockedViaOther   = "other"   // unlocked_at set outside the panel
)

// LockedAccountUser is the portal account (fairchild_be `users` row) the
// lock was recorded against, as it is now.
type LockedAccountUser struct {
	ID           string `json:"id"`
	FirstName    string `json:"firstName"`
	LastName     string `json:"lastName"`
	MiddleName   string `json:"middleName,omitempty"`
	Suffix       string `json:"suffix,omitempty"`
	Email        string `json:"email,omitempty"`
	MobileNumber string `json:"mobileNumber,omitempty"`
	Status       string `json:"status"`

	// Where an unlock text goes: the account's mobile, or a Young Saver's
	// guardian's - the same fallback fairchild_be uses for OTPs.
	NotifyMobile string `json:"-"`
}

// LockedAccount matches LockedAccount in the admin panel (src/models/admin.ts):
// one fairchild_be `locked_member_accounts` row. Timestamps are RFC 3339.
// UnlockedAt is when the lock was lifted or ran out; UnlockedBy is the admin
// who lifted it from the panel.
type LockedAccount struct {
	ID           string             `json:"id"`
	MemberNo     string             `json:"memberNo"`
	Status       string             `json:"status"`
	IPAddress    string             `json:"ipAddress,omitempty"`
	AttemptCount int                `json:"attemptCount"`
	Reason       string             `json:"reason"`
	LockedAt     string             `json:"lockedAt"`
	ExpiresAt    string             `json:"expiresAt"`
	NotifiedAt   string             `json:"notifiedAt,omitempty"`
	UnlockedAt   string             `json:"unlockedAt,omitempty"`
	UnlockedVia  string             `json:"unlockedVia,omitempty"`
	UnlockedBy   string             `json:"unlockedBy,omitempty"`
	Account      *LockedAccountUser `json:"account,omitempty"`
}

// LockedAccountListQuery is GET /admin/locked-accounts?status=&q=&month=&year=&page=&pageSize=.
// From/To (UTC, To exclusive) come from month or year; nil means the last
// DefaultWindowMonths months.
type LockedAccountListQuery struct {
	Status   string
	Search   string
	From     *time.Time
	To       *time.Time
	Page     int
	PageSize int
}

// DefaultWindowMonths is how far back the list goes without a month or year.
const DefaultWindowMonths = 3

// LockedAccountPage matches LockedAccountPage in the panel. StatusCounts
// honours the search and the date range.
type LockedAccountPage struct {
	Items        []LockedAccount `json:"items"`
	Total        int             `json:"total"`
	Page         int             `json:"page"`
	PageSize     int             `json:"pageSize"`
	StatusCounts map[string]int  `json:"statusCounts"`
}

// Outcome of one unlock notification channel.
const (
	NotifySent        = "sent"
	NotifyFailed      = "failed"
	NotifyNoAddress   = "no_address"  // nothing usable on the account
	NotifyUnavailable = "unavailable" // the provider isn't configured
)

// UnlockResult is the body of PUT /admin/locked-accounts/:id/unlock.
// Notifications is keyed by channel ("email", "sms") and is empty when the
// member ID has no portal account - nobody is messaged then.
type UnlockResult struct {
	Lock          *LockedAccount    `json:"lock"`
	Notifications map[string]string `json:"notifications"`
}
