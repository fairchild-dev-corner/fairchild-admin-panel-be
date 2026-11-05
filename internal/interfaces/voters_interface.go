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
	// Every member's voting status with their ledger keys, for per-branch counts.
	VoterStatuses(ctx context.Context) ([]models.VoterStatus, error)
	// The election on/off switch (`election_control`, a single row).
	GetElectionControl(ctx context.Context) (*models.ElectionControl, error)
	SetElectionControl(ctx context.Context, enabled bool, byID int64, by string, audit *audit_models.AuditLog) error
	// Empties the master list (everyone non-voting); returns how many were on it.
	ClearMasterList(ctx context.Context, audit *audit_models.AuditLog) (int64, error)
}

// PasswordConfirmerInterface re-checks a signed-in panel user's own password
// (implemented by the auth service, which applies the login lockout).
type PasswordConfirmerInterface interface {
	ConfirmPassword(ctx context.Context, userID int64, password, ip, userAgent string) error
}
