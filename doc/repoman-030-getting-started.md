# Getting started

## Install

Clone the repository, or copy its files into yours — conventionally under
`scripts/` or as a sibling `repoman/` directory. The tools are plain files
at this repository's own root, not a package to import, so "install" mostly
means "have the `.py` files somewhere on disk you can point at."

Requirements: Python 3.10 or later, no third-party dependencies for the
core tools. A handful of optional external tools unlock *better* validation
for specific file types but nothing requires them — see `doctor` below.

## Run `doctor` first

```
python3 doctor.py
```

This is an environment diagnostic, not a pass/fail test. It reports your
Python version, your platform, and which of four optional external tools
this environment has — `gofmt`, `bash`, `node`, PyYAML — each with what it
specifically enables:

```
[OK] Python 3.12.3 (>= 3.10, fine)
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

None of these are required. Every optional tool has a documented fallback
— a heuristic check, or an honest "not independently verified" — when it's
absent, and `selftest` passes cleanly either way. `doctor` exists so that's
a visible, informed choice rather than a silent one: you'll know from the
first run whether an edit to a `.go` file is getting real `gofmt`
validation or the heuristic path. `--quiet` drops the per-tool detail and
just confirms the Python/platform baseline.

## Run `selftest` — the actual acceptance gate

```
python3 selftest.py
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

`str_replace_extended.py`'s own selftest (`python3
str_replace_extended.py selftest`) and `ed.py`'s own (`python3 ed.py
selftest`) can each also be run standalone — `selftest.py` already calls
both as part of its own run, but running one in isolation is useful when
you're specifically debugging that tool rather than the whole suite.

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
