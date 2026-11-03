package repository

import (
	"context"
	"database/sql"
	"strconv"

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
