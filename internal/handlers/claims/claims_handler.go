package handlers

import (
	"errors"
	"net/http"
	"slices"
	"strconv"

	cc "fairchild-admin-panel-be/internal/constants"
	middlewares "fairchild-admin-panel-be/internal/middlewares"
	models "fairchild-admin-panel-be/internal/models/claims"
	service "fairchild-admin-panel-be/internal/services/claims"
	"fairchild-admin-panel-be/internal/utils"
	loggers "fairchild-admin-panel-be/internal/utils/loggers"

	"github.com/gin-gonic/gin"
)

const (
	defaultPageSize = 15
	maxPageSize     = 100
)

type ClaimsHandler struct {
	claimsService *service.ClaimsService
}

func NewHandler(claimsService *service.ClaimsService) *ClaimsHandler {
	return &ClaimsHandler{claimsService: claimsService}
}

// RegisterRoutes mounts onto the already-guarded /admin group. Admin and
// staff can view claims; approving (which changes a portal account's
// email/mobile) or rejecting one is admin-only, like the Portal page's
// contact update.
func (h *ClaimsHandler) RegisterRoutes(admin *gin.RouterGroup) {
	admin.GET("/account-claims", h.handleList)
	admin.GET("/account-claims/:id", h.handleGet)
	admin.PUT("/account-claims/:id/approve", middlewares.RequireRole(cc.ROLE_ADMIN), h.handleApprove)
	admin.PUT("/account-claims/:id/reject", middlewares.RequireRole(cc.ROLE_ADMIN), h.handleReject)
}

// handleList: GET /admin/account-claims?status=&q=&month=&year=&page=&pageSize=
// month (YYYY-MM) or year (YYYY), in Asia/Manila, limits the submission
// date; without either, every date.
func (h *ClaimsHandler) handleList(ctx *gin.Context) {
	q := models.ClaimListQuery{
		Status:   ctx.DefaultQuery("status", cc.APPLICATION_STATUS_PENDING),
		Search:   ctx.Query("q"),
		Page:     max(1, atoiOr(ctx.Query("page"), 1)),
		PageSize: min(max(1, atoiOr(ctx.Query("pageSize"), defaultPageSize)), maxPageSize),
	}
	from, to, ok := utils.ParsePeriod(ctx.Query("month"), ctx.Query("year"))
	if !ok || !slices.Contains(models.Statuses, q.Status) {
		loggers.GetCommonError(ctx, cc.INVALID_CLAIM_FILTER, http.StatusBadRequest)
		return
	}
	q.From, q.To = from, to

	page, err := h.claimsService.List(ctx, q)
	if err != nil {
		writeClaimError(ctx, err)
		return
	}
	loggers.SuccessResponse(ctx, page)
}

// handleGet: GET /admin/account-claims/:id
func (h *ClaimsHandler) handleGet(ctx *gin.Context) {
	c, err := h.claimsService.Get(ctx, ctx.Param("id"))
	if err != nil {
		writeClaimError(ctx, err)
		return
	}
	loggers.SuccessResponse(ctx, c)
}

// handleApprove: PUT /admin/account-claims/:id/approve
func (h *ClaimsHandler) handleApprove(ctx *gin.Context) {
	actor, ok := middlewares.ActorFromContext(ctx)
	if !ok {
		return
	}

	c, err := h.claimsService.Approve(ctx, ctx.Param("id"), actor)
	if err != nil {
		writeClaimError(ctx, err)
		return
	}
	loggers.SuccessResponse(ctx, c)
}

// handleReject: PUT /admin/account-claims/:id/reject {reason}
func (h *ClaimsHandler) handleReject(ctx *gin.Context) {
	actor, ok := middlewares.ActorFromContext(ctx)
	if !ok {
		return
	}

	var req models.RejectClaimRequest
	if !utils.ValidatePayload(ctx, &req) {
		return
	}

	c, err := h.claimsService.Reject(ctx, ctx.Param("id"), &req, actor)
	if err != nil {
		writeClaimError(ctx, err)
		return
	}
	loggers.SuccessResponse(ctx, c)
}

func writeClaimError(ctx *gin.Context, err error) {
	switch {
	case errors.Is(err, cc.ErrClaimNotFound):
		loggers.GetCommonError(ctx, err.Error(), http.StatusNotFound)
	case errors.Is(err, cc.ErrClaimAlreadyReviewed),
		errors.Is(err, cc.ErrClaimNoPortalAccount),
		errors.Is(err, cc.ErrPortalEmailTaken):
		loggers.GetCommonError(ctx, err.Error(), http.StatusConflict)
	case errors.Is(err, cc.ErrClaimInvalidContact):
		loggers.GetCommonError(ctx, err.Error(), http.StatusBadRequest)
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
