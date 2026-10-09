package config

import (
	"maps"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/demola234/tiny-tools/mock_machina/internal/model"
	"github.com/demola234/tiny-tools/mock_machina/internal/tmpl"
)

func (l *loader) templates(file string, f map[string]*yaml.Node, st *model.State) {
	if v := f["body"]; v != nil {
		if msg := BodyTemplateProblem(st.Body); msg != "" {
			where := "body"
			if st.Body.File != "" {
				where += " " + st.Body.File
			}
			l.add(file, v.Line, "%s: %s", where, msg)
		}
	}
	if v := f["headers"]; v != nil {
		for _, name := range slices.Sorted(maps.Keys(st.Headers)) {
			if _, err := tmpl.Parse(st.Headers[name]); err != nil {
				l.add(file, v.Line, "header %s: %v", name, err)
			}
		}
	}
}

func BodyTemplateProblem(b model.Body) string {
	if !tmpl.Has(b.Data) {
		return ""
	}
	var err error
	switch {
	case strings.HasPrefix(b.ContentType, jsonType):
		_, err = tmpl.ParseJSON(b.Data)
	case strings.HasPrefix(b.ContentType, "text/"):
		_, err = tmpl.Parse(string(b.Data))
	}
	if err != nil {
		return err.Error()
	}
	return ""
}
