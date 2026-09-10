# repoman documentation

repoman is repository-discipline tooling built for an editor with no
persistent memory of its own past sessions — precise, journaled text
editing; syntactic-role classification that makes mass substitution safe;
a register and dormant-guard tracker that keep "still open" and "still
passes" honest over time; and resumable, manifest-driven releases. Every
mechanism in it traces back to a real incident, converted from a one-time
lesson into something the tool now refuses to let happen the same way
twice.

## Chapters

| # | Document | Covers |
|---|---|---|
| 010 | [Why repoman exists](repoman-010-why.md) | The problem it solves, prevention vs. recovery, and the core design principles — each tied to a real incident |
| 020 | [Failure modes](repoman-020-failure-modes.md) | A scannable catalog: what each failure looks like, the mechanism that prevents it, the incident it traces to |
| 030 | [Getting started](repoman-030-getting-started.md) | Install, `doctor`, `selftest` as the acceptance gate, and opting in via `.repoman.json` |
| 035 | [Migrating from vendored Python](repoman-035-migration.md) | A verified shim template and generator script for replacing vendored Python `repoman` scripts with thin `gorepoman` shims |
| 040 | [Precise editing](repoman-040-editing.md) | `ed` and `strreplace` — handle-verified, journaled editing, with real worked examples |
| 050 | [The role classifier](repoman-050-roles.md) | `roles` — the full vocabulary, per-language classifier notes, and their documented limits |
| 055 | [Structured output: `--format`](repoman-055-format.md) | One flag, one meaning, on every reporting subcommand — the json data contract, the deterministic html path, and why an agent with shell access should relay html rather than re-render json |
| 060 | [Register and guards](repoman-060-register-and-guards.md) | Tracking open work through to closure, and keeping dormant tests honest about when they last ran |
| 065 | [The forbidden-string gate](repoman-065-badcode.md) | `badcode` — the mandatory release-blocking scan, why its config lives outside every repo, and how it integrates with `relcore` |
| 070 | [Releases](repoman-070-releases.md) | `syncver`, `relcore`, `gomod` — version sync and resumable, journaled, gated releases |
| 080 | [Staged-work tracking](repoman-080-waves.md) | `addwave` and `waveprogress` — grouping register items into a sequenced, visualized programme |
| 085 | [Cross-project status: `board`](repoman-085-board.md) | Aggregating several local checkouts by invoking each command's own `--format json` and merging — scope, exit-code behaviour, `--definition`/axis-configured boards, and what's deliberately not built yet |
| 086 | [Cross-project workspaces](repoman-086-workspace.md) | `workspace` — joining a shared, git-hosted directory across projects, and filing/resolving/pausing/dropping issues between them |
| 088 | [Provenance: detecting (and sanctioning) edits made outside repoman](repoman-088-provenance.md) | `provenance check`/`sanction` — catching a file touched by something other than repoman itself, and every write path's refusal to build on top of it |
| 090 | [Configuration reference](repoman-090-configuration.md) | Every `.repoman.json` key in one place, cross-referenced to the tool that reads it |
| — | [Cards are for humans vouching. Guards are for machines proving.](cards-and-guards.md) | Why a kanban board has no equivalent ceremony for a conversation with an agent — and what enforcing the rule at the point of action looks like instead |

## Proposals

| Document | Covers |
|---|---|
| [`ed insert`/`append`/`prepend`, and a ticketed `ed niplines`](proposals/ed-insert-and-ticketed-niplines.md) | Design intent for widening `ed`'s vocabulary beyond span-replacement: boundary/positional insertion reusing the existing handle-verification machinery, and a two-phase request/confirm ticket flow for line-range deletion, with TTL guidance |
| [T-08 target shape: a worked reference board](proposals/t08-board-target-shape.md) | A hand-built dependency-tier board against gorepoman's own register, checked in as a static reference for what T-08's generated output should look like |
