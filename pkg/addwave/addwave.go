// Package addwave adds a new wave to a project's staged-work
// programme deterministically -- translated from repoman/add_wave.py.
//
// The wave-tracking document's own per-item table (that pkg/
// waveprogress reads) and the wave-plan document's short pointer
// paragraph are updated, with the wave number and item numbers
// computed from the actual current state of both documents -- not
// supplied by the caller and not guessed.
//
// Ported from xolu's own scripts/add_wave.py, which carries the
// original design rationale in full, reproduced below with only the
// xolu-specific bindings changed to config-driven equivalents (see
// pkg/waveprogress's own header for the parallel list). Two
// differences worth calling out specifically, not just
// parameterization:
//
//   - The tracking-document insertion point no longer depends on a
//     hardcoded section marker (xolu's own original used a literal
//     "## 3." anchor -- fragile, and specific to that document's own
//     current numbering). This finds the LAST existing "### Wave N"
//     heading's own section end and inserts immediately after it, the
//     same robust pattern insertPlanParagraph below already uses for
//     the plan document -- one technique, applied consistently,
//     rather than two different techniques for what is the same
//     problem in two documents.
//   - Short names are no longer maintained by injecting a new
//     dict-literal line into a Python source file at runtime. They
//     are config data (wave_short_names in .repoman.json), written
//     via config.SaveKey -- mechanical JSON, not source-code surgery.
//
// A further adaptation from the Python original: rather than
// shelling out to a second interpreter invocation of
// wave_progress.py, this calls waveprogress.Run directly, in
// process -- same binary, same guarantee of using exactly the
// regeneration logic this run of repoman itself carries.
//
// Why this exists (xolu's own original rationale, unchanged): adding
// a wave by hand once nearly collided with an EXISTING soft
// reservation -- the tracking document's own prose already reserved
// a wave number for unrelated future work in a sentence, not a
// formal heading, which a naive "highest heading + 1" scan would have
// missed entirely. This tool scans for BOTH: formal headings and any
// other "wave N" mention anywhere in either document, so a soft
// reservation like that is caught and refused rather than silently
// overwritten.
//
// Item numbers are global and sequential across the whole programme,
// never reused once assigned -- this tool continues from the highest
// "| NN |" row it finds in any existing wave table.
//
// What this tool does NOT do: write the load-bearing prose. The plan
// document's own paragraph explaining why a wave exists, what it
// depends on, and how it's sequenced is exactly the kind of judgement
// call a human (or an LLM, thinking about it deliberately) writes
// each time -- this tool places it correctly and consistently, it
// doesn't compose it. Pass that text in with --plan-note.
//
// Usage:
//
//	repoman addwave \
//	    --name "some staged workstream" \
//	    --ideal-days 2 \
//	    --items-json '[{"summary": "...", "register_item": "T-1"}]' \
//	    --plan-note "Why this wave exists, what it depends on..." \
//	    [--wave-number 12]   // override the computed number; still checked for collision
//	    [--dry-run]          // print what would change, touch nothing
package addwave

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ha1tch/repoman/pkg/config"
	"github.com/ha1tch/repoman/pkg/waveprogress"
)

var (
	waveHeadingRe = regexp.MustCompile(`(?m)^### Wave (\S+) —`)
	waveMentionRe = regexp.MustCompile(`(?i)\bwave (\d+)\b`)
	itemRowRe     = regexp.MustCompile(`(?m)^\|\s*(\d+)\s*\|`)
	leadingDigits = regexp.MustCompile(`^\d+`)
	altHeadingRe  = regexp.MustCompile(`(?m)^\*\*Wave \S+ —`)
	sepOrHeadRe   = regexp.MustCompile(`(?m)^(### |## |---\s*$)`)
)

type item struct {
	Summary      string
	RegisterItem string
}

// existingWaveNumbers returns every integer wave number already in
// use OR reserved -- formal headings AND prose mentions like
// "plausibly wave 11". Alphabetic sub-waves (e.g. "9b") are excluded
// from this integer set deliberately; they share a numeric wave's own
// number by design, not a new one.
func existingWaveNumbers(text string) map[int]bool {
	nums := map[int]bool{}
	for _, m := range waveHeadingRe.FindAllStringSubmatch(text, -1) {
		if d := leadingDigits.FindString(m[1]); d != "" {
			if n, err := strconv.Atoi(d); err == nil {
				nums[n] = true
			}
		}
	}
	for _, m := range waveMentionRe.FindAllStringSubmatch(text, -1) {
		if n, err := strconv.Atoi(m[1]); err == nil {
			nums[n] = true
		}
	}
	return nums
}

