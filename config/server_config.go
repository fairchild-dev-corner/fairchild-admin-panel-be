package config

import (
	"database/sql"
	"log/slog"

	core "fairchild-admin-panel-be/cmd/api"
	models "fairchild-admin-panel-be/internal/models/config"
	"fairchild-admin-panel-be/pkg/database"

	"github.com/gin-gonic/gin"
)

func ServerConfig(
	hostConf *models.HostConfig,
	mysqlDB *sql.DB,
	branchDBs database.BranchDBs,
	jwtConf *models.JWTConfig,
	healthTargets []models.HealthCheckTarget,
	messagingConf *models.MessagingConfig,
) {

	if hostConf.BuildEnv != "dev" {
		gin.SetMode(gin.ReleaseMode)
	}

	ginServer := gin.Default()

	// Only trust X-Forwarded-For from Cloud Run's front end (link-local hop),
	// so ClientIP() - used by the IP rate limiter, login lockout and audit
	// log - is the address it appended, not a spoofable client-supplied one.
	if err := ginServer.SetTrustedProxies([]string{"169.254.0.0/16"}); err != nil {
		slog.Error("invalid trusted proxies", "err", err)
	}

	slog.Info("App Running", "addr", hostConf.Host)
	slog.Info("Running in --" + hostConf.BuildEnv + " mode & setup config")
	slog.Info("Starting Server", "port", hostConf.Port)

	// Run Server
	core.NewAPIServer(ginServer, hostConf, mysqlDB, branchDBs, jwtConf, healthTargets, messagingConf).Run()
}
