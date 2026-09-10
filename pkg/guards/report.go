package guards

import "strings"

// GuardSummary is one guard's list-view shape. LastExercised is empty
// when the guard has never been exercised -- text/html render that as
// "NEVER", json leaves it as an empty string rather than a sentinel a
// consumer would have to know to check for.
type GuardSummary struct {
	ID            string `json:"id"`
	Title         string `json:"title"`
	LastExercised string `json:"last_exercised"`
}

// GuardDetail is one guard's full registry block -- the same text
// cmdShow already prints, alongside its id/title so a consumer doesn't
// have to re-parse the block to recover them.
type GuardDetail struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Block string `json:"block"`
}

// StaleResult is the structured shape of `stale`.
type StaleResult struct {
	Since      string         `json:"since"`
	Stale      []GuardSummary `json:"stale"`
	AllCurrent bool           `json:"all_current"`
}

func (e *env) toSummaries(gs []guard) []GuardSummary {
	out := make([]GuardSummary, 0, len(gs))
	for _, g := range gs {
		out = append(out, GuardSummary{
			ID:            g.gid,
			Title:         g.title,
			LastExercised: lastDateOf(lastLineOf(g.block, e.lastRe)),
		})
	}
	return out
}

func (e *env) toDetail(g guard) GuardDetail {
	return GuardDetail{ID: g.gid, Title: g.title, Block: strings.TrimRight(g.block, " \t\r\n")}
}

// computeStale mirrors cmdStale's own logic exactly, but collects
// results instead of printing them. cmdStale itself is untouched.
func (e *env) computeStale(gs []guard, since string) StaleResult {
	var stale []GuardSummary
	for _, g := range gs {
		lastLine := lastLineOf(g.block, e.lastRe)
		if lastLine == "" {
			continue
		}
		ld := lastDateOf(lastLine)
		if ld == "" || ld < since {
			stale = append(stale, GuardSummary{ID: g.gid, Title: g.title, LastExercised: ld})
		}
	}
	if stale == nil {
		stale = []GuardSummary{}
	}
	return StaleResult{Since: since, Stale: stale, AllCurrent: len(stale) == 0}
}
