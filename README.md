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

If `github.com/releases` isn't reachable from a given environment for
any reason (network policy, a transient issue, restricted tool
permissions), the same binaries and the full documentation are also
mirrored as a plain static site at
[ha1tch.github.io/gorepoman](https://ha1tch.github.io/gorepoman/) --
built by CI from the same tagged release, a genuinely independent
access path (different host, plain static files, no release-asset
redirect through `api.github.com`), not just the same thing linked
twice:

```
curl -L https://ha1tch.github.io/gorepoman/bin/repoman-linux-amd64 -o repoman
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

Then run `repoman selftest` — the acceptance gate. 86 checks; exit 0 is the
gate. Do not trust a build whose selftest fails.

## Commands

All tools are subcommands of the one binary:

| Command | Python equivalent | Purpose |
|---|---|---|
| `repoman version` | — | Print the build version (Go-only; no Python equivalent) |
| `repoman badcode` | — | Forbidden-string release gate (Go-only; local config, never committed -- see below) |
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

## `badcode`: a release gate that never lives in a repo

`repoman badcode check [path ...]` scans a tree for forbidden text
strings — names, internal codenames, credential-shaped tokens,
anything that must never reach a release. The pattern list is
deliberately **never stored in this or any repository**: it's local,
per-machine config, because a blocklist committed alongside the code
it's supposed to protect can be edited by whoever has commit access —
including an agent — in the same change that would have been caught by
it. Keeping it out-of-band is the entire point.

Config location: `$REPOMAN_BADCODE_DIR` if set, otherwise the OS user
config directory (`~/.config/repoman` on Linux, `~/Library/Application
Support/repoman` on macOS, `%AppData%\repoman` on Windows). Two files,
both optional, both read if present:

```
# badcode.txt -- one pattern per line, # comments allowed
TOP_SECRET_INTERNAL_CODENAME
sk_live_example_forbidden_key
```

```json
// badcode.json -- adds an optional reason, included in the refusal message
[
  {"pattern": "REDACTED_CUSTOMER_NAME", "reason": "real customer name, NDA'd"}
]
```

Matching is a literal, case-insensitive substring search — not regex,
deliberately: this check's entire value is that it can't fail in a way
that looks like success, and regex adds failure modes (a pattern that
silently doesn't compile, an anchor off by one) that a plain substring
search doesn't have. No config at all is not an error — this is
optional, per-operator tooling — but it's never silently indistinguishable
from a real pass either; a `WARN` line says plainly that nothing was
actually checked.

**`repoman relcore` runs this check automatically, unconditionally,
as the literal first thing it does** — before reading whether
`release.steps` even exists, before any step in it runs, on every
invocation including `--resume`. This is deliberately *not* a
`release.steps` entry: anything expressible in `.repoman.json` can be
edited or removed by anyone with repo access, which would defeat the
entire point of a gate that's supposed to be unconditional. There is no
flag, environment variable, or config key anywhere in this codebase
that disables it. The only lever that exists is the local badcode
config itself — patterns are configurable; whether the check runs at
all is not. A real match halts the release with no override; the only
way forward is removing the matched content and re-running.

## Status

This translation mirrors the Python original at v0.8.0 feature-for-feature on
every command — `ed`, `strreplace`, `doctor`, `gomod`, `register`, `guards`,
`relcore`, `roles`, `syncver`, `waveprogress`, and `addwave` — all verified
via the shared 75-check `selftest` suite plus direct side-by-side behavioural
testing on live fixtures against the real compiled Python originals, and
`roles` in particular against a battery of dedicated stress fixtures covering
every classifier (Go, Markdown, Python, YAML, HTML) rather than just the
language-vocabulary sampler most other testing here uses. `badcode` (Go-only,
no Python original to compare against) is covered by its own 7 checks plus
4 covering its mandatory `relcore` integration, in the same suite, 86 in
total.

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
