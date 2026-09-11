// Package waveprogress regenerates a wave-tracking document's own
// "## 1. Progress at a glance" summary from each wave's own per-item
// status table below it -- translated from repoman/wave_progress.py.
//
// Ported from xolu's own scripts/wave_progress.py, which carries the
// original design rationale in full. That rationale is preserved
// here verbatim where it's still exactly true; only the bindings
// changed:
//
//   - The wave-tracking document and the register file it
//     cross-references are config-driven (wave_tracking, tracking in
//     .repoman.json), not hardcoded paths.
//   - Short names and wave themes are config-driven data
//     (wave_short_names, wave_themes), not language literals -- both
//     default to empty, so a consumer that never sets them sees
//     exactly the behaviour of a project with no waves configured
//     yet: no summary line to regenerate, nothing to break.
//   - The id-format handling reads the same id_prefix / id_separator /
//     legacy_id_prefix / legacy_id_separator / id_namespaces keys
//     pkg/register reads, generalizing the same idea to any number of
//     permanently coexisting namespaces (not just a one-time legacy
//     migration) -- re-implemented here rather than imported from
//     pkg/register, preserving this package's own original design
//     choice (from wave_progress.py) to remain free of a cross-package
//     dependency on pkg/register specifically for this one shared
//     regex fragment; config.Config.EffectiveIDNamespaces (pkg/config,
//     already a dependency here) is what both packages now build that
//     fragment from.
//
// Why this exists (xolu's own original rationale, unchanged): a
// hand-maintained progress summary drifts from the data it
// summarises. This tool makes the summary a pure function of the
// per-wave tables, which stay hand-maintained (their per-item Notes
// are genuinely bespoke prose, not something to generate) -- only the
// roll-up is mechanical.
//
// Per-wave percentage: a table row's Status column is one of the
// three project-wide markers (✓ done, ◐ partial, ☐ not started). A
// row counts as 1.0 item done, 0.5, or 0.0 respectively; percentage
// is that sum over the row count. A "~" prefix marks any wave
// containing a ◐ row.
//
// Each wave also gets a "debt:" subtitle line, listing open register
// items whose theme is mapped to that wave via wave_themes -- open
// technical debt that shares a wave's subject matter but was never
// one of that wave's own planned items. Items already counted as SOME
// wave's own item (per that wave's own table column 4, not a theme
// guess) are excluded even if their theme happens to match a
// different wave. Waves absent from wave_themes simply show no debt
// line.
//
// A wave also gets a "blockers:" subtitle when any of its own items
// or its debt is waiting on a currently-open prerequisite (an
// "After: X" field in the register where X is itself still open).
//
// Deliberately NOT preserved: hand-composed trailing annotations.
// Those are narrative, not data, and belong in each wave's own detail
// section below the summary (untouched by this tool).
//
// Requires the wave-tracking document to carry a "## 1. Progress at a
// glance" section with a fenced code block (```...```) and, somewhere
// in that document, a line of the form "Overall by item count: N of M
// items ≈ **P%**" -- a documented structural convention, the same way
// pkg/register requires "## <theme>" groupings, not itself a config
// key.
//
// Usage:
//
//	repoman waveprogress                  // regenerate in place
//	repoman waveprogress --check          // exit 1 if it would change, don't write
//	repoman waveprogress --show           // print to stdout, touch nothing
//	repoman waveprogress --html PATH      // standalone HTML file
//	repoman waveprogress --hide 6         // persist wave 6 as hidden
//	repoman waveprogress --unhide 6       // persist wave 6 as visible again
//	repoman waveprogress --show --include-hidden   // render everything anyway
package waveprogress

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/ha1tch/gorepoman/pkg/config"
	"github.com/ha1tch/gorepoman/pkg/report"
	"github.com/ha1tch/gorepoman/pkg/webhelp"
)

var statusWeight = map[string]float64{"✓": 1.0, "◐": 0.5, "☐": 0.0}

// wave is one parsed "### Wave <id> — ..." section.
type wave struct {
	ID         string
	DoneEquiv  float64
	Total      int
	HasPartial bool
	ItemTnums  map[string]bool
}

// env bundles the config-derived regexes and paths every function
// below needs, mirroring wave_progress.py's own module-level
// bindings (computed once, from config, at the start of a run).
type env struct {
	root string
	cfg  config.Config

	docPath      string
	trackingPath string
	resolvedPath string

	headingRe   *regexp.Regexp
	rowRe       *regexp.Regexp
	itemCol4Re  *regexp.Regexp
	rangeRe     *regexp.Regexp
	tnumRe      *regexp.Regexp
	rowTRe      *regexp.Regexp
	afterRe     *regexp.Regexp
	trackingRow *regexp.Regexp
	waveRowRe   *regexp.Regexp
	resolvedHdr *regexp.Regexp
}

