package services

import (
	"context"
	"fmt"
	"slices"
	"testing"

	audit_models "fairchild-admin-panel-be/internal/models/audit"
	models "fairchild-admin-panel-be/internal/models/members"
)

// fakePanel holds n regular panel members per branch, ids P-<branch>-<i>.
type fakePanel struct{ perBranch map[string]int }

func (f fakePanel) CountMembers(context.Context, string) (models.BranchCategoryCounts, error) {
	c := make(models.BranchCategoryCounts)
	for b, n := range f.perBranch {
		c.Add(b, "regular", n)
	}
	return c, nil
}

func (f fakePanel) ListMembers(_ context.Context, category string, branches []string, _ string, offset, limit int) ([]models.Member, error) {
	var all []models.Member
	for _, b := range models.Branches {
		if category == "regular" && slices.Contains(branches, b) {
			for i := range f.perBranch[b] {
				all = append(all, models.Member{ID: fmt.Sprintf("P-%s-%d", b, i)})
			}
		}
	}
	return window(all, offset, limit), nil
}

func (fakePanel) FindMemberByID(context.Context, int64) (*models.Member, error) { return nil, nil }
func (fakePanel) CreateMember(context.Context, *models.MemberRequest, int64, *audit_models.AuditLog) (int64, error) {
	return 0, nil
}
func (fakePanel) UpdateMember(context.Context, int64, *models.MemberRequest, int64, *audit_models.AuditLog) error {
	return nil
}
func (fakePanel) DeleteMember(context.Context, int64, int64, *audit_models.AuditLog) error {
	return nil
}

// fakeLedger holds n regular members per connected branch, ids L-<branch>-<i>.
type fakeLedger struct{ perBranch map[string]int }

func (f fakeLedger) Branches() []string {
	var out []string
	for _, b := range models.Branches {
		if _, ok := f.perBranch[b]; ok {
			out = append(out, b)
		}
	}
	return out
}

func (f fakeLedger) CountMembers(_ context.Context, branch, _ string) (map[string]int, error) {
	return map[string]int{"regular": f.perBranch[branch]}, nil
}

func (f fakeLedger) ListMembers(_ context.Context, branch, category, _ string, offset, limit int) ([]models.Member, error) {
	var all []models.Member
	if category == "regular" {
		for i := range f.perBranch[branch] {
			all = append(all, models.Member{ID: fmt.Sprintf("L-%s-%d", branch, i)})
		}
	}
	return window(all, offset, limit), nil
}

func (fakeLedger) FindMember(context.Context, string, int64) (*models.Member, error) { return nil, nil }

func window(all []models.Member, offset, limit int) []models.Member {
	if offset >= len(all) {
		return nil
	}
	return all[offset:min(len(all), offset+limit)]
}

func ids(items []models.Member) []string {
	out := make([]string, len(items))
	for i, m := range items {
		out[i] = m.ID
	}
	return out
}

func TestListMembersPagesAcrossSources(t *testing.T) {
	svc := NewMembersService(
		fakePanel{perBranch: map[string]int{"main": 2, "tiic": 1}},
		fakeLedger{perBranch: map[string]int{"main": 3, "knowles": 2}}, // tiic ledger not connected
	)
	list := func(page, size int, branches ...string) *models.MemberPage {
		t.Helper()
		p, err := svc.ListMembers(context.Background(), models.MemberListQuery{
			Category: "regular", Branches: branches, Page: page, PageSize: size,
		})
		if err != nil {
			t.Fatal(err)
		}
		return p
	}

	// 3 panel + 3 main + 2 knowles = 8, in source order.
	all := list(1, 100)
	want := []string{"P-main-0", "P-main-1", "P-tiic-0", "L-main-0", "L-main-1", "L-main-2", "L-knowles-0", "L-knowles-1"}
	if !slices.Equal(ids(all.Items), want) || all.Total != 8 {
		t.Fatalf("all = %v (total %d), want %v", ids(all.Items), all.Total, want)
	}

	// Pages of 3 stitch the sources together without gaps or repeats.
	var stitched []string
	for p := 1; p <= 3; p++ {
		stitched = append(stitched, ids(list(p, 3).Items)...)
	}
	if !slices.Equal(stitched, want) {
		t.Fatalf("stitched pages = %v, want %v", stitched, want)
	}

	// A page past the end serves the last page.
	if p := list(9, 3); p.Page != 3 || !slices.Equal(ids(p.Items), want[6:]) {
		t.Fatalf("page 9 = page %d %v, want page 3 %v", p.Page, ids(p.Items), want[6:])
	}

	// Branch filter: tiic only has a panel member (its ledger isn't connected).
	if p := list(1, 10, "tiic"); !slices.Equal(ids(p.Items), []string{"P-tiic-0"}) || p.Total != 1 {
		t.Fatalf("tiic = %v (total %d)", ids(p.Items), p.Total)
	}

	// Counts: categories over the selected branches, branches over the category.
	p := list(1, 10, "main", "knowles")
	if p.CategoryCounts["regular"] != 7 || p.CategoryCounts["kiddie"] != 0 {
		t.Fatalf("categoryCounts = %v", p.CategoryCounts)
	}
	if p.BranchCounts["main"] != 5 || p.BranchCounts["knowles"] != 2 || p.BranchCounts["tiic"] != 1 {
		t.Fatalf("branchCounts = %v", p.BranchCounts)
	}
}
