package ed

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ha1tch/repoman/pkg/roles"
)

const (
	MaxTxns  = 200
	MaxBytes = 10 * 1024 * 1024
	CtxBytes = 64
)

type Edit struct {
	File   string `json:"file"`
	Offset int    `json:"offset"`
	Old    string `json:"old"`
	New    string `json:"new"`
}

type Txn struct {
	ID    int    `json:"id"`
	At    string `json:"at"`
	Label string `json:"label"`
	Edits []Edit `json:"edits"`
}

type Evicted struct {
	Count   int               `json:"count"`
	Anchors map[string]string `json:"anchors"`
}

type Journal struct {
	Txns    []Txn          `json:"txns"`
	Marks   map[string]int `json:"marks"`
	Evicted Evicted        `json:"evicted"`
}

func journalPath() string {
	cwd, _ := os.Getwd()
	return filepath.Join(cwd, ".ed-journal.json")
}

func LoadJournal() Journal {
	j := Journal{
		Txns:  []Txn{},
		Marks: make(map[string]int),
		Evicted: Evicted{
			Count:   0,
			Anchors: make(map[string]string),
		},
	}
	b, err := os.ReadFile(journalPath())
	if err == nil {
		_ = json.Unmarshal(b, &j)
	}
	if j.Marks == nil {
		j.Marks = make(map[string]int)
	}
	if j.Evicted.Anchors == nil {
		j.Evicted.Anchors = make(map[string]string)
	}
	return j
}

func SaveJournal(j *Journal) error {
	storedSize := func(txns []Txn) int {
		total := 0
		for _, t := range txns {
			for _, e := range t.Edits {
				total += len(e.Old) + len(e.New)
			}
		}
		return total
	}

	for len(j.Txns) > MaxTxns || storedSize(j.Txns) > MaxBytes {
		ev := j.Txns[0]
		j.Txns = j.Txns[1:]
		j.Evicted.Count++
		for _, e := range ev.Edits {
			b, err := os.ReadFile(e.File)
			if err == nil {
				h := sha256.Sum256(b)
				j.Evicted.Anchors[e.File] = fmt.Sprintf("%x", h)[:16]
			}
		}
		newMarks := make(map[string]int)
		for k, v := range j.Marks {
			stillExists := false
			for _, t := range j.Txns {
				if t.ID == v {
					stillExists = true
					break
				}
			}
			if stillExists {
				newMarks[k] = v
			}
		}
		j.Marks = newMarks
	}

	tmp := journalPath() + ".tmp"
	b, err := json.Marshal(j)
	if err != nil {
		return err
	}
	if err := os.WriteFile(tmp, b, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, journalPath())
}

func SpanHash(text string, start, end int) string {
	lo := start - CtxBytes
	if lo < 0 {
		lo = 0
	}
	hi := end + CtxBytes
	if hi > len(text) {
		hi = len(text)
	}
	h := sha256.Sum256([]byte(text[lo:hi]))
	return fmt.Sprintf("%x", h)[:8]
}

func Record(j *Journal, edits []Edit, label string) {
	id := 1
	if len(j.Txns) > 0 {
		id = j.Txns[len(j.Txns)-1].ID + 1
	} else if j.Evicted.Count > 0 {
		id = j.Evicted.Count + 1
	}
	j.Txns = append(j.Txns, Txn{
		ID:    id,
		At:    time.Now().UTC().Format("2006-01-02T15:04:05Z"),
		Label: label,
		Edits: edits,
	})
	SaveJournal(j)
}

func parseHandle(h string) (string, int, int, string, error) {
	parts := strings.Split(h, ":")
	if len(parts) < 3 {
		return "", 0, 0, "", fmt.Errorf("malformed handle %q; expected file:start-end:hash", h)
	}
	hash := parts[len(parts)-1]
	span := parts[len(parts)-2]
	path := strings.Join(parts[:len(parts)-2], ":")

	spanParts := strings.Split(span, "-")
	if len(spanParts) != 2 {
		return "", 0, 0, "", fmt.Errorf("malformed span")
	}
	s, err1 := strconv.Atoi(spanParts[0])
	e, err2 := strconv.Atoi(spanParts[1])
	if err1 != nil || err2 != nil {
		return "", 0, 0, "", fmt.Errorf("malformed span bounds")
	}
	return path, s, e, hash, nil
}

