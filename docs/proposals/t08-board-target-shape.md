# T-08 target shape: a worked reference board

Updated: 2026-09-09

## What this is

T-08 ("board: cross-project dependency-order tiering") already names its
inspiration in the register: "the exact shape the real poesy wave board
demonstrated by hand this session." That reference was a board built for a
different project (poesy), by hand, before this repository had any of its
own worked example.

This proposal adds one: a hand-built dependency-tier board for
**gorepoman's own register**, refined over two rounds of real feedback in
session, checked into `docs/proposals/assets/t08-board-reference-2026-09-09.html`
as a static snapshot. It is not live — republishing the artifact it was
drawn from will not update this file, and this file will drift from the
register the moment either changes. That is deliberate: it is a target
shape to build toward, not a view to keep in sync by hand.

## Why a second reference, when T-08 already points at one

The poesy board demonstrated the concept in the abstract. This one is
concrete against the actual data structures T-08 will consume:
`docs/TRACKING.md`'s real `Blocks/after` column, this register's actual
open items (T-01 through T-25 at time of writing), actual wave
assignments, and a real fifth status (`☑`, complete-pending-release) that
came up only because two real items (T-20, T-21) reached that state during
the same session. Building it against gorepoman's own data surfaced two
design points the poesy board didn't have reason to show:

1. **Every unblocked item needs an explicit "unblocked" signal, not just
   items that gate something else.** The first draft of this board tagged
   a card "unlocks T-X" when something downstream depended on it, and left
   the tag off otherwise. That reads as ambiguous — a card with no tag is
   indistinguishable from one the tiering pass hasn't classified yet. The
   fix: every card in a computed tier carries *some* tag. A card that
   gates nothing downstream gets a neutral "unblocked, no dependents"
   marker instead of silence. **T-08's generated output should do the
   same** — never render a tier member with no explicit status marker.

2. **A "complete, pending release" item's tier placement and its
   readiness are two different facts, and both need to render.** T-20 and
   T-21 are unblocked (nothing in `Blocks/after` holds them back) so they
   correctly belong in the same tier as genuinely-unstarted work — but
   they are not something a person would "pick up" the way the rest of
   that tier is. The board handles this by keeping them in their computed
   tier (tier placement stays purely a function of the dependency graph,
   never overridden by status) while giving them a distinct visual
   treatment (a colored border, a `☑`-derived tag) and by splitting the
   header's summary stats into "ready to start" versus "code complete" so
   the two aren't silently conflated into one count. **T-08 should keep
   tier computation and status display as two independent concerns** —
   resist the temptation to let a `☑` item jump tiers or hide.

## What's in the snapshot

`assets/t08-board-reference-2026-09-09.html` is the actual HTML that was
live at the published artifact URL at the end of the session, self-
contained (fonts loaded from Google Fonts by link, everything else
inline). Open it directly in a browser to see the target rendering:
four tiers (Now / Next / Then / Last) computed from `Blocks/after`,
wave badges per card, "unlocks X, Y" tags on cards with dependents,
"unblocked, no dependents" tags on cards without, and a distinct
code-complete treatment for `☑` items. The footer text in the snapshot
itself explains the tiering rule in the same terms as above.

## What T-08 still needs to decide, independent of this reference

The open question already on file in T-08's own register body — whether a
soft override (an item whose only hard dependency is satisfied, but whose
own plan calls for something else to be exercised first) becomes a
distinct `Soft-after` field or stays a manual annotation — is unresolved by
this reference, deliberately. Neither the poesy board nor this one needed
a soft override for their respective real data, so neither settles the
question. Flag it when T-08 is actually built rather than deciding it here.
