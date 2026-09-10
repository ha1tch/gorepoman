package gomod

import (
	"bytes"
	"embed"
	"html/template"
)

//go:embed templates/check.html.tmpl
var templateFS embed.FS

var checkTmpl = template.Must(template.ParseFS(templateFS, "templates/check.html.tmpl"))

func renderCheckHTML(result CheckResult) (string, error) {
	var b bytes.Buffer
	if err := checkTmpl.Execute(&b, result); err != nil {
		return "", err
	}
	return b.String(), nil
}
