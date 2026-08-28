package selftest

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var waveNumRe = regexp.MustCompile(`Wave number: (\d+)`)

func runSection8jPlus(g *gate, root, cfgPath, wtPath, waveTrackingAfter5b string) int {
	self := g.self
	cfg := loadCfgMap(cfgPath)

	// 8j. waveprogress --html: a fresh, dedicated fixture with one
	// done wave, one 0%-complete wave, and one wave whose name
	// contains HTML-special characters -- checked for real escaping,
	// since wave names and debt/blocker ids come from documents this
	// tool does not control the content of.
	htmlFixtureTracking := "# Waves\n\n## 1. Progress at a glance\n\n```\n(placeholder)\n```\n\n" +
		"## 2. Wave-by-wave detail\n\n" +
		"### Wave 20 — done (1 items, ideal 1d)\n\n" +
		"| # | Summary | Status | Register item |\n|---|---|---|---|\n" +
		"| 90 | x | ✓ | Q-90 |\n\n**Wave 20: 1/1, complete.**\n\n" +
		"### Wave 21 — <script>evil</script> & \"quotes\" (1 items, ideal 1d)\n\n" +
		"| # | Summary | Status | Register item |\n|---|---|---|---|\n" +
		"| 91 | y | ☐ | Q-91 |\n\n**Wave 21: 0/1, not started.**\n\n" +
		"Overall by item count: 1 of 2 items \u2248 **50%**\n"
	mustWrite(wtPath, htmlFixtureTracking)
	originalShortNames, _ := cfg["wave_short_names"].(map[string]interface{})
	origShortNamesCopy := map[string]interface{}{}
	for k, v := range originalShortNames {
		origShortNamesCopy[k] = v
	}
	cfg["wave_short_names"] = map[string]interface{}{
		"20": "done", "21": `<script>evil</script> & "quotes"`,
	}
	saveCfgMap(cfgPath, cfg)
	htmlOut := filepath.Join(root, "waves.html")
	r := run(self, root, "waveprogress", "--html", htmlOut)
	if !g.check(r.code == 0 && fileExists(htmlOut), "waveprogress --html writes a file", r.stdout+r.stderr) {
		return 1
	}
	htmlContent := mustRead(htmlOut)
	if !g.check(strings.HasPrefix(htmlContent, "<!DOCTYPE html>") && strings.Contains(htmlContent, "<style>"),
		"output is a genuinely standalone HTML document (DOCTYPE + inline style, no dependency "+
			"on an external stylesheet or host CSS variables)", truncate(htmlContent, 200)) {
		return 1
	}
	if !g.check(strings.Contains(htmlContent, `class="fill success"`) && strings.Contains(htmlContent, `class="fill muted"`),
		"a complete wave gets the success colour class, an unstarted one gets muted", htmlContent) {
		return 1
	}
	if !g.check(!strings.Contains(htmlContent, "<script>evil</script>") &&
		strings.Contains(htmlContent, "&lt;script&gt;evil&lt;/script&gt;"),
		"a wave name containing HTML-special characters is actually escaped in the output, "+
			"not injected raw", htmlContent) {
		return 1
	}
	cfg["wave_short_names"] = origShortNamesCopy
	saveCfgMap(cfgPath, cfg)

	// Restore WAVE_TRACKING.md to the pre-8j-fixture state (i.e. back
	// to the two-real-wave document from §5b), not to some
	// intermediate state -- the exact snapshot captured in §8i, before
	// this section overwrote the file with its own dedicated fixture.
	return runSection8kPlus(g, root, cfgPath, wtPath, waveTrackingAfter5b)
}

