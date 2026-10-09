package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"

	"github.com/demola234/tiny-tools/mock_machina/internal/model"
)

const (
	DirName      = ".mockmachina"
	routesDir    = "routes"
	resourceExt  = ".yaml"
	misspeltExt  = ".yml"
	exampleRoute = "list: { route: GET /users, states: { ok: {} } }"
)

var (
	routeFields = []string{"route", "examples", "summary", "status", "owners", "request", "responses", "active", "mode", "rules", "serve", "crud", "generated", "states"}
	stateFields = []string{"status", "headers", "body", "latency", "fault", "set", "template", "validateRequest", "generated"}
	ownerFields = []string{"backend", "frontend"}
	fileFields  = []string{"owners", "status"}

	requiredRouteFields = []string{"route", "states"}
	requiredFieldHints  = map[string]string{"route": ", like route: GET /users"}

	resourceName = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
)

func Load(dir string) (*model.Project, Problems, error) { return LoadFS(os.DirFS(dir)) }

func LoadFS(fsys fs.FS) (*model.Project, Problems, error) {
	if _, err := fs.Stat(fsys, "."); err != nil {
		var pathErr *fs.PathError
		if errors.As(err, &pathErr) {
			err = pathErr.Err
		}
		return nil, nil, err
	}
	entries, err := fs.ReadDir(fsys, routesDir)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, nil, fmt.Errorf("reading %s: %w", routesDir, err)
	}

	l := &loader{fsys: fsys}
	p := &model.Project{Config: l.config()}
	p.Schemas = l.schemas()
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") || e.IsDir() {
			continue
		}
		switch path.Ext(name) {
		case resourceExt:
			p.Routes = append(p.Routes, l.resource(name)...)
		case misspeltExt:
			l.add(path.Join(routesDir, name), 0, "use the %s extension, so mockmachina reads this file", resourceExt)
		}
	}
	slices.SortFunc(p.Routes, func(a, b *model.Route) int { return strings.Compare(a.ID, b.ID) })
	l.checkDuplicates(p.Routes)
	if l.schemasOK {
		l.checkBodies(p)
	}
	l.probs.sort()
	return p, l.probs, nil
}

type loader struct {
	fsys        fs.FS
	probs       Problems
	data        map[string][]byte
	schemaNames []string
	schemasOK   bool
}

func (l *loader) add(file string, line int, format string, args ...any) {
	l.probs = append(l.probs, Problem{File: file, Line: line, Msg: fmt.Sprintf(format, args...)})
}

type resource struct {
	file   string
	name   string
	owners model.Owners
	status model.Status
}

func (l *loader) resource(fileName string) []*model.Route {
	res := resource{file: path.Join(routesDir, fileName), name: strings.TrimSuffix(fileName, resourceExt)}
	if !resourceName.MatchString(res.name) {
		l.add(res.file, 0, "file name %q must use lowercase letters, digits and dashes", res.name)
		return nil
	}
	root := l.parse(res.file)
	if root == nil {
		return nil
	}
	if root.Kind != yaml.MappingNode {
		l.add(res.file, root.Line, "expected routes by name, like %s", exampleRoute)
		return nil
	}

	var routeNodes [][2]*yaml.Node
	firstLine := map[string]int{}
	for key, val := range pairs(root) {
		name := key.Value
		switch {
		case strings.HasPrefix(name, "x-"):
		case firstLine[name] != 0:
			l.add(res.file, key.Line, "%q is defined twice (first on line %d)", name, firstLine[name])
		case name == "owners":
			res.owners = l.owners(res.file, val)
		case name == "status":
			res.status = l.lifecycle(res.file, val)
		default:
			routeNodes = append(routeNodes, [2]*yaml.Node{key, val})
		}
		if firstLine[name] == 0 {
			firstLine[name] = key.Line
		}
	}
	if len(routeNodes) == 0 {
		l.add(res.file, 0, "no routes in this file")
		return nil
	}
	routes := make([]*model.Route, 0, len(routeNodes))
	for _, n := range routeNodes {
		if r := l.route(res, n[0], n[1]); r != nil {
			routes = append(routes, r)
		}
	}
	return routes
}

