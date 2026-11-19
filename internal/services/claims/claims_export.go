package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	cc "fairchild-admin-panel-be/internal/constants"
	models "fairchild-admin-panel-be/internal/models/claims"
	"fairchild-admin-panel-be/internal/utils"
)

// MaxExportRows caps one export. Claims are submitted by hand, so this is far
// above what exists; it only stops a runaway query.
const MaxExportRows = 10000

var (
	exportHeaders = []string{
		"Claim ID", "Member ID", "Name (as claimed)", "Account name", "Requested email", "Requested mobile",
		"Message", "Status", "Rejection reason", "Submitted", "Reviewed", "Reviewed by",
	}
	exportWidths = []float64{12, 12, 26, 26, 32, 18, 40, 11, 32, 18, 18, 22}
)

// Export is every claim matching q (its status, search and date range; page
// and page size are ignored) as an .xlsx workbook, most recently submitted
// first. It returns the file's bytes and a suggested file name.
func (s *ClaimsService) Export(ctx context.Context, q models.ClaimListQuery) ([]byte, string, error) {
	q.Search = strings.TrimSpace(q.Search)
	q.Page, q.PageSize = 1, MaxExportRows
	claims, err := s.repo.List(ctx, q)
	if err != nil {
		return nil, "", err
	}

	rows := make([][]any, 0, len(claims))
	for _, c := range claims {
		rows = append(rows, []any{
			c.ID, c.MemberNo, c.FullName, accountName(c.Account), c.ProposedEmail, c.ProposedMobileNumber,
			c.Message, titleCase(c.Status), c.RejectionReason, utils.ManilaDateTime(c.SubmittedAt),
			utils.ManilaDateTime(c.ReviewedAt), c.ReviewedBy,
		})
	}
	file, err := utils.BuildXLSX("Claims", exportHeaders, exportWidths, rows)
	if err != nil {
		return nil, "", err
	}
	return file, exportFileName(q.Status), nil
}

// exportFileName: "account-claims-approved-2026-11-19.xlsx".
func exportFileName(status string) string {
	if status == "" || status == cc.FILTER_ALL {
		status = "all"
	}
	return fmt.Sprintf("account-claims-%s-%s.xlsx", status, time.Now().In(utils.ManilaTZ).Format("2006-01-02"))
}

func accountName(a *models.ClaimAccount) string {
	if a == nil {
		return ""
	}
	return strings.Join(strings.Fields(strings.Join([]string{a.FirstName, a.MiddleName, a.LastName, a.Suffix}, " ")), " ")
}

func titleCase(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
