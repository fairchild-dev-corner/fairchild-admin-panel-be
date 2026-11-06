package interfaces

import (
	"context"

	audit_models "fairchild-admin-panel-be/internal/models/audit"
	member_models "fairchild-admin-panel-be/internal/models/members"
	models "fairchild-admin-panel-be/internal/models/portal"
)

// PortalRepositoryInterface covers fairchild_be's `users` table (members'
// portal accounts). The only writes are adding an account for a ledger
// member, the contact update, the status change and the soft delete.
type PortalRepositoryInterface interface {
	// Every non-deleted account matching search, newest first, as light
	// rows the service filters by branch, counts and pages.
	ListKeys(ctx context.Context, search string) ([]models.PortalKey, error)
	// Full rows for ids (deleted ones skipped), in no particular order.
	FindByIDs(ctx context.Context, ids []int64) ([]models.PortalMember, error)
	// Returns nil, nil for an unknown or deleted id.
	FindByID(ctx context.Context, id int64) (*models.PortalMember, error)
	// Whether any account (deleted ones included) uses this member number.
	MemberNoTaken(ctx context.Context, memberNo string) (bool, error)
	// Inserts the `users` row and a password-less `user_account_cred` row;
	// returns the new id and sets audit.Target. cc.ErrPortalAccountExists /
	// cc.ErrPortalEmailTaken on a unique-key clash.
	Create(ctx context.Context, member *member_models.Member, req *models.AddPortalMemberRequest, memberType string, audit *audit_models.AuditLog) (int64, error)
	// UpdateContact, UpdateStatus and Remove return cc.ErrPortalMemberNotFound when no
	// live account has that id.
	UpdateContact(ctx context.Context, id int64, req *models.ContactUpdateRequest, audit *audit_models.AuditLog) error
	// Revokes the account's sessions when the new status is suspended, so
	// the member is signed out on their next refresh.
	UpdateStatus(ctx context.Context, id int64, status string, audit *audit_models.AuditLog) error
	Remove(ctx context.Context, id int64, audit *audit_models.AuditLog) error
}

// LedgerLookupInterface reads member numbers from the branch ledgers.
type LedgerLookupInterface interface {
	Branches() []string
	// Every member number on one branch ledger.
	LoadMemberNos(ctx context.Context, branch string) ([]models.LedgerLink, error)
	// The current member with that member number on one branch, or nil, nil.
	FindByMemberNo(ctx context.Context, branch, memberNo string) (*member_models.Member, error)
}
