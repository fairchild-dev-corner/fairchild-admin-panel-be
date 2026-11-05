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
	models "fairchild-admin-panel-be/internal/models/blasts"
	audit_repo "fairchild-admin-panel-be/internal/repositories/audit"
	"fairchild-admin-panel-be/internal/utils"
	"fairchild-admin-panel-be/pkg/database"
)

/**
* BlastsRepository
* @Description: SMS/email blasts (`message_blasts`) and their queued
* messages (`message_blast_recipients`), both owned by this service. The
* audience is read from fairchild_be's `users` and never written. Workers
* claim messages with FOR UPDATE SKIP LOCKED, so more than one server
* instance never sends the same message twice.
**/
type BlastsRepository struct {
	DB *sql.DB
}

func NewBlastsRepository(db *sql.DB) *BlastsRepository {
	return &BlastsRepository{DB: db}
}

// staleClaim is how long a claimed message may stay "sending" before
// another worker retries it (its worker most likely stopped mid-batch).
const staleClaim = 15 * time.Minute

// recipientInsertChunk keeps each INSERT well under max_allowed_packet.
const recipientInsertChunk = 500

func FormatBlastID(id int64) string {
	return app_models.FormatID(cc.MESSAGE_BLAST_ID_PREFIX, id)
}

// ListAudience reads every live portal account with one of these statuses.
// The text goes to the account's mobile, or a Young Saver's guardian's -
// the same fallback fairchild_be uses for OTPs.
func (r *BlastsRepository) ListAudience(ctx context.Context, statuses []string) ([]models.AudienceMember, error) {
	if len(statuses) == 0 {
		return nil, nil
	}
	args := make([]any, len(statuses))
	for i, s := range statuses {
		args[i] = s
	}
	rows, err := r.DB.QueryContext(ctx,
		`SELECT u.id, COALESCE(TRIM(u.member_id), ''), COALESCE(TRIM(u.user_client_id), ''), u.status,
		        TRIM(CONCAT_WS(' ', u.first_name, u.last_name)), COALESCE(TRIM(u.email), ''),
		        COALESCE(NULLIF(TRIM(u.mobile_number), ''),
		                 IF(u.member_type = 'young_saver', NULLIF(TRIM(u.guardian_mobile_number), ''), NULL), '')
		 FROM users u
		 WHERE u.status <> 'deleted' AND u.deleted_at IS NULL
		   AND u.status IN (?`+strings.Repeat(", ?", len(statuses)-1)+`)
		 ORDER BY u.id`,
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	members := make([]models.AudienceMember, 0, 1024)
	for rows.Next() {
		var m models.AudienceMember
		if err := rows.Scan(&m.UserID, &m.MemberNo, &m.ClientID, &m.Status, &m.Name, &m.Email, &m.Mobile); err != nil {
			return nil, err
		}
		members = append(members, m)
	}
	return members, rows.Err()
}

func (r *BlastsRepository) Create(ctx context.Context, b *models.Blast, createdByID int64, recipients []models.NewRecipient, audit *audit_models.AuditLog) (int64, error) {
	var id int64
	err := database.WithTx(ctx, r.DB, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx,
			`INSERT INTO message_blasts (channel, subject, email_body, sms_body, audience_statuses, audience_branches,
			                             test_mode, created_by_user_panel_id, created_by_name)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			b.Channel, utils.NullIfEmpty(b.Subject), utils.NullIfEmpty(b.EmailBody), utils.NullIfEmpty(b.SMSBody),
			strings.Join(b.Audience.Statuses, ","), strings.Join(b.Audience.Branches, ","),
			b.TestMode, createdByID, b.CreatedBy,
		)
		if err != nil {
			return err
		}
		if id, err = res.LastInsertId(); err != nil {
			return err
		}

		for start := 0; start < len(recipients); start += recipientInsertChunk {
			chunk := recipients[start:min(len(recipients), start+recipientInsertChunk)]
			args := make([]any, 0, len(chunk)*8)
			for _, rc := range chunk {
				status, errMsg := models.RecipientPending, sql.NullString{}
				if rc.Skip {
					status, errMsg = models.RecipientSkipped, utils.NullIfEmpty(cc.BLAST_SKIPPED_TEST_MODE)
				}
				args = append(args, id, rc.UserID, rc.MemberNo, rc.Name, rc.Channel, rc.Recipient, status, errMsg)
			}
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO message_blast_recipients (blast_id, user_id, member_id, name, channel, recipient, status, error)
				 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`+strings.Repeat(", (?, ?, ?, ?, ?, ?, ?, ?)", len(chunk)-1),
				args...,
			); err != nil {
				return err
			}
		}

		audit.Target = FormatBlastID(id)
		return audit_repo.InsertAuditLogTx(ctx, tx, audit)
	})
	return id, err
}

