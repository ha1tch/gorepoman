// Package guards implements the dormant-guard registry -- translated
// from repoman/guards.py.
//
// A dormant guard is any verification that does not run in the default
// test invocation: stress/build-tagged tests, fuzz targets, multi-core
// race harnesses, long-running local suites. The registry lives in
// docs/KNOWN_ISSUES.md ("Dormant guards" section, G-nn entries).
//
// THE PRINCIPLE THIS TOOL SERVES: a shipped guard that never runs guards
// nothing. A guard's specification (the test exists) and its execution
// record (it ran, when, where) are different facts -- only the second is
// evidence. This tool makes the execution record queryable and updatable
// so the release process can check guard CURRENCY mechanically instead
// of by memory.
//
// Commands:
//
//	list                     one line per guard: id, last exercised, title
//	show G-nn                print a guard's full registry block
//	handoff [G-nn ...]       emit a ready-to-run block for guards needing
//	                         hardware this environment lacks
//	record G-nn --date YYYY-MM-DD --env ENV [--note TEXT] [--dry-run]
//	                         update a guard's Last exercised line
//	stale [--since YYYY-MM-DD]
//	                         guards not exercised since the date
package guards

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/ha1tch/gorepoman/pkg/config"
)

var dateRe = regexp.MustCompile(`\d{4}-\d{2}-\d{2}`)
var invokeRe = regexp.MustCompile(`(?m)^\s*[-|]?\s*\*?\*?Invocation:?\*?\*?\s*` + "`" + `([^` + "`" + `]+)` + "`")
var sectionRe = regexp.MustCompile(`(?m)^## Dormant guards.*$`)

type guard struct {
	gid, title, block string
	span              [2]int
}

func lastLineOf(block string, lastRe *regexp.Regexp) string {
	m := lastRe.FindStringSubmatch(block)
	if m == nil {
		return ""
	}
	return strings.TrimSpace(m[1])
}

// lastDate returns the most recent ISO-8601 date mentioned in the
// guard's Last-exercised bullet, not just the first -- a bullet can
// carry multiple dated addenda ("DATE ... Previous: OLDER-DATE ..."),
// and lexicographic comparison of "YYYY-MM-DD" strings is equivalent to
// chronological comparison, so position in the text never matters.
func lastDateOf(lastLine string) string {
	if lastLine == "" {
		return ""
	}
	dates := dateRe.FindAllString(lastLine, -1)
	if len(dates) == 0 {
		return ""
	}
	sort.Strings(dates)
	return dates[len(dates)-1]
}

func invocationsOf(block string, tableInvokeRe *regexp.Regexp) []string {
	var out []string
	for _, m := range invokeRe.FindAllStringSubmatch(block, -1) {
		out = append(out, m[1])
	}
	for _, m := range tableInvokeRe.FindAllStringSubmatch(block, -1) {
		out = append(out, fmt.Sprintf("%s  # %s", m[2], m[1]))
	}
	return out
}

type env struct {
	root          string
	cfg           config.Config
	gp            string // guard id prefix, e.g. "G-"
	headRe        *regexp.Regexp
	lastRe        *regexp.Regexp
	tableInvokeRe *regexp.Regexp
	knownIssues   string
}

func newEnv() (*env, error) {
	root, cfg, err := config.Load("")
	if err != nil {
		return nil, err
	}
	gp := cfg.GuardIDPrefix
	if gp == "" {
		gp = "G-"
	}
	gpRe := regexp.QuoteMeta(gp)
	return &env{
		root:          root,
		cfg:           cfg,
		gp:            gp,
		headRe:        regexp.MustCompile(`(?m)^### (` + gpRe + `\d+[a-z]?)\.\s*(.*)$`),
		lastRe:        regexp.MustCompile(`(?m)^- \*\*Last exercised[^:*]*:\*\*\s*(.*(?:\n[ \t]+\S.*)*)`),
		tableInvokeRe: regexp.MustCompile(`(?m)^\|\s*(` + gpRe + `\d+[a-z])\s*\|[^|]*\|[^|]*\|\s*` + "`" + `([^` + "`" + `]+)` + "`" + `\s*\|`),
		knownIssues:   filepath.Join(root, cfg.KnownIssues),
	}, nil
}

// parse reads docs/KNOWN_ISSUES.md and returns its text plus every guard
// block found under the "Dormant guards" section.
func (e *env) parse() (string, []guard, error) {
	b, err := os.ReadFile(e.knownIssues)
	if err != nil {
		return "", nil, err
	}
	text := string(b)
	sec := sectionRe.FindStringIndex(text)
	if sec == nil {
		fmt.Fprintln(os.Stderr, "docs/KNOWN_ISSUES.md: no 'Dormant guards' section — Part 3 §8 "+
			"requires one; create it before registering guards")
		os.Exit(2)
	}
	heads := e.headRe.FindAllStringSubmatchIndex(text[sec[1]:], -1)
	var guards []guard
	for i, h := range heads {
		// Shift indices back to whole-text offsets (h is relative to text[sec[1]:]).
		start := sec[1] + h[0]
		gidStart, gidEnd := sec[1]+h[2], sec[1]+h[3]
		titleStart, titleEnd := sec[1]+h[4], sec[1]+h[5]
		var end int
		if i+1 < len(heads) {
			end = sec[1] + heads[i+1][0]
		} else {
			end = len(text)
		}
		guards = append(guards, guard{
			gid:   text[gidStart:gidEnd],
			title: strings.TrimSpace(text[titleStart:titleEnd]),
			block: text[start:end],
			span:  [2]int{start, end},
		})
	}
	return text, guards, nil
}

