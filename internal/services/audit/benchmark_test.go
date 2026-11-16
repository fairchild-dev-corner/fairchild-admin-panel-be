package services

import (
	"context"
	"fairchild-admin-panel-be/internal/interfaces"
	models "fairchild-admin-panel-be/internal/models/audit"
	"testing"
)

type benchmarkAuditRepo struct {
	interfaces.AuditRepositoryInterface
}

func (benchmarkAuditRepo) ListAuditLogs(context.Context, models.AuditLogQuery) ([]models.AuditLogEntry, int, error) {
	return []models.AuditLogEntry{}, 0, nil
}
func BenchmarkAudit(b *testing.B) {
	s := NewAuditService(benchmarkAuditRepo{})
	b.ReportAllocs()
	for b.Loop() {
		if _, err := s.ListElectionAuditLog(context.Background(), "", " query ", 1, 20); err != nil {
			b.Fatal(err)
		}
	}
}
