package repository

import (
	"context"
	"database/sql"
	"strconv"
	"strings"

	models "fairchild-admin-panel-be/internal/models/audit"
	"fairchild-admin-panel-be/internal/utils"
)

/**
* AuditRepository
* @Description: Reads admin_audit_logs. Writes go through InsertAuditLogTx,
* called by the other repositories inside the same transaction as the
* mutation being audited.
**/
type AuditRepository struct {
	DB *sql.DB
}

func NewAuditRepository(db *sql.DB) *AuditRepository {
	return &AuditRepository{DB: db}
}

// ListRecentAuditLogs returns the newest entries first.
func (r *AuditRepository) ListRecentAuditLogs(ctx context.Context, limit int) ([]models.AuditLogEntry, error) {
	rows, err := r.DB.QueryContext(ctx,
		`SELECT id, actor_name, action, target, created_at
		 FROM admin_audit_logs
		 ORDER BY created_at DESC, id DESC
		 LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	entries := make([]models.AuditLogEntry, 0)
	for rows.Next() {
		var e models.AuditLogEntry
		var id int64
		if err := rows.Scan(&id, &e.Actor, &e.Action, &e.Target, &e.At); err != nil {
			return nil, err
		}
		e.ID = strconv.FormatInt(id, 10)
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// ListAuditLogs pages the entries whose action is one of q.Actions (and
// q.Action, if set) and whose actor or target contains q.Search, newest
// first, with the total that match.
func (r *AuditRepository) ListAuditLogs(ctx context.Context, q models.AuditLogQuery) ([]models.AuditLogEntry, int, error) {
	if len(q.Actions) == 0 {
		return []models.AuditLogEntry{}, 0, nil
	}
	where := `action IN (?` + strings.Repeat(", ?", len(q.Actions)-1) + `)`
	args := make([]any, 0, len(q.Actions)+5)
	for _, a := range q.Actions {
		args = append(args, a)
	}
	if q.Action != "" {
		where += ` AND action = ?`
		args = append(args, q.Action)
	}
	if q.Search != "" {
		like := "%" + utils.EscapeLike(q.Search) + "%"
		where += ` AND (actor_name LIKE ? OR target LIKE ?)`
		args = append(args, like, like)
	}

	var total int
	if err := r.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM admin_audit_logs WHERE `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.DB.QueryContext(ctx,
		`SELECT id, actor_name, action, target, COALESCE(ip_address, ''), created_at
		 FROM admin_audit_logs
		 WHERE `+where+`
		 ORDER BY created_at DESC, id DESC
		 LIMIT ? OFFSET ?`,
		append(args, q.PageSize, (q.Page-1)*q.PageSize)...,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	entries := make([]models.AuditLogEntry, 0, q.PageSize)
	for rows.Next() {
		var e models.AuditLogEntry
		var id int64
		if err := rows.Scan(&id, &e.Actor, &e.Action, &e.Target, &e.IPAddress, &e.At); err != nil {
			return nil, 0, err
		}
		e.ID = strconv.FormatInt(id, 10)
		entries = append(entries, e)
	}
	return entries, total, rows.Err()
}

// InsertAuditLogTx writes one admin_audit_logs row inside tx, so the entry
// commits or rolls back together with the mutation it describes.
func InsertAuditLogTx(ctx context.Context, tx *sql.Tx, e *models.AuditLog) error {
	_, err := tx.ExecContext(ctx,
		`INSERT INTO admin_audit_logs (user_panel_id, actor_name, action, target, ip_address)
		 VALUES (?, ?, ?, ?, ?)`,
		e.UserPanelID, utils.Truncate(e.ActorName, 200), e.Action, e.Target, utils.NullIfEmpty(e.IPAddress),
	)
	return err
}
