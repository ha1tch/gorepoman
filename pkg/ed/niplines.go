package ed

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// T-23/T-24: the ticketed niplines two-phase flow's three CLI verbs --
// niplines (phase 1, request/preview), confirm and cancel (phase 2).
// Kept in their own file rather than growing ed.go further; each
// function here is dispatched from a thin case in ed.go's Run, the
// same shape every other verb (apply, insert, sub, ...) already
// follows, so niplines/confirm/cancel are wired in consistently with
// the rest of this package rather than as a special case.
//
// See docs/proposals/ed-insert-and-ticketed-niplines.md, Proposal 2,
// for the full design rationale; T-22 (pkg/ed/tickets.go) built the
// storage primitive both phases share, and preflight.go/nippreview.go
// built the two pieces of the phase-1 readback (is-it-still-valid,
// what-actually-changed) this file composes together.

func runNiplines(args []string) int {
	for _, a := range args[1:] {
		if a == "-h" || a == "--help" {
			fmt.Println("usage: repoman ed niplines [-h] [--ttl DURATION] FILE START END")
			fmt.Println()
			fmt.Println("positional arguments:")
			fmt.Println("  FILE")
			fmt.Println("  START  first line to remove, 1-based, inclusive")
			fmt.Println("  END    last line to remove, 1-based, inclusive")
			fmt.Println()
			fmt.Println("options:")
			fmt.Println("  -h, --help       show this help message and exit")
			fmt.Println("  --ttl DURATION   how long the ticket stays confirmable, as a Go")
			fmt.Println("                   duration (e.g. 20m, 1h). Default 10m; 1h ceiling --")
			fmt.Println("                   a nip needing longer review belongs in TRACKING.md")
			fmt.Println("                   as its own item, not a longer-lived ticket.")
			fmt.Println()
			fmt.Println("Phase 1 of the ticketed niplines flow: removes a LINE RANGE, which")
			fmt.Println("(unlike find/apply/sub) has no content anchor to hash-verify -- there is")
			fmt.Println("no substring being matched, only a position. Writes nothing to FILE.")
			fmt.Println("Prints a diff-style preview of the removal, a gofmt/vet preflight")
			fmt.Println("against the previewed result (a recognized source type only; advisory,")
			fmt.Println("never a refusal), and a ticket ID. Redeem the ticket with")
			fmt.Println("`repoman ed confirm <id>` to actually write, or `repoman ed cancel <id>`")
			fmt.Println("to discard it with no write, ever. See")
			fmt.Println("https://ha1tch.github.io/gorepoman/docs/repoman-040-editing.html")
			fmt.Println("for the full request/preview/confirm worked example.")
			return 0
		}
	}
	var ttl time.Duration
	haveTTL := false
	var positional []string
	for i := 1; i < len(args); i++ {
		if args[i] == "--ttl" {
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "--ttl requires a duration (e.g. 20m, 1h)")
				return 1
			}
			d, err := time.ParseDuration(args[i+1])
			if err != nil {
				fmt.Fprintf(os.Stderr, "REFUSED: --ttl %q is not a valid duration: %v\n", args[i+1], err)
				return 1
			}
			ttl = d
			haveTTL = true
			i++
			continue
		}
		positional = append(positional, args[i])
	}
	if len(positional) != 3 {
		fmt.Fprintln(os.Stderr, "Usage: repoman ed niplines <file> <start-line> <end-line> [--ttl DURATION]")
		return 1
	}
	path := positional[0]
	startLine, errS := strconv.Atoi(positional[1])
	endLine, errE := strconv.Atoi(positional[2])
	if errS != nil || errE != nil {
		fmt.Fprintln(os.Stderr, "REFUSED: START and END must be integers")
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
	original := string(b)

	postNip, preview, err := computeNip(original, startLine, endLine)
	if err != nil {
		fmt.Fprintf(os.Stderr, "REFUSED: %v\n", err)
		return 1
	}

	pf := Preflight(path, postNip)

	if !haveTTL {
		ttl = DefaultTicketTTL
	}
	store := LoadTicketStore()
	// Opportunistic cleanup: nothing in this CLI tool runs in the
	// background, so an abandoned ticket only gets removed when some
	// later niplines call happens to sweep for it -- same "nothing
	// lingers unbounded" property Journal's own MaxTxns/MaxBytes
	// eviction gives completed edits (tickets.go's own PruneExpired
	// doc comment names this call site as where it's expected to run).
	if n := PruneExpired(&store, time.Now().UTC()); n > 0 {
		_ = SaveTicketStore(&store)
	}
	preNipHash := HashContent(original)
	previewHash := HashContent(preview)
	t, err := IssueTicket(&store, path, startLine, endLine, preNipHash, previewHash, ttl)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}

	fmt.Print(preview)
	fmt.Println()
	if !pf.Available {
		fmt.Printf("preflight: not available for %s\n", filepath.Ext(path))
	} else if pf.Clean {
		fmt.Println("preflight: clean (gofmt, go vet)")
	} else {
		fmt.Println("preflight WARNING (advisory -- confirm anyway if this is expected):")
		for _, w := range pf.Warnings {
			fmt.Println("  " + w)
		}
	}
	fmt.Println()
	fmt.Printf("ticket: %s  (expires %s)\n", t.ID, t.ExpiresAt)
	fmt.Println("nothing written. `repoman ed confirm " + t.ID + "` to apply, `repoman ed cancel " + t.ID + "` to discard.")
	return 0
}

