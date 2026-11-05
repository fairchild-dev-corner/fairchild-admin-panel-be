package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	cc "fairchild-admin-panel-be/internal/constants"
	app_models "fairchild-admin-panel-be/internal/models/applications"
	audit_models "fairchild-admin-panel-be/internal/models/audit"
	models "fairchild-admin-panel-be/internal/models/locks"
	audit_repo "fairchild-admin-panel-be/internal/repositories/audit"
	portal_repo "fairchild-admin-panel-be/internal/repositories/portal"
	"fairchild-admin-panel-be/internal/utils"
	"fairchild-admin-panel-be/pkg/database"
)

/**
* LocksRepository
* @Description: fairchild_be's `locked_member_accounts` - written when a
* member ID has too many failed portal logins - joined to the live portal
* account (`users`) it was recorded against. The only write is an unlock,
* which clears the lock flag (the row stays as history) and writes the audit
* entry in one transaction. Who unlocked a lock is read back from that entry.
**/
type LocksRepository struct {
	DB *sql.DB
}

func NewLocksRepository(db *sql.DB) *LocksRepository {
	return &LocksRepository{DB: db}
}

// lockTTLHours must match fairchild_be's lockedMemberAccountTTLHours: a lock
// blocks login only while it's flagged, unexpired and this recent (older
// rows expire 10 days out but no longer block).
const lockTTLHours = 24

var activeLock = fmt.Sprintf(
	`(l.is_locked = TRUE AND l.expires_at > NOW() AND l.locked_at > NOW() - INTERVAL %d HOUR)`, lockTTLHours)

// unlockedAtExpr is when an inactive lock stopped blocking: when it was
// lifted, or else when it ran out.
var unlockedAtExpr = fmt.Sprintf(
	`COALESCE(l.unlocked_at, LEAST(l.expires_at, l.locked_at + INTERVAL %d HOUR))`, lockTTLHours)

// A lock whose account was removed shows as having no portal account.
const lockFrom = `
FROM locked_member_accounts l
LEFT JOIN users u ON u.id = l.user_id AND u.status <> 'deleted' AND u.deleted_at IS NULL`

var lockSelect = `
SELECT l.id, l.member_id, COALESCE(l.ip_address, ''), l.attempt_count, l.reason,
       l.locked_at, l.expires_at, l.notified_at, ` + activeLock + `, l.unlocked_at, ` + unlockedAtExpr + `,
       u.id, COALESCE(u.first_name, ''), COALESCE(u.last_name, ''), COALESCE(u.middle_name, ''),
       COALESCE(u.suffix, ''), COALESCE(u.email, ''), COALESCE(u.mobile_number, ''), COALESCE(u.status, ''),
       COALESCE(NULLIF(TRIM(u.mobile_number), ''),
                IF(u.member_type = 'young_saver', NULLIF(TRIM(u.guardian_mobile_number), ''), NULL), '')` + lockFrom

func FormatLockID(id int64) string {
	return app_models.FormatID(cc.LOCKED_ACCOUNT_ID_PREFIX, id)
}

func scanLock(row interface{ Scan(...any) error }) (*models.LockedAccount, error) {
	var l models.LockedAccount
	var id int64
	var lockedAt, expiresAt, unlockedAt time.Time
	var notified, liftedAt sql.NullTime
	var active bool
	var accountID sql.NullInt64
	var a models.LockedAccountUser

	if err := row.Scan(
		&id, &l.MemberNo, &l.IPAddress, &l.AttemptCount, &l.Reason,
		&lockedAt, &expiresAt, &notified, &active, &liftedAt, &unlockedAt,
		&accountID, &a.FirstName, &a.LastName, &a.MiddleName,
		&a.Suffix, &a.Email, &a.MobileNumber, &a.Status,
		&a.NotifyMobile,
	); err != nil {
		return nil, err
	}

	l.ID = FormatLockID(id)
	l.LockedAt = lockedAt.UTC().Format(time.RFC3339)
	l.ExpiresAt = expiresAt.UTC().Format(time.RFC3339)
	if notified.Valid {
		l.NotifiedAt = notified.Time.UTC().Format(time.RFC3339)
	}
	if active {
		l.Status = models.Statuses[0]
	} else {
		l.Status = models.Statuses[1]
		l.UnlockedAt = unlockedAt.UTC().Format(time.RFC3339)
		// attachUnlockers upgrades this to panel when an admin did it.
		l.UnlockedVia = models.UnlockedViaExpired
		if liftedAt.Valid {
			l.UnlockedVia = models.UnlockedViaOther
		}
	}
	if accountID.Valid {
		a.ID = portal_repo.FormatPortalID(accountID.Int64)
		l.Account = &a
	}
	return &l, nil
}

// searchWhere matches the member number, the lock id ("LK-00012" or "12"),
// or a substring of the account's name.
func searchWhere(search string) (string, []any) {
	like := "%" + utils.EscapeLike(search) + "%"
	searchID, _ := app_models.ParseID(cc.LOCKED_ACCOUNT_ID_PREFIX, search)
	return `(? = '' OR l.id = ? OR l.member_id = ?
	        OR CONCAT_WS(' ', u.first_name, u.middle_name, u.last_name, u.suffix) LIKE ?)`,
		[]any{search, searchID, search, like}
}

// statusWhere selects one tab and applies the date range to that tab's date:
// when it was locked, or when it was unlocked.
func statusWhere(q models.LockedAccountListQuery, status string) (string, []any) {
	cond, date := activeLock, `l.locked_at`
	if status == models.Statuses[1] {
		cond, date = `NOT `+activeLock, unlockedAtExpr
	}
	if q.From != nil && q.To != nil {
		return `(` + cond + ` AND ` + date + ` >= ? AND ` + date + ` < ?)`, []any{*q.From, *q.To}
	}
	return fmt.Sprintf(`(%s AND %s >= NOW() - INTERVAL %d MONTH)`, cond, date, models.DefaultWindowMonths), nil
}

