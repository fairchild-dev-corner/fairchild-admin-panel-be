package repository

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"strings"

	models "fairchild-admin-panel-be/internal/models/members"
)

// Statements of Account and client standing for one ledger member, ported
// from fairchild_be (repositories/soa, repositories/health) so the panel
// shows what the member portal shows. wkfcsoa is the legacy system's cache of
// pre-generated statements: one row per line (SEQNO) of one statement
// (CTRLNO, the "SOA No." printed on it). Grouped by CTRLNO, never by
// STATEMENTDATE, which isn't unique per client.

// soaWindowQuery lists the statements dated within the last
// SOAWindowMonths months, never ones dated after today (the ledger carries
// forward-projected statements). Args: clientID, branch code, months.
const soaWindowQuery = `
SELECT CTRLNO, MIN(STATEMENTDATE), MIN(CUTOFFDATE),
       SUM(COALESCE(PRINDUE,0) + COALESCE(INTDUE,0) + COALESCE(PENDUE,0) + COALESCE(INSDUE,0))
FROM wkfcsoa
WHERE CLIENTIDx = ? AND BR_CODE = ?
  AND STATEMENTDATE < CURDATE() + INTERVAL 1 DAY
  AND STATEMENTDATE >= CURDATE() - INTERVAL ? MONTH
GROUP BY CTRLNO
ORDER BY MIN(STATEMENTDATE) DESC, CTRLNO DESC`

// soaFallbackQuery is the member's single latest statement dated up to
// today, for when the window is empty. Args: clientID, branch code, twice.
const soaFallbackQuery = `
SELECT CTRLNO, MIN(STATEMENTDATE), MIN(CUTOFFDATE),
       SUM(COALESCE(PRINDUE,0) + COALESCE(INTDUE,0) + COALESCE(PENDUE,0) + COALESCE(INSDUE,0))
FROM wkfcsoa
WHERE CLIENTIDx = ? AND BR_CODE = ?
  AND CTRLNO = (
      SELECT CTRLNO FROM wkfcsoa
       WHERE CLIENTIDx = ? AND BR_CODE = ? AND STATEMENTDATE < CURDATE() + INTERVAL 1 DAY
       ORDER BY STATEMENTDATE DESC, CTRLNO DESC
       LIMIT 1
  )
GROUP BY CTRLNO`

// soaDetailQuery is every line of one statement, scoped to the member: a
// CTRLNO of someone else's yields no rows. TRIM(ACCOUNTGROUP) because the
// ledger has " DEPOSITS". MATURITYDATE and MEMBERSHIPDATE are free text.
const soaDetailQuery = `
SELECT SEQNO, TRIM(ACCOUNTGROUP), SLDESCR, REF_NO,
       PRINAMT, PRINBAL, AVBLBAL,
       COALESCE(PRINDUE,0), COALESCE(INTDUE,0), COALESCE(PENDUE,0), COALESCE(INSDUE,0),
       ACCOUNTSTATUS, MATURITYDATE, TERM,
       CLIENTID, CLIENTNAME, HOMEADDRESS, STATEMENTDATE, CUTOFFDATE,
       BRANCHMANAGER, EMPLOYEEID, MEMBERSHIPDATE, ORGACRONYM
FROM wkfcsoa
WHERE CLIENTIDx = ? AND BR_CODE = ? AND CTRLNO = ?
ORDER BY SEQNO`

// ListStatements returns the member's statements from the last
// SOAWindowMonths months, newest first, or their latest older one (Fallback).
func (r *LedgerMembersRepository) ListStatements(ctx context.Context, branch string, clientID int64) (*models.SOAList, error) {
	db, err := r.db(branch)
	if err != nil {
		return nil, err
	}
	code := ledgerBranchCodes[branch]

	list := &models.SOAList{WindowMonths: models.SOAWindowMonths}
	list.Items, err = scanStatements(db.QueryContext(ctx, soaWindowQuery, clientID, code, models.SOAWindowMonths))
	if err != nil {
		return nil, fmt.Errorf("list %s statements: %w", branch, err)
	}
	if len(list.Items) > 0 {
		return list, nil
	}

	list.Items, err = scanStatements(db.QueryContext(ctx, soaFallbackQuery, clientID, code, clientID, code))
	if err != nil {
		return nil, fmt.Errorf("find %s latest statement: %w", branch, err)
	}
	list.Fallback = len(list.Items) > 0
	return list, nil
}

