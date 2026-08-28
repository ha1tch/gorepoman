// Package gomod implements the go.mod/go.sum sanity gate — translated
// from repoman/gomod.py.
//
// Two checks, both aimed at the same failure mode: a go.mod or go.sum
// that only builds on the machine that produced it, because something
// environment-specific leaked into a file meant to be portable.
//
// replace-directive check (always enforced, fully offline, deterministic):
// uses `go mod edit -json` -- Go's own real parser -- to read every
// `replace` directive. A replace entry's New.Version being absent is
// Go's own unambiguous signal that the target is a local filesystem path
// rather than a module-registry redirect. An ABSOLUTE local-path target
// always fails the gate. A RELATIVE local-path target (./foo, ../foo) is
// a legitimate, if uncommon, monorepo pattern, so it warns rather than
// fails by default; --strict-relative-replace promotes that to a
// failure too.
//
// go.sum completeness check (best-effort; degrades to a warning, never a
// false failure, when the environment itself can't finish the check):
// uses `go list -deps -mod=readonly ./...` rather than `go build`, so it
// resolves the full import/module graph without compiling or linking
// anything.
//
// Exit codes: 0 (including when there are warnings but no errors), 1 (at
// least one error).
package gomod

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// runResult is (returncode, combined stdout+stderr). rc == nil means the
// command itself could not be started (e.g. `go` not on PATH) or timed
// out -- a different failure class from the command running and
// reporting its own non-zero exit.
type runResult struct {
	rc  *int
	out string
}

func run(cmd []string, cwd string) runResult {
	done := make(chan runResult, 1)
	c := exec.Command(cmd[0], cmd[1:]...)
	c.Dir = cwd
	var outBuf strings.Builder
	c.Stdout = &outBuf
	c.Stderr = &outBuf

	go func() {
		err := c.Run()
		if err != nil {
			if _, ok := err.(*exec.ExitError); !ok {
				// Could not start (e.g. not found on PATH).
				done <- runResult{rc: nil, out: fmt.Sprintf("%s: not found on PATH", cmd[0])}
				return
			}
		}
		code := c.ProcessState.ExitCode()
		done <- runResult{rc: &code, out: outBuf.String()}
	}()

	select {
	case res := <-done:
		return res
	case <-time.After(300 * time.Second):
		if c.Process != nil {
			c.Process.Kill()
		}
		return runResult{rc: nil, out: fmt.Sprintf("%s: timed out after 300s", strings.Join(cmd, " "))}
	}
}

var winAbsRe = regexp.MustCompile(`^[A-Za-z]:[\\/]|^\\\\`)

func isAbsolutePath(p string) bool {
	return strings.HasPrefix(p, "/") || winAbsRe.MatchString(p)
}

type modEditJSON struct {
	Replace []struct {
		Old struct {
			Path string `json:"Path"`
		} `json:"Old"`
		New struct {
			Path    string `json:"Path"`
			Version string `json:"Version"`
		} `json:"New"`
	} `json:"Replace"`
}

// checkReplaceDirectives returns (errors, warnings).
func checkReplaceDirectives(root string, strictRelative bool) ([]string, []string) {
	res := run([]string{"go", "mod", "edit", "-json"}, root)
	if res.rc == nil {
		return []string{fmt.Sprintf("go-tooling: %s", res.out)}, nil
	}
	if *res.rc != 0 {
		return []string{fmt.Sprintf("go-mod-parse: `go mod edit -json` failed: %s", strings.TrimSpace(res.out))}, nil
	}
	var data modEditJSON
	if err := json.Unmarshal([]byte(res.out), &data); err != nil {
		return []string{fmt.Sprintf("go-mod-parse: could not parse `go mod edit -json` output: %v", err)}, nil
	}

	var errors, warnings []string
	for _, r := range data.Replace {
		oldPath := r.Old.Path
		if oldPath == "" {
			oldPath = "?"
		}
		newPath := r.New.Path
		newVersion := r.New.Version
		if newVersion != "" {
			continue // a real module-registry redirect, not a local path
		}
		if isAbsolutePath(newPath) {
			errors = append(errors, fmt.Sprintf(
				"replace-absolute-path: `replace %s => %s` is an absolute local "+
					"filesystem path -- it will not exist on any machine but the "+
					"one that wrote it. Remove before release.", oldPath, newPath))
		} else if strings.HasPrefix(newPath, "./") || strings.HasPrefix(newPath, "../") {
			msg := fmt.Sprintf(
				"replace-relative-path: `replace %s => %s` is a local filesystem "+
					"path. Fine for a genuine monorepo layout if %s is meant to "+
					"ship alongside this repo; otherwise remove before release.",
				oldPath, newPath, newPath)
			if strictRelative {
				errors = append(errors, msg)
			} else {
				warnings = append(warnings, msg)
			}
		}
	}
	return errors, warnings
}

