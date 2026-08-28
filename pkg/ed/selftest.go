package ed

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// SelfTest is a direct port of ed.py's cmd_selftest: nine end-to-end
// paths against a synthetic file in a temp dir, re-invoking this same
// compiled binary as a real subprocess the same way pkg/strreplace's
// own SelfTest and pkg/selftest's acceptance gate do -- exercising
// argument parsing, working-directory handling, and real exit codes,
// not just in-process function calls.
//
// Replaces a previous stub in this package's own Run (case
// "selftest") that printed a fixed "pass" without checking anything --
// found and fixed as a real gap during a feature-parity review against
// the Python original, not a design choice: ed.py's own selftest is a
// genuine nine-path suite, and the Go port had silently dropped it.
func SelfTest() int {
	self, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if resolved, err := filepath.EvalSymlinks(self); err == nil {
		self = resolved
	}

	dir, err := os.MkdirTemp("", "ed-selftest-*")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer os.RemoveAll(dir)

	const original = "alpha beta\nalpha in `code alpha`\n"
	fPath := filepath.Join(dir, "t.md")
	if err := os.WriteFile(fPath, []byte(original), 0644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	run := func(args ...string) (stdout, stderr string, code int) {
		cmd := exec.Command(self, args...)
		cmd.Dir = dir
		var so, se bytes.Buffer
		cmd.Stdout = &so
		cmd.Stderr = &se
		err := cmd.Run()
		c := 0
		if err != nil {
			if ee, ok := err.(*exec.ExitError); ok {
				c = ee.ExitCode()
			} else {
				c = 1
			}
		}
		return so.String(), se.String(), c
	}

	fail := func(label, detail string) int {
		fmt.Fprintf(os.Stderr, "FAIL: %s\n%s\n", label, detail)
		return 1
	}
	readFile := func() string {
		b, _ := os.ReadFile(fPath)
		return string(b)
	}

	// 1. find prints handles with roles.
	so, se, _ := run("ed", "find", "alpha", "t.md")
	if !strings.Contains(so, "3 occurrence(s)") {
		return fail("1 find prints handles with roles", so+se)
	}
	lines := strings.Split(strings.TrimRight(so, "\n"), "\n")
	if len(lines) == 0 {
		return fail("1 find prints handles with roles", "no output lines")
	}
	fields := strings.Fields(lines[0])
	if len(fields) == 0 {
		return fail("1 find prints handles with roles", "no handle in first line")
	}

	// 2. sub refuses on wrong count, writes nothing.
	so, se, code := run("ed", "sub", "alpha", "omega", "t.md", "--expect", "2")
	if !(code == 1 && strings.Contains(se, "REFUSED")) {
		return fail("2 sub refuses on wrong count", so+se)
	}
	if !strings.Contains(readFile(), "alpha") {
		return fail("2 sub refuses on wrong count, writes nothing", readFile())
	}

	// 3. sub refuses on mixed roles without --force-roles.
	so, se, code = run("ed", "sub", "alpha", "omega", "t.md", "--expect", "3")
	if !(code == 1 && strings.Contains(se, "roles")) {
		return fail("3 sub refuses on mixed roles without --force-roles", so+se)
	}

	// 4. forced sub succeeds and journals.
	so, se, code = run("ed", "sub", "alpha", "omega", "t.md", "--expect", "3", "--force-roles")
	if code != 0 {
		return fail("4 forced sub succeeds and journals", so+se)
	}
	if strings.Count(readFile(), "omega") != 3 {
		return fail("4 forced sub succeeds and journals", readFile())
	}

	// 5. undo restores exactly.
	so, se, code = run("ed", "undo")
	if code != 0 {
		return fail("5 undo restores exactly", so+se)
	}
	if readFile() != original {
		return fail("5 undo restores exactly", readFile())
	}

	// 6. apply with a fresh handle works...
	so, _, _ = run("ed", "find", "beta", "t.md")
	fLines := strings.Split(strings.TrimRight(so, "\n"), "\n")
	if len(fLines) == 0 {
		return fail("6 apply with a fresh handle works", so)
	}
	handleFields := strings.Fields(fLines[0])
	if len(handleFields) == 0 {
		return fail("6 apply with a fresh handle works", so)
	}
	handle := handleFields[0]
	so, se, code = run("ed", "apply", handle, "--with", "gamma")
	if !(code == 0 && strings.Contains(readFile(), "gamma")) {
		return fail("6 apply with a fresh handle works", so+se)
	}

	// 7. ...and a stale handle refuses.
	so, se, code = run("ed", "apply", handle, "--with", "delta")
	if !(code == 1 && strings.Contains(se, "stale")) {
		return fail("7 a stale handle refuses", so+se)
	}

	// 8. mark + undo --since; unknown mark refuses.
	run("ed", "mark", "here")
	so, se, code = run("ed", "undo", "--since", "nowhere")
	if !(code == 1 && strings.Contains(se, "REFUSED")) {
		return fail("8 undo --since an unknown mark refuses", so+se)
	}

	// 9. undo --since a real mark with no later txns is a no-op.
	so, se, code = run("ed", "undo", "--since", "here")
	if !strings.Contains(so, "nothing to undo") {
		return fail("9 undo --since a real mark with no later txns is a no-op", so+se)
	}

	fmt.Println("selftest: all 9 paths green")
	return 0
}
