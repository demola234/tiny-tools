package config

import (
	"encoding/json"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/demola234/tiny-tools/mock_machina/internal/model"
)

const (
	firstErrorStatus = 400
	firstServerError = 500
	notFound         = 404
	reviewGenerated  = ` was written by an AI assistant; review it, then delete "generated: true"`
)

func Warnings(p *model.Project) Problems {
	var warnings Problems
	for _, r := range p.Routes {
		for _, msg := range routeWarnings(r) {
			warnings = append(warnings, Problem{File: r.Src.File, Line: r.Src.Line, Severity: Warning, Msg: r.ID + msg})
		}
		for _, ns := range r.States {
			if ns.State.Generated {
				warnings = append(warnings, Problem{
					File: ns.State.Src.File, Line: ns.State.Src.Line, Severity: Warning,
					Msg: r.ID + ` state "` + ns.Name + `"` + reviewGenerated,
				})
			}
		}
	}
	return warnings
}

func routeWarnings(r *model.Route) []string {
	var msgs []string
	if r.Summary == "" {
		msgs = append(msgs, " has no summary; add one line saying what it returns")
	}
	if r.Method != model.MethodCRUD {
		msgs = append(msgs, coverageWarnings(r)...)
	}
	if r.Owners.IsZero() {
		msgs = append(msgs, " has no owners; reviews can't be routed")
	}
	if r.Generated {
		msgs = append(msgs, reviewGenerated)
	}
	for _, resp := range r.Responses {
		if resp.Schema.Inline != nil && strings.Contains(string(resp.Schema.Inline.JSON), `"x-mockmachina-inferred":true`) {
			msgs = append(msgs, " response "+resp.Status+" has a schema guessed from examples; check its required and nullable fields, then delete x-mockmachina-inferred")
		}
	}
	for _, resp := range r.Responses {
		if resp.Status != "default" && !slices.ContainsFunc(r.States, func(ns model.NamedState) bool {
			return strconv.Itoa(ns.State.EffectiveStatus()) == resp.Status
		}) {
			msgs = append(msgs, " documents "+resp.Status+" but no state returns it")
		}
	}
	return msgs
}

func coverageWarnings(r *model.Route) []string {
	has := func(match func(status int) bool) bool {
		return slices.ContainsFunc(r.States, func(ns model.NamedState) bool { return match(ns.State.EffectiveStatus()) })
	}
	if !has(func(s int) bool { return s >= firstErrorStatus }) {
		return []string{" has no 4xx or 5xx state; apps can't test errors"}
	}
	var msgs []string
	segments := strings.Split(r.Path, "/")
	last := segments[len(segments)-1]
	if r.Method == model.MethodGet && !strings.HasPrefix(last, "{") && listsWithoutEmpty(r.States) {
		msgs = append(msgs, " has no state with an empty list; apps can't test their empty screen")
	}
	if param := pathParamName.FindString(r.Path); param != "" && !has(func(s int) bool { return s == notFound }) {
		msgs = append(msgs, " has no 404 state; apps can't test an unknown "+param)
	}
	writes := []model.Method{model.MethodPost, model.MethodPut, model.MethodPatch}
	if slices.Contains(writes, r.Method) && !has(func(s int) bool { return s >= firstErrorStatus && s < firstServerError }) {
		msgs = append(msgs, " has no 4xx state; apps can't test invalid input")
	}
	return msgs
}

func listsWithoutEmpty(states model.States) bool {
	lists, empty := false, false
	for _, ns := range states {
		if ns.State.EffectiveStatus() >= firstErrorStatus || !strings.HasPrefix(ns.State.Body.ContentType, jsonType) {
			continue
		}
		var body any
		if json.Unmarshal(ns.State.Body.Data, &body) != nil {
			continue
		}
		values := []any{body}
		if obj, ok := body.(map[string]any); ok {
			values = slices.Collect(maps.Values(obj))
		}
		for _, v := range values {
			if list, ok := v.([]any); ok {
				lists = true
				empty = empty || len(list) == 0
			}
		}
	}
	return lists && !empty
}
