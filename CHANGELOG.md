# Changelog

## [0.7.0] - 2026-08-28

- **Fixed `register add` refusing on a genuinely empty table** (header +
  separator, zero data rows) with the misleading message "cannot locate
  the status table" -- confirmed against source: the row-detector only
  ever searched for rows matching the ID pattern, so a table with zero
  such rows produced zero matches and the same refusal as no table at
  all, even though a perfectly valid table was right there. This was
  already a known, previously-documented limitation (the docs described
  a "genesis row" workaround), never actually fixed at the source until
  now. Fixed: when no ID-matching row exists, `add` now anchors after the
  table's own header ("| ID |...") and separator line instead, so a
  brand-new register with a real header but no rows yet works with no
  seed row required. Confirmed against the exact previously-failing
  case, confirmed `register check` passes on the result, and confirmed
  no regression against the existing (genesis-row-based) test fixtures --
  full selftest still 86/86.
- **`docs/repoman-030-getting-started.md` had a real, substantial gap**
  **missed in the earlier documentation-fix pass**: its entire "Install"
  section and the worked `doctor` example were still 100% Python
  (`Requirements: Python 3.10...`, a sample `doctor` run showing
  `[OK] Python 3.12.3`), inconsistent with the command syntax elsewhere
  in the same file, which had already been fixed. Found while actually
  rendering the doc into HTML for the new Pages mirror below -- reading
  the rendered output surfaced what a syntax-only sweep had missed.
  Rewrote the install section (binary-first, matching the README),
  the `doctor` explanation and sample output (Go version, not Python),
  and a remaining bare `python3 str_replace_extended.py selftest`
  reference. Swept the other nine docs for the same class of staleness;
  the one remaining `.py` reference found (`repoman-090-configuration.md`'s
  example release-gate config, `"run": "python3 scripts/my_gate.py"`) is
  a legitimate generic example of a user's own custom script, not a
  claim about gorepoman itself, and was left as-is.
- **New: a GitHub Pages mirror of the release binaries and documentation**
  (`.github/workflows/pages.yml`, `tools/build_site.py`), built the same
  way as the ZSP project's own site (`actions/upload-pages-artifact` +
  `actions/deploy-pages`, matching an already-proven pattern rather than
  inventing a new one). This exists for one specific reason: an agent
  that can reach the binary well enough to be doing anything at all may
  still, for reasons that were never fully diagnosed (network policy,
  a transient issue, restricted tool permissions in that specific
  sandbox), be unable to reach `github.com/releases` or search-discover
  the repository's own docs. The Pages site is a genuinely independent
  access path -- a different host, plain static file GETs, no
  release-asset redirect through `api.github.com` -- mirroring the exact
  same binaries (built via `make cross`, the same target already used
  and tested locally) and all ten docs, rendered to HTML. Triggered on
  the same tag push the real release fires on, deliberately not on every
  push to main, so the mirror always reflects an actual tagged release,
  never an untested main-branch build passed off as one. README and the
  `repoman-tools` skill both updated to name the mirror as a fallback.

## [0.6.1] - 2026-08-28

