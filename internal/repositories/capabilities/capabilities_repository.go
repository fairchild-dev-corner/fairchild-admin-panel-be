package repository

import (
	"context"
	"database/sql"

	audit_models "fairchild-admin-panel-be/internal/models/audit"
	audit_repo "fairchild-admin-panel-be/internal/repositories/audit"
	"fairchild-admin-panel-be/pkg/database"
)

/**
* CapabilitiesRepository
* @Description: `panel_user_capabilities` - what each manager, supervisor
* and staff account may see and do, one row per capability it has. Saving
* writes the rows and the audit entry in one transaction.
**/
type CapabilitiesRepository struct {
	DB *sql.DB
}

func NewCapabilitiesRepository(db *sql.DB) *CapabilitiesRepository {
	return &CapabilitiesRepository{DB: db}
}

func (r *CapabilitiesRepository) FindUserCapabilities(ctx context.Context, userPanelID int64) ([]string, error) {
	rows, err := r.DB.QueryContext(ctx, `SELECT capability FROM panel_user_capabilities WHERE user_panel_id = ?`, userPanelID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	keys := []string{}
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}
	return keys, rows.Err()
}

func (r *CapabilitiesRepository) SaveUserCapabilities(ctx context.Context, userPanelID int64, add, remove []string, grantedBy int64, audit *audit_models.AuditLog) error {
	return database.WithTx(ctx, r.DB, func(tx *sql.Tx) error {
		for _, key := range remove {
			if _, err := tx.ExecContext(ctx,
				`DELETE FROM panel_user_capabilities WHERE user_panel_id = ? AND capability = ?`,
				userPanelID, key); err != nil {
				return err
			}
		}
		for _, key := range add {
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO panel_user_capabilities (user_panel_id, capability, granted_by, granted_at)
				VALUES (?, ?, ?, NOW())
				ON DUPLICATE KEY UPDATE granted_by = VALUES(granted_by), granted_at = NOW()`,
				userPanelID, key, grantedBy); err != nil {
				return err
			}
		}
		return audit_repo.InsertAuditLogTx(ctx, tx, audit)
	})
}
