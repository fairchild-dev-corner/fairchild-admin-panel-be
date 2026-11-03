package services

import (
	"context"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	cc "fairchild-admin-panel-be/internal/constants"
	interfaces "fairchild-admin-panel-be/internal/interfaces"
	app_models "fairchild-admin-panel-be/internal/models/applications"
	audit_models "fairchild-admin-panel-be/internal/models/audit"
	member_models "fairchild-admin-panel-be/internal/models/members"
	models "fairchild-admin-panel-be/internal/models/portal"
	portal_repo "fairchild-admin-panel-be/internal/repositories/portal"
	"fairchild-admin-panel-be/internal/utils"
)

// ledgerIndexTTL bounds how stale a portal account's branch can be: a
// member number newly added to a ledger shows its branch within this long.
const ledgerIndexTTL = 10 * time.Minute

type PortalService struct {
	repo   interfaces.PortalRepositoryInterface
	ledger interfaces.LedgerLookupInterface

	mu          sync.Mutex
	index       map[string][]models.LedgerLink // member number -> ledger records
	indexLoaded time.Time
}

func NewPortalService(repo interfaces.PortalRepositoryInterface, ledger interfaces.LedgerLookupInterface) *PortalService {
	return &PortalService{repo: repo, ledger: ledger}
}

// ledgerIndex maps every ledger member number to its records, reloaded at
// most every ledgerIndexTTL. Built in Go rather than joined in SQL: ledger
// DBs are separate schemas (and may be separate servers) that are never
// joined with central tables. If a reload fails and an older index exists,
// the older one is served.
func (s *PortalService) ledgerIndex(ctx context.Context) (map[string][]models.LedgerLink, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.index != nil && time.Since(s.indexLoaded) < ledgerIndexTTL {
		return s.index, nil
	}

	index := make(map[string][]models.LedgerLink)
	for _, branch := range s.ledger.Branches() {
		links, err := s.ledger.LoadMemberNos(ctx, branch)
		if err != nil {
			if s.index != nil {
				return s.index, nil
			}
			return nil, err
		}
		for _, l := range links {
			index[l.MemberNo] = append(index[l.MemberNo], l)
		}
	}
	s.index, s.indexLoaded = index, time.Now()
	return index, nil
}

// resolveLedger picks an account's ledger record. When its member number is
// on more than one ledger, the one whose client id matches
// users.user_client_id wins, else the first branch in order.
func resolveLedger(index map[string][]models.LedgerLink, memberNo, clientID string) (models.LedgerLink, bool) {
	candidates := index[memberNo]
	if memberNo == "" || len(candidates) == 0 {
		return models.LedgerLink{}, false
	}
	for _, c := range candidates {
		if clientID != "" && clientID == strconv.FormatInt(c.ClientID, 10) {
			return c, true
		}
	}
	return candidates[0], true
}

func attach(m *models.PortalMember, link models.LedgerLink) {
	m.Branch = link.Branch
	m.Category = link.Category
	m.LedgerStatus = link.Status
	m.LedgerAddress = link.Address
}

// List filters, counts and pages accounts by status and ledger branch.
// With no branch filter every account is listed, including ones whose
// member number isn't on any ledger; picking branches lists only accounts
// on those ledgers.
func (s *PortalService) List(ctx context.Context, q models.PortalListQuery) (*models.PortalMemberPage, error) {
	keys, err := s.repo.ListKeys(ctx, strings.TrimSpace(q.Search))
	if err != nil {
		return nil, err
	}
	index, err := s.ledgerIndex(ctx)
	if err != nil {
		return nil, err
	}

	page := &models.PortalMemberPage{
		Page:         q.Page,
		PageSize:     q.PageSize,
		StatusCounts: make(map[string]int, len(models.Statuses)),
		BranchCounts: make(map[string]int, len(member_models.Branches)),
	}
	for _, b := range member_models.Branches {
		page.BranchCounts[b] = 0
	}
	for _, st := range models.Statuses {
		page.StatusCounts[st] = 0
	}

	var ids []int64
	for _, k := range keys {
		link, onLedger := resolveLedger(index, k.MemberNo, k.ClientID)
		if k.Status == q.Status && onLedger {
			page.BranchCounts[link.Branch]++
		}
		if len(q.Branches) > 0 && !(onLedger && slices.Contains(q.Branches, link.Branch)) {
			continue
		}
		if _, ok := page.StatusCounts[k.Status]; ok {
			page.StatusCounts[k.Status]++
		}
		if k.Status == q.Status {
			ids = append(ids, k.ID)
		}
	}

	page.Total = len(ids)
	if lastPage := max(1, (page.Total+q.PageSize-1)/q.PageSize); page.Page > lastPage {
		page.Page = lastPage
	}
	from := min(len(ids), (page.Page-1)*q.PageSize)
	pageIDs := ids[from:min(len(ids), from+q.PageSize)]

	rows, err := s.repo.FindByIDs(ctx, pageIDs)
	if err != nil {
		return nil, err
	}
	byID := make(map[string]models.PortalMember, len(rows))
	for _, m := range rows {
		byID[m.ID] = m
	}
	page.Items = make([]models.PortalMember, 0, len(pageIDs))
	for _, id := range pageIDs {
		m, ok := byID[portal_repo.FormatPortalID(id)]
		if !ok {
			continue // removed between the two queries
		}
		if link, onLedger := resolveLedger(index, m.MemberNo, m.ClientID); onLedger {
			attach(&m, link)
		}
		page.Items = append(page.Items, m)
	}
	return page, nil
}

