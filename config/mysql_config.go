package config

import (
	"errors"
	"fmt"
	"os"
	"strings"

	models "fairchild-admin-panel-be/internal/models/config"
)

/**
* Custom MySQL (central Auth DB) Configuration & Enviroment Setup
* @Description: Define Setup Configuration {Host, Port, User, Password, DBName}
**/
func GetMySQLConfig(env string) (*models.MySQLConfig, error) {
	switch env {
	case "dev":
		return models.NewMySQLConfig(
			Envs.MySQLLocalHost,
			Envs.MySQLLocalPort,
			Envs.MySQLLocalUser,
			Envs.MySQLLocalPassword,
			Envs.MySQLLocalDBName,
			env,
			"",
		), nil

	case "stage":
		caCert, err := resolveCACert(Envs.MySQLStagCACert)
		if err != nil {
			return nil, err
		}
		return models.NewMySQLConfig(
			Envs.MySQLStagHost,
			Envs.MySQLStagPort,
			Envs.MySQLStagUser,
			Envs.MySQLStagPassword,
			Envs.MySQLStagDBName,
			env,
			caCert,
		), nil

	case "prod":
		// without a CA cert toDriverConfig sets no TLS at all - never let prod
		// fall back to plaintext over the public internet
		if Envs.MySQLProdCACert == "" {
			return nil, errors.New("missing MYSQL_PROD_CA_CERT")
		}
		caCert, err := resolveCACert(Envs.MySQLProdCACert)
		if err != nil {
			return nil, err
		}
		return models.NewMySQLConfig(
			Envs.MySQLProdHost,
			Envs.MySQLProdPort,
			Envs.MySQLProdUser,
			Envs.MySQLProdPassword,
			Envs.MySQLProdDBName,
			env,
			caCert,
		), nil

	default:
		return nil, errors.New("unknown environment: " + env)
	}
}

// resolveCACert accepts MYSQL_*_CA_CERT as either the PEM itself or a path to
// a PEM file (relative to the working dir).
func resolveCACert(v string) (string, error) {
	v = strings.TrimSpace(v)
	if v == "" || strings.HasPrefix(v, "-----BEGIN") {
		return v, nil
	}

	pem, err := os.ReadFile(v)
	if err != nil {
		return "", fmt.Errorf("failed to read MySQL CA cert file %q: %w", v, err)
	}
	return string(pem), nil
}

// GetBranchMySQLConfigs returns one config per branch ledger DB (main,
// knowles, tiic, ...). Every branch schema sits on the same MySQL server as
// the central Auth DB, so each entry reuses its host/port/credentials/CA cert
// and only swaps the DB name. An empty list is allowed: the dashboard then
// reports zero active loans and the health page lists no ledger DBs.
func GetBranchMySQLConfigs(env string) (map[string]*models.MySQLConfig, error) {
	central, err := GetMySQLConfig(env)
	if err != nil {
		return nil, err
	}

	raw := Envs.MySQLLocalBranchDBs
	switch env {
	case "stage":
		raw = Envs.MySQLStagBranchDBs
	case "prod":
		raw = Envs.MySQLProdBranchDBs
	}

	pairs, err := parsePairs(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid branch DB list: %w", err)
	}

	confs := make(map[string]*models.MySQLConfig, len(pairs))
	for _, p := range pairs {
		confs[p.Key] = models.NewMySQLConfig(central.Host, central.Port, central.User, central.Password, p.Value, central.BuildEnv, central.CACert)
	}
	return confs, nil
}

type pair struct {
	Key   string
	Value string
}

// parsePairs splits "a=x,b=y" into ordered pairs, rejecting malformed and
// duplicate keys.
func parsePairs(raw string) ([]pair, error) {
	var out []pair
	seen := make(map[string]bool)

	for _, entry := range strings.Split(raw, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}

		key, value, ok := strings.Cut(entry, "=")
		key, value = strings.TrimSpace(key), strings.TrimSpace(value)
		if !ok || key == "" || value == "" {
			return nil, fmt.Errorf("invalid entry %q, expected key=value", entry)
		}
		if seen[key] {
			return nil, fmt.Errorf("duplicate key %q", key)
		}
		seen[key] = true
		out = append(out, pair{Key: key, Value: value})
	}

	return out, nil
}
