package relcore

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/ha1tch/gorepoman/pkg/badcode"
	"github.com/ha1tch/gorepoman/pkg/config"
	"github.com/ha1tch/gorepoman/pkg/syncver"
)

var (
	contaminationRe = regexp.MustCompile(`\.bak$|\.db$|-wal$|-shm$|-journal$|\.pprof$|\.prof$|\.test$|\.DS_Store$|/\._|__MACOSX|Thumbs\.db$`)
	magics          = []struct {
		m []byte
		k string
	}{
		{[]byte("\x7fELF"), "ELF"},
		{[]byte("\xca\xfe\xba\xbe"), "Mach-O"},
		{[]byte("\xfe\xed\xfa\xce"), "Mach-O"},
		{[]byte("\xfe\xed\xfa\xcf"), "Mach-O"},
		{[]byte("MZ"), "PE"},
	}
)

type Journal struct {
	Path    string
	Version string
	Data    map[string]map[string]interface{}
}

func loadJournal(root, version string) *Journal {
	j := &Journal{
		Path:    filepath.Join(root, ".release-state.json"),
		Version: version,
		Data:    make(map[string]map[string]interface{}),
	}
	b, err := os.ReadFile(j.Path)
	if err == nil {
		var doc struct {
			Version string                            `json:"version"`
			Steps   map[string]map[string]interface{} `json:"steps"`
		}
		if json.Unmarshal(b, &doc) == nil && doc.Version == version {
			if doc.Steps != nil {
				j.Data = doc.Steps
			}
		}
	}
	return j
}

func (j *Journal) record(step, status string, meta map[string]interface{}) {
	if j.Data[step] == nil {
		j.Data[step] = make(map[string]interface{})
	}
	j.Data[step]["status"] = status
	j.Data[step]["at"] = time.Now().UTC().Format("2006-01-02T15:04:05Z")
	for k, v := range meta {
		j.Data[step][k] = v
	}

	doc := map[string]interface{}{
		"version": j.Version,
		"steps":   j.Data,
	}
	b, _ := json.MarshalIndent(doc, "", " ")
	tmp := j.Path + ".tmp"
	os.WriteFile(tmp, b, 0644)
	os.Rename(tmp, j.Path)
}

func (j *Journal) green(step string) bool {
	if data, ok := j.Data[step]; ok {
		if status, ok := data["status"].(string); ok {
			return status == "ok"
		}
	}
	return false
}

func say(msg string) {
	fmt.Println(msg)
}

func runCmd(cmdStr string, logFile *os.File, root string, timeout int) int {
	logFile.WriteString(fmt.Sprintf("\n$ %s\n", cmdStr))
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", cmdStr)
	cmd.Dir = root
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			logFile.WriteString(fmt.Sprintf("\n[relcore] TIMEOUT after %ds\n", timeout))
			return 124
		}
		if exitError, ok := err.(*exec.ExitError); ok {
			return exitError.ExitCode()
		}
		return 1
	}
	return 0
}

func stepSyncver(root string, cfg *config.Config, version string) (map[string]interface{}, error) {
	err := syncver.SetVersion(version, root, cfg)
	if err != nil {
		return nil, fmt.Errorf("version sync failed: %v", err)
	}
	ok, detail := syncver.CheckDetail(root, cfg)
	if !ok {
		return nil, fmt.Errorf("version sync failed: %s", detail)
	}
	return map[string]interface{}{"version": version}, nil
}

func matchExclude(rel string, patterns []string) bool {
	base := filepath.Base(rel)
	for _, p := range patterns {
		if matched, _ := filepath.Match(p, base); matched {
			return true
		}
		if matched, _ := filepath.Match(p, rel); matched {
			return true
		}
	}
	return false
}

