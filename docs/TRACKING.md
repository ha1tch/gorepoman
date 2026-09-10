# Tracking

| ID | Summary | Theme | Priority | Status | Blocks |
|----|---------|-------|----------|--------|--------|

| T-01 | provenance check: detect out-of-band edits to journal-tracked files | provenance | P1 | ☑ | — |
| T-02 | provenance sanction: explicit override with mandatory reason | provenance | P1 | ☑ | T-01 |
| T-03 | wire provenance into relcore, unconditional alongside badcode | provenance | P1 | ☑ | T-2 |
| T-04 | badcode hits annotated with provenance status | provenance | P2 | ☑ | T-1 |
| T-05 | parse Blocks/after into real graph edges, with cycle detection | dependency-graph | P2 | ☑ | — |
| T-08 | board: cross-project dependency-order tiering | kanban | P2 | ☑ | T-05,T-06 |
| T-15 | relcore orchestrates GoReleaser as a resumable step | release-hardening | P3 | ☐ | — |
| T-16 | canonical CI workflow: badcode + selftest as a required PR check | release-hardening | P3 | ☐ | — |
| T-17 | roles backed by a real structural matcher (research spike) | release-hardening | P3 | ☐ | — |
| T-18 | guards: distinguish never-rerun from code-moved-since (underdesigned) | release-hardening | P3 | ☐ | — |
| T-20 | ed: append/prepend verbs, no handle required (file-boundary insertion) | ed-editing | P2 | ☑ | — |
| T-21 | ed: insert verb, handle-verified positional insertion | ed-editing | P2 | ☑ | — |
| T-22 | ed: pending-ticket storage design for niplines two-phase flow | ed-editing | P2 | ☐ | — |
| T-23 | ed: niplines request/preview (phase 1) with gofmt/vet preflight | ed-editing | P2 | ☐ | T-22 |
| T-24 | ed: niplines confirm/cancel (phase 2), TTL enforcement | ed-editing | P2 | ☐ | T-23 |
| T-25 | docs: repoman-040-editing.md coverage for insert/append/prepend/niplines | ed-editing | P3 | ☐ | T-20,T-21,T-23,T-24 |
| T-26 | Claimed-by: optional register-item field for advisory work claims | locking | P2 | ☐ | — |
| T-27 | register claim/release: write and clear Claimed-by, journaled | locking | P2 | ☐ | T-26 |
| T-28 | stale-claim policy: TTL or override for an abandoned Claimed-by | locking | P2 | ☐ | T-26,T-27 |
| T-29 | surface claims on register/board views (kanban, board, board --definition) | locking | P3 | ☐ | T-26 |
| T-30 | board --format json fails validation on every axis: legend field cannot be null | kanban | P2 | ☑ | — |
| T-31 | gate ed/strreplace write paths' own reads with a provenance check | provenance | P1 | ☑ | — |

## provenance

### T-01. provenance check: detect out-of-band edits to journal-tracked files

Theme: provenance · Priority: P1 · Status: ☑

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

### T-02. provenance sanction: explicit override with mandatory reason

Theme: provenance · Priority: P1 · Status: ☑ · Blocks/after: T-01

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

### T-03. wire provenance into relcore, unconditional alongside badcode

Theme: provenance · Priority: P1 · Status: ☑ · Blocks/after: T-2

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

### T-04. badcode hits annotated with provenance status

Theme: provenance · Priority: P2 · Status: ☑ · Blocks/after: T-1

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

### T-31. gate ed/strreplace write paths' own reads with a provenance check

Theme: provenance · Priority: P1 · Status: ☑

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

## dependency-graph

### T-05. parse Blocks/after into real graph edges, with cycle detection

Theme: dependency-graph · Priority: P2 · Status: ☑

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

## kanban

### T-08. board: cross-project dependency-order tiering

Theme: kanban · Priority: P2 · Status: ☑ · Blocks/after: T-05,T-06

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

### T-30. board --format json fails validation on every axis: legend field cannot be null

Theme: kanban · Priority: P2 · Status: ☑

