# Wave tracking

## 1. Progress at a glance

```
Wave 1  Provenance and gate hardening  ░░░░░░░░░░░░░░░░░░░░     0%  (0/4 items)
Wave 2  Structured dependency data  ██████████░░░░░░░░░░    50%  (1/2 items)
Wave 4  Kanban views                █████████████░░░░░░░    67%  (2/3 items)
Wave 5  Workspace mechanism         ████████████████████   100%  (5/5 items)
Wave 6  Release and CI hardening    ░░░░░░░░░░░░░░░░░░░░     0%  (0/4 items)
Wave 7  ed vocabulary: insert/append/prepend and ticketed niplines  ░░░░░░░░░░░░░░░░░░░░     0%  (0/6 items)
Wave 8  advisory work claims on register items  ░░░░░░░░░░░░░░░░░░░░     0%  (0/4 items)
```

Overall by item count: 8 of 28 items ≈ **29%**

### Wave 1 — Provenance and gate hardening (4 items, ideal 5.0d, added 2026-09-07)

| # | Summary | Status | Register item |
|---|---|---|---|
| 1 | provenance check: sha256-journal-based detection of out-of-band edits | ☐ | T-01 |
| 2 | provenance sanction: explicit override with mandatory reason | ☐ | T-02 |
| 3 | wire provenance into relcore, unconditional alongside badcode | ☐ | T-03 |
| 4 | badcode hits annotated with provenance status | ☐ | T-04 |

**Wave 1: 0/4, not started.**

### Wave 2 — Structured dependency data (2 items, ideal 4.0d, added 2026-09-07)

| # | Summary | Status | Register item |
|---|---|---|---|
| 5 | parse Blocks/after into real graph edges, with cycle detection | ☐ | T-05 |
| 6 | wave membership as a queryable field on a register item | ✓ | T-06 |
**Wave 2: 1/2, in progress.**

### Wave 4 — Kanban views (3 items, ideal 5.0d, added 2026-09-07)

| # | Summary | Status | Register item |
|---|---|---|---|
| 7 | register kanban view (single-project, status-grouped) | ✓ | T-07 |
| 8 | board: cross-project dependency-order tiering | ☐ | T-08 |
| 9 | board: configurable column labels / grouping axis | ✓ | T-09 |

**Wave 4: 2/3, in progress.**

### Wave 5 — Workspace mechanism (5 items, ideal 8.0d, added 2026-09-07)

| # | Summary | Status | Register item |
|---|---|---|---|
| 10 | config: workspaces key in .repoman.json | ✓ | T-10 |
| 11 | workspace participants file + target validation | ✓ | T-11 |
| 12 | repoman workspace join/leave/list | ✓ | T-12 |
| 13 | workspace issue/closure commands | ✓ | T-13 |
| 14 | Filed-by field parsing/validation on register items | ✓ | T-14 |
**Wave 5: 5/5, done.**

### Wave 6 — Release and CI hardening (4 items, ideal 6.0d, added 2026-09-07)

| # | Summary | Status | Register item |
|---|---|---|---|
| 15 | relcore orchestrates GoReleaser as a resumable step | ☐ | T-15 |
| 16 | canonical CI workflow: badcode + selftest as a required PR check | ☐ | T-16 |
| 17 | roles backed by a real structural matcher (research spike) | ☐ | T-17 |
| 18 | guards: distinguish never-rerun from code-moved-since | ☐ | T-18 |

**Wave 6: 0/4, not started.**

### Wave 7 — ed vocabulary: insert/append/prepend and ticketed niplines (6 items, ideal 6.0d, added 2026-09-08)

| # | Summary | Status | Register item |
|---|---|---|---|
| 19 | append/prepend verbs, no handle required | ☐ | T-20 |
| 20 | insert verb, handle-verified positional insertion | ☐ | T-21 |
| 21 | pending-ticket storage design for niplines | ☐ | T-22 |
| 22 | niplines request/preview (phase 1) with gofmt/vet preflight | ☐ | T-23 |
| 23 | niplines confirm/cancel (phase 2), TTL enforcement | ☐ | T-24 |
| 24 | docs: repoman-040-editing.md coverage for the new verbs | ☐ | T-25 |

**Wave 7: 0/6, not started.**

### Wave 8 — advisory work claims on register items (4 items, ideal 3.0d, added 2026-09-09)

| # | Summary | Status | Register item |
|---|---|---|---|
| 25 | Claimed-by: optional register-item field for advisory work claims | ☐ | T-26 |
| 26 | register claim/release: write and clear Claimed-by, journaled | ☐ | T-27 |
| 27 | stale-claim policy: TTL or override for an abandoned Claimed-by | ☐ | T-28 |
| 28 | surface claims on register/board views (kanban, board, board --definition) | ☐ | T-29 |

**Wave 8: 0/4, not started.**

---

