// Package messaging sends contact message replies over the same providers
// fairchild_be uses for OTPs: SMTP for email, Movider for SMS. The clients
// mirror fairchild_be's internal/services/mail and internal/services/sms.
package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"mime"
	"net/http"
	"net/smtp"
	"net/url"
	"regexp"
	"strings"
	"time"

	models "fairchild-admin-panel-be/internal/models/config"
)

// EmailSender delivers an HTML email.
type EmailSender interface {
	SendEmail(ctx context.Context, to, subject, htmlBody string) error
}

// SMSSender delivers a text message to an E.164 number.
type SMSSender interface {
	SendSMS(ctx context.Context, to, text string) error
}

/**
* SMTPClient
* @Description: net/smtp has no context support - ctx is only checked up
* front so an already-cancelled request doesn't still send. smtp.SendMail
* upgrades to STARTTLS itself when the server offers it (smtp.gmail.com:587
* does); implicit-TLS port 465 isn't supported.
**/
type SMTPClient struct {
	conf *models.SMTPConfig
}

func NewSMTPClient(conf *models.SMTPConfig) *SMTPClient {
	return &SMTPClient{conf: conf}
}

func (c *SMTPClient) SendEmail(ctx context.Context, to, subject, htmlBody string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	// Header values come from the DB and the server; refuse anything that
	// could inject extra headers.
	if strings.ContainsAny(to+subject, "\r\n") {
		return fmt.Errorf("smtp: invalid header value")
	}

	var msg strings.Builder
	fmt.Fprintf(&msg, "From: %s\r\n", c.conf.From)
	fmt.Fprintf(&msg, "To: %s\r\n", to)
	fmt.Fprintf(&msg, "Subject: %s\r\n", mime.QEncoding.Encode("UTF-8", subject))
	msg.WriteString("MIME-Version: 1.0\r\n")
	msg.WriteString("Content-Type: text/html; charset=\"UTF-8\"\r\n\r\n")
	msg.WriteString(htmlBody)

	addr := fmt.Sprintf("%s:%s", c.conf.Host, c.conf.Port)
	auth := smtp.PlainAuth("", c.conf.Username, c.conf.Password, c.conf.Host)
	return smtp.SendMail(addr, auth, envelopeAddress(c.conf.From), []string{to}, []byte(msg.String()))
}

// envelopeAddress takes the bare address from a From like
// "Fairchild Coop <no-reply@example.com>".
func envelopeAddress(from string) string {
	if i, j := strings.LastIndex(from, "<"), strings.LastIndex(from, ">"); i >= 0 && j > i {
		return from[i+1 : j]
	}
	return from
}

const moviderSMSEndpoint = "https://api.movider.co/v1/sms"

/**
* MoviderClient
* @Description: sends SMS via the Movider API
* (https://developer.movider.co/reference/post_sms).
**/
type MoviderClient struct {
	conf       *models.MoviderConfig
	httpClient *http.Client
}

func NewMoviderClient(conf *models.MoviderConfig) *MoviderClient {
	return &MoviderClient{conf: conf, httpClient: &http.Client{Timeout: 10 * time.Second}}
}

// Only bad_phone_number_list is decoded from a success: the other fields'
// JSON types aren't pinned down by Movider's docs.
type moviderSuccessResponse struct {
	BadPhoneNumberList []struct {
		Number string `json:"number"`
		Msg    string `json:"msg"`
	} `json:"bad_phone_number_list"`
}

type moviderErrorResponse struct {
	Error struct {
		Code        int    `json:"code"`
		Name        string `json:"name"`
		Description string `json:"description"`
	} `json:"error"`
}

func (m *MoviderClient) SendSMS(ctx context.Context, to, text string) error {
	form := url.Values{
		"api_key":    {m.conf.APIKey},
		"api_secret": {m.conf.APISecret},
		"to":         {to},
		"text":       {text},
	}
	if m.conf.SenderName != "" {
		form.Set("from", m.conf.SenderName)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, moviderSMSEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp moviderErrorResponse
		_ = json.NewDecoder(resp.Body).Decode(&errResp)
		return fmt.Errorf("movider: %s (code %d): %s", errResp.Error.Name, errResp.Error.Code, errResp.Error.Description)
	}

	var okResp moviderSuccessResponse
	if err := json.NewDecoder(resp.Body).Decode(&okResp); err != nil {
		return err
	}
	if len(okResp.BadPhoneNumberList) > 0 {
		return fmt.Errorf("movider: rejected number %s: %s", okResp.BadPhoneNumberList[0].Number, okResp.BadPhoneNumberList[0].Msg)
	}
	return nil
}

var e164Pattern = regexp.MustCompile(`^\+[1-9]\d{1,14}$`)

// NormalizeMobileNumber converts Philippine local formats ("09171234567",
// "9171234567", "639171234567") into E.164 ("+639171234567"), which Movider
// requires - the same rules as fairchild_be's NormalizeMobileNumber. ok is
// false for anything else.
func NormalizeMobileNumber(raw string) (string, bool) {
	var b strings.Builder
	for _, r := range raw {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	digits := b.String()

	switch {
	case strings.HasPrefix(strings.TrimSpace(raw), "+"):
		return "+" + digits, e164Pattern.MatchString("+" + digits)
	case strings.HasPrefix(digits, "63") && len(digits) == 12:
		return "+" + digits, true
	case strings.HasPrefix(digits, "0") && len(digits) == 11:
		return "+63" + digits[1:], true
	case len(digits) == 10:
		return "+63" + digits, true
	default:
		return "", false
	}
}
