package waveprogress

import (
	"bytes"
	"embed"
	"html/template"
)

//go:embed templates/report.html.tmpl
var templateFS embed.FS

var reportTmpl = template.Must(template.ParseFS(templateFS, "templates/report.html.tmpl"))

func renderReportHTML(data WaveReport) (string, error) {
	var b bytes.Buffer
	if err := reportTmpl.Execute(&b, data); err != nil {
		return "", err
	}
	return b.String(), nil
}
