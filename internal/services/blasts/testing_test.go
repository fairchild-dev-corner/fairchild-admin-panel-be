package services

import (
	models "fairchild-admin-panel-be/internal/models/blasts"
	config "fairchild-admin-panel-be/internal/models/config"
	"fmt"
	"strings"
	"testing"
)

func TestSecurityBlastRecipientsDeduplicateAndRespectTestMode(t *testing.T) {
	s := &BlastsService{conf: config.BlastConfig{TestMode: true}, allowed: map[string]bool{"member@example.com": true}}
	members := []models.AudienceMember{
		{Email: "MEMBER@example.com", Mobile: "09171234567"},
		{Email: "member@example.com", Mobile: "+639171234567"},
		{Email: "invalid", Mobile: "invalid"},
	}
	got, unreachable := s.recipients(members, models.ChannelBoth)
	if len(got) != 2 || unreachable != 1 {
		t.Fatalf("%+v unreachable=%d", got, unreachable)
	}
	for _, r := range got {
		if r.Channel == models.ChannelEmail && r.Skip {
			t.Fatal("allowlisted email skipped")
		}
		if r.Channel == models.ChannelSMS && !r.Skip {
			t.Fatal("non-allowlisted SMS would send")
		}
	}
}
func TestBlastContentValidation(t *testing.T) {
	for _, tc := range []struct {
		channel, subject, email, sms string
		fail                         bool
	}{
		{models.ChannelBoth, "Subject", "Email", "Text", false},
		{models.ChannelEmail, "", "Email", "", true},
		{models.ChannelSMS, "", "", "", true},
		{models.ChannelSMS, "", "", strings.Repeat("x", models.MaxSMSLength+1), true},
	} {
		err := checkContent(tc.channel, &tc.subject, &tc.email, &tc.sms)
		if (err != nil) != tc.fail {
			t.Fatalf("error=%v want fail=%v", err, tc.fail)
		}
	}
}
func BenchmarkBlasts(b *testing.B) {
	for _, n := range []int{100, 10000} {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			members := make([]models.AudienceMember, n)
			for i := range members {
				members[i] = models.AudienceMember{Email: fmt.Sprintf("member%d@example.com", i)}
			}
			s := &BlastsService{}
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				got, _ := s.recipients(members, models.ChannelEmail)
				if len(got) != n {
					b.Fatal(len(got))
				}
			}
		})
	}
}
