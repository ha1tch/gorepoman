# Resolved items

Closed items, newest first.

## [0.16.0] T-25 — docs: repoman-040-editing.md coverage for insert/append/prepend/niplines (v0.16.0, 2026-09-11)

Theme: ed-editing · closed 0.16.0 · 2026-09-11
Update docs/repoman-040-editing.md ("Precise editing: ed and
strreplace") to document the new verbs once they exist: append and
prepend as unconditionally-safe boundary operations; insert
--after/--before as a handle-verified variant of apply, with a
worked example showing it does not risk the anchor-duplication
failure apply's --with can; and niplines request/confirm/cancel as
a distinct two-phase flow, including the TTL table and why it is
not the default flow for apply/sub/find (existing hash verification
already covers that case; niplines earns extra ceremony because a
line range has no natural content anchor). Also resolves the open
question in the proposal doc about whether insert belongs in its
own subsection of the "which one, when" table or folds into the
existing apply section.

Blocked by T-20, T-21, T-23, T-24 (documents behavior, not design --
written after the verbs exist, not before).
See docs/proposals/ed-insert-and-ticketed-niplines.md.

Progress: append/prepend/insert are documented in
repoman-040-editing.md, with real verified worked examples (T-20,
T-21 now shipped, in v0.14.6). The niplines portion of this item's
filed scope remains genuinely blocked -- T-23 and T-24 do not exist
yet, so there is no request/confirm/cancel flow or TTL behavior to
document. Status stays not-started since the item's own scope is
not yet fully met; do not close on the partial.

Cross-ref: CHANGELOG 0.16.0.

## [0.16.0] T-24 — ed: niplines confirm/cancel (phase 2), TTL enforcement (v0.16.0, 2026-09-11)

Theme: ed-editing · closed 0.16.0 · 2026-09-11
Add repoman ed confirm TICKET-ID and repoman ed cancel TICKET-ID
(phase 2). confirm re-hashes the live file against the ticket's
recorded pre-nip hash and refuses -- the same way apply refuses a
stale handle -- if the file has drifted since the ticket was issued,
because the previewed diff no longer matches what confirm would
actually apply. On a hash match, performs the nip in one atomic
write and records it via journal.Record exactly like apply does, so
undo works on a confirmed nip identically to any other edit. cancel
(and plain expiry past the ticket's TTL) discards the ticket with no
write, ever.

Blocked by T-23 (there is nothing to confirm until request/preview
exists).
See docs/proposals/ed-insert-and-ticketed-niplines.md, Proposal 2.

Cross-ref: CHANGELOG 0.16.0.

## [0.16.0] T-23 — ed: niplines request/preview (phase 1) with gofmt/vet preflight (v0.16.0, 2026-09-11)

Theme: ed-editing · closed 0.16.0 · 2026-09-11
Add repoman ed niplines FILE START END (phase 1, request-only --
writes nothing to FILE). Computes the post-nip content, prints a
diff-style preview (removed lines clearly marked, a few lines of
context each side -- not a whole-file dump the caller has to
re-derive the diff from by eye), and for a recognized source type
runs a preflight check against the previewed content on a scratch
copy only (gofmt -l and go vet for .go; the repository's configured
linter otherwise), surfaced as a warning in the readback, never a
hard refusal. Issues a ticket recorded via the storage from T-22,
carrying both the pre-nip file hash and the exact preview-text hash,
plus an expiry. Default TTL 10 minutes; --ttl override up to a 1
hour cap for a human-review flow. No TTL longer than that is
supported -- longer-pending nips belong in TRACKING.md as their own
item, not as a long-lived ticket.

Blocked by T-22 (needs its storage shape decided first).
See docs/proposals/ed-insert-and-ticketed-niplines.md, Proposal 2.

Cross-ref: CHANGELOG 0.16.0.

## [0.15.0] T-39 — waveprogress: trackingRow regex couldn't see the ☑/✗ register statuses, causing spurious orphan warnings and stale wave rows (v0.15.0, 2026-09-11)

Theme: waves · closed 0.15.0 · 2026-09-11
- **Trigger:** while closing out this session's ten-plus pending register items and regenerating the wave tracker, waveprogress --show repeatedly warned that six items genuinely present and open in TRACKING.md (all at ☑, done-pending-release) were 'found in neither TRACKING.md nor RESOLVED.md'. Traced directly rather than worked around.

- **Scope:** pkg/waveprogress/waveprogress.go's own status-matching regexes (rowRe, itemCol4Re, waveRowRe, trackingRow) all used the character class [✓◐☐] -- only three of the register's own five legal status symbols (see pkg/register/register.go, which accepts ✓|◐|☐|✗|☑). rowRe/itemCol4Re/waveRowRe correctly parse a WAVE-TABLE row's own status cell, which genuinely is three-valued by design, so those three are unaffected. trackingRow parses TRACKING.md's REGISTER status column instead, a different, five-valued domain -- that one regex was the actual bug: any item at ☑ or ✗ silently fell out of syncWaveRowsFromRegister's openStatus map entirely, indistinguishable from an id that doesn't exist anywhere.

Fixed by widening trackingRow's character class to [✓◐☐✗☑], and adding a new wavelevelStatus(string) string helper that maps the register's five-symbol legend down to the wave-row's own three-symbol vocabulary before use (☑ reads as ◐ -- real, verified progress, just not yet closed; ✗ reads as ☐ -- contributes nothing). New tests: TestWavelevelStatus (direct mapping coverage) and TestSyncWaveRowsFromRegister_PendingReleaseCorrectsRowToHalf (the exact repro: a ☑ item must sync to ◐ with zero warnings, not fall into the orphan-warning branch). Confirmed both genuinely fail pre-fix via git stash (build fails outright, since wavelevelStatus is undefined without the fix) before confirming green post-fix.

Cross-ref: CHANGELOG 0.15.0.

## [0.15.0] T-31 — gate ed/strreplace write paths' own reads with a provenance check (v0.15.0, 2026-09-11)

Theme: provenance · closed 0.15.0 · 2026-09-11
- **Trigger:** a direct question (2026-09-09) about whether
  every read of a managed file could be gated by a provenance check as
  an invariant, since sha256 is cheap. Investigation found the read
  side of the codebase has no shared funnel the way the write side
  does (ed.Record is the one write chokepoint; reads are ~53 separate
  os.ReadFile/os.Open call sites across ~20 packages, most of them
  read-only consumers of TRACKING.md/config that were never at risk of
  writing on top of stale content). A global read-gate is therefore
  not a clean mechanical change -- it would need per-call-site role
  judgment, not a find/replace.

  Narrower and load-bearing scope, confirmed by reading the actual
  write paths: `ed apply`'s SpanHash staleness check already catches
  most out-of-band edits near the matched span as a side effect, but
  `ed sub` has NO staleness check at all (--expect only guards
  occurrence count, not whether the file changed since anything was
  last recorded), and `ed append`/`prepend` have none by design
  (they target file boundaries, not matched text, so an out-of-band
  edit immediately before one is invisible to them). `strreplace`
  already funnels every read through one function, readFileUTF8.

Scope: add a provenance.CheckOne(path) helper (thin wrapper around
the comparison logic Check() already has) and call it at the top of
every ed write path's own read -- apply, sub (per matched file, before
the plan is built), append/prepend, and undo's revertTxn -- plus
strreplace's readFileUTF8. A file with no recorded provenance yet
(never touched by repoman) passes through unchanged, exactly like
Check() treats it today. A mismatch refuses with the same
provenance-mismatch/provenance-missing wording `provenance check`
already uses, before any write happens -- closing the actual gap
(writing on top of content that diverged since repoman last touched
it) at the moment it matters, rather than only catching it later on a
manual on-demand check. Explicitly NOT in scope: register/board/
guards/syncver/etc.'s own reads (read-only consumers, not editors);
a global wrapper for all 53 read call sites; sanction/override (T-02,
separate); relcore wiring (T-03, separate).

Progress: implemented and verified (ed.CheckProvenance, a
dependency-free comparison primitive added to pkg/ed since ed cannot
import pkg/provenance -- provenance already imports ed, so the
reverse would cycle; provenance.Check/CheckOne now wrap this same
primitive instead of duplicating the sha256 comparison). Gated:
apply, insert, sub (whole-plan pre-scan before the write loop,
refusing all-or-nothing the same way --expect/roles mismatches
already do), append/prepend, undo's revertTxn, and strreplace's
single readFileUTF8 read funnel (covers stageSubOp's initial load,
the dry-run diff, and the write pass's own re-read in one place).

Building this immediately exposed a real deadlock: apply/sub/etc. now
refuse a mismatched file, including the ONLY documented v1 recovery
path (redoing the edit through repoman to re-sync the hash) -- since
that redo is itself a write these same gates now block. T-02 was
built immediately after, in the same session, specifically to resolve
this (see T-02's own Progress note) rather than shipping the gate
with no way out of it. 18 new selftest checks (170 -> 188): apply
refusing a handle freshly taken on tampered content (SpanHash alone
would have let it through), sub/append/strreplace each refusing and
then working normally again post-sanction. Verified live against a
real out-of-band edit for every one of the five ed write paths plus
strreplace before writing selftest coverage. Pending a batched
release.

Cross-ref: CHANGELOG 0.15.0.

## [0.15.0] T-30 — board --format json fails validation on every axis: legend field cannot be null (v0.15.0, 2026-09-11)

Theme: kanban · closed 0.15.0 · 2026-09-11
Confirmed pre-existing (reproduced against the unmodified status axis before writing any T-05/T-08 work): repoman board --definition ... --format json fails with "error: validation failed: data.legend: field cannot be null" regardless of axis. definitionReportSchema's legend field (pkg/board/definition_schema.go) is declared without .Required(), but queryfy's strict-mode Object() schema still rejects a literal JSON null for def.Legend when it arrives as a nil Go slice (workspace.Board.Legend, unset in a minimal definition). text and html formats are unaffected and were used to verify T-05/T-08's own dependency_order work end to end. Likely fix: either make Legend non-nullable at the schema level (empty array default) or give definitionReportSchema's legend field an explicit nullable allowance -- needs a decision, not guessed at here, since the same shape appears in board/schema.go's own envelope and possibly elsewhere any optional array field is validated this way.

Progress: implemented and verified. Root cause confirmed as diagnosed above: DefinitionReport.Legend (pkg/board/definition.go) has json tag "legend" with no omitempty, unlike DefinitionItem.Legend which already has omitempty -- a nil Legend therefore marshals to JSON null and queryfy's strict-mode schema rejects it even without .Required(). Fixed by normalizing a nil Legend to an empty []workspace.BoardLegendEntry{} at both DefinitionReport construction sites (RenderDefinition, for status/priority/theme axes, and renderDependencyOrder, for dependency_order), the same discipline Columns/Overflow.Items already followed a few lines below in the same file. Verified live against two independently constructed definition files -- the dependency_order definition used for T-05/T-08's own board, and a fresh minimal status-axis definition -- confirming both RenderDefinition and renderDependencyOrder code paths are fixed, not just the one this item's own diagnosis happened to reproduce against. pkg/selftest/section26.go's own doc comment, which had documented this as a still-open known gap and deliberately skipped --format json, is corrected and now carries a real regression check: the same linear-chain fixture already used for the text/html checks, re-run with --format json, asserting the envelope parses, legend decodes as an empty array rather than null, and the tiered column/item structure matches the text/html checks exactly (190/190 selftest checks green, up from 188). Pending a batched release with T-05/T-08/T-20/T-21/T-31.

Cross-ref: CHANGELOG 0.15.0.

## [0.15.0] T-26 — Claimed-by: optional register-item field for advisory work claims (v0.15.0, 2026-09-11)

Theme: locking · closed 0.15.0 · 2026-09-11
Add an optional `Claimed-by` field to the register item field line
(`Theme: x · Priority: Pn · Status: s · Wave: n · Filed-by: y ·
Blocks/after: z · Claimed-by: w`), same additive pattern T-06 used
for `Wave`: a new optional capture group in the existing field-line
regex, a new field on `Item`/`ItemSummary`, present-only-when-set. No
existing register, table row, or field line changes shape.

This is metadata about who intends to work an item, not a new axis
any board or tiering computation needs to read. `ItemSummary` already
gained `Blocks` for T-05/T-08's dependency-tier work in the same
release window this item is filed against; `Claimed-by` is
deliberately independent of that -- board rendering, tier computation,
and `RenderDefinition`'s axis dispatch never need to know a claim
exists. Confirmed no existing field-line regex reserves this slot
before filing.

Progress: implemented end-to-end. `fieldRe` in `pkg/register/register.go`
gained a new optional `(?: · Claimed-by: (\S+))?` group appended after
Blocks/after, verified against 6 representative field-line shapes
(with/without Wave, Blocks/after containing a comma, with/without the
new segment) via a standalone Go test program before trusting it in
the full build. `Item.ClaimedBy` and `ItemSummary.ClaimedBy` (json
`claimed_by,omitempty`) added; `summarySchema` gained a plain optional
`claimed_by` string field (no nullability trap here -- unlike T-30,
this is a string, not a slice, so empty-vs-null does not arise).
`register add` gained `--claimed-by CLAIMED_BY`, threaded through the
generated field-line template alongside the existing Wave/Filed-by/
Blocks-after segments, and its `--help` usage/options text. New
`section29.go` regression coverage (8 checks, selftest 203 -> 211):
`register add --claimed-by` writes the segment correctly; an item
without it gets no segment at all (omitempty, not an empty marker);
and a hand-written field line combining Wave, Filed-by, a
comma-bearing Blocks/after, and Claimed-by together all parse
correctly, confirming the new segment neither swallows nor is
swallowed by the one before it. Self-check note: an early manual
`--with` replacement while fixing this same file's `register add
--help` text corrupted leading-tab indentation on one line (the ed
apply tool re-anchors on the matched span's own surrounding
whitespace, so a hand-typed replacement with its own leading tabs
compounds rather than replaces); caught immediately by `gofmt -l`
before it reached a build, fixed with `gofmt -w` (a pure mechanical
formatter, not a substitution tool within the meaning of this
project's editing-tool restriction), and confirmed the file was never
provenance-tracked to begin with, so no stale journal record resulted.
Not yet done: T-27's `register claim`/`register release` subcommands
(separate, dependent register entry) and T-29's board/kanban surfacing
of claims (also separate, dependent). This item is code-complete and
tested, pending the next release for closure per this project's
established Progress-note convention.

Value shape: a free-text agent/session identifier, no enforced
syntax beyond non-empty (unlike Filed-by's `<project>:<prefix>-<n>`
shape, a claim identifies a worker, not another register's ticket).

Cross-ref: CHANGELOG 0.15.0.

## [0.15.0] T-22 — ed: pending-ticket storage design for niplines two-phase flow (v0.15.0, 2026-09-11)

Theme: ed-editing · closed 0.15.0 · 2026-09-11
Decide and implement where pending niplines tickets live: a new
sibling file (e.g. .ed-tickets.json) next to .ed-journal.json, or a
section of the journal itself. Leaning toward a separate file so the
journal's existing MaxTxns/MaxBytes eviction logic (SaveJournal)
never has to reason about unconfirmed, pending state -- a ticket is
not a completed edit and must not be evictable the same way. Each
ticket record needs: opaque ticket id, target file, requested line
range, hash of the pre-nip file, hash of the exact preview text
shown to the caller, issued-at timestamp, and TTL/expiry. This item
blocks T-23 and T-24, which build on top of whatever storage shape
this settles on.

See docs/proposals/ed-insert-and-ticketed-niplines.md, Proposal 2,
"Open questions" section.

Progress: decided the separate-file option, per the proposal's own
lean and confirmed against the actual `SaveJournal` eviction logic
(`pkg/ed/ed.go`) -- `MaxTxns`/`MaxBytes` eviction walks `j.Txns` by
count and serialized size, and folding tickets into `Journal` would
force that loop to special-case pending, unconfirmed entries for no
benefit. Implemented in new `pkg/ed/tickets.go`: `Ticket` (id, file,
1-based inclusive start/end line, pre-nip hash, preview-text hash,
issued-at, expires-at) and `TicketStore` (map keyed by id plus a
monotonic `NextID`, mirroring `Journal`'s own txn-id counter so a
discarded ticket's id is never reused) persisted to
`.ed-tickets.json`, atomic tmp-file-plus-rename write exactly like
`SaveJournal`. `IssueTicket`/`GetTicket`/`DiscardTicket`/
`PruneExpired`/`Ticket.IsExpired`/`HashContent` cover creation,
lookup, removal (used by both a successful confirm and a cancel),
expiry-driven bulk cleanup, and the shared sha256-hex digest shape
already used by `FileProvenance`. `DefaultTicketTTL` (10 min) and
`MaxTicketTTL` (1 hour, enforced as a hard refusal, never silently
clamped) follow the proposal doc's own TTL table. `.ed-tickets.json`
added to `.gitignore` alongside `.ed-journal.json`, same local
disposable-session-state treatment.

Verified via a standalone Go program exercising the full surface (20
checks: fresh-store, issue, persistence round-trip, id monotonicity
including non-reuse after discard, invalid-range and missing-hash
refusals, TTL-ceiling refusal and at-ceiling acceptance, expiry,
pruning, discard, unknown-id no-op) before committing to permanent
coverage. Permanent regression coverage added as `pkg/ed/
tickets_test.go` (12 `go test` cases, this project's existing but
previously singular precedent for native-Go package tests --
`pkg/workspace/schema_test.go` -- now has a second user) rather than
CLI-driven `selftest` coverage, since this item is purely a storage
primitive with no command-line surface of its own yet; `go test
./...` is not build-tagged or otherwise gated, so this is live
coverage under the default invocation, not a dormant guard requiring
separate registration under Part 3 §8 of the working agreement.

Deliberately out of scope here, left for T-23/T-24: `ed niplines`
(request/preview), `ed confirm`/`ed cancel` (redeem/discard), the
gofmt/vet preflight, and any CLI wiring in `Run()`. This item is the
storage shape alone, as its own spec asked for.

Cross-ref: CHANGELOG 0.15.0.

## [0.15.0] T-21 — ed: insert verb, handle-verified positional insertion (v0.15.0, 2026-09-11)

Theme: ed-editing · closed 0.15.0 · 2026-09-11
Progress: implemented and tested (repoman ed insert --after/--before,
5 new selftest paths including a stale-handle refusal and the
--after/--before mutual-exclusivity check, full gofmt/vet/build/
selftest gate green), pending a batched release with T-20.

Add repoman ed insert HANDLE --after|--before --with TEXT. Requires
a find-verified handle exactly like apply does, re-runs the same
SpanHash check before writing, and differs from apply only in which
boundary of the matched span the replacement text is spliced at
(text[:e]+TEXT+text[e:] for --after, text[:s]+TEXT+text[s:] for
--before). The matched span itself is never touched, which is the
point: this verb structurally cannot reproduce the anchor-duplication
bug that apply's --with is exposed to when a replacement string
accidentally restates part of the matched text (hit twice in this
session while editing cards-and-guards.md, both times caught on
reread and undone via ed undo). New branch in ed.Run() reusing
parseHandle, SpanHash, and Record verbatim.

See docs/proposals/ed-insert-and-ticketed-niplines.md, Proposal 1.

Cross-ref: CHANGELOG 0.15.0.

## [0.15.0] T-20 — ed: append/prepend verbs, no handle required (file-boundary insertion) (v0.15.0, 2026-09-11)

Theme: ed-editing · closed 0.15.0 · 2026-09-11
Progress: implemented and tested (repoman ed append/prepend, 5 new
selftest paths, full gofmt/vet/build/selftest gate green), pending a
batched release with T-21.

Add repoman ed append FILE --with TEXT and repoman ed prepend FILE
--with TEXT. Unconditionally safe: no find-verified handle needed,
because the target is a true file boundary (offset 0 or EOF), not
existing text being trusted. Implement as a new case in ed.Run()
equivalent to apply at a zero-width span at len(text) or 0,
reusing the existing atomic-write and journal.Record path so undo
works on it identically to any other edit.

See docs/proposals/ed-insert-and-ticketed-niplines.md, Proposal 1.

Cross-ref: CHANGELOG 0.15.0.

## [0.15.0] T-08 — board: cross-project dependency-order tiering (v0.15.0, 2026-09-11)

Theme: kanban · closed 0.15.0 · 2026-09-11
Extend board with cross-project, cross-wave dependency-order tiering
(a Now/Next/Then/Later-style computed view), the exact shape the real
poesy wave board demonstrated by hand this session. Depends on T-5
(parsed Blocks/after graph) and T-6 (wave membership field) -- without
those, this has no structured data to compute tiers from.

Open question carried from that same session: a real dependency chain
can have a legitimate soft override (an item whose only hard
dependency is satisfied, but whose own plan calls for something else
to be exercised first) -- the poesy board disclosed one by hand in its
footer. Whether that should be a distinct Soft-after field or stay a
manual annotation is not decided; flag it rather than silently pick
one when this is built.

See docs/proposals/t08-board-target-shape.md for a second worked
reference, built against this repository's own register data.

Implementation shortest-path confirmed in session, once T-05 lands:
compute tier(i) = 1 + max(tier of each blocker), 0 for items with no
unresolved blocker -- the exact by-hand logic already verified twice
this session for the reference board, ported to real edges. Wire
into board/definition.go's RenderDefinition: the dependency_order
axis is already schema-legal (workspace/schema.go's boardSchema
pattern already includes it) and its column semantics are already
documented in that file's own comment ("position IS the value:
column i collects tier i") -- only the current hard refusal at
RenderDefinition's dependency_order branch needs replacing with a
real column-assignment path; Overflow, legend matching, and
text/json/html rendering are axis-agnostic already and need no
changes. ProjectReport.OpenItems is []register.ItemSummary directly,
so cross-project boards pick up Blocks with no signature changes
once T-05 lands.

Progress: implemented and verified. renderDependencyOrder in
pkg/board/definition.go replaces the former hard refusal; tested
against a linear chain, a closed blocker, a real cycle, a self-loop,
narrow explicit columns with Overflow, and this project's own live
register (T-05 correctly lands in tier 0, T-08 in tier 1, now that
this item's own Blocks/after field names T-05 -- it previously named
only T-06, a pre-existing inconsistency with this item's own body
text, now corrected). The Soft-after question above remains open and
is unaffected by this: no live data needed it. Found here, fixed
separately: board --format json failed validation on every axis
(T-30, now ☑ -- see its own entry), unrelated to dependency_order
specifically -- text and html were unaffected and were used for all
verification of this item. Pending a batched release with T-05.

Cross-ref: CHANGELOG 0.15.0.

## [0.15.0] T-05 — parse Blocks/after into real graph edges, with cycle detection (v0.15.0, 2026-09-11)

Theme: dependency-graph · closed 0.15.0 · 2026-09-11
Parse the register's free-text Blocks/after field into real graph
edges (currently prose a person reads, not structure a function
walks) -- confirmed as a real gap during the dependency-order kanban
board work this session, where a hand-built board computed tiers from
this field manually since gorepoman itself cannot.

Must include cycle detection: nothing currently prevents A blocks-
after B blocks-after A, and a real graph walker needs a defined,
non-crashing answer for that case, not just an assumption it never
happens.

Implementation shortest-path confirmed in session: Item.Blocks is
already parsed and stored (register.go's fieldRe already captures
Blocks/after, same regex T-06 extended for Wave); the only gap is
(1) copy Blocks onto the exported ItemSummary in report.go -- the
same one-line gap Wave/FiledBy had before T-06 closed it -- and
(2) parse the comma-separated raw string into edges plus cycle
detection. Every live Blocks/after value in this register today is
bare comma-separated ids, no cross-project prefix syntax in use for
this field (that is Filed-by's own syntax, already separate).

Progress: implemented and verified (pkg/register/graph.go --
ParseBlocks, BuildGraph, DetectCycle, TierOf; wired into T-08's
dependency_order board render). 3 queryfy schemas that hand-mirror
ItemSummary's shape needed the same "blocks" field added -- confirmed
mandatory, not optional, since qf.Validate defaults to Strict mode.
7 new selftest checks (150 -> 157): linear chain, closed blocker
correctly not holding its dependent back, real cycle, self-loop,
overflow with narrow explicit columns, html output. Verified against
this project's own live register too. Pending a batched release with
T-08.

Cross-ref: CHANGELOG 0.15.0.

## [0.15.0] T-04 — badcode hits annotated with provenance status (v0.15.0, 2026-09-11)

Theme: provenance · closed 0.15.0 · 2026-09-11
Annotate a badcode hit with whether the offending line was written
through repoman (has a journal/provenance record) or arrived by a
raw edit outside it. No adjacent secret-scanning tool (Gitleaks,
TruffleHog) can do this, since none of them own the editing layer --
this is specific to gorepoman owning both.

Depends on T-1 (provenance must exist to check against).

Progress: implemented and verified. badcode.Match gained a Provenance
field (pkg/badcode/badcode.go) with three labels: "repoman" (the
matched file's current content matches repoman's own journal record),
"no provenance record" (the file has never been touched by
ed/strreplace at all), and "stale provenance record" (repoman wrote it
at some point, but it has since diverged outside repoman's own write
paths -- a provenance Mismatch/Missing in its own right, kept distinct
rather than folded into either of the other two). Note the annotation
is necessarily whole-file, not per-line: FileProvenance records one
hash per file, not per line, so a match's provenance reflects the file
it was found in, not the specific matched line's own history. Computed
in Check() via ed.CheckProvenance against the loaded journal (pkg/ed
already exposes this primitive for exactly this purpose; badcode
importing ed introduces no import cycle -- confirmed, ed only imports
roles/webhelp). Path normalization needed real care: journal keys are
an exact, unmodified copy of whatever path string ed/strreplace were
invoked with (relative or absolute depending on the caller), and
roles.Expand's own output has the same property depending on how the
badcode scan itself was invoked (relative for the standalone `badcode
check` CLI's default ".", absolute for relcore's own preflight, which
passes the absolute project root) -- so each match's provenance lookup
tries the exact path first, falling back to a cwd-relative form only
if that misses. An earlier version of this fix normalized every path
to cwd-relative unconditionally, which broke the exact-match case (an
absolute journal key looked up via its own relative form does not
match) -- caught by the new selftest coverage itself before this was
called done, not shipped and found later. Both `badcode check`'s own
CLI output and relcore's badcode preflight reporting were updated to
show the annotation (`[provenance-label]` in the error line); --help
text for `badcode check` documents the three labels and why no
adjacent tool can produce them. Verified live against three fixture
files (never-tracked, tracked-and-clean, tracked-then-tampered) before
writing selftest coverage. New selftest coverage added to
section12.go (badcode's own section): 6 new checks confirming all
three labels appear correctly and distinctly (203/203 selftest checks
green, up from 197). Pending a batched release with
T-03/T-05/T-08/T-20/T-21/T-30/T-31.

Cross-ref: CHANGELOG 0.15.0.

## [0.15.0] T-03 — wire provenance into relcore, unconditional alongside badcode (v0.15.0, 2026-09-11)

Theme: provenance · closed 0.15.0 · 2026-09-11
Wire provenance into relcore: runs unconditionally alongside badcode,
before anything else, on every release (including --resume). An
unsanctioned out-of-band edit blocks the release the same way a
badcode hit does.

Depends on T-1 and T-2.

Progress: implemented and verified. Added runProvenancePreflight to
pkg/relcore/relcore.go, modeled directly on the existing
runBadcodePreflight: not a release.steps entry (cannot be removed from
.repoman.json by anyone with repo access), not resumable, not
journaled through the skip-if-green path any other step can use, and
called unconditionally in Run immediately after the badcode gate,
before the release.steps loop -- exactly the ordering this item's own
title names. Calls provenance.Check() directly (the same function the
standalone `repoman provenance check` command uses) and reports each
mismatch the same way the CLI does; a real mismatch is a hard,
unrecoverable failure with no bypass, same as badcode -- the only
remedy is `repoman provenance sanction FILE --reason "..."` first,
then re-run. A project with nothing ever tracked by repoman is a soft
pass, mirroring badcode's own no-config soft pass: there is nothing
this gate could have caught yet. relcore's own --help text and the
release-workflow doc pointer were updated to mention both gates
together rather than badcode alone. Verified live end to end before
writing selftest coverage: a clean tracked file passes; an out-of-band
edit blocks before the release.steps loop runs at all; --resume does
not bypass it; sanctioning the mismatch un-blocks the very next run.
New selftest section (section28.go, chained after section27/T-01's
own provenance section) covers all of this as a real regression, using
a synthetic project separate from the shared selftest fixture, the
same isolation section13.go already uses for relcore/badcode
integration testing (197/197 selftest checks green, up from 190).
Pending a batched release with T-05/T-08/T-20/T-21/T-30/T-31.

Cross-ref: CHANGELOG 0.15.0.

## [0.15.0] T-02 — provenance sanction: explicit override with mandatory reason (v0.15.0, 2026-09-11)

Theme: provenance · closed 0.15.0 · 2026-09-11
`repoman provenance sanction <file> --reason "..."` lets a human
explicitly approve a detected out-of-band edit rather than treating
it as a permanent block. --reason is mandatory and gets recorded
alongside the hash at the moment of sanction. There is no other veto
mechanism -- declining to sanction and redoing the edit through
repoman is the only other path forward.

Depends on T-1 (the check this sanctions).
Progress: implemented and verified (ed.Sanction, exported alongside
CheckProvenance; CLI: `repoman provenance sanction <file> --reason
"..."`, dispatched from provenance.Run via a new runSanction). Built
immediately after T-31 landed, in the same session, because T-31's
own write-path gates closed the only documented v1 recovery path
(redoing the edit through repoman) -- that redo is itself a write, so
it was being refused too. Sanction re-syncs the recorded hash to
current on-disk content directly, without requiring the edit to be
redone. Refuses without --reason (mandatory, not optional -- the
audit trail is the point); refuses on a path with no mismatch on
record (not a silent no-op); refuses on a Missing path (nothing to
certify -- restore it first). 6 new selftest checks as part of
section27.go's T-31 coverage (170 -> 188 total, counting both T-31
and T-02 together): no-reason refusal, empty-reason refusal,
successful sanction re-syncing the hash, re-sanctioning refused,
ed apply working normally again post-sanction. Verified live before
writing selftest coverage. Pending a batched release.

Cross-ref: CHANGELOG 0.15.0.

## [0.15.0] T-01 — provenance check: detect out-of-band edits to journal-tracked files (v0.15.0, 2026-09-11)

Theme: provenance · closed 0.15.0 · 2026-09-11
sha256-journal-based detection of edits made to a journal-tracked file
outside repoman (raw str_replace, sed, hand-editing). Design v1 was
agreed before this session (see [[repoman]] memory) but never built --
this is the single most overdue item in this whole plan, since it is
the mechanism the zendis slip directly motivated.

Scope: repoman ed apply/sub, strreplace, and undo each record a
sha256 of the full file at write time. `repoman provenance check`
recomputes the current hash of every file with a journal entry and
reports a mismatch as an out-of-band edit. v1 scope is journal-tracked
files only, no git integration.

Progress: implemented and verified (pkg/ed/ed.go -- FileProvenance
struct and Journal.FileProvenance map, recordProvenance called from
both ed.Record -- the single funnel shared by ed and strreplace's
write paths -- and from revertTxn, undo's own write path, which
required threading *Journal through its signature since undo removes
a Txn rather than adding one and so never went through Record on its
own; pkg/provenance/provenance.go -- Check() and the `provenance
check` CLI, wired into cmd/repoman/main.go). 13 new selftest checks
(157 -> 170): clean state, a genuine out-of-band edit caught as a
mismatch, redoing the edit through repoman re-syncing the hash,
undo's own resync, and a deleted tracked file reported as Missing.
Also hand-verified live against a real out-of-band edit before
writing the selftest coverage. v1 scope only -- no sanction/override
(T-02) and not yet wired into relcore (T-03). Pending a batched
release.

Cross-ref: CHANGELOG 0.15.0.

## [0.14.12] T-38 — make cross embedded a v-prefixed version string, GoReleaser did not -- release binaries and the Pages mirror hashed differently for the same tag (v0.14.12, 2026-09-11)

Theme: release · closed 0.14.12 · 2026-09-11
- **Trigger:** B-15 (checksums.txt mismatch between the GitHub Release and the ha1tch.github.io mirror) was reopened for verification against v0.14.11; user pointed directly at a suspected one-character version-string inconsistency between build paths rather than asking for another release-page comparison.

- **Scope:** `.goreleaser.yaml` (used by `build-and-release.yml` for the real tagged GitHub Release) embeds the version via `-X main.version={{ .Version }}`, and GoReleaser's `.Version` template variable is documented to be the tag with its leading `v` already stripped -- e.g. `0.14.9`. `Makefile`'s `cross` target (used by `pages.yml` to build the GitHub Pages mirror) instead set `LDFLAGS := -s -w -X main.version=v$(VERSION)`, prepending a literal `v` that GoReleaser never adds -- e.g. `v0.14.9`. `cmd/repoman/main.go`'s `version` command prints this string verbatim (`fmt.Println("repoman " + version)`), so the two build paths produced binaries differing by exactly one embedded byte sequence for what was meant to be the same release -- enough to change every binary's SHA256, and therefore `checksums.txt` itself, since `-ldflags -X` compiles the string directly into the binary. Fixed by dropping the hardcoded `v` from the Makefile's LDFLAGS so both paths embed the bare `VERSION`-file value, matching GoReleaser's own convention (the real Release path, which is what `/releases/latest/download/` actually serves). No fix needed on the GoReleaser side -- its output was already correct; the Makefile was the outlier.

Cross-ref: CHANGELOG 0.14.12.

## [0.14.12] T-37 — addwave self-heals missing WAVE_TRACKING.md/WAVE_PLAN.md instead of refusing (v0.14.12, 2026-09-11)

Theme: waves · closed 0.14.12 · 2026-09-11
- **Trigger:** re-verification of the old B-05 bug report (2026-09-11) found the destructive part (partial-write corruption) already fixed, but a real gap remained: addwave hard-refused a project's first-ever call when WAVE_TRACKING.md/WAVE_PLAN.md did not exist yet, and that manual-seed requirement was never documented anywhere (not -h text, not repoman-080-waves.md). waveprogress.go already self-heals the equivalent situation on its own read path (see its own code comment crediting the original B-05 fix); addwave's write path never got the same treatment.

- **Scope:** pkg/addwave/addwave.go -- replaced requireDocs (hard refusal) with ensureDocs (creates a minimal skeleton for either/both missing files, using the identical skeleton text waveprogress.go already expects, so no separate self-heal is needed on the read side). --dry-run creates nothing on disk in this case, using the in-memory skeleton to still produce a coherent preview rather than crashing on a read of a file that was never going to exist. Documented in addwave -h and repoman-080-waves.md.

Cross-ref: CHANGELOG 0.14.12.

## [0.14.11] T-36 — board/workspace/provenance -h never live-fetched their docs chapter (webhelp) (v0.14.11, 2026-09-11)

Theme: docs · closed 0.14.11 · 2026-09-11
- **Trigger:** audit requested after the v0.14.10 release -- every other command's -h prints webhelp.SuppressionNote and attempts webhelp.PrintIfAvailable, but board.go and workspace.go never imported pkg/webhelp at all, and provenance.go (which does import it, for NormalizeBriefFirst) never called PrintIfAvailable on any of its three -h paths (top-level, check, sanction). All three chapters (repoman-085-board.md, repoman-086-workspace.md, repoman-088-provenance.md) existed, were correctly written, and were correctly published by CI/build_site.py -- the gap was purely in the CLI's own wiring, never in the docs.

- **Scope:** pkg/board/board.go, pkg/workspace/workspace.go, pkg/provenance/provenance.go -- add the webhelp import where missing, call webhelp.NormalizeBriefFirst at the top of every Run/runSanction, print webhelp.SuppressionNote unconditionally alongside embedded help, call webhelp.PrintIfAvailable with the correct doc chapter name on every -h exit point.

Progress: implemented and verified. All five -h exit points fixed (board top-level, workspace top-level, provenance top-level, provenance check, provenance sanction), each now importing pkg/webhelp where it was missing, calling webhelp.NormalizeBriefFirst at the top of its Run/runSanction, printing webhelp.SuppressionNote unconditionally, and calling webhelp.PrintIfAvailable with the correct chapter name (repoman-085-board, repoman-086-workspace, repoman-088-provenance). Verified genuinely fixed, not just plausible: confirmed live against the real https://ha1tch.github.io/gorepoman/ site (0.14.9, still the latest published tag) that all 20 doc chapters are present and correctly linked -- the docs themselves were never the problem. New selftest coverage (pkg/selftest/section21a.go, chained between section21 and section22) drives all five -h paths against a local httptest.Server and confirms three properties each: the suppression note prints unconditionally, a genuine fetch actually succeeds and its content appears, and --brief still suppresses it. Confirmed this coverage is real, not passing for the wrong reason: git-stashed the three fixed source files, rebuilt, and confirmed selftest fails (exit 1, "SELFTEST FAILED -- do not trust this build") with the new checks specifically failing, then restored the fix and confirmed green again. Selftest: 211 -> 226. No doc content changes needed -- repoman-085-board.md, repoman-086-workspace.md, repoman-088-provenance.md were all already correct and complete; this was purely a CLI-wiring gap that existing selftest coverage (section20/21) never exercised, since it only ever drove `ed -h`/`register -h`. Pending a batched release.

Cross-ref: CHANGELOG 0.14.11.

## [0.14.10] T-35 — waveprogress: derive each wave row's status from register/RESOLVED state before rendering (v0.14.10, 2026-09-10)

Theme: waves · closed 0.14.10 · 2026-09-10
Trigger: xolu team found nothing regenerated a wave row's own status outside of register close's own direct wiring (T-19/propagateCloseToWaves) -- a row could drift from reality whenever an item's closure happened some other way (RESOLVED.md gaining a closure header by hand, e.g.).
Scope: env.syncWaveRowsFromRegister in pkg/waveprogress/waveprogress.go, run before parseWaves on every waveprogress invocation. Derives each linked row's real status from RESOLVED.md's closure headers and TRACKING.md's own open rows; warns (never guesses) when an id is found in neither. Composes with the existing B-04 summary-line regeneration and --check staleness detection with no duplicated logic.

Progress: implemented and verified end-to-end against real files (register add -> addwave -> hand-edited RESOLVED.md closure header, bypassing register close entirely): the wave row, summary line, progress bar, and Overall percentage all correctly synced to done from the closure header alone. Also verified the orphan-id warning path (an id in neither document leaves its row untouched and prints a warning) and --check's staleness detection (reports stale before the sync, clean after). A row naming more than one id via a range takes the least-done member's status via combineStatus. No import cycle: kept local to pkg/waveprogress rather than reusing pkg/register's propagateCloseToWaves, since pkg/register already imports pkg/waveprogress. Regression coverage: pkg/waveprogress/sync_test.go. Documented in repoman-080-waves.md (new "Keeping a wave row's own status honest" section, both mechanisms side by side with real worked examples). Pending a batched release with T-34.

Cross-ref: CHANGELOG 0.14.10.

## [0.14.10] T-34 — wave_complete_word: config override for waveStatusWord's hardcoded 'done' (v0.14.10, 2026-09-10)

Theme: waves · closed 0.14.10 · 2026-09-10
Trigger: xolu team found waveStatusWord's B-04 fix hardcodes "done" with no override -- xolu's own established convention is "complete", so upgrading and running waveprogress once would silently rewrite every existing summary line project-wide, with no opt-out.
Scope: config.Config.WaveCompleteWord (json wave_complete_word, default empty meaning "done"), threaded through waveStatusWord's new completeWord parameter. Byte-identical default behavior for any project that never sets the key.

Progress: implemented as designed. Verified end-to-end through the real closure pipeline (register add -> addwave -> register close), not just a unit test: with wave_complete_word unset, a closed wave's summary line reads "...done."; with wave_complete_word set to "complete", the identical sequence produces "...complete." on the same real WAVE_TRACKING.md. Regression coverage: pkg/waveprogress/waveprogress_test.go. Documented in repoman-090-configuration.md (all-keys table plus a full entry) and repoman-080-waves.md is unaffected (display-only chapter; the field-line docs cover this). Pending a batched release with T-35.

Cross-ref: CHANGELOG 0.14.10.

## [0.14.10] T-33 — workspace join writes local .repoman.json before remote validation, no rollback (v0.14.10, 2026-09-10)

Theme: bugfix · closed 0.14.10 · 2026-09-10
Trigger: reported via two independent repros against v0.14.9 (a remote with no participants.json yet leaves a stale local entry with no rollback and no repair command).
Scope: pkg/workspace/workspace.go's runJoin wrote the local workspace membership entry before cloning/validating the remote at all -- any failure after that point (clone, participants.json read, participants.json write, push) left the stale entry behind with no rollback, and a retry then refused with "already a member of workspace".

Progress: fixed by reordering runJoin -- the remote side (clone, participants.json read/write, commit+push) is now validated and updated FIRST, and local .repoman.json is only written once that has genuinely succeeded; none of the remote-side calls depend on local state, so the reorder changes nothing about the success path. As a side effect this also fixed a second, related gap: when the remote already listed the project as a participant, the old code returned success without ever writing local membership -- now it does. Regression coverage: pkg/workspace/join_test.go, against real local git repos (a bare remote plus real clones, no mocks) -- confirmed these tests genuinely fail against the pre-fix code (including reproducing the exact "already a member" refusal on retry) before confirming they pass against the fix. Pending a batched release.

Cross-ref: CHANGELOG 0.14.10.

## [0.14.10] T-32 — ed find/roles find: preview truncation corrupts UTF-8 mid-codepoint on long multi-byte lines (v0.14.10, 2026-09-10)

Theme: bugfix · closed 0.14.10 · 2026-09-10
Trigger: reported via a re-tested repro against v0.14.9 (long runs of U+2591 block-drawing characters, decode failing at a mid-codepoint byte offset).
Scope: pkg/ed/ed.go's find/apply/append/prepend/insert/sub confirmation-message trims, and pkg/roles/roles.go's find command, all sliced preview/label text by raw byte count (s[:n]) rather than at a rune boundary -- confirmed to genuinely emit invalid UTF-8 on stdout when the cut point lands inside a multi-byte codepoint.

Progress: fixed via a shared truncateUTF8 helper (unexported in pkg/ed, plus an exported ed.TruncateUTF8 wrapper for other packages already importing pkg/ed) that backs up to the nearest rune boundary instead of cutting mid-codepoint. pkg/roles keeps its own local copy (roles sits below ed in the import graph, so it can't reuse ed.TruncateUTF8 without a cycle) -- pkg/badcode and pkg/gomod both reuse the exported form instead. Completion sweep found two more real, previously-undiscovered instances of the identical bug beyond the two originally reported (ed find, roles find): badcode.Check's own snippet truncation, at both its per-line and cross-line-fold match sites (pkg/badcode/badcode.go), and gomod's go-list-failure error-output truncation (pkg/gomod/gomod.go) -- all fixed the same way. Verified against the exact reported repro: the pre-fix binary genuinely produces invalid UTF-8 on this input (confirmed via a direct UTF-8 decode of the raw stdout bytes, not just a wrapping tool's error surface), the post-fix binary produces valid UTF-8 on the identical input; the two badcode sites were separately confirmed the same way (each test genuinely fails against pre-fix badcode.go with the exact expected incomplete-continuation-byte corruption, and passes against the fix). Regression coverage: pkg/ed/ed_test.go, pkg/roles/roles_test.go (property-testing truncateUTF8 against ASCII, the original repro, em-dashes, mixed-width runs, and four-byte emoji at every maxBytes from 0 to 130), and pkg/badcode/badcode_test.go (both truncation sites, using a byte-offset chosen so the cut genuinely lands mid-codepoint -- a pure run of one 3-byte character never triggers this at a length divisible by 3). Pending a batched release.

Cross-ref: CHANGELOG 0.14.10.

## [0.14.4] T-14 — Filed-by field parsing/validation on register items (v0.14.4, 2026-09-08)

Theme: workspace · closed 0.14.4 · 2026-09-08
Parse and validate a Filed-by field on a register item, naming the
originating project's issue (issue/<project>:<prefix>-<n>). Currently
this would only ever be free text in an item's body if written at
all -- no structured field, no validation that the referenced issue
id is well-formed. Depends on T-13 (the mechanism that actually
produces items carrying this field).

Cross-ref: CHANGELOG 0.14.4.

## [0.14.4] T-06 — wave membership as a queryable field on a register item (v0.14.4, 2026-09-08)

Theme: dependency-graph · closed 0.14.4 · 2026-09-08
Add wave membership as a queryable field on a register item itself,
not just implicit in WAVE_TRACKING.md's document layout. Confirmed
gap: register.ItemSummary carries id/status/priority/theme/title,
no wave -- a cross-wave dependency board has no structured way today
to know which wave an item belongs to without re-parsing prose.

Natural point to populate this: addwave already links an item via
register_item in --items-json; the wave number is known there and
should be written back onto the item.

Cross-ref: CHANGELOG 0.14.4.

## [0.14.3] T-13 — workspace issue/closure commands (CLI verb naming undecided) (v0.14.3, 2026-09-08)

Theme: workspace · closed 0.14.3 · 2026-09-08
Progress: implemented and tested end to end (newissue, resolveissue,
pauseissue, and a new dropissue) against a real local git remote,
including two real bugs caught by testing and fixed before landing:
a shell-escaping corruption of a newline byte, and an ID-reuse bug
where a resolved or dropped issue own number became reusable once its
file was deleted from the open directory -- fixed by deriving the
next id from the resolved/dropped logs too, not the open directory
alone. CLI naming resolved as subcommands, not flags. Pending a
batched release.

NAMING NOT DECIDED -- flagging rather than inventing silently. The
issue and closure JSON message shapes (gorepoman.workspace.issue/1,
gorepoman.workspace.closure/1) are fully designed, with queryfy
validation and the issue/<project>:<prefix>-<n> reference syntax
already settled. What was never settled: the actual CLI verb(s) that
produce and consume these messages -- unlike join/leave/list, no
command name was ever agreed for "file an issue into a workspace" or
"write a closure notice back." Candidates worth deciding between:
folding into repoman workspace (file-issue/close-issue subcommands),
or a separate repoman issue command. Depends on T-10 through T-12
existing first regardless of which name is chosen.

Cross-ref: CHANGELOG 0.14.3.

## [0.14.3] T-07 — register kanban view (single-project, status-grouped) (v0.14.3, 2026-09-08)

Theme: kanban · closed 0.14.3 · 2026-09-08
Progress: implemented and tested (repoman register list --view kanban).
The recorded Blocks/after: T-05 turned out not to reflect a genuine
technical requirement -- this needed only fixed, five-column status
grouping, none of the graph-parsing T-05 is actually about. Kept the
field as historical record rather than silently removed. Pending a
batched release.

Add a status-grouped kanban view as an option on the existing register
command (already decided: this is a register option, not a new
command). Today register --format html renders a flat list; this adds
a real, deterministic, queryfy-validated, embed.FS-templated grouped
layout alongside it, matching the same pattern already built for
syncver/gomod/guards/register/waveprogress.

No new command surface -- most likely a --view kanban (or similar)
flag alongside --format on register list.

Cross-ref: CHANGELOG 0.14.3.

## [0.14.3] T-19 — register close does not propagate to a linked item's WAVE_TRACKING.md checkbox (v0.14.3, 2026-09-08)

Theme: dependency-graph · closed 0.14.3 · 2026-09-08
register close (and register add, when it links a wave via
register_item) does not propagate to the linked item's own checkbox
in WAVE_TRACKING.md. Confirmed recurring, not a one-off: found and
hand-fixed for T-09/T-10/T-11/T-12 in one commit, then found again for
T-07 in the very next one, because closing T-07 through the normal
register close path did not touch its wave row either.

The register itself said this plainly: a checked-box hand-fix is
supposed to be the exception, filed the moment a pattern is spotted,
not repeated a third time by hand. This is that filing.

Scope: register close needs to look up whether the closed id appears
as a register_item value anywhere in WAVE_TRACKING.md, and if so
update that row's own checkbox in the same operation -- not as a
second manual step, and not only when someone happens to remember.

Cross-ref: CHANGELOG 0.14.3.

## [0.14.2] T-12 — repoman workspace join/leave/list (v0.14.2, 2026-09-07)

Theme: workspace · closed 0.14.2 · 2026-09-07
Progress: implemented and tested end-to-end against a real local git remote (join/leave/list all verified, remote state confirmed via independent clone), pending a batched release with T-10/T-11

repoman workspace join / leave / list -- decided command shape (one
command, three subcommands, not new top-level verbs, matching every
other multi-subcommand command in gorepoman). join writes to two
repos with two different credentials: the workspace's own
participants file (using a workspace-scoped credential), and the
local project's .repoman.json workspaces key (using the project's own
credential). leave is the mirror. list is read-only and takes
--format like every other reporting subcommand.

Cross-ref: CHANGELOG 0.14.2.

## [0.14.2] T-11 — workspace participants file + target validation (v0.14.2, 2026-09-07)

Theme: workspace · closed 0.14.2 · 2026-09-07
Progress: implemented and tested (pkg/workspace schemas, queryfy-validated), pending a batched release with T-10/T-12

Workspace participants file: a small versioned JSON object
(gorepoman.workspace.participants/1) naming which projects belong to
a workspace, used to validate an issue's target field before it is
written -- so a filed issue cannot be silently sent to an invalid
destination. Read-only from the filing project's side; membership
itself is managed by whoever administers that workspace repo.

Cross-ref: CHANGELOG 0.14.2.

## [0.14.2] T-10 — config: workspaces key in .repoman.json (name, remote, credential_env) (v0.14.2, 2026-09-07)

Theme: workspace · closed 0.14.2 · 2026-09-07
Progress: implemented and tested (Workspace struct + Workspaces field in pkg/config), pending a batched release with T-11/T-12

Config-side support for workspace membership: a workspaces key in
.repoman.json (name, remote, credential_env per workspace a project
belongs to). Decided: privacy of the metadata itself is not a
concern (open source or GitHub-permission-controlled), but
credential_env only ever names an environment variable -- the real
secret is provisioned separately per machine, never written into
this committed file, same principle as badcode's own config.

Cross-ref: CHANGELOG 0.14.2.

## [0.14.2] T-09 — board: configurable column labels / grouping axis (v0.14.2, 2026-09-07)

Theme: kanban · closed 0.14.2 · 2026-09-07
Progress: values/natural-order/auto-derive/overflow all implemented and
tested for status, priority, and theme axes. dependency_order is
explicitly rejected at render time -- it needs T-05/T-06 first, not
guessed at. Pending a batched release.

NEEDS A DESIGN DECISION BEFORE IMPLEMENTATION, not ready to build as
filed. Column labels (and possibly the grouping axis itself: status
vs dependency-order vs priority vs theme) should be configurable per
board -- but a board has no persisted identity today. board currently
takes bare positional directory paths and holds no state of its own.

Open questions to resolve first: where does a board's own
configuration live (a new file format? a flag-supplied manifest?);
does a board need a name/identity at all if it is meant to stay a
pure, regenerated projection of other data; what happens on a
dependency-order board deeper than the configured label sequence
(Tier 6 fallback, repeat the last label, or merge overflow into the
final column). Do not implement against assumed answers to these.

UPDATE 2026-09-07: most of the above is now resolved. Boards live in
the workspace repo (opt-in layer, not a requirement -- the plain,
no-workspace repoman board stays the free default). Overflow answers
the Tier-6 question: auto-created when needed, with a persisted
--hide/--unhide (matching waveprogress's own precedent, not a bare
field edit) and a separate --empty that clears its tracked-items
record with no data migration -- nothing is moved, since nothing
there was ever a second copy of real data. Legend is per-card notes
in a colour palette kept deliberately distinct from colours that
already carry fixed meaning (done/partial/not-started). Footer and
subtitle are optional hand-authored markdown, no dependency on any
computed mechanism. Default board is per-participant, recorded in the
workspace's own participants.json (default_boards), not in the board
definition itself -- confirmed: different participants may genuinely
want different default views of the same workspace. Schema
(gorepoman.workspace.board/1) and pkg/workspace config/join/leave/list
are implemented and tested (see T-10/T-11/T-12).

Still genuinely open, discovered while trying to implement rendering:
a column has a label and a note, but nothing says which underlying
data value it actually collects. For axis=status, does a column's
position imply which status symbol it holds, or does each column need
an explicit value field naming what it matches? This has to be
answered before rendering can be built -- not invented silently.

Cross-ref: CHANGELOG 0.14.2.

