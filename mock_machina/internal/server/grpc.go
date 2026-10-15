package server

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"sync/atomic"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/reflection"
	v1reflection "google.golang.org/grpc/reflection/grpc_reflection_v1"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"

	"github.com/demola234/tiny-tools/mock_machina/internal/clock"
	"github.com/demola234/tiny-tools/mock_machina/internal/model"
	"github.com/demola234/tiny-tools/mock_machina/internal/protos"
	"github.com/demola234/tiny-tools/mock_machina/internal/suggest"
)

const grpcOK = "OK"

type grpcRoute struct {
	rt       *model.Route
	method   protoreflect.MethodDescriptor
	compiled routeTemplates
}

type GRPCHandler struct {
	opts     Options
	routes   map[string]*grpcRoute
	names    []string
	registry *protoregistry.Files
	services map[string]grpc.ServiceInfo
}

func HasGRPC(p *model.Project) bool {
	return slices.ContainsFunc(p.Routes, func(r *model.Route) bool { return r.Method == model.MethodGRPC })
}

func NewGRPC(ctx context.Context, p *model.Project, opts Options) (*GRPCHandler, error) {
	if opts.Report == nil {
		opts.Report = func(Request) {}
	}
	if opts.Clock == nil {
		opts.Clock = clock.Real{}
	}
	if opts.Memory == nil {
		opts.Memory = NewMemory()
	}
	opts.scenarios = indexScenarios(p.Scenarios)
	h := &GRPCHandler{opts: opts, routes: map[string]*grpcRoute{}, registry: new(protoregistry.Files), services: map[string]grpc.ServiceInfo{}}
	if len(p.Protos) == 0 {
		return h, nil
	}
	set, probs := protos.Compile(p.Protos)
	if len(probs) > 0 {
		return nil, fmt.Errorf("%s:%d: %s", probs[0].File, probs[0].Line, probs[0].Msg)
	}
	reg, err := set.Registry()
	if err != nil {
		return nil, err
	}
	h.registry = reg
	for _, svc := range set.Services() {
		info := grpc.ServiceInfo{}
		for i := range svc.Methods().Len() {
			m := svc.Methods().Get(i)
			info.Methods = append(info.Methods, grpc.MethodInfo{Name: string(m.Name()), IsClientStream: m.IsStreamingClient(), IsServerStream: m.IsStreamingServer()})
		}
		h.services[string(svc.FullName())] = info
	}
	for _, rt := range p.Routes {
		if err := h.add(set, rt); err != nil {
			return nil, err
		}
	}
	slices.Sort(h.names)
	return h, nil
}

func (h *GRPCHandler) add(set *protos.Set, rt *model.Route) error {
	if rt.Method != model.MethodGRPC {
		return nil
	}
	m, ok := set.Method(rt.Path)
	if !ok {
		return fmt.Errorf("route %s: no .proto file defines %s", rt.ID, rt.Path)
	}
	states, readsBody, err := compileStates(rt)
	if err != nil {
		return err
	}
	streams, err := compileStreams(rt)
	if err != nil {
		return err
	}
	h.routes[rt.Path] = &grpcRoute{rt: rt, method: m, compiled: routeTemplates{states: states, readsBody: readsBody, streams: streams}}
	h.names = append(h.names, rt.Path)
	return nil
}

type LiveGRPC struct{ current atomic.Pointer[GRPCHandler] }

func NewLiveGRPC(h *GRPCHandler) *LiveGRPC {
	l := &LiveGRPC{}
	l.current.Store(h)
	return l
}

func (l *LiveGRPC) Swap(h *GRPCHandler) { l.current.Store(h) }

func (l *LiveGRPC) GetServiceInfo() map[string]grpc.ServiceInfo { return l.current.Load().services }

func (l *LiveGRPC) FindFileByPath(path string) (protoreflect.FileDescriptor, error) {
	return l.current.Load().registry.FindFileByPath(path)
}

func (l *LiveGRPC) FindDescriptorByName(name protoreflect.FullName) (protoreflect.Descriptor, error) {
	return l.current.Load().registry.FindDescriptorByName(name)
}

var _ protodesc.Resolver = (*LiveGRPC)(nil)

func GRPCServer(live *LiveGRPC) *grpc.Server {
	s := grpc.NewServer(grpc.UnknownServiceHandler(func(_ any, stream grpc.ServerStream) error {
		return live.current.Load().handle(stream)
	}))
	opts := reflection.ServerOptions{Services: live, DescriptorResolver: live, ExtensionResolver: new(protoregistry.Types)}
	v1reflection.RegisterServerReflectionServer(s, reflection.NewServerV1(opts))
	return s
}

func (h *GRPCHandler) handle(stream grpc.ServerStream) error {
	report := h.opts.reporter()
	full, _ := grpc.MethodFromServerStream(stream)
	name := strings.TrimPrefix(full, "/")
	gr, ok := h.routes[name]
	if !ok {
		msg := "no route for " + name
		if s, found := suggest.Closest(name, h.names); found {
			msg += fmt.Sprintf(" (did you mean %q?)", s)
		}
		report(Request{Method: string(model.MethodGRPC), Path: name, GRPCCode: "UNIMPLEMENTED", Problem: ProblemNoRoute})
		return status.Error(codes.Unimplemented, msg)
	}
	return h.serve(stream, gr, name, report)
}

