// Package workspace implements cross-project workspace membership
// (repoman workspace join/leave/list) and the board definitions a
// workspace can host -- both are plain, queryfy-validated JSON
// objects living in a workspace repo, not a new kind of state gorepoman
// invents machinery to manage. Read access to a workspace repo is
// free (an ordinary git clone); write access is whatever GitHub
// permissions already grant -- this package does not reason about
// either, only about the shape of the files themselves.
package workspace

import (
	"encoding/json"
	"fmt"

	qf "github.com/ha1tch/queryfy"
	"github.com/ha1tch/queryfy/builders"
)

// marshalThen round-trips v through JSON into a map[string]interface{}
// -- validating the actual bytes about to be written, not just the Go
// struct that produced them, same discipline pkg/report.AsMap uses
// for command output. Standalone config files like these have no
// tool/object/generated_at envelope to wrap -- this is the same idea
// without one.
func marshalThen(v interface{}) (map[string]interface{}, error) {
	buf, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var m map[string]interface{}
	if err := json.Unmarshal(buf, &m); err != nil {
		return nil, err
	}
	return m, nil
}

const ParticipantsSchemaVersion = 1
const BoardSchemaVersion = 1

// Participants is a workspace's own roster: which projects belong to
// it, and -- per-project, not workspace-wide, since different
// participants may want different default views -- which board each
// one sees by default when none is named explicitly.
type Participants struct {
	Schema        string            `json:"schema"`
	Workspace     string            `json:"workspace"`
	Projects      []string          `json:"projects"`
	DefaultBoards map[string]string `json:"default_boards,omitempty"`
}

var participantsSchema = builders.Object().
	Field("schema", builders.String().Pattern(`^gorepoman\.workspace\.participants/1$`).Required()).
	Field("workspace", builders.String().Required()).
	Field("projects", builders.Array().Of(builders.String()).Required()).
	Field("default_boards", builders.Object().AllowAdditional(true))

// ValidateParticipants checks a decoded Participants value against
// its schema -- via the same marshal-the-actual-bytes discipline used
// everywhere else in gorepoman, not just checking the Go struct.
func ValidateParticipants(p Participants) error {
	buf, err := marshalThen(p)
	if err != nil {
		return err
	}
	return qf.Validate(buf, participantsSchema)
}

// BoardColumn is one column in a board's own configured sequence.
// Values names which axis-field values this column collects -- for
// dependency_order this must be empty (position IS the value: column
// i collects tier i); for theme it is required (themes have no
// natural order to fall back on); for status/priority it is optional,
// falling back to each axis's own natural order by position when
// absent. An item matching no column's values (or, for
// dependency_order, deeper than the configured columns) goes to
// Overflow rather than disappearing.
type BoardColumn struct {
	Label  string   `json:"label"`
	Note   string   `json:"note"`
	Values []string `json:"values,omitempty"`
}

// BoardOverflow is the catch-all column for anything that does not
// fit the configured columns -- only relevant to axes whose bucket
// count can't be known in advance. Hidden persists across renders,
// same principle as waveprogress's own wave-visibility state: it
// changes display, never what's counted. Tracked is the record
// `--empty` clears -- items currently believed to be in overflow, not
// a second copy of real item data (nothing is ever written back
// there; clearing it just means the next render starts from nothing
// rather than carrying forward entries for items that are not there
// anymore).
type BoardOverflow struct {
	Hidden  bool     `json:"hidden"`
	Tracked []string `json:"tracked,omitempty"`
}

// BoardLegendEntry is a named, coloured note a board can attach to a
// card whose data matches AppliesWhen -- a simple field:value match
// against the item's own fields (e.g. {"status": "☑"}), proposed as
// the simplest shape that fits; not yet confirmed beyond that
// proposal. Legend colours are deliberately a separate palette from
// colours that already carry fixed meaning elsewhere (green=done,
// amber=partial, grey=not-started) so a legend note is never
// mistaken for one of those.
type BoardLegendEntry struct {
	Name        string                 `json:"name"`
	Color       string                 `json:"color"`
	AppliesWhen map[string]interface{} `json:"applies_when"`
}

