package gomod

import (
	"github.com/ha1tch/gorepoman/pkg/report"
	qf "github.com/ha1tch/queryfy"
	"github.com/ha1tch/queryfy/builders"
)

const SchemaVersion = 1

var checkResultSchema = builders.Object().
	Field("toolchain_available", builders.Bool().Required()).
	Field("errors", builders.Array().Of(builders.String()).Required()).
	Field("warnings", builders.Array().Of(builders.String()).Required()).
	Field("ok", builders.Bool().Required())

var envelopeSchema = builders.Object().
	Field("tool", builders.String().Pattern(`^gomod$`).Required()).
	Field("object", builders.String().Pattern(`^gomod-check$`).Required()).
	Field("schema_version", builders.Number().Required()).
	Field("data", checkResultSchema.Required()).
	Field("generated_at", builders.String().Required())

// Validate confirms the envelope report.EmitJSON is about to write
// matches the declared schema, against the actual marshaled bytes.
func Validate(result CheckResult) error {
	m, err := report.AsMap("gomod", "gomod-check", SchemaVersion, result)
	if err != nil {
		return err
	}
	return qf.Validate(m, envelopeSchema)
}