// checkGosumCompleteness returns (errors, warnings).
func checkGosumCompleteness(root string) ([]string, []string) {
	res := run([]string{"go", "list", "-deps", "-mod=readonly", "./..."}, root)
	if res.rc == nil {
		return nil, []string{fmt.Sprintf("go-tooling: %s -- go.sum completeness not checked", res.out)}
	}
	if strings.Contains(res.out, "missing go.sum entry") {
		var errs []string
		for _, ln := range strings.Split(res.out, "\n") {
			if strings.Contains(ln, "missing go.sum entry") {
				errs = append(errs, fmt.Sprintf("gosum-incomplete: %s", strings.TrimSpace(ln)))
			}
		}
		return errs, nil
	}
	if *res.rc != 0 {
		out := strings.TrimSpace(res.out)
		if len(out) > 300 {
			out = out[:300]
		}
		return nil, []string{fmt.Sprintf(
			"gosum-check-inconclusive: `go list -deps -mod=readonly ./...` failed "+
				"for a reason other than a missing go.sum entry, so completeness "+
				"could not be confirmed: %s", out)}
	}
	return nil, nil
}

func cmdCheck(root string, strictRelative bool) int {
	res := run([]string{"go", "version"}, root)
	if res.rc == nil {
		fmt.Printf("ERROR go-tooling: %s\n", res.out)
		fmt.Println("GOMOD CHECK FAIL: 1 error(s)")
		return 1
	}

	var errors, warnings []string
	e, w := checkReplaceDirectives(root, strictRelative)
	errors = append(errors, e...)
	warnings = append(warnings, w...)
	e, w = checkGosumCompleteness(root)
	errors = append(errors, e...)
	warnings = append(warnings, w...)

	for _, msg := range warnings {
		fmt.Printf("WARN %s\n", msg)
	}
	for _, msg := range errors {
		fmt.Printf("ERROR %s\n", msg)
	}
	if len(errors) > 0 {
		fmt.Printf("GOMOD CHECK FAIL: %d error(s)\n", len(errors))
		return 1
	}
	suffix := ""
	if len(warnings) > 0 {
		suffix = fmt.Sprintf(" (%d warning(s))", len(warnings))
	}
	fmt.Printf("GOMOD CHECK OK%s\n", suffix)
	return 0
}

// Run implements `repoman gomod check [path] [--strict-relative-replace]`.
func Run(args []string) int {
	if len(args) > 0 && (args[0] == "-h" || args[0] == "--help") {
		fmt.Println("usage: repoman gomod [-h] {check} ...")
		fmt.Println()
		fmt.Println("go.mod/go.sum sanity gate: no local-path replace directives escaping into a")
		fmt.Println("release, go.sum complete enough for a plain `go build -mod=readonly` to work")
		fmt.Println("with no extra magic.")
		fmt.Println()
		fmt.Println("positional arguments:")
		fmt.Println("  {check}")
		fmt.Println()
		fmt.Println("options:")
		fmt.Println("  -h, --help  show this help message and exit")
		return 0
	}
	if len(args) == 0 || args[0] != "check" {
		fmt.Fprintln(os.Stderr, "Usage: repoman gomod check [path] [--strict-relative-replace]")
		return 1
	}
	for _, a := range args[1:] {
		if a == "-h" || a == "--help" {
			fmt.Println("usage: repoman gomod check [-h] [--strict-relative-replace] [path]")
			fmt.Println()
			fmt.Println("positional arguments:")
			fmt.Println("  path                  module root (default: current directory)")
			fmt.Println()
			fmt.Println("options:")
			fmt.Println("  -h, --help            show this help message and exit")
			fmt.Println("  --strict-relative-replace")
			fmt.Println("                        also fail on relative-path (./..., ../...) replace")
			fmt.Println("                        directives, not just absolute ones")
			return 0
		}
	}
	path := "."
	strictRelative := false
	for _, a := range args[1:] {
		if a == "--strict-relative-replace" {
			strictRelative = true
		} else {
			path = a
		}
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return cmdCheck(abs, strictRelative)
}
