package router

import (
	"slices"
	"strings"

	logger "fairchild-admin-panel-be/internal/utils/loggers"

	"github.com/gin-gonic/gin"
)

/**
* App Router Configuration
* -------------------------
* Sets up the base API path (/api/v1), global error handling and CORS.
* CORS must run before auth so unauthenticated preflight requests pass;
* per-route guarding is applied via middlewares.RequirePanelAuth.
**/
type RouterConfig struct {
	router           *gin.Engine
	originsPermitted string
}

func NewRouterConfig(m *gin.Engine, origins string) *RouterConfig {
	return &RouterConfig{
		router:           m,
		originsPermitted: origins,
	}
}

func (r *RouterConfig) InitConfig() {
	r.router.Use(logger.ErrorHandler())
	r.router.Use(r.CorsMiddleware())
}

func (r *RouterConfig) RoutesGroup() *gin.RouterGroup {
	return r.router.Group("/api/v1")
}

// CorsMiddleware reflects an allow-listed Origin with credentials (the
// refresh cookie). The allow-list should hold the admin panel's origin, plus
// fairchild-fe's origin if it reads the public GET /site/status from here.
func (r *RouterConfig) CorsMiddleware() gin.HandlerFunc {
	var allowedOrigins []string
	for _, o := range strings.Split(r.originsPermitted, ",") {
		if o = strings.TrimSpace(o); o != "" {
			allowedOrigins = append(allowedOrigins, o)
		}
	}

	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")

		if slices.Contains(allowedOrigins, origin) {
			c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
			c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")
			c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
			c.Writer.Header().Add("Vary", "Origin")
		}

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}
