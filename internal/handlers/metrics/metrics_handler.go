package handlers

import (
	middlewares "fairchild-admin-panel-be/internal/middlewares"
	caps "fairchild-admin-panel-be/internal/models/capabilities"
	"net/http"

	service "fairchild-admin-panel-be/internal/services/metrics"
	loggers "fairchild-admin-panel-be/internal/utils/loggers"

	"github.com/gin-gonic/gin"
)

type MetricsHandler struct {
	metricsService *service.MetricsService
}

func NewHandler(metricsService *service.MetricsService) *MetricsHandler {
	return &MetricsHandler{metricsService: metricsService}
}

// RegisterRoutes mounts onto the already-guarded /admin group.
func (h *MetricsHandler) RegisterRoutes(admin *gin.RouterGroup) {
	admin.GET("/metrics", middlewares.RequireCapability(caps.MetricsView), h.handleGetMetrics)
}

// handleGetMetrics: GET /admin/metrics
func (h *MetricsHandler) handleGetMetrics(ctx *gin.Context) {
	metrics, err := h.metricsService.GetMetrics(ctx)
	if err != nil {
		loggers.GetCommonError(ctx, err.Error(), http.StatusInternalServerError)
		return
	}
	loggers.SuccessResponse(ctx, metrics)
}
