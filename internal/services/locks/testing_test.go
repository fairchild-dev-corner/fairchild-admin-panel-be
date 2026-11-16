package services

import (
	"context"
	"errors"
	cc "fairchild-admin-panel-be/internal/constants"
	"fairchild-admin-panel-be/internal/interfaces"
	audit "fairchild-admin-panel-be/internal/models/audit"
	models "fairchild-admin-panel-be/internal/models/locks"
	"testing"
)

type unlockRepo struct {
	interfaces.LocksRepositoryInterface
	id  int64
	a   *audit.AuditLog
	err error
}

func (r *unlockRepo) Unlock(_ context.Context, id int64, a *audit.AuditLog) error {
	r.id, r.a = id, a
	return r.err
}
func (r *unlockRepo) FindByID(context.Context, int64) (*models.LockedAccount, error) {
	return &models.LockedAccount{}, nil
}
func TestUnlockAuditsAndHandlesErrors(t *testing.T) {
	r := &unlockRepo{}
	s := NewLocksService(r, nil, nil)
	got, err := s.Unlock(context.Background(), "LK-00042", audit.Actor{UserPanelID: 7})
	if err != nil || got.Lock == nil || r.id != 42 || r.a == nil || r.a.Action != cc.AUDIT_UNLOCKED_MEMBER_ACCOUNT {
		t.Fatalf("bad unlock: %+v %v", r, err)
	}
	if _, err := s.Unlock(context.Background(), "bad", audit.Actor{}); !errors.Is(err, cc.ErrLockedAccountNotFound) {
		t.Fatal(err)
	}
	failure := errors.New("transaction failed")
	r.err = failure
	if _, err := s.Unlock(context.Background(), "42", audit.Actor{}); !errors.Is(err, failure) {
		t.Fatal(err)
	}
}
func BenchmarkLocks(b *testing.B) {
	s := NewLocksService(&unlockRepo{}, nil, nil)
	b.ReportAllocs()
	for b.Loop() {
		if _, err := s.Unlock(context.Background(), "LK-00042", audit.Actor{UserPanelID: 7}); err != nil {
			b.Fatal(err)
		}
	}
}
