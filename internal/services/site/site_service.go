package services

import (
	"context"
	"log/slog"
	"strings"
	"time"

	cc "fairchild-admin-panel-be/internal/constants"
	interfaces "fairchild-admin-panel-be/internal/interfaces"
	audit_models "fairchild-admin-panel-be/internal/models/audit"
	models "fairchild-admin-panel-be/internal/models/site"
)

// scheduleCheckInterval is how often the scheduler looks for a scheduled
// start that has passed, i.e. how late after it members may be signed out.
// fairchild_be refuses them from the start time itself regardless.
const scheduleCheckInterval = 30 * time.Second

type SiteService struct {
	repo interfaces.SiteRepositoryInterface
}

// NewSiteService starts the scheduler that signs members out when scheduled
// maintenance begins.
func NewSiteService(repo interfaces.SiteRepositoryInterface) *SiteService {
	s := &SiteService{repo: repo}
	go s.watchSchedule()
	return s
}

func (s *SiteService) watchSchedule() {
	ticker := time.NewTicker(scheduleCheckInterval)
	defer ticker.Stop()
	for range ticker.C {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		signedOut, err := s.repo.SignOutIfDue(ctx)
		cancel()
		if err != nil {
			slog.Error("scheduled maintenance sign-out failed", "err", err)
		} else if signedOut > 0 {
			slog.Info("scheduled maintenance started, members signed out", "sessions", signedOut)
		}
	}
}

func (s *SiteService) GetSiteStatus(ctx context.Context) (*models.SiteStatus, error) {
	return s.repo.GetSiteStatus(ctx)
}

// GetPublicSiteStatus is the unauthenticated view - no staff names.
func (s *SiteService) GetPublicSiteStatus(ctx context.Context) (*models.PublicSiteStatus, error) {
	status, err := s.repo.GetSiteStatus(ctx)
	if err != nil {
		return nil, err
	}
	return &models.PublicSiteStatus{
		MaintenanceMode: status.MaintenanceMode,
		StartsAt:        status.StartsAt,
		Message:         status.Message,
		EndsAt:          status.EndsAt,
		UpdatedAt:       status.UpdatedAt,
	}, nil
}

// UpdateSiteStatus saves the new state with an audit entry worded like the
// panel's mock ("Enabled maintenance mode", "Scheduled maintenance mode",
// "Updated maintenance message"). Maintenance that starts now signs every
// member out (see the repository); scheduled maintenance does when it starts.
func (s *SiteService) UpdateSiteStatus(ctx context.Context, req *models.UpdateSiteStatusRequest, actor audit_models.Actor) (*models.SiteStatus, error) {
	enabled := *req.MaintenanceMode
	startsAt := req.StartsAt
	if !enabled {
		startsAt = nil
	}
	if startsAt != nil {
		if !startsAt.After(time.Now()) || startsAt.After(time.Now().AddDate(0, 0, models.MaxScheduleAheadDays)) {
			return nil, cc.ErrInvalidMaintenanceStart
		}
	}

	signedOut, err := s.repo.UpdateSiteStatus(ctx, enabled, strings.TrimSpace(req.Message), startsAt, req.DurationMinutes, actor.UserPanelID,
		func(previous *models.SiteStatus) *audit_models.AuditLog {
			wasOn := previous.MaintenanceMode || previous.Scheduled()
			action := cc.AUDIT_UPDATED_MAINTENANCE_MESSAGE
			switch {
			case enabled && startsAt != nil && (!previous.Scheduled() || !previous.StartsAt.Equal(*startsAt)):
				action = cc.AUDIT_SCHEDULED_MAINTENANCE_MODE
			case enabled && startsAt == nil && !previous.MaintenanceMode:
				action = cc.AUDIT_ENABLED_MAINTENANCE_MODE
			case !enabled && wasOn:
				action = cc.AUDIT_DISABLED_MAINTENANCE_MODE
			}
			return actor.Log(action, cc.AUDIT_TARGET_SITE)
		},
	)
	if err != nil {
		return nil, err
	}

	status, err := s.repo.GetSiteStatus(ctx)
	if err != nil {
		return nil, err
	}
	if signedOut > 0 {
		status.MembersSignedOut = &signedOut
	}
	return status, nil
}
