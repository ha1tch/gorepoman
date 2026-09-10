# Provenance: detecting (and sanctioning) edits made outside repoman

`ed` and `strreplace` verify a matched span hasn't gone stale before
writing to it, and journal every write so `undo` can restore it exactly.
Neither of those protects a file from being touched by something *else*
entirely — a raw `str_replace`, `sed`, a hand-edit, or an agent reaching
for a generic editing tool instead of repoman's own. That gap was closed
by chance exactly once: mid-session, a file already under repoman's
journal was edited via a different tool entirely, and the only reason it
didn't ship broken was a human happening to review the diff afterward.
Provenance is the mechanism that catches it reliably, every time, instead
of by luck.

## The mechanism

Every repoman-mediated write to a journal-tracked file — `ed apply`,
`sub`, `append`, `prepend`, `insert`, `undo`, and `strreplace`'s own
substitution — records the sha256 of that file's full content at the
moment of the write, in `.ed-journal.json`'s `file_provenance` map
(`path -> {hash, at}`). This isn't tied to any one transaction: a
transaction can be evicted from the journal's rolling window while the
file it touched is still current and still worth checking, and `undo`
(which removes a transaction rather than adding one) still needs
somewhere to record the hash of the content it just restored.

`repoman provenance check` recomputes each tracked file's *current*
hash and compares it against what's recorded. A mismatch means the file
changed since repoman last touched it — there is no other way for the
hash to diverge:

```
$ repoman provenance check
PROVENANCE CHECK OK (14 file(s) tracked)

$ vim docs/TRACKING.md   # a hand-edit, outside repoman entirely

$ repoman provenance check
ERROR provenance-mismatch: docs/TRACKING.md changed outside repoman since 2026-09-09T02:54:51Z -- recorded 4bad60b1b60c..., now 26ee0dd9f7c5...
PROVENANCE CHECK FAIL: 1 mismatch(es)
```

A tracked file that no longer exists at all is reported distinctly, not
folded into the same message:

```
ERROR provenance-missing: gone.txt no longer exists (last known-good hash recorded 2026-09-09T02:54:51Z)
```

v1 scope, deliberately: journal-tracked files only, no git integration.
A file repoman has never written to has no provenance recorded and is
never a reason to refuse anything — this check has nothing to compare
it against.

## Every write path refuses to build on a mismatch

`check` alone only catches drift *after the fact*, on demand. The write
paths themselves — `ed apply`, `insert`, `sub`, `append`/`prepend`,
`undo`, and `strreplace`'s single read funnel — now refuse to touch a
file with a live provenance mismatch, before anything is written:

```
$ repoman ed apply sample.txt:0-9:92e78d0b --with "should not land"
REFUSED: sample.txt was edited outside repoman since 2026-09-09T02:54:51Z -- run `repoman provenance check` for details. Nothing written.
```

This closes a gap `apply`'s own `SpanHash` staleness check structurally
cannot: `SpanHash` verifies the matched span's own 64-byte context
window hasn't moved since `find` — and a fresh `find`/`apply` handle
taken on already-tampered content matches that tampered content just
fine, so `SpanHash` never fires. Provenance checks against the last
*repoman-recorded* state instead, which is exactly what a freshly-taken
handle can't see. Two write paths had no staleness protection at all
before this: `sub`'s `--expect N` only ever guarded occurrence count,
never whether the file had changed since anything was recorded; and
`append`/`prepend` have no matched-span check by design, since they
target a true file boundary rather than existing text.

## Sanction: the deliberate override

Gating every write path on provenance closed the only documented
recovery path along with the mismatch itself — redoing the same edit
through repoman used to re-sync the hash, but that redo is itself a
write, so it was being refused by the very mechanism it was meant to
satisfy. `repoman provenance sanction <file> --reason "..."` is the
way out: it re-syncs the recorded hash to the file's *current*
on-disk content directly, without requiring the edit to be redone.

```
$ repoman provenance sanction docs/TRACKING.md --reason "manual fix during an incident drill, reviewed and accepted"
SANCTIONED: docs/TRACKING.md -- accepted for this regression fixture

$ repoman provenance check
PROVENANCE CHECK OK (14 file(s) tracked)

$ repoman ed apply docs/TRACKING.md:0-40:a1b2c3d4 --with "normal editing again"
applied at docs/TRACKING.md:0: "..." -> "normal editing again"
```

`--reason` is mandatory — this is a deliberate override of a safety
check, and the audit trail is the entire point, not an optional
courtesy. `sanction` refuses in two other cases, both distinctly
worded: a path with no mismatch currently on record (nothing to
sanction — not a silent no-op), and a path that no longer exists
(there's no current content to certify; restore the file first, or
address its absence some other way).

## What's still v1-scoped

No git integration — provenance only knows what repoman itself wrote,
not what a `git checkout` or a merge landed. No sanction/override
beyond re-syncing the hash — there's no separate audit log of past
sanctions beyond the single `reason` recorded alongside the current
hash. `relcore` wiring is done (T-03): `relcore` now runs a
provenance pre-flight unconditionally on every release, immediately
alongside `badcode`, before any release.steps entry, including on
`--resume` — an unsanctioned out-of-band edit blocks the release the
same way a `badcode` hit does, with no bypass beyond sanctioning it
first. See `repoman relcore --help` and this project's own
docs/TRACKING.md (T-03) for the details. Badcode hits themselves also
carry a provenance annotation now (T-04): each match reports whether
the file it was found in currently matches repoman's own journal
record, has never been touched by repoman at all, or was tracked once
and has since diverged — a distinction no adjacent secret-scanning
tool can make, since none of them own the editing layer.
