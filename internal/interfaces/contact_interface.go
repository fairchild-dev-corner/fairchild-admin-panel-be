package interfaces

import (
	"context"

	audit_models "fairchild-admin-panel-be/internal/models/audit"
	models "fairchild-admin-panel-be/internal/models/contact"
)

// ContactRepositoryInterface reads fairchild_be's `contact_messages` and
// records replies in this service's `contact_message_replies`.
type ContactRepositoryInterface interface {
	// "unreplied" and "replied" counts matching search.
	CountByStatus(ctx context.Context, search string) (map[string]int, error)
	List(ctx context.Context, q models.ContactListQuery) ([]models.ContactMessage, error)
	// Returns nil, nil for an unknown id.
	FindByID(ctx context.Context, id int64) (*models.ContactMessage, error)
	InsertReply(ctx context.Context, messageID int64, channel, recipient, body string, audit *audit_models.AuditLog) error
}
