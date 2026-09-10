package selftest

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"strings"
)

// runSection27 regression-tests T-01: detecting a journal-tracked file
// edited OUTSIDE repoman (raw str_replace, sed, hand-editing, or a
// different editing tool entirely -- the real incident this closes,
// caught only by chance mid-session by a human reviewing a diff, not
// by any mechanism until now).
//
// Journal.FileProvenance records a sha256 of a file's full content at
// every repoman-mediated write -- ed's Record (shared by `ed` and
// `strreplace`) and ed's own undo path (revertTxn, which needed a
// separate fix: it doesn't create a new Txn, so it never went through
// Record on its own). `provenance check` recomputes and compares.
//
// Four scenarios, each verified against the real binary before being
// written here as a regression: clean state, a genuine out-of-band
// edit (the actual failure mode), a tracked file that no longer
// exists at all, and undo's own resync (the specific gap that
// required threading *Journal through revertTxn).
func runSection27(g *gate, root string) int {
	self := g.self

	provDir := root + "/section27-provenance"
	mustWrite(provDir+"/.repoman.json", `{"id_prefix":"T","id_separator":"-"}`)
	mustWrite(provDir+"/docs/TRACKING.md",
		"# Tracking\n\n"+
			"| ID | Summary | Theme | Priority | Status | Blocks |\n"+
			"|----|---------|-------|----------|--------|--------|\n\n")
	mustWrite(provDir+"/docs/RESOLVED.md", "# Resolved\n\nClosed items, newest first.\n")
	mustWrite(provDir+"/sample.txt", "hello world\n")

	// -- clean state: nothing tracked yet, check succeeds trivially --
	r := run(self, provDir, "provenance", "check")
	if !g.check(r.code == 0 && strings.Contains(r.stdout, "0 file(s) tracked"),
		"provenance: before any repoman-mediated write, check succeeds with nothing tracked",
		r.stdout+r.stderr) {
		return 1
	}

	// -- a real repoman write records provenance; check still passes --
	findR := run(self, provDir, "ed", "find", "hello world", "sample.txt")
	handle := strings.Fields(findR.stdout)[0]
	r = run(self, provDir, "ed", "apply", handle, "--with", "hello repoman")
	if !g.check(r.code == 0, "provenance: setup -- ed apply on sample.txt succeeds", r.stdout+r.stderr) {
		return 1
	}
	r = run(self, provDir, "provenance", "check")
	if !g.check(r.code == 0 && strings.Contains(r.stdout, "1 file(s) tracked"),
		"provenance: after a real repoman-mediated write, the file is tracked and check "+
			"still passes -- the hash repoman just recorded matches the content it just wrote",
		r.stdout+r.stderr) {
		return 1
	}

	// -- THE ACTUAL FAILURE MODE: a hand-edit bypassing repoman entirely
	// is caught as a mismatch, not silently missed.
	mustWrite(provDir+"/sample.txt", "tampered outside repoman\n")
	r = run(self, provDir, "provenance", "check")
	if !g.check(r.code != 0 && strings.Contains(r.stderr, "provenance-mismatch") &&
		strings.Contains(r.stderr, "sample.txt") && strings.Contains(r.stderr, "changed outside repoman"),
		"provenance: a file edited outside repoman since repoman's last write to it is "+
			"caught as a mismatch -- this is the actual incident T-01 closes: an edit that "+
			"bypasses find/apply's staleness check and the journal entirely, previously "+
			"caught only by chance, by a human reviewing a diff", r.stdout+r.stderr) {
		return 1
	}

	// -- T-31: ed apply itself now refuses to touch a mismatched file,
	// including via a fresh find/apply cycle that would otherwise look
	// like an unremarkable edit -- this is the real gap SpanHash alone
	// can't close, since a fresh handle taken on the TAMPERED content
	// matches that content just fine. The write must be blocked before
	// it lands, not just flagged afterward by a separate on-demand check.
	findR = run(self, provDir, "ed", "find", "tampered outside repoman", "sample.txt")
	handle = strings.Fields(findR.stdout)[0]
	r = run(self, provDir, "ed", "apply", handle, "--with", "should not land")
	if !g.check(r.code != 0 && strings.Contains(r.stderr, "was edited outside repoman"),
		"provenance/T-31: ed apply refuses to write on top of a mismatched file, even "+
			"with a handle freshly taken on its current (tampered) content -- SpanHash "+
			"alone would have let this through", r.stdout+r.stderr) {
		return 1
	}

	// -- T-02: sanction without --reason is refused; --reason is
	// mandatory, not optional, since this is a deliberate override of
	// a safety check and the audit trail is the entire point.
	r = run(self, provDir, "provenance", "sanction", "sample.txt")
	if !g.check(r.code != 0, "provenance/T-02: sanction without --reason is refused",
		r.stdout+r.stderr) {
		return 1
	}

	// -- T-02: sanction refuses a path with nothing to sanction (no
	// mismatch on record) rather than silently doing nothing.
	r = run(self, provDir, "provenance", "sanction", "sample.txt", "--reason", "")
	if !g.check(r.code != 0, "provenance/T-02: sanction with an empty --reason is refused "+
		"the same as a missing one", r.stdout+r.stderr) {
		return 1
	}

	// -- T-02: the real remedy -- sanction with a real reason re-syncs
	// the hash to CURRENT (still-tampered) content directly, without
	// requiring the edit to be redone.
	r = run(self, provDir, "provenance", "sanction", "sample.txt", "--reason", "accepted for this regression fixture")
	if !g.check(r.code == 0 && strings.Contains(r.stdout, "SANCTIONED"),
		"provenance/T-02: sanction with a real --reason succeeds and re-syncs the hash "+
			"to the file's current content", r.stdout+r.stderr) {
		return 1
	}
	r = run(self, provDir, "provenance", "check")
	if !g.check(r.code == 0, "provenance/T-02: check passes after sanction -- the mismatch "+
		"is cleared without redoing the edit", r.stdout+r.stderr) {
		return 1
	}

	// -- T-02: sanctioning again immediately (nothing left to sanction)
	// is refused, not silently accepted a second time.
	r = run(self, provDir, "provenance", "sanction", "sample.txt", "--reason", "trying again")
	if !g.check(r.code != 0, "provenance/T-02: sanctioning a path with no mismatch left is "+
		"refused, not a silent no-op", r.stdout+r.stderr) {
		return 1
	}

	// -- T-31/T-02 together: ed apply works normally again on the
	// sanctioned file -- sanction actually un-refuses the write path
	// it was blocking, closing the loop this whole scenario opened.
	findR = run(self, provDir, "ed", "find", "tampered outside repoman", "sample.txt")
	handle = strings.Fields(findR.stdout)[0]
	r = run(self, provDir, "ed", "apply", handle, "--with", "back to normal after sanction")
	if !g.check(r.code == 0, "provenance/T-31+T-02: ed apply works normally again on the "+
		"sanctioned file -- sanction actually un-refuses the write path it was blocking",
		r.stdout+r.stderr) {
		return 1
	}
	r = run(self, provDir, "provenance", "check")
	if !g.check(r.code == 0, "provenance/T-31+T-02: check still passes after that further "+
		"real edit -- the normal write path re-syncs the hash on its own, same as always",
		r.stdout+r.stderr) {
		return 1
	}

	// -- undo's own provenance resync: this is the specific gap that
	// required threading *Journal through revertTxn, since undo
	// removes a Txn rather than adding one and so never went through
	// ed.Record on its own.
	undoDir := root + "/section27-undo"
	mustWrite(undoDir+"/.repoman.json", `{"id_prefix":"T","id_separator":"-"}`)
	mustWrite(undoDir+"/docs/TRACKING.md",
		"# Tracking\n\n"+
			"| ID | Summary | Theme | Priority | Status | Blocks |\n"+
			"|----|---------|-------|----------|--------|--------|\n\n")
	mustWrite(undoDir+"/docs/RESOLVED.md", "# Resolved\n\nClosed items, newest first.\n")
	mustWrite(undoDir+"/f.txt", "version one\n")

	r = run(self, undoDir, "ed", "mark", "A")
	if !g.check(r.code == 0, "provenance/undo: setup -- mark A", r.stdout+r.stderr) {
		return 1
	}
	findR = run(self, undoDir, "ed", "find", "version one", "f.txt")
	handle = strings.Fields(findR.stdout)[0]
	r = run(self, undoDir, "ed", "apply", handle, "--with", "version two")
	if !g.check(r.code == 0, "provenance/undo: setup -- ed apply f.txt", r.stdout+r.stderr) {
		return 1
	}
	r = run(self, undoDir, "provenance", "check")
	if !g.check(r.code == 0, "provenance/undo: check passes right after the apply", r.stdout+r.stderr) {
		return 1
	}
	r = run(self, undoDir, "ed", "undo", "--since", "A")
	if !g.check(r.code == 0, "provenance/undo: undo --since A succeeds", r.stdout+r.stderr) {
		return 1
	}
	r = run(self, undoDir, "provenance", "check")
	if !g.check(r.code == 0 && strings.Contains(r.stdout, "1 file(s) tracked"),
		"provenance/undo: after undo reverts f.txt to its pre-edit content, check still "+
			"passes -- undo re-synced the recorded hash to match the reverted content via "+
			"the same recordProvenance helper Record uses, not left stale pointing at the "+
			"now-undone version", r.stdout+r.stderr) {
		return 1
	}

	// -- missing file: a tracked file deleted outside repoman is
	// reported distinctly (Missing), not confused with a content
	// mismatch or silently ignored.
	missingDir := root + "/section27-missing"
	mustWrite(missingDir+"/.repoman.json", `{"id_prefix":"T","id_separator":"-"}`)
	mustWrite(missingDir+"/docs/TRACKING.md",
		"# Tracking\n\n"+
			"| ID | Summary | Theme | Priority | Status | Blocks |\n"+
			"|----|---------|-------|----------|--------|--------|\n\n")
	mustWrite(missingDir+"/docs/RESOLVED.md", "# Resolved\n\nClosed items, newest first.\n")
	mustWrite(missingDir+"/gone.txt", "will be deleted\n")

	findR = run(self, missingDir, "ed", "find", "will be deleted", "gone.txt")
	handle = strings.Fields(findR.stdout)[0]
	r = run(self, missingDir, "ed", "apply", handle, "--with", "tracked before deletion")
	if !g.check(r.code == 0, "provenance/missing: setup -- ed apply gone.txt", r.stdout+r.stderr) {
		return 1
	}
	if err := os.Remove(missingDir + "/gone.txt"); err != nil {
		if !g.check(false, "provenance/missing: setup -- delete gone.txt outside repoman", err.Error()) {
			return 1
		}
	}
	r = run(self, missingDir, "provenance", "check")
	if !g.check(r.code != 0 && strings.Contains(r.stderr, "provenance-missing") &&
		strings.Contains(r.stderr, "gone.txt") && strings.Contains(r.stderr, "no longer exists"),
		"provenance/missing: a tracked file deleted outside repoman is reported as Missing, "+
			"distinctly from a content mismatch, rather than silently skipped or conflated",
		r.stdout+r.stderr) {
		return 1
	}

	// -- T-31: ed sub also refuses to touch a mismatched file (its
	// biggest gap before this: --expect only ever guarded occurrence
	// count, never whether the file changed since anything was
	// recorded, so sub had no staleness protection at all until now).
	subDir := root + "/section27-sub"
	mustWrite(subDir+"/.repoman.json", `{"id_prefix":"T","id_separator":"-"}`)
	mustWrite(subDir+"/docs/TRACKING.md",
		"# Tracking\n\n"+
			"| ID | Summary | Theme | Priority | Status | Blocks |\n"+
			"|----|---------|-------|----------|--------|--------|\n\n")
	mustWrite(subDir+"/docs/RESOLVED.md", "# Resolved\n\nClosed items, newest first.\n")
	mustWrite(subDir+"/sub.txt", "alpha beta\n")

	r = run(self, subDir, "ed", "sub", "alpha", "gamma", "sub.txt", "--expect", "1")
	if !g.check(r.code == 0, "provenance/T-31 sub: setup -- initial sub.txt substitution", r.stdout+r.stderr) {
		return 1
	}
	mustWrite(subDir+"/sub.txt", "tampered sub content\n")
	r = run(self, subDir, "ed", "sub", "tampered", "xyz", "sub.txt", "--expect", "1")
	if !g.check(r.code != 0 && strings.Contains(r.stderr, "was edited outside repoman"),
		"provenance/T-31 sub: ed sub refuses to touch a file that changed outside repoman, "+
			"closing the gap --expect's occurrence-count check never covered", r.stdout+r.stderr) {
		return 1
	}
	r = run(self, subDir, "provenance", "sanction", "sub.txt", "--reason", "regression fixture")
	if !g.check(r.code == 0, "provenance/T-31 sub: sanction unblocks it", r.stdout+r.stderr) {
		return 1
	}
	r = run(self, subDir, "ed", "sub", "tampered", "xyz", "sub.txt", "--expect", "1")
	if !g.check(r.code == 0, "provenance/T-31 sub: ed sub works normally again after sanction",
		r.stdout+r.stderr) {
		return 1
	}

	// -- T-31: ed append/prepend refuse too, even though they target a
	// true file boundary rather than matched text and so have no
	// SpanHash-style check of their own to fall back on.
	appDir := root + "/section27-append"
	mustWrite(appDir+"/.repoman.json", `{"id_prefix":"T","id_separator":"-"}`)
	mustWrite(appDir+"/docs/TRACKING.md",
		"# Tracking\n\n"+
			"| ID | Summary | Theme | Priority | Status | Blocks |\n"+
			"|----|---------|-------|----------|--------|--------|\n\n")
	mustWrite(appDir+"/docs/RESOLVED.md", "# Resolved\n\nClosed items, newest first.\n")
	mustWrite(appDir+"/app.txt", "line one\n")

	r = run(self, appDir, "ed", "append", "app.txt", "--with", "line two\n")
	if !g.check(r.code == 0, "provenance/T-31 append: setup -- initial append", r.stdout+r.stderr) {
		return 1
	}
	mustWrite(appDir+"/app.txt", "tampered append content\n")
	r = run(self, appDir, "ed", "append", "app.txt", "--with", "should not land\n")
	if !g.check(r.code != 0 && strings.Contains(r.stderr, "was edited outside repoman"),
		"provenance/T-31 append: ed append refuses to touch a file that changed outside "+
			"repoman, even though append has no SpanHash-style check of its own to fall "+
			"back on -- boundary edits were the biggest blind spot before this", r.stdout+r.stderr) {
		return 1
	}
	r = run(self, appDir, "provenance", "sanction", "app.txt", "--reason", "regression fixture")
	if !g.check(r.code == 0, "provenance/T-31 append: sanction unblocks it", r.stdout+r.stderr) {
		return 1
	}
	r = run(self, appDir, "ed", "append", "app.txt", "--with", "line two\n")
	if !g.check(r.code == 0, "provenance/T-31 append: ed append works normally again after "+
		"sanction", r.stdout+r.stderr) {
		return 1
	}

	// -- T-31: strreplace's own write path (readFileUTF8, the single
	// funnel every strreplace op reads through) refuses the same way.
	srDir := root + "/section27-strreplace"
	mustWrite(srDir+"/.repoman.json", `{"id_prefix":"T","id_separator":"-"}`)
	mustWrite(srDir+"/docs/TRACKING.md",
		"# Tracking\n\n"+
			"| ID | Summary | Theme | Priority | Status | Blocks |\n"+
			"|----|---------|-------|----------|--------|--------|\n\n")
	mustWrite(srDir+"/docs/RESOLVED.md", "# Resolved\n\nClosed items, newest first.\n")
	mustWrite(srDir+"/sr.txt", "hello strreplace\n")

	srPayload := func(search, replace string) string {
		b, _ := json.Marshal(map[string]interface{}{
			"v":      1,
			"atomic": false, // sr.txt has no registered syntax validator; not what this scenario is testing
			"ops": []interface{}{
				map[string]interface{}{
					"file":        "sr.txt",
					"search_b64":  base64.StdEncoding.EncodeToString([]byte(search)),
					"replace_b64": base64.StdEncoding.EncodeToString([]byte(replace)),
					"expect":      1,
					"roles":       []string{"text"},
				},
			},
		})
		return string(b)
	}

	r = runWithStdin(self, srDir, srPayload("hello strreplace\n", "hi strreplace\n"), "strreplace", "apply", "-")
	if !g.check(r.code == 0, "provenance/T-31 strreplace: setup -- initial substitution", r.stdout+r.stderr) {
		return 1
	}
	mustWrite(srDir+"/sr.txt", "tampered strreplace content\n")
	r = runWithStdin(self, srDir, srPayload("tampered strreplace content\n", "should not land\n"), "strreplace", "apply", "-")
	if !g.check(r.code != 0 && strings.Contains(r.stdout+r.stderr, "was edited outside repoman"),
		"provenance/T-31 strreplace: strreplace refuses to touch a file that changed outside "+
			"repoman, via its single readFileUTF8 read funnel", r.stdout+r.stderr) {
		return 1
	}
	r = run(self, srDir, "provenance", "sanction", "sr.txt", "--reason", "regression fixture")
	if !g.check(r.code == 0, "provenance/T-31 strreplace: sanction unblocks it", r.stdout+r.stderr) {
		return 1
	}
	r = runWithStdin(self, srDir, srPayload("tampered strreplace content\n", "back to normal\n"), "strreplace", "apply", "-")
	if !g.check(r.code == 0, "provenance/T-31 strreplace: strreplace works normally again "+
		"after sanction", r.stdout+r.stderr) {
		return 1
	}

	return runSection28(g, root)
}
