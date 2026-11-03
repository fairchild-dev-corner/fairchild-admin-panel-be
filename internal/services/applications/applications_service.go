package services

import (
	"context"
	"strings"

	cc "fairchild-admin-panel-be/internal/constants"
	interfaces "fairchild-admin-panel-be/internal/interfaces"
	models "fairchild-admin-panel-be/internal/models/applications"
	audit_models "fairchild-admin-panel-be/internal/models/audit"
	"fairchild-admin-panel-be/internal/utils"
)

type ApplicationsService struct {
	repo interfaces.ApplicationsRepositoryInterface
}

func NewApplicationsService(repo interfaces.ApplicationsRepositoryInterface) *ApplicationsService {
	return &ApplicationsService{repo: repo}
}

func (s *ApplicationsService) ListLoanApplications(ctx context.Context, status string) ([]models.LoanApplication, error) {
	return s.repo.ListLoanApplications(ctx, status)
}

// GetLoanApplication accepts "LA-01040" or "1040"; a malformed id is
// reported as not found.
func (s *ApplicationsService) GetLoanApplication(ctx context.Context, rawID string) (*models.LoanApplication, error) {
	id, ok := models.ParseID(cc.LOAN_APPLICATION_ID_PREFIX, rawID)
	if !ok {
		return nil, cc.ErrApplicationNotFound
	}
	app, err := s.repo.FindLoanApplicationByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if app == nil {
		return nil, cc.ErrApplicationNotFound
	}
	return app, nil
}

// ReviewLoanApplication records the decision (plus audit entry) and returns
// the updated application.
func (s *ApplicationsService) ReviewLoanApplication(ctx context.Context, rawID string, decision *models.ReviewDecisionRequest, actor audit_models.Actor) (*models.LoanApplication, error) {
	id, ok := models.ParseID(cc.LOAN_APPLICATION_ID_PREFIX, rawID)
	if !ok {
		return nil, cc.ErrApplicationNotFound
	}

	action := cc.AUDIT_REJECTED_LOAN_APPLICATION
	if decision.Status == cc.APPLICATION_STATUS_APPROVED {
		action = cc.AUDIT_APPROVED_LOAN_APPLICATION
	}

	audit := actor.Log(action, models.FormatID(cc.LOAN_APPLICATION_ID_PREFIX, id))
	if err := s.repo.ReviewLoanApplication(ctx, id, decision, actor.UserPanelID, audit); err != nil {
		return nil, err
	}

	return s.GetLoanApplication(ctx, rawID)
}

func (s *ApplicationsService) ListMembershipApplications(ctx context.Context, status, search string) ([]models.MembershipApplication, error) {
	return s.repo.ListMembershipApplications(ctx, status, strings.TrimSpace(search))
}

func (s *ApplicationsService) GetMembershipApplication(ctx context.Context, rawID string) (*models.MembershipApplication, error) {
	id, ok := models.ParseID(cc.MEMBERSHIP_APPLICATION_ID_PREFIX, rawID)
	if !ok {
		return nil, cc.ErrApplicationNotFound
	}
	app, err := s.repo.FindMembershipApplicationByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if app == nil {
		return nil, cc.ErrApplicationNotFound
	}
	return app, nil
}

// CreateMembershipApplication saves a walk-in/phone request as a pending
// application. It applies the public form's rules that struct tags can't:
// a valid PH mobile, no future birth date, and the coop-only fields.
func (s *ApplicationsService) CreateMembershipApplication(ctx context.Context, req *models.NewMembershipApplicationRequest, actor audit_models.Actor) (*models.MembershipApplication, error) {
	req.FirstName = strings.TrimSpace(req.FirstName)
	req.LastName = strings.TrimSpace(req.LastName)
	req.MiddleName = strings.TrimSpace(req.MiddleName)
	req.Suffix = strings.TrimSpace(req.Suffix)
	req.Email = strings.TrimSpace(req.Email)
	req.MobileNumber = utils.CompactMobile(req.MobileNumber)
	req.TypeOfWork = strings.TrimSpace(req.TypeOfWork)

	if req.FirstName == "" || req.LastName == "" {
		return nil, cc.ErrInvalidPayload
	}
	if utils.IsAfterToday(req.DateOfBirth) {
		return nil, cc.ErrDateOfBirthInFuture
	}
	if !utils.IsValidPhMobile(req.MobileNumber) {
		return nil, cc.ErrInvalidMobileNumber
	}
	if req.MembershipType == cc.MEMBERSHIP_TYPE_COOP {
		if req.Email == "" || req.SourceOfIncome == "" || req.TypeOfWork == "" {
			return nil, cc.ErrCoopApplicantIncomplete
		}
	} else {
		// Laboratory (minor) applicants don't declare income.
		req.SourceOfIncome, req.TypeOfWork = "", ""
	}

	audit := actor.Log(cc.AUDIT_ADDED_MEMBERSHIP_APPLICATION, "")
	id, err := s.repo.CreateMembershipApplication(ctx, req, audit)
	if err != nil {
		return nil, err
	}
	return s.GetMembershipApplication(ctx, models.FormatID(cc.MEMBERSHIP_APPLICATION_ID_PREFIX, id))
}

func (s *ApplicationsService) ReviewMembershipApplication(ctx context.Context, rawID string, decision *models.ReviewDecisionRequest, actor audit_models.Actor) (*models.MembershipApplication, error) {
	id, ok := models.ParseID(cc.MEMBERSHIP_APPLICATION_ID_PREFIX, rawID)
	if !ok {
		return nil, cc.ErrApplicationNotFound
	}

	action := cc.AUDIT_REJECTED_MEMBERSHIP_APPLICATION
	if decision.Status == cc.APPLICATION_STATUS_APPROVED {
		action = cc.AUDIT_APPROVED_MEMBERSHIP_APPLICATION
	}

	audit := actor.Log(action, models.FormatID(cc.MEMBERSHIP_APPLICATION_ID_PREFIX, id))
	if err := s.repo.ReviewMembershipApplication(ctx, id, decision, actor.UserPanelID, audit); err != nil {
		return nil, err
	}

	return s.GetMembershipApplication(ctx, rawID)
}
