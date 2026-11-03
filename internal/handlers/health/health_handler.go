package handlers

import (
	service "fairchild-admin-panel-be/internal/services/health"
	loggers "fairchild-admin-panel-be/internal/utils/loggers"

	"github.com/gin-gonic/gin"
)

type HealthHandler struct {
	healthService *service.HealthService
}

func NewHandler(healthService *service.HealthService) *HealthHandler {
	return &HealthHandler{healthService: healthService}
}

// RegisterRoutes mounts onto the already-guarded /admin group.
func (h *HealthHandler) RegisterRoutes(admin *gin.RouterGroup) {
	admin.GET("/health", h.handleGetHealth)
}

// RegisterPublicRoutes mounts an unauthenticated liveness probe for the
// hosting platform. It reveals nothing beyond "the process is up".
func (h *HealthHandler) RegisterPublicRoutes(router *gin.RouterGroup) {
	router.GET("/healthz", func(ctx *gin.Context) { loggers.SuccessResponse(ctx, "ok") })
}

// handleGetHealth: GET /admin/health. Individual failures are reported as
// "down" entries, so this always answers 200.
func (h *HealthHandler) handleGetHealth(ctx *gin.Context) {
	loggers.SuccessResponse(ctx, h.healthService.CheckAll(ctx))
}
