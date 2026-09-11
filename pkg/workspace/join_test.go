package workspace

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// These are B-14's regression tests: `workspace join` used to write
// the local `.repoman.json` workspace entry BEFORE validating
// anything on the remote side, with no rollback on failure. A failed
// join (e.g. the remote has no participants.json yet) left a stale
// entry behind; a retry then refused with "already a member of
// workspace", and the only way out was hand-editing the config --
// see the gorepoman-bugs memory file for the original repro against
// a real local bare-repo workspace. The fix reorders runJoin so the
// remote side is validated and updated first, and the local write
// only happens once that has genuinely succeeded.
//
// All of this runs against real local git repos (a bare "remote" plus
// a real clone) rather than mocks, matching how the bug was first
// found -- these tests would have failed against the pre-fix runJoin.

// requireGit skips the test if git isn't on PATH, rather than failing
// the whole suite in an environment that doesn't have it.
func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
}

// runGit runs a git command and fails the test on error, with stderr
// included in the failure message.
func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// newBareRemote creates a bare repo at dir/remote.git, suitable as a
// clone target, and returns its path.
func newBareRemote(t *testing.T, dir string) string {
	t.Helper()
	remote := filepath.Join(dir, "remote.git")
	if err := os.MkdirAll(remote, 0755); err != nil {
		t.Fatalf("mkdir remote: %v", err)
	}
	runGit(t, remote, "init", "--bare")
	return remote
}

// newProjectDir creates a real (non-bare) git repo to act as the
// local project that's joining a workspace -- config.Load's FindRoot
// needs a real .git to anchor on when there's no .repoman.json yet.
func newProjectDir(t *testing.T, dir string) string {
	t.Helper()
	proj := filepath.Join(dir, "myproject")
	if err := os.MkdirAll(proj, 0755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}
	runGit(t, proj, "init")
	return proj
}

// withProjectCwd runs fn with the working directory set to proj,
// restoring the original cwd afterward.
func withProjectCwd(t *testing.T, proj string, fn func()) {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd: %v", err)
	}
	if err := os.Chdir(proj); err != nil {
		t.Fatalf("os.Chdir: %v", err)
	}
	defer os.Chdir(orig)
	fn()
}

// readWorkspaces reads .repoman.json's "workspaces" key directly,
// bypassing config.Load's defaults-merging so a completely absent key
// is distinguishable from an empty list.
func readWorkspaces(t *testing.T, proj string) []map[string]interface{} {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(proj, ".repoman.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatalf("read .repoman.json: %v", err)
	}
	var doc map[string]interface{}
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatalf("unmarshal .repoman.json: %v", err)
	}
	raw, ok := doc["workspaces"]
	if !ok {
		return nil
	}
	var out []map[string]interface{}
	rb, _ := json.Marshal(raw)
	if err := json.Unmarshal(rb, &out); err != nil {
		t.Fatalf("unmarshal workspaces: %v", err)
	}
	return out
}

// TestJoin_FailedRemoteValidation_LeavesNoLocalEntry is the direct
// B-14 repro: joining a remote with no participants.json yet must
// fail cleanly, AND must not leave a stale entry in .repoman.json
// behind. Before the fix, this left exactly the entry a retry would
// then trip over.
func TestJoin_FailedRemoteValidation_LeavesNoLocalEntry(t *testing.T) {
	requireGit(t)
	dir := t.TempDir()
	remote := newBareRemote(t, dir) // deliberately never gets a participants.json
	proj := newProjectDir(t, dir)

	withProjectCwd(t, proj, func() {
		rc := runJoin([]string{"testws", remote})
		if rc == 0 {
			t.Fatal("expected join against a remote with no participants.json to fail, got exit 0")
		}
	})

	ws := readWorkspaces(t, proj)
	if len(ws) != 0 {
		t.Fatalf("expected no local workspace entry after a failed join, got %v", ws)
	}
}

