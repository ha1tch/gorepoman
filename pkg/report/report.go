// Package report gives every gorepoman subcommand that reports on the
// state of something it manages -- waves, the register, guards,
// version sync, gomod/badcode checks -- one uniform way to emit that
// state, instead of each command inventing its own output vocabulary.
//
// Two formats, one job apiece:
//
//   - EmitJSON writes a single Envelope as the data contract for other
//     systems (scripts, dashboards, a future cross-project rollup).
//     It carries no presentation information at all -- layout is the
//     consumer's problem, not gorepoman's.
//   - EmitHTML wraps a subcommand-supplied content fragment in the
//     same self-contained visual shell waveprogress --html already
//     established (see pkg/waveprogress's htmlHead/htmlTail), so every
//     subcommand's --format html output belongs to one family instead
//     of each drawing its own page. This is the deterministic path --
//     an agent session with shell access to repoman should relay this
//     output directly rather than re-deriving a rendering from
//     --format json each time.
//
// Error contract for both: on any failure, nothing is written to w --
// never a partial or differently-shaped payload. Callers must check
// their own error path BEFORE calling either Emit function; report
// never gets to see an error state, it only ever emits success.
package report

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"os"
	"time"
)

// Envelope is the uniform wrapper every reporting subcommand emits
// under --format json.
//
//   - Tool is the subcommand name ("syncver", "waveprogress", ...).
//   - Object fully disambiguates the shape of Data on its own -- e.g.
//     "syncver-status" vs "register-list" vs "register-item" -- so a
//     consumer never has to inspect Data just to know what kind of
//     thing it's holding.
//   - SchemaVersion is scoped to this Tool+Object pair alone. Bump it
//     only when THIS shape changes -- one command growing a field
//     must never force every other command's consumers to re-check
//     their parsing.
type Envelope struct {
	Tool          string      `json:"tool"`
	Object        string      `json:"object"`
	SchemaVersion int         `json:"schema_version"`
	Data          interface{} `json:"data"`
	GeneratedAt   string      `json:"generated_at"`
}

func newEnvelope(tool, object string, version int, data interface{}) Envelope {
	return Envelope{
		Tool:          tool,
		Object:        object,
		SchemaVersion: version,
		Data:          data,
		// RFC3339 in UTC, always -- a pinned format, not whatever
		// time.Now() happens to stringify to on this machine's
		// locale. Two runs of the same command, anywhere, must
		// produce identically-shaped timestamps.
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
	}
}

// EmitOrErr converts an Emit* call's result into the exit code every
// reporting subcommand's json/html branches should return: 0 on
// success, or 1 after printing the error to stderr on failure. One
// shared conversion instead of each subcommand hand-rolling its own
// "if err != nil { print; return 1 }" after every Emit call.
func EmitOrErr(err error) int {
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	return 0
}

// AsMap round-trips an Envelope through JSON into a map[string]interface{} into a map[string]interface{}
// -- the shape queryfy's builders validate against. This validates the
// actual bytes about to be emitted, not the Go struct that produced
// them, which is the stronger guarantee: a bug in how a field marshals
// is exactly the kind of thing this is meant to catch.
func AsMap(tool, object string, version int, data interface{}) (map[string]interface{}, error) {
	env := newEnvelope(tool, object, version, data)
	buf, err := json.Marshal(env)
	if err != nil {
		return nil, err
	}
	var m map[string]interface{}
	if err := json.Unmarshal(buf, &m); err != nil {
		return nil, err
	}
	return m, nil
}

// EmitJSON writes the envelope as a single, complete JSON document to
// w. Built into a buffer first and written in one call -- nothing
// partial reaches w if marshaling fails partway through.
func EmitJSON(w io.Writer, tool, object string, version int, data interface{}) error {
	env := newEnvelope(tool, object, version, data)
	// Built into a buffer via an Encoder (not MarshalIndent) so
	// SetEscapeHTML(false) can be set -- this is a data contract for
	// other systems, not markup ever destined for an HTML context, so
	// plain "<", ">", "&" are preferable to their \u003c-style escapes.
	// Still built fully in memory before the single Write -- nothing
	// partial reaches w if encoding fails partway through.
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(env); err != nil {
		return err
	}
	_, err := w.Write(buf.Bytes())
	return err
}