func (l *loader) route(res resource, key, node *yaml.Node) *model.Route {
	name := key.Value
	if !resourceName.MatchString(name) {
		l.add(res.file, key.Line, "route name %q must use lowercase letters, digits and dashes", name)
		return nil
	}
	if node.Kind != yaml.MappingNode {
		l.add(res.file, key.Line, "route %q must be a mapping of fields like route and states", name)
		return nil
	}
	r := &model.Route{
		ID:     res.name + "." + name,
		Group:  res.name,
		Owners: res.owners,
		Status: res.status,
		Serve:  model.ServeMock,
		Dir:    path.Join(routesDir, res.name),
		Src:    model.Source{File: res.file, Line: key.Line},
	}
	f := l.fields(res.file, node, routeFields, &r.Extensions)
	for _, field := range requiredRouteFields {
		if f[field] == nil {
			l.add(res.file, key.Line, "route %q needs %q%s", name, field, requiredFieldHints[field])
		}
	}
	l.request(f, r)
	l.examples(f, r)
	l.metadata(f, r)
	l.behavior(f, r)
	l.mode(f, r)
	l.rules(f, r)
	l.crud(f, r)
	l.contract(f, r)
	return r
}

func (l *loader) request(f map[string]*yaml.Node, r *model.Route) {
	v := f["route"]
	if v == nil {
		return
	}
	method, p, _ := strings.Cut(strings.TrimSpace(v.Value), " ")
	p = strings.TrimSpace(p)
	if p == "" {
		l.add(r.Src.File, v.Line, "route %q must be a method and a path, like GET /users", v.Value)
		return
	}
	r.Method, r.Path = model.Method(method), p
	if r.Method == model.MethodCRUD {
		if strings.HasSuffix(p, "}") {
			l.add(r.Src.File, v.Line, "a CRUD route's path names the collection, like CRUD /things; it adds /{id} itself")
		}
	} else {
		l.checkMethod(r.Src.File, v.Line, method)
	}
	if msg := pathProblem(p); msg != "" {
		l.add(r.Src.File, v.Line, "%s", msg)
	}
}

var pathParamName = regexp.MustCompile(`\{([^/{}]+)\}`)

func (l *loader) examples(f map[string]*yaml.Node, r *model.Route) {
	v := f["examples"]
	if v == nil {
		return
	}
	if v.Kind != yaml.MappingNode {
		l.add(r.Src.File, v.Line, "examples must map path parameters to values, like { id: u_1 }")
		return
	}
	var params []string
	for _, m := range pathParamName.FindAllStringSubmatch(r.Path, -1) {
		params = append(params, m[1])
	}
	r.Examples = make(map[string]string, len(v.Content)/2)
	for key, val := range pairs(v) {
		name := key.Value
		switch {
		case !slices.Contains(params, name):
			hint := "it has none"
			if len(params) > 0 {
				hint = closeMatchOr(name, params, "parameters: "+strings.Join(params, ", "))
			}
			l.add(r.Src.File, key.Line, "examples has %q, which isn't a parameter of %s %s (%s)", name, r.Method, r.Path, hint)
		case val.Kind != yaml.ScalarNode:
			l.add(r.Src.File, key.Line, "examples.%s must be a single value, like u_1", name)
		default:
			r.Examples[name] = val.Value
		}
	}
}

func (l *loader) metadata(f map[string]*yaml.Node, r *model.Route) {
	file := r.Src.File
	if v := f["summary"]; v != nil {
		r.Summary = l.oneLine(file, "summary", v)
	}
	if v := f["status"]; v != nil {
		r.Status = l.lifecycle(file, v)
	}
	if v := f["owners"]; v != nil {
		r.Owners = l.owners(file, v)
	}
	if v := f["generated"]; v != nil {
		r.Generated = l.boolean(file, "generated", v)
	}
	if v := f["serve"]; v != nil {
		r.Serve = l.serve(file, v)
	}
}

