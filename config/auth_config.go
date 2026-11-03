package config

import (
	"errors"
	"strconv"

	models "fairchild-admin-panel-be/internal/models/config"
)

// minJWTSecretLen guards against a placeholder secret making it to a real
// environment - HS256 needs at least 256 bits of key material.
const minJWTSecretLen = 32

// GetJWTConfig loads the admin signing secret and token lifetimes. Defaults
// are shorter than fairchild_be's member sessions (15m access / 12h refresh)
// since panel accounts can approve applications and take the site offline.
func GetJWTConfig() (*models.JWTConfig, error) {
	if len(Envs.AdminJWTAccessSecret) < minJWTSecretLen {
		return nil, errors.New("ADMIN_JWT_ACCESS_SECRET must be set and at least 32 characters")
	}

	accessTTL, err := strconv.Atoi(Envs.AdminJWTAccessTTLMinutes)
	if err != nil || accessTTL <= 0 {
		accessTTL = 15
	}

	refreshTTL, err := strconv.Atoi(Envs.AdminRefreshTTLHours)
	if err != nil || refreshTTL <= 0 {
		refreshTTL = 12
	}

	return models.NewJWTConfig(Envs.AdminJWTAccessSecret, accessTTL, refreshTTL), nil
}

// GetHealthCheckTargets parses HEALTH_CHECK_URLS ("name=url,name=url") into
// the external HTTP services probed by GET /admin/health.
func GetHealthCheckTargets() ([]models.HealthCheckTarget, error) {
	pairs, err := parsePairs(Envs.HealthCheckURLs)
	if err != nil {
		return nil, errors.New("invalid HEALTH_CHECK_URLS: " + err.Error())
	}

	targets := make([]models.HealthCheckTarget, 0, len(pairs))
	for _, p := range pairs {
		targets = append(targets, models.HealthCheckTarget{Name: p.Key, URL: p.Value})
	}
	return targets, nil
}
