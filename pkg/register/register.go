package register

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ha1tch/repoman/pkg/config"
)

type Row struct {
	Theme    string
	Priority string
	Status   string
	Raw      string
}

type Item struct {
	TID      string
	Title    string
	Theme    string
	Priority string
	Status   string
	Blocks   string
	Body     string
}

type Register struct {
	Text  string
	Rows  map[string]Row
	Items map[string]Item
	Spans map[string][2]int
	Root  string
	Cfg   *config.Config
	idAlt string
}

func idNum(tid string, cfg *config.Config) (int, error) {
	primary := cfg.IDPrefix + cfg.IDSeparator
	if strings.HasPrefix(tid, primary) {
		return strconv.Atoi(tid[len(primary):])
	}
	if cfg.LegacyIDPrefix != "" {
		legacy := cfg.LegacyIDPrefix + cfg.LegacyIDSeparator
		if strings.HasPrefix(tid, legacy) {
			return strconv.Atoi(tid[len(legacy):])
		}
	}
	return 0, fmt.Errorf("unrecognized id format: %q", tid)
}

func parse(text, root string, cfg *config.Config) *Register {
	reg := &Register{
		Text:  text,
		Rows:  make(map[string]Row),
		Items: make(map[string]Item),
		Spans: make(map[string][2]int),
		Root:  root,
		Cfg:   cfg,
	}

	idAlt := regexp.QuoteMeta(cfg.IDPrefix) + regexp.QuoteMeta(cfg.IDSeparator) + `\d+`
	if cfg.LegacyIDPrefix != "" {
		idAlt = `(?:` + regexp.QuoteMeta(cfg.IDPrefix) + regexp.QuoteMeta(cfg.IDSeparator) + `|` +
			regexp.QuoteMeta(cfg.LegacyIDPrefix) + regexp.QuoteMeta(cfg.LegacyIDSeparator) + `)\d+`
	}
	reg.idAlt = idAlt

	rowRe := regexp.MustCompile(`(?m)^\| (` + idAlt + `) \|([^|]*)\|([^|]*)\|([^|]*)\|([^|]*)\|.*$`)
	for _, m := range rowRe.FindAllStringSubmatch(text, -1) {
		reg.Rows[m[1]] = Row{
			Theme:    strings.TrimSpace(m[3]),
			Priority: strings.Trim(strings.TrimSpace(m[4]), "*"),
			Status:   strings.TrimSpace(m[5]),
			Raw:      m[0],
		}
	}

	headRe := regexp.MustCompile(`(?m)^### (` + idAlt + `)\. (.*)$`)
	heads := headRe.FindAllStringSubmatchIndex(text, -1)
	fieldRe := regexp.MustCompile(`(?m)^Theme: (\S+) · Priority: \*{0,2}(P\d)\*{0,2} · Status: (\S+)(?: · Blocks/after: (.*))?$`)

	for _, h := range heads {
		start := h[0]
		tid := text[h[2]:h[3]]
		title := strings.TrimSpace(text[h[4]:h[5]])

		tail := text[h[1]:]
		nextRe := regexp.MustCompile(`(?m)^(### |## |---\s*$)`)
		loc := nextRe.FindStringIndex(tail)
		end := h[1]
		if loc != nil {
			end += loc[0]
		} else {
			end += len(tail)
		}

		block := text[start:end]
		fm := fieldRe.FindStringSubmatchIndex(block)
		if fm == nil {
			continue
		}

		bodyStart := fm[1]
		blocks := ""
		if len(fm) > 8 && fm[8] != -1 {
			blocks = block[fm[8]:fm[9]]
		}

		reg.Items[tid] = Item{
			TID:      tid,
			Title:    title,
			Theme:    block[fm[2]:fm[3]],
			Priority: block[fm[4]:fm[5]],
			Status:   block[fm[6]:fm[7]],
			Blocks:   blocks,
			Body:     strings.TrimRight(strings.TrimLeft(block[bodyStart:], "\r\n"), " \t\r\n") + "\n",
		}
		reg.Spans[tid] = [2]int{start, end}
	}

	return reg
}

func (r *Register) NextID() string {
	maxID := 0
	checkID := func(tid string) {
		if n, err := idNum(tid, r.Cfg); err == nil && n > maxID {
			maxID = n
		}
	}

	for tid := range r.Rows {
		checkID(tid)
	}
	for tid := range r.Items {
		checkID(tid)
	}

	resPath := filepath.Join(r.Root, r.Cfg.Resolved)
	b, err := os.ReadFile(resPath)
	if err == nil {
		resText := string(b)
		headRe := regexp.MustCompile(`(?m)^## \[.*?\]\s+(` + r.idAlt + `)\s`)
		for _, m := range headRe.FindAllStringSubmatch(resText, -1) {
			checkID(m[1])
		}
	}

	next := maxID + 1
	digits := fmt.Sprintf("%02d", next)
	if next >= 100 {
		digits = fmt.Sprintf("%d", next)
	}
	return r.Cfg.IDPrefix + r.Cfg.IDSeparator + digits
}

