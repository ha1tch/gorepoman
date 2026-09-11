// Package provenance implements T-01/T-02: detecting an edit made to
// a journal-tracked file OUTSIDE repoman -- raw str_replace, sed,
// hand-editing, or (the real incident that prompted this) an agent
// reaching for a generic editing tool instead of repoman's own -- and
// explicitly sanctioning one when it turns out to be fine.
//
// THE ACTUAL INCIDENT THIS CLOSES: mid-session, a file already under
// repoman's journal was edited once via a different editing tool
// entirely, bypassing repoman's find/apply staleness check and
// journal recording. It was caught immediately by chance -- a human
// reviewing the diff -- not by any mechanism that would catch it
// reliably or automatically. This package is that mechanism.
//
// DESIGN: pkg/ed's Journal already gained a FileProvenance map (this
// same change) recording the sha256 of every journal-tracked file's
// full content as of the last repoman-mediated write to it --
// updated by ed.Record (used by both `ed` and `strreplace`'s write
// paths) and by ed's own undo path, so every write this journal knows
// about keeps the map current. Check here does the obvious half:
// recompute each tracked file's CURRENT hash and compare. A mismatch
// means the file changed since repoman last touched it, by
// definition outside repoman's own write paths -- there is no other
// way for the hash to diverge.
//
// T-31 wired this same comparison into ed and strreplace's own write
// paths, so they refuse to write on top of a mismatched file rather
// than only catching it later on an on-demand check -- which closed
// the actual gap but also closed the only documented recovery path
// (redoing the edit through repoman used to re-sync the hash; it
// still does, but now that same write is what's being refused). T-02
// (Sanction, this package's `provenance sanction` subcommand) is the
// deliberate human override that path needed: --reason is mandatory,
// and it re-syncs the hash to current content directly, without
// requiring the edit to be redone.
//
// SCOPE (v1): journal-tracked files only, no git integration.
package provenance

import (
	"fmt"
	"os"
	"sort"

	"github.com/ha1tch/gorepoman/pkg/ed"
	"github.com/ha1tch/gorepoman/pkg/webhelp"
)

// Mismatch is one journal-tracked file whose current content does
// not match the hash repoman last recorded for it.
type Mismatch struct {
	File        string
	RecordedAt  string
	RecordedHex string
	CurrentHex  string
	Missing     bool // true if the file no longer exists at all
}

// Check recomputes and compares every file in the journal's
// FileProvenance map against its current on-disk content. Returns
// mismatches in deterministic (sorted by path) order -- map iteration
// order in Go is intentionally randomized, and a check whose report
// order changes between identical runs is exactly the kind of thing
// that erodes trust in the check itself.
func Check() ([]Mismatch, error) {
	j := ed.LoadJournal()

	var files []string
	for f := range j.FileProvenance {
		files = append(files, f)
	}
	sort.Strings(files)

	var mismatches []Mismatch
	for _, f := range files {
		if m := checkFile(j, f); m != nil {
			mismatches = append(mismatches, *m)
		}
	}
	return mismatches, nil
}

// CheckOne checks a single path against an already-loaded journal --
// used by ed and strreplace's own write paths (T-31), which already
// have a Journal in scope and would otherwise reload it redundantly.
// Returns nil, nil for a file with no recorded provenance yet (never
// touched by repoman), exactly like Check() silently skips such files
// today -- a file repoman has never seen is not this check's business.
func CheckOne(j ed.Journal, path string) (*Mismatch, error) {
	return checkFile(j, path), nil
}

// checkFile is Check/CheckOne's shared comparison: recompute path's
// current sha256 and compare it against j's recorded hash, if any.
// Returns nil when there is nothing to report -- no provenance
// recorded, or the recorded hash still matches.
func checkFile(j ed.Journal, path string) *Mismatch {
	st := ed.CheckProvenance(j, path)
	if !st.Tracked {
		return nil
	}
	if st.Missing {
		return &Mismatch{File: path, RecordedAt: st.RecordedAt, RecordedHex: st.RecordedHex, Missing: true}
	}
	if st.Mismatch {
		return &Mismatch{File: path, RecordedAt: st.RecordedAt, RecordedHex: st.RecordedHex, CurrentHex: st.CurrentHex}
	}
	return nil
}

