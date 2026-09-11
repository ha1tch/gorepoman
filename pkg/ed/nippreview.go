package ed

import (
	"fmt"
	"strings"
)

// T-23: the diff-style preview niplines' request step prints, and the
// post-nip content it computes -- shared by the readback (so the
// caller sees exactly what changed, not the whole file re-derived by
// eye) and by the ticket's PreviewHash (so confirm can prove the
// caller saw *this* rendering, not a same-range request re-issued
// against different intervening content). Kept separate from
// preflight.go: this file answers "what changed", preflight.go
// answers "is the result still valid Go" -- niplines.go (the CLI
// verb itself, wired into Run) composes both.

// NipPreviewContextLines is how many unchanged lines of context are
// shown on each side of the removed range in the rendered preview --
// enough to place the change without reproducing the whole file.
const NipPreviewContextLines = 3

// splitLinesKeepEnds splits text into lines the way a 1-based line
// number in a niplines request expects: line 1 is the first line,
// regardless of whether the file ends with a trailing newline. Each
// returned element does NOT include its trailing "\n" -- callers
// that need to reassemble exact original bytes track that
// separately (see computeNip's hadTrailingNewline).
func splitLinesKeepEnds(text string) []string {
	if text == "" {
		return nil
	}
	trimmed := strings.TrimSuffix(text, "\n")
	return strings.Split(trimmed, "\n")
}

// computeNip returns the post-nip content (lines startLine..endLine,
// 1-based inclusive, removed) and a diff-style preview string
// showing that removal in context. err is non-nil for an
// out-of-range request, in which case both returned strings are
// empty -- the caller (niplines' request step) must not issue a
// ticket for a range that was never actually valid against the
// current file.
func computeNip(original string, startLine, endLine int) (postNip string, preview string, err error) {
	lines := splitLinesKeepEnds(original)
	n := len(lines)
	if startLine < 1 || endLine < startLine {
		return "", "", fmt.Errorf("invalid line range %d-%d", startLine, endLine)
	}
	if endLine > n {
		return "", "", fmt.Errorf("range %d-%d exceeds file length (%d lines)", startLine, endLine, n)
	}

	hadTrailingNewline := strings.HasSuffix(original, "\n") || original == ""

	var kept []string
	kept = append(kept, lines[:startLine-1]...)
	kept = append(kept, lines[endLine:]...)
	postNip = strings.Join(kept, "\n")
	if len(kept) > 0 && hadTrailingNewline {
		postNip += "\n"
	}

	ctxStart := startLine - NipPreviewContextLines
	if ctxStart < 1 {
		ctxStart = 1
	}
	ctxEnd := endLine + NipPreviewContextLines
	if ctxEnd > n {
		ctxEnd = n
	}

	var b strings.Builder
	fmt.Fprintf(&b, "--- lines %d-%d of %s (removing %d-%d) ---\n", ctxStart, ctxEnd, "file", startLine, endLine)
	for i := ctxStart; i <= ctxEnd; i++ {
		lineText := lines[i-1]
		if i >= startLine && i <= endLine {
			fmt.Fprintf(&b, "-%5d | %s\n", i, lineText)
		} else {
			fmt.Fprintf(&b, " %5d | %s\n", i, lineText)
		}
	}
	preview = b.String()
	return postNip, preview, nil
}

// nipByteSpan returns the exact byte offset range [start, end) within
// original that lines startLine..endLine (1-based, inclusive) occupy,
// chosen so that original[:start]+original[end:] is byte-for-byte
// identical to computeNip's postNip for the same range -- confirm's
// atomic write and undo-compatible Edit record need the byte-offset
// form, while computeNip's preview works line-oriented; both must
// describe the exact same removal, or a confirmed nip would not
// match the diff the caller approved.
//
// Ordinarily each removed line's OWN trailing newline is absorbed
// into the span (so the line before startLine keeps its newline, and
// the line after endLine is undisturbed). The one exception: when the
// removed range reaches the file's true last line AND the file has
// no trailing newline at all, there is no "own trailing newline" on
// that last line to absorb -- instead the newline BEFORE startLine
// (the one that had been separating it from the previous line) is
// the one that must go, otherwise the previous line is left with a
// dangling, now-meaningless newline at true EOF. This mirrors
// computeNip, which never adds a trailing newline it didn't already
// have and so naturally drops that same separator.
func nipByteSpan(original string, startLine, endLine int) (start, end int, err error) {
	lines := splitLinesKeepEnds(original)
	n := len(lines)
	if startLine < 1 || endLine < startLine || endLine > n {
		return 0, 0, fmt.Errorf("invalid line range %d-%d for a %d-line file", startLine, endLine, n)
	}

	lineStart := func(lineNo int) int {
		pos := 0
		for i := 0; i < lineNo-1; i++ {
			pos += len(lines[i]) + 1
		}
		return pos
	}

	noTrailingNewline := !strings.HasSuffix(original, "\n") && original != ""
	reachesTrueEOF := endLine == n

	start = lineStart(startLine)
	end = lineStart(endLine) + len(lines[endLine-1])
	if end < len(original) {
		end++ // absorb this line's own trailing newline
	}

	if noTrailingNewline && reachesTrueEOF && startLine > 1 {
		// No trailing newline left to absorb on the removed side --
		// absorb the separator before startLine instead, so the
		// previous line does not end up with a dangling "\n" at the
		// new true EOF.
		start--
	}

	if end > len(original) {
		end = len(original)
	}
	return start, end, nil
}
