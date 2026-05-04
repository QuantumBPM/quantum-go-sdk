package variables_test

import (
	"testing"

	"github.com/QuantumBPM/quantum-go-sdk/variables"
)

func TestSetAndLookup(t *testing.T) {
	v := variables.New().Set("amount", 100).Set("name", "Alice")
	if got, ok := v.Lookup("amount"); !ok || got != 100 {
		t.Fatalf("Lookup amount = %v, %v", got, ok)
	}
	if got, ok := v.Lookup("missing"); ok {
		t.Fatalf("Lookup missing returned %v, ok=true", got)
	}
}

func TestGetTyped(t *testing.T) {
	v := variables.From(map[string]any{
		"amount":  1000.0,
		"flag":    true,
		"nested":  map[string]any{"x": 1},
	})

	amt, err := variables.Get[float64](v, "amount")
	if err != nil || amt != 1000 {
		t.Fatalf("Get[float64] amount = %v, err=%v", amt, err)
	}

	flag, err := variables.Get[bool](v, "flag")
	if err != nil || !flag {
		t.Fatalf("Get[bool] flag = %v, err=%v", flag, err)
	}

	type nested struct {
		X int `json:"x"`
	}
	n, err := variables.Get[nested](v, "nested")
	if err != nil || n.X != 1 {
		t.Fatalf("Get[nested] nested = %+v, err=%v", n, err)
	}

	if _, err := variables.Get[int](v, "missing"); err == nil {
		t.Fatal("Get[int] missing: expected error")
	}
}

func TestAsTyped(t *testing.T) {
	type loan struct {
		RequestedAmt float64 `json:"requestedAmt"`
		Approved     bool    `json:"approved"`
	}
	v := variables.New().Set("requestedAmt", 5000).Set("approved", true)

	l, err := variables.As[loan](v)
	if err != nil {
		t.Fatalf("As[loan] err=%v", err)
	}
	if l.RequestedAmt != 5000 || !l.Approved {
		t.Fatalf("As[loan] = %+v", l)
	}
}

func TestFeelContextRoundTrip(t *testing.T) {
	v := variables.New().Set("a", 1).Set("b", "hello").Set("c", []any{1.0, 2.0})

	fctx, err := v.ToFeelContext()
	if err != nil {
		t.Fatalf("ToFeelContext err=%v", err)
	}

	back, err := variables.FromFeelContext(fctx)
	if err != nil {
		t.Fatalf("FromFeelContext err=%v", err)
	}

	if got, _ := variables.Get[float64](back, "a"); got != 1 {
		t.Fatalf("round-trip a = %v", got)
	}
	if got, _ := variables.Get[string](back, "b"); got != "hello" {
		t.Fatalf("round-trip b = %v", got)
	}
}

func TestWireMapEmpty(t *testing.T) {
	if variables.New().ToWireMap() != nil {
		t.Fatal("empty Vars should marshal to nil wire map")
	}
	if got := variables.FromWireMap(nil); len(got) != 0 {
		t.Fatalf("FromWireMap(nil) = %v", got)
	}
}
