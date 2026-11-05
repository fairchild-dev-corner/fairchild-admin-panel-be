package handlers

import (
	"errors"
	"net/http"
	"slices"
	"strconv"

	cc "fairchild-admin-panel-be/internal/constants"
	middlewares "fairchild-admin-panel-be/internal/middlewares"
	models "fairchild-admin-panel-be/internal/models/locks"
	service "fairchild-admin-panel-be/internal/services/locks"
	"fairchild-admin-panel-be/internal/utils"
	loggers "fairchild-admin-panel-be/internal/utils/loggers"

	"github.com/gin-gonic/gin"
)

const (
	defaultPageSize = 15
	maxPageSize     = 100
)

type LocksHandler struct {
	locksService *service.LocksService
}

func NewHandler(locksService *service.LocksService) *LocksHandler {
	return &LocksHandler{locksService: locksService}
}

// RegisterRoutes mounts onto the already-guarded /admin group. Admin and
// staff can view locked accounts; unlocking one is admin-only, like the
// other changes to a portal account.
func (h *LocksHandler) RegisterRoutes(admin *gin.RouterGroup) {
	admin.GET("/locked-accounts", h.handleList)
	admin.PUT("/locked-accounts/:id/unlock", middlewares.RequireRole(cc.ROLE_ADMIN), h.handleUnlock)
}

// handleList: GET /admin/locked-accounts?status=&q=&month=&year=&page=&pageSize=
// month is YYYY-MM and year YYYY (Asia/Manila); without either, the last
// three months.
func (h *LocksHandler) handleList(ctx *gin.Context) {
	q := models.LockedAccountListQuery{
		Status:   ctx.DefaultQuery("status", models.Statuses[0]),
		Search:   ctx.Query("q"),
		Page:     max(1, atoiOr(ctx.Query("page"), 1)),
		PageSize: min(max(1, atoiOr(ctx.Query("pageSize"), defaultPageSize)), maxPageSize),
	}
	from, to, ok := utils.ParsePeriod(ctx.Query("month"), ctx.Query("year"))
	if !ok || !slices.Contains(models.Statuses, q.Status) {
		loggers.GetCommonError(ctx, cc.INVALID_LOCK_FILTER, http.StatusBadRequest)
		return
	}
	q.From, q.To = from, to

	page, err := h.locksService.List(ctx, q)
	if err != nil {
		writeLockError(ctx, err)
		return
	}
	loggers.SuccessResponse(ctx, page)
}

// handleUnlock: PUT /admin/locked-accounts/:id/unlock
func (h *LocksHandler) handleUnlock(ctx *gin.Context) {
	actor, ok := middlewares.ActorFromContext(ctx)
	if !ok {
		return
	}

	result, err := h.locksService.Unlock(ctx, ctx.Param("id"), actor)
	if err != nil {
		writeLockError(ctx, err)
		return
	}
	loggers.SuccessResponse(ctx, result)
}

func writeLockError(ctx *gin.Context, err error) {
	switch {
	case errors.Is(err, cc.ErrLockedAccountNotFound):
		loggers.GetCommonError(ctx, err.Error(), http.StatusNotFound)
	default:
		loggers.GetCommonError(ctx, err.Error(), http.StatusInternalServerError)
	}
}

func atoiOr(s string, fallback int) int {
	if n, err := strconv.Atoi(s); err == nil {
		return n
	}
	return fallback
}
