package ed

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/ha1tch/gorepoman/pkg/roles"
	"github.com/ha1tch/gorepoman/pkg/webhelp"
)

const (
	MaxTxns  = 200
	MaxBytes = 10 * 1024 * 1024
	CtxBytes = 64
)

// truncateUTF8 returns s truncated to at most maxBytes bytes, never
// splitting a multi-byte UTF-8 rune. B-13 fix: every preview/label
// trim in this file used to slice by raw byte count (s[:n]), which
// silently produced invalid UTF-8 on stdout whenever the cut point
// landed inside a multi-byte codepoint -- confirmed reproducible with
// long runs of block-drawing characters (U+2591) in ed find's own
// line preview. If maxBytes itself lands mid-rune, back up to the
// start of that rune rather than keeping the partial bytes.
//
// TruncateUTF8 is the exported form, for other packages that already
// import pkg/ed anyway (badcode, provenance, strreplace) and need the
// same rune-safe truncation for their own preview/snippet text rather
// than duplicating this helper a third time.
func TruncateUTF8(s string, maxBytes int) string {
	return truncateUTF8(s, maxBytes)
}

func truncateUTF8(s string, maxBytes int) string {
	if len(s) <= maxBytes {
		return s
	}
	b := s[:maxBytes]
	for len(b) > 0 && !utf8.RuneStart(s[len(b)]) {
		b = b[:len(b)-1]
	}
	return b
}

type Edit struct {
	File   string `json:"file"`
	Offset int    `json:"offset"`
	Old    string `json:"old"`
	New    string `json:"new"`
}

type Txn struct {
	ID    int    `json:"id"`
	At    string `json:"at"`
	Label string `json:"label"`
	Edits []Edit `json:"edits"`
}

type Evicted struct {
	Count   int               `json:"count"`
	Anchors map[string]string `json:"anchors"`
}

// FileProvenance records the sha256 of one journal-tracked file's
// full content, as of the last repoman-mediated write to it --
// T-01's actual detection primitive. Deliberately NOT tied to a
// specific Txn: a Txn can be evicted (MaxTxns/MaxBytes) while the
// file it touched is still current and still worth checking, and
// undo (which removes a Txn rather than adding one) still needs
// somewhere to record the hash of the content it just restored.
type FileProvenance struct {
	Hash   string `json:"hash"`
	At     string `json:"at"`
	Reason string `json:"reason,omitempty"` // set only by Sanction (T-02); a normal repoman-mediated write never sets this -- it records a fresh, unremarkable hash instead
}

type Journal struct {
	Txns           []Txn                     `json:"txns"`
	Marks          map[string]int            `json:"marks"`
	Evicted        Evicted                   `json:"evicted"`
	FileProvenance map[string]FileProvenance `json:"file_provenance,omitempty"`
}

func journalPath() string {
	cwd, _ := os.Getwd()
	return filepath.Join(cwd, ".ed-journal.json")
}

func LoadJournal() Journal {
	j := Journal{
		Txns:  []Txn{},
		Marks: make(map[string]int),
		Evicted: Evicted{
			Count:   0,
			Anchors: make(map[string]string),
		},
	}
	b, err := os.ReadFile(journalPath())
	if err == nil {
		_ = json.Unmarshal(b, &j)
	}
	if j.Marks == nil {
		j.Marks = make(map[string]int)
	}
	if j.Evicted.Anchors == nil {
		j.Evicted.Anchors = make(map[string]string)
	}
	if j.FileProvenance == nil {
		j.FileProvenance = make(map[string]FileProvenance)
	}
	return j
}