func newEnv() (*env, error) {
	root, cfg, err := config.Load("")
	if err != nil {
		return nil, err
	}

	idAlt := idAltPattern(cfg)
	prefixAlt := prefixAltPattern(cfg)

	e := &env{
		root:         root,
		cfg:          cfg,
		docPath:      filepath.Join(root, cfg.WaveTracking),
		trackingPath: filepath.Join(root, cfg.Tracking),
		resolvedPath: filepath.Join(root, cfg.Resolved),

		headingRe:   regexp.MustCompile(`(?m)^### Wave (\S+) — [^(]+\(`),
		rowRe:       regexp.MustCompile(`(?m)^\|\s*\d+\s*\|.*\|\s*([✓◐☐])\s*\|`),
		itemCol4Re:  regexp.MustCompile(`(?m)^\|\s*\d+\s*\|[^|]*\|\s*[✓◐☐]\s*\|([^|]*)\|`),
		rangeRe:     regexp.MustCompile(`(` + prefixAlt + `)(\d+)\s+through\s+(?:` + prefixAlt + `)(\d+)`),
		tnumRe:      regexp.MustCompile(`(` + idAlt + `)`),
		rowTRe:      regexp.MustCompile(`(?m)^\|\s*(` + idAlt + `)\s*\|[^|]*\|\s*([a-z0-9-]+)\s*\|`),
		afterRe:     regexp.MustCompile(`After:\s*(` + idAlt + `)`),
		waveRowRe:   regexp.MustCompile(`(?m)^(\|\s*\d+\s*\|[^|]*\|\s*)([✓◐☐])(\s*\|)([^|]*)(\|)\s*$`),
		resolvedHdr: regexp.MustCompile(`(?m)^## \[[^\]]*\]\s+(` + idAlt + `)\s`),
		trackingRow: regexp.MustCompile(`(?m)^\| (` + idAlt + `) \| ([^|]*) \| ([a-z0-9-]+) \| (P\d) \| ([✓◐☐✗☑]) \| ([^|]*) \|`),
	}
	return e, nil
}

// sortedIDNamespaces returns cfg's recognised namespaces, longest
// prefix+separator first so a namespace whose prefix is a leading
// substring of another's (e.g. "T-" vs "TB-") never misclassifies the
// longer one's ids -- shared by every function below that builds a
// pattern or parses an id against these namespaces.
func sortedIDNamespaces(cfg config.Config) []config.EffectiveIDNamespace {
	nss := cfg.EffectiveIDNamespaces()
	sort.SliceStable(nss, func(i, j int) bool {
		return len(nss[i].Prefix+nss[i].Separator) > len(nss[j].Prefix+nss[j].Separator)
	})
	return nss
}

func idAltPattern(cfg config.Config) string {
	nss := sortedIDNamespaces(cfg)
	if len(nss) == 1 {
		return regexp.QuoteMeta(nss[0].Prefix) + regexp.QuoteMeta(nss[0].Separator) + `\d+`
	}
	parts := make([]string, len(nss))
	for i, ns := range nss {
		parts[i] = regexp.QuoteMeta(ns.Prefix) + regexp.QuoteMeta(ns.Separator)
	}
	return "(?:" + strings.Join(parts, "|") + `)\d+`
}

func prefixAltPattern(cfg config.Config) string {
	nss := sortedIDNamespaces(cfg)
	if len(nss) == 1 {
		return regexp.QuoteMeta(nss[0].Prefix) + regexp.QuoteMeta(nss[0].Separator)
	}
	parts := make([]string, len(nss))
	for i, ns := range nss {
		parts[i] = regexp.QuoteMeta(ns.Prefix) + regexp.QuoteMeta(ns.Separator)
	}
	return "(?:" + strings.Join(parts, "|") + ")"
}

// idParsed mirrors pkg/register's own idParsed (not imported, per
// this package's design choice above): the numeric part of an id
// plus which counter group it belongs to, so ids from different
// namespaces sort deterministically instead of tying on Num alone.
type idParsed struct {
	Num      int
	GroupKey string
}

// idNum returns the numeric portion of an id and which namespace's
// counter group it belongs to, trying the longest prefix+separator
// first (see sortedIDNamespaces).
func idNum(tid string, cfg config.Config) (idParsed, error) {
	for _, ns := range sortedIDNamespaces(cfg) {
		full := ns.Prefix + ns.Separator
		if strings.HasPrefix(tid, full) {
			n, err := strconv.Atoi(tid[len(full):])
			if err != nil {
				return idParsed{}, err
			}
			return idParsed{Num: n, GroupKey: ns.GroupKey}, nil
		}
	}
	return idParsed{}, fmt.Errorf("unrecognized id format: %q", tid)
}

// idLess orders two ids the same way pkg/register's own idLess does:
// ids sharing a counter group sort by number among themselves; ids
// from different groups sort by group key first ("" -- primary/
// legacy -- before any other namespace's own prefix).
func idLess(a, b string, cfg config.Config) bool {
	pa, errA := idNum(a, cfg)
	pb, errB := idNum(b, cfg)
	if errA != nil || errB != nil {
		return a < b
	}
	if pa.GroupKey != pb.GroupKey {
		return pa.GroupKey < pb.GroupKey
	}
	return pa.Num < pb.Num
}

// isVisible: absent entry = visible. This is the ONLY place either
// renderer consults visibility state -- renderTable and renderHTML
// both call this rather than each keeping their own notion of what's
// shown, so ASCII and HTML can never disagree about which waves are
// currently hidden.
func isVisible(cfg config.Config, waveID string) bool {
	v, ok := cfg.WaveVisibility[waveID]
	if !ok {
		return true
	}
	return v
}

