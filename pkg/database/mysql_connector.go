package database

import (
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	models "fairchild-admin-panel-be/internal/models/config"

	"github.com/go-sql-driver/mysql"
)

/**
* MySQLInitConnector opens a pooled *sql.DB and verifies connectivity with
* a bounded retry/backoff loop. multiStatements enables sending several
* ;-separated statements in a single Exec - required by cmd/migrate (whose
* .sql files batch multiple DDL statements) but left off for the app's
* runtime connection, which only ever runs single parameterized queries.
**/
func MySQLInitConnector(dbConfig *models.MySQLConfig, multiStatements bool) (*sql.DB, error) {

	cfg, err := toDriverConfig(dbConfig, multiStatements)
	if err != nil {
		return nil, err
	}

	db, err := mySQLConnectWithRetry(cfg, 5, 2*time.Second)
	if err != nil {
		return nil, err
	}

	// The admin panel has a handful of concurrent users - a small pool is
	// plenty and keeps this service from eating into fairchild_be's share of
	// the server's max_connections.
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetConnMaxIdleTime(2 * time.Minute)

	return db, nil
}

// toDriverConfig maps our app-level MySQLConfig onto the driver's own
// mysql.Config{} - so credentials/host/db name are carried as typed fields
// instead of a hand-built DSN string.
func toDriverConfig(c *models.MySQLConfig, multiStatements bool) (*mysql.Config, error) {
	cfg := mysql.NewConfig()
	cfg.User = c.User
	cfg.Passwd = c.Password
	cfg.Net = "tcp"
	cfg.Addr = fmt.Sprintf("%s:%s", c.Host, c.Port)
	cfg.DBName = c.DBName
	cfg.ParseTime = true
	cfg.Loc = time.UTC
	cfg.Collation = "utf8mb4_general_ci"
	cfg.MultiStatements = multiStatements
	// Pin the session to UTC. The driver serializes time.Time args in Loc
	// (UTC) but MySQL reads them in the session time zone, so on a server
	// running +08:00 an inserted expires_at would otherwise land 8h off.
	// With this, NOW(), inserted times and scanned TIMESTAMPs all agree.
	cfg.Params = map[string]string{"time_zone": "'+00:00'"}

	if c.CACert != "" {
		tlsConfigName, err := registerTLSConfig(c.BuildEnv, c.CACert)
		if err != nil {
			return nil, err
		}
		cfg.TLSConfig = tlsConfigName
	}

	return cfg, nil
}

// registerTLSConfig parses a PEM-encoded CA certificate and registers a
// named TLS config with the driver (e.g. DigitalOcean managed MySQL, which
// requires TLS and provides its own CA cert).
func registerTLSConfig(name, caCertPEM string) (string, error) {
	pool := x509.NewCertPool()
	if ok := pool.AppendCertsFromPEM([]byte(caCertPEM)); !ok {
		return "", fmt.Errorf("failed to parse CA certificate for MySQL TLS config %q", name)
	}

	if err := mysql.RegisterTLSConfig(name, &tls.Config{RootCAs: pool}); err != nil {
		return "", fmt.Errorf("failed to register MySQL TLS config %q: %w", name, err)
	}

	return name, nil
}

func mySQLConnectWithRetry(cfg *mysql.Config, maxRetries int, baseDelay time.Duration) (*sql.DB, error) {
	var err error

	for i := 0; i < maxRetries; i++ {
		slog.Info("Trying to connect to MySQL", "db", cfg.DBName, "attempt", i+1, "of", maxRetries)

		connector, connErr := mysql.NewConnector(cfg)
		if connErr != nil {
			// Malformed config (bad TLS name, invalid auth plugin, etc.) -
			// no amount of retrying fixes this, fail immediately.
			return nil, fmt.Errorf("invalid MySQL config: %w", connErr)
		}

		db := sql.OpenDB(connector)

		if pingErr := db.Ping(); pingErr != nil {
			slog.Error("Failed to ping MySQL", "db", cfg.DBName, "error", pingErr)
			db.Close()
			err = pingErr

			if i < maxRetries-1 {
				sleep := baseDelay * time.Duration(1<<i)
				slog.Info("Retrying MySQL connection", "in", sleep)
				time.Sleep(sleep)
			}
			continue
		}

		slog.Info("Successfully connected DB", "db", cfg.DBName)
		return db, nil
	}

	return nil, fmt.Errorf("could not connect to MySQL %q after %d attempts: %w", cfg.DBName, maxRetries, err)
}