func runConfirm(args []string) int {
	for _, a := range args[1:] {
		if a == "-h" || a == "--help" {
			fmt.Println("usage: repoman ed confirm [-h] TICKET-ID")
			fmt.Println()
			fmt.Println("positional arguments:")
			fmt.Println("  TICKET-ID")
			fmt.Println()
			fmt.Println("options:")
			fmt.Println("  -h, --help  show this help message and exit")
			fmt.Println()
			fmt.Println("Phase 2 of the ticketed niplines flow. Re-hashes the live file against")
			fmt.Println("the hash recorded when the ticket was issued and refuses -- the same way")
			fmt.Println("apply refuses a stale handle -- if the file drifted since request time:")
			fmt.Println("the preview that was approved is no longer the preview this would apply.")
			fmt.Println("On a match, performs the removal in one atomic write and records it in")
			fmt.Println("the journal exactly like apply does, so undo works on it identically to")
			fmt.Println("any other edit. An unknown or expired ticket refuses with no write.")
			return 0
		}
	}
	if len(args) != 2 {
		fmt.Fprintln(os.Stderr, "Usage: repoman ed confirm <ticket-id>")
		return 1
	}
	id := args[1]

	store := LoadTicketStore()
	t, found := GetTicket(store, id)
	if !found {
		fmt.Fprintf(os.Stderr, "REFUSED: no pending ticket %s (unknown, already confirmed/cancelled, or pruned)\n", id)
		return 1
	}
	if t.IsExpired(time.Now().UTC()) {
		_ = DiscardTicket(&store, id)
		fmt.Fprintf(os.Stderr, "REFUSED: ticket %s expired at %s -- re-run niplines to request a fresh one. Nothing written.\n", id, t.ExpiresAt)
		return 1
	}

	b, err := os.ReadFile(t.File)
	if err != nil {
		fmt.Fprintf(os.Stderr, "REFUSED: %s no longer exists\n", t.File)
		return 1
	}
	j := LoadJournal()
	if st := CheckProvenance(j, t.File); st.Mismatch || st.Missing {
		fmt.Fprintf(os.Stderr, "REFUSED: %s was edited outside repoman since %s -- run `repoman provenance check` for details. Nothing written.\n", t.File, st.RecordedAt)
		return 1
	}
	original := string(b)
	if HashContent(original) != t.PreNipHash {
		fmt.Fprintf(os.Stderr, "REFUSED: %s changed since niplines was requested (stale ticket) -- the previewed diff no longer matches what confirm would apply. Re-run niplines for a fresh preview and ticket. Nothing written.\n", t.File)
		return 1
	}

	s, e, err := nipByteSpan(original, t.StartLine, t.EndLine)
	if err != nil {
		fmt.Fprintf(os.Stderr, "REFUSED: %v\n", err)
		return 1
	}
	removed := original[s:e]
	newText := original[:s] + original[e:]
	if err := os.WriteFile(t.File, []byte(newText), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "REFUSED: could not write %s: %v\n", t.File, err)
		return 1
	}

	Record(&j, []Edit{{File: t.File, Offset: s, Old: removed, New: ""}}, "niplines "+filepath.Base(t.File))
	_ = DiscardTicket(&store, id)

	fmt.Printf("confirmed %s: removed lines %d-%d from %s\n", id, t.StartLine, t.EndLine, t.File)
	return 0
}

func runCancel(args []string) int {
	for _, a := range args[1:] {
		if a == "-h" || a == "--help" {
			fmt.Println("usage: repoman ed cancel [-h] TICKET-ID")
			fmt.Println()
			fmt.Println("positional arguments:")
			fmt.Println("  TICKET-ID")
			fmt.Println()
			fmt.Println("options:")
			fmt.Println("  -h, --help  show this help message and exit")
			fmt.Println()
			fmt.Println("Discards a pending niplines ticket with no write, ever. Cancelling an")
			fmt.Println("unknown, already-cancelled, or already-expired ticket is a no-op, not")
			fmt.Println("an error -- cancel is idempotent by design.")
			return 0
		}
	}
	if len(args) != 2 {
		fmt.Fprintln(os.Stderr, "Usage: repoman ed cancel <ticket-id>")
		return 1
	}
	id := args[1]
	store := LoadTicketStore()
	_, found := GetTicket(store, id)
	_ = DiscardTicket(&store, id)
	if found {
		fmt.Printf("cancelled %s. Nothing written.\n", id)
	} else {
		fmt.Printf("%s was already gone (unknown, confirmed, cancelled, or expired). Nothing written.\n", id)
	}
	return 0
}
