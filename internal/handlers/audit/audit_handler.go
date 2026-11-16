package handlers

import (
	middlewares "fairchild-admin-panel-be/internal/middlewares"
	caps "fairchild-admin-panel-be/internal/models/capabilities"
	"net/http"
	"strconv"

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

const (
	defaultPageSize = 20
	maxPageSize     = 100
)

// RegisterRoutes mounts onto the already-guarded /admin group.
func (h *AuditHandler) RegisterRoutes(admin *gin.RouterGroup) {
	admin.GET("/audit-log", middlewares.RequireCapability(caps.AuditView), h.handleListAuditLog)
	admin.GET("/election/audit-log", middlewares.RequireCapability(caps.ElectionAuditView), h.handleListElectionAuditLog)
}

// handleListElectionAuditLog: GET /admin/election/audit-log?action=&q=&page=&pageSize=
// - the actions taken on the Election pages, newest first.
func (h *AuditHandler) handleListElectionAuditLog(ctx *gin.Context) {
	page := max(1, atoiOr(ctx.Query("page"), 1))
	pageSize := min(max(1, atoiOr(ctx.Query("pageSize"), defaultPageSize)), maxPageSize)
	result, err := h.auditService.ListElectionAuditLog(ctx, ctx.Query("action"), ctx.Query("q"), page, pageSize)
	if err != nil {
		loggers.GetCommonError(ctx, err.Error(), http.StatusInternalServerError)
		return
	}
	loggers.SuccessResponse(ctx, result)
}

func atoiOr(s string, fallback int) int {
	if n, err := strconv.Atoi(s); err == nil {
		return n
	}
	return fallback
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
