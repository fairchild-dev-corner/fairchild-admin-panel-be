package database

import (
	"context"
	"database/sql"
	"fmt"
)

// WithTx runs fn inside a transaction, committing if it returns nil and
// rolling back otherwise. Admin mutations use it so the change and its
// admin_audit_logs row are written together or not at all.
func WithTx(ctx context.Context, db *sql.DB, fn func(tx *sql.Tx) error) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}

	if err := fn(tx); err != nil {
		tx.Rollback()
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}
