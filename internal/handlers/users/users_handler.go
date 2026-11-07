package handlers

import (
	"errors"
	"net/http"

	cc "fairchild-admin-panel-be/internal/constants"
	middlewares "fairchild-admin-panel-be/internal/middlewares"
	models "fairchild-admin-panel-be/internal/models/users"
	service "fairchild-admin-panel-be/internal/services/users"
	"fairchild-admin-panel-be/internal/utils"
	loggers "fairchild-admin-panel-be/internal/utils/loggers"

	"github.com/gin-gonic/gin"
)

type UsersHandler struct {
	usersService *service.UsersService
}

func NewHandler(usersService *service.UsersService) *UsersHandler {
	return &UsersHandler{usersService: usersService}
}

// RegisterRoutes mounts onto the already-guarded /admin group. Panel
// accounts are for super admins and admins only, and each can only add or
// change accounts below their own role.
func (h *UsersHandler) RegisterRoutes(admin *gin.RouterGroup) {
	adminOnly := middlewares.RequireAdmin()
	admin.GET("/users", adminOnly, h.handleList)
	admin.POST("/users", adminOnly, h.handleCreate)
	admin.PUT("/users/:id/role", adminOnly, h.handleUpdateRole)
	admin.PUT("/users/:id/status", adminOnly, h.handleUpdateStatus)
}

// writeError answers a users service error.
func writeError(ctx *gin.Context, err error) {
	switch {
	case errors.Is(err, cc.ErrPanelUserNotFound):
		loggers.GetCommonError(ctx, err.Error(), http.StatusNotFound)
	case errors.Is(err, cc.ErrCannotManagePanelUser), errors.Is(err, cc.ErrRoleNotAssignable):
		loggers.GetCommonError(ctx, err.Error(), http.StatusForbidden)
	case errors.Is(err, cc.ErrUsernameAlreadyExists), errors.Is(err, cc.ErrEmailAlreadyExists):
		loggers.GetCommonError(ctx, err.Error(), http.StatusConflict)
	case errors.Is(err, cc.ErrInvalidUsername), errors.Is(err, cc.ErrPasswordTooShort), errors.Is(err, cc.ErrInvalidPayload):
		loggers.GetCommonError(ctx, err.Error(), http.StatusBadRequest)
	default:
		loggers.GetCommonError(ctx, err.Error(), http.StatusInternalServerError)
	}
}

// handleList: GET /admin/users
func (h *UsersHandler) handleList(ctx *gin.Context) {
	users, err := h.usersService.List(ctx)
	if err != nil {
		loggers.GetCommonError(ctx, err.Error(), http.StatusInternalServerError)
		return
	}
	loggers.SuccessResponse(ctx, users)
}

// handleCreate: POST /admin/users {username, firstName, lastName, email, role, password}
func (h *UsersHandler) handleCreate(ctx *gin.Context) {
	actor, ok := middlewares.ActorFromContext(ctx)
	if !ok {
		return
	}
	by, _ := middlewares.PanelUserFromContext(ctx)
	var req models.CreatePanelUserRequest
	if !utils.ValidatePayload(ctx, &req) {
		return
	}
	user, err := h.usersService.Create(ctx, by, &req, actor)
	if err != nil {
		writeError(ctx, err)
		return
	}
	loggers.SuccessResponse(ctx, user)
}

// handleUpdateRole: PUT /admin/users/:id/role {role}
func (h *UsersHandler) handleUpdateRole(ctx *gin.Context) {
	actor, ok := middlewares.ActorFromContext(ctx)
	if !ok {
		return
	}
	by, _ := middlewares.PanelUserFromContext(ctx)
	var req models.UpdateRoleRequest
	if !utils.ValidatePayload(ctx, &req) {
		return
	}
	user, err := h.usersService.UpdateRole(ctx, by, ctx.Param("id"), &req, actor)
	if err != nil {
		writeError(ctx, err)
		return
	}
	loggers.SuccessResponse(ctx, user)
}

// handleUpdateStatus: PUT /admin/users/:id/status {status: active|suspended}
func (h *UsersHandler) handleUpdateStatus(ctx *gin.Context) {
	actor, ok := middlewares.ActorFromContext(ctx)
	if !ok {
		return
	}
	by, _ := middlewares.PanelUserFromContext(ctx)
	var req models.UpdateStatusRequest
	if !utils.ValidatePayload(ctx, &req) {
		return
	}
	user, err := h.usersService.UpdateStatus(ctx, by, ctx.Param("id"), &req, actor)
	if err != nil {
		writeError(ctx, err)
		return
	}
	loggers.SuccessResponse(ctx, user)
}
