package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	models "fairchild-admin-panel-be/internal/models/members"
	portal_models "fairchild-admin-panel-be/internal/models/portal"
	"fairchild-admin-panel-be/internal/utils"
	"fairchild-admin-panel-be/pkg/database"
)

/**
* LedgerMembersRepository
* @Description: Reads cooperative members from each branch ledger DB's
* `client` table. Read-only - the accounting system owns these rows - and,
* like every ledger query, never joined with central-DB tables.
**/
type LedgerMembersRepository struct {
	BranchDBs database.BranchDBs
}

func NewLedgerMembersRepository(branchDBs database.BranchDBs) *LedgerMembersRepository {
	return &LedgerMembersRepository{BranchDBs: branchDBs}
}

// ledgerBranchCodes maps the panel's branches to client.ClientIDBrCode
// (and BranchesID in each ledger's `branches` table).
var ledgerBranchCodes = map[string]int{"main": 1, "knowles": 2, "tiic": 3}

// clientTypeCategories maps client.ClientType (the ledger's `clienttype`
// table) to panel categories. Other types - Youth, Non-Member, Outlet,
// Supplier, Others - aren't members and are never listed.
var clientTypeCategories = map[int]string{1: "regular", 2: "associate", 3: "kiddie", 4: "aflatoun"}

func categoryClientType(category string) (int, bool) {
	for t, c := range clientTypeCategories {
		if c == category {
			return t, true
		}
	}
	return 0, false
}

// FormatLedgerMemberID renders a ledger member's id as "<branch code>-<client
// id>", e.g. 01-012345. It can't collide with a panel id (FC20310).
func FormatLedgerMemberID(branch string, clientID int64) string {
	return fmt.Sprintf("%02d-%06d", ledgerBranchCodes[branch], clientID)
}

// ParseLedgerMemberID is the inverse of FormatLedgerMemberID.
func ParseLedgerMemberID(raw string) (branch string, clientID int64, ok bool) {
	code, id, found := strings.Cut(strings.TrimSpace(raw), "-")
	if !found {
		return "", 0, false
	}
	brCode, err1 := strconv.Atoi(code)
	clientID, err2 := strconv.ParseInt(id, 10, 64)
	if err1 != nil || err2 != nil || clientID <= 0 {
		return "", 0, false
	}
	for b, c := range ledgerBranchCodes {
		if c == brCode {
			return b, clientID, true
		}
	}
	return "", 0, false
}

// currentMemberWhere keeps current members only: member client types, and
// not Closed (3), Disapproved (98) or deceased. Dormant (2) and InActive (4)
// accounts are still members and show as inactive. Args: branch code.
const currentMemberWhere = `
c.ClientIDBrCode = ? AND c.ClientID > 0
AND c.ClientType IN (1, 2, 3, 4)
AND (c.AccountStatus IS NULL OR c.AccountStatus NOT IN (3, 98))
AND c.DateDeceased IS NULL`

// About a fifth of main's members (mostly older records) have blank
// FName/LName and only AccountName, "LAST, FIRST MIDDLE". For those the
// surname is the part before the comma and the first name everything after.
const (
	ledgerLastName  = `COALESCE(NULLIF(TRIM(c.LName), ''), TRIM(SUBSTRING_INDEX(c.AccountName, ',', 1)), '')`
	ledgerFirstName = `COALESCE(NULLIF(TRIM(c.FName), ''),
         IF(COALESCE(TRIM(c.LName), '') = '' AND LOCATE(',', c.AccountName) > 0,
            TRIM(SUBSTRING(c.AccountName, LOCATE(',', c.AccountName) + 1)), NULL), '')`
)

// ledgerSearchWhere matches a substring of the name (or AccountName), email
// or cell number, the member number (OldClientID) or the client id. Args
// from ledgerSearchArgs.
const ledgerSearchWhere = `
AND (? = '' OR c.ClientID = ? OR c.OldClientID = ?
     OR CONCAT_WS(' ', c.FName, c.MName, c.LName, c.SName) LIKE ?
     OR CONCAT_WS(' ', c.LName, c.FName) LIKE ?
     OR c.AccountName LIKE ?
     OR c.EmailAddress LIKE ? OR c.CellNum LIKE ?)`