// allWaveTnums returns every id that is literally one of SOME wave's
// own numbered items, per that wave's own table column 4 -- not just
// literal id mentions (which also silently drop the middle of a "X
// through Y" range notation) but the expanded range too. Used as a
// global exclusion set for the debt calculation: a theme match alone
// isn't enough -- an id already counted as one wave's own item must
// never double up as a different wave's debt just because its theme
// happens to match.
func (e *env) allWaveTnums(text string) map[string]bool {
	result := map[string]bool{}
	for _, col4m := range e.itemCol4Re.FindAllStringSubmatch(text, -1) {
		col4 := col4m[1]
		for _, rm := range e.rangeRe.FindAllStringSubmatch(col4, -1) {
			prefix, lo, hi := rm[1], rm[2], rm[3]
			loN, _ := strconv.Atoi(lo)
			hiN, _ := strconv.Atoi(hi)
			for n := loN; n <= hiN; n++ {
				if n < 100 {
					result[fmt.Sprintf("%s%02d", prefix, n)] = true
				} else {
					result[fmt.Sprintf("%s%d", prefix, n)] = true
				}
			}
		}
		for _, tm := range e.tnumRe.FindAllString(col4, -1) {
			result[tm] = true
		}
	}
	return result
}

// syncWaveRowsFromRegister is FR-04: derives each wave-table row's
// own Status cell (✓/◐/☐) from the actual current state of the
// register/RESOLVED documents, and rewrites text to match before
// parseWaves ever reads it. Composes cleanly with the B-04 fix
// above -- this corrects the rows, B-04's existing regeneration then
// recomputes the summary sentence from those now-correct rows, with
// no duplicated logic between the two.
//
// Before this existed, nothing regenerated a wave row's own status
// outside of register close's own direct wiring (propagateCloseToWaves
// in pkg/register) -- a row could drift from reality whenever an
// item's closure or status change happened any other way (RESOLVED.md
// gaining a closure header by hand, or a status edit that bypassed
// register close entirely), and neither this command nor register
// close had any way to notice or repair that.
//
// Only rows that carry a register_item id (column 4, via itemCol4Re)
// are touched -- a wave item with no linked ticket yet has no
// register/RESOLVED state to derive from, and is left exactly as
// found, the same as before this function existed. A row whose id is
// found in NEITHER the tracking document nor RESOLVED.md's closure
// headers is left untouched and reported as a warning: this function
// corrects rows from real state, it never guesses at one.
//
// Ties are resolved in RESOLVED's favor: an id that (incorrectly)
// appears in both is closed, never open -- RESOLVED.md's closure
// headers are the more specific claim (this exact id, at this exact
// version, closed on this exact date) against TRACKING.md's plainer
// "still has an open row" signal.
func (e *env) syncWaveRowsFromRegister(text string) (string, []string) {
	var warnings []string

	closed := map[string]bool{}
	if b, err := os.ReadFile(e.resolvedPath); err == nil {
		for _, m := range e.resolvedHdr.FindAllStringSubmatch(string(b), -1) {
			closed[m[1]] = true
		}
	}

	openStatus := map[string]string{}
	if b, err := os.ReadFile(e.trackingPath); err == nil {
		for _, m := range e.trackingRow.FindAllStringSubmatch(string(b), -1) {
			openStatus[m[1]] = m[5]
		}
	}

	newText := e.waveRowRe.ReplaceAllStringFunc(text, func(row string) string {
		m := e.waveRowRe.FindStringSubmatch(row)
		if m == nil {
			return row
		}
		col4 := m[4]
		ids := e.tnumRe.FindAllString(col4, -1)
		if len(ids) == 0 {
			return row
		}

		want := ""
		for _, id := range ids {
			var idStatus string
			switch {
			case closed[id]:
				idStatus = "✓"
			case openStatus[id] != "":
				idStatus = wavelevelStatus(openStatus[id])
			default:
				warnings = append(warnings, fmt.Sprintf(
					"wave_progress: %s is linked from a wave row but found in neither %s nor %s's closure headers -- row left as-is",
					id, filepath.Base(e.trackingPath), filepath.Base(e.resolvedPath)))
				return row
			}
			// A row can name more than one id via a range (T-1 through
			// T-3): the row is only as done as its LEAST done member --
			// ✓ only if every one of them is ✓, ◐ if any progress at
			// all, ☐ only if literally none.
			want = combineStatus(want, idStatus)
		}
		if want == "" || want == m[2] {
			return row
		}
		return m[1] + want + m[3] + m[4] + m[5]
	})

	return newText, warnings
}

// combineStatus folds one more member status into a running row
// status: ✓ only survives if every member so far was also ✓; any ◐
// (or a mix of ✓ and ☐) downgrades to ◐; otherwise ☐. acc == "" means
// "first member seen", so its status is taken as-is.
func combineStatus(acc, next string) string {
	if acc == "" {
		return next
	}
	if acc == next {
		return acc
	}
	return "◐"
}

// wavelevelStatus maps a register item status (the full five-symbol
// legend: ✓◐☐✗☑, see pkg/register) down to the three-symbol
// vocabulary a wave-table row itself uses (✓◐☐). ✓ (closed) and ◐/☐
// pass through unchanged. ☑ (done, pending release) is genuine,
// real progress -- implementation is finished and verified -- but
// it is not yet closed via register close from a wave-tracking
// point of view, so it reads as ◐: neither fully done nor
// untouched. ✗ (dropped) contributes nothing to a wave's progress,
// so it reads as ☐. An unrecognised status (should not happen,
// since trackingRow's regex only ever captures one of the five
// legal symbols) also falls back to ☐ rather than panicking or
// silently passing through an unknown rune.
func wavelevelStatus(registerStatus string) string {
	switch registerStatus {
	case "✓", "◐", "☐":
		return registerStatus
	case "☑":
		return "◐"
	case "✗":
		return "☐"
	default:
		return "☐"
	}
}

