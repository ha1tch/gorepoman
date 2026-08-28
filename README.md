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

## Install

This tool is deliberately usable on projects that have nothing to do with
Go -- a ZX Spectrum assembly project or a plain static-site repo has no
reason to need a Go toolchain just to get repository-discipline tooling.
The primary distribution is therefore a prebuilt static binary, not
`go install` or building from source.

```
curl -L https://github.com/ha1tch/gorepoman/releases/latest/download/repoman-linux-amd64 -o repoman
chmod +x repoman
```

Swap `linux-amd64` for `linux-arm64`, `darwin-amd64`, or `darwin-arm64` to
match the target machine. The `/releases/latest/download/` path never
changes shape across releases, so this exact command keeps working on every
future release without being updated. To verify the download:

```
curl -LO https://github.com/ha1tch/gorepoman/releases/latest/download/checksums.txt
sha256sum --ignore-missing -c checksums.txt
```

### Building from source instead

Only needed for development on `gorepoman` itself, or a platform outside
the four the release binaries cover. Requires Go 1.21 or later; no
third-party dependencies.

```
go build -o repoman ./cmd/repoman
```

Either way, `repoman` is a single binary. Run it from anywhere; it operates
on the current working directory the same way the Python scripts do.

Run `repoman doctor` first — an environment diagnostic, not a pass/fail test:
Go version, platform, and which of the optional external tools (`gofmt`,
`bash`, `node`, PyYAML) this environment has, with what each enables.

Then run `repoman selftest` — the acceptance gate. 75 checks; exit 0 is the
gate. Do not trust a build whose selftest fails.

## Commands

All tools are subcommands of the one binary:

| Command | Python equivalent | Purpose |
|---|---|---|
| `repoman version` | — | Print the build version (Go-only; no Python equivalent) |
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
testing on live fixtures against the real compiled Python originals, and
`roles` in particular against a battery of dedicated stress fixtures covering
every classifier (Go, Markdown, Python, YAML, HTML) rather than just the
language-vocabulary sampler most other testing here uses.

Eight real, shared bugs have been found this way and fixed in **both**
languages together (fixing only the Go side would have broken parity rather
than restored it) — full writeups, fixtures, and ground-truth verification
for each are in `CHANGELOG.md`: `roles`'s Go backtick-string and block-
comment tracking, its Markdown fence and inline-code matching, Python's
triple-quote escape handling, YAML's comment detection, and `addwave`'s
item-range formatting. `_html_embedded_spans`'s one known `</script>`
ambiguity is a deliberate, documented exception — it matches real browser
parsing behaviour exactly, not a bug relative to any spec.

## License

GPLv3.0 - Copyright (c) 2026 haitch. https://ual.li
