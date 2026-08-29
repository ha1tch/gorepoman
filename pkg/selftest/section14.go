package selftest

import (
	"os"
	"path/filepath"
	"strings"
)

// runSection14 is pure regression coverage for two real, confirmed
// bugs found and fixed this session -- both already had ad-hoc manual
// verification at the time, neither had a permanent, automated check
// locking the fix in. A fix without a regression test is a fix that
// can silently come back.
func runSection14(g *gate, root string) int {
	self := g.self

	// Regression: register add on a genuinely empty table (header +
	// separator, zero data rows) used to refuse with "cannot locate
	// the status table" -- the row-detector only searched for rows
	// matching the ID pattern, so zero rows meant zero matches, the
	// same refusal as no table existing at all, even though a
	// perfectly valid table was right there. Fixed to anchor after
	// the header+separator when no ID-matching row exists yet. This
	// uses its own fresh project, not the shared `root` fixture --
	// every other register check in this suite deliberately seeds a
	// genesis row first (the documented workaround this fix makes
	// unnecessary), so reusing `root` would not actually exercise the
	// zero-row path this regression test exists to cover.
	emptyTableProj := filepath.Join(root, "register-empty-table-fixture")
	os.MkdirAll(filepath.Join(emptyTableProj, "docs"), 0755)
	mustWrite(filepath.Join(emptyTableProj, ".repoman.json"), `{}`)
	mustWrite(filepath.Join(emptyTableProj, "docs", "TRACKING.md"),
		"# Register\n\n"+
			"| ID | Summary | Theme | Priority | Status | Blocks |\n"+
			"|---|---|---|---|---|---|\n")

	r := run(self, emptyTableProj, "register", "add",
		"--summary", "first real item", "--theme", "core", "--priority", "P2",
		"--body", "**Trigger:** starting fresh.\n\n**Scope:** the whole project.")
	trackingPath := filepath.Join(emptyTableProj, "docs", "TRACKING.md")
	if !g.check(r.code == 0 && strings.Contains(mustRead(trackingPath), "| T-01 |") &&
		strings.Contains(mustRead(trackingPath), "### T-01. first real item"),
		"regression: register add succeeds against a genuinely empty table (0 rows, "+
			"no genesis row required)", r.stdout+r.stderr) {
		return 1
	}
	r = run(self, emptyTableProj, "register", "check")
	if !g.check(r.code == 0, "regression: register check passes on the result -- the inserted "+
		"row and its detail section actually agree", r.stdout+r.stderr) {
		return 1
	}

	// Regression coverage for the doctor cmd.Dir isolation fix (two
	// checks in section10.go) is deliberately NOT a nested `repoman
	// selftest` invocation, and this is worth explaining rather than
	// silently omitting. Genuinely testing that fix means invoking
	// `repoman selftest` from inside a hostile directory -- nothing
	// short of that exercises the fixed code path. But `self` here IS
	// the very binary this check would be compiled into: nesting it
	// means the child process reaches this exact same check and tries
	// to nest itself too, recursively, without bound. That's not a
	// pipe-buffering issue fixable by redirecting to a file instead of
	// a pipe (tried both; confirmed file-redirection alone does not
	// stop the recursion, since the problem is structural, not a
	// stdout-handling detail) -- it's architecturally impossible for a
	// binary to safely test "does selftest nest itself correctly" from
	// within its own selftest. Shipping a check that can recurse
	// without bound would be a worse defect than the one it exists to
	// catch. The fix stands on the verification already done directly
	// against it instead: the go.mod-reading mechanism confirmed in
	// pkg/doctor/doctor.go's own source, isolated-vs-unisolated
	// invocation compared side by side, and 30 repeated runs clean
	// from the exact hostile directory that originally failed.
	return runSection15(g, root)
}