func SaveJournal(j *Journal) error {
	storedSize := func(txns []Txn) int {
		total := 0
		for _, t := range txns {
			for _, e := range t.Edits {
				total += len(e.Old) + len(e.New)
			}
		}
		return total
	}

	for len(j.Txns) > MaxTxns || storedSize(j.Txns) > MaxBytes {
		ev := j.Txns[0]
		j.Txns = j.Txns[1:]
		j.Evicted.Count++
		for _, e := range ev.Edits {
			b, err := os.ReadFile(e.File)
			if err == nil {
				h := sha256.Sum256(b)
				j.Evicted.Anchors[e.File] = fmt.Sprintf("%x", h)[:16]
			}
		}
		newMarks := make(map[string]int)
		for k, v := range j.Marks {
			stillExists := false
			for _, t := range j.Txns {
				if t.ID == v {
					stillExists = true
					break
				}
			}
			if stillExists {
				newMarks[k] = v
			}
		}
		j.Marks = newMarks
	}

	tmp := journalPath() + ".tmp"
	b, err := json.Marshal(j)
	if err != nil {
		return err
	}
	if err := os.WriteFile(tmp, b, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, journalPath())
}

func SpanHash(text string, start, end int) string {
	lo := start - CtxBytes
	if lo < 0 {
		lo = 0
	}
	hi := end + CtxBytes
	if hi > len(text) {
		hi = len(text)
	}
	h := sha256.Sum256([]byte(text[lo:hi]))
	return fmt.Sprintf("%x", h)[:8]
}

func Record(j *Journal, edits []Edit, label string) {
	id := 1
	if len(j.Txns) > 0 {
		id = j.Txns[len(j.Txns)-1].ID + 1
	} else if j.Evicted.Count > 0 {
		id = j.Evicted.Count + 1
	}
	j.Txns = append(j.Txns, Txn{
		ID:    id,
		At:    time.Now().UTC().Format("2006-01-02T15:04:05Z"),
		Label: label,
		Edits: edits,
	})
	recordProvenance(j, edits)
	SaveJournal(j)
}

// recordProvenance hashes the CURRENT on-disk content of every
// unique file named in edits and stores it in j.FileProvenance --
// called only after the caller has already written the file (every
// call site does: os.WriteFile then Record, or the equivalent
// os.Rename-based atomic write in strreplace), so this always reads
// back exactly what was just written, not a stale in-memory copy.
// A read failure is silently skipped rather than aborting the
// journal write that already succeeded -- provenance tracking must
// never be the reason a real edit fails to record.
func recordProvenance(j *Journal, edits []Edit) {
	if j.FileProvenance == nil {
		j.FileProvenance = make(map[string]FileProvenance)
	}
	seen := make(map[string]bool)
	now := time.Now().UTC().Format("2006-01-02T15:04:05Z")
	for _, e := range edits {
		if seen[e.File] {
			continue
		}
		seen[e.File] = true
		b, err := os.ReadFile(e.File)
		if err != nil {
			continue
		}
		h := sha256.Sum256(b)
		j.FileProvenance[e.File] = FileProvenance{Hash: fmt.Sprintf("%x", h), At: now}
	}
}

// ProvenanceStatus is CheckProvenance's plain-facts result -- deliberately
// not the provenance package's own Mismatch type, since ed cannot import
// provenance (provenance already imports ed for Journal/LoadJournal; the
// reverse would be a cycle). This lives in ed because ed's own write
// paths (T-31: apply, sub, append/prepend, undo) are the actual callers
// that need to refuse a write before it lands on stale content -- the
// provenance package's Check/CheckOne wrap this same primitive for the
// on-demand `provenance check` CLI and its own Mismatch reporting.
type ProvenanceStatus struct {
	Tracked     bool // false: no provenance recorded for this path yet
	Mismatch    bool // true: recorded hash does not match current content
	Missing     bool // true: path is tracked but no longer exists
	RecordedAt  string
	RecordedHex string
	CurrentHex  string
}

// CheckProvenance compares path's current on-disk content against j's
// recorded hash for it, if any. A path with no recorded provenance
// (never touched by repoman) returns Tracked: false and is never a
// reason to refuse -- this check has nothing to compare against.
func CheckProvenance(j Journal, path string) ProvenanceStatus {
	fp, tracked := j.FileProvenance[path]
	if !tracked {
		return ProvenanceStatus{Tracked: false}
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return ProvenanceStatus{Tracked: true, Missing: true, RecordedAt: fp.At, RecordedHex: fp.Hash}
	}
	h := sha256.Sum256(b)
	current := fmt.Sprintf("%x", h)
	if current != fp.Hash {
		return ProvenanceStatus{Tracked: true, Mismatch: true, RecordedAt: fp.At, RecordedHex: fp.Hash, CurrentHex: current}
	}
	return ProvenanceStatus{Tracked: true}
}

