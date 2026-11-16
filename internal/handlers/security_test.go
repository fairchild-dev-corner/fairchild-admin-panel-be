package handlers_test

import (
	account "fairchild-admin-panel-be/internal/handlers/account"
	applications "fairchild-admin-panel-be/internal/handlers/applications"
	audit "fairchild-admin-panel-be/internal/handlers/audit"
	blasts "fairchild-admin-panel-be/internal/handlers/blasts"
	capabilities "fairchild-admin-panel-be/internal/handlers/capabilities"
	claims "fairchild-admin-panel-be/internal/handlers/claims"
	contact "fairchild-admin-panel-be/internal/handlers/contact"
	dashboard "fairchild-admin-panel-be/internal/handlers/dashboard"
	health "fairchild-admin-panel-be/internal/handlers/health"
	locks "fairchild-admin-panel-be/internal/handlers/locks"
	members "fairchild-admin-panel-be/internal/handlers/members"
	metrics "fairchild-admin-panel-be/internal/handlers/metrics"
	portal "fairchild-admin-panel-be/internal/handlers/portal"
	site "fairchild-admin-panel-be/internal/handlers/site"
	users "fairchild-admin-panel-be/internal/handlers/users"
	voters "fairchild-admin-panel-be/internal/handlers/voters"
	"fairchild-admin-panel-be/internal/middlewares"
	"github.com/gin-gonic/gin"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func guardedRoutes(t testing.TB) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	g := r.Group("/api/v1/admin", middlewares.RequirePanelAuth(nil))
	// No services: a request reaching a handler instead of being rejected will fail.
	for _, register := range []func(*gin.RouterGroup){
		account.NewHandler(nil).RegisterRoutes, applications.NewHandler(nil).RegisterRoutes,
		audit.NewHandler(nil).RegisterRoutes, blasts.NewHandler(nil, nil).RegisterRoutes,
		capabilities.NewHandler(nil).RegisterRoutes, claims.NewHandler(nil).RegisterRoutes,
		contact.NewHandler(nil).RegisterRoutes, dashboard.NewHandler(nil).RegisterRoutes,
		health.NewHandler(nil).RegisterRoutes, locks.NewHandler(nil).RegisterRoutes,
		members.NewHandler(nil).RegisterRoutes, metrics.NewHandler(nil).RegisterRoutes,
		portal.NewHandler(nil).RegisterRoutes, site.NewHandler(nil).RegisterRoutes,
		users.NewHandler(nil).RegisterRoutes, voters.NewHandler(nil).RegisterRoutes,
	} {
		register(g)
	}
	return r
}
func TestSecurityAllAdminHandlersRejectUnauthenticated(t *testing.T) {
	r := guardedRoutes(t)
	routes := r.Routes()
	if len(routes) < 50 {
		t.Fatalf("expected full admin route catalog, found %d", len(routes))
	}
	for _, route := range routes {
		t.Run(route.Method+" "+route.Path, func(t *testing.T) {
			path := route.Path
			for _, part := range strings.Split(path, "/") {
				if strings.HasPrefix(part, ":") {
					path = strings.ReplaceAll(path, part, "1")
				}
			}
			req := httptest.NewRequest(route.Method, path, strings.NewReader("{}"))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != http.StatusUnauthorized {
				t.Fatalf("got %d: %s", w.Code, w.Body.String())
			}
		})
	}
}
func BenchmarkHTTPUnauthorized(b *testing.B) {
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	b.Cleanup(func() { slog.SetDefault(previous) })
	r := guardedRoutes(b)
	b.ReportAllocs()
	for b.Loop() {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/admin/members", nil))
		if w.Code != http.StatusUnauthorized {
			b.Fatal(w.Code)
		}
	}
}