func revertTxn(t Txn) error {
	perFile := make(map[string][]Edit)
	for _, e := range t.Edits {
		perFile[e.File] = append(perFile[e.File], e)
	}

	staged := make(map[string]string)
	for fname, edits := range perFile {
		b, err := os.ReadFile(fname)
		if err != nil {
			return fmt.Errorf("%s no longer exists", fname)
		}
		text := string(b)

		sort.Slice(edits, func(i, j int) bool {
			return edits[i].Offset > edits[j].Offset
		})

		for _, e := range edits {
			s := e.Offset
			if s+len(e.New) > len(text) || text[s:s+len(e.New)] != e.New {
				return fmt.Errorf("%s changed at offset %d since the edit — cannot undo safely", fname, s)
			}
			text = text[:s] + e.Old + text[s+len(e.New):]
		}
		staged[fname] = text
	}

	for p, text := range staged {
		os.WriteFile(p, []byte(text), 0644)
	}
	return nil
}

func Run(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: repoman ed <find|apply|sub|undo|mark|log|selftest> ...")
		return 1
	}

	cmd := args[0]
	switch cmd {
	case "-h", "--help":
		fmt.Println("usage: repoman ed [-h] {find,apply,sub,undo,mark,log,selftest} ...")
		fmt.Println()
		fmt.Println("journaled precise text editing")
		fmt.Println()
		fmt.Println("positional arguments:")
		fmt.Println("  {find,apply,sub,undo,mark,log,selftest}")
		fmt.Println()
		fmt.Println("options:")
		fmt.Println("  -h, --help            show this help message and exit")
		return 0

	case "find":
		if len(args) >= 2 && (args[1] == "-h" || args[1] == "--help") {
			fmt.Println("usage: repoman ed find [-h] [--regex] term [paths ...]")
			fmt.Println()
			fmt.Println("positional arguments:")
			fmt.Println("  term")
			fmt.Println("  paths")
			fmt.Println()
			fmt.Println("options:")
			fmt.Println("  -h, --help  show this help message and exit")
			fmt.Println("  --regex")
			return 0
		}
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "find requires a term")
			return 1
		}
		term := args[1]
		targetPaths := []string{"."}
		isRegex := false
		for _, a := range args[2:] {
			if a == "--regex" {
				isRegex = true
			} else {
				if len(targetPaths) == 1 && targetPaths[0] == "." {
					targetPaths = []string{}
				}
				targetPaths = append(targetPaths, a)
			}
		}
		paths := roles.Expand(targetPaths)
		n := 0
		for _, occ := range roles.Occurrences(term, paths, isRegex) {
			b, _ := os.ReadFile(occ.Path)
			h := SpanHash(string(b), occ.Start, occ.End)
			lineTrim := strings.TrimSpace(occ.Line)
			if len(lineTrim) > 80 {
				lineTrim = lineTrim[:80]
			}
			fmt.Printf("%s:%d-%d:%s  [%s]  line %d: %s\n", occ.Path, occ.Start, occ.End, h, occ.Role, occ.LineNo, lineTrim)
			n++
		}
		fmt.Printf("%d occurrence(s)\n", n)
		return 0

	case "apply":
		for _, a := range args[1:] {
			if a == "-h" || a == "--help" {
				fmt.Println("usage: repoman ed apply [-h] --with REPLACEMENT handle")
				fmt.Println()
				fmt.Println("positional arguments:")
				fmt.Println("  handle")
				fmt.Println()
				fmt.Println("options:")
				fmt.Println("  -h, --help          show this help message and exit")
				fmt.Println("  --with REPLACEMENT")
				return 0
			}
		}
		if len(args) < 4 || args[2] != "--with" {
			fmt.Fprintln(os.Stderr, "Usage: repoman ed apply <handle> --with <text>")
			return 1
		}
		handle := args[1]
		replacement := args[3]
		path, s, e, hash, err := parseHandle(handle)
		if err != nil {
			fmt.Fprintf(os.Stderr, "REFUSED: %v\n", err)
			return 1
		}
		b, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "REFUSED: %s does not exist\n", path)
			return 1
		}
		text := string(b)
		if e > len(text) || SpanHash(text, s, e) != hash {
			fmt.Fprintf(os.Stderr, "REFUSED: %s changed since find (stale handle) — re-run find and use a fresh handle\n", path)
			return 1
		}
		old := text[s:e]
		newText := text[:s] + replacement + text[e:]
		os.WriteFile(path, []byte(newText), 0644)

		j := LoadJournal()
		Record(&j, []Edit{{File: path, Offset: s, Old: old, New: replacement}}, "apply "+filepath.Base(path))

		oldTrim := old
		if len(oldTrim) > 40 {
			oldTrim = oldTrim[:40]
		}
		newTrim := replacement
		if len(newTrim) > 40 {
			newTrim = newTrim[:40]
		}
		fmt.Printf("applied at %s:%d: %q -> %q\n", path, s, oldTrim, newTrim)
		return 0

	case "sub":
		for _, a := range args[1:] {
			if a == "-h" || a == "--help" {
				fmt.Println("usage: repoman ed sub [-h] --expect EXPECT [--force-roles] old new [paths ...]")
				fmt.Println()
				fmt.Println("positional arguments:")
				fmt.Println("  old")
				fmt.Println("  new")
				fmt.Println("  paths")
				fmt.Println()
				fmt.Println("options:")
				fmt.Println("  -h, --help       show this help message and exit")
				fmt.Println("  --expect EXPECT")
				fmt.Println("  --force-roles")
				return 0
			}
		}
		if len(args) < 5 { // sub old new --expect N
			fmt.Fprintln(os.Stderr, "Usage: repoman ed sub <old> <new> [path ...] --expect N [--force-roles]")
			return 1
		}
		oldText := args[1]
		newText := args[2]
		var expect int
		forceRoles := false
		targetPaths := []string{"."}

		parsingPaths := true
		for i := 3; i < len(args); i++ {
			if args[i] == "--expect" {
				if i+1 >= len(args) {
					fmt.Fprintln(os.Stderr, "--expect requires a number")
					return 1
				}
				expect, _ = strconv.Atoi(args[i+1])
				i++
				parsingPaths = false
			} else if args[i] == "--force-roles" {
				forceRoles = true
				parsingPaths = false
			} else if parsingPaths {
				if len(targetPaths) == 1 && targetPaths[0] == "." {
					targetPaths = []string{}
				}
				targetPaths = append(targetPaths, args[i])
			}
		}

		paths := roles.Expand(targetPaths)
		type planEntry struct {
			path string
			text string
			offs []int
		}
		var plan []planEntry
		total := 0
		roleSet := make(map[string]bool)

		for _, p := range paths {
			b, err := os.ReadFile(p)
			if err != nil {
				continue
			}
			text := string(b)
			var offs []int
			i := strings.Index(text, oldText)
			for i != -1 {
				offs = append(offs, i)
				roleSet[roles.Classify(p, text, i)] = true
				next := strings.Index(text[i+1:], oldText)
				if next == -1 {
					break
				}
				i += 1 + next
			}
			if len(offs) > 0 {
				plan = append(plan, planEntry{p, text, offs})
				total += len(offs)
				fmt.Printf("  %s: %d\n", p, len(offs))
			}
		}

		if total != expect {
			fmt.Fprintf(os.Stderr, "REFUSED: found %d occurrence(s), --expect %d. Nothing written. Re-count, or narrow the paths.\n", total, expect)
			return 1
		}
		if len(roleSet) > 1 && !forceRoles {
			var rs []string
			for r := range roleSet {
				rs = append(rs, r)
			}
			sort.Strings(rs)
			fmt.Fprintf(os.Stderr, "REFUSED: occurrences span multiple syntactic roles %v — one pass is not safe (working agreement §7.1). Split by role, or pass --force-roles if you have classified them as one treatment.\n", rs)
			return 1
		}

		var edits []Edit
		for _, e := range plan {
			newFileText := strings.ReplaceAll(e.text, oldText, newText)
			os.WriteFile(e.path, []byte(newFileText), 0644)
			for _, o := range e.offs {
				edits = append(edits, Edit{File: e.path, Offset: o, Old: oldText, New: newText})
			}
		}

		j := LoadJournal()
		labelOld := oldText
		if len(labelOld) > 30 {
			labelOld = labelOld[:30]
		}
		labelNew := newText
		if len(labelNew) > 30 {
			labelNew = labelNew[:30]
		}
		Record(&j, edits, fmt.Sprintf("sub %q->%q", labelOld, labelNew))
		fmt.Printf("replaced %d occurrence(s) across %d file(s)\n", total, len(plan))
		return 0

	case "undo":
		for _, a := range args[1:] {
			if a == "-h" || a == "--help" {
				fmt.Println("usage: repoman ed undo [-h] [--since SINCE] [n]")
				fmt.Println()
				fmt.Println("positional arguments:")
				fmt.Println("  n")
				fmt.Println()
				fmt.Println("options:")
				fmt.Println("  -h, --help     show this help message and exit")
				fmt.Println("  --since SINCE")
				return 0
			}
		}
		n := 1
		since := ""
		for i := 1; i < len(args); i++ {
			if args[i] == "--since" && i+1 < len(args) {
				since = args[i+1]
				i++
			} else {
				if parsed, err := strconv.Atoi(args[i]); err == nil {
					n = parsed
				}
			}
		}

		j := LoadJournal()
		var batch []Txn

		if since != "" {
			target, ok := j.Marks[since]
			if !ok {
				fmt.Fprintf(os.Stderr, "REFUSED: mark %q not in the journal (unknown, or evicted — %d transaction(s) have rolled off). No partial undo offered.\n", since, j.Evicted.Count)
				return 1
			}
			for _, t := range j.Txns {
				if t.ID > target {
					batch = append(batch, t)
				}
			}
		} else {
			if len(j.Txns) < n {
				n = len(j.Txns)
			}
			if n > 0 {
				batch = j.Txns[len(j.Txns)-n:]
			}
		}

		if len(batch) == 0 {
			fmt.Println("nothing to undo")
			return 0
		}

		for i := len(batch) - 1; i >= 0; i-- {
			t := batch[i]
			err := revertTxn(t)
			if err != nil {
				fmt.Fprintf(os.Stderr, "REFUSED at txn %d (%s): %v. Transactions after it were already reverted — journal log shows the boundary.\n", t.ID, t.Label, err)

				// Truncate journal to successful reverts
				var newTxns []Txn
				for _, xt := range j.Txns {
					if xt.ID <= t.ID {
						newTxns = append(newTxns, xt)
					}
				}
				j.Txns = newTxns
				SaveJournal(&j)
				return 1
			}
			// Remove from journal
			j.Txns = j.Txns[:len(j.Txns)-1]
			fmt.Printf("undone: txn %d (%s)\n", t.ID, t.Label)
		}
		SaveJournal(&j)
		return 0

	case "mark":
		if len(args) >= 2 && (args[1] == "-h" || args[1] == "--help") {
			fmt.Println("usage: repoman ed mark [-h] name")
			fmt.Println()
			fmt.Println("positional arguments:")
			fmt.Println("  name")
			fmt.Println()
			fmt.Println("options:")
			fmt.Println("  -h, --help  show this help message and exit")
			return 0
		}
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "mark requires a name")
			return 1
		}
		j := LoadJournal()
		id := 0
		if len(j.Txns) > 0 {
			id = j.Txns[len(j.Txns)-1].ID
		}
		j.Marks[args[1]] = id
		SaveJournal(&j)
		fmt.Printf("mark %q at txn %d\n", args[1], id)
		return 0

	case "log":
		for _, a := range args[1:] {
			if a == "-h" || a == "--help" {
				fmt.Println("usage: repoman ed log [-h] [n]")
				fmt.Println()
				fmt.Println("positional arguments:")
				fmt.Println("  n")
				fmt.Println()
				fmt.Println("options:")
				fmt.Println("  -h, --help  show this help message and exit")
				return 0
			}
		}
		n := 20
		if len(args) > 1 {
			if parsed, err := strconv.Atoi(args[1]); err == nil {
				n = parsed
			}
		}
		j := LoadJournal()
		start := len(j.Txns) - n
		if start < 0 {
			start = 0
		}
		for _, t := range j.Txns[start:] {
			fmt.Printf("txn %d  %s  %s  (%d edit(s))\n", t.ID, t.At, t.Label, len(t.Edits))
		}
		if j.Evicted.Count > 0 {
			fmt.Printf("[%d evicted; anchors held for %d file(s)]\n", j.Evicted.Count, len(j.Evicted.Anchors))
		}
		return 0

	case "selftest":
		if len(args) >= 2 && (args[1] == "-h" || args[1] == "--help") {
			fmt.Println("usage: repoman ed selftest [-h]")
			fmt.Println()
			fmt.Println("options:")
			fmt.Println("  -h, --help  show this help message and exit")
			return 0
		}
		return SelfTest()

	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", cmd)
		return 1
	}
}
