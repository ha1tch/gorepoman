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
