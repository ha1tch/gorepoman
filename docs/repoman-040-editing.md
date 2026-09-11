# Precise editing: `ed` and `strreplace`

Two tools, one discipline: never edit against a remembered or assumed
version of a file's content, never let a substitution touch text it wasn't
actually verified to be safe for, and always leave a record that can be
undone precisely. Every example below is real output from actually running
these tools — not illustrative text written to look plausible.

## Which one, when

**`ed`** — one handle-verified edit at a time, or a same-text substitution
across a small, already-understood set of files. This is the tool for "I
found the exact span I want to change, let me change exactly that," and for
the `mark`/`undo` checkpoint discipline around a multi-step campaign.

**`ed insert`** — adding text next to a verified span without restating any
of the matched text itself. Reach for this instead of `apply` whenever the
change is "put something new before/after this," not "replace this with
something else" — see below for why the distinction matters.

**`ed niplines`/`confirm`/`cancel`** — removing a line range with no content
anchor to verify against, where you want to see the removal rendered as a
diff and (for a recognized source type) checked by a real formatter/vet
pass *before* anything is written, not after. Two calls instead of one:
this is the deliberate cost of a request/preview/confirm flow, worth paying
specifically when a line-range deletion is the kind of edit `find`'s
content-hash handle can't describe. See below for the full flow and why it
isn't the default for every removal.

**`strreplace`** (`str_replace_extended.py`) — batched, payload-driven
substitution with format-aware syntax validation and an explicit role
census check per operation. Reach for this when you're changing the same
text across several occurrences, or several files, in one atomic
transaction, and want the tool to refuse outright if any of those
occurrences turn out not to be the same kind of thing.

They share one journal (`.ed-journal.json`) — an edit made through either
tool shows up in `ed.py log`, and either tool's edit can be undone through
`ed.py undo`.

## `ed`: find, then apply

`find` never lets you hand-type an anchor. It returns a handle — file,
byte span, and a content hash — for every occurrence of a term:

```
$ repoman ed find maxRetries server.go
server.go:34-44:73022d1a  [go-code]  line 5: const maxRetries = 3
1 occurrence(s)
```

`apply` takes that handle and a replacement, and re-verifies the hash
before writing anything:

```
$ repoman ed apply server.go:34-44:73022d1a --with maxConnectRetries
applied at server.go:34: "maxRetries" -> "maxConnectRetries"
```

Every edit is recorded:

```
$ repoman ed log
txn 1  2026-08-28T15:15:01Z  apply server.go  (1 edit(s))
```

## `append`, `prepend`, `insert`: adding text without restating what's around it

`apply` replaces a matched span with `--with TEXT` — which means adding
text *next to* something existing means writing a `--with` value that
restates part of the match and appends to it. Get that restatement wrong
— drop a character, duplicate a line — and the anchor itself is what's
now corrupted, silently, since `apply` has no way to know the replacement
was supposed to still contain the original text. `append`, `prepend`, and
`insert` exist so adding text never requires typing out text that's
already there.

### `append`/`prepend`: no handle needed, because there's nothing to verify

Both are unconditionally safe — no `find` first, no handle — because the
target is a true file boundary (true EOF, true offset 0), not existing
text being trusted the way a matched span is:

```
$ repoman ed append README.md --with "

## License

MIT"
appended to README.md: "\n\n## License\n\nMIT" (17 bytes)

$ repoman ed prepend README.md --with "<!-- auto-generated header, do not edit -->
"
prepended to README.md: "<!-- auto-generated header, do not edit " (44 bytes)
```

Both are recorded in the journal exactly like any other edit — `log` and
`undo` treat them the same as an `apply`.

### `insert`: a handle-verified variant of `apply` that never touches the match

`insert` requires a `find`-verified handle exactly like `apply` does, and
re-runs the same `SpanHash` check before writing. The difference is which
side of the span the text lands on — `--after` splices at the end of the
match, `--before` at the start — and, critically, the matched text itself
is never part of what's written:

```
$ repoman ed find "| 010 | Getting started | Install and first run |" docs/index.md --brief
docs/index.md:57-106:5787603c  [md-table]  line 5: | 010 | Getting started | Install and first run |
1 occurrence(s)

$ repoman ed insert docs/index.md:57-106:5787603c --before --with "| 005 | Overview | What this tool is for |
"
inserted before at docs/index.md:57: "| 005 | Overview | What this tool is for"
```

```
$ repoman ed find "| 020 | Configuration | Every config key |" docs/index.md --brief
docs/index.md:107-149:eea22a5c  [md-table]  line 6: | 020 | Configuration | Every config key |
1 occurrence(s)

$ repoman ed insert docs/index.md:107-149:eea22a5c --after --with "
| 030 | New page | Description |"
inserted after at docs/index.md:149: "\n| 030 | New page | Description |"
```

```markdown
# Documentation

| # | Document | Covers |
|---|---|---|
| 005 | Overview | What this tool is for |
| 010 | Getting started | Install and first run |
| 020 | Configuration | Every config key |
| 030 | New page | Description |
```

This is why `insert` exists rather than just documenting "be careful with
`apply`": there is no `--with` value for `insert` that *could* reproduce
the anchor-duplication bug, because the matched span is never part of the
text you're asked to supply — you write only what's new, and `insert`
splices it at the verified boundary.

`insert` inherits `apply`'s stale-handle refusal, for the same reason —
holding a handle across an intervening edit means the context around the
span may no longer be what was verified:

```
$ repoman ed find "| 010 | Getting started | Install and first run |" docs/index.md --brief
docs/index.md:57-106:5787603c  [md-table]  line 5: | 010 | Getting started | Install and first run |
1 occurrence(s)

$ repoman ed sub "Every config key" "Every configuration key, exhaustively" docs/index.md --expect 1
  docs/index.md: 1
replaced 1 occurrence(s) across 1 file(s)

$ repoman ed insert docs/index.md:57-106:5787603c --after --with "
new row"
REFUSED: docs/index.md changed since find (stale handle) — re-run find and use a fresh handle
```

## `sub`: the same discipline, for repeated text

`sub` takes an old string, a new string, and a required `--expect N` — the
exact count of occurrences you believe exist. Get the count wrong and
nothing is written:

```
$ repoman ed sub "connecting to" "dialing" server.go --expect 1
  server.go: 1
replaced 1 occurrence(s) across 1 file(s)
```

If the occurrences span more than one syntactic role, `sub` refuses the
same way `strreplace` does (see below) unless you pass `--force-roles`
after actually checking the census — never as a way to silence the
refusal without looking.

## `mark` and `undo`

`mark` names a checkpoint before a multi-step campaign begins. `undo
--since <mark>` reverts everything after it, in reverse order, stopping
cleanly at the first edit it can't safely revert rather than pretending
the rest succeeded:

```
$ repoman ed mark before_rename
mark "before_rename" at txn 1
$ repoman ed sub "connecting to" "dialing" server.go --expect 1
  server.go: 1
replaced 1 occurrence(s) across 1 file(s)
$ repoman ed undo --since before_rename
undone: txn 2 (sub "connecting to"->"dialing")
```

Note what `undo --since before_rename` did *not* touch: the earlier
`maxRetries` → `maxConnectRetries` rename, from before the mark, stayed in
place. `mark`/`undo --since` is scoped precisely to what happened after the
checkpoint, not "revert everything ever done to this file."

## A real stale-handle refusal

Hold a handle, let something else change the file, then try to use it:

```
$ repoman ed find "connecting to" server.go
server.go:105-118:8fa86ff8  [go-dquote-string]  line 8: fmt.Println("connecting to", host)
1 occurrence(s)

$ repoman ed sub "func connect" "func Connect" server.go --expect 1
  server.go: 1
replaced 1 occurrence(s) across 1 file(s)

$ repoman ed apply server.go:105-118:8fa86ff8 --with "reaching out to"
REFUSED: server.go changed since find (stale handle) — re-run find and use a fresh handle
```