func runSection8kPlus(g *gate, root, cfgPath, wtPath string, waveTrackingBeforeHTMLFixture string) int {
	self := g.self
	mustWrite(wtPath, waveTrackingBeforeHTMLFixture)

	// 8k. wave visibility: persisted DATA, not a rendering concern --
	// --hide/--unhide mutate .repoman.json directly, both the ASCII
	// and HTML renderers read the SAME map, and Overall always
	// reflects every wave regardless of what's currently hidden.
	r := run(self, root, "waveprogress", "--show")
	if !g.check(r.code == 0 && strings.Contains(r.stdout, "Wave 2"),
		"wave 2 visible by default before any --hide", r.stdout+r.stderr) {
		return 1
	}
	overallBefore := firstLineContaining(r.stdout, "Overall")

	r = run(self, root, "waveprogress", "--hide", "2")
	if !g.check(r.code == 0 && strings.Contains(r.stdout, "hidden") && strings.Contains(r.stdout, "persisted"),
		"--hide persists and confirms", r.stdout+r.stderr) {
		return 1
	}
	visCfg, _ := loadCfgMap(cfgPath)["wave_visibility"].(map[string]interface{})
	if !g.check(visCfg["2"] == false, "hidden state actually persisted to .repoman.json, not just printed",
		fmt.Sprintf("%v", visCfg)) {
		return 1
	}

	r = run(self, root, "waveprogress", "--show")
	if !g.check(r.code == 0 && !strings.Contains(r.stdout, "Wave 2") && strings.Contains(r.stdout, "Wave 1"),
		"hidden wave omitted from ASCII output, others unaffected", r.stdout+r.stderr) {
		return 1
	}
	overallAfterHide := firstLineContaining(r.stdout, "Overall")
	if !g.check(overallAfterHide == overallBefore,
		"Overall line is byte-identical whether or not a wave is hidden -- hiding is display-only, "+
			"the real total never changes", fmt.Sprintf("before=%q after=%q", overallBefore, overallAfterHide)) {
		return 1
	}

	htmlHidden := filepath.Join(root, "hidden.html")
	r = run(self, root, "waveprogress", "--html", htmlHidden)
	if !g.check(r.code == 0 && strings.Contains(r.stdout, "1 hidden"), "--html reports the hidden count", r.stdout+r.stderr) {
		return 1
	}
	if !g.check(!strings.Contains(mustRead(htmlHidden), "Wave 2 "),
		"HTML output respects the SAME persisted visibility state as ASCII -- one source of "+
			"truth, not two independently-tracked notions of what's shown", mustRead(htmlHidden)) {
		return 1
	}

	htmlAll := filepath.Join(root, "all.html")
	r = run(self, root, "waveprogress", "--html", htmlAll, "--include-hidden")
	if !g.check(r.code == 0 && strings.Contains(mustRead(htmlAll), "Wave 2 "),
		"--include-hidden overrides persisted visibility for one render, without changing the "+
			"persisted state itself", r.stdout+r.stderr) {
		return 1
	}

	r = run(self, root, "waveprogress", "--unhide", "2")
	if !g.check(r.code == 0 && strings.Contains(r.stdout, "visible"), "--unhide persists and confirms", r.stdout+r.stderr) {
		return 1
	}
	r = run(self, root, "waveprogress", "--show")
	if !g.check(r.code == 0 && strings.Contains(r.stdout, "Wave 2"), "wave 2 visible again in ASCII after --unhide", r.stdout+r.stderr) {
		return 1
	}

	r = run(self, root, "waveprogress", "--hide", "999")
	if !g.check(r.code == 0 && strings.Contains(strings.ToLower(r.stderr), "warning") && strings.Contains(r.stderr, "not a wave id"),
		"hiding an unrecognised wave id warns but still proceeds (the document may not be "+
			"regenerated yet)", r.stdout+r.stderr) {
		return 1
	}
	r = run(self, root, "waveprogress", "--unhide", "999") // cleanup
	if !g.check(r.code == 0, "cleanup: unhide the stray test entry", r.stderr) {
		return 1
	}

	// 8l. addwave automatically records visible=True for a newly
	// created wave -- not left to the absent-means-visible fallback
	// alone, matching wave_short_names's own precedent of an explicit,
	// discoverable record for every wave.
	r = run(self, root, "addwave", "--name", "vis test", "--ideal-days", "1.0",
		"--items-json", `[{"summary": "z", "register_item": "Q-90"}]`, "--plan-note", "t")
	if !g.check(r.code == 0, "addwave for the visibility-default check succeeds", r.stdout+r.stderr) {
		return 1
	}
	m := waveNumRe.FindStringSubmatch(r.stdout)
	newWaveNum := ""
	if m != nil {
		newWaveNum = m[1]
	}
	visCfg, _ = loadCfgMap(cfgPath)["wave_visibility"].(map[string]interface{})
	if !g.check(visCfg[newWaveNum] == true,
		"a newly created wave gets an explicit visible=True entry written automatically, "+
			"not left implicit", fmt.Sprintf("%v", visCfg)) {
		return 1
	}

	return runSection9Plus(g, root, cfgPath)
}