// Sanction is T-02: the only way to clear a provenance mismatch
// without redoing the edit through repoman. It re-syncs path's
// recorded hash to its CURRENT on-disk content -- accepting whatever
// is there now as the new known-good state -- and records reason
// alongside the hash at the moment of sanction, exactly as T-02's own
// register entry specifies. reason is mandatory (refused if blank):
// this is a deliberate human override of a safety check, and the
// audit trail is the entire point, not an optional courtesy.
//
// Deliberately narrow: refuses unless CheckProvenance currently
// reports a real Mismatch. Sanctioning a path with no mismatch (never
// tracked, or already matching) would silently do nothing meaningful
// and makes the command a confusing no-op; sanctioning a Missing path
// is refused too -- there is no current content to certify, so
// restoring the file (or accepting its absence some other way) has to
// come first.
func Sanction(path, reason string) error {
	if strings.TrimSpace(reason) == "" {
		return fmt.Errorf("REFUSED: --reason is mandatory. Nothing sanctioned.")
	}
	j := LoadJournal()
	st := CheckProvenance(j, path)
	if !st.Mismatch {
		if st.Missing {
			return fmt.Errorf("REFUSED: %s no longer exists -- nothing to sanction (restore it, or its absence needs a different remedy)", path)
		}
		return fmt.Errorf("REFUSED: no provenance mismatch recorded for %s -- nothing to sanction", path)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("REFUSED: %s: %v", path, err)
	}
	h := sha256.Sum256(b)
	if j.FileProvenance == nil {
		j.FileProvenance = make(map[string]FileProvenance)
	}
	j.FileProvenance[path] = FileProvenance{
		Hash:   fmt.Sprintf("%x", h),
		At:     time.Now().UTC().Format("2006-01-02T15:04:05Z"),
		Reason: reason,
	}
	return SaveJournal(&j)
}

func parseHandle(h string) (string, int, int, string, error) {
	parts := strings.Split(h, ":")
	if len(parts) < 3 {
		// B-08: give a specific, actionable message for the common
		// // mistake of copying only the trailing hash off find's output
		// instead of the full "file:start-end:hash" handle, rather than
		// the same generic "malformed handle" for every kind of mistake.
		if len(parts) == 1 && isLikelyBareHash(h) {
			return "", 0, 0, "", fmt.Errorf("%q looks like a bare hash, not a full handle -- apply needs the complete file:start-end:hash string find printed, not just the trailing hash", h)
		}
		return "", 0, 0, "", fmt.Errorf("malformed handle %q; expected file:start-end:hash", h)
	}
	hash := parts[len(parts)-1]
	span := parts[len(parts)-2]
	path := strings.Join(parts[:len(parts)-2], ":")

	spanParts := strings.Split(span, "-")
	if len(spanParts) != 2 {
		return "", 0, 0, "", fmt.Errorf("malformed span")
	}
	s, err1 := strconv.Atoi(spanParts[0])
	e, err2 := strconv.Atoi(spanParts[1])
	if err1 != nil || err2 != nil {
		return "", 0, 0, "", fmt.Errorf("malformed span bounds")
	}
	return path, s, e, hash, nil
}

// isLikelyBareHash reports whether s looks like just the 8-hex-char
// SpanHash a handle ends with, rather than a real path (which would
// contain a slash, a dot, or non-hex letters in virtually every case).
func isLikelyBareHash(s string) bool {
	if len(s) < 4 || len(s) > 16 {
		return false
	}
	for _, r := range s {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) {
			return false
		}
	}
	return true
}

