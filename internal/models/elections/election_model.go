package models

import "time"

// Election statuses, derived from published_at and the voting window.
const (
	StatusDraft    = "draft"    // not published; only admins see it
	StatusUpcoming = "upcoming" // published, voting hasn't started
	StatusOpen     = "open"     // voting now
	StatusClosed   = "closed"   // voting ended
)

// Statuses lists the statuses in tab order.
var Statuses = []string{StatusDraft, StatusUpcoming, StatusOpen, StatusClosed}

// Limits on what an admin can enter.
const (
	MaxTitleLength       = 200
	MaxDescriptionLength = 5000
	MaxPositionTitle     = 150
	MaxBioLength         = 1000
	MaxSeats             = 50
	MaxPositions         = 30
	MaxCandidates        = 50 // per position
)

// StatusAt works out an election's status at now.
func StatusAt(published bool, startsAt, endsAt, now time.Time) string {
	switch {
	case !published:
		return StatusDraft
	case now.Before(startsAt):
		return StatusUpcoming
	case now.Before(endsAt):
		return StatusOpen
	default:
		return StatusClosed
	}
}

// Candidate matches ElectionCandidate in the admin panel
// (src/models/admin.ts). Votes is only filled once voting has started.
type Candidate struct {
	ID       string `json:"id,omitempty"`
	MemberNo string `json:"memberNo"`
	Name     string `json:"name"`
	Bio      string `json:"bio,omitempty"`
	Votes    int    `json:"votes"`
}

// Position matches ElectionPosition in the panel.
type Position struct {
	ID         string      `json:"id,omitempty"`
	Title      string      `json:"title"`
	Seats      int         `json:"seats"`
	Candidates []Candidate `json:"candidates"`
}

// Election matches Election in the panel. Timestamps are RFC 3339.
// Positions is only filled on GET /admin/elections/:id.
type Election struct {
	ID             string `json:"id"`
	Title          string `json:"title"`
	Description    string `json:"description,omitempty"`
	StartsAt       string `json:"startsAt"`
	EndsAt         string `json:"endsAt"`
	Status         string `json:"status"`
	PublishedAt    string `json:"publishedAt,omitempty"`
	CreatedBy      string `json:"createdBy"`
	CreatedAt      string `json:"createdAt"`
	PositionCount  int    `json:"positionCount"`
	CandidateCount int    `json:"candidateCount"`
	// Members who have cast at least one vote, and members who may vote: on
	// the shared voters' master list with an active portal account.
	Voters         int `json:"voters"`
	EligibleVoters int `json:"eligibleVoters"`
	// Members on the voters' master list (see the voters package).
	MasterListCount int        `json:"masterListCount"`
	Positions       []Position `json:"positions,omitempty"`
}

// ElectionPage matches ElectionPage in the panel. StatusCounts covers every
// election regardless of the status filter.
type ElectionPage struct {
	Items        []Election     `json:"items"`
	Total        int            `json:"total"`
	Page         int            `json:"page"`
	PageSize     int            `json:"pageSize"`
	StatusCounts map[string]int `json:"statusCounts"`
}

// ElectionListQuery is GET /admin/elections?status=&page=&pageSize= (an empty
// status lists every election).
type ElectionListQuery struct {
	Status   string
	Page     int
	PageSize int
}

// CandidateInput is one candidate in a save request; the name comes from
// the member's portal account.
type CandidateInput struct {
	MemberNo string `json:"memberNo"`
	Bio      string `json:"bio"`
}

type PositionInput struct {
	Title      string           `json:"title"`
	Seats      int              `json:"seats"`
	Candidates []CandidateInput `json:"candidates"`
}

// SaveElectionRequest is the body of POST /admin/elections and
// PUT /admin/elections/:id. Positions and candidates replace what's saved.
// StartsAt/EndsAt are RFC 3339.
type SaveElectionRequest struct {
	Title       string          `json:"title"`
	Description string          `json:"description"`
	StartsAt    string          `json:"startsAt"`
	EndsAt      string          `json:"endsAt"`
	Positions   []PositionInput `json:"positions"`
}

// CandidateMember is the portal account behind a member number, for the
// candidate picker (GET /admin/elections/candidate-lookup?memberNo=).
type CandidateMember struct {
	UserID   int64  `json:"-"`
	MemberNo string `json:"memberNo"`
	Name     string `json:"name"`
	Status   string `json:"status"`
}

// ElectionDraft is a validated save request, ready to store.
type ElectionDraft struct {
	Title       string
	Description string
	StartsAt    time.Time
	EndsAt      time.Time
	Positions   []PositionDraft
}

type PositionDraft struct {
	Title      string
	Seats      int
	Candidates []CandidateDraft
}

type CandidateDraft struct {
	Member CandidateMember
	Bio    string
}
