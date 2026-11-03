package services

import (
	"context"
	"strings"

	cc "fairchild-admin-panel-be/internal/constants"
	interfaces "fairchild-admin-panel-be/internal/interfaces"
	audit_models "fairchild-admin-panel-be/internal/models/audit"
	models "fairchild-admin-panel-be/internal/models/site"
)

type SiteService struct {
	repo interfaces.SiteRepositoryInterface
}

func NewSiteService(repo interfaces.SiteRepositoryInterface) *SiteService {
	return &SiteService{repo: repo}
}

func (s *SiteService) GetSiteStatus(ctx context.Context) (*models.SiteStatus, error) {
	return s.repo.GetSiteStatus(ctx)
}

// GetPublicSiteStatus is the unauthenticated view for fairchild-fe.
func (s *SiteService) GetPublicSiteStatus(ctx context.Context) (*models.PublicSiteStatus, error) {
	status, err := s.repo.GetSiteStatus(ctx)
	if err != nil {
		return nil, err
	}
	return &models.PublicSiteStatus{
		MaintenanceMode: status.MaintenanceMode,
		Message:         status.Message,
		UpdatedAt:       status.UpdatedAt,
	}, nil
}

// UpdateSiteStatus saves the new state with an audit entry worded like the
// panel's mock ("Enabled maintenance mode", "Updated maintenance message").
func (s *SiteService) UpdateSiteStatus(ctx context.Context, req *models.UpdateSiteStatusRequest, actor audit_models.Actor) (*models.SiteStatus, error) {
	enabled := *req.MaintenanceMode

	err := s.repo.UpdateSiteStatus(ctx, enabled, strings.TrimSpace(req.Message), actor.UserPanelID,
		func(previous *models.SiteStatus) *audit_models.AuditLog {
			action := cc.AUDIT_UPDATED_MAINTENANCE_MESSAGE
			if previous.MaintenanceMode != enabled {
				action = cc.AUDIT_DISABLED_MAINTENANCE_MODE
				if enabled {
					action = cc.AUDIT_ENABLED_MAINTENANCE_MODE
				}
			}
			return actor.Log(action, cc.AUDIT_TARGET_SITE)
		},
	)
	if err != nil {
		return nil, err
	}

	return s.repo.GetSiteStatus(ctx)
}