// previousReleaseDate returns the date of the SECOND changelog entry --
// i.e. the previous release: the default staleness horizon.
func (e *env) previousReleaseDate() string {
	b, err := os.ReadFile(filepath.Join(e.root, e.cfg.Changelog))
	if err != nil {
		return "1970-01-01"
	}
	re := regexp.MustCompile(`(?m)^## \[[^\]]+\] - (\d{4}-\d{2}-\d{2})`)
	var dates []string
	for _, m := range re.FindAllStringSubmatch(string(b), -1) {
		dates = append(dates, m[1])
	}
	if len(dates) > 1 {
		return dates[1]
	}
	if len(dates) == 1 {
		return dates[0]
	}
	return "1970-01-01"
}

func (e *env) cmdList(guards []guard) int {
	for _, g := range guards {
		last := lastDateOf(lastLineOf(g.block, e.lastRe))
		if last == "" {
			last = "NEVER"
		}
		fmt.Printf("%-6s last=%-12s %s\n", g.gid, last, g.title)
	}
	return 0
}

func (e *env) cmdShow(guards []guard, gid string) int {
	for _, g := range guards {
		if g.gid == gid {
			fmt.Println(strings.TrimRight(g.block, " \t\r\n"))
			return 0
		}
	}
	fmt.Fprintf(os.Stderr, "no such guard: %s\n", gid)
	return 1
}

func (e *env) cmdHandoff(guards []guard, ids []string) int {
	idSet := make(map[string]bool)
	for _, id := range ids {
		idSet[id] = true
	}
	fmt.Println("# Dormant-guard handoff — run on multi-core hardware and report back")
	fmt.Println("# For each command: run from the repository root; capture the full")
	fmt.Println("# output; report PASS/FAIL, iteration counts, and wall time.")
	fmt.Println("# On report, the registry updates with:")
	fmt.Printf("#   repoman guards record <%snn> --date <date> --env m1\n", e.gp)
	fmt.Println()
	for _, g := range guards {
		if len(ids) > 0 && !idSet[g.gid] {
			continue
		}
		fmt.Printf("## %s. %s\n", g.gid, g.title)
		lastLine := lastLineOf(g.block, e.lastRe)
		if lastLine == "" {
			lastLine = "NEVER"
		}
		fmt.Printf("#  last exercised: %s\n", lastLine)
		invs := invocationsOf(g.block, e.tableInvokeRe)
		if len(invs) == 0 {
			invs = []string{"# (no invocation recorded in registry — see the guard's block)"}
		}
		for _, inv := range invs {
			fmt.Println(inv)
		}
		fmt.Println()
	}
	return 0
}

var previousExerciseRe = regexp.MustCompile(`\s*Previous( exercise)?:.*$`)