func ledgerSearchArgs(search string) []any {
	like := "%" + utils.EscapeLike(search) + "%"
	// A full ledger id ("01-012345") searches by its client id.
	clientID, _ := strconv.ParseInt(search, 10, 64)
	if _, id, ok := ParseLedgerMemberID(search); ok {
		clientID = id
	}
	return []any{search, clientID, search, like, like, like, like, like}
}

const ledgerMemberSelect = `
SELECT c.ClientID, COALESCE(TRIM(c.OldClientID), ''), c.ClientType,
       ` + ledgerFirstName + `, ` + ledgerLastName + `,
       COALESCE(TRIM(c.MName), ''), COALESCE(TRIM(c.SName), ''),
       c.DateOfBirth, c.Gender,
       COALESCE(TRIM(c.EmailAddress), ''),
       COALESCE(NULLIF(TRIM(c.CellNum), ''), TRIM(c.SMSNumber), ''),
       CONCAT_WS(', ', NULLIF(TRIM(c.ResAddStreet), ''), NULLIF(TRIM(c.ResAddBarangay), ''),
                       NULLIF(TRIM(c.ResAddCity), ''), NULLIF(TRIM(c.ResAddProvince), '')),
       c.AccountStatus, c.DateOpened
FROM client c`

func (r *LedgerMembersRepository) Branches() []string {
	var out []string
	for _, b := range models.Branches {
		if r.BranchDBs[b] != nil {
			out = append(out, b)
		}
	}
	return out
}

func (r *LedgerMembersRepository) db(branch string) (*sql.DB, error) {
	db := r.BranchDBs[branch]
	if db == nil {
		return nil, fmt.Errorf("no ledger DB configured for branch %q", branch)
	}
	return db, nil
}

func (r *LedgerMembersRepository) CountMembers(ctx context.Context, branch, search string) (map[string]int, error) {
	db, err := r.db(branch)
	if err != nil {
		return nil, err
	}

	rows, err := db.QueryContext(ctx,
		`SELECT c.ClientType, COUNT(*) FROM client c WHERE `+currentMemberWhere+ledgerSearchWhere+` GROUP BY c.ClientType`,
		append([]any{ledgerBranchCodes[branch]}, ledgerSearchArgs(search)...)...,
	)
	if err != nil {
		return nil, fmt.Errorf("count %s ledger members: %w", branch, err)
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var clientType, n int
		if err := rows.Scan(&clientType, &n); err != nil {
			return nil, err
		}
		counts[clientTypeCategories[clientType]] = n
	}
	return counts, rows.Err()
}

func (r *LedgerMembersRepository) ListMembers(ctx context.Context, branch, category, search string, offset, limit int) ([]models.Member, error) {
	db, err := r.db(branch)
	if err != nil {
		return nil, err
	}
	clientType, ok := categoryClientType(category)
	if !ok {
		return []models.Member{}, nil
	}

	args := append([]any{ledgerBranchCodes[branch]}, ledgerSearchArgs(search)...)
	args = append(args, clientType, limit, offset)

	// By the effective surname (see ledgerLastName), so AccountName-only
	// records sort among the rest instead of first.
	rows, err := db.QueryContext(ctx,
		ledgerMemberSelect+` WHERE `+currentMemberWhere+ledgerSearchWhere+`
		   AND c.ClientType = ?
		 ORDER BY `+ledgerLastName+`, `+ledgerFirstName+`, c.MName, c.SName, c.ClientID
		 LIMIT ? OFFSET ?`,
		args...,
	)
	if err != nil {
		return nil, fmt.Errorf("list %s ledger members: %w", branch, err)
	}
	defer rows.Close()

	members := make([]models.Member, 0, limit)
	for rows.Next() {
		m, err := scanLedgerMember(rows, branch)
		if err != nil {
			return nil, err
		}
		members = append(members, *m)
	}
	return members, rows.Err()
}

func (r *LedgerMembersRepository) FindMember(ctx context.Context, branch string, clientID int64) (*models.Member, error) {
	db, err := r.db(branch)
	if err != nil {
		return nil, err
	}

	m, err := scanLedgerMember(db.QueryRowContext(ctx,
		ledgerMemberSelect+` WHERE `+currentMemberWhere+` AND c.ClientID = ?`,
		ledgerBranchCodes[branch], clientID,
	), branch)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return m, err
}

// ledgerAccountStatuses names client.AccountStatus (the ledger's acctstat
// table).
var ledgerAccountStatuses = map[int64]string{1: "active", 2: "dormant", 3: "closed", 4: "inactive", 98: "disapproved", 99: "new"}

