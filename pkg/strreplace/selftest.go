package strreplace

import (
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/ha1tch/gorepoman/pkg/doctor"
)

func b64(s string) string {
	return base64.StdEncoding.EncodeToString([]byte(s))
}

func writeFile(t *testHarness, path, content string) {
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.fatal(err)
	}
}

func readFile(t *testHarness, path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		t.fatal(err)
	}
	return string(b)
}

type testHarness struct {
	err error
}

func (t *testHarness) fatal(err error) {
	if t.err == nil {
		t.err = err
	}
}

func payloadStr(v *int, ops []opJSON) payloadJSON {
	return payloadJSON{V: v, Ops: ops}
}

func intPtr(n int) *int       { return &n }
func boolPtr(b bool) *bool    { return &b }
func strPtr(s string) *string { return &s }

// SelfTest is a direct port of cmd_selftest: 14 end-to-end
// verification cases covering count/role/delimiter/syntax refusals,
// dry-run diffs, marker generation/sweeping, normalization-mismatch
// detection, every format's own validator, journal integration with
// pkg/ed's own undo, atomic rollback across ops in the same file, the
// false-positive guard for a legitimately-inserted balanced literal,
// and the byte-exact no-shell-reinterpretation regression case that
// originally motivated this whole tool.
func SelfTest() int {
	dir, err := os.MkdirTemp("", "strreplace-selftest-*")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer os.RemoveAll(dir)

	origWD, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer os.Chdir(origWD)

	th := &testHarness{}
	type result struct {
		name string
		ok   bool
	}
	var results []result
	record := func(name string, ok bool) { results = append(results, result{name, ok}) }

	v1 := intPtr(1)

	// 1. Basic sub: count + role match, applies, journals. Not a
	//    syntax-verification test -- explicitly disabled so this stays
	//    true regardless of whether gofmt happens to be on PATH.
	writeFile(th, "t.go", "package main\n\nfunc f() {\n\tx := 1\n\t_ = x\n}\n")
	r := applyPayload(payloadStr(v1, []opJSON{{
		File: "t.go", SearchB64: strPtr(b64("x := 1")), ReplaceB64: strPtr(b64("x := 2")),
		Expect: intPtr(1), Roles: []string{"go-code"}, SyntaxCheck: boolPtr(false),
	}}))
	record("1 basic sub applies", r["ok"] == true && strings.Contains(readFile(th, "t.go"), "x := 2"))

	// 2. Count mismatch refuses, writes nothing.
	writeFile(th, "t2.go", "package main\n\nvar a = 1\nvar b = 1\n")
	before2 := readFile(th, "t2.go")
	r = applyPayload(payloadStr(v1, []opJSON{{
		File: "t2.go", SearchB64: strPtr(b64("= 1")), ReplaceB64: strPtr(b64("= 2")),
		Expect: intPtr(1), Roles: []string{"go-code"},
	}}))
	record("2 count-mismatch refuses", r["ok"] == false && errCls(r) == "count-mismatch" && readFile(th, "t2.go") == before2)

	// 3. Missing 'expect' or 'roles' refuses (no silent default).
	r = applyPayload(payloadStr(v1, []opJSON{{
		File: "t.go", SearchB64: strPtr(b64("_ = x")), ReplaceB64: strPtr(b64("_ = y")),
	}}))
	record("3 missing expect/roles refuses", r["ok"] == false)

	// 4. Role mismatch: search spans code + comment -> refuses with real census.
	writeFile(th, "t3.go", "package main\n\n// alpha note\nvar alpha = 1\n")
	r = applyPayload(payloadStr(v1, []opJSON{{
		File: "t3.go", SearchB64: strPtr(b64("alpha")), ReplaceB64: strPtr(b64("beta")),
		Expect: intPtr(2), Roles: []string{"go-code"},
	}}))
	record("4 role-mismatch refuses w/ census",
		r["ok"] == false && errCls(r) == "role-mismatch" && strings.Contains(errMsg(r), "go-comment"))

	// 5. Delimiter-break: replacement injects an unescaped closing
	//    quote into a Go double-quoted string -> refuses.
	writeFile(th, "t4.go", "package main\n\nvar s = \"hello world\"\nvar n = 1\n")
	before4 := readFile(th, "t4.go")
	r = applyPayload(payloadStr(v1, []opJSON{{
		File: "t4.go", SearchB64: strPtr(b64("hello")), ReplaceB64: strPtr(b64(`hello" + injected + "`)),
		Expect: intPtr(1), Roles: []string{"go-dquote-string"},
	}}))
	record("5 delimiter-break refuses, writes nothing",
		r["ok"] == false && errCls(r) == "delimiter-break" && readFile(th, "t4.go") == before4)

	// 6. Syntax-check-failed: a structurally valid single op that
	//    still leaves the file syntactically broken -> rolled back.
	writeFile(th, "t5.go", "package main\n\nfunc f() {\n\treturn 1\n}\n")
	before5 := readFile(th, "t5.go")
	r = applyPayload(payloadStr(v1, []opJSON{{
		File: "t5.go", SearchB64: strPtr(b64("return 1\n}")), ReplaceB64: strPtr(b64("return 1")),
		Expect: intPtr(1), Roles: []string{"go-code"},
	}}))
	if doctor.WhichGofmt() != "" {
		record("6 syntax-check-failed rolls back",
			r["ok"] == false && errCls(r) == "syntax-check-failed" && readFile(th, "t5.go") == before5)
	} else {
		record("6 syntax-check (skipped, no gofmt on PATH)", true)
	}

	// 7. dry_run changes nothing on disk but returns a diff.
	writeFile(th, "t6.go", "package main\n\nvar q = 1\n")
	before6 := readFile(th, "t6.go")
	r = applyPayload(payloadStr(v1, []opJSON{{
		File: "t6.go", SearchB64: strPtr(b64("q = 1")), ReplaceB64: strPtr(b64("q = 2")),
		Expect: intPtr(1), Roles: []string{"go-code"}, DryRun: boolPtr(true),
	}}))
	diffOK := false
	if diffs, ok := r["dry_run_diffs"].(map[string]interface{}); ok {
		if d, ok := diffs["t6.go"].(string); ok {
			diffOK = strings.Contains(d, "q = 2")
		}
	}
	record("7 dry_run writes nothing, returns diff",
		r["ok"] == true && readFile(th, "t6.go") == before6 && diffOK)

	// 8. Marker generation: collision-checked, unique, sweeps cleanly.
	writeFile(th, "t7.md", "before\nafter\n")
	r = applyPayload(payloadStr(v1, []opJSON{{Op: strPtr("gen_markers"), File: "t7.md", Count: intPtr(2)}}))
	markers, _ := r["ops"].([]map[string]interface{})
	var m1, m2 string
	if len(markers) == 1 {
		if ms, ok := markers[0]["markers"].([]string); ok && len(ms) == 2 {
			m1, m2 = ms[0], ms[1]
		}
	}
	record("8a markers unique", m1 != "" && m1 != m2 && strings.HasPrefix(m1, MarkerOpen+"RM:"))
	r2 := applyPayload(payloadStr(v1, []opJSON{
		{File: "t7.md", SearchB64: strPtr(b64("before")),
			ReplaceB64: strPtr(b64("before" + m1 + "mid" + m2 + "after-marker")), Expect: intPtr(1), Roles: []string{"md-prose"}},
		{Op: strPtr("sweep_markers"), File: "t7.md"},
	}))
	record("8b sweep removes both markers", r2["ok"] == true && !strings.Contains(readFile(th, "t7.md"), MarkerOpen))

	// 9. Normalization mismatch: raw search finds 0, NFC/NFD would
	//    find the accented text -- refuses with a clear diagnostic,
	//    never silently auto-normalizes.
	combining := "e\u0301lan"  // e + combining acute (NFD-ish)
	precomposed := "\u00e9lan" // single precomposed é (NFC)
	writeFile(th, "t8.md", "word: "+precomposed+"\n")
	r = applyPayload(payloadStr(v1, []opJSON{{
		File: "t8.md", SearchB64: strPtr(b64(combining)), ReplaceB64: strPtr(b64("elan")),
		Expect: intPtr(1), Roles: []string{"md-prose"},
	}}))
	record("9 normalization-mismatch detected, not silently normalized",
		r["ok"] == false && errCls(r) == "normalization-mismatch")

	// 10. New format classifiers wired: python/json/yaml/shell all
	//     produce a clean single-role sub with a passing syntax check.
	writeFile(th, "t.py", "x = 1\ny = 2\n")
	r = applyPayload(payloadStr(v1, []opJSON{{
		File: "t.py", SearchB64: strPtr(b64("x = 1")), ReplaceB64: strPtr(b64("x = 3")),
		Expect: intPtr(1), Roles: []string{"python-code"},
	}}))
	record("10a python sub + ast validate", r["ok"] == true && syntaxValidated(r, "t.py") == true)

	writeFile(th, "t.json", `{"a": 1, "b": 2}`+"\n")
	r = applyPayload(payloadStr(v1, []opJSON{{
		File: "t.json", SearchB64: strPtr(b64(`"a": 1`)), ReplaceB64: strPtr(b64(`"a": 3`)),
		Expect: intPtr(1), Roles: []string{"json-code"},
	}}))
	record("10b json sub + validate", r["ok"] == true && syntaxValidated(r, "t.json") == true)

	writeFile(th, "t.yaml", "a: 1\nb: 2\n")
	r = applyPayload(payloadStr(v1, []opJSON{{
		File: "t.yaml", SearchB64: strPtr(b64("a: 1")), ReplaceB64: strPtr(b64("a: 3")),
		Expect: intPtr(1), Roles: []string{"yaml-code"},
	}}))
	sv := syntaxValidated(r, "t.yaml")
	record("10c yaml sub + validate", r["ok"] == true && (sv == true || sv == "heuristic"))

	writeFile(th, "t.sh", "#!/bin/bash\necho \"a\"\necho \"b\"\n")
	r = applyPayload(payloadStr(v1, []opJSON{{
		File: "t.sh", SearchB64: strPtr(b64(`echo "a"`)), ReplaceB64: strPtr(b64(`echo "c"`)),
		Expect: intPtr(1), Roles: []string{"shell-code"},
	}}))
	if _, err := exec.LookPath("bash"); err == nil {
		record("10d shell sub + bash -n validate", r["ok"] == true && syntaxValidated(r, "t.sh") == true)
	} else {
		record("10d shell sub (skipped, no bash on PATH)", true)
	}

	writeFile(th, "t.js", "function f(x) {\n  return x + 1;\n}\n")
	r = applyPayload(payloadStr(v1, []opJSON{{
		File: "t.js", SearchB64: strPtr(b64("return x + 1")), ReplaceB64: strPtr(b64("return x + 2")),
		Expect: intPtr(1), Roles: []string{"js-code"},
	}}))
	if _, err := exec.LookPath("node"); err == nil {
		record("10e js sub + node --check validate", r["ok"] == true && syntaxValidated(r, "t.js") == true)
	} else {
		record("10e js sub + heuristic validate (no node on PATH)", r["ok"] == true && syntaxValidated(r, "t.js") == "heuristic")
	}

	writeFile(th, "t.ts", "interface Foo { a: string; }\nfunction f(x: Foo): string {\n  return x.a;\n}\n")
	r = applyPayload(payloadStr(v1, []opJSON{{
		File: "t.ts", SearchB64: strPtr(b64("return x.a")), ReplaceB64: strPtr(b64("return x.a + x.a")),
		Expect: intPtr(1), Roles: []string{"ts-code"},
	}}))
	record("10f ts sub + heuristic validate (always heuristic)", r["ok"] == true && syntaxValidated(r, "t.ts") == "heuristic")

	writeFile(th, "t.css", ".a {\n  color: red;\n}\n")
	r = applyPayload(payloadStr(v1, []opJSON{{
		File: "t.css", SearchB64: strPtr(b64("color: red")), ReplaceB64: strPtr(b64("color: blue")),
		Expect: intPtr(1), Roles: []string{"css-code"},
	}}))
	record("10g css sub + heuristic validate", r["ok"] == true && syntaxValidated(r, "t.css") == "heuristic")

	writeFile(th, "t.html", "<div class=\"a\">\n  <p>hello</p>\n</div>\n")
	r = applyPayload(payloadStr(v1, []opJSON{{
		File: "t.html", SearchB64: strPtr(b64("hello")), ReplaceB64: strPtr(b64("goodbye")),
		Expect: intPtr(1), Roles: []string{"html-text"},
	}}))
	record("10h html sub + heuristic validate", r["ok"] == true && syntaxValidated(r, "t.html") == "heuristic")

	// 10i. The hard case, proven end to end: a substitution INSIDE a
	// nested template literal's own ${...} substitution must be
	// correctly role-classified as js-code (not js-template-string).
	writeFile(th, "t2.js", "const x = `outer ${`inner ${1 + 1}`} tail`;\n")
	r = applyPayload(payloadStr(v1, []opJSON{{
		File: "t2.js", SearchB64: strPtr(b64("1 + 1")), ReplaceB64: strPtr(b64("2 + 2")),
		Expect: intPtr(1), Roles: []string{"js-code"},
	}}))
	record("10i nested template-literal substitution correctly role-classified",
		r["ok"] == true && strings.Contains(readFile(th, "t2.js"), "2 + 2"))
	rWrong := applyPayload(payloadStr(v1, []opJSON{{
		File: "t2.js", SearchB64: strPtr(b64("tail")), ReplaceB64: strPtr(b64("end")),
		Expect: intPtr(1), Roles: []string{"js-code"}, DryRun: boolPtr(true),
	}}))
	record("10j outer template text correctly NOT js-code (role-mismatch refuses)",
		rWrong["ok"] == false && errCls(rWrong) == "role-mismatch")

	// 11. Journal integration: an applied op is undoable via `repoman ed`.
	writeFile(th, "t9.go", "package main\n\nvar k = 1\n")
	applyPayload(payloadStr(v1, []opJSON{{
		File: "t9.go", SearchB64: strPtr(b64("k = 1")), ReplaceB64: strPtr(b64("k = 9")),
		Expect: intPtr(1), Roles: []string{"go-code"},
	}}))
	undoOK := runEdUndo()
	record("11 ed undo reverts a str_replace_extended edit", undoOK && strings.Contains(readFile(th, "t9.go"), "k = 1"))

	// 12. atomic batch: a later op's refusal rolls back an earlier
	//     op's already-staged change in the SAME file. This is
	//     specifically about a REAL, gofmt-detected break -- not just
	//     any refusal -- so the assertion checks the actual error
	//     class, not merely ok==false. Without that, this looked like
	//     it passed even when gofmt was genuinely absent, for an
	//     unrelated reason (syntax-unverifiable, not
	//     syntax-check-failed) -- a real, confirmed false-confidence
	//     case: an operator reading "OK" here had no way to know their
	//     environment couldn't actually exercise what this claims to
	//     test. Same conditional pattern as check 6, for the same
	//     reason.
	writeFile(th, "t10.go", "package main\n\nvar m = 1\nvar n = 1\n")
	before10 := readFile(th, "t10.go")
	r = applyPayload(payloadJSON{V: v1, Atomic: boolPtr(true), Ops: []opJSON{
		{File: "t10.go", SearchB64: strPtr(b64("m = 1")), ReplaceB64: strPtr(b64("m = 2")),
			Expect: intPtr(1), Roles: []string{"go-code"}},
		{File: "t10.go", SearchB64: strPtr(b64("n = 1")), ReplaceB64: strPtr(b64("n = 1\n}")),
			Expect: intPtr(1), Roles: []string{"go-code"}}, // breaks Go syntax
	}})
	if doctor.WhichGofmt() != "" {
		record("12 atomic batch rolls back on later syntax failure",
			r["ok"] == false && errCls(r) == "syntax-check-failed" && readFile(th, "t10.go") == before10)
	} else {
		record("12 atomic batch rollback (skipped, no gofmt on PATH)", true)
	}

	// 13. False-positive guard: inserting a COMPLETE, self-balanced
	//     string literal into code is legitimate and must NOT be
	//     refused.
	//     Not a syntax-verification test -- explicitly disabled so this
	//     stays true regardless of whether gofmt happens to be on PATH.
	writeFile(th, "t11.go", "package main\n\nfunc f() {\n\t_ = 1\n}\n")
	r = applyPayload(payloadStr(v1, []opJSON{{
		File: "t11.go", SearchB64: strPtr(b64("_ = 1")),
		ReplaceB64: strPtr(b64(`_ = "a new literal" + "and another"`)),
		Expect:     intPtr(1), Roles: []string{"go-code"}, SyntaxCheck: boolPtr(false),
	}}))
	record("13 legitimate nested literal insertion NOT refused",
		r["ok"] == true && strings.Contains(readFile(th, "t11.go"), `"a new literal"`))

	// 14. Direct regression test for the incident that motivated this
	//     whole tool: a replacement containing real tab/newline bytes
	//     and an apostrophe must land byte-exact, never reinterpreted
	//     as escape sequences.
	writeFile(th, "t12.go", "package main\n\nvar x = 1\n")
	tricky := "// XOT192's own re-check, Aggregate's own comment\nvar y = 2\n\tvar z = 3"
	r = applyPayload(payloadStr(v1, []opJSON{{
		File: "t12.go", SearchB64: strPtr(b64("var x = 1")), ReplaceB64: strPtr(b64(tricky)),
		Expect: intPtr(1), Roles: []string{"go-code"}, SyntaxCheck: boolPtr(false),
	}}))
	onDisk := readFile(th, "t12.go")
	record("14 tabs/newlines/apostrophes land byte-exact, no shell reinterpretation",
		r["ok"] == true && strings.Contains(onDisk, tricky) && !strings.Contains(onDisk, `\n`) && !strings.Contains(onDisk, `\t`))

	// 15. Atomic + unverifiable != atomic + verified-fine. When syntax
	//     cannot be independently checked at all (gofmt genuinely
	//     unavailable -- forced here via a test-only override, since
	//     WhichGofmt's own fallback paths do a real filesystem stat on
	//     fixed locations that no PATH/GOROOT manipulation can hide if
	//     gofmt actually happens to be installed there on this host),
	//     atomic must refuse rather than silently let a syntax-breaking
	//     edit through. This is the regression test for a real,
	//     confirmed incident: a fresh, toolchain-free bootstrap (no Go
	//     installed, only the gorepoman binary itself, which needs no
	//     toolchain to run) hit exactly this gap in the field.
	writeFile(th, "t13.go", "package main\n\nvar m = 1\nvar n = 1\n")
	before13 := readFile(th, "t13.go")
	os.Setenv("REPOMAN_TEST_FORCE_NO_GOFMT", "1")
	r = applyPayload(payloadJSON{V: v1, Atomic: boolPtr(true), Ops: []opJSON{
		{File: "t13.go", SearchB64: strPtr(b64("m = 1")), ReplaceB64: strPtr(b64("m = 2")),
			Expect: intPtr(1), Roles: []string{"go-code"}},
		{File: "t13.go", SearchB64: strPtr(b64("n = 1")), ReplaceB64: strPtr(b64("n = 1\n}")),
			Expect: intPtr(1), Roles: []string{"go-code"}}, // breaks Go syntax
	}})
	os.Unsetenv("REPOMAN_TEST_FORCE_NO_GOFMT")
	record("15 atomic refuses (not silently writes) when syntax cannot be verified at all",
		r["ok"] == false && errCls(r) == "syntax-unverifiable" && readFile(th, "t13.go") == before13)

	fmt.Println("str_replace_extended selftest:")
	allOK := true
	for _, res := range results {
		mark := "OK "
		if !res.ok {
			mark = "FAIL"
		}
		fmt.Printf("  %s  %s\n", mark, res.name)
		allOK = allOK && res.ok
	}
	if allOK {
		fmt.Printf("ALL GREEN (%d paths)\n", len(results))
		return 0
	}
	fmt.Printf("FAILURES ABOVE (%d paths)\n", len(results))
	return 1
}

