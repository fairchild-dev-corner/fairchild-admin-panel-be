package models

// Limits on a master list upload. MaxToken caps each election token amount.
const (
	MaxMasterListBytes = 5 << 20
	MaxMasterListRows  = 50000
	MaxToken           = 100000
)

// Tokens are a listed member's election tokens in pesos, from the file's
// ATTENDANCE, COMPLETE SURVEY and LUCKY PICK PRIZE columns (0 when blank or
// missing). The member claims them on the portal.
type Tokens struct {
	Attendance float64 `json:"attendance,omitempty"`
	Survey     float64 `json:"survey,omitempty"`
	LuckyPick  float64 `json:"luckyPick,omitempty"`
}

// A listed member's category, from the file's Category column. A file
// without one lists everyone as voting. Only voting members may vote;
// non-voting and ineligible ones are listed so blasts can reach them.
const (
	CategoryVoting     = "voting"
	CategoryNonVoting  = "non_voting"
	CategoryIneligible = "ineligible"
)

// MasterListRow is one member read from an uploaded file. Row is the
// spreadsheet row number, for the admin to find it. Category is
// CategoryVoting, CategoryNonVoting or CategoryIneligible, or - in InvalidCategory - the text
// that couldn't be read; empty in a save request means voting.
type MasterListRow struct {
	Row int `json:"row"`
	// The workbook tab the row is on (empty for a CSV or single-sheet file).
	Sheet    string `json:"sheet,omitempty"`
	MemberNo string `json:"memberNo"`
	Name     string `json:"name,omitempty"`
	Category string `json:"category,omitempty"`
	// The file's Delinquent column (or a "Delinquent" category) is filled in.
	Delinquent bool `json:"delinquent,omitempty"`
	// Why an ineligible member is ineligible: the requirements they didn't
	// meet, from the Remarks column or the Delinquent / Less than 4k /
	// Unpaid Membership Fee / NO TIN columns. Shown to them on the portal.
	Reasons []string `json:"reasons,omitempty"`
	Tokens
}

// Limits on a member's ineligibility reasons.
const (
	MaxReasons      = 10
	MaxReasonLength = 300
)

// MasterListMember is a row matched to a portal account.
type MasterListMember struct {
	MemberNo    string   `json:"memberNo"`
	ListedName  string   `json:"listedName,omitempty"`
	AccountName string   `json:"accountName"`
	Status      string   `json:"status"`
	Voting      bool     `json:"voting"`
	Category    string   `json:"category"`
	Delinquent  bool     `json:"delinquent,omitempty"`
	Reasons     []string `json:"reasons,omitempty"`
	Tokens
	UserID int64 `json:"-"`
}

// MasterListCheck is POST /admin/voters/master-list/check: what the uploaded
// file holds, before anything is saved. Members are the rows to save;
// NotFound have no portal account and are left out. A member's account
// status doesn't matter: listed as voting is voting. InvalidCategory have a
// blank or unknown category and are left out; Conflicts repeat a member with
// a different category than its first row, which is the one kept.
type MasterListCheck struct {
	FileName        string             `json:"fileName"`
	Rows            int                `json:"rows"`
	Members         []MasterListMember `json:"members"`
	VotingCount     int                `json:"votingCount"`
	NonVotingCount  int                `json:"nonVotingCount"`
	IneligibleCount int                `json:"ineligibleCount"`
	DelinquentCount int                `json:"delinquentCount"`
	NotFound        []MasterListRow    `json:"notFound"`
	Duplicates      []MasterListRow    `json:"duplicates"`
	Conflicts       []MasterListRow    `json:"conflicts"`
	InvalidCategory []MasterListRow    `json:"invalidCategory"`
	// Rows with no member ID.
	Blank int `json:"blank"`
	// The list this upload would replace.
	CurrentCount int `json:"currentCount"`
	// The workbook tabs read, for a multi-tab master list.
	Sheets []MasterListSheet `json:"sheets,omitempty"`
}

// How a tab of a multi-tab master list is read: everyone ineligible (an
// "All Ineligible" tab), everyone voting ("Voting List Only") or by each
// row's Category (a branch's "Main List", "Knowles List", ...).
const (
	SheetIneligible = "ineligible"
	SheetVoting     = "voting"
	SheetBranch     = "branch"
)

// MasterListSheet is one tab read from an uploaded workbook.
type MasterListSheet struct {
	Name string `json:"name"`
	Role string `json:"role"`
	// Rows with a member ID.
	Rows int `json:"rows"`
}

// SaveMasterListRequest is the body of PUT /admin/voters/master-list: the
// members the check found, confirmed by the admin. They're matched to portal
// accounts again; ones that no longer match are left out.
type SaveMasterListRequest struct {
	FileName string          `json:"fileName"`
	Members  []MasterListRow `json:"members"`
}

// MasterListSummary is GET /admin/voters/master-list: the current list and
// its last upload (empty fields when nothing has been uploaded).
type MasterListSummary struct {
	Count    int `json:"count"`
	Eligible int `json:"eligible"`
	// Listed with category non-voting.
	NonVoting  int    `json:"nonVoting"`
	FileName   string `json:"fileName,omitempty"`
	UploadedAt string `json:"uploadedAt,omitempty"`
	UploadedBy string `json:"uploadedBy,omitempty"`
}

// Voter statuses on the Members List page: voting (eligible) and non-voting
// (ineligible); "all" lists both.
const (
	VoterAll        = "all"
	VoterEligible   = "eligible"
	VoterIneligible = "ineligible"
)

