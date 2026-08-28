# Why repoman exists

## The problem this is actually solving

Most editing tools are built on an assumption that doesn't hold here: that
the thing making the edit remembers making the last one, and the one before
that, and will still be around tomorrow to answer for a mistake.

An LLM editing a repository doesn't have that. Each session may start with
no memory of the last one — sometimes not even a summary, just a fresh
context and a working tree. The editor might be a different instance
entirely, hours or days later, that has never seen this repository make this
particular mistake before, because the version of it that saw it happen is
gone. Institutional memory that a human team keeps as scar tissue — *we
don't pipe the release script's output through `head`, we learned that the
hard way* — has nowhere to live unless something outside the editor's own
head writes it down and, ideally, makes the mistake structurally harder to
repeat rather than just documented.

repoman is that "something outside." It exists to convert one-time incidents
into permanent, mechanically-enforced properties of the toolchain, so the
next session — whoever or whatever is running it — inherits the lesson
without having to have lived through the incident.

## Prevention, not just recovery

There are two different things a tool can do about the fact that mistakes
happen:

- Make mistakes *cheap to undo* — checkpoints, auto-commits, an undo
  button.
- Make certain classes of mistake *impossible to make in the first place* —
  refuse the unsafe action before it happens, rather than clean up after it.

Both matter, and repoman does some of the first (the `ed` journal, `mark`/
`undo`). But its center of gravity is the second, and that's a deliberate
choice, not an oversight. Recovery assumes someone notices the mistake
before it compounds — reviews the diff, runs the tests, reads the commit
message closely enough to catch that it's describing something that didn't
actually happen. An editor working across long, possibly-compacted,
possibly-unsupervised sessions can't assume that someone is always watching
closely enough, fast enough, every time. So where it's practical, repoman
tries to make the wrong action simply not go through, rather than trusting
that it'll be caught and reverted later.

The tagline version: **a tool whose violations are structurally unmakeable
beats a tool whose violations are merely forbidden.** A rule that says
"don't do X" relies on the discipline of whoever's operating the tool, every
single time, forever. A refusal baked into the tool doesn't.

## The core principles

Each of these traces back to a real incident, not a hypothetical. The
incident is what earned the principle its place here — repoman doesn't
guard against things that merely seem risky in the abstract, it guards
against things that already happened once and got a mechanism written so
they can't happen the same way twice.

### Refuse, don't guess

When a tool can't be sure an action is safe, the correct response is a
clear, immediate refusal — not a best-effort attempt, not a silent
fallback, not proceeding with a caveat buried in the output. `ed.py sub`
refuses outright if the actual occurrence count doesn't match `--expect`.
`gomod.py` fails loudly and specifically when `go` isn't on `PATH`, rather
than silently skipping the checks that need it — because a check that
degrades quietly is indistinguishable, from the outside, of a check that
passed. A `go.sum`-incompleteness check that can't run because the network
is down reports itself as skipped, not green.

The alternative — quietly doing *something* reasonable-looking and moving
on — is how small uncertainties turn into large, undetected ones. A refusal
costs a moment of friction. A silent wrong guess costs however long it takes
someone to notice, which for an unsupervised agent might be never.

### Classify before you touch

A single string of text can mean completely different things depending on
where it sits — a variable name in real code, a string literal passed
somewhere else entirely, a comment, a table cell, prose. Treating every
occurrence of a search term as interchangeable is the single most common way
automated substitution destroys things it wasn't supposed to touch. `roles`
exists specifically to answer "what is this occurrence, syntactically,
*before* I touch it" — and `str_replace_extended`'s own safety check refuses
a substitution outright if the occurrences it's about to touch don't all
share the same role. One pass is safe only when every occurrence is the same
kind of thing; anything else needs to be split into separate, role-aware
passes.

This isn't a defense against a hypothetical. It's the direct, mechanized
answer to the discipline that string-substitution disasters keep proving
teams don't reliably apply by hand under time pressure.

### Journal everything; mark before you campaign

Every `ed`/`strreplace` edit is recorded — file, offset, old text, new
text, timestamp — in a shared journal, whether the edit came from a single
targeted `sub` or from `apply_payload`'s batch path. `mark` drops a named
checkpoint before any multi-file campaign begins; `undo --since <mark>`
reverts everything after it in one motion, in reverse order, stopping
cleanly at the first edit it can't safely revert (its target changed since)
rather than pretending the rest succeeded.

The point isn't just "undo exists." It's that undo is *precise* — scoped to
exactly the edits a specific piece of work made, not "revert the whole
working tree and hope nothing else was in flight," and not dependent on
whoever's editing having the presence of mind to `git commit` at exactly
the right granularity beforehand.

### A guard that never runs guards nothing

A test that exists, and is even well-specified, tells you nothing if it
never actually executes. The incident that earned this one its own registry
and its own release-gate check: a race guard was shipped with its invariant
precisely specified — and the invariant it encoded turned out never to have
been implemented at all. Nobody caught it, because the guard was gated
behind a build tag that the default test invocation never touched. It sat
there, specified and silent, until someone finally ran it by hand a month
later and it failed instantly.

`guards` exists because "the test exists" and "the test was verified
recently, for real, in an environment that could actually exercise it" are
different facts, and only the second one is evidence. Every dormant guard —
anything gated behind a build tag, an environment variable, specific
hardware, or deferred to local execution — gets a last-exercised date and an
environment. The release gate checks that date against the last release,
not just that the guard is *mentioned* somewhere.

### Fail loud, never silently degrade

A release script's exit code, piped through `head` or `grep` before being
checked, stops meaning anything — the pipeline as a whole can report
success while the actual step failed. This exact defect showed up enough
times in this project's own history to get named and guarded against
directly: never pipe an orchestrator's or a guard's output before its exit
status has been checked; where a guarded step feeds later steps
(changelog entries, version bumps, archive generation), the whole sequence
from guard to final artifact runs as one `&&` chain, never as
newline-separated commands that'll happily run regardless of what came
before. A guard that aborts correctly while everything downstream of it
proceeds anyway produces records describing changes that never happened —
which is worse than having no guard at all, because it looks like
diligence.

### Verify against ground truth, not against your own reasoning

Careful reasoning about what a classifier or a script *should* do is not
the same as checking what it actually does against an independent source of
truth. Every fix that's gone into this project's classifiers followed the
same discipline: build a fixture that isolates the specific failure shape,
then check the result against something outside the tool itself — a real
language's own compiler or parser (`ast.parse` for Python's escaping
rules), a real reference implementation (PyYAML for YAML's comment rule),
or a well-specified external standard (CommonMark's actual fence- and
code-span-matching rules) — rather than trusting that the code "looks
right" on inspection. Several real, shared bugs in the role classifiers
survived exactly that kind of inspection for a long time; every one of them
was found by testing against ground truth, not by reading the code harder.

## What this document set covers

This is the first of a staged set. `repoman-020-failure-modes.md` catalogs
the specific failure modes above (and others) as a scannable reference —
what each one looks like in practice, and which mechanism stops it. The
documents after that cover the tools themselves: getting started, precise
editing, role classification, the register and dormant-guard workflow,
releases, staged-work tracking, and the full `.repoman.json` configuration
reference.
