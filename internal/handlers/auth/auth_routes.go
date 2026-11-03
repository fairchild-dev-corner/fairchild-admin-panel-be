package handlers

import (
	"net/http"
	"time"

	cc "fairchild-admin-panel-be/internal/constants"
	middlewares "fairchild-admin-panel-be/internal/middlewares"
	service "fairchild-admin-panel-be/internal/services/auth"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

const (
	// Same budget as fairchild_be's /auth group: ~5 requests per 10s per IP.
	// The DB-backed lockout in AuthService.Login is the real brute-force
	// guard; this just sheds floods before they reach Argon2.
	authRateLimitRefill = 2 * time.Second
	authRateLimitBurst  = 5
)

type AuthHandler struct {
	authService *service.AuthService
	buildEnv    string
}

func NewHandler(authService *service.AuthService, buildEnv string) *AuthHandler {
	return &AuthHandler{authService: authService, buildEnv: buildEnv}
}

func (h *AuthHandler) RegisterRoutes(router *gin.RouterGroup) {
	auth := router.Group("/auth")
	limiter := middlewares.NewIPRateLimiter(rate.Every(authRateLimitRefill), authRateLimitBurst)
	auth.Use(limiter.Middleware())

	auth.POST("/login", h.handleLogin)
	auth.POST("/refresh", h.handleRefreshToken)
	auth.POST("/logout", h.handleLogout)
}

// setRefreshTokenCookie stores the refresh token as an HttpOnly cookie
// scoped to /api/v1/auth (refresh + logout are the only readers). Dev runs
// panel and API on localhost (same-site, plain HTTP) so Lax/insecure works;
// stage/prod may sit on different sites, which needs SameSite=None; Secure.
func setRefreshTokenCookie(ctx *gin.Context, buildEnv, token string, maxAgeSeconds int) {
	secure := buildEnv != "dev"
	if secure {
		ctx.SetSameSite(http.SameSiteNoneMode)
	} else {
		ctx.SetSameSite(http.SameSiteLaxMode)
	}
	ctx.SetCookie(cc.REFRESH_TOKEN_COOKIE_NAME, token, maxAgeSeconds, cc.REFRESH_TOKEN_COOKIE_PATH, "", secure, true)
}

// clearRefreshTokenCookie must use the same attributes as
// setRefreshTokenCookie or the browser treats it as a different cookie.
func clearRefreshTokenCookie(ctx *gin.Context, buildEnv string) {
	setRefreshTokenCookie(ctx, buildEnv, "", -1)
}

func clientInfo(ctx *gin.Context) service.ClientInfo {
	return service.ClientInfo{IP: ctx.ClientIP(), UserAgent: ctx.Request.UserAgent()}
}
