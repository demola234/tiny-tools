package cli_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/demola234/tiny-tools/mock_machina/internal/protos"
)

const startProto = `syntax = "proto3";
package shop.v1;
service OrderService { rpc GetOrder(GetOrderRequest) returns (Order); }
message GetOrderRequest { string order_id = 1; }
message Order { string id = 1; string note = 2; }
`

func TestStart_ServesGRPC(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), ".mockmachina")
	writeFile(t, filepath.Join(dir, "protos", "orders.proto"), startProto)
	route := filepath.Join(dir, "routes", "orders.yaml")
	writeFile(t, route, "get:\n  route: GRPC shop.v1.OrderService/GetOrder\n  states:\n    ok:\n      body: { id: \"{{ body.order_id }}\", note: first }\n")
	port := freePort(t)
	lines, stop := startProject(t, dir, "--grpc-port", port, "--watch-interval", "20ms")
	defer stop()
	banner := nextLine(t, lines)
	notes := nextLine(t, lines)
	if !strings.Contains(notes, "gRPC on 127.0.0.1:"+port) {
		t.Fatalf("banner %q, notes %q; want the gRPC address", banner, notes)
	}
	go func() {
		for range lines {
		}
	}()

	set, _ := protos.Compile(map[string]string{"orders.proto": startProto})
	m, _ := set.Method("shop.v1.OrderService/GetOrder")
	conn, err := grpc.NewClient("127.0.0.1:"+port, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	call := func() string {
		in, _ := protos.FromJSON(m.Input(), []byte(`{"order_id":"o_7"}`))
		out := protos.New(m.Output())
		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		defer cancel()
		if err := conn.Invoke(ctx, "/shop.v1.OrderService/GetOrder", in, out); err != nil {
			return err.Error()
		}
		data, _ := protos.ToJSON(out)
		return string(data)
	}
	if got := call(); got != `{"id":"o_7","note":"first"}` {
		t.Fatalf("GetOrder = %s", got)
	}
	replaceIn(t, route, "note: first", "note: second")
	eventually(t, "the reload to reach gRPC", func() bool { return call() == `{"id":"o_7","note":"second"}` })
}
