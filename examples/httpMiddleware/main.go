package httpMiddleware

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/aanshu-ss/s2-otel-instrumentation-go/pulse_otel"
	"go.opentelemetry.io/otel/attribute"
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
	tracer *pulse_otel.Tracer
)

func main() {
	// Initialize OpenTelemetry
	config := pulse_otel.DefaultConfig()
	config.ServiceName = "user-api"
	config.ServiceVersion = "1.0.0"
	config.Environment = "development"
	config.SetEndpoint("http://localhost:4318")
	config.AddResourceAttribute("api.type", "rest")
	config.AddResourceAttribute("team", "backend")

	_, err := pulse_otel.NewPulseOtelManager(config)
	if err != nil {
		log.Fatalf("Failed to initialize OpenTelemetry: %v", err)
	}

	defer func() {
		_, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		// otelManager.Shutdown(ctx)
	}()

	// Initialize tracer
	tracer = pulse_otel.NewTracer("user-api")

	// Create HTTP middleware
	middleware := pulse_otel.NewHTTPMiddleware("user-api")

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

	// Simulate database call with child span
	users, err := fetchUsersFromDB(ctx)
	if err != nil {
		tracer.RecordError(ctx, err)

		writeErrorResponse(w, "Failed to fetch users", http.StatusInternalServerError)
		return
	}

	tracer.AddSpanAttribute(ctx, "users.count", string(len(users)))

	writeSuccessResponse(w, users)
}

func createUserHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var user User
	if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Add request attributes
	tracer.AddSpanAttributes(ctx,
		attribute.String("user.name", user.Name),
		attribute.String("user.email", user.Email),
	)

	// Create user in database
	createdUser, err := createUserInDB(ctx, user)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Add response attributes
	tracer.AddSpanAttributes(ctx,
		attribute.String("created.user_id", createdUser.ID),
		attribute.String("response.status", "created"),
	)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(createdUser)

}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	tracer.AddSpanAttribute(ctx, "handler.name", "health")

	response := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().UTC(),
		"service":   "user-api",
	}

	writeSuccessResponse(w, response)
}

// Simulate database operations with child spans
func fetchUsersFromDB(ctx context.Context) ([]User, error) {
	return pulse_otel.WithSpanReturnTyped(tracer, ctx, "db.fetch_users", func(ctx context.Context) ([]User, error) {
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

		tracer.AddSpanAttribute(ctx, "db.rows_affected", strconv.Itoa(len(users)))
		return users, nil
	})
}

func createUserInDB(ctx context.Context, user User) (User, error) {
	return pulse_otel.WithSpanReturnTyped(tracer, ctx, "db.create_user", func(ctx context.Context) (User, error) {
		tracer.AddSpanAttributes(ctx,
			attribute.String("db.operation", "INSERT"),
			attribute.String("db.table", "users"),
			attribute.String("user.email", user.Email),
		)

		// Simulate database write latency
		time.Sleep(100 * time.Millisecond)

		user.ID = fmt.Sprintf("user_%d", time.Now().Unix())
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
