package models

// Metrics matches Metrics in the admin panel (src/models/admin.ts).
type Metrics struct {
	ApplicationsPerMonth []ApplicationsPerMonth `json:"applicationsPerMonth"`
	LoansByProduct       []LoansByProduct       `json:"loansByProduct"`
	// No visitor analytics source exists yet - always an empty list until
	// one is wired in (see README).
	DailyVisitors []TimeSeriesPoint `json:"dailyVisitors"`
}

type ApplicationsPerMonth struct {
	Label       string `json:"label"`
	Loans       int    `json:"loans"`
	Memberships int    `json:"memberships"`
}

type LoansByProduct struct {
	Product string  `json:"product"`
	Count   int     `json:"count"`
	Amount  float64 `json:"amount"`
}

type TimeSeriesPoint struct {
	Label string `json:"label"`
	Value int    `json:"value"`
}

// MonthCount is one "YYYY-MM" bucket from the repository.
type MonthCount struct {
	Month string
	Count int
}
