package models

/**
* MySQL Database Configuration
* @Description: Holds connection parameters, mapped onto the driver's
* mysql.Config{} by pkg/database.MySQLInitConnector.
**/
type MySQLConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
	BuildEnv string
	// CACert is the PEM-encoded CA certificate used to verify the server's
	// TLS certificate. Empty means plaintext TCP (local dev only).
	CACert string
}

func NewMySQLConfig(host, port, user, password, dbName, buildEnv, caCert string) *MySQLConfig {
	return &MySQLConfig{
		Host:     host,
		Port:     port,
		User:     user,
		Password: password,
		DBName:   dbName,
		BuildEnv: buildEnv,
		CACert:   caCert,
	}
}

/**
* JWT Configuration
* @Description: Signing secret and lifetimes for panel access tokens and
* refresh sessions. Refresh tokens are opaque (only their hash is stored), so
* they need no signing secret.
**/
type JWTConfig struct {
	AccessSecret     string
	AccessTTLMinutes int
	RefreshTTLHours  int
}

func NewJWTConfig(accessSecret string, accessTTLMinutes, refreshTTLHours int) *JWTConfig {
	return &JWTConfig{
		AccessSecret:     accessSecret,
		AccessTTLMinutes: accessTTLMinutes,
		RefreshTTLHours:  refreshTTLHours,
	}
}

// HealthCheckTarget is an external HTTP service shown on the health page.
type HealthCheckTarget struct {
	Name string
	URL  string
}

// SMTPConfig is the mailbox contact message replies are sent from.
type SMTPConfig struct {
	Host     string
	Port     string
	Username string
	Password string
	From     string
}

// MoviderConfig is the Movider SMS account (fairchild_be's OTP gateway)
// contact message replies are texted from. SenderName is optional.
type MoviderConfig struct {
	APIKey     string
	APISecret  string
	SenderName string
}

// MessagingConfig holds the optional outbound channels; a nil one is
// disabled.
type MessagingConfig struct {
	SMTP    *SMTPConfig
	Movider *MoviderConfig
	Blast   BlastConfig
}

// BlastConfig paces SMS/email blasts. In test mode (any build but prod)
// only TestRecipients (emails and mobile numbers, as configured) are
// actually messaged; everyone else is recorded as skipped.
type BlastConfig struct {
	TestMode       bool
	TestRecipients []string
	EmailsPerMin   int
	SMSPerMin      int
}