// ExtractFormat scans args for --format VALUE or --format=VALUE (in
// either order, anywhere in args -- not just first position, the
// same convention webhelp.NormalizeBriefFirst already established
// for --brief) and returns the requested format plus args with that
// flag removed. Absent, it defaults to "text" -- every reporting
// subcommand's existing output, unchanged. This is the one place
// this parsing lives; every reporting subcommand calls it instead of
// re-deriving its own copy.
//
// Valid values are "text" (default), "json", and "html" -- callers
// reject anything else themselves, since the right error message
// ("unknown format %q for this command") is the calling subcommand's
// job, not this helper's.
func ExtractFormat(args []string) (format string, rest []string) {
	format = "text"
	rest = make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--format" && i+1 < len(args):
			format = args[i+1]
			i++
		case len(a) > len("--format=") && a[:len("--format=")] == "--format=":
			format = a[len("--format="):]
		default:
			rest = append(rest, a)
		}
	}
	return format, rest
}

// shellHead/shellTail are the same CSS variables and page structure and page structure
// waveprogress's own htmlHead/htmlTail already established -- deliberately
// duplicated here rather than imported (pkg/waveprogress's constants are
// unexported, and pulling report into that package to reach them would
// invert the dependency direction every other reporting command needs).
// If either copy changes, change both -- they are meant to render
// identically.
const shellHead = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{title}</title>
<style>
:root {
  --bg: #ffffff; --surface: #f4f3f1; --text: #0b0b0b; --text-2: #52514e; --text-3: #898781;
  --border: #e1e0d9;
  --success: #0ca30c;
  --warn-track: #fab219; --warn-bg: #faeeda; --warn-text: #854f0b;
  --acc-track: #2a78d6; --acc-bg: #e6f1fb; --acc-text: #0c447c;
  --muted-track: #d3d1c7;
}
@media (prefers-color-scheme: dark) {
  :root {
    --bg: #16161a; --surface: #232327; --text: #f0efec; --text-2: #c3c2b7; --text-3: #898781;
    --border: #333338;
    --success: #2ed92e;
    --warn-track: #fab219; --warn-bg: #4a3417; --warn-text: #f5c876;
    --acc-track: #4a95e6; --acc-bg: #163a5c; --acc-text: #a9d1f7;
    --muted-track: #3a3a3e;
  }
}
* { box-sizing: border-box; }
body {
  font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Helvetica, Arial, sans-serif;
  background: var(--bg); color: var(--text); margin: 0; padding: 2rem 1.25rem;
}
.wrap { max-width: 720px; margin: 0 auto; }
h1 { font-size: 15px; font-weight: 600; margin: 0 0 1.25rem; color: var(--text-2); }
.meta { font-size: 11px; color: var(--text-3); border-top: 1px solid var(--border); margin-top: 1.5rem; padding-top: 10px; }
</style>
</head>
<body>
<div class="wrap">
`

const shellTail = `</div>
</body>
</html>
`

// EmitHTML wraps body -- an already-rendered inner-content fragment
// produced by the calling subcommand's own embedded template -- in
// the shared shell above. report never sees the subcommand's typed
// data; it only knows title/tool/object/body, so adding a field to
// one command's payload can never require a change here.
func EmitHTML(w io.Writer, title, tool, object, body string) error {
	var b bytes.Buffer
	b.WriteString(bytesReplaceTitle(shellHead, title))
	fmt.Fprintf(&b, "<h1>%s</h1>\n", template.HTMLEscapeString(title))
	b.WriteString(body)
	fmt.Fprintf(&b, "<p class=\"meta\">%s &middot; %s &middot; generated %s</p>\n",
		template.HTMLEscapeString(tool),
		template.HTMLEscapeString(object),
		template.HTMLEscapeString(time.Now().UTC().Format(time.RFC3339)))
	b.WriteString(shellTail)
	_, err := w.Write(b.Bytes())
	return err
}

func bytesReplaceTitle(head, title string) string {
	esc := template.HTMLEscapeString(title)
	out := make([]byte, 0, len(head)+len(esc))
	for i := 0; i < len(head); i++ {
		if i+7 <= len(head) && head[i:i+7] == "{title}" {
			out = append(out, esc...)
			i += 6
			continue
		}
		out = append(out, head[i])
	}
	return string(out)
}
