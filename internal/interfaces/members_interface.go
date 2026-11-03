package interfaces

import (
	"context"

	audit_models "fairchild-admin-panel-be/internal/models/audit"
	models "fairchild-admin-panel-be/internal/models/members"
)

// MembersRepositoryInterface covers admin_members, the members added from
// the panel.
type MembersRepositoryInterface interface {
	// search "" means no search; otherwise it matches name, email, mobile or id.
	CountMembers(ctx context.Context, search string) (models.BranchCategoryCounts, error)
	// Newest first, restricted to one category and the given branches.
	ListMembers(ctx context.Context, category string, branches []string, search string, offset, limit int) ([]models.Member, error)
	// Returns nil, nil for an unknown or deleted id.
	FindMemberByID(ctx context.Context, id int64) (*models.Member, error)
	// Returns the new row id; sets audit.Target once it's known.
	CreateMember(ctx context.Context, req *models.MemberRequest, actorID int64, audit *audit_models.AuditLog) (int64, error)
	// UpdateMember and DeleteMember return cc.ErrMemberNotFound when no live
	// row has that id.
	UpdateMember(ctx context.Context, id int64, req *models.MemberRequest, actorID int64, audit *audit_models.AuditLog) error
	DeleteMember(ctx context.Context, id int64, actorID int64, audit *audit_models.AuditLog) error
}

// LedgerMembersRepositoryInterface reads members from the branch ledger DBs.
// Read-only: those rows belong to the accounting system.
type LedgerMembersRepositoryInterface interface {
	// Branches lists the branches whose ledger DB is connected.
	Branches() []string
	// category -> count for one branch.
	CountMembers(ctx context.Context, branch, search string) (map[string]int, error)
	// Alphabetical by surname.
	ListMembers(ctx context.Context, branch, category, search string, offset, limit int) ([]models.Member, error)
	// Returns nil, nil when the client isn't a current member.
	FindMember(ctx context.Context, branch string, clientID int64) (*models.Member, error)
}