// TestJoin_FailedRemoteValidation_RetryAfterFixSucceeds confirms the
// actual end-to-end repair: after a failed join (no rollback needed,
// per the test above) fixing the remote and retrying the SAME join
// call succeeds outright -- no more "already a member" refusal, no
// hand-editing .repoman.json.
func TestJoin_FailedRemoteValidation_RetryAfterFixSucceeds(t *testing.T) {
	requireGit(t)
	dir := t.TempDir()
	remote := newBareRemote(t, dir)
	proj := newProjectDir(t, dir)

	withProjectCwd(t, proj, func() {
		if rc := runJoin([]string{"testws", remote}); rc == 0 {
			t.Fatal("expected the first join (empty remote) to fail")
		}
	})

	// Fix the remote: seed it with a real participants.json commit,
	// exactly what a workspace admin would do.
	seed := filepath.Join(dir, "seed")
	runGit(t, dir, "clone", remote, seed)
	seedParticipants := `{"schema":"gorepoman.workspace.participants/1","workspace":"testws","projects":[]}` + "\n"
	if err := os.WriteFile(filepath.Join(seed, participantsFile), []byte(seedParticipants), 0644); err != nil {
		t.Fatalf("write seed participants.json: %v", err)
	}
	runGit(t, seed, "add", participantsFile)
	runGit(t, seed, "commit", "-m", "seed participants.json")
	runGit(t, seed, "push")

	withProjectCwd(t, proj, func() {
		rc := runJoin([]string{"testws", remote})
		if rc != 0 {
			t.Fatal("expected the retried join to succeed now that the remote has participants.json -- got a non-zero exit, meaning the earlier failure left something behind")
		}
	})

	ws := readWorkspaces(t, proj)
	if len(ws) != 1 {
		t.Fatalf("expected exactly one local workspace entry after the successful retry, got %v", ws)
	}
	if ws[0]["name"] != "testws" {
		t.Fatalf("expected workspace name %q, got %v", "testws", ws[0]["name"])
	}
}

// TestJoin_Success_RecordsLocalMembership is the ordinary happy path:
// a clean join against a properly seeded remote succeeds in one call
// and records local membership.
func TestJoin_Success_RecordsLocalMembership(t *testing.T) {
	requireGit(t)
	dir := t.TempDir()
	remote := newBareRemote(t, dir)
	proj := newProjectDir(t, dir)

	seed := filepath.Join(dir, "seed")
	runGit(t, dir, "clone", remote, seed)
	seedParticipants := `{"schema":"gorepoman.workspace.participants/1","workspace":"testws","projects":[]}` + "\n"
	if err := os.WriteFile(filepath.Join(seed, participantsFile), []byte(seedParticipants), 0644); err != nil {
		t.Fatalf("write seed participants.json: %v", err)
	}
	runGit(t, seed, "add", participantsFile)
	runGit(t, seed, "commit", "-m", "seed participants.json")
	runGit(t, seed, "push")

	withProjectCwd(t, proj, func() {
		rc := runJoin([]string{"testws", remote, "--name", "myproject"})
		if rc != 0 {
			t.Fatal("expected a clean join to succeed")
		}
	})

	ws := readWorkspaces(t, proj)
	if len(ws) != 1 || ws[0]["name"] != "testws" || ws[0]["project_name"] != "myproject" {
		t.Fatalf("unexpected local workspace state: %v", ws)
	}
}

// TestJoin_AlreadyParticipant_StillRecordsLocalMembership covers the
// case the reordering also fixed as a side effect: previously, if the
// remote already listed this project as a participant, runJoin
// printed a message and returned 0 WITHOUT ever writing local
// .repoman.json -- a successful-looking join that recorded nothing
// locally. Now local membership is recorded either way.
func TestJoin_AlreadyParticipant_StillRecordsLocalMembership(t *testing.T) {
	requireGit(t)
	dir := t.TempDir()
	remote := newBareRemote(t, dir)
	proj := newProjectDir(t, dir)

	// Seed the remote with THIS project already listed.
	seed := filepath.Join(dir, "seed")
	runGit(t, dir, "clone", remote, seed)
	seedParticipants := `{"schema":"gorepoman.workspace.participants/1","workspace":"testws","projects":["myproject"]}` + "\n"
	if err := os.WriteFile(filepath.Join(seed, participantsFile), []byte(seedParticipants), 0644); err != nil {
		t.Fatalf("write seed participants.json: %v", err)
	}
	runGit(t, seed, "add", participantsFile)
	runGit(t, seed, "commit", "-m", "seed participants.json with myproject already present")
	runGit(t, seed, "push")

	withProjectCwd(t, proj, func() {
		rc := runJoin([]string{"testws", remote, "--name", "myproject"})
		if rc != 0 {
			t.Fatal("expected join to succeed when the remote already lists this project")
		}
	})

	ws := readWorkspaces(t, proj)
	if len(ws) != 1 || ws[0]["name"] != "testws" {
		t.Fatalf("expected local membership to be recorded even though the remote already listed this project, got: %v", ws)
	}
}
