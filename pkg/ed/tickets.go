package ed

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// T-22: pending-ticket storage for the ticketed niplines two-phase flow
// (docs/proposals/ed-insert-and-ticketed-niplines.md, Proposal 2). A
// line-range delete has no content anchor the way find/apply's
// SpanHash does, so niplines (T-23/T-24, not implemented here) is
// designed as request-then-confirm: phase 1 issues a Ticket recording
// what would change and previews it without writing anything; phase 2
// (confirm) redeems that same ticket, re-verifying the file hasn't
// drifted, or cancel discards it. This file is only the storage
// primitive both phases share -- it does not implement niplines,
// confirm, or cancel itself.
//
// Tickets deliberately live in their own sibling file
// (.ed-tickets.json), not inside Journal/.ed-journal.json, per the
// proposal's own open question and this project's resolution of it:
// SaveJournal's MaxTxns/MaxBytes eviction walks j.Txns by count and
// serialized size to decide what to age out, and a pending, unconfirmed
// ticket is not a completed edit -- it must never be evictable the same
// way a txn is, and folding it into Journal would force the eviction
// loop to special-case ticket entries for no benefit. A confirmed
// ticket becomes a normal Record/Txn exactly like any other edit, at
// which point it is removed from the ticket store entirely; it never
// transitions into journal state in place.

// Ticket is one pending niplines request: a proposed deletion of
// [StartLine, EndLine] (1-based, inclusive) from File, not yet
// written. Both hashes matter for confirm's re-verification --
// PreNipHash proves the live file has not drifted since the request
// (the same class of check apply's SpanHash performs for a targeted
// substitution), and PreviewHash proves confirm is redeeming the
// exact preview the caller was shown, not a same-range request issued
// fresh with different intervening content. IssuedAt/ExpiresAt bound
// how long a ticket can sit unconfirmed; see TTL guidance in the
// proposal doc -- this package enforces the ceiling (MaxTicketTTL) at
// creation time but leaves the caller (T-23) to choose the actual TTL
// within it.
type Ticket struct {
	ID          string `json:"id"`
	File        string `json:"file"`
	StartLine   int    `json:"start_line"`
	EndLine     int    `json:"end_line"`
	PreNipHash  string `json:"pre_nip_hash"`
	PreviewHash string `json:"preview_hash"`
	IssuedAt    string `json:"issued_at"`
	ExpiresAt   string `json:"expires_at"`
}

// TicketStore is the on-disk shape of .ed-tickets.json: pending
// tickets keyed by ID, plus a monotonic counter so IDs are never
// reused within a project even after a ticket is confirmed, cancelled,
// or pruned. NextID starts at 1 the same way Journal's txn IDs do.
type TicketStore struct {
	Tickets map[string]Ticket `json:"tickets"`
	NextID  int               `json:"next_id"`
}

const (
	// DefaultTicketTTL is phase 1's default when the caller (T-23)
	// does not pass an explicit --ttl: long enough for a gofmt/vet/
	// build preflight to run without racing the clock, short enough
	// that a forgotten ticket does not linger as an unexplained
	// pending mutation. See the proposal doc's TTL table.
	DefaultTicketTTL = 10 * time.Minute

	// MaxTicketTTL is the hard ceiling T-23's --ttl override may not
	// exceed, sized for a human actually reading the diff rather than
	// an agent's same-turn preflight. A nip that needs longer belongs
	// in TRACKING.md as its own item, re-issued as a fresh ticket when
	// someone is ready -- not a longer-lived ticket competing with the
	// register as a second, informal tracking surface.
	MaxTicketTTL = 1 * time.Hour
)

func ticketStorePath() string {
	cwd, _ := os.Getwd()
	return filepath.Join(cwd, ".ed-tickets.json")
}

// LoadTicketStore reads .ed-tickets.json, returning an empty,
// ready-to-use store if the file does not exist yet or fails to
// parse -- mirroring LoadJournal's own tolerant-of-absence behavior,
// since "no tickets file yet" is the normal state for every project
// before its first niplines request.
func LoadTicketStore() TicketStore {
	s := TicketStore{
		Tickets: make(map[string]Ticket),
		NextID:  1,
	}
	b, err := os.ReadFile(ticketStorePath())
	if err == nil {
		_ = json.Unmarshal(b, &s)
	}
	if s.Tickets == nil {
		s.Tickets = make(map[string]Ticket)
	}
	if s.NextID < 1 {
		s.NextID = 1
	}
	return s
}

// SaveTicketStore writes s atomically (tmp file + rename), the same
// pattern SaveJournal uses, so a crash or interruption mid-write can
// never leave .ed-tickets.json truncated or half-written.
func SaveTicketStore(s *TicketStore) error {
	tmp := ticketStorePath() + ".tmp"
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(tmp, b, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, ticketStorePath())
}

