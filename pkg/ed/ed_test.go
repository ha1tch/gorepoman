package ed

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// TestTruncateUTF8_NeverSplitsARune is B-13's regression coverage:
// truncateUTF8 must never return a string that ends mid-codepoint,
// no matter where maxBytes happens to land relative to rune
// boundaries. Every result is checked with utf8.ValidString, which
// is the same check the original bug report used to confirm the
// corruption (a direct UTF-8 decode of ed find's own stdout, not
// just a wrapping tool's error surface).
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

// TestTruncateUTF8_ShortStringUnchanged confirms the common case
// (nothing to truncate) is returned exactly as-is, not just
// "still valid UTF-8" -- a truncation function that's correct only
// on the input it actually needs to shorten would be easy to get
// subtly wrong on the pass-through path.
func TestTruncateUTF8_ShortStringUnchanged(t *testing.T) {
	s := "hello, ░ world"
	got := truncateUTF8(s, 1000)
	if got != s {
		t.Fatalf("expected unchanged string, got %q", got)
	}
}

// TestTruncateUTF8_ExactRuneBoundary confirms a maxBytes that lands
// exactly on a rune boundary keeps the full rune rather than
// over-trimming defensively.
func TestTruncateUTF8_ExactRuneBoundary(t *testing.T) {
	// U+2591 is 3 bytes (0xE2 0x96 0x91). Five of them is 15 bytes,
	// a clean boundary.
	s := strings.Repeat("░", 5) + "tail"
	got := truncateUTF8(s, 15)
	want := strings.Repeat("░", 5)
	if got != want {
		t.Fatalf("expected exact boundary to keep all 5 runes, got %q (%d bytes)", got, len(got))
	}
}
