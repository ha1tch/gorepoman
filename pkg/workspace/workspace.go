package workspace

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ha1tch/gorepoman/pkg/config"
	"github.com/ha1tch/gorepoman/pkg/report"
	"github.com/ha1tch/gorepoman/pkg/webhelp"
)

const participantsFile = "participants.json"

const help = `usage: repoman workspace {join,leave,list,newissue,resolveissue,pauseissue,dropissue} [-h] [--format text|json|html]

join <name> <remote> [--name PROJECT] [--credential-env VAR]
    Join a cross-project workspace. Writes to two repos with two
    different credentials: this project's own .repoman.json (its own
    credential, i.e. however this session already writes to this
    repo), and the workspace's own participants.json (whatever
    credential reaches that remote -- an ordinary git clone/push,
    nothing gorepoman-specific; read access to a workspace is free,
    write access is whatever GitHub permissions already grant).
    --name overrides the project name (default: this directory's own
    basename, same convention repoman board already uses).
    --credential-env records which environment variable names the
    secret for future writes to this workspace -- never the secret
    itself, provisioned separately per machine, same as badcode.

leave <name>
    Mirror of join: removes this project from both .repoman.json and
    the workspace's participants.json (including any default_boards
    entry naming this project).

list
    Read-only. Which workspaces this project has joined, from its own
    .repoman.json. Takes --format like every other reporting
    subcommand.

newissue <workspace> <target> <theme> <title> --body-file PATH
          [--parent ID] [--name FILER]
    File an issue against another project in the workspace. <target>
    is a label only, not validated against the target's own repo.
    The filer defaults to the project name recorded at join time for
    this workspace; --name overrides it. Issue ids are
    <filer>:<prefix>-N and are never reused, even across resolved or
    dropped issues.

resolveissue <workspace> <ref> --resolved-as ID --version V
             [--note TEXT]
    Close an issue as done. Moves it to an append-only resolved log
    (never edited in place); --resolved-as names the filer's own
    register id that actually closed it, --version the version it
    shipped in.

pauseissue <workspace> <ref> --reason TEXT
    The one non-terminal event: the issue stays open and filed, just
    not being actively pursued. Does not move or delete the issue.

dropissue <workspace> <ref> --reason TEXT
    Terminal, like resolveissue, but for an issue that will not be
    resolved. Moved to an append-only dropped log, never silently
    deleted. --reason is mandatory: dropping must leave as clear a
    trace as closing.

See repoman-086-workspace.md for worked examples of the full
join/newissue/resolveissue lifecycle.
`

// gitIn runs a git subcommand with cwd set to dir.
func gitIn(dir string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return out, fmt.Errorf("git %v: %w: %s", args, err, string(out))
	}
	return out, nil
}

func loadParticipants(dir string) (Participants, error) {
	b, err := os.ReadFile(filepath.Join(dir, participantsFile))
	if err != nil {
		return Participants{}, err
	}
	var p Participants
	if err := json.Unmarshal(b, &p); err != nil {
		return Participants{}, err
	}
	return p, nil
}

func saveParticipants(dir string, p Participants) error {
	if err := ValidateParticipants(p); err != nil {
		return err
	}
	b, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, participantsFile), append(b, '\n'), 0644)
}

func cloneWorkspace(remote string) (string, error) {
	tmp, err := os.MkdirTemp("", "repoman-workspace-*")
	if err != nil {
		return "", err
	}
	if _, err := gitIn("", "clone", remote, tmp); err != nil {
		os.RemoveAll(tmp)
		return "", err
	}
	return tmp, nil
}

func commitAndPush(dir, message string) error {
	if _, err := gitIn(dir, "add", participantsFile); err != nil {
		return err
	}
	if _, err := gitIn(dir, "commit", "-m", message); err != nil {
		return err
	}
	if _, err := gitIn(dir, "push"); err != nil {
		return err
	}
	return nil
}

func contains(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}

func without(ss []string, s string) []string {
	out := make([]string, 0, len(ss))
	for _, x := range ss {
		if x != s {
			out = append(out, x)
		}
	}
	return out
}

