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
	member_models "fairchild-admin-panel-be/internal/models/members"
	models "fairchild-admin-panel-be/internal/models/portal"
	audit_repo "fairchild-admin-panel-be/internal/repositories/audit"
	"fairchild-admin-panel-be/internal/utils"
	"fairchild-admin-panel-be/pkg/database"

	"github.com/go-sql-driver/mysql"
)

/**
* PortalRepository
* @Description: Members' portal accounts in fairchild_be's `users` table.
* Besides reads, the panel may only add an account for a ledger member,
* change an account's email and mobile number, and remove an account the
* way fairchild_be does (status = 'deleted' + deleted_at, sessions revoked).
* Every write records its audit entry in the same transaction.
**/
type PortalRepository struct {
	DB *sql.DB
}

func NewPortalRepository(db *sql.DB) *PortalRepository {
	return &PortalRepository{DB: db}
}

const portalSelect = `
SELECT u.id, COALESCE(u.member_id, ''), COALESCE(u.user_client_id, ''),
       COALESCE(u.first_name, ''), COALESCE(u.last_name, ''), COALESCE(u.middle_name, ''), COALESCE(u.suffix, ''),
       u.date_of_birth, COALESCE(u.gender, ''), COALESCE(u.nationality, ''),
       COALESCE(u.email, ''), u.email_verified_at IS NOT NULL, COALESCE(u.mobile_number, ''),
       COALESCE(NULLIF(TRIM(u.address), ''), NULLIF(TRIM(u.location), ''), ''),
       u.member_type, u.status, u.membership_date, u.created_at,
       COALESCE(u.guardian_full_name, ''), COALESCE(u.guardian_mobile_number, ''),
       (SELECT c.last_login_at FROM user_account_cred c WHERE c.user_id = u.id),
       (SELECT COALESCE(s.ip_address, '') FROM auth_sessions s WHERE s.user_id = u.id ORDER BY s.id DESC LIMIT 1)
FROM users u`

// The last login is user_account_cred.last_login_at, which fairchild_be sets
// on every kind of sign-in (password + OTP, social, SSO). Its IP comes from
// the newest auth_sessions row, which a token refresh also creates, so it's
// the IP the member was last active from.

// liveWhere excludes removed accounts.
const liveWhere = `u.status <> 'deleted' AND u.deleted_at IS NULL`

func FormatPortalID(id int64) string {
	return app_models.FormatID(cc.PORTAL_MEMBER_ID_PREFIX, id)
}

func scanPortalMember(row interface{ Scan(...any) error }) (*models.PortalMember, error) {
	var m models.PortalMember
	var id int64
	var dob, joined sql.NullTime
	var registered time.Time
	var lastLogin sql.NullTime
	var lastIP sql.NullString

	if err := row.Scan(
		&id, &m.MemberNo, &m.ClientID,
		&m.FirstName, &m.LastName, &m.MiddleName, &m.Suffix,
		&dob, &m.Gender, &m.Nationality,
		&m.Email, &m.EmailVerified, &m.MobileNumber,
		&m.Address,
		&m.MemberType, &m.Status, &joined, &registered,
		&m.GuardianFullName, &m.GuardianMobileNumber,
		&lastLogin, &lastIP,
	); err != nil {
		return nil, err
	}

	m.ID = FormatPortalID(id)
	if dob.Valid {
		m.DateOfBirth = dob.Time.Format(time.DateOnly)
	}
	if joined.Valid {
		m.JoinedAt = joined.Time.Format(time.DateOnly)
	}
	m.RegisteredAt = registered.UTC().Format(time.RFC3339)
	if lastLogin.Valid {
		m.LastLoginAt = lastLogin.Time.UTC().Format(time.RFC3339)
	}
	m.LastLoginIP = lastIP.String
	return &m, nil
}

