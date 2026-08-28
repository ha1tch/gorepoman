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
$ python3 add_wave.py --name "connection pool rewrite" --ideal-days 3 \
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
$ python3 wave_progress.py --show
Wave 1  connection pool rewrite     ░░░░░░░░░░░░░░░░░░░░     0%  (0/2 items)

Overall by item count: 0 of 2 items ≈ **0%**
```

Visibility is a display concern, stored per-wave in `.repoman.json`, and
deliberately independent of the real total: hiding a wave from view doesn't
stop its work from counting toward `Overall`.

```
$ python3 wave_progress.py --hide 1
wave 1: hidden (persisted to .repoman.json)

$ python3 wave_progress.py --show
every wave is currently hidden -- pass --include-hidden to render anyway, or --unhide <id> to bring one back
```

That message goes to stderr with nothing on stdout — plain `--show` won't
silently print an empty table when every wave happens to be hidden, it says
so. `--include-hidden` renders everything regardless of persisted state,
for one invocation, without changing what's stored:

```
$ python3 wave_progress.py --show --include-hidden
Wave 1  connection pool rewrite     ░░░░░░░░░░░░░░░░░░░░     0%  (0/2 items)

Overall by item count: 0 of 2 items ≈ **0%**

$ python3 wave_progress.py --unhide 1
wave 1: visible (persisted to .repoman.json)
```

`--html PATH` renders the identical underlying data as a standalone HTML
document — genuinely standalone, inline styles, no dependency on an
external stylesheet — for presenting wave progress somewhere other than a
terminal:

```
$ python3 wave_progress.py --html out.html
wave_progress: wrote out.html (1 waves)
```

Both display forms — ASCII and HTML — read the exact same persisted
visibility state, so a wave hidden from one can never appear in the other
by accident; there's one source of truth for what's shown, not two
independently-tracked notions of it.
