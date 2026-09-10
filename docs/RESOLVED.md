# Resolved items

Closed items, newest first.

## [0.14.4] T-14 — Filed-by field parsing/validation on register items (v0.14.4, 2026-09-08)

Theme: workspace · closed 0.14.4 · 2026-09-08
Parse and validate a Filed-by field on a register item, naming the
originating project's issue (issue/<project>:<prefix>-<n>). Currently
this would only ever be free text in an item's body if written at
all -- no structured field, no validation that the referenced issue
id is well-formed. Depends on T-13 (the mechanism that actually
produces items carrying this field).

Cross-ref: CHANGELOG 0.14.4.

## [0.14.4] T-06 — wave membership as a queryable field on a register item (v0.14.4, 2026-09-08)

Theme: dependency-graph · closed 0.14.4 · 2026-09-08
Add wave membership as a queryable field on a register item itself,
not just implicit in WAVE_TRACKING.md's document layout. Confirmed
gap: register.ItemSummary carries id/status/priority/theme/title,
no wave -- a cross-wave dependency board has no structured way today
to know which wave an item belongs to without re-parsing prose.

Natural point to populate this: addwave already links an item via
register_item in --items-json; the wave number is known there and
should be written back onto the item.

Cross-ref: CHANGELOG 0.14.4.

## [0.14.3] T-13 — workspace issue/closure commands (CLI verb naming undecided) (v0.14.3, 2026-09-08)

Theme: workspace · closed 0.14.3 · 2026-09-08
Progress: implemented and tested end to end (newissue, resolveissue,
pauseissue, and a new dropissue) against a real local git remote,
including two real bugs caught by testing and fixed before landing:
a shell-escaping corruption of a newline byte, and an ID-reuse bug
where a resolved or dropped issue own number became reusable once its
file was deleted from the open directory -- fixed by deriving the
next id from the resolved/dropped logs too, not the open directory
alone. CLI naming resolved as subcommands, not flags. Pending a
batched release.

NAMING NOT DECIDED -- flagging rather than inventing silently. The
issue and closure JSON message shapes (gorepoman.workspace.issue/1,
gorepoman.workspace.closure/1) are fully designed, with queryfy
validation and the issue/<project>:<prefix>-<n> reference syntax
already settled. What was never settled: the actual CLI verb(s) that
produce and consume these messages -- unlike join/leave/list, no
command name was ever agreed for "file an issue into a workspace" or
"write a closure notice back." Candidates worth deciding between:
folding into repoman workspace (file-issue/close-issue subcommands),
or a separate repoman issue command. Depends on T-10 through T-12
existing first regardless of which name is chosen.

Cross-ref: CHANGELOG 0.14.3.

## [0.14.3] T-07 — register kanban view (single-project, status-grouped) (v0.14.3, 2026-09-08)

Theme: kanban · closed 0.14.3 · 2026-09-08
Progress: implemented and tested (repoman register list --view kanban).
The recorded Blocks/after: T-05 turned out not to reflect a genuine
technical requirement -- this needed only fixed, five-column status
grouping, none of the graph-parsing T-05 is actually about. Kept the
field as historical record rather than silently removed. Pending a
batched release.

Add a status-grouped kanban view as an option on the existing register
command (already decided: this is a register option, not a new
command). Today register --format html renders a flat list; this adds
a real, deterministic, queryfy-validated, embed.FS-templated grouped
layout alongside it, matching the same pattern already built for
syncver/gomod/guards/register/waveprogress.

No new command surface -- most likely a --view kanban (or similar)
flag alongside --format on register list.

Cross-ref: CHANGELOG 0.14.3.

## [0.14.3] T-19 — register close does not propagate to a linked item's WAVE_TRACKING.md checkbox (v0.14.3, 2026-09-08)

Theme: dependency-graph · closed 0.14.3 · 2026-09-08
register close (and register add, when it links a wave via
register_item) does not propagate to the linked item's own checkbox
in WAVE_TRACKING.md. Confirmed recurring, not a one-off: found and
hand-fixed for T-09/T-10/T-11/T-12 in one commit, then found again for
T-07 in the very next one, because closing T-07 through the normal
register close path did not touch its wave row either.

The register itself said this plainly: a checked-box hand-fix is
supposed to be the exception, filed the moment a pattern is spotted,
not repeated a third time by hand. This is that filing.

Scope: register close needs to look up whether the closed id appears
as a register_item value anywhere in WAVE_TRACKING.md, and if so
update that row's own checkbox in the same operation -- not as a
second manual step, and not only when someone happens to remember.