func scanStatements(rows *sql.Rows, err error) ([]models.SOASummary, error) {
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]models.SOASummary, 0)
	for rows.Next() {
		var s models.SOASummary
		var statementDate, cutoffDate sql.NullTime
		var totalDue sql.NullFloat64
		if err := rows.Scan(&s.CtrlNo, &statementDate, &cutoffDate, &totalDue); err != nil {
			return nil, err
		}
		s.StatementDate = ledgerDate(statementDate)
		s.CutoffDate = ledgerDate(cutoffDate)
		s.TotalDue = totalDue.Float64
		items = append(items, s)
	}
	return items, rows.Err()
}

// FindStatement returns one of the member's statements with its lines, or
// nil, nil when they have no statement with that CTRLNO.
func (r *LedgerMembersRepository) FindStatement(ctx context.Context, branch string, clientID, ctrlNo int64) (*models.SOAStatement, error) {
	db, err := r.db(branch)
	if err != nil {
		return nil, err
	}

	rows, err := db.QueryContext(ctx, soaDetailQuery, clientID, ledgerBranchCodes[branch], ctrlNo)
	if err != nil {
		return nil, fmt.Errorf("find %s statement: %w", branch, err)
	}
	defer rows.Close()

	var stmt *models.SOAStatement
	for rows.Next() {
		var line models.SOALine
		var accountGroup, descr, refNo, accountStatus, maturity, term sql.NullString
		var lineClientID, clientName, address, manager, idNo, membershipDate, org sql.NullString
		var prinAmt, prinBal, avblBal sql.NullFloat64
		var statementDate, cutoffDate sql.NullTime

		if err := rows.Scan(
			&line.SeqNo, &accountGroup, &descr, &refNo,
			&prinAmt, &prinBal, &avblBal,
			&line.PrincipalDue, &line.InterestDue, &line.PenaltyDue, &line.InsuranceDue,
			&accountStatus, &maturity, &term,
			&lineClientID, &clientName, &address, &statementDate, &cutoffDate,
			&manager, &idNo, &membershipDate, &org,
		); err != nil {
			return nil, err
		}
		line.AccountGroup = accountGroup.String
		line.Description = strings.TrimSpace(descr.String)
		line.RefNo = strings.TrimSpace(refNo.String)
		line.PrincipalAmount = nullFloat(prinAmt)
		line.PrincipalBalance = nullFloat(prinBal)
		line.AvailableBalance = nullFloat(avblBal)
		line.AccountStatus = strings.TrimSpace(accountStatus.String)
		line.MaturityDate = strings.TrimSpace(maturity.String)
		line.Term = strings.TrimSpace(term.String)

		if stmt == nil {
			stmt = &models.SOAStatement{
				CtrlNo:         ctrlNo,
				StatementDate:  ledgerDate(statementDate),
				CutoffDate:     ledgerDate(cutoffDate),
				ClientID:       strings.TrimSpace(lineClientID.String),
				ClientName:     strings.TrimSpace(clientName.String),
				HomeAddress:    strings.TrimSpace(address.String),
				IDNo:           strings.TrimSpace(idNo.String),
				MembershipDate: strings.TrimSpace(membershipDate.String),
				BranchManager:  strings.TrimSpace(manager.String),
				OrgAcronym:     strings.TrimSpace(org.String),
				Lines:          make([]models.SOALine, 0),
			}
		}
		stmt.TotalDue += line.PrincipalDue + line.InterestDue + line.PenaltyDue + line.InsuranceDue
		stmt.Lines = append(stmt.Lines, line)
	}
	if stmt != nil {
		stmt.TotalDue = math.Round(stmt.TotalDue*100) / 100
	}
	return stmt, rows.Err()
}

