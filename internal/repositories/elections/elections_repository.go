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
	models "fairchild-admin-panel-be/internal/models/elections"
	audit_repo "fairchild-admin-panel-be/internal/repositories/audit"
	"fairchild-admin-panel-be/internal/utils"
	"fairchild-admin-panel-be/pkg/database"
)

/**
* ElectionsRepository
* @Description: Elections set up from the panel (`elections`,
* `election_positions`, `election_candidates`), all owned by this service.
* `election_votes` is only read here (results, turnout); the member portal
* writes it. Candidates and the voter count come from fairchild_be's
* `users`, which is never written. Every change writes its audit entry in
* the same transaction.
**/
type ElectionsRepository struct {
	DB *sql.DB
}

func NewElectionsRepository(db *sql.DB) *ElectionsRepository {
	return &ElectionsRepository{DB: db}
}

func FormatElectionID(id int64) string {
	return app_models.FormatID(cc.ELECTION_ID_PREFIX, id)
}

func formatChildID(prefix string, id int64) string {
	return app_models.FormatID(prefix, id)
}

// An eligible voter is on the shared voters' master list with an active
// portal account.
const electionSelect = `
SELECT e.id, e.title, COALESCE(e.description, ''), e.starts_at, e.ends_at, e.published_at,
       e.created_by_name, e.created_at,
       (SELECT COUNT(*) FROM election_positions p WHERE p.election_id = e.id),
       (SELECT COUNT(*) FROM election_candidates c JOIN election_positions p ON p.id = c.position_id WHERE p.election_id = e.id),
       (SELECT COUNT(DISTINCT v.user_id) FROM election_votes v WHERE v.election_id = e.id),
       (SELECT COUNT(*) FROM voter_master_list),
       (SELECT COUNT(*) FROM voter_master_list m JOIN users u ON u.id = m.user_id
        WHERE u.status = 'active' AND u.deleted_at IS NULL)
FROM elections e`

func scanElection(row interface{ Scan(...any) error }) (*models.Election, int64, bool, error) {
	var e models.Election
	var id int64
	var starts, ends, created time.Time
	var published sql.NullTime
	if err := row.Scan(&id, &e.Title, &e.Description, &starts, &ends, &published,
		&e.CreatedBy, &created, &e.PositionCount, &e.CandidateCount, &e.Voters,
		&e.MasterListCount, &e.EligibleVoters); err != nil {
		return nil, 0, false, err
	}
	e.ID = FormatElectionID(id)
	e.StartsAt = starts.UTC().Format(time.RFC3339)
	e.EndsAt = ends.UTC().Format(time.RFC3339)
	e.CreatedAt = created.UTC().Format(time.RFC3339)
	if published.Valid {
		e.PublishedAt = published.Time.UTC().Format(time.RFC3339)
	}
	return &e, id, published.Valid, nil
}

func (r *ElectionsRepository) ListAll(ctx context.Context) ([]models.Election, []bool, error) {
	rows, err := r.DB.QueryContext(ctx, electionSelect+` ORDER BY e.starts_at DESC, e.id DESC`)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	var elections []models.Election
	var published []bool
	for rows.Next() {
		e, _, pub, err := scanElection(rows)
		if err != nil {
			return nil, nil, err
		}
		elections = append(elections, *e)
		published = append(published, pub)
	}
	return elections, published, rows.Err()
}

