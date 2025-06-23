package pulse_otel

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// Tracer wraps OpenTelemetry tracer with additional utilities
type Tracer struct {
	tracer trace.Tracer
	name   string
}

func NewTracer(name string) *Tracer {
	return &Tracer{
		tracer: otel.Tracer(name),
		name:   name,
	}
}

// StartSpan starts a new span with the given name
func (t *Tracer) StartSpan(ctx context.Context, spanName string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return t.tracer.Start(ctx, spanName, opts...)
}

// StartSpanWithParent starts a new span with a parent span context
func (t *Tracer) StartSpanWithParent(parentCtx context.Context, spanName string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return t.tracer.Start(parentCtx, spanName, opts...)
}

// AddSpanAttributes adds attributes to the current span in context
func (t *Tracer) AddSpanAttributes(ctx context.Context, attributes ...attribute.KeyValue) {
	span := trace.SpanFromContext(ctx)
	if span != nil {
		span.SetAttributes(attributes...)
	}
}

// AddSpanAttribute adds a single attribute to the current span in context
func (t *Tracer) AddSpanAttribute(ctx context.Context, key, value string) {
	t.AddSpanAttributes(ctx, attribute.String(key, value))
}

// RecordError records an error in the current span
func (t *Tracer) RecordError(ctx context.Context, err error, opts ...trace.EventOption) {
	span := trace.SpanFromContext(ctx)
	if span != nil {
		span.RecordError(err, opts...)
		span.SetStatus(codes.Error, err.Error())
	}
}

// SetSpanStatus sets the status of the current span
func (t *Tracer) SetSpanStatus(ctx context.Context, code codes.Code, description string) {
	span := trace.SpanFromContext(ctx)
	if span != nil {
		span.SetStatus(code, description)
	}
}

// FinishSpan safely finishes a span
func (t *Tracer) FinishSpan(span trace.Span) {
	if span != nil {
		span.End()
	}
}

// WithSpan executes a function within a new span
func (t *Tracer) WithSpan(ctx context.Context, spanName string, fn func(context.Context) error, opts ...trace.SpanStartOption) error {
	ctx, span := t.StartSpan(ctx, spanName, opts...)
	defer span.End()

	if err := fn(ctx); err != nil {
		t.RecordError(ctx, err)
		return err
	}

	return nil
}

// GetTraceID returns the trace ID from the current context
func (t *Tracer) GetTraceID(ctx context.Context) string {
	span := trace.SpanFromContext(ctx)
	if span != nil {
		return span.SpanContext().TraceID().String()
	}
	return ""
}

// GetSpanID returns the span ID from the current context
func (t *Tracer) GetSpanID(ctx context.Context) string {
	span := trace.SpanFromContext(ctx)
	if span != nil {
		return span.SpanContext().SpanID().String()
	}
	return ""
}

// CreateChildSpan creates a child span from the current context
func (t *Tracer) CreateChildSpan(ctx context.Context, spanName string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return t.tracer.Start(ctx, spanName, opts...)
}

// WithSpanReturn executes a function within a new span and returns a value
func (t *Tracer) WithSpanReturn(ctx context.Context, spanName string, fn func(context.Context) (interface{}, error), opts ...trace.SpanStartOption) (interface{}, error) {
	ctx, span := t.StartSpan(ctx, spanName, opts...)
	defer span.End()

	result, err := fn(ctx)
	if err != nil {
		t.RecordError(ctx, err)
		return result, err
	}

	return result, nil
}

// WithSpanReturnTyped executes a function within a new span and returns a typed value
func WithSpanReturnTyped[T any](t *Tracer, ctx context.Context, spanName string, fn func(context.Context) (T, error), opts ...trace.SpanStartOption) (T, error) {
	ctx, span := t.StartSpan(ctx, spanName, opts...)
	defer span.End()

	result, err := fn(ctx)
	if err != nil {
		t.RecordError(ctx, err)
		return result, err
	}

	return result, nil
}

// SpanHelper provides convenient span operations
type SpanHelper struct {
	span trace.Span
	ctx  context.Context
}

// NewSpanHelper creates a new span helper
func NewSpanHelper(ctx context.Context, span trace.Span) *SpanHelper {
	return &SpanHelper{
		span: span,
		ctx:  ctx,
	}
}

// AddAttribute adds an attribute to the span
func (s *SpanHelper) AddAttribute(key, value string) *SpanHelper {
	s.span.SetAttributes(attribute.String(key, value))
	return s
}

// AddAttributes adds multiple attributes to the span
func (s *SpanHelper) AddAttributes(attrs ...attribute.KeyValue) *SpanHelper {
	s.span.SetAttributes(attrs...)
	return s
}

// SetStatus sets the span status
func (s *SpanHelper) SetStatus(code codes.Code, description string) *SpanHelper {
	s.span.SetStatus(code, description)
	return s
}

// RecordError records an error
func (s *SpanHelper) RecordError(err error) *SpanHelper {
	s.span.RecordError(err)
	s.span.SetStatus(codes.Error, err.Error())
	return s
}

// End ends the span
func (s *SpanHelper) End() {
	s.span.End()
}

// Context returns the span context
func (s *SpanHelper) Context() context.Context {
	return s.ctx
}
