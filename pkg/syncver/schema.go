package syncver

import (
	"github.com/ha1tch/gorepoman/pkg/report"
	qf "github.com/ha1tch/queryfy"
	"github.com/ha1tch/queryfy/builders"
)

// SchemaVersion is this package's own schema_version for the
// "syncver-status" object -- bumped only when SyncStatus's shape
// changes, independent of every other subcommand's own version.
const SchemaVersion = 1

var targetSchema = builders.Object().
	Field("file", builders.String().Required()).
	Field("value", builders.String().Required()).
	Field("match", builders.Bool().Required())

var statusSchema = builders.Object().
	Field("version", builders.String().Required()).
	Field("in_sync", builders.Bool().Required()).
	Field("targets", builders.Array().Of(targetSchema).Required())

var envelopeSchema = builders.Object().
	Field("tool", builders.String().Pattern(`^syncver$`).Required()).
	Field("object", builders.String().Pattern(`^syncver-status$`).Required()).
	Field("schema_version", builders.Number().Required()).
	Field("data", statusSchema.Required()).
	Field("generated_at", builders.String().Required())

// Validate confirms the envelope report.EmitJSON is about to write for
// this status actually matches the declared schema -- validating the
// real JSON bytes via a marshal/unmarshal round trip, not the Go
// struct that produced them, so a marshaling bug is exactly the kind
// of thing this catches rather than something it takes on faith.
func Validate(status SyncStatus) error {
	m, err := report.AsMap("syncver", "syncver-status", SchemaVersion, status)
	if err != nil {
		return err
	}
	return qf.Validate(m, envelopeSchema)
}
