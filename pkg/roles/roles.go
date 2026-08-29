// Package roles implements syntactic-role classification for text
// occurrences -- translated from repoman/roles.py.
//
// Mechanizes the mass-substitution rule: before any substitution,
// classify every occurrence of the target by its syntactic role; a
// single pass is safe only when all occurrences share one role and
// one correct treatment. This package answers "what roles does this
// text appear in?" so that judgment starts from facts.
//
// Used both as a library (pkg/ed and pkg/strreplace call Classify)
// and as a CLI auditor (`repoman roles <term> [path ...]`), which
// prints every occurrence with its role. Roles are HEURISTIC and
// advisory -- they inform the classification step, they do not
// replace it.
//
// Role vocabulary:
//
//	go-backtick-string | go-dquote-string | go-comment | go-code
//	md-fence | md-inline-code | md-table | md-heading | md-prose
//	python-string | python-comment | python-code
//	json-string | json-code
//	yaml-string | yaml-comment | yaml-code
//	shell-squote-string | shell-dquote-string | shell-backtick-string
//	shell-comment | shell-code
//	js-string | js-template-string | js-comment | js-code
//	ts-string | ts-template-string | ts-comment | ts-code
//	css-string | css-comment | css-code
//	html-comment | html-tag | html-attr-dquote | html-attr-squote | html-text
//	text
//
// Known classifier limitations (heuristic by design, documented
// rather than silently wrong -- see each role function's own doc
// comment for detail): YAML block scalars (| and >) misclassify as
// yaml-code; shell heredocs misclassify as shell-code; Python
// f-string {expr} interiors classify as python-string, not a
// separate role; JS/TS regex literals are not specially recognised
// (the classic regex-vs-division ambiguity is not resolved) and JSX
// tag/expression structure is opaque, both documented on jsScan;
// HTML's <script>/<style> BODY content delegates to js-*/css-*
// roles, but inline onclick=/style= attribute content does not,
// classifying as plain html-attr-*.
//
// Not yet supported at all -- deferred, not forgotten: SQL
// (standalone .sql files -- the dangerous case, SQL built via Go
// string concatenation, is already covered by go-code's own
// delimiter-integrity awareness), Z80/x86 assembly (.asm), and ual (a
// first-party, still-evolving language whose own string/comment rules
// aren't settled enough yet for a classifier to be worth building
// before they are). Add support for any of these the same way
// JS/TS/CSS/HTML earned theirs: a real, current project actively
// needing it, not a speculative future one.
package roles

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

// goScanState is the lexical state at a byte offset within Go source,
// as determined by goScanTo's stateful forward scan.
type goScanState int

const (
	goNormal goScanState = iota
	goLineComment
	goBlockComment
	goDquote
	goRuneLit
	goBacktick
)

// goScanTo scans text from the start up to offset, tracking line
// comments, block comments, rune literals, double-quoted strings, and
// backtick raw strings, and returns the lexical state at offset.
// goRole uses this for backtick strings and block comments -- the two
// constructs that can span multiple lines -- rather than the naive
// whole-text delimiter counts each used to rely on (an odd backtick
// count for raw strings; "/*" count exceeding "*/" count for block
// comments). Both naive counts got the same class of thing wrong: a
// delimiter-like sequence appearing inside a comment, rune literal, or
// double-quoted string earlier in the file throws off the count
// without actually opening or closing anything, misclassifying
// everything after it for the rest of the file. Found via two separate
// real cases: roles.go's own "roles present" Printf line, for the
// backtick count; a string literal containing an unmatched "/*"
// permanently flipping the block-comment fallback for the rest of the
// file (plain code after it misclassified as go-comment, even past a
// real block comment properly closing later on), for the block-comment
// count -- confirmed with a dedicated fixture, not just reasoned about.
func goScanTo(text string, offset int) goScanState {
	if offset > len(text) {
		offset = len(text)
	}
	state := goNormal
	n := len(text)
	i := 0
	for i < offset {
		c := text[i]
		switch state {
		case goLineComment:
			if c == '\n' {
				state = goNormal
			}
			i++
		case goBlockComment:
			if c == '*' && i+1 < n && text[i+1] == '/' {
				state = goNormal
				i += 2
			} else {
				i++
			}
		case goBacktick:
			if c == '`' {
				state = goNormal
			}
			i++
		case goDquote:
			if c == '\\' && i+1 < n {
				i += 2
			} else {
				if c == '"' {
					state = goNormal
				}
				i++
			}
		case goRuneLit:
			if c == '\\' && i+1 < n {
				i += 2
			} else {
				if c == '\'' {
					state = goNormal
				}
				i++
			}
		default:
			switch {
			case c == '`':
				state = goBacktick
				i++
			case c == '"':
				state = goDquote
				i++
			case c == '\'':
				state = goRuneLit
				i++
			case c == '/' && i+1 < n && text[i+1] == '/':
				state = goLineComment
				i += 2
			case c == '/' && i+1 < n && text[i+1] == '*':
				state = goBlockComment
				i += 2
			default:
				i++
			}
		}
	}
	return state
}

