package handlers

import (
	caps "fairchild-admin-panel-be/internal/models/capabilities"
	"time"

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
// fairchild-fe; fairchild_be reads the same row for its own copy.
func (h *SiteHandler) RegisterPublicRoutes(router *gin.RouterGroup) {
	limiter := middlewares.NewIPRateLimiter(rate.Every(publicStatusRateLimitRefill), publicStatusRateLimitBurst)
	router.GET("/site/status", limiter.Middleware(), h.handleGetPublicSiteStatus)
}

// RegisterRoutes mounts onto the already-guarded /admin group. Taking the
// member portal offline is admins only unless allowed for staff on the
// Capabilities page.
func (h *SiteHandler) RegisterRoutes(admin *gin.RouterGroup) {
	admin.GET("/site-status", middlewares.RequireCapability(caps.MaintenanceView), h.handleGetSiteStatus)
	admin.PUT("/site-status", middlewares.RequireCapability(caps.MaintenanceManage), h.handleUpdateSiteStatus)
}
