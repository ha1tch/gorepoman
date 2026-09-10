package register

import (
	"fmt"
	"os"

	"github.com/ha1tch/gorepoman/pkg/config"
	"github.com/ha1tch/gorepoman/pkg/report"
	qf "github.com/ha1tch/queryfy"
	"github.com/ha1tch/queryfy/builders"
)

const KanbanSchemaVersion = 1

// kanbanColumns is the fixed, always-shown sequence for a single-
// project kanban view. Unlike a workspace board's flexible columns,
// this needs no values/natural-order/auto-derive machinery: every
// item already has exactly one of these five statuses (A3 already
// enforces that), so there is no Overflow case to handle and no
// reason to omit a column just because it happens to be empty right
// now -- an empty "Dropped" column is itself useful information.
var kanbanColumns = []struct{ Status, Label string }{
	{"☐", "Not started"},
	{"◐", "In progress"},
	{"☑", "Complete, pending release"},
	{"✓", "Done"},
	{"✗", "Dropped"},
}

type KanbanColumn struct {
	Status string        `json:"status"`
	Label  string        `json:"label"`
	Items  []ItemSummary `json:"items"`
}

type KanbanReport struct {
	Columns []KanbanColumn `json:"columns"`
}

func toKanban(reg *Register, cfg *config.Config) KanbanReport {
	items := toSummaries(reg, cfg)
	var out KanbanReport
	for _, kc := range kanbanColumns {
		col := KanbanColumn{Status: kc.Status, Label: kc.Label, Items: []ItemSummary{}}
		for _, it := range items {
			if it.Status == kc.Status {
				col.Items = append(col.Items, it)
			}
		}
		out.Columns = append(out.Columns, col)
	}
	return out
}

var kanbanColumnSchema = builders.Object().
	Field("status", builders.String().Required()).
	Field("label", builders.String().Required()).
	Field("items", builders.Array().Of(summarySchema).Required())

var kanbanReportSchema = builders.Object().
	Field("columns", builders.Array().Of(kanbanColumnSchema).Required())

var kanbanEnvelopeSchema = builders.Object().
	Field("tool", builders.String().Pattern(`^register$`).Required()).
	Field("object", builders.String().Pattern(`^register-kanban$`).Required()).
	Field("schema_version", builders.Number().Required()).
	Field("data", kanbanReportSchema.Required()).
	Field("generated_at", builders.String().Required())

func ValidateKanban(data KanbanReport) error {
	m, err := report.AsMap("register", "register-kanban", KanbanSchemaVersion, data)
	if err != nil {
		return err
	}
	return qf.Validate(m, kanbanEnvelopeSchema)
}

func runKanban(reg *Register, cfg *config.Config, format string) int {
	data := toKanban(reg, cfg)
	switch format {
	case "text":
		for _, col := range data.Columns {
			fmt.Printf("%s %s (%d)\n", col.Status, col.Label, len(col.Items))
			for _, it := range col.Items {
				fmt.Printf("  %-6s %s  [%s]  %s\n", it.ID, it.Priority, it.Theme, it.Title)
			}
			fmt.Println()
		}
		return 0
	case "json":
		if err := ValidateKanban(data); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
		return report.EmitOrErr(report.EmitJSON(os.Stdout, "register", "register-kanban", KanbanSchemaVersion, data))
	case "html":
		body, err := renderKanbanHTML(data)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
		return report.EmitOrErr(report.EmitHTML(os.Stdout, "register kanban", "register", "register-kanban", body))
	default:
		fmt.Fprintf(os.Stderr, "unknown format %q (want text, json, or html)\n", format)
		return 1
	}
}