func goRole(text string, offset int) string {
	// Backtick strings and block comments can both span lines: resolve
	// them with a real stateful scan, not naive whole-text delimiter
	// counts (see goScanTo's own doc comment for why the naive versions
	// were wrong -- and wrong in real, exploitable ways, not just in
	// theory).
	switch goScanTo(text, offset) {
	case goBacktick:
		return "go-backtick-string"
	case goBlockComment:
		return "go-comment"
	}
	lineStart := strings.LastIndex(text[:offset], "\n") + 1
	line := text[lineStart:offset]
	// Comment?
	if strings.Contains(stripGoStrings(line), "//") {
		return "go-comment"
	}
	// Double-quoted string state within the line.
	inDQ := false
	for i := 0; i < len(line); i++ {
		c := line[i]
		if c == '\\' && inDQ {
			i++
			continue
		}
		if c == '"' {
			inDQ = !inDQ
		}
	}
	if inDQ {
		return "go-dquote-string"
	}
	return "go-code"
}

func stripGoStrings(line string) string {
	var out strings.Builder
	inDQ := false
	for i := 0; i < len(line); i++ {
		c := line[i]
		if c == '\\' && inDQ {
			i++
			continue
		}
		if c == '"' {
			inDQ = !inDQ
			continue
		}
		if !inDQ {
			out.WriteByte(c)
		}
	}
	return out.String()
}

// mdInFence reports whether offset falls inside a Markdown fenced
// code block. Tracks actual CommonMark-ish fence-matching semantics --
// a fence opens on a line of 3+ backticks (optionally indented up to 3
// spaces, optionally followed by an info string); it closes on a LATER
// such line whose backtick run is at least as long as the opener's and
// has nothing else on it -- rather than a naive count of every line-
// leading run of 3+ backticks as an equal toggle regardless of length.
// The naive version misclassifies a SHORTER backtick run appearing as
// literal content inside a LONGER-delimited fence (e.g. a line
// demonstrating what a fence marker looks like, written inside a
// 4-backtick outer fence so the example itself doesn't terminate the
// outer fence) as closing that fence -- and everything genuinely still
// inside it afterward as prose instead. Confirmed with a dedicated
// nested-fence fixture, not just reasoned about: a single unpaired
// 3-backtick line inside a 4-backtick fence flips this in both
// directions at once (content still inside the fence reads as prose;
// real prose after the fence actually closes reads as still-fenced).
func mdInFence(text string, offset int) bool {
	openLen := 0
	pos := 0
	n := len(text)
	for pos < offset {
		nl := strings.IndexByte(text[pos:], '\n')
		var lineEnd int
		if nl == -1 {
			lineEnd = n
		} else {
			lineEnd = pos + nl
		}
		if lineEnd > offset {
			break
		}
		line := text[pos:lineEnd]
		stripped := strings.TrimLeft(line, " ")
		indent := len(line) - len(stripped)
		if indent <= 3 {
			run := 0
			for run < len(stripped) && stripped[run] == '`' {
				run++
			}
			if run >= 3 {
				rest := strings.TrimSpace(stripped[run:])
				if openLen == 0 {
					openLen = run
				} else if run >= openLen && rest == "" {
					openLen = 0
				}
			}
		}
		if nl == -1 {
			break
		}
		pos = lineEnd + 1
	}
	return openLen != 0
}

type mdBacktickRun struct {
	start, length int
}

// mdLineBacktickRuns finds every maximal run of backtick characters in
// line, as (start, length) pairs.
func mdLineBacktickRuns(line string) []mdBacktickRun {
	var runs []mdBacktickRun
	i := 0
	n := len(line)
	for i < n {
		if line[i] == '`' {
			j := i
			for j < n && line[j] == '`' {
				j++
			}
			runs = append(runs, mdBacktickRun{i, j - i})
			i = j
		} else {
			i++
		}
	}
	return runs
}