Worth noticing: the edit that invalidated the handle (`func connect` →
`func Connect`, on an earlier line) didn't change the byte *offsets* of the
`"connecting to"` span at all — same length, same position. It went stale
anyway, because the hash covers a context window around the span, not just
the span's own bytes. That's deliberate: an edit nearby is exactly the kind
of change that should make you re-verify before touching the neighboring
text, even when the raw offsets still line up.

## `strreplace`: payloads, not shell arguments

`repoman strreplace` never accepts search or replace text as CLI
arguments -- only as a base64-encoded JSON payload (`v: 1`, a list of
`ops`), passed as a file path or `-` for stdin:

```
$ repoman strreplace apply payload.json
```

Each `op` names a file, the search and replace text (base64-encoded), a
required `expect` count, and a required `roles` list -- the set of
syntactic roles every occurrence this op touches is allowed to have.

## A real role-mismatch refusal, and the correct fix

`handler.py`:

```python
DEFAULT_TIMEOUT = 30

def fetch(url, timeout=DEFAULT_TIMEOUT):
    # timeout is in seconds
    return call(url, timeout)
```

Searching for `timeout` finds three occurrences across two roles --
`python-code` twice, `python-comment` once. A payload asserting `roles:
["python-code"]` against all three refuses, and nothing gets written --
the refusal lands on stderr, the same structured result on stdout:

```
$ repoman strreplace apply payload.json
{
  "error": {
    "cls": "role-mismatch",
    "message": "handler.py: real role census {\"python-code\": 2, \"python-comment\": 1} is not a subset of asserted roles [python-code]. Split into one op per role, or correct the 'roles' list if the census is actually what you intended."
  },
  "ok": false,
  "ops_completed_before_refusal": 0
}
```

The correct fix is two ops, one per role. `occurrences` selects which
1-indexed matches an op touches (`expect` is checked *after* that filter --
it's the count this op is actually touching, not the raw total the search
found in the file):

```json
{
  "v": 1,
  "ops": [
    {
      "op": "sub", "file": "handler.py",
      "search_b64": "...", "replace_b64": "...",
      "expect": 2, "occurrences": [1, 3], "roles": ["python-code"]
    },
    {
      "op": "sub", "file": "handler.py",
      "search_b64": "...", "replace_b64": "...",
      "expect": 1, "roles": ["python-comment"]
    }
  ]
}
```

Op 1 touches occurrences 1 and 3 (the two `python-code` ones); op 2, run
after, finds the one remaining occurrence (the comment, now the only
`timeout` left in the file) and touches that:

```
$ repoman strreplace apply payload.json
{
  "dry_run_diffs": {},
  "markers": [],
  "ok": true,
  "ops": [
    {"applied": true, "file": "handler.py", "matched": 2, "op": "sub", "roles": {"python-code": 2}},
    {"applied": true, "file": "handler.py", "matched": 1, "op": "sub", "roles": {"python-comment": 1}}
  ],
  "syntax_results": {"handler.py": {"detail": "", "validated": true}},
  "written": ["handler.py"]
}
```

```python
DEFAULT_TIMEOUT = 30

def fetch(url, deadline=DEFAULT_TIMEOUT):
    # deadline is in seconds
    return call(url, deadline)
```

Every occurrence renamed, none of them touched by an op that hadn't
verified its role first.

## `niplines`, `confirm`, `cancel`: removing a line range, with a preview you approve before anything is written

A line range has no content anchor the way `find`'s handle does — there is
no substring being matched, only a position. `niplines`/`confirm`/`cancel`
handle that case as a deliberate two-phase flow instead of stretching
`apply`'s single-call model to cover it: request a removal, see exactly
what it would produce, then separately approve or discard it.

### Phase 1: `niplines` previews, checks, and issues a ticket — writes nothing

```
$ repoman ed niplines server.go 6 9
--- lines 3-12 of file (removing 6-9) ---
     3 | import "fmt"
     4 |
     5 | const maxRetries = 3
-    6 |
-    7 | func deprecatedHelper() {
-    8 | 	fmt.Println("this helper is no longer used")
-    9 | }
    10 |
    11 | func main() {
    12 | 	fmt.Println("starting up")

preflight: clean (gofmt, go vet)

ticket: nip-1  (expires 2026-09-11T03:27:43Z)
nothing written. `repoman ed confirm nip-1` to apply, `repoman ed cancel nip-1` to discard.
```

