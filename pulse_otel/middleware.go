package pulse_otel

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

// HTTPMiddleware provides HTTP instrumentation middleware
type HTTPMiddleware struct {
	pulseTraceManager *PulseTraceManager
	serviceName       string
}

// NewHTTPMiddleware creates a new HTTP middleware with project support
func NewHTTPMiddleware(serviceName string, baseConfig *Config) *HTTPMiddleware {
	return &HTTPMiddleware{
		pulseTraceManager: NewPulseTraceManager(baseConfig),
		serviceName:       serviceName,
	}
}

// GetPulseTraceManager returns the pulse trace manager instance
func (m *HTTPMiddleware) GetPulseTraceManager() *PulseTraceManager {
	return m.pulseTraceManager
}

// Shutdown gracefully shuts down the middleware and its project manager
func (m *HTTPMiddleware) Shutdown(ctx context.Context) error {
	return m.pulseTraceManager.Shutdown(ctx)
}

// Handler wraps an http.Handler with opentelemetry instrumentation
func (m *HTTPMiddleware) Handler(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract project ID from header first
		projectID := r.Header.Get("project-id")

		// If not found in header, check JSON body
		if projectID == "" {
			projectID = m.extractProjectIDFromBody(r)
		}

		// Fallback to default if still not found
		if projectID == "" {
			projectID = "default"
		}

		// Get project-specific tracer provider
		provider, err := m.pulseTraceManager.GetTracerProvider(projectID)
		if err != nil {
			// Log error and use default behavior
			fmt.Printf("Error getting project provider for %s: %v\n", projectID, err)
			handler.ServeHTTP(w, r)
			return
		}

		// Create project-specific tracer
		tracer := provider.Tracer(m.serviceName)

		ctx := otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))

		spanName := fmt.Sprintf("%s %s", r.Method, r.URL.Path)
		ctx, span := tracer.Start(ctx, spanName,
			trace.WithSpanKind(trace.SpanKindServer),
			trace.WithAttributes(
				semconv.HTTPRoute(r.URL.Path),
				attribute.String("project.id", projectID),
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

// extractProjectIDFromBody attempts to extract project-id from the request body
func (m *HTTPMiddleware) extractProjectIDFromBody(r *http.Request) string {
	// Only check for project-id in POST/PUT/PATCH requests with JSON content
	if r.Method != http.MethodPost && r.Method != http.MethodPut && r.Method != http.MethodPatch {
		return ""
	}

	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" && contentType != "application/json; charset=utf-8" {
		return ""
	}

	// Read the body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return ""
	}

	// Restore the body for the actual handler to use
	r.Body = io.NopCloser(bytes.NewBuffer(body))

	// Parse JSON to extract project-id
	var jsonData map[string]interface{}
	if err := json.Unmarshal(body, &jsonData); err != nil {
		return ""
	}

	// Extract project-id from JSON
	if projectID, exists := jsonData["project-id"]; exists {
		if projectIDStr, ok := projectID.(string); ok {
			return projectIDStr
		}
	}

	return ""
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

// InstrumentHandler is a convenience function to instrument a single handler
func InstrumentHandler(serviceName string, pattern string, handler http.HandlerFunc, baseConfig *Config) (string, http.HandlerFunc) {
	middleware := NewHTTPMiddleware(serviceName, baseConfig)
	return pattern, middleware.HandlerFunc(handler)
}
