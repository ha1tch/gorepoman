// Package config implements repository root discovery and
// configuration -- translated from repoman/config.py.
//
// A repository opts in by carrying `.repoman.json` at its root (an
// empty object is a valid opt-in: every key has a default). Root
// discovery walks upward from the current directory for
// `.repoman.json`, then for `.git`, else uses the current directory.
//
// Defaults encode a documented set of repository conventions
// (tracking register, resolution record, known-issues/dormant-guards
// documents, plain VERSION file); any of them can be overridden per
// repository.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// VersionTarget is one extra file that must carry the version; Match
// is a regex with exactly one capture group.
type VersionTarget struct {
	File  string `json:"file"`
	Match string `json:"match"`
}

// Release holds the release-manifest schema; see relcore.py / relcore.go
// for the step and archive schemas. Steps and Archive are left as
// loosely-typed JSON (map[string]interface{}) because their shape is
// defined by the manifest author, not by this package.
type Release struct {
	Steps   []map[string]interface{} `json:"steps"`
	Archive map[string]interface{}   `json:"archive"`
}

// Config mirrors config.py's DEFAULTS dict, field for field.
type Config struct {
	IDPrefix    string `json:"id_prefix"`
	IDSeparator string `json:"id_separator"`

	// LegacyIDPrefix / LegacyIDSeparator: for a project that migrated
	// id shape mid-project (a real xolu need, not hypothetical --
	// T-1..T-163 permanently frozen in "T-NNN" shape, T-164 onward
	// forward-only in a new "XOTNNN" shape). Empty LegacyIDPrefix (the
	// default) means single-format behaviour, byte-identical to before
	// this key existed -- these two keys are additive and change
	// nothing for a consumer that never sets them.
	LegacyIDPrefix    string `json:"legacy_id_prefix"`
	LegacyIDSeparator string `json:"legacy_id_separator"`

	Tracking    string `json:"tracking"`
	Resolved    string `json:"resolved"`
	KnownIssues string `json:"known_issues"`

	// GuardIDPrefix: full prefix (including separator) for dormant-
	// guard ids, e.g. "G-" -> "G-13". A single string is sufficient
	// generality here -- guards have no mid-project migration need the
	// way register ids sometimes do.
	GuardIDPrefix string `json:"guard_id_prefix"`

	Changelog      string          `json:"changelog"`
	VersionFile    string          `json:"version_file"`
	VersionTargets []VersionTarget `json:"version_targets"`

	// Staged-work ("wave") tracking -- optional feature, config keys
	// exist with safe empty/generic defaults so a consumer that never
	// touches waves sees no behaviour change. WaveShortNames is
	// auto-maintained by add_wave.py / addwave (never hand-edited);
	// WaveThemes is hand-curated when used at all -- a theme-to-wave
	// mapping is a judgement call about which open debt genuinely
	// belongs to a wave's own subject matter, not mechanically
	// derivable, so an empty default (no debt cross-referencing) is
	// the correct behaviour for a consumer that hasn't made those
	// calls yet.
	WaveTracking   string              `json:"wave_tracking"`
	WavePlan       string              `json:"wave_plan"`
	WaveShortNames map[string]string   `json:"wave_short_names"`
	WaveThemes     map[string][]string `json:"wave_themes"`

	// WaveVisibility: wave_id -> visible. Absent = visible (default),
	// matching the additive-default rule -- a consumer that never sets
	// this sees every wave, same as before this key existed. Persisted
	// DATA, not a rendering concern: both the ASCII and HTML renderers
	// read the same map, so visibility state cannot drift between the
	// two display forms the way it would if each carried its own
	// separate notion of it.
	WaveVisibility map[string]bool `json:"wave_visibility"`

	// WaveHTMLTitle: heading text for waveprogress --html output.
	// Cosmetic only; default is generic on purpose.
	WaveHTMLTitle string `json:"wave_html_title"`

	Release Release `json:"release"`
}

