package services

import (
	"errors"
	cc "fairchild-admin-panel-be/internal/constants"
	"github.com/golang-jwt/jwt/v4"
	"testing"
	"time"
)

func TestSecurityTokenSigningMethodAndIdentity(t *testing.T) {
	s := newTestTokenService()
	for _, tc := range []struct {
		name   string
		method jwt.SigningMethod
		id     int64
		key    any
	}{
		{"none", jwt.SigningMethodNone, 1, jwt.UnsafeAllowNoneSignatureType},
		{"HS384", jwt.SigningMethodHS384, 1, []byte(testSecret)},
		{"zero user", jwt.SigningMethodHS256, 0, []byte(testSecret)},
		{"negative user", jwt.SigningMethodHS256, -1, []byte(testSecret)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			claims := Claims{UserPanelID: tc.id, RegisteredClaims: jwt.RegisteredClaims{Issuer: cc.JWT_ISSUER, Audience: jwt.ClaimStrings{cc.JWT_AUDIENCE}, ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))}}
			token, err := jwt.NewWithClaims(tc.method, claims).SignedString(tc.key)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := s.ParseAccessToken(token); !errors.Is(err, cc.ErrInvalidAccessToken) {
				t.Fatal("unsafe token accepted", err)
			}
		})
	}
}
func TestSecurityOpaqueTokensUnique(t *testing.T) {
	seen := map[string]bool{}
	for range 100 {
		token, err := GenerateOpaqueToken()
		if err != nil || len(token) != 64 || seen[token] {
			t.Fatalf("bad refresh token: length=%d err=%v", len(token), err)
		}
		seen[token] = true
	}
}
func BenchmarkAuth(b *testing.B) {
	s := newTestTokenService()
	token, err := s.GenerateAccessToken(42, "staff", cc.ROLE_STAFF)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if _, err := s.ParseAccessToken(token); err != nil {
			b.Fatal(err)
		}
	}
}
func BenchmarkPasswordHash(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		if _, err := HashPassword("correct horse battery"); err != nil {
			b.Fatal(err)
		}
	}
}
func FuzzParseAccessToken(f *testing.F) {
	for _, s := range []string{"", "invalid", "a.b.c", "eyJhbGciOiJub25lIn0.e30."} {
		f.Add(s)
	}
	s := newTestTokenService()
	f.Fuzz(func(t *testing.T, raw string) {
		claims, err := s.ParseAccessToken(raw)
		if err == nil && (claims == nil || claims.UserPanelID <= 0) {
			t.Fatal("invalid identity accepted")
		}
	})
}
