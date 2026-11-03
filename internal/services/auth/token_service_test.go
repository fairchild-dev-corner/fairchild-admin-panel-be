package services

import (
	"errors"
	"testing"
	"time"

	cc "fairchild-admin-panel-be/internal/constants"
	models "fairchild-admin-panel-be/internal/models/config"

	"github.com/golang-jwt/jwt/v4"
)

const testSecret = "0123456789abcdef0123456789abcdef"

func newTestTokenService() *TokenService {
	return NewTokenService(models.NewJWTConfig(testSecret, 15, 12))
}

func TestAccessTokenRoundTrip(t *testing.T) {
	ts := newTestTokenService()

	tok, err := ts.GenerateAccessToken(42, "jdoe", cc.ROLE_ADMIN)
	if err != nil {
		t.Fatal(err)
	}

	claims, err := ts.ParseAccessToken(tok)
	if err != nil {
		t.Fatalf("valid token rejected: %v", err)
	}
	if claims.UserPanelID != 42 || claims.Username != "jdoe" || claims.Role != cc.ROLE_ADMIN {
		t.Fatalf("unexpected claims: %+v", claims)
	}
}

// A member token from fairchild_be has no admin iss/aud. Even if both
// services were misconfigured with the same secret, it must not verify.
func TestAccessTokenRejectsForeignIssuerAndAudience(t *testing.T) {
	ts := newTestTokenService()

	sign := func(c jwt.Claims) string {
		s, err := jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString([]byte(testSecret))
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	exp := jwt.NewNumericDate(time.Now().Add(time.Hour))

	cases := map[string]string{
		"member-style token": sign(jwt.MapClaims{"uid": 42, "email": "a@b.c", "exp": exp.Unix()}),
		"wrong audience": sign(Claims{UserPanelID: 42, RegisteredClaims: jwt.RegisteredClaims{
			Issuer: cc.JWT_ISSUER, Audience: jwt.ClaimStrings{"fairchild-fe"}, ExpiresAt: exp,
		}}),
		"wrong issuer": sign(Claims{UserPanelID: 42, RegisteredClaims: jwt.RegisteredClaims{
			Issuer: "fairchild_be", Audience: jwt.ClaimStrings{cc.JWT_AUDIENCE}, ExpiresAt: exp,
		}}),
		"expired": sign(Claims{UserPanelID: 42, RegisteredClaims: jwt.RegisteredClaims{
			Issuer: cc.JWT_ISSUER, Audience: jwt.ClaimStrings{cc.JWT_AUDIENCE},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Minute)),
		}}),
	}

	for name, tok := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := ts.ParseAccessToken(tok); !errors.Is(err, cc.ErrInvalidAccessToken) {
				t.Fatalf("expected ErrInvalidAccessToken, got %v", err)
			}
		})
	}
}

func TestAccessTokenRejectsOtherSecret(t *testing.T) {
	other := NewTokenService(models.NewJWTConfig("ffffffffffffffffffffffffffffffff", 15, 12))
	tok, _ := other.GenerateAccessToken(1, "x", cc.ROLE_ADMIN)

	if _, err := newTestTokenService().ParseAccessToken(tok); err == nil {
		t.Fatal("token signed with another secret was accepted")
	}
}

func TestPasswordHashRoundTrip(t *testing.T) {
	hash, err := HashPassword("correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	if err := CheckPassword(hash, "correct horse battery"); err != nil {
		t.Fatalf("correct password rejected: %v", err)
	}
	if err := CheckPassword(hash, "wrong"); err == nil {
		t.Fatal("wrong password accepted")
	}
	if err := CheckPassword("$2a$10$notargon", "x"); err == nil {
		t.Fatal("non-argon2id hash accepted")
	}
}
