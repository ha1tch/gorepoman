package strreplace

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/ha1tch/gorepoman/pkg/ed"
	"github.com/ha1tch/gorepoman/pkg/roles"
)

// ── single-op staging (in-memory; no I/O beyond initial read) ───────

// fileState tracks staged in-memory content per file plus the order
// files were first touched, so the syntax-check and write passes
// below iterate deterministically (matching a Python dict's own
// insertion-order iteration, which the original relies on implicitly).
type fileState struct {
	text  map[string]string
	order []string
}

func newFileState() *fileState {
	return &fileState{text: map[string]string{}}
}

func (fs *fileState) ensureLoaded(file string) (string, error) {
	if t, ok := fs.text[file]; ok {
		return t, nil
	}
	t, err := readFileUTF8(file)
	if err != nil {
		return "", err
	}
	fs.text[file] = t
	fs.order = append(fs.order, file)
	return t, nil
}

// stageSubOp mutates fs.text[file] in place on success. Returns
// *Refused on any refusal class, and a per-op result map (pre-write)
// on success.
func stageSubOp(op opJSON, fs *fileState) (map[string]interface{}, error) {
	file := op.File
	text, err := fs.ensureLoaded(file)
	if err != nil {
		return nil, err
	}

	if op.SearchB64 == nil || op.ReplaceB64 == nil {
		return nil, refused("invalid-utf8-payload", "%s: op missing search_b64/replace_b64", file)
	}
	search, err := b64DecodeUTF8(*op.SearchB64, file+": search_b64")
	if err != nil {
		return nil, err
	}
	replace, err := b64DecodeUTF8(*op.ReplaceB64, file+": replace_b64")
	if err != nil {
		return nil, err
	}

	if op.Expect == nil {
		return nil, refused("count-mismatch", "%s: 'expect' is required, no default", file)
	}
	if len(op.Roles) == 0 {
		return nil, refused("role-mismatch", "%s: 'roles' is required, no default -- "+
			"name every role you expect this match to occupy", file)
	}
	expect := *op.Expect
	assertedRoles := map[string]bool{}
	for _, r := range op.Roles {
		assertedRoles[r] = true
	}

	offsets := findAll(text, search)
	if len(offsets) == 0 {
		if nerr := checkNormalization(text, search); nerr != nil {
			return nil, nerr
		}
		return nil, refused("count-mismatch", "%s: 0 occurrence(s) found, expected %d", file, expect)
	}

	if op.Occurrences != nil {
		selected := make([]int, 0, len(op.Occurrences))
		for _, idx := range op.Occurrences {
			if idx < 1 || idx > len(offsets) {
				return nil, refused("count-mismatch",
					"%s: occurrences index %d out of range (1..%d found)", file, idx, len(offsets))
			}
			selected = append(selected, offsets[idx-1])
		}
		offsets = selected
	}

	if len(offsets) != expect {
		return nil, refused("count-mismatch",
			"%s: found %d occurrence(s) (after any 'occurrences' filter), expected %d. Nothing written.",
			file, len(offsets), expect)
	}

	census := map[string]int{}
	for _, off := range offsets {
		role := roles.Classify(file, text, off)
		census[role]++
	}
	for r := range census {
		if !assertedRoles[r] {
			sortedAsserted := make([]string, 0, len(assertedRoles))
			for a := range assertedRoles {
				sortedAsserted = append(sortedAsserted, a)
			}
			sort.Strings(sortedAsserted)
			return nil, refused("role-mismatch",
				"%s: real role census %s is not a subset of asserted roles %v. Split into one op "+
					"per role, or correct the 'roles' list if the census is actually what you intended.",
				file, censusString(census), sortedAsserted)
		}
	}

	// Apply in descending offset order so earlier offsets stay valid;
	// delimiter-integrity checked per-occurrence against the text as
	// staged so far (matches pkg/ed's own `sub` command's staging
	// discipline).
	sortedOffsets := append([]int{}, offsets...)
	sort.Sort(sort.Reverse(sort.IntSlice(sortedOffsets)))
	for _, off := range sortedOffsets {
		if derr := checkDelimiterIntegrity(file, text, off, off+len(search), replace); derr != nil {
			return nil, derr
		}
		text = text[:off] + replace + text[off+len(search):]
	}

	fs.text[file] = text
	return map[string]interface{}{
		"op": "sub", "file": file, "matched": len(offsets),
		"roles": census, "applied": true,
	}, nil
}

