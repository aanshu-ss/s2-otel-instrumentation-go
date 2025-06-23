// package main

// import (
// 	"context"
// 	"log"
// 	"time"

// 	"github.com/aanshu-ss/s2-otel-instrumentation-go/otelutils"
// 	"go.opentelemetry.io/otel/attribute"
// )

// func main() {
// 	// Create configuration
// 	config := otelutils.DefaultConfig()
// 	config.ServiceName = "basic-example"
// 	config.ServiceVersion = "1.0.0"
// 	config.Environment = "development"
// 	config.SetEndpoint("http://localhost:4318") // Your OTEL collector endpoint
// 	config.AddResourceAttribute("team", "platform")
// 	config.AddResourceAttribute("component", "example")

// 	// Initialize OpenTelemetry
// 	otelManager, err := otelutils.NewOtelManager(config)
// 	if err != nil {
// 		log.Fatalf("Failed to initialize OpenTelemetry: %v", err)
// 	}

// 	// Ensure proper cleanup
// 	defer func() {
// 		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
// 		defer cancel()
// 		if err := otelManager.Shutdown(ctx); err != nil {
// 			log.Printf("Failed to shutdown OpenTelemetry: %v", err)
// 		}
// 	}()

// 	// Create tracer and logger
// 	tracer := otelutils.NewTracer("basic-example")
// 	logger := otelutils.NewLogger("basic-example")

// 	// Example usage
// 	ctx := context.Background()

// 	// Start a new trace
// 	ctx, span := tracer.StartSpan(ctx, "main-operation")
// 	defer span.End()

// 	// Add span attributes
// 	tracer.AddSpanAttributes(ctx,
// 		attribute.String("operation.type", "example"),
// 		attribute.String("user.id", "user123"),
// 		attribute.Int("batch.size", 100),
// 	)

// 	// Log with trace context
// 	logger.Info(ctx, "Starting main operation",
// 		attribute.String("component", "main"),
// 		attribute.String("action", "start"),
// 	)

// 	// Simulate some work with child spans
// 	simulateWork(ctx, tracer, logger)

// 	logger.Info(ctx, "Completed main operation")

// 	// Wait a bit for export
// 	time.Sleep(2 * time.Second)
// }

// func simulateWork(ctx context.Context, tracer *otelutils.Tracer, logger *otelutils.Logger) {
// 	// Create child span
// 	err := tracer.WithSpan(ctx, "simulate-work", func(ctx context.Context) error {
// 		// Add attributes to child span
// 		tracer.AddSpanAttribute(ctx, "work.type", "simulation")
// 		tracer.AddSpanAttribute(ctx, "work.duration", "1s")

// 		// Log in child span context
// 		logger.Debug(ctx, "Performing simulated work")

// 		// Simulate work
// 		time.Sleep(1 * time.Second)

// 		// Log completion
// 		logger.Info(ctx, "Work completed successfully")

// 		return nil
// 	})

// 	if err != nil {
// 		logger.Error(ctx, "Work failed", attribute.String("error", err.Error()))
// 	}
// }
