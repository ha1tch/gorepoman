package board

import (
	"github.com/ha1tch/gorepoman/pkg/report"
	qf "github.com/ha1tch/queryfy"
	"github.com/ha1tch/queryfy/builders"
)

const SchemaVersion = 1

// itemSchema and waveSchema mirror register.ItemSummary and
// waveprogress.WaveStatus's own field shapes -- queryfy schemas
// aren't themselves importable across packages, so this is a
// deliberate, small, by-convention duplication of two already-stable
// shapes, not a new design.
var itemSchema = builders.Object().
	Field("id", builders.String().Required()).
	Field("status", builders.String().Required()).
	Field("priority", builders.String().Required()).
	Field("theme", builders.String().Required()).
	Field("title", builders.String().Required()).
	Field("wave", builders.String()).
	Field("filed_by", builders.String()).
	Field("blocks", builders.String())

var blockerSchema = builders.Object().
	Field("blocker", builders.String().Required()).
	Field("blocked", builders.Array().Of(builders.String()).Required())

var waveSchema = builders.Object().
	Field("id", builders.String().Required()).
	Field("name", builders.String().Required()).
	Field("pct", builders.Number().Required()).
	Field("has_partial", builders.Bool().Required()).
	Field("done", builders.Number().Required()).
	Field("total", builders.Number().Required()).
	Field("debt", builders.Array().Of(builders.String()).Required()).
	Field("blockers", builders.Array().Of(blockerSchema).Required())

var projectSchema = builders.Object().
	Field("name", builders.String().Required()).
	Field("path", builders.String().Required()).
	Field("register_error", builders.String().Required()).
	Field("open_items", builders.Array().Of(itemSchema).Required()).
	Field("wave_error", builders.String().Required()).
	Field("wave_overall_pct", builders.Number().Required()).
	Field("waves", builders.Array().Of(waveSchema).Required())

var reportSchema = builders.Object().
	Field("projects", builders.Array().Of(projectSchema).Required())

var envelopeSchema = builders.Object().
	Field("tool", builders.String().Pattern(`^board$`).Required()).
	Field("object", builders.String().Pattern(`^board-report$`).Required()).
	Field("schema_version", builders.Number().Required()).
	Field("data", reportSchema.Required()).
	Field("generated_at", builders.String().Required())

func Validate(data BoardReport) error {
	m, err := report.AsMap("board", "board-report", SchemaVersion, data)
	if err != nil {
		return err
	}
	return qf.Validate(m, envelopeSchema)
}
