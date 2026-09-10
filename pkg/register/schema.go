package register

import (
	"github.com/ha1tch/gorepoman/pkg/report"
	qf "github.com/ha1tch/queryfy"
	"github.com/ha1tch/queryfy/builders"
)

const SchemaVersion = 1

var summarySchema = builders.Object().
	Field("id", builders.String().Required()).
	Field("status", builders.String().Required()).
	Field("priority", builders.String().Required()).
	Field("theme", builders.String().Required()).
	Field("title", builders.String().Required()).
	Field("wave", builders.String()).
	Field("filed_by", builders.String()).
	Field("blocks", builders.String()).
	Field("claimed_by", builders.String())

var listEnvelopeSchema = builders.Object().
	Field("tool", builders.String().Pattern(`^register$`).Required()).
	Field("object", builders.String().Pattern(`^register-list$`).Required()).
	Field("schema_version", builders.Number().Required()).
	Field("data", builders.Array().Of(summarySchema).Required()).
	Field("generated_at", builders.String().Required())

var detailSchema = builders.Object().
	Field("id", builders.String().Required()).
	Field("title", builders.String().Required()).
	Field("block", builders.String().Required())

var detailEnvelopeSchema = builders.Object().
	Field("tool", builders.String().Pattern(`^register$`).Required()).
	Field("object", builders.String().Pattern(`^register-item$`).Required()).
	Field("schema_version", builders.Number().Required()).
	Field("data", detailSchema.Required()).
	Field("generated_at", builders.String().Required())

var checkSchema = builders.Object().
	Field("errors", builders.Array().Of(builders.String()).Required()).
	Field("open_items", builders.Number().Required()).
	Field("ok", builders.Bool().Required())

var checkEnvelopeSchema = builders.Object().
	Field("tool", builders.String().Pattern(`^register$`).Required()).
	Field("object", builders.String().Pattern(`^register-check$`).Required()).
	Field("schema_version", builders.Number().Required()).
	Field("data", checkSchema.Required()).
	Field("generated_at", builders.String().Required())

func ValidateList(data []ItemSummary) error {
	m, err := report.AsMap("register", "register-list", SchemaVersion, data)
	if err != nil {
		return err
	}
	return qf.Validate(m, listEnvelopeSchema)
}

func ValidateDetail(data ItemDetail) error {
	m, err := report.AsMap("register", "register-item", SchemaVersion, data)
	if err != nil {
		return err
	}
	return qf.Validate(m, detailEnvelopeSchema)
}

func ValidateCheck(data CheckResult) error {
	m, err := report.AsMap("register", "register-check", SchemaVersion, data)
	if err != nil {
		return err
	}
	return qf.Validate(m, checkEnvelopeSchema)
}