// BoardScope narrows a board to a subset of the workspace's own
// participants. Absent/empty means every current participant.
type BoardScope struct {
	Projects []string `json:"projects,omitempty"`
}

// Board is one board definition -- a representation of tracking
// already kept elsewhere (the register, wave data), never a second
// system of record. Subtitle and Footer are optional, hand-authored
// markdown; nothing here is auto-generated from them.
type Board struct {
	Schema   string             `json:"schema"`
	Name     string             `json:"name"`
	Title    string             `json:"title"`
	Subtitle string             `json:"subtitle"`
	Footer   string             `json:"footer"`
	Scope    BoardScope         `json:"scope"`
	Axis     string             `json:"axis"`
	Columns  []BoardColumn      `json:"columns,omitempty"`
	Overflow BoardOverflow      `json:"overflow"`
	Legend   []BoardLegendEntry `json:"legend,omitempty"`
}

var legendColorPattern = `^(cyan|purple|orange|brown|blue)$`

var boardColumnSchema = builders.Object().
	Field("label", builders.String().Required()).
	Field("note", builders.String()).
	Field("values", builders.Array().Of(builders.String()))

var boardOverflowSchema = builders.Object().
	Field("hidden", builders.Bool()).
	Field("tracked", builders.Array().Of(builders.String()))

var boardLegendEntrySchema = builders.Object().
	Field("name", builders.String().Required()).
	Field("color", builders.String().Pattern(legendColorPattern).Required()).
	Field("applies_when", builders.Object().AllowAdditional(true).Required())

var boardScopeSchema = builders.Object().
	Field("projects", builders.Array().Of(builders.String()))

var boardSchema = builders.Object().
	Field("schema", builders.String().Pattern(`^gorepoman\.workspace\.board/1$`).Required()).
	Field("name", builders.String().Required()).
	Field("title", builders.String()).
	Field("subtitle", builders.String()).
	Field("footer", builders.String()).
	Field("scope", boardScopeSchema).
	Field("axis", builders.String().Pattern(`^(status|priority|theme|dependency_order)$`).Required()).
	Field("columns", builders.Array().Of(boardColumnSchema)).
	Field("overflow", boardOverflowSchema).
	Field("legend", builders.Array().Of(boardLegendEntrySchema))

// Issue is a request one project files against another, living in
// the target's own issues/<filer>/ directory until resolved, paused,
// or dropped. The id format is <filer>:<prefix>-<n> -- the prefix
// carries no provenance on its own (every project may collide on the
// same prefix); the id own leading segment is what identifies the
// filer.
type Issue struct {
	Schema string `json:"schema"`
	ID     string `json:"id"`
	Target string `json:"target"`
	Parent string `json:"parent,omitempty"`
	Theme  string `json:"theme"`
	Title  string `json:"title"`
	Filed  string `json:"filed"`
	Body   string `json:"body"`
}

// Closure records an issue resolved into a target project own
// register -- moved into an append-only resolved log, never edited in
// place, same principle as register close moving an item into
// RESOLVED.md rather than editing TRACKING.md in place.
type Closure struct {
	Schema     string `json:"schema"`
	Ref        string `json:"ref"`
	ResolvedAs string `json:"resolved_as"`
	Version    string `json:"version"`
	Date       string `json:"date"`
	Note       string `json:"note,omitempty"`
}

// Pause is the one non-terminal event in an issue lifecycle -- the
// issue stays open, filed, real, just not being actively pursued
// right now. Unlike Closure/Drop it does not move the issue anywhere.
type Pause struct {
	Schema string `json:"schema"`
	Ref    string `json:"ref"`
	Date   string `json:"date"`
	Reason string `json:"reason"`
}

// Drop is terminal, same shape as Closure but for an issue that will
// not be resolved -- moved to an append-only dropped log, never
// silently deleted. Reason is mandatory, same principle as
// provenance sanction own mandatory reason: dropping leaves a
// permanent, attributable trace, not a quieter one than closing.
type Drop struct {
	Schema string `json:"schema"`
	Ref    string `json:"ref"`
	Date   string `json:"date"`
	Reason string `json:"reason"`
}

