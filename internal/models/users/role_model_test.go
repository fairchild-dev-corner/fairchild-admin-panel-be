package models

import (
	"testing"

	cc "fairchild-admin-panel-be/internal/constants"
)

func TestCanAssign(t *testing.T) {
	tests := []struct {
		actor, role string
		want        bool
	}{
		{cc.ROLE_SUPER_ADMIN, cc.ROLE_SUPER_ADMIN, true},
		{cc.ROLE_SUPER_ADMIN, cc.ROLE_ADMIN, true},
		{cc.ROLE_SUPER_ADMIN, cc.ROLE_STAFF, true},
		{cc.ROLE_ADMIN, cc.ROLE_SUPER_ADMIN, false},
		{cc.ROLE_ADMIN, cc.ROLE_ADMIN, false},
		{cc.ROLE_ADMIN, cc.ROLE_MANAGER, true},
		{cc.ROLE_ADMIN, cc.ROLE_STAFF, true},
		{cc.ROLE_MANAGER, cc.ROLE_STAFF, false}, // only admins manage accounts
		{cc.ROLE_SUPERVISOR, cc.ROLE_STAFF, false},
		{cc.ROLE_SUPER_ADMIN, "member", false},
	}
	for _, tt := range tests {
		if got := CanAssign(tt.actor, tt.role); got != tt.want {
			t.Errorf("CanAssign(%s, %s) = %v, want %v", tt.actor, tt.role, got, tt.want)
		}
	}
}

func TestCanManage(t *testing.T) {
	if CanManage(1, cc.ROLE_SUPER_ADMIN, 1, cc.ROLE_SUPER_ADMIN) {
		t.Error("no one manages their own account")
	}
	if !CanManage(1, cc.ROLE_SUPER_ADMIN, 2, cc.ROLE_SUPER_ADMIN) {
		t.Error("a super admin manages another super admin")
	}
	if CanManage(1, cc.ROLE_ADMIN, 2, cc.ROLE_ADMIN) {
		t.Error("an admin doesn't manage another admin")
	}
	if !CanManage(1, cc.ROLE_ADMIN, 2, cc.ROLE_SUPERVISOR) {
		t.Error("an admin manages a supervisor")
	}
}

func TestRank(t *testing.T) {
	if !(Rank(cc.ROLE_SUPER_ADMIN) > Rank(cc.ROLE_ADMIN) && Rank(cc.ROLE_ADMIN) > Rank(cc.ROLE_MANAGER) &&
		Rank(cc.ROLE_MANAGER) > Rank(cc.ROLE_SUPERVISOR) && Rank(cc.ROLE_SUPERVISOR) > Rank(cc.ROLE_STAFF) &&
		Rank(cc.ROLE_STAFF) > Rank("member")) {
		t.Error("ranks out of order")
	}
}