var serveModes = []string{string(model.ServeMock), string(model.ServeProxy)}

func (l *loader) serve(file string, v *yaml.Node) model.Serve {
	if slices.Contains(serveModes, v.Value) {
		return model.Serve(v.Value)
	}
	l.add(file, v.Line, "serve %q isn't valid (%s)", v.Value, closeMatchOr(v.Value, serveModes, "expected mock or proxy"))
	return model.ServeMock
}

func (l *loader) boolean(file, field string, v *yaml.Node) bool {
	var b bool
	if v.Kind != yaml.ScalarNode || v.Tag != "!!bool" || v.Decode(&b) != nil {
		l.add(file, v.Line, "%s must be true or false", field)
	}
	return b
}

func (l *loader) behavior(f map[string]*yaml.Node, r *model.Route) {
	file := r.Src.File
	if v := f["states"]; v != nil {
		r.States = l.states(file, r.Dir, v)
	}
	v := f["active"]
	if v == nil {
		if len(r.States) > 0 {
			r.Active = r.States[0].Name
		}
		return
	}
	r.Active = v.Value
	if _, ok := r.States.Get(r.Active); !ok && len(r.States) > 0 {
		names := r.States.Names()
		l.add(file, v.Line, "active state %q doesn't exist (%s)", r.Active,
			closeMatchOr(r.Active, names, "states: "+strings.Join(names, ", ")))
	}
}

var yamlLine = regexp.MustCompile(`^yaml: line (\d+): (.*)$`)

func (l *loader) parse(file string) *yaml.Node {
	data, err := fs.ReadFile(l.fsys, file)
	if err != nil {
		l.add(file, 0, "can't read this file: %v", err)
		return nil
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		l.yamlError(file, err)
		return nil
	}
	if len(doc.Content) == 0 {
		l.add(file, 0, "file is empty")
		return nil
	}
	return doc.Content[0]
}

func (l *loader) yamlError(file string, err error) {
	m := yamlLine.FindStringSubmatch(err.Error())
	if m == nil {
		l.add(file, 0, "invalid YAML: %s", strings.TrimPrefix(err.Error(), "yaml: "))
		return
	}
	line, msg := 0, m[2]
	_, _ = fmt.Sscan(m[1], &line)
	if strings.Contains(msg, "found character that cannot start any token") {
		msg = "indent with spaces, not tabs"
	}
	l.add(file, line, "invalid YAML: %s", msg)
}

func (l *loader) fields(file string, node *yaml.Node, known []string, extensions *[]model.Extension) map[string]*yaml.Node {
	found := make(map[string]*yaml.Node, len(known))
	firstLine := make(map[string]int, len(known))
	for key, val := range pairs(node) {
		name := key.Value
		switch {
		case extensions != nil && strings.HasPrefix(name, "x-"):
			var v any
			_ = val.Decode(&v)
			*extensions = append(*extensions, model.Extension{Key: name, Value: v})
		case !slices.Contains(known, name):
			l.add(file, key.Line, "unknown field %q (%s)", name,
				closeMatchOr(name, known, "expected one of: "+strings.Join(known, ", ")))
		case found[name] != nil:
			l.add(file, key.Line, "%q is defined twice (first on line %d)", name, firstLine[name])
		default:
			found[name], firstLine[name] = val, key.Line
		}
	}
	return found
}

func (l *loader) oneLine(file, field string, v *yaml.Node) string {
	if v.Kind != yaml.ScalarNode || strings.Contains(v.Value, "\n") {
		l.add(file, v.Line, "%s must be one line of text", field)
		return ""
	}
	return v.Value
}