func runSection9Plus(g *gate, root, cfgPath string) int {
	self := g.self

	// 9. pkg/strreplace's own embedded selftest, run as a subprocess
	// the same way `ed selftest` is checked in §1, then a real
	// (non-dry-run) apply against this fixture's own TRACKING.md,
	// proving two things the tool's own isolated selftest cannot: it
	// interoperates with a real repoman-managed register document, and
	// the shared-journal claim (edits from either tool visible to
	// both) holds in the full repoman context, not just the tool's own
	// synthetic temp dir.
	r := run(self, "", "strreplace", "selftest")
	if !g.check(r.code == 0 && strings.Contains(r.stdout, "ALL GREEN") && !strings.Contains(r.stdout, "FAIL"),
		"strreplace selftest", r.stdout+r.stderr) {
		return 1
	}

	srePayload, _ := json.Marshal(map[string]interface{}{
		"v": 1,
		"ops": []interface{}{
			map[string]interface{}{
				"file":        "docs/TRACKING.md",
				"search_b64":  base64.StdEncoding.EncodeToString([]byte("# Register\n")),
				"replace_b64": base64.StdEncoding.EncodeToString([]byte("# Register (str_replace_extended-edited)\n")),
				"expect":      1,
				"roles":       []string{"md-heading"},
			},
		},
	})
	cmd := exec.Command(self, "strreplace", "apply", "-")
	cmd.Dir = root
	cmd.Stdin = strings.NewReader(string(srePayload))
	out, err := cmd.CombinedOutput()
	code := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else {
			code = 1
		}
	}
	tPath := filepath.Join(root, "docs", "TRACKING.md")
	if !g.check(code == 0 && strings.Contains(mustRead(tPath), "str_replace_extended-edited"),
		"strreplace applies a real edit against a repoman fixture", string(out)) {
		return 1
	}
	r = run(self, root, "ed", "log")
	if !g.check(r.code == 0 && strings.Contains(r.stdout, "str_replace_extended"),
		"ed log sees the strreplace edit -- shared journal confirmed", r.stdout+r.stderr) {
		return 1
	}
	r = run(self, root, "ed", "undo")
	if !g.check(r.code == 0 && strings.HasPrefix(mustRead(tPath), "# Register\n"),
		"ed undo reverts the strreplace edit", r.stdout+r.stderr) {
		return 1
	}

	return runSection10Plus(g, root, cfgPath)
}

func runSection10Plus(g *gate, root, cfgPath string) int {
	self := g.self

	// 10. doctor: environment diagnostic, checked for real rather than
	// just "it runs". Every real tool this sandbox actually has must
	// be correctly detected; genuinely hiding a tool via an emptied
	// PATH must correctly flip it to "not found" with a fallback
	// stated. --quiet must never change what's true, only how much
	// detail is printed. Never affects this gate's own pass/fail (see
	// pkg/doctor's own doc comment for why) -- exit 0 is asserted here
	// even with every optional tool hidden.
	report := doctorCheckViaCLI(self, root)
	if !g.check(report.goOK && report.platform != "", "doctor basic shape", fmt.Sprintf("%+v", report)) {
		return 1
	}
	realBash, _ := exec.LookPath("bash")
	if !g.check((realBash != "") == report.bashFound, "doctor correctly detects a real, present tool",
		fmt.Sprintf("found=%v real=%q", report.bashFound, realBash)) {
		return 1
	}

	emptyEnv := append(os.Environ(), "PATH=/nonexistent-empty-path")
	cmd := exec.Command(self, "doctor")
	cmd.Env = filterAndSet(emptyEnv, "PATH", "/nonexistent-empty-path")
	out, err := cmd.CombinedOutput()
	rc := exitCodeOf(err)
	if !g.check(rc == 0 && strings.Contains(string(out), "not found"),
		"doctor under an emptied PATH reports missing tools, still exits 0 (absence is "+
			"informational, not a failure)", string(out)) {
		return 1
	}

	cmd = exec.Command(self, "doctor", "--quiet")
	cmd.Env = filterAndSet(emptyEnv, "PATH", "/nonexistent-empty-path")
	out, err = cmd.CombinedOutput()
	rc = exitCodeOf(err)
	if !g.check(rc == 0 && !strings.Contains(string(out), "enables:"),
		"doctor --quiet omits per-tool detail but still exits 0", string(out)) {
		return 1
	}

	return runSection11(g, root, cfgPath)
}

