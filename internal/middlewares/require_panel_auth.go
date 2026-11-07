package middlewares

import (
	"errors"
	"net/http"
	"slices"
	"strings"

	cc "fairchild-admin-panel-be/internal/constants"
	audit_models "fairchild-admin-panel-be/internal/models/audit"
	models "fairchild-admin-panel-be/internal/models/auth"
	auth_service "fairchild-admin-panel-be/internal/services/auth"
	loggers "fairchild-admin-panel-be/internal/utils/loggers"

	"github.com/gin-gonic/gin"
)

const panelUserKey = "panel_user"

// RequirePanelAuth validates the "Authorization: Bearer <token>" header and
// loads the caller's users_panel row, rejecting anyone whose account isn't
// active. This is the real authorization boundary for every
// /api/v1/admin/* route - the panel's RequireAdminGuard is UI only.
func RequirePanelAuth(authService *auth_service.AuthService) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		header := ctx.GetHeader("Authorization")
		const prefix = "Bearer "

		if !strings.HasPrefix(header, prefix) {
			loggers.GetCommonError(ctx, cc.MISSING_BEARER_TOKEN, http.StatusUnauthorized)
			ctx.Abort()
			return
		}

		user, err := authService.Authenticate(ctx, strings.TrimPrefix(header, prefix))
		if err != nil {
			if errors.Is(err, cc.ErrInvalidAccessToken) {
				loggers.GetCommonError(ctx, err.Error(), http.StatusUnauthorized)
			} else {
				loggers.GetCommonError(ctx, err.Error(), http.StatusInternalServerError)
			}
			ctx.Abort()
			return
		}

		ctx.Set(panelUserKey, user)
		ctx.Next()
	}
}

// RequireRole must run after RequirePanelAuth. It answers 403 when the
// caller's current role (from the DB, not the token) isn't in roles.
func RequireRole(roles ...string) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		user, ok := PanelUserFromContext(ctx)
		if !ok || !slices.Contains(roles, user.Role) {
			loggers.GetCommonError(ctx, cc.FORBIDDEN, http.StatusForbidden)
			ctx.Abort()
			return
		}
		ctx.Next()
	}
}

// RequireAdmin is RequireRole for super admins and admins, who manage
// accounts and capabilities.
func RequireAdmin() gin.HandlerFunc {
	return RequireRole(cc.ROLE_SUPER_ADMIN, cc.ROLE_ADMIN)
}

// ActorFromContext builds the audit actor for the authenticated panel user.
// It answers 401 itself when RequirePanelAuth didn't run.
func ActorFromContext(ctx *gin.Context) (audit_models.Actor, bool) {
	user, ok := PanelUserFromContext(ctx)
	if !ok {
		loggers.GetCommonError(ctx, cc.MISSING_BEARER_TOKEN, http.StatusUnauthorized)
		return audit_models.Actor{}, false
	}
	return audit_models.Actor{UserPanelID: user.ID, Name: user.DisplayName(), IP: ctx.ClientIP()}, true
}

// PanelUserFromContext returns the authenticated panel user set by
// RequirePanelAuth.
func PanelUserFromContext(ctx *gin.Context) (*models.UserPanel, bool) {
	v, exists := ctx.Get(panelUserKey)
	if !exists {
		return nil, false
	}
	user, ok := v.(*models.UserPanel)
	return user, ok && user != nil
}
