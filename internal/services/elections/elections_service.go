package services

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	cc "fairchild-admin-panel-be/internal/constants"
	interfaces "fairchild-admin-panel-be/internal/interfaces"
	app_models "fairchild-admin-panel-be/internal/models/applications"
	audit_models "fairchild-admin-panel-be/internal/models/audit"
	models "fairchild-admin-panel-be/internal/models/elections"
)

type ElectionsService struct {
	repo interfaces.ElectionsRepositoryInterface
	now  func() time.Time
}

func NewElectionsService(repo interfaces.ElectionsRepositoryInterface) *ElectionsService {
	return &ElectionsService{repo: repo, now: time.Now}
}

func invalid(format string, args ...any) error {
	return &cc.ElectionInputError{Msg: fmt.Sprintf(format, args...)}
}

func parseID(raw string) (int64, error) {
	id, ok := app_models.ParseID(cc.ELECTION_ID_PREFIX, raw)
	if !ok {
		return 0, cc.ErrElectionNotFound
	}
	return id, nil
}

// withStatus fills Status from the publish state and the voting window.
func (s *ElectionsService) withStatus(e *models.Election, published bool) {
	starts, _ := time.Parse(time.RFC3339, e.StartsAt)
	ends, _ := time.Parse(time.RFC3339, e.EndsAt)
	e.Status = models.StatusAt(published, starts, ends, s.now())
}

// List pages elections, newest voting start first, optionally one status.
func (s *ElectionsService) List(ctx context.Context, q models.ElectionListQuery) (*models.ElectionPage, error) {
	all, published, err := s.repo.ListAll(ctx)
	if err != nil {
		return nil, err
	}

	page := &models.ElectionPage{Page: q.Page, PageSize: q.PageSize, StatusCounts: map[string]int{}}
	for _, st := range models.Statuses {
		page.StatusCounts[st] = 0
	}
	items := []models.Election{}
	for i := range all {
		e := all[i]
		s.withStatus(&e, published[i])
		page.StatusCounts[e.Status]++
		if q.Status == "" || e.Status == q.Status {
			items = append(items, e)
		}
	}

	page.Total = len(items)
	if lastPage := max(1, (page.Total+q.PageSize-1)/q.PageSize); page.Page > lastPage {
		page.Page = lastPage
	}
	from := min(len(items), (page.Page-1)*q.PageSize)
	page.Items = items[from:min(len(items), from+q.PageSize)]
	return page, nil
}

func (s *ElectionsService) Get(ctx context.Context, rawID string) (*models.Election, error) {
	id, err := parseID(rawID)
	if err != nil {
		return nil, err
	}
	return s.get(ctx, id)
}

func (s *ElectionsService) get(ctx context.Context, id int64) (*models.Election, error) {
	e, published, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if e == nil {
		return nil, cc.ErrElectionNotFound
	}
	s.withStatus(e, published)
	return e, nil
}

// LookupCandidate finds the portal account behind a member number, for the
// candidate picker. Only active members can run.
func (s *ElectionsService) LookupCandidate(ctx context.Context, memberNo string) (*models.CandidateMember, error) {
	memberNo = strings.TrimSpace(memberNo)
	if memberNo == "" {
		return nil, invalid("Enter a member ID")
	}
	found, err := s.repo.FindMembers(ctx, []string{memberNo})
	if err != nil {
		return nil, err
	}
	m, ok := found[strings.ToUpper(memberNo)]
	if !ok {
		return nil, invalid("No portal account uses member ID %s", memberNo)
	}
	if m.Status != "active" {
		return nil, invalid("Member %s's portal account isn't active, so they can't run", m.MemberNo)
	}
	return &m, nil
}

// draft validates a save request and resolves each candidate's member
// number to their portal account.
func (s *ElectionsService) draft(ctx context.Context, req *models.SaveElectionRequest) (*models.ElectionDraft, error) {
	d := &models.ElectionDraft{
		Title:       strings.TrimSpace(req.Title),
		Description: strings.TrimSpace(req.Description),
	}
	if d.Title == "" || utf8.RuneCountInString(d.Title) > models.MaxTitleLength {
		return nil, invalid("Enter a title (up to %d characters)", models.MaxTitleLength)
	}
	if utf8.RuneCountInString(d.Description) > models.MaxDescriptionLength {
		return nil, invalid("The description can be up to %d characters", models.MaxDescriptionLength)
	}
	var err error
	if d.StartsAt, err = time.Parse(time.RFC3339, req.StartsAt); err != nil {
		return nil, invalid("Enter when voting starts")
	}
	if d.EndsAt, err = time.Parse(time.RFC3339, req.EndsAt); err != nil {
		return nil, invalid("Enter when voting ends")
	}
	if !d.EndsAt.After(d.StartsAt) {
		return nil, invalid("Voting must end after it starts")
	}
	if len(req.Positions) > models.MaxPositions {
		return nil, invalid("An election can have up to %d positions", models.MaxPositions)
	}

	var memberNos []string
	for _, p := range req.Positions {
		for _, c := range p.Candidates {
			if m := strings.TrimSpace(c.MemberNo); m != "" {
				memberNos = append(memberNos, m)
			}
		}
	}
	members, err := s.repo.FindMembers(ctx, memberNos)
	if err != nil {
		return nil, err
	}

	for i, p := range req.Positions {
		pd := models.PositionDraft{Title: strings.TrimSpace(p.Title), Seats: p.Seats}
		if pd.Title == "" || utf8.RuneCountInString(pd.Title) > models.MaxPositionTitle {
			return nil, invalid("Position %d needs a title (up to %d characters)", i+1, models.MaxPositionTitle)
		}
		if pd.Seats < 1 || pd.Seats > models.MaxSeats {
			return nil, invalid("%s: seats must be between 1 and %d", pd.Title, models.MaxSeats)
		}
		if len(p.Candidates) > models.MaxCandidates {
			return nil, invalid("%s can have up to %d candidates", pd.Title, models.MaxCandidates)
		}
		var seen []string
		for _, c := range p.Candidates {
			memberNo := strings.TrimSpace(c.MemberNo)
			if memberNo == "" {
				return nil, invalid("%s: every candidate needs a member ID", pd.Title)
			}
			m, ok := members[strings.ToUpper(memberNo)]
			if !ok {
				return nil, invalid("%s: no portal account uses member ID %s", pd.Title, memberNo)
			}
			if m.Status != "active" {
				return nil, invalid("%s: member %s's portal account isn't active, so they can't run", pd.Title, m.MemberNo)
			}
			if slices.Contains(seen, strings.ToUpper(m.MemberNo)) {
				return nil, invalid("%s: member %s is listed twice", pd.Title, m.MemberNo)
			}
			seen = append(seen, strings.ToUpper(m.MemberNo))
			bio := strings.TrimSpace(c.Bio)
			if utf8.RuneCountInString(bio) > models.MaxBioLength {
				return nil, invalid("%s: a candidate's note can be up to %d characters", pd.Title, models.MaxBioLength)
			}
			pd.Candidates = append(pd.Candidates, models.CandidateDraft{Member: m, Bio: bio})
		}
		d.Positions = append(d.Positions, pd)
	}
	return d, nil
}