func (r *LocksRepository) CountByStatus(ctx context.Context, q models.LockedAccountListQuery) (map[string]int, error) {
	lockedWhere, lockedArgs := statusWhere(q, models.Statuses[0])
	unlockedWhere, unlockedArgs := statusWhere(q, models.Statuses[1])
	where, searchArgs := searchWhere(q.Search)

	args := append(append(lockedArgs, unlockedArgs...), searchArgs...)
	var locked, unlocked int
	err := r.DB.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(`+lockedWhere+`), 0), COALESCE(SUM(`+unlockedWhere+`), 0)`+lockFrom+` WHERE `+where,
		args...,
	).Scan(&locked, &unlocked)
	if err != nil {
		return nil, err
	}
	return map[string]int{models.Statuses[0]: locked, models.Statuses[1]: unlocked}, nil
}

// List returns one page of the tab, newest first by that tab's date.
func (r *LocksRepository) List(ctx context.Context, q models.LockedAccountListQuery) ([]models.LockedAccount, error) {
	status, statusArgs := statusWhere(q, q.Status)
	where, searchArgs := searchWhere(q.Search)
	order := `l.locked_at DESC, l.id DESC`
	if q.Status == models.Statuses[1] {
		order = unlockedAtExpr + ` DESC, l.id DESC`
	}

	args := append(append(statusArgs, searchArgs...), q.PageSize, (q.Page-1)*q.PageSize)
	rows, err := r.DB.QueryContext(ctx,
		lockSelect+` WHERE `+status+` AND `+where+` ORDER BY `+order+` LIMIT ? OFFSET ?`,
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	locks := make([]models.LockedAccount, 0, q.PageSize)
	for rows.Next() {
		l, err := scanLock(rows)
		if err != nil {
			return nil, err
		}
		locks = append(locks, *l)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return locks, r.attachUnlockers(ctx, locks)
}

// FindByID returns nil, nil for an unknown id.
func (r *LocksRepository) FindByID(ctx context.Context, id int64) (*models.LockedAccount, error) {
	l, err := scanLock(r.DB.QueryRowContext(ctx, lockSelect+` WHERE l.id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	locks := []models.LockedAccount{*l}
	if err := r.attachUnlockers(ctx, locks); err != nil {
		return nil, err
	}
	return &locks[0], nil
}

// unlockAuditSlack is how far apart an unlock's unlocked_at and its audit
// entry's created_at may be. Both are written in the same transaction, so
// they're normally the same second.
const unlockAuditSlack = 5 * time.Second

// attachUnlockers fills UnlockedBy from the panel's unlock audit entries.
// locked_member_accounts has no unlocked-by column and the audit target is
// the member number, so an entry is matched by member number and time.
func (r *LocksRepository) attachUnlockers(ctx context.Context, locks []models.LockedAccount) error {
	var targets []any
	for _, l := range locks {
		if l.UnlockedVia == models.UnlockedViaOther {
			targets = append(targets, l.MemberNo)
		}
	}
	if len(targets) == 0 {
		return nil
	}

	args := append([]any{cc.AUDIT_UNLOCKED_MEMBER_ACCOUNT}, targets...)
	rows, err := r.DB.QueryContext(ctx,
		`SELECT target, actor_name, created_at FROM admin_audit_logs
		 WHERE action = ? AND target IN (?`+strings.Repeat(", ?", len(targets)-1)+`)`,
		args...,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	type entry struct {
		actor string
		at    time.Time
	}
	byMember := make(map[string][]entry)
	for rows.Next() {
		var target string
		var e entry
		if err := rows.Scan(&target, &e.actor, &e.at); err != nil {
			return err
		}
		byMember[strings.ToUpper(target)] = append(byMember[strings.ToUpper(target)], e)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for i := range locks {
		if locks[i].UnlockedVia != models.UnlockedViaOther {
			continue
		}
		unlocked, _ := time.Parse(time.RFC3339, locks[i].UnlockedAt)
		for _, e := range byMember[strings.ToUpper(locks[i].MemberNo)] {
			if d := e.at.Sub(unlocked); d >= -unlockAuditSlack && d <= unlockAuditSlack {
				locks[i].UnlockedBy, locks[i].UnlockedVia = e.actor, models.UnlockedViaPanel
				break
			}
		}
	}
	return nil
}

// Unlock lifts every active lock on the lock's member ID, the same UPDATE
// fairchild_be's own unlock runs, and audits it against the member number.
// cc.ErrLockedAccountNotFound if the lock is unknown or no longer active.
func (r *LocksRepository) Unlock(ctx context.Context, id int64, audit *audit_models.AuditLog) error {
	return database.WithTx(ctx, r.DB, func(tx *sql.Tx) error {
		var memberNo string
		err := tx.QueryRowContext(ctx,
			`SELECT l.member_id FROM locked_member_accounts l WHERE l.id = ? AND `+activeLock+` FOR UPDATE`,
			id,
		).Scan(&memberNo)
		if errors.Is(err, sql.ErrNoRows) {
			return cc.ErrLockedAccountNotFound
		}
		if err != nil {
			return err
		}

		if _, err := tx.ExecContext(ctx,
			`UPDATE locked_member_accounts SET is_locked = FALSE, unlocked_at = NOW()
			 WHERE member_id = ? AND is_locked = TRUE`,
			memberNo,
		); err != nil {
			return err
		}

		audit.Target = memberNo
		return audit_repo.InsertAuditLogTx(ctx, tx, audit)
	})
}