- **Fixed a real, confirmed gap in `strreplace`'s atomic guarantee: when a**
  **file's syntax could not be independently verified at all (validator**
  **returns "unverifiable", not "verified and bad"), `atomic: true` did**
  **not refuse -- it silently proceeded to write the file anyway,** exactly
  as if validation had passed. Root-caused via `v.Validated.(bool)`'s own
  Go semantics: a `nil` interface value fails that type assertion (`ok ==
  false`), so the refusal branch -- gated on `ok && !vb` -- was skipped
  entirely for the unverifiable case, not just the verified-bad case it
  was written for. Confirmed against a real field incident, not a
  synthetic worry: a toolchain-free bootstrap (the `gorepoman` binary
  itself needs no Go toolchain to run -- that was always the point of
  shipping a static binary -- but `gofmt`, used to independently verify
  `.go` file edits, still does) hit this exactly, twice, in two
  independent sessions. "gofmt unavailable" is not a rare misconfiguration
  in that scenario -- it is the *normal* state of any toolchain-free
  bootstrap on a Go project, which means this bug was not an occasional
  flake; it would have silently broken the atomic guarantee on `.go`
  files every single time someone followed the tool's own documented,
  intended install path (binary only, no toolchain) on a Go repository.
  Fixed: atomic mode now explicitly refuses (`cls: "syntax-unverifiable"`)
  when a touched file's syntax cannot be independently checked, with a
  message naming the fix (install the needed tool, or set `atomic:false`
  to proceed without the guarantee) rather than silently downgrading a
  strong promise to a weaker one nobody asked for. Non-atomic mode's
  existing, deliberately different contract (best-effort: drop what can't
  be verified, keep the rest) is untouched -- confirmed unaffected by
  direct testing, not just left alone on the assumption it would be.
  New selftest check 15 covers this specifically, using a test-only
  override (`REPOMAN_TEST_FORCE_NO_GOFMT`) rather than a conditional
  skip, since `WhichGofmt`'s own fallback paths do a real filesystem stat
  on fixed locations that no environment-variable manipulation alone can
  reliably hide -- meaning a naive conditional test (matching the
  existing check 6's own pattern) would silently never exercise the real
  code path on any host that happens to have gofmt installed, which is
  most development and CI machines, including the one this was built and
  tested on.
- Verified the CI-published binary release infrastructure itself is not
  the problem here and needs no fix: the documented bootstrap command
  (`curl -L .../releases/latest/download/repoman-linux-amd64`) resolves
  and runs correctly, and even a plain shallow `git clone` surfaces the
  release tag via `git tag -l`. A session concluding "no tagged release
  published yet" during this investigation was not describing a real
  infrastructure gap.

## [0.6.0] - 2026-08-28

- **`relcore` now runs `badcode` automatically, unconditionally, as the**
  **literal first thing it does on every invocation** -- before checking
  whether `release.steps` even exists, before any step runs, including on
  `--resume`. This corrects a real design gap in 0.5.0: `badcode` shipped
  as a standalone command the operator had to remember to wire into their
  own release manifest as an ordinary `run` step, which meant a release
  could still happen without it ever running -- optional-if-remembered,
  not the hard gate it was meant to be. The fix is deliberately NOT a
  `release.steps` entry: anything expressible in `.repoman.json` can be
  edited or removed by anyone with repo access, which would defeat the
  entire point of a check that's supposed to be unconditional. The
  pre-flight is not part of the resumable-steps journal at all, so there
  is nothing for `--resume` to skip -- confirmed directly: a real match
  blocks identically on a fresh run and a `--resume` run of the exact
  same release. There is no flag, environment variable, or config key
  anywhere in this codebase that disables it; the only lever that exists
  is the local badcode config itself, which was already the intended
  design -- patterns are configurable, whether the check runs is not.
  No config at all remains a soft pass (unchanged from 0.5.0's own
  design), with the `WARN` visible in the release's own output rather
  than suppressed.
- Added 4 new selftest checks covering this integration specifically,
  using a synthetic project separate from the shared fixture the rest of
  the suite depends on (real match blocks before any step runs; `--resume`
  does not bypass it; removing the matched content lets the release
  proceed; no-config soft pass still lets a release through) -- 86 checks
  total, up from 82.

## [0.5.0] - 2026-08-28

- **Added `repoman badcode check [path ...]`** -- a release-blocking scan
  for forbidden text strings, loaded from a local config that is
  deliberately never stored in this or any repository (a blocklist
  committed alongside the code it protects can be edited by whoever has
  commit access, including an agent, in the same change it was meant to
  catch -- keeping it out-of-band is the entire point). Config:
  `$REPOMAN_BADCODE_DIR` or the OS user-config directory, two optional
  files (`badcode.txt`, one pattern per line; `badcode.json`, adds an
  optional per-pattern `reason` included in the refusal message).
  Literal case-insensitive substring matching, not regex -- deliberately,
  since this check's entire value is that it can't fail in a way that
  looks like success, and regex adds failure modes a plain substring
  search doesn't have. Binary files are skipped (a coincidental byte
  match inside a compiled artifact is noise, not a real leak). No config
  at all is a soft pass, not a hard failure, but says so plainly via a
  `WARN` line rather than looking identical to a real, clean check --
  matching this project's own dormant-guards principle that a check
  which never actually ran proves nothing.
