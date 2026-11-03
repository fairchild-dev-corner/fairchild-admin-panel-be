package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	models "fairchild-admin-panel-be/internal/models/metrics"
)

/**
* MetricsRepository
* @Description: Aggregates for GET /admin/metrics, from loan_applications and
* membership_applications in the central Auth DB.
**/
type MetricsRepository struct {
	DB *sql.DB
}

func NewMetricsRepository(db *sql.DB) *MetricsRepository {
	return &MetricsRepository{DB: db}
}

// The session runs in UTC (see pkg/database), so buckets are shifted to
// Manila time with a fixed offset - CONVERT_TZ with numeric offsets doesn't
// need the server's time zone tables loaded. The Philippines has no DST.
const monthBucketSQL = `DATE_FORMAT(CONVERT_TZ(created_at, '+00:00', '+08:00'), '%%Y-%%m')`

func (r *MetricsRepository) LoanApplicationsPerMonth(ctx context.Context, since time.Time) ([]models.MonthCount, error) {
	return r.countPerMonth(ctx, "loan_applications", since)
}

func (r *MetricsRepository) MembershipApplicationsPerMonth(ctx context.Context, since time.Time) ([]models.MonthCount, error) {
	return r.countPerMonth(ctx, "membership_applications", since)
}

// countPerMonth is only ever called with the fixed table names above.
func (r *MetricsRepository) countPerMonth(ctx context.Context, table string, since time.Time) ([]models.MonthCount, error) {
	rows, err := r.DB.QueryContext(ctx,
		fmt.Sprintf(`SELECT `+monthBucketSQL+` AS ym, COUNT(*)
			FROM %s
			WHERE created_at >= ?
			GROUP BY ym`, table),
		since.UTC(),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make([]models.MonthCount, 0)
	for rows.Next() {
		var c models.MonthCount
		if err := rows.Scan(&c.Month, &c.Count); err != nil {
			return nil, err
		}
		counts = append(counts, c)
	}
	return counts, rows.Err()
}

// ApprovedLoansByProduct counts approved loan applications and their total
// amount per product, all time.
func (r *MetricsRepository) ApprovedLoansByProduct(ctx context.Context) ([]models.LoansByProduct, error) {
	rows, err := r.DB.QueryContext(ctx,
		`SELECT product, COUNT(*), COALESCE(SUM(amount), 0)
		 FROM loan_applications
		 WHERE status = 'approved'
		 GROUP BY product`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]models.LoansByProduct, 0)
	for rows.Next() {
		var p models.LoansByProduct
		if err := rows.Scan(&p.Product, &p.Count, &p.Amount); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