// LoadMemberNos returns every member number (client.OldClientID) on one
// branch ledger with what the Portal page shows about it. Unlike the member
// list it includes closed accounts, so the page can show that a ledger
// account was closed. ~20k small rows per branch; the caller caches it.
func (r *LedgerMembersRepository) LoadMemberNos(ctx context.Context, branch string) ([]portal_models.LedgerLink, error) {
	db, err := r.db(branch)
	if err != nil {
		return nil, err
	}

	rows, err := db.QueryContext(ctx,
		`SELECT c.ClientID, TRIM(c.OldClientID), c.ClientType, c.AccountStatus,
		        CONCAT_WS(', ', NULLIF(TRIM(c.ResAddStreet), ''), NULLIF(TRIM(c.ResAddBarangay), ''),
		                        NULLIF(TRIM(c.ResAddCity), ''), NULLIF(TRIM(c.ResAddProvince), ''))
		 FROM client c
		 WHERE c.ClientIDBrCode = ? AND c.ClientID > 0 AND TRIM(c.OldClientID) <> ''`,
		ledgerBranchCodes[branch],
	)
	if err != nil {
		return nil, fmt.Errorf("load %s ledger member numbers: %w", branch, err)
	}
	defer rows.Close()

	var links []portal_models.LedgerLink
	for rows.Next() {
		var l portal_models.LedgerLink
		var clientType, status sql.NullInt64
		var address sql.NullString
		if err := rows.Scan(&l.ClientID, &l.MemberNo, &clientType, &status, &address); err != nil {
			return nil, err
		}
		l.Branch = branch
		l.Category = clientTypeCategories[int(clientType.Int64)]
		l.Status = ledgerAccountStatuses[status.Int64]
		l.Address = address.String
		links = append(links, l)
	}
	return links, rows.Err()
}

// FindByMemberNo returns the current member (see currentMemberWhere) with
// that member number on one branch ledger, or nil, nil.
func (r *LedgerMembersRepository) FindByMemberNo(ctx context.Context, branch, memberNo string) (*models.Member, error) {
	db, err := r.db(branch)
	if err != nil {
		return nil, err
	}

	m, err := scanLedgerMember(db.QueryRowContext(ctx,
		ledgerMemberSelect+` WHERE `+currentMemberWhere+` AND TRIM(c.OldClientID) = ?
		 ORDER BY c.ClientID LIMIT 1`,
		ledgerBranchCodes[branch], memberNo,
	), branch)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return m, err
}

func scanLedgerMember(row interface{ Scan(...any) error }, branch string) (*models.Member, error) {
	var m models.Member
	var clientID int64
	var clientType int
	var dob, opened sql.NullTime
	var gender, accountStatus sql.NullInt64

	if err := row.Scan(
		&clientID, &m.MemberNo, &clientType,
		&m.FirstName, &m.LastName, &m.MiddleName, &m.Suffix,
		&dob, &gender, &m.Email, &m.MobileNumber, &m.Address,
		&accountStatus, &opened,
	); err != nil {
		return nil, err
	}

	m.ID = FormatLedgerMemberID(branch, clientID)
	m.Source = models.SourceLedger
	m.Branch = branch
	m.Category = clientTypeCategories[clientType]
	// Minors' programs are laboratory memberships, as in the panel's form.
	m.MembershipType = "coop"
	if m.Category == "kiddie" || m.Category == "aflatoun" {
		m.MembershipType = "laboratory"
	}
	m.DateOfBirth = ledgerDate(dob)
	m.JoinedAt = ledgerDate(opened)
	// gender table: 1 Male, 2 Female, 3 N/A.
	switch gender.Int64 {
	case 1:
		m.Gender = "male"
	case 2:
		m.Gender = "female"
	}
	// acctstat table: 1 Active, 99 New; 2 Dormant and 4 InActive are inactive.
	m.Status = "inactive"
	if accountStatus.Valid && (accountStatus.Int64 == 1 || accountStatus.Int64 == 99) {
		m.Status = "active"
	}
	return &m, nil
}

// ledgerDate formats a ledger DATE/DATETIME as YYYY-MM-DD, or "" when it's
// missing or a zero/placeholder date.
func ledgerDate(t sql.NullTime) string {
	if !t.Valid || t.Time.Year() < 1900 {
		return ""
	}
	return t.Time.Format(time.DateOnly)
}