func (l *loader) lifecycle(file string, v *yaml.Node) model.Status {
	s := model.Status(v.Value)
	if s.Valid() {
		return s
	}
	valid := make([]string, 0, len(model.Statuses()))
	for _, st := range model.Statuses() {
		valid = append(valid, string(st))
	}
	if _, err := strconv.Atoi(v.Value); err == nil {
		l.add(file, v.Line, "status %q isn't valid here: HTTP status codes go inside a state (route status is one of: %s)",
			v.Value, strings.Join(valid, ", "))
		return s
	}
	l.add(file, v.Line, "status %q isn't valid (%s)", v.Value,
		closeMatchOr(v.Value, valid, "expected one of: "+strings.Join(valid, ", ")))
	return s
}

func (l *loader) owners(file string, v *yaml.Node) model.Owners {
	var o model.Owners
	if v.Kind != yaml.MappingNode {
		l.add(file, v.Line, "owners must map backend and frontend to lists of GitHub handles")
		return o
	}
	f := l.fields(file, v, ownerFields, nil)
	for name, handles := range map[string]*[]string{"backend": &o.Backend, "frontend": &o.Frontend} {
		if n := f[name]; n != nil && n.Decode(handles) != nil {
			l.add(file, n.Line, "owners.%s must be a list of GitHub handles, like [ademola]", name)
		}
	}
	return o
}

func (l *loader) checkMethod(file string, line int, method string) {
	if msg := methodProblem(method); msg != "" {
		l.add(file, line, "%s", msg)
	}
}

func methodProblem(method string) string {
	switch {
	case model.Method(method).Valid():
		return ""
	case model.Method(strings.ToUpper(method)).Valid():
		return fmt.Sprintf("method %q must be uppercase: %s", method, strings.ToUpper(method))
	}
	methods := make([]string, 0, len(model.Methods()))
	for _, m := range model.Methods() {
		methods = append(methods, string(m))
	}
	return fmt.Sprintf("method %q isn't an HTTP method (expected one of: %s)", method, strings.Join(methods, ", "))
}

func (l *loader) states(file, dir string, node *yaml.Node) model.States {
	if node.Kind != yaml.MappingNode {
		l.add(file, node.Line, "%q must map state names to their responses", "states")
		return nil
	}
	if len(node.Content) == 0 {
		l.add(file, node.Line, "%q needs at least one state", "states")
		return nil
	}
	var states model.States
	for key, val := range pairs(node) {
		name := key.Value
		switch {
		case model.IsYAMLKeyword(name):
			l.add(file, key.Line, "state %q is a YAML keyword; pick another name", name)
		case !model.ValidStateName(name):
			l.add(file, key.Line, "state %q must use lowercase letters, digits and underscores, starting with a letter", name)
		}
		states = append(states, model.NamedState{Name: name, State: l.state(file, dir, key, val)})
	}
	return states
}

func (l *loader) state(file, dir string, key, node *yaml.Node) *model.State {
	st := &model.State{Src: model.Source{File: file, Line: key.Line}}
	if isNull(node) {
		return st
	}
	if node.Kind != yaml.MappingNode {
		l.add(file, key.Line, "state %q must be a mapping of fields like status and body", key.Value)
		return st
	}
	f := l.fields(file, node, stateFields, &st.Extensions)
	if v := f["status"]; v != nil {
		l.status(file, v, st)
	}
	if v := f["headers"]; v != nil {
		if err := v.Decode(&st.Headers); err != nil {
			l.add(file, v.Line, "headers must map names to values, like { Cache-Control: no-store }")
		}
	}
	if v := f["body"]; v != nil {
		st.Body = l.body(file, dir, v)
	}
	if v := f["latency"]; v != nil {
		l.latency(file, v, st)
	}
	if v := f["fault"]; v != nil {
		st.Fault = l.fault(file, v)
	}
	if v := f["set"]; v != nil {
		st.Set = l.set(file, v)
	}
	if v := f["generated"]; v != nil {
		st.Generated = l.boolean(file, "generated", v)
	}
	if v := f["validateRequest"]; v != nil {
		st.SkipRequestValidation = !l.boolean(file, "validateRequest", v)
	}
	if v := f["template"]; v != nil {
		st.Verbatim = !l.boolean(file, "template", v)
	}
	if !st.Verbatim {
		l.templates(file, f, st)
	}
	return st
}

