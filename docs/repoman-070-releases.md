# Releases: `syncver`, `relcore`, `gomod`

Three tools covering one arc: keep the version string consistent
everywhere it's written, run a release as a sequence of journaled,
resumable steps rather than one big script that has to succeed end to end
or leave you guessing where it stopped, and — for Go projects specifically
— catch a class of `go.mod` mistake that only breaks for whoever isn't the
machine that wrote it.

## `syncver`: one version, everywhere it needs to appear

`version_targets` in `.repoman.json` names every file (and the regex to
find the version string inside it) that has to agree with `VERSION`:

```
$ repoman syncver show
version: 0.1.0  sync: ok

$ repoman syncver bump-minor
version set to 0.2.0
```

`bump-minor` writes `VERSION` and every configured target in one motion.
`bump-patch` and `bump-major` work the same way; `set <version>` sets an
exact version directly; `check` verifies everything still agrees and exits
non-zero if it doesn't — the check to wire into CI or a pre-release gate.

`show` and `check` both take `--format` (`repoman-055-format.md`) for
scripted consumption — a per-target breakdown, not just the flattened
message the default text form shows:

```
$ repoman syncver show --format json
{
  "tool": "syncver",
  "object": "syncver-status",
  "schema_version": 1,
  "data": {
    "version": "1.4.2",
    "in_sync": false,
    "targets": [
      {"file": "docs/README.md", "value": "1.4.0", "match": false}
    ]
  },
  "generated_at": "2026-08-30T14:00:00Z"
}
```

`check --format json`'s exit code carries the same meaning as the
default text form's — non-zero when out of sync — so it drops into a CI
step exactly like `check` always has, just with a real payload alongside
the exit code instead of only a status line.

## `relcore`: a release is a sequence of steps, not one script

`.repoman.json`'s `release.steps` list names each step as either a
`builtin` (`syncver`, `archive`) or a `run` shell command, and marks it
`always` (re-runs every time, resumed or not — `syncver` should be one of
these) or `resumable` (journaled; skipped on `--resume` if it already
succeeded).

A run where a step genuinely fails halts cleanly, rather than continuing
past a broken step:

```
$ repoman relcore 0.2.0
relcore 0.2.0 at rel_doc2 (log: release-0.2.0.log)
-- sync
   ok sync (0s)
-- build
   FAIL build: exit 1; see the release log
   run halted; fix and re-run with --resume
```

Fix whatever actually broke, then resume — steps already recorded as
succeeded aren't re-run; the ones that weren't, are:

```
$ repoman relcore 0.2.0 --resume
relcore 0.2.0 at rel_doc2 (log: release-0.2.0.log, resuming)
-- sync
   ok sync (0s)
-- build
   ok build (0s)
-- archive
   ok archive (0s)

release v0.2.0 prepared (0s)
```

`sync` re-ran both times — it's marked `always`. `build` ran once, failed,
then ran again and succeeded on resume, because a failed step was never
recorded as done. Had `build` succeeded the first time, `--resume` would
have skipped straight past it to whatever came next.

The `archive` builtin packages `release.archive.sources` into a
`<project>-v<version>-checkpoint.zip`, with an auto-generated
`MANIFEST.sha256` and any `release.archive.exclude` patterns genuinely left
out — not just expected to be absent. Its own exclusion list also covers
its own generated output (`MANIFEST.sha256`, `.release-state.json`,
`release-*.log`) by name specifically because a stale copy of one of those
sitting in the source tree once got swept in as an ordinary file and
produced a duplicate entry — see `repoman-020-failure-modes.md` #7.

**Never pipe `relcore`'s own output through `head`/`tail`/`grep` before
checking its exit code.** The log file exists for reading afterward; the
exit code is the only thing to trust in the moment — see
`repoman-020-failure-modes.md` #3 for what happens when that rule gets
skipped under debugging pressure.

### `relcore`'s mandatory pre-flight: `badcode` and `provenance`, before any step runs

Before `release.steps` is even read — not a step itself, not resumable,
not skippable by `--resume` — `relcore` runs two gates unconditionally:
`badcode check` (`repoman-065-badcode.md`) against the whole tree, and a
`provenance check` (T-03, `repoman-088-provenance.md`) confirming every
journal-tracked file still matches what repoman itself last wrote. Both
show up in the output even on a clean run, so a real pass is visibly
distinct from "nothing to check yet":

