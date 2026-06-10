package workers

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	"github.com/QuantumBPM/quantum-go-sdk/generated"
)

const tracerScope = "quantumbpm/go-sdk"

func startJobSpan(ctx context.Context, job *generated.ExternalJob) (context.Context, trace.Span) {
	if job.TraceContext != nil {
		ctx = otel.GetTextMapPropagator().Extract(ctx, propagation.MapCarrier(*job.TraceContext))
	}
	attrs := []attribute.KeyValue{
		attribute.String("bpmn.task_type", job.TaskType),
		attribute.String("bpmn.node_id", job.NodeID),
		attribute.String("bpmn.process_instance_id", job.WorkflowID),
		attribute.String("bpmn.execution_key", job.ExecutionKey),
	}
	if job.BusinessId != nil && *job.BusinessId != "" {
		attrs = append(attrs, attribute.String("bpmn.business_id", *job.BusinessId))
	}
	return otel.Tracer(tracerScope).Start(ctx, "bpmn.external-task.execute",
		trace.WithSpanKind(trace.SpanKindConsumer),
		trace.WithAttributes(attrs...))
}
