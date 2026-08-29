package selftest

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
)

// runSection21 regression-tests --brief: a per-call override for the
// case REPOMAN_NO_WEB_HELP doesn't fit well -- a session that hasn't
// made the standing "always terse" decision but wants one specific
// call not to wait on a fetch or print a full chapter. Three
// properties matter, each getting its own check: --brief genuinely
// suppresses the fetch (not just happens to fail fast); it works
// regardless of whether it appears before or after -h/--help (several
// commands dispatch strictly on argv[0], so this needed an actual
// fix, not just a documentation note); and the mechanism itself is
// named in the embedded help text every command already prints, so a
// session with no prior exposure to this tool can discover it without
// needing to already know it exists.
//
// Like runSection20, the suppression check deliberately calls
// exec.Command directly rather than going through this file's own
// run()/runWithEnv() helpers, which unconditionally set
// REPOMAN_NO_WEB_HELP via noWebHelpEnv() -- exactly right for every
// other check in this suite, but it would make it impossible to
// distinguish "--brief suppressed this" from "the env var did".
func runSection21(g *gate, root string) int {
	self := g.self

	// --brief genuinely suppresses the fetch, confirmed against a
	// real local server rather than an address nothing answers --
	// the point here is specifically that content WOULD have been
	// available and --brief still skipped it, not merely that nothing
	// went wrong.
	const fakeChapter = "# A fake chapter\n\nJust for this regression test.\n"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(fakeChapter))
	}))
	defer ts.Close()

	env := []string{
		"PATH=/usr/bin:/bin",
		"REPOMAN_WEBHELP_BASE_URL=" + ts.URL + "/docs/",
	}

	cmd := exec.Command(self, "ed", "-h", "--brief")
	cmd.Dir = root
	cmd.Env = env
	out, _ := cmd.CombinedOutput()
	if !g.check(strings.Contains(string(out), "usage: repoman ed") &&
		!strings.Contains(string(out), "Full chapter") &&
		!strings.Contains(string(out), "A fake chapter"),
		"regression: --brief suppresses a fetch that would genuinely have succeeded, "+
			"confirmed against a live local server, not just an unreachable address", string(out)) {
		return 1
	}

	// Order independence: several commands dispatch strictly on
	// argv[0] (`switch args[0] { case "-h", "--help": ... }`), so
	// `--brief -h` originally failed as an unrecognized command while
	// `-h --brief` worked. Both orders, across a command using each
	// dispatch style, must now produce real help.
	orderCases := [][]string{
		{"ed", "-h", "--brief"},
		{"ed", "--brief", "-h"},
		{"register", "-h", "--brief"},
		{"register", "--brief", "-h"},
	}
	for _, args := range orderCases {
		cmd := exec.Command(self, args...)
		cmd.Dir = root
		cmd.Env = env
		out, _ := cmd.CombinedOutput()
		if !g.check(strings.Contains(string(out), "usage: repoman") &&
			!strings.Contains(string(out), "unknown command"),
			fmt.Sprintf("regression: `repoman %s` shows real help regardless of --brief/-h "+
				"order, not \"unknown command\"", strings.Join(args, " ")), string(out)) {
			return 1
		}
	}

	// The mechanism is self-discoverable: named in the embedded help
	// text itself, not only in source comments or the changelog --
	// checked here by content, not by trusting that it was written
	// down somewhere.
	r := run(self, root, "ed", "-h")
	if !g.check(strings.Contains(r.stdout, "REPOMAN_NO_WEB_HELP") && strings.Contains(r.stdout, "--brief"),
		"regression: both suppression mechanisms are named in the embedded help text itself, "+
			"discoverable without prior knowledge of either one", r.stdout) {
		return 1
	}

	return runSection22(g, root)
}