func nextWaveNumber(trackingText, planText string, requested *int) (int, error) {
	taken := existingWaveNumbers(trackingText)
	for n := range existingWaveNumbers(planText) {
		taken[n] = true
	}

	if requested != nil {
		if taken[*requested] {
			sorted := make([]int, 0, len(taken))
			for n := range taken {
				sorted = append(sorted, n)
			}
			sort.Ints(sorted)
			return 0, fmt.Errorf("--wave-number %d collides with an existing heading or prose reservation. Taken/reserved: %v", *requested, sorted)
		}
		return *requested, nil
	}

	maxTaken := 0
	for n := range taken {
		if n > maxTaken {
			maxTaken = n
		}
	}
	candidate := maxTaken + 1
	for taken[candidate] {
		candidate++
	}
	return candidate, nil
}

func nextItemNumber(text string) int {
	matches := itemRowRe.FindAllStringSubmatch(text, -1)
	if len(matches) == 0 {
		return 1
	}
	max := 0
	for _, m := range matches {
		if n, err := strconv.Atoi(m[1]); err == nil && n > max {
			max = n
		}
	}
	return max + 1
}

// pyFloatStr mimics Python's default float-to-string conversion
// closely enough for the ideal-days values this tool handles: always
// at least one digit after the decimal point (Python's str(2.0) is
// "2.0", not "2").
func pyFloatStr(f float64) string {
	s := strconv.FormatFloat(f, 'f', -1, 64)
	if !strings.Contains(s, ".") {
		s += ".0"
	}
	return s
}

func buildTrackingSection(waveNum int, name string, idealDays float64, items []item, startItem int) string {
	today := time.Now().Format("2006-01-02")
	plural := "s"
	if len(items) == 1 {
		plural = ""
	}
	lines := []string{
		fmt.Sprintf("### Wave %d — %s (%d item%s, ideal %sd, added %s)",
			waveNum, name, len(items), plural, pyFloatStr(idealDays), today),
		"",
		"| # | Summary | Status | Register item |",
		"|---|---|---|---|",
	}
	for i, it := range items {
		itemNum := startItem + i
		reg := it.RegisterItem
		if reg == "" {
			reg = "not yet filed"
		}
		lines = append(lines, fmt.Sprintf("| %d | %s | ☐ | %s |", itemNum, it.Summary, reg))
	}
	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("**Wave %d: 0/%d, not started.**", waveNum, len(items)))
	lines = append(lines, "")
	return strings.Join(lines, "\n")
}

func buildPlanParagraph(waveNum int, name string, idealDays float64, planNote string) string {
	today := time.Now().Format("2006-01-02")
	return fmt.Sprintf("**Wave %d — %s (≈ %sd, added %s).** %s\n", waveNum, name, pyFloatStr(idealDays), today, planNote)
}

// insertTrackingSection inserts after the LAST existing "### Wave N"
// heading's own section (the next "### ", "## ", or "---" separator,
// whichever comes first) -- or at the end of the document if this is
// the very first wave. No hardcoded section-number anchor: this works
// regardless of how the rest of the document is structured or
// numbered.
//
// A trailing "---" separator is added after the new section ONLY
// when none exists yet at the insertion point. When a PREVIOUS
// insertion's own trailing "---" is what's found instead, it is
// reused, not duplicated -- the new section is inserted before it,
// with a blank line restored ahead of it (section itself always ends
// in exactly one newline; one more is needed for correct blank-line
// spacing). Getting this reuse-vs-add distinction wrong was a real
// bug in the Python original, not a hypothetical one, caught only by
// testing four sequential insertions against a realistic fixture --
// preserved here rather than re-derived, along with the third branch
// below (nothing at all follows the last wave's own content, so
// unlike the other two branches there is no PRE-EXISTING blank line
// to inherit spacing from, and one must be added explicitly).
func insertTrackingSection(trackingPath, section string) error {
	b, err := os.ReadFile(trackingPath)
	if err != nil {
		return err
	}
	text := string(b)

	headings := waveHeadingRe.FindAllStringIndex(text, -1)
	var newText string
	if len(headings) > 0 {
		last := headings[len(headings)-1]
		tail := text[last[1]:]
		nmLoc := sepOrHeadRe.FindStringSubmatchIndex(tail)
		switch {
		case nmLoc != nil:
			groupText := strings.TrimSpace(tail[nmLoc[2]:nmLoc[3]])
			insertionPoint := last[1] + nmLoc[0]
			if groupText == "---" {
				newText = text[:insertionPoint] + section + "\n" + text[insertionPoint:]
			} else {
				newText = text[:insertionPoint] + section + "\n---\n\n" + text[insertionPoint:]
			}
		default:
			insertionPoint := last[1] + len(tail) // == len(text)
			newText = strings.TrimRight(text[:insertionPoint], " \t\n\r") + "\n\n" + section + "\n---\n\n" + text[insertionPoint:]
		}
	} else {
		// First wave in this document: append at the end.
		newText = strings.TrimRight(text, " \t\n\r") + "\n\n" + section
	}
	return os.WriteFile(trackingPath, []byte(newText), 0644)
}

