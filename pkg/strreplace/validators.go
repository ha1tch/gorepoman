package strreplace

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/ha1tch/repoman/pkg/doctor"
	"github.com/ha1tch/repoman/pkg/roles"
)

// SyntaxResult is the JSON-facing shape of one file's syntax check.
// Validated is one of: true (real check passed), false (real or
// heuristic check failed), "heuristic" (a reduced-confidence check
// passed), or nil (no validator registered, or the external tool a
// real check needs is absent and no heuristic exists for this
// format).
type SyntaxResult struct {
	Validated interface{} `json:"validated"`
	Detail    string      `json:"detail"`
}

// validatorFunc mirrors str_replace_extended.py's own per-format
// validator signature: returns (validated, detail) where validated is
// bool, "heuristic", or nil.
type validatorFunc func(path, text string) (interface{}, string)

func runExternal(bin string, args []string, suffix, text string, timeoutErr string) (bool, string, error) {
	tf, err := os.CreateTemp("", "strreplace-*"+suffix)
	if err != nil {
		return false, "", err
	}
	tmp := tf.Name()
	defer os.Remove(tmp)
	if _, err := tf.WriteString(text); err != nil {
		tf.Close()
		return false, "", err
	}
	tf.Close()

	full := append(append([]string{}, args...), tmp)
	cmd := exec.Command(bin, full...)
	out, runErr := cmd.CombinedOutput()
	if runErr != nil {
		if _, ok := runErr.(*exec.ExitError); ok {
			return false, strings.TrimSpace(string(out)), nil
		}
		return false, "", runErr
	}
	return true, "", nil
}

// validateGo shells out to gofmt -e (the same lookup doctor.WhichGofmt
// documents and reports on), reusing it directly so this validator can
// never drift from what doctor's own environment report describes.
func validateGo(path, text string) (interface{}, string) {
	gofmt := doctor.WhichGofmt()
	if gofmt == "" {
		return nil, "gofmt not found on PATH -- syntax not independently verified"
	}
	ok, detail, err := runExternal(gofmt, []string{"-e"}, ".go", text, "")
	if err != nil {
		return nil, err.Error()
	}
	if !ok {
		if detail == "" {
			detail = "gofmt reported a parse error"
		}
		return false, detail
	}
	return true, ""
}

// validatePython shells to a python3 interpreter's own `ast.parse`,
// the closest Go equivalent to gofmt -e/bash -n: a real parse by the
// language's own reference implementation, not a heuristic. Absent
// python3, no independent check is possible (Go has no Python parser
// of its own) -- reported as "not independently verified", the same
// as gofmt or bash being absent.
func validatePython(path, text string) (interface{}, string) {
	python3, err := exec.LookPath("python3")
	if err != nil {
		return nil, "python3 not found on PATH -- syntax not independently verified"
	}
	cmd := exec.Command(python3, "-c", "import ast,sys; ast.parse(sys.stdin.read())")
	cmd.Stdin = strings.NewReader(text)
	out, runErr := cmd.CombinedOutput()
	if runErr != nil {
		if _, ok := runErr.(*exec.ExitError); ok {
			detail := strings.TrimSpace(string(out))
			if detail == "" {
				detail = "python3 ast.parse reported a syntax error"
			}
			return false, detail
		}
		return nil, runErr.Error()
	}
	return true, ""
}

func validateJSON(path, text string) (interface{}, string) {
	var v interface{}
	if err := json.Unmarshal([]byte(text), &v); err != nil {
		return false, err.Error()
	}
	return true, ""
}

// validateYAML: this Go binary has no YAML library of its own (pure
// stdlib, matching this project's zero-third-party-dependency
// convention), so a real check means shelling to a Python interpreter
// with PyYAML installed -- the same external dependency
// doctor.Check's own PyYAML probe describes. Absent either python3 or
// its yaml module, falls back to the same structural heuristic the
// Python original uses when PyYAML itself is absent.
func validateYAML(path, text string) (interface{}, string) {
	python3, err := exec.LookPath("python3")
	if err != nil {
		return validateYAMLHeuristic(text)
	}
	cmd := exec.Command(python3, "-c", "import yaml,sys; yaml.safe_load(sys.stdin.read())")
	cmd.Stdin = strings.NewReader(text)
	out, runErr := cmd.CombinedOutput()
	if runErr == nil {
		return true, ""
	}
	outStr := string(out)
	if strings.Contains(outStr, "ModuleNotFoundError") || strings.Contains(outStr, "ImportError") {
		return validateYAMLHeuristic(text)
	}
	if _, ok := runErr.(*exec.ExitError); ok {
		detail := strings.TrimSpace(outStr)
		if detail == "" {
			detail = "yaml.safe_load reported a parse error"
		}
		return false, detail
	}
	return nil, runErr.Error()
}

