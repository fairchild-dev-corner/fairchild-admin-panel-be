package models

// SOAWindowMonths is how far back GET /admin/members/:id/soa looks, from
// today.
const SOAWindowMonths = 2

// SOASummary matches SOASummary in the admin panel (src/models/admin.ts):
// one pre-generated Statement of Account from the branch ledger's wkfcsoa
// (one CTRLNO, the "SOA No." printed on the document). Dates are YYYY-MM-DD.
type SOASummary struct {
	CtrlNo        int64   `json:"ctrlNo"`
	StatementDate string  `json:"statementDate"`
	CutoffDate    string  `json:"cutoffDate"`
	TotalDue      float64 `json:"totalDue"`
}

// SOAList is GET /admin/members/:id/soa: the member's statements from the
// last SOAWindowMonths months, newest first. When there are none, Items is
// their single most recent past statement and Fallback is true.
type SOAList struct {
	Items        []SOASummary `json:"items"`
	WindowMonths int          `json:"windowMonths"`
	Fallback     bool         `json:"fallback"`
}

// SOALine is one line (SEQNO) of a statement: a deposit, receivable or loan
// account.
type SOALine struct {
	SeqNo            int64    `json:"seqNo"`
	AccountGroup     string   `json:"accountGroup"` // DEPOSITS | ACCOUNTS RECEIVABLE | LOANS RECEIVABLE
	Description      string   `json:"description,omitempty"`
	RefNo            string   `json:"refNo,omitempty"`
	PrincipalAmount  *float64 `json:"principalAmount,omitempty"`
	PrincipalBalance *float64 `json:"principalBalance,omitempty"`
	AvailableBalance *float64 `json:"availableBalance,omitempty"`
	PrincipalDue     float64  `json:"principalDue"`
	InterestDue      float64  `json:"interestDue"`
	PenaltyDue       float64  `json:"penaltyDue"`
	InsuranceDue     float64  `json:"insuranceDue"`
	AccountStatus    string   `json:"accountStatus,omitempty"`
	MaturityDate     string   `json:"maturityDate,omitempty"`
	Term             string   `json:"term,omitempty"`
}

// SOAStatement is GET /admin/members/:id/soa/:ctrlNo: one statement with
// its header as printed on the legacy document.
type SOAStatement struct {
	CtrlNo         int64     `json:"ctrlNo"`
	StatementDate  string    `json:"statementDate"`
	CutoffDate     string    `json:"cutoffDate"`
	ClientID       string    `json:"clientId,omitempty"` // the Account No. on the document
	ClientName     string    `json:"clientName"`
	HomeAddress    string    `json:"homeAddress,omitempty"`
	IDNo           string    `json:"idNo,omitempty"` // a ready-to-print label, e.g. "ID No.: 010116"
	MembershipDate string    `json:"membershipDate,omitempty"`
	BranchManager  string    `json:"branchManager,omitempty"`
	OrgAcronym     string    `json:"orgAcronym,omitempty"` // the member's Group
	TotalDue       float64   `json:"totalDue"`
	Lines          []SOALine `json:"lines"`
}

// ClientStanding is GET /admin/members/:id/standing: the member's rolling
// 12-month coop health score, as the member portal shows it. Each part is
// 0-100; Score is their average.
type ClientStanding struct {
	ShareCapitalBuildupPct float64 `json:"shareCapitalBuildupPct"`
	SavingsConsistencyPct  float64 `json:"savingsConsistencyPct"`
	LoanRepaymentPct       float64 `json:"loanRepaymentPct"`
	Score                  float64 `json:"score"`
	// good (80+), attention (50-79) or risk (below 50), the portal's tiers.
	Tier string `json:"tier"`
}

// StandingTier maps a score to the member portal's tiers.
func StandingTier(score float64) string {
	switch {
	case score >= 80:
		return "good"
	case score >= 50:
		return "attention"
	default:
		return "risk"
	}
}