func stepArchive(root string, cfg *config.Config, version string) (map[string]interface{}, error) {
	a := cfg.Release.Archive
	sourcesIf, ok := a["sources"].([]interface{})
	if !ok || len(sourcesIf) == 0 {
		return nil, fmt.Errorf("archive builtin requires release.archive.sources")
	}

	repoName := filepath.Base(root)
	nameFmt := "{repo}-v{version}-checkpoint.zip"
	if n, ok := a["name"].(string); ok {
		nameFmt = n
	}
	zipNameStr := strings.Replace(strings.Replace(nameFmt, "{repo}", repoName, -1), "{version}", version, -1)
	zipPath := filepath.Join(root, zipNameStr)
	os.Remove(zipPath)

	var exclude []string
	if exIf, ok := a["exclude"].([]interface{}); ok {
		for _, e := range exIf {
			if s, ok := e.(string); ok {
				exclude = append(exclude, s)
			}
		}
	}
	exclude = append(exclude, ".release-state.json", "release-*.log", ".ed-journal.json", "MANIFEST.sha256", zipNameStr)

	zf, err := os.Create(zipPath)
	if err != nil {
		return nil, err
	}
	zw := zip.NewWriter(zf)

	var manifest []string
	count := 0

	for _, srcIf := range sourcesIf {
		src := srcIf.(string)
		path := filepath.Join(root, src)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			zw.Close()
			zf.Close()
			return nil, fmt.Errorf("archive source missing: %s", src)
		}

		filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			rel, _ := filepath.Rel(root, p)
			if matchExclude(rel, exclude) {
				return nil
			}

			b, err := os.ReadFile(p)
			if err != nil {
				return err
			}

			w, err := zw.Create(filepath.ToSlash(rel))
			if err != nil {
				return err
			}
			w.Write(b)

			h := sha256.Sum256(b)
			manifest = append(manifest, fmt.Sprintf("%x  %s", h, filepath.ToSlash(rel)))
			count++
			return nil
		})
	}

	mw, _ := zw.Create("MANIFEST.sha256")
	mw.Write([]byte(strings.Join(manifest, "\n") + "\n"))
	zw.Close()
	zf.Close()

	// Verify archive
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, err
	}
	defer zr.Close()

	var dirty []string
	var bad []string

	for _, f := range zr.File {
		if f.Name == "MANIFEST.sha256" || strings.HasSuffix(f.Name, "/") {
			continue
		}
		if contaminationRe.MatchString(f.Name) {
			dirty = append(dirty, f.Name)
		}

		rc, err := f.Open()
		if err == nil {
			head := make([]byte, 4)
			n, _ := io.ReadFull(rc, head)
			rc.Close()
			for _, m := range magics {
				if n >= len(m.m) && string(head[:len(m.m)]) == string(m.m) {
					bad = append(bad, fmt.Sprintf("%s (%s)", m.k, f.Name))
				}
			}
		}
	}

	if len(dirty) > 0 {
		return nil, fmt.Errorf("archive contains artifacts: %v", dirty[:min(5, len(dirty))])
	}
	if len(bad) > 0 {
		return nil, fmt.Errorf("archive contains binaries: %v", bad[:min(5, len(bad))])
	}

	info, _ := os.Stat(zipPath)
	size := info.Size()
	warnMB := int64(3)
	if wIf, ok := a["size_warn_mb"].(float64); ok {
		warnMB = int64(wIf)
	}
	if size > warnMB*1024*1024 {
		say(fmt.Sprintf("   !! archive is %.1f MB — exceeds the %d MB ceiling", float64(size)/1048576.0, warnMB))
	}

	return map[string]interface{}{"files": count, "bytes": size, "zip": zipNameStr}, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// runBadcodePreflight is the mandatory, unconditional gate described
// at its call site in Run: not a release.steps entry, not resumable,
// not skippable, not journaled through the skip-if-green path any
// other step can use. It scans the entire repository root (the
// broadest reasonable scope for "must never reach a release,
// full stop" -- not just whatever release.archive.sources happens to
// list, since content that never went into the archive could still
// have been committed and pushed). No patterns configured is a soft
// pass (LoadPatterns/Check already implement that -- see pkg/badcode),
// which this function does not override; a real match is a hard,
// unrecoverable failure with no bypass of any kind.
func runBadcodePreflight(root, version string) int {
	say("-- badcode (mandatory pre-flight, not skippable)")

	patterns, dir, err := badcode.LoadPatterns()
	if err != nil {
		say(fmt.Sprintf("   FAIL badcode: %v", err))
		return 1
	}

	journal := loadJournal(root, version)

	if len(patterns) == 0 {
		say(fmt.Sprintf("   WARN no badcode patterns configured in %s -- nothing checked", dir))
		journal.record("__badcode_preflight__", "ok", map[string]interface{}{"patterns": 0})
		say("   ok badcode (0 patterns configured)")
		return 0
	}

	matches := badcode.Check([]string{root}, patterns)
	if len(matches) > 0 {
		for _, m := range matches {
			reason := ""
			if m.Pattern.Reason != "" {
				reason = fmt.Sprintf(" (%s)", m.Pattern.Reason)
			}
			say(fmt.Sprintf("   ERROR badcode-match: pattern %q%s found in %s:%d: %s",
				m.Pattern.Text, reason, m.File, m.Line, m.Snippet))
		}
		journal.record("__badcode_preflight__", "fail", map[string]interface{}{"matches": len(matches)})
		say(fmt.Sprintf("   FAIL badcode: %d match(es) -- this gate has no override; remove the "+
			"matched content and re-run", len(matches)))
		return 1
	}

	journal.record("__badcode_preflight__", "ok", map[string]interface{}{"patterns": len(patterns)})
	say(fmt.Sprintf("   ok badcode (%d pattern(s) checked)", len(patterns)))
	return 0
}

