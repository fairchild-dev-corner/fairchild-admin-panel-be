package handlers

import (
	"errors"
	"net/http"

	cc "fairchild-admin-panel-be/internal/constants"
	middlewares "fairchild-admin-panel-be/internal/middlewares"
	models "fairchild-admin-panel-be/internal/models/account"
	service "fairchild-admin-panel-be/internal/services/account"
	"fairchild-admin-panel-be/internal/utils"
	loggers "fairchild-admin-panel-be/internal/utils/loggers"

	"github.com/gin-gonic/gin"
)

type AccountHandler struct {
	accountService *service.AccountService
}

func NewHandler(accountService *service.AccountService) *AccountHandler {
	return &AccountHandler{accountService: accountService}
}

// RegisterRoutes mounts onto the already-guarded /admin group. Every panel
// user (admin or staff) can read and change their own account; the user
// always comes from the access token, never from the request.
func (h *AccountHandler) RegisterRoutes(admin *gin.RouterGroup) {
	admin.GET("/account", h.handleGet)
	admin.PUT("/account/profile", h.handleUpdateProfile)
	admin.PUT("/account/password", h.handleChangePassword)
}

// handleGet: GET /admin/account
func (h *AccountHandler) handleGet(ctx *gin.Context) {
	user, ok := middlewares.PanelUserFromContext(ctx)
	if !ok {
		loggers.GetCommonError(ctx, cc.MISSING_BEARER_TOKEN, http.StatusUnauthorized)
		return
	}

	account, err := h.accountService.Get(ctx, user.ID)
	if err != nil {
		writeAccountError(ctx, err)
		return
	}
	loggers.SuccessResponse(ctx, account)
}

// handleUpdateProfile: PUT /admin/account/profile {firstName, lastName, email, currentPassword}
func (h *AccountHandler) handleUpdateProfile(ctx *gin.Context) {
	actor, ok := middlewares.ActorFromContext(ctx)
	if !ok {
		return
	}
	var req models.UpdateProfileRequest
	if !utils.ValidatePayload(ctx, &req) {
		return
	}
	account, err := h.accountService.UpdateProfile(ctx, &req, actor, ctx.Request.UserAgent())
	if err != nil {
		writeAccountError(ctx, err)
		return
	}
	loggers.SuccessResponse(ctx, account)
}

// handleChangePassword: PUT /admin/account/password {currentPassword, newPassword}
// - signs the user out everywhere.
func (h *AccountHandler) handleChangePassword(ctx *gin.Context) {
	actor, ok := middlewares.ActorFromContext(ctx)
	if !ok {
		return
	}
	var req models.ChangePasswordRequest
	if !utils.ValidatePayload(ctx, &req) {
		return
	}
	if err := h.accountService.ChangePassword(ctx, &req, actor, ctx.Request.UserAgent()); err != nil {
		writeAccountError(ctx, err)
		return
	}
	loggers.SuccessResponse(ctx, gin.H{"signedOut": true})
}

func writeAccountError(ctx *gin.Context, err error) {
	switch {
	case errors.Is(err, cc.ErrInvalidAccessToken):
		loggers.GetCommonError(ctx, err.Error(), http.StatusUnauthorized)
	// 403, not 401: the panel treats 401 as an expired session and signs out.
	case errors.Is(err, cc.ErrPasswordIncorrect):
		loggers.GetCommonError(ctx, err.Error(), http.StatusForbidden)
	case errors.Is(err, cc.ErrAccountTemporarilyLocked):
		loggers.GetCommonError(ctx, err.Error(), http.StatusTooManyRequests)
	case errors.Is(err, cc.ErrEmailAlreadyExists):
		loggers.GetCommonError(ctx, err.Error(), http.StatusConflict)
	case errors.Is(err, cc.ErrPasswordTooShort),
		errors.Is(err, cc.ErrPasswordUnchanged),
		errors.Is(err, cc.ErrInvalidPayload):
		loggers.GetCommonError(ctx, err.Error(), http.StatusBadRequest)
	default:
		loggers.GetCommonError(ctx, err.Error(), http.StatusInternalServerError)
	}
}