// mdInInlineCode reports whether offsetInLine falls inside a Markdown
// inline code span on this line, using CommonMark's actual matching
// rule -- a backtick run opens a span only if a LATER run of the exact
// same length exists later on the line to close it; an unmatched run
// is literal text, not a delimiter -- instead of a naive per-character
// backtick parity count. The naive version treated every single
// backtick character as an equal toggle regardless of run length or
// whether it has a real partner, so a stray, unmatched backtick
// anywhere earlier on the line -- with no matching closer anywhere in
// the rest of the line -- permanently flipped "inside code" for
// everything after it on that line, even though CommonMark renders
// that backtick as ordinary literal text and everything after it as
// ordinary prose. Confirmed with a dedicated fixture: a line with one
// genuine `a`-style code span, then one unrelated stray backtick with
// no partner, then a word that should read as plain prose -- the naive
// count misread it as still being inside code.
func mdInInlineCode(line string, offsetInLine int) bool {
	runs := mdLineBacktickRuns(line)
	i := 0
	n := len(runs)
	for i < n {
		start, length := runs[i].start, runs[i].length
		if start >= offsetInLine {
			return false
		}
		j := i + 1
		closer := -1
		for j < n {
			if runs[j].length == length {
				closer = j
				break
			}
			j++
		}
		if closer == -1 {
			i++
			continue
		}
		closeStart, closeLength := runs[closer].start, runs[closer].length
		closeEnd := closeStart + closeLength
		if offsetInLine < closeEnd {
			return true
		}
		i = closer + 1
	}
	return false
}

func mdRole(text string, offset int) string {
	before := text[:offset]
	if mdInFence(text, offset) {
		return "md-fence"
	}
	lineStart := strings.LastIndex(before, "\n") + 1
	lineEnd := strings.Index(text[offset:], "\n")
	var line string
	if lineEnd == -1 {
		line = text[lineStart:]
	} else {
		line = text[lineStart : offset+lineEnd]
	}
	if mdInInlineCode(line, offset-lineStart) {
		return "md-inline-code"
	}
	trimmed := strings.TrimLeft(line, " \t")
	if strings.HasPrefix(trimmed, "|") {
		return "md-table"
	}
	if strings.HasPrefix(trimmed, "#") {
		return "md-heading"
	}
	return "md-prose"
}

// jsScan does a whole-text forward scan with an explicit state STACK,
// not a single state variable -- needed because template literals
// nest arbitrarily (a `${...}` substitution can itself contain
// another template literal, which can contain another substitution,
// and so on; this is idiomatic in real JS/TS, not an edge case). A
// single-state scan, as used for Python/Go strings, would
// mis-terminate at the FIRST inner backtick and misclassify
// everything after it.
//
// Frames: {"str", delim} for '/" strings; {"tmpl"} for a template
// literal's own text; {"subst", depth} for the code inside a `${`
// substitution, tracking brace depth so nested braces (object
// literals, arrow-function bodies) inside the substitution don't
// close it early; {"lc"} / {"bc"} for // and /* */ comments. lang
// ("js" or "ts") only changes the returned role names -- TypeScript's
// type-level syntax (annotations, generics, interfaces) introduces no
// new string/comment/template delimiter, so the scan itself is
// identical for both.
//
// KNOWN LIMITATION, documented rather than silently wrong: regex
// literals (/pattern/flags) are not specially recognised -- the
// classic regex-vs-division lexing ambiguity in JS is not resolved
// here. A regex body is scanned as ordinary code; if it contains an
// unescaped quote or comment-opening sequence, this can misclassify
// following text. Rare in practice, and the delimiter-integrity check
// in pkg/strreplace still catches most resulting damage via
// role-divergence -- the same mitigation already relied on for
// YAML's own block-scalar limitation and shell's own heredoc
// limitation. JSX (`<Component>...</Component>`, `{expr}` containers
// in .jsx/.tsx) is likewise not specially modelled: JSX attribute
// strings and any template literals inside a JSX expression container
// are still classified correctly (same quote delimiters as plain
// JS), but element/tag structure itself is opaque here, the same
// boundary htmlRole draws around embedded script/style bodies.
type jsFrame struct {
	kind  string // "lc" | "bc" | "str" | "tmpl" | "subst"
	delim byte   // for "str"
	depth int    // for "subst"
}

