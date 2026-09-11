package waveprogress

import (
	"os"
	"path/filepath"
	"testing"
)

// These are FR-04's regression tests: nothing used to regenerate a
// wave row's own Status cell outside of `register close`'s own direct
// wiring (T-19/propagateCloseToWaves, in pkg/register) -- a row could
// drift from reality whenever an item's closure happened any other
// way, e.g. RESOLVED.md gaining a closure header by some other means.
// syncWaveRowsFromRegister closes that gap: it derives each row's
// real status from RESOLVED.md's closure headers and TRACKING.md's
// own open-item rows, and corrects the wave document before parseWaves
// ever reads it.

// withTempProject creates a real project directory (with .repoman.json
// and docs/) and runs fn with the working directory set to it,
// restoring the original cwd afterward. newEnv() resolves paths
// relative to cwd via config.Load(""), so this is what makes a real
// *env usable in a test without hand-constructing one.
func withTempProject(t *testing.T, fn func(dir string)) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".repoman.json"), []byte("{}\n"), 0644); err != nil {
		t.Fatalf("write .repoman.json: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "docs"), 0755); err != nil {
		t.Fatalf("mkdir docs: %v", err)
	}
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("os.Chdir: %v", err)
	}
	defer os.Chdir(orig)
	fn(dir)
}

const waveDocFixture = `# Wave tracking

## 1. Progress at a glance

` + "```" + `
Wave 1  test wave                   ░░░░░░░░░░░░░░░░░░░░     0%  (0/1 items)
` + "```" + `

Overall by item count: 0 of 1 items ≈ **0%**

### Wave 1 — test wave (1 item, ideal 1.0d, added 2026-09-10)

| # | Summary | Status | Register item |
|---|---|---|---|
| 1 | thing | ☐ | T-02 |

**Wave 1: 0/1, not started.**
`

const trackingFixtureOpen = `# Tracking

## debt

| ID | Summary | Theme | Priority | Status | Blocks |
|---|---|---|---|---|---|
| T-02 | thing | x | P1 | ☐ | — |
`

const trackingFixtureNoT02 = `# Tracking

## debt

| ID | Summary | Theme | Priority | Status | Blocks |
|---|---|---|---|---|---|
`

func resolvedFixtureClosingT02() string {
	return `# Resolved

## [0.1.0] T-02 — thing (v0.1.0, 2026-09-10)

Theme: x · closed 0.1.0 · 2026-09-10
closed by hand, bypassing register close

Cross-ref: CHANGELOG 0.1.0.
`
}

