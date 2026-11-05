package models

import "time"

// Statuses lists account_claim_requests.status values, in tab order.
var Statuses = []string{"pending", "approved", "rejected"}

// ClaimAccount is the portal account (fairchild_be `users` row) whose
// member_id matches the claim, as it is now.
type ClaimAccount struct {
	ID            string `json:"id"`
	FirstName     string `json:"firstName"`
	LastName      string `json:"lastName"`
	MiddleName    string `json:"middleName,omitempty"`
	Suffix        string `json:"suffix,omitempty"`
	Email         string `json:"email,omitempty"`
	EmailVerified bool   `json:"emailVerified"`
	MobileNumber  string `json:"mobileNumber,omitempty"`
	Status        string `json:"status"`
}

// ClaimRequest matches ClaimRequest in the admin panel (src/models/admin.ts):
// one fairchild_be `account_claim_requests` row, submitted from the portal's
// Claim Account form by a member with no usable email/mobile on file.
// SubmittedAt and ReviewedAt are RFC 3339.
type ClaimRequest struct {
	ID                   string        `json:"id"`
	MemberNo             string        `json:"memberNo"`
	FullName             string        `json:"fullName"`
	ProposedEmail        string        `json:"proposedEmail,omitempty"`
	ProposedMobileNumber string        `json:"proposedMobileNumber,omitempty"`
	Message              string        `json:"message,omitempty"`
	Status               string        `json:"status"`
	RejectionReason      string        `json:"rejectionReason,omitempty"`
	SubmittedAt          string        `json:"submittedAt"`
	ReviewedAt           string        `json:"reviewedAt,omitempty"`
	ReviewedBy           string        `json:"reviewedBy,omitempty"`
	Account              *ClaimAccount `json:"account,omitempty"`
}

// ClaimListQuery is GET /admin/account-claims?status=&q=&page=&pageSize=.
// From/To (UTC, To exclusive) limit the submission date and come from
// ?month=YYYY-MM or ?year=YYYY; nil means any date.
type ClaimListQuery struct {
	Status   string
	Search   string
	From     *time.Time
	To       *time.Time
	Page     int
	PageSize int
}

// ClaimPage matches ClaimRequestPage in the panel. StatusCounts honours the
// search and the date filter.
type ClaimPage struct {
	Items        []ClaimRequest `json:"items"`
	Total        int            `json:"total"`
	Page         int            `json:"page"`
	PageSize     int            `json:"pageSize"`
	StatusCounts map[string]int `json:"statusCounts"`
}

// ApproveResult is the body of PUT /admin/account-claims/:id/approve.
// Notifications is keyed by channel ("email", "sms"), with the same outcomes
// as an unlock (locks.NotifySent etc.): the member is told on the email and
// mobile number that were just put on their account.
type ApproveResult struct {
	Claim         *ClaimRequest     `json:"claim"`
	Notifications map[string]string `json:"notifications"`
}

// RejectResult is the body of PUT /admin/account-claims/:id/reject.
// Notifications are as in ApproveResult, but go to the email and mobile
// number already on the member's portal account: the claimed ones were just
// turned down, so they may not be the member's.
type RejectResult struct {
	Claim         *ClaimRequest     `json:"claim"`
	Notifications map[string]string `json:"notifications"`
}

// RejectClaimRequest is the body of PUT /admin/account-claims/:id/reject.
type RejectClaimRequest struct {
	Reason string `json:"reason" validate:"max=500"`
}
