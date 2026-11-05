package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	cc "fairchild-admin-panel-be/internal/constants"
	app_models "fairchild-admin-panel-be/internal/models/applications"
	audit_models "fairchild-admin-panel-be/internal/models/audit"
	models "fairchild-admin-panel-be/internal/models/claims"
	audit_repo "fairchild-admin-panel-be/internal/repositories/audit"
	portal_repo "fairchild-admin-panel-be/internal/repositories/portal"
	"fairchild-admin-panel-be/internal/utils"
	"fairchild-admin-panel-be/pkg/database"

	"github.com/go-sql-driver/mysql"
)

/**
* ClaimsRepository
* @Description: fairchild_be's `account_claim_requests` - requests from the
* portal's Claim Account form - joined to the portal account (`users`) with
* the same member_id. The only writes are a review: approving copies the
* proposed email/mobile onto that account, rejecting records the reason.
* Both flip the claim's status and write the audit entry in one transaction.
* Who reviewed a claim is read back from that audit entry.
**/
type ClaimsRepository struct {
	DB *sql.DB
}

func NewClaimsRepository(db *sql.DB) *ClaimsRepository {
	return &ClaimsRepository{DB: db}
}

// The account join skips removed accounts; users.member_id is unique, so a
// claim matches at most one.
const claimFrom = `
FROM account_claim_requests c
LEFT JOIN users u ON u.member_id = c.member_id AND u.status <> 'deleted' AND u.deleted_at IS NULL`

const claimSelect = `
SELECT c.id, c.member_id, COALESCE(c.full_name, ''), COALESCE(c.proposed_email, ''),
       COALESCE(c.proposed_mobile_number, ''), COALESCE(c.message, ''), c.status,
       COALESCE(c.rejection_reason, ''), c.created_at, c.reviewed_at,
       u.id, COALESCE(u.first_name, ''), COALESCE(u.last_name, ''), COALESCE(u.middle_name, ''),
       COALESCE(u.suffix, ''), COALESCE(u.email, ''), u.email_verified_at IS NOT NULL,
       COALESCE(u.mobile_number, ''), COALESCE(u.status, '')` + claimFrom

func FormatClaimID(id int64) string {
	return app_models.FormatID(cc.ACCOUNT_CLAIM_ID_PREFIX, id)
}

func scanClaim(row interface{ Scan(...any) error }) (*models.ClaimRequest, error) {
	var c models.ClaimRequest
	var id int64
	var submitted time.Time
	var reviewed sql.NullTime
	var accountID sql.NullInt64
	var a models.ClaimAccount

	if err := row.Scan(
		&id, &c.MemberNo, &c.FullName, &c.ProposedEmail,
		&c.ProposedMobileNumber, &c.Message, &c.Status,
		&c.RejectionReason, &submitted, &reviewed,
		&accountID, &a.FirstName, &a.LastName, &a.MiddleName,
		&a.Suffix, &a.Email, &a.EmailVerified,
		&a.MobileNumber, &a.Status,
	); err != nil {
		return nil, err
	}

	c.ID = FormatClaimID(id)
	c.SubmittedAt = submitted.UTC().Format(time.RFC3339)
	if reviewed.Valid {
		c.ReviewedAt = reviewed.Time.UTC().Format(time.RFC3339)
	}
	if accountID.Valid {
		a.ID = portal_repo.FormatPortalID(accountID.Int64)
		c.Account = &a
	}
	return &c, nil
}

// filterWhere matches the search - the member number, the claim id
// ("CR-00012" or "12"), or a substring of the claimed name, the proposed
// contact details or the matched account's name - and the submission-date
// range, when there is one.
func filterWhere(q models.ClaimListQuery) (string, []any) {
	like := "%" + utils.EscapeLike(q.Search) + "%"
	searchID, _ := app_models.ParseID(cc.ACCOUNT_CLAIM_ID_PREFIX, q.Search)
	where := `(? = '' OR c.id = ? OR c.member_id = ?
	        OR c.full_name LIKE ? OR c.proposed_email LIKE ? OR c.proposed_mobile_number LIKE ?
	        OR CONCAT_WS(' ', u.first_name, u.middle_name, u.last_name, u.suffix) LIKE ?)`
	args := []any{q.Search, searchID, q.Search, like, like, like, like}
	if q.From != nil && q.To != nil {
		where += ` AND c.created_at >= ? AND c.created_at < ?`
		args = append(args, *q.From, *q.To)
	}
	return where, args
}