// validateYAMLHeuristic: PyYAML (real or via subprocess) unavailable --
// reduced-confidence structural lint only. Result is reported as
// "heuristic", never conflated with a real parse.
func validateYAMLHeuristic(text string) (interface{}, string) {
	if strings.Contains(text, "\t") {
		return "heuristic", "contains literal tab character(s) -- invalid in YAML indentation"
	}
	for _, pair := range [][2]byte{{'{', '}'}, {'[', ']'}} {
		opener, closer := pair[0], pair[1]
		depth := 0
		inSQ, inDQ := false, false
		i := 0
		for i < len(text) {
			c := text[i]
			switch {
			case inDQ:
				if c == '\\' {
					i += 2
					continue
				}
				if c == '"' {
					inDQ = false
				}
			case inSQ:
				if c == '\'' {
					inSQ = false
				}
			case c == '"':
				inDQ = true
			case c == '\'':
				inSQ = true
			case c == opener:
				depth++
			case c == closer:
				depth--
				if depth < 0 {
					return false, "unbalanced '" + string(closer) + "' with no matching '" + string(opener) + "'"
				}
			}
			i++
		}
		if depth != 0 {
			return false, "unbalanced '" + string(opener) + "' -- unclosed"
		}
	}
	return "heuristic", ""
}

var mdFenceLineRe = regexp.MustCompile(`(?m)^` + "```")
var mdSepRowRe = regexp.MustCompile(`^\|?[\s:|-]+\|?$`)

func validateMarkdown(path, text string) (interface{}, string) {
	fences := len(mdFenceLineRe.FindAllString(text, -1))
	if fences%2 != 0 {
		return false, "unbalanced code fences"
	}
	inFence := false
	type widthInfo struct {
		width int
		line  int
	}
	var current *widthInfo
	for ln, line := range strings.Split(text, "\n") {
		lineNo := ln + 1
		if strings.HasPrefix(line, "```") {
			inFence = !inFence
			current = nil
			continue
		}
		if inFence {
			continue
		}
		stripped := strings.TrimSpace(line)
		if strings.HasPrefix(stripped, "|") {
			width := strings.Count(stripped, "|")
			switch {
			case current == nil:
				current = &widthInfo{width: width, line: lineNo}
			case mdSepRowRe.MatchString(stripped):
				// separator row -- width can legitimately differ in dash count
			case width != current.width:
				return false, "table column count changes at line " + strconv.Itoa(lineNo)
			}
		} else {
			current = nil
		}
	}
	return true, ""
}

func validateShell(path, text string) (interface{}, string) {
	bash, err := exec.LookPath("bash")
	if err != nil {
		return nil, "bash not found on PATH -- syntax not independently verified"
	}
	ok, detail, err := runExternal(bash, []string{"-n"}, ".sh", text, "")
	if err != nil {
		return nil, err.Error()
	}
	if !ok {
		if detail == "" {
			detail = "bash -n reported a syntax error"
		}
		return false, detail
	}
	return true, ""
}

// jsTsFrame mirrors _js_ts_balance_check's own state-transition
// shape (necessarily duplicated from pkg/roles's own JS/TS scanner,
// not imported -- this project's tools stay standalone for one
// shared scan, the same choice pkg/roles/pkg/waveprogress already
// made for their own shared regex fragments), but as ONE linear pass
// tracking bracket balance rather than pkg/roles's own
// per-offset-classify shape -- classifying once per character to
// validate a whole file would be O(n^2).
type jsTsFrame struct {
	kind  byte // 'l'=line comment, 'b'=block comment, 's'=string, 't'=template, 'u'=subst
	delim byte
	depth int
}