func jsScan(text string, offset int, lang string) string {
	var stack []jsFrame
	i := 0
	for i < offset {
		c := text[i]
		var top *jsFrame
		if len(stack) > 0 {
			top = &stack[len(stack)-1]
		}
		if top == nil {
			if i+2 <= len(text) && text[i:i+2] == "//" {
				stack = append(stack, jsFrame{kind: "lc"})
				i += 2
				continue
			}
			if i+2 <= len(text) && text[i:i+2] == "/*" {
				stack = append(stack, jsFrame{kind: "bc"})
				i += 2
				continue
			}
			if c == '\'' || c == '"' {
				stack = append(stack, jsFrame{kind: "str", delim: c})
				i++
				continue
			}
			if c == '`' {
				stack = append(stack, jsFrame{kind: "tmpl"})
				i++
				continue
			}
			i++
			continue
		}
		switch top.kind {
		case "lc":
			if c == '\n' {
				stack = stack[:len(stack)-1]
			}
			i++
		case "bc":
			if i+2 <= len(text) && text[i:i+2] == "*/" {
				stack = stack[:len(stack)-1]
				i += 2
				continue
			}
			i++
		case "str":
			if c == '\\' {
				i += 2
				continue
			}
			if c == top.delim {
				stack = stack[:len(stack)-1]
			}
			i++
		case "tmpl":
			if c == '\\' {
				i += 2
				continue
			}
			if c == '`' {
				stack = stack[:len(stack)-1]
				i++
				continue
			}
			if i+2 <= len(text) && text[i:i+2] == "${" {
				stack = append(stack, jsFrame{kind: "subst", depth: 1})
				i += 2
				continue
			}
			i++
		case "subst":
			if i+2 <= len(text) && text[i:i+2] == "//" {
				stack = append(stack, jsFrame{kind: "lc"})
				i += 2
				continue
			}
			if i+2 <= len(text) && text[i:i+2] == "/*" {
				stack = append(stack, jsFrame{kind: "bc"})
				i += 2
				continue
			}
			if c == '\'' || c == '"' {
				stack = append(stack, jsFrame{kind: "str", delim: c})
				i++
				continue
			}
			if c == '`' {
				stack = append(stack, jsFrame{kind: "tmpl"})
				i++
				continue
			}
			if c == '{' {
				top.depth++
				i++
				continue
			}
			if c == '}' {
				if top.depth == 1 {
					stack = stack[:len(stack)-1]
				} else {
					top.depth--
				}
				i++
				continue
			}
			i++
		}
	}
	if len(stack) == 0 {
		return lang + "-code"
	}
	switch stack[len(stack)-1].kind {
	case "subst":
		return lang + "-code"
	case "str":
		return lang + "-string"
	case "tmpl":
		return lang + "-template-string"
	case "lc", "bc":
		return lang + "-comment"
	}
	return lang + "-code"
}

func jsRole(text string, offset int) string { return jsScan(text, offset, "js") }
func tsRole(text string, offset int) string { return jsScan(text, offset, "ts") }

// cssRole does a whole-text forward scan: /* */ block comments (CSS
// has no line comments), '...'/"..." strings with backslash escapes.
// KNOWN LIMITATION: an unquoted url(...) value's content is
// classified as plain css-code, not distinguished from a
// selector/property/value -- this classifier draws the same
// string/comment/code boundary as the JSON classifier, no
// finer-grained than that.
func cssRole(text string, offset int) string {
	i := 0
	state := "" // "" | "comment" | "str"
	var delim byte
	for i < offset {
		c := text[i]
		if state == "" {
			if i+2 <= len(text) && text[i:i+2] == "/*" {
				state = "comment"
				i += 2
				continue
			}
			if c == '\'' || c == '"' {
				state = "str"
				delim = c
				i++
				continue
			}
			i++
			continue
		}
		if state == "comment" {
			if i+2 <= len(text) && text[i:i+2] == "*/" {
				state = ""
				i += 2
				continue
			}
			i++
			continue
		}
		// state == "str"
		if c == '\\' {
			i += 2
			continue
		}
		if c == delim {
			state = ""
		}
		i++
	}
	switch state {
	case "":
		return "css-code"
	case "comment":
		return "css-comment"
	default:
		return "css-string"
	}
}

var htmlEmbedOpenRe = regexp.MustCompile(`(?i)<\s*(script|style)\b[^>]*>`)

