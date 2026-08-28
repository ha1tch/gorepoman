# The role classifier

`roles` answers one question — "what kind of thing, syntactically, is this
occurrence of this text?" — for every language this toolchain touches. It's
what makes the mass-substitution refusal in `ed`/`strreplace` possible
(`repoman-040-editing.md`); this document covers the classifier itself: its
vocabulary, how to use it standalone, and — because these are heuristics,
not real parsers — exactly what each one does and doesn't model, honestly.

## Using it standalone

`roles` is both a library (`classify()`, used internally by `ed` and
`strreplace`) and a CLI auditor:

```
python3 roles.py <term> [path ...]
```

A single search term, run against a small mixed-language project:

```
$ python3 roles.py Port config.go notes.md settings.yaml
config.go:3: [go-comment] // Port is the default listen port
config.go:4: [go-code] const Port = 8080
notes.md:3: [md-inline-code] The default `Port` value is 8080.
notes.md:6: [md-fence] Port = 8080
settings.yaml:2: [yaml-code] Port: 8080

roles present: ['go-code', 'go-comment', 'md-fence', 'md-inline-code', 'yaml-code'] (5 occurrence(s))
MULTIPLE ROLES: a single substitution pass is NOT safe; write one targeted pass per role.
```

Five occurrences of the same literal string, five different roles, across
three languages, from one command — which is exactly the situation `ed
sub`/`strreplace` would refuse to touch in a single pass. Run `roles`
*before* a substitution across an unfamiliar or large set of files, not
just after a refusal — it's cheap, and it turns "how many roles am I about
to touch" from a guess into a fact.

## Role vocabulary

```
go-backtick-string | go-dquote-string | go-comment | go-code
md-fence | md-inline-code | md-table | md-heading | md-prose
python-string | python-comment | python-code
json-string | json-code
yaml-string | yaml-comment | yaml-code
shell-squote-string | shell-dquote-string | shell-backtick-string
shell-comment | shell-code
js-string | js-template-string | js-comment | js-code
ts-string | ts-template-string | ts-comment | ts-code
css-string | css-comment | css-code
html-comment | html-tag | html-attr-dquote | html-attr-squote | html-text
text
```

`text` is the fallback for any file extension none of the classifiers
below own.

## Per-language notes and known limits

These are heuristics, deliberately, not full parsers — fast, dependency-free
role classification for a mass-substitution safety check, not a compiler
front end. Every limitation below is a documented, deliberate scope
boundary, not a silent gap discovered by a user; several of them are
documented *because* they were bugs once, found and fixed by checking
against outside ground truth rather than trusting the code on inspection
(the full incident list is in the Go-port `CHANGELOG.md`; the short version
is in `repoman-020-failure-modes.md` #8).

**Go** — a real stateful scan tracks line comments, block comments, rune
literals, double-quoted strings, and backtick raw strings together, so a
delimiter character appearing inside one of those (a stray backtick in a
comment, a `/*` inside a string) can't be mistaken for a genuine one
elsewhere in the file. No known gaps at this vocabulary's grain.

**Markdown** — fence detection matches CommonMark's actual rule: a fence
opens on a line of 3+ backticks and closes only on a *later* line whose run
is at least as long, with nothing else on it, so a shorter example marker
written inside a longer-delimited fence doesn't prematurely close it.
Inline-code detection uses the same real run-length matching CommonMark
uses for code spans — an unmatched backtick is literal text, not an opener.
Table and heading detection is a simple line-prefix check (`|`, `#`); table
rows and headings aren't otherwise structurally verified.

**Python** — a whole-file stateful scan; triple-quoted strings can span
lines so (unlike Go's line-local approach) this one has to walk from the
start of the file. Backslash-escaping is honoured identically for
single- and triple-quoted strings. F-string `{expr}` interiors classify as
plain `python-string`, not a separate role — the delimiter-integrity check
in `strreplace` already covers the structural risk there, so this stays a
role heuristic rather than a real f-string parser.

**JSON** — the simplest of these: one string delimiter, one escape
character, no comments.

**YAML** — line-local, since flow-style quoted scalars don't span lines. A
`#` starts a comment only at line-start or when preceded by whitespace,
matching shell's rule — a URL fragment or any other mid-word `#` in a plain
scalar doesn't get mistaken for one. Known gap: PyYAML itself additionally
treats a `#` as starting a comment immediately after a closing quote or
flow indicator with *no* preceding space, which this whitespace-only
heuristic doesn't model — that would need flow-context tracking beyond what
a line-local scan does. Block scalars (`|`, `>`) aren't modeled; their body
lines classify as `yaml-code` even though they're technically string
content.

**Shell** — single quotes correctly have no escape handling at all (real
shell semantics: single-quoted content is 100% literal, not even a
backslash escapes anything inside it); double quotes and backtick command
substitution are escape-aware. A `#` starts a comment only where a real
shell would start a new token — preceded by whitespace, `;`, `|`, `&`, `(`,
or at the very start of the file. Known gap: heredocs (`<<EOF ... EOF`)
aren't modeled; their body misclassifies as `shell-code` — treat
heredoc-containing files as needing manual review here.

**JavaScript / TypeScript** — one scanner, shared by both (TypeScript's
type-level syntax introduces no new string/comment/template delimiter). A
real state *stack*, not a single state variable, specifically because
template literals nest arbitrarily — a `${...}` substitution can itself
contain another template literal, which can contain another substitution.
Known gaps, both named rather than silently wrong: regex literals aren't
specially recognised (the classic regex-vs-division lexing ambiguity is
unresolved), and JSX tag/expression structure is opaque — attribute strings
and template literals inside a JSX expression container still classify
correctly, but the tag structure itself isn't modeled.

**CSS** — block comments and both quote types, backslash-escaped. Known
gap: an unquoted `url(...)` value's content classifies as plain
`css-code`, not distinguished from a selector, property, or value — the
same string/comment/code granularity as the JSON classifier, no finer.

**HTML** — delegates to the JS/CSS scanners above for anything inside a
real `<script>`/`<style>` body, so editing inside embedded script gets
JS-aware roles rather than one undifferentiated `html-text`. Otherwise:
comments, tags, and single-/double-quoted attribute values, each their own
role. Known gap, and a deliberate one: a literal, unescaped `</script>`
inside a script body's own string content is indistinguishable from a
genuine closing tag — this is the same ambiguity real browsers have for
exactly this reason (the standard workaround in real code is escaping it as
`<\/script>`), not a rougher approximation than actual HTML parsing.
Inline event-handler (`onclick=`) and inline `style=` attribute content
is *not* delegated to JS/CSS — it classifies as plain `html-attr-*`.

## Not yet supported at all

SQL as standalone `.sql` files, Z80/x86 assembly, and `ual` (a first-party,
still-evolving language whose string/comment rules aren't settled enough
yet to be worth building a classifier around before they are). SQL built
via Go string concatenation is already covered by Go's own classifier —
it's standalone `.sql` files specifically that have no classifier. Add
support for any of these the same way JS/TS/CSS/HTML earned theirs: a real,
current project actively needing it, not a speculative future one.