func Run(args []string) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: repoman relcore <version> [--resume]")
		return 1
	}
	if args[0] == "-h" || args[0] == "--help" {
		fmt.Println("usage: repoman relcore [-h] [--resume] version")
		fmt.Println()
		fmt.Println("manifest-driven release")
		fmt.Println()
		fmt.Println("positional arguments:")
		fmt.Println("  version")
		fmt.Println()
		fmt.Println("options:")
		fmt.Println("  -h, --help  show this help message and exit")
		fmt.Println("  --resume")
		fmt.Println()
		fmt.Println("Runs `badcode check` unconditionally first, before any release.steps")
		fmt.Println("entry, including on --resume. See")
		fmt.Println("https://ha1tch.github.io/gorepoman/docs/repoman-070-releases.html")
		fmt.Println("for the full release workflow, and repoman-065-badcode.html for")
		fmt.Println("that gate specifically.")
		return 0
	}

	version := args[0]
	resume := false
	for _, a := range args[1:] {
		if a == "--resume" {
			resume = true
		}
	}

	root, cfg, err := config.Load("")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	// badcode: mandatory pre-flight, first thing that runs, before
	// anything else -- deliberately NOT a release.steps entry in
	// .repoman.json. If it were expressible in that manifest, it
	// could be removed from that manifest by anyone with repo access,
	// which defeats the entire point. It is not resumable, not
	// journaled through the skip-if-green path any other step can use
	// (journal.green is never consulted for it), and has no flag, env
	// var, or config key anywhere in this codebase that disables it.
	// The only lever that exists is the local badcode config itself --
	// see pkg/badcode's own doc comment for why that is local and
	// never repo-committed.
	if rc := runBadcodePreflight(root, version); rc != 0 {
		return rc
	}

	if len(cfg.Release.Steps) == 0 {
		say("no release.steps in .repoman.json — nothing to orchestrate")
		return 1
	}

	journal := loadJournal(root, version)
	logPath := filepath.Join(root, fmt.Sprintf("release-%s.log", version))
	logFile, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer logFile.Close()

	resuming := ""
	if resume {
		resuming = ", resuming"
	}
	say(fmt.Sprintf("relcore %s at %s (log: release-%s.log%s)", version, filepath.Base(root), version, resuming))

	t0 := time.Now()
	for _, s := range cfg.Release.Steps {
		name, _ := s["name"].(string)
		always, _ := s["always"].(bool)
		resumable, _ := s["resumable"].(bool)

		if resume && resumable && !always && journal.green(name) {
			say(fmt.Sprintf("-- %s: journaled green, skipped", name))
			continue
		}
		say(fmt.Sprintf("-- %s", name))

		t := time.Now()
		var meta map[string]interface{}
		var stepErr error

		if builtin, ok := s["builtin"].(string); ok {
			if builtin == "syncver" {
				meta, stepErr = stepSyncver(root, &cfg, version)
			} else if builtin == "archive" {
				meta, stepErr = stepArchive(root, &cfg, version)
			} else {
				stepErr = fmt.Errorf("unknown builtin step: %q", builtin)
			}
			if meta == nil {
				meta = make(map[string]interface{})
			}
		} else {
			runStr, _ := s["run"].(string)
			timeout := 600
			if tIf, ok := s["timeout"].(float64); ok {
				timeout = int(tIf)
			}
			rc := runCmd(runStr, logFile, root, timeout)
			if rc != 0 {
				stepErr = fmt.Errorf("exit %d; see the release log", rc)
			}
			meta = make(map[string]interface{})
		}

		if stepErr != nil {
			journal.record(name, "fail", map[string]interface{}{"error": stepErr.Error()})
			say(fmt.Sprintf("   FAIL %s: %v", name, stepErr))
			say("   run halted; fix and re-run with --resume")
			return 1
		}

		meta["seconds"] = time.Since(t).Seconds()
		journal.record(name, "ok", meta)
		say(fmt.Sprintf("   ok %s (%.0fs)", name, time.Since(t).Seconds()))
	}

	say(fmt.Sprintf("\nrelease v%s prepared (%.0fs)", version, time.Since(t0).Seconds()))
	return 0
}