type htmlSpan struct {
	start, end int
	lang       string
}

// htmlEmbeddedSpans returns every <script>...</script> /
// <style>...</style> BODY span (the text strictly between the
// opening tag's own closing '>' and the matching closing tag), in
// document order. A separate pre-pass rather than interleaving
// delegation into htmlRole's own scan below -- finding matching
// spans first, then deciding whether offset falls in one, is far
// simpler to get right than threading a sub-scanner into a single
// linear walk.
//
// KNOWN LIMITATION: the closing-tag search is a literal,
// case-insensitive text search for </script> or </style> -- it does
// not parse attributes on the closing tag (not valid HTML in
// practice), and like a real browser's own HTML parser, has no way
// to distinguish a literal "</script>" appearing unescaped inside a
// script body's own string content from a genuine closing tag. Real
// HTML has the identical hazard for exactly this reason (the
// documented escape is <\/script>); this is not a rougher
// approximation than HTML's own parsing rules, it is the same one.
// Span is one embedded <script>/<style> body span, as returned by
// HTMLEmbeddedSpans.
type Span struct {
	Start, End int
	Lang       string
}

// HTMLEmbeddedSpans exposes htmlEmbeddedSpans for callers outside this
// package (pkg/strreplace's HTML syntax validator reuses this to
// exclude embedded <script>/<style> bodies from its own tag-matching,
// the same way str_replace_extended.py reaches into roles.py's own
// _html_embedded_spans despite the leading underscore -- Python
// doesn't enforce that privacy, Go does, so this thin exported
// wrapper is the one addition needed to preserve that reuse).
func HTMLEmbeddedSpans(text string) []Span {
	spans := htmlEmbeddedSpans(text)
	out := make([]Span, len(spans))
	for i, s := range spans {
		out[i] = Span{Start: s.start, End: s.end, Lang: s.lang}
	}
	return out
}

func htmlEmbeddedSpans(text string) []htmlSpan {
	var spans []htmlSpan
	for _, m := range htmlEmbedOpenRe.FindAllStringSubmatchIndex(text, -1) {
		tagname := strings.ToLower(text[m[2]:m[3]])
		bodyStart := m[1]
		closeRe := regexp.MustCompile(`(?i)</\s*` + tagname + `\s*>`)
		loc := closeRe.FindStringIndex(text[bodyStart:])
		bodyEnd := len(text)
		if loc != nil {
			bodyEnd = bodyStart + loc[0]
		}
		lang := "css"
		if tagname == "script" {
			lang = "js"
		}
		spans = append(spans, htmlSpan{start: bodyStart, end: bodyEnd, lang: lang})
	}
	return spans
}

// htmlRole delegates to jsRole/cssRole for any offset falling inside
// an embedded <script>/<style> body (see htmlEmbeddedSpans) -- an
// editor working inside embedded script needs JS-aware roles, not a
// single undifferentiated "html-text". Otherwise, a whole-text
// forward scan over plain HTML: <!-- --> comments, < > tags with
// attribute values tracked in their own roles (single- vs
// double-quoted, matching the same multi-quote-type granularity
// shell-squote-string/shell-dquote-string already established), and
// plain text content between tags.
//
// KNOWN LIMITATION, named rather than silently unhandled: inline
// event-handler attributes (onclick="...") and inline style
// attributes (style="...") are NOT delegated to jsRole/cssRole --
// their content classifies as plain html-attr-dquote/squote, the
// same as any other attribute value.
func htmlRole(text string, offset int) string {
	for _, span := range htmlEmbeddedSpans(text) {
		if span.start <= offset && offset < span.end {
			sub := text[span.start:span.end]
			rel := offset - span.start
			if span.lang == "js" {
				return jsRole(sub, rel)
			}
			return cssRole(sub, rel)
		}
		if offset < span.start {
			break // spans are in document order; none later can match either
		}
	}
	i := 0
	state := "text"
	for i < offset {
		c := text[i]
		switch state {
		case "text":
			if i+4 <= len(text) && text[i:i+4] == "<!--" {
				state = "comment"
				i += 4
				continue
			}
			if c == '<' {
				state = "tag"
				i++
				continue
			}
			i++
		case "comment":
			if i+3 <= len(text) && text[i:i+3] == "-->" {
				state = "text"
				i += 3
				continue
			}
			i++
		case "tag":
			if c == '"' {
				state = "attr-dq"
				i++
				continue
			}
			if c == '\'' {
				state = "attr-sq"
				i++
				continue
			}
			if c == '>' {
				state = "text"
				i++
				continue
			}
			i++
		case "attr-dq":
			if c == '"' {
				state = "tag"
			}
			i++
		case "attr-sq":
			if c == '\'' {
				state = "tag"
			}
			i++
		}
	}
	switch state {
	case "comment":
		return "html-comment"
	case "tag":
		return "html-tag"
	case "attr-dq":
		return "html-attr-dquote"
	case "attr-sq":
		return "html-attr-squote"
	default:
		return "html-text"
	}
}

