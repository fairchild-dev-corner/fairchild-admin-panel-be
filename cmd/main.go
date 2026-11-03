package main

import (
	"log/slog"

	conf "fairchild-admin-panel-be/config"
	cc "fairchild-admin-panel-be/internal/constants"
	"fairchild-admin-panel-be/pkg/database"
)

var BuildEnv string

func main() {

	hostConf, hErr := conf.GetHostConfig(BuildEnv)
	if hErr != nil {
		slog.Error("Failed to initialize host config", "error", hErr)
		panic(cc.UNINITIALIZED_PANIC_STATE)
	}

	mysqlConf, mErr := conf.GetMySQLConfig(BuildEnv)
	if mErr != nil {
		slog.Error("Failed to initialize MySQL config", "error", mErr)
		panic(cc.UNINITIALIZED_PANIC_STATE)
	}

	branchConfs, bErr := conf.GetBranchMySQLConfigs(BuildEnv)
	if bErr != nil {
		slog.Error("Failed to initialize branch MySQL config", "error", bErr)
		panic(cc.UNINITIALIZED_PANIC_STATE)
	}
	if len(branchConfs) == 0 {
		slog.Warn("No branch ledger DBs configured - dashboard active-loan totals will be 0")
	}

	jwtConf, jErr := conf.GetJWTConfig()
	if jErr != nil {
		slog.Error("Failed to initialize JWT config", "error", jErr)
		panic(cc.UNINITIALIZED_PANIC_STATE)
	}

	healthTargets, htErr := conf.GetHealthCheckTargets()
	if htErr != nil {
		slog.Error("Failed to initialize health check config", "error", htErr)
		panic(cc.UNINITIALIZED_PANIC_STATE)
	}

	/*** Initialize MySQL Connector (central Auth DB) ***/
	mysqlDB, mysqlErr := database.MySQLInitConnector(mysqlConf, false)
	if mysqlErr != nil {
		slog.Error("Error connecting to MySQL", "error", mysqlErr)
		panic(cc.UNINITIALIZED_PANIC_STATE)
	}

	defer mysqlDB.Close()
	// Schema is managed via versioned migrations - see cmd/migrate.
	// Run `make migrate-up-dev` before starting the app.

	/*** Initialize MySQL Connectors (branch ledger DBs, read-only) ***/
	branchDBs, branchErr := database.InitBranchDBs(branchConfs)
	if branchErr != nil {
		slog.Error("Error connecting to branch MySQL", "error", branchErr)
		panic(cc.UNINITIALIZED_PANIC_STATE)
	}

	defer branchDBs.Close()

	// Connect Server
	conf.ServerConfig(hostConf, mysqlDB, branchDBs, jwtConf, healthTargets)
}
