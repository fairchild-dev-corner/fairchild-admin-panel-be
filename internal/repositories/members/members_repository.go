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
	models "fairchild-admin-panel-be/internal/models/members"
	audit_repo "fairchild-admin-panel-be/internal/repositories/audit"
	"fairchild-admin-panel-be/internal/utils"
	"fairchild-admin-panel-be/pkg/database"
)

/**
* MembersRepository
* @Description: CRUD over admin_members (owned by this service): members
* added from the panel, listed alongside the read-only ledger members (see
* LedgerMembersRepository). Deleted rows are kept with deleted_at set and
* never returned. Every write records its audit entry in the same
* transaction.
**/
type MembersRepository struct {
	DB *sql.DB
}

func NewMembersRepository(db *sql.DB) *MembersRepository {
	return &MembersRepository{DB: db}
}

const memberSelect = `
SELECT id, membership_type, category, branch, first_name, last_name, middle_name, suffix,
       date_of_birth, gender, nationality, email, mobile_number, address,
       guardian_full_name, guardian_mobile_number, status, joined_at
FROM admin_members`

func scanMember(row interface{ Scan(...any) error }) (*models.Member, error) {
	var m models.Member
	var id int64
	var dob, joinedAt time.Time
	var middleName, suffix, gender, nationality, email, mobile, address, guardianName, guardianMobile sql.NullString

	if err := row.Scan(
		&id, &m.MembershipType, &m.Category, &m.Branch, &m.FirstName, &m.LastName, &middleName, &suffix,
		&dob, &gender, &nationality, &email, &mobile, &address,
		&guardianName, &guardianMobile, &m.Status, &joinedAt,
	); err != nil {
		return nil, err
	}

	m.ID = FormatMemberID(id)
	m.Source = models.SourcePanel
	m.MiddleName = middleName.String
	m.Suffix = suffix.String
	m.DateOfBirth = dob.Format(time.DateOnly)
	m.Gender = gender.String
	m.Nationality = nationality.String
	m.Email = email.String
	m.MobileNumber = mobile.String
	m.Address = address.String
	m.GuardianFullName = guardianName.String
	m.GuardianMobileNumber = guardianMobile.String
	m.JoinedAt = joinedAt.Format(time.DateOnly)
	return &m, nil
}

// FormatMemberID renders a row id as the panel's member id: 20310 -> FC20310.
func FormatMemberID(id int64) string {
	return cc.MEMBER_ID_PREFIX + app_models.FormatID("", id)
}

// searchWhere matches live members against search: a substring of the
// name, email or mobile, or the member id ("FC20310" or "20310"). It
// returns the condition and its args.
func searchWhere(search string) (string, []any) {
	like := "%" + utils.EscapeLike(search) + "%"
	searchID, _ := app_models.ParseID(cc.MEMBER_ID_PREFIX, search)
	return `deleted_at IS NULL
		   AND (? = '' OR id = ?
		        OR CONCAT_WS(' ', first_name, middle_name, last_name, suffix) LIKE ?
		        OR CONCAT_WS(' ', last_name, first_name) LIKE ?
		        OR email LIKE ? OR mobile_number LIKE ?)`,
		[]any{search, searchID, like, like, like, like}
}

