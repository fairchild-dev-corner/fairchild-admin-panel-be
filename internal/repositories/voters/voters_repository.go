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

	"github.com/go-sql-driver/mysql"
)

/**
* VotersRepository
* @Description: The voters' master list (`voter_master_list`, with its
* upload log `voter_master_list_uploads`), owned by this service, joined to
* fairchild_be's `users` (read only) to say which members can vote. An
* upload replaces the whole list and writes its audit entry in the same
* transaction. Also marks, undoes and resets members' election token claims
* in fairchild_be's `voter_token_claims` (the portal records claims there).
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
		         WHERE m.is_voting AND `+liveAccount+`),
		        (SELECT COUNT(*) FROM voter_master_list WHERE NOT is_voting)`,
	).Scan(&s.Count, &s.Eligible, &s.NonVoting)
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
			args := make([]any, 0, len(chunk)*10)
			for _, m := range chunk {
				args = append(args, m.UserID, m.MemberNo, m.ListedName, m.Voting, m.Category, m.Delinquent, joinReasons(m.Reasons),
					m.Attendance, m.Survey, m.LuckyPick)
			}
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO voter_master_list
				 (user_id, member_no, listed_name, is_voting, category, is_delinquent, ineligible_reasons,
				  attendance_token, survey_token, lucky_pick_token)
				 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`+
					strings.Repeat(", (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)", len(chunk)-1),
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

// accountName is a portal account's name as "Last, First Middle Suffix".
const accountName = `CONCAT_WS(', ', NULLIF(TRIM(u.last_name), ''),
	                 NULLIF(TRIM(CONCAT_WS(' ', u.first_name, u.middle_name, u.suffix)), ''))`

// SetVoting puts one live portal account on the master list with category
// (adding it if it isn't listed), replacing its ineligibility reasons, and
// audits it in the same transaction.
func (r *VotersRepository) SetVoting(ctx context.Context, userID int64, category string, reasons []string, audit *audit_models.AuditLog) error {
	return database.WithTx(ctx, r.DB, func(tx *sql.Tx) error {
		var memberNo, name string
		err := tx.QueryRowContext(ctx,
			`SELECT COALESCE(TRIM(u.member_id), ''), COALESCE(`+accountName+`, '')
			 FROM users u WHERE u.id = ? AND `+liveAccount,
			userID).Scan(&memberNo, &name)
		if errors.Is(err, sql.ErrNoRows) {
			return cc.ErrPortalMemberNotFound
		}
		if err != nil {
			return err
		}
		if memberNo == "" {
			return &cc.MasterListInputError{Msg: "This portal account has no member ID, so it can't be on the master list"}
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO voter_master_list (user_id, member_no, listed_name, is_voting, category, ineligible_reasons)
			 VALUES (?, ?, ?, ?, ?, ?)
			 ON DUPLICATE KEY UPDATE
			   is_voting = VALUES(is_voting),
			   category = VALUES(category),
			   ineligible_reasons = VALUES(ineligible_reasons)`,
			userID, memberNo, name, category == models.CategoryVoting, category, joinReasons(reasons)); err != nil {
			return err
		}
		audit.Target = memberNo
		return audit_repo.InsertAuditLogTx(ctx, tx, audit)
	})
}

// votersFrom is every live portal account, marked with whether it's on the
// master list, plus listed members whose account has since been removed.
// can_vote: on the list as voting, whatever the account's status (active,
// inactive, suspended or pending review). token_total: their election
// tokens, which only voting members get.
const votersFrom = `(
	SELECT COALESCE(TRIM(u.member_id), '') AS member_no,
	       ` + accountName + ` AS name,
	       u.id AS user_id, u.status AS account_status, m.id IS NOT NULL AS on_list,
	       COALESCE(m.is_voting, FALSE) AS listed_voting,
	       (m.id IS NOT NULL AND m.is_voting) AS can_vote,
	       COALESCE(CASE WHEN m.is_voting THEN ` + tokenTotal + ` END, 0) AS token_total,
	       COALESCE(m.category, '') AS category, COALESCE(m.ineligible_reasons, '') AS ineligible_reasons
	FROM users u
	LEFT JOIN voter_master_list m ON m.user_id = u.id
	WHERE ` + liveAccount + `
	UNION ALL
	SELECT m.member_no, m.listed_name, m.user_id, NULL, TRUE, m.is_voting, FALSE, 0,
	       m.category, COALESCE(m.ineligible_reasons, '')
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
		`SELECT x.member_no, COALESCE(x.name, ''), x.user_id, COALESCE(x.account_status, ''), x.on_list, x.listed_voting, x.can_vote,
		        x.token_total, x.category, x.ineligible_reasons
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
	userIDs := make([]int64, 0, q.PageSize)
	for rows.Next() {
		var v models.Voter
		var userID int64
		var reasons string
		if err := rows.Scan(&v.MemberNo, &v.Name, &userID, &v.AccountStatus, &v.OnMasterList, &v.ListedVoting, &v.CanVote,
			&v.TokenTotal, &v.Category, &reasons); err != nil {
			return nil, err
		}
		if v.Category == models.CategoryIneligible {
			v.IneligibleReasons = splitReasons(reasons)
		}
		userIDs = append(userIDs, userID)
		switch {
		case v.AccountStatus == "":
			v.Reason = models.ReasonNoAccount
		case !v.OnMasterList:
			v.Reason = models.ReasonNotListed
		case !v.ListedVoting && v.Category == models.CategoryIneligible:
			v.Reason = models.ReasonListedIneligible
		case !v.ListedVoting:
			v.Reason = models.ReasonListedNonVoting
		}
		if v.AccountStatus != "" {
			v.AccountID = app_models.FormatID(cc.PORTAL_MEMBER_ID_PREFIX, userID)
		}
		voters = append(voters, v)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	claimed, err := r.claimedUsers(ctx, q.ClaimYear, userIDs)
	if err != nil {
		return nil, err
	}
	for i := range voters {
		voters[i].TokenClaimed = claimed[userIDs[i]]
	}
	return voters, nil
}

// tokenTotal is a master list row's election tokens in pesos.
const tokenTotal = `(m.attendance_token + m.survey_token + m.lucky_pick_token)`

// claimsTableMissing: voter_token_claims is created by fairchild_be's
// migrations. Until they've run, read it as having no claims rather than
// breaking the Members List.
func claimsTableMissing(err error) bool {
	var mysqlErr *mysql.MySQLError
	return errors.As(err, &mysqlErr) && mysqlErr.Number == 1146
}

// claimedUsers reports which of userIDs have claimed their tokens in year.
func (r *VotersRepository) claimedUsers(ctx context.Context, year int, userIDs []int64) (map[int64]bool, error) {
	claimed := map[int64]bool{}
	if len(userIDs) == 0 {
		return claimed, nil
	}
	args := []any{year}
	for _, id := range userIDs {
		args = append(args, id)
	}
	rows, err := r.DB.QueryContext(ctx,
		`SELECT user_id FROM voter_token_claims WHERE claim_year = ? AND user_id IN (?`+
			strings.Repeat(", ?", len(userIDs)-1)+`)`,
		args...)
	if claimsTableMissing(err) {
		return claimed, nil
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		claimed[id] = true
	}
	return claimed, rows.Err()
}

// SetTokenClaim records the member's election token claim for year with the
// amounts on the master list (e.g. paid out at the branch), or removes it,
// and audits it in the same transaction. Already in that state: nothing
// changes and nothing is audited.
func (r *VotersRepository) SetTokenClaim(ctx context.Context, userID int64, year int, claimed bool, audit *audit_models.AuditLog) error {
	return database.WithTx(ctx, r.DB, func(tx *sql.Tx) error {
		var memberNo string
		var canVote bool
		var tokens models.Tokens
		err := tx.QueryRowContext(ctx,
			`SELECT COALESCE(TRIM(u.member_id), ''), COALESCE(m.is_voting, FALSE),
			        COALESCE(m.attendance_token, 0), COALESCE(m.survey_token, 0), COALESCE(m.lucky_pick_token, 0)
			 FROM users u LEFT JOIN voter_master_list m ON m.user_id = u.id
			 WHERE u.id = ? AND `+liveAccount,
			userID).Scan(&memberNo, &canVote, &tokens.Attendance, &tokens.Survey, &tokens.LuckyPick)
		if errors.Is(err, sql.ErrNoRows) {
			return cc.ErrPortalMemberNotFound
		}
		if err != nil {
			return err
		}

		var res sql.Result
		if claimed {
			// Tokens are for voting members only, as on the portal.
			if !canVote {
				return &cc.MasterListInputError{Msg: "Only voting members can claim election tokens"}
			}
			if tokens.Attendance+tokens.Survey+tokens.LuckyPick <= 0 {
				return &cc.MasterListInputError{Msg: "This member has no election tokens on the master list"}
			}
			res, err = tx.ExecContext(ctx,
				`INSERT IGNORE INTO voter_token_claims (user_id, claim_year, attendance_amount, survey_amount, lucky_pick_amount)
				 VALUES (?, ?, ?, ?, ?)`,
				userID, year, tokens.Attendance, tokens.Survey, tokens.LuckyPick)
		} else {
			res, err = tx.ExecContext(ctx,
				`DELETE FROM voter_token_claims WHERE user_id = ? AND claim_year = ?`, userID, year)
		}
		if err != nil {
			return err
		}
		if n, err := res.RowsAffected(); err != nil || n == 0 {
			return err
		}
		audit.Target = memberNo
		return audit_repo.InsertAuditLogTx(ctx, tx, audit)
	})
}

// TokenClaimsSummary counts the token claims on record, every year.
func (r *VotersRepository) TokenClaimsSummary(ctx context.Context) (*models.TokenClaimsSummary, error) {
	var s models.TokenClaimsSummary
	err := r.DB.QueryRowContext(ctx,
		`SELECT COUNT(*), COALESCE(SUM(attendance_amount + survey_amount + lucky_pick_amount), 0) FROM voter_token_claims`,
	).Scan(&s.Claimed, &s.Total)
	if claimsTableMissing(err) {
		return &models.TokenClaimsSummary{}, nil
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// ClearTokenClaims deletes every token claim, so members can claim again at
// the next election, and audits it in the same transaction. Returns how many
// were cleared.
func (r *VotersRepository) ClearTokenClaims(ctx context.Context, audit *audit_models.AuditLog) (int64, error) {
	var cleared int64
	err := database.WithTx(ctx, r.DB, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `DELETE FROM voter_token_claims`)
		if err != nil {
			return err
		}
		if cleared, err = res.RowsAffected(); err != nil {
			return err
		}
		audit.Target = "election token claims"
		return audit_repo.InsertAuditLogTx(ctx, tx, audit)
	})
	return cleared, err
}

func (r *VotersRepository) GetElectionControl(ctx context.Context) (*models.ElectionControl, error) {
	var c models.ElectionControl
	var at time.Time
	err := r.DB.QueryRowContext(ctx,
		`SELECT enabled, COALESCE(updated_by_name, ''), updated_at FROM election_control WHERE id = 1`,
	).Scan(&c.Enabled, &c.UpdatedBy, &at)
	if errors.Is(err, sql.ErrNoRows) {
		// The migration seeds the row; treat a missing one as off.
		return &c, nil
	}
	if err != nil {
		return nil, err
	}
	c.UpdatedAt = at.UTC().Format(time.RFC3339)
	return &c, nil
}

// SetElectionControl turns the election on or off and audits it in the same
// transaction.
func (r *VotersRepository) SetElectionControl(ctx context.Context, enabled bool, byID int64, by string, audit *audit_models.AuditLog) error {
	return database.WithTx(ctx, r.DB, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO election_control (id, enabled, updated_by_user_panel_id, updated_by_name) VALUES (1, ?, ?, ?)
			 ON DUPLICATE KEY UPDATE enabled = VALUES(enabled), updated_by_user_panel_id = VALUES(updated_by_user_panel_id),
			                         updated_by_name = VALUES(updated_by_name), updated_at = NOW()`,
			enabled, byID, by); err != nil {
			return err
		}
		audit.Target = "election"
		return audit_repo.InsertAuditLogTx(ctx, tx, audit)
	})
}

// ClearMasterList empties the master list - every member becomes
// non-voting - and audits it in the same transaction. The upload log is
// kept. Returns how many members were on it.
func (r *VotersRepository) ClearMasterList(ctx context.Context, audit *audit_models.AuditLog) (int64, error) {
	var cleared int64
	err := database.WithTx(ctx, r.DB, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `DELETE FROM voter_master_list`)
		if err != nil {
			return err
		}
		if cleared, err = res.RowsAffected(); err != nil {
			return err
		}
		audit.Target = "master list"
		return audit_repo.InsertAuditLogTx(ctx, tx, audit)
	})
	return cleared, err
}

// VoterStatuses reads every member's voting status with the keys needed to
// find their ledger branch: live portal accounts, plus listed members whose
// account was removed since (non-voting, no client id).
func (r *VotersRepository) VoterStatuses(ctx context.Context) ([]models.VoterStatus, error) {
	rows, err := r.DB.QueryContext(ctx,
		`SELECT COALESCE(TRIM(u.member_id), ''), COALESCE(TRIM(u.user_client_id), ''),
		        m.id IS NOT NULL AND m.is_voting, m.id IS NOT NULL, COALESCE(m.category, '')
		 FROM users u
		 LEFT JOIN voter_master_list m ON m.user_id = u.id
		 WHERE `+liveAccount+`
		 UNION ALL
		 SELECT m.member_no, '', FALSE, TRUE, m.category
		 FROM voter_master_list m
		 LEFT JOIN users u ON u.id = m.user_id AND `+liveAccount+`
		 WHERE u.id IS NULL`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	statuses := make([]models.VoterStatus, 0, 16384)
	for rows.Next() {
		var v models.VoterStatus
		if err := rows.Scan(&v.MemberNo, &v.ClientID, &v.CanVote, &v.OnList, &v.Category); err != nil {
			return nil, err
		}
		statuses = append(statuses, v)
	}
	return statuses, rows.Err()
}

// joinReasons stores a member's ineligibility reasons one per line, or NULL.
func joinReasons(reasons []string) any {
	if len(reasons) == 0 {
		return nil
	}
	return strings.Join(reasons, "\n")
}

// splitReasons reads ineligible_reasons (one per line).
func splitReasons(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			out = append(out, line)
		}
	}
	return out
}
