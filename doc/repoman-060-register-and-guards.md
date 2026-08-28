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
$ python3 register.py add \
    --summary "connection pool leaks under retry storm" \
    --theme "store" --priority P2 \
    --body "**Trigger:** found while load-testing the retry path.

**Scope:** pool.go's checkout/checkin pair; a checkout that errors before checkin never returns its slot."
   add T-01: TRACKING.md updated
filed T-01; run `register.py check` — and remember the status table and field lines must not diverge

$ python3 register.py list
T-0  ☐  P4  [meta]  genesis
T-01  ☐  P2  [store]  connection pool leaks under retry storm

$ python3 register.py close T-01 --version 0.2.0
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

### `check` catches real inconsistency, not just malformed files

`check` verifies the status table and the per-item detail sections
structurally agree with each other — not merely that the file parses.
Deliberately editing just the table's priority column, leaving the detail
section's own `Priority:` field untouched:

```
$ python3 register.py check
ERROR [A3] T-0: table ('meta', 'P1', '☐') vs detail ('meta', 'P4', '☐')
REGISTER CHECK FAIL: 1 error(s)
```

This is the check that a real bug in `register.py close` itself once
slipped past — a row-capture regex matching only five of six table cells,
orphaning the status row on closure — found while closing the very item
filed to track something unrelated. `check` runs against a
deliberately-broken fixture in `selftest`, specifically so it's proven to
*catch* a real mismatch, not just pass on well-formed data.

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
$ python3 guards.py stale --since 2026-08-01
STALE (not exercised since 2026-08-01):
  G-01   last=2026-06-01  concurrent-writer race guard (`store_race_test.go`)
Each must be run, handed off (guards.py handoff), or its skip recorded explicitly in the release's changelog entry.
```

Once the guard has actually been run somewhere that could exercise it,
`record` updates its entry — and only `record` updates it; the date is
never hand-edited:

```
$ python3 guards.py record G-01 --date 2026-08-28 --env "ci-linux-x64" \
    --note "ran under go test -race, clean"
G-01 recorded: 2026-08-28 env:ci-linux-x64 — ran under go test -race, clean Previous: 2026-06-01 env:m1-macbook

$ python3 guards.py stale
all guards exercised since 2026-08-01
```

The record is only evidence if it reflects a real run. Coverage of one
guard is never inferred from another guard's recorded campaign, and a
guard whose gating hardware genuinely isn't available in-session gets
handed to a human with the exact invocation rather than marked current on
the strength of a partial or simulated run.

## Where this feeds into a release

A release gate — `repoman-070-releases.md` covers `relcore` itself — checks
both of these together: the register contains no closed (✓) items left
behind, `RESOLVED.md` has changed append-only since the last tag, and every
dormant-guard entry has either been exercised since the last tag or has its
skip explicitly recorded in that release's notes. Neither tool is
release-blocking on its own; the release gate is what makes the two of them
matter at the moment it actually counts.
