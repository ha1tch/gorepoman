package selftest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// runSection15 regression-tests the badcode cross-line-boundary fix:
// a forbidden pattern split across an ordinary line wrap ("TOPSECRET"
// at the end of one line, "VALUE" starting the next) used to sail
// straight past the check entirely, because matching was purely
// per-line. An accidental line wrap is enough to create this by
// complete accident, not just deliberate evasion -- exactly the class
// of gap a mandatory, unconditional release gate has no business
// leaving open. Fixed by additionally checking a newline-stripped fold
// of the text, mapped back to real line numbers, alongside the
// original per-line pass (which still runs first and is unchanged).
func runSection15(g *gate, root string) int {
	self := g.self

	badcodeConfigDir := filepath.Join(filepath.Dir(root), "badcode-config-crossline-fixture")
	os.MkdirAll(badcodeConfigDir, 0755)
	mustWrite(filepath.Join(badcodeConfigDir, "badcode.txt"), "TOPSECRETVALUE\n")

	srcDir := filepath.Join(root, "badcode-crossline-src")
	os.MkdirAll(srcDir, 0755)

	env := filterAndSet(os.Environ(), "REPOMAN_BADCODE_DIR", badcodeConfigDir)
	run := func(args ...string) runResult { return runWithEnv(self, srcDir, env, args...) }

	// The actual regression: pattern split across exactly one line
	// wrap, no space at the join point (the case that originally
	// slipped through completely).
	mustWrite(filepath.Join(srcDir, "leak.txt"), "some text TOPSECRET\nVALUE more text\n")
	r := run("badcode", "check", ".")
	if !g.check(r.code == 1 && strings.Contains(r.stderr, "spans lines 1-2") &&
		strings.Contains(r.stderr, "pattern split across a line wrap"),
		"regression: badcode catches a forbidden pattern split across an ordinary line wrap, "+
			"not just when it happens to land on a single line", r.stdout+r.stderr) {
		return 1
	}
	os.Remove(filepath.Join(srcDir, "leak.txt"))

	// A wider span (3 lines) -- confirms the fix isn't hardcoded to a
	// 2-line assumption.
	mustWrite(filepath.Join(srcDir, "wide.txt"), "prefix TOP\nSECRET\nVALUE suffix\n")
	r = run("badcode", "check", ".")
	if !g.check(r.code == 1 && strings.Contains(r.stderr, "spans lines 1-3"),
		"regression: a pattern split across three lines is caught too, not just a single wrap",
		r.stdout+r.stderr) {
		return 1
	}
	os.Remove(filepath.Join(srcDir, "wide.txt"))

	// The false-positive guard: two lines that are individually
	// unrelated, with nothing forbidden anywhere, must not trip the
	// fold pass just because adjacent lines exist.
	mustWrite(filepath.Join(srcDir, "clean.txt"), "this is completely unrelated\ntext with no forbidden content at all\n")
	r = run("badcode", "check", ".")
	if !g.check(r.code == 0 && strings.Contains(r.stdout, "BADCODE CHECK OK"),
		"regression: ordinary adjacent lines with no real match present are not "+
			"false-flagged by the fold pass", r.stdout+r.stderr) {
		return 1
	}
	os.Remove(filepath.Join(srcDir, "clean.txt"))

	// A match entirely on one line must be reported exactly once, not
	// twice (once by the per-line pass, once by the fold pass) -- the
	// startLine == endLine guard in the fold pass exists specifically
	// to prevent this double-count.
	mustWrite(filepath.Join(srcDir, "sameline.txt"), "this line has TOPSECRETVALUE entirely on it\n")
	r = run("badcode", "check", ".")
	matchCount := strings.Count(r.stderr, "badcode-match")
	if !g.check(r.code == 1 && matchCount == 1,
		"regression: a same-line match is reported exactly once, not double-counted between "+
			"the per-line pass and the fold pass", fmt.Sprintf("matchCount=%d: %s", matchCount, r.stdout+r.stderr)) {
		return 1
	}

	return runSection16(g, root)
}
