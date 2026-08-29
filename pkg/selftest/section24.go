package selftest

import (
	"fmt"
	"os/exec"
	"strings"
)

// runSection24 regression-tests the doctor LiveGoVersion fix: a real,
// confirmed field incident, not a hypothetical one. `installedGoVersion`
// used to structurally never reach its own live-PATH fallback branch
// (runtime.Version() is never empty for a compiled Go binary, so the
// exec.LookPath("go") branch beneath it could never execute) -- meaning
// doctor's single "Go X.Y.Z" line always reported this binary's own
// compile-time version and nothing else, with no way to tell the two
// apart. That produced the same real confusion, independently, more
// than once: a session reading "Go 1.21.13" concluding something about
// its own environment's toolchain, when the number had no relationship
// to what's actually on PATH there at all. Fixed by reporting both
// facts as two separate, clearly-labeled lines.
func runSection24(g *gate, root string) int {
	self := g.self

	// With a real go toolchain reachable, both lines should reflect it
	// -- the binary's own compile-time version (fixed, from when this
	// binary was built) and a separately-confirmed live version.
	if _, err := exec.LookPath("go"); err == nil {
		cmd := exec.Command(self, "doctor", "--quiet")
		cmd.Dir = root
		out, _ := cmd.CombinedOutput()
		if !g.check(strings.Contains(string(out), "This binary's own Go") &&
			strings.Contains(string(out), "A live go toolchain is also on PATH"),
			"regression: with a real toolchain present, doctor reports both this binary's "+
				"own compile-time Go version and a separately-confirmed live one, not one "+
				"conflated line", string(out)) {
			return 1
		}
	}

	// With no live toolchain at all, the binary's own version still
	// reports (it's compiled in, not a PATH lookup), but the live line
	// must clearly say none was found -- not silently omit itself, and
	// not claim a version it never confirmed. A guaranteed-nonexistent
	// directory, not a small allowlist like /usr/bin:/bin -- confirmed
	// as a real field incident, not a hypothetical risk: GitHub's own
	// Ubuntu runner images ship a real go toolchain reachable from
	// exactly those directories, which this exact check's first
	// version used and which broke it in CI on the very first run,
	// despite passing cleanly in every sandbox this had been tested in
	// up to that point.
	cmd := exec.Command(self, "doctor", "--quiet")
	cmd.Dir = root
	cmd.Env = []string{"PATH=/nonexistent-empty-path"}
	out, _ := cmd.CombinedOutput()
	if !g.check(strings.Contains(string(out), "This binary's own Go") &&
		strings.Contains(string(out), "No live go toolchain on PATH"),
		"regression: with no live toolchain at all, doctor still reports this binary's own "+
			"compile-time version, but clearly states no live one was found rather than "+
			"conflating the two or silently omitting the line", string(out)) {
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