```
$ repoman relcore 0.1.0
-- badcode (mandatory pre-flight, not skippable)
   ok badcode (3 pattern(s) checked)
-- provenance (mandatory pre-flight, not skippable)
   ok provenance (no out-of-band edits detected)
relcore 0.1.0 at myproject (log: release-0.1.0.log)
-- sync
   ok sync (0s)

release v0.1.0 prepared (0s)
```

An out-of-band edit — a journal-tracked file changed by something other
than `ed`/`strreplace` since repoman last touched it — blocks the entire
release, the same way a `badcode` match does, with no `release.steps`
entry ever reached:

```
$ echo "tampered" > tracked.txt
$ repoman relcore 0.2.0
-- badcode (mandatory pre-flight, not skippable)
   ok badcode (3 pattern(s) checked)
-- provenance (mandatory pre-flight, not skippable)
   ERROR provenance-mismatch: tracked.txt changed outside repoman since 2026-09-10T02:15:26Z -- recorded 428c84d4521e..., now 92e78d0b0329...
   FAIL provenance: 1 mismatch(es) -- this gate has no override; run 'repoman provenance sanction FILE --reason "..."' for each one first, then re-run
```

There is no flag to bypass either gate. `repoman provenance sanction
FILE --reason "..."` (accepting the current content as the new
known-good state, with a mandatory audit trail) is the only way through
a real mismatch — exactly the same shape as fixing a `badcode` match by
removing the matched content. Both pre-flights re-run identically on
`--resume`, because neither is part of the resumable-steps journal at
all; there is nothing for `--resume` to skip.

## `gomod`: a go.mod/go.sum sanity gate

Built specifically for one incident shape: a `replace` directive pointing
at an absolute local filesystem path — something that only exists on the
machine that wrote it — reaching a committed `go.mod`. `gomod check` fails
on exactly that:

```
$ repoman gomod check .
GOMOD CHECK OK

$ echo 'replace example.com/internal-lib => /home/dev/internal-lib' >> go.mod
$ repoman gomod check .
ERROR replace-absolute-path: `replace example.com/internal-lib => /home/dev/internal-lib` is an absolute local filesystem path -- it will not exist on any machine but the one that wrote it. Remove before release.
GOMOD CHECK FAIL: 1 error(s)
```

A *relative*-path replace (`./foo`, `../foo`) is a legitimate monorepo
pattern, so it only warns by default — `--strict-relative-replace`
promotes that to a failure too, for projects that want the stricter rule.
A separate, best-effort check catches an incomplete `go.sum` via `go
list`'s own dependency resolution (never `go build`, so a CGO package
missing system libraries can't be mistaken for a `go.sum` problem) — and
degrades to a warning rather than a false failure when the environment
itself can't finish the check (no network, no `go` on `PATH`), rather than
either silently skipping or falsely failing.

Wire it into a project's own `relcore` steps as an ordinary `run` step:

```json
{"name": "go-sanity", "run": "repoman gomod check", "always": true}
```

`check --format json` (`repoman-055-format.md`) turns the same finding
into a structured payload — `errors`/`warnings` as real arrays a CI
system can act on individually, not lines to re-parse:

```
$ repoman gomod check --format json
{
  "tool": "gomod",
  "object": "gomod-check",
  "schema_version": 1,
  "data": {
    "toolchain_available": true,
    "errors": [
      "replace-absolute-path: `replace example.com/internal-lib => /home/dev/internal-lib` is an absolute local filesystem path -- it will not exist on any machine but the one that wrote it. Remove before release."
    ],
    "warnings": [],
    "ok": false
  },
  "generated_at": "2026-08-30T14:00:00Z"
}
```

The toolchain-unavailable soft-pass is visible in the payload too
(`"toolchain_available": false`, `"ok": true`) rather than only as an
absence of output — a consumer can tell "nothing to check here" apart
from "checked, and it's clean" without re-deriving it from silence.

## Putting it together

A full release gate — see `repoman-060-register-and-guards.md` for the
register and dormant-guard half of this — checks all of it at once: version
strings in sync, the register clean of closed items, every dormant guard
current or its skip recorded, and, for Go projects, `gomod check` clean.
`relcore` itself never needs remembering to run `badcode`/`provenance` —
both are unconditional pre-flights baked into `relcore` regardless of
what any release-hygiene script does or does not check. Where a
release-hygiene script exists for a project, the rest of these checks
belong in it rather than in anyone's memory of the steps.
