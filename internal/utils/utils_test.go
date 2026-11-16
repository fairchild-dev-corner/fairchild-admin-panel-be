package utils

import (
	"strings"
	"testing"
	"time"
)

func TestParsePeriod(t *testing.T) {
	from, to, ok := ParsePeriod("2024-02", "")
	if !ok || from.Format(time.RFC3339) != "2024-01-31T16:00:00Z" || to.Format(time.RFC3339) != "2024-02-29T16:00:00Z" {
		t.Fatalf("bad leap-month bounds: %v %v %v", from, to, ok)
	}
	from, to, ok = ParsePeriod("", "2025")
	if !ok || to.Sub(*from) != 365*24*time.Hour {
		t.Fatal("bad year bounds")
	}
	for _, tc := range [][2]string{{"2024-13", ""}, {"2024-01", "2024"}, {"", "abc"}, {"2024-1", ""}} {
		if _, _, ok := ParsePeriod(tc[0], tc[1]); ok {
			t.Fatalf("accepted %v", tc)
		}
	}
	if from, to, ok := ParsePeriod("", ""); !ok || from != nil || to != nil {
		t.Fatal("empty range")
	}
}

func TestFiltersAndOptionalStrings(t *testing.T) {
	counts := map[string]int{"active": 2, "pending": 3}
	if TotalFor(counts, "all") != 5 || TotalFor(counts, "active") != 2 || TotalFor(counts, "missing") != 0 {
		t.Fatal("incorrect totals")
	}
	if !IsTabFilter([]string{"active"}, "all") || IsTabFilter([]string{"active"}, "deleted") {
		t.Fatal("incorrect filter")
	}
	if NullIfEmpty("").Valid || !NullIfEmpty("a").Valid {
		t.Fatal("incorrect SQL null")
	}
	if got := MustStringSHA256("abc"); got != "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad" {
		t.Fatal(got)
	}
}

func TestSecurityEscapeLike(t *testing.T) {
	for input, want := range map[string]string{"plain": "plain", "100%": `100\%`, "a_b": `a\_b`, `a\b`: `a\\b`, `\%_`: `\\\%\_`} {
		if got := EscapeLike(input); got != want {
			t.Errorf("%q: %q want %q", input, got, want)
		}
	}
}

func TestMobileValidation(t *testing.T) {
	for _, s := range []string{"09171234567", "9171234567", "639171234567", "+639171234567"} {
		if !IsValidPhMobile(s) {
			t.Error(s)
		}
	}
	for _, s := range []string{"", "08171234567", "0917123456", "091712345678", "+639171234567\n"} {
		if IsValidPhMobile(s) {
			t.Errorf("accepted %q", s)
		}
	}
	if CompactMobile(" 0917 123\t4567 ") != "09171234567" {
		t.Fatal("compact failed")
	}
}

func BenchmarkUtilities(b *testing.B) {
	input := strings.Repeat("search%_\\", 20)
	b.ReportAllocs()
	for b.Loop() {
		_ = EscapeLike(input)
	}
}
