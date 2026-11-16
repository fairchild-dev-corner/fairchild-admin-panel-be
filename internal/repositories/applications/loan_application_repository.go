package repository

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"time"

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

// loanHistoryLimit caps the member's other applications on the detail page.
const loanHistoryLimit = 20

// FindLoanApplicant reads the portal account (fairchild_be's users, read
// only) behind an application: the one it was submitted from, or else the
// live account with its member number. nil when none matches.
func (r *ApplicationsRepository) FindLoanApplicant(ctx context.Context, id int64) (*models.LoanApplicant, error) {
	var a models.LoanApplicant
	var userID int64
	var membershipDate sql.NullTime
	err := r.DB.QueryRowContext(ctx,
		`SELECT u.id, COALESCE(u.email, ''), COALESCE(u.mobile_number, ''),
		        COALESCE(NULLIF(TRIM(u.address), ''), NULLIF(TRIM(u.location), ''), ''),
		        COALESCE(u.member_type, ''), u.membership_date
		 FROM loan_applications la
		 JOIN users u ON u.id = la.user_id OR (la.user_id IS NULL AND TRIM(u.member_id) = la.member_id)
		 WHERE la.id = ? AND u.status <> 'deleted' AND u.deleted_at IS NULL
		 ORDER BY u.id = la.user_id DESC, u.id
		 LIMIT 1`,
		id,
	).Scan(&userID, &a.Email, &a.MobileNumber, &a.Address, &a.MemberType, &membershipDate)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	a.AccountID = models.FormatID(cc.PORTAL_MEMBER_ID_PREFIX, userID)
	if membershipDate.Valid {
		a.MembershipDate = membershipDate.Time.Format(time.DateOnly)
	}
	return &a, nil
}

// ListMemberLoanApplications returns the member's other applications, newest
// first.
func (r *ApplicationsRepository) ListMemberLoanApplications(ctx context.Context, memberID string, excludeID int64) ([]models.LoanApplication, error) {
	rows, err := r.DB.QueryContext(ctx,
		loanApplicationSelect+`
		 WHERE la.member_id = ? AND la.id <> ?
		 ORDER BY la.created_at DESC, la.id DESC
		 LIMIT ?`,
		memberID, excludeID, loanHistoryLimit,
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

// ListLoanApplicationNotes returns the staff notes on an application, oldest
// first.
func (r *ApplicationsRepository) ListLoanApplicationNotes(ctx context.Context, id int64) ([]models.LoanApplicationNote, error) {
	rows, err := r.DB.QueryContext(ctx,
		noteSelect+` WHERE n.loan_application_id = ? ORDER BY n.created_at, n.id`,
		id,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	notes := make([]models.LoanApplicationNote, 0)
	for rows.Next() {
		n, err := scanNote(rows)
		if err != nil {
			return nil, err
		}
		notes = append(notes, *n)
	}
	return notes, rows.Err()
}

// noteSelect reads notes with their author's panel uuid.
const noteSelect = `
SELECT n.id, COALESCE(n.user_panel_id, 0), COALESCE(u.uuid, ''), n.author_name, n.body, n.created_at, n.edited_at
FROM loan_application_notes n
LEFT JOIN users_panel u ON u.id = n.user_panel_id`

func scanNote(row interface{ Scan(...any) error }) (*models.LoanApplicationNote, error) {
	var n models.LoanApplicationNote
	var noteID int64
	var editedAt sql.NullTime
	if err := row.Scan(&noteID, &n.AuthorPanelID, &n.AuthorID, &n.Author, &n.Body, &n.CreatedAt, &editedAt); err != nil {
		return nil, err
	}
	n.ID = strconv.FormatInt(noteID, 10)
	if editedAt.Valid {
		n.EditedAt = &editedAt.Time
	}
	return &n, nil
}

// FindLoanApplicationNote returns one note on application id, or nil.
func (r *ApplicationsRepository) FindLoanApplicationNote(ctx context.Context, id, noteID int64) (*models.LoanApplicationNote, error) {
	n, err := scanNote(r.DB.QueryRowContext(ctx,
		noteSelect+` WHERE n.id = ? AND n.loan_application_id = ?`, noteID, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return n, err
}

// InsertLoanApplicationNote adds a staff note and its audit entry in one
// transaction.
func (r *ApplicationsRepository) InsertLoanApplicationNote(ctx context.Context, id, authorID int64, author, body string, audit *audit_models.AuditLog) (*models.LoanApplicationNote, error) {
	var note *models.LoanApplicationNote
	err := database.WithTx(ctx, r.DB, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx,
			`INSERT INTO loan_application_notes (loan_application_id, user_panel_id, author_name, body)
			 VALUES (?, ?, ?, ?)`,
			id, authorID, utils.Truncate(author, 200), body,
		)
		if err != nil {
			return err
		}
		noteID, err := res.LastInsertId()
		if err != nil {
			return err
		}
		if note, err = scanNote(tx.QueryRowContext(ctx, noteSelect+` WHERE n.id = ?`, noteID)); err != nil {
			return err
		}
		return audit_repo.InsertAuditLogTx(ctx, tx, audit)
	})
	return note, err
}

// UpdateLoanApplicationNote replaces a note's body, marks it edited, and
// writes the audit entry in one transaction.
func (r *ApplicationsRepository) UpdateLoanApplicationNote(ctx context.Context, id, noteID int64, body string, audit *audit_models.AuditLog) (*models.LoanApplicationNote, error) {
	var note *models.LoanApplicationNote
	err := database.WithTx(ctx, r.DB, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx,
			`UPDATE loan_application_notes SET body = ?, edited_at = NOW()
			 WHERE id = ? AND loan_application_id = ?`,
			body, noteID, id,
		); err != nil {
			return err
		}
		var err error
		if note, err = scanNote(tx.QueryRowContext(ctx, noteSelect+` WHERE n.id = ?`, noteID)); err != nil {
			return err
		}
		return audit_repo.InsertAuditLogTx(ctx, tx, audit)
	})
	return note, err
}

// DeleteLoanApplicationNote removes a note and writes the audit entry in one
// transaction.
func (r *ApplicationsRepository) DeleteLoanApplicationNote(ctx context.Context, id, noteID int64, audit *audit_models.AuditLog) error {
	return database.WithTx(ctx, r.DB, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx,
			`DELETE FROM loan_application_notes WHERE id = ? AND loan_application_id = ?`, noteID, id,
		); err != nil {
			return err
		}
		return audit_repo.InsertAuditLogTx(ctx, tx, audit)
	})
}

// RecordApplicantMessage audits a text or email sent to a loan applicant.
// The message itself isn't stored.
func (r *ApplicationsRepository) RecordApplicantMessage(ctx context.Context, audit *audit_models.AuditLog) error {
	return database.WithTx(ctx, r.DB, func(tx *sql.Tx) error {
		return audit_repo.InsertAuditLogTx(ctx, tx, audit)
	})
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
