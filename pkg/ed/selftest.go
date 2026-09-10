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

	// 10. append writes to true EOF, no handle needed, and journals.
	beforeAppend := readFile()
	so, se, code = run("ed", "append", "t.md", "--with", "ZAPPENDED\n")
	if !(code == 0 && strings.HasSuffix(readFile(), "ZAPPENDED\n")) {
		return fail("10 append writes to true EOF and journals", so+se+readFile())
	}
	if readFile() != beforeAppend+"ZAPPENDED\n" {
		return fail("10 append writes to true EOF and journals", "existing content was disturbed: "+readFile())
	}

	// 11. undo restores the append exactly, same journal/undo path as apply.
	so, se, code = run("ed", "undo")
	if !(code == 0 && readFile() == beforeAppend) {
		return fail("11 undo restores an append exactly", so+se+readFile())
	}

	// 12. prepend writes to true offset 0, no handle needed, and journals.
	beforePrepend := readFile()
	so, se, code = run("ed", "prepend", "t.md", "--with", "ZPREPENDED\n")
	if !(code == 0 && strings.HasPrefix(readFile(), "ZPREPENDED\n")) {
		return fail("12 prepend writes to true offset 0 and journals", so+se+readFile())
	}
	if readFile() != "ZPREPENDED\n"+beforePrepend {
		return fail("12 prepend writes to true offset 0 and journals", "existing content was disturbed: "+readFile())
	}

	// 13. undo restores the prepend exactly.
	so, se, code = run("ed", "undo")
	if !(code == 0 && readFile() == beforePrepend) {
		return fail("13 undo restores a prepend exactly", so+se+readFile())
	}

	// 14. append/prepend refuse a missing file rather than creating one.
	so, se, code = run("ed", "append", "does-not-exist.md", "--with", "x")
	if !(code == 1 && strings.Contains(se, "REFUSED")) {
		return fail("14 append refuses a missing file rather than creating one", so+se)
	}
	if _, statErr := os.Stat(filepath.Join(dir, "does-not-exist.md")); statErr == nil {
		return fail("14 append refuses a missing file rather than creating one", "file was created anyway")
	}

	// 15. insert --after requires a fresh handle and leaves the matched span untouched.
	// (searches for "gamma", not "beta": test 4's forced sub already replaced every
	// "beta" with "gamma" earlier in this same synthetic file, and nothing before
	// this point restores that substitution -- only test 5's undo runs, and that
	// undoes test 4's sub itself, restoring "alpha"/"omega", not "beta".)
	so, _, _ = run("ed", "find", "gamma", "t.md")
	iLines := strings.Split(strings.TrimRight(so, "\n"), "\n")
	if len(iLines) == 0 {
		return fail("15 insert --after requires a fresh handle", so)
	}
	iHandleFields := strings.Fields(iLines[0])
	if len(iHandleFields) == 0 {
		return fail("15 insert --after requires a fresh handle", so)
	}
	iHandle := iHandleFields[0]
	beforeInsertAfter := readFile()
	so, se, code = run("ed", "insert", iHandle, "--after", "--with", "-INSERTED-AFTER-")
	if !(code == 0 && strings.Contains(readFile(), "gamma-INSERTED-AFTER-")) {
		return fail("15 insert --after lands immediately after the matched span, matched text untouched", so+se+readFile())
	}

	// 16. undo restores an insert --after exactly.
	so, se, code = run("ed", "undo")
	if !(code == 0 && readFile() == beforeInsertAfter) {
		return fail("16 undo restores an insert --after exactly", so+se+readFile())
	}

	// 17. insert --before lands immediately before the matched span, matched text untouched.
	so, _, _ = run("ed", "find", "gamma", "t.md")
	iLines = strings.Split(strings.TrimRight(so, "\n"), "\n")
	iHandleFields = strings.Fields(iLines[0])
	iHandle = iHandleFields[0]
	so, se, code = run("ed", "insert", iHandle, "--before", "--with", "-INSERTED-BEFORE-")
	if !(code == 0 && strings.Contains(readFile(), "-INSERTED-BEFORE-gamma")) {
		return fail("17 insert --before lands immediately before the matched span, matched text untouched", so+se+readFile())
	}
	so, se, code = run("ed", "undo")
	if code != 0 {
		return fail("17 undo after insert --before", so+se)
	}

	// 18. insert refuses a stale handle, same as apply.
	so, _, _ = run("ed", "find", "gamma", "t.md")
	iLines = strings.Split(strings.TrimRight(so, "\n"), "\n")
	iHandleFields = strings.Fields(iLines[0])
	staleHandle := iHandleFields[0]
	run("ed", "insert", staleHandle, "--after", "--with", "-FIRST-")
	so, se, code = run("ed", "insert", staleHandle, "--after", "--with", "-SECOND-")
	if !(code == 1 && strings.Contains(se, "stale")) {
		return fail("18 insert refuses a stale handle", so+se)
	}
	run("ed", "undo")

	// 19. insert refuses when both --after and --before are given, or neither.
	so, _, _ = run("ed", "find", "gamma", "t.md")
	iLines = strings.Split(strings.TrimRight(so, "\n"), "\n")
	iHandleFields = strings.Fields(iLines[0])
	bothHandle := iHandleFields[0]
	so, se, code = run("ed", "insert", bothHandle, "--after", "--before", "--with", "-X-")
	if !(code == 1 && strings.Contains(se, "exactly one")) {
		return fail("19 insert refuses --after and --before together", so+se)
	}
	so, se, code = run("ed", "insert", bothHandle, "--with", "-X-")
	if !(code == 1 && strings.Contains(se, "exactly one")) {
		return fail("19 insert refuses neither --after nor --before", so+se)
	}
	if readFile() != beforeInsertAfter {
		return fail("19 insert refuses malformed --after/--before combos, writes nothing", readFile())
	}

	fmt.Println("selftest: all 19 paths green")
	return 0
}
