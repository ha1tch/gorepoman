# Tracking

| ID | Summary | Theme | Priority | Status | Blocks |
|----|---------|-------|----------|--------|--------|

| T-15 | relcore orchestrates GoReleaser as a resumable step | release-hardening | P3 | ☐ | — |
| T-16 | canonical CI workflow: badcode + selftest as a required PR check | release-hardening | P3 | ☐ | — |
| T-17 | roles backed by a real structural matcher (research spike) | release-hardening | P3 | ☐ | — |
| T-18 | guards: distinguish never-rerun from code-moved-since (underdesigned) | release-hardening | P3 | ☐ | — |
| T-27 | register claim/release: write and clear Claimed-by, journaled | locking | P2 | ☐ | T-26 |
| T-28 | stale-claim policy: TTL or override for an abandoned Claimed-by | locking | P2 | ☐ | T-26,T-27 |
| T-29 | surface claims on register/board views (kanban, board, board --definition) | locking | P3 | ☐ | T-26 |

## release-hardening

### T-15. relcore orchestrates GoReleaser as a resumable step

Theme: release-hardening · Priority: P3 · Status: ☐

Have relcore orchestrate GoReleaser: a release.steps entry that runs
goreleaser release as a resumable step. Closes a real, confirmed
irony -- gorepoman has no .repoman.json / release.steps of its own at
all (fixed by this same wave plan) and ships itself via GoReleaser +
git tags directly, bypassing relcore's own badcode-gate-first,
journaled-resume discipline for the one release that matters most to
this project.

### T-16. canonical CI workflow: badcode + selftest as a required PR check

Theme: release-hardening · Priority: P3 · Status: ☐

Ship a canonical CI workflow (a maintained GitHub Actions file, the
way Gitleaks ships one) running badcode and selftest as a required,
unbypassable PR check. Confirmed gap: badcode currently only runs
locally, as part of relcore -- advisory in the same sense the mature
secret-scanning research calls pre-commit hooks advisory, since
nothing stops a PR from merging without anyone having run it.

### T-17. roles backed by a real structural matcher (research spike)

Theme: release-hardening · Priority: P3 · Status: ☐

RESEARCH SPIKE, not a small feature -- scope accordingly. Back roles'
classification with a real AST-aware structural matcher (ast-grep- or
comby-class), rather than the current hand-rolled regex classifier,
while keeping ed's own session/journal/undo layer on top -- that
layer is gorepoman's actual differentiator and has no equivalent in
ast-grep or comby, neither of which offers a stateful, reversible
editing session. This is explicitly about replacing the matching
engine underneath, not the workflow built on it.

### T-18. guards: distinguish never-rerun from code-moved-since (underdesigned)

Theme: release-hardening · Priority: P3 · Status: ☐

UNDERDESIGNED, no concrete mechanism proposed yet -- needs a design
pass before this can be scoped into a real item. guards currently has
no way to distinguish "this guard has not been re-run since the last
release" from "the code this guard covers has changed since it was
last exercised" -- both currently look identical (a last-exercised
date older than the release date). The second case is arguably more
urgent than the first, since a pass recorded against code that no
longer exists is not real evidence of anything.

## locking

### T-27. register claim/release: write and clear Claimed-by, journaled

Theme: locking · Priority: P2 · Status: ☐ · Wave: 8 · Blocks/after: T-26

New subcommands `register claim <id> --by <who>` and `register
release <id>` writing/clearing the `Claimed-by` field from T-26,
journaled the same way `register add`/`close` already are (no raw
file writes). `claim` on an item already claimed by someone else
refuses by default (optimistic, not a hard lock -- see T-28 for why);
`claim` on an item already claimed by the SAME `--by` value is a
no-op success, not a refusal (idempotent re-claim, e.g. a session
resuming after a restart). `release` on an unclaimed item is a no-op
success, not a refusal.

Depends on T-26 (the field itself must exist and parse before a
command can write it).

Explicitly NOT in scope here: nothing about this enforces exclusivity
on the underlying file edits an agent makes while working the item --
`ed apply`'s own SpanHash staleness check is the only thing that
actually prevents two concurrent writers from corrupting the same
file, and that mechanism is unrelated to and unaffected by this. A
claim is a courtesy signal other agents can check before starting
work, not a lock on the filesystem.

### T-28. stale-claim policy: TTL or override for an abandoned Claimed-by

Theme: locking · Priority: P2 · Status: ☐ · Wave: 8 · Blocks/after: T-26,T-27

Design and document a stale-claim policy: what happens when a claim's
owning agent/session never releases it (crashed, timed out, was
never told to). A claim with no expiry is worse than no claim at all
-- it would eventually block every other agent from an item nobody
is actually working, and nothing here should be able to deadlock the
register.

Candidate shapes to decide between, not yet chosen: (a) a TTL on the
Claimed-by field itself (a timestamp alongside the identifier,
`register claim` refuses only within the TTL window, expires
automatically after); (b) no TTL, `--force` always available to any
caller (claims are advisory only, never actually block, purely
informational); (c) TTL plus explicit `register claim --force`
override that logs the override same as workspace `dropissue`'s
mandatory `--reason` does. Whatever is chosen must never require a
human in the loop to unstick the register -- that would make claims
worse than the no-claims status quo.

Depends on T-26/T-27 existing first (nothing to make stale until the
field and commands are real).

### T-29. surface claims on register/board views (kanban, board, board --definition)

Theme: locking · Priority: P3 · Status: ☐ · Wave: 8 · Blocks/after: T-26

Surface claims on every existing report/board surface that already
renders per-item state, so a claim is actually visible where an agent
would look before picking up work -- not a fifth thing to separately
query. `register list --view kanban`, plain `board`, and
`board --definition` (any axis, including the new dependency_order
axis from T-05/T-08) all already render one card/row per item;
add the claim (when present) as a small marker consistent with how
Wave badges and status symbols already render, no new column.

Depends on T-26 (field must exist) and, for the board surfaces
specifically, does not depend on T-05/T-08 being done first --
dependency_order is one axis among several this already needs to
work on (status/priority/theme), so this item does not block or get
blocked by the boards work.