func (r *ClaimsRepository) CountByStatus(ctx context.Context, q models.ClaimListQuery) (map[string]int, error) {
	where, args := filterWhere(q)
	rows, err := r.DB.QueryContext(ctx, `SELECT c.status, COUNT(*)`+claimFrom+` WHERE `+where+` GROUP BY c.status`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make(map[string]int, len(models.Statuses))
	for _, s := range models.Statuses {
		counts[s] = 0
	}
	for rows.Next() {
		var status string
		var n int
		if err := rows.Scan(&status, &n); err != nil {
			return nil, err
		}
		counts[status] = n
	}
	return counts, rows.Err()
}

// List returns one page of claims with this status, most recently submitted
// first.
func (r *ClaimsRepository) List(ctx context.Context, q models.ClaimListQuery) ([]models.ClaimRequest, error) {
	where, args := filterWhere(q)
	const order = `c.created_at DESC, c.id DESC`
	args = append([]any{q.Status}, args...)
	args = append(args, q.PageSize, (q.Page-1)*q.PageSize)

	rows, err := r.DB.QueryContext(ctx,
		claimSelect+` WHERE c.status = ? AND `+where+` ORDER BY `+order+` LIMIT ? OFFSET ?`,
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	claims := make([]models.ClaimRequest, 0, q.PageSize)
	for rows.Next() {
		c, err := scanClaim(rows)
		if err != nil {
			return nil, err
		}
		claims = append(claims, *c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return claims, r.attachReviewers(ctx, claims)
}

// FindByID returns nil, nil for an unknown id.
func (r *ClaimsRepository) FindByID(ctx context.Context, id int64) (*models.ClaimRequest, error) {
	c, err := scanClaim(r.DB.QueryRowContext(ctx, claimSelect+` WHERE c.id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	claims := []models.ClaimRequest{*c}
	if err := r.attachReviewers(ctx, claims); err != nil {
		return nil, err
	}
	return &claims[0], nil
}

// attachReviewers fills ReviewedBy from the approve/reject audit entries.
// account_claim_requests has no reviewer column, and the audit entry is
// written in the same transaction as the review.
func (r *ClaimsRepository) attachReviewers(ctx context.Context, claims []models.ClaimRequest) error {
	var targets []any
	for _, c := range claims {
		if c.Status != cc.APPLICATION_STATUS_PENDING {
			targets = append(targets, c.ID)
		}
	}
	if len(targets) == 0 {
		return nil
	}

	args := append([]any{cc.AUDIT_APPROVED_ACCOUNT_CLAIM, cc.AUDIT_REJECTED_ACCOUNT_CLAIM}, targets...)
	rows, err := r.DB.QueryContext(ctx,
		`SELECT target, actor_name FROM admin_audit_logs
		 WHERE action IN (?, ?) AND target IN (?`+strings.Repeat(", ?", len(targets)-1)+`)
		 ORDER BY id`,
		args...,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	reviewers := make(map[string]string, len(targets))
	for rows.Next() {
		var target, actor string
		if err := rows.Scan(&target, &actor); err != nil {
			return err
		}
		reviewers[target] = actor // latest wins
	}
	for i := range claims {
		claims[i].ReviewedBy = reviewers[claims[i].ID]
	}
	return rows.Err()
}

// Approve copies the claim's proposed email and mobile number (already
// validated by the service) onto the portal account with the claim's
// member_id, and marks the claim approved. cc.ErrClaimAlreadyReviewed if
// it's no longer pending, cc.ErrClaimNoPortalAccount if no live account has
// that member_id, cc.ErrPortalEmailTaken on a uq_users_email clash.
func (r *ClaimsRepository) Approve(ctx context.Context, id int64, email, mobile string, audit *audit_models.AuditLog) error {
	return database.WithTx(ctx, r.DB, func(tx *sql.Tx) error {
		memberNo, err := lockPendingClaim(ctx, tx, id)
		if err != nil {
			return err
		}

		var userID int64
		err = tx.QueryRowContext(ctx,
			`SELECT id FROM users WHERE member_id = ? AND status <> 'deleted' AND deleted_at IS NULL FOR UPDATE`,
			memberNo,
		).Scan(&userID)
		if errors.Is(err, sql.ErrNoRows) {
			return cc.ErrClaimNoPortalAccount
		}
		if err != nil {
			return err
		}

		if _, err := tx.ExecContext(ctx,
			`UPDATE users SET email = ?, mobile_number = ? WHERE id = ?`,
			utils.NullIfEmpty(email), utils.NullIfEmpty(mobile), userID,
		); err != nil {
			var myErr *mysql.MySQLError
			if errors.As(err, &myErr) && myErr.Number == 1062 {
				return cc.ErrPortalEmailTaken
			}
			return err
		}

		if _, err := tx.ExecContext(ctx,
			`UPDATE account_claim_requests SET status = 'approved', rejection_reason = NULL, reviewed_at = NOW() WHERE id = ?`,
			id,
		); err != nil {
			return err
		}

		return audit_repo.InsertAuditLogTx(ctx, tx, audit)
	})
}

// Reject marks a pending claim rejected with an optional reason.
func (r *ClaimsRepository) Reject(ctx context.Context, id int64, reason string, audit *audit_models.AuditLog) error {
	return database.WithTx(ctx, r.DB, func(tx *sql.Tx) error {
		if _, err := lockPendingClaim(ctx, tx, id); err != nil {
			return err
		}

		if _, err := tx.ExecContext(ctx,
			`UPDATE account_claim_requests SET status = 'rejected', rejection_reason = ?, reviewed_at = NOW() WHERE id = ?`,
			utils.NullIfEmpty(reason), id,
		); err != nil {
			return err
		}

		return audit_repo.InsertAuditLogTx(ctx, tx, audit)
	})
}

// lockPendingClaim locks the claim row and returns its member_id.
func lockPendingClaim(ctx context.Context, tx *sql.Tx, id int64) (string, error) {
	var memberNo, status string
	err := tx.QueryRowContext(ctx,
		`SELECT member_id, status FROM account_claim_requests WHERE id = ? FOR UPDATE`, id,
	).Scan(&memberNo, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return "", cc.ErrClaimNotFound
	}
	if err != nil {
		return "", err
	}
	if status != cc.APPLICATION_STATUS_PENDING {
		return "", cc.ErrClaimAlreadyReviewed
	}
	return memberNo, nil
}
