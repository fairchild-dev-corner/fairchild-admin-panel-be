package models

const (
	HealthUp       = "up"
	HealthDegraded = "degraded"
	HealthDown     = "down"
)

// ServiceHealth matches ServiceHealth in the admin panel (src/models/admin.ts).
type ServiceHealth struct {
	Name      string `json:"name"`
	Status    string `json:"status"`
	LatencyMs *int64 `json:"latencyMs"`
}
