package middlewares

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

	cc "fairchild-admin-panel-be/internal/constants"
	models "fairchild-admin-panel-be/internal/models/auth"
	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

func TestSecurityAdminRoleBoundary(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, role := range []string{"", cc.ROLE_STAFF, cc.ROLE_SUPERVISOR, cc.ROLE_MANAGER, cc.ROLE_ADMIN, cc.ROLE_SUPER_ADMIN, "unknown"} {
		t.Run(role, func(t *testing.T) {
			r := gin.New()
			if role != "" {
				r.Use(func(c *gin.Context) { c.Set(panelUserKey, &models.UserPanel{ID: 1, Role: role}) })
			}
			r.GET("/", RequireAdmin(), func(c *gin.Context) { c.Status(http.StatusNoContent) })
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
			want := http.StatusForbidden
			if role == cc.ROLE_ADMIN || role == cc.ROLE_SUPER_ADMIN {
				want = http.StatusNoContent
			}
			if w.Code != want {
				t.Fatalf("got %d want %d", w.Code, want)
			}
		})
	}
}

func TestSecurityMissingBearerNeverCallsAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, header := range []string{"", "Basic abc", "bearer abc"} {
		r := gin.New()
		r.GET("/", RequirePanelAuth(nil), func(c *gin.Context) { t.Fatal("protected handler ran") })
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Authorization", header)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Fatal(w.Code)
		}
	}
}

func TestSecurityIPRateLimiter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	limiter := &IPRateLimiter{visitors: map[string]*visitor{}, rate: rate.Limit(0), burst: 2}
	r := gin.New()
	if err := r.SetTrustedProxies(nil); err != nil {
		t.Fatal(err)
	}
	r.GET("/", limiter.Middleware(), func(c *gin.Context) { c.Status(http.StatusNoContent) })
	for i, want := range []int{204, 204, 429} {
		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = "192.0.2.1:1234"
		// Untrusted forwarded headers must not bypass the bucket.
		req.Header.Set("X-Forwarded-For", []string{"198.51.100.1", "198.51.100.2", "198.51.100.3"}[i])
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != want {
			t.Fatalf("request %d: %d want %d", i, w.Code, want)
		}
	}
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.0.2.2:1234"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 204 {
		t.Fatal("independent IP was blocked")
	}
}

func TestSecurityConcurrentRateLimit(t *testing.T) {
	limiter := &IPRateLimiter{visitors: map[string]*visitor{}, rate: 0, burst: 5}
	var wg sync.WaitGroup
	var allowed atomic.Int32
	for range 50 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if limiter.getLimiter("192.0.2.1").Allow() {
				allowed.Add(1)
			}
		}()
	}
	wg.Wait()
	if got := allowed.Load(); got != 5 {
		t.Fatalf("allowed %d want 5", got)
	}
}

func BenchmarkMiddlewareRateLimiter(b *testing.B) {
	limiter := &IPRateLimiter{visitors: map[string]*visitor{}, rate: rate.Inf, burst: 10}
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if !limiter.getLimiter("192.0.2.1").Allow() {
				b.Fatal("unexpected rate limit")
			}
		}
	})
}
