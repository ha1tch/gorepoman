# repoman (Go)

A Go translation of [repoman](https://github.com/ha1tch/repoman) — repository-
discipline tooling: precise journaled text editing, syntactic-role auditing,
tracked-work registers, dormant-guard currency, version sync, interruptible
release orchestration, staged-wave tracking, and a go.mod/go.sum sanity gate —
compiled into a single static binary. A repository opts in with `.repoman.json`
at its root, same as the Python original.

This is a translation, not a fork with its own design: command names, flags,
file formats, `.repoman.json` keys, and output text are kept identical to the
Python original wherever the two languages allow it, so the two are
interchangeable in a working session. Where they must differ (installation,
building), that's called out below.

## Requirements

- Go 1.21 or later. No third-party dependencies.

## Install

```
go build -o repoman ./cmd/repoman
```

This produces a single `repoman` binary. Run it from anywhere; it operates on
the current working directory the same way the Python scripts do.

Run `repoman doctor` first — an environment diagnostic, not a pass/fail test:
Go version, platform, and which of the optional external tools (`gofmt`,
`bash`, `node`, PyYAML) this environment has, with what each enables.

Then run `repoman selftest` — the acceptance gate. 75 checks; exit 0 is the
gate. Do not trust a build whose selftest fails.

## Commands

All tools are subcommands of the one binary:

| Command | Python equivalent | Purpose |
|---|---|---|
| `repoman doctor` | `doctor.py` | Environment diagnostic |
| `repoman ed` | `ed.py` | Journaled handle-based text editing (`find`/`apply`/`sub`/`undo`/`mark`/`log`) |
| `repoman roles` | `roles.py` | Syntactic-role auditor |
| `repoman strreplace` | `str_replace_extended.py` | Format-aware, base64-payload, journaled substitution |
| `repoman register` | `register.py` | Live tracked-work register (`docs/TRACKING.md`) operations |
| `repoman guards` | `guards.py` | Dormant-guard currency tracking |
| `repoman syncver` | `syncver.py` | Version synchronization across files |
| `repoman relcore` | `relcore.py` | Manifest-driven, resumable release orchestration |
| `repoman waveprogress` | `wave_progress.py` | Regenerate staged-wave tracking documents (ASCII/HTML) |
| `repoman addwave` | `add_wave.py` | Add a new staged-work wave |
| `repoman gomod` | `gomod.py` | go.mod/go.sum sanity gate (replace-directive and go.sum completeness checks) |
| `repoman selftest` | `selftest.py` | Acceptance gate |

Every command accepts `-h`/`--help`, at both the top level and on each
subcommand, matching the Python original's argparse-generated help text.
`syncver --help` and `roles --help` are unsupported in both languages by
design/pre-existing quirk, not a Go-specific gap (`roles` is a positional-
args search tool by design; `syncver.py --help` is a bug that predates this
translation and was reproduced faithfully rather than silently fixed here).

## Status

This translation mirrors the Python original at v0.8.0 feature-for-feature on
every command — `ed`, `strreplace`, `doctor`, `gomod`, `register`, `guards`,
`relcore`, `roles`, `syncver`, `waveprogress`, and `addwave` — all verified
via the shared 75-check `selftest` suite plus direct side-by-side behavioural
testing on live fixtures against the real compiled Python originals:
identical multi-language role-classification fixtures for `roles`; identical
edit/register/guard workflows for `ed`/`register`/`guards`/`relcore`;
`show`/`set`/`check`/all three `bump-*` variants for `syncver`; single- and
multi-item wave creation with byte-diffed `WAVE_TRACKING.md`/`WAVE_PLAN.md`/
`.repoman.json` for `addwave`; `--show`/`--hide`/`--unhide`/`--include-hidden`
plus a byte-diffed generated HTML file for `waveprogress`.

Two real, shared bugs were found this way and fixed in **both** languages
(fixing only the Go side would have broken parity rather than restored it):

- `roles`'s Go-string classifier tracked raw-string (backtick) state with a
  naive whole-file backtick count, which misclassifies code after an odd
  number of backticks embedded in comments, rune literals, or double-quoted
  strings earlier in the file — in both directions (false positive and false
  negative). Replaced with a real stateful scan in both `roles.go` and
  `roles.py`. See `CHANGELOG.md` for the full writeup and the stress fixture
  that exercises both failure directions.
- `addwave`'s final confirmation line used a different item-range format for
  single-item waves than the Python original's own `add_wave.py` does — an
  inconsistency in Python's own two messages that Go now matches rather than
  "corrects."

## License

GPLv3.0 - Copyright (c) 2026 haitch. https://ual.li
