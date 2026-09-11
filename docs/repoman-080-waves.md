# Staged-work tracking: `addwave` and `waveprogress`

The register (`repoman-060-register-and-guards.md`) tracks individual open
items. Waves are a different, complementary grouping: a staged programme of
work — "wave 1 is the pool rewrite, wave 2 is the metrics work that depends
on it" — with its own progress bars, item numbering, and a plan document
explaining *why* each wave exists. Register items can belong to a wave;
waves don't replace the register.

## `addwave`: wave and item numbers are computed, never supplied

Wave numbers and item numbers come from the actual current state of
`docs/WAVE_TRACKING.md` and `docs/WAVE_PLAN.md` — not typed in by hand —
specifically because a hand-typed number can collide with an existing
*soft* reservation (a wave number already mentioned in prose somewhere,
not yet a formal heading) that a naive "highest heading + 1" scan would
miss. `addwave` scans for both.

```
$ repoman addwave --name "connection pool rewrite" --ideal-days 3 \
    --items-json '[{"summary": "replace mutex with sync.Pool", "register_item": "T-02"}, {"summary": "add pool-exhaustion metric"}]' \
    --plan-note "Retry storms are exposing lock contention in the current pool; moving to sync.Pool removes the shared mutex entirely."
Wave number: 1  (computed; checked against headings + prose reservations)
Item numbers: 1-2

--- WAVE_TRACKING.md section ---
### Wave 1 — connection pool rewrite (2 items, ideal 3.0d, added 2026-08-28)

| # | Summary | Status | Register item |
|---|---|---|---|
| 1 | replace mutex with sync.Pool | ☐ | T-02 |
| 2 | add pool-exhaustion metric | ☐ | not yet filed |

**Wave 1: 0/2, not started.**

--- WAVE_PLAN.md paragraph ---
**Wave 1 — connection pool rewrite (≈ 3.0d, added 2026-08-28).** Retry storms are exposing lock contention in the current pool; moving to sync.Pool removes the shared mutex entirely.

wave_progress: regenerated (1 waves)
```

An item's `register_item` is optional — the second item here has none yet
("not yet filed"), and can be linked to a real register item later once
one's been filed for it. Item numbers are global and sequential across the
whole programme, never reused once assigned, continuing from the highest
existing row in any wave's table.

A project's very first `addwave` call needs no manual setup: if
`docs/WAVE_TRACKING.md` and `docs/WAVE_PLAN.md` (or whatever
`wave_tracking`/`wave_plan` in `.repoman.json` point at) don't exist
yet, `addwave` creates both with a minimal skeleton before adding the
wave, announced on stdout rather than done silently:

```
$ repoman addwave --name "first wave" --ideal-days 1 \
    --items-json '[{"summary": "needs a wave", "register_item": "T-01"}]' \
    --plan-note "fixture"
created docs/WAVE_TRACKING.md (was missing -- minimal skeleton)
created docs/WAVE_PLAN.md (was missing -- minimal skeleton)
Wave number: 1  (computed; checked against headings + prose reservations)
...
```

`--dry-run` still writes nothing to disk in this case — it previews
using the same skeleton content held in memory instead of reading it
back from files that were never created.

What `addwave` deliberately does *not* do: write the load-bearing prose.
`--plan-note` is a judgment call about why the wave exists and what it
depends on — written deliberately each time, not generated.

## `waveprogress`: rendering, hiding, and the HTML form

`--show` renders the same data as ASCII progress bars:

```
$ repoman waveprogress --show
Wave 1  connection pool rewrite     ░░░░░░░░░░░░░░░░░░░░     0%  (0/2 items)

Overall by item count: 0 of 2 items ≈ **0%**
```

Visibility is a display concern, stored per-wave in `.repoman.json`, and
deliberately independent of the real total: hiding a wave from view doesn't
stop its work from counting toward `Overall`.

```
$ repoman waveprogress --hide 1
wave 1: hidden (persisted to .repoman.json)

$ repoman waveprogress --show
every wave is currently hidden -- pass --include-hidden to render anyway, or --unhide <id> to bring one back
```

That message goes to stderr with nothing on stdout — plain `--show` won't
silently print an empty table when every wave happens to be hidden, it says
so. `--include-hidden` renders everything regardless of persisted state,
for one invocation, without changing what's stored:

```
$ repoman waveprogress --show --include-hidden
Wave 1  connection pool rewrite     ░░░░░░░░░░░░░░░░░░░░     0%  (0/2 items)

Overall by item count: 0 of 2 items ≈ **0%**

$ repoman waveprogress --unhide 1
wave 1: visible (persisted to .repoman.json)
```

