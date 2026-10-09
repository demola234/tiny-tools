package server

import (
	"testing"

	"github.com/demola234/tiny-tools/mock_machina/internal/model"
	"github.com/demola234/tiny-tools/mock_machina/internal/seed"
)

func TestFaultRate(t *testing.T) {
	t.Parallel()

	f := &model.Fault{Type: model.FaultReset, Rate: 0.3}
	fired := 0
	for call := 1; call <= 10000; call++ {
		if fires(f, seed.New(9), "pay.create", call) {
			fired++
		}
	}
	if fired < 2800 || fired > 3200 {
		t.Errorf("rate 0.3 fired %d times in 10000", fired)
	}
	if !fires(&model.Fault{Rate: 1}, seed.New(9), "x", 1) {
		t.Error("rate 1 didn't fire")
	}
	if fires(nil, seed.New(9), "x", 1) {
		t.Error("no fault fired")
	}
}
