package services

import (
	"context"
	"errors"
	cc "fairchild-admin-panel-be/internal/constants"
	"fairchild-admin-panel-be/internal/interfaces"
	audit "fairchild-admin-panel-be/internal/models/audit"
	models "fairchild-admin-panel-be/internal/models/claims"
	"testing"
)

type claimRepo struct {
	interfaces.ClaimsRepositoryInterface
	claim *models.ClaimRequest
	err   error
}

func (r claimRepo) FindByID(context.Context, int64) (*models.ClaimRequest, error) {
	return r.claim, r.err
}
func TestSecurityClaimApprovalGuards(t *testing.T) {
	for _, tc := range []struct {
		name  string
		claim models.ClaimRequest
		want  error
	}{
		{"reviewed", models.ClaimRequest{Status: cc.APPLICATION_STATUS_APPROVED}, cc.ErrClaimAlreadyReviewed},
		{"no account", models.ClaimRequest{Status: cc.APPLICATION_STATUS_PENDING}, cc.ErrClaimNoPortalAccount},
		{"bad contact", models.ClaimRequest{Status: cc.APPLICATION_STATUS_PENDING, Account: &models.ClaimAccount{}, ProposedEmail: "bad", ProposedMobileNumber: "123"}, cc.ErrClaimInvalidContact},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := NewClaimsService(claimRepo{claim: &tc.claim}, nil, nil)
			if _, err := s.Approve(context.Background(), "CR-00001", audit.Actor{}); !errors.Is(err, tc.want) {
				t.Fatalf("got %v want %v", err, tc.want)
			}
		})
	}
}
func TestClaimGetErrors(t *testing.T) {
	failure := errors.New("query failed")
	for _, tc := range []struct {
		repo claimRepo
		id   string
		want error
	}{
		{claimRepo{}, "bad", cc.ErrClaimNotFound}, {claimRepo{}, "1", cc.ErrClaimNotFound}, {claimRepo{err: failure}, "1", failure},
	} {
		if _, err := NewClaimsService(tc.repo, nil, nil).Get(context.Background(), tc.id); !errors.Is(err, tc.want) {
			t.Fatal(err)
		}
	}
}
func BenchmarkClaims(b *testing.B) {
	s := NewClaimsService(claimRepo{claim: &models.ClaimRequest{}}, nil, nil)
	b.ReportAllocs()
	for b.Loop() {
		if _, err := s.Get(context.Background(), "CR-00001"); err != nil {
			b.Fatal(err)
		}
	}
}
