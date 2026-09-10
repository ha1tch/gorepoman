# Board theme specimen: poesy/zendis palette and card layout

Updated: 2026-09-09

## What this is

A checked-in static snapshot of the dependency-order board's current
default look, at `docs/proposals/assets/board-theme-specimen-2026-09-09.html`.
It is not live — the file will drift from the register the moment either
changes. That is deliberate, same principle as
`t08-board-reference-2026-09-09.html` in this same directory: a reference
for what a rendered board should look like, not a view kept in sync by
hand.

This specimen settled after several rounds of real feedback across two
sibling projects' own boards (poesy's wave board, zendis's pattern
board) and this repository's own dependency-order board, converging on
one shared visual language:

- **Palette tokens**: `--bg`, `--surface`, `--surface-2` (a sunken
  panel tone), `--border`, `--text`/`--text-2`/`--text-3` (three text
  weights), `--accent` (teal, used for in-progress/link state),
  `--done`/`--done-bg` (green, "unlocks X" and completion), `--wait`/
  `--wait-bg` (amber, "waits on X" and P1 urgency). Both light and dark
  variants defined on `:root`, redefined under
  `@media (prefers-color-scheme: dark)` guarded by
  `:root:not([data-theme="light"])`, and again under
  `:root[data-theme="dark"]` so an explicit toggle wins in both
  directions.
- **Type**: IBM Plex Sans for prose, IBM Plex Mono for anything
  identifier-shaped (ids, tags, stat numbers) — loaded from Google
  Fonts, the one external host the Artifact CSP admits for
  stylesheets.
- **Card anatomy**: a top row split left/right — priority badge and id
  on the left (`.card-top-left`), wave/context tag on the right; a
  title line; a wrapping tag row below carrying status tags (`waits`,
  `unlocks X`, `code complete, pending release`) as small colored
  pills, one color per semantic meaning rather than per column.
- **Column anatomy**: a heading with an item count, then a fixed-height
  note (`min-height` reserving two full lines) so columns whose note
  wraps differently still start their first card at the same row —
  this was a real bug, caught and fixed on this repository's own board
  once several columns' notes turned out to be different lengths.
- **Two palettes, one page**: this specimen keeps its own earlier
  "earth" palette (warmer, higher-contrast) reachable via a small
  top-right switcher and a `data-palette="earth"` attribute, so
  choosing this poesy/zendis palette as the default didn't require
  discarding the design work already done on the alternative.

## Using this specimen

When building or restyling a dependency-order, wave, or kanban-style
board for any project, start from this file's token set and card/column
structure rather than reinventing one. Swap the register data; keep the
token names, the card anatomy, and the fixed-height column note fix.