// hashBytes is the same sha256-hex-digest shape Journal's own
// provenance hashing uses (recordProvenance, CheckProvenance,
// Sanction), reused here so a ticket's PreNipHash/PreviewHash are
// directly comparable with FileProvenance.Hash if a future caller
// ever wants to cross-check them -- not required by T-22 itself, but
// free consistency worth keeping.
func hashBytes(b []byte) string {
	h := sha256.Sum256(b)
	return fmt.Sprintf("%x", h)
}

// HashContent exposes hashBytes for callers (T-23's niplines request
// step) that need to hash a pre-nip file or a computed preview
// string using the exact same digest shape this store records and
// IssueTicket/RedeemTicket compare against.
func HashContent(content string) string {
	return hashBytes([]byte(content))
}

// IssueTicket records a new pending ticket for file/startLine/endLine,
// with the given pre-nip and preview hashes (computed by the caller --
// T-23's request/preview step, not this function, since only the
// caller knows how the post-nip preview text was rendered) and a TTL.
// ttl is clamped into (0, MaxTicketTTL]: zero or negative falls back
// to DefaultTicketTTL, and anything above MaxTicketTTL is refused
// outright rather than silently clamped down -- a caller asking for
// more than the ceiling has misunderstood the TTL policy and should
// see that, not get a silently shorter ticket than requested.
func IssueTicket(s *TicketStore, file string, startLine, endLine int, preNipHash, previewHash string, ttl time.Duration) (Ticket, error) {
	if startLine < 1 || endLine < startLine {
		return Ticket{}, fmt.Errorf("REFUSED: invalid line range %d-%d", startLine, endLine)
	}
	if preNipHash == "" || previewHash == "" {
		return Ticket{}, fmt.Errorf("REFUSED: a ticket needs both a pre-nip file hash and a preview-text hash")
	}
	if ttl > MaxTicketTTL {
		return Ticket{}, fmt.Errorf("REFUSED: requested TTL %s exceeds the %s ceiling -- a nip that needs "+
			"longer to review belongs in TRACKING.md as its own item, not a longer-lived ticket", ttl, MaxTicketTTL)
	}
	if ttl <= 0 {
		ttl = DefaultTicketTTL
	}

	now := time.Now().UTC()
	id := fmt.Sprintf("nip-%d", s.NextID)
	s.NextID++

	t := Ticket{
		ID:          id,
		File:        file,
		StartLine:   startLine,
		EndLine:     endLine,
		PreNipHash:  preNipHash,
		PreviewHash: previewHash,
		IssuedAt:    now.Format("2006-01-02T15:04:05Z"),
		ExpiresAt:   now.Add(ttl).Format("2006-01-02T15:04:05Z"),
	}
	if s.Tickets == nil {
		s.Tickets = make(map[string]Ticket)
	}
	s.Tickets[id] = t
	return t, SaveTicketStore(s)
}

// GetTicket looks up a pending ticket by ID. found is false both for
// an unknown ID and for one that was already confirmed/cancelled and
// removed -- callers (T-24's confirm/cancel) treat both the same way:
// there is nothing pending under that ID to act on.
func GetTicket(s TicketStore, id string) (t Ticket, found bool) {
	t, found = s.Tickets[id]
	return
}

// IsExpired reports whether t's TTL has elapsed as of now. A malformed
// ExpiresAt (should not happen -- always written by IssueTicket) is
// treated as expired rather than permanently live, erring toward the
// ticket store's own policy that a ticket must not linger indefinitely.
func (t Ticket) IsExpired(now time.Time) bool {
	exp, err := time.Parse("2006-01-02T15:04:05Z", t.ExpiresAt)
	if err != nil {
		return true
	}
	return now.After(exp)
}

// DiscardTicket removes a ticket from the store unconditionally --
// used by both a successful confirm (the ticket is now a real Txn,
// recorded separately via Record; it has no further use as a pending
// ticket) and by cancel (no write ever happens; the ticket simply
// stops existing). Removing an unknown ID is a no-op, not an error --
// mirrors cancel's own idempotence expectations (cancelling twice, or
// cancelling an already-expired ticket, should not itself fail).
func DiscardTicket(s *TicketStore, id string) error {
	if s.Tickets == nil {
		return nil
	}
	delete(s.Tickets, id)
	return SaveTicketStore(s)
}

// PruneExpired removes every ticket whose TTL has elapsed as of now,
// returning how many were pruned. Not wired into any automatic path
// by T-22 itself (there is no background process in a CLI tool); T-23
// or T-24's own commands are expected to call this opportunistically
// (e.g. at the top of niplines/confirm/cancel) so an abandoned ticket
// does not accumulate in the store forever, the same "nothing lingers
// unbounded" property Journal's own MaxTxns/MaxBytes eviction gives
// completed edits.
func PruneExpired(s *TicketStore, now time.Time) int {
	if s.Tickets == nil {
		return 0
	}
	pruned := 0
	for id, t := range s.Tickets {
		if t.IsExpired(now) {
			delete(s.Tickets, id)
			pruned++
		}
	}
	return pruned
}
