package services

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestComposerClipPreservesUnicode(t *testing.T) {
	got := clip("é日🙂abc", 3)
	if !utf8.ValidString(got) || utf8.RuneCountInString(got) != 3 {
		t.Fatalf("%q", got)
	}
}
func BenchmarkComposer(b *testing.B) {
	input := strings.Repeat("é日🙂announcement ", 500)
	b.ReportAllocs()
	for b.Loop() {
		_ = clip(input, 4500)
	}
}
