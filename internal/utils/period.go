package utils

import "time"

// ParsePeriod turns ?month=YYYY-MM or ?year=YYYY into a UTC [from, to)
// range in the cooperative's time zone. Neither gives nil, nil; both, or a
// malformed value, is not ok.
func ParsePeriod(month, year string) (from, to *time.Time, ok bool) {
	var start, end time.Time
	switch {
	case month != "" && year != "":
		return nil, nil, false
	case month != "":
		t, err := time.ParseInLocation("2006-01", month, ManilaTZ)
		if err != nil {
			return nil, nil, false
		}
		start, end = t, t.AddDate(0, 1, 0)
	case year != "":
		t, err := time.ParseInLocation("2006", year, ManilaTZ)
		if err != nil {
			return nil, nil, false
		}
		start, end = t, t.AddDate(1, 0, 0)
	default:
		return nil, nil, true
	}
	start, end = start.UTC(), end.UTC()
	return &start, &end, true
}