func errCls(r map[string]interface{}) string {
	if e, ok := r["error"].(map[string]interface{}); ok {
		if s, ok := e["cls"].(string); ok {
			return s
		}
	}
	return ""
}

func errMsg(r map[string]interface{}) string {
	if e, ok := r["error"].(map[string]interface{}); ok {
		if s, ok := e["message"].(string); ok {
			return s
		}
	}
	return ""
}

func syntaxValidated(r map[string]interface{}, file string) interface{} {
	sr, ok := r["syntax_results"].(map[string]interface{})
	if !ok {
		return nil
	}
	entry, ok := sr[file].(map[string]interface{})
	if !ok {
		return nil
	}
	return entry["validated"]
}

// runEdUndo shells to this same binary's own `ed undo` -- mirroring
// the Python original's own subprocess call to ed.py, kept as a real
// subprocess call here too (rather than calling ed.Run in-process)
// so the test genuinely exercises the same cross-process journal file
// handoff a real two-invocation session relies on.
func runEdUndo() bool {
	self, err := os.Executable()
	if err != nil {
		return false
	}
	// A selftest run happens inside `repoman strreplace selftest`,
	// itself invoked from the `repoman` binary -- re-invoke that same
	// binary with `ed undo` rather than assuming a standalone ed
	// binary exists on PATH.
	cmd := exec.Command(self, "ed", "undo")
	cmd.Dir = mustGetwd()
	out, err := cmd.CombinedOutput()
	_ = out
	return err == nil
}

func mustGetwd() string {
	wd, _ := os.Getwd()
	return wd
}
