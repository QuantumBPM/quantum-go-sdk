// Package variables provides the Vars type for passing values to and from
// the QuantumBPM DMN engine and BPMN runtime.
//
// Vars is a thin map[string]any with helpers for typed access and for
// converting to/from the wire types used by the generated client.
package variables

import (
	"encoding/json"
	"fmt"

	"github.com/QuantumBPM/quantum-go-sdk/generated"
)

// Vars holds a set of named variables. The same type is used for DMN
// evaluation contexts, BPMN process variables, and external job payloads.
type Vars map[string]any

// New returns an empty Vars.
func New() Vars { return Vars{} }

// From copies a map into a new Vars.
func From(m map[string]any) Vars {
	out := make(Vars, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// Set assigns name=value and returns the same Vars for chaining.
func (v Vars) Set(name string, value any) Vars {
	v[name] = value
	return v
}

// Lookup returns the raw value at name and whether it was present.
func (v Vars) Lookup(name string) (any, bool) {
	val, ok := v[name]
	return val, ok
}

// Get decodes the value at name into T using a JSON round-trip. The round-trip
// normalizes numeric types and nested structures, so callers can pass Go
// structs, slices, or maps as T.
func Get[T any](v Vars, name string) (T, error) {
	var zero T
	raw, ok := v[name]
	if !ok {
		return zero, fmt.Errorf("variables: %q not set", name)
	}
	b, err := json.Marshal(raw)
	if err != nil {
		return zero, fmt.Errorf("variables: marshal %q: %w", name, err)
	}
	var out T
	if err := json.Unmarshal(b, &out); err != nil {
		return zero, fmt.Errorf("variables: decode %q into %T: %w", name, out, err)
	}
	return out, nil
}

// As decodes the entire Vars into a value of type T using a JSON round-trip.
// Use it for opt-in typed handlers in the workers package.
func As[T any](v Vars) (T, error) {
	var out T
	b, err := json.Marshal(v)
	if err != nil {
		return out, err
	}
	if err := json.Unmarshal(b, &out); err != nil {
		return out, err
	}
	return out, nil
}

// ToFeelContext converts the Vars to a FeelContext for DMN evaluation.
// The conversion is a JSON round-trip, which faithfully preserves nested
// structures while letting the FEEL engine impose its type rules on receipt.
func (v Vars) ToFeelContext() (generated.FeelContext, error) {
	if v == nil {
		return generated.FeelContext{}, nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var fctx generated.FeelContext
	if err := json.Unmarshal(b, &fctx); err != nil {
		return nil, err
	}
	return fctx, nil
}

// FromFeelContext lifts a FeelContext into a Vars value.
func FromFeelContext(fctx generated.FeelContext) (Vars, error) {
	if fctx == nil {
		return New(), nil
	}
	b, err := json.Marshal(fctx)
	if err != nil {
		return nil, err
	}
	var v Vars
	if err := json.Unmarshal(b, &v); err != nil {
		return nil, err
	}
	return v, nil
}

// ToWireMap returns the Vars as the *generated.VariableMap shape the
// generated BPMN endpoints accept. Returns nil for an empty/nil Vars so that
// the optional field can be omitted from the request body.
func (v Vars) ToWireMap() *generated.VariableMap {
	if len(v) == 0 {
		return nil
	}
	m := generated.VariableMap(v)
	return &m
}

// FromWireMap copies a *generated.VariableMap (typical generated BPMN
// response shape) into a Vars. Numbers arrive as json.Number - exact
// decimals survive the read path.
func FromWireMap(m *generated.VariableMap) Vars {
	if m == nil {
		return New()
	}
	return From(map[string]interface{}(*m))
}
