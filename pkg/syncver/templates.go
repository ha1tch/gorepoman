package syncver

import (
	"bytes"
	"embed"
	"html/template"
)

//go:embed templates/status.html.tmpl
var templateFS embed.FS

var statusTmpl = template.Must(template.ParseFS(templateFS, "templates/status.html.tmpl"))

// renderStatusHTML executes the embedded template against status and
// returns the inner-content fragment for report.EmitHTML to wrap. The
// template is compiled into the binary at build time via embed.FS --
// gorepoman's own version is the template's version, no separate file
// on disk that can drift out of sync with the binary that reads it.
func renderStatusHTML(status SyncStatus) (string, error) {
	var b bytes.Buffer
	if err := statusTmpl.Execute(&b, status); err != nil {
		return "", err
	}
	return b.String(), nil
}
