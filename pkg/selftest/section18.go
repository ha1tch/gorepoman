package selftest

import (
	"fmt"
	"strings"
)

// runSection18 regression-tests the two discoverability fixes: a
// fresh session, asked to move debt tickets into a new wave, found
// nothing pointing it at how -- `waveprogress -h` produced no help at
// all (it fell straight through to normal execution and printed an
// unrelated runtime error instead), and `addwave -h`, while thorough,
// never said in so many words that an existing register ticket is
// associated with a wave via --items-json rather than some separate
// "move" command.
func runSection18(g *gate, root string) int {
	self := g.self

	// The core bug: waveprogress -h/--help used to fall through to
	// real execution (which then failed with an unrelated "no
	// wave-tracking document" error, since this fixture has none
	// configured) instead of ever printing help text.
	for _, form := range []string{"-h", "--help"} {
		r := run(self, root, "waveprogress", form)
		if !g.check(r.code == 0 && strings.Contains(r.stdout, "usage: repoman waveprogress") &&
			!strings.Contains(r.stdout+r.stderr, "no wave-tracking document"),
			fmt.Sprintf("regression: `repoman waveprogress %s` prints help and exits 0, "+
				"rather than falling through to real execution and failing", form),
			r.stdout+r.stderr) {
			return 1
		}
	}
	// The help text itself explains the actual relationship to the
	// register, not just the flag list -- confirmed by content, not
	// merely a successful exit code.
	r := run(self, root, "waveprogress", "-h")
	if !g.check(strings.Contains(r.stdout, "register_item") &&
		strings.Contains(r.stdout, "repoman-080-waves.md"),
		"regression: waveprogress's help explains the register_item relationship and "+
			"points at the full worked example, not just the flag list", r.stdout) {
		return 1
	}

	// addwave's help now says explicitly that an existing ticket is
	// attached via --items-json, not some separate command -- this is
	// the actual gap a fresh session hit, not just a missing feature.
	r = run(self, root, "addwave", "-h")
	if !g.check(strings.Contains(r.stdout, "no separate command for putting an EXISTING register") &&
		strings.Contains(r.stdout, "repoman-080-waves.md"),
		"regression: addwave's help explicitly states there is no separate \"move a ticket "+
			"into a wave\" command, and points at the full worked example", r.stdout) {
		return 1
	}

	return runSection19(g, root)
}
