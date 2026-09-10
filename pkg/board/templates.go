package board

import (
	"bytes"
	"embed"
	"html/template"
)

//go:embed templates/report.html.tmpl templates/definition.html.tmpl
var templateFS embed.FS

var reportTmpl = template.Must(template.ParseFS(templateFS, "templates/report.html.tmpl"))
var definitionTmpl = template.Must(template.ParseFS(templateFS, "templates/definition.html.tmpl"))

func renderReportHTML(data BoardReport) (string, error) {
	var b bytes.Buffer
	if err := reportTmpl.Execute(&b, data); err != nil {
		return "", err
	}
	return b.String(), nil
}

func renderDefinitionHTML(data DefinitionReport) (string, error) {
	var b bytes.Buffer
	if err := definitionTmpl.Execute(&b, data); err != nil {
		return "", err
	}
	return b.String(), nil
}