func runJoin(args []string) int {
	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: repoman workspace join <name> <remote> [--name PROJECT] [--credential-env VAR]")
		return 1
	}
	name, remote := args[0], args[1]
	projectName := ""
	credentialEnv := ""
	for i := 2; i < len(args); i++ {
		switch args[i] {
		case "--name":
			if i+1 < len(args) {
				projectName = args[i+1]
				i++
			}
		case "--credential-env":
			if i+1 < len(args) {
				credentialEnv = args[i+1]
				i++
			}
		}
	}
	if projectName == "" {
		wd, err := os.Getwd()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		projectName = filepath.Base(wd)
	}

	// Local membership check first -- cheap, no network, and the
	// right thing to refuse on before doing any remote work at all.
	root, cfg, err := config.Load("")
	if err != nil {
		fmt.Fprintln(os.Stderr, "error loading config:", err)
		return 1
	}
	for _, w := range cfg.Workspaces {
		if w.Name == name {
			fmt.Fprintf(os.Stderr, "already a member of workspace %q\n", name)
			return 1
		}
	}

	// B-14 fix: the remote side is validated and updated FIRST, and
	// local .repoman.json is only written once that has genuinely
	// succeeded. Previously the local write happened before any of
	// this, so a failure here (no participants.json yet on the
	// remote, a clone failure, a push failure) left a stale
	// workspace entry in .repoman.json with no rollback -- a retry
	// then refused with "already a member", and the only way out was
	// hand-editing the config. None of cloneWorkspace/loadParticipants/
	// saveParticipants/commitAndPush depend on local state, so
	// reordering changes nothing about the success path.
	tmp, err := cloneWorkspace(remote)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error cloning workspace:", err)
		return 1
	}
	defer os.RemoveAll(tmp)

	p, err := loadParticipants(tmp)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error reading workspace participants.json:", err)
		return 1
	}
	alreadyParticipant := contains(p.Projects, projectName)
	if !alreadyParticipant {
		p.Projects = append(p.Projects, projectName)
		if err := saveParticipants(tmp, p); err != nil {
			fmt.Fprintln(os.Stderr, "error writing workspace participants.json:", err)
			return 1
		}
		if err := commitAndPush(tmp, fmt.Sprintf("workspace: %s joins", projectName)); err != nil {
			fmt.Fprintln(os.Stderr, "error pushing to workspace:", err)
			fmt.Fprintln(os.Stderr, "nothing local was changed -- fix the problem above and retry")
			return 1
		}
	}

	// Remote side confirmed (either just joined, or already listed
	// as a participant) -- now, and only now, record membership
	// locally.
	cfg.Workspaces = append(cfg.Workspaces, config.Workspace{Name: name, Remote: remote, CredentialEnv: credentialEnv, ProjectName: projectName})
	if err := config.SaveKey(root, "workspaces", cfg.Workspaces); err != nil {
		fmt.Fprintln(os.Stderr, "error saving local workspace membership:", err)
		fmt.Fprintln(os.Stderr, "the remote side already lists this project as a participant -- fix the problem above and retry; retrying will not double-join the remote")
		return 1
	}

	if alreadyParticipant {
		fmt.Printf("workspace %q already lists %q as a participant; local membership recorded\n", name, projectName)
	} else {
		fmt.Printf("joined workspace %q as %q\n", name, projectName)
	}
	return 0
}