// searchWhere matches a substring of the name, email or mobile, the member
// number, or the account id ("PU-00012" or "12").
func searchWhere(search string) (string, []any) {
	like := "%" + utils.EscapeLike(search) + "%"
	searchID, _ := app_models.ParseID(cc.PORTAL_MEMBER_ID_PREFIX, search)
	return `(? = '' OR u.id = ? OR u.member_id = ?
	        OR CONCAT_WS(' ', u.first_name, u.middle_name, u.last_name, u.suffix) LIKE ?
	        OR CONCAT_WS(' ', u.last_name, u.first_name) LIKE ?
	        OR u.email LIKE ? OR u.mobile_number LIKE ?)`,
		[]any{search, searchID, search, like, like, like, like}
}

func (r *PortalRepository) ListKeys(ctx context.Context, search string) ([]models.PortalKey, error) {
	where, args := searchWhere(search)
	rows, err := r.DB.QueryContext(ctx,
		`SELECT u.id, COALESCE(TRIM(u.member_id), ''), COALESCE(TRIM(u.user_client_id), ''), u.status
		 FROM users u WHERE `+liveWhere+` AND `+where+`
		 ORDER BY u.created_at DESC, u.id DESC`,
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	keys := make([]models.PortalKey, 0, 1024)
	for rows.Next() {
		var k models.PortalKey
		if err := rows.Scan(&k.ID, &k.MemberNo, &k.ClientID, &k.Status); err != nil {
			return nil, err
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

func (r *PortalRepository) FindByIDs(ctx context.Context, ids []int64) ([]models.PortalMember, error) {
	if len(ids) == 0 {
		return []models.PortalMember{}, nil
	}
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}

	rows, err := r.DB.QueryContext(ctx,
		portalSelect+` WHERE `+liveWhere+` AND u.id IN (?`+strings.Repeat(", ?", len(ids)-1)+`)`,
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	members := make([]models.PortalMember, 0, len(ids))
	for rows.Next() {
		m, err := scanPortalMember(rows)
		if err != nil {
			return nil, err
		}
		members = append(members, *m)
	}
	return members, rows.Err()
}

func (r *PortalRepository) MemberNoTaken(ctx context.Context, memberNo string) (bool, error) {
	var exists int
	err := r.DB.QueryRowContext(ctx, `SELECT 1 FROM users WHERE member_id = ? LIMIT 1`, memberNo).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

// Create gives a ledger member a portal account the way migrated accounts
// exist today: a `users` row plus a `user_account_cred` row with no
// password, which the member sets via the portal's Forgot Password (email,
// then an OTP to the mobile number).
func (r *PortalRepository) Create(ctx context.Context, m *member_models.Member, req *models.AddPortalMemberRequest, memberType string, audit *audit_models.AuditLog) (int64, error) {
	var id int64
	err := database.WithTx(ctx, r.DB, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx,
			`INSERT INTO users
				(uuid, first_name, middle_name, last_name, suffix, member_id, user_client_id,
				 date_of_birth, gender, mobile_number, email, address, membership_date,
				 status, member_type, guardian_full_name, guardian_mobile_number)
			 VALUES (UUID(), ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'active', ?, ?, ?)`,
			utils.NullIfEmpty(m.FirstName), utils.NullIfEmpty(m.MiddleName), utils.NullIfEmpty(m.LastName),
			utils.NullIfEmpty(m.Suffix), req.MemberNo, utils.NullIfEmpty(ledgerClientID(m.ID)),
			utils.NullIfEmpty(m.DateOfBirth), utils.NullIfEmpty(m.Gender), req.MobileNumber, req.Email,
			utils.NullIfEmpty(utils.Truncate(m.Address, 255)), utils.NullIfEmpty(m.JoinedAt),
			memberType, utils.NullIfEmpty(req.GuardianFullName), utils.NullIfEmpty(req.GuardianMobileNumber),
		)
		if err != nil {
			var myErr *mysql.MySQLError
			if errors.As(err, &myErr) && myErr.Number == 1062 {
				if strings.Contains(myErr.Message, "uq_users_email") {
					return cc.ErrPortalEmailTaken
				}
				return cc.ErrPortalAccountExists
			}
			return err
		}
		if id, err = res.LastInsertId(); err != nil {
			return err
		}

		if _, err := tx.ExecContext(ctx,
			`INSERT INTO user_account_cred (user_id, member_id, password_hash) VALUES (?, ?, NULL)`,
			id, req.MemberNo,
		); err != nil {
			return err
		}

		audit.Target = FormatPortalID(id)
		return audit_repo.InsertAuditLogTx(ctx, tx, audit)
	})
	return id, err
}

// ledgerClientID extracts the client id from a ledger member id
// ("01-012345" -> "12345"), the form users.user_client_id stores.
func ledgerClientID(memberID string) string {
	_, id, ok := strings.Cut(memberID, "-")
	if !ok {
		return ""
	}
	return strings.TrimLeft(id, "0")
}

func (r *PortalRepository) FindByID(ctx context.Context, id int64) (*models.PortalMember, error) {
	m, err := scanPortalMember(r.DB.QueryRowContext(ctx, portalSelect+` WHERE u.id = ? AND `+liveWhere, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return m, err
}

// UpdateContact sets email and mobile_number only (empty = NULL). A clash on
// uq_users_email becomes cc.ErrPortalEmailTaken.
func (r *PortalRepository) UpdateContact(ctx context.Context, id int64, req *models.ContactUpdateRequest, audit *audit_models.AuditLog) error {
	return database.WithTx(ctx, r.DB, func(tx *sql.Tx) error {
		if err := lockLiveAccount(ctx, tx, id); err != nil {
			return err
		}

		if _, err := tx.ExecContext(ctx,
			`UPDATE users SET email = ?, mobile_number = ? WHERE id = ?`,
			utils.NullIfEmpty(req.Email), utils.NullIfEmpty(req.MobileNumber), id,
		); err != nil {
			var myErr *mysql.MySQLError
			if errors.As(err, &myErr) && myErr.Number == 1062 {
				return cc.ErrPortalEmailTaken
			}
			return err
		}

		return audit_repo.InsertAuditLogTx(ctx, tx, audit)
	})
}

// UpdateStatus sets users.status. Suspending also revokes the account's
// sessions; fairchild_be refuses to sign a suspended account in or refresh
// its tokens. Inactive accounts keep using the portal.
func (r *PortalRepository) UpdateStatus(ctx context.Context, id int64, status string, audit *audit_models.AuditLog) error {
	return database.WithTx(ctx, r.DB, func(tx *sql.Tx) error {
		if err := lockLiveAccount(ctx, tx, id); err != nil {
			return err
		}

		if _, err := tx.ExecContext(ctx, `UPDATE users SET status = ? WHERE id = ?`, status, id); err != nil {
			return err
		}

		if status == "suspended" {
			if _, err := tx.ExecContext(ctx,
				`UPDATE auth_sessions SET revoked_at = NOW() WHERE user_id = ? AND revoked_at IS NULL`, id,
			); err != nil {
				return err
			}
		}

		return audit_repo.InsertAuditLogTx(ctx, tx, audit)
	})
}

// Remove soft-deletes the account the way fairchild_be marks deleted users,
// and revokes its sessions so the member is signed out on their next refresh.
func (r *PortalRepository) Remove(ctx context.Context, id int64, audit *audit_models.AuditLog) error {
	return database.WithTx(ctx, r.DB, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx,
			`UPDATE users SET status = 'deleted', deleted_at = NOW()
			 WHERE id = ? AND status <> 'deleted' AND deleted_at IS NULL`,
			id,
		)
		if err != nil {
			return err
		}
		if n, err := res.RowsAffected(); err != nil {
			return err
		} else if n == 0 {
			return cc.ErrPortalMemberNotFound
		}

		if _, err := tx.ExecContext(ctx,
			`UPDATE auth_sessions SET revoked_at = NOW() WHERE user_id = ? AND revoked_at IS NULL`, id,
		); err != nil {
			return err
		}

		return audit_repo.InsertAuditLogTx(ctx, tx, audit)
	})
}

func lockLiveAccount(ctx context.Context, tx *sql.Tx, id int64) error {
	var exists int
	err := tx.QueryRowContext(ctx, `SELECT 1 FROM users u WHERE u.id = ? AND `+liveWhere+` FOR UPDATE`, id).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return cc.ErrPortalMemberNotFound
	}
	return err
}
