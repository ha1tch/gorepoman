package guards

import (
	"bytes"
	"embed"
	"html/template"
)

//go:embed templates/list.html.tmpl templates/detail.html.tmpl templates/stale.html.tmpl
var templateFS embed.FS

var listTmpl = template.Must(template.ParseFS(templateFS, "templates/list.html.tmpl"))
var detailTmpl = template.Must(template.ParseFS(templateFS, "templates/detail.html.tmpl"))
var staleTmpl = template.Must(template.ParseFS(templateFS, "templates/stale.html.tmpl"))

func renderListHTML(data []GuardSummary) (string, error) {
	var b bytes.Buffer
	if err := listTmpl.Execute(&b, data); err != nil {
		return "", err
	}
	return b.String(), nil
}

func renderDetailHTML(data GuardDetail) (string, error) {
	var b bytes.Buffer
	if err := detailTmpl.Execute(&b, data); err != nil {
		return "", err
	}
	return b.String(), nil
}

func renderStaleHTML(data StaleResult) (string, error) {
	var b bytes.Buffer
	if err := staleTmpl.Execute(&b, data); err != nil {
		return "", err
	}
	return b.String(), nil
}