func (e *env) parseWaves(text string) []wave {
	headings := e.headingRe.FindAllStringSubmatchIndex(text, -1)
	var waves []wave
	for i, m := range headings {
		waveID := text[m[2]:m[3]]
		start := m[1]
		end := len(text)
		if i+1 < len(headings) {
			end = headings[i+1][0]
		}
		section := text[start:end]
		rowMatches := e.rowRe.FindAllStringSubmatch(section, -1)
		if len(rowMatches) == 0 {
			fmt.Fprintf(os.Stderr, "WARNING: Wave %s heading found but no table rows parsed -- skipped, check the table format\n", waveID)
			continue
		}
		var doneEquiv float64
		hasPartial := false
		for _, rm := range rowMatches {
			status := rm[1]
			doneEquiv += statusWeight[status]
			if status == "◐" {
				hasPartial = true
			}
		}
		waves = append(waves, wave{
			ID:         waveID,
			DoneEquiv:  doneEquiv,
			Total:      len(rowMatches),
			HasPartial: hasPartial,
			ItemTnums:  e.allWaveTnums(section),
		})
	}
	return waves
}

// debtByWave returns open register items, grouped by wave via
// wave_themes, excluding anything that's already SOME wave's own
// item. Themes with no wave mapping are simply absent from the
// result.
func (e *env) debtByWave(fullText string) map[string][]string {
	result := map[string][]string{}
	b, err := os.ReadFile(e.trackingPath)
	if err != nil {
		return result
	}
	text := string(b)
	alreadyAWaveItem := e.allWaveTnums(fullText)

	themeToWave := map[string]string{}
	for waveID, themes := range e.cfg.WaveThemes {
		for _, t := range themes {
			themeToWave[t] = waveID
		}
	}

	for _, m := range e.rowTRe.FindAllStringSubmatch(text, -1) {
		tnum, theme := m[1], m[2]
		if alreadyAWaveItem[tnum] {
			continue
		}
		if waveID, ok := themeToWave[theme]; ok {
			result[waveID] = append(result[waveID], tnum)
		}
	}
	return result
}

func bar(pct float64) string {
	filled := pyRound(pct / 5)
	if filled < 0 {
		filled = 0
	}
	if filled > 20 {
		filled = 20
	}
	return strings.Repeat("█", filled) + strings.Repeat("░", 20-filled)
}

func fmtItemCount(doneEquiv float64, total int) string {
	return fmt.Sprintf("%s/%d items", formatG(doneEquiv), total)
}

type blockerEntry struct {
	Blocker string
	Blocked []string
}

