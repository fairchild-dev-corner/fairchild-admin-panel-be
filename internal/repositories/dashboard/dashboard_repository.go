package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	models "fairchild-admin-panel-be/internal/models/dashboard"
	"fairchild-admin-panel-be/pkg/database"
)

/**
* DashboardRepository
* @Description: Aggregate counts for GET /admin/dashboard. Central counts
* come from the Auth DB; active-loan totals are summed per branch ledger DB
* (never joined with auth tables - different schemas).
**/
type DashboardRepository struct {
	DB        *sql.DB
	BranchDBs database.BranchDBs
}

func NewDashboardRepository(db *sql.DB, branchDBs database.BranchDBs) *DashboardRepository {
	return &DashboardRepository{DB: db, BranchDBs: branchDBs}
}

// CountCentral reads member, pending-application and maintenance counts in
// one round trip. monthStart is the first instant of the current month in
// the cooperative's time zone.
func (r *DashboardRepository) CountCentral(ctx context.Context, monthStart time.Time) (*models.CentralCounts, error) {
	var c models.CentralCounts
	var maintenance sql.NullBool

	err := r.DB.QueryRowContext(ctx,
		`SELECT
			(SELECT COUNT(*) FROM users WHERE status <> 'deleted' AND deleted_at IS NULL),
			(SELECT COUNT(*) FROM users WHERE status <> 'deleted' AND deleted_at IS NULL AND created_at >= ?),
			(SELECT COUNT(*) FROM loan_applications WHERE status = 'pending'),
			(SELECT COUNT(*) FROM membership_applications WHERE status = 'pending'),
			(SELECT maintenance_mode FROM site_status WHERE id = 1)`,
		monthStart.UTC(),
	).Scan(&c.TotalMembers, &c.NewMembersThisMonth, &c.PendingLoanApplications, &c.PendingMembershipApplications, &maintenance)
	if err != nil {
		return nil, err
	}

	c.MaintenanceMode = maintenance.Bool
	return &c, nil
}

// activeLoanTotalsQuery counts loans (LoanSLC_CODE = 12) with an outstanding
// balance and sums that balance. The balance definition matches
// fairchild_be's per-member activeLoansQuery (loans_repository.go): the sum
// of the loan's sldtl SLE_CODE = 11 entries, active when > 0. sldtl is
// aggregated before touching `loan` so a duplicated loan row can't double
// a balance.
const activeLoanTotalsQuery = `
SELECT COUNT(*), COALESCE(SUM(b.balance), 0)
FROM (
    SELECT s.SL_BRCODE, s.SL_CLIENTID, s.REF_NO, s.SLT_CODE, SUM(s.AMT) AS balance
    FROM sldtl s
    WHERE s.SLC_CODE = 12 AND s.SLE_CODE = 11
    GROUP BY s.SL_BRCODE, s.SL_CLIENTID, s.REF_NO, s.SLT_CODE
    HAVING balance > 0
) b
WHERE EXISTS (
    SELECT 1 FROM loan l
    WHERE l.LoanBR_CODE = b.SL_BRCODE AND l.ClientIDLoan = b.SL_CLIENTID
      AND l.LoanREF_NO = b.REF_NO AND l.LoanSLC_CODE = 12 AND l.LoanSLT_CODE = b.SLT_CODE
)`

func (r *DashboardRepository) SumActiveLoans(ctx context.Context) (*models.LoanTotals, error) {
	var total models.LoanTotals

	for _, code := range r.BranchDBs.Codes() {
		var count int
		var balance float64
		if err := r.BranchDBs[code].QueryRowContext(ctx, activeLoanTotalsQuery).Scan(&count, &balance); err != nil {
			return nil, fmt.Errorf("active loan totals for branch %q: %w", code, err)
		}
		total.Count += count
		total.Balance += balance
	}

	return &total, nil
}
