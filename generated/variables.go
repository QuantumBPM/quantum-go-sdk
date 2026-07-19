package generated

import (
	"bytes"
	"encoding/json"
)

// VariableMap is the wire type for BPMN/DMN variable payloads. The spec
// points variable-shaped properties at it via x-go-type so responses decode
// numbers as json.Number - exact decimals survive instead of being narrowed
// through float64. Marshalling needs no override: json.Number marshals
// verbatim. Hand-written companion to the generated client (only *.gen.go
// files are regenerated).
type VariableMap map[string]interface{}

func (m *VariableMap) UnmarshalJSON(b []byte) error {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	return dec.Decode((*map[string]interface{})(m))
}
