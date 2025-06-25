package pulse_otel

import (
	"context"
	"fmt"
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

// TenantManager manages OpenTelemetry providers for multiple tenants
type TenantManager struct {
	tenantProviders map[string]*trace.TracerProvider
	baseConfig      *Config
	mutex           sync.RWMutex
}

// NewTenantManager creates a new tenant manager
func NewTenantManager(baseConfig *Config) *TenantManager {
	if baseConfig == nil {
		baseConfig = DefaultConfig()
	}

	return &TenantManager{
		tenantProviders: make(map[string]*trace.TracerProvider),
		baseConfig:      baseConfig,
	}
}

// GetTracerProvider returns or creates a tracer provider for a specific tenant
func (tm *TenantManager) GetTracerProvider(tenantID string) (*trace.TracerProvider, error) {
	tm.mutex.RLock()
	provider, exists := tm.tenantProviders[tenantID]
	tm.mutex.RUnlock()

	if exists {
		return provider, nil
	}

	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	// Double-check after acquiring write lock
	if provider, exists := tm.tenantProviders[tenantID]; exists {
		return provider, nil
	}

	// Create new provider for tenant
	provider, err := tm.createTenantProvider(tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to create provider for tenant %s: %w", tenantID, err)
	}

	tm.tenantProviders[tenantID] = provider
	return provider, nil
}

func (tm *TenantManager) createTenantProvider(tenantID string) (*trace.TracerProvider, error) {
	ctx := context.Background()

	// Create tenant-specific resource
	res, err := tm.createTenantResource(tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Create tenant-specific endpoint
	tenantEndpoint := fmt.Sprintf("http://%s:4318", tenantID)

	// Create OTLP HTTP exporter for this tenant
	exporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpoint(tenantEndpoint),
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

func (tm *TenantManager) createTenantResource(tenantID string) (*resource.Resource, error) {
	// Start with base attributes
	attributes := []attribute.KeyValue{
		semconv.ServiceName(tm.baseConfig.ServiceName),
		semconv.ServiceVersion(tm.baseConfig.ServiceVersion),
		semconv.DeploymentEnvironment(tm.baseConfig.Environment),
		attribute.String("tenant.id", tenantID),
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

// Shutdown gracefully shuts down all tenant providers
func (tm *TenantManager) Shutdown(ctx context.Context) error {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	var errors []error
	for tenantID, provider := range tm.tenantProviders {
		if err := provider.Shutdown(ctx); err != nil {
			errors = append(errors, fmt.Errorf("failed to shutdown provider for tenant %s: %w", tenantID, err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("shutdown errors: %v", errors)
	}

	return nil
}

// GetTracer returns a tenant-specific tracer
func (tm *TenantManager) GetTracer(tenantID, tracerName string) (*Tracer, error) {
	provider, err := tm.GetTracerProvider(tenantID)
	if err != nil {
		return nil, err
	}

	tracer := provider.Tracer(tracerName)
	return &Tracer{
		tracer: tracer,
		name:   tracerName,
	}, nil
}
