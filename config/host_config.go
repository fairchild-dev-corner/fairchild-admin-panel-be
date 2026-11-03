package config

import (
	"errors"

	models "fairchild-admin-panel-be/internal/models/config"
)

/**
* Custom Host Configuration & Enviroment Setup
* @Description: Define Setup Configuration {Host, Port, ENV, AllowedOrigins}
**/
func GetHostConfig(env string) (*models.HostConfig, error) {
	switch env {
	case "dev":
		return models.NewHostConfig(Envs.PublicHost, Envs.Port, env, Envs.Dev_Allowed_Origins), nil
	case "stage":
		return models.NewHostConfig(Envs.PublicHost, Envs.Port, env, Envs.Stag_Allowed_Origins), nil
	case "prod":
		return models.NewHostConfig(Envs.PublicHost, Envs.Port, env, Envs.Prod_Allowed_Origins), nil
	default:
		return nil, errors.New("unknown environment: " + env)
	}
}
