package services

import (
	"context"
	cc "fairchild-admin-panel-be/internal/constants"
	"testing"
)

func BenchmarkCapabilities(b *testing.B) {
	s, _ := newTestService(map[int64][]string{4: {"members.view"}})
	u := panelUser(4, cc.ROLE_STAFF)
	b.ReportAllocs()
	for b.Loop() {
		ok, err := s.Allows(context.Background(), u, "members.view")
		if err != nil || !ok {
			b.Fatal(ok, err)
		}
	}
}