func (e *env) cmdRecord(text string, guards []guard, gid, date, envName, note string, dry bool) int {
	if !regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`).MatchString(date) {
		fmt.Fprintf(os.Stderr, "invalid date: %s (want YYYY-MM-DD)\n", date)
		return 1
	}
	for _, g := range guards {
		if g.gid != gid {
			continue
		}
		m := e.lastRe.FindStringSubmatchIndex(g.block)
		if m == nil {
			fmt.Fprintf(os.Stderr, "%s: no '- **Last exercised:**' line in its block — "+
				"add one by hand first\n", gid)
			return 1
		}
		old := strings.TrimSpace(g.block[m[2]:m[3]])
		newVal := fmt.Sprintf("%s env:%s", date, envName)
		if note != "" {
			newVal += fmt.Sprintf(" — %s", note)
		}
		prev := previousExerciseRe.ReplaceAllString(old, "")
		newVal += fmt.Sprintf(" Previous: %s", prev)
		newBlock := g.block[:m[2]] + newVal + g.block[m[3]:]
		newText := text[:g.span[0]] + newBlock + text[g.span[1]:]
		if dry {
			fmt.Printf("(dry-run) %s Last exercised would become:\n  %s\n", gid, newVal)
			return 0
		}
		if err := os.WriteFile(e.knownIssues, []byte(newText), 0644); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		fmt.Printf("%s recorded: %s\n", gid, newVal)
		return 0
	}
	fmt.Fprintf(os.Stderr, "no such guard: %s\n", gid)
	return 1
}

func (e *env) cmdStale(guards []guard, since string) int {
	var stale []guard
	for _, g := range guards {
		// Sub-entries (G-03a...) are covered by their parent's collective
		// Last exercised line; only top-level blocks carry the record.
		lastLine := lastLineOf(g.block, e.lastRe)
		if lastLine == "" {
			continue
		}
		ld := lastDateOf(lastLine)
		if ld == "" || ld < since {
			stale = append(stale, g)
		}
	}
	if len(stale) == 0 {
		fmt.Printf("all guards exercised since %s\n", since)
		return 0
	}
	fmt.Printf("STALE (not exercised since %s):\n", since)
	for _, g := range stale {
		ld := lastDateOf(lastLineOf(g.block, e.lastRe))
		if ld == "" {
			ld = "NEVER"
		}
		fmt.Printf("  %-6s last=%s  %s\n", g.gid, ld, g.title)
	}
	fmt.Println("Each must be run, handed off (guards handoff), or its skip " +
		"recorded explicitly in the release's changelog entry.")
	return 1
}

// Run implements the `repoman guards <list|show|handoff|record|stale> ...` CLI.
func Run(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: repoman guards <list|show|handoff|record|stale> ...")
		return 1
	}
	if args[0] == "-h" || args[0] == "--help" {
		fmt.Println("usage: repoman guards [-h] {list,show,handoff,record,stale} ...")
		fmt.Println()
		fmt.Println("Dormant-guard registry operations")
		fmt.Println()
		fmt.Println("positional arguments:")
		fmt.Println("  {list,show,handoff,record,stale}")
		fmt.Println()
		fmt.Println("options:")
		fmt.Println("  -h, --help            show this help message and exit")
		return 0
	}
	if len(args) >= 2 && (args[1] == "-h" || args[1] == "--help") {
		switch args[0] {
		case "list":
			fmt.Println("usage: repoman guards list [-h]")
			fmt.Println()
			fmt.Println("options:")
			fmt.Println("  -h, --help  show this help message and exit")
			return 0
		case "show":
			fmt.Println("usage: repoman guards show [-h] guard")
			fmt.Println()
			fmt.Println("positional arguments:")
			fmt.Println("  guard")
			fmt.Println()
			fmt.Println("options:")
			fmt.Println("  -h, --help  show this help message and exit")
			return 0
		case "handoff":
			fmt.Println("usage: repoman guards handoff [-h] [guards ...]")
			fmt.Println()
			fmt.Println("positional arguments:")
			fmt.Println("  guards")
			fmt.Println()
			fmt.Println("options:")
			fmt.Println("  -h, --help  show this help message and exit")
			return 0
		case "record":
			fmt.Println("usage: repoman guards record [-h] --date DATE --env ENV [--note NOTE] [--dry-run]")
			fmt.Println("                        guard")
			fmt.Println()
			fmt.Println("positional arguments:")
			fmt.Println("  guard")
			fmt.Println()
			fmt.Println("options:")
			fmt.Println("  -h, --help   show this help message and exit")
			fmt.Println("  --date DATE")
			fmt.Println("  --env ENV")
			fmt.Println("  --note NOTE")
			fmt.Println("  --dry-run")
			return 0
		case "stale":
			fmt.Println("usage: repoman guards stale [-h] [--since SINCE]")
			fmt.Println()
			fmt.Println("options:")
			fmt.Println("  -h, --help     show this help message and exit")
			fmt.Println("  --since SINCE  default: previous release's changelog date")
			return 0
		}
	}
	e, err := newEnv()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	text, guards, err := e.parse()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	cmd := args[0]
	rest := args[1:]
	switch cmd {
	case "list":
		return e.cmdList(guards)

	case "show":
		if len(rest) < 1 {
			fmt.Fprintln(os.Stderr, "show requires a guard id")
			return 1
		}
		return e.cmdShow(guards, rest[0])

	case "handoff":
		return e.cmdHandoff(guards, rest)

	case "record":
		if len(rest) < 1 {
			fmt.Fprintln(os.Stderr, "record requires a guard id")
			return 1
		}
		gid := rest[0]
		var date, envName, note string
		dry := false
		for i := 1; i < len(rest); i++ {
			switch rest[i] {
			case "--date":
				if i+1 < len(rest) {
					date = rest[i+1]
					i++
				}
			case "--env":
				if i+1 < len(rest) {
					envName = rest[i+1]
					i++
				}
			case "--note":
				if i+1 < len(rest) {
					note = rest[i+1]
					i++
				}
			case "--dry-run":
				dry = true
			}
		}
		if date == "" || envName == "" {
			fmt.Fprintln(os.Stderr, "record requires --date and --env")
			return 1
		}
		return e.cmdRecord(text, guards, gid, date, envName, note, dry)

	case "stale":
		since := ""
		for i := 0; i < len(rest); i++ {
			if rest[i] == "--since" && i+1 < len(rest) {
				since = rest[i+1]
				i++
			}
		}
		if since == "" {
			since = e.previousReleaseDate()
		}
		return e.cmdStale(guards, since)

	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", cmd)
		return 1
	}
}
