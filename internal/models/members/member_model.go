package models

// Where a member record lives. Panel members (admin_members) can be edited
// from the panel; ledger members (a branch DB's `client` table) are owned by
// the accounting system and are read-only here.
const (
	SourcePanel  = "panel"
	SourceLedger = "ledger"
)

// Categories and Branches mirror MemberCategory / MemberBranch in the panel,
// in the order the panel shows them.
var (
	Categories = []string{"regular", "associate", "kiddie", "aflatoun"}
	Branches   = []string{"main", "knowles", "tiic"}
)

// GuardianCategories are the minors' programs; their members need a
// guardian on file (GUARDIAN_CATEGORIES in the panel).
var GuardianCategories = []string{"kiddie", "aflatoun"}

// Member matches Member in the admin panel (src/models/admin.ts). Dates are
// YYYY-MM-DD; ledger members may have no birth or membership date on file.
type Member struct {
	ID                   string `json:"id"`
	Source               string `json:"source"`
	MemberNo             string `json:"memberNo,omitempty"`
	MembershipType       string `json:"membershipType"`
	Category             string `json:"category"`
	Branch               string `json:"branch"`
	FirstName            string `json:"firstName"`
	LastName             string `json:"lastName"`
	MiddleName           string `json:"middleName,omitempty"`
	Suffix               string `json:"suffix,omitempty"`
	DateOfBirth          string `json:"dateOfBirth,omitempty"`
	Gender               string `json:"gender,omitempty"`
	Nationality          string `json:"nationality,omitempty"`
	Email                string `json:"email,omitempty"`
	MobileNumber         string `json:"mobileNumber,omitempty"`
	Address              string `json:"address,omitempty"`
	GuardianFullName     string `json:"guardianFullName,omitempty"`
	GuardianMobileNumber string `json:"guardianMobileNumber,omitempty"`
	Status               string `json:"status"`
	JoinedAt             string `json:"joinedAt,omitempty"`
}

// MemberListQuery is GET /admin/members?category=&branches=&q=&page=&pageSize=.
// Branches empty means every branch.
type MemberListQuery struct {
	Category string
	Branches []string
	Search   string
	Page     int
	PageSize int
}

// MemberPage matches MemberPage in the panel: one page of the selected
// category, plus the counts the tabs and branch filter show.
// CategoryCounts covers the selected branches; BranchCounts covers the
// selected category. Both honour the search.
type MemberPage struct {
	Items          []Member       `json:"items"`
	Total          int            `json:"total"`
	Page           int            `json:"page"`
	PageSize       int            `json:"pageSize"`
	CategoryCounts map[string]int `json:"categoryCounts"`
	BranchCounts   map[string]int `json:"branchCounts"`
}

// BranchCategoryCounts is branch -> category -> number of members.
type BranchCategoryCounts map[string]map[string]int

// Add adds n to [branch][category].
func (c BranchCategoryCounts) Add(branch, category string, n int) {
	if c[branch] == nil {
		c[branch] = make(map[string]int)
	}
	c[branch][category] += n
}

// MemberRequest is the body of POST /admin/members and PUT
// /admin/members/:id (MemberInput in the panel). PUT replaces the whole
// record, so an omitted optional field is cleared.
type MemberRequest struct {
	MembershipType       string `json:"membershipType" validate:"required,oneof=coop laboratory"`
	Category             string `json:"category" validate:"required,oneof=regular associate kiddie aflatoun"`
	Branch               string `json:"branch" validate:"required,oneof=main knowles tiic"`
	FirstName            string `json:"firstName" validate:"required,max=100"`
	LastName             string `json:"lastName" validate:"required,max=100"`
	MiddleName           string `json:"middleName" validate:"max=100"`
	Suffix               string `json:"suffix" validate:"max=20"`
	DateOfBirth          string `json:"dateOfBirth" validate:"required,datetime=2006-01-02"`
	Gender               string `json:"gender" validate:"required,oneof=female male prefer-not-to-say"`
	Nationality          string `json:"nationality" validate:"max=60"`
	Email                string `json:"email" validate:"omitempty,email,max=255"`
	MobileNumber         string `json:"mobileNumber" validate:"max=20"`
	Address              string `json:"address" validate:"max=500"`
	GuardianFullName     string `json:"guardianFullName" validate:"max=200"`
	GuardianMobileNumber string `json:"guardianMobileNumber" validate:"max=20"`
	Status               string `json:"status" validate:"required,oneof=active inactive"`
	JoinedAt             string `json:"joinedAt" validate:"required,datetime=2006-01-02"`
}
