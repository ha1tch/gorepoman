package register

import (
	"bytes"
	"embed"
	"html/template"
)

//go:embed templates/list.html.tmpl templates/detail.html.tmpl templates/check.html.tmpl templates/kanban.html.tmpl
var templateFS embed.FS

var listTmpl = template.Must(template.ParseFS(templateFS, "templates/list.html.tmpl"))
var detailTmpl = template.Must(template.ParseFS(templateFS, "templates/detail.html.tmpl"))
var checkTmpl = template.Must(template.ParseFS(templateFS, "templates/check.html.tmpl"))
var kanbanTmpl = template.Must(template.ParseFS(templateFS, "templates/kanban.html.tmpl"))

func renderListHTML(data []ItemSummary) (string, error) {
	var b bytes.Buffer
	if err := listTmpl.Execute(&b, data); err != nil {
		return "", err
	}
	return b.String(), nil
}

func renderDetailHTML(data ItemDetail) (string, error) {
	var b bytes.Buffer
	if err := detailTmpl.Execute(&b, data); err != nil {
		return "", err
	}
	return b.String(), nil
}

func renderCheckHTML(data CheckResult) (string, error) {
	var b bytes.Buffer
	if err := checkTmpl.Execute(&b, data); err != nil {
		return "", err
	}
	return b.String(), nil
}

func renderKanbanHTML(data KanbanReport) (string, error) {
	var b bytes.Buffer
	if err := kanbanTmpl.Execute(&b, data); err != nil {
		return "", err
	}
	return b.String(), nil
}