func nullFloat(f sql.NullFloat64) *float64 {
	if !f.Valid {
		return nil
	}
	return &f.Float64
}

// standingQuery is fairchild_be's coop health score over the trailing 12
// calendar months:
//   - share capital build-up / savings consistency: the % of those months
//     with a net-positive posting to SLC_CODE 31 / 22;
//   - loan repayment: the % of paid loan amounts on loans not written off
//     (LoanSTATUS 12/13), 100 when there are no loans;
//   - score: the average of the three.
//
// Args: branch code and clientID, three times.
const standingQuery = `
WITH RECURSIVE months AS (
  SELECT DATE_FORMAT(CURDATE(), '%Y-%m-01') AS month_start, 0 AS n
  UNION ALL
  SELECT DATE_SUB(month_start, INTERVAL 1 MONTH), n + 1
  FROM months WHERE n < 11
),
savings_by_month AS (
  SELECT DATE_FORMAT(s.TR_DATE, '%Y-%m-01') AS month_start, SUM(s.AMT * -1) AS net_amt
  FROM sldtl s
  WHERE s.SL_BRCODE = ? AND s.SL_CLIENTID = ? AND s.SLC_CODE = 22
    AND s.TR_DATE >= (SELECT MIN(month_start) FROM months)
  GROUP BY DATE_FORMAT(s.TR_DATE, '%Y-%m-01')
),
sharecap_by_month AS (
  SELECT DATE_FORMAT(s.TR_DATE, '%Y-%m-01') AS month_start, SUM(s.AMT * -1) AS net_amt
  FROM sldtl s
  WHERE s.SL_BRCODE = ? AND s.SL_CLIENTID = ? AND s.SLC_CODE = 31
    AND s.TR_DATE >= (SELECT MIN(month_start) FROM months)
  GROUP BY DATE_FORMAT(s.TR_DATE, '%Y-%m-01')
),
savings_score AS (
  SELECT ROUND(100.0 * SUM(CASE WHEN sb.net_amt > 0 THEN 1 ELSE 0 END) / COUNT(*), 0) AS pct
  FROM months m LEFT JOIN savings_by_month sb ON sb.month_start = m.month_start
),
sharecap_score AS (
  SELECT ROUND(100.0 * SUM(CASE WHEN sc.net_amt > 0 THEN 1 ELSE 0 END) / COUNT(*), 0) AS pct
  FROM months m LEFT JOIN sharecap_by_month sc ON sc.month_start = m.month_start
),
loan_repayment AS (
  SELECT COALESCE(ROUND(100.0 * SUM(CASE WHEN l.LoanSTATUS NOT IN (12,13) THEN l.LoanPAMT ELSE 0 END)
                        / NULLIF(SUM(l.LoanPAMT), 0), 0), 100) AS pct
  FROM loan l
  WHERE l.LoanBR_CODE = ? AND l.ClientIDLoan = ?
    AND l.LoanTR_DATE <= CURDATE()
    AND (l.LoanSTATUS <> 17 OR l.LoanMAT_DATE >= (SELECT MIN(month_start) FROM months))
)
SELECT sc.pct, sv.pct, lr.pct, ROUND((sc.pct + sv.pct + lr.pct) / 3, 0)
FROM sharecap_score sc, savings_score sv, loan_repayment lr`

// FindStanding returns the member's client standing (coop health score).
func (r *LedgerMembersRepository) FindStanding(ctx context.Context, branch string, clientID int64) (*models.ClientStanding, error) {
	db, err := r.db(branch)
	if err != nil {
		return nil, err
	}
	code := ledgerBranchCodes[branch]

	var s models.ClientStanding
	if err := db.QueryRowContext(ctx, standingQuery, code, clientID, code, clientID, code, clientID).Scan(
		&s.ShareCapitalBuildupPct, &s.SavingsConsistencyPct, &s.LoanRepaymentPct, &s.Score,
	); err != nil {
		return nil, fmt.Errorf("find %s client standing: %w", branch, err)
	}
	s.Tier = models.StandingTier(s.Score)
	return &s, nil
}
