// Package badcode implements a release-blocking scan for forbidden
// text strings -- names, internal codenames, credentials-shaped
// tokens, "DO NOT SHIP" markers, anything that must never reach a
// release under any circumstances.
//
// THE DESIGN CONSTRAINT THAT SHAPES EVERYTHING HERE: the pattern list
// is deliberately NEVER stored in any repository. It lives in a local,
// per-machine config directory instead -- because a blocklist that
// ships inside the repo it's protecting can be edited by whoever has
// commit access, including an overly-permissive or compromised agent,
// in the same change that would have been caught by it. Keeping it
// local and out-of-band is the entire point: the check exists outside
// the reach of "just edit the config to stop being blocked."
//
// Config location: the OS-appropriate user config directory (per
// os.UserConfigDir() -- ~/.config on Linux, ~/Library/Application
// Support on macOS, %AppData% on Windows) under a "repoman" subdirectory,
// overridable via the REPOMAN_BADCODE_DIR environment variable. Two
// file formats are read, both optional, both additive if both exist:
//
//	badcode.txt   one pattern per line; blank lines and lines
//	              starting with '#' are ignored.
//	badcode.json  [{"pattern": "...", "reason": "..."}, ...] --
//	              "reason" is optional, and included in the refusal
//	              message when a pattern matches, so a match is
//	              actionable rather than just a bare string.
//
// Matching is a literal, case-insensitive substring search -- not
// regex. Regex adds failure modes (a pattern that silently doesn't
// compile, ReDoS, an off-by-one in an anchor) to a check whose entire
// value is that it never has a false "clean" result; a plain substring
// search can't fail in a way that looks like success.
//
// No configured patterns at all is NOT an error -- this is optional,
// local, per-operator tooling, and an operator who hasn't set it up
// yet shouldn't have every release blocked. But it is never silently
// treated as "checked and clean" either, matching this project's own
// guards principle (repoman-020-failure-modes.md #4): a check that
// never actually ran proves nothing, and this one says so plainly
// rather than looking identical to a real pass.
package badcode

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ha1tch/gorepoman/pkg/roles"
	"github.com/ha1tch/gorepoman/pkg/webhelp"
)

// Pattern is one forbidden string.
type Pattern struct {
	Text   string
	Reason string
	Source string // which config file this pattern came from
}

// Match is one place a Pattern was found.
type Match struct {
	Pattern Pattern
	File    string
	Line    int
	Snippet string
}

type jsonEntry struct {
	Pattern string `json:"pattern"`
	Reason  string `json:"reason"`
}

// ConfigDir returns the directory badcode.txt/badcode.json are read
// from: $REPOMAN_BADCODE_DIR if set, otherwise
// os.UserConfigDir()/repoman.
func ConfigDir() (string, error) {
	if d := os.Getenv("REPOMAN_BADCODE_DIR"); d != "" {
		return d, nil
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "repoman"), nil
}

// LoadPatterns reads every pattern from badcode.txt and badcode.json
// in the config directory. Neither file existing is not an error --
// it returns an empty, non-nil slice.
func LoadPatterns() ([]Pattern, string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return nil, "", err
	}
	var patterns []Pattern

	txtPath := filepath.Join(dir, "badcode.txt")
	if b, err := os.ReadFile(txtPath); err == nil {
		scanner := bufio.NewScanner(bytes.NewReader(b))
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			patterns = append(patterns, Pattern{Text: line, Source: txtPath})
		}
	}

	jsonPath := filepath.Join(dir, "badcode.json")
	if b, err := os.ReadFile(jsonPath); err == nil {
		var entries []jsonEntry
		if jerr := json.Unmarshal(b, &entries); jerr != nil {
			return nil, dir, fmt.Errorf("%s: invalid JSON: %w", jsonPath, jerr)
		}
		for _, e := range entries {
			if strings.TrimSpace(e.Pattern) == "" {
				continue
			}
			patterns = append(patterns, Pattern{Text: e.Pattern, Reason: e.Reason, Source: jsonPath})
		}
	}

	return patterns, dir, nil
}

// looksBinary reports whether the first chunk of data contains a NUL
// byte -- the same simple, standard heuristic git itself uses to
// decide whether to treat a file as binary. Binary release artifacts
// (compiled binaries, images) are skipped: a coincidental byte
// sequence matching a text pattern inside a binary is noise, not a
// real leak of that string as text.
func looksBinary(b []byte) bool {
	n := len(b)
	if n > 8192 {
		n = 8192
	}
	return bytes.IndexByte(b[:n], 0) != -1
}

