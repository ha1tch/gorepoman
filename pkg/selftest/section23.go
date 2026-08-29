package selftest

import (
	"os/exec"
	"strings"
)

// runSection23 regression-tests a real, confirmed gap: checks in this
// project's own test suites that had no business depending on gofmt
// at all -- basic substitution mechanics, a search==replace no-op,
// role-classification correctness -- were nonetheless refused under
// atomic mode whenever gofmt was genuinely absent, purely because they
// targeted a .go file with default settings and never explicitly
// disabled syntax checking. This escaped detection for as long as it
// did because every environment this suite had actually been tested
// in already had gofmt reachable via WhichGofmt's own fallback-stat
// paths (checking fixed locations directly, bypassing PATH) -- so
// REPOMAN_TEST_FORCE_NO_GOFMT, the one mechanism that genuinely
// bypasses that fallback, had only ever been exercised by the one
// check built specifically to use it (15), never by anything else
// that happened to share its dependency by accident. Found via an
// actual field report: a genuinely fresh, toolchain-free session
// reported gorepoman's own acceptance gate failing on this exact
// class of check, which the ambient-environment testing habits of
// every session working on gorepoman itself, including this one,
// had never surfaced.
func runSection23(g *gate, root string) int {
	self := g.self

	// strreplace's own embedded selftest (checks 1, 12, 13) now passes
	// cleanly with gofmt genuinely, guaranteed absent -- not by luck of
	// which fallback location this particular host happens to have it
	// at.
	cmd := exec.Command(self, "strreplace", "selftest")
	cmd.Dir = root
	cmd.Env = []string{"PATH=/usr/bin:/bin", "REPOMAN_TEST_FORCE_NO_GOFMT=1"}
	out, err := cmd.CombinedOutput()
	if !g.check(err == nil && strings.Contains(string(out), "ALL GREEN"),
		"regression: strreplace's own embedded selftest passes cleanly with gofmt "+
			"genuinely absent, not just skipped-and-silently-passing", string(out)) {
		return 1
	}

	// The adversarial search==replace no-op check (section16) targets
	// a .go file too, and had the exact same latent dependency --
	// confirmed fixed the same way, via a direct subprocess call with
	// the force-override set, matching how the original bug was
	// actually found.
	fixtureDir := root + "/section23-noop-fixture"
	mustWrite(fixtureDir+"/t.go", "package main\n")
	noopPayload := `{"v":1,"ops":[{"op":"sub","file":"t.go","search_b64":"cGFja2FnZSBtYWlu",` +
		`"replace_b64":"cGFja2FnZSBtYWlu","expect":1,"roles":["go-code"],"syntax_check":false}]}`
	cmd = exec.Command(self, "strreplace", "apply", "-")
	cmd.Dir = fixtureDir
	cmd.Env = []string{"PATH=/usr/bin:/bin", "REPOMAN_TEST_FORCE_NO_GOFMT=1"}
	cmd.Stdin = strings.NewReader(noopPayload)
	out, err = cmd.CombinedOutput()
	if !g.check(err == nil && strings.Contains(string(out), `"written": []`),
		"regression: the search==replace no-op check succeeds with gofmt genuinely absent, "+
			"not just when this host happens to have one reachable", string(out)) {
		return 1
	}

	return runSection24(g, root)
}
