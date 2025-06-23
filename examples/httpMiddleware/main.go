package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/aanshu-ss/s2-otel-instrumentation-go/otelutils"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

type User struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

type Response struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

var (
	tracer *otelutils.Tracer
	logger *otelutils.Logger
)

func main() {
	// Initialize OpenTelemetry
	config := otelutils.DefaultConfig()
	config.ServiceName = "user-api"
	config.ServiceVersion = "1.0.0"
	config.Environment = "development"
	config.SetEndpoint("http://localhost:4318")
	config.AddResourceAttribute("api.type", "rest")
	config.AddResourceAttribute("team", "backend")

	otelManager, err := otelutils.NewOtelManager(config)
	if err != nil {
		log.Fatalf("Failed to initialize OpenTelemetry: %v", err)
	}

	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		otelManager.Shutdown(ctx)
	}()

	// Initialize tracer and logger
	tracer = otelutils.NewTracer("user-api")
	logger = otelutils.NewLogger("user-api")

	// Create HTTP middleware
	middleware := otelutils.NewHTTPMiddleware("user-api")

	// Setup routes with instrumentation
	http.Handle("/users", middleware.Handler(http.HandlerFunc(getUsersHandler)))
	http.Handle("/users/create", middleware.Handler(http.HandlerFunc(createUserHandler)))
	http.Handle("/health", middleware.Handler(http.HandlerFunc(healthHandler)))

	log.Println("Starting server on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func getUsersHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Add custom span attributes
	tracer.AddSpanAttributes(ctx,
		attribute.String("handler.name", "getUsers"),
		attribute.String("operation.type", "read"),
	)

	logger.Info(ctx, "Getting users list",
		attribute.String("method", r.Method),
		attribute.String("endpoint", "/users"),
	)

	// Simulate database call with child span
	users, err := fetchUsersFromDB(ctx)
	if err != nil {
		tracer.RecordError(ctx, err)
		logger.Error(ctx, "Failed to fetch users", attribute.String("error", err.Error()))

		writeErrorResponse(w, "Failed to fetch users", http.StatusInternalServerError)
		return
	}

	tracer.AddSpanAttribute(ctx, "users.count", string(len(users)))
	logger.Info(ctx, "Successfully retrieved users",
		attribute.Int("users.count", len(users)),
	)

	writeSuccessResponse(w, users)
}

func createUserHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	tracer.AddSpanAttributes(ctx,
		attribute.String("handler.name", "createUser"),
		attribute.String("operation.type", "write"),
	)

	if r.Method != http.MethodPost {
		tracer.SetSpanStatus(ctx, codes.Error, "Method not allowed")
		writeErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var user User
	if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
		tracer.RecordError(ctx, err)
		logger.Error(ctx, "Invalid request body", attribute.String("error", err.Error()))
		writeErrorResponse(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	tracer.AddSpanAttribute(ctx, "user.email", user.Email)
	logger.Info(ctx, "Creating new user",
		attribute.String("user.email", user.Email),
		attribute.String("user.name", user.Name),
	)

	// Simulate user creation with child span
	createdUser, err := createUserInDB(ctx, user)
	if err != nil {
		tracer.RecordError(ctx, err)
		logger.Error(ctx, "Failed to create user", attribute.String("error", err.Error()))
		writeErrorResponse(w, "Failed to create user", http.StatusInternalServerError)
		return
	}

	logger.Info(ctx, "User created successfully",
		attribute.String("user.id", createdUser.ID),
	)

	writeSuccessResponse(w, createdUser)
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	tracer.AddSpanAttribute(ctx, "handler.name", "health")
	logger.Debug(ctx, "Health check requested")

	response := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().UTC(),
		"service":   "user-api",
	}

	writeSuccessResponse(w, response)
}

// Simulate database operations with child spans
func fetchUsersFromDB(ctx context.Context) ([]User, error) {
	return tracer.WithSpanReturn(ctx, "db.fetch_users", func(ctx context.Context) ([]User, error) {
		tracer.AddSpanAttributes(ctx,
			attribute.String("db.operation", "SELECT"),
			attribute.String("db.table", "users"),
		)

		// Simulate database latency
		time.Sleep(50 * time.Millisecond)

		users := []User{
			{ID: "1", Name: "John Doe", Email: "john@example.com"},
			{ID: "2", Name: "Jane Smith", Email: "jane@example.com"},
		}

		tracer.AddSpanAttribute(ctx, "db.rows_affected", string(len(users)))
		return users, nil
	})
}

func createUserInDB(ctx context.Context, user User) (User, error) {
	return tracer.WithSpanReturn(ctx, "db.create_user", func(ctx context.Context) (User, error) {
		tracer.AddSpanAttributes(ctx,
			attribute.String("db.operation", "INSERT"),
			attribute.String("db.table", "users"),
			attribute.String("user.email", user.Email),
		)

		// Simulate database write latency
		time.Sleep(100 * time.Millisecond)

		user.ID = "generated-id-123"

		tracer.AddSpanAttribute(ctx, "user.id", user.ID)
		return user, nil
	})
}

func writeSuccessResponse(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(Response{
		Success: true,
		Data:    data,
	})
}

func writeErrorResponse(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(Response{
		Success: false,
		Error:   message,
	})
}
