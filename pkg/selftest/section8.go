package selftest

import (
	"archive/zip"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

func jsonGet(m map[string]interface{}, path ...string) interface{} {
	var cur interface{} = m
	for _, p := range path {
		mm, ok := cur.(map[string]interface{})
		if !ok {
			return nil
		}
		cur = mm[p]
	}
	return cur
}

func runSection6Plus(g *gate, root, cfgPath, tPath, resolvedPath, wtPath string) int {
	self := g.self

	// 6. relcore: full run, journal, archive with manifest.
	mustWrite(filepath.Join(root, "leak.secret"), "x") // excluded, proves policy
	r := run(self, root, "relcore", "0.0.2")
	if !g.check(r.code == 0 && strings.Contains(r.stdout, "release v0.0.2 prepared"), "relcore full run", r.stdout+r.stderr) {
		return 1
	}
	zipMatches, _ := filepath.Glob(filepath.Join(root, "*-v0.0.2-checkpoint.zip"))
	if !g.check(len(zipMatches) == 1, "archive produced", fmt.Sprintf("%v", zipMatches)) {
		return 1
	}
	zipPath := zipMatches[0]

	checkZip := func(zp string) (names []string, err error) {
		zr, err := zip.OpenReader(zp)
		if err != nil {
			return nil, err
		}
		defer zr.Close()
		for _, f := range zr.File {
			names = append(names, f.Name)
		}
		return names, nil
	}

	names, err := checkZip(zipPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if !g.check(containsStr(names, "MANIFEST.sha256"), "manifest embedded", fmt.Sprintf("%v", names)) {
		return 1
	}
	hasSecret := false
	for _, n := range names {
		if strings.Contains(n, "secret") {
			hasSecret = true
		}
	}
	if !g.check(!hasSecret, "exclusion honoured", fmt.Sprintf("%v", names)) {
		return 1
	}

	// 6b. Adversarial: a stale MANIFEST.sha256 already sitting in the
	// source tree (e.g. left over from a prior checkpoint extracted
	// back into a working copy) must not produce a duplicate entry.
	// The archive builtin generates its own manifest and must treat
	// that filename as reserved output, not a source file to sweep in
	// on top of.
	mustWrite(filepath.Join(root, "docs", "MANIFEST.sha256"), "stale, from a prior release\n")
	r = run(self, root, "relcore", "0.0.2", "--resume")
	if !g.check(r.code == 0, "relcore re-run with a stale docs/MANIFEST.sha256 present", r.stdout+r.stderr) {
		return 1
	}
	names2, err := checkZip(zipPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	countManifest := 0
	hasDocsManifest := false
	for _, n := range names2 {
		if n == "MANIFEST.sha256" {
			countManifest++
		}
		if n == "docs/MANIFEST.sha256" {
			hasDocsManifest = true
		}
	}
	if !g.check(!hasDocsManifest && countManifest == 1,
		"the exclude list matches by basename, the same as every other pattern already in it "+
			"(*.log, *.png, ...), so a same-named file anywhere in the tree is excluded too, not "+
			"just one sitting at archive root", fmt.Sprintf("%v", names2)) {
		return 1
	}

	// A stale MANIFEST.sha256 at the ARCHIVE ROOT, matching how a real
	// repo's archive.sources typically includes "." (the whole tree)
	// -- that's the shape that collides with the builtin's own
	// top-level manifest write.
	cfg2 := loadCfgMap(cfgPath)
	release2, _ := cfg2["release"].(map[string]interface{})
	archive2, _ := release2["archive"].(map[string]interface{})
	origSources, _ := archive2["sources"].([]interface{})
	sourcesWithManifest := append(append([]interface{}{}, origSources...), "MANIFEST.sha256")
	archive2["sources"] = sourcesWithManifest
	saveCfgMap(cfgPath, cfg2)
	mustWrite(filepath.Join(root, "MANIFEST.sha256"), "stale, from a prior release\n")
	r = run(self, root, "relcore", "0.0.2", "--resume")
	if !g.check(r.code == 0, "relcore re-run with a stale root-level MANIFEST.sha256 present", r.stdout+r.stderr) {
		return 1
	}
	names3, err := checkZip(zipPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	countManifest3 := 0
	for _, n := range names3 {
		if n == "MANIFEST.sha256" {
			countManifest3++
		}
	}
	if !g.check(countManifest3 == 1,
		"exactly one root-level MANIFEST.sha256 entry, not duplicated by a pre-existing stale "+
			"copy at archive root", fmt.Sprintf("%v", names3)) {
		return 1
	}
	archive2["sources"] = origSources
	saveCfgMap(cfgPath, cfg2)
	os.Remove(filepath.Join(root, "MANIFEST.sha256"))

	// 7. relcore: failing step halts; --resume skips the green build.
	cfg := loadCfgMap(cfgPath)
	release, _ := cfg["release"].(map[string]interface{})
	steps, _ := release["steps"].([]interface{})
	newSteps := append([]interface{}{}, steps[:2]...)
	newSteps = append(newSteps, map[string]interface{}{"name": "breaker", "run": "exit 3", "resumable": true})
	newSteps = append(newSteps, steps[2:]...)
	release["steps"] = newSteps
	saveCfgMap(cfgPath, cfg)
	r = run(self, root, "relcore", "0.0.2")
	if !g.check(r.code == 1 && strings.Contains(r.stdout, "FAIL breaker"), "failure halts the run", r.stdout) {
		return 1
	}
	var stepsNoBreaker []interface{}
	for _, s := range newSteps {
		sm, _ := s.(map[string]interface{})
		if sm["name"] != "breaker" {
			stepsNoBreaker = append(stepsNoBreaker, s)
		}
	}
	release["steps"] = stepsNoBreaker
	saveCfgMap(cfgPath, cfg)
	r = run(self, root, "relcore", "0.0.2", "--resume")
	if !g.check(r.code == 0 && strings.Contains(r.stdout, "build: journaled green, skipped"),
		"resume skips journaled step", r.stdout) {
		return 1
	}

	return runSection8(g, root, cfgPath, tPath, resolvedPath, wtPath)
}

func containsStr(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func runSection8(g *gate, root, cfgPath, tPath, resolvedPath, wtPath string) int {
	self := g.self

	// 8a. syncver: a real mismatch is correctly detected. ADAPTATION:
	// the Python original's own version of this test verified that
	// syncver.check() returns a genuine bool rather than an always-
	// truthy tuple (a real bug in this project's own history). That
	// entire footgun class is structurally impossible in Go --
	// pkg/syncver.CheckDetail's signature is `(bool, string)`,
	// enforced by the compiler, not something a unit test can
	// meaningfully re-verify at runtime. What's left worth checking
	// here is the BEHAVIOURAL half: a real mismatch is genuinely
	// caught via the CLI's own exit code.
	mustWrite(filepath.Join(root, "app.py"), `VERSION = "9.9.9"`+"\n") // deliberately desynced
	r := run(self, root, "syncver", "check")
	if !g.check(r.code == 1 && strings.Contains(r.stdout, "MISMATCH"),
		"syncver check correctly detects a real, deliberately-introduced mismatch (the "+
			"behavioural half of the historical check()-truthiness bug; the type-safety half "+
			"is a Go compiler guarantee, not a runtime-testable property)", r.stdout+r.stderr) {
		return 1
	}
	r = run(self, root, "syncver", "set", "0.0.2") // resync for later checks
	if !g.check(r.code == 0, "syncver resynced after adversarial mismatch", r.stderr) {
		return 1
	}

	// 8b. register: a closure narrative that QUOTES a much higher,
	// foreign-looking id in free-form prose must not corrupt next_id()
	// -- the exact bug (a CHANGELOG mentioning a much higher id in
	// prose jumped a project's sequence far ahead) the anchored
	// RESOLVED.md scan exists to prevent.
	originalResolved := mustRead(resolvedPath)
	mustWrite(resolvedPath, originalResolved+
		"\n## [0.0.2] Q-50 — narrative quoting a foreign id (v0.0.2, 2026-01-03)\n\n"+
		"As described in another project's own Q-999 report, this closure is unrelated to that number.\n")
	r = run(self, root, "register", "add", "--summary", "third", "--theme",
		"core", "--priority", "P3", "--body", "- **Trigger:** t.", "--dry-run")
	if !g.check(r.code == 0 && strings.Contains(r.stdout, "Q-51") && !strings.Contains(r.stdout, "Q-1000"),
		"next_id honours a real structured closure header (Q-50) but ignores a foreign-looking "+
			"id quoted in free-form prose (Q-999)", r.stdout+r.stderr) {
		return 1
	}
	mustWrite(resolvedPath, originalResolved) // restore

	// 8c. register: legacy and primary id formats coexist without
	// collision -- a real, permanent need (an old id range frozen
	// forever, new work in a new shape from a fixed point onward).
	cfg := loadCfgMap(cfgPath)
	cfg["legacy_id_prefix"] = "L"
	cfg["legacy_id_separator"] = "-"
	saveCfgMap(cfgPath, cfg)
	trackingText := mustRead(tPath)
	mustWrite(tPath, strings.Replace(trackingText,
		"| Q-01 | seed item | core | P2 | ☐ | — |\n",
		"| Q-01 | seed item | core | P2 | ☐ | — |\n"+
			"| L-3 | legacy-format item | core | P3 | ☐ | — |\n", 1))
	trackingText = mustRead(tPath)
	mustWrite(tPath, strings.Replace(trackingText,
		"### Q-01. seed item",
		"### L-3. legacy-format item\n\nTheme: core · Priority: P3 · Status: ☐\n\n"+
			"- **Trigger:** fixture, legacy format.\n\n---\n\n### Q-01. seed item", 1))
	r = run(self, root, "register", "check")
	if !g.check(r.code == 0 && strings.Contains(r.stdout, "2 open"),
		"legacy-format id (L-3) parses alongside primary (Q-01)", r.stdout+r.stderr) {
		return 1
	}
	r = run(self, root, "register", "add", "--summary", "fourth", "--theme",
		"core", "--priority", "P3", "--body", "- **Trigger:** t.", "--dry-run")
	if !g.check(r.code == 0 && strings.Contains(r.stdout, "Q-04"),
		"next_id computes from the union of both formats but issues only the primary shape",
		r.stdout+r.stderr) {
		return 1
	}
	delete(cfg, "legacy_id_prefix")
	delete(cfg, "legacy_id_separator")
	saveCfgMap(cfgPath, cfg)
	trackingText = mustRead(tPath)
	trackingText = strings.Replace(trackingText, "| L-3 | legacy-format item | core | P3 | ☐ | — |\n", "", 1)
	trackingText = strings.Replace(trackingText,
		"### L-3. legacy-format item\n\nTheme: core · Priority: P3 · Status: ☐\n\n"+
			"- **Trigger:** fixture, legacy format.\n\n---\n\n", "", 1)
	mustWrite(tPath, trackingText)

	// 8d. register: check() genuinely catches drift, not just passes
	// on well-formed data -- deliberately desync the status table's
	// priority column from the detail block's own field line.
	originalTracking := mustRead(tPath)
	mustWrite(tPath, strings.Replace(originalTracking,
		"| Q-01 | seed item | core | P2 | ☐ | — |",
		"| Q-01 | seed item | core | P4 | ☐ | — |", 1)) // P2 -> P4, drifted
	r = run(self, root, "register", "check")
	if !g.check(r.code == 1 && strings.Contains(r.stdout, "A3"),
		"register check catches a real table-vs-detail mismatch", r.stdout+r.stderr) {
		return 1
	}
	mustWrite(tPath, originalTracking) // restore
	r = run(self, root, "register", "check")
	if !g.check(r.code == 0, "register check clean again after restore", r.stdout) {
		return 1
	}

	// 8e. guards: a non-default guard_id_prefix works end to end, not
	// just at import time.
	kPath := filepath.Join(root, "docs", "KNOWN_ISSUES.md")
	originalKnownIssues := mustRead(kPath)
	cfg["guard_id_prefix"] = "GRD-"
	saveCfgMap(cfgPath, cfg)
	mustWrite(kPath, strings.Replace(originalKnownIssues, "### G-01. fixture guard", "### GRD-01. fixture guard", 1))
	r = run(self, root, "guards", "list")
	if !g.check(r.code == 0 && strings.Contains(r.stdout, "GRD-01"),
		"non-default guard_id_prefix (GRD-) parses", r.stdout+r.stderr) {
		return 1
	}
	r = run(self, root, "guards", "record", "GRD-01", "--date", "2026-01-04", "--env", "ci")
	if !g.check(r.code == 0, "record works against the custom prefix", r.stderr) {
		return 1
	}
	delete(cfg, "guard_id_prefix")
	saveCfgMap(cfgPath, cfg)
	mustWrite(kPath, originalKnownIssues) // restore default-prefix fixture

	// 8f. guards: a hand-written bullet with two dates NOT in append
	// order -- the later date must be picked regardless of its
	// position in the text.
	twoDates := regexp.MustCompile(`(?m)^- \*\*Last exercised:\*\*.*$`).ReplaceAllString(originalKnownIssues,
		"- **Last exercised:** 2025-12-01 env:m1, following up on the 2026-01-10 finding described above.")
	mustWrite(kPath, twoDates)
	r = run(self, root, "guards", "list")
	if !g.check(r.code == 0 && strings.Contains(r.stdout, "2026-01-10"),
		"last_date picks the later date even when it appears earlier in a hand-written, "+
			"non-append-order bullet", r.stdout+r.stderr) {
		return 1
	}
	mustWrite(kPath, originalKnownIssues) // restore

	// 8g. waves: debt cross-referencing actually fires for a real
	// synthetic case. Add an open register item whose theme is mapped
	// via wave_themes but is not yet any wave's own planned item.
	trackingText = mustRead(tPath)
	trackingText = strings.Replace(trackingText,
		"| Q-01 | seed item | core | P2 | ☐ | — |\n",
		"| Q-01 | seed item | core | P2 | ☐ | — |\n"+
			"| Q-05 | orphaned debt item | extra | P3 | ☐ | — |\n", 1)
	trackingText = strings.Replace(trackingText,
		"### Q-01. seed item",
		"### Q-05. orphaned debt item\n\nTheme: extra · Priority: P3 · Status: ☐\n\n"+
			"- **Trigger:** fixture.\n\n---\n\n### Q-01. seed item", 1)
	mustWrite(tPath, trackingText)
	cfg["wave_themes"] = map[string]interface{}{"1": []interface{}{"extra"}}
	saveCfgMap(cfgPath, cfg)
	r = run(self, root, "waveprogress", "--show")
	if !g.check(r.code == 0 && strings.Contains(r.stdout, "debt: Q-05"),
		"an open item's theme correctly surfaces as wave debt", r.stdout+r.stderr) {
		return 1
	}
	delete(cfg, "wave_themes")
	saveCfgMap(cfgPath, cfg)
	mustWrite(tPath, originalTracking) // restore (drops Q-05 too)

	// 8i. waves: a heading with zero table rows underneath it must
	// warn and skip, never crash the whole regeneration.
	originalWaveTracking := mustRead(wtPath)
	mustWrite(wtPath, originalWaveTracking+
		"\n### Wave 99 — malformed, no rows (0 items, ideal 0d, added 2026-01-01)\n\n(nothing here)\n")
	r = run(self, root, "waveprogress", "--show")
	if !g.check(r.code == 0 && strings.Contains(r.stderr, "WARNING") && !strings.Contains(r.stdout, "Wave 99"),
		"a malformed wave heading (no rows) warns and is skipped, not a crash", r.stdout+r.stderr) {
		return 1
	}
	mustWrite(wtPath, originalWaveTracking) // restore

	return runSection8jPlus(g, root, cfgPath, wtPath, originalWaveTracking)
}
