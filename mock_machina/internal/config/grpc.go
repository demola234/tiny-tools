package config

import (
	"io/fs"
	"path"
	"regexp"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/demola234/tiny-tools/mock_machina/internal/model"
	"github.com/demola234/tiny-tools/mock_machina/internal/protos"
	"github.com/demola234/tiny-tools/mock_machina/internal/tmpl"
)

const protosDir = "protos"

var (
	grpcMethod = regexp.MustCompile(`^[A-Za-z_][\w.]*\.[A-Za-z_]\w*/[A-Za-z_]\w*$`)
	grpcCodes  = model.GRPCCodes()
)

func (l *loader) protos() map[string]string {
	sources := map[string]string{}
	_ = fs.WalkDir(l.fsys, protosDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return fs.SkipDir
		}
		if d.IsDir() || path.Ext(p) != ".proto" {
			return nil
		}
		data, err := fs.ReadFile(l.fsys, p)
		if err == nil {
			sources[p] = string(data)
		}
		return nil
	})
	if len(sources) == 0 {
		return nil
	}
	l.protoSources = sources
	set, probs := protos.Compile(sources)
	for _, p := range probs {
		l.add(p.File, p.Line, "%s", p.Msg)
	}
	l.protoSet = set
	return sources
}

func (l *loader) grpcStatus(file string, method model.Method, f map[string]*yaml.Node, st *model.State) {
	code, msg := f["grpc"], f["message"]
	if method != model.MethodGRPC {
		for _, v := range []*yaml.Node{code, msg} {
			if v != nil {
				l.add(file, v.Line, "%s only works on GRPC routes", map[*yaml.Node]string{code: "grpc", msg: "message"}[v])
			}
		}
		return
	}
	if s := f["status"]; s != nil {
		l.add(file, s.Line, "gRPC routes use grpc: NOT_FOUND instead of an HTTP status")
	}
	if code != nil {
		st.GRPC.Code = code.Value
		if !slices.Contains(grpcCodes, code.Value) {
			l.add(file, code.Line, "grpc %q isn't a gRPC status (%s)", code.Value, closeMatchOr(code.Value, grpcCodes, "statuses: "+strings.Join(grpcCodes, ", ")))
		}
	}
	if msg != nil {
		st.GRPC.Message = msg.Value
		if st.GRPC.Code == "" || st.GRPC.Code == "OK" {
			l.add(file, msg.Line, "message goes with an error status, like grpc: NOT_FOUND")
		}
	}
}

func (l *loader) grpcRoute(f map[string]*yaml.Node, r *model.Route) {
	v := f["route"]
	if r.Method != model.MethodGRPC || v == nil {
		return
	}
	switch {
	case !grpcMethod.MatchString(r.Path):
		l.add(r.Src.File, v.Line, "a GRPC route names a method like shop.v1.OrderService/GetOrder")
	case l.protoSet == nil:
		if len(l.protoSources) == 0 {
			l.add(r.Src.File, v.Line, "no .proto file defines %s; add one under protos/", r.Path)
		}
	default:
		m, ok := l.protoSet.Method(r.Path)
		switch {
		case !ok:
			methods := l.protoSet.Methods()
			l.add(r.Src.File, v.Line, "no .proto file defines %s (%s)", r.Path, closeMatchOr(r.Path, methods, "methods: "+strings.Join(methods, ", ")))
		case m.IsStreamingClient():
			l.add(r.Src.File, v.Line, "%s takes a stream from the client; only unary and server-streaming methods can be mocked", r.Path)
		}
	}
}

func (l *loader) grpcDescriptor(r *model.Route) (protoreflect.MethodDescriptor, bool) {
	if r.Method != model.MethodGRPC || l.protoSet == nil {
		return nil, false
	}
	m, ok := l.protoSet.Method(r.Path)
	if !ok || m.IsStreamingClient() {
		return nil, false
	}
	return m, true
}

func (l *loader) grpcState(file string, r *model.Route, f map[string]*yaml.Node, st *model.State) {
	m, ok := l.grpcDescriptor(r)
	if !ok {
		return
	}
	out := m.Output()
	if !m.IsStreamingServer() {
		l.grpcUnary(file, r, f, st, out)
		return
	}
	if st.Stream == nil {
		return
	}
	l.checkMessages(file, f["send"], out, "send", st.Stream.Send)
	l.checkMessages(file, f["push"], out, "push", st.Stream.Push)
}

func (l *loader) checkMessages(file string, v *yaml.Node, out protoreflect.MessageDescriptor, label string, ms []model.Message) {
	for i, msg := range ms {
		if msg.Text || tmpl.Has(msg.Data) {
			continue
		}
		if _, err := protos.FromJSON(out, msg.Data); err != nil {
			l.add(file, v.Line, "%s %d doesn't fit %s: %s", label, i+1, out.FullName(), strings.TrimSpace(err.Error()))
		}
	}
}

func (l *loader) grpcUnary(file string, r *model.Route, f map[string]*yaml.Node, st *model.State, out protoreflect.MessageDescriptor) {
	for _, name := range streamFields {
		if v := f[name]; v != nil {
			l.add(file, v.Line, "%s only works on server-streaming methods; %s returns one message, so use body", name, r.Path)
			return
		}
	}
	v := f["body"]
	if v == nil || len(st.Body.Data) == 0 || tmpl.Has(st.Body.Data) {
		return
	}
	if _, err := protos.FromJSON(out, st.Body.Data); err != nil {
		l.add(file, v.Line, "body doesn't fit %s: %s", out.FullName(), strings.TrimSpace(err.Error()))
	}
}
