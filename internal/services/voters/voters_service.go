package services

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	cc "fairchild-admin-panel-be/internal/constants"
	interfaces "fairchild-admin-panel-be/internal/interfaces"
	audit_models "fairchild-admin-panel-be/internal/models/audit"
	models "fairchild-admin-panel-be/internal/models/voters"
)

type VotersService struct {
	repo interfaces.VotersRepositoryInterface
}

func NewVotersService(repo interfaces.VotersRepositoryInterface) *VotersService {
	return &VotersService{repo: repo}
}

func invalid(format string, args ...any) error {
	return &cc.MasterListInputError{Msg: fmt.Sprintf(format, args...)}
}

func (s *VotersService) Summary(ctx context.Context) (*models.MasterListSummary, error) {
	return s.repo.Summary(ctx)
}

// CheckMasterList reads an uploaded file and reports which members have a
// portal account, without saving anything.
func (s *VotersService) CheckMasterList(ctx context.Context, fileName string, file io.Reader) (*models.MasterListCheck, error) {
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
