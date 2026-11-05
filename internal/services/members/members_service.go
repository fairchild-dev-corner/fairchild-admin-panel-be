package services

import (
	"context"
	"slices"
	"strings"

	cc "fairchild-admin-panel-be/internal/constants"
	interfaces "fairchild-admin-panel-be/internal/interfaces"
	app_models "fairchild-admin-panel-be/internal/models/applications"
	audit_models "fairchild-admin-panel-be/internal/models/audit"
	models "fairchild-admin-panel-be/internal/models/members"
	members_repo "fairchild-admin-panel-be/internal/repositories/members"
	"fairchild-admin-panel-be/internal/utils"
)

type MembersService struct {
	repo   interfaces.MembersRepositoryInterface
	ledger interfaces.LedgerMembersRepositoryInterface
}

func NewMembersService(repo interfaces.MembersRepositoryInterface, ledger interfaces.LedgerMembersRepositoryInterface) *MembersService {
	return &MembersService{repo: repo, ledger: ledger}
}

const (
	DefaultPageSize = 15
	MaxPageSize     = 100
)

// segment is one source's slice of the merged list: panel members first
// (newest first), then each branch ledger in branch order (by surname).
type segment struct {
	branch string // "" for the panel segment
	size   int
}

// ListMembers returns one page of members from admin_members and the
// connected branch ledgers combined, plus the tab and branch-filter counts.
// Paging walks the sources in a fixed order, so each page needs at most a
// LIMIT/OFFSET query per source it overlaps.
func (s *MembersService) ListMembers(ctx context.Context, q models.MemberListQuery) (*models.MemberPage, error) {
	q.Search = strings.TrimSpace(q.Search)
	branches := q.Branches
	if len(branches) == 0 {
		branches = models.Branches
	}

	panelCounts, err := s.repo.CountMembers(ctx, q.Search)
	if err != nil {
		return nil, err
	}
	counts := make(models.BranchCategoryCounts)
	for b, byCategory := range panelCounts {
		for c, n := range byCategory {
			counts.Add(b, c, n)
		}
	}
	ledgerCounts := make(map[string]map[string]int)
	for _, b := range s.ledger.Branches() {
		byCategory, err := s.ledger.CountMembers(ctx, b, q.Search)
		if err != nil {
			return nil, err
		}
		ledgerCounts[b] = byCategory
		for c, n := range byCategory {
			counts.Add(b, c, n)
		}
	}

	page := &models.MemberPage{
		Items:          []models.Member{},
		Page:           q.Page,
		PageSize:       q.PageSize,
		CategoryCounts: make(map[string]int, len(models.Categories)),
		BranchCounts:   make(map[string]int, len(models.Branches)),
	}
	for _, c := range models.Categories {
		for _, b := range branches {
			page.CategoryCounts[c] += counts[b][c]
		}
	}
	for _, b := range models.Branches {
		page.BranchCounts[b] = counts[b][q.Category]
	}
	page.Total = page.CategoryCounts[q.Category]

	// Past the end (e.g. the last row of the last page was deleted): serve
	// the last page instead of an empty one.
	if lastPage := max(1, (page.Total+q.PageSize-1)/q.PageSize); page.Page > lastPage {
		page.Page = lastPage
	}

	segments := []segment{{size: 0}}
	for _, b := range branches {
		segments[0].size += panelCounts[b][q.Category]
	}
	for _, b := range models.Branches {
		if slices.Contains(branches, b) && ledgerCounts[b] != nil {
			segments = append(segments, segment{branch: b, size: ledgerCounts[b][q.Category]})
		}
	}

	offset, remaining := (page.Page-1)*q.PageSize, q.PageSize
	for _, seg := range segments {
		if remaining == 0 {
			break
		}
		if offset >= seg.size {
			offset -= seg.size
			continue
		}
		n := min(remaining, seg.size-offset)

		var items []models.Member
		if seg.branch == "" {
			items, err = s.repo.ListMembers(ctx, q.Category, branches, q.Search, offset, n)
		} else {
			items, err = s.ledger.ListMembers(ctx, seg.branch, q.Category, q.Search, offset, n)
		}
		if err != nil {
			return nil, err
		}
		page.Items = append(page.Items, items...)
		offset, remaining = 0, remaining-n
	}
	return page, nil
}

// GetMember accepts a panel id ("FC20310" or "20310") or a ledger id
// ("01-012345"); anything else is reported as not found.
func (s *MembersService) GetMember(ctx context.Context, rawID string) (*models.Member, error) {
	if branch, clientID, ok := members_repo.ParseLedgerMemberID(rawID); ok {
		if !slices.Contains(s.ledger.Branches(), branch) {
			return nil, cc.ErrMemberNotFound
		}
		m, err := s.ledger.FindMember(ctx, branch, clientID)
		if err != nil {
			return nil, err
		}
		if m == nil {
			return nil, cc.ErrMemberNotFound
		}
		return m, nil
	}

	id, ok := app_models.ParseID(cc.MEMBER_ID_PREFIX, rawID)
	if !ok {
		return nil, cc.ErrMemberNotFound
	}
	m, err := s.repo.FindMemberByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if m == nil {
		return nil, cc.ErrMemberNotFound
	}
	return m, nil
}

