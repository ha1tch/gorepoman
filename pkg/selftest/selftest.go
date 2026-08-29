// Package selftest implements repoman's acceptance gate -- translated
// from repoman/selftest.py.
//
// Prints doctor's own environment summary first -- informational
// only, never affecting this gate's own pass/fail, since an absent
// optional tool (gofmt/bash/node/PyYAML) is a supported operating
// mode here, not a defect. See pkg/doctor's own doc comment for why
// that is a separate tool rather than folded into this one's own
// pass/fail logic.
//
// Builds a synthetic repository in a temp directory and exercises
// every tool against it: ed (its own selftest is embedded in this
// binary, invoked via pkg/strreplace's own selftest pattern isn't
// quite right here -- see note below), roles, syncver (set/check/
// regex target), register (add/close/check round-trip), guards (list/
// stale/record), waves (addwave creates a wave, config persistence,
// staleness detection, collision refusal), relcore (full run, failure
// halt, resume, archive with manifest + contamination guard), and a
// final adversarial section (§8) that targets specific real bugs
// found in this project's own history rather than speculative edge
// cases.
//
// ADAPTATION FROM THE PYTHON ORIGINAL: every "run(tool, args...)" call
// in the Python original spawns `python3 tool.py args...` as a
// subprocess. This port instead re-invokes THIS SAME compiled binary
// (via os.Executable()) with the matching subcommand -- e.g. `repoman
// register add ...` -- which is the exact Go analogue: one compiled
// multi-command binary standing in for the original's directory of
// standalone scripts, exercised the same black-box, real-subprocess
// way rather than calling package functions in-process (which would
// not exercise argument parsing, working-directory handling, or exit
// codes the same way a real invocation does).
package selftest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ha1tch/gorepoman/pkg/doctor"
)

type runResult struct {
	stdout, stderr string
	code           int
}

func run(self, cwd string, args ...string) runResult {
	cmd := exec.Command(self, args...)
	cmd.Dir = cwd
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	code := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else {
			code = 1
		}
	}
	return runResult{stdout.String(), stderr.String(), code}
}

// runWithStdin is run() with a payload fed on stdin, for commands
// (like `strreplace apply -`) that read their input that way rather
// than from a file argument.
func runWithStdin(self, cwd, stdin string, args ...string) runResult {
	cmd := exec.Command(self, args...)
	cmd.Dir = cwd
	cmd.Stdin = strings.NewReader(stdin)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	code := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else {
			code = 1
		}
	}
	return runResult{stdout.String(), stderr.String(), code}
}

// gate carries the running state of the acceptance walk: which
// binary to re-invoke, which directory to run it in, and how many
// checks have passed so far.
type gate struct {
	self   string
	cwd    string
	checks int
}

// check prints and counts a pass, or prints and signals an abort on a
// fail -- mirroring ok()'s own sys.exit(1)-on-failure semantics via a
// bool the caller must act on (`if !g.check(...) { return 1 }`),
// since Go has no direct equivalent to unwinding straight out of the
// enclosing function from a nested closure.
func (g *gate) check(cond bool, label, detail string) bool {
	if !cond {
		fmt.Printf("FAIL: %s\n%s\n", label, detail)
		// Deliberately loud and impossible to misread, symmetric with
		// "selftest: all N checks green" on success. A "FAIL:" line
		// alone was observed, independently, in more than one session,
		// getting misread against a check's own description text
		// (several check names legitimately contain the phrase "exits
		// 0" as part of describing what they verify -- e.g. "doctor
		// under an emptied PATH... still exits 0" -- which reads, on a
		// fast skim, uncomfortably close to a claim about this run's
		// own result). This line carries no such ambiguity, and -- more
		// importantly than wording -- it is deliberately the last thing
		// printed before the process exits non-zero, specifically so it
		// survives being piped through `tail` even at a small N. It is
		// not a fix for the actual failure mode observed in practice
		// (piping through `tail`/`head` without `pipefail` silently
		// replaces this process's own exit code with the pipe's --
		// nothing printed to stdout can fix a shell construct that
		// never looks at stdout to determine success) -- see
		// repoman-030-getting-started.md for that half of it -- but it
		// means the plain text of the output is never itself the
		// ambiguous part.
		fmt.Println("SELFTEST FAILED -- do not trust this build")
		return false
	}
	g.checks++
	fmt.Printf("ok  %s\n", label)
	return true
}

func mustWrite(path, content string) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		panic(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		panic(err)
	}
}

func mustRead(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	return string(b)
}

func loadCfgMap(path string) map[string]interface{} {
	m := map[string]interface{}{}
	if b, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(b, &m)
	}
	return m
}

func saveCfgMap(path string, m map[string]interface{}) {
	b, _ := json.Marshal(m)
	mustWrite(path, string(b))
}

