package models

// DashboardSummary matches DashboardSummary in the admin panel
// (src/models/admin.ts).
type DashboardSummary struct {
	// Registered member portal accounts (fairchild_be's `users`), not every
	// cooperative member in the branch ledgers.
	TotalMembers                  int     `json:"totalMembers"`
	NewMembersThisMonth           int     `json:"newMembersThisMonth"`
	PendingLoanApplications       int     `json:"pendingLoanApplications"`
	PendingMembershipApplications int     `json:"pendingMembershipApplications"`
	ActiveLoans                   int     `json:"activeLoans"`
	ActiveLoanBalance             float64 `json:"activeLoanBalance"`
	MaintenanceMode               bool    `json:"maintenanceMode"`
}

// CentralCounts is everything the dashboard reads from the central Auth DB.
type CentralCounts struct {
	TotalMembers                  int
	NewMembersThisMonth           int
	PendingLoanApplications       int
	PendingMembershipApplications int
	MaintenanceMode               bool
}

// LoanTotals is the active-loan aggregate for one or more branch ledgers.
type LoanTotals struct {
	Count   int
	Balance float64
}