// TestSyncWaveRowsFromRegister_ClosedInResolvedCorrectsRow is the
// direct FR-04 repro: T-02 closed via a RESOLVED.md closure header
// alone (register close never ran) must flip the wave row to ✓.
func TestSyncWaveRowsFromRegister_ClosedInResolvedCorrectsRow(t *testing.T) {
	withTempProject(t, func(dir string) {
		if err := os.WriteFile(filepath.Join(dir, "docs", "TRACKING.md"), []byte(trackingFixtureOpen), 0644); err != nil {
			t.Fatalf("write TRACKING.md: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, "docs", "RESOLVED.md"), []byte(resolvedFixtureClosingT02()), 0644); err != nil {
			t.Fatalf("write RESOLVED.md: %v", err)
		}

		e, err := newEnv()
		if err != nil {
			t.Fatalf("newEnv: %v", err)
		}

		got, warnings := e.syncWaveRowsFromRegister(waveDocFixture)
		if len(warnings) != 0 {
			t.Fatalf("expected no warnings, got %v", warnings)
		}
		if got == waveDocFixture {
			t.Fatal("expected the row to be corrected, text is unchanged")
		}

		waves := e.parseWaves(got)
		if len(waves) != 1 {
			t.Fatalf("expected 1 wave parsed from corrected text, got %d", len(waves))
		}
		if waves[0].DoneEquiv != 1.0 || waves[0].Total != 1 {
			t.Fatalf("expected the corrected wave to read as 1/1 done, got %v/%v", waves[0].DoneEquiv, waves[0].Total)
		}
	})
}

// TestSyncWaveRowsFromRegister_StillOpenLeavesRowUnchanged confirms
// the sync is a genuine no-op when the linked item really is still
// open in TRACKING.md and has no RESOLVED.md entry -- the common
// case, which must never spuriously rewrite the file.
func TestSyncWaveRowsFromRegister_StillOpenLeavesRowUnchanged(t *testing.T) {
	withTempProject(t, func(dir string) {
		if err := os.WriteFile(filepath.Join(dir, "docs", "TRACKING.md"), []byte(trackingFixtureOpen), 0644); err != nil {
			t.Fatalf("write TRACKING.md: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, "docs", "RESOLVED.md"), []byte("# Resolved\n"), 0644); err != nil {
			t.Fatalf("write RESOLVED.md: %v", err)
		}

		e, err := newEnv()
		if err != nil {
			t.Fatalf("newEnv: %v", err)
		}

		got, warnings := e.syncWaveRowsFromRegister(waveDocFixture)
		if len(warnings) != 0 {
			t.Fatalf("expected no warnings, got %v", warnings)
		}
		if got != waveDocFixture {
			t.Fatalf("expected no change when the item is genuinely still open, got a diff")
		}
	})
}

// TestSyncWaveRowsFromRegister_OrphanIDWarnsAndLeavesRowAlone covers
// the id-found-nowhere case: this function corrects rows from real
// state, it never guesses -- an id absent from both TRACKING.md and
// RESOLVED.md's closure headers must produce a warning and an
// untouched row, not a silently wrong one.
func TestSyncWaveRowsFromRegister_OrphanIDWarnsAndLeavesRowAlone(t *testing.T) {
	withTempProject(t, func(dir string) {
		if err := os.WriteFile(filepath.Join(dir, "docs", "TRACKING.md"), []byte(trackingFixtureNoT02), 0644); err != nil {
			t.Fatalf("write TRACKING.md: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, "docs", "RESOLVED.md"), []byte("# Resolved\n"), 0644); err != nil {
			t.Fatalf("write RESOLVED.md: %v", err)
		}

		e, err := newEnv()
		if err != nil {
			t.Fatalf("newEnv: %v", err)
		}

		got, warnings := e.syncWaveRowsFromRegister(waveDocFixture)
		if len(warnings) != 1 {
			t.Fatalf("expected exactly 1 warning for the orphan id, got %d: %v", len(warnings), warnings)
		}
		if got != waveDocFixture {
			t.Fatal("expected the row to be left untouched when its id resolves nowhere")
		}
	})
}

// TestCombineStatus covers the range-of-ids folding rule directly:
// a wave row can name more than one register id (a "T-1 through T-3"
// range), and the combined status must be the LEAST-done member --
// all ✓ only if every member is ✓, otherwise ◐ if there's a mix,
// otherwise ☐.
func TestCombineStatus(t *testing.T) {
	cases := []struct {
		acc, next, want string
	}{
		{"", "✓", "✓"},
		{"", "☐", "☐"},
		{"✓", "✓", "✓"},
		{"✓", "☐", "◐"},
		{"✓", "◐", "◐"},
		{"☐", "☐", "☐"},
		{"☐", "✓", "◐"},
		{"◐", "✓", "◐"},
	}
	for _, tc := range cases {
		got := combineStatus(tc.acc, tc.next)
		if got != tc.want {
			t.Errorf("combineStatus(%q, %q) = %q, want %q", tc.acc, tc.next, got, tc.want)
		}
	}
}

// TestWavelevelStatus covers the register-status-to-wave-row-status
// mapping directly: a wave row only ever shows one of three symbols
// (done/half/empty), but a linked register item can legitimately be
// any of the register's own five (done/half/empty/dropped/pending-
// release). Confirmed missing entirely before this fix -- the prior
// version of trackingRow's own regex could not even capture a
// pending-release status, so this mapping never ran for one; this
// test exercises the mapping in isolation, independent of the regex
// fix (covered separately below).
func TestWavelevelStatus(t *testing.T) {
	cases := []struct{ in, want string }{
		{"✓", "✓"},
		{"◐", "◐"},
		{"☐", "☐"},
		{"☑", "◐"},
		{"✗", "☐"},
		{"?", "☐"},
	}
	for _, tc := range cases {
		got := wavelevelStatus(tc.in)
		if got != tc.want {
			t.Errorf("wavelevelStatus(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

const trackingFixturePendingRelease = `# Tracking

## debt

| ID | Summary | Theme | Priority | Status | Blocks |
|---|---|---|---|---|---|
| T-02 | thing | x | P1 | ☑ | — |
`

// TestSyncWaveRowsFromRegister_PendingReleaseCorrectsRowToHalf is the
// direct repro of the bug found in the 2026-09-11 session: a register
// item genuinely present in TRACKING.md at ☑ (done, pending release)
// was previously invisible to trackingRow's regex ([✓◐☐] only),
// so syncWaveRowsFromRegister treated it as found nowhere and emitted
// a spurious "found in neither" warning instead of correcting the
// row to ◐ (real progress, not yet closed).
func TestSyncWaveRowsFromRegister_PendingReleaseCorrectsRowToHalf(t *testing.T) {
	withTempProject(t, func(dir string) {
		if err := os.WriteFile(filepath.Join(dir, "docs", "TRACKING.md"), []byte(trackingFixturePendingRelease), 0644); err != nil {
			t.Fatalf("write TRACKING.md: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, "docs", "RESOLVED.md"), []byte("# Resolved\n"), 0644); err != nil {
			t.Fatalf("write RESOLVED.md: %v", err)
		}

		e, err := newEnv()
		if err != nil {
			t.Fatalf("newEnv: %v", err)
		}

		got, warnings := e.syncWaveRowsFromRegister(waveDocFixture)
		if len(warnings) != 0 {
			t.Fatalf("expected no warnings for a ☑ item genuinely present in TRACKING.md, got %v", warnings)
		}
		if got == waveDocFixture {
			t.Fatal("expected the row to be corrected from ☐ to ◐, text is unchanged")
		}

		waves := e.parseWaves(got)
		if len(waves) != 1 {
			t.Fatalf("expected 1 wave parsed from corrected text, got %d", len(waves))
		}
		if waves[0].DoneEquiv != 0.5 || waves[0].Total != 1 {
			t.Fatalf("expected the corrected wave to read as half-done (☑ maps to ◐), got %v/%v", waves[0].DoneEquiv, waves[0].Total)
		}
	})
}
