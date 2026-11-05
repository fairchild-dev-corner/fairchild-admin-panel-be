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
	models "fairchild-admin-panel-be/internal/models/voters"
	audit_repo "fairchild-admin-panel-be/internal/repositories/audit"
	"fairchild-admin-panel-be/internal/utils"
	"fairchild-admin-panel-be/pkg/database"
)

/**
* VotersRepository
* @Description: The voters' master list (`voter_master_list`, with its
* upload log `voter_master_list_uploads`), owned by this service, joined to
* fairchild_be's `users` (read only) to say which members can vote. An
* upload replaces the whole list and writes its audit entry in the same
* transaction.
**/
type VotersRepository struct {
	DB *sql.DB
}

func NewVotersRepository(db *sql.DB) *VotersRepository {
	return &VotersRepository{DB: db}
}

// liveAccount is a portal account that hasn't been removed.
const liveAccount = `u.status <> 'deleted' AND u.deleted_at IS NULL`

// masterListChunk keeps each INSERT well under max_allowed_packet.
const masterListChunk = 1000

// LoadAccounts reads every live portal account with a member number, to
// match an uploaded master list against.
func (r *VotersRepository) LoadAccounts(ctx context.Context) ([]models.AccountRef, error) {
	rows, err := r.DB.QueryContext(ctx,
		`SELECT u.id, TRIM(u.member_id), TRIM(CONCAT_WS(' ', u.first_name, u.middle_name, u.last_name, u.suffix)), u.status
		 FROM users u WHERE `+liveAccount+` AND COALESCE(TRIM(u.member_id), '') <> ''`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	accounts := make([]models.AccountRef, 0, 16384)
	for rows.Next() {
		var a models.AccountRef
		if err := rows.Scan(&a.UserID, &a.MemberNo, &a.Name, &a.Status); err != nil {
			return nil, err
		}
		accounts = append(accounts, a)
	}
	return accounts, rows.Err()
}

// Summary describes the current master list and its last upload.
func (r *VotersRepository) Summary(ctx context.Context) (*models.MasterListSummary, error) {
	var s models.MasterListSummary
	err := r.DB.QueryRowContext(ctx,
		`SELECT (SELECT COUNT(*) FROM voter_master_list),
		        (SELECT COUNT(*) FROM voter_master_list m JOIN users u ON u.id = m.user_id
		         WHERE u.status = 'active' AND u.deleted_at IS NULL)`,
	).Scan(&s.Count, &s.Eligible)
	if err != nil {
		return nil, err
	}

	var at time.Time
	err = r.DB.QueryRowContext(ctx,
		`SELECT COALESCE(file_name, ''), uploaded_by_name, uploaded_at
		 FROM voter_master_list_uploads ORDER BY id DESC LIMIT 1`,
	).Scan(&s.FileName, &s.UploadedBy, &at)
	if errors.Is(err, sql.ErrNoRows) {
		return &s, nil
	}
	if err != nil {
		return nil, err
	}
	s.UploadedAt = at.UTC().Format(time.RFC3339)
	return &s, nil
}

// ReplaceMasterList swaps the whole list for members and logs the upload.
func (r *VotersRepository) ReplaceMasterList(ctx context.Context, fileName string, members []models.MasterListMember, byID int64, by string, audit *audit_models.AuditLog) error {
	return database.WithTx(ctx, r.DB, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `DELETE FROM voter_master_list`); err != nil {
			return err
		}
		for start := 0; start < len(members); start += masterListChunk {
			chunk := members[start:min(len(members), start+masterListChunk)]
			args := make([]any, 0, len(chunk)*3)
			for _, m := range chunk {
				args = append(args, m.UserID, m.MemberNo, m.ListedName)
			}
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO voter_master_list (user_id, member_no, listed_name) VALUES (?, ?, ?)`+
					strings.Repeat(", (?, ?, ?)", len(chunk)-1),
				args...); err != nil {
				return err
			}
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO voter_master_list_uploads (file_name, member_count, uploaded_by_user_panel_id, uploaded_by_name)
			 VALUES (?, ?, ?, ?)`,
			utils.NullIfEmpty(fileName), len(members), byID, by); err != nil {
			return err
		}
		audit.Target = fileName
		if audit.Target == "" {
			audit.Target = "master list"
		}
		return audit_repo.InsertAuditLogTx(ctx, tx, audit)
	})
}

