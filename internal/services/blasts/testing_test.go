package services

import (
	"context"
	interfaces "fairchild-admin-panel-be/internal/interfaces"
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

type audienceRepo struct {
	interfaces.BlastsRepositoryInterface
	members []models.AudienceMember
	byIDs   []int64
}

func (r *audienceRepo) ListAudience(context.Context, []string) ([]models.AudienceMember, error) {
	return r.members, nil
}
func (r *audienceRepo) ListAudienceByIDs(_ context.Context, ids []int64) ([]models.AudienceMember, error) {
	r.byIDs = ids
	return r.members[:1], nil
}

type noBranches struct{}

func (noBranches) BranchResolver(context.Context) (func(string, string) (string, bool), error) {
	return func(string, string) (string, bool) { return "", false }, nil
}

type fakeEmail struct{}

func (fakeEmail) SendEmail(context.Context, string, string, string) error { return nil }

func TestAudienceGroups(t *testing.T) {
	repo := &audienceRepo{members: []models.AudienceMember{
		{UserID: 1, Category: models.GroupVoting},
		{UserID: 2, Category: models.GroupNonVoting},
		{UserID: 3, Category: models.GroupIneligible, Delinquent: true},
		{UserID: 4, Category: models.GroupIneligible},
		{UserID: 5}, // not on the master list
	}}
	s := &BlastsService{repo: repo, branches: noBranches{}}
	ids := func(ms []models.AudienceMember) (out []int64) {
		for _, m := range ms {
			out = append(out, m.UserID)
		}
		return out
	}

	all, _, counts, err := s.audience(context.Background(), models.AudienceRequest{Statuses: []string{"active"}})
	if err != nil || len(all) != 5 {
		t.Fatalf("no groups: %v %v", ids(all), err)
	}
	if counts[models.GroupVoting] != 1 || counts[models.GroupIneligible] != 2 || counts[models.GroupDelinquent] != 1 {
		t.Errorf("group counts = %v", counts)
	}

	got, _, _, _ := s.audience(context.Background(), models.AudienceRequest{
		Statuses: []string{"active"}, Groups: []string{models.GroupNonVoting, models.GroupDelinquent},
	})
	if fmt.Sprint(ids(got)) != "[2 3]" {
		t.Errorf("non-voting or delinquent = %v", ids(got))
	}

	req := models.AudienceRequest{Channel: models.ChannelEmail, MemberIDs: []string{"PU-00001", "PU-00001"}, Groups: []string{"voting"}}
	s.email = fakeEmail{}
	if err := s.checkAudience(&req); err != nil || len(req.MemberIDs) != 1 || len(req.Groups) != 0 || len(req.Statuses) != 0 {
		t.Fatalf("picked: %+v %v", req, err)
	}
	if _, _, _, err := s.audience(context.Background(), req); err != nil || fmt.Sprint(repo.byIDs) != "[1]" {
		t.Errorf("picked ids = %v %v", repo.byIDs, err)
	}

	bad := models.AudienceRequest{Channel: models.ChannelEmail, Statuses: []string{"active"}, Groups: []string{"vip"}}
	if err := s.checkAudience(&bad); err == nil {
		t.Error("unknown group accepted")
	}
	bad = models.AudienceRequest{Channel: models.ChannelEmail, MemberIDs: []string{"nope"}}
	if err := s.checkAudience(&bad); err == nil {
		t.Error("bad member id accepted")
	}
}
