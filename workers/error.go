package workers

import (
	"fmt"

	"github.com/QuantumBPM/quantum-go-sdk/variables"
)

// BpmnError is a typed error a handler can return to fail a job with a BPMN
// error code. The runtime translates it into a ThrowError call against the
// originating service task — matching boundary error events on the task can
// then route the exception in the model.
//
// Variables, when provided, are merged into the instance scope as part of
// the error throw.
type BpmnError struct {
	Code      string
	Variables variables.Vars
}

// Error implements error.
func (e *BpmnError) Error() string { return fmt.Sprintf("bpmn error: %s", e.Code) }

// NewBpmnError constructs a BpmnError. Pass nil for vars when there are
// no supplementary variables.
func NewBpmnError(code string, vars variables.Vars) *BpmnError {
	return &BpmnError{Code: code, Variables: vars}
}
