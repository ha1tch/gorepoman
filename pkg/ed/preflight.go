package ed

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// T-23: the preflight check niplines runs against the *previewed*
// post-nip content on a scratch copy, never against the live file
// (docs/proposals/ed-insert-and-ticketed-niplines.md, Proposal 2,
// Phase 1). Dispatched by file extension, the same shape
// pkg/roles.Classify already uses for its own per-language
// dispatch -- extending to a new language is one new case and one
// new function, not a change to niplines itself.
//
// Deliberately not wired to any language beyond .go in this pass:
// gorepoman's own tree has no other language to test the interface
// against, and a speculative second implementation with nothing
// real exercising it would be exactly the kind of unverified guess
// this project's own working agreement warns against (Part 1 §3.3).
// A future language adds its own case in Preflight and its own
// function below, following goPreflight's shape.

// PreflightResult is what a preflight check reports back to the
// caller (T-23's niplines request step) for inclusion in the ticket
// readback. Available is false when no preflight exists for the
// file's extension at all -- distinct from Clean, which means a
// preflight ran and found nothing to warn about. Warnings is always
// empty when Available is false; niplines surfaces "no preflight
// available for this file type" in that case, never a fabricated
// clean bill of health.
type PreflightResult struct {
	Available bool     `json:"available"`
	Clean     bool     `json:"clean"`
	Warnings  []string `json:"warnings,omitempty"`
}

// Preflight runs the preflight check registered for path's
// extension against content -- the computed post-nip text, not
// whatever is currently on disk at path. path is used only to
// determine which language's checker applies and to give the
// scratch copy a matching extension (gofmt/vet care about file
// extension, not the original path); content is written to a
// temporary file under a fresh temp directory, checked, and the
// temp directory is removed before returning, so nothing outlives
// this one call and the live file at path is never touched.
func Preflight(path string, content string) PreflightResult {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".go":
		return goPreflight(content)
	default:
		return PreflightResult{Available: false}
	}
}

// goPreflight runs gofmt -l and go vet against content on a scratch
// copy. Both are advisory: a non-empty gofmt -l output or a go vet
// complaint is surfaced as a warning string, never a hard refusal --
// per the proposal, the caller (a human or an agent re-reading the
// ticket) decides whether a warning means cancel or confirm-anyway.
// go vet needs a real package to typecheck against (it cannot check
// a single file in isolation the way gofmt can), so the scratch copy
// is written as its own single-file package under a throwaway
// module -- go vet ./... from inside that directory, not go vet on
// the bare file path, which would fail outside any module/GOPATH
// context. A failure to even RUN either tool (missing gofmt/go on
// PATH, scratch-dir I/O error) is itself surfaced as a warning
// rather than silently treated as "clean" -- an agent seeing no
// warnings should be able to trust that absence means the checks
// actually ran, not that they were skipped.
func goPreflight(content string) PreflightResult {
	dir, err := os.MkdirTemp("", "ed-preflight-*")
	if err != nil {
		return PreflightResult{Available: true, Clean: false,
			Warnings: []string{"preflight: could not create scratch dir: " + err.Error()}}
	}
	defer os.RemoveAll(dir)

	scratchFile := filepath.Join(dir, "preflight.go")
	if err := os.WriteFile(scratchFile, []byte(content), 0644); err != nil {
		return PreflightResult{Available: true, Clean: false,
			Warnings: []string{"preflight: could not write scratch copy: " + err.Error()}}
	}

	var warnings []string

	if out, err := exec.Command("gofmt", "-l", scratchFile).CombinedOutput(); err != nil {
		warnings = append(warnings, "gofmt: could not run ("+err.Error()+")")
	} else if strings.TrimSpace(string(out)) != "" {
		warnings = append(warnings, "gofmt: would reformat this content")
	}

	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module edpreflight\n\ngo 1.21\n"), 0644); err != nil {
		warnings = append(warnings, "go vet: could not write scratch go.mod: "+err.Error())
	} else {
		cmd := exec.Command("go", "vet", "./...")
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			trimmed := strings.TrimSpace(string(out))
			if trimmed == "" {
				trimmed = err.Error()
			}
			warnings = append(warnings, "go vet: "+trimmed)
		}
	}

	return PreflightResult{Available: true, Clean: len(warnings) == 0, Warnings: warnings}
}
