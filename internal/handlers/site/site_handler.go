package handlers

import (
	"errors"
	"net/http"

	cc "fairchild-admin-panel-be/internal/constants"
	middlewares "fairchild-admin-panel-be/internal/middlewares"
	models "fairchild-admin-panel-be/internal/models/site"
	"fairchild-admin-panel-be/internal/utils"
	loggers "fairchild-admin-panel-be/internal/utils/loggers"

	"github.com/gin-gonic/gin"
)

// handleGetPublicSiteStatus: GET /site/status (public)
func (h *SiteHandler) handleGetPublicSiteStatus(ctx *gin.Context) {
	status, err := h.siteService.GetPublicSiteStatus(ctx)
	if err != nil {
		loggers.GetCommonError(ctx, err.Error(), http.StatusInternalServerError)
		return
	}
	loggers.SuccessResponse(ctx, status)
}

// handleGetSiteStatus: GET /admin/site-status
func (h *SiteHandler) handleGetSiteStatus(ctx *gin.Context) {
	status, err := h.siteService.GetSiteStatus(ctx)
	if err != nil {
		loggers.GetCommonError(ctx, err.Error(), http.StatusInternalServerError)
		return
	}
	loggers.SuccessResponse(ctx, status)
}

// handleUpdateSiteStatus: PUT /admin/site-status (admin role only)
func (h *SiteHandler) handleUpdateSiteStatus(ctx *gin.Context) {
	actor, ok := middlewares.ActorFromContext(ctx)
	if !ok {
		return
	}

	var req models.UpdateSiteStatusRequest
	if !utils.ValidatePayload(ctx, &req) {
		return
	}

	status, err := h.siteService.UpdateSiteStatus(ctx, &req, actor)
	if err != nil {
		if errors.Is(err, cc.ErrInvalidMaintenanceStart) {
			loggers.GetCommonError(ctx, err.Error(), http.StatusBadRequest)
			return
		}
		loggers.GetCommonError(ctx, err.Error(), http.StatusInternalServerError)
		return
	}
	loggers.SuccessResponse(ctx, status)
}