func revertTxn(j *Journal, t Txn) error {
	perFile := make(map[string][]Edit)
	for _, e := range t.Edits {
		perFile[e.File] = append(perFile[e.File], e)
	}

	staged := make(map[string]string)
	for fname, edits := range perFile {
		b, err := os.ReadFile(fname)
		if err != nil {
			return fmt.Errorf("%s no longer exists", fname)
		}
		if st := CheckProvenance(*j, fname); st.Mismatch || st.Missing {
			return fmt.Errorf("%s was edited outside repoman since %s -- run `repoman provenance check` for details; undo refuses to revert on top of an unrecorded change", fname, st.RecordedAt)
		}
		text := string(b)

		sort.Slice(edits, func(i, j int) bool {
			return edits[i].Offset > edits[j].Offset
		})

		for _, e := range edits {
			s := e.Offset
			if s+len(e.New) > len(text) || text[s:s+len(e.New)] != e.New {
				return fmt.Errorf("%s changed at offset %d since the edit — cannot undo safely", fname, s)
			}
			text = text[:s] + e.Old + text[s+len(e.New):]
		}
		staged[fname] = text
	}

	for p, text := range staged {
		os.WriteFile(p, []byte(text), 0644)
	}
	var touched []Edit
	for p := range staged {
		touched = append(touched, Edit{File: p})
	}
	recordProvenance(j, touched)
	return nil
}