func runLeave(args []string) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: repoman workspace leave <name>")
		return 1
	}
	name := args[0]

	root, cfg, err := config.Load("")
	if err != nil {
		fmt.Fprintln(os.Stderr, "error loading config:", err)
		return 1
	}
	var found *config.Workspace
	var remaining []config.Workspace
	for _, w := range cfg.Workspaces {
		if w.Name == name {
			wCopy := w
			found = &wCopy
			continue
		}
		remaining = append(remaining, w)
	}
	if found == nil {
		fmt.Fprintf(os.Stderr, "not a member of workspace %q\n", name)
		return 1
	}
	if remaining == nil {
		remaining = []config.Workspace{}
	}
	if err := config.SaveKey(root, "workspaces", remaining); err != nil {
		fmt.Fprintln(os.Stderr, "error saving local workspace membership:", err)
		return 1
	}

	wd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	projectName := filepath.Base(wd)

	tmp, err := cloneWorkspace(found.Remote)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error cloning workspace:", err)
		return 1
	}
	defer os.RemoveAll(tmp)

	p, err := loadParticipants(tmp)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error reading workspace participants.json:", err)
		return 1
	}
	p.Projects = without(p.Projects, projectName)
	if p.DefaultBoards != nil {
		delete(p.DefaultBoards, projectName)
	}
	if err := saveParticipants(tmp, p); err != nil {
		fmt.Fprintln(os.Stderr, "error writing workspace participants.json:", err)
		return 1
	}
	if err := commitAndPush(tmp, fmt.Sprintf("workspace: %s leaves", projectName)); err != nil {
		fmt.Fprintln(os.Stderr, "error pushing to workspace:", err)
		fmt.Fprintln(os.Stderr, "local .repoman.json was already updated -- rejoin and retry, or push the workspace side by hand")
		return 1
	}

	fmt.Printf("left workspace %q\n", name)
	return 0
}

func runList(format string) int {
	_, cfg, err := config.Load("")
	if err != nil {
		fmt.Fprintln(os.Stderr, "error loading config:", err)
		return 1
	}
	switch format {
	case "text":
		if len(cfg.Workspaces) == 0 {
			fmt.Println("not a member of any workspace")
			return 0
		}
		for _, w := range cfg.Workspaces {
			fmt.Printf("%-20s %s\n", w.Name, w.Remote)
		}
		return 0
	case "json":
		data := cfg.Workspaces
		return report.EmitOrErr(report.EmitJSON(os.Stdout, "workspace", "workspace-list", 1, data))
	case "html":
		var body string
		for _, w := range cfg.Workspaces {
			body += fmt.Sprintf(`<div style="padding:4px 0;border-bottom:1px solid var(--border);font-size:13px;"><strong>%s</strong> <span style="color:var(--text-2);">%s</span></div>`, w.Name, w.Remote)
		}
		return report.EmitOrErr(report.EmitHTML(os.Stdout, "workspaces", "workspace", "workspace-list", body))
	default:
		fmt.Fprintf(os.Stderr, "unknown format %q (want text, json, or html)\n", format)
		return 1
	}
}

// Run implements `repoman workspace <join|leave|list> ...`.
func Run(args []string) int {
	args = webhelp.NormalizeBriefFirst(args)
	format, args := report.ExtractFormat(args)
	for _, a := range args {
		if a == "-h" || a == "--help" {
			fmt.Print(help)
			fmt.Println(webhelp.SuppressionNote)
			webhelp.PrintIfAvailable(os.Stdout, "repoman-086-workspace", args)
			return 0
		}
	}
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: repoman workspace <join|leave|list> ...")
		return 1
	}
	switch args[0] {
	case "join":
		return runJoin(args[1:])
	case "leave":
		return runLeave(args[1:])
	case "list":
		return runList(format)
	case "newissue":
		return runNewIssue(args[1:])
	case "resolveissue":
		return runResolveIssue(args[1:])
	case "pauseissue":
		return runPauseIssue(args[1:])
	case "dropissue":
		return runDropIssue(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown workspace command: %s\n", args[0])
		return 1
	}
}

// resolveWorkspaceRemote looks up a joined workspace by name in this
// project own .repoman.json, returning its remote so the issue
// commands can reuse it without asking for it again on every call.
func resolveWorkspaceRemote(name string) (string, error) {
	remote, _, err := resolveWorkspaceInfo(name)
	return remote, err
}

// resolveWorkspaceInfo also returns the project name stored at join
// time for this specific workspace -- newissue defaults to this
// rather than independently re-deriving from the current directory,
// so a project cannot register as one name and file issues under a
// different one by accident.
func resolveWorkspaceInfo(name string) (remote, projectName string, err error) {
	_, cfg, err := config.Load("")
	if err != nil {
		return "", "", err
	}
	for _, w := range cfg.Workspaces {
		if w.Name == name {
			return w.Remote, w.ProjectName, nil
		}
	}
	return "", "", fmt.Errorf("not a member of workspace %q -- repoman workspace join first", name)
}

