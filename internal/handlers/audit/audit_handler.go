package handlers

import (
	middlewares "fairchild-admin-panel-be/internal/middlewares"
	caps "fairchild-admin-panel-be/internal/models/capabilities"
	"net/http"

	service "fairchild-admin-panel-be/internal/services/audit"
	loggers "fairchild-admin-panel-be/internal/utils/loggers"

	"github.com/gin-gonic/gin"
)

type AuditHandler struct {
	auditService *service.AuditService
}

func NewHandler(auditService *service.AuditService) *AuditHandler {
	return &AuditHandler{auditService: auditService}
}

// RegisterRoutes mounts onto the already-guarded /admin group.
func (h *AuditHandler) RegisterRoutes(admin *gin.RouterGroup) {
	admin.GET("/audit-log", middlewares.RequireCapability(caps.AuditView), h.handleListAuditLog)
}

// handleListAuditLog: GET /admin/audit-log
func (h *AuditHandler) handleListAuditLog(ctx *gin.Context) {
	entries, err := h.auditService.ListAuditLog(ctx)
	if err != nil {
		loggers.GetCommonError(ctx, err.Error(), http.StatusInternalServerError)
		return
	}
	loggers.SuccessResponse(ctx, entries)
}
