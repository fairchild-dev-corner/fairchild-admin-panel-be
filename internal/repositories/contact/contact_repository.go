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
	models "fairchild-admin-panel-be/internal/models/contact"
	audit_repo "fairchild-admin-panel-be/internal/repositories/audit"
	portal_repo "fairchild-admin-panel-be/internal/repositories/portal"
	"fairchild-admin-panel-be/internal/utils"
	"fairchild-admin-panel-be/pkg/database"
)

/**
* ContactRepository
* @Description: fairchild_be's `contact_messages` (the public Contact Us
* form), read only, plus this service's `contact_message_replies`. A message
* is "replied" once it has a reply row. The portal account with the same
* email (users.email is unique) is joined for text replies.
**/
type ContactRepository struct {
	DB *sql.DB
}

func NewContactRepository(db *sql.DB) *ContactRepository {
	return &ContactRepository{DB: db}
}

const contactFrom = `
FROM contact_messages m
LEFT JOIN users u ON u.email = m.email AND u.status <> 'deleted' AND u.deleted_at IS NULL`

// The account's own mobile, or a Young Saver's guardian's - the same
// fallback fairchild_be uses for OTPs.
const contactSelect = `
SELECT m.id, m.name, m.email, m.message, m.created_at,
       u.id, COALESCE(u.member_id, ''), COALESCE(u.first_name, ''), COALESCE(u.last_name, ''),
       COALESCE(NULLIF(TRIM(u.mobile_number), ''),
                IF(u.member_type = 'young_saver', NULLIF(TRIM(u.guardian_mobile_number), ''), NULL), '')` + contactFrom

const repliedWhere = `EXISTS (SELECT 1 FROM contact_message_replies r WHERE r.contact_message_id = m.id)`

func FormatMessageID(id int64) string {
	return app_models.FormatID(cc.CONTACT_MESSAGE_ID_PREFIX, id)
}

func scanMessage(row interface{ Scan(...any) error }) (*models.ContactMessage, int64, error) {
	var m models.ContactMessage
	var id int64
	var received time.Time
	var accountID sql.NullInt64
	var a models.ContactAccount

	if err := row.Scan(
		&id, &m.Name, &m.Email, &m.Message, &received,
		&accountID, &a.MemberNo, &a.FirstName, &a.LastName, &a.MobileNumber,
	); err != nil {
		return nil, 0, err
	}

	m.ID = FormatMessageID(id)
	m.ReceivedAt = received.UTC().Format(time.RFC3339)
	m.Replies = []models.ContactReply{}
	if accountID.Valid {
		a.ID = portal_repo.FormatPortalID(accountID.Int64)
		m.Account = &a
	}
	return &m, id, nil
}

// searchWhere matches the message id ("CM-00012" or "12") or a substring of
// the sender's name, email or the message.
func searchWhere(search string) (string, []any) {
	like := "%" + utils.EscapeLike(search) + "%"
	searchID, _ := app_models.ParseID(cc.CONTACT_MESSAGE_ID_PREFIX, search)
	return `(? = '' OR m.id = ? OR m.name LIKE ? OR m.email LIKE ? OR m.message LIKE ?)`,
		[]any{search, searchID, like, like, like}
}

func (r *ContactRepository) CountByStatus(ctx context.Context, search string) (map[string]int, error) {
	where, args := searchWhere(search)
	var replied, total int
	if err := r.DB.QueryRowContext(ctx,
		`SELECT COUNT(*), COALESCE(SUM(`+repliedWhere+`), 0) FROM contact_messages m WHERE `+where,
		args...,
	).Scan(&total, &replied); err != nil {
		return nil, err
	}
	return map[string]int{"unreplied": total - replied, "replied": replied}, nil
}

// List returns one page of messages with this status. Unreplied messages are
// oldest first (the order to answer them in); replied ones, and every message
// for cc.FILTER_ALL, newest first.
func (r *ContactRepository) List(ctx context.Context, q models.ContactListQuery) ([]models.ContactMessage, error) {
	where, args := searchWhere(q.Search)
	status, order := `NOT `+repliedWhere, `m.created_at, m.id`
	switch q.Status {
	case "replied":
		status, order = repliedWhere, `m.created_at DESC, m.id DESC`
	case cc.FILTER_ALL:
		status, order = `TRUE`, `m.created_at DESC, m.id DESC`
	}
	args = append(args, q.PageSize, (q.Page-1)*q.PageSize)

	rows, err := r.DB.QueryContext(ctx,
		contactSelect+` WHERE `+status+` AND `+where+` ORDER BY `+order+` LIMIT ? OFFSET ?`,
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	messages := make([]models.ContactMessage, 0, q.PageSize)
	ids := make([]int64, 0, q.PageSize)
	for rows.Next() {
		m, id, err := scanMessage(rows)
		if err != nil {
			return nil, err
		}
		messages = append(messages, *m)
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return messages, r.attachReplies(ctx, messages, ids)
}

// FindByID returns nil, nil for an unknown id.
func (r *ContactRepository) FindByID(ctx context.Context, id int64) (*models.ContactMessage, error) {
	m, _, err := scanMessage(r.DB.QueryRowContext(ctx, contactSelect+` WHERE m.id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	messages := []models.ContactMessage{*m}
	if err := r.attachReplies(ctx, messages, []int64{id}); err != nil {
		return nil, err
	}
	return &messages[0], nil
}

func (r *ContactRepository) attachReplies(ctx context.Context, messages []models.ContactMessage, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	args := make([]any, len(ids))
	index := make(map[int64]int, len(ids))
	for i, id := range ids {
		args[i] = id
		index[id] = i
	}

	rows, err := r.DB.QueryContext(ctx,
		`SELECT id, contact_message_id, channel, recipient, body, sent_by_name, created_at
		 FROM contact_message_replies
		 WHERE contact_message_id IN (?`+strings.Repeat(", ?", len(ids)-1)+`)
		 ORDER BY created_at, id`,
		args...,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var rp models.ContactReply
		var id, messageID int64
		var sent time.Time
		if err := rows.Scan(&id, &messageID, &rp.Channel, &rp.Recipient, &rp.Body, &rp.SentBy, &sent); err != nil {
			return err
		}
		rp.ID = app_models.FormatID(cc.CONTACT_REPLY_ID_PREFIX, id)
		rp.SentAt = sent.UTC().Format(time.RFC3339)
		if i, ok := index[messageID]; ok {
			messages[i].Replies = append(messages[i].Replies, rp)
		}
	}
	return rows.Err()
}

// InsertReply records a reply the provider already accepted, with its audit
// entry.
func (r *ContactRepository) InsertReply(ctx context.Context, messageID int64, channel, recipient, body string, audit *audit_models.AuditLog) error {
	return database.WithTx(ctx, r.DB, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO contact_message_replies
				(contact_message_id, channel, recipient, body, sent_by_user_panel_id, sent_by_name)
			 VALUES (?, ?, ?, ?, ?, ?)`,
			messageID, channel, recipient, body, audit.UserPanelID, utils.Truncate(audit.ActorName, 200),
		); err != nil {
			return err
		}
		return audit_repo.InsertAuditLogTx(ctx, tx, audit)
	})
}
