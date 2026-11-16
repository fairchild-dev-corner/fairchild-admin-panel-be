package services

import (
	"context"
	models "fairchild-admin-panel-be/internal/models/members"
	"testing"
)

func BenchmarkMembers(b *testing.B) {
	s := NewMembersService(fakePanel{perBranch: map[string]int{"main": 100}}, fakeLedger{perBranch: map[string]int{"main": 100, "tiic": 100}})
	q := models.MemberListQuery{Category: "regular", Page: 1, PageSize: 15}
	b.ReportAllocs()
	for b.Loop() {
		if _, err := s.ListMembers(context.Background(), q); err != nil {
			b.Fatal(err)
		}
	}
}
