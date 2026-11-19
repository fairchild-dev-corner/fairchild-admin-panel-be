package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	cc "fairchild-admin-panel-be/internal/constants"
	models "fairchild-admin-panel-be/internal/models/locks"
	"fairchild-admin-panel-be/internal/utils"
)

// MaxExportRows caps one export. Locks only come from failed sign-ins, so this
// is far above what exists; it only stops a runaway query.
const MaxExportRows = 10000

var (
	exportHeaders = []string{
		"Lock ID", "Member ID", "Account name", "Email", "Mobile", "Account status", "Failed attempts",
		"IP address", "Reason", "Status", "Locked", "Expires", "Unlocked", "Unlocked how", "Unlocked by",
		"Lock email sent",
	}
	exportWidths = []float64{12, 12, 26, 30, 16, 14, 10, 16, 36, 10, 18, 18, 18, 16, 22, 18}
)

var unlockedHow = map[string]string{
	models.UnlockedViaPanel:   "Admin unlocked",
	models.UnlockedViaExpired: "Expired on its own",
	models.UnlockedViaOther:   "Other",
}

// Export is every lock matching q (its status, search and date range; page
// and page size are ignored) as an .xlsx workbook, newest first. Like the
// list, a query with no date range covers the last DefaultWindowMonths months.
// It returns the file's bytes and a suggested file name.
func (s *LocksService) Export(ctx context.Context, q models.LockedAccountListQuery) ([]byte, string, error) {
	q.Search = strings.TrimSpace(q.Search)
	q.Page, q.PageSize = 1, MaxExportRows
	locks, err := s.repo.List(ctx, q)
	if err != nil {
		return nil, "", err
	}

	rows := make([][]any, 0, len(locks))
	for _, l := range locks {
		var name, email, mobile, accountStatus string
		if a := l.Account; a != nil {
			name = strings.Join(strings.Fields(strings.Join([]string{a.FirstName, a.MiddleName, a.LastName, a.Suffix}, " ")), " ")
			email, mobile, accountStatus = a.Email, a.MobileNumber, a.Status
		}
		status := "Locked"
		if l.Status != models.Statuses[0] {
			status = "Unlocked"
		}
		rows = append(rows, []any{
			l.ID, l.MemberNo, name, email, mobile, accountStatus, l.AttemptCount,
			l.IPAddress, l.Reason, status, utils.ManilaDateTime(l.LockedAt), utils.ManilaDateTime(l.ExpiresAt),
			utils.ManilaDateTime(l.UnlockedAt), unlockedHow[l.UnlockedVia], l.UnlockedBy,
			utils.ManilaDateTime(l.NotifiedAt),
		})
	}
	file, err := utils.BuildXLSX("Locked accounts", exportHeaders, exportWidths, rows)
	if err != nil {
		return nil, "", err
	}
	return file, exportFileName(q.Status), nil
}

// exportFileName: "locked-accounts-unlocked-2026-11-19.xlsx".
func exportFileName(status string) string {
	if status == "" {
		status = cc.FILTER_ALL
	}
	return fmt.Sprintf("locked-accounts-%s-%s.xlsx", status, time.Now().In(utils.ManilaTZ).Format("2006-01-02"))
}
