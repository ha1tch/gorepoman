package selftest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// runSection29 regression-tests T-26: the optional "Claimed-by" register
// field-line segment, an advisory marker for which agent/person has
// picked up an open item -- purely additive, following the exact
// pattern T-06 (Wave) and later Filed-by/Blocks-after set: a new
// optional non-capturing group appended to fieldRe, a new Item struct
// field, a new ItemSummary JSON field (json:"claimed_by,omitempty"),
// and a new register-add flag that writes the segment into the
// generated section template. Board rendering, tier computation, and
// RenderDefinition's axis dispatch never consume it -- deliberately
// out of scope here, same as Wave/Filed-by/Blocks-after before it.
//
// Uses its own fresh project rather than the shared `root` fixture: a
// clean, minimal TRACKING.md makes the exact field-line shape under
// test unambiguous, and this section adds several items whose ids
// would otherwise collide with the shared fixture's own sequence.
func runSection29(g *gate, root string) int {
	self := g.self

	proj := filepath.Join(root, "register-claimed-by-fixture")
	os.MkdirAll(filepath.Join(proj, "docs"), 0755)
	mustWrite(filepath.Join(proj, ".repoman.json"), `{}`)
	mustWrite(filepath.Join(proj, "docs", "TRACKING.md"),
		"# Tracking\n\n"+
			"| ID | Summary | Theme | Priority | Status | Blocks |\n"+
			"|----|---------|-------|----------|--------|--------|\n\n")
	mustWrite(filepath.Join(proj, "docs", "RESOLVED.md"), "# Resolved\n\nClosed items, newest first.\n")

	trackingPath := filepath.Join(proj, "docs", "TRACKING.md")

	// register add --claimed-by writes the segment into the generated
	// field line, in the correct position after Blocks/after.
	r := run(self, proj, "register", "add",
		"--summary", "needs a claimant", "--theme", "core", "--priority", "P2",
		"--claimed-by", "agent-x",
		"--body", "- **Trigger:** fixture.\n\n- **Scope:** fixture.")
	if !g.check(r.code == 0 && strings.Contains(r.stdout, "T-01"),
		"claimed-by: register add --claimed-by succeeds", r.stdout+r.stderr) {
		return 1
	}
	if !g.check(strings.Contains(mustRead(trackingPath), "Status: ☐ · Claimed-by: agent-x"),
		"claimed-by: the generated field line carries the Claimed-by segment", mustRead(trackingPath)) {
		return 1
	}

	// Backward compatibility: an item added WITHOUT --claimed-by gets no
	// segment at all -- the field is genuinely optional, not defaulted
	// to an empty marker.
	r = run(self, proj, "register", "add",
		"--summary", "unclaimed", "--theme", "core", "--priority", "P3",
		"--body", "- **Trigger:** fixture.\n\n- **Scope:** fixture.")
	if !g.check(r.code == 0 && strings.Contains(r.stdout, "T-02"),
		"claimed-by: register add without --claimed-by still succeeds", r.stdout+r.stderr) {
		return 1
	}
	if !g.check(strings.Contains(mustRead(trackingPath), "### T-02. unclaimed\n\nTheme: core · Priority: P3 · Status: ☐\n\n"),
		"claimed-by: an item without --claimed-by gets a field line with no Claimed-by "+
			"segment at all -- unchanged from before T-26", mustRead(trackingPath)) {
		return 1
	}

	// A pre-existing field line combining every optional segment --
	// Wave, Filed-by, Blocks/after (with a comma inside it), and
	// Claimed-by -- all parse together, in order, without the new
	// Claimed-by segment's regex swallowing the preceding Blocks/after
	// segment or vice versa.
	content := mustRead(trackingPath)
	content += "\n## multi-field\n\n### T-03. all optional segments together\n\n" +
		"Theme: multi-field · Priority: P1 · Status: ☐ · Wave: 3 · Filed-by: someone · " +
		"Blocks/after: T-01, T-02 · Claimed-by: agent-y\n\n" +
		"- **Trigger:** fixture.\n\n- **Scope:** fixture.\n"
	mustWrite(trackingPath, content)

	r = run(self, proj, "register", "list", "--format", "json")
	if !g.check(r.code == 0, "claimed-by: register list --format json succeeds", r.stdout+r.stderr) {
		return 1
	}
	var envelope struct {
		Data []struct {
			ID        string `json:"id"`
			Wave      string `json:"wave"`
			FiledBy   string `json:"filed_by"`
			Blocks    string `json:"blocks"`
			ClaimedBy string `json:"claimed_by"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(r.stdout), &envelope); err != nil {
		if !g.check(false, "claimed-by: list --format json output actually parses as JSON", r.stdout) {
			return 1
		}
	}
	byID := map[string]struct {
		ID        string `json:"id"`
		Wave      string `json:"wave"`
		FiledBy   string `json:"filed_by"`
		Blocks    string `json:"blocks"`
		ClaimedBy string `json:"claimed_by"`
	}{}
	for _, item := range envelope.Data {
		byID[item.ID] = item
	}

	if !g.check(byID["T-01"].ClaimedBy == "agent-x",
		"claimed-by: json output for T-01 carries claimed_by from register add", r.stdout) {
		return 1
	}
	if !g.check(byID["T-02"].ClaimedBy == "",
		"claimed-by: json output for T-02 (never claimed) omits claimed_by -- omitempty, "+
			"not an empty-string marker", r.stdout) {
		return 1
	}
	t3 := byID["T-03"]
	if !g.check(t3.Wave == "3" && t3.FiledBy == "someone" && t3.Blocks == "T-01, T-02" && t3.ClaimedBy == "agent-y",
		"claimed-by: json output for T-03 shows Wave, Filed-by, Blocks/after (with its "+
			"internal comma intact) and Claimed-by all parsed correctly together -- the new "+
			"segment's regex does not swallow or get swallowed by the segment before it",
		fmt.Sprintf("%+v\n\n%s", t3, r.stdout)) {
		return 1
	}

	return runSection29a(g, root)
}
