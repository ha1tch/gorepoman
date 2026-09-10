package ed

import (
	"os"
	"testing"
	"time"
)

// withTempCwd runs fn inside a fresh temporary directory, restoring
// the original working directory afterward. LoadTicketStore/
// SaveTicketStore resolve .ed-tickets.json relative to os.Getwd(),
// the same way journalPath does for .ed-journal.json, so isolating
// cwd per test is what keeps these tests from reading or writing each
// other's ticket stores when run together.
func withTempCwd(t *testing.T, fn func()) {
	t.Helper()
	dir := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("os.Chdir: %v", err)
	}
	defer os.Chdir(orig)
	fn()
}

func TestLoadTicketStore_FreshIsEmpty(t *testing.T) {
	withTempCwd(t, func() {
		s := LoadTicketStore()
		if len(s.Tickets) != 0 {
			t.Fatalf("expected zero tickets, got %d", len(s.Tickets))
		}
		if s.NextID != 1 {
			t.Fatalf("expected NextID 1, got %d", s.NextID)
		}
	})
}

func TestIssueTicket_DefaultsAndFieldsRecorded(t *testing.T) {
	withTempCwd(t, func() {
		s := LoadTicketStore()
		tk, err := IssueTicket(&s, "some/file.go", 10, 20, "prehash", "previewhash", 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if tk.ID != "nip-1" {
			t.Fatalf("expected id nip-1, got %s", tk.ID)
		}
		if tk.File != "some/file.go" || tk.StartLine != 10 || tk.EndLine != 20 {
			t.Fatalf("ticket fields not recorded correctly: %+v", tk)
		}
		if tk.PreNipHash != "prehash" || tk.PreviewHash != "previewhash" {
			t.Fatalf("ticket hashes not recorded correctly: %+v", tk)
		}
	})
}

func TestIssueTicket_PersistsAcrossReload(t *testing.T) {
	withTempCwd(t, func() {
		s := LoadTicketStore()
		tk, err := IssueTicket(&s, "f.go", 1, 2, "a", "b", 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		reloaded := LoadTicketStore()
		got, found := GetTicket(reloaded, tk.ID)
		if !found {
			t.Fatal("ticket not found after reload -- SaveTicketStore/LoadTicketStore did not round-trip")
		}
		if got.PreNipHash != "a" || got.PreviewHash != "b" {
			t.Fatalf("reloaded ticket hashes do not match what was issued: %+v", got)
		}
	})
}

func TestIssueTicket_IDsIncrementAndNeverReuse(t *testing.T) {
	withTempCwd(t, func() {
		s := LoadTicketStore()
		t1, _ := IssueTicket(&s, "f.go", 1, 2, "a", "b", 0)
		t2, _ := IssueTicket(&s, "f.go", 3, 4, "c", "d", 0)
		if t1.ID != "nip-1" || t2.ID != "nip-2" {
			t.Fatalf("expected nip-1 then nip-2, got %s then %s", t1.ID, t2.ID)
		}
		// Discard the first, issue a third -- the freed ID must not be
		// reused, since a stale reference to "nip-1" (e.g. in a
		// half-read log) must never later resolve to a different ticket.
		_ = DiscardTicket(&s, t1.ID)
		t3, _ := IssueTicket(&s, "f.go", 5, 6, "e", "f", 0)
		if t3.ID != "nip-3" {
			t.Fatalf("expected the freed nip-1 slot to NOT be reused; got %s", t3.ID)
		}
	})
}

func TestIssueTicket_RefusesInvalidLineRange(t *testing.T) {
	withTempCwd(t, func() {
		s := LoadTicketStore()
		if _, err := IssueTicket(&s, "f.go", 20, 10, "a", "b", 0); err == nil {
			t.Fatal("expected an error: end line before start line")
		}
		if _, err := IssueTicket(&s, "f.go", 0, 5, "a", "b", 0); err == nil {
			t.Fatal("expected an error: start line below 1")
		}
	})
}

func TestIssueTicket_RefusesMissingHashes(t *testing.T) {
	withTempCwd(t, func() {
		s := LoadTicketStore()
		if _, err := IssueTicket(&s, "f.go", 1, 2, "", "b", 0); err == nil {
			t.Fatal("expected an error: empty pre-nip hash")
		}
		if _, err := IssueTicket(&s, "f.go", 1, 2, "a", "", 0); err == nil {
			t.Fatal("expected an error: empty preview hash")
		}
	})
}

func TestIssueTicket_TTLCeiling(t *testing.T) {
	withTempCwd(t, func() {
		s := LoadTicketStore()
		if _, err := IssueTicket(&s, "f.go", 1, 2, "a", "b", MaxTicketTTL+time.Minute); err == nil {
			t.Fatal("expected an error: ttl above MaxTicketTTL must be refused, not silently clamped")
		}
		if _, err := IssueTicket(&s, "f.go", 1, 2, "a", "b", MaxTicketTTL); err != nil {
			t.Fatalf("ttl exactly at the ceiling should be allowed, got: %v", err)
		}
	})
}

func TestTicket_IsExpired(t *testing.T) {
	withTempCwd(t, func() {
		s := LoadTicketStore()
		short, _ := IssueTicket(&s, "f.go", 1, 2, "a", "b", 1*time.Millisecond)
		time.Sleep(5 * time.Millisecond)
		if !short.IsExpired(time.Now().UTC()) {
			t.Fatal("a ticket past its TTL should report IsExpired true")
		}

		long, _ := IssueTicket(&s, "f.go", 1, 2, "a", "b", DefaultTicketTTL)
		if long.IsExpired(time.Now().UTC()) {
			t.Fatal("a freshly issued default-TTL ticket should not be expired")
		}
	})
}

func TestPruneExpired_RemovesOnlyExpired(t *testing.T) {
	withTempCwd(t, func() {
		s := LoadTicketStore()
		expiring, _ := IssueTicket(&s, "exp.go", 1, 2, "a", "b", 1*time.Millisecond)
		surviving, _ := IssueTicket(&s, "long.go", 1, 2, "a", "b", DefaultTicketTTL)
		time.Sleep(5 * time.Millisecond)

		pruned := PruneExpired(&s, time.Now().UTC())
		if pruned != 1 {
			t.Fatalf("expected exactly 1 pruned, got %d", pruned)
		}
		if _, found := GetTicket(s, expiring.ID); found {
			t.Fatal("the expired ticket should be gone after pruning")
		}
		if _, found := GetTicket(s, surviving.ID); !found {
			t.Fatal("the non-expired ticket should survive pruning")
		}
	})
}

func TestDiscardTicket_RemovesAndPersists(t *testing.T) {
	withTempCwd(t, func() {
		s := LoadTicketStore()
		tk, _ := IssueTicket(&s, "f.go", 1, 2, "a", "b", 0)

		if err := DiscardTicket(&s, tk.ID); err != nil {
			t.Fatalf("unexpected error discarding: %v", err)
		}
		reloaded := LoadTicketStore()
		if _, found := GetTicket(reloaded, tk.ID); found {
			t.Fatal("discarded ticket should be gone after reload -- discard did not persist")
		}
	})
}

func TestDiscardTicket_UnknownIDIsNoop(t *testing.T) {
	withTempCwd(t, func() {
		s := LoadTicketStore()
		if err := DiscardTicket(&s, "nip-9999"); err != nil {
			t.Fatalf("discarding an unknown id should be a no-op, got error: %v", err)
		}
	})
}

func TestHashContent_Deterministic(t *testing.T) {
	a := HashContent("same content\n")
	b := HashContent("same content\n")
	c := HashContent("different content\n")
	if a != b {
		t.Fatal("HashContent should be deterministic for identical input")
	}
	if a == c {
		t.Fatal("HashContent should differ for different input")
	}
}
