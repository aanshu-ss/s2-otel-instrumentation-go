package pulse_otel

import (
	"fmt"
	"net/http"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
)

// HTTPMiddleware provides HTTP instrumentation middleware
type HTTPMiddleware struct {
	tracer trace.Tracer
}

// NewHTTPMiddleware creates a new HTTP middleware
func NewHTTPMiddleware(serviceName string) *HTTPMiddleware {
	return &HTTPMiddleware{
		tracer: otel.Tracer(serviceName)}
}

// Handler wraps an http.Handler with opentelemetry instrumentation
func (m *HTTPMiddleware) Handler(handler http.Handler) http.Handler {

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))

		spanName := fmt.Sprintf("%s %s", r.Method, r.URL.Path)
		ctx, span := m.tracer.Start(ctx, spanName,
			trace.WithSpanKind(trace.SpanKindServer),
			trace.WithAttributes(
				semconv.HTTPMethod(r.Method),
				semconv.HTTPURL(r.URL.String()),
				semconv.HTTPRoute(r.URL.Path),
				semconv.HTTPScheme(r.URL.Scheme),
			),
		)
		defer span.End()
		wrappedWriter := &responseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		// Execute the handler with the instrumented context
		r = r.WithContext(ctx)
		start := time.Now()
		handler.ServeHTTP(wrappedWriter, r)
		duration := time.Since(start)

		// Add response attributes
		span.SetAttributes(
			semconv.HTTPStatusCode(wrappedWriter.statusCode),
			attribute.Float64("http.duration_ms", float64(duration.Nanoseconds())/1000000),
		)

		// Set span status based on HTTP status code
		if wrappedWriter.statusCode >= 400 {
			span.SetStatus(codes.Error, fmt.Sprintf("HTTP %d", wrappedWriter.statusCode))
		}

		// Inject trace context into response headers
		otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(w.Header()))
	})
}

func (m *HTTPMiddleware) HandlerFunc(handler http.HandlerFunc) http.HandlerFunc {
	return m.Handler(handler).ServeHTTP
}

// responseWriter wraps http.ResponseWriter to capture response details
type responseWriter struct {
	http.ResponseWriter
	statusCode   int
	bytesWritten int64
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(data []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(data)
	rw.bytesWritten += int64(n)
	return n, err
}

// // GinMiddleware provides Gin framework middleware
// func GinMiddleware(serviceName string) gin.HandlerFunc {
// 	tracer := otel.Tracer(serviceName)

// 	return func(c *gin.Context) {
// 		// Extract trace context
// 		ctx := otel.GetTextMapPropagator().Extract(c.Request.Context(), propagation.HeaderCarrier(c.Request.Header))

// 		spanName := fmt.Sprintf("%s %s", c.Request.Method, c.FullPath())
// 		ctx, span := tracer.Start(ctx, spanName,
// 			trace.WithSpanKind(trace.SpanKindServer),
// 			trace.WithAttributes(
// 				semconv.HTTPMethod(c.Request.Method),
// 				semconv.HTTPURL(c.Request.URL.String()),
// 				semconv.HTTPRoute(c.FullPath()),
// 				semconv.HTTPScheme(c.Request.URL.Scheme),
// 				semconv.HTTPHost(c.Request.Host),
// 				semconv.HTTPUserAgent(c.Request.UserAgent()),
// 				semconv.HTTPRequestContentLength(int64(c.Request.ContentLength)),
// 				attribute.String("gin.handler_name", c.HandlerName()),
// 			),
// 		)
// 		defer span.End()

// 		// Store context in Gin context
// 		c.Request = c.Request.WithContext(ctx)

// 		start := time.Now()
// 		c.Next()
// 		duration := time.Since(start)

// 		// Add response attributes
// 		span.SetAttributes(
// 			semconv.HTTPStatusCode(c.Writer.Status()),
// 			attribute.Float64("http.duration_ms", float64(duration.Nanoseconds())/1000000),
// 			attribute.Int("gin.writer_size", c.Writer.Size()),
// 		)

// 		// Record errors if any
// 		if len(c.Errors) > 0 {
// 			span.RecordError(c.Errors.Last())
// 			span.SetStatus(codes.Error, c.Errors.String())
// 		}

// 		// Set span status based on HTTP status code
// 		if c.Writer.Status() >= 400 {
// 			span.SetStatus(codes.Error, fmt.Sprintf("HTTP %d", c.Writer.Status()))
// 		}

// 		// Inject trace context into response headers
// 		otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(c.Writer.Header()))
// 	}
// }

// InstrumentHandler is a convenience function to instrument a single handler
func InstrumentHandler(serviceName string, pattern string, handler http.HandlerFunc) (string, http.HandlerFunc) {
	middleware := NewHTTPMiddleware(serviceName)
	return pattern, middleware.HandlerFunc(handler)
}
