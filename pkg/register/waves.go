package register

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ha1tch/gorepoman/pkg/config"
	"github.com/ha1tch/gorepoman/pkg/waveprogress"
)

// propagateCloseToWaves marks tid done in the wave-tracking document
// if any wave row references it via the register_item column, then
// regenerates wave progress so the per-wave summary lines and the
// "Progress at a glance" block stay in step -- not just the checkbox.
// Returns true if a row was actually updated.
//
// A wave row is `| N | task | status | T-ID |` -- four columns, the
// register id last. Only an exact whole-cell match on the last column
// counts, so T-1 never matches T-10.
//
// Absent or unconfigured wave tracking is not an error: most
// repositories do not use waves at all, and closing an item there
// must behave exactly as it always has.
func propagateCloseToWaves(root string, cfg *config.Config, tid string, dryRun bool) bool {
	if cfg.WaveTracking == "" {
		return false
	}
	path := filepath.Join(root, cfg.WaveTracking)
	b, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	text := string(b)

	// Match the row's status cell only when the id cell is exactly tid.
	rowRe := regexp.MustCompile(`(?m)^(\|[^|\n]*\|[^|\n]*\|)([^|\n]*)(\|\s*` + regexp.QuoteMeta(tid) + `\s*\|)\s*$`)
	if !rowRe.MatchString(text) {
		return false
	}
	newText := rowRe.ReplaceAllString(text, "${1} ✓ ${3}")
	if newText == text {
		return false
	}
	if dryRun {
		return true
	}
	if err := os.WriteFile(path, []byte(newText), 0644); err != nil {
		return false
	}
	// Regenerate so the summary lines and progress block reflect the
	// change in the same operation -- addwave already sets this exact
	// precedent by calling waveprogress.Run directly.
	waveprogress.Run(nil)
	return true
}

// filedByRe is the bare cross-project reference form used inside a
// labeled field: <project>:<prefix>-<n>. The decided convention is
// that the issue/ prefix marks a standalone reference; inside a
// labeled field like Filed-by the label already says what kind of
// thing it is, so the stored form is bare. NormalizeFiledBy tolerates
// a leading issue/ on input and strips it, rather than rejecting a
// form a person could reasonably have written.
var filedByRe = regexp.MustCompile(`^[A-Za-z0-9_.-]+:[A-Za-z]+-\d+$`)

// NormalizeFiledBy validates a Filed-by value and returns its stored
// (bare) form, or an error naming exactly what shape was expected.
func NormalizeFiledBy(v string) (string, error) {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "issue/")
	if !filedByRe.MatchString(v) {
		return "", fmt.Errorf("%q is not a cross-project reference -- expected <project>:<prefix>-<n>, e.g. xoluman:T-1234", v)
	}
	return v, nil
}

// SetItemWave writes (or overwrites) the Wave field on tid own
// field line in TRACKING.md -- T-06, the missing half of what B-04
// and this file already do in the other direction. addwave already
// knows which wave number an item belongs to the moment it links one
// via register_item in --items-json; this is that number written
// back onto the item itself, so it is real, queryable data
// (register list --format json shows it) instead of only implicit in
// WAVE_TRACKING.md own document layout.
//
// Deliberately additive: Wave is a field-line addition like Progress,
// not a new table column -- adding a mandatory column would have
// been a breaking format change to every register everywhere. An
// item with no Wave line is untouched by everything else in the
// register; this only ever adds or updates that one optional segment.
func SetItemWave(root string, cfg *config.Config, tid, wave string) error {
	path := filepath.Join(root, cfg.Tracking)
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	text := string(b)

	headRe := regexp.MustCompile(`(?m)^### (` + regexp.QuoteMeta(tid) + `)\. .*$`)
	loc := headRe.FindStringIndex(text)
	if loc == nil {
		return fmt.Errorf("no such item: %s", tid)
	}

	tail := text[loc[1]:]
	// Every optional segment must be captured and re-emitted, not just
	// the ones this function happens to care about -- an earlier version
	// only preserved Blocks/after and would have silently dropped a
	// Filed-by segment when rewriting Wave. Caught while adding
	// Filed-by (T-14), before it could happen in a real register.
	fieldRe := regexp.MustCompile(`(?m)^Theme: (\S+) · Priority: (\*{0,2}P\d\*{0,2}) · Status: (\S+)(?: · Wave: \S+)?(?: · Filed-by: (\S+))?(?: · Blocks/after: (.*))?$`)
	fm := fieldRe.FindStringSubmatchIndex(tail)
	if fm == nil {
		return fmt.Errorf("%s: could not find its own field line", tid)
	}

	theme := tail[fm[2]:fm[3]]
	priority := tail[fm[4]:fm[5]]
	status := tail[fm[6]:fm[7]]
	filedBy := ""
	if len(fm) > 8 && fm[8] != -1 {
		filedBy = tail[fm[8]:fm[9]]
	}
	blocks := ""
	if len(fm) > 10 && fm[10] != -1 {
		blocks = tail[fm[10]:fm[11]]
	}
	fSec := ""
	if filedBy != "" {
		fSec = " · Filed-by: " + filedBy
	}
	bSec := ""
	if blocks != "" {
		bSec = " · Blocks/after: " + blocks
	}
	newLine := fmt.Sprintf("Theme: %s · Priority: %s · Status: %s · Wave: %s%s%s", theme, priority, status, wave, fSec, bSec)

	newTail := tail[:fm[0]] + newLine + tail[fm[1]:]
	newText := text[:loc[1]] + newTail
	return os.WriteFile(path, []byte(newText), 0644)
}
