package selftest

import (
	"fmt"
	"strings"
)

// runSection22 regression-tests the badcode-config-recreation
// reminder added at three points where it actually matters -- found
// worth adding after a session correctly recreated a project's
// badcode config from its own memory, with nothing in gorepoman
// itself prompting that step beyond a bare "0 patterns configured"
// count. The actual patterns can never live in this repository (the
// entire point of keeping the config out-of-band), but the
// PROCEDURAL reminder to check for and recreate one can, and now
// does, at the moment each surface would naturally be seen: the
// runtime WARN itself, the top-level workflow walkthrough, and
// relcore's own help, not only in the docs chapter that already had
// this said once.
func runSection22(g *gate, root string) int {
	self := g.self

	// Explicit, guaranteed-empty config directory -- this sandbox has
	// had a real badcode config configured for manual testing earlier
	// in this same session, so relying on the ambient environment
	// having none would be testing whatever happens to be left over,
	// not the actual no-config code path.
	emptyConfigDir := root + "/badcode-priming-empty-fixture"
	env := append(append([]string{}, "PATH=/usr/bin:/bin"), "REPOMAN_BADCODE_DIR="+emptyConfigDir)
	r := runWithEnv(self, root, env, "badcode", "check", ".")
	if !g.check(r.code == 0 && strings.Contains(r.stdout, "WARN no badcode patterns configured") &&
		strings.Contains(r.stdout, "check your own") &&
		strings.Contains(r.stdout, "notes/memory") &&
		strings.Contains(r.stdout, "deliberately never stored"),
		"regression: the no-config WARN itself names the actionable next step (check your "+
			"own notes/memory) rather than only stating the count is zero", r.stdout) {
		return 1
	}

	r = run(self, root, "help")
	if !g.check(strings.Contains(r.stdout, "check your own notes/memory and recreate it before") &&
		strings.Contains(r.stdout, "release work"),
		"regression: the top-level workflow's releasing step names the same reminder, not "+
			"just \"relcore runs badcode first\"", r.stdout) {
		return 1
	}

	r = run(self, root, "relcore", "-h")
	if !g.check(strings.Contains(r.stdout, "check your own notes/memory and recreate it before "+
		"running this"),
		"regression: relcore's own help names the same reminder at the point someone is "+
			"actually about to run a release", r.stdout) {
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
