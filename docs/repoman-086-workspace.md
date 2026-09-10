# Workspace: cross-project membership and issue exchange

A `board` renders one project's own register. A workspace is the layer
above that: a shared, git-hosted directory that a set of projects have
all joined, used for two distinct things — declaring membership (so a
board `--definition` can live in a shared place instead of being copied
into every project) and exchanging issues (one project filing a request
against another, without either project's own register format having to
know anything about the other).

Nothing here merges automatically into a board's rendering. Joining a
workspace and filing an issue into it are both real, running git
operations against a real remote; a board that wants to reflect any of
it still needs a `--definition` pointing at a file in that same
checked-out workspace, or a project reading the workspace's issues on
its own initiative. See [repoman-085-board.md](repoman-085-board.md) for
how a board consumes a definition once one exists.

## Two repos, two credentials

`repoman workspace join <name> <remote>` writes to two places at once,
each under its own credential:

- **This project's own `.repoman.json`** — `workspaces: [{name, remote,
  credential_env, project_name}]` — written with whatever credential
  this session already uses to write to this repo. No secret is stored;
  `credential_env` only ever *names* an environment variable that will
  hold the secret on whatever machine writes here later, the same
  principle `badcode`'s own config follows.
- **The workspace's own `participants.json`**, in the workspace repo
  itself — an ordinary `git clone` / edit / `commit` / `push`, nothing
  gorepoman-specific about the push itself. Read access to a workspace
  is free (anyone can clone it); write access is whatever the remote's
  own permissions already grant the credential in use.

```
$ repoman workspace join acme-platform git@github.com:acme/workspace.git --name acme-web
joined workspace "acme-platform" as "acme-web"
```

`--name` overrides the project name recorded on the workspace side
(default: the current directory's own basename, the same convention
`board` already uses elsewhere). `--credential-env` records the
variable name for future pushes; omitted, nothing is recorded and
whatever `git` already has configured is used as-is.

`repoman workspace leave <name>` is the exact mirror: removes the entry
from local `.repoman.json`, and from the workspace's
`participants.json` (including any `default_boards` entry naming this
project).

`repoman workspace list` is read-only — which workspaces this project
has joined, from its own `.repoman.json` alone, no clone involved.
Takes `--format text|json|html` like every other reporting subcommand.

```
$ repoman workspace list
acme-platform        git@github.com:acme/workspace.git
```

## Filing an issue against another project

`repoman workspace newissue <workspace> <target> <theme> <title>
--body-file PATH [--parent ID] [--name FILER]` clones the workspace,
writes a new issue file under `issues/<filer>/<prefix>-<n>.json`,
commits, and pushes:

```
$ repoman workspace newissue acme-platform acme-api auth \
    "Token refresh races under concurrent requests" \
    --body-file /tmp/issue-body.md
filed acme-web:T-14 against acme-api
```

`<target>` is the project the issue is filed *against* — purely a
label copied into the issue; nothing about `newissue` reaches into the
target project's own repo or register. `<theme>` and `<title>` mirror
the register's own vocabulary intentionally, but the issue lives
entirely in the workspace, not in either project's `TRACKING.md`.

The filer defaults to the project name recorded at `join` time for
this workspace (not re-derived from the current directory), so a
project cannot register under one name and file issues under a
different one by accident. `--name` overrides this explicitly when
needed. The issue's own `--parent` is optional — a free-text id for
linking a filed issue back to something in the filer's own register,
carried as a label only, not validated against anything.

An issue's id is `<filer>:<prefix>-<n>`, e.g. `acme-web:T-14` — the
filer segment is what actually identifies provenance; the numeric
prefix alone (`T-14`) carries none, since every project filing into
the same workspace may independently use the same prefix.

**IDs are never reused.** The next number for a given filer is derived
from every number that filer has ever used across three places at
once — the currently-open `issues/<filer>/` directory, plus the
append-only `resolved.jsonl` and `dropped.jsonl` logs — not the open
directory alone. An issue's file is deleted from the open directory
the moment it resolves or drops, so scanning only that directory would
make a closed number look available again; this was a real bug, caught
by testing the full resolve-then-file-another sequence, not by
inspection, and is why the current version parses each log line as
real JSON rather than pattern-matching text — an earlier fix matched
the wrong field name, silently fixed nothing, and was only caught by
re-running the same scenario on an independent fixture.

## Resolving, pausing, and dropping

An issue has exactly one non-terminal event and two terminal ones.

**`repoman workspace resolveissue <workspace> <ref> --resolved-as ID
--version V [--note TEXT]`** — the issue is done. Appends a `Closure`
record to `issues/resolved.jsonl` (schema
`gorepoman.workspace.closure/1`: `ref`, `resolved_as`, `version`,
`date`, optional `note`) and deletes the open issue file. `--resolved-
as` is the filer's own register id that actually closed it (so the
audit trail connects the cross-project ask to the concrete register
item), `--version` the version it shipped in. Same principle as the
register's own closure procedure: move to an append-only log, never
edit in place.

```
$ repoman workspace resolveissue acme-platform acme-web:T-14 --resolved-as T-88 --version 1.4.0
resolved acme-web:T-14 as T-88
```

**`repoman workspace pauseissue <workspace> <ref> --reason TEXT`** —
the one non-terminal event. The issue stays open, filed, real, just
not being actively pursued right now; unlike resolve/drop this does
not move or delete the issue file, only appends a `Pause` record
(schema `gorepoman.workspace.pause/1`) to `issues/paused.jsonl` as a
visible marker alongside it. `--reason` is required.

**`repoman workspace dropissue <workspace> <ref> --reason TEXT`** — the
other terminal event, for an issue that will not be resolved. Same
shape as resolve: appends a `Drop` record (schema
`gorepoman.workspace.drop/1`) to `issues/dropped.jsonl` and deletes the
open issue file — moved, not silently deleted. `--reason` is
mandatory here for the same reason `provenance sanction`'s `--reason`
is mandatory: dropping something must leave as clear a trace as
closing it, not a quieter one.

```
$ repoman workspace dropissue acme-platform acme-web:T-09 --reason "superseded by T-14, same root cause"
dropped acme-web:T-09: superseded by T-14, same root cause (moved to issues/dropped.jsonl, not deleted)
```

## What's still v1-scoped

No automatic board integration — a board only reflects a workspace's
issues if a `--definition` in that workspace is explicitly rendered, or
a project reads `issues/<its-own-name>/` on its own initiative; there
is no merge step that pulls open workspace issues into a project's own
register or `TRACKING.md` automatically. No notification mechanism —
filing, resolving, pausing, or dropping an issue is a git push and
nothing else; a project finds out by looking. No cross-workspace
search — each workspace is its own independent clone target, with no
index across workspaces a project has joined.