// votersFrom is every live portal account, marked with whether it's on the
// master list, plus listed members whose account has since been removed.
// can_vote: on the list with an active account.
const votersFrom = `(
	SELECT COALESCE(TRIM(u.member_id), '') AS member_no,
	       CONCAT_WS(', ', NULLIF(TRIM(u.last_name), ''),
	                 NULLIF(TRIM(CONCAT_WS(' ', u.first_name, u.middle_name, u.suffix)), '')) AS name,
	       u.id AS user_id, u.status AS account_status, m.id IS NOT NULL AS on_list,
	       (m.id IS NOT NULL AND u.status = 'active') AS can_vote
	FROM users u
	LEFT JOIN voter_master_list m ON m.user_id = u.id
	WHERE ` + liveAccount + `
	UNION ALL
	SELECT m.member_no, m.listed_name, m.user_id, NULL, TRUE, FALSE
	FROM voter_master_list m
	LEFT JOIN users u ON u.id = m.user_id AND ` + liveAccount + `
	WHERE u.id IS NULL
) x`

func voterSearch(search string) (string, []any) {
	like := "%" + utils.EscapeLike(search) + "%"
	return `(? = '' OR x.member_no = ? OR COALESCE(x.name, '') LIKE ?)`, []any{search, search, like}
}

// CountVoters counts members who can and can't vote, matching search.
func (r *VotersRepository) CountVoters(ctx context.Context, search string) (map[string]int, error) {
	where, args := voterSearch(search)
	var eligible, ineligible int
	err := r.DB.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(x.can_vote), 0), COALESCE(SUM(NOT x.can_vote), 0) FROM `+votersFrom+` WHERE `+where,
		args...,
	).Scan(&eligible, &ineligible)
	if err != nil {
		return nil, err
	}
	return map[string]int{
		models.VoterAll: eligible + ineligible, models.VoterEligible: eligible, models.VoterIneligible: ineligible,
	}, nil
}

// ListVoters pages one tab (or everyone) by name. Members on the master list
// come first, so on the non-voting tab those blocked by their account lead.
func (r *VotersRepository) ListVoters(ctx context.Context, q models.VoterListQuery) ([]models.Voter, error) {
	where, args := voterSearch(q.Search)
	if q.Status != models.VoterAll {
		where += ` AND x.can_vote = ?`
		args = append(args, q.Status == models.VoterEligible)
	}
	args = append(args, q.PageSize, (q.Page-1)*q.PageSize)

	rows, err := r.DB.QueryContext(ctx,
		`SELECT x.member_no, COALESCE(x.name, ''), x.user_id, COALESCE(x.account_status, ''), x.on_list, x.can_vote
		 FROM `+votersFrom+`
		 WHERE `+where+`
		 ORDER BY x.on_list DESC, COALESCE(x.name, '') = '', x.name, x.member_no
		 LIMIT ? OFFSET ?`,
		args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	voters := make([]models.Voter, 0, q.PageSize)
	for rows.Next() {
		var v models.Voter
		var userID int64
		if err := rows.Scan(&v.MemberNo, &v.Name, &userID, &v.AccountStatus, &v.OnMasterList, &v.CanVote); err != nil {
			return nil, err
		}
		switch {
		case v.AccountStatus == "":
			v.Reason = models.ReasonNoAccount
		case !v.OnMasterList:
			v.Reason = models.ReasonNotListed
		case !v.CanVote:
			v.Reason = models.ReasonAccountInactive
		}
		if v.AccountStatus != "" {
			v.AccountID = app_models.FormatID(cc.PORTAL_MEMBER_ID_PREFIX, userID)
		}
		voters = append(voters, v)
	}
	return voters, rows.Err()
}