func filerName(override string) (string, error) {
	if override != "" {
		return override, nil
	}
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return filepath.Base(wd), nil
}

// nextIssueNumber derives the next id from every number this filer
// has ever used -- open issues (issues/<filer>/), plus the resolved
// and dropped logs, not the open directory alone. An earlier version
// only scanned the open directory: a resolved or dropped issue file
// is deleted from there, which made its number look available again
// and a second issue could have collided with a real, closed one --
// the exact "IDs are never reused" violation the register itself is
// built to prevent. Caught by testing the full resolve-then-file-
// another sequence, not by inspection.
// refNumber extracts N from a ref shaped like filer:prefix-N, but
// only when its filer and prefix actually match the ones asked for --
// returns ok=false otherwise so a caller never mistakes an unrelated
// project own ref for one of its own.
func refNumber(ref, filer, prefix string) (int, bool) {
	parts := strings.SplitN(ref, ":", 2)
	if len(parts) != 2 || parts[0] != filer {
		return 0, false
	}
	idParts := strings.SplitN(parts[1], "-", 2)
	if len(idParts) != 2 || idParts[0] != prefix {
		return 0, false
	}
	n, err := strconv.Atoi(idParts[1])
	if err != nil {
		return 0, false
	}
	return n, true
}

// nextIssueNumber derives the next id from every number this filer
// has ever used -- open issues (issues/<filer>/), plus the resolved
// and dropped logs, not the open directory alone. Two real bugs
// happened here before this version, both caught by testing rather
// than inspection: an earlier version only scanned the open
// directory, so a resolved or dropped issue own number (its file
// deleted) looked available again -- the exact IDs-are-never-reused
// violation the register itself is built to prevent. The fix for
// that then shipped with a regex matching the wrong JSON field name
// ("id" instead of "ref", the field Closure/Drop actually use), so it
// silently matched nothing and did not actually fix anything -- caught
// by re-running the same scenario on an independent, fresh fixture
// rather than trusting the first passing test. This version parses
// each log line as real JSON rather than pattern-matching the raw
// text, closing that whole class of wrong-field-name risk rather than
// trying to get a second regex right where the first one was not.
func nextIssueNumber(wsDir, filer, prefix string) int {
	max := 0

	dir := filepath.Join(wsDir, "issues", filer)
	if entries, err := os.ReadDir(dir); err == nil {
		openRe := regexp.MustCompile(`^` + regexp.QuoteMeta(prefix) + `-(\d+)\.json$`)
		for _, e := range entries {
			if m := openRe.FindStringSubmatch(e.Name()); m != nil {
				n, _ := strconv.Atoi(m[1])
				if n > max {
					max = n
				}
			}
		}
	}

	scanLog := func(path string, getRef func([]byte) (string, bool)) {
		b, err := os.ReadFile(path)
		if err != nil {
			return
		}
		for _, line := range strings.Split(string(b), "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			ref, ok := getRef([]byte(line))
			if !ok {
				continue
			}
			if n, ok := refNumber(ref, filer, prefix); ok && n > max {
				max = n
			}
		}
	}
	scanLog(filepath.Join(wsDir, "issues", "resolved.jsonl"), func(b []byte) (string, bool) {
		var c Closure
		if err := json.Unmarshal(b, &c); err != nil {
			return "", false
		}
		return c.Ref, c.Ref != ""
	})
	scanLog(filepath.Join(wsDir, "issues", "dropped.jsonl"), func(b []byte) (string, bool) {
		var d Drop
		if err := json.Unmarshal(b, &d); err != nil {
			return "", false
		}
		return d.Ref, d.Ref != ""
	})

	return max + 1
}

func appendJSONL(path string, v interface{}) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(append(b, byte(10)))
	return err
}