func Run(argv []string) int {
	argv = webhelp.NormalizeBriefFirst(argv)
	if len(argv) > 0 && (argv[0] == "-h" || argv[0] == "--help") {
		fmt.Println("usage: repoman provenance [-h] {check,sanction} ...")
		fmt.Println()
		fmt.Println("Detect edits made to a journal-tracked file OUTSIDE repoman --")
		fmt.Println("see 'repoman provenance check --help' and 'repoman provenance")
		fmt.Println("sanction --help' for details.")
		fmt.Println()
		fmt.Println("positional arguments:")
		fmt.Println("  {check,sanction}")
		fmt.Println()
		fmt.Println("options:")
		fmt.Println("  -h, --help  show this help message and exit")
		fmt.Println(webhelp.SuppressionNote)
		webhelp.PrintIfAvailable(os.Stdout, "repoman-088-provenance", argv)
		return 0
	}
	if len(argv) == 0 || (argv[0] != "check" && argv[0] != "sanction") {
		fmt.Fprintln(os.Stderr, "Usage: repoman provenance {check|sanction} ...")
		return 1
	}

	if argv[0] == "sanction" {
		return runSanction(argv[1:])
	}

	if len(argv) >= 2 && (argv[1] == "-h" || argv[1] == "--help") {
		fmt.Println("usage: repoman provenance check [-h]")
		fmt.Println()
		fmt.Println("Recomputes the sha256 of every journal-tracked file (.ed-journal.json's")
		fmt.Println("file_provenance map, updated on every repoman-mediated write -- ed")
		fmt.Println("apply/sub/append/prepend/insert/undo, and strreplace) and compares it")
		fmt.Println("against the hash repoman last recorded. A mismatch means the file was")
		fmt.Println("edited outside repoman since then -- there is no other way for the hash")
		fmt.Println("to diverge. Refuses (exit 1) if any mismatch is found, or if a tracked")
		fmt.Println("file no longer exists at all.")
		fmt.Println()
		fmt.Println("v1 scope: journal-tracked files only, no git integration. ed and")
		fmt.Println("strreplace's own write paths refuse to touch a mismatched file (T-31);")
		fmt.Println("to accept a real out-of-band edit, use 'repoman provenance sanction")
		fmt.Println("<file> --reason \"...\"' first -- that re-syncs the recorded hash and")
		fmt.Println("clears the block. Redoing the SAME edit through repoman also re-syncs")
		fmt.Println("it, without needing sanction, since that write updates the hash itself.")
		fmt.Println()
		fmt.Println("options:")
		fmt.Println("  -h, --help  show this help message and exit")
		fmt.Println(webhelp.SuppressionNote)
		webhelp.PrintIfAvailable(os.Stdout, "repoman-088-provenance", argv)
		return 0
	}

	mismatches, err := Check()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR provenance-check: %v\n", err)
		return 1
	}

	if len(mismatches) == 0 {
		j := ed.LoadJournal()
		fmt.Printf("PROVENANCE CHECK OK (%d file(s) tracked)\n", len(j.FileProvenance))
		return 0
	}

	for _, m := range mismatches {
		if m.Missing {
			fmt.Fprintf(os.Stderr, "ERROR provenance-missing: %s no longer exists (last known-good hash recorded %s)\n",
				m.File, m.RecordedAt)
			continue
		}
		fmt.Fprintf(os.Stderr, "ERROR provenance-mismatch: %s changed outside repoman since %s -- recorded %s..., now %s...\n",
			m.File, m.RecordedAt, m.RecordedHex[:12], m.CurrentHex[:12])
	}
	fmt.Printf("PROVENANCE CHECK FAIL: %d mismatch(es)\n", len(mismatches))
	return 1
}

// runSanction implements T-02: `repoman provenance sanction <file>
// --reason "..."`. A thin CLI wrapper -- all the actual logic (refuse
// without --reason, refuse unless a real mismatch is on record,
// re-sync the hash to current content) lives in ed.Sanction, which
// owns the journal read/write this needs.
func runSanction(argv []string) int {
	argv = webhelp.NormalizeBriefFirst(argv)
	for _, a := range argv {
		if a == "-h" || a == "--help" {
			fmt.Println("usage: repoman provenance sanction FILE --reason REASON")
			fmt.Println()
			fmt.Println("Explicitly accepts a detected out-of-band edit: re-syncs FILE's")
			fmt.Println("recorded provenance hash to its CURRENT on-disk content, clearing")
			fmt.Println("the mismatch and un-refusing ed/strreplace's own write paths (T-31)")
			fmt.Println("for it. --reason is mandatory and is recorded alongside the hash --")
			fmt.Println("this is a deliberate override of a safety check, and the audit trail")
			fmt.Println("is the entire point.")
			fmt.Println()
			fmt.Println("Refused if FILE has no mismatch on record (nothing to sanction), or")
			fmt.Println("no longer exists (there is no current content to certify -- restore")
			fmt.Println("it first).")
			fmt.Println()
			fmt.Println("positional arguments:")
			fmt.Println("  FILE")
			fmt.Println()
			fmt.Println("options:")
			fmt.Println("  -h, --help       show this help message and exit")
			fmt.Println("  --reason REASON")
			fmt.Println(webhelp.SuppressionNote)
			webhelp.PrintIfAvailable(os.Stdout, "repoman-088-provenance", argv)
			return 0
		}
	}
	if len(argv) < 3 || argv[1] != "--reason" {
		fmt.Fprintln(os.Stderr, "Usage: repoman provenance sanction FILE --reason REASON")
		return 1
	}
	path := argv[0]
	reason := argv[2]
	if err := ed.Sanction(path, reason); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}
	fmt.Printf("SANCTIONED: %s -- %s\n", path, reason)
	return 0
}
