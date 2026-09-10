package board

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/ha1tch/gorepoman/pkg/register"
	"github.com/ha1tch/gorepoman/pkg/report"
	"github.com/ha1tch/gorepoman/pkg/workspace"
)

// DefinitionItem is one register item as it appears on a definition-
// driven board: the same fields register.ItemSummary already has,
// plus which project it came from (a definition-driven board is
// usually cross-project) and which legend entries (if any) apply to
// it.
type DefinitionItem struct {
	Project string `json:"project"`
	register.ItemSummary
	Legend []string `json:"legend,omitempty"`
}

// DefinitionColumn is one rendered column: the definition's own
// label/note, plus the items that landed in it.
type DefinitionColumn struct {
	Label string           `json:"label"`
	Note  string           `json:"note"`
	Items []DefinitionItem `json:"items"`
}

// DefinitionOverflow mirrors workspace.BoardOverflow's Hidden flag,
// plus whatever landed there this render.
type DefinitionOverflow struct {
	Hidden bool             `json:"hidden"`
	Items  []DefinitionItem `json:"items"`
}

// DefinitionReport is the structured shape of a definition-driven
// board render.
type DefinitionReport struct {
	Name     string                       `json:"name"`
	Title    string                       `json:"title"`
	Subtitle string                       `json:"subtitle"`
	Footer   string                       `json:"footer"`
	Axis     string                       `json:"axis"`
	Columns  []DefinitionColumn           `json:"columns"`
	Overflow DefinitionOverflow           `json:"overflow"`
	Legend   []workspace.BoardLegendEntry `json:"legend"`
}

// axisValue returns the field a definition item is grouped by for a
// given axis. dependency_order has no field-based value at all --
// its column is a computed tier, not a field on the item -- so it is
// handled entirely separately (see renderDependencyOrder).
func axisValue(it register.ItemSummary, axis string) string {
	switch axis {
	case "status":
		return it.Status
	case "priority":
		return it.Priority
	case "theme":
		return it.Theme
	default:
		return ""
	}
}

// legendMatches reports whether every key in a legend entry's
// AppliesWhen matches the item's own corresponding field -- a plain
// AND across whatever fields are named, proposed as the simplest
// shape that fits (status/priority/theme/id/title); not yet confirmed
// beyond that proposal.
func legendMatches(it register.ItemSummary, appliesWhen map[string]interface{}) bool {
	for k, v := range appliesWhen {
		want, ok := v.(string)
		if !ok {
			return false
		}
		var got string
		switch k {
		case "status":
			got = it.Status
		case "priority":
			got = it.Priority
		case "theme":
			got = it.Theme
		case "id":
			got = it.ID
		case "title":
			got = it.Title
		default:
			return false
		}
		if got != want {
			return false
		}
	}
	return len(appliesWhen) > 0
}

// boardFlatItem pairs one register item with the project it came
// from -- package-scoped (not local to RenderDefinition) because
// renderDependencyOrder, the dependency_order axis's own render
// path, needs the same shape.
type boardFlatItem struct {
	project string
	item    register.ItemSummary
}

