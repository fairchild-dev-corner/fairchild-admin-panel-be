package config

import (
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	PublicHost string
	Port       string

	Dev_Allowed_Origins  string
	Stag_Allowed_Origins string
	Prod_Allowed_Origins string

	// MySQL (central Auth DB, e.g. fccmpc_coop_auth_central) - the same
	// schema fairchild_be uses. users_panel and the other admin tables live
	// here; cmd/migrate tracks them in its own migrations table so the two
	// services' migration histories never collide.
	MySQLLocalHost     string
	MySQLLocalPort     string
	MySQLLocalUser     string
	MySQLLocalPassword string
	MySQLLocalDBName   string
	MySQLStagHost      string
	MySQLStagPort      string
	MySQLStagUser      string
	MySQLStagPassword  string
	MySQLStagDBName    string
	MySQLStagCACert    string
	MySQLProdHost      string
	MySQLProdPort      string
	MySQLProdUser      string
	MySQLProdPassword  string
	MySQLProdDBName    string
	MySQLProdCACert    string

	// MySQL (branch ledger DBs) - comma-separated branch_code=db_name pairs,
	// same format as fairchild_be. Read-only; only used for the dashboard's
	// active-loan totals and the health page.
	MySQLLocalBranchDBs string
	MySQLStagBranchDBs  string
	MySQLProdBranchDBs  string

	// Auth / JWT - must NOT reuse fairchild_be's JWT_ACCESS_SECRET, otherwise
	// a member access token would verify here (issuer/audience checks are a
	// second line of defence, not the only one).
	AdminJWTAccessSecret     string
	AdminJWTAccessTTLMinutes string
	AdminRefreshTTLHours     string

	// Health page - comma-separated name=url pairs probed by GET /admin/health,
	// e.g. "fairchild_be API=http://localhost:8080/api/v1/".
	HealthCheckURLs string
}

/*Create Singleton*/
var Envs = initConfig()

func initConfig() Config {

	//reload enviroment var - ENV_FILE points at a mounted secret on Cloud Run,
	//otherwise fall back to ./.env for local runs
	if envFile := os.Getenv("ENV_FILE"); envFile != "" {
		godotenv.Load(envFile)
	} else {
		godotenv.Load()
	}

	return Config{
		PublicHost: getEnv("PUBLIC_HOST", "http://localhost"),
		Port:       getEnv("PORT", "8081"),

		Dev_Allowed_Origins:  getEnv("DEV_ALLOWED_ORIGINS", ""),
		Stag_Allowed_Origins: getEnv("STAG_ALLOWED_ORIGINS", ""),
		Prod_Allowed_Origins: getEnv("PROD_ALLOWED_ORIGINS", ""),

		MySQLLocalHost:     getEnv("MYSQL_LOCAL_HOST", ""),
		MySQLLocalPort:     getEnv("MYSQL_LOCAL_PORT", "3306"),
		MySQLLocalUser:     getEnv("MYSQL_LOCAL_USER", ""),
		MySQLLocalPassword: getEnv("MYSQL_LOCAL_PASSWORD", ""),
		MySQLLocalDBName:   getEnv("MYSQL_LOCAL_DB_NAME", ""),
		MySQLStagHost:      getEnv("MYSQL_STAG_HOST", ""),
		MySQLStagPort:      getEnv("MYSQL_STAG_PORT", ""),
		MySQLStagUser:      getEnv("MYSQL_STAG_USER", ""),
		MySQLStagPassword:  getEnv("MYSQL_STAG_PASSWORD", ""),
		MySQLStagDBName:    getEnv("MYSQL_STAG_DB_NAME", ""),
		MySQLStagCACert:    getEnv("MYSQL_STAG_CA_CERT", ""),
		MySQLProdHost:      getEnv("MYSQL_PROD_HOST", ""),
		MySQLProdPort:      getEnv("MYSQL_PROD_PORT", ""),
		MySQLProdUser:      getEnv("MYSQL_PROD_USER", ""),
		MySQLProdPassword:  getEnv("MYSQL_PROD_PASSWORD", ""),
		MySQLProdDBName:    getEnv("MYSQL_PROD_DB_NAME", ""),
		MySQLProdCACert:    getEnv("MYSQL_PROD_CA_CERT", ""),

		MySQLLocalBranchDBs: getEnv("MYSQL_LOCAL_BRANCH_DBS", ""),
		MySQLStagBranchDBs:  getEnv("MYSQL_STAG_BRANCH_DBS", ""),
		MySQLProdBranchDBs:  getEnv("MYSQL_PROD_BRANCH_DBS", ""),

		AdminJWTAccessSecret:     getEnv("ADMIN_JWT_ACCESS_SECRET", ""),
		AdminJWTAccessTTLMinutes: getEnv("ADMIN_JWT_ACCESS_TTL_MINUTES", ""),
		AdminRefreshTTLHours:     getEnv("ADMIN_REFRESH_TTL_HOURS", ""),

		HealthCheckURLs: getEnv("HEALTH_CHECK_URLS", ""),
	}
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
