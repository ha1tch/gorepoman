# Structured output: `--format`

Every subcommand that reports on the state of something it manages —
`syncver show`/`check`, `gomod check`, `guards list`/`show`/`stale`,
`register list`/`show`/`check`, `waveprogress --show`/`--check`,
`board`, `workspace list` — used to invent its own output vocabulary. A
script consuming more than one of them had to learn a different shape
for each. `--format` is the fix: one flag, present on every reporting
subcommand, always meaning the same thing regardless of which object
it's attached to — the same idea as `kubectl -o`, `docker --format`,
`gh --json`.

```
$ repoman syncver show --format json
$ repoman register check --format json
$ repoman guards stale --format html
```

Three values, and this document is the one place their meaning is
defined — the individual tool chapters describe what each command's
*data* looks like, not what `--format` itself does, so that doesn't
have to be re-explained five times or risk drifting between
descriptions.

## `text` — unchanged, and provably so

The default. Every reporting subcommand's plain-text output is exactly
what it was before this flag existed — not "very similar," identical.
Where a command's json/html path is built from newly-added, separately
computed data (rather than the original printing code with the format
check inserted around it), that original code path is left completely
untouched and called as-is for `text`; `selftest`'s existing coverage
of every one of these commands passing unchanged is what actually
backs that claim, not just intent.

## `json` — a portable data contract

The same envelope from every command:

```json
{
  "tool": "syncver",
  "object": "syncver-status",
  "schema_version": 1,
  "data": { "...": "command-specific" },
  "generated_at": "2026-08-31T02:41:49Z"
}
```

- **`tool`** is the subcommand name.
- **`object`** fully disambiguates the shape of `data` on its own —
  `register-list` versus `register-item` versus `register-check` — so
  a consumer never has to inspect `data` just to know what kind of
  thing it's holding.
- **`schema_version`** is scoped to this `tool`+`object` pair alone,
  bumped only when *that* shape changes. One command growing a field
  never forces every other command's consumers to re-check their
  parsing.
- **`generated_at`** is always RFC3339 in UTC — a pinned format, not
  whatever the local machine's clock happens to stringify to.

`data` carries no presentation information at all — no colors, no
layout, no HTML. That's deliberate: JSON is the contract other
systems build against; how it gets drawn on a screen is the
consumer's problem, not gorepoman's, and coupling the two would mean
every renderer's private styling choices could reach back and change
what other systems have to parse.

Every command's `data` is validated against a real schema
(`github.com/ha1tch/queryfy`) before it's written — the actual bytes
about to ship, via a marshal/round-trip, not just the Go struct that
produced them. On a validation failure, nothing is written to stdout
and the process exits non-zero; a consumer piping this into `jq` never
has to guess whether what it received is real data or a
differently-shaped error.

## `html` — the deterministic rendering path

A self-contained document — real inline CSS, a `prefers-color-scheme`
dark-mode block, no dependency on any external stylesheet or font —
written to stdout, in the same visual family across every command:

```
$ repoman guards stale --format html > stale.html
```

This is the answer to a question that comes up specifically when an
AI agent session is the one rendering gorepoman's output inline: two
different sessions handed the same `--format json` and asked to
render a board or a table will make independent, possibly different
layout choices — column order, truncation, spacing — because nothing
in JSON pins down presentation, by design. **An agent with shell
access to `repoman` should call `--format html` directly and relay
its output as-is, rather than interpreting `--format json` into a
live-drawn rendering each time.** That's not a style preference — it's
the only one of the three formats where the same input is guaranteed
to produce the same output, because the layout decision is made once,
in compiled Go code (`html/template`, compiled in via `embed.FS`), not
re-derived by whichever model happens to be reading the JSON that day.

## Where the flag can appear

Anywhere in the command's own arguments, not just first position —
`repoman register show T-01 --format json` and
`repoman register show --format json T-01` are equivalent. Absent,
every command behaves exactly as if `--format text` had been passed.
An unrecognized value (`--format yaml`) is an error, not a silent
fallback to text.

`--format` only appears on subcommands that report state. Mutating
operations — `register add`/`close`, `guards record`, `syncver
set`/`bump-*`, `waveprogress` with no flags (regenerate in place),
`--hide`/`--unhide` — don't take it; there's no alternate-format
question to ask about an action that writes something rather than
describing it.

## One deprecated exception: `waveprogress --html PATH`

`waveprogress` had its own `--html PATH` (writes a file) before this
design existed. It still works, unchanged, and isn't going away — but
`--show --format html` (writes to stdout, same shape every other
command's html output uses) is the form to reach for now; see
`repoman-080-waves.md` for both, side by side.
