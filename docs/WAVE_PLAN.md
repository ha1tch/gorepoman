# Wave plan

**Wave 1 — Provenance and gate hardening (≈ 5.0d, added 2026-09-07).** The oldest overdue item in this plan: design v1 was agreed before this session but never built. Everything else in this plan is new ground; this wave is a promise already made and not yet kept.


**Wave 2 — Structured dependency data (≈ 4.0d, added 2026-09-07).** Foundational for wave 3: neither kanban item there has structured data to compute a tier from without this.


**Wave 4 — Kanban views (≈ 5.0d, added 2026-09-07).** T-09 is not ready to build as filed -- it needs its own design pass (board persistence and identity are undecided) before implementation, not just before release.


**Wave 5 — Workspace mechanism (≈ 8.0d, added 2026-09-07).** The largest wave, and the one with a genuine open naming decision (T-13) -- resolve that before implementing it, not while implementing it.


**Wave 6 — Release and CI hardening (≈ 6.0d, added 2026-09-07).** T-17 and T-18 are exploratory, not scoped features -- treat their ideal-days as a research budget, not a delivery estimate.


**Wave 7 — ed vocabulary: insert/append/prepend and ticketed niplines (≈ 6.0d, added 2026-09-08).** Implements docs/proposals/ed-insert-and-ticketed-niplines.md. Two independent-but-related additions to ed's vocabulary: a handle-verified insert/append/prepend family (T-20, T-21) that reuses apply's existing SpanHash verification and closes a real anchor-duplication bug hit twice in session; and a two-phase request/confirm ticket flow for line-range deletion (T-22 through T-24), since a bare line-numbered niplines would reintroduce the unverified-anchor risk find/apply was built to eliminate. T-22 (ticket storage shape) blocks T-23, which blocks T-24 -- build in that order. T-25 documents the finished behavior last, not the design.


**Wave 8 — advisory work claims on register items (≈ 3.0d, added 2026-09-09).** Makes multi-agent work claims possible without making them a dependency of the boards work (Wave 2/T-05, Wave 4/T-08). The two are deliberately independent: a claim is an optional Claimed-by field alongside Wave/Filed-by/Blocks-after, read by nothing the dependency-tier board computation touches. Boards can ship first; this wave can land before, after, or interleaved with them with no ordering requirement either way. T-26 (the field) blocks T-27 (claim/release commands), which blocks T-28 (stale-claim policy -- claims must never be able to deadlock the register, so this is not optional hardening, it is part of making claims safe to turn on at all). T-29 (surfacing claims on existing views) only needs T-26 and is independent of T-27/T-28 and of T-05/T-08's dependency_order axis specifically -- it applies to every existing axis (status/priority/theme) whether or not dependency_order ever ships.