func (h *GRPCHandler) serve(stream grpc.ServerStream, gr *grpcRoute, name string, report func(Request)) error {
	ctx := stream.Context()
	in := protos.New(gr.method.Input())
	if err := stream.RecvMsg(in); err != nil {
		return err
	}
	body, err := protos.ToJSON(in)
	if err != nil {
		return status.Error(codes.InvalidArgument, err.Error())
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "/"+name, bytes.NewReader(body))
	if err != nil {
		return status.Error(codes.Internal, err.Error())
	}
	md, _ := metadata.FromIncomingContext(ctx)
	for k, vs := range md {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	if scenario, unknown := h.opts.scenarios.unknown(req); unknown {
		report(Request{Method: string(model.MethodGRPC), Path: name, Route: gr.rt.ID, GRPCCode: "INVALID_ARGUMENT", Problem: ProblemUnknownScenario})
		return status.Error(codes.InvalidArgument, h.opts.scenarios.unknownMessage(scenario))
	}
	x := &exchange{rt: gr.rt, opts: h.opts, req: req, call: h.opts.Memory.call(gr.rt.ID), withBody: true, report: report}
	state, reason, rule := pickState(gr.rt, req, h.opts, x.call, x.facts)
	rec := Request{Method: string(model.MethodGRPC), Path: name, Route: gr.rt.ID, State: state, Reason: reason, Rule: rule}
	err = x.answerGRPC(ctx, stream, gr, state, &rec)
	report(rec)
	return err
}

func (x *exchange) answerGRPC(ctx context.Context, stream grpc.ServerStream, gr *grpcRoute, name string, rec *Request) error {
	st, ok := gr.rt.States.Get(name)
	if !ok {
		rec.GRPCCode, rec.Problem = "INVALID_ARGUMENT", ProblemUnknownState
		return status.Errorf(codes.InvalidArgument, "route %s has no state %q (states: %s)", gr.rt.ID, name, strings.Join(gr.rt.States.Names(), ", "))
	}
	if x.opts.Clock.Sleep(ctx, delay(st, x.opts.Seed, gr.rt.ID, x.call)) != nil {
		rec.GRPCCode, rec.Problem = "CANCELLED", ProblemClientLeft
		return status.FromContextError(ctx.Err()).Err()
	}
	if err := x.grpcFault(ctx, st, rec); err != nil {
		return err
	}
	t := gr.compiled.states[name]
	if len(st.Set) > 0 {
		x.assign(st.Set, t)
	}
	code := st.GRPC.Code
	if code == "" {
		code = grpcOK
	}
	rec.GRPCCode = code
	if code != grpcOK {
		return status.Error(grpcCode(code), st.GRPC.Message)
	}
	if gr.method.IsStreamingServer() {
		return x.streamGRPC(ctx, stream, gr, gr.compiled.streams[name])
	}
	data := st.Body.Data
	if t != nil {
		data = t.apply(http.Header{}, data, x.env())
	}
	out, err := protos.FromJSON(gr.method.Output(), orEmpty(data))
	if err != nil {
		rec.GRPCCode = "INTERNAL"
		return status.Errorf(codes.Internal, "state %s body doesn't fit %s: %v", name, gr.method.Output().FullName(), err)
	}
	return stream.SendMsg(out)
}

func orEmpty(data []byte) []byte {
	if len(bytes.TrimSpace(data)) == 0 {
		return []byte("{}")
	}
	return data
}

func (x *exchange) grpcFault(ctx context.Context, st *model.State, rec *Request) error {
	if !fires(st.Fault, x.opts.Seed, x.rt.ID, x.call) {
		return nil
	}
	rec.Fault = st.Fault.Type
	if st.Fault.Type == model.FaultTimeout {
		_ = x.opts.Clock.Sleep(ctx, maxHold)
		rec.GRPCCode = "DEADLINE_EXCEEDED"
		return status.Error(codes.DeadlineExceeded, "the mock never answered (fault: timeout)")
	}
	rec.GRPCCode = "UNAVAILABLE"
	return status.Errorf(codes.Unavailable, "connection dropped (fault: %s)", st.Fault.Type)
}

func (x *exchange) streamGRPC(ctx context.Context, ss grpc.ServerStream, gr *grpcRoute, s *stream) error {
	if s == nil {
		s = &stream{}
	}
	x.facts()
	var n atomic.Int64
	send := func(_ context.Context, data []byte) error {
		out, err := protos.FromJSON(gr.method.Output(), orEmpty(data))
		if err != nil {
			return status.Errorf(codes.Internal, "message doesn't fit %s: %v", gr.method.Output().FullName(), err)
		}
		return ss.SendMsg(out)
	}
	err := x.schedule(ctx, s, send, &n)
	switch {
	case errors.Is(err, errClosing) && s.close.Drop:
		return status.Error(codes.Unavailable, "stream reset by server")
	case errors.Is(err, errClosing):
		return nil
	}
	return err
}

func grpcCode(name string) codes.Code {
	var c codes.Code
	if err := c.UnmarshalJSON([]byte(`"` + name + `"`)); err != nil {
		return codes.Unknown
	}
	return c
}