Cross-ref: CHANGELOG 0.14.3.

## [0.14.2] T-12 — repoman workspace join/leave/list (v0.14.2, 2026-09-07)

Theme: workspace · closed 0.14.2 · 2026-09-07
Progress: implemented and tested end-to-end against a real local git remote (join/leave/list all verified, remote state confirmed via independent clone), pending a batched release with T-10/T-11

repoman workspace join / leave / list -- decided command shape (one
command, three subcommands, not new top-level verbs, matching every
other multi-subcommand command in gorepoman). join writes to two
repos with two different credentials: the workspace's own
participants file (using a workspace-scoped credential), and the
local project's .repoman.json workspaces key (using the project's own
credential). leave is the mirror. list is read-only and takes
--format like every other reporting subcommand.

Cross-ref: CHANGELOG 0.14.2.

## [0.14.2] T-11 — workspace participants file + target validation (v0.14.2, 2026-09-07)

Theme: workspace · closed 0.14.2 · 2026-09-07
Progress: implemented and tested (pkg/workspace schemas, queryfy-validated), pending a batched release with T-10/T-12

Workspace participants file: a small versioned JSON object
(gorepoman.workspace.participants/1) naming which projects belong to
a workspace, used to validate an issue's target field before it is
written -- so a filed issue cannot be silently sent to an invalid
destination. Read-only from the filing project's side; membership
itself is managed by whoever administers that workspace repo.

Cross-ref: CHANGELOG 0.14.2.

## [0.14.2] T-10 — config: workspaces key in .repoman.json (name, remote, credential_env) (v0.14.2, 2026-09-07)

Theme: workspace · closed 0.14.2 · 2026-09-07
Progress: implemented and tested (Workspace struct + Workspaces field in pkg/config), pending a batched release with T-11/T-12

Config-side support for workspace membership: a workspaces key in
.repoman.json (name, remote, credential_env per workspace a project
belongs to). Decided: privacy of the metadata itself is not a
concern (open source or GitHub-permission-controlled), but
credential_env only ever names an environment variable -- the real
secret is provisioned separately per machine, never written into
this committed file, same principle as badcode's own config.

Cross-ref: CHANGELOG 0.14.2.

## [0.14.2] T-09 — board: configurable column labels / grouping axis (v0.14.2, 2026-09-07)

Theme: kanban · closed 0.14.2 · 2026-09-07
Progress: values/natural-order/auto-derive/overflow all implemented and
tested for status, priority, and theme axes. dependency_order is
explicitly rejected at render time -- it needs T-05/T-06 first, not
guessed at. Pending a batched release.

NEEDS A DESIGN DECISION BEFORE IMPLEMENTATION, not ready to build as
filed. Column labels (and possibly the grouping axis itself: status
vs dependency-order vs priority vs theme) should be configurable per
board -- but a board has no persisted identity today. board currently
takes bare positional directory paths and holds no state of its own.

Open questions to resolve first: where does a board's own
configuration live (a new file format? a flag-supplied manifest?);
does a board need a name/identity at all if it is meant to stay a
pure, regenerated projection of other data; what happens on a
dependency-order board deeper than the configured label sequence
(Tier 6 fallback, repeat the last label, or merge overflow into the
final column). Do not implement against assumed answers to these.

UPDATE 2026-09-07: most of the above is now resolved. Boards live in
the workspace repo (opt-in layer, not a requirement -- the plain,
no-workspace repoman board stays the free default). Overflow answers
the Tier-6 question: auto-created when needed, with a persisted
--hide/--unhide (matching waveprogress's own precedent, not a bare
field edit) and a separate --empty that clears its tracked-items
record with no data migration -- nothing is moved, since nothing
there was ever a second copy of real data. Legend is per-card notes
in a colour palette kept deliberately distinct from colours that
already carry fixed meaning (done/partial/not-started). Footer and
subtitle are optional hand-authored markdown, no dependency on any
computed mechanism. Default board is per-participant, recorded in the
workspace's own participants.json (default_boards), not in the board
definition itself -- confirmed: different participants may genuinely
want different default views of the same workspace. Schema
(gorepoman.workspace.board/1) and pkg/workspace config/join/leave/list
are implemented and tested (see T-10/T-11/T-12).

Still genuinely open, discovered while trying to implement rendering:
a column has a label and a note, but nothing says which underlying
data value it actually collects. For axis=status, does a column's
position imply which status symbol it holds, or does each column need
an explicit value field naming what it matches? This has to be
answered before rendering can be built -- not invented silently.

Cross-ref: CHANGELOG 0.14.2.