// pythonRole does a whole-text forward scan tracking string/comment
// state. Triple-quoted strings can span lines, so (unlike Go's
// line-local scan) this must walk from the start of the file --
// O(offset) per call, acceptable at source-file scale. f-strings are
// classified as ordinary python-string (no separate role): the
// {expr} interior of an f-string is still Python code, but
// distinguishing it would need real brace-depth tracking inside the
// string, which the delimiter-integrity check in pkg/strreplace's own
// precheck already covers structurally -- this classifier stays a
// role heuristic.
//
// Backslash-escaping is honoured for BOTH single- and triple-quoted
// delimiters, not just single-char ones -- Python's own escaping
// rules apply identically inside triple-quoted strings (a backslash
// always consumes the next character as a unit, same as in a plain
// quoted string). An earlier version only skipped the escaped
// character when len(delim) == 1, so an escaped quote immediately
// followed by two more literal quote characters -- still fully legal
// inside a triple-quoted string -- was misread as a genuine closing
// triple-quote one character too early, confirmed against CPython's
// own ast.parse as ground truth, not just reasoned about. That
// false-early close then cascades: content genuinely still inside
// the string reads as code, and the real closing triple-quote -- now
// seen while "outside" a string -- gets misread as OPENING a brand
// new one, which stays open (misreading everything after it as still
// being inside a string) until a further matching delimiter happens
// to appear, if ever.
func pythonRole(text string, offset int) string {
	i := 0
	inString := false
	var delim string
	for i < offset {
		c := text[i]
		if !inString {
			if i+3 <= len(text) && (text[i:i+3] == `"""` || text[i:i+3] == "'''") {
				inString = true
				delim = text[i : i+3]
				i += 3
				continue
			}
			if c == '"' || c == '\'' {
				inString = true
				delim = string(c)
				i++
				continue
			}
			if c == '#' {
				nl := strings.Index(text[i:], "\n")
				if nl == -1 || offset <= i+nl {
					return "python-comment"
				}
				i = i + nl + 1
				continue
			}
			i++
			continue
		}
		if c == '\\' {
			i += 2
			continue
		}
		if i+len(delim) <= len(text) && text[i:i+len(delim)] == delim {
			i += len(delim)
			inString = false
			delim = ""
			continue
		}
		i++
	}
	if inString {
		return "python-string"
	}
	return "python-code"
}

// jsonRole does a whole-text forward scan. JSON has exactly one
// string delimiter ("), one escape char, no comments -- the simplest
// of these.
func jsonRole(text string, offset int) string {
	i := 0
	inStr := false
	for i < offset {
		c := text[i]
		if inStr {
			if c == '\\' {
				i += 2
				continue
			}
			if c == '"' {
				inStr = false
			}
			i++
			continue
		}
		if c == '"' {
			inStr = true
		}
		i++
	}
	if inStr {
		return "json-string"
	}
	return "json-code"
}

