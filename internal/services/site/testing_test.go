package services

import (
	"context"
	"encoding/json"
	"errors"
	cc "fairchild-admin-panel-be/internal/constants"
	"fairchild-admin-panel-be/internal/interfaces"
	audit "fairchild-admin-panel-be/internal/models/audit"
	models "fairchild-admin-panel-be/internal/models/site"
	"strings"
	"testing"
	"time"
)

type siteTestRepo struct {
	interfaces.SiteRepositoryInterface
	status  models.SiteStatus
	action  string
	message string
}

func (r *siteTestRepo) GetSiteStatus(context.Context) (*models.SiteStatus, error) {
	copy := r.status
	return &copy, nil
}
func (r *siteTestRepo) UpdateSiteStatus(_ context.Context, enabled bool, message string, start *time.Time, _ *int, _ int64, makeAudit func(*models.SiteStatus) *audit.AuditLog) (int64, error) {
	r.action = makeAudit(&r.status).Action
	r.message = message
	r.status.MaintenanceMode = enabled
	r.status.StartsAt = start
	return 2, nil
}
func TestSecurityPublicSiteStatusHidesStaff(t *testing.T) {
	s := &SiteService{repo: &siteTestRepo{status: models.SiteStatus{UpdatedBy: "PRIVATE-STAFF-NAME", MaintenanceMode: true, Message: "Maintenance"}}}
	got, err := s.GetPublicSiteStatus(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "PRIVATE") || strings.Contains(string(data), "updatedBy") || !got.MaintenanceMode {
		t.Fatalf("%s", data)
	}
}
func TestSiteScheduleValidationAndAudit(t *testing.T) {
	enabled := true
	r := &siteTestRepo{}
	s := &SiteService{repo: r}
	for _, start := range []time.Time{time.Now().Add(-time.Hour), time.Now().AddDate(0, 0, 31)} {
		if _, err := s.UpdateSiteStatus(context.Background(), &models.UpdateSiteStatusRequest{MaintenanceMode: &enabled, StartsAt: &start}, audit.Actor{}); !errors.Is(err, cc.ErrInvalidMaintenanceStart) {
			t.Fatal(err)
		}
	}
	got, err := s.UpdateSiteStatus(context.Background(), &models.UpdateSiteStatusRequest{MaintenanceMode: &enabled, Message: "  Maintenance  "}, audit.Actor{UserPanelID: 1})
	if err != nil || r.action != cc.AUDIT_ENABLED_MAINTENANCE_MODE || r.message != "Maintenance" || got.MembersSignedOut == nil || *got.MembersSignedOut != 2 {
		t.Fatalf("%+v %v", got, err)
	}
}
func BenchmarkSite(b *testing.B) {
	s := &SiteService{repo: &siteTestRepo{}}
	b.ReportAllocs()
	for b.Loop() {
		if _, err := s.GetPublicSiteStatus(context.Background()); err != nil {
			b.Fatal(err)
		}
	}
}
