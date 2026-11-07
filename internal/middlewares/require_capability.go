package middlewares

import (
	"context"
	"net/http"

	cc "fairchild-admin-panel-be/internal/constants"
	models "fairchild-admin-panel-be/internal/models/auth"
	loggers "fairchild-admin-panel-be/internal/utils/loggers"

	"github.com/gin-gonic/gin"
)

const capabilityCheckerKey = "capability_checker"

// CapabilityChecker is CapabilitiesService.Allows.
type CapabilityChecker func(ctx context.Context, user *models.UserPanel, key string) (bool, error)

// WithCapabilities makes the checker available to RequireCapability. It runs
// on the /admin group, after RequirePanelAuth.
func WithCapabilities(allows CapabilityChecker) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		ctx.Set(capabilityCheckerKey, allows)
		ctx.Next()
	}
}

// RequireCapability answers 403 unless the caller has at least one of keys
// (internal/models/capabilities). Super admins and admins have every
// capability; managers, supervisors and staff have what an admin assigned
// to their account on the Capabilities page. Routes shared by several pages
// list each page's key.
func RequireCapability(keys ...string) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		user, ok := PanelUserFromContext(ctx)
		v, _ := ctx.Get(capabilityCheckerKey)
		allows, hasChecker := v.(CapabilityChecker)
		if !ok || !hasChecker {
			loggers.GetCommonError(ctx, cc.FORBIDDEN, http.StatusForbidden)
			ctx.Abort()
			return
		}

		for _, key := range keys {
			allowed, err := allows(ctx, user, key)
			if err != nil {
				loggers.GetCommonError(ctx, err.Error(), http.StatusInternalServerError)
				ctx.Abort()
				return
			}
			if allowed {
				ctx.Next()
				return
			}
		}
		loggers.GetCommonError(ctx, cc.FORBIDDEN, http.StatusForbidden)
		ctx.Abort()
	}
}
