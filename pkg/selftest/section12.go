package selftest

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// runSection12 exercises badcode: forbidden-string scanning loaded
// from a local config directory that is deliberately NOT under root
// (root is the synthetic repository fixture every other section in
// this file operates on) -- the whole design point of badcode is that
// its pattern list lives outside any repository, so the fixture for
// it has to actually be outside this one, not just asserted to be in
// a comment.
func runSection12(g *gate, root string) int {
	self := g.self

	badcodeConfigDir := filepath.Join(filepath.Dir(root), "badcode-config-fixture")
	os.MkdirAll(badcodeConfigDir, 0755)
	mustWrite(filepath.Join(badcodeConfigDir, "badcode.txt"),
		"# fixture patterns\nTOP_SECRET_FIXTURE_TOKEN\n")
	mustWrite(filepath.Join(badcodeConfigDir, "badcode.json"),
		`[{"pattern": "internal.fixture.example.com", "reason": "fixture-only hostname, never real"}]`)

	badcodeSrcDir := filepath.Join(root, "badcode-src")
	os.MkdirAll(badcodeSrcDir, 0755)
	mustWrite(filepath.Join(badcodeSrcDir, "clean.go"), "package main\n\nfunc main() {}\n")

	env := filterAndSet(os.Environ(), "REPOMAN_BADCODE_DIR", badcodeConfigDir)

	run := func(args ...string) runResult {
		return runWithEnv(self, root, env, args...)
	}

	r := run("badcode", "check", badcodeSrcDir)
	if !g.check(r.code == 0 && strings.Contains(r.stdout, "BADCODE CHECK OK (2 pattern(s) checked)"),
		"badcode: a clean tree passes, both badcode.txt and badcode.json patterns loaded "+
			"(1 + 1 = 2, confirming both files are read, not just whichever is checked first)",
		r.stdout+r.stderr) {
		return 1
	}

	// Case-insensitive match, deliberately different case from the
	// configured pattern -- badcode's whole value proposition fails if
	// a case difference lets a forbidden string through.
	mustWrite(filepath.Join(badcodeSrcDir, "leak.go"),
		"package main\n\n// references Top_Secret_Fixture_Token and internal.Fixture.Example.com\nfunc leak() {}\n")

	r = run("badcode", "check", badcodeSrcDir)
	if !g.check(r.code == 1 && strings.Contains(r.stdout, "BADCODE CHECK FAIL: 2 match(es)"),
		"badcode: refuses on a real match, case-insensitive (fixture uses different case "+
			"than the configured pattern for both entries)", r.stdout+r.stderr) {
		return 1
	}
	if !g.check(strings.Contains(r.stderr, `pattern "internal.fixture.example.com" (fixture-only hostname, never real)`),
		"badcode: the reason field from badcode.json is included in the refusal message, "+
			"not just the bare pattern", r.stderr) {
		return 1
	}
	if !g.check(strings.Contains(r.stderr, "leak.go:3"),
		"badcode: reports the actual file and line number of the match", r.stderr) {
		return 1
	}

	// A binary file containing the forbidden bytes as a coincidental
	// sequence must NOT be flagged -- that's noise, not a real leak of
	// the string as text.
	os.Remove(filepath.Join(badcodeSrcDir, "leak.go"))
	binPath := filepath.Join(badcodeSrcDir, "artifact.bin")
	binData := append([]byte{0x00, 0x01, 0x02}, []byte("TOP_SECRET_FIXTURE_TOKEN")...)
	if err := os.WriteFile(binPath, binData, 0644); err != nil {
		panic(err)
	}
	r = run("badcode", "check", badcodeSrcDir)
	if !g.check(r.code == 0 && strings.Contains(r.stdout, "BADCODE CHECK OK"),
		"badcode: a binary file containing the forbidden bytes as a coincidental byte "+
			"sequence is skipped, not flagged", r.stdout+r.stderr) {
		return 1
	}

	// No config at all: soft pass, but never silently indistinguishable
	// from a real check having run -- the WARN line is the assertion
	// that matters here, not just the exit code.
	emptyConfigDir := filepath.Join(filepath.Dir(root), "badcode-config-empty-fixture")
	envEmpty := filterAndSet(os.Environ(), "REPOMAN_BADCODE_DIR", emptyConfigDir)
	r = runWithEnv(self, root, envEmpty, "badcode", "check", badcodeSrcDir)
	if !g.check(r.code == 0 && strings.Contains(r.stdout, "WARN no badcode patterns configured") &&
		strings.Contains(r.stdout, "BADCODE CHECK OK (0 patterns configured)"),
		"badcode: no config at all is a soft pass, not a hard failure -- but says so plainly "+
			"rather than looking identical to a real, clean check", r.stdout) {
		return 1
	}

	// Regression: `repoman badcode --help` (bare, no subcommand) must
	// show top-level help, not the generic usage-error path -- this
	// exact bug was found and fixed in this project's own history
	// (see CHANGELOG), the same class of gap `guards`/`relcore`/
	// `register`/`gomod` all had before their own top-level --help fixes.
	r = run("badcode", "--help")
	if !g.check(r.code == 0 && strings.Contains(r.stdout, "usage: repoman badcode"),
		"badcode --help (bare, no subcommand) shows help and exits 0, not the generic "+
			"usage-error path", r.stdout+r.stderr) {
		return 1
	}

	return runSection13(g, root)
}

// runWithEnv is run() with an explicit environment rather than the
// current process's inherited one -- needed here specifically because
// REPOMAN_BADCODE_DIR must vary between sub-checks within this same
// section, which plain run()'s always-inherit-os.Environ() can't do.
func runWithEnv(self, cwd string, env []string, args ...string) runResult {
	cmd := exec.Command(self, args...)
	cmd.Dir = cwd
	cmd.Env = env
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	code := exitCodeOf(err)
	return runResult{stdout.String(), stderr.String(), code}
}
