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

## Never `sed`, never `awk`, never the bare string-replace primitive

Line-by-line tools corrupt multiline strings, handle Unicode
platform-dependently, and become unreadable past trivial complexity —
and, more fundamentally, none of them know what a "role" is. `ed` and
`strreplace` exist specifically to replace that entire category of tool,
not to sit alongside it for the easy cases. See
`repoman-020-failure-modes.md` #1 and #2 for what happens without this
discipline, and `repoman-050-roles.md` for the classifier these tools both
depend on.
