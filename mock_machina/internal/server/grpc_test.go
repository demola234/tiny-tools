package server_test

import (
	"context"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/demola234/tiny-tools/mock_machina/internal/config"
	"github.com/demola234/tiny-tools/mock_machina/internal/model"
	"github.com/demola234/tiny-tools/mock_machina/internal/protos"
	"github.com/demola234/tiny-tools/mock_machina/internal/server"
	"github.com/demola234/tiny-tools/mock_machina/internal/testkit"
)

const grpcProto = `syntax = "proto3";
package shop.v1;
service OrderService {
  rpc GetOrder(GetOrderRequest) returns (Order);
  rpc WatchOrder(GetOrderRequest) returns (stream OrderEvent);
}
message GetOrderRequest { string order_id = 1; }
message Order { string id = 1; Status status = 2; }
message OrderEvent { Status status = 1; }
enum Status { STATUS_UNSPECIFIED = 0; PACKED = 1; SHIPPED = 2; }
`

const grpcProject = `get:
  route: GRPC shop.v1.OrderService/GetOrder
  rules:
    - when: { body.order_id: o_404 }
      state: not_found
  states:
    found:
      body: { id: "{{ body.order_id }}", status: SHIPPED }
    not_found:
      grpc: NOT_FOUND
      message: order not found
watch:
  route: GRPC shop.v1.OrderService/WatchOrder
  states:
    happy:
      send: { status: PACKED }
      every: 10ms
      push: { status: SHIPPED }
      close: { after: 35ms }
    cut:
      send: { status: PACKED }
      close: { after: 10ms, drop: true }
`

type grpcFixture struct {
	conn *grpc.ClientConn
	set  *protos.Set
	got  <-chan server.Request
}

