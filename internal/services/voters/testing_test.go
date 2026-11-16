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
	if _, err := s.SetVoting(context.Background(), "1", true, audit.Actor{}); !errors.Is(err, cc.ErrElectionOff) {
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
				rows, _, _, err := parseMasterList("list.csv", data)
				if err != nil || len(rows) != n {
					b.Fatalf("rows=%d err=%v", len(rows), err)
				}
			}
		})
	}
}
