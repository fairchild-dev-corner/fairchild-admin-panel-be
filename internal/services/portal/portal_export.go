package services

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	cc "fairchild-admin-panel-be/internal/constants"
	models "fairchild-admin-panel-be/internal/models/portal"
	portal_repo "fairchild-admin-panel-be/internal/repositories/portal"
	"fairchild-admin-panel-be/internal/utils"
)

const (
	// MaxExportRows caps one export (there are about 13,000 accounts today).
	MaxExportRows = 50000
	// exportChunk keeps each IN (...) lookup well under the placeholder limit.
	exportChunk = 1000
)

var (
	exportHeaders = []string{
		"Account ID", "Member ID", "Last name", "First name", "Middle name", "Suffix", "Email", "Email verified",
		"Mobile", "Branch", "Member type", "Category", "Status", "Registered", "Last login",
	}
	exportWidths = []float64{12, 14, 22, 22, 18, 8, 32, 14, 16, 12, 14, 14, 16, 18, 18}
)

// Export is every portal account matching q (its status, branches and search;
// page and page size are ignored) as an .xlsx workbook, newest first, with the
// branch each account's ledger record gives it. It returns the file's bytes
// and a suggested file name.
func (s *PortalService) Export(ctx context.Context, q models.PortalListQuery) ([]byte, string, error) {
	keys, err := s.repo.ListKeys(ctx, strings.TrimSpace(q.Search))
	if err != nil {
		return nil, "", err
	}
	index, err := s.ledgerIndex(ctx)
	if err != nil {
		return nil, "", err
	}

	var ids []int64
	for _, k := range keys {
		inTab := k.Status == q.Status || (q.Status == cc.FILTER_ALL && slices.Contains(models.Statuses, k.Status))
		if !inTab {
			continue
		}
		link, onLedger := resolveLedger(index, k.MemberNo, k.ClientID)
		if len(q.Branches) > 0 && !(onLedger && slices.Contains(q.Branches, link.Branch)) {
			continue
		}
		if ids = append(ids, k.ID); len(ids) == MaxExportRows {
			break
		}
	}

	byID := make(map[string]models.PortalMember, len(ids))
	for from := 0; from < len(ids); from += exportChunk {
		found, err := s.repo.FindByIDs(ctx, ids[from:min(len(ids), from+exportChunk)])
		if err != nil {
			return nil, "", err
		}
		for _, m := range found {
			byID[m.ID] = m
		}
	}

	rows := make([][]any, 0, len(ids))
	for _, id := range ids {
		m, ok := byID[portal_repo.FormatPortalID(id)]
		if !ok {
			continue // removed since the list was read
		}
		if link, onLedger := resolveLedger(index, m.MemberNo, m.ClientID); onLedger {
			attach(&m, link)
		}
		verified := "No"
		if m.EmailVerified {
			verified = "Yes"
		}
		status := statusLabels[m.Status]
		if status == "" {
			status = m.Status
		}
		rows = append(rows, []any{
			m.ID, m.MemberNo, m.LastName, m.FirstName, m.MiddleName, m.Suffix, m.Email, verified,
			m.MobileNumber, branchLabel(m.Branch), m.MemberType, m.Category, status,
			utils.ManilaDateTime(m.RegisteredAt), utils.ManilaDateTime(m.LastLoginAt),
		})
	}

	file, err := utils.BuildXLSX("Portal accounts", exportHeaders, exportWidths, rows)
	if err != nil {
		return nil, "", err
	}
	return file, exportFileName(q.Status), nil
}

// branchLabel: "knowles" -> "Knowles", "tiic" -> "TIIC"; "" (not on a ledger)
// stays empty.
func branchLabel(branch string) string {
	switch branch {
	case "":
		return ""
	case "tiic":
		return "TIIC"
	default:
		return strings.ToUpper(branch[:1]) + branch[1:]
	}
}

// exportFileName: "portal-accounts-pending_review-2026-11-19.xlsx".
func exportFileName(status string) string {
	if status == "" {
		status = cc.FILTER_ALL
	}
	return fmt.Sprintf("portal-accounts-%s-%s.xlsx", status, time.Now().In(utils.ManilaTZ).Format("2006-01-02"))
}
