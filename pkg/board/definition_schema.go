package board

import (
	"github.com/ha1tch/gorepoman/pkg/report"
	qf "github.com/ha1tch/queryfy"
	"github.com/ha1tch/queryfy/builders"
)

const DefinitionSchemaVersion = 1

var definitionItemSchema = builders.Object().
	Field("project", builders.String().Required()).
	Field("id", builders.String().Required()).
	Field("status", builders.String().Required()).
	Field("priority", builders.String().Required()).
	Field("theme", builders.String().Required()).
	Field("title", builders.String().Required()).
	Field("wave", builders.String()).
	Field("filed_by", builders.String()).
	Field("blocks", builders.String()).
	Field("legend", builders.Array().Of(builders.String()))

var definitionColumnSchema = builders.Object().
	Field("label", builders.String().Required()).
	Field("note", builders.String()).
	Field("items", builders.Array().Of(definitionItemSchema).Required())

var definitionOverflowSchema = builders.Object().
	Field("hidden", builders.Bool().Required()).
	Field("items", builders.Array().Of(definitionItemSchema).Required())

var definitionLegendSchema = builders.Object().
	Field("name", builders.String().Required()).
	Field("color", builders.String().Required()).
	Field("applies_when", builders.Object().AllowAdditional(true).Required())

var definitionReportSchema = builders.Object().
	Field("name", builders.String().Required()).
	Field("title", builders.String()).
	Field("subtitle", builders.String()).
	Field("footer", builders.String()).
	Field("axis", builders.String().Required()).
	Field("columns", builders.Array().Of(definitionColumnSchema).Required()).
	Field("overflow", definitionOverflowSchema.Required()).
	Field("legend", builders.Array().Of(definitionLegendSchema))

var definitionEnvelopeSchema = builders.Object().
	Field("tool", builders.String().Pattern(`^board$`).Required()).
	Field("object", builders.String().Pattern(`^board-definition-report$`).Required()).
	Field("schema_version", builders.Number().Required()).
	Field("data", definitionReportSchema.Required()).
	Field("generated_at", builders.String().Required())

// ValidateDefinitionReport checks the actual bytes about to ship,
// same discipline as every other command's --format json path.
func ValidateDefinitionReport(rep DefinitionReport) error {
	m, err := report.AsMap("board", "board-definition-report", DefinitionSchemaVersion, rep)
	if err != nil {
		return err
	}
	return qf.Validate(m, definitionEnvelopeSchema)
}