`--html PATH` renders the identical underlying data as a standalone HTML
document — genuinely standalone, inline styles, no dependency on an
external stylesheet — for presenting wave progress somewhere other than a
terminal:

```
$ repoman waveprogress --html out.html
wave_progress: wrote out.html (1 waves)
```

Both display forms — ASCII and HTML — read the exact same persisted
visibility state, so a wave hidden from one can never appear in the other
by accident; there's one source of truth for what's shown, not two
independently-tracked notions of it.

**`--html PATH` is superseded by `--show --format html`** (writes to
stdout, same shape every other command's html output uses — see
`repoman-055-format.md`) — kept working, not removed, since real usage
may already depend on it writing a file directly:

```
$ repoman waveprogress --show --format html > out.html
```

`--show` and `--check` both take `--format json` too, for anything
reading wave progress mechanically:

```
$ repoman waveprogress --show --format json
{
  "tool": "waveprogress",
  "object": "waveprogress-report",
  "schema_version": 1,
  "data": {
    "waves": [
      {
        "id": "1", "name": "connection pool rewrite", "pct": 0,
        "has_partial": false, "done": 0, "total": 2,
        "debt": [], "blockers": []
      }
    ],
    "overall_done": 0, "overall_total": 2, "overall_pct": 0,
    "hidden_count": 0
  },
  "generated_at": "2026-08-30T14:00:00Z"
}
```

`debt` and `blockers` carry the same information the ASCII and HTML
views already show as extra lines under a wave — real arrays a
consumer can act on, not text to re-parse. `check --format json`
returns `{"stale": true|false}`, with the same non-zero-when-stale
exit code the text form has always used.

## Keeping a wave row's own status honest

A wave table row's Status cell (`✓`/`◐`/`☐`) is data written once, when
`addwave` creates the row — nothing updates it automatically just because
the linked register item's real status changes elsewhere, unless one of
the two mechanisms below runs.

**`register close` propagates directly.** Closing an item that a wave
row references via its Register item column updates that row's checkbox
in the same operation, and regenerates wave progress immediately after:

```
$ repoman register close T-02 --version 0.1.0
   close T-02 (record): RESOLVED.md updated
   close T-02 (register): TRACKING.md updated
wave_progress: regenerated (1 waves)
closed T-02 at v0.1.0. Remaining by hand: the CHANGELOG entry for 0.1.0 should cross-reference this closure (the changelog says what shipped; RESOLVED.md says what was wrong — they reference, never duplicate).
   wave: T-02 marked done in docs/WAVE_TRACKING.md and wave progress regenerated
```

This is the common path and needs nothing extra — closing through
`register close` keeps both documents in step by construction.

**`waveprogress` itself re-derives every row before rendering,
independent of how an item was closed.** A row can drift from reality
whenever an item's status changes some other way — `RESOLVED.md` gaining
a closure header by hand, for instance, bypassing `register close`
entirely. Every `waveprogress` invocation corrects this first, before
anything else runs: each row with a linked register item is checked
against `RESOLVED.md`'s own closure headers and `TRACKING.md`'s current
open rows, and rewritten to match before the summary line, progress bar,
or `Overall` percentage are computed from it.

```
$ cat >> docs/RESOLVED.md <<'EOF'

## [0.1.0] T-02 — thing (v0.1.0, 2026-09-10)

Theme: x · closed 0.1.0 · 2026-09-10
closed by hand, bypassing register close

Cross-ref: CHANGELOG 0.1.0.
EOF

$ repoman waveprogress
wave_progress: regenerated (1 waves)

$ grep "Wave 1:" docs/WAVE_TRACKING.md
**Wave 1: 1/1, done.**
```

`--check` sees this the same way it sees any other drift — a row that
needs correcting is stale, exactly like a summary line that needs
recomputing:

```
$ repoman waveprogress --check
wave_progress: wave-tracking document is stale -- run without --check to regenerate
```

A row whose linked id is found in **neither** document — not open in
`TRACKING.md`, not closed in `RESOLVED.md` — is left exactly as found,
and reported as a warning rather than guessed at:

```
$ repoman waveprogress
wave_progress: T-02 is linked from a wave row but found in neither TRACKING.md nor RESOLVED.md's closure headers -- row left as-is
wave_progress: already up to date
```

A row naming more than one id (a `T-1 through T-3` range) takes the
**least-done** member's status: all `✓` only if every member is `✓`,
`◐` if the members disagree or any one of them is only partially done,
`☐` only if literally none of them are closed.