// jsTsBalanceCheck is a single-pass structural check: (){}[] balance,
// counted only in code state -- never inside a string, comment, or
// template literal -- plus confirms every string/comment/template
// opened is also closed by EOF. A substitution's own `${`/`}`
// delimiter pair is deliberately NOT pushed onto the general
// bracket-balance stack -- only braces GENUINELY nested inside the
// substitution's own code (object literals, block bodies) are; the
// substitution's own nesting is tracked by the state stack's own
// depth counter instead.
func jsTsBalanceCheck(text string) (bool, string) {
	var stack []jsTsFrame
	var balance []byte
	pairs := map[byte]byte{')': '(', '}': '{', ']': '['}
	i, n := 0, len(text)

	for i < n {
		c := text[i]
		var top *jsTsFrame
		if len(stack) > 0 {
			top = &stack[len(stack)-1]
		}
		kind := byte(0)
		if top != nil {
			kind = top.kind
		}

		if top == nil || kind == 'u' {
			if i+2 <= n && text[i:i+2] == "//" {
				stack = append(stack, jsTsFrame{kind: 'l'})
				i += 2
				continue
			}
			if i+2 <= n && text[i:i+2] == "/*" {
				stack = append(stack, jsTsFrame{kind: 'b'})
				i += 2
				continue
			}
			if c == '\'' || c == '"' {
				stack = append(stack, jsTsFrame{kind: 's', delim: c})
				i++
				continue
			}
			if c == '`' {
				stack = append(stack, jsTsFrame{kind: 't'})
				i++
				continue
			}
			if kind == 'u' {
				if c == '{' {
					top.depth++
					balance = append(balance, '{')
					i++
					continue
				}
				if c == '}' {
					if top.depth == 1 {
						stack = stack[:len(stack)-1]
					} else {
						if len(balance) == 0 || balance[len(balance)-1] != '{' {
							return false, "unmatched '}' in substitution"
						}
						balance = balance[:len(balance)-1]
						top.depth--
					}
					i++
					continue
				}
			}
			if c == '(' || c == '{' || c == '[' {
				balance = append(balance, c)
				i++
				continue
			}
			if c == ')' || c == '}' || c == ']' {
				if len(balance) == 0 || balance[len(balance)-1] != pairs[c] {
					return false, "unmatched '" + string(c) + "'"
				}
				balance = balance[:len(balance)-1]
				i++
				continue
			}
			i++
			continue
		}

		switch kind {
		case 'l':
			if c == '\n' {
				stack = stack[:len(stack)-1]
			}
			i++
		case 'b':
			if i+2 <= n && text[i:i+2] == "*/" {
				stack = stack[:len(stack)-1]
				i += 2
				continue
			}
			i++
		case 's':
			if c == '\\' {
				i += 2
				continue
			}
			if c == top.delim {
				stack = stack[:len(stack)-1]
			}
			i++
		case 't':
			if c == '\\' {
				i += 2
				continue
			}
			if c == '`' {
				stack = stack[:len(stack)-1]
				i++
				continue
			}
			if i+2 <= n && text[i:i+2] == "${" {
				stack = append(stack, jsTsFrame{kind: 'u', depth: 1})
				i += 2
				continue
			}
			i++
		}
	}

	if len(stack) > 0 {
		kind := stack[len(stack)-1].kind
		if kind != 'l' { // an unterminated line comment is closed by EOF, fine
			names := map[byte]string{'s': "an unterminated string", 't': "an unterminated template literal",
				'b': "an unterminated /* comment", 'u': "an unterminated ${...} substitution"}
			return false, "file ends inside " + names[kind]
		}
	}
	if len(balance) > 0 {
		return false, "unbalanced '" + string(balance[len(balance)-1]) + "' -- unclosed"
	}
	return true, ""
}

// validateJS: .js/.mjs/.cjs (NOT .jsx) tries `node --check` first when
// Node is on PATH -- a real parse, the closest equivalent here to
// gofmt -e/bash -n. Falls back to the role-aware balance heuristic
// when Node is absent. .jsx is deliberately excluded from the `node
// --check` path even when Node is present: plain Node cannot parse
// JSX at all, so a real .jsx file would be wrongly reported as a
// syntax error by the "real" check -- worse than not checking, since
// it reports false confidently.
func validateJS(path, text string) (interface{}, string) {
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".js" || ext == ".mjs" || ext == ".cjs" {
		if node, err := exec.LookPath("node"); err == nil {
			ok, detail, runErr := runExternal(node, []string{"--check"}, ".js", text, "")
			if runErr == nil {
				if !ok {
					if detail == "" {
						detail = "node --check reported a syntax error"
					}
					return false, detail
				}
				return true, ""
			}
		}
	}
	ok, detail := jsTsBalanceCheck(text)
	if ok {
		return "heuristic", detail
	}
	return false, detail
}

// validateTS: .ts/.tsx always heuristic. No universally-available
// tool parses TypeScript's own type-level syntax the way
// gofmt/bash/python parse their own formats -- plain `node --check`
// would wrongly reject valid TS syntax it cannot itself parse (the
// same JSX problem validateJS avoids, plus TS's own type syntax on
// top). Same role-aware balance heuristic as .jsx's own fallback path.
func validateTS(path, text string) (interface{}, string) {
	ok, detail := jsTsBalanceCheck(text)
	if ok {
		return "heuristic", detail
	}
	return false, detail
}

