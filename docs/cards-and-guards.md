# Cards are for humans vouching. Guards are for machines proving.

### Why a kanban board has no equivalent ceremony for a conversation with an agent — and what enforcing the rule at the point of action looks like instead

Software project management is changing. Common sense, ethics and conduct are implicit in what a human developer brings, and with a human at the helm, you can still count on them.

Kanban boards exist because software teams needed a place for humans to vouch for each other (a card moves to "done" because someone attests it's done). That worked when a human wrote the code. But most developers now spend a large share of their time talking to agents, and that conversation has no equivalent ceremony: no register, no journal, nothing that catches a bad edit before it lands.

Management works by persuasion: you explain, you review, you build trust over time. Agents don't hold up their end of that. There is no version of an agent that can be mentored and guided the way a person can, there is only a prompt that produces something and a hope that what comes out is good. If you want more than hope, the only option left is to enforce the rule at the point of action.

Cards are for humans vouching. Guards are for machines proving.

That is what gorepoman [https://lnkd.in/dVJquQ9d] is for: the same register that feeds the board also drives the checks. Every edit is journaled and reversible before it is ever committed. A substitution that would touch prose and code at once is refused until it is split. A forbidden-string gate runs before any release and cannot be resumed around. A guard that was written but never run is recorded as exactly that, not as evidence. A diff that rewrites a docstring and quietly changes a return type in the same hunk isn't one edit (it's two edits wearing the same commit, one an agent can be trusted to make alone, one that needs a human to actually look at it).

The register doesn't ask whether the diff is dangerous. It asks whether the diff is one thing. If it isn't, it comes back split, before either half reaches the forbidden-string gate, because a check that clears the pair together is really only checking the half it understood.

This is not commit history with extra steps. A log tells you what happened, it can't tell you that a rule you thought was enforced was never wired up, and it can't stop a bad edit before it's made. The register does both. None of this replaces the human driver: it's what lets human attention go where only a human can put it.

Below is one view into that register, generated straight from it, not a separate status page: a card can't say "done" unless the guard behind it agrees, and can't say "ready" unless its dependencies actually cleared. Items only move from "Next" to "Now" once every blocker ahead of them is resolved, and "code complete, pending release" is tracked as its own state, not collapsed into "done." This board is just the part of the register that happens to render as a picture.

Every card here was checked before it moved, not after. Afterwards is mitigation.
