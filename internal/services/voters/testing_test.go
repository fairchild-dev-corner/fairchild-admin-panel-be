package services

import (
	"context"
	"errors"
	cc "fairchild-admin-panel-be/internal/constants"
	"fairchild-admin-panel-be/internal/interfaces"
	audit "fairchild-admin-panel-be/internal/models/audit"
	models "fairchild-admin-panel-be/internal/models/voters"
	"fmt"
	"strings"
	"testing"
)

type electionRepo struct {
	interfaces.VotersRepositoryInterface
	enabled bool
}

func (r electionRepo) GetElectionControl(context.Context) (*models.ElectionControl, error) {
	return &models.ElectionControl{Enabled: r.enabled}, nil
}
func TestSecurityElectionOffBlocksMutations(t *testing.T) {
	s := NewVotersService(electionRepo{}, nil, nil)
	if _, err := s.CheckMasterList(context.Background(), "list.csv", strings.NewReader("")); !errors.Is(err, cc.ErrElectionOff) {
		t.Fatal(err)
	}
	if _, err := s.SaveMasterList(context.Background(), &models.SaveMasterListRequest{}, audit.Actor{}); !errors.Is(err, cc.ErrElectionOff) {
		t.Fatal(err)
	}
	if _, err := s.SetVoting(context.Background(), "1", models.SetVotingRequest{Voting: &[]bool{true}[0]}, audit.Actor{}); !errors.Is(err, cc.ErrElectionOff) {
		t.Fatal(err)
	}
}
func TestSecurityMasterListUploadLimit(t *testing.T) {
	s := NewVotersService(electionRepo{enabled: true}, nil, nil)
	_, err := s.CheckMasterList(context.Background(), "list.csv", strings.NewReader(strings.Repeat("x", models.MaxMasterListBytes+1)))
	var inputErr *cc.MasterListInputError
	if !errors.As(err, &inputErr) {
		t.Fatalf("expected input error, got %v", err)
	}
}
func BenchmarkVoters(b *testing.B) {
	for _, n := range []int{100, 10000} {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			var csv strings.Builder
			csv.WriteString("Member ID,Name,Category\n")
			for i := range n {
				fmt.Fprintf(&csv, "%06d,Member %d,voting\n", i, i)
			}
			data := []byte(csv.String())
			b.SetBytes(int64(len(data)))
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				rows, _, _, err := parseRows("list.csv", data)
				if err != nil || len(rows) != n {
					b.Fatalf("rows=%d err=%v", len(rows), err)
				}
			}
		})
	}
}

type setVotingRepo struct {
	electionRepo
	category string
	reasons  []string
	action   string
}

func (r *setVotingRepo) SetVoting(_ context.Context, _ int64, category string, reasons []string, a *audit.AuditLog) error {
	r.category, r.reasons, r.action = category, reasons, a.Action
	return nil
}
func (r *setVotingRepo) Summary(context.Context) (*models.MasterListSummary, error) {
	return &models.MasterListSummary{}, nil
}

func TestSetVotingIneligibleNeedsReason(t *testing.T) {
	repo := &setVotingRepo{electionRepo: electionRepo{enabled: true}}
	s := NewVotersService(repo, nil, nil)
	ctx := context.Background()

	var inputErr *cc.MasterListInputError
	if _, err := s.SetVoting(ctx, "PU-00001", models.SetVotingRequest{Category: models.CategoryIneligible, Reasons: []string{" "}}, audit.Actor{}); !errors.As(err, &inputErr) {
		t.Fatalf("want an input error without a reason, got %v", err)
	}
	if _, err := s.SetVoting(ctx, "PU-00001", models.SetVotingRequest{
		Category: models.CategoryIneligible, Reasons: []string{"Have submitted the TIN", "have submitted the  TIN"},
	}, audit.Actor{}); err != nil {
		t.Fatal(err)
	}
	if repo.category != models.CategoryIneligible || len(repo.reasons) != 1 || repo.action != cc.AUDIT_SET_MEMBER_INELIGIBLE {
		t.Errorf("repo = %+v", repo)
	}
	// Voting drops the reasons.
	if _, err := s.SetVoting(ctx, "PU-00001", models.SetVotingRequest{Category: models.CategoryVoting, Reasons: []string{"x"}}, audit.Actor{}); err != nil {
		t.Fatal(err)
	}
	if repo.category != models.CategoryVoting || repo.reasons != nil {
		t.Errorf("repo = %+v", repo)
	}
}