func writeWithDiff(path, newText string, dryRun bool, label string) {
	b, err := os.ReadFile(path)
	old := ""
	if err == nil {
		old = string(b)
	}
	if old == newText {
		fmt.Printf("   no change: %s\n", filepath.Base(path))
		return
	}
	if dryRun {
		// Simplified dry-run diff representation.
		fmt.Printf("   (dry-run) %s: %s not written\n", label, filepath.Base(path))
		return
	}
	os.WriteFile(path, []byte(newText), 0644)
	fmt.Printf("   %s: %s updated\n", label, filepath.Base(path))
}

func Run(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: repoman register <list|show|add|close|check> ...")
		return 1
	}
	cmd := args[0]
	if cmd == "-h" || cmd == "--help" {
		fmt.Println("usage: repoman register [-h] {list,show,add,close,check} ...")
		fmt.Println()
		fmt.Println("Live-register operations (docs/TRACKING.md)")
		fmt.Println()
		fmt.Println("positional arguments:")
		fmt.Println("  {list,show,add,close,check}")
		fmt.Println()
		fmt.Println("options:")
		fmt.Println("  -h, --help            show this help message and exit")
		fmt.Println()
		fmt.Println("See module docstring for the closure procedure this enforces.")
		return 0
	}
	if len(args) >= 2 && (args[1] == "-h" || args[1] == "--help") {
		switch cmd {
		case "list":
			fmt.Println("usage: repoman register list [-h]")
			fmt.Println()
			fmt.Println("options:")
			fmt.Println("  -h, --help  show this help message and exit")
			return 0
		case "show":
			fmt.Println("usage: repoman register show [-h] item")
			fmt.Println()
			fmt.Println("positional arguments:")
			fmt.Println("  item")
			fmt.Println()
			fmt.Println("options:")
			fmt.Println("  -h, --help  show this help message and exit")
			return 0
		case "add":
			fmt.Println("usage: repoman register add [-h] [--id ID] --summary SUMMARY --theme THEME")
			fmt.Println("                       --priority PRIORITY [--status STATUS] [--blocks BLOCKS]")
			fmt.Println("                       [--body BODY] [--body-file BODY_FILE] [--dry-run]")
			fmt.Println()
			fmt.Println("options:")
			fmt.Println("  -h, --help            show this help message and exit")
			fmt.Println("  --id ID               explicit id (default: next free)")
			fmt.Println("  --summary SUMMARY")
			fmt.Println("  --theme THEME")
			fmt.Println("  --priority PRIORITY   P1 (highest) .. P4")
			fmt.Println("  --status STATUS")
			fmt.Println("  --blocks BLOCKS")
			fmt.Println("  --body BODY")
			fmt.Println("  --body-file BODY_FILE")
			fmt.Println("  --dry-run")
			return 0
		case "close":
			fmt.Println("usage: repoman register close [-h] --version VERSION [--date DATE] [--dry-run] item")
			fmt.Println()
			fmt.Println("positional arguments:")
			fmt.Println("  item")
			fmt.Println()
			fmt.Println("options:")
			fmt.Println("  -h, --help         show this help message and exit")
			fmt.Println("  --version VERSION")
			fmt.Println("  --date DATE")
			fmt.Println("  --dry-run")
			return 0
		case "check":
			fmt.Println("usage: repoman register check [-h]")
			fmt.Println()
			fmt.Println("options:")
			fmt.Println("  -h, --help  show this help message and exit")
			return 0
		}
	}

	root, cfg, err := config.Load("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		return 1
	}

	trkPath := filepath.Join(root, cfg.Tracking)
	b, err := os.ReadFile(trkPath)
	if err != nil && cmd != "add" {
		fmt.Fprintf(os.Stderr, "cannot read %s: %v\n", cfg.Tracking, err)
		return 1
	}
	reg := parse(string(b), root, &cfg)

	switch cmd {
	case "list":
		var tids []string
		for tid := range reg.Items {
			tids = append(tids, tid)
		}
		sort.Slice(tids, func(i, j int) bool {
			n1, _ := idNum(tids[i], &cfg)
			n2, _ := idNum(tids[j], &cfg)
			return n1 < n2
		})
		for _, tid := range tids {
			it := reg.Items[tid]
			fmt.Printf("%s  %s  %s  [%s]  %s\n", tid, it.Status, it.Priority, it.Theme, it.Title)
		}
		return 0

	case "show":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "show requires an item ID")
			return 1
		}
		tid := args[1]
		if span, ok := reg.Spans[tid]; ok {
			fmt.Println(strings.TrimRight(reg.Text[span[0]:span[1]], "\r\n"))
			return 0
		}
		fmt.Fprintf(os.Stderr, "no such item: %s\n", tid)
		return 1

	case "add":
		var id, summary, theme, priority, status, blocks, body, bodyFile string
		dryRun := false
		status = "☐"

		for i := 1; i < len(args); i++ {
			switch args[i] {
			case "--id":
				if i+1 < len(args) {
					id = args[i+1]
					i++
				}
			case "--summary":
				if i+1 < len(args) {
					summary = args[i+1]
					i++
				}
			case "--theme":
				if i+1 < len(args) {
					theme = args[i+1]
					i++
				}
			case "--priority":
				if i+1 < len(args) {
					priority = args[i+1]
					i++
				}
			case "--status":
				if i+1 < len(args) {
					status = args[i+1]
					i++
				}
			case "--blocks":
				if i+1 < len(args) {
					blocks = args[i+1]
					i++
				}
			case "--body":
				if i+1 < len(args) {
					body = args[i+1]
					i++
				}
			case "--body-file":
				if i+1 < len(args) {
					bodyFile = args[i+1]
					i++
				}
			case "--dry-run":
				dryRun = true
			}
		}

		if summary == "" || theme == "" || priority == "" {
			fmt.Fprintln(os.Stderr, "add requires --summary, --theme, and --priority")
			return 1
		}

		tid := id
		if tid == "" {
			tid = reg.NextID()
		}
		if _, ok := reg.Rows[tid]; ok {
			fmt.Fprintf(os.Stderr, "id already exists: %s (ids are never reused)\n", tid)
			return 1
		}
		if _, ok := reg.Items[tid]; ok {
			fmt.Fprintf(os.Stderr, "id already exists: %s (ids are never reused)\n", tid)
			return 1
		}
		if status != "✓" && status != "◐" && status != "☐" && status != "✗" {
			fmt.Fprintf(os.Stderr, "invalid status %q; use one of {✓ ◐ ☐ ✗}\n", status)
			return 1
		}

		if bodyFile != "" {
			b, _ := os.ReadFile(bodyFile)
			body = string(b)
		}
		if strings.TrimSpace(body) == "" {
			fmt.Fprintln(os.Stderr, "a register item needs a body (--body / --body-file): "+
				"at minimum a Trigger line and a Scope line")
			return 1
		}

		text := reg.Text
		rowRe := regexp.MustCompile(`(?m)^\| ` + reg.idAlt + ` \|.*$`)
		rowMatches := rowRe.FindAllStringIndex(text, -1)
		if len(rowMatches) == 0 {
			fmt.Fprintln(os.Stderr, "cannot locate the status table")
			return 1
		}
		lastRowEnd := rowMatches[len(rowMatches)-1][1]

		bStr := blocks
		if bStr == "" {
			bStr = "—"
		}
		row := fmt.Sprintf("| %s | %s | %s | %s | %s | %s |", tid, summary, theme, priority, status, bStr)
		text = text[:lastRowEnd] + "\n" + row + text[lastRowEnd:]

		bSec := ""
		if blocks != "" {
			bSec = " · Blocks/after: " + blocks
		}
		section := fmt.Sprintf("### %s. %s\n\nTheme: %s · Priority: %s · Status: %s%s\n\n%s\n\n",
			tid, summary, theme, priority, status, bSec, strings.TrimRight(body, " \t\r\n"))

		themeRe := regexp.MustCompile(`(?m)^## ` + regexp.QuoteMeta(theme) + `\s*$`)
		tm := themeRe.FindStringIndex(text)
		if tm != nil {
			tail := text[tm[1]:]
			nextRe := regexp.MustCompile(`(?m)^## |^---\s*$`)
			nm := nextRe.FindStringIndex(tail)
			ins := tm[1]
			if nm != nil {
				ins += nm[0]
			} else {
				ins += len(tail)
			}
			text = text[:ins] + section + text[ins:]
		} else {
			if strings.HasSuffix(strings.TrimRight(text, " \t\r\n"), "---") {
				idx := strings.LastIndex(strings.TrimRight(text, " \t\r\n"), "\n---")
				if idx != -1 {
					text = text[:idx] + fmt.Sprintf("\n## %s\n\n", theme) + section + text[idx:]
				} else {
					text = strings.TrimRight(text, " \t\r\n") + fmt.Sprintf("\n\n## %s\n\n", theme) + section
				}
			} else {
				text = strings.TrimRight(text, " \t\r\n") + fmt.Sprintf("\n\n## %s\n\n", theme) + section
			}
		}

		writeWithDiff(trkPath, text, dryRun, "add "+tid)
		if !dryRun {
			fmt.Printf("filed %s; run `repoman register check` — and remember the status "+
				"table and field lines must not diverge\n", tid)
		}
		return 0

	case "close":
		var tid, version, date string
		dryRun := false
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "close requires an item ID")
			return 1
		}
		tid = args[1]
		for i := 2; i < len(args); i++ {
			switch args[i] {
			case "--version":
				if i+1 < len(args) {
					version = args[i+1]
					i++
				}
			case "--date":
				if i+1 < len(args) {
					date = args[i+1]
					i++
				}
			case "--dry-run":
				dryRun = true
			}
		}
		if version == "" {
			fmt.Fprintln(os.Stderr, "close requires --version")
			return 1
		}
		if date == "" {
			date = time.Now().Format("2006-01-02")
		}

		it, ok := reg.Items[tid]
		if !ok {
			fmt.Fprintf(os.Stderr, "no such item in the register: %s\n", tid)
			return 1
		}
		row, ok := reg.Rows[tid]
		if !ok {
			fmt.Fprintf(os.Stderr, "%s has a detail section but no table row — fix A2 first\n", tid)
			return 1
		}

		entry := fmt.Sprintf("## [%s] %s — %s (v%s, %s)\n\nTheme: %s · closed %s · %s\n%s\n\nCross-ref: CHANGELOG %s.\n\n",
			version, tid, it.Title, version, date, it.Theme, version, date, strings.TrimRight(it.Body, " \t\r\n"), version)

		resPath := filepath.Join(root, cfg.Resolved)
		resText := ""
		if b, err := os.ReadFile(resPath); err == nil {
			resText = string(b)
		}

		firstRe := regexp.MustCompile(`(?m)^## `)
		loc := firstRe.FindStringIndex(resText)
		if loc == nil {
			fmt.Fprintln(os.Stderr, "RESOLVED.md: cannot find insertion point (no '## ' entry)")
			return 1
		}
		newRes := resText[:loc[0]] + entry + resText[loc[0]:]

		span := reg.Spans[tid]
		text := reg.Text[:span[0]] + reg.Text[span[1]:]
		text = strings.Replace(text, row.Raw+"\n", "", 1)

		// Drop a theme group emptied by the removal.
		// Go doesn't support (?=...), so we match the next delimiter and put it back with $1.
		emptyThemeRe := regexp.MustCompile(`(?m)^## ` + regexp.QuoteMeta(it.Theme) + `\s*\n+(## |--- *$)`)
		text = emptyThemeRe.ReplaceAllString(text, "$1")

		writeWithDiff(resPath, newRes, dryRun, "close "+tid+" (record)")
		writeWithDiff(trkPath, text, dryRun, "close "+tid+" (register)")
		if !dryRun {
			fmt.Printf("closed %s at v%s. Remaining by hand: the CHANGELOG entry for %s should "+
				"cross-reference this closure (the changelog says what shipped; RESOLVED.md says "+
				"what was wrong — they reference, never duplicate).\n", tid, version, version)
		}
		return 0

	case "check":
		var errs []string
		for tid, r := range reg.Rows {
			if strings.Contains(r.Raw, "✓") {
				errs = append(errs, fmt.Sprintf("[A1] closed item in register: %s", r.Raw))
			}
			if _, ok := reg.Items[tid]; !ok {
				errs = append(errs, fmt.Sprintf("[A2] row without detail block: %s", tid))
			}
		}
		for tid, it := range reg.Items {
			if r, ok := reg.Rows[tid]; !ok {
				errs = append(errs, fmt.Sprintf("[A2] detail block without row: %s", tid))
			} else {
				if r.Theme != it.Theme || r.Priority != it.Priority || r.Status != it.Status {
					errs = append(errs, fmt.Sprintf("[A3] %s: table [%s %s %s] vs detail [%s %s %s]",
						tid, r.Theme, r.Priority, r.Status, it.Theme, it.Priority, it.Status))
				}
			}
			if it.Status != "✓" && it.Status != "◐" && it.Status != "☐" && it.Status != "✗" {
				errs = append(errs, fmt.Sprintf("[A3] %s: unknown status symbol %q", tid, it.Status))
			}
		}

		for _, e := range errs {
			fmt.Printf("ERROR %s\n", e)
		}
		if len(errs) > 0 {
			fmt.Printf("REGISTER CHECK FAIL: %d error(s)\n", len(errs))
			return 1
		}
		fmt.Printf("REGISTER CHECK OK: %d open item(s)\n", len(reg.Items))
		return 0

	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", cmd)
		return 1
	}
}
