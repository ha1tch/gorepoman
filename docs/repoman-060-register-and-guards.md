# The register and dormant guards

Two related tools for the same underlying problem: keeping a claim about
the state of the project — "this is still open," "this test still passes"
— honest over time, rather than trusting that it was true once and assuming
it stayed that way. See `repoman-020-failure-modes.md` #4–#6 for the
incidents these were built against.

## The register: `docs/TRACKING.md` and `docs/RESOLVED.md`

`TRACKING.md` holds **open items only** — a status table at the top, detail
sections below grouped by theme. `RESOLVED.md` holds closed items,
**append-only, newest first**, with the full detail text preserved verbatim
as it stood at closure. An item's presence in `TRACKING.md` after it's been
closed is itself a defect; a closed item never gets a tombstone left behind
in the live register.

One thing worth knowing before your first `add`: **a brand-new register
needs one seed row already in the table** for `add` to anchor new rows
after — it inserts each new row immediately after the last existing one
matching the ID pattern, so an empty table (header only, no rows) has
nowhere to anchor. A genesis row solves this cleanly:

```markdown
| ID | Summary | Theme | Priority | Status | Blocks |
|---|---|---|---|---|---|
| T-0 | genesis | meta | P4 | ☐ | — |
```

with a matching `### T-0. genesis` detail section below it, and a
corresponding first entry in `RESOLVED.md` if you want the "register
created" moment recorded there too. `close` has the same requirement of
`RESOLVED.md` — it needs at least one existing `## ` entry to insert the
next closure after.

### Adding and closing an item — real output, start to finish

```
$ repoman register add \
    --summary "connection pool leaks under retry storm" \
    --theme "store" --priority P2 \
    --body "**Trigger:** found while load-testing the retry path.

**Scope:** pool.go's checkout/checkin pair; a checkout that errors before checkin never returns its slot."
   add T-01: TRACKING.md updated
filed T-01; run `register.py check` — and remember the status table and field lines must not diverge

$ repoman register list
T-0  ☐  P4  [meta]  genesis
T-01  ☐  P2  [store]  connection pool leaks under retry storm

$ repoman register close T-01 --version 0.2.0
   close T-01 (record): RESOLVED.md updated
   close T-01 (register): TRACKING.md updated
closed T-01 at v0.2.0. Remaining by hand: the CHANGELOG entry for 0.2.0 should cross-reference this closure (the changelog says what shipped; RESOLVED.md says what was wrong — they reference, never duplicate).
```

After closing, `T-01` is gone from `TRACKING.md` entirely — row and detail
section both — and its full detail text now lives in `RESOLVED.md`,
stamped with the closing version and date:

```markdown
## [0.2.0] T-01 — connection pool leaks under retry storm (v0.2.0, 2026-08-28)

Theme: store · closed 0.2.0 · 2026-08-28

**Trigger:** found while load-testing the retry path.

**Scope:** pool.go's checkout/checkin pair; a checkout that errors before checkin never returns its slot.

Cross-ref: CHANGELOG 0.2.0.
```

`close` does the mechanical move correctly and completely — but it
explicitly doesn't write your changelog entry for you, and says so. That
cross-reference is a judgment call about what the release actually shipped,
left to you rather than generated.

### The field line: required fields, and four optional ones

Every item's detail section carries one field line directly under its
heading:

```
Theme: <theme> · Priority: <Pn> · Status: <symbol>
```

Four more segments are each independently optional, appended in this
order when present — an item can carry any subset of them, including
none:

```
Theme: <theme> · Priority: <Pn> · Status: <symbol> · Wave: <n> · Filed-by: <ref> · Blocks/after: <ids> · Claimed-by: <who>
```

| Segment | Written by | Shape | Meaning |
|---|---|---|---|
| `Wave: <n>` | `register add --wave`, or `addwave` linking an existing item into a new wave | a bare wave number | which staged-work wave (`repoman-080-waves.md`) this item belongs to |
| `Filed-by: <ref>` | `register add --filed-by` | `<project>:<prefix>-<n>`, e.g. `xoluman:T-1234` — validated, not free text; a leading `issue/` is tolerated on input and stripped | a cross-project reference: this item exists because another project's own register pointed at it |
| `Blocks/after: <ids>` | `register add --blocks` | a comma-separated list of item IDs, free text otherwise (not validated against the register) | dependency ordering for `board --definition ... --axis dependency_order` (`repoman-085-board.md`) — this item is blocked until every ID listed here closes |
| `Claimed-by: <who>` | `register add --claimed-by` | free text, unrestricted | advisory only: who has picked up this open item. Nothing in `board`, tier computation, or `RenderDefinition`'s axis dispatch reads it — it exists purely so a reader (or `register list --view kanban`, below) can see who's on something, without making that claim a dependency of anything |

`Filed-by` and `Claimed-by` look similar (both name an actor) but differ
in kind: `Filed-by` is a structured, validated pointer used for real
cross-project bookkeeping; `Claimed-by` is unvalidated advisory text with
no cross-project meaning at all. Passing a bare name to `--filed-by` is
refused outright, naming the expected shape:

```
$ repoman register add --summary "x" --theme store --priority P2 \
    --filed-by "on-call rotation" --body "Trigger: x.

Scope: y."
--filed-by: "on-call rotation" is not a cross-project reference -- expected <project>:<prefix>-<n>, e.g. xoluman:T-1234
```

A worked example combining several of these — `--wave`/`--filed-by` on one
item, `--blocks`/`--claimed-by` on a second that depends on it:

```
$ repoman register add --summary "connection pool leaks under retry storm" \
    --theme store --priority P2 --wave 3 --filed-by "xoluman:T-1234" \
    --body "Trigger: found while load-testing the retry path.

Scope: pool.go's checkout/checkin pair; a checkout that errors before checkin never returns its slot."
   add T-01: TRACKING.md updated
filed T-01; run `repoman register check` — and remember the status table and field lines must not diverge

$ repoman register add --summary "retry backoff needs jitter" \
    --theme store --priority P3 --blocks T-01 --claimed-by "on-call" \
    --body "Trigger: same investigation.

Scope: backoff.go."
   add T-02: TRACKING.md updated
filed T-02; run `repoman register check` — and remember the status table and field lines must not diverge
```

producing:

```markdown
### T-01. connection pool leaks under retry storm

Theme: store · Priority: P2 · Status: ☐ · Wave: 3 · Filed-by: xoluman:T-1234

Trigger: found while load-testing the retry path.

Scope: pool.go's checkout/checkin pair; a checkout that errors before checkin never returns its slot.

### T-02. retry backoff needs jitter

Theme: store · Priority: P3 · Status: ☐ · Blocks/after: T-01 · Claimed-by: on-call

Trigger: same investigation.

Scope: backoff.go.
```

The status table's own `Blocks` column and each item's `Blocks/after:`
field line are the same fact recorded twice, deliberately — `check`
(below) verifies they never diverge.

### The five statuses

```
☐  not started
◐  partial / in progress
☑  code-complete, held for a batched release (this project's own
   convention: "done" but not yet closed to RESOLVED.md — see
   `repoman-070-releases.md`'s closure procedure)
✓  done and closed (only ever appears briefly, between finishing and
   `register close` actually moving the item to RESOLVED.md)
✗  dropped
```

`register add --status` accepts any of these (default `☐`); an unrecognized
value is refused rather than silently accepted:

```
$ repoman register add --summary "x" --theme store --priority P4 --status X \
    --body "Trigger: x.

Scope: y."
invalid status "X"; use one of {✓ ◐ ☐ ✗ ☑}
```

### `register list`: flat, or grouped by status (`--view kanban`)

The default `list` prints one line per item, sorted the same deterministic
way every other view does:

```
$ repoman register list
T-0  ☐  P4  [meta]  genesis
T-01  ☐  P2  [store]  connection pool leaks under retry storm
T-02  ☐  P3  [store]  retry backoff needs jitter
```

`--view kanban` groups the same items into the five status columns above
— always all five, even when a column is empty, since an empty "Dropped"
column is itself useful information:

```
$ repoman register list --view kanban
☐ Not started (2)
  T-0    P4  [meta]  genesis
  T-02   P3  [store]  retry backoff needs jitter

◐ In progress (1)
  T-03   P3  [store]  in progress example

☑ Complete, pending release (1)
  T-01   P2  [store]  connection pool leaks under retry storm

✓ Done (0)

✗ Dropped (0)
```

`--view kanban` works with every `--format` value, same as the default
list — `--format json` returns the same five columns as structured data
(`"object": "register-kanban"`), each column's `items` an array of the
same per-item summary `list`'s own json form uses, so a consumer doesn't
need two different item shapes depending on which view it asked for:

```
$ repoman register list --view kanban --format json
{
  "tool": "register",
  "object": "register-kanban",
  "schema_version": 1,
  "data": {
    "columns": [
      {"status": "☐", "label": "Not started", "items": [...]},
      {"status": "◐", "label": "In progress", "items": [...]},
      {"status": "☑", "label": "Complete, pending release", "items": [...]},
      {"status": "✓", "label": "Done", "items": []},
      {"status": "✗", "label": "Dropped", "items": []}
    ]
  },
  "generated_at": "2026-08-30T14:00:00Z"
}
```

### `register add`'s full flag set

```
repoman register add [-h] [--id ID] [--id-prefix PREFIX] --summary SUMMARY
                      --theme THEME --priority PRIORITY [--status STATUS]
                      [--wave WAVE] [--filed-by FILED_BY] [--blocks BLOCKS]
                      [--claimed-by CLAIMED_BY] [--body BODY]
                      [--body-file BODY_FILE] [--dry-run]
```

`--summary`/`--theme`/`--priority` and one of `--body`/`--body-file` are
the only required inputs. `--id` overrides the next-free-id-in-namespace
default with an explicit ID (useful for a genesis row, or restoring an ID
from history); `--id-prefix` picks which configured
[id namespace](repoman-090-configuration.md) to allocate from when a
project has more than one live namespace (`id_namespaces` in
`.repoman.json`) — omit it for the primary namespace. `--body-file` reads
the body from a file instead of the command line, for a body too long or
too awkward to quote inline:

```
$ repoman register add --summary "body from file" --theme store --priority P4 \
    --body-file trigger-and-scope.txt
   add T-04: TRACKING.md updated
filed T-04; run `repoman register check` — and remember the status table and field lines must not diverge
```

`--dry-run` computes and prints what would change without writing
anything:

```
$ repoman register add --summary "dry run example" --theme store --priority P4 \
    --dry-run --body "Trigger: x.

Scope: y."
   (dry-run) add T-04: TRACKING.md not written
```

### `check` catches real inconsistency, not just malformed files

`check` verifies the status table and the per-item detail sections
structurally agree with each other — not merely that the file parses.
Deliberately editing just the table's priority column, leaving the detail
section's own `Priority:` field untouched:

```
$ repoman register check
ERROR [A3] T-0: table [meta P1 ☐] vs detail [meta P4 ☐]
REGISTER CHECK FAIL: 1 error(s)
```

This is the check that a real bug in `register.py close` itself once
slipped past — a row-capture regex matching only five of six table cells,
orphaning the status row on closure — found while closing the very item
filed to track something unrelated. `check` runs against a
deliberately-broken fixture in `selftest`, specifically so it's proven to
*catch* a real mismatch, not just pass on well-formed data.

### Scripted consumption: `--format`

`list`, `show`, and `check` all take `--format` (`repoman-055-format.md`)
for anything reading the register mechanically rather than a person
reading a terminal:

```
$ repoman register check --format json
{
  "tool": "register",
  "object": "register-check",
  "schema_version": 1,
  "data": {
    "errors": [
      "[A3] T-0: table [meta P1 ☐] vs detail [meta P4 ☐]"
    ],
    "open_items": 2,
    "ok": false
  },
  "generated_at": "2026-08-30T14:00:00Z"
}
```

`show <id> --format json` returns the item's raw section text alongside
its id and title (`{"id": ..., "title": ..., "block": "..."}`) — the
same text `show`'s default form prints, structured rather than requiring
a consumer to re-parse `### T-01. ...` headings out of plain text.

## Dormant guards: `docs/KNOWN_ISSUES.md`

A dormant guard is any check that doesn't run in the default test
invocation — build-tagged tests, environment-gated tests, anything needing
specific hardware, anything deferred to local execution. Each one gets an
entry: gate condition, exact invocation, and a last-exercised date and
environment.

```markdown
### G-01. concurrent-writer race guard (`store_race_test.go`)

- **Gate:** build tag `stress`
- **Invocation:** `go test -tags stress -run TestConcurrentWriters ./pkg/store/...`
- **Last exercised:** 2026-06-01 env:m1-macbook
```

`stale` checks every guard's last-exercised date against a cutoff — either
given explicitly with `--since`, or defaulting to the previous release's
own changelog date:

```
$ repoman guards stale --since 2026-08-01
STALE (not exercised since 2026-08-01):
  G-01   last=2026-06-01  concurrent-writer race guard (`store_race_test.go`)
Each must be run, handed off (guards.py handoff), or its skip recorded explicitly in the release's changelog entry.
```

Once the guard has actually been run somewhere that could exercise it,
`record` updates its entry — and only `record` updates it; the date is
never hand-edited:

```
$ repoman guards record G-01 --date 2026-08-28 --env "ci-linux-x64" \
    --note "ran under go test -race, clean"
G-01 recorded: 2026-08-28 env:ci-linux-x64 — ran under go test -race, clean Previous: 2026-06-01 env:m1-macbook

$ repoman guards stale
all guards exercised since 2026-08-01
```

The record is only evidence if it reflects a real run. Coverage of one
guard is never inferred from another guard's recorded campaign, and a
guard whose gating hardware genuinely isn't available in-session gets
handed to a human with the exact invocation rather than marked current on
the strength of a partial or simulated run.

### Scripted consumption: `--format`

`list`, `show`, and `stale` all take `--format` (`repoman-055-format.md`):

```
$ repoman guards stale --since 2026-08-01 --format json
{
  "tool": "guards",
  "object": "guards-stale",
  "schema_version": 1,
  "data": {
    "since": "2026-08-01",
    "stale": [
      {"id": "G-01", "title": "concurrent-writer race guard (`store_race_test.go`)", "last_exercised": "2026-06-01"}
    ],
    "all_current": false
  },
  "generated_at": "2026-08-30T14:00:00Z"
}
```

`stale --format json`'s exit code still carries the same meaning as the
text form's — non-zero whenever `all_current` is `false` — so it slots
into a release-gate script exactly like `stale` always has.

## Where this feeds into a release

A release gate — `repoman-070-releases.md` covers `relcore` itself — checks
both of these together: the register contains no closed (✓) items left
behind, `RESOLVED.md` has changed append-only since the last tag, and every
dormant-guard entry has either been exercised since the last tag or has its
skip explicitly recorded in that release's notes. Neither tool is
release-blocking on its own; the release gate is what makes the two of them
matter at the moment it actually counts.
