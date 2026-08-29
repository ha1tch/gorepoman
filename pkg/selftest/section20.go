package selftest

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
)

// runSection20 regression-tests pkg/webhelp, the optional live-fetch
// addition to -h/--help output. Three properties matter here, and
// each gets its own check rather than being inferred from the others:
// REPOMAN_NO_WEB_HELP genuinely suppresses the attempt (not just
// "happens to fail fast"); an unreachable host produces the embedded
// help text with nothing extra, never an error or a hang; and, when a
// fetch genuinely succeeds, the content actually appears, correctly
// delineated from the embedded text.
//
// The success-path check deliberately does not go through this
// package's own run()/runWithEnv() helpers -- both unconditionally
// set REPOMAN_NO_WEB_HELP via noWebHelpEnv(), which is exactly right
// for every other check in this suite but would make it impossible to
// ever exercise the real fetch code path here. This is the one
// intentional exception, calling exec.Command directly with an
// explicit, minimal environment that deliberately omits that
// variable -- pointed at a local httptest.Server, not the real
// internet, so it stays fully deterministic and network-independent
// despite exercising genuinely live HTTP code.
func runSection20(g *gate, root string) int {
	self := g.self

	// REPOMAN_NO_WEB_HELP genuinely suppresses the attempt. Pointed at
	// an address nothing is listening on (rather than a real
	// unreachable host) so a passing check here means the variable
	// itself was honored, not that the network happened to be slow.
	cmd := exec.Command(self, "ed", "-h")
	cmd.Dir = root
	cmd.Env = []string{
		"PATH=/usr/bin:/bin",
		"REPOMAN_NO_WEB_HELP=1",
		"REPOMAN_WEBHELP_BASE_URL=http://127.0.0.1:1/docs/",
	}
	out, _ := cmd.CombinedOutput()
	if !g.check(strings.Contains(string(out), "usage: repoman ed") &&
		!strings.Contains(string(out), "Full chapter"),
		"regression: REPOMAN_NO_WEB_HELP genuinely suppresses the fetch attempt, even when "+
			"pointed at an address that would otherwise be tried", string(out)) {
		return 1
	}

	// An unreachable host, WITHOUT the suppression variable set,
	// produces the embedded help text with nothing extra -- graceful
	// failure, not an error printed, not a hang. Port 1 is used
	// deliberately: nothing listens there, so this fails immediately
	// rather than waiting out the real timeout, keeping this check
	// itself fast.
	cmd = exec.Command(self, "ed", "-h")
	cmd.Dir = root
	cmd.Env = []string{
		"PATH=/usr/bin:/bin",
		"REPOMAN_WEBHELP_BASE_URL=http://127.0.0.1:1/docs/",
	}
	out, _ = cmd.CombinedOutput()
	if !g.check(strings.Contains(string(out), "usage: repoman ed") &&
		!strings.Contains(string(out), "Full chapter") &&
		!strings.Contains(string(out), "error") && !strings.Contains(string(out), "ERROR"),
		"regression: an unreachable host produces the embedded help text with nothing extra "+
			"and no visible error, not a hang or a printed failure", string(out)) {
		return 1
	}

	// The genuine success path: a real, local, deterministic server
	// serving real content. Confirms the fetch, the delineation, and
	// the fetched content all actually appear together, in order --
	// not just that no error occurred.
	const fakeChapter = "# A fake chapter\n\nJust for this regression test.\n"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(fakeChapter))
	}))
	defer ts.Close()

	cmd = exec.Command(self, "ed", "-h")
	cmd.Dir = root
	cmd.Env = []string{
		"PATH=/usr/bin:/bin",
		"REPOMAN_WEBHELP_BASE_URL=" + ts.URL + "/docs/",
	}
	out, _ = cmd.CombinedOutput()
	outStr := string(out)
	embeddedIdx := strings.Index(outStr, "usage: repoman ed")
	delimIdx := strings.Index(outStr, "Full chapter, fetched live")
	contentIdx := strings.Index(outStr, "A fake chapter")
	if !g.check(embeddedIdx >= 0 && delimIdx > embeddedIdx && contentIdx > delimIdx &&
		strings.Contains(outStr, "Just for this regression test."),
		"regression: a genuinely successful fetch shows the embedded help text first, then "+
			"the delineation, then the fetched content itself, in that order", outStr) {
		return 1
	}

	fmt.Printf("selftest: all %d checks green\n", g.checks)
	return 0
}