Confirmed pre-existing (reproduced against the unmodified status axis before writing any T-05/T-08 work): repoman board --definition ... --format json fails with "error: validation failed: data.legend: field cannot be null" regardless of axis. definitionReportSchema's legend field (pkg/board/definition_schema.go) is declared without .Required(), but queryfy's strict-mode Object() schema still rejects a literal JSON null for def.Legend when it arrives as a nil Go slice (workspace.Board.Legend, unset in a minimal definition). text and html formats are unaffected and were used to verify T-05/T-08's own dependency_order work end to end. Likely fix: either make Legend non-nullable at the schema level (empty array default) or give definitionReportSchema's legend field an explicit nullable allowance -- needs a decision, not guessed at here, since the same shape appears in board/schema.go's own envelope and possibly elsewhere any optional array field is validated this way.

Progress: implemented and verified. Root cause confirmed as diagnosed above: DefinitionReport.Legend (pkg/board/definition.go) has json tag "legend" with no omitempty, unlike DefinitionItem.Legend which already has omitempty -- a nil Legend therefore marshals to JSON null and queryfy's strict-mode schema rejects it even without .Required(). Fixed by normalizing a nil Legend to an empty []workspace.BoardLegendEntry{} at both DefinitionReport construction sites (RenderDefinition, for status/priority/theme axes, and renderDependencyOrder, for dependency_order), the same discipline Columns/Overflow.Items already followed a few lines below in the same file. Verified live against two independently constructed definition files -- the dependency_order definition used for T-05/T-08's own board, and a fresh minimal status-axis definition -- confirming both RenderDefinition and renderDependencyOrder code paths are fixed, not just the one this item's own diagnosis happened to reproduce against. pkg/selftest/section26.go's own doc comment, which had documented this as a still-open known gap and deliberately skipped --format json, is corrected and now carries a real regression check: the same linear-chain fixture already used for the text/html checks, re-run with --format json, asserting the envelope parses, legend decodes as an empty array rather than null, and the tiered column/item structure matches the text/html checks exactly (190/190 selftest checks green, up from 188). Pending a batched release with T-05/T-08/T-20/T-21/T-31.

## release-hardening

### T-15. relcore orchestrates GoReleaser as a resumable step

Theme: release-hardening · Priority: P3 · Status: ☐

Have relcore orchestrate GoReleaser: a release.steps entry that runs
goreleaser release as a resumable step. Closes a real, confirmed
irony -- gorepoman has no .repoman.json / release.steps of its own at
all (fixed by this same wave plan) and ships itself via GoReleaser +
git tags directly, bypassing relcore's own badcode-gate-first,
journaled-resume discipline for the one release that matters most to
this project.

### T-16. canonical CI workflow: badcode + selftest as a required PR check

Theme: release-hardening · Priority: P3 · Status: ☐

Ship a canonical CI workflow (a maintained GitHub Actions file, the
way Gitleaks ships one) running badcode and selftest as a required,
unbypassable PR check. Confirmed gap: badcode currently only runs
locally, as part of relcore -- advisory in the same sense the mature
secret-scanning research calls pre-commit hooks advisory, since
nothing stops a PR from merging without anyone having run it.

### T-17. roles backed by a real structural matcher (research spike)

Theme: release-hardening · Priority: P3 · Status: ☐

RESEARCH SPIKE, not a small feature -- scope accordingly. Back roles'
classification with a real AST-aware structural matcher (ast-grep- or
comby-class), rather than the current hand-rolled regex classifier,
while keeping ed's own session/journal/undo layer on top -- that
layer is gorepoman's actual differentiator and has no equivalent in
ast-grep or comby, neither of which offers a stateful, reversible
editing session. This is explicitly about replacing the matching
engine underneath, not the workflow built on it.

### T-18. guards: distinguish never-rerun from code-moved-since (underdesigned)

Theme: release-hardening · Priority: P3 · Status: ☐

UNDERDESIGNED, no concrete mechanism proposed yet -- needs a design
pass before this can be scoped into a real item. guards currently has
no way to distinguish "this guard has not been re-run since the last
release" from "the code this guard covers has changed since it was
last exercised" -- both currently look identical (a last-exercised
date older than the release date). The second case is arguably more
urgent than the first, since a pass recorded against code that no
longer exists is not real evidence of anything.

## ed-editing

### T-20. ed: append/prepend verbs, no handle required (file-boundary insertion)

Theme: ed-editing · Priority: P2 · Status: ☑ · Wave: 7

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

### T-21. ed: insert verb, handle-verified positional insertion

