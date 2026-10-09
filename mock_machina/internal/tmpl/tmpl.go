package tmpl

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/demola234/tiny-tools/mock_machina/internal/fake"
	"github.com/demola234/tiny-tools/mock_machina/internal/match"
	"github.com/demola234/tiny-tools/mock_machina/internal/model"
	"github.com/demola234/tiny-tools/mock_machina/internal/seed"
	"github.com/demola234/tiny-tools/mock_machina/internal/suggest"
)

type Env struct {
	Facts  match.Facts
	Rand   *rand.Rand
	Now    time.Time
	Locale string
}

type Text struct {
	parts []part
}

type part struct {
	lit  string
	expr *expr
}

type expr struct {
	scope model.Scope
	key   string
	gen   *generator
	args  []int
}

type generator struct {
	name   string
	nargs  int
	usage  string
	random bool
	eval   func(env Env, args []int) any
}

var generators = append([]*generator{
	{name: "uuid", random: true, eval: func(env Env, _ []int) any { return seed.UUID(env.Rand) }},
	{name: "now", eval: func(env Env, _ []int) any { return env.Now.UTC().Format(time.RFC3339) }},
	{name: "now.unix", eval: func(env Env, _ []int) any { return float64(env.Now.Unix()) }},
	{
		name: "random.int", nargs: 2, random: true, usage: "random.int needs two whole numbers, like {{ random.int 1 100 }}",
		eval: func(env Env, a []int) any { return float64(a[0] + env.Rand.IntN(a[1]-a[0]+1)) },
	},
}, fakeGenerators()...)

func fakeGenerators() []*generator {
	names := fake.Names()
	gens := make([]*generator, len(names))
	for i, name := range names {
		gens[i] = &generator{name: "fake." + name, random: true, eval: func(env Env, _ []int) any {
			v, _ := fake.Generate(name, env.Locale, env.Rand, env.Now)
			return v
		}}
	}
	return gens
}

var nameHints = map[model.Scope]string{
	model.ScopePath: "path.id", model.ScopeQuery: "query.page", model.ScopeHeader: "header.authorization",
	model.ScopeCookie: "cookie.session", model.ScopeBody: "body.email", model.ScopeVar: "var.plan",
}

func Parse(s string) (Text, error) {
	var t Text
	for {
		i := strings.Index(s, "{{")
		if i < 0 {
			if s != "" {
				t.parts = append(t.parts, part{lit: s})
			}
			return t, nil
		}
		if i > 0 {
			t.parts = append(t.parts, part{lit: s[:i]})
		}
		rest := s[i+2:]
		j := strings.Index(rest, "}}")
		if j < 0 {
			return Text{}, fmt.Errorf("template %q has no closing }}", s[i:])
		}
		e, err := parseExpr(rest[:j], s[i:i+j+4])
		if err != nil {
			return Text{}, err
		}
		t.parts = append(t.parts, part{expr: e})
		s = rest[j+2:]
	}
}

func parseExpr(inner, whole string) (*expr, error) {
	fields := strings.Fields(inner)
	if len(fields) == 0 {
		return nil, fmt.Errorf("template %q is empty", whole)
	}
	name, args := fields[0], fields[1:]
	scope, key, dotted := strings.Cut(name, ".")
	if slices.Contains(model.Scopes(), model.Scope(scope)) {
		return requestExpr(name, model.Scope(scope), key, dotted, args)
	}
	for _, g := range generators {
		if g.name == name {
			return generatorExpr(g, args)
		}
	}
	return nil, unknown(name, scope, key, dotted)
}

func requestExpr(name string, scope model.Scope, key string, dotted bool, args []string) (*expr, error) {
	switch {
	case scope == model.ScopeCall && dotted:
		return nil, fmt.Errorf("template %q takes no name; write call", name)
	case scope != model.ScopeCall && key == "":
		return nil, fmt.Errorf("template %q needs a name, like %s", name, nameHints[scope])
	case len(args) > 0:
		return nil, fmt.Errorf("%s takes no arguments", name)
	}
	return &expr{scope: scope, key: key}, nil
}

func generatorExpr(g *generator, args []string) (*expr, error) {
	if g.nargs == 0 {
		if len(args) > 0 {
			return nil, fmt.Errorf("%s takes no arguments", g.name)
		}
		return &expr{gen: g}, nil
	}
	nums := make([]int, 0, len(args))
	for _, a := range args {
		n, err := strconv.Atoi(a)
		if err != nil {
			return nil, fmt.Errorf("%s", g.usage)
		}
		nums = append(nums, n)
	}
	if len(nums) != g.nargs || nums[0] > nums[1] {
		return nil, fmt.Errorf("%s", g.usage)
	}
	return &expr{gen: g, args: nums}, nil
}

func unknown(name, scope, key string, dotted bool) error {
	scopes := make([]string, 0, len(model.Scopes()))
	for _, s := range model.Scopes() {
		scopes = append(scopes, string(s))
	}
	if s, ok := suggest.Closest(scope, scopes); ok && dotted {
		return fmt.Errorf("unknown template %q (did you mean %q?)", name, s+"."+key)
	}
	gens := make([]string, len(generators))
	listed := []string{}
	for i, g := range generators {
		gens[i] = g.name
		if !strings.HasPrefix(g.name, "fake.") {
			listed = append(listed, g.name)
		}
	}
	if s, ok := suggest.Closest(name, gens); ok {
		return fmt.Errorf("unknown template %q (did you mean %q?)", name, s)
	}
	return fmt.Errorf("unknown template %q (templates: path.<name>, query.<name>, header.<name>, cookie.<name>, body.<a.b.0>, var.<name>, call, %s, fake.*)",
		name, strings.Join(listed, ", "))
}

func (t Text) Eval(env Env) any {
	if env.Rand == nil {
		env.Rand = rand.New(rand.NewPCG(0, 0)) //nolint:gosec // mock data, reproducible by design
	}
	if len(t.parts) == 1 && t.parts[0].expr != nil {
		return t.parts[0].expr.value(env)
	}
	var b strings.Builder
	for _, p := range t.parts {
		if p.expr == nil {
			b.WriteString(p.lit)
			continue
		}
		b.WriteString(text(p.expr.value(env)))
	}
	return b.String()
}

func (t Text) UsesBody() bool {
	return slices.ContainsFunc(t.parts, func(p part) bool { return p.expr != nil && p.expr.scope == model.ScopeBody })
}

func (e *expr) value(env Env) any {
	if e.gen != nil {
		return e.gen.eval(env, e.args)
	}
	v, ok := match.Lookup(e.scope, e.key, env.Facts)
	if !ok {
		return ""
	}
	return v
}

func text(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(x)
	case nil:
		return ""
	}
	return string(marshal(v))
}

func marshal(v any) []byte {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
	return bytes.TrimSuffix(b.Bytes(), []byte("\n"))
}

func Has(data []byte) bool { return bytes.Contains(data, []byte("{{")) }

func (t Text) UsesRandom() bool {
	return slices.ContainsFunc(t.parts, func(p part) bool { return p.expr != nil && p.expr.gen != nil && p.expr.gen.random })
}
