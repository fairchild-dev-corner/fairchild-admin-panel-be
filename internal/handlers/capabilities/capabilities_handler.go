package handlers

import (
	"errors"
	"net/http"

	cc "fairchild-admin-panel-be/internal/constants"
	middlewares "fairchild-admin-panel-be/internal/middlewares"
	models "fairchild-admin-panel-be/internal/models/capabilities"
	service "fairchild-admin-panel-be/internal/services/capabilities"
	"fairchild-admin-panel-be/internal/utils"
	loggers "fairchild-admin-panel-be/internal/utils/loggers"

	"github.com/gin-gonic/gin"
)

type CapabilitiesHandler struct {
	capabilitiesService *service.CapabilitiesService
}

func NewHandler(capabilitiesService *service.CapabilitiesService) *CapabilitiesHandler {
	return &CapabilitiesHandler{capabilitiesService: capabilitiesService}
}

// RegisterRoutes mounts onto the already-guarded /admin group. Every panel
// user reads their own capabilities; only super admins and admins see the
// catalog and set an account's.
func (h *CapabilitiesHandler) RegisterRoutes(admin *gin.RouterGroup) {
	adminOnly := middlewares.RequireAdmin()
	admin.GET("/capabilities/mine", h.handleMine)
	admin.GET("/capabilities", adminOnly, h.handleCatalog)
	admin.GET("/users/:id/capabilities", adminOnly, h.handleGetForUser)
	admin.PUT("/users/:id/capabilities", adminOnly, h.handleUpdateForUser)
}

// handleMine: GET /admin/capabilities/mine
func (h *CapabilitiesHandler) handleMine(ctx *gin.Context) {
	user, ok := middlewares.PanelUserFromContext(ctx)
	if !ok {
		loggers.GetCommonError(ctx, cc.MISSING_BEARER_TOKEN, http.StatusUnauthorized)
		return
	}
	mine, err := h.capabilitiesService.Mine(ctx, user)
	if err != nil {
		loggers.GetCommonError(ctx, err.Error(), http.StatusInternalServerError)
		return
	}
	loggers.SuccessResponse(ctx, mine)
}

// handleCatalog: GET /admin/capabilities
func (h *CapabilitiesHandler) handleCatalog(ctx *gin.Context) {
	loggers.SuccessResponse(ctx, h.capabilitiesService.Catalog())
}

// handleGetForUser: GET /admin/users/:id/capabilities
func (h *CapabilitiesHandler) handleGetForUser(ctx *gin.Context) {
	res, err := h.capabilitiesService.ForUser(ctx, ctx.Param("id"))
	if errors.Is(err, cc.ErrPanelUserNotFound) {
		loggers.GetCommonError(ctx, err.Error(), http.StatusNotFound)
		return
	}
	if err != nil {
		loggers.GetCommonError(ctx, err.Error(), http.StatusInternalServerError)
		return
	}
	loggers.SuccessResponse(ctx, res)
}

// handleUpdateForUser: PUT /admin/users/:id/capabilities {capabilities: [key]}
func (h *CapabilitiesHandler) handleUpdateForUser(ctx *gin.Context) {
	actor, ok := middlewares.ActorFromContext(ctx)
	if !ok {
		return
	}
	by, _ := middlewares.PanelUserFromContext(ctx)
	var req models.UpdateUserCapabilitiesRequest
	if !utils.ValidatePayload(ctx, &req) {
		return
	}
	res, err := h.capabilitiesService.UpdateForUser(ctx, by, actor, ctx.Param("id"), &req)
	switch {
	case err == nil:
		loggers.SuccessResponse(ctx, res)
	case errors.Is(err, cc.ErrPanelUserNotFound):
		loggers.GetCommonError(ctx, err.Error(), http.StatusNotFound)
	case errors.Is(err, cc.ErrCannotManagePanelUser):
		loggers.GetCommonError(ctx, err.Error(), http.StatusForbidden)
	case errors.Is(err, cc.ErrUnknownCapability), errors.Is(err, cc.ErrCapabilitiesNotEditable):
		loggers.GetCommonError(ctx, err.Error(), http.StatusBadRequest)
	default:
		loggers.GetCommonError(ctx, err.Error(), http.StatusInternalServerError)
	}
}
