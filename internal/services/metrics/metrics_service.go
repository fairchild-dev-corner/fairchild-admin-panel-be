package services

import (
	"context"
	"slices"
	"time"

	interfaces "fairchild-admin-panel-be/internal/interfaces"
	app_models "fairchild-admin-panel-be/internal/models/applications"
	models "fairchild-admin-panel-be/internal/models/metrics"
	"fairchild-admin-panel-be/internal/utils"
)

// monthsShown matches the panel's six-month application chart.
const monthsShown = 6

type MetricsService struct {
	repo interfaces.MetricsRepositoryInterface
}

func NewMetricsService(repo interfaces.MetricsRepositoryInterface) *MetricsService {
	return &MetricsService{repo: repo}
}

func (s *MetricsService) GetMetrics(ctx context.Context) (*models.Metrics, error) {
	perMonth, err := s.applicationsPerMonth(ctx, time.Now().In(utils.ManilaTZ))
	if err != nil {
		return nil, err
	}

	byProduct, err := s.loansByProduct(ctx)
	if err != nil {
		return nil, err
	}

	return &models.Metrics{
		ApplicationsPerMonth: perMonth,
		LoansByProduct:       byProduct,
		DailyVisitors:        []models.TimeSeriesPoint{},
	}, nil
}

// applicationsPerMonth returns exactly monthsShown buckets ending with the
// current month, oldest first, zero-filled.
func (s *MetricsService) applicationsPerMonth(ctx context.Context, now time.Time) ([]models.ApplicationsPerMonth, error) {
	firstMonth := time.Date(now.Year(), now.Month()-(monthsShown-1), 1, 0, 0, 0, 0, utils.ManilaTZ)

	loans, err := s.repo.LoanApplicationsPerMonth(ctx, firstMonth)
	if err != nil {
		return nil, err
	}
	memberships, err := s.repo.MembershipApplicationsPerMonth(ctx, firstMonth)
	if err != nil {
		return nil, err
	}

	byMonth := func(counts []models.MonthCount) map[string]int {
		m := make(map[string]int, len(counts))
		for _, c := range counts {
			m[c.Month] = c.Count
		}
		return m
	}
	loanCounts, membershipCounts := byMonth(loans), byMonth(memberships)

	out := make([]models.ApplicationsPerMonth, 0, monthsShown)
	for i := 0; i < monthsShown; i++ {
		month := firstMonth.AddDate(0, i, 0)
		key := month.Format("2006-01")
		out = append(out, models.ApplicationsPerMonth{
			Label:       month.Format("Jan"),
			Loans:       loanCounts[key],
			Memberships: membershipCounts[key],
		})
	}
	return out, nil
}

// loansByProduct lists every known product in the panel's order (zero-filled
// so the chart's categories are stable), then any unexpected product names.
func (s *MetricsService) loansByProduct(ctx context.Context) ([]models.LoansByProduct, error) {
	rows, err := s.repo.ApprovedLoansByProduct(ctx)
	if err != nil {
		return nil, err
	}

	out := make([]models.LoansByProduct, 0, len(app_models.LoanProducts))
	for _, product := range app_models.LoanProducts {
		entry := models.LoansByProduct{Product: product}
		if i := slices.IndexFunc(rows, func(r models.LoansByProduct) bool { return r.Product == product }); i >= 0 {
			entry = rows[i]
		}
		out = append(out, entry)
	}
	for _, r := range rows {
		if !slices.Contains(app_models.LoanProducts, r.Product) {
			out = append(out, r)
		}
	}
	return out, nil
}
