package models

// Limits on a master list upload.
const (
	MaxMasterListBytes = 5 << 20
	MaxMasterListRows  = 50000
)

// MasterListRow is one member read from an uploaded file. Row is the
// spreadsheet row number, for the admin to find it.
type MasterListRow struct {
	Row      int    `json:"row"`
	MemberNo string `json:"memberNo"`
	Name     string `json:"name,omitempty"`
}

// MasterListMember is a row matched to a portal account.
type MasterListMember struct {
	MemberNo    string `json:"memberNo"`
	ListedName  string `json:"listedName,omitempty"`
	AccountName string `json:"accountName"`
	Status      string `json:"status"`
	UserID      int64  `json:"-"`
}

// MasterListCheck is POST /admin/voters/master-list/check: what the uploaded
// file holds, before anything is saved. Members are the rows to save;
// NotFound have no portal account and are left out. Inactive are in Members
// but can't vote until their account is active.
type MasterListCheck struct {
	FileName   string             `json:"fileName"`
	Rows       int                `json:"rows"`
	Members    []MasterListMember `json:"members"`
	NotFound   []MasterListRow    `json:"notFound"`
	Inactive   []MasterListMember `json:"inactive"`
	Duplicates []MasterListRow    `json:"duplicates"`
	// Rows with no member ID.
	Blank int `json:"blank"`
	// The list this upload would replace.
	CurrentCount int `json:"currentCount"`
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
	Count      int    `json:"count"`
	Eligible   int    `json:"eligible"`
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

// Why a member can't vote.
const (
	ReasonNotListed       = "not_listed"       // not on the master list
	ReasonAccountInactive = "account_inactive" // listed, but the account isn't active
	ReasonNoAccount       = "no_account"       // listed, but the account was removed since
)

// Voter matches Voter in the panel (src/models/admin.ts): one member and
// whether they may vote.
type Voter struct {
	MemberNo      string `json:"memberNo"`
	Name          string `json:"name"`
	AccountID     string `json:"accountId,omitempty"`
	AccountStatus string `json:"accountStatus,omitempty"`
	OnMasterList  bool   `json:"onMasterList"`
	CanVote       bool   `json:"canVote"`
	Reason        string `json:"reason,omitempty"`
}

// VoterListQuery is GET /admin/voters?status=&q=&page=&pageSize=.
type VoterListQuery struct {
	Status   string
	Search   string
	Page     int
	PageSize int
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
