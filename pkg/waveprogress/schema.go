package waveprogress

import (
	"github.com/ha1tch/gorepoman/pkg/report"
	qf "github.com/ha1tch/queryfy"
	"github.com/ha1tch/queryfy/builders"
)

const SchemaVersion = 1

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

var reportSchema = builders.Object().
	Field("waves", builders.Array().Of(waveSchema).Required()).
	Field("overall_done", builders.Number().Required()).
	Field("overall_total", builders.Number().Required()).
	Field("overall_pct", builders.Number().Required()).
	Field("hidden_count", builders.Number().Required())

var reportEnvelopeSchema = builders.Object().
	Field("tool", builders.String().Pattern(`^waveprogress$`).Required()).
	Field("object", builders.String().Pattern(`^waveprogress-report$`).Required()).
	Field("schema_version", builders.Number().Required()).
	Field("data", reportSchema.Required()).
	Field("generated_at", builders.String().Required())

var checkSchema = builders.Object().
	Field("stale", builders.Bool().Required())

var checkEnvelopeSchema = builders.Object().
	Field("tool", builders.String().Pattern(`^waveprogress$`).Required()).
	Field("object", builders.String().Pattern(`^waveprogress-check$`).Required()).
	Field("schema_version", builders.Number().Required()).
	Field("data", checkSchema.Required()).
	Field("generated_at", builders.String().Required())

func ValidateReport(data WaveReport) error {
	m, err := report.AsMap("waveprogress", "waveprogress-report", SchemaVersion, data)
	if err != nil {
		return err
	}
	return qf.Validate(m, reportEnvelopeSchema)
}

func ValidateCheck(data CheckResult) error {
	m, err := report.AsMap("waveprogress", "waveprogress-check", SchemaVersion, data)
	if err != nil {
		return err
	}
	return qf.Validate(m, checkEnvelopeSchema)
}
