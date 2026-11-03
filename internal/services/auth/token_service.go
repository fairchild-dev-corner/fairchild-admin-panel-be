package services

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	cc "fairchild-admin-panel-be/internal/constants"
	models "fairchild-admin-panel-be/internal/models/config"

	"github.com/golang-jwt/jwt/v4"
)

// Claims is the JWT payload signed into every panel access token. Role is
// informational only - RequirePanelAuth re-reads the user's current role and
// status from users_panel on every request.
type Claims struct {
	UserPanelID int64  `json:"upid"`
	Username    string `json:"username"`
	Role        string `json:"role"`
	jwt.RegisteredClaims
}

type TokenService struct {
	cfg *models.JWTConfig
}

func NewTokenService(cfg *models.JWTConfig) *TokenService {
	return &TokenService{cfg: cfg}
}

func (t *TokenService) AccessTokenTTL() time.Duration {
	return time.Duration(t.cfg.AccessTTLMinutes) * time.Minute
}

func (t *TokenService) RefreshTokenTTL() time.Duration {
	return time.Duration(t.cfg.RefreshTTLHours) * time.Hour
}

func (t *TokenService) GenerateAccessToken(userPanelID int64, username, role string) (string, error) {
	now := time.Now()

	claims := Claims{
		UserPanelID: userPanelID,
		Username:    username,
		Role:        role,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    cc.JWT_ISSUER,
			Audience:  jwt.ClaimStrings{cc.JWT_AUDIENCE},
			Subject:   fmt.Sprintf("%d", userPanelID),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(t.AccessTokenTTL())),
		},
	}

	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(t.cfg.AccessSecret))
}

// ParseAccessToken verifies signature, expiry, issuer and audience.
func (t *TokenService) ParseAccessToken(tokenStr string) (*Claims, error) {
	claims := &Claims{}

	token, err := jwt.ParseWithClaims(tokenStr, claims, func(tok *jwt.Token) (interface{}, error) {
		if tok.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method: %v", tok.Header["alg"])
		}
		return []byte(t.cfg.AccessSecret), nil
	})
	if err != nil || !token.Valid {
		return nil, cc.ErrInvalidAccessToken
	}

	// jwt/v4's Valid() only checks time-based claims; iss/aud must be
	// verified explicitly.
	if !claims.VerifyIssuer(cc.JWT_ISSUER, true) || !claims.VerifyAudience(cc.JWT_AUDIENCE, true) || claims.UserPanelID <= 0 {
		return nil, cc.ErrInvalidAccessToken
	}

	return claims, nil
}

// GenerateOpaqueToken produces the raw refresh token handed to the client.
// Only its SHA-256 hash (utils.MustStringSHA256) is ever persisted.
func GenerateOpaqueToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
