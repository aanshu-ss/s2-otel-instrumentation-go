package pulse_otel

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
)

// Config holds OpenTelemetry configuration
type Config struct {
	ServiceName        string
	ServiceVersion     string
	Environment        string
	CollectorEndpoint  string
	Headers            map[string]string
	Timeout            time.Duration
	ResourceAttributes map[string]string
}

// DefaultConfig returns a default configuration
func DefaultConfig() *Config {
	return &Config{
		ServiceName:        "default-service",
		ServiceVersion:     "1.0.0",
		Environment:        "development",
		CollectorEndpoint:  "http://localhost:4318",
		Headers:            make(map[string]string),
		Timeout:            30 * time.Second,
		ResourceAttributes: make(map[string]string),
	}
}

// OtelManager manages OpenTelemetry providers
type OtelManager struct {
	tracerProvider *trace.TracerProvider
	loggerProvider *log.LoggerProvider
	config         *Config
}

// NewOtelManager creates a new OpenTelemetry manager
func NewPulseOtelManager(config *Config) (*OtelManager, error) {
	if config == nil {
		config = DefaultConfig()
	}

	manager := &OtelManager{
		config: config,
	}

	if err := manager.initializeTracing(); err != nil {
		return nil, fmt.Errorf("failed to initialize tracing: %w", err)
	}

	// if err := manager.initializeLogging(); err != nil {
	// 	return nil, fmt.Errorf("failed to initialize logging: %w", err)
	// }

	return manager, nil
}

func (m *OtelManager) initializeTracing() error {
	ctx := context.Background()

	// Create resource
	res, err := m.createResource()
	if err != nil {
		return fmt.Errorf("failed to create resource: %w", err)
	}

	// Create OTLP HTTP exporter
	exporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpoint(m.config.CollectorEndpoint),
		otlptracehttp.WithHeaders(m.config.Headers),
		otlptracehttp.WithTimeout(m.config.Timeout),
		otlptracehttp.WithInsecure(), // Use WithTLSClientConfig for production
	)
	if err != nil {
		return fmt.Errorf("failed to create trace exporter: %w", err)
	}

	// Create tracer provider
	m.tracerProvider = trace.NewTracerProvider(
		trace.WithBatcher(exporter),
		trace.WithResource(res),
		trace.WithSampler(trace.AlwaysSample()),
	)

	// Set global tracer provider
	otel.SetTracerProvider(m.tracerProvider)

	// Set global propagator
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return nil
}

// func (m *OtelManager) initializeLogging() error {
//     ctx := context.Background()

//     // Create resource
//     res, err := m.createResource()
//     if err != nil {
//         return fmt.Errorf("failed to create resource: %w", err)
//     }

//     // Create OTLP HTTP log exporter
//     exporter, err := otlploghttp.New(ctx,
//         otlploghttp.WithEndpoint(m.config.CollectorEndpoint),
//         otlploghttp.WithHeaders(m.config.Headers),
//         otlploghttp.WithTimeout(m.config.Timeout),
//         otlploghttp.WithInsecure(), // Use WithTLSClientConfig for production
//     )
//     if err != nil {
//         return fmt.Errorf("failed to create log exporter: %w", err)
//     }

//     // Create logger provider
//     m.loggerProvider = log.NewLoggerProvider(
//         log.WithProcessor(log.NewBatchProcessor(exporter)),
//         log.WithResource(res),
//     )

//     // Set global logger provider
//     otel.SetLoggerProvider(m.loggerProvider)

//     return nil
// }

func (m *OtelManager) createResource() (*resource.Resource, error) {
	// Start with base attributes
	attributes := []attribute.KeyValue{
		semconv.ServiceName(m.config.ServiceName),
		semconv.ServiceVersion(m.config.ServiceVersion),
		semconv.DeploymentEnvironment(m.config.Environment),
	}

	// Add custom resource attributes
	for key, value := range m.config.ResourceAttributes {
		attributes = append(attributes, attribute.String(key, value))
	}

	return resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			attributes...,
		),
	)
}

// Shutdown gracefully shuts down the OpenTelemetry providers
func (m *OtelManager) Shutdown(ctx context.Context) error {
	var err error

	if m.tracerProvider != nil {
		if shutdownErr := m.tracerProvider.Shutdown(ctx); shutdownErr != nil {
			err = fmt.Errorf("failed to shutdown tracer provider: %w", shutdownErr)
		}
	}

	if m.loggerProvider != nil {
		if shutdownErr := m.loggerProvider.Shutdown(ctx); shutdownErr != nil {
			if err != nil {
				err = fmt.Errorf("%w; failed to shutdown logger provider: %w", err, shutdownErr)
			} else {
				err = fmt.Errorf("failed to shutdown logger provider: %w", shutdownErr)
			}
		}
	}

	return err
}

// AddResourceAttribute adds a resource attribute to the configuration
func (c *Config) AddResourceAttribute(key, value string) {
	if c.ResourceAttributes == nil {
		c.ResourceAttributes = make(map[string]string)
	}
	c.ResourceAttributes[key] = value
}

// SetEndpoint sets the collector endpoint
func (c *Config) SetEndpoint(endpoint string) {
	c.CollectorEndpoint = endpoint
}

// AddHeader adds a header for the OTLP exporter
func (c *Config) AddHeader(key, value string) {
	if c.Headers == nil {
		c.Headers = make(map[string]string)
	}
	c.Headers[key] = value
}
