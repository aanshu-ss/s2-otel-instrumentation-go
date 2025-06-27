package pulse_otel

import (
	"context"
	"fmt"
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
	tenantManager *TenantManager
	serviceName   string
}

// NewHTTPMiddleware creates a new HTTP middleware with tenant support
func NewHTTPMiddleware(serviceName string, baseConfig *Config) *HTTPMiddleware {
	return &HTTPMiddleware{
		tenantManager: NewTenantManager(baseConfig),
		serviceName:   serviceName,
	}
}

// GetTenantManager returns the tenant manager instance
func (m *HTTPMiddleware) GetTenantManager() *TenantManager {
	return m.tenantManager
}

// Shutdown gracefully shuts down the middleware and its tenant manager
func (m *HTTPMiddleware) Shutdown(ctx context.Context) error {
	return m.tenantManager.Shutdown(ctx)
}

// Handler wraps an http.Handler with opentelemetry instrumentation
func (m *HTTPMiddleware) Handler(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract tenant ID from header
		tenantID := r.Header.Get("x-tenant-id")
		if tenantID == "" {
			tenantID = "default" // fallback to default tenant
		}

		// Get tenant-specific tracer provider
		provider, err := m.tenantManager.GetTracerProvider(tenantID)
		if err != nil {
			// Log error and use default behavior
			fmt.Printf("Error getting tenant provider for %s: %v\n", tenantID, err)
			handler.ServeHTTP(w, r)
			return
		}

		// Create tenant-specific tracer
		tracer := provider.Tracer(m.serviceName)

		ctx := otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))

		spanName := fmt.Sprintf("%s %s", r.Method, r.URL.Path)
		ctx, span := tracer.Start(ctx, spanName,
			trace.WithSpanKind(trace.SpanKindServer),
			trace.WithAttributes(
				semconv.HTTPRoute(r.URL.Path),
				attribute.String("tenant.id", tenantID),
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
