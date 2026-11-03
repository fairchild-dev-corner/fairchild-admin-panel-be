package services

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"sync"
	"time"

	config_models "fairchild-admin-panel-be/internal/models/config"
	models "fairchild-admin-panel-be/internal/models/health"
	"fairchild-admin-panel-be/pkg/database"
)

const (
	probeTimeout = 3 * time.Second
	// Anything slower than this still works but is flagged as degraded.
	degradedThreshold = 1 * time.Second
)

type probe struct {
	name  string
	check func(ctx context.Context) error
}

// HealthService probes the central Auth DB, every branch ledger DB and the
// configured HTTP services (HEALTH_CHECK_URLS) concurrently.
type HealthService struct {
	probes []probe
}

func NewHealthService(mysqlDB *sql.DB, branchDBs database.BranchDBs, targets []config_models.HealthCheckTarget) *HealthService {
	probes := []probe{{name: "Auth database", check: mysqlDB.PingContext}}

	for _, code := range branchDBs.Codes() {
		probes = append(probes, probe{
			name:  fmt.Sprintf("Ledger database (%s)", code),
			check: branchDBs[code].PingContext,
		})
	}

	client := &http.Client{Timeout: probeTimeout}
	for _, t := range targets {
		probes = append(probes, probe{name: t.Name, check: httpCheck(client, t.URL)})
	}

	return &HealthService{probes: probes}
}

// httpCheck treats any response below 500 as up - a 404 from an API's bare
// /api/v1/ path still proves the service is listening and routing.
func httpCheck(client *http.Client, url string) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return err
		}
		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		resp.Body.Close()
		if resp.StatusCode >= 500 {
			return fmt.Errorf("status %d", resp.StatusCode)
		}
		return nil
	}
}

func (s *HealthService) CheckAll(ctx context.Context) []models.ServiceHealth {
	results := make([]models.ServiceHealth, len(s.probes))

	var wg sync.WaitGroup
	for i, p := range s.probes {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i] = runProbe(ctx, p)
		}()
	}
	wg.Wait()

	return results
}

func runProbe(ctx context.Context, p probe) models.ServiceHealth {
	ctx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()

	start := time.Now()
	err := p.check(ctx)
	elapsed := time.Since(start)

	if err != nil {
		return models.ServiceHealth{Name: p.name, Status: models.HealthDown}
	}

	latency := elapsed.Milliseconds()
	status := models.HealthUp
	if elapsed > degradedThreshold {
		status = models.HealthDegraded
	}
	return models.ServiceHealth{Name: p.name, Status: status, LatencyMs: &latency}
}
