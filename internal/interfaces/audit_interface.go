package interfaces

import (
	"context"

	models "fairchild-admin-panel-be/internal/models/audit"
)

type AuditRepositoryInterface interface {
	ListRecentAuditLogs(ctx context.Context, limit int) ([]models.AuditLogEntry, error)
	// One page of entries matching q, newest first, and how many match.
	ListAuditLogs(ctx context.Context, q models.AuditLogQuery) ([]models.AuditLogEntry, int, error)
}
