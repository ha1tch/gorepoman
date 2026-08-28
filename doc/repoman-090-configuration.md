# Configuration reference: `.repoman.json`

Every key here has a documented default; an empty `{}` is a fully valid
opt-in (see `repoman-030-getting-started.md`). You only add a key when you
want to diverge from its default. This document lists every key once, in
one place — the individual tool documents describe how each tool *behaves*,
not what every configuration key means, specifically so a key's meaning
doesn't have to be looked up in more than one document or risk drifting
between descriptions.

## All keys at a glance

| Key | Default | Read by |
|---|---|---|
| `id_prefix` | `"T"` | `register` |
| `id_separator` | `"-"` | `register` |
| `legacy_id_prefix` | `""` | `register` |
| `legacy_id_separator` | `"-"` | `register` |
| `tracking` | `"docs/TRACKING.md"` | `register` |
| `resolved` | `"docs/RESOLVED.md"` | `register` |
| `known_issues` | `"docs/KNOWN_ISSUES.md"` | `guards` |
| `guard_id_prefix` | `"G-"` | `guards` |
| `changelog` | `"CHANGELOG.md"` | `guards` (previous-release date), `relcore` |
| `version_file` | `"VERSION"` | `syncver`, `relcore` |
| `version_targets` | `[]` | `syncver` |
| `wave_tracking` | `"docs/WAVE_TRACKING.md"` | `waveprogress`, `addwave` |
| `wave_plan` | `"docs/WAVE_PLAN.md"` | `waveprogress`, `addwave` |
| `wave_short_names` | `{}` | `waveprogress`, `addwave` |
| `wave_themes` | `{}` | `waveprogress` |
| `wave_visibility` | `{}` | `waveprogress` |
| `wave_html_title` | `"wave progress"` | `waveprogress` |
| `release` | `{"steps": [], "archive": {}}` | `relcore` |

## Register ids

**`id_prefix`** (default `"T"`) and **`id_separator`** (default `"-"`) —
the prefix and separator `register.py next_id()` uses for every new id it
generates. The default reproduces this project's own original,
only-ever-tested shape (`T-1`, `T-2`, ...) exactly for any consumer that
doesn't set these.

**`legacy_id_prefix`** (default `""`, meaning disabled) and
**`legacy_id_separator`** (default `"-"`) — for a project that migrated id
shape *mid-project* and needs both forms recognized: ids already issued in
the old shape stay frozen in that shape permanently, while `next_id()`
issues only the new shape going forward. This isn't hypothetical scope —
the documented real case is a project whose ids `T-1` through `T-163` are
permanently frozen in `T-NNN` shape, with `T-164` onward forward-only in a
new `XOTNNN` shape. Leaving `legacy_id_prefix` empty (the default) means
single-format behavior, byte-identical to before these two keys existed —
they're additive and change nothing for a consumer that never sets them.

## Register, resolved, and known-issues paths

**`tracking`** (default `"docs/TRACKING.md"`) and **`resolved`** (default
`"docs/RESOLVED.md"`) — where `register` reads and writes the live register
and the closed-item record. **`known_issues`** (default
`"docs/KNOWN_ISSUES.md"`) — where `guards` reads and writes the
dormant-guard table. All three are plain paths, relative to the repository
root.

## Guards

**`guard_id_prefix`** (default `"G-"`) — the full prefix, including
separator, for dormant-guard ids (`"G-"` → `G-13`). A single string is
enough generality here — guards don't have the mid-project id-migration
need `legacy_id_prefix` exists for.

## Version sync

**`changelog`** (default `"CHANGELOG.md"`) — read by `guards stale` to
derive the previous release's date when `--since` isn't given explicitly,
and by `relcore` for its own bookkeeping.

**`version_file`** (default `"VERSION"`) — the single file `syncver`
treats as the source of truth for the current version.

**`version_targets`** (default `[]`) — every *other* file that has to
agree with `version_file`. Each entry:

```json
{"file": "app.py", "match": "VERSION = \"([0-9.]+)\""}
```

`match` is a regex with exactly one capture group — the group is what gets
replaced on `set`/`bump-*`, and what gets compared on `check`.

## Staged-work (wave) tracking

All optional — every key here defaults to empty or generic, specifically
so a consumer that never touches waves sees no behavior change at all.

**`wave_tracking`** (default `"docs/WAVE_TRACKING.md"`) and **`wave_plan`**
(default `"docs/WAVE_PLAN.md"`) — the two documents `addwave` writes to and
`waveprogress` reads from.

**`wave_short_names`** (default `{}`) — `wave_id -> short name`,
auto-maintained by `addwave`. Never hand-edited.

**`wave_themes`** (default `{}`) — `wave_id -> [themes]`, hand-curated when
used at all. Which open register debt genuinely belongs to a given wave's
subject matter is a judgment call, not something mechanically derivable
from the data alone — the empty default (no debt cross-referencing) is the
correct starting behavior until those calls have actually been made.

**`wave_visibility`** (default `{}`) — `wave_id -> bool`. Absent means
visible, matching the same additive-default rule as everything else here —
a consumer who's never touched this sees every wave, exactly as before the
key existed. This is persisted *data*, not a per-renderer setting: both the
ASCII and HTML renderers read this same dict, so visibility can't drift
between the two display forms the way it would if each tracked its own
separate notion of what's shown.

**`wave_html_title`** (default `"wave progress"`) — the heading text for
`waveprogress --html` output. Cosmetic; the generic default is deliberate.

## Release

**`release`** (default `{"steps": [], "archive": {}}`) — the manifest
`relcore` executes. Full schema:

```json
{
  "release": {
    "steps": [
      {"name": "build", "run": "make build", "resumable": true},
      {"name": "test",  "run": "make test",  "resumable": true, "timeout": 900},
      {"name": "gate",  "run": "python3 scripts/my_gate.py", "always": true},
      {"name": "sync",    "builtin": "syncver", "always": true},
      {"name": "archive", "builtin": "archive", "always": true}
    ],
    "archive": {
      "name": "{repo}-v{version}-checkpoint.zip",
      "sources": ["README.md", "CHANGELOG.md", "VERSION", "src"],
      "exclude": ["*.tmp", "*.log"],
      "size_warn_mb": 3
    }
  }
}
```

Each step in `steps` is either a shell command (`run`) or a `builtin`
(`syncver`, `archive` — currently the only two). `always: true` re-runs the
step every time, resumed or not; `resumable: true` means the step is
journaled and skipped on `--resume` if it already succeeded — a step marked
neither runs once, and isn't specially skipped on resume the way a
`resumable` one is. `timeout` (seconds, default 600) bounds how long a
`run` step is allowed to take.

`archive.sources` lists what the `archive` builtin packages;
`archive.exclude` adds glob patterns on top of the builtin's own
always-excluded self-generated output (`MANIFEST.sha256`,
`.release-state.json`, `release-*.log`) — see
`repoman-020-failure-modes.md` #7 for why that exclusion exists.
`archive.name` supports `{repo}` and `{version}` placeholders;
`size_warn_mb` (default `3`) flags an archive larger than that many
megabytes rather than silently producing an unexpectedly large one.