// yamlRole does a line-local scan: flow-style quoted scalars
// ('...'/"...") don't span lines, so line-local is correct for them.
// KNOWN LIMITATION, documented rather than silently wrong: block
// scalars (| and >) are NOT modeled -- their body lines classify as
// yaml-code even though they are technically string content.
// pkg/strreplace's own delimiter-integrity precheck (role-divergence,
// not delimiter tables) still catches most damage from an edit
// landing inside one, since introducing new YAML structure
// mid-block-scalar changes indentation/role for everything after it.
//
// A "#" only starts a comment at the start of the line or when
// preceded by whitespace -- matching the same rule shellRole already
// applies, and confirmed against PyYAML's own parser as ground
// truth: "url: http://x.com#frag" keeps the "#frag" as part of the
// plain scalar value (no preceding whitespace), while a trailing
// " #comment" after it is a real comment. An earlier version treated
// every bare "#" outside quotes as a comment regardless of what
// preceded it, so a URL fragment or any other mid-word "#" in a
// plain scalar wrongly turned the rest of that line into
// yaml-comment. KNOWN LIMITATION, not chased further: PyYAML also
// treats "#" as starting a comment immediately after a CLOSING quote
// or flow indicator with no space (e.g. "'value'#c"), which this
// whitespace-only heuristic does not model -- full fidelity there
// would need flow-context tracking well beyond what this line-local
// scan does elsewhere.
func yamlRole(text string, offset int) string {
	lineStart := strings.LastIndex(text[:offset], "\n") + 1
	line := text[lineStart:offset]
	inSQ, inDQ := false, false
	i := 0
	for i < len(line) {
		c := line[i]
		if inDQ {
			if c == '\\' {
				i += 2
				continue
			}
			if c == '"' {
				inDQ = false
			}
			i++
			continue
		}
		if inSQ {
			if c == '\'' && i+2 <= len(line) && line[i:i+2] == "''" {
				i += 2
				continue
			}
			if c == '\'' {
				inSQ = false
			}
			i++
			continue
		}
		if c == '"' {
			inDQ = true
			i++
			continue
		}
		if c == '\'' {
			inSQ = true
			i++
			continue
		}
		if c == '#' && (i == 0 || line[i-1] == ' ' || line[i-1] == '\t') {
			return "yaml-comment"
		}
		i++
	}
	if inDQ || inSQ {
		return "yaml-string"
	}
	return "yaml-code"
}

// shellRole does a whole-text forward scan: single quotes (no
// escapes inside), double quotes (backslash-escaped), backtick
// command substitution, and # comments (only where # starts a token,
// not mid-word like a literal '#' in an argument). KNOWN LIMITATION:
// heredocs (<<EOF ... EOF) are NOT modeled -- their body
// misclassifies as shell-code. Flag heredoc-containing files for
// manual review rather than trust this classifier there.
func shellRole(text string, offset int) string {
	i := 0
	state := "" // "" | "sq" | "dq" | "bq"
	for i < offset {
		c := text[i]
		switch state {
		case "sq":
			if c == '\'' {
				state = ""
			}
			i++
			continue
		case "dq":
			if c == '\\' {
				i += 2
				continue
			}
			if c == '"' {
				state = ""
			}
			i++
			continue
		case "bq":
			if c == '\\' {
				i += 2
				continue
			}
			if c == '`' {
				state = ""
			}
			i++
			continue
		}
		if c == '\'' {
			state = "sq"
			i++
			continue
		}
		if c == '"' {
			state = "dq"
			i++
			continue
		}
		if c == '`' {
			state = "bq"
			i++
			continue
		}
		if c == '#' && (i == 0 || strings.ContainsRune(" \t\n;|&(", rune(text[i-1]))) {
			nl := strings.Index(text[i:], "\n")
			if nl == -1 || offset <= i+nl {
				return "shell-comment"
			}
			i = i + nl + 1
			continue
		}
		i++
	}
	switch state {
	case "sq":
		return "shell-squote-string"
	case "dq":
		return "shell-dquote-string"
	case "bq":
		return "shell-backtick-string"
	default:
		return "shell-code"
	}
}

// Classify returns the syntactic role of the occurrence starting at
// byte offset in text, based on path's extension.
func Classify(path string, text string, offset int) string {
	suffix := strings.ToLower(filepath.Ext(path))
	switch suffix {
	case ".go":
		return goRole(text, offset)
	case ".md", ".markdown":
		return mdRole(text, offset)
	case ".py":
		return pythonRole(text, offset)
	case ".json":
		return jsonRole(text, offset)
	case ".yml", ".yaml":
		return yamlRole(text, offset)
	case ".sh", ".bash":
		return shellRole(text, offset)
	case ".js", ".mjs", ".cjs", ".jsx":
		return jsRole(text, offset)
	case ".ts", ".tsx":
		return tsRole(text, offset)
	case ".css":
		return cssRole(text, offset)
	case ".html", ".htm":
		return htmlRole(text, offset)
	default:
		return "text"
	}
}

// Occurrence is one match of a term in a file, with its syntactic
// role and line context.
type Occurrence struct {
	Path   string
	Start  int
	End    int
	Role   string
	LineNo int
	Line   string
}

