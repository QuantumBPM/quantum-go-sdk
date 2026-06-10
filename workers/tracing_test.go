package workers

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/QuantumBPM/quantum-go-sdk/generated"
)

const sampledTraceparent = "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01"

func TestStartJobSpan_ContinuesInstanceTrace(t *testing.T) {
	sr := tracetest.NewSpanRecorder()
	otel.SetTracerProvider(sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr)))
	otel.SetTextMapPropagator(propagation.TraceContext{})

	biz := "order-42"
	job := &generated.ExternalJob{
		TaskType:     "charge-card",
		NodeID:       "Task_Charge",
		WorkflowID:   "wf-1",
		ExecutionKey: "wf-1:Task_Charge:root:1",
		BusinessId:   &biz,
		TraceContext: &map[string]string{"traceparent": sampledTraceparent},
	}

	_, span := startJobSpan(context.Background(), job)
	span.End()

	ended := sr.Ended()
	if len(ended) != 1 {
		t.Fatalf("expected 1 span, got %d", len(ended))
	}
	s := ended[0]

	if got := s.SpanContext().TraceID().String(); got != "0af7651916cd43dd8448eb211c80319c" {
		t.Errorf("worker span did not join instance trace: traceID=%s", got)
	}
	if got := s.Parent().SpanID().String(); got != "b7ad6b7169203331" {
		t.Errorf("worker span parent mismatch: parentSpanID=%s", got)
	}
	attrs := map[string]string{}
	for _, a := range s.Attributes() {
		attrs[string(a.Key)] = a.Value.Emit()
	}
	for k, want := range map[string]string{
		"bpmn.task_type":           "charge-card",
		"bpmn.node_id":             "Task_Charge",
		"bpmn.process_instance_id": "wf-1",
		"bpmn.business_id":         "order-42",
	} {
		if attrs[k] != want {
			t.Errorf("attr %s = %q, want %q", k, attrs[k], want)
		}
	}
}

func TestStartJobSpan_NoTraceContext_IsRoot(t *testing.T) {
	sr := tracetest.NewSpanRecorder()
	otel.SetTracerProvider(sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr)))
	otel.SetTextMapPropagator(propagation.TraceContext{})

	job := &generated.ExternalJob{TaskType: "t", NodeID: "n", WorkflowID: "wf"}
	_, span := startJobSpan(context.Background(), job)
	span.End()

	if s := sr.Ended(); len(s) != 1 || s[0].Parent().IsValid() {
		t.Fatalf("expected 1 root span with no valid parent, got %+v", s)
	}
}