func insertPlanParagraph(planPath, paragraph string) error {
	b, err := os.ReadFile(planPath)
	if err != nil {
		return err
	}
	text := string(b)

	headings := waveHeadingRe.FindAllStringIndex(text, -1)
	if len(headings) == 0 {
		headings = altHeadingRe.FindAllStringIndex(text, -1)
	}

	insertionPoint := len(text)
	if len(headings) > 0 {
		last := headings[len(headings)-1]
		if idx := strings.Index(text[last[1]:], "\n\n"); idx != -1 {
			insertionPoint = last[1] + idx + 2
		}
	}
	newText := text[:insertionPoint] + "\n" + paragraph + "\n" + text[insertionPoint:]
	return os.WriteFile(planPath, []byte(newText), 0644)
}

// insertShortName adds this wave's entry to wave_short_names in
// .repoman.json. Without this, pkg/waveprogress renders "(unnamed
// wave N)" -- a real gap found by running xolu's own original tool
// end to end the first time, not a hypothetical: short display names
// are hand-curated by design (a wave's short form isn't mechanically
// derivable from its full name), but leaving that as a separate
// manual follow-up undermines the determinism this tool exists for.
// Guarded: refuses if the key already exists rather than silently
// overwriting.
func insertShortName(root string, cfg config.Config, waveNum int, name string) error {
	key := strconv.Itoa(waveNum)
	if _, ok := cfg.WaveShortNames[key]; ok {
		fmt.Fprintf(os.Stderr, "wave_short_names already has an entry for %q -- not touching it\n", key)
		return nil
	}
	names := map[string]string{}
	for k, v := range cfg.WaveShortNames {
		names[k] = v
	}
	names[key] = name
	return config.SaveKey(root, "wave_short_names", names)
}

// insertDefaultVisibility adds an explicit visible=true entry to
// wave_visibility in .repoman.json for this wave. pkg/waveprogress's
// own isVisible already treats an ABSENT entry as visible, so this
// call is not strictly required for correctness -- it exists so
// wave_visibility reads as a complete, honest record of every wave's
// own status at a glance (matching wave_short_names's own precedent:
// explicitly written for every wave, not left to a fallback a reader
// would need to know about). Guarded the same way: refuses if the key
// already exists, since a wave that was deliberately hidden before
// this call somehow ran should never be silently un-hidden by it.
func insertDefaultVisibility(root string, cfg config.Config, waveNum int) error {
	key := strconv.Itoa(waveNum)
	if _, ok := cfg.WaveVisibility[key]; ok {
		return nil
	}
	vis := map[string]bool{}
	for k, v := range cfg.WaveVisibility {
		vis[k] = v
	}
	vis[key] = true
	return config.SaveKey(root, "wave_visibility", vis)
}

func requireDocs(trackingPath, planPath string) error {
	var missing []string
	if fi, err := os.Stat(trackingPath); err != nil || fi.IsDir() {
		missing = append(missing, trackingPath)
	}
	if fi, err := os.Stat(planPath); err != nil || fi.IsDir() {
		missing = append(missing, planPath)
	}
	if len(missing) > 0 {
		return fmt.Errorf("wave-tracking and/or wave-plan document not found: %s -- set wave_tracking/wave_plan in .repoman.json and create them (see repoman/README.md for the expected shape) before adding a wave",
			strings.Join(missing, ", "))
	}
	return nil
}

