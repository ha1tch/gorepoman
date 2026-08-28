# Changelog

## [0.2.0] - 2026-08-28

- **Fixed: `gomod --help` and `gomod check --help` — the worst of the**
  **`--help` gaps found across this whole effort.** `gomod --help` printed
  nothing useful, just the generic stderr usage line, exit 1 (Python's
  `gomod.py --help` shows a proper argparse subcommand help, exit 0).
  Worse: `gomod check --help` didn't fail at all — it silently treated
  `"--help"` as the `path` positional argument and actually ran the real
  check against a directory literally named `--help`, which doesn't exist,
  producing a confusing tool-availability error instead of help text. Found
  during a systematic `--help`-exit-code sweep across all 12 commands and
  every subcommand, added specifically because this exact tool had already
  been glanced at and wrongly assumed fine earlier in this effort — its
  generic usage line happened to look enough like real help text to pass a
  casual check. Fixed both: top-level `-h`/`--help` now short-circuits
  before the `args[0] != "check"` dispatch, and `check`'s own arg loop
  checks for `-h`/`--help` before treating anything as the path. Confirmed
  real `gomod check` operation (clean pass and a real absolute-replace
  failure) still works correctly afterward, and `selftest` remains 75/75.

- **Fixed a real, shared bug in `roles`'s Go-string classifier (both**
  **languages, not just this translation).** `goRole`/`_go_role` tracked
  backtick raw-string state with a naive whole-file backtick count. Any
  backtick appearing inside a `//`/`/* */` comment, a rune literal
  (`'`'`), or a double-quoted string (`"`"`) earlier in the file flips
  that count's parity without actually opening or closing a raw string,
  misclassifying everything after it for the rest of the file — in
  either direction: plain code inside another string can misclassify as
  `go-backtick-string`, and text genuinely inside a real raw string can
  misclassify as *not* being in one. Caught because `roles.go` was
  misclassifying its own "roles present" `Printf` line as
  `go-backtick-string` when it's actually a `go-dquote-string`. Replaced
  the naive count with a real stateful forward scan (`inGoBacktickString`
  in Go, `_in_go_backtick_string` in Python) that tracks line comments,
  block comments, rune literals, and double-quoted strings so backticks
  inside any of those are correctly ignored. Fixed identically in the Go
  port and in the Python original (`roles.py`) to keep both correct and
  in parity — verified with a dedicated 6-case stress fixture exercising
  both the false-positive and false-negative directions of the bug, plus
  the original self-referential case; all match between languages and
  match hand-verified expected output. `selftest` remains 75/75 green in
  both after the fix (Python's own gate too, not just the Go port's).
- **Completed `addwave --help`.** Previously fell through to the same
  generic "missing required flags" usage message as calling it with no
  arguments (exit 1). Now prints the full argparse-equivalent help text
  — including the design-rationale prose from the Python original's
  docstring — and exits 0, matching `add_wave.py --help`.
- **Fixed: `addwave`'s final confirmation line used the wrong item-range**
  **format for single-item waves.** For a wave with one item, Go printed
  `items 1`; Python's `add_wave.py` always prints the `start-end` range
  form (`items 1-1`) in this specific message, even when they're equal —
  an inconsistency with its own earlier "Item numbers: 1" line (which
  *does* collapse), but Go should still match Python's actual behaviour
  rather than "fix" an inconsistency Python itself has. Multi-item waves
  already matched (`items 2-4`) — the code was reusing one range-string
  variable for two messages that use different formatting rules. Found
  and fixed via a direct side-by-side single-item vs multi-item test, not
  caught by `selftest` (which only exercises multi-item waves).
- **Fixed: `guards`, `relcore`, and `register` all crashed on `--help`**
  instead of showing usage — the most severe of the `--help` gaps found
  so far, since these three load real project state (`.repoman.json`,
  `docs/KNOWN_ISSUES.md`, `docs/TRACKING.md`) *before* dispatching on the
  subcommand, so `--help` failed with a file-not-found error even outside
  any repoman-enabled repository. Fixed all three (and every one of
  their subcommands — `list`/`show`/`handoff`/`record`/`stale` for
  `guards`; `list`/`show`/`add`/`close`/`check` for `register`) to check
  for `-h`/`--help` before any project-state loading, matching the Python
  originals' argparse help text and exiting 0.
- **Directly verified (not just via `selftest`) that `syncver`,**
  **`waveprogress`, and `addwave` match the Python originals exactly** —
  the three tools flagged last round as selftest-only-verified.
  `syncver`: `show`/`set`/`check` (both in-sync and mismatch)/all three
  `bump-*` variants/unknown-command, all byte-identical output and exit
  codes. `waveprogress`: `--show`/`--hide`/`--unhide`/`--include-hidden`,
  plus a byte-for-byte diff of generated `--html` output — identical.
  `addwave`: single- and multi-item waves, with resulting
  `WAVE_TRACKING.md`/`WAVE_PLAN.md`/`.repoman.json` diffed byte-for-byte
  — identical (aside from the range-formatting bug above, now fixed).
- **`gofmt -w` applied across the whole tree.** `gofmt -l` had flagged 9
  files: `main.go`/`gomod.go`/`syncver.go`/`extract.go` were pre-existing
  (missing trailing newline, present since before this translation
  effort started, not introduced by any change here); the other 5 were
  indentation drift introduced by this round's own hand-authored edits
  (`ed.go`, `guards.go`, `register.go`, `relcore.go`, `addwave.go`).
  Whitespace-only; confirmed via `selftest` (still 75/75) and re-running
  the `roles` fixture comparison that nothing else shifted.

## [0.1.0] - 2026-08-28

- **Initial versioned baseline of the Go translation.** Prior to this, the
  translation had no `VERSION`, `CHANGELOG.md`, `README.md`, or `LICENSE` at
  all — this release adds all four, matching the Python original's GPLv3.0
  licensing (a translation of GPLv3-licensed code carries the same license;
  it is not a new, independently-licensed work).
- **Fixed: five subcommands rejected `-h`/`--help` outright** instead of
  showing usage, matching the Python original's argparse behaviour
  everywhere except `syncver.py`/`syncver` (a pre-existing, faithfully-
  reproduced quirk broken identically in both languages, left as-is
  rather than silently diverging from the original) and `roles.py`/`roles`
  (a positional-args search tool by design in both languages -- `--help`
  is treated as a literal search term, not a bug). Two severities found:
  - `ed` and `strreplace`: every subcommand printed `unknown command:
    --help` and exited 1.
  - `guards`, `relcore`, and `register`: worse -- these tools load their
    project state (`.repoman.json`, `docs/KNOWN_ISSUES.md`,
    `docs/TRACKING.md`) *before* dispatching on the subcommand, so
    `--help` crashed with a file-not-found error instead of showing
    usage, even outside any repoman-enabled repository.
  Fixed all five (and every one of their subcommands) to check for
  `-h`/`--help` before any project-state loading, printing usage text
  matching the Python original's argparse output and exiting 0. Verified
  individually against every subcommand of all five tools; `selftest`
  remains 75/75 green throughout, and normal (non-help, non-crashing)
  operation of `guards list`, `ed find/apply/sub`, etc. was re-confirmed
  against live fixtures after each fix.
- **Noted, not fixed (would break parity, not restore it):** `addwave`'s
  `--help` in Go falls through to the same generic usage-on-missing-
  required-flags message as calling it with no arguments at all (exit 1,
  no positional/option descriptions), where the Python original's
  `add_wave.py --help` prints a full argparse help block with the
  module's design-rationale docstring and exits 0. Lower severity than
  the crashes above -- it doesn't error, it's just far less informative
  -- and reproducing the Python docstring's full prose in Go felt like
  scope creep for this pass; flagging for a follow-up.
- Confirmed via a full 75-check `selftest` cross-comparison and direct
  side-by-side behavioural testing (identical fixtures, both binaries) that
  `ed` (find/apply/sub/mark/log/undo), `register` (list/check/close), and
  `gomod` (all 7 checks including the go.mod replace-directive and go.sum
  completeness gates) match the Python original's behaviour exactly, output
  formatting aside (Go's `%q` uses double quotes where Python's `repr()`
  uses single quotes — cosmetic only).
- `roles` has no dedicated coverage in the shared `selftest` suite in either
  language, and hasn't yet had a direct side-by-side comparison pass here;
  flagged in the README as provisional pending that.
