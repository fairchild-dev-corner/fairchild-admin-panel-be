package router

import (
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"testing"
)

func TestSecurityCORSAllowlist(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, origin := range []string{"https://panel.example.com", "https://evil.example", "https://panel.example.com.evil.example", "null"} {
		r := gin.New()
		cfg := NewRouterConfig(r, " https://panel.example.com ")
		cfg.InitConfig()
		called := false
		r.GET("/", func(c *gin.Context) { called = true; c.Status(204) })
		req := httptest.NewRequest("OPTIONS", "/", nil)
		req.Header.Set("Origin", origin)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != 204 || called {
			t.Fatal("preflight reached handler")
		}
		allowed := w.Header().Get("Access-Control-Allow-Origin")
		if origin == "https://panel.example.com" {
			if allowed != origin || w.Header().Get("Access-Control-Allow-Credentials") != "true" {
				t.Fatal(w.Header())
			}
		} else if allowed != "" || w.Header().Get("Access-Control-Allow-Credentials") != "" {
			t.Fatalf("untrusted origin allowed: %s", origin)
		}
	}
}
