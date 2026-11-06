package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

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

// activeWhere is maintenance in force right now: switched on, started, and
// not yet lifted. scheduledWhere is switched on but not started yet. Both run
// in SQL so this service and fairchild_be judge them by the same (database)
// clock. COALESCE: with starts_at NULL the comparison is NULL, not false.
const (
	activeWhere    = `(s.maintenance_mode AND (s.starts_at IS NULL OR s.starts_at <= NOW()) AND (s.ends_at IS NULL OR s.ends_at > NOW()))`
	scheduledWhere = `(s.maintenance_mode AND COALESCE(s.starts_at > NOW(), FALSE))`
)

const siteStatusSelect = `
SELECT ` + activeWhere + ` AS active, ` + scheduledWhere + ` AS scheduled,
       s.starts_at, s.ends_at, s.message, s.updated_at,
       COALESCE(NULLIF(TRIM(CONCAT(u.first_name, ' ', u.last_name)), ''), u.username)
FROM site_status s
LEFT JOIN users_panel u ON u.id = s.updated_by_user_panel_id
WHERE s.id = 1`

func scanSiteStatus(row interface{ Scan(...any) error }) (*models.SiteStatus, error) {
	var s models.SiteStatus
	var scheduled bool
	var startsAt, endsAt sql.NullTime
	var updatedBy sql.NullString
	err := row.Scan(&s.MaintenanceMode, &scheduled, &startsAt, &endsAt, &s.Message, &s.UpdatedAt, &updatedBy)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errSiteStatusMissing
	}
	if err != nil {
		return nil, err
	}
	if scheduled {
		s.StartsAt = &startsAt.Time
	}
	// A lapsed end time means nothing once maintenance is over.
	if (s.MaintenanceMode || scheduled) && endsAt.Valid {
		s.EndsAt = &endsAt.Time
	}
	s.UpdatedBy = updatedBy.String
	return &s, nil
}

func (r *SiteRepository) GetSiteStatus(ctx context.Context) (*models.SiteStatus, error) {
	return scanSiteStatus(r.DB.QueryRowContext(ctx, siteStatusSelect))
}

// UpdateSiteStatus locks the row, reads the previous state, writes the new
// one and the audit entry built from the previous state - all in one
// transaction, so two admins toggling at once get a correct audit trail. If
// the save puts maintenance in force, member sessions are signed out in the
// same transaction (see signOutIfDue) and the count returned.
func (r *SiteRepository) UpdateSiteStatus(ctx context.Context, maintenanceMode bool, message string, startsAt *time.Time, durationMinutes *int, updatedBy int64, audit func(previous *models.SiteStatus) *audit_models.AuditLog) (int64, error) {
	var signedOut int64
	err := database.WithTx(ctx, r.DB, func(tx *sql.Tx) error {
		previous, err := scanSiteStatus(tx.QueryRowContext(ctx, siteStatusSelect+` FOR UPDATE`))
		if err != nil {
			return err
		}

		// starts_at: the scheduled start, or NULL for "now" (and when off).
		var start any
		if maintenanceMode && startsAt != nil {
			start = *startsAt
		}

		// ends_at, counted from the start: NULL when off or cleared,
		// start+n when a duration is given, otherwise whatever was set while
		// maintenance was already on or scheduled - unless the new start is
		// past it.
		endsAtExpr, endsArgs := "NULL", []any{}
		switch {
		case !maintenanceMode:
		case durationMinutes != nil && *durationMinutes > 0:
			endsAtExpr, endsArgs = "COALESCE(?, NOW()) + INTERVAL ? MINUTE", []any{start, *durationMinutes}
		case durationMinutes == nil && (previous.MaintenanceMode || previous.Scheduled()):
			endsAtExpr, endsArgs = "CASE WHEN ends_at > COALESCE(?, NOW()) THEN ends_at END", []any{start}
		}

		// updated_at is set explicitly: ON UPDATE CURRENT_TIMESTAMP doesn't
		// fire when a save changes nothing, and the panel keys its form on
		// updatedAt to know the save landed.
		args := append([]any{maintenanceMode, message, start}, endsArgs...)
		args = append(args, updatedBy)
		if _, err := tx.ExecContext(ctx,
			`UPDATE site_status
			 SET maintenance_mode = ?, message = ?, starts_at = ?, ends_at = `+endsAtExpr+`,
			     updated_by_user_panel_id = ?, updated_at = NOW()
			 WHERE id = 1`,
			args...,
		); err != nil {
			return err
		}

		// Off or not started yet: the next window signs members out afresh.
		if _, err := tx.ExecContext(ctx,
			`UPDATE site_status s SET s.signed_out_at = NULL WHERE s.id = 1 AND NOT `+activeWhere,
		); err != nil {
			return err
		}

		if signedOut, err = signOutIfDue(ctx, tx); err != nil {
			return err
		}
		return audit_repo.InsertAuditLogTx(ctx, tx, audit(previous))
	})
	return signedOut, err
}

// SignOutIfDue signs every member out once a scheduled start has passed.
// Safe to call from every server instance: only one claims the window.
func (r *SiteRepository) SignOutIfDue(ctx context.Context) (int64, error) {
	var signedOut int64
	err := database.WithTx(ctx, r.DB, func(tx *sql.Tx) error {
		var err error
		signedOut, err = signOutIfDue(ctx, tx)
		return err
	})
	return signedOut, err
}

// signOutIfDue claims the current maintenance window (signed_out_at) if it's
// in force and nobody has yet, then revokes every live member session in
// auth_sessions (fairchild_be's table; rows only, never its schema), so
// members have to sign in again once it lifts.
func signOutIfDue(ctx context.Context, tx *sql.Tx) (int64, error) {
	res, err := tx.ExecContext(ctx,
		`UPDATE site_status s SET s.signed_out_at = NOW()
		 WHERE s.id = 1 AND s.signed_out_at IS NULL AND `+activeWhere)
	if err != nil {
		return 0, err
	}
	if claimed, err := res.RowsAffected(); err != nil || claimed == 0 {
		return 0, err
	}

	res, err = tx.ExecContext(ctx,
		`UPDATE auth_sessions SET revoked_at = NOW()
		 WHERE revoked_at IS NULL AND expires_at > NOW()`)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
