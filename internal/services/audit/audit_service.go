package services

import (
	"context"

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
