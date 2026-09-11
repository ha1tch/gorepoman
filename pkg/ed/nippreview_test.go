package ed

import (
	"strings"
	"testing"
)

// TestComputeNip covers the line-oriented preview/post-nip computation
// niplines' request step (T-23) uses for its diff-style readback: the
// removed lines are dropped from the post-nip content, marked with a
// leading "-" in the rendered preview, and surrounding context is
// included -- confirmed against both the happy path and every
// malformed-range refusal niplines itself must surface as REFUSED.
func TestComputeNip(t *testing.T) {
	original := "L1\nL2\nL3\nL4\nL5\nL6\nL7\nL8\nL9\nL10\n"
	post, preview, err := computeNip(original, 4, 6)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(post, "L4") || strings.Contains(post, "L5") || strings.Contains(post, "L6") {
		t.Fatalf("post-nip content still contains removed lines: %q", post)
	}
	if !strings.Contains(post, "L3") || !strings.Contains(post, "L7") {
		t.Fatalf("post-nip content lost surrounding lines: %q", post)
	}
	if !strings.Contains(preview, "-    4 | L4") {
		t.Fatalf("preview does not mark removed line 4: %q", preview)
	}
	if !strings.Contains(preview, "    3 | L3") {
		t.Fatalf("preview missing context line 3: %q", preview)
	}
}

func TestComputeNip_RefusesInvalidRanges(t *testing.T) {
	original := "L1\nL2\nL3\nL4\nL5\nL6\nL7\nL8\nL9\n"
	cases := []struct {
		name       string
		start, end int
	}{
		{"start below 1", 0, 3},
		{"end before start", 5, 3},
		{"end beyond file length", 8, 100},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, err := computeNip(original, tc.start, tc.end); err == nil {
				t.Fatalf("expected an error for range %d-%d", tc.start, tc.end)
			}
		})
	}
}

// TestNipByteSpan_AgreesWithComputeNip is the load-bearing invariant
// confirm (T-24) depends on: nipByteSpan's byte-offset removal and
// computeNip's line-based removal must describe EXACTLY the same
// post-nip content, across every trailing-newline edge case, or a
// confirmed nip would not match the diff the caller approved in
// niplines' own preview. Found a real bug here during implementation
// (the no-trailing-newline, removal-reaches-true-EOF case) before
// this test existed in its current form -- kept as full coverage of
// every shape that class of bug could recur in, not just the one
// case that failed.
func TestNipByteSpan_AgreesWithComputeNip(t *testing.T) {
	cases := []struct {
		name     string
		original string
		start    int
		end      int
	}{
		{"middle range, trailing newline", "L1\nL2\nL3\nL4\nL5\n", 2, 3},
		{"single line", "a\nb\nc\n", 2, 2},
		{"first line", "a\nb\nc\n", 1, 1},
		{"last line, trailing newline", "a\nb\nc\n", 3, 3},
		{"last line, NO trailing newline", "a\nb\nc", 3, 3},
		{"whole file", "a\nb\nc\n", 1, 3},
		{"remove all but last, no trailing nl", "a\nb\nc", 1, 2},
		{"remove all but last, trailing nl", "a\nb\nc\n", 1, 2},
		{"single line file, no trailing nl", "onlyline", 1, 1},
		{"single line file, trailing nl", "onlyline\n", 1, 1},
		{"remove everything, no trailing nl", "a\nb\nc", 1, 3},
		{"two lines, remove first, no trailing nl on last", "a\nb", 1, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			postNip, _, err := computeNip(tc.original, tc.start, tc.end)
			if err != nil {
				t.Fatalf("computeNip error: %v", err)
			}
			s, e, err := nipByteSpan(tc.original, tc.start, tc.end)
			if err != nil {
				t.Fatalf("nipByteSpan error: %v", err)
			}
			reconstructed := tc.original[:s] + tc.original[e:]
			if reconstructed != postNip {
				t.Fatalf("nipByteSpan disagrees with computeNip:\n  byteSpan-based: %q\n  computeNip:     %q\n  span=[%d,%d) removed=%q",
					reconstructed, postNip, s, e, tc.original[s:e])
			}
		})
	}
}
