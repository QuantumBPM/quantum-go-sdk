package bpmn

import (
	"context"

	"github.com/QuantumBPM/quantum-go-sdk/generated"
	"github.com/QuantumBPM/quantum-go-sdk/variables"
)

// MessageOption tunes a published message.
type MessageOption func(*messageOpts)

type messageOpts struct {
	correlationKeys *CorrelationKeys
	ttl             *string
}

// WithCorrelationKeys narrows delivery to subscriptions whose stored
// correlation values match the supplied keys. Without it, a message broadcasts
// to subscriptions that have no correlation requirement.
func WithCorrelationKeys(keys CorrelationKeys) MessageOption {
	return func(o *messageOpts) { o.correlationKeys = &keys }
}

// WithMessageTTL sets the buffered message lifetime. Accepts ISO 8601
// duration, Go duration, or RFC 3339 timestamp.
func WithMessageTTL(ttl string) MessageOption {
	return func(o *messageOpts) { o.ttl = &ttl }
}

// PublishMessage delivers a BPMN message to matching subscriptions.
func (c *Client) PublishMessage(ctx context.Context, name string, vars variables.Vars, opts ...MessageOption) error {
	o := &messageOpts{}
	for _, opt := range opts {
		opt(o)
	}
	resp, err := c.api.PublishBpmnMessageWithResponse(ctx, c.projectID, generated.PublishBpmnMessageJSONRequestBody{
		MessageName:     name,
		CorrelationKeys: o.correlationKeys,
		Ttl:             o.ttl,
		Variables:       vars.ToWireMap(),
	})
	if err != nil {
		return err
	}
	if resp.StatusCode() >= 200 && resp.StatusCode() < 300 {
		return nil
	}
	return errorFromResponses("publish-message", resp.StatusCode())
}

// SignalOption tunes a published signal.
type SignalOption func(*signalOpts)

type signalOpts struct {
	ttl *string
}

// WithSignalTTL sets the buffered signal lifetime.
func WithSignalTTL(ttl string) SignalOption {
	return func(o *signalOpts) { o.ttl = &ttl }
}

// PublishSignal broadcasts a BPMN signal to all matching subscribers.
func (c *Client) PublishSignal(ctx context.Context, name string, vars variables.Vars, opts ...SignalOption) error {
	o := &signalOpts{}
	for _, opt := range opts {
		opt(o)
	}
	resp, err := c.api.PublishBpmnSignalWithResponse(ctx, c.projectID, generated.PublishBpmnSignalJSONRequestBody{
		SignalName: name,
		Ttl:        o.ttl,
		Variables:  vars.ToWireMap(),
	})
	if err != nil {
		return err
	}
	if resp.StatusCode() >= 200 && resp.StatusCode() < 300 {
		return nil
	}
	return errorFromResponses("publish-signal", resp.StatusCode())
}
