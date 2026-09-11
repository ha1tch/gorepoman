package roles

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// TestTruncateUTF8_NeverSplitsARune is B-13's regression coverage for
// this package's own copy of the helper (kept local rather than
// shared with pkg/ed's identical function -- see the doc comment on
// truncateUTF8 for why). `repoman roles` has the same line-preview
// trim as `ed find` and was found to share the same bug: slicing by
// raw byte count corrupts a multi-byte UTF-8 rune whenever the cut
// point lands mid-codepoint.
func TestTruncateUTF8_NeverSplitsARune(t *testing.T) {
	cases := []struct {
		name string
		s    string
	}{
		{"empty", ""},
		{"ascii-only", strings.Repeat("x", 200)},
		{"the-original-repro", strings.Repeat("░", 200) + "MARKER" + strings.Repeat("░", 50)},
		{"em-dash-run", strings.Repeat("—", 200)},
		{"mixed-widths", "a░b░░c" + strings.Repeat("░", 100)},
		{"four-byte-emoji", strings.Repeat("\U0001F600", 100)},
	}
	for _, tc := range cases {
		for maxBytes := 0; maxBytes <= 130; maxBytes++ {
			got := truncateUTF8(tc.s, maxBytes)
			if !utf8.ValidString(got) {
				t.Fatalf("%s: truncateUTF8(_, %d) produced invalid UTF-8: %q", tc.name, maxBytes, got)
			}
			if len(got) > maxBytes {
				t.Fatalf("%s: truncateUTF8(_, %d) returned %d bytes, longer than the requested max", tc.name, maxBytes, len(got))
			}
		}
	}
}