Theme: ed-editing · Priority: P2 · Status: ☑ · Wave: 7

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

### T-22. ed: pending-ticket storage design for niplines two-phase flow

Theme: ed-editing · Priority: P2 · Status: ☐ · Wave: 7

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

### T-23. ed: niplines request/preview (phase 1) with gofmt/vet preflight

Theme: ed-editing · Priority: P2 · Status: ☐ · Wave: 7 · Blocks/after: T-22

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

### T-24. ed: niplines confirm/cancel (phase 2), TTL enforcement

Theme: ed-editing · Priority: P2 · Status: ☐ · Wave: 7 · Blocks/after: T-23

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

### T-25. docs: repoman-040-editing.md coverage for insert/append/prepend/niplines

Theme: ed-editing · Priority: P3 · Status: ☐ · Wave: 7 · Blocks/after: T-20,T-21,T-23,T-24

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

## locking

### T-26. Claimed-by: optional register-item field for advisory work claims

Theme: locking · Priority: P2 · Status: ☐ · Wave: 8

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

Value shape: a free-text agent/session identifier, no enforced
syntax beyond non-empty (unlike Filed-by's `<project>:<prefix>-<n>`
shape, a claim identifies a worker, not another register's ticket).

### T-27. register claim/release: write and clear Claimed-by, journaled

Theme: locking · Priority: P2 · Status: ☐ · Wave: 8 · Blocks/after: T-26

New subcommands `register claim <id> --by <who>` and `register
release <id>` writing/clearing the `Claimed-by` field from T-26,
journaled the same way `register add`/`close` already are (no raw
file writes). `claim` on an item already claimed by someone else
refuses by default (optimistic, not a hard lock -- see T-28 for why);
`claim` on an item already claimed by the SAME `--by` value is a
no-op success, not a refusal (idempotent re-claim, e.g. a session
resuming after a restart). `release` on an unclaimed item is a no-op
success, not a refusal.

Depends on T-26 (the field itself must exist and parse before a
command can write it).

Explicitly NOT in scope here: nothing about this enforces exclusivity
on the underlying file edits an agent makes while working the item --
`ed apply`'s own SpanHash staleness check is the only thing that
actually prevents two concurrent writers from corrupting the same
file, and that mechanism is unrelated to and unaffected by this. A
claim is a courtesy signal other agents can check before starting
work, not a lock on the filesystem.

### T-28. stale-claim policy: TTL or override for an abandoned Claimed-by

Theme: locking · Priority: P2 · Status: ☐ · Wave: 8 · Blocks/after: T-26,T-27

Design and document a stale-claim policy: what happens when a claim's
owning agent/session never releases it (crashed, timed out, was
never told to). A claim with no expiry is worse than no claim at all
-- it would eventually block every other agent from an item nobody
is actually working, and nothing here should be able to deadlock the
register.

Candidate shapes to decide between, not yet chosen: (a) a TTL on the
Claimed-by field itself (a timestamp alongside the identifier,
`register claim` refuses only within the TTL window, expires
automatically after); (b) no TTL, `--force` always available to any
caller (claims are advisory only, never actually block, purely
informational); (c) TTL plus explicit `register claim --force`
override that logs the override same as workspace `dropissue`'s
mandatory `--reason` does. Whatever is chosen must never require a
human in the loop to unstick the register -- that would make claims
worse than the no-claims status quo.

Depends on T-26/T-27 existing first (nothing to make stale until the
field and commands are real).

### T-29. surface claims on register/board views (kanban, board, board --definition)

Theme: locking · Priority: P3 · Status: ☐ · Wave: 8 · Blocks/after: T-26

Surface claims on every existing report/board surface that already
renders per-item state, so a claim is actually visible where an agent
would look before picking up work -- not a fifth thing to separately
query. `register list --view kanban`, plain `board`, and
`board --definition` (any axis, including the new dependency_order
axis from T-05/T-08) all already render one card/row per item;
add the claim (when present) as a small marker consistent with how
Wave badges and status symbols already render, no new column.

Depends on T-26 (field must exist) and, for the board surfaces
specifically, does not depend on T-05/T-08 being done first --
dependency_order is one axis among several this already needs to
work on (status/priority/theme), so this item does not block or get
blocked by the boards work.