func censusString(census map[string]int) string {
	keys := make([]string, 0, len(census))
	for k := range census {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%q: %d", k, census[k]))
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

func stageMarkerOp(op opJSON, fs *fileState, markerLog *[]string) (map[string]interface{}, error) {
	file := op.File
	text, err := fs.ensureLoaded(file)
	if err != nil {
		return nil, err
	}

	switch op.kind() {
	case "gen_markers":
		count := 1
		if op.Count != nil {
			count = *op.Count
		}
		made := make([]string, 0, count)
		for i := 0; i < count; i++ {
			m := genMarker(text + strings.Join(made, ""))
			made = append(made, m)
		}
		*markerLog = append(*markerLog, made...)
		return map[string]interface{}{"op": "gen_markers", "file": file, "markers": made, "applied": false}, nil

	case "sweep_markers":
		n := len(markerRe.FindAllString(text, -1))
		newText := markerRe.ReplaceAllString(text, "")
		fs.text[file] = newText
		return map[string]interface{}{"op": "sweep_markers", "file": file, "removed": n, "applied": n > 0}, nil

	default:
		return nil, refused("invalid-utf8-payload", "unknown op %q", op.kind())
	}
}

// ── top-level payload application ───────────────────────────────────

func refusedResult(err error, opsCompleted int) map[string]interface{} {
	if r, ok := err.(*Refused); ok {
		return map[string]interface{}{
			"ok":                           false,
			"error":                        map[string]interface{}{"cls": r.Cls, "message": r.Message},
			"ops_completed_before_refusal": opsCompleted,
		}
	}
	return map[string]interface{}{
		"ok":                           false,
		"error":                        map[string]interface{}{"cls": "invalid-utf8-payload", "message": err.Error()},
		"ops_completed_before_refusal": opsCompleted,
	}
}

// applyPayload is dict-in/dict-out logic, no filesystem writes if
// every relevant op is dry_run (marker ops other than sweep never
// write on their own). Under atomic (default true): nothing is
// written unless every op stages cleanly.
func applyPayload(payload payloadJSON) map[string]interface{} {
	if payload.V == nil || *payload.V != 1 {
		var v interface{}
		if payload.V != nil {
			v = *payload.V
		}
		return map[string]interface{}{"ok": false, "error": map[string]interface{}{
			"cls": "invalid-utf8-payload", "message": fmt.Sprintf("unsupported payload version %v, expected 1", v),
		}}
	}
	atomic := true
	if payload.Atomic != nil {
		atomic = *payload.Atomic
	}

	fs := newFileState()
	var markerLog []string
	var opResults []map[string]interface{}
	dryRunFiles := map[string]bool{}

	for _, op := range payload.Ops {
		kind := op.kind()
		effDry := false
		if payload.Defaults.DryRun != nil {
			effDry = *payload.Defaults.DryRun
		}
		if op.DryRun != nil {
			effDry = *op.DryRun
		}

		switch kind {
		case "sub":
			res, err := stageSubOp(op, fs)
			if err != nil {
				return refusedResult(err, len(opResults))
			}
			if effDry {
				dryRunFiles[op.File] = true
			}
			opResults = append(opResults, res)
		case "gen_markers", "sweep_markers":
			res, err := stageMarkerOp(op, fs, &markerLog)
			if err != nil {
				return refusedResult(err, len(opResults))
			}
			opResults = append(opResults, res)
		default:
			return refusedResult(refused("invalid-utf8-payload", "unknown op kind %q", kind), len(opResults))
		}
	}

	// Syntax-check every touched, non-dry-run file against its final
	// staged content -- once per file, not once per op.
	syntaxResults := map[string]interface{}{}
	for _, file := range fs.order {
		if dryRunFiles[file] {
			continue
		}
		if _, ok := fs.text[file]; !ok {
			continue
		}
		text := fs.text[file]
		effCheck := true
		if payload.Defaults.SyntaxCheck != nil {
			effCheck = *payload.Defaults.SyntaxCheck
		}
		for _, op := range payload.Ops {
			if op.File == file && op.SyntaxCheck != nil {
				effCheck = *op.SyntaxCheck
			}
		}
		if !effCheck {
			continue
		}
		v := validateSyntax(file, text)
		syntaxResults[file] = map[string]interface{}{"validated": v.Validated, "detail": v.Detail}
		if vb, ok := v.Validated.(bool); ok && !vb {
			if atomic {
				return map[string]interface{}{
					"ok": false,
					"error": map[string]interface{}{
						"cls": "syntax-check-failed", "message": fmt.Sprintf("%s: %s", file, v.Detail),
					},
					"syntax_results": syntaxResults,
				}
			}
			// non-atomic: drop this file from the write set, keep others
			delete(fs.text, file)
		}
	}

	diffs := map[string]interface{}{}
	for _, file := range fs.order {
		if !dryRunFiles[file] {
			continue
		}
		text, ok := fs.text[file]
		if !ok {
			continue
		}
		orig, err := readFileUTF8(file)
		if err != nil {
			return refusedResult(err, len(opResults))
		}
		diffs[file] = unifiedDiff(orig, text, file)
	}

	var written []string
	dryRunAll := false
	if payload.DryRunAll != nil {
		dryRunAll = *payload.DryRunAll
	}
	if !dryRunAll {
		var journalEdits []ed.Edit
		for _, file := range fs.order {
			if dryRunFiles[file] {
				continue
			}
			text, ok := fs.text[file]
			if !ok {
				continue // dropped due to a non-atomic syntax failure
			}
			orig, err := readFileUTF8(file)
			if err != nil {
				return refusedResult(err, len(opResults))
			}
			if orig == text {
				continue
			}
			tmp := file + ".tmp"
			if err := os.WriteFile(tmp, []byte(text), 0644); err != nil {
				return refusedResult(refused("invalid-utf8-payload", "%s: %v", file, err), len(opResults))
			}
			if err := os.Rename(tmp, file); err != nil {
				return refusedResult(refused("invalid-utf8-payload", "%s: %v", file, err), len(opResults))
			}
			written = append(written, file)
			journalEdits = append(journalEdits, ed.Edit{File: file, Offset: 0, Old: orig, New: text})
		}
		if len(journalEdits) > 0 {
			j := ed.LoadJournal()
			label := fmt.Sprintf("str_replace_extended (%d file(s))", len(journalEdits))
			ed.Record(&j, journalEdits, label)
		}
	}

	if written == nil {
		written = []string{}
	}
	if markerLog == nil {
		markerLog = []string{}
	}
	return map[string]interface{}{
		"ok":             true,
		"ops":            opResults,
		"written":        written,
		"dry_run_diffs":  diffs,
		"syntax_results": syntaxResults,
		"markers":        markerLog,
	}
}

// ── unified diff (see the package doc comment's ADAPTATION note) ───

func splitLinesKeepEnds(s string) []string {
	if s == "" {
		return nil
	}
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i+1])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

type diffOp struct {
	kind byte // ' ' equal, '-' delete, '+' insert
	line string
}

// diffLines computes a line-level diff via a standard LCS dynamic
// program, falling back to a whole-block replace for pathologically
// large inputs (documented adaptation: Go's stdlib has no difflib
// equivalent; correctness and boundedness matter more here than
// matching Python's exact chosen hunks, since this output is an
// informational dry-run diff, not a decision input).
func diffLines(a, b []string) []diffOp {
	n, m := len(a), len(b)
	if n*m > 4_000_000 {
		ops := make([]diffOp, 0, n+m)
		for _, l := range a {
			ops = append(ops, diffOp{'-', l})
		}
		for _, l := range b {
			ops = append(ops, diffOp{'+', l})
		}
		return ops
	}
	dp := make([][]int32, n+1)
	for i := range dp {
		dp[i] = make([]int32, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if a[i] == b[j] {
				dp[i][j] = dp[i+1][j+1] + 1
			} else if dp[i+1][j] >= dp[i][j+1] {
				dp[i][j] = dp[i+1][j]
			} else {
				dp[i][j] = dp[i][j+1]
			}
		}
	}
	var ops []diffOp
	i, j := 0, 0
	for i < n && j < m {
		if a[i] == b[j] {
			ops = append(ops, diffOp{' ', a[i]})
			i++
			j++
		} else if dp[i+1][j] >= dp[i][j+1] {
			ops = append(ops, diffOp{'-', a[i]})
			i++
		} else {
			ops = append(ops, diffOp{'+', b[j]})
			j++
		}
	}
	for i < n {
		ops = append(ops, diffOp{'-', a[i]})
		i++
	}
	for j < m {
		ops = append(ops, diffOp{'+', b[j]})
		j++
	}
	return ops
}

// formatUnifiedDiff renders ops as a unified diff with 3 lines of
// context, merging overlapping/adjacent context windows into single
// hunks -- the same grouping convention difflib.unified_diff uses,
// though exact hunk-boundary edge cases (e.g. a pure insertion at
// byte 0) are not guaranteed byte-identical to Python's own output.
func formatUnifiedDiff(ops []diffOp, fromFile, toFile string) string {
	var changedIdx []int
	for idx, op := range ops {
		if op.kind != ' ' {
			changedIdx = append(changedIdx, idx)
		}
	}
	if len(changedIdx) == 0 {
		return ""
	}
	const context = 3
	type region struct{ start, end int }
	var regions []region
	for _, idx := range changedIdx {
		s := idx - context
		if s < 0 {
			s = 0
		}
		e := idx + context + 1
		if e > len(ops) {
			e = len(ops)
		}
		if len(regions) > 0 && s <= regions[len(regions)-1].end {
			if e > regions[len(regions)-1].end {
				regions[len(regions)-1].end = e
			}
		} else {
			regions = append(regions, region{s, e})
		}
	}

	aLineAt := make([]int, len(ops)+1)
	bLineAt := make([]int, len(ops)+1)
	aLineAt[0], bLineAt[0] = 1, 1
	for idx, op := range ops {
		aLineAt[idx+1] = aLineAt[idx]
		bLineAt[idx+1] = bLineAt[idx]
		if op.kind == ' ' || op.kind == '-' {
			aLineAt[idx+1]++
		}
		if op.kind == ' ' || op.kind == '+' {
			bLineAt[idx+1]++
		}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "--- a/%s\n", fromFile)
	fmt.Fprintf(&b, "+++ b/%s\n", toFile)
	for _, r := range regions {
		aStart := aLineAt[r.start]
		bStart := bLineAt[r.start]
		aCount, bCount := 0, 0
		for _, op := range ops[r.start:r.end] {
			if op.kind == ' ' || op.kind == '-' {
				aCount++
			}
			if op.kind == ' ' || op.kind == '+' {
				bCount++
			}
		}
		fmt.Fprintf(&b, "@@ -%d,%d +%d,%d @@\n", aStart, aCount, bStart, bCount)
		for _, op := range ops[r.start:r.end] {
			b.WriteByte(op.kind)
			b.WriteString(op.line)
			if !strings.HasSuffix(op.line, "\n") {
				b.WriteString("\n\\ No newline at end of file\n")
			}
		}
	}
	return b.String()
}

func unifiedDiff(a, b, name string) string {
	aLines := splitLinesKeepEnds(a)
	bLines := splitLinesKeepEnds(b)
	ops := diffLines(aLines, bLines)
	return formatUnifiedDiff(ops, name, name)
}

// ── CLI ──────────────────────────────────────────────────────────────

func cmdApply(argv []string) int {
	if len(argv) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: repoman strreplace apply <-|payload.json>")
		return 1
	}
	var raw []byte
	var err error
	if argv[0] == "-" {
		raw, err = io.ReadAll(os.Stdin)
	} else {
		raw, err = os.ReadFile(argv[0])
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "REFUSED: cannot read payload: %v\n", err)
		return 1
	}

	var payload payloadJSON
	if err := json.Unmarshal(raw, &payload); err != nil {
		fmt.Fprintf(os.Stderr, "REFUSED: payload is not valid JSON: %v\n", err)
		return 1
	}

	result := applyPayload(payload)
	out, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(out))

	ok, _ := result["ok"].(bool)
	if !ok {
		if errObj, ok := result["error"].(map[string]interface{}); ok {
			fmt.Fprintf(os.Stderr, "REFUSED: %v: %v\n", errObj["cls"], errObj["message"])
		}
		return 1
	}
	return 0
}