func runSection11(g *gate, root, cfgPath string) int {
	self := g.self

	// 11. gomod: go.mod/go.sum sanity gate. The replace-directive
	// checks are fully offline and deterministic, so they run
	// unconditionally. The go.sum-completeness check is inherently
	// network-adjacent, so that one sub-test is skipped, not failed,
	// if this environment genuinely has no outbound network access.
	gomodDir := filepath.Join(root, "gomod-fixture")
	os.MkdirAll(gomodDir, 0755)
	mustWrite(filepath.Join(gomodDir, "go.mod"),
		"module example.com/fixture\n\ngo 1.21\n\nrequire github.com/foo/bar v1.2.3\n\n"+
			"replace github.com/foo/bar => /root/go/pkg/mod/github.com/foo/bar@v1.2.3\n"+
			"replace github.com/baz/qux => ../local-qux\n"+
			"replace github.com/legit/thing => github.com/legit/thing v1.9.9\n")
	r := run(self, root, "gomod", "check", gomodDir)
	if !g.check(r.code == 1 && strings.Contains(r.stdout, "replace-absolute-path"),
		"gomod fails on an absolute-path replace directive (the exact shape of a real "+
			"path-leak incident)", r.stdout+r.stderr) {
		return 1
	}
	if !g.check(strings.Contains(r.stdout, "replace-relative-path") && !strings.Contains(r.stdout, "ERROR replace-relative-path"),
		"a relative-path replace warns, not fails, by default", r.stdout) {
		return 1
	}
	if !g.check(!strings.Contains(r.stdout, "github.com/legit/thing"),
		"a versioned replace (a real registry redirect) is never flagged", r.stdout) {
		return 1
	}
	r = run(self, root, "gomod", "check", "--strict-relative-replace", gomodDir)
	if !g.check(r.code == 1 && strings.Contains(r.stdout, "ERROR replace-relative-path"),
		"--strict-relative-replace promotes the relative case to a failure too", r.stdout+r.stderr) {
		return 1
	}

	cleanDir := filepath.Join(root, "gomod-clean")
	os.MkdirAll(cleanDir, 0755)
	mustWrite(filepath.Join(cleanDir, "go.mod"), "module example.com/clean\n\ngo 1.21\n")
	r = run(self, root, "gomod", "check", cleanDir)
	if !g.check(r.code == 0 && strings.TrimSpace(r.stdout) == "GOMOD CHECK OK",
		"a clean go.mod with no replace directives and no dependencies passes cleanly", r.stdout+r.stderr) {
		return 1
	}

	cmd := exec.Command(self, "gomod", "check", cleanDir)
	cmd.Env = filterAndSet(os.Environ(), "PATH", "/nonexistent-empty-path")
	out, err := cmd.CombinedOutput()
	rc := exitCodeOf(err)
	if !g.check(rc == 1 && strings.Contains(string(out), "go-tooling"),
		"gomod fails clearly (not silently) when go itself is not on PATH -- this check cannot "+
			"degrade gracefully the way an optional tool can", string(out)) {
		return 1
	}

	networkDir := filepath.Join(root, "gomod-network")
	os.MkdirAll(networkDir, 0755)
	mustWrite(filepath.Join(networkDir, "go.mod"),
		"module example.com/network\n\ngo 1.21\n\nrequire github.com/ha1tch/zen80 v0.1.0\n")
	mustWrite(filepath.Join(networkDir, "main.go"),
		"package main\n\nimport _ \"github.com/ha1tch/zen80/z80\"\n\nfunc main() {}\n")

	networkUp := false
	if goBin, err := exec.LookPath("go"); err == nil {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		probe := exec.CommandContext(ctx, goBin, "list", "-m", "-versions", "github.com/ha1tch/zen80")
		probe.Dir = networkDir
		err := probe.Run()
		cancel()
		if err == nil {
			networkUp = true
		}
	}
	if networkUp {
		r = run(self, root, "gomod", "check", networkDir)
		if !g.check(r.code == 1 && strings.Contains(r.stdout, "gosum-incomplete"),
			"a real dependency with no go.sum entry at all is caught via go list's own "+
				"\"missing go.sum entry\" text", r.stdout+r.stderr) {
			return 1
		}
	} else {
		fmt.Println("skip  gosum-incomplete detection (no outbound network in this environment -- not this tool's own defect)")
	}

	return runSection12(g, root)
}

// ── small local helpers ──────────────────────────────────────────────

func fileExists(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir()
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func firstLineContaining(s, sub string) string {
	for _, ln := range strings.Split(s, "\n") {
		if strings.Contains(ln, sub) {
			return ln
		}
	}
	return ""
}

func filterAndSet(env []string, key, value string) []string {
	out := make([]string, 0, len(env)+1)
	prefix := key + "="
	for _, e := range env {
		if !strings.HasPrefix(e, prefix) {
			out = append(out, e)
		}
	}
	out = append(out, prefix+value)
	return out
}

func exitCodeOf(err error) int {
	if err == nil {
		return 0
	}
	if ee, ok := err.(*exec.ExitError); ok {
		return ee.ExitCode()
	}
	return 1
}

type doctorReport struct {
	goOK      bool
	platform  string
	bashFound bool
}

// doctorCheckViaCLI shells to `repoman doctor` (rather than importing
// pkg/doctor directly and re-parsing its Report) and does a light
// text scan of the output for the handful of facts this section
// actually asserts on -- deliberately black-box, the same way every
// other section in this file exercises its subject through the real
// CLI rather than an in-process call.
func doctorCheckViaCLI(self, cwd string) doctorReport {
	r := run(self, cwd, "doctor")
	rep := doctorReport{}
	rep.goOK = strings.Contains(r.stdout, "[OK] Go")
	if m := regexp.MustCompile(`Platform: (.+)`).FindStringSubmatch(r.stdout); m != nil {
		rep.platform = strings.TrimSpace(m[1])
	}
	rep.bashFound = strings.Contains(r.stdout, "[OK] bash")
	return rep
}
