package repository

import (
	"context"
	"database/sql"
	"errors"

	audit_models "fairchild-admin-panel-be/internal/models/audit"
	models "fairchild-admin-panel-be/internal/models/site"
	audit_repo "fairchild-admin-panel-be/internal/repositories/audit"
	"fairchild-admin-panel-be/pkg/database"
)

/**
* SiteRepository
* @Description: Reads/writes the single-row site_status table.
**/
type SiteRepository struct {
	DB *sql.DB
}

func NewSiteRepository(db *sql.DB) *SiteRepository {
	return &SiteRepository{DB: db}
}

// errSiteStatusMissing means migration 000003's seed row is gone.
var errSiteStatusMissing = errors.New("site_status row missing - re-run migrations")

const siteStatusSelect = `
SELECT s.maintenance_mode, s.message, s.updated_at,
       COALESCE(NULLIF(TRIM(CONCAT(u.first_name, ' ', u.last_name)), ''), u.username)
FROM site_status s
LEFT JOIN users_panel u ON u.id = s.updated_by_user_panel_id
WHERE s.id = 1`

func scanSiteStatus(row interface{ Scan(...any) error }) (*models.SiteStatus, error) {
	var s models.SiteStatus
	var updatedBy sql.NullString
	err := row.Scan(&s.MaintenanceMode, &s.Message, &s.UpdatedAt, &updatedBy)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errSiteStatusMissing
	}
	if err != nil {
		return nil, err
	}
	s.UpdatedBy = updatedBy.String
	return &s, nil
}

func (r *SiteRepository) GetSiteStatus(ctx context.Context) (*models.SiteStatus, error) {
	return scanSiteStatus(r.DB.QueryRowContext(ctx, siteStatusSelect))
}

// UpdateSiteStatus locks the row, reads the previous state, writes the new
// one and the audit entry built from the previous state - all in one
// transaction, so two admins toggling at once get a correct audit trail.
func (r *SiteRepository) UpdateSiteStatus(ctx context.Context, maintenanceMode bool, message string, updatedBy int64, audit func(previous *models.SiteStatus) *audit_models.AuditLog) error {
	return database.WithTx(ctx, r.DB, func(tx *sql.Tx) error {
		previous, err := scanSiteStatus(tx.QueryRowContext(ctx, siteStatusSelect+` FOR UPDATE`))
		if err != nil {
			return err
		}

		// updated_at is set explicitly: ON UPDATE CURRENT_TIMESTAMP doesn't
		// fire when a save changes nothing, and the panel keys its form on
		// updatedAt to know the save landed.
		if _, err := tx.ExecContext(ctx,
			`UPDATE site_status
			 SET maintenance_mode = ?, message = ?, updated_by_user_panel_id = ?, updated_at = NOW()
			 WHERE id = 1`,
			maintenanceMode, message, updatedBy,
		); err != nil {
			return err
		}

		return audit_repo.InsertAuditLogTx(ctx, tx, audit(previous))
	})
}
