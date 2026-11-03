package handlers

import (
	"errors"
	"net/http"

	cc "fairchild-admin-panel-be/internal/constants"
	models "fairchild-admin-panel-be/internal/models/auth"
	"fairchild-admin-panel-be/internal/utils"
	loggers "fairchild-admin-panel-be/internal/utils/loggers"

	"github.com/gin-gonic/gin"
)

// handleLogin: POST /auth/login { username, password } -> tokens. The
// refresh token goes out only as an HttpOnly cookie.
func (h *AuthHandler) handleLogin(ctx *gin.Context) {
	var req models.LoginRequest
	if !utils.ValidatePayload(ctx, &req) {
		return
	}

	tokens, err := h.authService.Login(ctx, &req, clientInfo(ctx))
	if err != nil {
		switch {
		case errors.Is(err, cc.ErrInvalidCredentials):
			loggers.GetCommonError(ctx, err.Error(), http.StatusUnauthorized)
		case errors.Is(err, cc.ErrAccountSuspended):
			loggers.GetCommonError(ctx, err.Error(), http.StatusForbidden)
		case errors.Is(err, cc.ErrAccountTemporarilyLocked):
			loggers.GetCommonError(ctx, err.Error(), http.StatusTooManyRequests)
		default:
			loggers.GetCommonError(ctx, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	setRefreshTokenCookie(ctx, h.buildEnv, tokens.RefreshToken, int(tokens.RefreshTTL))
	loggers.SuccessResponse(ctx, tokens.Response)
}

// handleRefreshToken: POST /auth/refresh - reads the refresh cookie, rotates
// it, and returns a fresh access token.
func (h *AuthHandler) handleRefreshToken(ctx *gin.Context) {
	refreshToken, _ := ctx.Cookie(cc.REFRESH_TOKEN_COOKIE_NAME)

	tokens, err := h.authService.Refresh(ctx, refreshToken, clientInfo(ctx))
	if err != nil {
		if errors.Is(err, cc.ErrInvalidRefreshToken) {
			clearRefreshTokenCookie(ctx, h.buildEnv)
			loggers.GetCommonError(ctx, err.Error(), http.StatusUnauthorized)
			return
		}
		loggers.GetCommonError(ctx, err.Error(), http.StatusInternalServerError)
		return
	}

	setRefreshTokenCookie(ctx, h.buildEnv, tokens.RefreshToken, int(tokens.RefreshTTL))
	loggers.SuccessResponse(ctx, tokens.Response)
}

// handleLogout: POST /auth/logout - revokes the session and clears the
// cookie. Always succeeds from the client's point of view.
func (h *AuthHandler) handleLogout(ctx *gin.Context) {
	refreshToken, _ := ctx.Cookie(cc.REFRESH_TOKEN_COOKIE_NAME)

	if err := h.authService.Logout(ctx, refreshToken); err != nil {
		loggers.GetCommonError(ctx, err.Error(), http.StatusInternalServerError)
		return
	}

	clearRefreshTokenCookie(ctx, h.buildEnv)
	loggers.SuccessResponse(ctx, cc.SUCCESS_LOGOUT)
}