// Run implements `repoman addwave [flags]`.
func Run(argv []string) int {
	for _, a := range argv {
		if a == "-h" || a == "--help" {
			fmt.Println("usage: repoman addwave [-h] --name NAME --ideal-days IDEAL_DAYS --items-json")
			fmt.Println("                   ITEMS_JSON --plan-note PLAN_NOTE")
			fmt.Println("                   [--wave-number WAVE_NUMBER] [--dry-run]")
			fmt.Println("")
			fmt.Println("repoman addwave -- adds a new wave to a project's staged-work")
			fmt.Println("programme deterministically: the wave-tracking document's own")
			fmt.Println("per-item table (that waveprogress reads) and the wave-plan")
			fmt.Println("document's short pointer paragraph, with the wave number and item")
			fmt.Println("numbers computed from the actual current state of both documents --")
			fmt.Println("not supplied by the caller and not guessed.")
			fmt.Println("")
			fmt.Println("Translated from repoman/add_wave.py, which carries the original")
			fmt.Println("design rationale in full -- reproduced here since it still governs")
			fmt.Println("this command's behaviour unchanged:")
			fmt.Println("")
			fmt.Println("  - The tracking-document insertion point does not depend on a")
			fmt.Println("    hardcoded section marker. It finds the LAST existing \"### Wave N\"")
			fmt.Println("    heading's own section end and inserts immediately after it, the")
			fmt.Println("    same pattern used for the plan document.")
			fmt.Println("  - Wave short names are config data (wave_short_names in")
			fmt.Println("    .repoman.json), written via ordinary JSON I/O -- not source-code")
			fmt.Println("    surgery.")
			fmt.Println("")
			fmt.Println("Why this exists: adding a wave by hand once nearly collided with an")
			fmt.Println("EXISTING soft reservation -- the tracking document's own prose")
			fmt.Println("already reserved a wave number for unrelated future work in a")
			fmt.Println("sentence, not a formal heading, which a naive \"highest heading + 1\"")
			fmt.Println("scan would have missed entirely. This command scans for BOTH: formal")
			fmt.Println("headings and any other \"wave N\" mention anywhere in either document,")
			fmt.Println("so a soft reservation like that is caught and refused rather than")
			fmt.Println("silently overwritten.")
			fmt.Println("")
			fmt.Println("Item numbers are global and sequential across the whole programme,")
			fmt.Println("never reused once assigned -- this command continues from the")
			fmt.Println("highest \"| NN |\" row it finds in any existing wave table.")
			fmt.Println("")
			fmt.Println("What this command does NOT do: write the load-bearing prose. The")
			fmt.Println("plan document's own paragraph explaining why a wave exists, what it")
			fmt.Println("depends on, and how it's sequenced is a judgement call written each")
			fmt.Println("time, not composed here. Pass that text in with --plan-note.")
			fmt.Println("")
			fmt.Println("Usage:")
			fmt.Println("    repoman addwave \\")
			fmt.Println("        --name \"some staged workstream\" \\")
			fmt.Println("        --ideal-days 2 \\")
			fmt.Println("        --items-json '[{\"summary\": \"...\", \"register_item\": \"T-1\"}]' \\")
			fmt.Println("        --plan-note \"Why this wave exists, what it depends on...\" \\")
			fmt.Println("        [--wave-number 12]   # override the computed number; still checked for collision")
			fmt.Println("        [--dry-run]          # print what would change, touch nothing")
			fmt.Println("")
			fmt.Println("options:")
			fmt.Println("  -h, --help            show this help message and exit")
			fmt.Println("  --name NAME")
			fmt.Println("  --ideal-days IDEAL_DAYS")
			fmt.Println("  --items-json ITEMS_JSON")
			fmt.Println("                        JSON list: [{\"summary\": \"...\", \"register_item\":")
			fmt.Println("                        \"T-1\"}]")
			fmt.Println("  --plan-note PLAN_NOTE")
			fmt.Println("                        Rationale paragraph for the wave-plan document --")
			fmt.Println("                        written deliberately, not generated.")
			fmt.Println("  --wave-number WAVE_NUMBER")
			fmt.Println("                        Override the computed wave number. Still checked for")
			fmt.Println("                        collision.")
			fmt.Println("  --dry-run")
			return 0
		}
	}
	var name, itemsJSON, planNote, idealDaysStr string
	var waveNumber *int
	dryRun := false

	for i := 0; i < len(argv); i++ {
		switch argv[i] {
		case "--name":
			i++
			if i < len(argv) {
				name = argv[i]
			}
		case "--ideal-days":
			i++
			if i < len(argv) {
				idealDaysStr = argv[i]
			}
		case "--items-json":
			i++
			if i < len(argv) {
				itemsJSON = argv[i]
			}
		case "--plan-note":
			i++
			if i < len(argv) {
				planNote = argv[i]
			}
		case "--wave-number":
			i++
			if i < len(argv) {
				if n, err := strconv.Atoi(argv[i]); err == nil {
					waveNumber = &n
				}
			}
		case "--dry-run":
			dryRun = true
		}
	}

	if name == "" || idealDaysStr == "" || itemsJSON == "" || planNote == "" {
		fmt.Fprintln(os.Stderr, "usage: repoman addwave --name NAME --ideal-days DAYS --items-json JSON --plan-note NOTE [--wave-number N] [--dry-run]")
		return 1
	}
	idealDays, err := strconv.ParseFloat(idealDaysStr, 64)
	if err != nil {
		fmt.Fprintf(os.Stderr, "--ideal-days is not a number: %v\n", err)
		return 1
	}

	root, cfg, err := config.Load("")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	trackingPath := filepath.Join(root, cfg.WaveTracking)
	planPath := filepath.Join(root, cfg.WavePlan)

	if err := requireDocs(trackingPath, planPath); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	var rawItems []map[string]interface{}
	if err := json.Unmarshal([]byte(itemsJSON), &rawItems); err != nil {
		fmt.Fprintf(os.Stderr, "--items-json is not valid JSON: %v\n", err)
		return 1
	}
	if len(rawItems) == 0 {
		fmt.Fprintln(os.Stderr, "--items-json must be a non-empty JSON list")
		return 1
	}
	items := make([]item, 0, len(rawItems))
	for _, raw := range rawItems {
		summary, ok := raw["summary"].(string)
		if !ok {
			fmt.Fprintf(os.Stderr, "item missing 'summary': %v\n", raw)
			return 1
		}
		reg, _ := raw["register_item"].(string)
		items = append(items, item{Summary: summary, RegisterItem: reg})
	}

	trackingText, err := os.ReadFile(trackingPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	planText, err := os.ReadFile(planPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	waveNum, err := nextWaveNumber(string(trackingText), string(planText), waveNumber)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	startItem := nextItemNumber(string(trackingText))
	endItem := startItem + len(items) - 1

	trackingSection := buildTrackingSection(waveNum, name, idealDays, items, startItem)
	planParagraph := buildPlanParagraph(waveNum, name, idealDays, planNote)

	fmt.Printf("Wave number: %d  (computed; checked against headings + prose reservations)\n", waveNum)
	itemRange := strconv.Itoa(startItem)
	if endItem != startItem {
		itemRange += fmt.Sprintf("-%d", endItem)
	}
	fmt.Printf("Item numbers: %s\n\n", itemRange)
	fmt.Printf("--- %s section ---\n", filepath.Base(trackingPath))
	fmt.Println(trackingSection)
	fmt.Printf("--- %s paragraph ---\n", filepath.Base(planPath))
	fmt.Println(planParagraph)

	if dryRun {
		fmt.Println("(--dry-run: nothing written)")
		return 0
	}

	if err := insertTrackingSection(trackingPath, trackingSection); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if err := insertPlanParagraph(planPath, planParagraph); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if err := insertShortName(root, cfg, waveNum, name); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if err := insertDefaultVisibility(root, cfg, waveNum); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	rc := waveprogress.Run(nil)
	if rc != 0 {
		fmt.Fprintf(os.Stderr, "waveprogress regeneration failed -- check %s's own fenced block by hand\n", filepath.Base(trackingPath))
		return rc
	}

	fmt.Printf("\naddwave: Wave %d added, items %d-%d, bars regenerated.\n", waveNum, startItem, endItem)
	return 0
}
