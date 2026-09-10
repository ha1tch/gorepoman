package waveprogress

import (
	"fmt"
	"os"

	"github.com/ha1tch/gorepoman/pkg/report"
)

// emitCheckResult handles --check's json/html output. Called only
// from inside the existing check branches in Run, never in place of
// them -- the text-format messages and exit codes those branches
// already produce are untouched.
func emitCheckResult(format string, data CheckResult) int {
	if err := ValidateCheck(data); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	var ec int
	switch format {
	case "json":
		ec = report.EmitOrErr(report.EmitJSON(os.Stdout, "waveprogress", "waveprogress-check", SchemaVersion, data))
	case "html":
		status := `<span style="color:var(--success);">up to date</span>`
		if data.Stale {
			status = `<span style="color:var(--warn-text);">stale</span>`
		}
		body := fmt.Sprintf("<p style=\"font-size:13px;\">%s</p>\n", status)
		ec = report.EmitOrErr(report.EmitHTML(os.Stdout, "waveprogress check", "waveprogress", "waveprogress-check", body))
	default:
		fmt.Fprintf(os.Stderr, "unknown format %q (want text, json, or html)\n", format)
		return 1
	}
	if ec == 0 && data.Stale {
		return 1
	}
	return ec
}

// WaveStatus is one wave's structured status -- the same numbers
// renderTable already formats into a line, kept here as real data
// instead of a pre-formatted string.
type WaveStatus struct {
	ID         string        `json:"id"`
	Name       string        `json:"name"`
	Pct        int           `json:"pct"`
	HasPartial bool          `json:"has_partial"`
	Done       float64       `json:"done"`
	Total      int           `json:"total"`
	Debt       []string      `json:"debt"`
	Blockers   []BlockerInfo `json:"blockers"`
}

type BlockerInfo struct {
	Blocker string   `json:"blocker"`
	Blocked []string `json:"blocked"`
}

// WaveReport is the structured shape of `--show`.
type WaveReport struct {
	Waves        []WaveStatus `json:"waves"`
	OverallDone  float64      `json:"overall_done"`
	OverallTotal int          `json:"overall_total"`
	OverallPct   int          `json:"overall_pct"`
	HiddenCount  int          `json:"hidden_count"`
}

// CheckResult is the structured shape of `--check`.
type CheckResult struct {
	Stale bool `json:"stale"`
}

// buildReport mirrors renderTable's and renderOverall's own
// computation exactly (same debt/blockers helpers, same rounding),
// but collects structured data instead of formatted lines. Neither
// renderTable nor renderOverall is touched -- this is new code
// alongside them, so --show's and the file-writing --html's existing
// text/ASCII output are provably unchanged.
func (e *env) buildReport(visibleWaves, allWaves []wave, fullText string) WaveReport {
	debt := e.debtByWave(fullText)
	blockers := e.blockersByWave(fullText, visibleWaves)

	statuses := make([]WaveStatus, 0, len(visibleWaves))
	for _, w := range visibleWaves {
		pct := 0.0
		if w.Total > 0 {
			pct = 100 * w.DoneEquiv / float64(w.Total)
		}

		wd := debt[w.ID]
		if wd == nil {
			wd = []string{}
		}

		var bis []BlockerInfo
		for _, be := range blockers[w.ID] {
			bis = append(bis, BlockerInfo{Blocker: be.Blocker, Blocked: be.Blocked})
		}
		if bis == nil {
			bis = []BlockerInfo{}
		}

		statuses = append(statuses, WaveStatus{
			ID:         w.ID,
			Name:       e.cfg.WaveShortNames[w.ID],
			Pct:        pyRound(pct),
			HasPartial: w.HasPartial,
			Done:       w.DoneEquiv,
			Total:      w.Total,
			Debt:       wd,
			Blockers:   bis,
		})
	}

	var totalDone float64
	var totalItems int
	for _, w := range allWaves {
		totalDone += w.DoneEquiv
		totalItems += w.Total
	}
	overallPct := 0
	if totalItems > 0 {
		overallPct = pyRound(100 * totalDone / float64(totalItems))
	}

	return WaveReport{
		Waves:        statuses,
		OverallDone:  totalDone,
		OverallTotal: totalItems,
		OverallPct:   overallPct,
		HiddenCount:  len(allWaves) - len(visibleWaves),
	}
}
