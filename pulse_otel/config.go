package pulse_otel

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
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

// PulseTraceManager manages OpenTelemetry providers for multiple projects
type PulseTraceManager struct {
	projectTraceProviders map[string]*trace.TracerProvider
	baseConfig            *Config
	mutex                 sync.RWMutex
}

// NewPulseTraceManager creates a new pulse trace manager
func NewPulseTraceManager(baseConfig *Config) *PulseTraceManager {
	if baseConfig == nil {
		baseConfig = DefaultConfig()
	}

	return &PulseTraceManager{
		projectTraceProviders: make(map[string]*trace.TracerProvider),
		baseConfig:            baseConfig,
	}
}

// GetTracerProvider returns or creates a tracer provider for a specific project
func (tm *PulseTraceManager) GetTracerProvider(projectID string) (*trace.TracerProvider, error) {
	tm.mutex.RLock()
	provider, exists := tm.projectTraceProviders[projectID]
	tm.mutex.RUnlock()

	if exists {
		return provider, nil
	}

	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	// Double-check after acquiring write lock
	if provider, exists := tm.projectTraceProviders[projectID]; exists {
		return provider, nil
	}

	// Create new provider for project
	provider, err := tm.createTraceProvider(projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to create provider for project %s: %w", projectID, err)
	}

	tm.projectTraceProviders[projectID] = provider
	return provider, nil
}

func (tm *PulseTraceManager) createTraceProvider(projectID string) (*trace.TracerProvider, error) {
	ctx := context.Background()

	// Create project-specific resource
	res, err := tm.createProjectTraceResource(projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Form project-specific collector endpoint
	collectorEndpoint := strings.Replace(OTEL_COLLECTOR_ENDPOINT, "{PROJECTID_PLACEHOLDER}", projectID, 1)

	// Create OTLP HTTP exporter for this project
	exporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpoint(collectorEndpoint),
		otlptracehttp.WithHeaders(tm.baseConfig.Headers),
		otlptracehttp.WithTimeout(tm.baseConfig.Timeout),
		otlptracehttp.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create trace exporter: %w", err)
	}

	// Create tracer provider
	provider := trace.NewTracerProvider(
		trace.WithBatcher(exporter),
		trace.WithResource(res),
		trace.WithSampler(trace.AlwaysSample()),
	)

	return provider, nil
}

func (tm *PulseTraceManager) createProjectTraceResource(projectID string) (*resource.Resource, error) {
	// Start with base attributes
	attributes := []attribute.KeyValue{
		semconv.ServiceName(tm.baseConfig.ServiceName),
		semconv.ServiceVersion(tm.baseConfig.ServiceVersion),
		semconv.DeploymentEnvironment(tm.baseConfig.Environment),
		attribute.String("project.id", projectID),
	}

	// Add custom resource attributes
	for key, value := range tm.baseConfig.ResourceAttributes {
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

// Shutdown gracefully shuts down all project providers
func (tm *PulseTraceManager) Shutdown(ctx context.Context) error {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	var errors []error
	for projectID, provider := range tm.projectTraceProviders {
		if err := provider.Shutdown(ctx); err != nil {
			errors = append(errors, fmt.Errorf("failed to shutdown provider for project %s: %w", projectID, err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("shutdown errors: %v", errors)
	}

	return nil
}

// GetTracer returns a project-specific tracer
func (tm *PulseTraceManager) GetTracer(projectID, tracerName string) (*Tracer, error) {
	provider, err := tm.GetTracerProvider(projectID)
	if err != nil {
		return nil, err
	}

	tracer := provider.Tracer(tracerName)
	return &Tracer{
		tracer: tracer,
		name:   tracerName,
	}, nil
}
