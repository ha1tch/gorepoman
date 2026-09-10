package register

import (
	"fmt"
	"sort"
	"strings"

	"github.com/ha1tch/gorepoman/pkg/config"
)

type ItemSummary struct {
	ID        string `json:"id"`
	Status    string `json:"status"`
	Priority  string `json:"priority"`
	Theme     string `json:"theme"`
	Title     string `json:"title"`
	Wave      string `json:"wave,omitempty"`
	FiledBy   string `json:"filed_by,omitempty"`
	Blocks    string `json:"blocks,omitempty"`
	ClaimedBy string `json:"claimed_by,omitempty"`
}

type ItemDetail struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Block string `json:"block"`
}

type CheckResult struct {
	Errors    []string `json:"errors"`
	OpenItems int      `json:"open_items"`
	OK        bool     `json:"ok"`
}

// sortedTIDs returns reg.Items' keys in the same numeric order
// cmdList already sorts by -- shared here so list's json/html paths
// can't drift from list's own text-path ordering.
func sortedTIDs(reg *Register, cfg *config.Config) []string {
	var tids []string
	for tid := range reg.Items {
		tids = append(tids, tid)
	}
	sort.SliceStable(tids, func(i, j int) bool {
		return idLess(tids[i], tids[j], cfg)
	})
	return tids
}

func toSummaries(reg *Register, cfg *config.Config) []ItemSummary {
	tids := sortedTIDs(reg, cfg)
	out := make([]ItemSummary, 0, len(tids))
	for _, tid := range tids {
		it := reg.Items[tid]
		out = append(out, ItemSummary{ID: tid, Status: it.Status, Priority: it.Priority, Theme: it.Theme, Title: it.Title, Wave: it.Wave, FiledBy: it.FiledBy, Blocks: it.Blocks, ClaimedBy: it.ClaimedBy})
	}
	return out
}

func toDetail(reg *Register, tid string) (ItemDetail, bool) {
	span, ok := reg.Spans[tid]
	if !ok {
		return ItemDetail{}, false
	}
	it := reg.Items[tid]
	return ItemDetail{
		ID:    tid,
		Title: it.Title,
		Block: strings.TrimRight(reg.Text[span[0]:span[1]], "\r\n"),
	}, true
}

// computeCheck mirrors the "check" case's own logic exactly (same
// A1/A2/A3 rules, same messages), but with deterministic, sorted
// iteration instead of Go's unordered map iteration -- the original
// text path's ERROR line ordering was never guaranteed either, so
// this doesn't change behaviour someone could have depended on, it
// just makes the new json/html paths reproducible where the old one
// happened not to need to be.
func computeCheck(reg *Register, cfg *config.Config) CheckResult {
	var errs []string

	var rowTIDs []string
	for tid := range reg.Rows {
		rowTIDs = append(rowTIDs, tid)
	}
	sort.Strings(rowTIDs)
	for _, tid := range rowTIDs {
		r := reg.Rows[tid]
		if strings.Contains(r.Raw, "✓") {
			errs = append(errs, fmt.Sprintf("[A1] closed item in register: %s", r.Raw))
		}
		if _, ok := reg.Items[tid]; !ok {
			errs = append(errs, fmt.Sprintf("[A2] row without detail block: %s", tid))
		}
	}

	var itemTIDs []string
	for tid := range reg.Items {
		itemTIDs = append(itemTIDs, tid)
	}
	sort.Strings(itemTIDs)
	for _, tid := range itemTIDs {
		it := reg.Items[tid]
		if r, ok := reg.Rows[tid]; !ok {
			errs = append(errs, fmt.Sprintf("[A2] detail block without row: %s", tid))
		} else if r.Theme != it.Theme || r.Priority != it.Priority || r.Status != it.Status {
			errs = append(errs, fmt.Sprintf("[A3] %s: table [%s %s %s] vs detail [%s %s %s]",
				tid, r.Theme, r.Priority, r.Status, it.Theme, it.Priority, it.Status))
		}
		if it.Status != "✓" && it.Status != "◐" && it.Status != "☐" && it.Status != "✗" && it.Status != "☑" {
			errs = append(errs, fmt.Sprintf("[A3] %s: unknown status symbol %q", tid, it.Status))
		}
	}

	if errs == nil {
		errs = []string{}
	}
	return CheckResult{Errors: errs, OpenItems: len(reg.Items), OK: len(errs) == 0}
}