func Run(args []string) int {
	args = webhelp.NormalizeBriefFirst(args)
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: repoman ed <find|apply|append|prepend|insert|niplines|confirm|cancel|sub|undo|mark|log|selftest> ...")
		return 1
	}

	cmd := args[0]
	switch cmd {
	case "-h", "--help":
		fmt.Println("usage: repoman ed [-h] {find,apply,append,prepend,insert,niplines,confirm,cancel,sub,undo,mark,log,selftest} ...")
		fmt.Println()
		fmt.Println("journaled precise text editing")
		fmt.Println()
		fmt.Println("positional arguments:")
		fmt.Println("  {find,apply,append,prepend,insert,niplines,confirm,cancel,sub,undo,mark,log,selftest}")
		fmt.Println()
		fmt.Println("options:")
		fmt.Println("  -h, --help            show this help message and exit")
		fmt.Println()
		fmt.Println("See https://ha1tch.github.io/gorepoman/docs/repoman-040-editing.html")
		fmt.Println("for worked examples of the full find -> apply/sub workflow.")
		fmt.Println(webhelp.SuppressionNote)
		webhelp.PrintIfAvailable(os.Stdout, "repoman-040-editing", args)
		return 0

	case "find":
		for _, a := range args[1:] {
			if a == "-h" || a == "--help" {
				fmt.Println("usage: repoman ed find [-h] [--regex] term [paths ...]")
				fmt.Println()
				fmt.Println("positional arguments:")
				fmt.Println("  term")
				fmt.Println("  paths")
				fmt.Println()
				fmt.Println("options:")
				fmt.Println("  -h, --help  show this help message and exit")
				fmt.Println("  --regex")
				return 0
			}
		}
		// B-01 fix: scan for --regex anywhere in args[1:], not just after
		// the term -- it silently became the literal search term before
		// (`ed find --regex PAT file` searched for "--regex" itself).
		isRegex := false
		var positional []string
		for _, a := range args[1:] {
			if a == "--regex" {
				isRegex = true
			} else {
				positional = append(positional, a)
			}
		}
		if len(positional) < 1 {
			fmt.Fprintln(os.Stderr, "find requires a term")
			return 1
		}
		term := positional[0]
		targetPaths := []string{"."}
		if len(positional) > 1 {
			targetPaths = positional[1:]
		}
		paths := roles.Expand(targetPaths)
		n := 0
		for _, occ := range roles.Occurrences(term, paths, isRegex) {
			b, _ := os.ReadFile(occ.Path)
			h := SpanHash(string(b), occ.Start, occ.End)
			lineTrim := truncateUTF8(strings.TrimSpace(occ.Line), 80)
			fmt.Printf("%s:%d-%d:%s  [%s]  line %d: %s\n", occ.Path, occ.Start, occ.End, h, occ.Role, occ.LineNo, lineTrim)
			n++
		}
		fmt.Printf("%d occurrence(s)\n", n)
		return 0

	case "apply":
		for _, a := range args[1:] {
			if a == "-h" || a == "--help" {
				fmt.Println("usage: repoman ed apply [-h] --with REPLACEMENT handle")
				fmt.Println()
				fmt.Println("positional arguments:")
				fmt.Println("  handle")
				fmt.Println()
				fmt.Println("options:")
				fmt.Println("  -h, --help          show this help message and exit")
				fmt.Println("  --with REPLACEMENT")
				return 0
			}
		}
		if len(args) < 4 || args[2] != "--with" {
			fmt.Fprintln(os.Stderr, "Usage: repoman ed apply <handle> --with <text>")
			return 1
		}
		handle := args[1]
		replacement := args[3]
		path, s, e, hash, err := parseHandle(handle)
		if err != nil {
			fmt.Fprintf(os.Stderr, "REFUSED: %v\n", err)
			return 1
		}
		b, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "REFUSED: %s does not exist\n", path)
			return 1
		}
		j := LoadJournal()
		if st := CheckProvenance(j, path); st.Mismatch || st.Missing {
			fmt.Fprintf(os.Stderr, "REFUSED: %s was edited outside repoman since %s -- run `repoman provenance check` for details. Nothing written.\n", path, st.RecordedAt)
			return 1
		}
		text := string(b)
		if e > len(text) || SpanHash(text, s, e) != hash {
			fmt.Fprintf(os.Stderr, "REFUSED: %s changed since find (stale handle) — re-run find and use a fresh handle\n", path)
			return 1
		}
		old := text[s:e]
		newText := text[:s] + replacement + text[e:]
		os.WriteFile(path, []byte(newText), 0644)

		Record(&j, []Edit{{File: path, Offset: s, Old: old, New: replacement}}, "apply "+filepath.Base(path))

		oldTrim := truncateUTF8(old, 40)
		newTrim := truncateUTF8(replacement, 40)
		fmt.Printf("applied at %s:%d: %q -> %q\n", path, s, oldTrim, newTrim)
		return 0

	case "append", "prepend":
		for _, a := range args[1:] {
			if a == "-h" || a == "--help" {
				fmt.Printf("usage: repoman ed %s FILE --with TEXT\n", cmd)
				fmt.Println()
				fmt.Println("positional arguments:")
				fmt.Println("  FILE")
				fmt.Println()
				fmt.Println("options:")
				fmt.Println("  -h, --help    show this help message and exit")
				fmt.Println("  --with TEXT")
				fmt.Println()
				fmt.Println("Unconditionally safe: no find-verified handle is needed, because the")
				fmt.Println("target is a true file boundary (offset 0 for prepend, EOF for append),")
				fmt.Println("not existing text being trusted the way apply's matched span is.")
				return 0
			}
		}
		if len(args) < 4 || args[2] != "--with" {
			fmt.Fprintf(os.Stderr, "Usage: repoman ed %s FILE --with TEXT\n", cmd)
			return 1
		}
		path := args[1]
		addition := args[3]
		b, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "REFUSED: %s does not exist\n", path)
			return 1
		}
		j := LoadJournal()
		if st := CheckProvenance(j, path); st.Mismatch || st.Missing {
			fmt.Fprintf(os.Stderr, "REFUSED: %s was edited outside repoman since %s -- run `repoman provenance check` for details. Nothing written.\n", path, st.RecordedAt)
			return 1
		}
		text := string(b)
		var offset int
		var newText string
		if cmd == "append" {
			offset = len(text)
			newText = text + addition
		} else {
			offset = 0
			newText = addition + text
		}
		os.WriteFile(path, []byte(newText), 0644)

		Record(&j, []Edit{{File: path, Offset: offset, Old: "", New: addition}}, cmd+" "+filepath.Base(path))

		addTrim := truncateUTF8(addition, 40)
		fmt.Printf("%sed to %s: %q (%d bytes)\n", cmd, path, addTrim, len(addition))
		return 0

	case "insert":
		for _, a := range args[1:] {
			if a == "-h" || a == "--help" {
				fmt.Println("usage: repoman ed insert HANDLE --after|--before --with TEXT")
				fmt.Println()
				fmt.Println("positional arguments:")
				fmt.Println("  HANDLE")
				fmt.Println()
				fmt.Println("options:")
				fmt.Println("  -h, --help    show this help message and exit")
				fmt.Println("  --after       insert immediately after the matched span")
				fmt.Println("  --before      insert immediately before the matched span")
				fmt.Println("  --with TEXT")
				fmt.Println()
				fmt.Println("Requires a find-verified handle exactly like apply does, and re-runs")
				fmt.Println("the same SpanHash check before writing. Unlike apply, the matched span")
				fmt.Println("itself is never touched -- only text before or after it is inserted --")
				fmt.Println("so this verb cannot reproduce apply's anchor-duplication failure mode,")
				fmt.Println("where a --with replacement accidentally restates part of the match.")
				return 0
			}
		}
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: repoman ed insert <handle> --after|--before --with <text>")
			return 1
		}
		handle := args[1]
		isAfter := false
		isBefore := false
		insertion := ""
		haveWith := false
		for i := 2; i < len(args); i++ {
			switch args[i] {
			case "--after":
				isAfter = true
			case "--before":
				isBefore = true
			case "--with":
				if i+1 >= len(args) {
					fmt.Fprintln(os.Stderr, "Usage: repoman ed insert <handle> --after|--before --with <text>")
					return 1
				}
				insertion = args[i+1]
				haveWith = true
				i++
			}
		}
		if isAfter == isBefore {
			fmt.Fprintln(os.Stderr, "Usage: repoman ed insert <handle> --after|--before --with <text> (exactly one of --after/--before)")
			return 1
		}
		if !haveWith {
			fmt.Fprintln(os.Stderr, "Usage: repoman ed insert <handle> --after|--before --with <text>")
			return 1
		}
		path, s, e, hash, err := parseHandle(handle)
		if err != nil {
			fmt.Fprintf(os.Stderr, "REFUSED: %v\n", err)
			return 1
		}
		b, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "REFUSED: %s does not exist\n", path)
			return 1
		}
		j := LoadJournal()
		if st := CheckProvenance(j, path); st.Mismatch || st.Missing {
			fmt.Fprintf(os.Stderr, "REFUSED: %s was edited outside repoman since %s -- run `repoman provenance check` for details. Nothing written.\n", path, st.RecordedAt)
			return 1
		}
		text := string(b)
		if e > len(text) || SpanHash(text, s, e) != hash {
			fmt.Fprintf(os.Stderr, "REFUSED: %s changed since find (stale handle) — re-run find and use a fresh handle\n", path)
			return 1
		}
		var offset int
		if isAfter {
			offset = e
		} else {
			offset = s
		}
		newText := text[:offset] + insertion + text[offset:]
		os.WriteFile(path, []byte(newText), 0644)

		Record(&j, []Edit{{File: path, Offset: offset, Old: "", New: insertion}}, "insert "+filepath.Base(path))

		insTrim := truncateUTF8(insertion, 40)
		side := "after"
		if isBefore {
			side = "before"
		}
		fmt.Printf("inserted %s at %s:%d: %q\n", side, path, offset, insTrim)
		return 0

	case "niplines":
		return runNiplines(args)

	case "confirm":
		return runConfirm(args)

	case "cancel":
		return runCancel(args)

	case "sub":
		for _, a := range args[1:] {
			if a == "-h" || a == "--help" {
				fmt.Println("usage: repoman ed sub [-h] --expect EXPECT [--force-roles] old new [paths ...]")
				fmt.Println()
				fmt.Println("positional arguments:")
				fmt.Println("  old")
				fmt.Println("  new")
				fmt.Println("  paths")
				fmt.Println()
				fmt.Println("options:")
				fmt.Println("  -h, --help       show this help message and exit")
				fmt.Println("  --expect EXPECT")
				fmt.Println("  --force-roles")
				return 0
			}
		}
		// B-01 fix: --expect and --force-roles are now recognized anywhere in
		// args[1:], not just from index 3 onward -- previously
		// `ed sub --expect 1 OLD NEW file` silently took "--expect" as OLD
		// and "1" as NEW, found 0 real occurrences, and since --expect was
		// then never actually parsed (expect defaulted to its zero value),
		// 0 == 0 passed as success. expectSeen now makes a missing --expect
		// an explicit refusal regardless of what total happens to be,
		// closing that specific 0-equals-0 coincidence for good.
		var oldText, newText string
		var expect int
		expectSeen := false
		forceRoles := false
		var positional []string
		for i := 1; i < len(args); i++ {
			switch {
			case args[i] == "--expect":
				if i+1 >= len(args) {
					fmt.Fprintln(os.Stderr, "--expect requires a number")
					return 1
				}
				expect, _ = strconv.Atoi(args[i+1])
				expectSeen = true
				i++
			case args[i] == "--force-roles":
				forceRoles = true
			default:
				positional = append(positional, args[i])
			}
		}
		if len(positional) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: repoman ed sub <old> <new> [path ...] --expect N [--force-roles]")
			return 1
		}
		if !expectSeen {
			fmt.Fprintln(os.Stderr, "REFUSED: --expect N is required. Nothing written.")
			return 1
		}
		oldText = positional[0]
		newText = positional[1]
		targetPaths := []string{"."}
		if len(positional) > 2 {
			targetPaths = positional[2:]
		}

		paths := roles.Expand(targetPaths)
		type planEntry struct {
			path string
			text string
			offs []int
		}
		var plan []planEntry
		total := 0
		roleSet := make(map[string]bool)

		for _, p := range paths {
			b, err := os.ReadFile(p)
			if err != nil {
				continue
			}
			text := string(b)
			var offs []int
			i := strings.Index(text, oldText)
			for i != -1 {
				offs = append(offs, i)
				roleSet[roles.Classify(p, text, i)] = true
				next := strings.Index(text[i+1:], oldText)
				if next == -1 {
					break
				}
				i += 1 + next
			}
			if len(offs) > 0 {
				plan = append(plan, planEntry{p, text, offs})
				total += len(offs)
				fmt.Printf("  %s: %d\n", p, len(offs))
			}
		}

		if total != expect {
			fmt.Fprintf(os.Stderr, "REFUSED: found %d occurrence(s), --expect %d. Nothing written. Re-count, or narrow the paths.\n", total, expect)
			return 1
		}
		if len(roleSet) > 1 && !forceRoles {
			var rs []string
			for r := range roleSet {
				rs = append(rs, r)
			}
			sort.Strings(rs)
			fmt.Fprintf(os.Stderr, "REFUSED: occurrences span multiple syntactic roles %v — one pass is not safe (working agreement §7.1). Split by role, or pass --force-roles if you have classified them as one treatment.\n", rs)
			return 1
		}

		j := LoadJournal()
		for _, e := range plan {
			if st := CheckProvenance(j, e.path); st.Mismatch || st.Missing {
				fmt.Fprintf(os.Stderr, "REFUSED: %s was edited outside repoman since %s -- run `repoman provenance check` for details. Nothing written.\n", e.path, st.RecordedAt)
				return 1
			}
		}

		var edits []Edit
		for _, e := range plan {
			newFileText := strings.ReplaceAll(e.text, oldText, newText)
			os.WriteFile(e.path, []byte(newFileText), 0644)
			for _, o := range e.offs {
				edits = append(edits, Edit{File: e.path, Offset: o, Old: oldText, New: newText})
			}
		}

		labelOld := truncateUTF8(oldText, 30)
		labelNew := truncateUTF8(newText, 30)
		Record(&j, edits, fmt.Sprintf("sub %q->%q", labelOld, labelNew))
		fmt.Printf("replaced %d occurrence(s) across %d file(s)\n", total, len(plan))
		return 0

	case "undo":
		for _, a := range args[1:] {
			if a == "-h" || a == "--help" {
				fmt.Println("usage: repoman ed undo [-h] [--since SINCE] [n]")
				fmt.Println()
				fmt.Println("positional arguments:")
				fmt.Println("  n")
				fmt.Println()
				fmt.Println("options:")
				fmt.Println("  -h, --help     show this help message and exit")
				fmt.Println("  --since SINCE")
				return 0
			}
		}
		n := 1
		since := ""
		for i := 1; i < len(args); i++ {
			if args[i] == "--since" && i+1 < len(args) {
				since = args[i+1]
				i++
			} else {
				if parsed, err := strconv.Atoi(args[i]); err == nil {
					n = parsed
				}
			}
		}

		j := LoadJournal()
		var batch []Txn

		if since != "" {
			target, ok := j.Marks[since]
			if !ok {
				fmt.Fprintf(os.Stderr, "REFUSED: mark %q not in the journal (unknown, or evicted — %d transaction(s) have rolled off). No partial undo offered.\n", since, j.Evicted.Count)
				return 1
			}
			for _, t := range j.Txns {
				if t.ID > target {
					batch = append(batch, t)
				}
			}
		} else {
			if len(j.Txns) < n {
				n = len(j.Txns)
			}
			if n > 0 {
				batch = j.Txns[len(j.Txns)-n:]
			}
		}

		if len(batch) == 0 {
			fmt.Println("nothing to undo")
			return 0
		}

		for i := len(batch) - 1; i >= 0; i-- {
			t := batch[i]
			err := revertTxn(&j, t)
			if err != nil {
				fmt.Fprintf(os.Stderr, "REFUSED at txn %d (%s): %v. Transactions after it were already reverted — journal log shows the boundary.\n", t.ID, t.Label, err)

				// Truncate journal to successful reverts
				var newTxns []Txn
				for _, xt := range j.Txns {
					if xt.ID <= t.ID {
						newTxns = append(newTxns, xt)
					}
				}
				j.Txns = newTxns
				SaveJournal(&j)
				return 1
			}
			// Remove from journal
			j.Txns = j.Txns[:len(j.Txns)-1]
			fmt.Printf("undone: txn %d (%s)\n", t.ID, t.Label)
		}
		SaveJournal(&j)
		return 0

	case "mark":
		if len(args) >= 2 && (args[1] == "-h" || args[1] == "--help") {
			fmt.Println("usage: repoman ed mark [-h] name")
			fmt.Println()
			fmt.Println("positional arguments:")
			fmt.Println("  name")
			fmt.Println()
			fmt.Println("options:")
			fmt.Println("  -h, --help  show this help message and exit")
			return 0
		}
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "mark requires a name")
			return 1
		}
		j := LoadJournal()
		id := 0
		if len(j.Txns) > 0 {
			id = j.Txns[len(j.Txns)-1].ID
		}
		j.Marks[args[1]] = id
		SaveJournal(&j)
		fmt.Printf("mark %q at txn %d\n", args[1], id)
		return 0

	case "log":
		for _, a := range args[1:] {
			if a == "-h" || a == "--help" {
				fmt.Println("usage: repoman ed log [-h] [n]")
				fmt.Println()
				fmt.Println("positional arguments:")
				fmt.Println("  n")
				fmt.Println()
				fmt.Println("options:")
				fmt.Println("  -h, --help  show this help message and exit")
				return 0
			}
		}
		n := 20
		if len(args) > 1 {
			if parsed, err := strconv.Atoi(args[1]); err == nil {
				n = parsed
			}
		}
		j := LoadJournal()
		start := len(j.Txns) - n
		if start < 0 {
			start = 0
		}
		for _, t := range j.Txns[start:] {
			fmt.Printf("txn %d  %s  %s  (%d edit(s))\n", t.ID, t.At, t.Label, len(t.Edits))
		}
		if j.Evicted.Count > 0 {
			fmt.Printf("[%d evicted; anchors held for %d file(s)]\n", j.Evicted.Count, len(j.Evicted.Anchors))
		}
		return 0

	case "selftest":
		if len(args) >= 2 && (args[1] == "-h" || args[1] == "--help") {
			fmt.Println("usage: repoman ed selftest [-h]")
			fmt.Println()
			fmt.Println("options:")
			fmt.Println("  -h, --help  show this help message and exit")
			return 0
		}
		return SelfTest()

	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", cmd)
		return 1
	}
}
