package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	cc "fairchild-admin-panel-be/internal/constants"
	models "fairchild-admin-panel-be/internal/models/applications"
	audit_models "fairchild-admin-panel-be/internal/models/audit"
	audit_repo "fairchild-admin-panel-be/internal/repositories/audit"
	"fairchild-admin-panel-be/internal/utils"
	"fairchild-admin-panel-be/pkg/database"
)

const membershipApplicationSelect = `
SELECT m.id, m.membership_type, m.first_name, m.last_name, m.middle_name, m.suffix,
       m.email, m.mobile_number, m.date_of_birth, m.gender, m.source_of_income,
       m.type_of_work, m.status, m.created_at,
       r.reviewed_at, r.note, ` + reviewerNameSQL + `
FROM membership_applications m
LEFT JOIN membership_application_reviews r ON r.membership_application_id = m.id
LEFT JOIN users_panel u ON u.id = r.reviewed_by_user_panel_id`

func scanMembershipApplication(row interface{ Scan(...any) error }) (*models.MembershipApplication, error) {
	var a models.MembershipApplication
	var id int64
	var dob time.Time
	var middleName, suffix, email, mobile, gender, sourceOfIncome, typeOfWork, reviewNote, reviewedBy sql.NullString
	var reviewedAt sql.NullTime

	if err := row.Scan(
		&id, &a.MembershipType, &a.FirstName, &a.LastName, &middleName, &suffix,
		&email, &mobile, &dob, &gender, &sourceOfIncome,
		&typeOfWork, &a.Status, &a.SubmittedAt,
		&reviewedAt, &reviewNote, &reviewedBy,
	); err != nil {
		return nil, err
	}

	a.ID = models.FormatID(cc.MEMBERSHIP_APPLICATION_ID_PREFIX, id)
	a.MiddleName = middleName.String
	a.Suffix = suffix.String
	a.DateOfBirth = dob.Format("2006-01-02")
	a.Email = email.String
	a.MobileNumber = mobile.String
	a.Gender = gender.String
	a.SourceOfIncome = sourceOfIncome.String
	a.TypeOfWork = typeOfWork.String
	// Rows reviewed directly in the DB before the panel existed have a
	// non-pending status but no review row - reviewedAt/By stay empty.
	if reviewedAt.Valid {
		a.ReviewedAt = &reviewedAt.Time
	}
	a.ReviewNote = reviewNote.String
	a.ReviewedBy = reviewedBy.String
	return &a, nil
}

// ListMembershipApplications returns applications newest first; status "" =
// all, search "" = no search. search matches a substring of the name, email
// or mobile, or the application id ("MA-00560" or "560").
func (r *ApplicationsRepository) ListMembershipApplications(ctx context.Context, status, search string) ([]models.MembershipApplication, error) {
	like := "%" + utils.EscapeLike(search) + "%"
	searchID, _ := models.ParseID(cc.MEMBERSHIP_APPLICATION_ID_PREFIX, search)

	rows, err := r.DB.QueryContext(ctx,
		membershipApplicationSelect+`
		 WHERE (? = '' OR m.status = ?)
		   AND (? = '' OR m.id = ?
		        OR CONCAT_WS(' ', m.first_name, m.middle_name, m.last_name, m.suffix) LIKE ?
		        OR CONCAT_WS(' ', m.last_name, m.first_name) LIKE ?
		        OR m.email LIKE ? OR m.mobile_number LIKE ?)
		 ORDER BY m.created_at DESC, m.id DESC
		 LIMIT ?`,
		status, status,
		search, searchID, like, like, like, like,
		listLimit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	apps := make([]models.MembershipApplication, 0)
	for rows.Next() {
		a, err := scanMembershipApplication(rows)
		if err != nil {
			return nil, err
		}
		apps = append(apps, *a)
	}
	return apps, rows.Err()
}

// FindMembershipApplicationByID returns nil, nil when the id doesn't exist.
func (r *ApplicationsRepository) FindMembershipApplicationByID(ctx context.Context, id int64) (*models.MembershipApplication, error) {
	a, err := scanMembershipApplication(r.DB.QueryRowContext(ctx, membershipApplicationSelect+` WHERE m.id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return a, err
}

// CreateMembershipApplication inserts a pending application entered by staff
// and writes the audit entry in the same transaction, returning the new row
// id. This is the one INSERT into fairchild_be's membership_applications:
// it fills the same columns as fairchild-fe's public form, so the row is
// reviewed exactly like an online one. audit.Target is set here, once the
// id is known.
func (r *ApplicationsRepository) CreateMembershipApplication(ctx context.Context, req *models.NewMembershipApplicationRequest, audit *audit_models.AuditLog) (int64, error) {
	var id int64
	err := database.WithTx(ctx, r.DB, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx,
			`INSERT INTO membership_applications
				(membership_type, first_name, last_name, middle_name, suffix, email, mobile_number,
				 date_of_birth, gender, source_of_income, type_of_work, status)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			req.MembershipType, req.FirstName, req.LastName,
			utils.NullIfEmpty(req.MiddleName), utils.NullIfEmpty(req.Suffix),
			utils.NullIfEmpty(req.Email), utils.NullIfEmpty(req.MobileNumber),
			req.DateOfBirth, utils.NullIfEmpty(req.Gender),
			utils.NullIfEmpty(req.SourceOfIncome), utils.NullIfEmpty(req.TypeOfWork),
			cc.APPLICATION_STATUS_PENDING,
		)
		if err != nil {
			return err
		}
		if id, err = res.LastInsertId(); err != nil {
			return err
		}

		audit.Target = models.FormatID(cc.MEMBERSHIP_APPLICATION_ID_PREFIX, id)
		return audit_repo.InsertAuditLogTx(ctx, tx, audit)
	})
	return id, err
}

// ReviewMembershipApplication flips membership_applications.status from
// pending, upserts the review metadata, and writes the audit entry - all in
// one transaction. The upsert covers an application that was reviewed,
// manually reset to pending in the DB, and reviewed again.
func (r *ApplicationsRepository) ReviewMembershipApplication(ctx context.Context, id int64, decision *models.ReviewDecisionRequest, reviewerID int64, audit *audit_models.AuditLog) error {
	return database.WithTx(ctx, r.DB, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx,
			`UPDATE membership_applications SET status = ? WHERE id = ? AND status = 'pending'`,
			decision.Status, id,
		)
		if err != nil {
			return err
		}
		if n, err := res.RowsAffected(); err != nil {
			return err
		} else if n == 0 {
			return explainNoRowsReviewed(ctx, tx, "membership_applications", id)
		}

		if _, err := tx.ExecContext(ctx,
			`INSERT INTO membership_application_reviews
				(membership_application_id, decision, note, reviewed_by_user_panel_id, reviewed_at)
			 VALUES (?, ?, ?, ?, NOW())
			 ON DUPLICATE KEY UPDATE
				decision = VALUES(decision),
				note = VALUES(note),
				reviewed_by_user_panel_id = VALUES(reviewed_by_user_panel_id),
				reviewed_at = VALUES(reviewed_at)`,
			id, decision.Status, utils.NullIfEmpty(decision.Note), reviewerID,
		); err != nil {
			return err
		}

		return audit_repo.InsertAuditLogTx(ctx, tx, audit)
	})
}
