package selftest

import (
	"fmt"
	"strings"
)

// runSection19 regression-tests the second round of discoverability
// work: four more subcommands turned out to have zero help handling
// at all (roles ran a real scan treating "-h" as its search term;
// syncver rejected it as an unknown command; doctor and selftest both
// silently ran their real, full operations -- selftest running the
// entire suite being the worst of the four), plus the new top-level
// "Typical workflow" section and the docs URLs added across every
// subcommand's help text.
func runSection19(g *gate, root string) int {
	self := g.self

	// The four subcommands that previously had no help handling at
	// all now show real help and exit 0, without performing their
	// real operation as a side effect.
	cases := []struct {
		cmd, mustContain, mustNotContain string
	}{
		{"roles", "usage: repoman roles", "roles present:"},
		{"syncver", "usage: repoman syncver", "unknown command"},
		{"doctor", "usage: repoman doctor", "Platform:"},
		{"selftest", "usage: repoman selftest", "-- acceptance gate --"},
	}
	for _, c := range cases {
		r := run(self, root, c.cmd, "-h")
		if !g.check(r.code == 0 && strings.Contains(r.stdout, c.mustContain) &&
			!strings.Contains(r.stdout+r.stderr, c.mustNotContain),
			fmt.Sprintf("regression: `repoman %s -h` shows real help and exits 0, without "+
				"performing %s's actual operation as a side effect", c.cmd, c.cmd),
			r.stdout+r.stderr) {
			return 1
		}
	}

	// The top-level workflow section is present and mentions the
	// actual sequence, not just a bare command list.
	r := run(self, root, "help")
	if !g.check(strings.Contains(r.stdout, "Typical workflow") &&
		strings.Contains(r.stdout, "register_item") &&
		strings.Contains(r.stdout, "https://ha1tch.github.io/gorepoman/"),
		"regression: top-level help includes a typical-workflow walkthrough and the full "+
			"docs URL, not just the bare command list", r.stdout) {
		return 1
	}

	// Spot-check URL references landed in a representative sample of
	// subcommand help texts, not just the two that already had one
	// before this round. badcode's URL lives on the deeper `check`
	// subcommand's help, not the bare top-level one -- confirmed
	// directly before writing this case, since the two show different
	// text and only one carries the link.
	urlCases := []struct {
		args            []string
		wantURLFragment string
	}{
		{[]string{"ed", "-h"}, "repoman-040-editing.html"},
		{[]string{"register", "-h"}, "repoman-060-register-and-guards.html"},
		{[]string{"relcore", "-h"}, "repoman-070-releases.html"},
		{[]string{"badcode", "check", "-h"}, "repoman-065-badcode.html"},
	}
	for _, c := range urlCases {
		r := run(self, root, c.args...)
		if !g.check(strings.Contains(r.stdout, c.wantURLFragment),
			fmt.Sprintf("regression: `repoman %s` links to its matching docs chapter",
				strings.Join(c.args, " ")), r.stdout) {
			return 1
		}
	}

	return runSection20(g, root)
}
