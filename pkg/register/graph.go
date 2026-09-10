package register

import "strings"

// Edge is one dependency edge: From is blocked by (comes after) To.
// The direction matches the register's own Blocks/after field: an
// item's Blocks/after names what it depends on, not what depends on
// it -- T-08's own field line, "Blocks/after: T-06", means T-08 comes
// after T-06, not the reverse.
type Edge struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// ParseBlocks splits one item's raw Blocks/after field into a list of
// target ids. The live format across every register seen so far is
// bare comma-separated ids ("T-20,T-21,T-23,T-24") -- no cross-project
// "<project>:<prefix>-<n>" syntax in use for this field, unlike
// Filed-by which already owns that shape. Empty and whitespace-only
// entries are dropped rather than producing a spurious empty target,
// so a trailing comma or accidental double comma degrades gracefully
// instead of surfacing as a broken edge.
func ParseBlocks(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var out []string
	for _, part := range strings.Split(raw, ",") {
		p := strings.TrimSpace(part)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// BuildGraph turns every open item's Blocks field into a flat edge
// list. An edge target that names an id not present in items (closed,
// typo'd, or from a namespace this register doesn't track) is kept as
// an edge rather than silently dropped -- TierOf below is what decides
// what to do with a dangling reference, not this function; a graph
// builder that quietly discards edges would make a real data problem
// invisible instead of surfacing it.
func BuildGraph(items []ItemSummary) []Edge {
	var edges []Edge
	for _, it := range items {
		for _, to := range ParseBlocks(it.Blocks) {
			edges = append(edges, Edge{From: it.ID, To: to})
		}
	}
	return edges
}

// DetectCycle reports whether the edge list contains a cycle, and if
// so, one concrete cycle as a slice of ids starting and ending at the
// same id (e.g. ["T-1", "T-2", "T-1"]) -- named identifiers a person
// can act on, not just a boolean. Standard DFS with a three-colour
// visiting set (white/grey/black via two maps) so a self-loop
// (A blocks-after A) is caught the same way a longer cycle is, not as
// a special case.
func DetectCycle(edges []Edge) (bool, []string) {
	adj := make(map[string][]string)
	nodes := make(map[string]bool)
	for _, e := range edges {
		adj[e.From] = append(adj[e.From], e.To)
		nodes[e.From] = true
		nodes[e.To] = true
	}

	const (
		white = 0
		grey  = 1
		black = 2
	)
	color := make(map[string]int)
	var path []string

	var visit func(n string) []string
	visit = func(n string) []string {
		color[n] = grey
		path = append(path, n)
		for _, next := range adj[n] {
			switch color[next] {
			case grey:
				// Found the back-edge that closes the cycle: return the
				// path from next's first occurrence through here, plus
				// next again to close the loop visibly.
				start := 0
				for i, p := range path {
					if p == next {
						start = i
						break
					}
				}
				cyc := append([]string{}, path[start:]...)
				return append(cyc, next)
			case white:
				if found := visit(next); found != nil {
					return found
				}
			}
		}
		path = path[:len(path)-1]
		color[n] = black
		return nil
	}

	// Deterministic iteration order: a map range over `nodes` would
	// make which cycle gets reported (when more than one exists)
	// non-reproducible between runs, which matters for a function
	// whose whole point is a stable, actionable answer.
	var ordered []string
	for n := range nodes {
		ordered = append(ordered, n)
	}
	sortStrings(ordered)

	for _, n := range ordered {
		if color[n] == white {
			path = nil
			if found := visit(n); found != nil {
				return true, found
			}
		}
	}
	return false, nil
}

// sortStrings is a tiny local insertion sort rather than importing
// sort for one call site -- deterministic order is all that's needed
// here, not performance on a large set (register item counts are in
// the tens, not thousands).
func sortStrings(ss []string) {
	for i := 1; i < len(ss); i++ {
		for j := i; j > 0 && ss[j-1] > ss[j]; j-- {
			ss[j-1], ss[j] = ss[j], ss[j-1]
		}
	}
}

// TierOf computes each item's dependency tier from its edges: tier 0
// for an item with no unresolved blocker, tier N = 1 + the maximum
// tier among its blockers otherwise -- the same by-hand rule verified
// twice this session building the reference board
// (docs/proposals/t08-board-target-shape.md) and the "Path to Boards"
// artifact, now computed instead of traced by hand.
//
// A dangling edge (blocks-after an id not present in items -- closed,
// typo'd, or foreign) is treated as already resolved: it does not
// hold its dependent back, since there is no open item left to wait
// on. This mirrors how a closed item already behaves today (T-08's
// own Blocks/after: T-06 does not block T-08, because T-06 is
// closed and therefore doesn't appear in the open-items list this
// function is given).
//
// Returns (tiers, ok). ok is false only when the graph has a cycle --
// tiers is nil in that case, since a cyclic graph has no well-defined
// tier assignment; the caller is expected to have already run
// DetectCycle for a reportable cycle path rather than only a bool.
func TierOf(items []ItemSummary) (map[string]int, bool) {
	edges := BuildGraph(items)
	if hasCycle, _ := DetectCycle(edges); hasCycle {
		return nil, false
	}

	known := make(map[string]bool, len(items))
	for _, it := range items {
		known[it.ID] = true
	}
	blockers := make(map[string][]string)
	for _, e := range edges {
		if known[e.To] {
			blockers[e.From] = append(blockers[e.From], e.To)
		}
		// A dangling edge (target not in known) contributes nothing --
		// see the doc comment above.
	}

	tiers := make(map[string]int, len(items))
	var compute func(id string, visiting map[string]bool) int
	compute = func(id string, visiting map[string]bool) int {
		if t, ok := tiers[id]; ok {
			return t
		}
		bs := blockers[id]
		if len(bs) == 0 {
			tiers[id] = 0
			return 0
		}
		max := -1
		for _, b := range bs {
			// visiting guards against a cycle DetectCycle somehow missed
			// (defence in depth, not the primary guard) -- treat a
			// re-entrant call as contributing tier 0 rather than
			// recursing forever.
			if visiting[b] {
				continue
			}
			visiting[b] = true
			t := compute(b, visiting)
			delete(visiting, b)
			if t > max {
				max = t
			}
		}
		result := max + 1
		tiers[id] = result
		return result
	}

	for _, it := range items {
		if _, ok := tiers[it.ID]; !ok {
			compute(it.ID, map[string]bool{it.ID: true})
		}
	}
	return tiers, true
}
