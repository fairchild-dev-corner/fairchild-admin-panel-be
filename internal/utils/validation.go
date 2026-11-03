package utils

import (
	"regexp"
	"strings"
	"time"
)

// Same rule as the panel's isValidPhMobile: 09XXXXXXXXX, 9XXXXXXXXX,
// 639XXXXXXXXX or +639XXXXXXXXX.
var phMobilePattern = regexp.MustCompile(`^(\+?63|0)?9\d{9}$`)

// CompactMobile drops the spaces staff type into mobile numbers
// ("0908 112 2689"), so they're stored and compared in one form.
func CompactMobile(s string) string {
	return strings.Join(strings.Fields(s), "")
}

// IsValidPhMobile reports whether s (already compacted) is a PH mobile number.
func IsValidPhMobile(s string) bool {
	return phMobilePattern.MatchString(s)
}

// IsAfterToday reports whether a YYYY-MM-DD date is later than today in the
// cooperative's time zone. Dates compare correctly as strings in that format.
func IsAfterToday(date string) bool {
	return date > time.Now().In(ManilaTZ).Format(time.DateOnly)
}
