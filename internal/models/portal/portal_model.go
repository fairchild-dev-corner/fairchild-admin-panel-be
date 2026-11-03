package models

import members "fairchild-admin-panel-be/internal/models/members"

// Statuses lists the portal account statuses the panel shows as tabs, in
// order. fairchild_be's users.status also has "deleted", which is never
// listed.
var Statuses = []string{"active", "pending_review", "inactive", "suspended"}

// PortalMember matches PortalMember in the admin panel (src/models/admin.ts):
// one fairchild_be `users` row (a member's portal account), plus what its
// branch ledger record adds. Dates are YYYY-MM-DD; RegisteredAt is RFC 3339.
type PortalMember struct {
	ID            string `json:"id"`
	MemberNo      string `json:"memberNo,omitempty"`
	ClientID      string `json:"clientId,omitempty"`
	Username      string `json:"username,omitempty"`
	FirstName     string `json:"firstName"`
	LastName      string `json:"lastName"`
	MiddleName    string `json:"middleName,omitempty"`
	Suffix        string `json:"suffix,omitempty"`
	DateOfBirth   string `json:"dateOfBirth,omitempty"`
	Gender        string `json:"gender,omitempty"`
	Nationality   string `json:"nationality,omitempty"`
	Email         string `json:"email,omitempty"`
	EmailVerified bool   `json:"emailVerified"`
	MobileNumber  string `json:"mobileNumber,omitempty"`
	Address       string `json:"address,omitempty"`
	MemberType    string `json:"memberType"`
	Status        string `json:"status"`
	JoinedAt      string `json:"joinedAt,omitempty"`
	RegisteredAt  string `json:"registeredAt"`

	GuardianFullName     string `json:"guardianFullName,omitempty"`
	GuardianMobileNumber string `json:"guardianMobileNumber,omitempty"`

	// From the branch ledger, when the member number is found there.
	Branch        string `json:"branch,omitempty"`
	Category      string `json:"category,omitempty"`
	LedgerStatus  string `json:"ledgerStatus,omitempty"`
	LedgerAddress string `json:"ledgerAddress,omitempty"`
}

// PortalListQuery is GET /admin/portal-members?status=&branches=&q=&page=&pageSize=.
type PortalListQuery struct {
	Status   string
	Branches []string // empty = every branch, including accounts not on a ledger
	Search   string
	Page     int
	PageSize int
}

// PortalMemberPage matches PortalMemberPage in the panel. StatusCounts covers
// the selected branches; BranchCounts covers the selected status. Both honour
// the search.
type PortalMemberPage struct {
	Items        []PortalMember `json:"items"`
	Total        int            `json:"total"`
	Page         int            `json:"page"`
	PageSize     int            `json:"pageSize"`
	StatusCounts map[string]int `json:"statusCounts"`
	BranchCounts map[string]int `json:"branchCounts"`
}

// ContactUpdateRequest is the body of PUT /admin/portal-members/:id. Email and
// mobile are the only fields the panel may change on a portal account; an
// empty value clears it.
type ContactUpdateRequest struct {
	Email        string `json:"email" validate:"omitempty,email,max=255"`
	MobileNumber string `json:"mobileNumber" validate:"max=20"`
}

// LedgerLink is what a branch ledger knows about a member number.
type LedgerLink struct {
	MemberNo string
	Branch   string
	ClientID int64
	Category string
	Status   string
	Address  string
}

// PortalKey is the slice of a `users` row needed to filter, count and page
// the list by branch before loading full rows.
type PortalKey struct {
	ID       int64
	MemberNo string
	ClientID string
	Status   string
}

// AddPortalMemberRequest is the body of POST /admin/portal-members: give a
// ledger member a portal account. Names, birth date and the rest are copied
// from the ledger record server-side, never taken from the request. Email
// and mobile are required so the member can set a password via Forgot
// Password; a guardian is required for kiddie/aflatoun (Young Saver).
type AddPortalMemberRequest struct {
	MemberNo             string `json:"memberNo" validate:"required,max=50"`
	Branch               string `json:"branch" validate:"required,oneof=main knowles tiic"`
	Email                string `json:"email" validate:"required,email,max=255"`
	MobileNumber         string `json:"mobileNumber" validate:"required,max=20"`
	GuardianFullName     string `json:"guardianFullName" validate:"max=150"`
	GuardianMobileNumber string `json:"guardianMobileNumber" validate:"max=20"`
}

// LedgerCandidate is one GET /admin/portal-lookup result: a ledger member
// with that member number, and whether a portal account already uses it.
type LedgerCandidate struct {
	Member           members.Member `json:"member"`
	HasPortalAccount bool           `json:"hasPortalAccount"`
}
