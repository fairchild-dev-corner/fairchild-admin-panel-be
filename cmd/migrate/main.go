package main

import (
	"embed"
	"errors"
	"log"
	"log/slog"
	"os"

	conf "fairchild-admin-panel-be/config"
	cc "fairchild-admin-panel-be/internal/constants"
	"fairchild-admin-panel-be/pkg/database"

	"github.com/golang-migrate/migrate/v4"
	migratemysql "github.com/golang-migrate/migrate/v4/database/mysql"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

var BuildEnv string

// The migrations are compiled into the binary, so the Docker image can run
// them on start-up without shipping the .sql files beside it.
//
//go:embed migrations/*.sql
var migrationsFS embed.FS

// migrationsTable keeps this service's migration history apart from
// fairchild_be's. Both run golang-migrate against the same central Auth DB
// (fccmpc_coop_auth_central); with the default "schema_migrations" table
// each would see the other's version number and skip or re-run migrations.
const migrationsTable = "admin_panel_schema_migrations"

func main() {
	if len(os.Args) < 2 {
		log.Fatal("usage: migrate <up|down>")
	}

	direction := os.Args[1]
	if direction != "up" && direction != "down" {
		log.Fatalf("unknown direction %q: expected \"up\" or \"down\"", direction)
	}

	mysqlConf, mErr := conf.GetMySQLConfig(BuildEnv)
	if mErr != nil {
		slog.Error("Failed to initialize MySQL config", "error", mErr)
		panic(cc.UNINITIALIZED_PANIC_STATE)
	}

	db, err := database.MySQLInitConnector(mysqlConf, true)
	if err != nil {
		log.Fatalf("failed to connect to MySQL: %v", err)
	}
	defer db.Close()

	driver, err := migratemysql.WithInstance(db, &migratemysql.Config{MigrationsTable: migrationsTable})
	if err != nil {
		log.Fatalf("failed to init migrate driver: %v", err)
	}

	source, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		log.Fatalf("failed to read embedded migrations: %v", err)
	}

	m, err := migrate.NewWithInstance("iofs", source, "mysql", driver)
	if err != nil {
		log.Fatalf("failed to init migrate instance: %v", err)
	}

	if err := runMigration(m, direction); err != nil {
		log.Fatalf("migration %s failed: %v", direction, err)
	}

	log.Printf("migration %s complete", direction)
}

// runMigration applies every pending migration (up) or rolls back every
// applied migration (down). migrate.ErrNoChange just means the schema was
// already at the target state - not a real failure.
func runMigration(m *migrate.Migrate, direction string) error {
	var err error

	if direction == "up" {
		err = m.Up()
	} else {
		err = m.Down()
	}

	if errors.Is(err, migrate.ErrNoChange) {
		return nil
	}
	return err
}
