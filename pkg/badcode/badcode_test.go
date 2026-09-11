package badcode

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

// TestCheck_SnippetIsValidUTF8OnLongMultiByteLine is B-13's regression
// coverage for badcode's own snippet truncation (Check's per-line
// pass, badcode.go around the "snippet := ed.TruncateUTF8(...)" call):
// before the fix, a match on a long line of multi-byte characters
// could produce a Snippet that was genuinely invalid UTF-8, the same
// root cause as ed find's own preview truncation -- both sliced by
// raw byte count rather than at a rune boundary.
func TestCheck_SnippetIsValidUTF8OnLongMultiByteLine(t *testing.T) {
	dir := t.TempDir()
	// A single 1-byte "x" pushes every subsequent 3-byte U+2591 off a
	// clean multiple-of-3 boundary, so the snippet's 120-byte cut
	// point lands mid-codepoint (confirmed: byte 120 falls inside the
	// rune starting at byte 118) -- a pure repeat of only 3-byte
	// characters never triggers this, since 120 is itself a multiple
	// of 3 and every cut lands clean.
	line := "x" + strings.Repeat("░", 60) + "SECRETVALUE" + strings.Repeat("░", 50)
	path := filepath.Join(dir, "longline.txt")
	if err := os.WriteFile(path, []byte(line+"\n"), 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	matches := Check([]string{dir}, []Pattern{{Text: "SECRETVALUE", Reason: "test"}})
	if len(matches) != 1 {
		t.Fatalf("expected exactly 1 match, got %d", len(matches))
	}
	if !utf8.ValidString(matches[0].Snippet) {
		t.Fatalf("snippet is not valid UTF-8: %q", matches[0].Snippet)
	}
}

// TestCheck_FoldedSnippetIsValidUTF8 covers the SECOND truncation
// site in Check (the cross-line "fold pass", for a pattern that spans
// more than one source line) -- a distinct code path from the
// per-line pass above, with its own separate truncation that needed
// the same fix.
func TestCheck_FoldedSnippetIsValidUTF8(t *testing.T) {
	dir := t.TempDir()
	// The fold pass's own snippet is joined[pos:pos+len(lp)] -- ONLY
	// truncated at 120 bytes if the pattern itself (lp) is over 120
	// bytes long, so the pattern here has to be long, not just the
	// surrounding file content. "x" (1 byte) ahead of 60 repeats of
	// U+2591 (3 bytes each) pushes the 120-byte cut off a multiple-of-3
	// boundary, landing it mid-codepoint (confirmed at byte 118-119).
	// The pattern must also span a real newline in the source file to
	// exercise the FOLD pass rather than the per-line pass above --
	// Check's "joined" text has every newline byte removed, so the
	// on-disk content carries a real line break in the middle of the
	// long run while the pattern searched for is the newline-free
	// joined form.
	onDisk := "x" + strings.Repeat("░", 30) + "\n" + strings.Repeat("░", 30) + "secretvalue"
	pattern := "x" + strings.Repeat("░", 60) + "secretvalue"
	path := filepath.Join(dir, "folded.txt")
	if err := os.WriteFile(path, []byte(onDisk+"\n"), 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	matches := Check([]string{dir}, []Pattern{{Text: pattern, Reason: "test"}})
	if len(matches) != 1 {
		t.Fatalf("expected exactly 1 fold-pass match, got %d", len(matches))
	}
	if !utf8.ValidString(matches[0].Snippet) {
		t.Fatalf("folded snippet is not valid UTF-8: %q", matches[0].Snippet)
	}
}
