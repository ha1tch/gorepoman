package selftest

import (
	"encoding/json"
	"fmt"
	"strings"
)

// runSection25 regression-tests T-05/B-10: a register with a second,
// permanently live id namespace (poesy's real case -- T-nn for
// general debt, BF-nn for bugfixes, both open at once, neither ever
// retired) used to be silently invisible to `register check` and
// `register list`: undercounting open items with no warning at all,
// because parsing only ever recognised the primary namespace plus one
// optional LegacyIDPrefix fallback meant for a one-time id-shape
// migration, not two namespaces coexisting indefinitely. Fixed by
// letting .repoman.json declare any number of namespaces
// (id_namespaces), each with its own independent NextID counter, via
// config.Config.EffectiveIDNamespaces -- consumed by pkg/register and
// (separately, by design -- see waveprogress.go's own doc comment)
// pkg/waveprogress.
//
// Two independent fixture repositories, each in its own subdirectory
// with its own .repoman.json/TRACKING.md/RESOLVED.md (config.FindRoot
// walks upward for the nearest .repoman.json, so nesting them under
// the shared selftest root does not leak either one's config into the
// other, or into the big shared fixture the rest of this suite uses):
//
//   - section25-multins: a brand-new second namespace (BF), proving
//     independent per-namespace counters, full visibility in check
//     and list, deterministic ordering, and refusal of an
//     unconfigured --id-prefix.
//   - section25-legacy: the OTHER existing mechanism, LegacyIDPrefix,
//     confirmed still folded into the primary namespace's ONE shared
//     counter exactly as before this change (a real regression risk:
//     it would be easy for a GroupKey mistake to accidentally give
//     legacy ids their own independent counter instead).
func runSection25(g *gate, root string) int {
	self := g.self

	// -- multi-namespace: T (primary) and BF (declared in id_namespaces) --
	multiDir := root + "/section25-multins"
	mustWrite(multiDir+"/.repoman.json", `{`+
		`"id_prefix":"T","id_separator":"-",`+
		`"id_namespaces":[{"prefix":"BF","separator":"-"}]}`)
	mustWrite(multiDir+"/docs/TRACKING.md",
		"# Tracking\n\n"+
			"| ID | Summary | Theme | Priority | Status | Blocks |\n"+
			"|----|---------|-------|----------|--------|--------|\n\n")
	mustWrite(multiDir+"/docs/RESOLVED.md", "# Resolved\n\nClosed items, newest first.\n")

	addArgs := func(prefix, summary, theme string) []string {
		a := []string{"register", "add", "--summary", summary, "--theme", theme,
			"--priority", "P1", "--body", "- **Trigger:** fixture."}
		if prefix != "" {
			a = append(a, "--id-prefix", prefix)
		}
		return a
	}

	r := run(self, multiDir, addArgs("", "primary item one", "core")...)
	if !g.check(r.code == 0 && strings.Contains(r.stdout, "T-01"),
		"multi-namespace: add with no --id-prefix uses the primary namespace (T-01)", r.stdout+r.stderr) {
		return 1
	}
	r = run(self, multiDir, addArgs("BF", "bugfix one", "bugs")...)
	if !g.check(r.code == 0 && strings.Contains(r.stdout, "BF-01"),
		"multi-namespace: --id-prefix BF allocates from its own namespace, starting at BF-01, "+
			"not continuing T's sequence", r.stdout+r.stderr) {
		return 1
	}
	r = run(self, multiDir, addArgs("", "primary item two", "core")...)
	if !g.check(r.code == 0 && strings.Contains(r.stdout, "T-02"),
		"multi-namespace: a further primary-namespace add reaches T-02, proving BF-01 did not "+
			"consume a number from T's own counter", r.stdout+r.stderr) {
		return 1
	}
	r = run(self, multiDir, addArgs("BF", "bugfix two", "bugs")...)
	if !g.check(r.code == 0 && strings.Contains(r.stdout, "BF-02"),
		"multi-namespace: BF's own counter continues independently at BF-02, not colliding with "+
			"or restarting relative to T's numbers", r.stdout+r.stderr) {
		return 1
	}

	r = run(self, multiDir, "register", "check")
	if !g.check(r.code == 0 && strings.Contains(r.stdout, "4 open"),
		"multi-namespace: check counts all four items across both namespaces -- this is the "+
			"actual B-10 fix: a second live namespace is no longer silently invisible", r.stdout) {
		return 1
	}

	r1 := run(self, multiDir, "register", "list")
	if !g.check(r1.code == 0 && strings.Contains(r1.stdout, "T-01") && strings.Contains(r1.stdout, "T-02") &&
		strings.Contains(r1.stdout, "BF-01") && strings.Contains(r1.stdout, "BF-02"),
		"multi-namespace: list shows every item from both namespaces, not just the primary one",
		r1.stdout+r1.stderr) {
		return 1
	}
	r2 := run(self, multiDir, "register", "list")
	if !g.check(r2.stdout == r1.stdout,
		"multi-namespace: list order is deterministic across repeated runs -- a plain numeric "+
			"tie between namespaces (T-01 and BF-01 both parse to Num=1) used to leave relative "+
			"order to Go's unordered map iteration; sort.SliceStable plus a group-aware "+
			"comparator fixes that", fmt.Sprintf("run1:\n%s\nrun2:\n%s", r1.stdout, r2.stdout)) {
		return 1
	}

	r = run(self, multiDir, "register", "list", "--format", "json")
	if !g.check(r.code == 0, "multi-namespace: list --format json succeeds too", r.stdout+r.stderr) {
		return 1
	}
	var jsonEnvelope struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(r.stdout), &jsonEnvelope); err != nil {
		if !g.check(false, "multi-namespace: list --format json output actually parses as JSON", r.stdout) {
			return 1
		}
	}
	gotIDs := map[string]bool{}
	for _, item := range jsonEnvelope.Data {
		gotIDs[item.ID] = true
	}
	if !g.check(len(jsonEnvelope.Data) == 4 && gotIDs["T-01"] && gotIDs["T-02"] && gotIDs["BF-01"] && gotIDs["BF-02"],
		"multi-namespace: the json path sees the same four items as the text path -- both "+
			"paths share sortedTIDs, so they cannot drift from each other the way the text path "+
			"used to (it had its own separate inline sort before this fix)", r.stdout) {
		return 1
	}

	r = run(self, multiDir, addArgs("ZZ", "should be refused", "core")...)
	if !g.check(r.code != 0 && strings.Contains(r.stderr, "ZZ") && strings.Contains(r.stderr, "not a configured namespace"),
		"multi-namespace: --id-prefix for a namespace nobody declared is refused with an "+
			"actionable message, not silently accepted or silently defaulted to the primary "+
			"namespace", r.stdout+r.stderr) {
		return 1
	}
	r = run(self, multiDir, "register", "check")
	if !g.check(r.code == 0 && strings.Contains(r.stdout, "4 open"),
		"multi-namespace: the refused add above wrote nothing -- still exactly four items", r.stdout) {
		return 1
	}

	// -- legacy fold: LegacyIDPrefix still shares ONE counter with the
	// primary namespace, unchanged by this session's work. Seeds an
	// existing legacy-shaped id (T-05) and confirms the very next
	// primary-namespace add continues that same sequence at 6, rather
	// than starting its own independent namespace at 1 -- the failure
	// this would produce if GroupKey were wired wrong is a silent,
	// no-error id collision risk with the old T-nn ids the moment
	// enough new ones were added, which no build or vet check would
	// ever catch on its own.
	legacyDir := root + "/section25-legacy"
	mustWrite(legacyDir+"/.repoman.json", `{`+
		`"id_prefix":"XOT","id_separator":"",`+
		`"legacy_id_prefix":"T","legacy_id_separator":"-"}`)
	mustWrite(legacyDir+"/docs/TRACKING.md",
		"# Tracking\n\n"+
			"| ID | Summary | Theme | Priority | Status | Blocks |\n"+
			"|----|---------|-------|----------|--------|--------|\n\n"+
			"| T-05 | legacy item | core | P1 | ☐ | — |\n\n"+
			"## core\n\n### T-05. legacy item\n\n"+
			"Theme: core · Priority: P1 · Status: ☐\n\n"+
			"- **Trigger:** fixture.\n\n")
	mustWrite(legacyDir+"/docs/RESOLVED.md", "# Resolved\n\nClosed items, newest first.\n")

	r = run(self, legacyDir, "register", "add", "--summary", "forward-only item", "--theme", "core",
		"--priority", "P1", "--body", "- **Trigger:** fixture.")
	if !g.check(r.code == 0 && strings.Contains(r.stdout, "XOT06"),
		"legacy fold: a primary-namespace add after an existing T-05 continues the SAME shared "+
			"counter at 6 (XOT06), proving legacy ids still fold into the primary namespace's "+
			"counter group rather than gaining an independent one of their own -- the historical, "+
			"pre-this-change migration behaviour, unchanged", r.stdout+r.stderr) {
		return 1
	}
	r = run(self, legacyDir, "register", "check")
	if !g.check(r.code == 0 && strings.Contains(r.stdout, "2 open"),
		"legacy fold: check still sees both the legacy-shaped id and the new primary one", r.stdout) {
		return 1
	}

	return runSection26(g, root)
}
