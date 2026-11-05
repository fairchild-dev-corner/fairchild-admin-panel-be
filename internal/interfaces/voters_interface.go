package interfaces

import (
	"context"

	audit_models "fairchild-admin-panel-be/internal/models/audit"
	models "fairchild-admin-panel-be/internal/models/voters"
)

// VotersRepositoryInterface covers this service's `voter_master_list` and
// `voter_master_list_uploads`, joined to fairchild_be's `users` (read only).
type VotersRepositoryInterface interface {
	// Every live portal account with a member number.
	LoadAccounts(ctx context.Context) ([]models.AccountRef, error)
	Summary(ctx context.Context) (*models.MasterListSummary, error)
	// Replaces the whole list and logs the upload.
	ReplaceMasterList(ctx context.Context, fileName string, members []models.MasterListMember, byID int64, by string, audit *audit_models.AuditLog) error
	// Members who can and can't vote (and all), matching search.
	CountVoters(ctx context.Context, search string) (map[string]int, error)
	ListVoters(ctx context.Context, q models.VoterListQuery) ([]models.Voter, error)
}
