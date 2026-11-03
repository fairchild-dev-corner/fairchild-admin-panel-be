package models

type HostConfig struct {
	Host           string
	Port           string
	BuildEnv       string
	AllowedOrigins string
}

func NewHostConfig(host string, port string, buildEnv string, origins string) *HostConfig {
	return &HostConfig{
		Host:           host,
		Port:           port,
		BuildEnv:       buildEnv,
		AllowedOrigins: origins,
	}
}
