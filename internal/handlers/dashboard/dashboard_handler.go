package handlers

import (
	"net/http"

	service "fairchild-admin-panel-be/internal/services/dashboard"
	loggers "fairchild-admin-panel-be/internal/utils/loggers"

	"github.com/gin-gonic/gin"
)

type DashboardHandler struct {
	dashboardService *service.DashboardService
}

func NewHandler(dashboardService *service.DashboardService) *DashboardHandler {
	return &DashboardHandler{dashboardService: dashboardService}
}

// RegisterRoutes mounts onto the already-guarded /admin group.
func (h *DashboardHandler) RegisterRoutes(admin *gin.RouterGroup) {
	admin.GET("/dashboard", h.handleGetDashboard)
}

// handleGetDashboard: GET /admin/dashboard
func (h *DashboardHandler) handleGetDashboard(ctx *gin.Context) {
	summary, err := h.dashboardService.GetSummary(ctx)
	if err != nil {
		loggers.GetCommonError(ctx, err.Error(), http.StatusInternalServerError)
		return
	}
	loggers.SuccessResponse(ctx, summary)
}