// ledgerMember resolves a ledger id to a current member's branch and client
// id. Panel members have no ledger accounts: ErrNotLedgerMember.
func (s *MembersService) ledgerMember(ctx context.Context, rawID string) (string, int64, error) {
	if _, err := s.GetMember(ctx, rawID); err != nil {
		return "", 0, err
	}
	branch, clientID, ok := members_repo.ParseLedgerMemberID(rawID)
	if !ok {
		return "", 0, cc.ErrNotLedgerMember
	}
	return branch, clientID, nil
}

// ListStatements returns a ledger member's statements of account from the
// last two months (or their latest one).
func (s *MembersService) ListStatements(ctx context.Context, rawID string) (*models.SOAList, error) {
	branch, clientID, err := s.ledgerMember(ctx, rawID)
	if err != nil {
		return nil, err
	}
	return s.ledger.ListStatements(ctx, branch, clientID)
}

func (s *MembersService) GetStatement(ctx context.Context, rawID string, ctrlNo int64) (*models.SOAStatement, error) {
	branch, clientID, err := s.ledgerMember(ctx, rawID)
	if err != nil {
		return nil, err
	}
	stmt, err := s.ledger.FindStatement(ctx, branch, clientID, ctrlNo)
	if err != nil {
		return nil, err
	}
	if stmt == nil {
		return nil, cc.ErrStatementNotFound
	}
	return stmt, nil
}

func (s *MembersService) GetStanding(ctx context.Context, rawID string) (*models.ClientStanding, error) {
	branch, clientID, err := s.ledgerMember(ctx, rawID)
	if err != nil {
		return nil, err
	}
	return s.ledger.FindStanding(ctx, branch, clientID)
}

func (s *MembersService) CreateMember(ctx context.Context, req *models.MemberRequest, actor audit_models.Actor) (*models.Member, error) {
	if err := normalizeMember(req); err != nil {
		return nil, err
	}

	audit := actor.Log(cc.AUDIT_ADDED_MEMBER, "")
	id, err := s.repo.CreateMember(ctx, req, actor.UserPanelID, audit)
	if err != nil {
		return nil, err
	}
	return s.GetMember(ctx, members_repo.FormatMemberID(id))
}

func (s *MembersService) UpdateMember(ctx context.Context, rawID string, req *models.MemberRequest, actor audit_models.Actor) (*models.Member, error) {
	if _, _, ok := members_repo.ParseLedgerMemberID(rawID); ok {
		return nil, cc.ErrLedgerMemberReadOnly
	}
	id, ok := app_models.ParseID(cc.MEMBER_ID_PREFIX, rawID)
	if !ok {
		return nil, cc.ErrMemberNotFound
	}
	if err := normalizeMember(req); err != nil {
		return nil, err
	}

	audit := actor.Log(cc.AUDIT_UPDATED_MEMBER, members_repo.FormatMemberID(id))
	if err := s.repo.UpdateMember(ctx, id, req, actor.UserPanelID, audit); err != nil {
		return nil, err
	}
	return s.GetMember(ctx, rawID)
}

func (s *MembersService) DeleteMember(ctx context.Context, rawID string, actor audit_models.Actor) error {
	if _, _, ok := members_repo.ParseLedgerMemberID(rawID); ok {
		return cc.ErrLedgerMemberReadOnly
	}
	id, ok := app_models.ParseID(cc.MEMBER_ID_PREFIX, rawID)
	if !ok {
		return cc.ErrMemberNotFound
	}
	audit := actor.Log(cc.AUDIT_DELETED_MEMBER, members_repo.FormatMemberID(id))
	return s.repo.DeleteMember(ctx, id, actor.UserPanelID, audit)
}

// normalizeMember trims the request and applies the Members form's rules
// that struct tags can't: dates not in the future, valid PH mobiles, and a
// guardian (instead of the member's own mobile) for kiddie/aflatoun.
func normalizeMember(req *models.MemberRequest) error {
	for _, f := range []*string{
		&req.FirstName, &req.LastName, &req.MiddleName, &req.Suffix, &req.Nationality,
		&req.Email, &req.Address, &req.GuardianFullName,
	} {
		*f = strings.TrimSpace(*f)
	}
	req.MobileNumber = utils.CompactMobile(req.MobileNumber)
	req.GuardianMobileNumber = utils.CompactMobile(req.GuardianMobileNumber)

	if req.FirstName == "" || req.LastName == "" {
		return cc.ErrInvalidPayload
	}
	if utils.IsAfterToday(req.DateOfBirth) {
		return cc.ErrDateOfBirthInFuture
	}
	if utils.IsAfterToday(req.JoinedAt) {
		return cc.ErrJoinedAtInFuture
	}

	if slices.Contains(models.GuardianCategories, req.Category) {
		if req.GuardianFullName == "" || req.GuardianMobileNumber == "" {
			return cc.ErrGuardianRequired
		}
	} else {
		// A member moved out of a minors' program keeps no stale guardian.
		req.GuardianFullName, req.GuardianMobileNumber = "", ""
		if req.MobileNumber == "" {
			return cc.ErrMobileNumberRequired
		}
	}

	for _, mobile := range []string{req.MobileNumber, req.GuardianMobileNumber} {
		if mobile != "" && !utils.IsValidPhMobile(mobile) {
			return cc.ErrInvalidMobileNumber
		}
	}
	return nil
}