The preflight check is advisory, never a refusal — recognized source types
(currently `.go`, via `gofmt -l` and `go vet` against a scratch copy) get
checked against the *post-removal* content before you decide whether to
confirm. A file type with no registered checker reports
`preflight: not available for .ext` rather than silently skipping the line;
`niplines` always tells you whether it looked or not.

A dirty result still issues a ticket — the check is information for you to
weigh, not a gate:

```
$ repoman ed niplines server.go 5 5
--- lines 2-8 of file (removing 5-5) ---
     2 |
     3 | import "fmt"
     4 |
-    5 | const maxRetries = 3
     6 |
     7 | func main() {
     8 | 	fmt.Println("starting up")

preflight WARNING (advisory -- confirm anyway if this is expected):
  gofmt: would reformat this content

ticket: nip-2  (expires 2026-09-11T03:27:52Z)
nothing written. `repoman ed confirm nip-2` to apply, `repoman ed cancel nip-2` to discard.
```

### Phase 2: `confirm` redeems the ticket, or `cancel` discards it

```
$ repoman ed confirm nip-1
confirmed nip-1: removed lines 6-9 from server.go
```

Recorded in the journal exactly like `apply` — `log` and `undo` treat it
the same as any other edit:

```
$ repoman ed log
txn 1  2026-09-11T03:17:47Z  niplines server.go  (1 edit(s))
```

`confirm` re-hashes the live file against the hash recorded when the
ticket was issued, the same stale-handle discipline `apply`/`insert` use
for a `find` handle. Anything that changed the file between `niplines` and
`confirm` — including an edit through `ed` itself — invalidates the
ticket:

```
$ repoman ed niplines server.go 5 5
[...ticket: nip-2...]

$ repoman ed append server.go --with "
// a note added after the ticket was issued"
appended to server.go: "\n// a note added after the ticket was is" (44 bytes)

$ repoman ed confirm nip-2
REFUSED: server.go changed since niplines was requested (stale ticket) -- the previewed diff no longer matches what confirm would apply. Re-run niplines for a fresh preview and ticket. Nothing written.
```

`cancel` discards a ticket with no write, ever, and is idempotent —
cancelling twice, or cancelling something already confirmed or expired, is
not an error:

```
$ repoman ed cancel nip-2
cancelled nip-2. Nothing written.

$ repoman ed confirm nip-2
REFUSED: no pending ticket nip-2 (unknown, already confirmed/cancelled, or pruned)
```

### Why a ticket, and why it expires

A ticket defaults to a 10-minute TTL — long enough for the same agent turn
that requested the removal to also confirm it, short enough that a
half-finished flow doesn't linger indefinitely. `--ttl` raises that up to a
1-hour ceiling, for a human actually reading the preview before deciding.
Nothing longer is supported on purpose: a removal that needs more review
time than that belongs in `TRACKING.md` as its own tracked item, not as a
longer-lived ticket sitting outside the register where nothing else can
see it.

This is also why `niplines`/`confirm`/`cancel` are not how `apply`/`sub`/
`find` themselves work, and why they stay that way — a content-anchored
edit already has a verifiable handle and a single-call model that's
correct for it. The two-call ticket flow exists specifically for the case
that model doesn't cover: a position-based removal with no anchor to
re-verify in one shot, where a preview genuinely needs to be seen before
the write happens.

## Never `sed`, never `awk`, never the bare string-replace primitive

Line-by-line tools corrupt multiline strings, handle Unicode
platform-dependently, and become unreadable past trivial complexity —
and, more fundamentally, none of them know what a "role" is. `ed` and
`strreplace` exist specifically to replace that entire category of tool,
not to sit alongside it for the easy cases. See
`repoman-020-failure-modes.md` #1 and #2 for what happens without this
discipline, and `repoman-050-roles.md` for the classifier these tools both
depend on.
