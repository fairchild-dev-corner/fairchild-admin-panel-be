package services

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"strings"

	cc "fairchild-admin-panel-be/internal/constants"
	interfaces "fairchild-admin-panel-be/internal/interfaces"
	audit_models "fairchild-admin-panel-be/internal/models/audit"
	member_models "fairchild-admin-panel-be/internal/models/members"
	models "fairchild-admin-panel-be/internal/models/voters"
)

type VotersService struct {
	repo      interfaces.VotersRepositoryInterface
	passwords interfaces.PasswordConfirmerInterface
	branches  interfaces.BranchResolverInterface
}

func NewVotersService(
	repo interfaces.VotersRepositoryInterface,
	passwords interfaces.PasswordConfirmerInterface,
	branches interfaces.BranchResolverInterface,
) *VotersService {
	return &VotersService{repo: repo, passwords: passwords, branches: branches}
}

// Stats counts voting and non-voting members per ledger branch, matched the
// same way as the Portal page. Members on no ledger count under NoBranch.
func (s *VotersService) Stats(ctx context.Context) (*models.VoterStats, error) {
	statuses, err := s.repo.VoterStatuses(ctx)
	if err != nil {
		return nil, err
	}
	branchOf, err := s.branches.BranchResolver(ctx)
	if err != nil {
		return nil, err
	}

	stats := &models.VoterStats{Voting: map[string]int{}, NonVoting: map[string]int{}}
	for _, b := range slices.Concat(member_models.Branches, []string{models.NoBranch}) {
		stats.Voting[b], stats.NonVoting[b] = 0, 0
	}
	for _, v := range statuses {
		branch, ok := branchOf(v.MemberNo, v.ClientID)
		if !ok {
			branch = models.NoBranch
		}
		if v.CanVote {
			stats.Voting[branch]++
			stats.TotalVoting++
		} else {
			stats.NonVoting[branch]++
			stats.TotalNonVoting++
		}
	}
	return stats, nil
}

func invalid(format string, args ...any) error {
	return &cc.MasterListInputError{Msg: fmt.Sprintf(format, args...)}
}

func (s *VotersService) Summary(ctx context.Context) (*models.MasterListSummary, error) {
	return s.repo.Summary(ctx)
}

func (s *VotersService) ElectionControl(ctx context.Context) (*models.ElectionControl, error) {
	return s.repo.GetElectionControl(ctx)
}

// SetElectionControl turns the election on or off once the admin's own
// password checks out (a wrong one counts toward the login lockout).
func (s *VotersService) SetElectionControl(ctx context.Context, req *models.SetElectionControlRequest, actor audit_models.Actor, userAgent string) (*models.ElectionControl, error) {
	if err := s.passwords.ConfirmPassword(ctx, actor.UserPanelID, req.Password, actor.IP, userAgent); err != nil {
		return nil, err
	}
	action := cc.AUDIT_TURNED_ELECTION_OFF
	if *req.Enabled {
		action = cc.AUDIT_TURNED_ELECTION_ON
	}
	if err := s.repo.SetElectionControl(ctx, *req.Enabled, actor.UserPanelID, actor.Name, actor.Log(action, "")); err != nil {
		return nil, err
	}
	return s.repo.GetElectionControl(ctx)
}

// requireElectionOn: the master list only changes while the election is on.
func (s *VotersService) requireElectionOn(ctx context.Context) error {
	c, err := s.repo.GetElectionControl(ctx)
	if err != nil {
		return err
	}
	if !c.Enabled {
		return cc.ErrElectionOff
	}
	return nil
}

