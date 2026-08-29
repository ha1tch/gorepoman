package selftest

import (
	"fmt"
	"strings"
)

// runSection17 regression-tests the help-flag fix: a fresh session
// hit "unknown command" on help, --help, and -help in turn at the top
// level before eventually discovering -h worked, and even then only
// once tried against a specific subcommand rather than bare. All four
// spellings are now recognized at the top level, and the two a
// subcommand's own parser didn't already handle (bare "help",
// single-dash "-help") are normalized to "-h" before dispatch, so
// every subcommand gets all four without any change to its own logic.
func runSection17(g *gate, root string) int {
	self := g.self

	for _, form := range []string{"help", "-h", "--help", "-help"} {
		r := run(self, root, form)
		if !g.check(r.code == 0 && strings.Contains(r.stdout, "Usage: repoman <command>"),
			fmt.Sprintf("regression: top-level `repoman %s` shows usage and exits 0, "+
				"not \"unknown command\"", form), r.stdout+r.stderr) {
			return 1
		}
	}

	// The two forms a subcommand's own parser never recognized on its
	// own (bare "help", single-dash "-help") must now reach it as "-h"
	// via the top-level normalization -- checked against a subcommand
	// whose own parser was, and remains, untouched.
	for _, form := range []string{"help", "-help"} {
		r := run(self, root, "addwave", form)
		if !g.check(r.code == 0 && strings.Contains(r.stdout, "usage: repoman addwave"),
			fmt.Sprintf("regression: `repoman addwave %s` reaches the subcommand's own -h "+
				"handling via normalization, not \"unknown\" or a parse error", form), r.stdout+r.stderr) {
			return 1
		}
	}
	// The two forms the subcommand already handled natively must be
	// unaffected by the change -- confirms the normalization only ever
	// adds coverage, never disturbs what already worked.
	for _, form := range []string{"-h", "--help"} {
		r := run(self, root, "addwave", form)
		if !g.check(r.code == 0 && strings.Contains(r.stdout, "usage: repoman addwave"),
			fmt.Sprintf("regression: `repoman addwave %s`, already correct before this fix, "+
				"is unaffected by it", form), r.stdout+r.stderr) {
			return 1
		}
	}

	return runSection18(g, root)
}
