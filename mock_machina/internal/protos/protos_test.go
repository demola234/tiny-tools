package protos_test

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/demola234/tiny-tools/mock_machina/internal/protos"
)

const orders = `syntax = "proto3";
package shop.v1;

import "google/protobuf/timestamp.proto";

service OrderService {
  rpc GetOrder(GetOrderRequest) returns (Order);
  rpc WatchOrder(GetOrderRequest) returns (stream OrderEvent);
  rpc Upload(stream Chunk) returns (Order);
}

message GetOrderRequest { string order_id = 1; }
message Chunk { bytes data = 1; }
message Order {
  string id = 1;
  Status status = 2;
  int64 total_kobo = 3;
  google.protobuf.Timestamp placed_at = 4;
}
message OrderEvent { Status status = 1; }
enum Status { STATUS_UNSPECIFIED = 0; PACKED = 1; SHIPPED = 2; }
`

func compile(t *testing.T, files map[string]string) *protos.Set {
	t.Helper()
	set, probs := protos.Compile(files)
	if len(probs) > 0 {
		t.Fatalf("problems: %v", probs)
	}
	return set
}

func TestCompile_FindsMethods(t *testing.T) {
	t.Parallel()

	set := compile(t, map[string]string{"protos/shop/v1/orders.proto": orders})
	want := []string{"shop.v1.OrderService/GetOrder", "shop.v1.OrderService/Upload", "shop.v1.OrderService/WatchOrder"}
	if diff := cmp.Diff(want, set.Methods()); diff != "" {
		t.Errorf("methods (-want +got):\n%s", diff)
	}
	m, ok := set.Method("shop.v1.OrderService/WatchOrder")
	if !ok || m.IsStreamingClient() || !m.IsStreamingServer() || m.Output().Name() != "OrderEvent" {
		t.Errorf("WatchOrder = %v %v", m, ok)
	}
}

func TestCompile_ReportsFileAndLine(t *testing.T) {
	t.Parallel()

	_, probs := protos.Compile(map[string]string{"protos/bad.proto": "syntax = \"proto3\";\nmessage A {\n  strin id = 1;\n}\n"})
	if len(probs) != 1 || probs[0].File != "protos/bad.proto" || probs[0].Line != 3 || !strings.Contains(probs[0].Msg, "strin") {
		t.Errorf("problems = %+v", probs)
	}
}

func TestMessages_JSONRoundTrip(t *testing.T) {
	t.Parallel()

	set := compile(t, map[string]string{"protos/orders.proto": orders})
	m, _ := set.Method("shop.v1.OrderService/GetOrder")
	msg, err := protos.FromJSON(m.Output(), []byte(`{"id":"o 1","status":"SHIPPED","total_kobo":"4500","placed_at":"2026-10-14T09:00:00Z"}`))
	if err != nil {
		t.Fatal(err)
	}
	back, err := protos.ToJSON(msg)
	if err != nil {
		t.Fatal(err)
	}
	if string(back) != `{"id":"o 1","status":"SHIPPED","total_kobo":"4500","placed_at":"2026-10-14T09:00:00Z"}` {
		t.Errorf("round trip = %s", back)
	}
	if _, err := protos.FromJSON(m.Output(), []byte(`{"idd":"x"}`)); err == nil || !strings.Contains(err.Error(), "idd") {
		t.Errorf("unknown field: %v", err)
	}
}
