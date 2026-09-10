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

	"github.com/ha1tch/gorepoman/pkg/config"
	"github.com/ha1tch/gorepoman/pkg/report"
	"github.com/ha1tch/gorepoman/pkg/webhelp"
)

type Row struct {
	Theme    string
	Priority string
	Status   string
	Raw      string
}

type Item struct {
	TID       string
	Title     string
	Theme     string
	Priority  string
	Status    string
	Wave      string
	FiledBy   string
	Blocks    string
	ClaimedBy string
	Body      string
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

// idParsed is the result of resolving a register id against the
// project's configured namespaces: its numeric part, which counter
// group it belongs to (see config.EffectiveIDNamespace), and the
// literal prefix it was written with.
type idParsed struct {
	Num      int
	GroupKey string
	Prefix   string
}

// parseID resolves tid against every namespace cfg recognises,
// longest prefix+separator first so a namespace whose prefix is a
// leading substring of another's (e.g. "T-" vs "TB-") never
// misclassifies the longer one's ids.
func parseID(tid string, cfg *config.Config) (idParsed, error) {
	nss := cfg.EffectiveIDNamespaces()
	sort.SliceStable(nss, func(i, j int) bool {
		return len(nss[i].Prefix+nss[i].Separator) > len(nss[j].Prefix+nss[j].Separator)
	})
	for _, ns := range nss {
		full := ns.Prefix + ns.Separator
		if strings.HasPrefix(tid, full) {
			n, err := strconv.Atoi(tid[len(full):])
			if err != nil {
				return idParsed{}, err
			}
			return idParsed{Num: n, GroupKey: ns.GroupKey, Prefix: ns.Prefix}, nil
		}
	}
	return idParsed{}, fmt.Errorf("unrecognized id format: %q", tid)
}

// idLess orders two ids for display: ids sharing a counter group (the
// primary namespace, and legacy ids folded into it) sort purely by
// number among themselves, since they represent one continuous
// sequence; ids from any other namespace sort within their own group
// by number, and groups themselves sort by group key ("" -- the
// primary/legacy group -- first, then each other namespace
// alphabetically by its own prefix). Falls back to a plain byte
// comparison for anything that fails to parse -- should not happen
// for an id already accepted into reg.Items/reg.Rows, but never
// crashes the ordering over it.
func idLess(a, b string, cfg *config.Config) bool {
	pa, errA := parseID(a, cfg)
	pb, errB := parseID(b, cfg)
	if errA != nil || errB != nil {
		return a < b
	}
	if pa.GroupKey != pb.GroupKey {
		return pa.GroupKey < pb.GroupKey
	}
	return pa.Num < pb.Num
}

// buildIDAlt returns the regex alternation matching any id in any
// namespace cfg recognises (primary, legacy if configured, and each
// entry in IDNamespaces) -- longest prefix+separator first, for the
// same reason parseID orders them that way.
func buildIDAlt(cfg *config.Config) string {
	nss := cfg.EffectiveIDNamespaces()
	sort.SliceStable(nss, func(i, j int) bool {
		return len(nss[i].Prefix+nss[i].Separator) > len(nss[j].Prefix+nss[j].Separator)
	})
	if len(nss) == 1 {
		return regexp.QuoteMeta(nss[0].Prefix) + regexp.QuoteMeta(nss[0].Separator) + `\d+`
	}
	parts := make([]string, len(nss))
	for i, ns := range nss {
		parts[i] = regexp.QuoteMeta(ns.Prefix) + regexp.QuoteMeta(ns.Separator)
	}
	return `(?:` + strings.Join(parts, `|`) + `)\d+`
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

	idAlt := buildIDAlt(cfg)
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
	// T-06: Wave is an optional field between Status and Blocks/after --
	// additive, backward compatible: an item with no Wave line parses
	// exactly as before (group 4 is simply absent), and no existing
	// table column changes at all. Deliberately a field-line addition
	// like Progress:, not a new mandatory table column -- that would
	// have been a breaking format change to every register everywhere.
	// T-26: Claimed-by follows the same additive pattern, as the
	// trailing optional segment after Blocks/after -- an advisory
	// work-claim identifier with no enforced syntax beyond non-empty,
	// deliberately independent of tier computation and board rendering
	// (neither RenderDefinition's axis dispatch nor dependency_order
	// tiering ever reads it).
	fieldRe := regexp.MustCompile(`(?m)^Theme: (\S+) · Priority: \*{0,2}(P\d)\*{0,2} · Status: (\S+)(?: · Wave: (\S+))?(?: · Filed-by: (\S+))?(?: · Blocks/after: ([^·\n]*[^ ·\n]))?(?: · Claimed-by: (\S+))?$`)

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
		wave := ""
		if len(fm) > 8 && fm[8] != -1 {
			wave = block[fm[8]:fm[9]]
		}
		filedBy := ""
		if len(fm) > 10 && fm[10] != -1 {
			filedBy = block[fm[10]:fm[11]]
		}
		blocks := ""
		if len(fm) > 12 && fm[12] != -1 {
			blocks = block[fm[12]:fm[13]]
		}
		claimedBy := ""
		if len(fm) > 14 && fm[14] != -1 {
			claimedBy = block[fm[14]:fm[15]]
		}

		reg.Items[tid] = Item{
			TID:       tid,
			Title:     title,
			Theme:     block[fm[2]:fm[3]],
			Priority:  block[fm[4]:fm[5]],
			Status:    block[fm[6]:fm[7]],
			Wave:      wave,
			FiledBy:   filedBy,
			Blocks:    blocks,
			ClaimedBy: claimedBy,
			Body:      strings.TrimRight(strings.TrimLeft(block[bodyStart:], "\r\n"), " \t\r\n") + "\n",
		}
		reg.Spans[tid] = [2]int{start, end}
	}

	return reg
}

