package models

import "testing"

func TestFormatAndParseID(t *testing.T) {
	if got := FormatID("LA-", 1040); got != "LA-01040" {
		t.Fatalf("FormatID = %q", got)
	}

	valid := map[string]int64{"LA-01040": 1040, "la-1040": 1040, "1040": 1040, " LA-7 ": 7}
	for raw, want := range valid {
		if got, ok := ParseID("LA-", raw); !ok || got != want {
			t.Errorf("ParseID(%q) = %d, %v; want %d", raw, got, ok, want)
		}
	}

	for _, raw := range []string{"", "LA-", "MA-01040", "LA-0", "-5", "LA-1x", "1; DROP TABLE"} {
		if _, ok := ParseID("LA-", raw); ok {
			t.Errorf("ParseID(%q) should fail", raw)
		}
	}
}