// ready checks what a published election needs: voting in the future, a
// voters' master list, and every position with at least one candidate.
func (s *ElectionsService) ready(startsAt time.Time, positions []models.Position, masterListCount int) error {
	if !startsAt.After(s.now()) {
		return invalid("Voting must start in the future to publish")
	}
	if masterListCount == 0 {
		return invalid("Upload the voters' master list on the Generate Master List page before publishing")
	}
	if len(positions) == 0 {
		return invalid("Add at least one position before publishing")
	}
	for _, p := range positions {
		if len(p.Candidates) == 0 {
			return invalid("%s has no candidates yet", p.Title)
		}
	}
	return nil
}

// draftPositions turns a validated draft's positions into the shape ready()
// checks.
func draftPositions(d *models.ElectionDraft) []models.Position {
	out := make([]models.Position, len(d.Positions))
	for i, p := range d.Positions {
		out[i] = models.Position{Title: p.Title, Candidates: make([]models.Candidate, len(p.Candidates))}
	}
	return out
}

func (s *ElectionsService) Create(ctx context.Context, req *models.SaveElectionRequest, actor audit_models.Actor) (*models.Election, error) {
	d, err := s.draft(ctx, req)
	if err != nil {
		return nil, err
	}
	id, err := s.repo.Create(ctx, d, actor.UserPanelID, actor.Name, actor.Log(cc.AUDIT_CREATED_ELECTION, ""))
	if err != nil {
		return nil, err
	}
	return s.get(ctx, id)
}

// Update saves changes to a draft, or to a published election whose voting
// hasn't started - which must stay ready to vote on.
func (s *ElectionsService) Update(ctx context.Context, rawID string, req *models.SaveElectionRequest, actor audit_models.Actor) (*models.Election, error) {
	current, err := s.Get(ctx, rawID)
	if err != nil {
		return nil, err
	}
	if current.Status == models.StatusOpen || current.Status == models.StatusClosed {
		return nil, cc.ErrElectionLocked
	}
	d, err := s.draft(ctx, req)
	if err != nil {
		return nil, err
	}
	if current.Status == models.StatusUpcoming {
		if err := s.ready(d.StartsAt, draftPositions(d), current.MasterListCount); err != nil {
			return nil, err
		}
	}
	id, _ := parseID(rawID)
	if err := s.repo.Update(ctx, id, d, actor.Log(cc.AUDIT_UPDATED_ELECTION, "")); err != nil {
		return nil, err
	}
	return s.get(ctx, id)
}

// Publish makes a draft visible to members; voting opens at its start.
func (s *ElectionsService) Publish(ctx context.Context, rawID string, actor audit_models.Actor) (*models.Election, error) {
	current, err := s.Get(ctx, rawID)
	if err != nil {
		return nil, err
	}
	if current.Status != models.StatusDraft {
		return nil, cc.ErrElectionNotDraft
	}
	starts, _ := time.Parse(time.RFC3339, current.StartsAt)
	if err := s.ready(starts, current.Positions, current.MasterListCount); err != nil {
		return nil, err
	}
	id, _ := parseID(rawID)
	if err := s.repo.SetPublished(ctx, id, true, actor.Log(cc.AUDIT_PUBLISHED_ELECTION, "")); err != nil {
		return nil, err
	}
	return s.get(ctx, id)
}

// Unpublish turns an upcoming election back into a draft.
func (s *ElectionsService) Unpublish(ctx context.Context, rawID string, actor audit_models.Actor) (*models.Election, error) {
	id, err := parseID(rawID)
	if err != nil {
		return nil, err
	}
	if err := s.repo.SetPublished(ctx, id, false, actor.Log(cc.AUDIT_UNPUBLISHED_ELECTION, "")); err != nil {
		return nil, err
	}
	return s.get(ctx, id)
}

func (s *ElectionsService) Delete(ctx context.Context, rawID string, actor audit_models.Actor) error {
	id, err := parseID(rawID)
	if err != nil {
		return err
	}
	return s.repo.Delete(ctx, id, actor.Log(cc.AUDIT_DELETED_ELECTION, ""))
}