const maxLatency = time.Minute

var latencyFields = []string{"base", "jitter"}

func (l *loader) latency(file string, v *yaml.Node, st *model.State) {
	if v.Kind != yaml.MappingNode {
		st.Latency = l.duration(file, "latency", v)
		return
	}
	f := l.fields(file, v, latencyFields, nil)
	if f["base"] == nil {
		l.add(file, v.Line, "latency needs a base, like { base: 2s, jitter: 500ms }")
		return
	}
	st.Latency = l.duration(file, "latency base", f["base"])
	if j := f["jitter"]; j != nil {
		st.Jitter = l.duration(file, "latency jitter", j)
	}
	if st.Latency+st.Jitter > maxLatency {
		l.add(file, v.Line, "latency %v ± %v can reach %v, over the 1m limit", st.Latency, st.Jitter, st.Latency+st.Jitter)
	}
}

func (l *loader) duration(file, label string, v *yaml.Node) time.Duration {
	if _, err := strconv.ParseFloat(v.Value, 64); err == nil {
		l.add(file, v.Line, "%s %q has no unit; write %sms", label, v.Value, v.Value)
		return 0
	}
	d, err := time.ParseDuration(v.Value)
	switch {
	case err != nil:
		l.add(file, v.Line, "%s %q isn't a duration, like 250ms or 2s", label, v.Value)
	case d < 0:
		l.add(file, v.Line, "%s %q can't be negative", label, v.Value)
	case d > maxLatency:
		l.add(file, v.Line, "%s %q is over the 1m limit", label, v.Value)
	default:
		return d
	}
	return 0
}

const (
	minStatus = 100
	maxStatus = 599
)

func (l *loader) status(file string, v *yaml.Node, st *model.State) {
	if err := v.Decode(&st.Status); err != nil {
		l.add(file, v.Line, "status must be a number, like 200")
		return
	}
	if st.Status < minStatus || st.Status > maxStatus {
		l.add(file, v.Line, "status %d isn't an HTTP status code (%d-%d)", st.Status, minStatus, maxStatus)
	}
}

func (l *loader) body(file, dir string, node *yaml.Node) model.Body {
	if isNull(node) {
		return model.Body{}
	}
	if node.Kind == yaml.ScalarNode && node.Value == "generate" {
		return model.Body{Generate: true, ContentType: jsonType}
	}
	if node.Kind != yaml.ScalarNode {
		data, err := toJSON(node)
		if err != nil {
			l.add(file, node.Line, "body can't be written as JSON: %v", err)
		}
		return model.Body{Data: data, ContentType: jsonType}
	}
	name := path.Join(dir, node.Value)
	if !fs.ValidPath(name) {
		l.add(file, node.Line, "body %q points outside the project", node.Value)
		return model.Body{}
	}
	data, err := fs.ReadFile(l.fsys, name)
	if err != nil {
		l.add(file, node.Line, "body file %q not found in %s", node.Value, dir)
		return model.Body{}
	}
	return model.Body{File: node.Value, Data: data, ContentType: contentType(node.Value)}
}

func (l *loader) checkDuplicates(routes []*model.Route) {
	owners := make(map[string]string, len(routes))
	for _, r := range routes {
		if r.Method == "" || r.Path == "" {
			continue
		}
		key := r.Key()
		if first, taken := owners[key]; taken {
			l.add(r.Src.File, r.Src.Line, "%s is already defined by route %q", key, first)
			continue
		}
		owners[key] = r.ID
	}
}

func isNull(node *yaml.Node) bool { return node.Kind == yaml.ScalarNode && node.Tag == "!!null" }

func pairs(node *yaml.Node) func(yield func(key, val *yaml.Node) bool) {
	return func(yield func(key, val *yaml.Node) bool) {
		for i := 0; i+1 < len(node.Content); i += 2 {
			if !yield(node.Content[i], node.Content[i+1]) {
				return
			}
		}
	}
}
