package guards

import (
	"github.com/ha1tch/gorepoman/pkg/report"
	qf "github.com/ha1tch/queryfy"
	"github.com/ha1tch/queryfy/builders"
)

const SchemaVersion = 1

var summarySchema = builders.Object().
	Field("id", builders.String().Required()).
	Field("title", builders.String().Required()).
	Field("last_exercised", builders.String().Required())

var listEnvelopeSchema = builders.Object().
	Field("tool", builders.String().Pattern(`^guards$`).Required()).
	Field("object", builders.String().Pattern(`^guards-list$`).Required()).
	Field("schema_version", builders.Number().Required()).
	Field("data", builders.Array().Of(summarySchema).Required()).
	Field("generated_at", builders.String().Required())

var detailSchema = builders.Object().
	Field("id", builders.String().Required()).
	Field("title", builders.String().Required()).
	Field("block", builders.String().Required())

var detailEnvelopeSchema = builders.Object().
	Field("tool", builders.String().Pattern(`^guards$`).Required()).
	Field("object", builders.String().Pattern(`^guards-detail$`).Required()).
	Field("schema_version", builders.Number().Required()).
	Field("data", detailSchema.Required()).
	Field("generated_at", builders.String().Required())

var staleSchema = builders.Object().
	Field("since", builders.String().Required()).
	Field("stale", builders.Array().Of(summarySchema).Required()).
	Field("all_current", builders.Bool().Required())

var staleEnvelopeSchema = builders.Object().
	Field("tool", builders.String().Pattern(`^guards$`).Required()).
	Field("object", builders.String().Pattern(`^guards-stale$`).Required()).
	Field("schema_version", builders.Number().Required()).
	Field("data", staleSchema.Required()).
	Field("generated_at", builders.String().Required())

func ValidateList(data []GuardSummary) error {
	m, err := report.AsMap("guards", "guards-list", SchemaVersion, data)
	if err != nil {
		return err
	}
	return qf.Validate(m, listEnvelopeSchema)
}

func ValidateDetail(data GuardDetail) error {
	m, err := report.AsMap("guards", "guards-detail", SchemaVersion, data)
	if err != nil {
		return err
	}
	return qf.Validate(m, detailEnvelopeSchema)
}

func ValidateStale(data StaleResult) error {
	m, err := report.AsMap("guards", "guards-stale", SchemaVersion, data)
	if err != nil {
		return err
	}
	return qf.Validate(m, staleEnvelopeSchema)
}