// Occurrences returns every occurrence of term in the given files
// (regex-compiled if regex is true, otherwise matched literally).
// Files that don't exist, can't be read, or aren't valid UTF-8 are
// silently skipped, matching roles.py's own UnicodeDecodeError/OSError
// handling.
func Occurrences(term string, paths []string, regex bool) []Occurrence {
	pattern := term
	if !regex {
		pattern = regexp.QuoteMeta(term)
	}
	pat, err := regexp.Compile(pattern)
	if err != nil {
		return nil
	}

	var out []Occurrence
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		if !utf8.Valid(b) {
			continue
		}
		text := string(b)
		for _, m := range pat.FindAllStringIndex(text, -1) {
			start, end := m[0], m[1]
			lineNo := strings.Count(text[:start], "\n") + 1
			ls := strings.LastIndex(text[:start], "\n") + 1
			le := strings.Index(text[start:], "\n")
			var line string
			if le == -1 {
				line = text[ls:]
			} else {
				line = text[ls : start+le]
			}
			out = append(out, Occurrence{
				Path:   p,
				Start:  start,
				End:    end,
				Role:   Classify(p, text, start),
				LineNo: lineNo,
				Line:   line,
			})
		}
	}
	return out
}

// Expand resolves a list of paths into a flat, sorted list of files:
// directories are walked recursively (skipping any path with a
// ".git" component), plain files are included as-is, and anything
// that's neither is silently dropped.
func Expand(paths []string) []string {
	var out []string
	for _, p := range paths {
		fi, err := os.Stat(p)
		if err != nil {
			continue
		}
		if fi.IsDir() {
			var files []string
			_ = filepath.WalkDir(p, func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					return nil
				}
				if d.IsDir() {
					return nil
				}
				for _, part := range strings.Split(filepath.ToSlash(path), "/") {
					if part == ".git" {
						return nil
					}
				}
				files = append(files, path)
				return nil
			})
			sort.Strings(files)
			out = append(out, files...)
		} else {
			out = append(out, p)
		}
	}
	return out
}

// Run implements the CLI auditor: `repoman roles <term> [path ...]`.
const rolesHelp = `usage: repoman roles [-h] TERM [path ...]

repoman roles -- finds every occurrence of TERM across the given
paths (default: the current directory) and classifies each one by
its syntactic role (go-code, md-prose, md-fence, string-literal,
comment, and more) so a mass substitution can be split into
per-role passes instead of run blindly. This is the tool that makes
"classify every occurrence by its syntactic role before a mass
substitution" (this project's own central rule) actually checkable
rather than a manual judgment call.

Usage:
    repoman roles TERM [path ...]

Examples:
    repoman roles "oldName"              # search the whole tree
    repoman roles "oldName" pkg/ cmd/    # search specific paths only

Each match prints as:
    path:line: [role] trimmed line content

Options:
  -h, --help   Show this help message and exit.

See https://ha1tch.github.io/gorepoman/docs/repoman-050-roles.html
for the full per-language reference and known limits.
`

func Run(argv []string) int {
	for _, a := range argv {
		if a == "-h" || a == "--help" {
			fmt.Print(rolesHelp)
			return 0
		}
	}
	if len(argv) < 1 {
		fmt.Println("Usage: repoman roles <term> [path ...]")
		return 1
	}
	term := argv[0]
	targets := argv[1:]
	if len(targets) == 0 {
		targets = []string{"."}
	}
	paths := Expand(targets)

	byRole := map[string]int{}
	total := 0
	for _, occ := range Occurrences(term, paths, false) {
		byRole[occ.Role]++
		total++
		lineTrim := strings.TrimSpace(occ.Line)
		if len(lineTrim) > 90 {
			lineTrim = lineTrim[:90]
		}
		fmt.Printf("%s:%d: [%s] %s\n", occ.Path, occ.LineNo, occ.Role, lineTrim)
	}

	if total > 0 {
		names := make([]string, 0, len(byRole))
		for r := range byRole {
			names = append(names, r)
		}
		sort.Strings(names)
		quoted := make([]string, len(names))
		for i, n := range names {
			quoted[i] = "'" + n + "'"
		}
		fmt.Printf("\nroles present: [%s] (%d occurrence(s))\n", strings.Join(quoted, ", "), total)
		if len(byRole) > 1 {
			fmt.Println("MULTIPLE ROLES: a single substitution pass is NOT safe; write one targeted pass per role.")
		}
	} else {
		fmt.Println("no occurrences")
	}
	return 0
}