func runNewIssue(args []string) int {
	if len(args) < 4 {
		fmt.Fprintln(os.Stderr, "Usage: repoman workspace newissue <workspace> <target> <theme> <title> --body-file PATH [--parent ID] [--name FILER]")
		return 1
	}
	wsName, target, theme, title := args[0], args[1], args[2], args[3]
	bodyFile, parent, filerOverride := "", "", ""
	for i := 4; i < len(args); i++ {
		switch args[i] {
		case "--body-file":
			if i+1 < len(args) {
				bodyFile = args[i+1]
				i++
			}
		case "--parent":
			if i+1 < len(args) {
				parent = args[i+1]
				i++
			}
		case "--name":
			if i+1 < len(args) {
				filerOverride = args[i+1]
				i++
			}
		}
	}
	if bodyFile == "" {
		fmt.Fprintln(os.Stderr, "--body-file is required")
		return 1
	}
	body, err := os.ReadFile(bodyFile)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error reading body file:", err)
		return 1
	}

	remote, joinedAs, err := resolveWorkspaceInfo(wsName)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	filer, err := filerName(filerOverride)
	if filerOverride == "" && joinedAs != "" {
		filer, err = joinedAs, nil
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	_, cfg, err := config.Load("")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	prefix := cfg.IDPrefix
	if prefix == "" {
		prefix = "T"
	}

	tmp, err := cloneWorkspace(remote)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error cloning workspace:", err)
		return 1
	}
	defer os.RemoveAll(tmp)

	n := nextIssueNumber(tmp, filer, prefix)
	localID := fmt.Sprintf("%s-%d", prefix, n)
	fullID := fmt.Sprintf("%s:%s", filer, localID)

	issue := Issue{
		Schema: "gorepoman.workspace.issue/1", ID: fullID, Target: target,
		Parent: parent, Theme: theme, Title: title,
		Filed: time.Now().UTC().Format("2006-01-02"), Body: string(body),
	}
	if err := ValidateIssue(issue); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}

	dir := filepath.Join(tmp, "issues", filer)
	if err := os.MkdirAll(dir, 0755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	b, _ := json.MarshalIndent(issue, "", "  ")
	relPath := filepath.Join("issues", filer, localID+".json")
	if err := os.WriteFile(filepath.Join(tmp, relPath), append(b, (byte)(10)), 0644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if _, err := gitIn(tmp, "add", relPath); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if _, err := gitIn(tmp, "commit", "-m", fmt.Sprintf("issue: file %s against %s", fullID, target)); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if _, err := gitIn(tmp, "push"); err != nil {
		fmt.Fprintln(os.Stderr, "error pushing:", err)
		return 1
	}
	fmt.Printf("filed %s against %s\n", fullID, target)
	return 0
}

// findOpenIssue locates ref (a full <filer>:<id> reference) somewhere
// under issues/*/ in the cloned workspace, returning its relative
// path and decoded contents.
func findOpenIssue(wsDir, ref string) (string, Issue, error) {
	parts := strings.SplitN(ref, ":", 2)
	if len(parts) != 2 {
		return "", Issue{}, fmt.Errorf("ref %q is not in filer:id form", ref)
	}
	relPath := filepath.Join("issues", parts[0], parts[1]+".json")
	b, err := os.ReadFile(filepath.Join(wsDir, relPath))
	if err != nil {
		return "", Issue{}, fmt.Errorf("no open issue found at %s: %w", relPath, err)
	}
	var issue Issue
	if err := json.Unmarshal(b, &issue); err != nil {
		return "", Issue{}, err
	}
	return relPath, issue, nil
}

func runResolveIssue(args []string) int {
	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: repoman workspace resolveissue <workspace> <ref> --resolved-as ID --version V [--note TEXT]")
		return 1
	}
	wsName, ref := args[0], args[1]
	resolvedAs, version, note := "", "", ""
	for i := 2; i < len(args); i++ {
		switch args[i] {
		case "--resolved-as":
			if i+1 < len(args) {
				resolvedAs = args[i+1]
				i++
			}
		case "--version":
			if i+1 < len(args) {
				version = args[i+1]
				i++
			}
		case "--note":
			if i+1 < len(args) {
				note = args[i+1]
				i++
			}
		}
	}
	if resolvedAs == "" || version == "" {
		fmt.Fprintln(os.Stderr, "--resolved-as and --version are both required")
		return 1
	}

	remote, err := resolveWorkspaceRemote(wsName)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	tmp, err := cloneWorkspace(remote)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error cloning workspace:", err)
		return 1
	}
	defer os.RemoveAll(tmp)

	relPath, _, err := findOpenIssue(tmp, ref)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	closure := Closure{
		Schema: "gorepoman.workspace.closure/1", Ref: ref, ResolvedAs: resolvedAs,
		Version: version, Date: time.Now().UTC().Format("2006-01-02"), Note: note,
	}
	if err := ValidateClosure(closure); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	if err := appendJSONL(filepath.Join(tmp, "issues", "resolved.jsonl"), closure); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if err := os.Remove(filepath.Join(tmp, relPath)); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if _, err := gitIn(tmp, "add", "-A"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if _, err := gitIn(tmp, "commit", "-m", fmt.Sprintf("issue: resolve %s as %s", ref, resolvedAs)); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if _, err := gitIn(tmp, "push"); err != nil {
		fmt.Fprintln(os.Stderr, "error pushing:", err)
		return 1
	}
	fmt.Printf("resolved %s as %s\n", ref, resolvedAs)
	return 0
}

