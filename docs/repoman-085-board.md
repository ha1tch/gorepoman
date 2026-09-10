# Cross-project status: `board`

The five reporting commands (`register`, `guards`, `syncver`, `gomod`,
`waveprogress`) each answer "what's the state of *this* project."
`board` answers a different question — "what's the state of *these*
projects, together" — and it does that without owning any new
parsing. It shells out to this same binary's own `register list
--format json` and `waveprogress --show --format json` against each
project directory in turn, and merges the resulting envelopes. The
five commands already own the schema and the parsing for their own
data; `board`'s only job is enumerate, fetch, merge, present.

## Scope, on purpose

Local directories only, given as positional arguments — no manifest
file naming a fixed project list, no network fetch of a remote
project's register. This is deliberately the smaller, simpler half of
a larger design (see `repoman-055-format.md` for the flag every
reporting command shares) — a persistent list of projects, and
fetching a project that isn't checked out locally, are real, separate
questions left open rather than answered by assumption here.

`board` also does not merge in anything from a cross-project workspace
issue — a request one project has filed against another, not yet
picked up. That mechanism doesn't exist as running code yet, so
there's nothing real for `board` to merge from there. This is
distinct from workspace *membership* and board *definitions*, both of
which are real (see `repoman-086-workspace.md` and `--definition`
below) — a board definition can live in a workspace repo as a shared,
opt-in layer, but `board` itself does no workspace-aware fetching: you
still point `--definition` at a local file path, whether that file
came from a plain directory or a workspace clone. When cross-project
issue merging does land, that'll be an addition to what `board` reads,
not a change to how it works.

## What it actually does, end to end

Each directory needs its own `.repoman.json`, exactly as if you'd `cd`
there and run the commands yourself — `board` doesn't share or infer
configuration across projects, each one is independent:

```
$ repoman board /path/to/regfixture /path/to/wavefixture
regfixture  (/path/to/regfixture)
  T-01   ☐  P1  [durability]  Wave apply drops trailing shard
  waves: no wave-tracking document at /path/to/regfixture/docs/WAVE_TRACKING.md -- nothing to do (set wave_tracking in .repoman.json if waves are wanted)

wavefixture  (/path/to/wavefixture)
  register: no open items
  waves: 75% overall
