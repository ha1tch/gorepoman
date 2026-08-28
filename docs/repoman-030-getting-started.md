# Getting started

## Install

No toolchain required -- it's a single static binary:

```
curl -L https://github.com/ha1tch/gorepoman/releases/latest/download/repoman-linux-amd64 -o repoman
chmod +x repoman
```

Swap `linux-amd64` for the platform in use -- see the README for the full
table (Linux/macOS/Windows/FreeBSD/OpenBSD/NetBSD/DragonFly, amd64 and
arm64). Building from source instead requires Go 1.21 or later and no
third-party dependencies: `go build -o repoman ./cmd/repoman`.

A handful of optional external tools unlock *better* validation for
specific file types but nothing requires them -- see `doctor` below.

## Run `doctor` first

```
repoman doctor
```

This is an environment diagnostic, not a pass/fail test. It reports the Go
version this binary was compiled with, the current platform, and which of
four optional external tools this environment has — `gofmt`, `bash`, `node`, PyYAML — each with what it
specifically enables:

```
[OK] Go 1.27.0 (>= 1.21 (this project's go.mod floor), fine)
[OK] Platform: Ubuntu 24.04.4 LTS

[OK] gofmt (/usr/local/go/bin/gofmt) -- usage: gofmt [flags] [path ...]
     enables: real gofmt -e syntax validation for .go substitutions

[OK] bash (/usr/bin/bash) -- GNU bash, version 5.2.21(1)-release ...
     enables: real bash -n syntax validation for .sh/.bash substitutions

[OK] node (/usr/bin/node) -- v22.22.2
     enables: real node --check syntax validation for .js/.mjs/.cjs ...

[OK] pyyaml -- 6.0.3
     enables: real yaml.safe_load validation for .yaml/.yml substitutions
```

(`pyyaml` above is a real dependency of this Go binary too, not a leftover
-- YAML validation shells out to `python3 -c "import yaml..."` under the
hood, the same as the `.py` and JSON validators shell out to their own
respective checkers.)

None of these are required. Every optional tool has a documented fallback
— a heuristic check, or an honest "not independently verified" — when it's
absent, and `selftest` passes cleanly either way. `doctor` exists so that's
a visible, informed choice rather than a silent one: you'll know from the
first run whether an edit to a `.go` file is getting real `gofmt`
validation or the heuristic path. `--quiet` drops the per-tool detail and
just confirms the Go/platform baseline.

## Run `selftest` — the actual acceptance gate

```
repoman selftest
```

This is the gate that matters. It prints `doctor`'s environment summary
first (informational only — an absent optional tool never fails this gate,
that's a supported operating mode, not a defect), then exercises every
tool against a synthetic repository it builds in a temp directory: `ed`'s
own embedded selftest, `roles`, `syncver`, `register`, `guards`, wave
tracking, `relcore`, and a final section targeting specific real bugs found
in this project's own history rather than speculative edge cases.

**Do not trust an installation whose `selftest` fails.** A red gate means
something in this environment doesn't match what the tools assume — fall
back to plain, careful manual editing rather than trusting `ed`/
`str_replace_extended` in that state, and flag it before doing anything
else.

`strreplace`'s own selftest (`repoman strreplace selftest`) and `ed`'s
own (`repoman ed selftest`) can each also be run standalone — `selftest`
already calls both as part of its own run, but running one in isolation
is useful when you're specifically debugging that tool rather than the
whole suite.

## Opting in: `.repoman.json`

A repository opts in by having a `.repoman.json` file at its root. An empty
object is entirely valid:

```json
{}
```

Every configuration key has a documented default (see
`repoman-090-configuration.md` for the full reference), so an empty `{}`
means "everything at its default" — `docs/TRACKING.md` for the register,
`T-` prefixed ids, no staged-wave tracking configured, and so on. You only
add keys for the things you actually want to diverge from default: a
different id prefix, wave tracking paths, version-sync targets, a release
manifest.

The tools work perfectly well with no `.repoman.json` at all for anything
that doesn't need project-specific state — `ed` and `roles` in particular
need nothing beyond a target path. The config file matters once you start
using `register`, `guards`, `syncver`, `relcore`, or the wave-tracking
tools, since those read and write to paths this file controls.

## What's next

- `repoman-040-editing.md` — `ed` and `strreplace`, the precise, journaled
  editing tools, with worked examples.
- `repoman-050-roles.md` — the syntactic-role classifier that makes mass
  substitution safe, and its per-language notes and known limits.
- `repoman-060-register-and-guards.md` — tracking open work and keeping
  dormant tests honest.
- `repoman-070-releases.md` and `repoman-080-waves.md` — version sync,
  resumable releases, and staged-work tracking.
- `repoman-090-configuration.md` — every `.repoman.json` key, in one place.
