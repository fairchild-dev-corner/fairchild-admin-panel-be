package interfaces

import (
	"context"

	models "fairchild-admin-panel-be/internal/models/audit"
)

type AuditRepositoryInterface interface {
	ListRecentAuditLogs(ctx context.Context, limit int) ([]models.AuditLogEntry, error)
}
