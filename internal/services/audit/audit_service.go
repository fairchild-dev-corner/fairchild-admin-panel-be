package services

import (
	"context"
	"slices"
	"strings"

	cc "fairchild-admin-panel-be/internal/constants"
	interfaces "fairchild-admin-panel-be/internal/interfaces"
	models "fairchild-admin-panel-be/internal/models/audit"
)

// auditLogPageSize caps GET /admin/audit-log - the panel renders the list
// without pagination.
const auditLogPageSize = 200

type AuditService struct {
	repo interfaces.AuditRepositoryInterface
}

func NewAuditService(repo interfaces.AuditRepositoryInterface) *AuditService {
	return &AuditService{repo: repo}
}

func (s *AuditService) ListAuditLog(ctx context.Context) ([]models.AuditLogEntry, error) {
	return s.repo.ListRecentAuditLogs(ctx, auditLogPageSize)
}

// ListElectionAuditLog pages the actions taken on the Election pages (who
// turned the election on or off, uploaded or reset the master list, switched
// a member's voting status, marked or reset token claims). An action that
// isn't an election action matches nothing.
func (s *AuditService) ListElectionAuditLog(ctx context.Context, action, search string, page, pageSize int) (*models.ElectionAuditPage, error) {
	q := models.AuditLogQuery{
		Actions: cc.ElectionAuditActions, Action: action, Search: strings.TrimSpace(search), Page: page, PageSize: pageSize,
	}
	if action != "" && !slices.Contains(cc.ElectionAuditActions, action) {
		q.Actions = nil
	}
	items, total, err := s.repo.ListAuditLogs(ctx, q)
	if err != nil {
		return nil, err
	}
	result := &models.ElectionAuditPage{
		Items: items, Total: total, Page: page, PageSize: pageSize, Actions: cc.ElectionAuditActions,
	}
	// Past the last page (e.g. the log shrank): show the last one instead.
	if lastPage := max(1, (total+pageSize-1)/pageSize); page > lastPage {
		q.Page, result.Page = lastPage, lastPage
		if result.Items, _, err = s.repo.ListAuditLogs(ctx, q); err != nil {
			return nil, err
		}
	}
	return result, nil
}
