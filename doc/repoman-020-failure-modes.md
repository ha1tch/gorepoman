# Failure modes repoman exists to prevent

A reference, not a narrative — for jumping into mid-task when something
feels like it might be one of these. Each entry: what it looks like, why
it's specifically dangerous for an editor without persistent memory of its
own past sessions, the mechanism that closes it off, and the real incident
that earned it a place here. See `repoman-010-why.md` for the design
principles these all trace back to.

## 1. Uniform substitution across mixed syntactic roles

**Looks like:** a search-and-replace, or a regex, applied identically to
every occurrence of a string — without checking whether all of those
occurrences are actually the same *kind* of thing. The same text can be a
real identifier in one place, a string literal passed to something else
entirely in another, a comment, a table cell, or plain prose — and a single
substitution rule that's correct for one is often silently wrong for the
rest.

**Why it's dangerous here specifically:** an agent doesn't have the
peripheral, half-conscious sense a human maintainer builds up for "this
file has SQL strings mixed with Go identifiers, be careful" — that
awareness has to be made explicit and mechanical, or it isn't there at all.

**Prevented by:** `roles` classifies every occurrence by syntactic role
before any substitution is allowed to proceed; `str_replace_extended`'s own
delimiter-integrity check refuses outright if the occurrences it's about to
touch don't all share one role, printing the real census so the split can
be done properly instead of forced through with `--force-roles` on a
guess.

**Real incident:** this is the general failure mode `roles` was built to
close off entirely, not a single incident — it's the class of bug that
"classify before you substitute" as a working-agreement rule exists to
prevent by hand, mechanized so it doesn't depend on remembering to apply
the rule every time.

## 2. Editing against a stale or misremembered anchor

**Looks like:** constructing the text to search for from memory or from
context earlier in a conversation, rather than from a fresh, verified read
of the file as it currently exists — and then having that anchor either not
match (best case) or match somewhere unintended (worse case), or match
correctly at the moment of writing but the file having changed underneath
by the time the edit actually applies.

**Prevented by:** `ed.py find` returns a handle carrying the exact byte
span and a content hash; `apply` re-verifies that hash against the file's
current state and refuses — "stale handle, re-run find" — rather than
writing into a position whose surrounding content it can no longer vouch
for. Anchors are never hand-typed from memory; they always come from a
fresh `find`.

**Real incident:** anchors built from memory of what the text *should* say
are the documented root cause behind requiring `find` first, always — real,
confirmed damage happened more than once from exactly this: an anchor
consuming a function signature without restoring it, silently orphaning the
function body until a build finally failed on it; a separate incident
leaving a function's own closing braces dangling mid-file, caught only
because the file happened to be re-viewed afterward, not because anything
in the toolchain flagged it at the time.

## 3. A swallowed exit code letting downstream steps proceed anyway

