package selftest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// runSection28 regression-tests T-03: relcore's mandatory provenance
// pre-flight, wired in immediately alongside runBadcodePreflight and
// exercised the same way section13.go already exercises that gate --
// a real out-of-band edit must block before any release.steps entry
// runs, --resume must not bypass it, and the release must proceed
// once the mismatch is actually resolved (sanctioned, in this case --
// provenance has no "fix" equivalent to badcode's "remove the leaked
// content", since sanction *is* the accept-this-edit remedy T-02
// built for exactly this). A synthetic project entirely separate from
// the shared `root` fixture, same reasoning as section13: relcore
// needs a real .repoman.json + release.steps of its own here, and
// this section should not disturb the shared fixture's own journal
// state other sections depend on.
func runSection28(g *gate, root string) int {
	self := g.self

	proj := filepath.Join(root, "relcore-provenance-fixture")
	os.MkdirAll(proj, 0755)
	mustWrite(filepath.Join(proj, ".repoman.json"),
		`{"release": {"steps": [{"name": "noop", "run": "true", "always": true}]}}`)
	mustWrite(filepath.Join(proj, "VERSION"), "0.1.0\n")
	mustWrite(filepath.Join(proj, "tracked.txt"), "hello\n")

	// No badcode config in this fixture -- isolate it from whatever
	// REPOMAN_BADCODE_DIR the ambient environment happens to have, the
	// same way section22 does, so a real leftover config elsewhere in
	// this sandbox can't make this section's badcode gate fail instead
	// of passing through to the provenance gate this section actually
	// tests.
	emptyBadcodeDir := filepath.Join(filepath.Dir(root), "badcode-config-relcore-provenance-fixture")
	env := filterAndSet(os.Environ(), "REPOMAN_BADCODE_DIR", emptyBadcodeDir)
	run := func(args ...string) runResult { return runWithEnv(self, proj, env, args...) }

	// Track tracked.txt through repoman's own write path first --
	// nothing to check provenance against until a journal entry exists.
	r := run("ed", "append", "tracked.txt", "--with", "journal line\n")
	if !g.check(r.code == 0, "relcore/provenance: setup -- ed append tracks tracked.txt", r.stdout+r.stderr) {
		return 1
	}

	// Clean state: relcore proceeds through the real step, and the
	// provenance gate's own "ok" line is visible in the output.
	r = run("relcore", "0.1.0")
	if !g.check(r.code == 0 && strings.Contains(r.stdout, "ok provenance") &&
		strings.Contains(r.stdout, "-- noop") && strings.Contains(r.stdout, "release v0.1.0 prepared"),
		"relcore/provenance: a clean, fully-tracked project proceeds through the real "+
			"release step, with the provenance gate's own ok line visible", r.stdout+r.stderr) {
		return 1
	}

	// Now the actual failure mode: edit tracked.txt outside repoman
	// entirely (a plain file write, not through ed/strreplace). Must
	// block before the "noop" step, and before any release-in-progress
	// output, exactly the same shape section13 requires of badcode.
	mustWrite(filepath.Join(proj, "tracked.txt"), "tampered outside repoman\n")
	r = run("relcore", "0.2.0")
	if !g.check(r.code == 1 && strings.Contains(r.stdout, "FAIL provenance") &&
		strings.Contains(r.stdout, "provenance-mismatch") &&
		!strings.Contains(r.stdout, "-- noop") && !strings.Contains(r.stdout, "release v0.2.0 prepared"),
		"relcore/provenance: an unsanctioned out-of-band edit blocks before any "+
			"release.steps entry runs at all", r.stdout+r.stderr) {
		return 1
	}

	// --resume must not bypass it -- provenance is not a release.steps
	// entry and is never consulted through journal.green, same as
	// badcode's own pre-flight.
	r = run("relcore", "0.2.0", "--resume")
	if !g.check(r.code == 1 && strings.Contains(r.stdout, "FAIL provenance"),
		"relcore/provenance: --resume does not bypass a real provenance mismatch -- it is "+
			"not part of the resumable-steps journal at all, so there is nothing for "+
			"--resume to skip", r.stdout+r.stderr) {
		return 1
	}

	// Sanction it -- the actual remedy this gate is built around, not
	// a workaround: confirms the release now proceeds.
	r = run("provenance", "sanction", "tracked.txt", "--reason", "regression fixture: expected tamper")
	if !g.check(r.code == 0, "relcore/provenance: sanctioning the mismatch succeeds", r.stdout+r.stderr) {
		return 1
	}
	r = run("relcore", "0.2.0")
	if !g.check(r.code == 0 && strings.Contains(r.stdout, "ok provenance") &&
		strings.Contains(r.stdout, "-- noop") && strings.Contains(r.stdout, "release v0.2.0 prepared"),
		"relcore/provenance: once the mismatch is sanctioned, the release proceeds -- the "+
			"gate blocks a real unaccounted-for edit, not the release mechanism itself",
		r.stdout+r.stderr) {
		return 1
	}

	// A project that has never used ed/strreplace at all (nothing
	// tracked, ever) is a soft pass here too, same principle as
	// badcode's own no-config soft pass: there is nothing this gate
	// could have caught yet, so it does not manufacture a failure out
	// of having nothing to check.
	proj2 := filepath.Join(root, "relcore-provenance-untracked-fixture")
	os.MkdirAll(proj2, 0755)
	mustWrite(filepath.Join(proj2, ".repoman.json"),
		`{"release": {"steps": [{"name": "noop", "run": "true", "always": true}]}}`)
	mustWrite(filepath.Join(proj2, "VERSION"), "0.1.0\n")
	mustWrite(filepath.Join(proj2, "untouched.txt"), "never seen by repoman\n")
	r = runWithEnv(self, proj2, env, "relcore", "0.1.0")
	if !g.check(r.code == 0 && strings.Contains(r.stdout, "ok provenance") &&
		strings.Contains(r.stdout, "release v0.1.0 prepared"),
		"relcore/provenance: a project with nothing ever tracked by repoman is a soft pass "+
			"too -- there is nothing this gate could have caught yet", r.stdout+r.stderr) {
		return 1
	}

	if len(g.deferred) > 0 {
		fmt.Printf("selftest: all %d checks green (%d deferred -- optional toolchain missing)\n",
			g.checks, len(g.deferred))
		fmt.Println()
		fmt.Println("Deferred checks are not failures -- each needs a tool this bootstrap")
		fmt.Println("deliberately does not require. Install it, then re-run `repoman selftest`")
		fmt.Println("for full coverage:")
		seen := map[string]bool{}
		for _, reason := range g.deferred {
			if seen[reason] {
				continue
			}
			seen[reason] = true
			fmt.Println("  - " + reason)
		}
	} else {
		fmt.Printf("selftest: all %d checks green\n", g.checks)
	}
	return 0
}