// Run implements `repoman selftest [args]` -- the acceptance gate.
func Run(_ []string) int {
	fmt.Println("-- environment --")
	doctor.Run([]string{"--quiet"})
	fmt.Println("-- acceptance gate --")

	self, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	// Resolve symlinks (e.g. `go run`'s tmp binary path patterns) so
	// re-invocation is robust regardless of how this binary was
	// launched.
	if resolved, err := filepath.EvalSymlinks(self); err == nil {
		self = resolved
	}

	g := &gate{self: self}

	// 1. ed's own selftest -- now a real 9-path suite (see
	// pkg/ed/selftest.go; ported after a feature-parity review found
	// the Go tree had previously stubbed this out).
	r := run(self, "", "ed", "selftest")
	if !g.check(r.code == 0 && strings.Contains(r.stdout, "9 paths green"), "ed selftest", r.stdout+r.stderr) {
		return 1
	}

	dir, err := os.MkdirTemp("", "repoman-selftest-*")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer os.RemoveAll(dir)
	root := dir
	g.cwd = root
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		root = resolved
		g.cwd = root
	}

	if err := os.MkdirAll(filepath.Join(root, "docs"), 0755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	cfgPath := filepath.Join(root, ".repoman.json")
	saveCfgMap(cfgPath, map[string]interface{}{
		"id_prefix": "Q",
		"version_targets": []interface{}{
			map[string]interface{}{"file": "app.py", "match": `VERSION = "([0-9.]+)"`},
		},
		"release": map[string]interface{}{
			"steps": []interface{}{
				map[string]interface{}{"name": "sync", "builtin": "syncver", "always": true},
				map[string]interface{}{"name": "build", "run": "echo building > built.txt", "resumable": true},
				map[string]interface{}{"name": "archive", "builtin": "archive", "always": true},
			},
			"archive": map[string]interface{}{
				"sources": []interface{}{"VERSION", "app.py", "docs"},
				"exclude": []interface{}{"*.secret"},
			},
		},
	})
	mustWrite(filepath.Join(root, "VERSION"), "0.0.1\n")
	mustWrite(filepath.Join(root, "app.py"), `VERSION = "0.0.1"`+"\n")
	mustWrite(filepath.Join(root, "CHANGELOG.md"),
		"## [0.0.2] - 2026-01-02\n\n## [0.0.1] - 2026-01-01\n")
	tPath := filepath.Join(root, "docs", "TRACKING.md")
	mustWrite(tPath,
		"# Register\n\nVersion: 0.0.1\n\n## Status table\n\n"+
			"| ID | Summary | Theme | Priority | Status | Blocks |\n"+
			"|---|---|---|---|---|---|\n"+
			"| Q-01 | seed item | core | P2 | ☐ | — |\n\n"+
			"## core\n\n### Q-01. seed item\n\n"+
			"Theme: core · Priority: P2 · Status: ☐\n\n"+
			"- **Trigger:** fixture.\n\n---\n")
	resolvedPath := filepath.Join(root, "docs", "RESOLVED.md")
	mustWrite(resolvedPath,
		"# Resolved\n\nClosed items, newest first.\n\n"+
			"## [0.0.0] Q-00 — genesis (v0.0.0, 2026-01-01)\n\ndone.\n")
	mustWrite(filepath.Join(root, "docs", "KNOWN_ISSUES.md"),
		"# Known issues\n\nVersion: 0.0.1\n\n## Dormant guards\n\n"+
			"### G-01. fixture guard (`x_test.go`)\n\n"+
			"- **Gate:** build tag `stress`\n"+
			"- **Invocation:** `go test -tags stress ./...`\n"+
			"- **Last exercised:** 2025-12-01 env:m1\n")

	// 2. syncver: set writes both the file and the regex target.
	r = run(self, root, "syncver", "set", "0.0.2")
	if !g.check(r.code == 0 && strings.Contains(mustRead(filepath.Join(root, "app.py")), `VERSION = "0.0.2"`),
		"syncver set + regex target", r.stderr) {
		return 1
	}
	r = run(self, root, "syncver", "check")
	if !g.check(r.code == 0, "syncver check", r.stdout+r.stderr) {
		return 1
	}

	// 3. register with a non-default prefix: add, check, close.
	r = run(self, root, "register", "add", "--summary", "second", "--theme",
		"core", "--priority", "P3", "--body", "- **Trigger:** t.")
	if !g.check(r.code == 0 && strings.Contains(r.stdout, "Q-02"), "register add Q-02", r.stdout+r.stderr) {
		return 1
	}
	r = run(self, root, "register", "check")
	if !g.check(r.code == 0 && strings.Contains(r.stdout, "2 open"), "register check", r.stdout) {
		return 1
	}
	r = run(self, root, "register", "close", "Q-02", "--version", "0.0.2")
	if !g.check(r.code == 0, "register close", r.stdout+r.stderr) {
		return 1
	}
	r = run(self, root, "register", "check")
	if !g.check(r.code == 0 && strings.Contains(r.stdout, "1 open"), "close removed row AND detail", r.stdout) {
		return 1
	}
	if !g.check(strings.Contains(mustRead(resolvedPath), "Q-02"), "closure recorded in RESOLVED", "") {
		return 1
	}

	// 4. guards: list, stale against previous release date, record.
	r = run(self, root, "guards", "list")
	if !g.check(strings.Contains(r.stdout, "G-01"), "guards list", r.stdout+r.stderr) {
		return 1
	}
	r = run(self, root, "guards", "stale")
	if !g.check(r.code == 1 && strings.Contains(r.stdout, "G-01"), "stale detects unexercised guard", r.stdout) {
		return 1
	}
	r = run(self, root, "guards", "record", "G-01", "--date", "2026-01-03", "--env", "ci")
	if !g.check(r.code == 0, "guards record", r.stderr) {
		return 1
	}
	r = run(self, root, "guards", "stale")
	if !g.check(r.code == 0, "stale clean after record", r.stdout) {
		return 1
	}

	// 5. waves: addwave creates a wave, waveprogress regenerates the
	// summary, --check confirms it's fresh, wave_short_names persists
	// to .repoman.json, and an explicit collision with an already-
	// taken number is correctly refused.
	wtPath := filepath.Join(root, "docs", "WAVE_TRACKING.md")
	mustWrite(wtPath,
		"# Waves\n\n## 1. Progress at a glance\n\n```\n(placeholder)\n```\n\n"+
			"Overall by item count: 0 of 0 items \u2248 **0%**\n")
	mustWrite(filepath.Join(root, "docs", "WAVE_PLAN.md"), "# Wave plan\n")
	r = run(self, root, "addwave", "--name", "first wave", "--ideal-days", "1.0",
		"--items-json", `[{"summary": "seed item", "register_item": "Q-01"}]`,
		"--plan-note", "fixture wave.")
	if !g.check(r.code == 0 && strings.Contains(r.stdout, "Wave number: 1"), "addwave creates wave 1", r.stdout+r.stderr) {
		return 1
	}
	if !g.check(strings.Contains(mustRead(wtPath), "### Wave 1"), "wave section inserted", "") {
		return 1
	}
	if !g.check(strings.Contains(mustRead(cfgPath), `"1":"first wave"`) ||
		strings.Contains(mustRead(cfgPath), `"1": "first wave"`),
		"wave_short_names persisted to config", "") {
		return 1
	}
	r = run(self, root, "waveprogress", "--check")
	if !g.check(r.code == 0, "waveprogress already fresh after addwave", r.stdout+r.stderr) {
		return 1
	}
	r = run(self, root, "addwave", "--name", "collision", "--ideal-days", "1.0",
		"--items-json", `[{"summary": "x"}]`, "--plan-note", "t",
		"--wave-number", "1", "--dry-run")
	if !g.check(r.code == 1 && strings.Contains(r.stderr, "collides"), "addwave refuses an explicit collision", r.stdout+r.stderr) {
		return 1
	}

	// 5b. A SECOND real (non-dry-run) wave insertion, specifically
	// targeting a real bug found in the Python original's own session:
	// the first version of the separator-reuse logic always added a
	// trailing "---" regardless of whether one already existed from
	// the PREVIOUS insertion, producing a duplicate. The single-
	// insertion check above could never have caught this -- it only
	// manifests on the second insertion onward.
	r = run(self, root, "addwave", "--name", "second wave", "--ideal-days", "1.0",
		"--items-json", `[{"summary": "another item", "register_item": "Q-02"}]`,
		"--plan-note", "fixture wave two.")
	if !g.check(r.code == 0 && strings.Contains(r.stdout, "Wave number: 2"),
		"addwave creates wave 2 (second real insertion)", r.stdout+r.stderr) {
		return 1
	}
	wtText := mustRead(wtPath)
	if !g.check(strings.Contains(wtText, "**Wave 1: 0/1, not started.**\n\n### Wave 2"),
		"proper blank-line spacing between wave 1 and wave 2, no run-together heading "+
			"and no gratuitous separator between two adjacent waves", fmt.Sprintf("%q", wtText)) {
		return 1
	}
	standaloneSeps := regexp.MustCompile(`(?m)^---\s*$`).FindAllString(wtText, -1)
	if !g.check(len(standaloneSeps) == 1 && strings.HasSuffix(strings.TrimRight(wtText, " \t\n"), "---"),
		"exactly one trailing separator, after the last wave (nothing else follows it in this "+
			"minimal fixture) -- not zero, not a duplicate, and not confused with a markdown "+
			"table's own |---|---| row", fmt.Sprintf("%q", wtText)) {
		return 1
	}

	return runSection6Plus(g, root, cfgPath, tPath, resolvedPath, wtPath)
}
