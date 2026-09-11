package selftest

import (
	"os"
	"path/filepath"
	"strings"
)

// runSection29a regression-tests the addwave self-heal fix (the
// docs/UX half of the old B-05 report, re-verified 2026-09-11): a
// project's very first `addwave` call, with WAVE_TRACKING.md and
// WAVE_PLAN.md not yet created, must now succeed by creating a
// minimal skeleton for both -- rather than refusing with a message
// naming an undocumented manual-seed requirement, which is what
// happened before this fix (destructive-on-partial-write B-05 was
// already fixed; this section covers the remaining refuse-instead-
// of-self-heal gap). Uses its own fresh project directory, never
// touching docs/WAVE_TRACKING.md or docs/WAVE_PLAN.md under the
// shared `root` fixture, which other sections depend on already
// existing in their pre-seeded shape.
func runSection29a(g *gate, root string) int {
	self := g.self

	proj := filepath.Join(root, "addwave-selfheal-fixture")
	os.MkdirAll(filepath.Join(proj, "docs"), 0755)
	mustWrite(filepath.Join(proj, ".repoman.json"), `{}`)
	mustWrite(filepath.Join(proj, "docs", "TRACKING.md"),
		"# Tracking\n\n"+
			"| ID | Summary | Theme | Priority | Status | Blocks |\n"+
			"|----|---------|-------|----------|--------|--------|\n\n")
	mustWrite(filepath.Join(proj, "docs", "RESOLVED.md"), "# Resolved\n\nClosed items, newest first.\n")

	waveTrackingPath := filepath.Join(proj, "docs", "WAVE_TRACKING.md")
	wavePlanPath := filepath.Join(proj, "docs", "WAVE_PLAN.md")

	// Confirm the fixture genuinely starts without either file --
	// the point of this section only holds if neither exists yet.
	if !g.check(!fileExists(waveTrackingPath) && !fileExists(wavePlanPath),
		"addwave-selfheal: fixture genuinely starts with neither WAVE_TRACKING.md nor "+
			"WAVE_PLAN.md present", "") {
		return 1
	}

	r := run(self, proj, "register", "add",
		"--summary", "needs a wave", "--theme", "core", "--priority", "P1",
		"--body", "- **Trigger:** fixture.\n\n- **Scope:** fixture.")
	if !g.check(r.code == 0 && strings.Contains(r.stdout, "T-01"),
		"addwave-selfheal: register add for the fixture item succeeds", r.stdout+r.stderr) {
		return 1
	}

	// The actual regression check: addwave against a project with
	// neither file present must succeed (exit 0), not refuse.
	r = run(self, proj, "addwave", "--name", "first wave", "--ideal-days", "1",
		"--items-json", `[{"summary":"needs a wave","register_item":"T-01"}]`,
		"--plan-note", "fixture")
	if !g.check(r.code == 0,
		"addwave-selfheal: addwave succeeds on a project with neither wave document "+
			"present yet, rather than refusing", r.stdout+r.stderr) {
		return 1
	}
	if !g.check(strings.Contains(r.stdout, "created") && strings.Contains(r.stdout, "WAVE_TRACKING.md") &&
		strings.Contains(r.stdout, "WAVE_PLAN.md"),
		"addwave-selfheal: addwave's own output names both files it created, not a silent "+
			"success", r.stdout) {
		return 1
	}

	// Both files must actually exist now, with real, usable content --
	// not just an exit code claiming success.
	if !g.check(fileExists(waveTrackingPath) && fileExists(wavePlanPath),
		"addwave-selfheal: both WAVE_TRACKING.md and WAVE_PLAN.md exist on disk after "+
			"the call", "") {
		return 1
	}
	trackingContent := mustRead(waveTrackingPath)
	if !g.check(strings.Contains(trackingContent, "## 1. Progress at a glance") &&
		strings.Contains(trackingContent, "Overall by item count:") &&
		strings.Contains(trackingContent, "### Wave 1 — first wave") &&
		strings.Contains(trackingContent, "T-01"),
		"addwave-selfheal: the self-healed WAVE_TRACKING.md carries both the required "+
			"skeleton section and the new wave's own table, in the same document", trackingContent) {
		return 1
	}

	// waveprogress must already agree with what addwave just wrote --
	// confirms the skeleton addwave creates is genuinely the same
	// shape waveprogress's own reader expects, not a look-alike that
	// happens to satisfy addwave's own checks only.
	r = run(self, proj, "waveprogress", "--check")
	if !g.check(r.code == 0,
		"addwave-selfheal: waveprogress --check finds the self-healed document already "+
			"up to date, confirming addwave's skeleton matches what waveprogress itself expects",
		r.stdout+r.stderr) {
		return 1
	}

	// A second, independent fresh project confirms --dry-run creates
	// nothing on disk even when both files are missing, while still
	// producing a coherent preview rather than crashing on the read
	// it can no longer do from disk.
	dryProj := filepath.Join(root, "addwave-selfheal-dryrun-fixture")
	os.MkdirAll(filepath.Join(dryProj, "docs"), 0755)
	mustWrite(filepath.Join(dryProj, ".repoman.json"), `{}`)
	mustWrite(filepath.Join(dryProj, "docs", "TRACKING.md"),
		"# Tracking\n\n"+
			"| ID | Summary | Theme | Priority | Status | Blocks |\n"+
			"|----|---------|-------|----------|--------|--------|\n\n")
	mustWrite(filepath.Join(dryProj, "docs", "RESOLVED.md"), "# Resolved\n\nClosed items, newest first.\n")

	r = run(self, dryProj, "register", "add",
		"--summary", "needs a wave", "--theme", "core", "--priority", "P1",
		"--body", "- **Trigger:** fixture.\n\n- **Scope:** fixture.")
	if !g.check(r.code == 0 && strings.Contains(r.stdout, "T-01"),
		"addwave-selfheal: register add for the dry-run fixture succeeds", r.stdout+r.stderr) {
		return 1
	}

	r = run(self, dryProj, "addwave", "--name", "first wave", "--ideal-days", "1",
		"--items-json", `[{"summary":"needs a wave","register_item":"T-01"}]`,
		"--plan-note", "fixture", "--dry-run")
	if !g.check(r.code == 0 && strings.Contains(r.stdout, "nothing written"),
		"addwave-selfheal: --dry-run against a project missing both wave documents "+
			"succeeds with a coherent preview, not a crash", r.stdout+r.stderr) {
		return 1
	}
	if !g.check(!fileExists(filepath.Join(dryProj, "docs", "WAVE_TRACKING.md")) &&
		!fileExists(filepath.Join(dryProj, "docs", "WAVE_PLAN.md")),
		"addwave-selfheal: --dry-run genuinely creates neither file on disk, even though "+
			"both were missing", "") {
		return 1
	}

	return runSection30(g, root)
}
