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
| 040 | [Precise editing](repoman-040-editing.md) | `ed` and `strreplace` — handle-verified, journaled editing, with real worked examples |
| 050 | [The role classifier](repoman-050-roles.md) | `roles` — the full vocabulary, per-language classifier notes, and their documented limits |
| 060 | [Register and guards](repoman-060-register-and-guards.md) | Tracking open work through to closure, and keeping dormant tests honest about when they last ran |
| 065 | [The forbidden-string gate](repoman-065-badcode.md) | `badcode` — the mandatory release-blocking scan, why its config lives outside every repo, and how it integrates with `relcore` |
| 070 | [Releases](repoman-070-releases.md) | `syncver`, `relcore`, `gomod` — version sync and resumable, journaled, gated releases |
| 080 | [Staged-work tracking](repoman-080-waves.md) | `addwave` and `waveprogress` — grouping register items into a sequenced, visualized programme |
| 090 | [Configuration reference](repoman-090-configuration.md) | Every `.repoman.json` key in one place, cross-referenced to the tool that reads it |
