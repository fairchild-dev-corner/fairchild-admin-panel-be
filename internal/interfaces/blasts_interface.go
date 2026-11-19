package interfaces

import (
	"context"

	audit_models "fairchild-admin-panel-be/internal/models/audit"
	models "fairchild-admin-panel-be/internal/models/blasts"
)

// BlastsRepositoryInterface covers this service's `message_blasts` and
// `message_blast_recipients`, plus the read of fairchild_be's `users` that
// picks a blast's audience.
type BlastsRepositoryInterface interface {
	// Live portal accounts with one of these statuses.
	ListAudience(ctx context.Context, statuses []string) ([]models.AudienceMember, error)
	// These live portal accounts, whatever their status.
	ListAudienceByIDs(ctx context.Context, userIDs []int64) ([]models.AudienceMember, error)
	// Live portal accounts matching a name, member ID, client ID or email.
	SearchAudience(ctx context.Context, search string, limit int) ([]models.AudienceMember, error)

	// Create stores the blast and its recipients and writes the audit entry
	// in one transaction. blast.ID is ignored.
	Create(ctx context.Context, blast *models.Blast, createdByID int64, recipients []models.NewRecipient, audit *audit_models.AuditLog) (int64, error)
	List(ctx context.Context, page, pageSize int) ([]models.Blast, int, error)
	// Returns nil, nil for an unknown id.
	FindByID(ctx context.Context, id int64) (*models.Blast, error)
	Failures(ctx context.Context, id int64, limit int) ([]models.BlastFailure, error)
	// One page of a blast's messages; status "pending" includes ones being sent.
	Recipients(ctx context.Context, blastID int64, status string, page, pageSize int) ([]models.BlastRecipient, int, error)
	// cc.ErrBlastNotSending unless the blast is still sending.
	Cancel(ctx context.Context, id int64, audit *audit_models.AuditLog) error
	// Removes a finished or cancelled blast and its messages;
	// cc.ErrBlastStillSending while it's sending.
	Delete(ctx context.Context, id int64, audit *audit_models.AuditLog) error
	// Removes every blast that isn't sending; returns how many.
	DeleteFinished(ctx context.Context, audit *audit_models.AuditLog) (int, error)
	InsertAudit(ctx context.Context, audit *audit_models.AuditLog) error

	// Worker side. Returns nil, nil when no blast is sending.
	NextSendingBlast(ctx context.Context) (*models.BlastJob, error)
	// Claims up to limit pending messages (and retries stale claims).
	ClaimBatch(ctx context.Context, blastID int64, limit int) ([]models.QueuedMessage, error)
	MarkResult(ctx context.Context, recipientID int64, status, errMsg string) error
	BlastStatus(ctx context.Context, blastID int64) (string, error)
	// Marks the blast completed once no message is pending or being sent.
	FinishIfDone(ctx context.Context, blastID int64) error
}

// BranchResolverInterface gives a portal account's ledger branch, matched
// the way the Portal page does.
type BranchResolverInterface interface {
	BranchResolver(ctx context.Context) (func(memberNo, clientID string) (string, bool), error)
}