```

Two genuinely different projects, shown honestly as they are: the
first has open register items but no waves configured; the second has
an empty register but an active wave. Neither absence is treated as a
failure of the command itself — a project not using one of these
mechanisms is a normal, expected state, not an error to work around.

## `--format` works exactly as it does everywhere else

`board` takes the same flag every reporting command does
(`repoman-055-format.md`) — `text` (above), `json`, or `html`. The
envelope is the same shape, just with `board`/`board-report` as its
`tool`/`object`:

```
$ repoman board /path/to/regfixture /path/to/wavefixture --format json
{
  "tool": "board",
  "object": "board-report",
  "schema_version": 1,
  "data": {
    "projects": [
      {
        "name": "regfixture",
        "path": "/path/to/regfixture",
        "register_error": "",
        "open_items": [
          {"id": "T-01", "status": "☐", "priority": "P1", "theme": "durability", "title": "Wave apply drops trailing shard"}
        ],
        "wave_error": "no wave-tracking document at /path/to/regfixture/docs/WAVE_TRACKING.md -- nothing to do (set wave_tracking in .repoman.json if waves are wanted)",
        "wave_overall_pct": 0,
        "waves": []
      },
      {
        "name": "wavefixture",
        "path": "/path/to/wavefixture",
        "register_error": "",
        "open_items": [],
        "wave_error": "",
        "wave_overall_pct": 75,
        "waves": [
          {"id": "1", "name": "Report format", "pct": 75, "has_partial": true, "done": 1.5, "total": 2, "debt": ["T-05"], "blockers": []}
        ]
      }
    ]
  },
  "generated_at": "2026-08-31T13:19:06Z"
}
```

`register_error` and `wave_error` are always present, empty string on
success — a consumer checks these two fields directly rather than
inferring success from whether `open_items`/`waves` happens to be
non-empty, which would conflate "genuinely nothing open" with
"couldn't be read at all." `open_items` reuses `register.ItemSummary`'s
own shape field-for-field (`repoman-060-register-and-guards.md`);
`waves` reuses `waveprogress.WaveStatus`'s (`repoman-080-waves.md`) —
the same schema each command already validates its own output
against, not a third shape invented for the merged view.

`--format html` renders the same data as a self-contained document, in
the same visual family as every other command's html output.

## `--definition FILE`: a configured, single-project board

The plain `repoman board dir [dir ...]` mode above is the free
default — a flat per-project view, nothing to opt into. `--definition
FILE` layers a *configured* board on top of the same gathered data:
grouped columns, an axis to group by, an overflow bucket, and an
optional legend. It still needs at least one project directory
argument (the definition doesn't name projects itself; `scope` inside
it can narrow which of the given projects are actually shown):

```
$ repoman board . --definition dep-order.json --format text
```

A board definition is a plain JSON file (schema
`gorepoman.workspace.board/1`) — it can live anywhere: a local file
next to the definition's own project, or checked out from a shared
workspace repo (`repoman-086-workspace.md`) so several projects can
point at the same one. `board` itself doesn't know or care which —
`--definition` just takes a path.

```json
{
  "schema": "gorepoman.workspace.board/1",
  "name": "dep-order",
  "title": "Dependency order",
  "scope": {"projects": []},
  "axis": "dependency_order",
  "columns": [
    {"label": "Now"},
    {"label": "Next"}
  ],
  "overflow": {"hidden": false}
}
```

`scope.projects` (default: every project given on the command line)
narrows to a named subset. `title`/`subtitle`/`footer` are optional,
hand-authored markdown — nothing here is auto-generated from them.

### Axes

Four, each grouping items by a different field:

- **`status`** — the register's own five-symbol status (☐/◐/☑/✓/✗).
- **`priority`** — P1 through P4 (or however many priority levels the
  register actually uses).
- **`theme`** — the register's free-text theme field.
- **`dependency_order`** — computed, not a stored field: tier(item) =
  1 + max(tier of each open blocker named in `Blocks/after`), or 0 for
  anything unblocked. Parsed from the same free-text field `register`
  already reads, via `pkg/register/graph.go`'s cycle-detecting graph
  builder. A blocker that's closed (absent from the open-items graph
  entirely) does not hold its dependent back — it's treated as already
  resolved, the rule a person tracing this by hand would use. A real
  cycle — including a one-item self-loop — refuses the whole render
  with the concrete path named:

  ```
  $ repoman board . --definition dep-order.json
  error: dependency cycle detected: T-80 -> T-81 -> T-80
  ```

### Columns and Overflow

**`columns`** (optional) is the sequence of buckets to render, each
`{"label", "note", "values"}`. `values` names which of the axis's own
field values this column collects, letting several underlying values
merge into one column. For `status`/`priority`, omitting `values`
falls back to that axis's own natural order by position; `theme` has
no natural order, so a theme column must name its values explicitly —
enforced at validation time, not discovered mid-render. For
`dependency_order`, `values` must stay empty: **position IS the
value** — column *i* collects tier *i*, nothing to name.

Anything matching no configured column — or, for `dependency_order`,
deeper than the columns configured — lands in **`overflow`** rather
than disappearing or silently growing an unconfigured column. When a
definition supplies no `columns` at all, one column is auto-derived
per value (or, for `dependency_order`, per tier) actually *present* in
the data — not per every value the axis could theoretically hold,
regardless of use. `overflow.hidden` (default `false`) controls
whether the bucket renders when empty; `overflow.tracked` persists
across renders the same way wave visibility does — it's display state,
never a second copy of item data.

### Legend

`legend` (optional) attaches a named, coloured note to any card whose
data matches `applies_when` — a simple field:value match (e.g.
`{"status": "☑"}`). Colours come from a palette (`cyan`, `purple`,
`orange`, `brown`, `blue`) deliberately separate from the colours that
already carry fixed meaning elsewhere (green=done, amber=partial,
grey=not-started), so a legend note is never mistaken for one of
those.

## Exit codes: a status view, not a gate

`board` always exits `0` for a normal run, regardless of what any
individual project reports — a project with no wave tracking
configured, or with open register items, is not a failure of `board`
itself, only real data being shown honestly. It exits non-zero only
for a genuinely bad invocation: no directories given, or a path that
isn't a directory at all. This is different from `register check` or
`guards stale`, which exist specifically to fail a CI gate — `board`
exists to be looked at, not to gate anything.
