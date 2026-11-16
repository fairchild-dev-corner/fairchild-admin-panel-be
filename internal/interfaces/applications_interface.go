package interfaces

import (
	"context"

	models "fairchild-admin-panel-be/internal/models/applications"
	audit_models "fairchild-admin-panel-be/internal/models/audit"
)

type ApplicationsRepositoryInterface interface {
	// status "" means all statuses.
	ListLoanApplications(ctx context.Context, status string) ([]models.LoanApplication, error)
	FindLoanApplicationByID(ctx context.Context, id int64) (*models.LoanApplication, error)
	ReviewLoanApplication(ctx context.Context, id int64, decision *models.ReviewDecisionRequest, reviewerID int64, audit *audit_models.AuditLog) error
	// The portal account behind application id, or nil when none matches.
	FindLoanApplicant(ctx context.Context, id int64) (*models.LoanApplicant, error)
	// The member's other applications (not excludeID), newest first.
	ListMemberLoanApplications(ctx context.Context, memberID string, excludeID int64) ([]models.LoanApplication, error)
	// Writes the audit entry for a message sent to a loan applicant.
	RecordApplicantMessage(ctx context.Context, audit *audit_models.AuditLog) error
	// Staff notes on application id, oldest first.
	ListLoanApplicationNotes(ctx context.Context, id int64) ([]models.LoanApplicationNote, error)
	// Adds a note and its audit entry; returns the new note.
	InsertLoanApplicationNote(ctx context.Context, id, authorID int64, author, body string, audit *audit_models.AuditLog) (*models.LoanApplicationNote, error)
	// One note on application id, or nil.
	FindLoanApplicationNote(ctx context.Context, id, noteID int64) (*models.LoanApplicationNote, error)
	// Replaces a note's body (marking it edited) with its audit entry;
	// returns the updated note.
	UpdateLoanApplicationNote(ctx context.Context, id, noteID int64, body string, audit *audit_models.AuditLog) (*models.LoanApplicationNote, error)
	// Deletes a note with its audit entry.
	DeleteLoanApplicationNote(ctx context.Context, id, noteID int64, audit *audit_models.AuditLog) error

	// search "" means no search; otherwise it matches name, email, mobile or id.
	ListMembershipApplications(ctx context.Context, status, search string) ([]models.MembershipApplication, error)
	FindMembershipApplicationByID(ctx context.Context, id int64) (*models.MembershipApplication, error)
	// Returns the new row id; sets audit.Target once it's known.
	CreateMembershipApplication(ctx context.Context, req *models.NewMembershipApplicationRequest, audit *audit_models.AuditLog) (int64, error)
	ReviewMembershipApplication(ctx context.Context, id int64, decision *models.ReviewDecisionRequest, reviewerID int64, audit *audit_models.AuditLog) error
}
