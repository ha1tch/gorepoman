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

Swap `linux-amd64` for the asset matching the target machine:

| OS | Architecture | Asset |
|---|---|---|
| Linux | x86-64 | `repoman-linux-amd64` |
| Linux | ARM64 | `repoman-linux-arm64` |
| macOS | Intel | `repoman-darwin-amd64` |
| macOS | Apple Silicon | `repoman-darwin-arm64` |
| Windows | x86-64 | `repoman-windows-amd64.exe` |
| Windows | ARM64 | `repoman-windows-arm64.exe` |
| FreeBSD | x86-64 | `repoman-freebsd-amd64` |
| FreeBSD | ARM64 | `repoman-freebsd-arm64` |
| OpenBSD | x86-64 | `repoman-openbsd-amd64` |
| OpenBSD | ARM64 | `repoman-openbsd-arm64` |
| NetBSD | x86-64 | `repoman-netbsd-amd64` |
| NetBSD | ARM64 | `repoman-netbsd-arm64` |
| DragonFly BSD | x86-64 | `repoman-dragonfly-amd64` |

DragonFly BSD is x86-64 only -- Go itself has no `dragonfly/arm64` port.
The two OpenBSD binaries come out dynamically linked against the
system's own `/usr/libexec/ld.so`, unlike the other eleven -- this is
long-standing, documented Go-on-OpenBSD behaviour (its W^X/pledge
exec-protection model requires it), not a build issue, and that loader
is part of every real OpenBSD install, not an extra dependency to fetch.

The `/releases/latest/download/` path never changes shape across releases,
so any of these exact commands keeps working on every future release
without being updated. To verify the download:

```
curl -LO https://github.com/ha1tch/gorepoman/releases/latest/download/checksums.txt
sha256sum --ignore-missing -c checksums.txt
```

On Windows (PowerShell): `curl.exe` is built in on Windows 10/11, and
`Get-FileHash` replaces `sha256sum` -- check its output against the
matching line in `checksums.txt` by hand, since `-c` isn't available:

```
curl.exe -LO https://github.com/ha1tch/gorepoman/releases/latest/download/repoman-windows-amd64.exe
curl.exe -LO https://github.com/ha1tch/gorepoman/releases/latest/download/checksums.txt
Get-FileHash repoman-windows-amd64.exe -Algorithm SHA256
```

On the BSDs, the base install typically ships `sha256` rather than GNU
coreutils' `sha256sum` -- check its output against the matching line in
`checksums.txt` by hand:

```
sha256 repoman-freebsd-amd64
```

### Building from source instead

Only needed for development on `gorepoman` itself, or a platform outside
the thirteen the release binaries cover. Requires Go 1.21 or later; no
third-party dependencies. `make cross` builds all thirteen release
targets locally into `dist/` (see the `Makefile`), or just build for the
current platform:

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
