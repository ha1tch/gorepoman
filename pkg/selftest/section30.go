package selftest

import "fmt"

// runSection30 is the true tail of the selftest chain: prints the
// final summary line, listing any deferred checks (tests that need a
// tool this bootstrap deliberately doesn't require) separately from
// a plain "all N checks green" when there are none. Relocated here
// from the end of runSection29 when runSection29a was inserted ahead
// of it -- the summary itself is unchanged, only which function ends
// the chain.
func runSection30(g *gate, root string) int {
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