// validateCSS: always heuristic -- no standard CSS-aware parser is
// reliably available to shell out to. Checks brace balance (counted
// only outside strings/comments) and that no string/comment is left
// open at EOF -- structural sanity, not a real CSS grammar check.
func validateCSS(path, text string) (interface{}, string) {
	i, n := 0, len(text)
	state := "" // "" | "comment" | "sq" | "dq"
	balance := 0
	for i < n {
		c := text[i]
		switch state {
		case "":
			if i+2 <= n && text[i:i+2] == "/*" {
				state = "comment"
				i += 2
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
			if c == '{' {
				balance++
				i++
				continue
			}
			if c == '}' {
				balance--
				if balance < 0 {
					return false, "unmatched '}'"
				}
				i++
				continue
			}
			i++
		case "comment":
			if i+2 <= n && text[i:i+2] == "*/" {
				state = ""
				i += 2
				continue
			}
			i++
		default: // "sq" or "dq"
			delim := byte('\'')
			if state == "dq" {
				delim = '"'
			}
			if c == '\\' {
				i += 2
				continue
			}
			if c == delim {
				state = ""
			}
			i++
		}
	}
	if state == "comment" {
		return false, "file ends inside an unterminated /* comment"
	}
	if state == "sq" || state == "dq" {
		return false, "file ends inside an unterminated string"
	}
	if balance != 0 {
		if balance > 0 {
			return false, "unbalanced '{' -- unclosed"
		}
		return false, "extra, unmatched '}'"
	}
	return "heuristic", ""
}

var voidElements = map[string]bool{
	"area": true, "base": true, "br": true, "col": true, "embed": true, "hr": true, "img": true,
	"input": true, "link": true, "meta": true, "param": true, "source": true, "track": true, "wbr": true,
}
var tagRe = regexp.MustCompile(`<(/?)([a-zA-Z][a-zA-Z0-9-]*)\b([^>]*)>`)

// validateHTML: always heuristic, like CSS. Checks tag-open/tag-close
// balance (void elements and self-closing '/>' tags correctly
// excluded from the stack) -- NOT a real HTML5 parser, does not
// validate nesting rules a real parser auto-corrects, only that every
// tag needing a close has one and every closing tag matches what's
// actually open.
//
// <script>/<style> BODY spans are excluded from tag-matching entirely
// (reusing pkg/roles's own HTMLEmbeddedSpans) -- their content is not
// HTML, and a JS comparison like `if (a < b)` would otherwise
// false-positive match this validator's own tag regex as if `<b)`
// were the start of a tag. The opening/closing <script> and <style>
// TAGS themselves are still matched normally; only what is between
// them is skipped.
func validateHTML(path, text string) (interface{}, string) {
	embedded := roles.HTMLEmbeddedSpans(text)
	inEmbeddedBody := func(pos int) bool {
		for _, s := range embedded {
			if s.Start <= pos && pos < s.End {
				return true
			}
		}
		return false
	}

	var stack []string
	for _, m := range tagRe.FindAllStringSubmatchIndex(text, -1) {
		if inEmbeddedBody(m[0]) {
			continue
		}
		closing := text[m[2]:m[3]] == "/"
		name := strings.ToLower(text[m[4]:m[5]])
		attrs := strings.TrimRight(text[m[6]:m[7]], " \t\n\r")
		selfClosing := strings.HasSuffix(attrs, "/")
		if closing {
			if len(stack) == 0 {
				return false, "closing </" + name + "> with nothing open"
			}
			if stack[len(stack)-1] != name {
				return false, "</" + name + "> does not match open <" + stack[len(stack)-1] + ">"
			}
			stack = stack[:len(stack)-1]
		} else if voidElements[name] || selfClosing {
			// no-op
		} else {
			stack = append(stack, name)
		}
	}
	if len(stack) > 0 {
		rev := make([]string, len(stack))
		for i, s := range stack {
			rev[len(stack)-1-i] = s
		}
		return false, "unclosed tag(s): " + strings.Join(rev, ", ")
	}
	return "heuristic", ""
}

var validators = map[string]validatorFunc{
	".go":   validateGo,
	".py":   validatePython,
	".json": validateJSON,
	".yml":  validateYAML, ".yaml": validateYAML,
	".md": validateMarkdown, ".markdown": validateMarkdown,
	".sh": validateShell, ".bash": validateShell,
	".js": validateJS, ".mjs": validateJS, ".cjs": validateJS, ".jsx": validateJS,
	".ts": validateTS, ".tsx": validateTS,
	".css":  validateCSS,
	".html": validateHTML, ".htm": validateHTML,
}

// validateSyntax returns {"validated": True|False|"heuristic"|None,
// "detail": str}. nil/None means: no validator for this format,
// nothing checked, and the caller should not read that as a pass.
func validateSyntax(path, text string) SyntaxResult {
	ext := strings.ToLower(filepath.Ext(path))
	fn, ok := validators[ext]
	if !ok {
		return SyntaxResult{Validated: nil, Detail: "no syntax validator registered for " + extOrNoExt(ext)}
	}
	v, detail := fn(path, text)
	return SyntaxResult{Validated: v, Detail: detail}
}
