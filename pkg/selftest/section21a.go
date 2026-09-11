package selftest

import (
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
)

// runSection21a is coverage for a real gap found by hand, not by this
// suite: runSection20/21 prove the webhelp mechanism itself works
// (suppression, --brief, a genuine fetch, delineation) but only ever
// drive it through `ed -h` and `register -h` -- so nothing here would
// have caught a command that never wires webhelp.PrintIfAvailable
// into its own -h path at all. That's exactly what had happened to
// `board`, `workspace`, and `provenance` (all three subcommand paths:
// top-level, `check`, `sanction`): full docs chapters existed and
// were correctly published by CI, but the CLI itself never offered to
// fetch them live. Fixed in the same pass that added this check --
// this section exists so a future command added without the wiring
// fails loudly instead of silently matching the same pattern again.
//
// Same technique as runSection20's success-path check: a local
// httptest.Server serving known content, confirmed to actually appear
// in each command's -h output. Every command below is expected to
// already print webhelp.SuppressionNote unconditionally in its
// embedded help text (checked directly, not just inferred from the
// fetch succeeding) -- a command that dropped that line while still
// fetching would be a regression of its own.
func runSection21a(g *gate, root string) int {
	self := g.self

	const fakeChapter = "# A fake chapter\n\nSection21a verification content.\n"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(fakeChapter))
	}))
	defer ts.Close()

	env := []string{
		"PATH=/usr/bin:/bin",
		"REPOMAN_WEBHELP_BASE_URL=" + ts.URL + "/docs/",
	}

	cases := []struct {
		label string
		args  []string
	}{
		{"board", []string{"board", "-h"}},
		{"workspace", []string{"workspace", "-h"}},
		{"provenance", []string{"provenance", "-h"}},
		{"provenance check", []string{"provenance", "check", "-h"}},
		{"provenance sanction", []string{"provenance", "sanction", "-h"}},
	}

	for _, c := range cases {
		cmd := exec.Command(self, c.args...)
		cmd.Dir = root
		cmd.Env = env
		out, _ := cmd.CombinedOutput()
		outStr := string(out)

		if !g.check(strings.Contains(outStr, "REPOMAN_NO_WEB_HELP") && strings.Contains(outStr, "--brief"),
			"webhelp-coverage: `repoman "+c.label+" -h` prints the suppression note "+
				"unconditionally, same as every other command", outStr) {
			return 1
		}

		delimIdx := strings.Index(outStr, "Full chapter, fetched live")
		contentIdx := strings.Index(outStr, "Section21a verification content")
		if !g.check(delimIdx >= 0 && contentIdx > delimIdx,
			"webhelp-coverage: `repoman "+c.label+" -h` actually live-fetches its doc "+
				"chapter when reachable, not just embedded help", outStr) {
			return 1
		}

		// --brief still suppresses it here too, same as every other
		// command -- confirms NormalizeBriefFirst is wired in, not
		// just PrintIfAvailable.
		briefArgs := append([]string{}, c.args...)
		briefArgs = append(briefArgs, "--brief")
		cmd = exec.Command(self, briefArgs...)
		cmd.Dir = root
		cmd.Env = env
		out, _ = cmd.CombinedOutput()
		outStr = string(out)
		if !g.check(!strings.Contains(outStr, "Full chapter") &&
			!strings.Contains(outStr, "Section21a verification content"),
			"webhelp-coverage: `repoman "+c.label+" -h --brief` suppresses a fetch that "+
				"would genuinely have succeeded", outStr) {
			return 1
		}
	}

	return runSection22(g, root)
}
