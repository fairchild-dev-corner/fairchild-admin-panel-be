package repository

import (
	"context"
	"database/sql"
	"fmt"

	cc "fairchild-admin-panel-be/internal/constants"
)

/**
* ApplicationsRepository
* @Description: Wraps the central Auth DB pool for loan_applications (ours)
* and membership_applications (fairchild_be's - we flip its status column,
* keep review metadata in membership_application_reviews, and insert pending
* rows only for requests staff enter manually).
**/
type ApplicationsRepository struct {
	DB *sql.DB
}

func NewApplicationsRepository(db *sql.DB) *ApplicationsRepository {
	return &ApplicationsRepository{DB: db}
}

// listLimit caps list endpoints - the panel renders them unpaginated.
const listLimit = 500

// reviewerNameSQL renders the reviewing panel user (aliased u) the same way
// UserPanel.DisplayName does in Go.
const reviewerNameSQL = `COALESCE(NULLIF(TRIM(CONCAT(u.first_name, ' ', u.last_name)), ''), u.username)`

// explainNoRowsReviewed is called when a "WHERE status = 'pending'" review
// UPDATE touched nothing, to tell "doesn't exist" apart from "already
// reviewed". Runs inside the review's transaction.
func explainNoRowsReviewed(ctx context.Context, tx *sql.Tx, table string, id int64) error {
	var exists int
	err := tx.QueryRowContext(ctx, fmt.Sprintf(`SELECT 1 FROM %s WHERE id = ?`, table), id).Scan(&exists)
	if err == sql.ErrNoRows {
		return cc.ErrApplicationNotFound
	}
	if err != nil {
		return err
	}
	return cc.ErrApplicationAlreadyReviewed
}