- Directory-walking and `.git` exclusion reused directly from `roles.Expand`
  rather than reimplemented.
- Caught and fixed during the same pass, before it shipped: `repoman
  badcode --help` (bare, no subcommand) initially fell through to the
  generic usage-error path instead of showing help -- the exact same
  class of top-level-`--help`-gated-behind-a-subcommand-match bug found
  and fixed in `guards`/`relcore`/`register`/`gomod` earlier in this
  project's history. Found by actually running the bare `--help` form,
  not just the documented `badcode check --help` form.
- Added 7 new selftest checks covering `badcode` (clean pass with both
  config files loaded, a real case-insensitive match with the JSON
  `reason` field surfaced, correct file/line reporting, binary-file skip,
  no-config soft pass, and the `--help` regression) -- 82 checks total,
  up from 75. No Python equivalent exists for this command, so unlike
  every other section in this suite it has nothing to differentially
  test against; the fixture-based checks are the only verification.

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

- **Added the full `repoman-000` through `repoman-090` documentation set**
  **under `docs/`** -- why the tool exists, failure modes it prevents,
  getting started, precise editing, the role classifier, the register and
  dormant guards, releases, staged-wave tracking, and the full
  `.repoman.json` reference. Originally written and verified against the
  Python original for real, ground-truth output; before landing here,
  every command-line invocation and every shown output line was checked
  against this Go binary specifically, not assumed to carry over --
  `roles` itself was used to classify every occurrence of each Python
  script name across the docs first, which correctly flagged that 5 of 12
  names span multiple syntactic roles (command examples mixed with prose
  references to the script by name) and can't be blindly substituted.
  Real output differences found and fixed this way: `ed`'s
  apply/mark/undo confirmation messages use double quotes in Go where
  Python's used single (`"maxRetries"` vs `'maxRetries'`); `register
  check`'s mismatch error uses Go's bracketed `[a b c]` formatting, not
  Python's `('a', 'b', 'c')` tuple repr; and the `strreplace` section
  needed a real rewrite, not a substitution -- Go has no equivalent of
  Python's "import `apply_payload` as a library" pattern, so that whole
  workflow (and its JSON key ordering, which differs between the two
  languages' JSON marshaling) was redone against the actual Go CLI.
- **Added a `Makefile`** -- `build`, `run`, `selftest`, `vet`, `fmt`/
  `fmt-check`, `verify` (the full local gate matching what CI runs),
  `cross` (cross-compiles all four release targets into `dist/` with a
  `checksums.txt`, using the same matrix as `.goreleaser.yaml`), and
  `release-dry-run`. `release-dry-run` deliberately does not
  auto-install `goreleaser` if it's missing -- it prints instructions and
  exits, rather than silently pulling a large dependency tree the way an
  earlier session did by mistake. Every target was actually run, not just
  written: `make cross` reproduced the identical four checksums from the
  original manual cross-compile pass, confirming the build is
  deterministic.

- **Added Windows to the release matrix -- six targets now, not four:**
  `linux-amd64`, `linux-arm64`, `darwin-amd64`, `darwin-arm64`,
  `windows-amd64.exe`, `windows-arm64.exe`. Checked for real portability
  risk before touching any release config, not assumed safe just because
  `CGO_ENABLED=0` cross-compilation succeeds: found two places in
  `guards.go` building paths via manual `root + "/" + ...` string
  concatenation, inconsistent with every other package in this codebase
  (which uses `filepath.Join` throughout). Fixed both to use
  `filepath.Join`. Confirmed the Windows cross-compile succeeded both
  before and after that fix -- the manual concatenation wasn't actually
  broken on Windows (the OS accepts forward slashes), but leaving it
  inconsistent with the rest of the codebase's own convention wasn't
  right either. `.goreleaser.yaml`'s `archives.name_template` now adds
  `.exe` conditionally (`{{ if eq .Os "windows" }}`) -- present only for
  Windows, absent for the other four, so the same `/releases/latest/
  download/` bootstrap pattern still holds. `Makefile`'s `cross` target
  updated identically (same conditional suffix logic, plain shell this
  time). All six targets actually built via `make cross`, not just
  configured: `file` confirms correct PE32+/ELF/Mach-O format and
  architecture for each, matching what `.goreleaser.yaml` will produce
  from the same source.

- **Added the BSD family to the release matrix -- thirteen targets now,**
  **not six:** FreeBSD, OpenBSD, and NetBSD (amd64 and arm64 each), plus
  DragonFly BSD (amd64 only -- confirmed via `go tool dist list` that Go
  has no `dragonfly/arm64` port at all, not a choice made here).
  `.goreleaser.yaml`'s build matrix now excludes that one combination
  explicitly (`ignore: [{goos: dragonfly, goarch: arm64}]`) rather than
  assuming a uniform `goos` x `goarch` product would just work.
  Checked for OS-specific code before touching any config, not assumed
  safe: `doctor`'s own `runtime.GOOS` switch already falls through
  correctly to an honest "unconfirmed platform" report for anything it
  doesn't specifically recognise (already-correct, already-documented
  behaviour, not something that needed changing). All thirteen targets
  actually cross-compiled via `make cross`, not just added to config --
  `file` confirms correct format and architecture for every one. One
  genuinely interesting, verified-not-assumed finding along the way: the
  two OpenBSD binaries come out dynamically linked against
  `/usr/libexec/ld.so`, unlike all eleven other targets. Checked against
  Go's own issue tracker before treating this as expected rather than a
  build problem -- this is long-standing, intentional Go-on-OpenBSD
  behaviour (OpenBSD's W^X/pledge exec-protection model requires it),
  and that loader ships as part of every real OpenBSD install, not an
  extra dependency the binary would need someone to separately provide.

- **Fixed a real bug shipped in the actual v0.4.0 release: both Windows**
  **assets were uploaded as `repoman-windows-amd64.exe.exe` and**
  **`repoman-windows-arm64.exe.exe`** -- double extension, not caught
  before release because the actual `goreleaser` binary was never run
  end-to-end in this sandbox (installing it pulls a large dependency
  tree -- see the earlier incident this project's own working agreement
  was written to prevent repeating). The `.goreleaser.yaml`
  `archives.name_template` explicitly appended `.exe` for Windows
  (`{{ if eq .Os "windows" }}.exe{{ end }}`); GoReleaser itself already
  appends `.exe` automatically and unconditionally to every Windows
  binary under `format: binary`, regardless of what `name_template`
  produces -- the two combined. Confirmed against a matching, independent
  report of the identical behaviour (`goreleaser/goreleaser#1221`) before
  reapplying the fix, not just inferred from the symptom. Fix: removed
  the explicit `.exe` from `name_template` entirely -- GoReleaser's own
  automatic suffixing is sufficient and was always going to fire either
  way. Caught because the person running the actual release read the
  real GitHub Releases page and reported the exact asset names back,
  which is what surfaced this -- worth naming plainly: this specific
  class of bug is invisible to everything this project's own local
  checks (`make verify`, `make cross`) can catch, since neither of them
  invokes GoReleaser itself, only plain `go build`. A real
  `goreleaser release --snapshot --skip=publish --clean` dry run (the
  `make release-dry-run` target exists for exactly this) would have
  caught it before any release, and remains the honest gap until it's
  actually run once.

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