const blastSelect = `
SELECT b.id, b.channel, COALESCE(b.subject, ''), COALESCE(b.email_body, ''), COALESCE(b.sms_body, ''),
       b.audience_statuses, b.audience_branches, b.status, b.test_mode, b.created_by_name,
       b.created_at, b.finished_at
FROM message_blasts b`

func scanBlast(row interface{ Scan(...any) error }) (*models.Blast, int64, error) {
	var b models.Blast
	var id int64
	var statuses, branches string
	var created time.Time
	var finished sql.NullTime
	if err := row.Scan(&id, &b.Channel, &b.Subject, &b.EmailBody, &b.SMSBody,
		&statuses, &branches, &b.Status, &b.TestMode, &b.CreatedBy, &created, &finished); err != nil {
		return nil, 0, err
	}
	b.ID = FormatBlastID(id)
	b.Audience = models.BlastAudience{Statuses: splitList(statuses), Branches: splitList(branches)}
	b.CreatedAt = created.UTC().Format(time.RFC3339)
	if finished.Valid {
		b.FinishedAt = finished.Time.UTC().Format(time.RFC3339)
	}
	b.ByChannel = map[string]models.BlastCounts{}
	return &b, id, nil
}

func splitList(s string) []string {
	out := []string{}
	for _, v := range strings.Split(s, ",") {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
}

// List returns one page of blasts, newest first, with their counts.
func (r *BlastsRepository) List(ctx context.Context, page, pageSize int) ([]models.Blast, int, error) {
	var total int
	if err := r.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM message_blasts`).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.DB.QueryContext(ctx, blastSelect+` ORDER BY b.id DESC LIMIT ? OFFSET ?`, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	blasts := make([]models.Blast, 0, pageSize)
	ids := make([]int64, 0, pageSize)
	for rows.Next() {
		b, id, err := scanBlast(rows)
		if err != nil {
			return nil, 0, err
		}
		blasts = append(blasts, *b)
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return blasts, total, r.attachCounts(ctx, blasts, ids)
}

func (r *BlastsRepository) FindByID(ctx context.Context, id int64) (*models.Blast, error) {
	b, _, err := scanBlast(r.DB.QueryRowContext(ctx, blastSelect+` WHERE b.id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	blasts := []models.Blast{*b}
	if err := r.attachCounts(ctx, blasts, []int64{id}); err != nil {
		return nil, err
	}
	return &blasts[0], nil
}

// attachCounts tallies each blast's messages by channel and outcome.
func (r *BlastsRepository) attachCounts(ctx context.Context, blasts []models.Blast, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	args := make([]any, len(ids))
	pos := make(map[int64]int, len(ids))
	for i, id := range ids {
		args[i], pos[id] = id, i
	}
	rows, err := r.DB.QueryContext(ctx,
		`SELECT blast_id, channel, status, COUNT(*) FROM message_blast_recipients
		 WHERE blast_id IN (?`+strings.Repeat(", ?", len(ids)-1)+`)
		 GROUP BY blast_id, channel, status`,
		args...,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var id int64
		var channel, status string
		var n int
		if err := rows.Scan(&id, &channel, &status, &n); err != nil {
			return err
		}
		b := &blasts[pos[id]]
		b.Counts.Add(status, n)
		c := b.ByChannel[channel]
		c.Add(status, n)
		b.ByChannel[channel] = c
	}
	return rows.Err()
}

// Failures lists a blast's most recent failed messages, then skipped ones
// other than test-mode skips (there can be thousands of those).
func (r *BlastsRepository) Failures(ctx context.Context, id int64, limit int) ([]models.BlastFailure, error) {
	rows, err := r.DB.QueryContext(ctx,
		`SELECT member_id, name, channel, recipient, status, COALESCE(error, '')
		 FROM message_blast_recipients
		 WHERE blast_id = ? AND (status = 'failed' OR (status = 'skipped' AND COALESCE(error, '') <> ?))
		 ORDER BY status = 'failed' DESC, id DESC LIMIT ?`,
		id, cc.BLAST_SKIPPED_TEST_MODE, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	failures := []models.BlastFailure{}
	for rows.Next() {
		var f models.BlastFailure
		if err := rows.Scan(&f.MemberNo, &f.Name, &f.Channel, &f.Recipient, &f.Status, &f.Error); err != nil {
			return nil, err
		}
		failures = append(failures, f)
	}
	return failures, rows.Err()
}

// Cancel stops a sending blast: its unsent messages, including ones a worker
// has claimed, are skipped. A worker checks the status before each send; a
// message already in flight finishes and its real outcome replaces the skip.
func (r *BlastsRepository) Cancel(ctx context.Context, id int64, audit *audit_models.AuditLog) error {
	return database.WithTx(ctx, r.DB, func(tx *sql.Tx) error {
		var status string
		err := tx.QueryRowContext(ctx, `SELECT status FROM message_blasts WHERE id = ? FOR UPDATE`, id).Scan(&status)
		if errors.Is(err, sql.ErrNoRows) {
			return cc.ErrBlastNotFound
		}
		if err != nil {
			return err
		}
		if status != models.StatusSending {
			return cc.ErrBlastNotSending
		}

		if _, err := tx.ExecContext(ctx,
			`UPDATE message_blasts SET status = 'cancelled', finished_at = NOW() WHERE id = ?`, id); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE message_blast_recipients SET status = 'skipped', error = ?
			 WHERE blast_id = ? AND status IN ('pending', 'sending')`,
			cc.BLAST_SKIPPED_CANCELLED, id); err != nil {
			return err
		}

		audit.Target = FormatBlastID(id)
		return audit_repo.InsertAuditLogTx(ctx, tx, audit)
	})
}

// InsertAudit records an action with no other write (a test send).
func (r *BlastsRepository) InsertAudit(ctx context.Context, audit *audit_models.AuditLog) error {
	return database.WithTx(ctx, r.DB, func(tx *sql.Tx) error {
		return audit_repo.InsertAuditLogTx(ctx, tx, audit)
	})
}

func (r *BlastsRepository) NextSendingBlast(ctx context.Context) (*models.BlastJob, error) {
	var j models.BlastJob
	err := r.DB.QueryRowContext(ctx,
		`SELECT id, COALESCE(subject, ''), COALESCE(email_body, ''), COALESCE(sms_body, '')
		 FROM message_blasts WHERE status = 'sending' ORDER BY id LIMIT 1`,
	).Scan(&j.ID, &j.Subject, &j.EmailBody, &j.SMSBody)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &j, nil
}

func (r *BlastsRepository) ClaimBatch(ctx context.Context, blastID int64, limit int) ([]models.QueuedMessage, error) {
	var batch []models.QueuedMessage
	err := database.WithTx(ctx, r.DB, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx,
			`UPDATE message_blast_recipients SET status = 'pending', claimed_at = NULL
			 WHERE blast_id = ? AND status = 'sending' AND claimed_at < NOW() - INTERVAL ? SECOND`,
			blastID, int(staleClaim.Seconds())); err != nil {
			return err
		}

		rows, err := tx.QueryContext(ctx,
			`SELECT id, channel, recipient, name FROM message_blast_recipients
			 WHERE blast_id = ? AND status = 'pending' ORDER BY id LIMIT ? FOR UPDATE SKIP LOCKED`,
			blastID, limit)
		if err != nil {
			return err
		}
		ids := []any{}
		for rows.Next() {
			var m models.QueuedMessage
			if err := rows.Scan(&m.ID, &m.Channel, &m.Recipient, &m.Name); err != nil {
				rows.Close()
				return err
			}
			batch = append(batch, m)
			ids = append(ids, m.ID)
		}
		rows.Close()
		if err := rows.Err(); err != nil || len(ids) == 0 {
			return err
		}

		_, err = tx.ExecContext(ctx,
			`UPDATE message_blast_recipients SET status = 'sending', claimed_at = NOW()
			 WHERE id IN (?`+strings.Repeat(", ?", len(ids)-1)+`)`, ids...)
		return err
	})
	return batch, err
}

func (r *BlastsRepository) MarkResult(ctx context.Context, recipientID int64, status, errMsg string) error {
	if len(errMsg) > 255 {
		errMsg = errMsg[:255]
	}
	_, err := r.DB.ExecContext(ctx,
		`UPDATE message_blast_recipients
		 SET status = ?, error = ?, sent_at = IF(? = 'sent', NOW(), sent_at)
		 WHERE id = ?`,
		status, utils.NullIfEmpty(errMsg), status, recipientID)
	return err
}

func (r *BlastsRepository) BlastStatus(ctx context.Context, blastID int64) (string, error) {
	var status string
	err := r.DB.QueryRowContext(ctx, `SELECT status FROM message_blasts WHERE id = ?`, blastID).Scan(&status)
	return status, err
}

func (r *BlastsRepository) FinishIfDone(ctx context.Context, blastID int64) error {
	_, err := r.DB.ExecContext(ctx,
		`UPDATE message_blasts SET status = 'completed', finished_at = NOW()
		 WHERE id = ? AND status = 'sending'
		   AND NOT EXISTS (SELECT 1 FROM message_blast_recipients
		                   WHERE blast_id = ? AND status IN ('pending', 'sending'))`,
		blastID, blastID)
	return err
}
