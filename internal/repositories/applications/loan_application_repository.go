package repository

import (
	"context"
	"database/sql"
	"errors"

	cc "fairchild-admin-panel-be/internal/constants"
	models "fairchild-admin-panel-be/internal/models/applications"
	audit_models "fairchild-admin-panel-be/internal/models/audit"
	audit_repo "fairchild-admin-panel-be/internal/repositories/audit"
	"fairchild-admin-panel-be/internal/utils"
	"fairchild-admin-panel-be/pkg/database"
)

const loanApplicationSelect = `
SELECT la.id, la.member_id, la.member_name, la.product, la.amount, la.term_months, la.purpose,
       la.status, la.created_at, la.reviewed_at, la.review_note, ` + reviewerNameSQL + `
FROM loan_applications la
LEFT JOIN users_panel u ON u.id = la.reviewed_by_user_panel_id`

func scanLoanApplication(row interface{ Scan(...any) error }) (*models.LoanApplication, error) {
	var a models.LoanApplication
	var id int64
	var reviewedAt sql.NullTime
	var reviewNote, reviewedBy sql.NullString

	if err := row.Scan(
		&id, &a.MemberID, &a.MemberName, &a.Product, &a.Amount, &a.TermMonths, &a.Purpose,
		&a.Status, &a.SubmittedAt, &reviewedAt, &reviewNote, &reviewedBy,
	); err != nil {
		return nil, err
	}

	a.ID = models.FormatID(cc.LOAN_APPLICATION_ID_PREFIX, id)
	if reviewedAt.Valid {
		a.ReviewedAt = &reviewedAt.Time
	}
	a.ReviewNote = reviewNote.String
	a.ReviewedBy = reviewedBy.String
	return &a, nil
}

// ListLoanApplications returns applications newest first; status "" = all.
func (r *ApplicationsRepository) ListLoanApplications(ctx context.Context, status string) ([]models.LoanApplication, error) {
	rows, err := r.DB.QueryContext(ctx,
		loanApplicationSelect+`
		 WHERE (? = '' OR la.status = ?)
		 ORDER BY la.created_at DESC, la.id DESC
		 LIMIT ?`,
		status, status, listLimit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	apps := make([]models.LoanApplication, 0)
	for rows.Next() {
		a, err := scanLoanApplication(rows)
		if err != nil {
			return nil, err
		}
		apps = append(apps, *a)
	}
	return apps, rows.Err()
}

// FindLoanApplicationByID returns nil, nil when the id doesn't exist.
func (r *ApplicationsRepository) FindLoanApplicationByID(ctx context.Context, id int64) (*models.LoanApplication, error) {
	a, err := scanLoanApplication(r.DB.QueryRowContext(ctx, loanApplicationSelect+` WHERE la.id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return a, err
}

// ReviewLoanApplication approves/rejects a pending application and writes
// the audit entry in the same transaction. The "AND status = 'pending'"
// guard makes concurrent reviews safe: exactly one reviewer wins, the other
// gets ErrApplicationAlreadyReviewed.
func (r *ApplicationsRepository) ReviewLoanApplication(ctx context.Context, id int64, decision *models.ReviewDecisionRequest, reviewerID int64, audit *audit_models.AuditLog) error {
	return database.WithTx(ctx, r.DB, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx,
			`UPDATE loan_applications
			 SET status = ?, review_note = ?, reviewed_by_user_panel_id = ?, reviewed_at = NOW()
			 WHERE id = ? AND status = 'pending'`,
			decision.Status, utils.NullIfEmpty(decision.Note), reviewerID, id,
		)
		if err != nil {
			return err
		}
		if n, err := res.RowsAffected(); err != nil {
			return err
		} else if n == 0 {
			return explainNoRowsReviewed(ctx, tx, "loan_applications", id)
		}

		return audit_repo.InsertAuditLogTx(ctx, tx, audit)
	})
}
