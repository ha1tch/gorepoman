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
	"fmt"
	"os"
	"path/filepath"
)

// VersionTarget is one extra file that must carry the version; Match
// is a regex with exactly one capture group.
type VersionTarget struct {
	File  string `json:"file"`
	Match string `json:"match"`
}

// IDNamespace is one additional, independently-numbered id prefix a
// register can recognise alongside the primary IDPrefix/IDSeparator --
// e.g. a project running "T-nn" for general debt and "BF-nn" for
// bugfixes side by side, both permanently live. Each entry gets its
// own independent NextID counter, never sharing a sequence with the
// primary namespace or any other entry (unlike LegacyIDPrefix, a
// one-time migration aid where old and new shapes share one counter).
type IDNamespace struct {
	Prefix    string `json:"prefix"`
	Separator string `json:"separator"`
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

	// IDNamespaces: additional id prefixes recognised alongside the
	// primary IDPrefix/IDSeparator, each with its own independent
	// NextID counter -- for a project running two (or more) id shapes
	// permanently side by side (e.g. "T-nn" for general debt and
	// "BF-nn" for bugfixes, both live indefinitely), as distinct from
	// LegacyIDPrefix above, which is for a one-time migration where the
	// old shape is retired in favour of the new one and the two share a
	// single counter. Empty (the default) is byte-identical to
	// behaviour before this key existed.
	IDNamespaces []IDNamespace `json:"id_namespaces"`

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

	// WaveCompleteWord: FR-03. The word waveStatusWord's B-04 fix
	// writes into a wave's own summary line ("**Wave N: k/n,
	// <word>.**") once that wave reaches 100%. Hardcoded to "done"
	// before this key existed, with no override -- a project whose own
	// established convention uses a different word (xolu: "complete")
	// had every existing summary line silently rewritten the first
	// time it ran waveprogress on an upgraded binary, a one-time,
	// unavoidable terminology migration bundled into the upgrade.
	// Empty string (the default) means "done", byte-identical to
	// behaviour before this key existed -- only a project that sets
	// this explicitly sees anything different.
	WaveCompleteWord string `json:"wave_complete_word"`

	Release Release `json:"release"`

	// Workspaces: which cross-project workspace(s) this project belongs
	// to. Absent (nil/empty) means no workspace membership at all --
	// additive default, no behaviour change for a project that never
	// touches this. CredentialEnv only ever names an environment
	// variable; the real secret is provisioned separately per machine,
	// same principle as badcode's own config -- never write a live
	// credential into this committed file.
	Workspaces []Workspace `json:"workspaces"`
}

// Workspace is one cross-project coordination repo this project has
// joined, per repoman workspace join.
type Workspace struct {
	Name          string `json:"name"`
	Remote        string `json:"remote"`
	CredentialEnv string `json:"credential_env"`
	ProjectName   string `json:"project_name"`
}

// EffectiveIDNamespace pairs one recognised id prefix/separator with
// the counter group NextID must use for it: GroupKey is "" for the
// primary namespace and, when configured, the legacy namespace folded
// into it (so a legacy id and a primary id are never allocated the
// same number twice -- this preserves the historical single-counter
// migration behaviour byte-for-byte). Every other namespace gets its
// own independent group, keyed by its own prefix.
type EffectiveIDNamespace struct {
	Prefix    string
	Separator string
	GroupKey  string
}

// EffectiveIDNamespaces returns every id prefix this register
// recognises: the primary namespace, the legacy namespace if
// configured (folded into the primary namespace counter group), then
// each entry from IDNamespaces in order (each its own independent
// counter group). A namespace whose prefix duplicates one already
// present is skipped -- first occurrence wins, primary first, then
// legacy, then IDNamespaces in order -- so a config that
// (redundantly) repeats the primary prefix inside IDNamespaces never
// creates a second pattern alternative or a phantom second counter
// group for it.
func (c *Config) EffectiveIDNamespaces() []EffectiveIDNamespace {
	seen := map[string]bool{}
	var out []EffectiveIDNamespace

	add := func(prefix, sep, group string) {
		if prefix == "" || seen[prefix] {
			return
		}
		seen[prefix] = true
		out = append(out, EffectiveIDNamespace{Prefix: prefix, Separator: sep, GroupKey: group})
	}

	add(c.IDPrefix, c.IDSeparator, "")
	if c.LegacyIDPrefix != "" {
		add(c.LegacyIDPrefix, c.LegacyIDSeparator, "")
	}
	for _, ns := range c.IDNamespaces {
		add(ns.Prefix, ns.Separator, ns.Prefix)
	}
	return out
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
		IDNamespaces:      []IDNamespace{},
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
		WaveCompleteWord:  "", // empty means "done" -- see the field's own doc comment
		Release: Release{
			Steps:   []map[string]interface{}{},
			Archive: map[string]interface{}{},
		},
		Workspaces: []Workspace{},
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
	// B-06 fix: a version_targets entry with the wrong field name (the
	// shape is only documented in repoman-090, not in --help; "pattern"
	// is an easy mistake for "match") used to be accepted silently --
	// Match stayed at its zero value, and syncver reported "<no match>"
	// for the file, which reads as a content mismatch rather than what
	// it actually is, a config error. A file entry with no regex to
	// match against is never valid, so it's rejected here explicitly.
	for i, vt := range cfg.VersionTargets {
		if vt.File != "" && vt.Match == "" {
			return "", Config{}, fmt.Errorf(
				"version_targets[%d]: %q has no \"match\" field (or it's empty) -- "+
					"each entry needs {\"file\": ..., \"match\": \"regex with one capture group\"}; "+
					"check for a typo like \"pattern\" instead of \"match\"", i, vt.File)
		}
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