// Defaults returns a fresh copy of the default configuration. A fresh
// value is returned on every call (rather than a shared package-level
// var) so that a caller merging user overrides on top of it can never
// accidentally mutate a shared default.
func Defaults() Config {
	return Config{
		IDPrefix:          "T",
		IDSeparator:       "-",
		LegacyIDPrefix:    "",
		LegacyIDSeparator: "-",
		Tracking:          "docs/TRACKING.md",
		Resolved:          "docs/RESOLVED.md",
		KnownIssues:       "docs/KNOWN_ISSUES.md",
		GuardIDPrefix:     "G-",
		Changelog:         "CHANGELOG.md",
		VersionFile:       "VERSION",
		VersionTargets:    []VersionTarget{},
		WaveTracking:      "docs/WAVE_TRACKING.md",
		WavePlan:          "docs/WAVE_PLAN.md",
		WaveShortNames:    map[string]string{},
		WaveThemes:        map[string][]string{},
		WaveVisibility:    map[string]bool{},
		WaveHTMLTitle:     "wave progress",
		Release: Release{
			Steps:   []map[string]interface{}{},
			Archive: map[string]interface{}{},
		},
	}
}

// FindRoot walks upward from start (the current directory if start is
// empty) looking first for `.repoman.json`, then for `.git`; if
// neither is found anywhere up to the filesystem root, start itself
// (resolved to an absolute path) is returned.
func FindRoot(start string) (string, error) {
	p := start
	if p == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		p = cwd
	}
	p, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}

	for _, candidate := range ancestors(p) {
		if fi, err := os.Stat(filepath.Join(candidate, ".repoman.json")); err == nil && !fi.IsDir() {
			return candidate, nil
		}
	}
	for _, candidate := range ancestors(p) {
		if _, err := os.Stat(filepath.Join(candidate, ".git")); err == nil {
			return candidate, nil
		}
	}
	return p, nil
}

// ancestors returns p followed by each of its parent directories, up
// to and including the filesystem root.
func ancestors(p string) []string {
	out := []string{p}
	for {
		parent := filepath.Dir(p)
		if parent == p {
			break
		}
		out = append(out, parent)
		p = parent
	}
	return out
}

// Load discovers the repository root (see FindRoot) and returns it
// together with its configuration: defaults with any keys present in
// `.repoman.json` overlaid on top. Map-valued keys (WaveShortNames,
// WaveThemes, WaveVisibility) are merged key-by-key onto the defaults
// rather than replaced wholesale -- this relies on encoding/json's own
// documented behaviour of merging into a non-nil destination map,
// which reproduces config.py's `cfg[k].update(v)` behaviour for free.
// Non-map keys (including Release, whose Steps/Archive sub-fields are
// merged the same way by the same mechanism) are simply overwritten
// when present in the file, exactly as in config.py.
func Load(start string) (string, Config, error) {
	root, err := FindRoot(start)
	if err != nil {
		return "", Config{}, err
	}

	cfg := Defaults()

	f := filepath.Join(root, ".repoman.json")
	b, err := os.ReadFile(f)
	if err != nil {
		if os.IsNotExist(err) {
			return root, cfg, nil
		}
		return "", Config{}, err
	}

	if err := json.Unmarshal(b, &cfg); err != nil {
		return "", Config{}, err
	}
	return root, cfg, nil
}

// SaveKey persists a single top-level key's value into
// `.repoman.json`, creating the file if absent. It reads and writes
// only the file's own on-disk JSON (never the defaults-merged view
// Load returns) -- a consumer who has never customised
// `.repoman.json` keeps a minimal file after this runs, not a full
// dump of every default. Used by addwave for wave_short_names, the
// one piece of wave state this package ever writes on its own rather
// than leaving to a human.
func SaveKey(root, key string, value interface{}) error {
	f := filepath.Join(root, ".repoman.json")

	doc := map[string]interface{}{}
	if b, err := os.ReadFile(f); err == nil {
		if err := json.Unmarshal(b, &doc); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	doc[key] = value

	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	out = append(out, '\n')
	return os.WriteFile(f, out, 0644)
}
