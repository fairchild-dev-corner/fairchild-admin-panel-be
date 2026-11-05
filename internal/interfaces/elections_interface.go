package interfaces

import (
	"context"

	audit_models "fairchild-admin-panel-be/internal/models/audit"
	models "fairchild-admin-panel-be/internal/models/elections"
)

// ElectionsRepositoryInterface covers this service's `elections`,
// `election_positions`, `election_candidates` and (read only)
// `election_votes`, plus reads of fairchild_be's `users` for candidates and
// of the voters' master list for eligibility counts.
type ElectionsRepositoryInterface interface {
	// Every election, newest voting start first, with its counts. Status is
	// filled by the caller.
	ListAll(ctx context.Context) ([]models.Election, []bool, error)
	// Returns nil, false, nil for an unknown id.
	FindByID(ctx context.Context, id int64) (*models.Election, bool, error)
	// Live portal accounts by member number (upper-cased keys).
	FindMembers(ctx context.Context, memberNos []string) (map[string]models.CandidateMember, error)

	Create(ctx context.Context, d *models.ElectionDraft, createdByID int64, createdBy string, audit *audit_models.AuditLog) (int64, error)
	// cc.ErrElectionLocked once a published election's voting has started.
	Update(ctx context.Context, id int64, d *models.ElectionDraft, audit *audit_models.AuditLog) error
	// Publish needs a draft; unpublish needs one whose voting hasn't started.
	SetPublished(ctx context.Context, id int64, publish bool, audit *audit_models.AuditLog) error
	// Drafts only.
	Delete(ctx context.Context, id int64, audit *audit_models.AuditLog) error
}
