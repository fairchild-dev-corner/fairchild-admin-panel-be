package config

import (
	"strconv"
	"strings"

	models "fairchild-admin-panel-be/internal/models/config"
)

// GetMessagingConfig loads the channels used to message members (contact
// replies, unlock notices, blasts). Both are optional: a channel missing any
// required key is left nil. Blasts run in test mode unless env is "prod".
func GetMessagingConfig(env string) *models.MessagingConfig {
	conf := &models.MessagingConfig{
		Blast: models.BlastConfig{
			TestMode:     env != "prod",
			EmailsPerMin: positiveInt(Envs.BlastEmailsPerMinute, 30),
			SMSPerMin:    positiveInt(Envs.BlastSMSPerMinute, 60),
		},
	}
	for _, r := range strings.Split(Envs.BlastTestRecipients, ",") {
		if r = strings.TrimSpace(r); r != "" {
			conf.Blast.TestRecipients = append(conf.Blast.TestRecipients, r)
		}
	}

	host, user, pass, from := strings.TrimSpace(Envs.SMTPHost), Envs.SMTPUsername, Envs.SMTPPassword, strings.TrimSpace(Envs.SMTPFrom)
	if host != "" && user != "" && pass != "" && from != "" {
		port := strings.TrimSpace(Envs.SMTPPort)
		if port == "" {
			port = "587"
		}
		conf.SMTP = &models.SMTPConfig{Host: host, Port: port, Username: user, Password: pass, From: from}
	}

	if Envs.MoviderAPIKey != "" && Envs.MoviderAPISecret != "" {
		conf.Movider = &models.MoviderConfig{
			APIKey:     Envs.MoviderAPIKey,
			APISecret:  Envs.MoviderAPISecret,
			SenderName: strings.TrimSpace(Envs.MoviderSenderName),
		}
	}
	return conf
}

func positiveInt(raw string, fallback int) int {
	if n, err := strconv.Atoi(strings.TrimSpace(raw)); err == nil && n > 0 {
		return n
	}
	return fallback
}