// RenderDefinition groups items (already gathered from one or more
// projects, e.g. via ProjectReport) into a DefinitionReport per def's
// axis and columns, following the rules settled for values: explicit
// per-column values win; status/priority fall back to their natural
// order when a column omits values; theme has none, so an author
// must have supplied values (ValidateBoard already enforces this);
// columns entirely absent auto-derives one column per distinct value
// actually present in the data. Anything matching no column goes to
// Overflow -- nothing silently disappears.
func RenderDefinition(def workspace.Board, projects []ProjectReport) (DefinitionReport, error) {
	var flat []boardFlatItem
	for _, p := range projects {
		for _, it := range p.OpenItems {
			flat = append(flat, boardFlatItem{project: p.Name, item: it})
		}
	}

	if def.Axis == "dependency_order" {
		return renderDependencyOrder(def, flat)
	}

	cols := def.Columns
	if len(cols) == 0 {
		// Rule 7: one column per value actually present in the data --
		// not one per possible natural-order value regardless of use,
		// which would litter the board with empty columns (P4/P5 on a
		// project that never uses them, say). Natural order decides the
		// SEQUENCE among values that are present; it does not manufacture
		// columns for values that are not.
		present := make(map[string]bool)
		for _, fi := range flat {
			present[axisValue(fi.item, def.Axis)] = true
		}
		var order []string
		seen := make(map[string]bool)
		for _, v := range workspace.NaturalOrderFor(def.Axis) {
			if present[v] {
				order = append(order, v)
				seen[v] = true
			}
		}
		for _, fi := range flat {
			v := axisValue(fi.item, def.Axis)
			if !seen[v] {
				seen[v] = true
				order = append(order, v)
			}
		}
		for _, v := range order {
			cols = append(cols, workspace.BoardColumn{Label: v, Values: []string{v}})
		}
	}

	// Resolve each column's effective value set: explicit values win;
	// absent values on status/priority fall back to natural order by
	// position (column i collects natural-order value i).
	natural := workspace.NaturalOrderFor(def.Axis)
	effective := make([][]string, len(cols))
	for i, c := range cols {
		if len(c.Values) > 0 {
			effective[i] = c.Values
		} else if i < len(natural) {
			effective[i] = []string{natural[i]}
		}
	}

	valueToCol := make(map[string]int)
	for i, vals := range effective {
		for _, v := range vals {
			valueToCol[v] = i
		}
	}

	out := DefinitionReport{
		Name: def.Name, Title: def.Title, Subtitle: def.Subtitle, Footer: def.Footer,
		Axis: def.Axis, Legend: def.Legend,
	}
	if out.Legend == nil {
		// T-30: a nil slice serializes as JSON null, which queryfy's
		// strict-mode schema rejects for an optional array field even
		// without .Required() -- normalize to an always-present empty
		// array here, same discipline Columns/Overflow.Items already
		// follow a few lines below.
		out.Legend = []workspace.BoardLegendEntry{}
	}
	for _, c := range cols {
		out.Columns = append(out.Columns, DefinitionColumn{Label: c.Label, Note: c.Note, Items: []DefinitionItem{}})
	}
	out.Overflow = DefinitionOverflow{Hidden: def.Overflow.Hidden, Items: []DefinitionItem{}}

	for _, fi := range flat {
		di := DefinitionItem{Project: fi.project, ItemSummary: fi.item}
		for _, le := range def.Legend {
			if legendMatches(fi.item, le.AppliesWhen) {
				di.Legend = append(di.Legend, le.Name)
			}
		}
		v := axisValue(fi.item, def.Axis)
		if idx, ok := valueToCol[v]; ok {
			out.Columns[idx].Items = append(out.Columns[idx].Items, di)
		} else {
			out.Overflow.Items = append(out.Overflow.Items, di)
		}
	}

	return out, nil
}

// renderDependencyOrder is RenderDefinition's dependency_order branch:
// tier(i) = 1 + max(blocker tiers), 0 for an item with no unresolved
// blocker (register.TierOf) -- the exact by-hand rule verified twice
// against this project's own register before being built
// (docs/proposals/t08-board-target-shape.md and its "Path to Boards"
// companion). A cycle makes tiering undefined, so it is reported as
// an error naming one concrete cycle (register.DetectCycle's return),
// not silently mis-tiered or dropped.
//
// Column assignment follows workspace.BoardColumn's own documented
// contract for this axis (schema.go: "position IS the value: column
// i collects tier i"): def.Columns, when given, map to tier numbers
// by position and every item whose tier is deeper than the configured
// columns goes to Overflow; when def.Columns is empty, one column is
// auto-derived per tier actually present (0..max), the same Rule 7
// already used for status/priority/theme when they have no explicit
// columns either.
func renderDependencyOrder(def workspace.Board, flat []boardFlatItem) (DefinitionReport, error) {
	items := make([]register.ItemSummary, len(flat))
	for i, fi := range flat {
		items[i] = fi.item
	}

	edges := register.BuildGraph(items)
	if hasCycle, cyc := register.DetectCycle(edges); hasCycle {
		return DefinitionReport{}, fmt.Errorf("dependency_order: Blocks/after graph has a cycle: %s -- tiering is undefined until this is broken", strings.Join(cyc, " -> "))
	}

	tiers, ok := register.TierOf(items)
	if !ok {
		// TierOf itself re-checks for a cycle; this path should be
		// unreachable given the check above, but a silent nil map is
		// worse than a defensive error naming the real gap.
		return DefinitionReport{}, fmt.Errorf("dependency_order: tier computation failed unexpectedly after cycle check passed")
	}

	maxTier := -1
	for _, t := range tiers {
		if t > maxTier {
			maxTier = t
		}
	}

	cols := def.Columns
	if len(cols) == 0 {
		for t := 0; t <= maxTier; t++ {
			cols = append(cols, workspace.BoardColumn{Label: fmt.Sprintf("Tier %d", t)})
		}
	}

	out := DefinitionReport{
		Name: def.Name, Title: def.Title, Subtitle: def.Subtitle, Footer: def.Footer,
		Axis: def.Axis, Legend: def.Legend,
	}
	if out.Legend == nil {
		// T-30: a nil slice serializes as JSON null, which queryfy's
		// strict-mode schema rejects for an optional array field even
		// without .Required() -- normalize to an always-present empty
		// array here, same discipline Columns/Overflow.Items already
		// follow a few lines below.
		out.Legend = []workspace.BoardLegendEntry{}
	}
	for _, c := range cols {
		out.Columns = append(out.Columns, DefinitionColumn{Label: c.Label, Note: c.Note, Items: []DefinitionItem{}})
	}
	out.Overflow = DefinitionOverflow{Hidden: def.Overflow.Hidden, Items: []DefinitionItem{}}

	for _, fi := range flat {
		di := DefinitionItem{Project: fi.project, ItemSummary: fi.item}
		for _, le := range def.Legend {
			if legendMatches(fi.item, le.AppliesWhen) {
				di.Legend = append(di.Legend, le.Name)
			}
		}
		tier := tiers[fi.item.ID]
		if tier >= 0 && tier < len(cols) {
			out.Columns[tier].Items = append(out.Columns[tier].Items, di)
		} else {
			out.Overflow.Items = append(out.Overflow.Items, di)
		}
	}

	return out, nil
}

