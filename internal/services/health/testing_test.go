package services

import (
	"context"
	"errors"
	models "fairchild-admin-panel-be/internal/models/health"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthProbeResults(t *testing.T) {
	s := &HealthService{probes: []probe{
		{name: "up", check: func(ctx context.Context) error {
			if _, ok := ctx.Deadline(); !ok {
				t.Error("probe has no deadline")
			}
			return nil
		}},
		{name: "down", check: func(context.Context) error { return errors.New("secret connection string") }},
	}}
	got := s.CheckAll(context.Background())
	if len(got) != 2 || got[0].Name != "up" || got[0].Status != models.HealthUp || got[0].LatencyMs == nil || got[1].Status != models.HealthDown || got[1].LatencyMs != nil {
		t.Fatalf("%+v", got)
	}
}
func TestHTTPHealthStatusAndCancellation(t *testing.T) {
	for _, status := range []int{200, 404, 500, 503} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(status) }))
		err := httpCheck(server.Client(), server.URL)(context.Background())
		server.Close()
		if (err != nil) != (status >= 500) {
			t.Fatalf("%d: %v", status, err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := runProbe(ctx, probe{name: "canceled", check: func(ctx context.Context) error { return ctx.Err() }})
	if err.Status != models.HealthDown {
		t.Fatal(err)
	}
}
func BenchmarkHealth(b *testing.B) {
	s := &HealthService{probes: []probe{{name: "local", check: func(context.Context) error { return nil }}}}
	b.ReportAllocs()
	for b.Loop() {
		if got := s.CheckAll(context.Background()); len(got) != 1 {
			b.Fatal(got)
		}
	}
}