// NextID returns the next unused id in the given namespace (the
// literal prefix, e.g. "BF" -- not its counter group key). An empty
// prefix means the primary namespace (Cfg.IDPrefix). The primary and
// (if configured) legacy namespaces share one counter, matching the
// historical single-sequence migration behaviour; every other
// namespace in Cfg.IDNamespaces gets its own counter that never
// shares numbers with any other namespace. The caller is expected to
// have already validated prefix against Cfg.EffectiveIDNamespaces()
// (register add's --id-prefix does); an unrecognised prefix here
// still gets a sane id of its own rather than panicking, treated as
// a brand-new, independently-numbered group of one.
func (r *Register) NextID(prefix string) string {
	if prefix == "" {
		prefix = r.Cfg.IDPrefix
	}

	target := config.EffectiveIDNamespace{Prefix: prefix, Separator: r.Cfg.IDSeparator, GroupKey: prefix}
	for _, ns := range r.Cfg.EffectiveIDNamespaces() {
		if ns.Prefix == prefix {
			target = ns
			break
		}
	}

	maxID := 0
	checkID := func(tid string) {
		p, err := parseID(tid, r.Cfg)
		if err == nil && p.GroupKey == target.GroupKey && p.Num > maxID {
			maxID = p.Num
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
	return target.Prefix + target.Separator + digits
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
	args = webhelp.NormalizeBriefFirst(args)
	format, args := report.ExtractFormat(args)
	view := ""
	var afterView []string
	for i := 0; i < len(args); i++ {
		if args[i] == "--view" {
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "--view requires a value")
				return 1
			}
			view = args[i+1]
			i++
			continue
		}
		afterView = append(afterView, args[i])
	}
	args = afterView
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: repoman register <list|show|add|close|check> ... [--format text|json|html]")
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
		fmt.Println("See https://ha1tch.github.io/gorepoman/docs/repoman-060-register-and-guards.html")
		fmt.Println("for the closure procedure this enforces and worked examples.")
		fmt.Println(webhelp.SuppressionNote)
		webhelp.PrintIfAvailable(os.Stdout, "repoman-060-register-and-guards", args)
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
			fmt.Println("usage: repoman register add [-h] [--id ID] [--id-prefix PREFIX] --summary SUMMARY")
			fmt.Println("                       --theme THEME --priority PRIORITY [--status STATUS]")
			fmt.Println("                       [--wave WAVE] [--filed-by FILED_BY] [--blocks BLOCKS]")
			fmt.Println("                       [--claimed-by CLAIMED_BY] [--body BODY]")
			fmt.Println("                       [--body-file BODY_FILE] [--dry-run]")
			fmt.Println()
			fmt.Println("options:")
			fmt.Println("  -h, --help            show this help message and exit")
			fmt.Println("  --id ID               explicit id (default: next free in that namespace)")
			fmt.Println("  --id-prefix PREFIX    which configured id namespace to allocate from")
			fmt.Println("                        (default: the primary namespace); see id_namespaces")
			fmt.Println("                        in .repoman.json")
			fmt.Println("  --summary SUMMARY")
			fmt.Println("  --theme THEME")
			fmt.Println("  --priority PRIORITY   P1 (highest) .. P4")
			fmt.Println("  --status STATUS")
			fmt.Println("  --wave WAVE")
			fmt.Println("  --filed-by FILED_BY")
			fmt.Println("  --blocks BLOCKS")
			fmt.Println("  --claimed-by CLAIMED_BY")
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
		if view == "kanban" {
			return runKanban(reg, &cfg, format)
		}
		switch format {
		case "text":
			tids := sortedTIDs(reg, &cfg)
			for _, tid := range tids {
				it := reg.Items[tid]
				fmt.Printf("%s  %s  %s  [%s]  %s\n", tid, it.Status, it.Priority, it.Theme, it.Title)
			}
			return 0
		case "json":
			data := toSummaries(reg, &cfg)
			if err := ValidateList(data); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				return 1
			}
			return report.EmitOrErr(report.EmitJSON(os.Stdout, "register", "register-list", SchemaVersion, data))
		case "html":
			body, err := renderListHTML(toSummaries(reg, &cfg))
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				return 1
			}
			return report.EmitOrErr(report.EmitHTML(os.Stdout, "register", "register", "register-list", body))
		default:
			fmt.Fprintf(os.Stderr, "unknown format %q (want text, json, or html)\n", format)
			return 1
		}

	case "show":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "show requires an item ID")
			return 1
		}
		tid := args[1]
		switch format {
		case "text":
			if span, ok := reg.Spans[tid]; ok {
				fmt.Println(strings.TrimRight(reg.Text[span[0]:span[1]], "\r\n"))
				return 0
			}
			fmt.Fprintf(os.Stderr, "no such item: %s\n", tid)
			return 1
		case "json", "html":
			detail, ok := toDetail(reg, tid)
			if !ok {
				fmt.Fprintf(os.Stderr, "no such item: %s\n", tid)
				return 1
			}
			if format == "json" {
				if err := ValidateDetail(detail); err != nil {
					fmt.Fprintf(os.Stderr, "error: %v\n", err)
					return 1
				}
				return report.EmitOrErr(report.EmitJSON(os.Stdout, "register", "register-item", SchemaVersion, detail))
			}
			body, err := renderDetailHTML(detail)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				return 1
			}
			return report.EmitOrErr(report.EmitHTML(os.Stdout, detail.ID, "register", "register-item", body))
		default:
			fmt.Fprintf(os.Stderr, "unknown format %q (want text, json, or html)\n", format)
			return 1
		}

	case "add":
		var id, summary, theme, priority, status, wave, filedBy, blocks, claimedBy, body, bodyFile, idPrefix string
		dryRun := false
		status = "☐"

		for i := 1; i < len(args); i++ {
			switch args[i] {
			case "--id":
				if i+1 < len(args) {
					id = args[i+1]
					i++
				}
			case "--id-prefix":
				if i+1 < len(args) {
					idPrefix = args[i+1]
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
			case "--wave":
				if i+1 < len(args) {
					wave = args[i+1]
					i++
				}
			case "--filed-by":
				if i+1 < len(args) {
					filedBy = args[i+1]
					i++
				}
			case "--blocks":
				if i+1 < len(args) {
					blocks = args[i+1]
					i++
				}
			case "--claimed-by":
				if i+1 < len(args) {
					claimedBy = args[i+1]
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
		if idPrefix != "" {
			valid := false
			var known []string
			for _, ns := range cfg.EffectiveIDNamespaces() {
				known = append(known, ns.Prefix)
				if ns.Prefix == idPrefix {
					valid = true
				}
			}
			if !valid {
				fmt.Fprintf(os.Stderr, "--id-prefix %q is not a configured namespace (known: %s) -- "+
					"add it to id_namespaces in .repoman.json first\n", idPrefix, strings.Join(known, ", "))
				return 1
			}
		}
		if filedBy != "" {
			normalized, err := NormalizeFiledBy(filedBy)
			if err != nil {
				fmt.Fprintln(os.Stderr, "--filed-by:", err)
				return 1
			}
			filedBy = normalized
		}
		claimedBy = strings.TrimSpace(claimedBy)

		tid := id
		if tid == "" {
			tid = reg.NextID(idPrefix)
		}
		if _, ok := reg.Rows[tid]; ok {
			fmt.Fprintf(os.Stderr, "id already exists: %s (ids are never reused)\n", tid)
			return 1
		}
		if _, ok := reg.Items[tid]; ok {
			fmt.Fprintf(os.Stderr, "id already exists: %s (ids are never reused)\n", tid)
			return 1
		}
		if status != "✓" && status != "◐" && status != "☐" && status != "✗" && status != "☑" {
			fmt.Fprintf(os.Stderr, "invalid status %q; use one of {✓ ◐ ☐ ✗ ☑}\n", status)
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
		var lastRowEnd int
		if len(rowMatches) == 0 {
			// No existing row matches the ID pattern -- but a genuinely
			// empty table (header + separator, zero data rows yet) is a
			// real, valid table, not "no table located". Anchor after the
			// status table's own header+separator instead: the header row
			// starts with "| ID |" (the documented column name), followed
			// immediately by a markdown table separator line. Matching
			// that pair specifically, not just any separator row in the
			// file, avoids anchoring on an unrelated table elsewhere in
			// the document (a worked example in prose, for instance).
			headerRe := regexp.MustCompile(`(?m)^\| ?ID ?\|.*\n\|[-:| ]+\|\s*$`)
			hm := headerRe.FindStringIndex(text)
			if hm == nil {
				fmt.Fprintln(os.Stderr, "cannot locate the status table")
				return 1
			}
			lastRowEnd = hm[1]
		} else {
			lastRowEnd = rowMatches[len(rowMatches)-1][1]
		}

		bStr := blocks
		if bStr == "" {
			bStr = "—"
		}
		row := fmt.Sprintf("| %s | %s | %s | %s | %s | %s |", tid, summary, theme, priority, status, bStr)
		text = text[:lastRowEnd] + "\n" + row + text[lastRowEnd:]

		wSec := ""
		if wave != "" {
			wSec = " · Wave: " + wave
		}
		fSec := ""
		if filedBy != "" {
			fSec = " · Filed-by: " + filedBy
		}
		bSec := ""
		if blocks != "" {
			bSec = " · Blocks/after: " + blocks
		}
		cSec := ""
		if claimedBy != "" {
			cSec = " · Claimed-by: " + claimedBy
		}
		section := fmt.Sprintf("### %s. %s\n\nTheme: %s · Priority: %s · Status: %s%s%s%s%s\n\n%s\n\n",
			tid, summary, theme, priority, status, wSec, fSec, bSec, cSec, strings.TrimRight(body, " \t\r\n"))

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

		// B-02 fix: a repository's very first-ever closure has only header
		// prose in RESOLVED.md, no "## " entry yet -- that used to be
		// refused outright. Append after whatever text already exists
		// instead of requiring a heading that can't exist before the
		// first real closure.
		firstRe := regexp.MustCompile(`(?m)^## `)
		var newRes string
		if loc := firstRe.FindStringIndex(resText); loc != nil {
			newRes = resText[:loc[0]] + entry + resText[loc[0]:]
		} else {
			sep := ""
			if resText != "" && !strings.HasSuffix(resText, "\n") {
				sep = "\n"
			}
			if resText != "" {
				sep += "\n"
			}
			newRes = resText + sep + entry
		}

		span := reg.Spans[tid]
		text := reg.Text[:span[0]] + reg.Text[span[1]:]
		text = strings.Replace(text, row.Raw+"\n", "", 1)

		// Drop a theme group emptied by the removal.
		// Go doesn't support (?=...), so we match the next delimiter and put it back with $1.
		// B-03 fix: added \z (true end of text) as a third valid follow-up --
		// previously only "another heading follows" or "the closing --- rule
		// follows" counted, so a theme heading emptied by closing its last
		// item was left as an orphan whenever it also happened to be the
		// last theme in the file, with nothing after it to match against.
		emptyThemeRe := regexp.MustCompile(`(?m)^## ` + regexp.QuoteMeta(it.Theme) + `\s*\n+(## |--- *$|\z)`)
		text = emptyThemeRe.ReplaceAllString(text, "$1")

		writeWithDiff(resPath, newRes, dryRun, "close "+tid+" (record)")
		writeWithDiff(trkPath, text, dryRun, "close "+tid+" (register)")

		// T-19 fix: propagate the closure to a linked wave row. Before
		// this, closing an item that a wave table referenced via its
		// register_item column left that row own checkbox untouched --
		// the two documents drifted apart on every single closure, and
		// were hand-patched three separate times before this was filed
		// and fixed. This is the same operation, not a second manual
		// step someone has to remember.
		propagated := propagateCloseToWaves(root, &cfg, tid, dryRun)

		if !dryRun {
			fmt.Printf("closed %s at v%s. Remaining by hand: the CHANGELOG entry for %s should "+
				"cross-reference this closure (the changelog says what shipped; RESOLVED.md says "+
				"what was wrong — they reference, never duplicate).\n", tid, version, version)
			if propagated {
				fmt.Printf("   wave: %s marked done in %s and wave progress regenerated\n", tid, cfg.WaveTracking)
			}
		}
		return 0

	case "check":
		switch format {
		case "text":
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
				if it.Status != "✓" && it.Status != "◐" && it.Status != "☐" && it.Status != "✗" && it.Status != "☑" {
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
		case "json":
			data := computeCheck(reg, &cfg)
			if err := ValidateCheck(data); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				return 1
			}
			ec := report.EmitOrErr(report.EmitJSON(os.Stdout, "register", "register-check", SchemaVersion, data))
			if ec == 0 && !data.OK {
				return 1
			}
			return ec
		case "html":
			data := computeCheck(reg, &cfg)
			body, err := renderCheckHTML(data)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				return 1
			}
			ec := report.EmitOrErr(report.EmitHTML(os.Stdout, "register check", "register", "register-check", body))
			if ec == 0 && !data.OK {
				return 1
			}
			return ec
		default:
			fmt.Fprintf(os.Stderr, "unknown format %q (want text, json, or html)\n", format)
			return 1
		}

	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", cmd)
		return 1
	}
}
