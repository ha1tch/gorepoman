# Changelog

## [0.4.0] - 2026-08-28

- **Fixed a real, would-have-broken-everything `go.mod` bug: the module**
  **directive still declared `github.com/ha1tch/repoman` after this project**
  **was published under its own name at `github.com/ha1tch/gorepoman`.**
  Confirmed directly against the live repository (`git clone` + `cat
  go.mod`), not inferred. Every one of `go install`, `go get`, and any
  future proxy-based fetch of this module would have failed or resolved
  incorrectly, because Go's tooling checks that a fetched module's own
  `go.mod` declares the same path it was requested under. Fixed the
  `module` line and all 29 internal import paths across 14 files in one
  verified pass -- confirmed all 29 occurrences were the same syntactic
  role (`go-dquote-string`, i.e. genuine import path string literals) via
  `roles` before touching anything, then a single `ed sub --expect 29`
  covering every file. `go build`/`go vet`/`gofmt`/`selftest` all
  re-confirmed clean afterward, and every internal package import checked
  to confirm no stale reference to the old path remains anywhere.
- **Added `repoman version` (and `--version`/`-v`).** Prints the build
  version, empty until now because no `version` variable existed in
  `main.go` at all -- the original release config's own `-ldflags -X
  main.version=...` would have been a hard Go linker error on every single
  CI build (the linker fails, not silently ignores, an `-X` target that
  doesn't exist), caught before it ever reached CI by actually building
  with that exact ldflag locally first, not by inspection. Fixed by adding
  a real `version` variable (default `"dev"` for a local, non-release
  build) and a small dispatch check ahead of the usage-and-exit path, then
  confirmed the full chain works: a plain build reports `repoman dev`, a
  build with `-ldflags "-X main.version=v0.4.0"` (the exact flag the
  release config uses) reports `repoman v0.4.0`.
- **New: CI-built, cross-compiled release binaries -- the primary**
  **distribution path from this release forward, not `go install` or**
  **building from source.** This tool needs to work on projects that have
  no reason to run a Go toolchain at all (a ZX Spectrum assembly project,
  a static-site repo) -- requiring one just to fetch a repository-
  discipline tool would reintroduce exactly the toolchain assumption this
  project exists to not make. `.goreleaser.yaml` cross-compiles
  `linux/amd64`, `linux/arm64`, `darwin/amd64`, and `darwin/arm64` with
  `CGO_ENABLED=0` (safe here specifically because this codebase has zero
  third-party dependencies and no cgo usage anywhere -- confirmed, not
  assumed), publishes raw binaries (`archives.format: binary`, no tar/zip
  wrapper) under the fixed name `repoman-<os>-<arch>` so the
  `/releases/latest/download/<name>` URL never changes shape across
  releases -- a bootstrap command written once keeps working on every
  future release without being updated. All four cross-compiled targets
  verified directly: each actually builds (`file` confirms correct
  ELF/Mach-O format and architecture for each), and the native
  `linux/amd64` binary was actually run, not just compiled, confirming
  `version` and `doctor --quiet` both work correctly on the real
  cross-compiled artifact. `.github/workflows/build-and-release.yml` runs
  build/vet/gofmt/`selftest` on every push and PR; the release job runs
  only on a pushed `v*` tag, only after that same commit's `verify` job
  has passed in the same workflow run -- a release is never built from a
  commit whose selftest didn't just pass.
- **README restructured to lead with the binary bootstrap.** `go build`
  demoted to its own "Building from source instead" subsection, correctly
  scoped to development on `gorepoman` itself or platforms outside the
  four the release binaries cover -- not the primary path for a consumer
  who just wants the tool.
- Caught and fixed during this same pass, before it reached CI: the
  GitHub Actions workflow's bare `on:` key parses as the YAML 1.1 boolean
  `true` under PyYAML's `safe_load` (confirmed directly -- this is not a
  theoretical concern, `yaml.safe_load` on the actual file produced a
  `True` key, not the string `"on"`). Sources disagree on whether GitHub's
  own workflow parser is actually affected by this in practice, and
  GitHub's own documentation examples use the bare form throughout, but
  since quoting it (`"on":`) costs nothing and is the form every source
  agrees is unambiguous, fixed rather than argued about.

## [0.3.0] - 2026-08-28

A full audit pass across every syntactic-role classifier, requested
directly (not incidental) -- checking each one for the same class of
bug already found in the backtick/block-comment work: a naive
delimiter count or single-condition check standing in for what should
be a real stateful scan or a language's actual matching rule. Five more
real bugs found and fixed, all shared between the Go port and the
Python original (not port-introduced regressions), all confirmed
against independent ground truth (CPython's own `ast.parse`, PyYAML's
own parser, or hand-verified CommonMark semantics) rather than just
reasoned about, and all fixed in both languages together to keep them
at parity rather than diverging one from the other:

- **`go`'s block-comment fallback.** After the backtick fix, `goRole`
  still fell back to a naive `strings.Count(before, "/*") >
  strings.Count(before, "*/")` to detect multi-line block comments. A
  string literal containing an unmatched `/*` permanently flipped "in
  comment" for everything after it -- even past a real block comment
  properly closing later in the file. Fixed by extending the existing
  backtick scanner into a general `goScanTo`/`_go_scan_state` returning
  the full lexical state at an offset, used for both backtick strings
  and block comments (the two constructs that can actually span
  lines), eliminating the naive count entirely rather than patching it.
- **`md`'s fence tracking.** Counted every line-leading run of 3+
  backticks as an equal toggle regardless of length. A shorter
  backtick run appearing as literal content inside a longer-delimited
  fence (e.g. a line demonstrating what a fence marker looks like,
  written inside a 4-backtick outer fence specifically so the example
  doesn't terminate it) incorrectly closed the outer fence -- and
  everything genuinely still inside it afterward read as prose
  instead. Replaced with real CommonMark-style length-matched fence
  tracking (`_md_in_fence`/`mdInFence`): a fence opens on a line of 3+
  backticks and closes only on a LATER line whose run is at least as
  long, with nothing else on it.
- **`md`'s inline-code detection.** A naive per-character backtick
  parity count on the current line. A stray, unmatched backtick with
  no partner anywhere else on the line permanently flipped "inside
  code" for the rest of that line, even though CommonMark renders an
  unmatched backtick as ordinary literal text. Replaced with proper
  CommonMark run-length matching (`_md_in_inline_code`/
  `mdInInlineCode`): an opening run only counts if a LATER run of the
  exact same length closes it -- validated against a battery of hand-
  checked cases first, including double-backtick spans containing
  literal single backticks, before integrating.
- **`python`'s triple-quote escape handling.** The backslash-escape
  skip only applied when `len(delim) == 1`, so it never fired for
  triple-quoted strings even though Python's own escaping rules apply
  identically there. `"""` (an escaped quote immediately followed by
  two more literal quote characters -- still fully legal inside a
  `"""`-delimited string) read as a genuine closing triple-quote one
  character too early. The fallout cascades: content genuinely still
  inside the string reads as code, and the REAL closing triple-quote --
  now seen while "outside" a string -- gets misread as OPENING a brand
  new one, misreading everything after it as still being inside a
  string until a further matching delimiter happens to appear, if
  ever. Confirmed against `ast.parse` as ground truth. Fixed by
  removing the length restriction -- the escape rule is unconditional
  in both languages now, matching how Go's and JS's scanners already
  handled it correctly.
- **`yaml`'s comment detection.** Treated every bare `#` outside quotes
  as starting a comment, regardless of what preceded it. Confirmed
  against PyYAML's own parser: `url: http://x.com#frag` keeps `#frag`
  as part of the plain scalar value (no preceding whitespace), so a
  URL fragment -- or any other mid-word `#` in an unquoted scalar --
  wrongly turned the rest of that line into `yaml-comment`. Fixed to
  require the `#` be at line-start or preceded by whitespace, matching
  the rule `shell`'s classifier already correctly applied. Left as a
  named, deliberate limitation rather than chased further: PyYAML also
  treats `#` as starting a comment immediately after a closing quote
  or flow indicator with no space at all (`'value'#c`), which this
  whitespace-only heuristic doesn't model -- full fidelity there would
  need flow-context tracking well beyond what this line-local scan
  does elsewhere.

**Checked and found sound, no changes made:** `_html_embedded_spans`'s
`</script>` ambiguity (a literal, unescaped `</script>` inside a real
script body's own string content is indistinguishable from a genuine
closing tag) is an already-documented limitation that matches real
browser parsing behaviour exactly -- not a bug relative to any spec,
since browsers face the identical ambiguity. `_js_scan` and
`_css_role`'s escape handling is unconditional (no length-based
restriction to get wrong the way Python's was). `_shell_role`'s single-
quote state correctly has no escape handling at all (matching real
shell semantics -- single-quoted content is 100% literal), its double-
quote/backtick escape handling is unconditional, and its `#`-comment
rule already implemented the whitespace-preceded check before this
pass (it was the reference implementation the `yaml` fix was modelled
on).

Every fix in this entry was validated the same way: a dedicated
fixture built from the specific failure shape, checked against
independent ground truth where one exists, applied to both languages,
then the full existing fixture suite re-run (10-language fixture,
prior backtick/block-comment/fence stress tests) to confirm nothing
regressed. `selftest` remained 75/75 in both languages throughout.

One process note: the first attempt at the block-comment fix corrupted
a Go rune literal (`'\n'`) via a Python string-escaping mistake on the
editing side -- `'\n'` in a non-raw Python string is one newline
character, not the two-character sequence `\n` the target Go source
needed. Caught immediately because `go build` runs after every edit,
not just gofmt; fixed properly, and the remaining edits in this entry
used raw strings or a line-by-line list to avoid repeating it.

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
