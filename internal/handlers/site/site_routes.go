package handlers

import (
	"time"

	cc "fairchild-admin-panel-be/internal/constants"
	middlewares "fairchild-admin-panel-be/internal/middlewares"
	service "fairchild-admin-panel-be/internal/services/site"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

const (
	// The public status endpoint is polled by every fairchild-fe visitor's
	// browser, so it gets a generous per-IP budget (~2 req/s sustained).
	publicStatusRateLimitRefill = 500 * time.Millisecond
	publicStatusRateLimitBurst  = 20
)

type SiteHandler struct {
	siteService *service.SiteService
}

func NewHandler(siteService *service.SiteService) *SiteHandler {
	return &SiteHandler{siteService: siteService}
}

// RegisterPublicRoutes mounts GET /site/status - no auth, read by
// fairchild-fe to show its maintenance banner.
func (h *SiteHandler) RegisterPublicRoutes(router *gin.RouterGroup) {
	limiter := middlewares.NewIPRateLimiter(rate.Every(publicStatusRateLimitRefill), publicStatusRateLimitBurst)
	router.GET("/site/status", limiter.Middleware(), h.handleGetPublicSiteStatus)
}

// RegisterRoutes mounts onto the already-guarded /admin group. Taking the
// public site offline is admin-only; staff can see the current state.
func (h *SiteHandler) RegisterRoutes(admin *gin.RouterGroup) {
	admin.GET("/site-status", h.handleGetSiteStatus)
	admin.PUT("/site-status", middlewares.RequireRole(cc.ROLE_ADMIN), h.handleUpdateSiteStatus)
}
