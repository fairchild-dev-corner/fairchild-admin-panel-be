package handlers

import (
	cc "fairchild-admin-panel-be/internal/constants"
	"github.com/gin-gonic/gin"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSecurityRefreshCookieAttributes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, env := range []string{"dev", "stage", "prod"} {
		t.Run(env, func(t *testing.T) {
			w := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(w)
			setRefreshTokenCookie(ctx, env, "test-token", 3600)
			cookies := w.Result().Cookies()
			if len(cookies) != 1 {
				t.Fatal(cookies)
			}
			c := cookies[0]
			if !c.HttpOnly || c.Path != cc.REFRESH_TOKEN_COOKIE_PATH || c.Name != cc.REFRESH_TOKEN_COOKIE_NAME || c.Secure != (env != "dev") {
				t.Fatal(c)
			}
			want := http.SameSiteNoneMode
			if env == "dev" {
				want = http.SameSiteLaxMode
			}
			if c.SameSite != want {
				t.Fatal(c)
			}
			w = httptest.NewRecorder()
			ctx, _ = gin.CreateTestContext(w)
			clearRefreshTokenCookie(ctx, env)
			cleared := w.Result().Cookies()[0]
			if cleared.MaxAge != -1 || cleared.Path != c.Path || cleared.Secure != c.Secure || cleared.SameSite != c.SameSite {
				t.Fatal("cookie deletion attributes differ")
			}
		})
	}
}
func TestLoginRejectsInvalidPayload(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, body := range []string{"{", "{}", `{"username":"staff"}`} {
		r := gin.New()
		h := NewHandler(nil, "dev")
		r.POST("/login", h.handleLogin)
		w := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/login", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		if w.Code != 400 {
			t.Fatalf("%q: %d", body, w.Code)
		}
	}
}
