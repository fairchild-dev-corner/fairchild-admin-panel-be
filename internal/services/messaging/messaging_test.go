package messaging

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	models "fairchild-admin-panel-be/internal/models/config"
)

func TestNormalizeMobileNumber(t *testing.T) {
	for _, raw := range []string{"09171234567", "9171234567", "639171234567", "+63 917 123 4567"} {
		got, ok := NormalizeMobileNumber(raw)
		if !ok || got != "+639171234567" {
			t.Errorf("%q: %q, %v", raw, got, ok)
		}
	}
	for _, raw := range []string{"", "abc", "+0", "+1234567890123456"} {
		if _, ok := NormalizeMobileNumber(raw); ok {
			t.Errorf("accepted %q", raw)
		}
	}
}

func TestFriendlyFirstNameAndSMS(t *testing.T) {
	for raw, want := range map[string]string{"": "", "  ": "", "JOSEPH GLENN": "Joseph", "ÉLODIE": "Élodie"} {
		if got := FriendlyFirstName(raw); got != want {
			t.Errorf("%q: got %q want %q", raw, got, want)
		}
	}
	if !strings.HasPrefix(AccountUnlockedSMS("JOSEPH GLENN"), "Hi Joseph,") {
		t.Fatal("unlock greeting")
	}
	if !strings.HasPrefix(ClaimApprovedSMS(""), "Hi,") {
		t.Fatal("blank greeting")
	}
	if got := ClaimRejectedSMS("ANA", "  Missing document!!! "); !strings.Contains(got, "Reason: Missing document.") {
		t.Fatal(got)
	}
}

func TestSecurityEmailTemplatesEscapeInput(t *testing.T) {
	payload := "<script>alert('x')</script>"
	renderers := map[string]func() (string, error){
		"contact": func() (string, error) {
			return RenderContactReply(ContactReplyData{BrandName: "FORGED", Name: payload, Body: payload, Original: payload, ReceivedAt: payload})
		},
		"unlock": func() (string, error) {
			return RenderAccountUnlocked(AccountUnlockedData{BrandName: "FORGED", Name: payload, MemberNo: payload, UnlockedAt: payload})
		},
		"announcement": func() (string, error) {
			return RenderAnnouncement(AnnouncementData{BrandName: "FORGED", Subject: payload, Body: payload})
		},
		"approved": func() (string, error) {
			return RenderClaimApproved(ClaimApprovedData{BrandName: "FORGED", Name: payload, MemberNo: payload, Email: payload, Mobile: payload})
		},
		"rejected": func() (string, error) {
			return RenderClaimRejected(ClaimRejectedData{BrandName: "FORGED", Name: payload, MemberNo: payload, Reason: payload})
		},
	}
	for name, render := range renderers {
		t.Run(name, func(t *testing.T) {
			body, err := render()
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(body, "<script>") || !strings.Contains(body, "&lt;script&gt;") {
				t.Fatal("unsafe or missing escaped payload")
			}
			if strings.Contains(body, "FORGED") || !strings.Contains(body, BrandName) {
				t.Fatal("brand was overridden")
			}
		})
	}
}

func TestSecuritySMTPRejectsHeaderInjectionAndCancellation(t *testing.T) {
	// nil config intentionally proves validation happens before SMTP/network access.
	client := NewSMTPClient(nil)
	for _, tc := range []struct{ to, subject string }{
		{"member@example.com\r\nBcc: attacker@example.com", "subject"},
		{"member@example.com", "subject\nBcc: attacker@example.com"},
	} {
		if err := client.SendEmail(context.Background(), tc.to, tc.subject, "body"); err == nil {
			t.Fatal("header injection accepted")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := client.SendEmail(ctx, "member@example.com", "subject", "body"); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestMoviderResponses(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
		fail   bool
	}{
		{"sent", 200, "{}", false},
		{"rejected number", 200, `{"bad_phone_number_list":[{"number":"+639171234567","msg":"invalid"}]}`, true},
		{"provider error", 401, `{"error":{"code":401,"name":"unauthorized","description":"invalid credentials"}}`, true},
		{"invalid json", 200, "{", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := NewMoviderClient(&models.MoviderConfig{APIKey: "test-key", APISecret: "test-secret", SenderName: "FCCMPC"})
			c.httpClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if r.URL.String() != moviderSMSEndpoint || r.Method != http.MethodPost {
					t.Fatal("unexpected request")
				}
				data, err := io.ReadAll(r.Body)
				if err != nil {
					t.Fatal(err)
				}
				form, err := url.ParseQuery(string(data))
				if err != nil {
					t.Fatal(err)
				}
				if form.Get("to") != "+639171234567" || form.Get("text") != "hello & goodbye" || form.Get("from") != "FCCMPC" || form.Get("api_key") != "test-key" {
					t.Fatalf("bad form: %v", form)
				}
				return &http.Response{StatusCode: tc.status, Body: io.NopCloser(strings.NewReader(tc.body)), Header: make(http.Header)}, nil
			})
			err := c.SendSMS(context.Background(), "+639171234567", "hello & goodbye")
			if (err != nil) != tc.fail {
				t.Fatalf("error=%v want failure=%v", err, tc.fail)
			}
		})
	}
}

func BenchmarkMessaging(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		if _, err := RenderAnnouncement(AnnouncementData{Subject: "Member update", Body: strings.Repeat("Hello member. ", 20)}); err != nil {
			b.Fatal(err)
		}
	}
}

func FuzzNormalizeMobileNumber(f *testing.F) {
	for _, s := range []string{"09171234567", "+639171234567", "", "+0", "\r\n", "é"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, raw string) {
		got, ok := NormalizeMobileNumber(raw)
		if ok && !e164Pattern.MatchString(got) {
			t.Fatalf("invalid E.164: %q", got)
		}
		if ok {
			again, valid := NormalizeMobileNumber(got)
			if !valid || again != got {
				t.Fatalf("normalization not stable: %q", got)
			}
		}
	})
}