func runPauseIssue(args []string) int {
	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: repoman workspace pauseissue <workspace> <ref> --reason TEXT")
		return 1
	}
	wsName, ref := args[0], args[1]
	reason := ""
	for i := 2; i < len(args); i++ {
		if args[i] == "--reason" && i+1 < len(args) {
			reason = args[i+1]
			i++
		}
	}
	if reason == "" {
		fmt.Fprintln(os.Stderr, "--reason is required")
		return 1
	}

	remote, err := resolveWorkspaceRemote(wsName)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	tmp, err := cloneWorkspace(remote)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error cloning workspace:", err)
		return 1
	}
	defer os.RemoveAll(tmp)

	if _, _, err := findOpenIssue(tmp, ref); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	pause := Pause{Schema: "gorepoman.workspace.pause/1", Ref: ref, Date: time.Now().UTC().Format("2006-01-02"), Reason: reason}
	if err := ValidatePause(pause); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	if err := appendJSONL(filepath.Join(tmp, "issues", "paused.jsonl"), pause); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if _, err := gitIn(tmp, "add", "issues/paused.jsonl"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if _, err := gitIn(tmp, "commit", "-m", fmt.Sprintf("issue: pause %s", ref)); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if _, err := gitIn(tmp, "push"); err != nil {
		fmt.Fprintln(os.Stderr, "error pushing:", err)
		return 1
	}
	fmt.Printf("paused %s (still open, not deleted or moved)\n", ref)
	return 0
}

func runDropIssue(args []string) int {
	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: repoman workspace dropissue <workspace> <ref> --reason TEXT")
		return 1
	}
	wsName, ref := args[0], args[1]
	reason := ""
	for i := 2; i < len(args); i++ {
		if args[i] == "--reason" && i+1 < len(args) {
			reason = args[i+1]
			i++
		}
	}
	if reason == "" {
		fmt.Fprintln(os.Stderr, "--reason is required -- dropping an issue must leave as clear a trace as closing one")
		return 1
	}

	remote, err := resolveWorkspaceRemote(wsName)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	tmp, err := cloneWorkspace(remote)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error cloning workspace:", err)
		return 1
	}
	defer os.RemoveAll(tmp)

	relPath, _, err := findOpenIssue(tmp, ref)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	drop := Drop{Schema: "gorepoman.workspace.drop/1", Ref: ref, Date: time.Now().UTC().Format("2006-01-02"), Reason: reason}
	if err := ValidateDrop(drop); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	if err := appendJSONL(filepath.Join(tmp, "issues", "dropped.jsonl"), drop); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if err := os.Remove(filepath.Join(tmp, relPath)); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if _, err := gitIn(tmp, "add", "-A"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if _, err := gitIn(tmp, "commit", "-m", fmt.Sprintf("issue: drop %s (%s)", ref, reason)); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if _, err := gitIn(tmp, "push"); err != nil {
		fmt.Fprintln(os.Stderr, "error pushing:", err)
		return 1
	}
	fmt.Printf("dropped %s: %s (moved to issues/dropped.jsonl, not deleted)\n", ref, reason)
	return 0
}