// Run implements `repoman strreplace <apply|selftest> ...`.
func Run(argv []string) int {
	if len(argv) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: repoman strreplace <apply|selftest> ...")
		return 1
	}
	switch argv[0] {
	case "-h", "--help":
		fmt.Println("usage: repoman strreplace [-h] {apply,selftest} ...")
		fmt.Println()
		fmt.Println("format-aware journaled text substitution")
		fmt.Println()
		fmt.Println("positional arguments:")
		fmt.Println("  {apply,selftest}")
		fmt.Println()
		fmt.Println("options:")
		fmt.Println("  -h, --help        show this help message and exit")
		return 0
	case "apply":
		for _, a := range argv[1:] {
			if a == "-h" || a == "--help" {
				fmt.Println("usage: repoman strreplace apply [-h] payload")
				fmt.Println()
				fmt.Println("positional arguments:")
				fmt.Println("  payload     '-' for stdin, or a path to a JSON payload file")
				fmt.Println()
				fmt.Println("options:")
				fmt.Println("  -h, --help  show this help message and exit")
				return 0
			}
		}
		return cmdApply(argv[1:])
	case "selftest":
		if len(argv) >= 2 && (argv[1] == "-h" || argv[1] == "--help") {
			fmt.Println("usage: repoman strreplace selftest [-h]")
			fmt.Println()
			fmt.Println("options:")
			fmt.Println("  -h, --help  show this help message and exit")
			return 0
		}
		return SelfTest()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", argv[0])
		return 1
	}
}
