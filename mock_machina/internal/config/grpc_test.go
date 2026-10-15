package config_test

import (
	"testing"

	"github.com/demola234/tiny-tools/mock_machina/internal/model"
)

const ordersProto = `syntax = "proto3";
package shop.v1;
service OrderService {
  rpc GetOrder(GetOrderRequest) returns (Order);
  rpc WatchOrder(GetOrderRequest) returns (stream OrderEvent);
  rpc Upload(stream GetOrderRequest) returns (Order);
}
message GetOrderRequest { string order_id = 1; }
message Order { string id = 1; Status status = 2; }
message OrderEvent { Status status = 1; }
enum Status { STATUS_UNSPECIFIED = 0; PACKED = 1; SHIPPED = 2; }
`

const grpcRoutes = `get:
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
      send: [{ status: PACKED }]
      every: 1s
      push: { status: SHIPPED }
      close: { after: 3s }
`

func TestLoadFS_GRPC(t *testing.T) {
	t.Parallel()

	p := loadClean(t, map[string]string{"protos/shop/orders.proto": ordersProto, "routes/orders.yaml": grpcRoutes})
	r, _ := p.Route("orders.get")
	if r.Method != model.MethodGRPC || r.Path != "shop.v1.OrderService/GetOrder" {
		t.Errorf("route = %s %s", r.Method, r.Path)
	}
	nf, _ := r.States.Get("not_found")
	if nf.GRPC.Code != "NOT_FOUND" || nf.GRPC.Message != "order not found" {
		t.Errorf("not_found = %+v", nf.GRPC)
	}
	if p.Protos["protos/shop/orders.proto"] == "" {
		t.Errorf("protos = %v", p.Protos)
	}
}

func TestLoadFS_GRPCProblems(t *testing.T) {
	t.Parallel()

	route := func(r string) map[string]string {
		return map[string]string{"protos/orders.proto": ordersProto, "routes/orders.yaml": r}
	}
	unary := func(state string) string {
		return "get:\n  route: GRPC shop.v1.OrderService/GetOrder\n  states:\n    s:\n" + state
	}
	tests := []struct {
		name  string
		files map[string]string
		want  string
	}{
		{"broken proto", map[string]string{"protos/x.proto": "syntax = \"proto3\";\nmessage A { strin a = 1; }\n"}, `protos/x.proto:2: field A.a: unknown type strin`},
		{"no protos", map[string]string{"routes/orders.yaml": unary("      body: {}\n")}, `routes/orders.yaml:2: no .proto file defines shop.v1.OrderService/GetOrder; add one under protos/`},
		{"unknown method", route("get:\n  route: GRPC shop.v1.OrderService/GetOrdr\n  states:\n    s: {}\n"), `routes/orders.yaml:2: no .proto file defines shop.v1.OrderService/GetOrdr (did you mean "shop.v1.OrderService/GetOrder"?)`},
		{"bad path", route("get:\n  route: GRPC /GetOrder\n  states:\n    s: {}\n"), `routes/orders.yaml:2: a GRPC route names a method like shop.v1.OrderService/GetOrder`},
		{"client streaming", route("up:\n  route: GRPC shop.v1.OrderService/Upload\n  states:\n    s: {}\n"), `routes/orders.yaml:2: shop.v1.OrderService/Upload takes a stream from the client; only unary and server-streaming methods can be mocked`},
		{"bad code", route(unary("      grpc: NOT_FOUN\n")), `routes/orders.yaml:5: grpc "NOT_FOUN" isn't a gRPC status (did you mean "NOT_FOUND"?)`},
		{"message with ok", route(unary("      message: hi\n")), `routes/orders.yaml:5: message goes with an error status, like grpc: NOT_FOUND`},
		{"status on grpc", route(unary("      status: 404\n")), `routes/orders.yaml:5: gRPC routes use grpc: NOT_FOUND instead of an HTTP status`},
		{"body field unknown", route(unary("      body: { idd: x }\n")), `routes/orders.yaml:5: body doesn't fit shop.v1.Order: unknown field "idd"`},
		{"stream fields on unary", route(unary("      send: { id: x }\n")), `routes/orders.yaml:5: send only works on server-streaming methods; shop.v1.OrderService/GetOrder returns one message, so use body`},
		{"push doesn't fit", route("w:\n  route: GRPC shop.v1.OrderService/WatchOrder\n  states:\n    s:\n      every: 1s\n      push: { stage: x }\n"), `routes/orders.yaml:6: push 1 doesn't fit shop.v1.OrderEvent: unknown field "stage"`},
		{"grpc field on http", map[string]string{"routes/orders.yaml": "get:\n  route: GET /x\n  states:\n    s:\n      grpc: NOT_FOUND\n"}, `routes/orders.yaml:5: grpc only works on GRPC routes`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, probs := load(t, tc.files)
			if got := probs.String(); got != tc.want {
				t.Errorf("problems:\n%s\nwant:\n%s", got, tc.want)
			}
		})
	}
}