func grpcServer(t *testing.T) grpcFixture {
	t.Helper()
	proj, probs, err := config.LoadFS(testkit.MapFS(t, map[string]string{"protos/orders.proto": grpcProto, "routes/orders.yaml": grpcProject, "scenarios/lost.yaml": "states: { orders.get: not_found }\n"}))
	if err != nil || len(probs) > 0 {
		t.Fatalf("load: %v %v", probs, err)
	}
	got := make(chan server.Request, 16)
	h, err := server.NewGRPC(t.Context(), proj, server.Options{Report: func(r server.Request) { got <- r }})
	if err != nil {
		t.Fatal(err)
	}
	ln, err := new(net.ListenConfig).Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := server.GRPCServer(server.NewLiveGRPC(h))
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(srv.Stop)
	conn, err := grpc.NewClient(ln.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	set, _ := protos.Compile(proj.Protos)
	return grpcFixture{conn: conn, set: set, got: got}
}

func (f grpcFixture) method(t *testing.T, name string) protoreflect.MethodDescriptor {
	t.Helper()
	m, ok := f.set.Method(name)
	if !ok {
		t.Fatalf("no method %s", name)
	}
	return m
}

func (f grpcFixture) getOrder(ctx context.Context, t *testing.T, id string) (string, error) {
	t.Helper()
	m := f.method(t, "shop.v1.OrderService/GetOrder")
	in, _ := protos.FromJSON(m.Input(), []byte(`{"order_id":"`+id+`"}`))
	out := protos.New(m.Output())
	if err := f.conn.Invoke(ctx, "/shop.v1.OrderService/GetOrder", in, out); err != nil {
		return "", err
	}
	data, _ := protos.ToJSON(out)
	return string(data), nil
}

func TestGRPC_Unary(t *testing.T) {
	t.Parallel()

	f := grpcServer(t)
	got, err := f.getOrder(t.Context(), t, "o_1")
	if err != nil || got != `{"id":"o_1","status":"SHIPPED"}` {
		t.Fatalf("GetOrder = %s, %v", got, err)
	}
	if r := <-f.got; r.Method != "GRPC" || r.Path != "shop.v1.OrderService/GetOrder" || r.State != "found" || r.GRPCCode != "OK" {
		t.Errorf("report = %+v", r)
	}
	_, err = f.getOrder(t.Context(), t, "o_404")
	if st, _ := status.FromError(err); st.Code() != codes.NotFound || st.Message() != "order not found" {
		t.Errorf("o_404 = %v", err)
	}
}

func TestGRPC_StateFromMetadata(t *testing.T) {
	t.Parallel()

	f := grpcServer(t)
	ctx := metadata.AppendToOutgoingContext(t.Context(), "x-mock-state", "not_found")
	_, err := f.getOrder(ctx, t, "o_1")
	if st, _ := status.FromError(err); st.Code() != codes.NotFound {
		t.Errorf("err = %v", err)
	}
}

func TestGRPC_ScenarioFromMetadata(t *testing.T) {
	t.Parallel()

	f := grpcServer(t)
	_, err := f.getOrder(metadata.AppendToOutgoingContext(t.Context(), "x-mock-scenario", "lost"), t, "o_1")
	if st, _ := status.FromError(err); st.Code() != codes.NotFound {
		t.Errorf("scenario lost: err = %v", err)
	}
	if r := <-f.got; r.Reason != server.ReasonScenario {
		t.Errorf("reason %q, want scenario", r.Reason)
	}
	_, err = f.getOrder(metadata.AppendToOutgoingContext(t.Context(), "x-mock-scenario", "lots"), t, "o_1")
	if st, _ := status.FromError(err); st.Code() != codes.InvalidArgument || st.Message() != `unknown scenario "lots" (did you mean "lost"?)` {
		t.Errorf("scenario lots: err = %v", err)
	}
	if r := <-f.got; r.Problem != server.ProblemUnknownScenario || r.GRPCCode != "INVALID_ARGUMENT" {
		t.Errorf("report = %+v", r)
	}
}

func (f grpcFixture) watch(ctx context.Context, t *testing.T) ([]string, error) {
	t.Helper()
	m := f.method(t, "shop.v1.OrderService/WatchOrder")
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	s, err := f.conn.NewStream(ctx, &grpc.StreamDesc{ServerStreams: true}, "/shop.v1.OrderService/WatchOrder")
	if err != nil {
		return nil, err
	}
	in, _ := protos.FromJSON(m.Input(), []byte(`{"order_id":"o_1"}`))
	if err := s.SendMsg(in); err != nil {
		return nil, err
	}
	_ = s.CloseSend()
	var events []string
	for {
		out := protos.New(m.Output())
		if err := s.RecvMsg(out); err != nil {
			if errors.Is(err, io.EOF) {
				return events, nil
			}
			return events, err
		}
		data, _ := protos.ToJSON(out)
		events = append(events, string(data))
	}
}

func TestGRPC_ServerStream(t *testing.T) {
	t.Parallel()

	f := grpcServer(t)
	events, err := f.watch(t.Context(), t)
	if err != nil || len(events) < 3 || events[0] != `{"status":"PACKED"}` || events[1] != `{"status":"SHIPPED"}` {
		t.Errorf("events = %q, %v", events, err)
	}
	cut, err := f.watch(metadata.AppendToOutgoingContext(t.Context(), "x-mock-state", "cut"), t)
	if st, _ := status.FromError(err); len(cut) != 1 || st.Code() != codes.Unavailable {
		t.Errorf("cut = %q, %v", cut, err)
	}
}

func TestGRPC_UnknownMethod(t *testing.T) {
	t.Parallel()

	f := grpcServer(t)
	err := f.conn.Invoke(t.Context(), "/shop.v1.OrderService/GetOrdr", protos.New(f.method(t, "shop.v1.OrderService/GetOrder").Input()), protos.New(f.method(t, "shop.v1.OrderService/GetOrder").Output()))
	if st, _ := status.FromError(err); st.Code() != codes.Unimplemented || st.Message() != `no route for shop.v1.OrderService/GetOrdr (did you mean "shop.v1.OrderService/GetOrder"?)` {
		t.Errorf("err = %v", err)
	}
}

var _ = model.MethodGRPC