// runDefinition loads and validates a board definition, renders it
// against already-gathered projects (narrowed to def.Scope.Projects
// when non-empty), and dispatches on format.
func runDefinition(path string, projects []ProjectReport, format string) int {
	b, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading %s: %v\n", path, err)
		return 1
	}
	var def workspace.Board
	if err := json.Unmarshal(b, &def); err != nil {
		fmt.Fprintf(os.Stderr, "error parsing %s: %v\n", path, err)
		return 1
	}
	if err := workspace.ValidateBoard(def); err != nil {
		fmt.Fprintf(os.Stderr, "error: %s: %v\n", path, err)
		return 1
	}

	scoped := projects
	if len(def.Scope.Projects) > 0 {
		scoped = nil
		want := make(map[string]bool)
		for _, p := range def.Scope.Projects {
			want[p] = true
		}
		for _, p := range projects {
			if want[p.Name] {
				scoped = append(scoped, p)
			}
		}
	}

	rep, err := RenderDefinition(def, scoped)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	switch format {
	case "text":
		printDefinitionText(rep)
		return 0
	case "json":
		if err := ValidateDefinitionReport(rep); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
		return report.EmitOrErr(report.EmitJSON(os.Stdout, "board", "board-definition-report", DefinitionSchemaVersion, rep))
	case "html":
		body, err := renderDefinitionHTML(rep)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
		title := rep.Title
		if title == "" {
			title = rep.Name
		}
		return report.EmitOrErr(report.EmitHTML(os.Stdout, title, "board", "board-definition-report", body))
	default:
		fmt.Fprintf(os.Stderr, "unknown format %q (want text, json, or html)\n", format)
		return 1
	}
}

func printDefinitionText(rep DefinitionReport) {
	title := rep.Title
	if title == "" {
		title = rep.Name
	}
	fmt.Println(title)
	if rep.Subtitle != "" {
		fmt.Println(rep.Subtitle)
	}
	fmt.Println()
	for _, col := range rep.Columns {
		fmt.Printf("%s (%d)\n", col.Label, len(col.Items))
		if col.Note != "" {
			fmt.Printf("  %s\n", col.Note)
		}
		for _, it := range col.Items {
			legend := ""
			if len(it.Legend) > 0 {
				legend = "  [" + joinComma(it.Legend) + "]"
			}
			fmt.Printf("  %-6s %s  [%s]  %s%s\n", it.ID, it.Status, it.Project, it.Title, legend)
		}
		fmt.Println()
	}
	if !rep.Overflow.Hidden && len(rep.Overflow.Items) > 0 {
		fmt.Printf("Overflow (%d)\n", len(rep.Overflow.Items))
		for _, it := range rep.Overflow.Items {
			fmt.Printf("  %-6s %s  [%s]  %s\n", it.ID, it.Status, it.Project, it.Title)
		}
		fmt.Println()
	}
	if rep.Footer != "" {
		fmt.Println(rep.Footer)
	}
}

func joinComma(ss []string) string {
	out := ""
	for i, s := range ss {
		if i > 0 {
			out += ", "
		}
		out += s
	}
	return out
}