var issueSchema = builders.Object().
	Field("schema", builders.String().Pattern(`^gorepoman\.workspace\.issue/1$`).Required()).
	Field("id", builders.String().Required()).
	Field("target", builders.String().Required()).
	Field("parent", builders.String()).
	Field("theme", builders.String().Required()).
	Field("title", builders.String().Required()).
	Field("filed", builders.String().Required()).
	Field("body", builders.String().Required())

var closureSchema = builders.Object().
	Field("schema", builders.String().Pattern(`^gorepoman\.workspace\.closure/1$`).Required()).
	Field("ref", builders.String().Required()).
	Field("resolved_as", builders.String().Required()).
	Field("version", builders.String().Required()).
	Field("date", builders.String().Required()).
	Field("note", builders.String())

var pauseSchema = builders.Object().
	Field("schema", builders.String().Pattern(`^gorepoman\.workspace\.pause/1$`).Required()).
	Field("ref", builders.String().Required()).
	Field("date", builders.String().Required()).
	Field("reason", builders.String().Required())

var dropSchema = builders.Object().
	Field("schema", builders.String().Pattern(`^gorepoman\.workspace\.drop/1$`).Required()).
	Field("ref", builders.String().Required()).
	Field("date", builders.String().Required()).
	Field("reason", builders.String().Required())

func ValidateIssue(v Issue) error {
	m, err := marshalThen(v)
	if err != nil {
		return err
	}
	return qf.Validate(m, issueSchema)
}

func ValidateClosure(v Closure) error {
	m, err := marshalThen(v)
	if err != nil {
		return err
	}
	return qf.Validate(m, closureSchema)
}

func ValidatePause(v Pause) error {
	m, err := marshalThen(v)
	if err != nil {
		return err
	}
	return qf.Validate(m, pauseSchema)
}

func ValidateDrop(v Drop) error {
	m, err := marshalThen(v)
	if err != nil {
		return err
	}
	return qf.Validate(m, dropSchema)
}

// ValidateBoard checks a decoded Board value against its schema, plus
// three cross-field rules queryfy's schema builders cannot express on
// their own -- these depend on comparing fields against each other
// and against the axis, not just checking one field's own shape.
func ValidateBoard(b Board) error {
	buf, err := marshalThen(b)
	if err != nil {
		return err
	}
	if err := qf.Validate(buf, boardSchema); err != nil {
		return err
	}

	seen := make(map[string]string) // value -> column label that already claimed it
	for _, col := range b.Columns {
		switch b.Axis {
		case "dependency_order":
			if len(col.Values) > 0 {
				return fmt.Errorf("column %q: values is not allowed on a dependency_order axis -- position IS the value (column i collects tier i)", col.Label)
			}
		case "theme":
			if len(col.Values) == 0 {
				return fmt.Errorf("column %q: values is required on a theme axis -- themes have no natural order to fall back on", col.Label)
			}
		}
		for _, v := range col.Values {
			if other, ok := seen[v]; ok {
				return fmt.Errorf("value %q claimed by both column %q and column %q -- an item cannot belong to two columns", v, other, col.Label)
			}
			seen[v] = col.Label
		}
	}
	return nil
}

// naturalOrder gives the fallback column order for axes where one
// exists (status, priority) and a column omits values -- used only
// when the board author has not been explicit; theme has no
// equivalent, which is exactly why theme requires values.
var naturalOrder = map[string][]string{
	"status":   {"☐", "◐", "☑", "✓", "✗"},
	"priority": {"P1", "P2", "P3", "P4", "P5"},
}

// NaturalOrderFor returns the fallback column order for axis, or nil
// if the axis has none (theme, dependency_order) -- exported so a
// renderer outside this package can auto-derive columns the same way
// ValidateBoard reasons about them.
func NaturalOrderFor(axis string) []string {
	return naturalOrder[axis]
}
