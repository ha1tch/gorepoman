package selftest

import (
	"os"
	"path/filepath"
	"strings"
)

// runSection13 exercises the mandatory relcore pre-flight integration
// specifically -- badcode.go's own checks (section12) cover the
// standalone `badcode check` command; this section covers the
// property that actually matters for release safety: relcore refuses
// to run ANY release step, including on --resume, while a real match
// exists, using a synthetic project entirely separate from the shared
// `root` fixture the rest of this suite uses (relcore needs a real
// .repoman.json + release.steps of its own here, and this section
// should not disturb the shared fixture's own careful journal/resume
// state that other sections depend on).
func runSection13(g *gate, root string) int {
	self := g.self

	badcodeConfigDir := filepath.Join(filepath.Dir(root), "badcode-config-relcore-fixture")
	os.MkdirAll(badcodeConfigDir, 0755)
	mustWrite(filepath.Join(badcodeConfigDir, "badcode.txt"), "RELCORE_FIXTURE_LEAK_TOKEN\n")

	proj := filepath.Join(root, "relcore-badcode-fixture")
	os.MkdirAll(proj, 0755)
	mustWrite(filepath.Join(proj, ".repoman.json"),
		`{"release": {"steps": [{"name": "noop", "run": "true", "always": true}]}}`)
	mustWrite(filepath.Join(proj, "VERSION"), "0.1.0\n")

	env := filterAndSet(os.Environ(), "REPOMAN_BADCODE_DIR", badcodeConfigDir)
	run := func(args ...string) runResult { return runWithEnv(self, proj, env, args...) }

	// A real match must block before the "noop" step -- and before
	// any of relcore's own release-in-progress output -- ever appears.
	mustWrite(filepath.Join(proj, "leak.txt"), "RELCORE_FIXTURE_LEAK_TOKEN\n")
	r := run("relcore", "0.1.0")
	if !g.check(r.code == 1 && strings.Contains(r.stdout, "FAIL badcode") &&
		!strings.Contains(r.stdout, "-- noop") && !strings.Contains(r.stdout, "release v0.1.0 prepared"),
		"relcore: a real badcode match blocks before any release.steps entry runs at all",
		r.stdout+r.stderr) {
		return 1
	}

	// --resume must not bypass it. Same fixture, same leak still
	// present: still blocked, identically.
	r = run("relcore", "0.1.0", "--resume")
	if !g.check(r.code == 1 && strings.Contains(r.stdout, "FAIL badcode"),
		"relcore: --resume does not bypass a real badcode match -- it is not part of the "+
			"resumable-steps journal at all, so there is nothing for --resume to skip",
		r.stdout+r.stderr) {
		return 1
	}

	// Fix it: remove the leak, confirm the release now actually
	// proceeds through the real step.
	os.Remove(filepath.Join(proj, "leak.txt"))
	r = run("relcore", "0.1.0")
	if !g.check(r.code == 0 && strings.Contains(r.stdout, "ok badcode") &&
		strings.Contains(r.stdout, "-- noop") && strings.Contains(r.stdout, "release v0.1.0 prepared"),
		"relcore: once the matched content is actually removed, the release proceeds -- "+
			"the gate blocks real content, not the release mechanism itself", r.stdout+r.stderr) {
		return 1
	}

	// No config at all: relcore still proceeds (soft pass), same as
	// the standalone command -- this integration doesn't change that
	// half of the design, only the "can it be skipped" half.
	proj2 := filepath.Join(root, "relcore-badcode-noconfig-fixture")
	os.MkdirAll(proj2, 0755)
	mustWrite(filepath.Join(proj2, ".repoman.json"),
		`{"release": {"steps": [{"name": "noop", "run": "true", "always": true}]}}`)
	mustWrite(filepath.Join(proj2, "VERSION"), "0.1.0\n")
	emptyConfigDir := filepath.Join(filepath.Dir(root), "badcode-config-relcore-empty-fixture")
	envEmpty := filterAndSet(os.Environ(), "REPOMAN_BADCODE_DIR", emptyConfigDir)
	r = runWithEnv(self, proj2, envEmpty, "relcore", "0.1.0")
	if !g.check(r.code == 0 && strings.Contains(r.stdout, "WARN no badcode patterns configured") &&
		strings.Contains(r.stdout, "release v0.1.0 prepared"),
		"relcore: no badcode config at all is a soft pass here too -- the release proceeds, "+
			"with the WARN visible in the release's own output, not suppressed", r.stdout+r.stderr) {
		return 1
	}

	return runSection14(g, root)
}
