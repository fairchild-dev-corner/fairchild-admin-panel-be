package handlers

import (
	"errors"
	caps "fairchild-admin-panel-be/internal/models/capabilities"
	"net/http"
	"slices"
	"strconv"
	"strings"

	cc "fairchild-admin-panel-be/internal/constants"
	middlewares "fairchild-admin-panel-be/internal/middlewares"
	member_models "fairchild-admin-panel-be/internal/models/members"
	models "fairchild-admin-panel-be/internal/models/portal"
	service "fairchild-admin-panel-be/internal/services/portal"
	"fairchild-admin-panel-be/internal/utils"
	loggers "fairchild-admin-panel-be/internal/utils/loggers"

	"github.com/gin-gonic/gin"
)

const (
	defaultPageSize = 15
	maxPageSize     = 100
)

type PortalHandler struct {
	portalService *service.PortalService
}

func NewHandler(portalService *service.PortalService) *PortalHandler {
	return &PortalHandler{portalService: portalService}
}

// RegisterRoutes mounts onto the already-guarded /admin group. Adding an
// account for a ledger member, changing an account's email/mobile or status,
// or removing it is one capability (admins, and staff when allowed on the
// Capabilities page).
func (h *PortalHandler) RegisterRoutes(admin *gin.RouterGroup) {
	need := middlewares.RequireCapability
	admin.GET("/portal-members", need(caps.PortalView), h.handleList)
	admin.GET("/portal-lookup", need(caps.PortalManage), h.handleLookup)
	admin.POST("/portal-members", need(caps.PortalManage), h.handleAdd)
	admin.GET("/portal-members/:id", need(caps.PortalView), h.handleGet)
	admin.PUT("/portal-members/:id", need(caps.PortalManage), h.handleUpdateContact)
	admin.PUT("/portal-members/:id/status", need(caps.PortalManage), h.handleUpdateStatus)
	admin.DELETE("/portal-members/:id", need(caps.PortalManage), h.handleRemove)
}

// handleList: GET /admin/portal-members?status=&branches=&q=&page=&pageSize=
func (h *PortalHandler) handleList(ctx *gin.Context) {
	q := models.PortalListQuery{
		Status:   ctx.DefaultQuery("status", "active"),
		Search:   ctx.Query("q"),
		Page:     max(1, atoiOr(ctx.Query("page"), 1)),
		PageSize: min(max(1, atoiOr(ctx.Query("pageSize"), defaultPageSize)), maxPageSize),
	}
	if !utils.IsTabFilter(models.Statuses, q.Status) {
		loggers.GetCommonError(ctx, cc.INVALID_PORTAL_FILTER, http.StatusBadRequest)
		return
	}
	for _, b := range strings.Split(ctx.Query("branches"), ",") {
		if b = strings.TrimSpace(b); b == "" || slices.Contains(q.Branches, b) {
			continue
		}
		if !slices.Contains(member_models.Branches, b) {
			loggers.GetCommonError(ctx, cc.INVALID_PORTAL_FILTER, http.StatusBadRequest)
			return
		}
		q.Branches = append(q.Branches, b)
	}

	page, err := h.portalService.List(ctx, q)
	if err != nil {
		writePortalError(ctx, err)
		return
	}
	loggers.SuccessResponse(ctx, page)
}

// handleGet: GET /admin/portal-members/:id
func (h *PortalHandler) handleGet(ctx *gin.Context) {
	m, err := h.portalService.Get(ctx, ctx.Param("id"))
	if err != nil {
		writePortalError(ctx, err)
		return
	}
	loggers.SuccessResponse(ctx, m)
}

// handleLookup: GET /admin/portal-lookup?memberNo= - ledger members with that
// member number, for the Add to portal dialog.
func (h *PortalHandler) handleLookup(ctx *gin.Context) {
	candidates, err := h.portalService.Lookup(ctx, ctx.Query("memberNo"))
	if err != nil {
		writePortalError(ctx, err)
		return
	}
	loggers.SuccessResponse(ctx, candidates)
}

// handleAdd: POST /admin/portal-members
func (h *PortalHandler) handleAdd(ctx *gin.Context) {
	actor, ok := middlewares.ActorFromContext(ctx)
	if !ok {
		return
	}

	var req models.AddPortalMemberRequest
	if !utils.ValidatePayload(ctx, &req) {
		return
	}

	m, err := h.portalService.Add(ctx, &req, actor)
	if err != nil {
		writePortalError(ctx, err)
		return
	}
	loggers.SuccessResponse(ctx, m)
}

// handleUpdateContact: PUT /admin/portal-members/:id {email, mobileNumber}
func (h *PortalHandler) handleUpdateContact(ctx *gin.Context) {
	actor, ok := middlewares.ActorFromContext(ctx)
	if !ok {
		return
	}

	var req models.ContactUpdateRequest
	if !utils.ValidatePayload(ctx, &req) {
		return
	}

	m, err := h.portalService.UpdateContact(ctx, ctx.Param("id"), &req, actor)
	if err != nil {
		writePortalError(ctx, err)
		return
	}
	loggers.SuccessResponse(ctx, m)
}

// handleUpdateStatus: PUT /admin/portal-members/:id/status {status}
func (h *PortalHandler) handleUpdateStatus(ctx *gin.Context) {
	actor, ok := middlewares.ActorFromContext(ctx)
	if !ok {
		return
	}

	var req models.StatusUpdateRequest
	if !utils.ValidatePayload(ctx, &req) {
		return
	}

	m, err := h.portalService.UpdateStatus(ctx, ctx.Param("id"), &req, actor)
	if err != nil {
		writePortalError(ctx, err)
		return
	}
	loggers.SuccessResponse(ctx, m)
}

// handleRemove: DELETE /admin/portal-members/:id
func (h *PortalHandler) handleRemove(ctx *gin.Context) {
	actor, ok := middlewares.ActorFromContext(ctx)
	if !ok {
		return
	}

	if err := h.portalService.Remove(ctx, ctx.Param("id"), actor); err != nil {
		writePortalError(ctx, err)
		return
	}
	loggers.SuccessResponse(ctx, nil)
}

func writePortalError(ctx *gin.Context, err error) {
	switch {
	case errors.Is(err, cc.ErrPortalMemberNotFound):
		loggers.GetCommonError(ctx, err.Error(), http.StatusNotFound)
	case errors.Is(err, cc.ErrPortalEmailTaken), errors.Is(err, cc.ErrPortalAccountExists):
		loggers.GetCommonError(ctx, err.Error(), http.StatusConflict)
	case errors.Is(err, cc.ErrInvalidMobileNumber), errors.Is(err, cc.ErrGuardianRequired), errors.Is(err, cc.ErrNotOnLedger):
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
