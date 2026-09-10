package syncver

import (
	"os"
	"path/filepath"
	"regexp"

	"github.com/ha1tch/gorepoman/pkg/config"
)

// TargetSync is one configured version_targets entry's agreement (or
// disagreement) with the canonical VERSION file.
type TargetSync struct {
	File  string `json:"file"`
	Value string `json:"value"`
	Match bool   `json:"match"`
}

// SyncStatus is the structured shape of what `show`/`check` report --
// the same information CheckDetail already flattens into one message
// string, kept here as real per-target data so --format json/html
// don't have to re-parse that message to recover it.
type SyncStatus struct {
	Version string       `json:"version"`
	InSync  bool         `json:"in_sync"`
	Targets []TargetSync `json:"targets"`
}

// CheckDetailed walks the same targets CheckDetail does, but collects
// a full per-target breakdown instead of returning at the first
// mismatch -- CheckDetail's own behaviour (and every existing caller
// of it) is untouched; this is new code alongside it, not a
// replacement.
func CheckDetailed(root string, cfg *config.Config) SyncStatus {
	canon := GetVersion(root, cfg)
	status := SyncStatus{Version: canon, InSync: true, Targets: make([]TargetSync, 0, len(cfg.VersionTargets))}
	if canon == "" {
		status.InSync = false
		return status
	}

	for _, t := range cfg.VersionTargets {
		ts := TargetSync{File: t.File}
		b, err := os.ReadFile(filepath.Join(root, t.File))
		if err != nil {
			ts.Value = "<unreadable>"
			ts.Match = false
			status.InSync = false
			status.Targets = append(status.Targets, ts)
			continue
		}
		pat, err := regexp.Compile("(?m)" + t.Match)
		if err != nil {
			ts.Value = "<bad pattern>"
			ts.Match = false
			status.InSync = false
			status.Targets = append(status.Targets, ts)
			continue
		}
		m := pat.FindStringSubmatch(string(b))
		got := "<no match>"
		if len(m) > 1 {
			got = m[1]
		}
		ts.Value = got
		ts.Match = got == canon
		if !ts.Match {
			status.InSync = false
		}
		status.Targets = append(status.Targets, ts)
	}
	return status
}
