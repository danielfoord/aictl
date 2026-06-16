package handoff

import (
	_ "embed"
	"text/template"
)

//go:embed handoff.tmpl
var handoffTemplate string

// tmpl is parsed once at init. The template ranges only over slices (fixed
// order) and reads only struct fields, keeping rendering deterministic.
var tmpl = template.Must(template.New("handoff").Parse(handoffTemplate))