func (s *PortalService) Get(ctx context.Context, rawID string) (*models.PortalMember, error) {
	id, ok := app_models.ParseID(cc.PORTAL_MEMBER_ID_PREFIX, rawID)
	if !ok {
		return nil, cc.ErrPortalMemberNotFound
	}
	m, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if m == nil {
		return nil, cc.ErrPortalMemberNotFound
	}
	index, err := s.ledgerIndex(ctx)
	if err != nil {
		return nil, err
	}
	if link, onLedger := resolveLedger(index, m.MemberNo, m.ClientID); onLedger {
		attach(m, link)
	}
	return m, nil
}

// Lookup finds the current members with this member number on every
// connected ledger (usually one), for the Add to portal dialog.
func (s *PortalService) Lookup(ctx context.Context, memberNo string) ([]models.LedgerCandidate, error) {
	memberNo = strings.TrimSpace(memberNo)
	candidates := []models.LedgerCandidate{}
	if memberNo == "" {
		return candidates, nil
	}

	taken, err := s.repo.MemberNoTaken(ctx, memberNo)
	if err != nil {
		return nil, err
	}
	for _, branch := range s.ledger.Branches() {
		m, err := s.ledger.FindByMemberNo(ctx, branch, memberNo)
		if err != nil {
			return nil, err
		}
		if m != nil {
			candidates = append(candidates, models.LedgerCandidate{Member: *m, HasPortalAccount: taken})
		}
	}
	return candidates, nil
}

// Add gives a current ledger member a portal account. Everything but the
// contact details (and a minor's guardian) is copied from the ledger record.
func (s *PortalService) Add(ctx context.Context, req *models.AddPortalMemberRequest, actor audit_models.Actor) (*models.PortalMember, error) {
	req.MemberNo = strings.TrimSpace(req.MemberNo)
	req.Email = strings.TrimSpace(req.Email)
	req.MobileNumber = utils.CompactMobile(req.MobileNumber)
	req.GuardianFullName = strings.TrimSpace(req.GuardianFullName)
	req.GuardianMobileNumber = utils.CompactMobile(req.GuardianMobileNumber)

	if !slices.Contains(s.ledger.Branches(), req.Branch) {
		return nil, cc.ErrNotOnLedger
	}
	m, err := s.ledger.FindByMemberNo(ctx, req.Branch, req.MemberNo)
	if err != nil {
		return nil, err
	}
	if m == nil {
		return nil, cc.ErrNotOnLedger
	}

	if !utils.IsValidPhMobile(req.MobileNumber) {
		return nil, cc.ErrInvalidMobileNumber
	}
	memberType := "regular"
	if slices.Contains(member_models.GuardianCategories, m.Category) {
		memberType = "young_saver"
		if req.GuardianFullName == "" || req.GuardianMobileNumber == "" {
			return nil, cc.ErrGuardianRequired
		}
		if !utils.IsValidPhMobile(req.GuardianMobileNumber) {
			return nil, cc.ErrInvalidMobileNumber
		}
	} else {
		req.GuardianFullName, req.GuardianMobileNumber = "", ""
	}

	if taken, err := s.repo.MemberNoTaken(ctx, req.MemberNo); err != nil {
		return nil, err
	} else if taken {
		return nil, cc.ErrPortalAccountExists
	}

	id, err := s.repo.Create(ctx, m, req, memberType, actor.Log(cc.AUDIT_ADDED_PORTAL_ACCOUNT, ""))
	if err != nil {
		return nil, err
	}
	return s.Get(ctx, portal_repo.FormatPortalID(id))
}

// UpdateContact changes only the account's email and mobile number.
func (s *PortalService) UpdateContact(ctx context.Context, rawID string, req *models.ContactUpdateRequest, actor audit_models.Actor) (*models.PortalMember, error) {
	id, ok := app_models.ParseID(cc.PORTAL_MEMBER_ID_PREFIX, rawID)
	if !ok {
		return nil, cc.ErrPortalMemberNotFound
	}
	req.Email = strings.TrimSpace(req.Email)
	req.MobileNumber = utils.CompactMobile(req.MobileNumber)
	if req.MobileNumber != "" && !utils.IsValidPhMobile(req.MobileNumber) {
		return nil, cc.ErrInvalidMobileNumber
	}

	audit := actor.Log(cc.AUDIT_UPDATED_PORTAL_CONTACT, portal_repo.FormatPortalID(id))
	if err := s.repo.UpdateContact(ctx, id, req, audit); err != nil {
		return nil, err
	}
	return s.Get(ctx, rawID)
}

func (s *PortalService) Remove(ctx context.Context, rawID string, actor audit_models.Actor) error {
	id, ok := app_models.ParseID(cc.PORTAL_MEMBER_ID_PREFIX, rawID)
	if !ok {
		return cc.ErrPortalMemberNotFound
	}
	return s.repo.Remove(ctx, id, actor.Log(cc.AUDIT_REMOVED_PORTAL_ACCOUNT, portal_repo.FormatPortalID(id)))
}
