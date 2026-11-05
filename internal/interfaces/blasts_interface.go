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

	// Create stores the blast and its recipients and writes the audit entry
	// in one transaction. blast.ID is ignored.
	Create(ctx context.Context, blast *models.Blast, createdByID int64, recipients []models.NewRecipient, audit *audit_models.AuditLog) (int64, error)
	List(ctx context.Context, page, pageSize int) ([]models.Blast, int, error)
	// Returns nil, nil for an unknown id.
	FindByID(ctx context.Context, id int64) (*models.Blast, error)
	Failures(ctx context.Context, id int64, limit int) ([]models.BlastFailure, error)
	// cc.ErrBlastNotSending unless the blast is still sending.
	Cancel(ctx context.Context, id int64, audit *audit_models.AuditLog) error
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