// blockersByWave: for each wave's own items plus its debt items,
// checks their "After: X" field and keeps only references that
// resolve to a CURRENTLY OPEN item. Returns wave id -> [(blocker,
// [items it blocks in this wave]), ...], grouped by blocker. A
// blocker can be outside the wave entirely -- that cross-wave case is
// the useful signal, not filtered out.
func (e *env) blockersByWave(fullText string, waves []wave) map[string][]blockerEntry {
	result := map[string][]blockerEntry{}
	b, err := os.ReadFile(e.trackingPath)
	if err != nil {
		return result
	}
	trackingText := string(b)
	rows := e.trackingRow.FindAllStringSubmatch(trackingText, -1)

	openTnums := map[string]bool{}
	for _, r := range rows {
		openTnums[r[1]] = true
	}
	afterField := map[string]string{}
	for _, r := range rows {
		tnum, blocks := r[1], r[6]
		if am := e.afterRe.FindStringSubmatch(blocks); am != nil && openTnums[am[1]] {
			afterField[tnum] = am[1]
		}
	}

	debt := e.debtByWave(fullText)
	for _, w := range waves {
		scope := map[string]bool{}
		for t := range w.ItemTnums {
			scope[t] = true
		}
		for _, t := range debt[w.ID] {
			scope[t] = true
		}
		items := make([]string, 0, len(scope))
		for t := range scope {
			items = append(items, t)
		}
		sort.SliceStable(items, func(i, j int) bool {
			return idLess(items[i], items[j], e.cfg)
		})

		byBlocker := map[string][]string{}
		for _, item := range items {
			if blocker, ok := afterField[item]; ok {
				byBlocker[blocker] = append(byBlocker[blocker], item)
			}
		}
		if len(byBlocker) > 0 {
			keys := make([]string, 0, len(byBlocker))
			for k := range byBlocker {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			var entries []blockerEntry
			for _, k := range keys {
				entries = append(entries, blockerEntry{Blocker: k, Blocked: byBlocker[k]})
			}
			result[w.ID] = entries
		}
	}
	return result
}

func (e *env) renderTable(waves []wave, fullText string) string {
	debt := e.debtByWave(fullText)
	blockers := e.blockersByWave(fullText, waves)

	labelW := 0
	for _, w := range waves {
		if l := len("Wave " + w.ID); l > labelW {
			labelW = l
		}
	}
	labelW += 2

	var lines []string
	for _, w := range waves {
		pct := 0.0
		if w.Total > 0 {
			pct = 100 * w.DoneEquiv / float64(w.Total)
		}
		pctS := fmt.Sprintf("%d%%", pyRound(pct))
		if w.HasPartial {
			pctS = "~" + pctS
		}
		label := padRight("Wave "+w.ID, labelW)
		name := e.cfg.WaveShortNames[w.ID]
		if name == "" {
			name = fmt.Sprintf("(unnamed wave %s)", w.ID)
		}
		lines = append(lines, fmt.Sprintf("%s%s  %s  %s  (%s)",
			label, padRight(name, 26), bar(pct), padLeft(pctS, 5), fmtItemCount(w.DoneEquiv, w.Total)))

		indent := strings.Repeat(" ", labelW+26+2)
		if wd := debt[w.ID]; len(wd) > 0 {
			lines = append(lines, indent+"debt: "+strings.Join(wd, ", "))
		}
		if wb := blockers[w.ID]; len(wb) > 0 {
			var parts []string
			for _, be := range wb {
				parts = append(parts, fmt.Sprintf("%s blocks %s", be.Blocker, strings.Join(be.Blocked, ", ")))
			}
			lines = append(lines, indent+"blockers: "+strings.Join(parts, "; "))
		}
	}
	return strings.Join(lines, "\n")
}

func renderOverall(waves []wave) string {
	var totalDone float64
	var totalItems int
	for _, w := range waves {
		totalDone += w.DoneEquiv
		totalItems += w.Total
	}
	pct := 0
	if totalItems > 0 {
		pct = pyRound(100 * totalDone / float64(totalItems))
	}
	return fmt.Sprintf("Overall by item count: %s of %d items \u2248 **%d%%**", formatG(totalDone), totalItems, pct)
}

const htmlHead = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{title}</title>
<style>
:root {
  --bg: #ffffff; --surface: #f4f3f1; --text: #0b0b0b; --text-2: #52514e; --text-3: #898781;
  --border: #e1e0d9;
  --success: #0ca30c;
  --warn-track: #fab219; --warn-bg: #faeeda; --warn-text: #854f0b;
  --acc-track: #2a78d6; --acc-bg: #e6f1fb; --acc-text: #0c447c;
  --muted-track: #d3d1c7;
}
@media (prefers-color-scheme: dark) {
  :root {
    --bg: #16161a; --surface: #232327; --text: #f0efec; --text-2: #c3c2b7; --text-3: #898781;
    --border: #333338;
    --success: #2ed92e;
    --warn-track: #fab219; --warn-bg: #4a3417; --warn-text: #f5c876;
    --acc-track: #4a95e6; --acc-bg: #163a5c; --acc-text: #a9d1f7;
    --muted-track: #3a3a3e;
  }
}
* { box-sizing: border-box; }
body {
  font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Helvetica, Arial, sans-serif;
  background: var(--bg); color: var(--text); margin: 0; padding: 2rem 1.25rem;
}
.wrap { max-width: 720px; margin: 0 auto; }
h1 { font-size: 15px; font-weight: 600; margin: 0 0 1.25rem; color: var(--text-2); }
.row { display: grid; grid-template-columns: 160px 1fr; gap: 10px; padding: 5px 0; }
.row.sub { padding: 0 0 5px; }
.wname { font-size: 13px; font-weight: 600; text-align: right; }
.sublabel { font-size: 11px; color: var(--text-3); text-align: right; padding-top: 1px; }
.barwrap { display: flex; align-items: center; gap: 8px; }
.track { flex: 1; height: 7px; border-radius: 4px; background: var(--surface); overflow: hidden; }
.fill { height: 100%; border-radius: 4px; }
.fill.success { background: var(--success); }
.fill.warn { background: var(--warn-track); }
.fill.muted { background: var(--muted-track); }
.fill.acc { background: var(--acc-track); }
.count { font-size: 11px; color: var(--text-2); white-space: nowrap; }
.pills { display: flex; gap: 6px; flex-wrap: wrap; }
.pill { font-size: 11px; padding: 1px 7px; border-radius: 6px; white-space: nowrap; }
.pill.debt { background: var(--warn-bg); color: var(--warn-text); }
.pill.blocker { background: var(--acc-bg); color: var(--acc-text); }
.overall { border-top: 1px solid var(--border); margin-top: 6px; padding-top: 10px; }
.overall .wname { font-size: 14px; }
.overall .count { font-weight: 600; }
</style>
</head>
<body>
<div class="wrap">
`

const htmlTail = `</div>
</body>
</html>
`

func htmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	return s
}

// renderHTML is a standalone HTML rendering of the same wave/debt/
// blocker data renderTable already computes for the terminal --
// genuinely portable: real hex colours (light + dark via
// prefers-color-scheme), no dependency on any host page's own CSS
// variables, opens correctly in any browser with the file handed to
// it directly. Layout matches this project's own established
// terminal convention exactly, not a redesign: wave name (or a
// "debt"/"blocked by" sublabel) right-aligned in a fixed left column,
// bar/pills left-aligned in the column beside it, starting at the
// same point the bar itself starts.
func (e *env) renderHTML(waves []wave, fullText string, title string) string {
	debt := e.debtByWave(fullText)
	blockers := e.blockersByWave(fullText, waves)

	var b strings.Builder
	b.WriteString(strings.Replace(htmlHead, "{title}", htmlEscape(title), 1))
	fmt.Fprintf(&b, "<h1>%s</h1>\n", htmlEscape(title))

	fillClass := func(pct float64, hasPartial bool) string {
		if pct <= 0 {
			return "muted"
		}
		if pct >= 100 && !hasPartial {
			return "success"
		}
		return "warn"
	}

	for _, w := range waves {
		pct := 0.0
		if w.Total > 0 {
			pct = 100 * w.DoneEquiv / float64(w.Total)
		}
		pctS := fmt.Sprintf("%d%%", pyRound(pct))
		if w.HasPartial {
			pctS = "~" + pctS
		}
		name := e.cfg.WaveShortNames[w.ID]
		if name == "" {
			name = fmt.Sprintf("(unnamed wave %s)", w.ID)
		}
		cls := fillClass(pct, w.HasPartial)
		widthPct := pct
		if widthPct > 100 {
			widthPct = 100
		}
		fmt.Fprintf(&b, `<div class="row"><span class="wname">Wave %s — %s</span><div class="barwrap">`+
			`<div class="track"><div class="fill %s" style="width:%s%%"></div></div>`+
			`<span class="count">%s · %s</span></div></div>`+"\n",
			htmlEscape(w.ID), htmlEscape(name), cls, formatG4(widthPct), pctS, fmtItemCount(w.DoneEquiv, w.Total))

		if wd := debt[w.ID]; len(wd) > 0 {
			var pills strings.Builder
			for _, d := range wd {
				fmt.Fprintf(&pills, `<span class="pill debt">%s</span>`, htmlEscape(d))
			}
			fmt.Fprintf(&b, `<div class="row sub"><span class="sublabel">debt</span><div class="pills">%s</div></div>`+"\n", pills.String())
		}
		if wb := blockers[w.ID]; len(wb) > 0 {
			var pills strings.Builder
			for _, be := range wb {
				fmt.Fprintf(&pills, `<span class="pill blocker">%s → %s</span>`, htmlEscape(be.Blocker), htmlEscape(strings.Join(be.Blocked, ", ")))
			}
			fmt.Fprintf(&b, `<div class="row sub"><span class="sublabel">blocked by</span><div class="pills">%s</div></div>`+"\n", pills.String())
		}
	}

	var totalDone float64
	var totalItems int
	for _, w := range waves {
		totalDone += w.DoneEquiv
		totalItems += w.Total
	}
	overallPct := 0.0
	if totalItems > 0 {
		overallPct = 100 * totalDone / float64(totalItems)
	}
	widthPct := overallPct
	if widthPct > 100 {
		widthPct = 100
	}
	fmt.Fprintf(&b, `<div class="row overall"><span class="wname">Overall</span><div class="barwrap">`+
		`<div class="track"><div class="fill acc" style="width:%s%%"></div></div>`+
		`<span class="count">%d%% · %s</span></div></div>`+"\n",
		formatG4(widthPct), pyRound(overallPct), fmtItemCount(totalDone, totalItems))

	b.WriteString(htmlTail)
	return b.String()
}

// setVisibility implements --hide/--unhide: a single, distinct
// mutation, same shape as pkg/guards's own `record` command --
// persists to .repoman.json via config.SaveKey, confirms, exits.
// Never combined with a render operation in the same invocation, so
// it's always obvious from the command alone whether state changed or
// something was only displayed.
func (e *env) setVisibility(waveID string, visible bool) int {
	var docText string
	if b, err := os.ReadFile(e.docPath); err == nil {
		docText = string(b)
	}
	known := map[string]bool{}
	if docText != "" {
		for _, w := range e.parseWaves(docText) {
			known[w.ID] = true
		}
	}
	if len(known) > 0 && !known[waveID] {
		names := make([]string, 0, len(known))
		for k := range known {
			names = append(names, k)
		}
		sort.Strings(names)
		fmt.Fprintf(os.Stderr, "warning: %q is not a wave id found in %s (known: %s) -- setting anyway, in case the wave document hasn't been regenerated yet\n",
			waveID, filepath.Base(e.docPath), strings.Join(names, ", "))
	}

	visibility := map[string]bool{}
	for k, v := range e.cfg.WaveVisibility {
		visibility[k] = v
	}
	visibility[waveID] = visible

	if err := config.SaveKey(e.root, "wave_visibility", visibility); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	state := "hidden"
	if visible {
		state = "visible"
	}
	fmt.Printf("wave %s: %s (persisted to .repoman.json)\n", waveID, state)
	return 0
}

const waveprogressHelp = `usage: repoman waveprogress [-h] [--show] [--check] [--html PATH]
                             [--hide WAVE_ID] [--unhide WAVE_ID]
                             [--include-hidden]

repoman waveprogress -- regenerates a wave-tracking document's own
"Progress at a glance" summary from each wave's per-item status
table, and renders that same data as ASCII or HTML progress bars.
It does not create waves or assign register items to one -- that's
` + "`repoman addwave`" + `'s job (run ` + "`repoman addwave -h`" + ` for that). This
command only ever reads and re-renders what addwave, and hand edits
to each wave's own item table afterward, have already put there.

How waves relate to the register: the register (` + "`repoman register`" + `)
tracks individual open items (T-nn). Waves are a separate, staged
programme of work -- "wave 1 is the pool rewrite, wave 2 depends on
it" -- with their own progress bars and a plan document explaining
why each wave exists. A register item can belong to a wave via that
wave's own ` + "`register_item`" + ` field, set when the item was added with
` + "`addwave`" + `'s --items-json; waves don't replace the register, and
there is no separate "move a ticket into a wave" command -- an
existing ticket is associated with a wave at the point the wave's
items are defined. See
https://ha1tch.github.io/gorepoman/docs/repoman-080-waves.html for the
full worked example, including exactly what that JSON looks like.

Without flags, this regenerates the tracking document's summary
section in place and exits -- the normal, everyday invocation:

    repoman waveprogress

Flags:
  --show              Render every visible wave as ASCII progress
                       bars instead of regenerating the file.
  --check             Exit non-zero if the summary is stale (CI use)
                       without writing anything.
  --html PATH         Render the same view as --show to an HTML
                       file at PATH instead of ASCII. Superseded by
                       --show --format html (writes to stdout, same
                       shape every other reporting command uses) --
                       kept working, not removed.
  --format FORMAT      For --show/--check: text (default), json, or
                       html. json is a stable, queryfy-validated data
                       contract; html is a self-contained document in
                       gorepoman's own visual style, written to stdout.
  --hide WAVE_ID       Persist a wave as hidden from --show/--html
                       (its work still counts toward the overall
                       total -- visibility is a display concern,
                       not a completion one).
  --unhide WAVE_ID     Reverse --hide for one wave id.
  --include-hidden    For one invocation of --show/--html, render
                       every wave regardless of persisted visibility,
                       without changing what's stored.

Requires wave_tracking to be set in .repoman.json (the path to
WAVE_TRACKING.md); with no waves configured, there is nothing to
regenerate, and this command says so plainly rather than failing
with an unrelated-looking error.
`

// Run implements `repoman waveprogress [flags]`.
func Run(argv []string) int {
	argv = webhelp.NormalizeBriefFirst(argv)
	format, argv := report.ExtractFormat(argv)
	for _, a := range argv {
		if a == "-h" || a == "--help" {
			fmt.Print(waveprogressHelp)
			fmt.Println(webhelp.SuppressionNote)
			webhelp.PrintIfAvailable(os.Stdout, "repoman-080-waves", argv)
			return 0
		}
	}

	e, err := newEnv()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	if flag, i := findFlag(argv, "--hide", "--unhide"); flag != "" {
		if i+1 >= len(argv) {
			fmt.Fprintf(os.Stderr, "%s requires a wave id argument\n", flag)
			return 1
		}
		return e.setVisibility(argv[i+1], flag == "--unhide")
	}

	check := containsArg(argv, "--check")
	show := containsArg(argv, "--show")
	includeHidden := containsArg(argv, "--include-hidden")
	var htmlOut string
	if i := indexOfArg(argv, "--html"); i != -1 {
		if i+1 >= len(argv) {
			fmt.Fprintln(os.Stderr, "--html requires a path argument")
			return 1
		}
		htmlOut = argv[i+1]
	}

	if fi, err := os.Stat(e.docPath); err != nil || fi.IsDir() {
		fmt.Fprintf(os.Stderr, "no wave-tracking document at %s -- nothing to do (set wave_tracking in .repoman.json if waves are wanted)\n", e.docPath)
		return 1
	}
	b, err := os.ReadFile(e.docPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	text := string(b)

	// FR-04: correct each wave rows own Status cell against real
	// register/RESOLVED state before parsing -- see
	// syncWaveRowsFromRegister own doc comment. A row with nothing to
	// correct is untouched; text is unchanged in that case and this is
	// a no-op all the way through to the final write-vs-check comparison
	// below.
	synced, warnings := e.syncWaveRowsFromRegister(text)
	for _, w := range warnings {
		fmt.Fprintln(os.Stderr, w)
	}
	text = synced

	allWaves := e.parseWaves(text)
	if len(allWaves) == 0 {
		fmt.Fprintln(os.Stderr, "no waves parsed -- aborting, not touching the file")
		return 1
	}

	// Overall always reflects every wave, regardless of what's
	// currently displayed -- visibility is a display concern (per
	// isVisible's own doc comment), and hiding a wave from view
	// doesn't mean its work stops counting toward the real total.
	var visibleWaves []wave
	if includeHidden {
		visibleWaves = allWaves
	} else {
		for _, w := range allWaves {
			if isVisible(e.cfg, w.ID) {
				visibleWaves = append(visibleWaves, w)
			}
		}
	}
	if len(visibleWaves) == 0 {
		fmt.Fprintln(os.Stderr, "every wave is currently hidden -- pass --include-hidden to render anyway, or --unhide <id> to bring one back")
		return 1
	}

	newTable := e.renderTable(visibleWaves, text)
	newOverall := renderOverall(allWaves)

	if htmlOut != "" {
		title := e.cfg.WaveHTMLTitle
		if title == "" {
			title = "wave progress"
		}
		html := e.renderHTML(visibleWaves, text, title)
		if err := os.WriteFile(htmlOut, []byte(html), 0644); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		hiddenN := len(allWaves) - len(visibleWaves)
		note := ""
		if hiddenN > 0 {
			note = fmt.Sprintf(", %d hidden", hiddenN)
		}
		fmt.Printf("wave_progress: wrote %s (%d waves%s)\n", htmlOut, len(visibleWaves), note)
		if !show && !check {
			return 0
		}
	}

	if show {
		switch format {
		case "text":
			fmt.Println(newTable)
			fmt.Println()
			fmt.Println(newOverall)
			return 0
		case "json":
			data := e.buildReport(visibleWaves, allWaves, text)
			if err := ValidateReport(data); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				return 1
			}
			return report.EmitOrErr(report.EmitJSON(os.Stdout, "waveprogress", "waveprogress-report", SchemaVersion, data))
		case "html":
			data := e.buildReport(visibleWaves, allWaves, text)
			body, err := renderReportHTML(data)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				return 1
			}
			return report.EmitOrErr(report.EmitHTML(os.Stdout, "wave progress", "waveprogress", "waveprogress-report", body))
		default:
			fmt.Fprintf(os.Stderr, "unknown format %q (want text, json, or html)\n", format)
			return 1
		}
	}

	tableRe := regexp.MustCompile(`(?s)(## 1\. Progress at a glance\n.*?` + "```" + `\n).*?(\n` + "```" + `\n)`)
	loc := tableRe.FindStringSubmatchIndex(text)
	if loc == nil {
		// B-05 fix: a fresh project's wave-tracking document may not
		// have this section at all yet -- self-heal by inserting a
		// minimal skeleton rather than refusing. Previously this left
		// addwave's own prior writes (the new wave's table, plan
		// paragraph, short name) sitting half-updated with the failure
		// happening only on this later, separate check, and required
		// knowing the exact skeleton shape to pre-seed by hand -- never
		// stated in -h text or the docs, only recoverable from the
		// selftest fixture.
		skeleton := "## 1. Progress at a glance\n\n```\nplaceholder\n```\n\nOverall by item count: 0 of 0 items \u2248 **0%**\n\n"
		insertAt := 0
		if titleLoc := regexp.MustCompile(`(?m)^# .*\n\n?`).FindStringIndex(text); titleLoc != nil {
			insertAt = titleLoc[1]
		}
		text = text[:insertAt] + skeleton + text[insertAt:]
		loc = tableRe.FindStringSubmatchIndex(text)
		if loc == nil {
			fmt.Fprintln(os.Stderr, "internal error: inserted skeleton still did not match -- please report this")
			return 1
		}
	}
	newText := text[:loc[0]] + text[loc[2]:loc[3]] + newTable + text[loc[4]:loc[5]] + text[loc[1]:]

	overallRe := regexp.MustCompile(`Overall by item count: [\d.]+ of \d+ items \x{2248} \*\*\d+%\*\*`)
	if !overallRe.MatchString(newText) {
		fmt.Fprintln(os.Stderr, "could not find the 'Overall by item count' line")
		return 1
	}
	newText = overallRe.ReplaceAllLiteralString(newText, newOverall)

	// B-04 fix: also regenerate each wave's own "**Wave N: k/n, status.**"
	// line, not just section 1's fenced block. addwave writes this line
	// once, when the wave is created, and nothing regenerated it after --
	// it could read "0/4, not started" indefinitely while the table
	// above it showed every item done and section 1 said 100%.
	for _, w := range allWaves {
		waveLineRe := regexp.MustCompile(`\*\*Wave ` + regexp.QuoteMeta(w.ID) + `: \d+/\d+, [^*]+\.\*\*`)
		if waveLineRe.MatchString(newText) {
			fresh := fmt.Sprintf("**Wave %s: %s/%d, %s.**", w.ID, formatG(w.DoneEquiv), w.Total, waveStatusWord(w, e.cfg.WaveCompleteWord))
			newText = waveLineRe.ReplaceAllLiteralString(newText, fresh)
		}
	}

	if newText == text {
		if check && format != "text" {
			return emitCheckResult(format, CheckResult{Stale: false})
		}
		fmt.Println("wave_progress: already up to date")
		return 0
	}
	if check {
		if format != "text" {
			return emitCheckResult(format, CheckResult{Stale: true})
		}
		fmt.Println("wave_progress: wave-tracking document is stale -- run without --check to regenerate")
		return 1
	}

	if err := os.WriteFile(e.docPath, []byte(newText), 0644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	hiddenN := len(allWaves) - len(visibleWaves)
	note := ""
	if hiddenN > 0 {
		note = fmt.Sprintf(", %d hidden", hiddenN)
	}
	fmt.Printf("wave_progress: regenerated (%d waves%s)\n", len(visibleWaves), note)
	return 0
}

// --- small local helpers -------------------------------------------------

func containsArg(argv []string, flag string) bool {
	for _, a := range argv {
		if a == flag {
			return true
		}
	}
	return false
}

func indexOfArg(argv []string, flag string) int {
	for i, a := range argv {
		if a == flag {
			return i
		}
	}
	return -1
}

func findFlag(argv []string, flags ...string) (string, int) {
	for i, a := range argv {
		for _, f := range flags {
			if a == f {
				return f, i
			}
		}
	}
	return "", -1
}

func padRight(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}

func padLeft(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return strings.Repeat(" ", width-len(s)) + s
}

// pyRound rounds half to even, matching Python 3's built-in round()
// -- used wherever the original computes a display percentage, so
// exact-.5 boundaries render identically to the Python tool.
// waveStatusWord mirrors the phrase addwave hardcodes as "not started"
// for a brand-new wave, extended to the two states a wave actually
// passes through afterward -- used by the B-04 fix above to keep a
// wave's own summary line honest as it progresses. completeWord is
// FR-03's override for the "done" word (config key
// wave_complete_word); pass "" to get the original hardcoded "done",
// exactly as before this parameter existed.
func waveStatusWord(w wave, completeWord string) string {
	if completeWord == "" {
		completeWord = "done"
	}
	switch {
	case w.DoneEquiv <= 0:
		return "not started"
	case w.DoneEquiv >= float64(w.Total) && w.Total > 0:
		return completeWord
	default:
		return "in progress"
	}
}

func pyRound(x float64) int {
	floor := math.Floor(x)
	diff := x - floor
	if diff < 0.5 {
		return int(floor)
	}
	if diff > 0.5 {
		return int(floor) + 1
	}
	if int64(floor)%2 == 0 {
		return int(floor)
	}
	return int(floor) + 1
}

// formatG mimics Python's f"{x:g}" -- shortest representation,
// trailing zeros and an unnecessary decimal point trimmed.
func formatG(x float64) string {
	return strconv.FormatFloat(x, 'g', -1, 64)
}

// formatG4 mimics Python's f"{x:.4g}" -- general format, 4
// significant digits.
func formatG4(x float64) string {
	return strconv.FormatFloat(x, 'g', 4, 64)
}