var VoterStatuses = []string{VoterAll, VoterEligible, VoterIneligible}

// Why a member can't vote. The account's status (active, inactive,
// suspended, pending review) never matters.
const (
	ReasonNotListed       = "not_listed"        // not on the master list
	ReasonListedNonVoting  = "listed_non_voting" // listed with category non-voting
	ReasonListedIneligible = "listed_ineligible" // listed with category ineligible
	ReasonNoAccount        = "no_account"        // listed, but the account was removed since
)

// Voter matches Voter in the panel (src/models/admin.ts): one member and
// whether they may vote.
type Voter struct {
	MemberNo      string `json:"memberNo"`
	Name          string `json:"name"`
	AccountID     string `json:"accountId,omitempty"`
	AccountStatus string `json:"accountStatus,omitempty"`
	OnMasterList  bool   `json:"onMasterList"`
	// On the master list as voting (they may still be blocked by their account).
	ListedVoting bool   `json:"listedVoting"`
	CanVote      bool   `json:"canVote"`
	Reason       string `json:"reason,omitempty"`
	// The master list category (voting, non_voting, ineligible; empty when
	// not listed), and an ineligible member's reasons.
	Category          string   `json:"category,omitempty"`
	IneligibleReasons []string `json:"ineligibleReasons,omitempty"`
	// Election tokens on the master list (pesos), and whether they're claimed
	// this year (fairchild_be's voter_token_claims).
	TokenTotal   float64 `json:"tokenTotal"`
	TokenClaimed bool    `json:"tokenClaimed"`
}

// VoterListQuery is GET /admin/voters?status=&q=&page=&pageSize=.
type VoterListQuery struct {
	Status   string
	Search   string
	Page     int
	PageSize int
	// Which year's token claims to show (set by the service, Asia/Manila).
	ClaimYear int
}

// VoterPage matches VoterPage in the panel. StatusCounts (all, eligible,
// ineligible) honours the search.
type VoterPage struct {
	Items        []Voter        `json:"items"`
	Total        int            `json:"total"`
	Page         int            `json:"page"`
	PageSize     int            `json:"pageSize"`
	StatusCounts map[string]int `json:"statusCounts"`
}

// AccountRef is a live portal account, for matching master list rows.
type AccountRef struct {
	UserID   int64
	MemberNo string
	Name     string
	Status   string
}

// ElectionControl is GET /admin/election-control: whether the yearly
// election period is on. While it's off the master list can't be uploaded.
type ElectionControl struct {
	Enabled   bool   `json:"enabled"`
	UpdatedAt string `json:"updatedAt"`
	UpdatedBy string `json:"updatedBy,omitempty"`
}

// SetElectionControlRequest is the body of PUT /admin/election-control; the
// admin's own password confirms the change.
type SetElectionControlRequest struct {
	Enabled  *bool  `json:"enabled" validate:"required"`
	Password string `json:"password" validate:"required"`
}

// SetTokenClaimRequest is the body of PUT /admin/voters/:accountId/token-claim:
// mark the member's election tokens claimed this year (e.g. paid at the
// branch), or undo that.
type SetTokenClaimRequest struct {
	Claimed *bool `json:"claimed" validate:"required"`
}

// TokenClaimsSummary is GET /admin/voters/token-claims: the election token
// claims on record (every year), which the Controller's reset clears.
type TokenClaimsSummary struct {
	Claimed int     `json:"claimed"`
	Total   float64 `json:"total"`
}

// ResetTokenClaimsRequest is the body of POST
// /admin/voters/token-claims/reset; the admin's own password confirms it.
type ResetTokenClaimsRequest struct {
	Password string `json:"password" validate:"required"`
}

// SetVotingRequest is the body of PUT /admin/voters/:accountId/voting: put
// the member on the master list as voting, non-voting or ineligible. An
// ineligible member needs at least one reason (the requirements they didn't
// meet), shown to them on the portal. Without category, voting picks voting
// or non-voting.
type SetVotingRequest struct {
	Voting   *bool    `json:"voting"`
	Category string   `json:"category" validate:"omitempty,oneof=voting non_voting ineligible"`
	Reasons  []string `json:"reasons"`
}

// ResetMasterListRequest is the body of POST /admin/voters/master-list/reset;
// the admin's own password confirms it.
type ResetMasterListRequest struct {
	Password string `json:"password" validate:"required"`
}

// VoterStatus is the slice of a member needed to count voters by branch.
type VoterStatus struct {
	MemberNo string
	ClientID string
	CanVote  bool
	// On the master list, and its category (empty when not listed).
	OnList   bool
	Category string
}

// NoBranch is the key for members whose member number isn't on any branch
// ledger (or whose account was removed since they were listed).
const NoBranch = "none"

// VoterStats is GET /admin/voters/stats: members per ledger branch (main,
// knowles, tiic, plus NoBranch) in four groups, and the totals. Voting,
// NonVoting and Ineligible are listed on the master list with that category;
// LeftOut aren't on the master list at all.
type VoterStats struct {
	Voting          map[string]int `json:"voting"`
	NonVoting       map[string]int `json:"nonVoting"`
	Ineligible      map[string]int `json:"ineligible"`
	LeftOut         map[string]int `json:"leftOut"`
	TotalVoting     int            `json:"totalVoting"`
	TotalNonVoting  int            `json:"totalNonVoting"`
	TotalIneligible int            `json:"totalIneligible"`
	TotalLeftOut    int            `json:"totalLeftOut"`
}