func (r *MembersRepository) CountMembers(ctx context.Context, search string) (models.BranchCategoryCounts, error) {
	where, args := searchWhere(search)
	rows, err := r.DB.QueryContext(ctx,
		`SELECT branch, category, COUNT(*) FROM admin_members WHERE `+where+` GROUP BY branch, category`,
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make(models.BranchCategoryCounts)
	for rows.Next() {
		var branch, category string
		var n int
		if err := rows.Scan(&branch, &category, &n); err != nil {
			return nil, err
		}
		counts.Add(branch, category, n)
	}
	return counts, rows.Err()
}

// ListMembers returns one page of live members in category and branches
// (which must not be empty), newest first.
func (r *MembersRepository) ListMembers(ctx context.Context, category string, branches []string, search string, offset, limit int) ([]models.Member, error) {
	where, args := searchWhere(search)
	args = append(args, category)
	for _, b := range branches {
		args = append(args, b)
	}
	args = append(args, limit, offset)

	rows, err := r.DB.QueryContext(ctx,
		memberSelect+` WHERE `+where+`
		   AND category = ?
		   AND branch IN (?`+strings.Repeat(", ?", len(branches)-1)+`)
		 ORDER BY created_at DESC, id DESC
		 LIMIT ? OFFSET ?`,
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	members := make([]models.Member, 0)
	for rows.Next() {
		m, err := scanMember(rows)
		if err != nil {
			return nil, err
		}
		members = append(members, *m)
	}
	return members, rows.Err()
}

func (r *MembersRepository) FindMemberByID(ctx context.Context, id int64) (*models.Member, error) {
	m, err := scanMember(r.DB.QueryRowContext(ctx, memberSelect+` WHERE id = ? AND deleted_at IS NULL`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return m, err
}

// memberValues is req in admin_members column order (membership_type
// through joined_at), shared by INSERT and UPDATE.
func memberValues(req *models.MemberRequest) []any {
	return []any{
		req.MembershipType, req.Category, req.Branch, req.FirstName, req.LastName,
		utils.NullIfEmpty(req.MiddleName), utils.NullIfEmpty(req.Suffix),
		req.DateOfBirth, utils.NullIfEmpty(req.Gender), utils.NullIfEmpty(req.Nationality),
		utils.NullIfEmpty(req.Email), utils.NullIfEmpty(req.MobileNumber), utils.NullIfEmpty(req.Address),
		utils.NullIfEmpty(req.GuardianFullName), utils.NullIfEmpty(req.GuardianMobileNumber),
		req.Status, req.JoinedAt,
	}
}

func (r *MembersRepository) CreateMember(ctx context.Context, req *models.MemberRequest, actorID int64, audit *audit_models.AuditLog) (int64, error) {
	var id int64
	err := database.WithTx(ctx, r.DB, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx,
			`INSERT INTO admin_members
				(membership_type, category, branch, first_name, last_name, middle_name, suffix,
				 date_of_birth, gender, nationality, email, mobile_number, address,
				 guardian_full_name, guardian_mobile_number, status, joined_at,
				 created_by_user_panel_id, updated_by_user_panel_id)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			append(memberValues(req), actorID, actorID)...,
		)
		if err != nil {
			return err
		}
		if id, err = res.LastInsertId(); err != nil {
			return err
		}

		audit.Target = FormatMemberID(id)
		return audit_repo.InsertAuditLogTx(ctx, tx, audit)
	})
	return id, err
}

// UpdateMember replaces every editable column. The existence check is a
// locking read rather than RowsAffected, which is 0 when nothing changed.
func (r *MembersRepository) UpdateMember(ctx context.Context, id int64, req *models.MemberRequest, actorID int64, audit *audit_models.AuditLog) error {
	return database.WithTx(ctx, r.DB, func(tx *sql.Tx) error {
		if err := lockLiveMember(ctx, tx, id); err != nil {
			return err
		}

		if _, err := tx.ExecContext(ctx,
			`UPDATE admin_members SET
				membership_type = ?, category = ?, branch = ?, first_name = ?, last_name = ?,
				middle_name = ?, suffix = ?, date_of_birth = ?, gender = ?, nationality = ?,
				email = ?, mobile_number = ?, address = ?,
				guardian_full_name = ?, guardian_mobile_number = ?, status = ?, joined_at = ?,
				updated_by_user_panel_id = ?
			 WHERE id = ?`,
			append(memberValues(req), actorID, id)...,
		); err != nil {
			return err
		}

		return audit_repo.InsertAuditLogTx(ctx, tx, audit)
	})
}

func (r *MembersRepository) DeleteMember(ctx context.Context, id int64, actorID int64, audit *audit_models.AuditLog) error {
	return database.WithTx(ctx, r.DB, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx,
			`UPDATE admin_members SET deleted_at = NOW(), updated_by_user_panel_id = ?
			 WHERE id = ? AND deleted_at IS NULL`,
			actorID, id,
		)
		if err != nil {
			return err
		}
		if n, err := res.RowsAffected(); err != nil {
			return err
		} else if n == 0 {
			return cc.ErrMemberNotFound
		}

		return audit_repo.InsertAuditLogTx(ctx, tx, audit)
	})
}

// lockLiveMember row-locks a live member for the rest of tx, so a
// concurrent delete can't land between the check and the write.
func lockLiveMember(ctx context.Context, tx *sql.Tx, id int64) error {
	var exists int
	err := tx.QueryRowContext(ctx,
		`SELECT 1 FROM admin_members WHERE id = ? AND deleted_at IS NULL FOR UPDATE`, id,
	).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return cc.ErrMemberNotFound
	}
	return err
}