// Check scans every file under paths (directories expanded via
// roles.Expand, so .git is already excluded) for every pattern, and
// returns every match found -- not just the first, so one run gives
// the complete picture rather than a whack-a-mole loop.
func Check(paths []string, patterns []Pattern) []Match {
	if len(patterns) == 0 {
		return nil
	}
	lowerPatterns := make([]string, len(patterns))
	for i, p := range patterns {
		lowerPatterns[i] = strings.ToLower(p.Text)
	}

	var matches []Match
	for _, f := range roles.Expand(paths) {
		b, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		if looksBinary(b) {
			continue
		}
		text := string(b)
		lines := strings.Split(text, "\n")
		// joined is text with every newline BYTE REMOVED (not replaced
		// with a space -- that would just substitute one separator for
		// another and still miss the case a wrap creates). lineOf[i]
		// gives the 1-indexed source line the i-th byte of joined came
		// from, built in the same pass, so a match found in joined can
		// be mapped back to real line numbers without re-scanning.
		// Exists to catch a pattern split across a line boundary -- an
		// ordinary line wrap is enough to do this by complete accident,
		// not just deliberate evasion, and that's exactly the class of
		// gap a literal-substring check has no business leaving open
		// given what this tool exists to catch.
		var joinedB strings.Builder
		lineOf := make([]int, 0, len(text))
		lineNo := 1
		for i := 0; i < len(text); i++ {
			if text[i] == '\n' {
				lineNo++
				continue
			}
			joinedB.WriteByte(text[i])
			lineOf = append(lineOf, lineNo)
		}
		lowerJoined := strings.ToLower(joinedB.String())
		joined := joinedB.String()

		for i, lp := range lowerPatterns {
			// Per-line pass: the precise, single-line matches.
			for lineNo, line := range lines {
				if strings.Contains(strings.ToLower(line), lp) {
					snippet := strings.TrimSpace(line)
					if len(snippet) > 120 {
						snippet = snippet[:120]
					}
					matches = append(matches, Match{
						Pattern: patterns[i],
						File:    f,
						Line:    lineNo + 1,
						Snippet: snippet,
					})
				}
			}
			// Fold pass: search the newline-stripped text for anything
			// that genuinely spans more than one source line -- a
			// same-line match here was already reported above, so it's
			// skipped to avoid double-counting, not missed.
			searchFrom := 0
			for {
				idx := strings.Index(lowerJoined[searchFrom:], lp)
				if idx < 0 {
					break
				}
				pos := searchFrom + idx
				searchFrom = pos + 1
				if pos+len(lp) > len(lineOf) {
					continue
				}
				startLine := lineOf[pos]
				endLine := lineOf[pos+len(lp)-1]
				if startLine == endLine {
					continue // fully on one line -- the per-line pass already has it
				}
				snippet := joined[pos : pos+len(lp)]
				if len(snippet) > 120 {
					snippet = snippet[:120]
				}
				matches = append(matches, Match{
					Pattern: patterns[i],
					File:    f,
					Line:    startLine,
					Snippet: fmt.Sprintf("[spans lines %d-%d, pattern split across a line wrap] %s",
						startLine, endLine, snippet),
				})
			}
		}
	}
	return matches
}

// Run implements `repoman badcode check [path ...]`.
func Run(argv []string) int {
	argv = webhelp.NormalizeBriefFirst(argv)
	if len(argv) > 0 && (argv[0] == "-h" || argv[0] == "--help") {
		fmt.Println("usage: repoman badcode [-h] {check} ...")
		fmt.Println()
		fmt.Println("Scan for forbidden strings loaded from a local, never-committed")
		fmt.Println("config -- see 'repoman badcode check --help' for details.")
		fmt.Println()
		fmt.Println("positional arguments:")
		fmt.Println("  {check}")
		fmt.Println()
		fmt.Println("options:")
		fmt.Println("  -h, --help  show this help message and exit")
		return 0
	}
	if len(argv) == 0 || argv[0] != "check" {
		fmt.Fprintln(os.Stderr, "Usage: repoman badcode check [path ...]")
		return 1
	}
	if len(argv) >= 2 && (argv[1] == "-h" || argv[1] == "--help") {
		fmt.Println("usage: repoman badcode check [-h] [path ...]")
		fmt.Println()
		fmt.Println("Scans path(s) (default: current directory) for any pattern")
		fmt.Println("configured in the local badcode.txt/badcode.json config --")
		fmt.Println("never stored in this or any other repository. Refuses (exit 1)")
		fmt.Println("if any pattern is found anywhere in the scanned files.")
		fmt.Println()
		fmt.Println("positional arguments:")
		fmt.Println("  path        file(s)/directory(ies) to scan (default: .)")
		fmt.Println()
		fmt.Println("options:")
		fmt.Println("  -h, --help  show this help message and exit")
		fmt.Println()
		fmt.Println("See https://ha1tch.github.io/gorepoman/docs/repoman-065-badcode.html")
		fmt.Println("for the full design rationale and how this integrates with relcore.")
		fmt.Println(webhelp.SuppressionNote)
		webhelp.PrintIfAvailable(os.Stdout, "repoman-065-badcode", argv)
		return 0
	}

	paths := argv[1:]
	if len(paths) == 0 {
		paths = []string{"."}
	}

	patterns, dir, err := LoadPatterns()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR badcode-config: %v\n", err)
		return 1
	}

	if len(patterns) == 0 {
		fmt.Printf("WARN no badcode patterns configured in %s -- nothing checked\n", dir)
		fmt.Println("     if this project has patterns from a prior session, check your own")
		fmt.Println("     notes/memory and recreate them here before release work -- this config")
		fmt.Println("     is deliberately never stored in the repository itself")
		fmt.Println("BADCODE CHECK OK (0 patterns configured)")
		return 0
	}

	matches := Check(paths, patterns)
	if len(matches) == 0 {
		fmt.Printf("BADCODE CHECK OK (%d pattern(s) checked)\n", len(patterns))
		return 0
	}

	for _, m := range matches {
		reason := ""
		if m.Pattern.Reason != "" {
			reason = fmt.Sprintf(" (%s)", m.Pattern.Reason)
		}
		fmt.Fprintf(os.Stderr, "ERROR badcode-match: pattern %q%s found in %s:%d: %s\n",
			m.Pattern.Text, reason, m.File, m.Line, m.Snippet)
	}
	fmt.Printf("BADCODE CHECK FAIL: %d match(es)\n", len(matches))
	return 1
}
