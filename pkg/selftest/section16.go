package selftest

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func b64s(s string) string { return base64.StdEncoding.EncodeToString([]byte(s)) }

// runSection16 is adversarial coverage: deliberately malformed,
// boundary, and hostile-shaped inputs, checking that this project's
// own stated guarantees (refuse rather than guess; true atomicity;
// byte-exact matching, no silent Unicode normalization) actually hold
// under them rather than just under the well-formed inputs the rest
// of this suite exercises. Every case here was verified manually
// against the real binary before being written down as a permanent
// check -- a written-and-assumed adversarial test is worse than none,
// since it would look like coverage without being any.
func runSection16(g *gate, root string) int {
	self := g.self

	advDir := filepath.Join(root, "adversarial-fixture")
	os.MkdirAll(advDir, 0755)
	target := filepath.Join(advDir, "t.go")
	mustWrite(target, "package main\n")

	applyPayload := func(payload string) runResult {
		return runWithStdin(self, advDir, payload, "strreplace", "apply", "-")
	}

	// A negative expect count must refuse, not silently treat it as
	// "any count" or panic on an unexpected value.
	neg := fmt.Sprintf(`{"v":1,"ops":[{"op":"sub","file":"t.go","search_b64":%q,"replace_b64":%q,"expect":-1,"roles":["go-code"]}]}`,
		b64s("package"), b64s("package"))
	r := applyPayload(neg)
	if !g.check(r.code == 1 && strings.Contains(r.stdout+r.stderr, "count-mismatch"),
		"adversarial: a negative expect count refuses cleanly (count-mismatch), rather than "+
			"being treated as unbounded or crashing", r.stdout+r.stderr) {
		return 1
	}
	if !g.check(mustRead(target) == "package main\n",
		"adversarial: the file is untouched after the negative-expect refusal", mustRead(target)) {
		return 1
	}

	// Zero as an expect count, against text that actually appears
	// once, must also refuse -- "expect 0" is a real claim ("this
	// should NOT be present") and a real occurrence contradicts it.
	zero := fmt.Sprintf(`{"v":1,"ops":[{"op":"sub","file":"t.go","search_b64":%q,"replace_b64":%q,"expect":0,"roles":["go-code"]}]}`,
		b64s("package"), b64s("package"))
	r = applyPayload(zero)
	if !g.check(r.code == 1 && strings.Contains(r.stdout+r.stderr, "count-mismatch"),
		"adversarial: expect=0 against text that is actually present refuses, rather than "+
			"silently treating zero as a no-op success", r.stdout+r.stderr) {
		return 1
	}

	// An empty search string must not match "everywhere" or corrupt
	// the file -- it should simply find nothing, and a non-zero
	// expect against nothing found refuses cleanly.
	emptySearch := fmt.Sprintf(`{"v":1,"ops":[{"op":"sub","file":"t.go","search_b64":%q,"replace_b64":%q,"expect":1,"roles":["go-code"]}]}`,
		b64s(""), b64s("X"))
	r = applyPayload(emptySearch)
	if !g.check(r.code == 1 && strings.Contains(r.stdout+r.stderr, "0 occurrence"),
		"adversarial: an empty search string finds 0 occurrences and refuses against a "+
			"non-zero expect, rather than matching everywhere", r.stdout+r.stderr) {
		return 1
	}
	if !g.check(mustRead(target) == "package main\n",
		"adversarial: the file is untouched after the empty-search refusal", mustRead(target)) {
		return 1
	}

	// search == replace: a genuine no-op. Must succeed (the content
	// genuinely is what was asked for), but must not rewrite the file
	// on disk just to write identical bytes back -- confirmed by an
	// empty "written" list, not merely a successful exit code. Not a
	// syntax-verification test -- explicitly disabled so this stays
	// true regardless of whether gofmt happens to be on PATH (a real,
	// confirmed gap: this used to fail in a genuinely toolchain-free
	// environment for a reason entirely unrelated to what it claims to
	// test).
	noop := fmt.Sprintf(`{"v":1,"ops":[{"op":"sub","file":"t.go","search_b64":%q,"replace_b64":%q,"expect":1,"roles":["go-code"],"syntax_check":false}]}`,
		b64s("package main"), b64s("package main"))
	r = applyPayload(noop)
	if !g.check(r.code == 0 && strings.Contains(r.stdout, `"written": []`),
		"adversarial: search==replace succeeds as a genuine no-op, and the file is not "+
			"rewritten on disk for identical content", r.stdout+r.stderr) {
		return 1
	}

	// A zero-width joiner smuggled into the search text must not
	// match real content that lacks it -- byte-exact matching, no
	// silent Unicode normalization collapsing it away.
	zwjSearch := fmt.Sprintf(`{"v":1,"ops":[{"op":"sub","file":"t.go","search_b64":%q,"replace_b64":%q,"expect":1,"roles":["go-code"]}]}`,
		b64s("package\u200d main"), b64s("package main"))
	r = applyPayload(zwjSearch)
	if !g.check(r.code == 1 && strings.Contains(r.stdout+r.stderr, "0 occurrence"),
		"adversarial: a zero-width joiner smuggled into the search text does not match real "+
			"content lacking it -- byte-exact, no Unicode normalization", r.stdout+r.stderr) {
		return 1
	}

	// True atomicity at real scale: 150 ops, 149 individually valid,
	// the last deliberately broken. Every prior op being individually
	// correct must not matter -- nothing gets written, at all, and
	// the refusal correctly reports how far it got before stopping.
	scaleDir := filepath.Join(advDir, "scale")
	os.MkdirAll(scaleDir, 0755)
	var opsJSON strings.Builder
	opsJSON.WriteString(`{"v":1,"atomic":true,"defaults":{"syntax_check":false},"ops":[`)
	const n = 150
	for i := 0; i < n; i++ {
		fname := fmt.Sprintf("scale/file%d.go", i)
		mustWrite(filepath.Join(advDir, fname), fmt.Sprintf("package main\n\nvar x%d = %d\n", i, i))
		search := fmt.Sprintf("var x%d = %d", i, i)
		replace := fmt.Sprintf("var x%d = %d", i, i*1000)
		if i == n-1 {
			// deliberately break only the very last op
			search = "THIS TEXT DOES NOT EXIST ANYWHERE"
		}
		if i > 0 {
			opsJSON.WriteString(",")
		}
		fmt.Fprintf(&opsJSON, `{"op":"sub","file":%q,"search_b64":%q,"replace_b64":%q,"expect":1,"roles":["go-code"]}`,
			fname, b64s(search), b64s(replace))
	}
	opsJSON.WriteString("]}")
	r = applyPayload(opsJSON.String())
	allUnchanged := true
	for i := 0; i < n; i++ {
		expected := fmt.Sprintf("var x%d = %d\n", i, i)
		if !strings.Contains(mustRead(filepath.Join(advDir, fmt.Sprintf("scale/file%d.go", i))), expected) {
			allUnchanged = false
			break
		}
	}
	if !g.check(r.code == 1 && strings.Contains(r.stdout, `"ops_completed_before_refusal": 149`) &&
		allUnchanged, "adversarial: a 150-op atomic batch with only the last op broken writes "+
		"nothing at all -- true all-or-nothing, not just for small batches", r.stdout+r.stderr) {
		return 1
	}

	return runSection17(g, root)
}