// CheckMasterList reads an uploaded file and reports which members have a
// portal account, without saving anything.
func (s *VotersService) CheckMasterList(ctx context.Context, fileName string, file io.Reader) (*models.MasterListCheck, error) {
	if err := s.requireElectionOn(ctx); err != nil {
		return nil, err
	}
	data, err := io.ReadAll(io.LimitReader(file, models.MaxMasterListBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > models.MaxMasterListBytes {
		return nil, invalid("The file is too large (up to %d MB)", models.MaxMasterListBytes>>20)
	}

	rows, blank, err := parseMasterList(fileName, data)
	if err != nil {
		return nil, err
	}
	accounts, err := s.repo.LoadAccounts(ctx)
	if err != nil {
		return nil, err
	}
	summary, err := s.repo.Summary(ctx)
	if err != nil {
		return nil, err
	}
	members, notFound, duplicates := matchRows(newAccountIndex(accounts), rows)

	check := &models.MasterListCheck{
		FileName: filepath.Base(fileName), Rows: len(rows), Members: members, NotFound: notFound,
		Inactive: []models.MasterListMember{}, Duplicates: duplicates, Blank: blank, CurrentCount: summary.Count,
	}
	for _, m := range members {
		if m.Status != "active" {
			check.Inactive = append(check.Inactive, m)
		}
	}
	return check, nil
}

// SaveMasterList replaces the master list with the confirmed members.
// They're matched to portal accounts again, so one removed since the check
// is left out.
func (s *VotersService) SaveMasterList(ctx context.Context, req *models.SaveMasterListRequest, actor audit_models.Actor) (*models.MasterListSummary, error) {
	if err := s.requireElectionOn(ctx); err != nil {
		return nil, err
	}
	if len(req.Members) == 0 {
		return nil, invalid("The master list has no members with a portal account")
	}
	if len(req.Members) > models.MaxMasterListRows {
		return nil, invalid("A master list can have up to %d members", models.MaxMasterListRows)
	}
	accounts, err := s.repo.LoadAccounts(ctx)
	if err != nil {
		return nil, err
	}
	members, _, _ := matchRows(newAccountIndex(accounts), req.Members)
	if len(members) == 0 {
		return nil, invalid("None of these members has a portal account")
	}

	fileName := strings.TrimSpace(filepath.Base(req.FileName))
	if fileName == "." || len(fileName) > 255 {
		fileName = ""
	}
	if err := s.repo.ReplaceMasterList(ctx, fileName, members, actor.UserPanelID, actor.Name,
		actor.Log(cc.AUDIT_UPLOADED_MASTER_LIST, "")); err != nil {
		return nil, err
	}
	return s.repo.Summary(ctx)
}

// List pages every member (or only voting / non-voting ones).
func (s *VotersService) List(ctx context.Context, q models.VoterListQuery) (*models.VoterPage, error) {
	q.Search = strings.TrimSpace(q.Search)
	counts, err := s.repo.CountVoters(ctx, q.Search)
	if err != nil {
		return nil, err
	}

	page := &models.VoterPage{Total: counts[q.Status], Page: q.Page, PageSize: q.PageSize, StatusCounts: counts}
	if lastPage := max(1, (page.Total+q.PageSize-1)/q.PageSize); page.Page > lastPage {
		page.Page = lastPage
	}
	q.Page = page.Page
	if page.Items, err = s.repo.ListVoters(ctx, q); err != nil {
		return nil, err
	}
	return page, nil
}

// ResetMasterList empties the master list so every member is non-voting,
// once the admin's own password checks out. Only while the election is on,
// like uploading a new list.
func (s *VotersService) ResetMasterList(ctx context.Context, req *models.ResetMasterListRequest, actor audit_models.Actor, userAgent string) (*models.MasterListSummary, error) {
	if err := s.requireElectionOn(ctx); err != nil {
		return nil, err
	}
	if err := s.passwords.ConfirmPassword(ctx, actor.UserPanelID, req.Password, actor.IP, userAgent); err != nil {
		return nil, err
	}
	if _, err := s.repo.ClearMasterList(ctx, actor.Log(cc.AUDIT_RESET_MASTER_LIST, "")); err != nil {
		return nil, err
	}
	return s.repo.Summary(ctx)
}
