package database

import (
	"database/sql"
	"fmt"
	"slices"

	models "fairchild-admin-panel-be/internal/models/config"
)

// BranchDBs holds one pooled connection per branch ledger DB (main, knowles,
// tiic, ...). Read-only from this service: only aggregate queries for the
// dashboard and pings for the health page ever run against them.
type BranchDBs map[string]*sql.DB

const branchMaxOpenConns = 3

// InitBranchDBs connects to every configured branch, failing fast (and
// closing whatever already opened) if any one of them is unreachable.
func InitBranchDBs(confs map[string]*models.MySQLConfig) (BranchDBs, error) {
	dbs := make(BranchDBs, len(confs))
	for code, c := range confs {
		db, err := MySQLInitConnector(c, false)
		if err != nil {
			dbs.Close()
			return nil, fmt.Errorf("connect branch %q (%s): %w", code, c.DBName, err)
		}
		db.SetMaxOpenConns(branchMaxOpenConns)
		db.SetMaxIdleConns(1)
		dbs[code] = db
	}
	return dbs, nil
}

func (b BranchDBs) Close() {
	for _, db := range b {
		db.Close()
	}
}

// Codes returns the branch codes in a stable order, so the dashboard and
// health page list branches the same way on every request.
func (b BranchDBs) Codes() []string {
	codes := make([]string, 0, len(b))
	for code := range b {
		codes = append(codes, code)
	}
	slices.Sort(codes)
	return codes
}