// FindByID loads the election with its positions, candidates and their vote
// counts, in the order they were entered.
func (r *ElectionsRepository) FindByID(ctx context.Context, id int64) (*models.Election, bool, error) {
	e, _, published, err := scanElection(r.DB.QueryRowContext(ctx, electionSelect+` WHERE e.id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}

	rows, err := r.DB.QueryContext(ctx,
		`SELECT p.id, p.title, p.seats, c.id, c.member_no, c.name, COALESCE(c.bio, ''),
		        (SELECT COUNT(*) FROM election_votes v WHERE v.candidate_id = c.id)
		 FROM election_positions p
		 LEFT JOIN election_candidates c ON c.position_id = p.id
		 WHERE p.election_id = ?
		 ORDER BY p.sort_order, p.id, c.sort_order, c.id`, id)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()

	e.Positions = []models.Position{}
	var lastPos int64
	for rows.Next() {
		var posID int64
		var pos models.Position
		var candID sql.NullInt64
		var c models.Candidate
		var memberNo, name sql.NullString
		if err := rows.Scan(&posID, &pos.Title, &pos.Seats, &candID, &memberNo, &name, &c.Bio, &c.Votes); err != nil {
			return nil, false, err
		}
		if posID != lastPos {
			pos.ID = formatChildID("P-", posID)
			pos.Candidates = []models.Candidate{}
			e.Positions = append(e.Positions, pos)
			lastPos = posID
		}
		if candID.Valid {
			c.ID, c.MemberNo, c.Name = formatChildID("C-", candID.Int64), memberNo.String, name.String
			last := &e.Positions[len(e.Positions)-1]
			last.Candidates = append(last.Candidates, c)
		}
	}
	return e, published, rows.Err()
}

func (r *ElectionsRepository) FindMembers(ctx context.Context, memberNos []string) (map[string]models.CandidateMember, error) {
	found := map[string]models.CandidateMember{}
	if len(memberNos) == 0 {
		return found, nil
	}
	args := make([]any, len(memberNos))
	for i, m := range memberNos {
		args[i] = m
	}
	rows, err := r.DB.QueryContext(ctx,
		`SELECT id, member_id, TRIM(CONCAT_WS(' ', first_name, middle_name, last_name, suffix)), status
		 FROM users
		 WHERE status <> 'deleted' AND deleted_at IS NULL
		   AND member_id IN (?`+strings.Repeat(", ?", len(memberNos)-1)+`)`,
		args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var m models.CandidateMember
		if err := rows.Scan(&m.UserID, &m.MemberNo, &m.Name, &m.Status); err != nil {
			return nil, err
		}
		found[strings.ToUpper(m.MemberNo)] = m
	}
	return found, rows.Err()
}

func (r *ElectionsRepository) Create(ctx context.Context, d *models.ElectionDraft, createdByID int64, createdBy string, audit *audit_models.AuditLog) (int64, error) {
	var id int64
	err := database.WithTx(ctx, r.DB, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx,
			`INSERT INTO elections (title, description, starts_at, ends_at, created_by_user_panel_id, created_by_name)
			 VALUES (?, ?, ?, ?, ?, ?)`,
			d.Title, utils.NullIfEmpty(d.Description), d.StartsAt.UTC(), d.EndsAt.UTC(), createdByID, createdBy)
		if err != nil {
			return err
		}
		if id, err = res.LastInsertId(); err != nil {
			return err
		}
		if err := insertPositions(ctx, tx, id, d.Positions); err != nil {
			return err
		}
		audit.Target = FormatElectionID(id)
		return audit_repo.InsertAuditLogTx(ctx, tx, audit)
	})
	return id, err
}

func insertPositions(ctx context.Context, tx *sql.Tx, electionID int64, positions []models.PositionDraft) error {
	for i, p := range positions {
		res, err := tx.ExecContext(ctx,
			`INSERT INTO election_positions (election_id, title, seats, sort_order) VALUES (?, ?, ?, ?)`,
			electionID, p.Title, p.Seats, i)
		if err != nil {
			return err
		}
		posID, err := res.LastInsertId()
		if err != nil {
			return err
		}
		for j, c := range p.Candidates {
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO election_candidates (position_id, user_id, member_no, name, bio, sort_order)
				 VALUES (?, ?, ?, ?, ?, ?)`,
				posID, c.Member.UserID, c.Member.MemberNo, c.Member.Name, utils.NullIfEmpty(c.Bio), j); err != nil {
				return err
			}
		}
	}
	return nil
}

// lockElection reads an election's publish state and whether its voting has
// started, locking the row for the rest of the transaction.
func lockElection(ctx context.Context, tx *sql.Tx, id int64) (published, started bool, err error) {
	err = tx.QueryRowContext(ctx,
		`SELECT published_at IS NOT NULL, starts_at <= NOW() FROM elections WHERE id = ? FOR UPDATE`, id,
	).Scan(&published, &started)
	if errors.Is(err, sql.ErrNoRows) {
		return false, false, cc.ErrElectionNotFound
	}
	return published, started, err
}

// Update replaces the election's details, positions and candidates. Allowed
// for a draft, or a published election whose voting hasn't started (so no
// votes exist yet).
func (r *ElectionsRepository) Update(ctx context.Context, id int64, d *models.ElectionDraft, audit *audit_models.AuditLog) error {
	return database.WithTx(ctx, r.DB, func(tx *sql.Tx) error {
		published, started, err := lockElection(ctx, tx, id)
		if err != nil {
			return err
		}
		if published && started {
			return cc.ErrElectionLocked
		}

		if _, err := tx.ExecContext(ctx,
			`UPDATE elections SET title = ?, description = ?, starts_at = ?, ends_at = ? WHERE id = ?`,
			d.Title, utils.NullIfEmpty(d.Description), d.StartsAt.UTC(), d.EndsAt.UTC(), id); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM election_positions WHERE election_id = ?`, id); err != nil {
			return err
		}
		if err := insertPositions(ctx, tx, id, d.Positions); err != nil {
			return err
		}
		audit.Target = FormatElectionID(id)
		return audit_repo.InsertAuditLogTx(ctx, tx, audit)
	})
}

func (r *ElectionsRepository) SetPublished(ctx context.Context, id int64, publish bool, audit *audit_models.AuditLog) error {
	return database.WithTx(ctx, r.DB, func(tx *sql.Tx) error {
		published, started, err := lockElection(ctx, tx, id)
		if err != nil {
			return err
		}
		switch {
		case publish && published:
			return cc.ErrElectionNotDraft
		case !publish && !published:
			return cc.ErrElectionNotPublished
		case !publish && started:
			return cc.ErrElectionLocked
		}

		query := `UPDATE elections SET published_at = NOW() WHERE id = ?`
		if !publish {
			query = `UPDATE elections SET published_at = NULL WHERE id = ?`
		}
		if _, err := tx.ExecContext(ctx, query, id); err != nil {
			return err
		}
		audit.Target = FormatElectionID(id)
		return audit_repo.InsertAuditLogTx(ctx, tx, audit)
	})
}

func (r *ElectionsRepository) Delete(ctx context.Context, id int64, audit *audit_models.AuditLog) error {
	return database.WithTx(ctx, r.DB, func(tx *sql.Tx) error {
		published, _, err := lockElection(ctx, tx, id)
		if err != nil {
			return err
		}
		if published {
			return cc.ErrElectionNotDraft
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM elections WHERE id = ?`, id); err != nil {
			return err
		}
		audit.Target = FormatElectionID(id)
		return audit_repo.InsertAuditLogTx(ctx, tx, audit)
	})
}