**Looks like:** a guard or release step's output piped through `head`,
`tail`, or `grep` before its exit status is checked — the pipeline as a
whole reports success (or just doesn't get checked at all) even though the
step it's wrapping actually failed.

**Why it's dangerous here specifically:** this is the failure mode that
produces the most convincing false record — a changelog entry describing a
release step that never actually ran cleanly, a version bump applied on top
of a build that didn't pass. It doesn't look like a failure. It looks like
success, right up until someone tries to rely on what it claimed.

**Prevented by:** never piping an orchestrator's or a guard's own output
through anything before its exit code has been checked — the log exists for
reading afterward, the exit code is the only thing that gets trusted in the
moment. Where a guarded step feeds later steps, the whole sequence from
guard to final artifact runs as a single `&&` chain, not as
newline-separated commands that execute regardless of what came before.

**Real incident:** documented as a recurring defect three separate times in
this project's own history before the rule was written down explicitly —
recurring specifically because debugging momentum is exactly the moment
this kind of discipline is most likely to be forgotten and most needed.

## 4. Dormant guard rot

**Looks like:** a test that exists, is even correctly specified, and simply
never runs as part of the normal test invocation — gated behind a build
tag, an environment variable, specific hardware, or deferred to local
execution — so its own correctness (does the invariant it checks actually
hold, does the code it's testing even still exist) goes unverified
indefinitely.

**Prevented by:** every dormant guard is listed in a table with its gating
condition and a last-exercised date and environment. The release gate
checks that every entry has been exercised since the last tag, or that its
skip is explicitly recorded — never silently assumed current because it's
merely present in the codebase.

**Real incident:** the canonical case — a race guard shipped with its
invariant precisely specified, and the invariant it encoded turned out
never to have actually been implemented. Nobody caught it, because the
guard sat behind a build tag the default suite never touched. Discovered a
month later, the first time anyone actually ran it, and it failed
instantly. A guard's specification and its execution record are different
facts; only the second is evidence, and coverage of one guard is never
inferred from another guard's recorded campaign.

## 5. A tool's own maintained state getting corrupted by the tool itself

**Looks like:** the register, the tracking document, or any other file a
tool is supposed to keep consistent developing an internal inconsistency
*because of* an operation the tool itself performed — not external
tampering, a bug in the maintenance logic.

**Prevented by:** `register check` verifies the status table and the
per-item detail sections actually agree with each other, structurally —
not just that the file parses. This check is exercised against a
deliberately-broken fixture, not only well-formed data, specifically so it
proves it *catches* a real mismatch rather than merely passing when nothing
is wrong.

**Real incident:** `register.py close`'s own row-capture regex once matched
only five cells of a six-cell table row, orphaning the status row on
closure — found while closing the very item that had been filed to track a
different, unrelated bug. The tool caught its own defect on its first real
use, which is the entire reason `check` gets run against adversarial
fixtures now rather than trusted on the strength of its own design.

## 6. Documentation claiming a fix or removal that didn't actually land

**Looks like:** a changelog entry, a comment, or a register closure stating
that something was done — a directory removed, a bug fixed — when the
actual change never made it into the committed state, or got silently
reverted by something later.

**Why it's dangerous here specifically:** a future session (with no memory
of writing that changelog entry) has every reason to trust it. There's
nothing internally inconsistent about a false "done" claim; it only shows
up when someone happens to check the actual state against what was
claimed.

**Prevented by:** the closure procedure requires a verification record —
what ran, where, when — not just a status flip; register `check` and the
release gate both verify the register's own table against its detail
sections rather than trusting either in isolation.

**Real incident:** a dead, actively-misleading duplicate directory was
documented as removed in one changelog entry, complete with an explanation
of how it had been silently corrupting an earlier version of `selftest`.
It was still sitting on disk, unchanged, several releases later — the
documented fix had simply never been committed. Found only because a later
packaging pass happened to notice it was still there. The Go translation of
this same project independently turned up the identical pattern in its own
right: a `.gitignore` the changelog claimed was added in a specific release
was absent from that release's actual committed tree.

## 7. Packaging self-contamination from stale artifacts already in the tree

**Looks like:** a release archive step sweeping in a file it generates
itself — a manifest, a checksum file, a release log — because an earlier,
now-stale copy of that same generated file happens to already be sitting in
the source tree when the archive step runs, producing a duplicate or
corrupted entry in the output.

**Prevented by:** self-generated packaging output (manifest files, release
logs, state files, checkpoint zips) is excluded from archive sources by
name, not merely expected to be absent.

**Real incident:** a release archive builtin wrote a duplicate manifest
entry specifically because a stale manifest from an earlier checkpoint
session was still sitting at the archive root in a downstream repository's
working tree — a leftover from a prior extraction, unrelated to the
release currently running, that nonetheless got swept in as an ordinary
source file and then collided with the archiver's own freshly-generated
copy of the same file.

## 8. Naive delimiter-counting heuristics drifting from real language semantics

**Looks like:** a classifier that tracks "am I inside a string / comment /
code block" by counting delimiter-like characters across the whole file,
rather than genuinely tracking lexical state — so a delimiter character
that appears for an unrelated reason (inside a different kind of string,
inside a comment, as a literal example) throws off the count without
actually opening or closing anything, silently misclassifying everything
after it.

**Prevented by:** replacing naive counts with real stateful scans that
track comments, strings, and other constructs explicitly, and — critically
— checking the result against an independent, external ground truth
(a real compiler's or parser's own behavior, or a language's actual
specification) rather than trusting that the logic looks right on
inspection.

**Real incident:** this exact shape of bug turned up five separate times
across the role classifiers on direct audit — a Go classifier
misclassifying its own source after an unrelated backtick or block-comment
marker earlier in the file threw off a whole-file count; a Markdown fence
tracker that didn't account for delimiter length, so a shorter marker
inside a longer-delimited fence incorrectly closed it; the same tracker's
inline-code detection breaking on a single stray, unmatched backtick;
Python's triple-quote handling reading an escaped quote as a genuine
closing delimiter one character early; YAML's comment detection treating
any bare `#` as a comment regardless of what preceded it. All five were
confirmed against something outside the tool itself before being trusted
as real, and fixed identically everywhere the same logic was duplicated.

## 9. Trusting a single-core sandbox run as proof of concurrency safety

**Looks like:** a concurrency guard passing cleanly inside a sandboxed
test environment, and that pass being treated as evidence the underlying
race condition doesn't exist.

**Why it's dangerous here specifically:** a sandbox without true
multi-core parallelism structurally cannot exercise most races — a clean
pass there proves the code *ran*, not that it's *safe*, and the two are
easy to conflate under time pressure.

**Prevented by:** stress tests requiring genuine multi-core parallelism are
explicitly deferred to the human running real hardware, with the exact
invocation to run; a sandbox pass of a concurrency test is never accepted
as the evidence a dormant guard's last-exercised record requires.

**Real incident:** a codebase-precedent race condition survived months of
routine single-core sandbox testing without ever manifesting, standing as
the concrete reminder that reasoning about concurrency correctness — or a
clean CI run in an environment that can't actually stress it — is not the
same as verifying it.

## 10. Silent divergence between a translation and its original

**Looks like:** a ported or translated version of a tool quietly drifting
from the behavior of the original it was translated from — not because
anyone changed it on purpose, but because a bug in one side was never
checked against the other, so neither the port nor the original is
actually being validated against anything beyond its own internal logic.

**Prevented by:** the two implementations are tested against each other
directly and continuously — identical fixtures run through both, output
diffed byte-for-byte — rather than each being trusted independently on the
strength of its own test suite passing. A shared bug found this way gets
fixed in both together, deliberately, specifically because fixing only one
side would widen the gap rather than close it.

**Real incident:** a Go translation of this project accumulated several
real divergences from the Python original before being checked this way —
subcommands that crashed on `--help` instead of showing usage where the
original didn't, an item-range format that differed for a single-item edge
case. None of these were caught by either side's own test suite passing;
all of them surfaced only once the two were run side by side on purpose.
