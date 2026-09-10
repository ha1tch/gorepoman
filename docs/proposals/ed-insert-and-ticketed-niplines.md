# Proposal: `ed insert`/`append`/`prepend`, and a ticketed `ed niplines`

Updated: 2026-09-08

## Origin

Filed after a real editing session on this repository (the
`cards-and-guards.md` essay and its two cross-references). Every edit made
in that session was a genuine span replacement and went through
`find`/`apply` correctly, but two of the early `apply` calls corrupted
their target files by letting the `--with` replacement text re-include
part of the matched anchor instead of cleanly replacing it — caught on
reread, undone via `ed undo`, and redone correctly. Both incidents were
avoidable if the primitive available had been "insert relative to this
verified position" rather than "replace this verified span with text that
must itself restate any part of the span you want to keep."

Separately, during design discussion, a related but distinct need came up:
deleting a line range has no natural content anchor the way a targeted
substitution does, and a bare `niplines(start, end)` by line number alone
would reintroduce exactly the class of risk `find`/`apply` was built to
eliminate — a hand-typed, unverified anchor. The two problems are related
(both are about widening `ed`'s vocabulary beyond span-replacement) but
call for different solutions, covered as two proposals below.

## Current state (verified against `pkg/ed/ed.go`)

- `find` returns a handle: `file:start-end:hash`, where `hash` is
  `SpanHash` — a SHA-256 over a 64-byte context window around the span,
  truncated to 8 hex chars.
- `apply <handle> --with TEXT` re-verifies `SpanHash` against the live
  file before writing, refuses on mismatch ("stale handle"), and on
  success replaces `text[s:e]` with `TEXT` in one atomic write, recording
  an `Edit{File, Offset, Old, New}` in the journal for `undo`.
- Every verb in `ed` — `find`, `apply`, `sub`, `undo`, `mark`, `log` — is
  byte-offset-and-content-hash keyed. `Occurrence.LineNo` exists only for
  human-readable display in `find` output; nothing in `ed` addresses an
  edit by line number today.
- There is no insertion primitive. An insertion is currently faked as a
  span replacement where the replacement text starts or ends with a copy
  of the matched anchor plus new content — which is precisely the pattern
  that produced the duplication bug above.

## Proposal 1: `insert`, `append`, `prepend`

Three verbs, two safety tiers, reusing `apply`'s existing verification
machinery rather than adding new machinery:

### `append FILE --with TEXT` / `prepend FILE --with TEXT`

Unconditionally safe, no handle required, because there is no existing
text being trusted — only a file boundary (true EOF, true offset 0).
Equivalent to `apply` at a zero-width span at `len(text)` or `0`, but
exposed as a dedicated verb so the intent is explicit in the journal and
in the command itself, rather than encoded as an awkward zero-width
`apply`.

```
$ repoman ed append README.md --with "

## New section
..."
appended to README.md (14 bytes)
```

### `insert <handle> --after|--before --with TEXT`

Requires a `find`-verified handle exactly like `apply` does, and re-runs
the same `SpanHash` check before writing. The only change from `apply` is
which boundary of the span the text lands on:

- `--after`: `text[:e] + TEXT + text[e:]`
- `--before`: `text[:s] + TEXT + text[s:]`

Neither variant touches `text[s:e]` itself — the matched span is left
completely alone, which is the point: this verb cannot reproduce the
duplication bug, because there is no replacement string that could
possibly restate the anchor. The anchor is never in the string you type.

```
$ repoman ed find "cross-referenced to the tool that reads it" docs/repoman-000-index.md --brief
docs/repoman-000-index.md:2796-2838:eae6ff05  [md-table]  line 28: ...

$ repoman ed insert docs/repoman-000-index.md:2796-2838:eae6ff05 --after --with '
| — | [New page](new-page.md) | Description |'
inserted after docs/repoman-000-index.md:2838
```

Implementation cost is small: a new `case "insert":` branch in
`ed.Run()` that parses `--after`/`--before` (mutually exclusive, one
required), reuses `parseHandle`, `SpanHash`, and `Record` verbatim, and
differs from the existing `apply` branch only in which of `s`/`e` the
replacement is spliced at.

## Proposal 2: ticketed `niplines`

A line-range delete is different in kind from everything else `ed` does:
there is no substring to hash-verify, because the operation is defined by
position, not content. Two options were considered and rejected before
landing on the ticket flow below:

- **Bare `niplines(start, end)` by line number.** Rejected: this is a
  blind line-range operation with no content check at all — worse than
  `sed -i 'N,Md'` in one respect, because a `repoman` verb that looks
  like it belongs to the verified-handle family but silently isn't
  inherits unearned trust.
- **`niplines` anchored to two `find`-verified handles** (first line and
  last line of the range, each hash-checked). Safer than bare line
  numbers, but still commits in one shot — it does not address the
  separate problem that nothing today forces a look at the *result*
  before it lands, only at the *starting point*.

The proposed design instead makes `niplines` inherently two-phase, so the
preview and the eventual write are the same request, not two independently
issued commands a caller could mismatch:

### Phase 1 — request

```
$ repoman ed niplines cards-and-guards.md 10 20
```

Repoman does not write anything. It computes the post-nip file, and
prints:

- A diff-style preview: the lines being removed, clearly marked, with a
  few lines of context on each side — not just "here is the whole file
  now," which makes the caller re-derive what changed by eye instead of
  seeing it directly.
- For a recognized source file type, a preflight check run against the
  *previewed* content on a scratch copy, never against the live file:
  `gofmt -l` and `go vet` for `.go`, or the repository's configured
  linter for other types. This is surfaced as a warning in the ticket
  readback, not a hard refusal — the caller decides whether a
  gofmt/vet complaint means "cancel" or "confirm anyway, I'll fix it
  next." Full `go build`/`go test` is a separate opt-in
  (`--verify-build`), not automatic, since it is a real time cost the
  request step should not impose unconditionally.
- A ticket: an opaque ID, plus the hash of the pre-nip file and the hash
  of the exact preview text shown. Both hashes matter — the second is
  what makes confirm prove the caller saw *this* diff, not just that the
  file was unchanged.
- An expiry timestamp (see TTL below).

Nothing is written to `cards-and-guards.md`. The ticket is the only
record of the pending request, held server-side (i.e., by `repoman`,
typically in `.ed-journal.json` or a sibling pending-tickets file) —
not reconstructable from a caller-supplied line range alone, which is
what forces confirm to redeem the *same* request rather than a
caller's fresh restatement of it.

### Phase 2 — confirm or cancel

```
$ repoman ed confirm <ticket-id>
$ repoman ed cancel <ticket-id>
```

`confirm` re-hashes the live file and refuses, the same way `apply`
refuses a stale handle, if it no longer matches the hash recorded at
request time — the file drifted underneath the pending ticket, so the
preview that was approved is no longer the preview that would apply. On
a hash match, `confirm` performs the nip in one atomic write and records
it in the journal exactly like `apply` does, so `undo` works on it
identically to any other edit.

`cancel` (or simple expiry) discards the ticket with no write, ever.

### TTL

The wait exists to force an actual look at the result before it commits
— gofmt/vet, a build, or a human reading the diff — so the TTL should be
sized to who is expected to be doing that looking, not set to one
generic default:

| Confirming party | Default TTL | Rationale |
|---|---|---|
| An agent re-reading its own tool output in the same turn | 10 minutes | Long enough for a gofmt/vet/build preflight to run without racing the clock; short enough that a forgotten ticket doesn't linger as an unexplained pending mutation. |
| A human deciding after actually reading the diff | 1 hour (explicit `--ttl`, capped) | Honest about how human review actually happens — a person context-switches and comes back; a short TTL here just trains people to rush the read or re-issue tickets with no added safety. |
| Anything longer | Not supported as a ticket | A nip that needs more than an hour to get reviewed is a `TRACKING.md` item (Part 3 of the working-agreement doc taxonomy), re-issued as a fresh ticket when someone is ready — not a long-lived ticket competing with the register as a second, informal tracking surface. |

A long TTL is not a correctness risk, because `confirm` re-verifies the
file hash regardless of how much time has passed — it only affects how
often a stale ticket has to be re-issued, which is friction, not a safety
gap. The ceiling matters for a different reason: keeping tickets
ephemeral by policy, so genuinely long-pending work stays in the one
register rather than accumulating as an ambient parallel to-do list.

## Scope note

Neither proposal touches `apply`, `sub`, `find`, `mark`, or `undo`. The
ticket flow is deliberately not proposed as the default for those —
`apply`'s existing immediate hash-verified write is already the right
tradeoff for a precisely targeted, verified-anchor edit, and adding
ceremony there would slow down the common case for no safety gain the
hash check doesn't already provide. Two-phase confirmation earns its
overhead specifically where the operation has no natural content anchor
and its blast radius isn't obvious from the command alone — which today
means `niplines`, and nothing else.

## Open questions for whoever implements this

- Where do pending tickets live — a new `.ed-tickets.json` sibling to
  `.ed-journal.json`, or a section of the journal itself? A separate file
  keeps `MaxTxns`/`MaxBytes` eviction logic (journal.go's `SaveJournal`)
  from having to reason about pending, unconfirmed state.
- Does `insert --after`/`--before` need its own entry in
  `repoman-040-editing.md`'s "which one, when" table, or is it folded
  into the existing `apply` section as a variant?
- Should `niplines`'s preflight check be pluggable per-language (the way
  `roles` already classifies per-language), or hardcoded to `gofmt`/`vet`
  for Go with a generic "no preflight available" fallback for everything
  else, at least for a first cut?
