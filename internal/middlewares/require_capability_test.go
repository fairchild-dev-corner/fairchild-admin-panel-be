package middlewares

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	cc "fairchild-admin-panel-be/internal/constants"
	models "fairchild-admin-panel-be/internal/models/auth"

	"github.com/gin-gonic/gin"
)

func capabilityRouter(role string, allows CapabilityChecker, keys ...string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(ctx *gin.Context) {
		if role != "" {
			ctx.Set(panelUserKey, &models.UserPanel{ID: 1, Role: role})
		}
	})
	if allows != nil {
		r.Use(WithCapabilities(allows))
	}
	r.GET("/x", RequireCapability(keys...), func(ctx *gin.Context) { ctx.Status(http.StatusNoContent) })
	return r
}

func status(r *gin.Engine) int {
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/x", nil))
	return w.Code
}

// staffMay allows accounts below admin the given keys only.
func staffMay(keys ...string) CapabilityChecker {
	return func(_ context.Context, user *models.UserPanel, key string) (bool, error) {
		if user.Role == cc.ROLE_ADMIN || user.Role == cc.ROLE_SUPER_ADMIN {
			return true, nil
		}
		for _, k := range keys {
			if k == key {
				return true, nil
			}
		}
		return false, nil
	}
}

func TestRequireCapability(t *testing.T) {
	tests := []struct {
		name   string
		role   string
		allows CapabilityChecker
		keys   []string
		want   int
	}{
		{"admin", cc.ROLE_ADMIN, staffMay(), []string{"a"}, http.StatusNoContent},
		{"super admin", cc.ROLE_SUPER_ADMIN, staffMay(), []string{"a"}, http.StatusNoContent},
		{"staff allowed", cc.ROLE_STAFF, staffMay("a"), []string{"a"}, http.StatusNoContent},
		{"manager refused", cc.ROLE_MANAGER, staffMay("b"), []string{"a"}, http.StatusForbidden},
		{"staff refused", cc.ROLE_STAFF, staffMay("b"), []string{"a"}, http.StatusForbidden},
		{"any of several keys", cc.ROLE_STAFF, staffMay("c"), []string{"a", "b", "c"}, http.StatusNoContent},
		{"no user", "", staffMay("a"), []string{"a"}, http.StatusForbidden},
		{"no checker fails closed", cc.ROLE_STAFF, nil, []string{"a"}, http.StatusForbidden},
		{
			"checker error",
			cc.ROLE_STAFF,
			func(context.Context, *models.UserPanel, string) (bool, error) { return false, errors.New("db down") },
			[]string{"a"},
			http.StatusInternalServerError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := status(capabilityRouter(tt.role, tt.allows, tt.keys...)); got != tt.want {
				t.Errorf("status %d, want %d", got, tt.want)
			}
		})
	}
}
