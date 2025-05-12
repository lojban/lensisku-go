// This is the main entry point of the Lensisku Go application.
// It's responsible for initializing configurations, database connections,
// services, handlers (controllers), setting up the HTTP router and middleware,
// and starting the HTTP server. It also handles graceful shutdown.
//
// Analogy to Nest.js: This file is similar to `main.ts` in a Nest.js application,
// where the Nest application instance is created, modules are configured,
// middleware is applied, and the application is bootstrapped to listen for requests.
// @title Lensisku API
// @version 1.0
// @description API for Lensisku, providing various application functionalities.
// @contact.name API Support
// @contact.email admin@lojban.org
// @license.name MIT
// @license.url https://opensource.org/licenses/MIT
// @BasePath /
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Type 'Bearer YOUR_JWT_TOKEN' to authorize
package main

// Standard library imports
import (
	// `_ "github.com/user/lensisku-go/docs"` imports the generated Swagger docs package
	// for its side effect: registering the Swagger spec. The underscore `_` indicates
	// that the package is imported only for these side effects, and its exported names
	// are not directly used in this file.
	"context"       // Moved for standard library grouping
	"encoding/json" // for local writeError
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	httpSwagger "github.com/swaggo/http-swagger"
	_ "github.com/user/lensisku-go/docs" // Generated Swagger docs

	// Third-party libraries
	// `chi` is a lightweight, idiomatic and composable router for building HTTP services in Go.
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	// `chi/cors` provides CORS (Cross-Origin Resource Sharing) middleware.
	"github.com/go-chi/cors"
	// `godotenv` loads environment variables from a .env file, useful for development.
	"github.com/joho/godotenv"

	// Internal application packages (modules)
	"github.com/user/lensisku-go/apperror"
	"github.com/user/lensisku-go/auth"
	"github.com/user/lensisku-go/background" // For background embedding service
	"github.com/user/lensisku-go/comments"   // Import for comments feature
	"github.com/user/lensisku-go/config"
	"github.com/user/lensisku-go/db"
	"github.com/user/lensisku-go/users" // Import for user profile management
)

// `main` is the entry point function for the executable.
func main() {
	// Load .env file
	// This is often used in development to set environment variables without
	// modifying the system environment. In production, variables are usually set directly.
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: .env file not found or error loading it: %v", err)
	}

	// Load application configuration using the `config` package.
	// `cfg` will hold all configuration settings (database, auth, server).
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize database connection pools using the loaded configuration.
	// `appPool` for general application use, `importPool` for specific import tasks.
	appPool, importPool, err := db.NewDBPools(cfg.DBPools)
	if err != nil {
		// `log.Fatalf` prints the message and exits the application.
		log.Fatalf("Failed to create database pools: %v", err)
	}
	defer appPool.Close()
	defer importPool.Close()

	// Enable required PostgreSQL extensions using import pool
	if err := db.EnableExtensions(importPool); err != nil {
		log.Fatalf("Failed to enable extensions: %v", err)
	}

	// Run database migrations. This section is currently commented out.
	// Migrations ensure the database schema is up-to-date with the application's requirements.
	// Migrations disabled
	// if err := db.RunMigrations(importPool, "./migrations"); err != nil {
	// 	log.Fatalf("Failed to run migrations: %v", err)
	// }

	// Start background embedding calculator
	// ELI5: This is like starting a separate, continuously running helper factory (our embedding service)
	// that will do its work in the background. We give it a way to connect to the database (appPool)
	// and a special signal (embeddingStopChan) to tell it when to shut down.
	// `embeddingStopChan` is a channel used to signal the background service to stop gracefully.
	embeddingStopChan := make(chan struct{})
	background.StartEmbeddingCalculatorService(appPool, embeddingStopChan) // This function launches its own goroutines internally
	log.Println("Background embedding calculator service initiated.")

	// Initialize auth service
	// Services encapsulate business logic. They are instantiated here and their dependencies (like db pool, config) are injected.
	// This is manual dependency injection, common in Go. Nest.js uses a DI container.
	authService := auth.NewAuthService(appPool, *cfg.Auth) // Dereference cfg.Auth
	// Handlers (controllers) use services to process requests.
	authHandlers := auth.NewHandlers(authService)

	// Initialize user service and handlers
	userService := users.NewUserService(appPool)
	userHandlers := users.NewUserHandlers(userService)

	// Initialize comments service and handlers, following the same pattern.
	commentService := comments.NewCommentService(appPool)
	commentHandlers := comments.NewCommentHandler(commentService)

	// Create router and configure middleware
	// `chi.NewRouter()` creates a new Chi router instance.
	r := chi.NewRouter()

	// Middleware setup:
	// Middleware are functions that process requests before they reach the route handlers.
	// `r.Use(...)` applies middleware to all routes registered on this router `r`.
	// In Nest.js, global middleware can be applied using `app.use()`.

	// IMPORTANT: Chi requires all middleware to be registered before any routes
	// Global middleware
	// `middleware.Logger` logs incoming requests.
	r.Use(middleware.Logger) // Log all requests
	// `middleware.Recoverer` recovers from panics in handlers and returns a 500 error.
	r.Use(middleware.Recoverer)                 // Recover from panics
	r.Use(middleware.RequestID)                 // Add request ID to context
	r.Use(middleware.RealIP)                    // Get real IP from proxy headers
	r.Use(middleware.Timeout(60 * time.Second)) // Timeout long-running requests

	// CORS middleware configuration
	r.Use(cors.Handler(cors.Options{
		// `AllowedOrigins: []string{"*"}` allows requests from any origin. For production, this should be restricted.
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Error handling middleware
	// This is a custom middleware for more fine-grained panic recovery and error logging,
	// potentially integrating with the `apperror` system.
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			// `defer func() { ... }()` with `recover()` is a common Go pattern for panic handling.
			defer func() {
				// Recover from panics and convert to 500 error
				if rvr := recover(); rvr != nil {
					log.Printf("Panic: %+v", rvr)
					err := apperror.NewInternalError("internal server error", nil)
					writeError(ww, err)
				}
			}()
			next.ServeHTTP(ww, r)
		})
	})

	// Swagger UI endpoint
	// `httpSwagger.Handler` serves the Swagger UI, using the documentation generated by `swaggo/swag`.
	// `/swagger/doc.json` is the conventional path for the OpenAPI spec JSON file.
	r.Get("/swagger/*", httpSwagger.Handler(
		httpSwagger.URL("/swagger/doc.json"),
	))

	// Auth routes
	// `r.Route("/auth", ...)` groups routes under the "/auth" prefix.
	// This is similar to defining a controller with a base path in Nest.js.
	r.Route("/auth", func(r chi.Router) {
		// `r.Post(...)` maps HTTP POST requests to the specified path to the handler function.
		r.Post("/register", authHandlers.HandleRegister())
		r.Post("/login", authHandlers.HandleLogin())
		r.Post("/refresh", authHandlers.HandleRefreshToken())
	})

	// User profile routes (protected by JWT middleware)
	// These routes are grouped under "/users".
	r.Route("/users", func(r chi.Router) {
		// `r.Use(auth.JWTMiddleware(cfg.Auth))` applies the JWT authentication middleware
		// specifically to this group of routes. Only authenticated users can access these.
		// This is analogous to applying an AuthGuard to a controller or specific routes in Nest.js.
		// Apply JWT middleware to all routes in this group
		r.Use(auth.JWTMiddleware(cfg.Auth)) // cfg.Auth contains JWTSecret

		r.Get("/me", userHandlers.HandleGetUserProfile())
		r.Put("/me", userHandlers.HandleUpdateUserProfile())
	})

	// Comments routes
	// These routes are grouped under "/api/v1/comments".
	// The `/api/v1` prefix is a common practice for versioning APIs.
	r.Route("/api/v1/comments", func(r chi.Router) { // Using /api/v1 prefix for consistency
		// Apply JWT middleware to all routes in this group
		// This ensures that comment-related actions require authentication.
		r.Use(auth.JWTMiddleware(cfg.Auth))
		commentHandlers.RegisterRoutes(r) // Register comment specific routes
	})

	addr := fmt.Sprintf(":%s", cfg.Server.Port)

	// Create server with graceful shutdown
	// `http.Server` provides more control over server behavior than `http.ListenAndServe`.
	srv := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	// The server is started in a separate goroutine so that the main goroutine can continue
	// to listen for shutdown signals.
	go func() {
		log.Printf("Server starting on %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()
	// Wait for interrupt signal
	// This section handles graceful shutdown of the server.
	// ELI5: This part of the code is like having an ear to the ground, listening for a special signal
	// from the operating system that says "it's time to stop" (like when you press Ctrl+C in the terminal).
	// `make(chan os.Signal, 1)` creates a buffered channel to receive OS signals.
	quit := make(chan os.Signal, 1) // `quit` is a channel that will receive the "stop" signal.
	// Tell Go to send SIGINT (Ctrl+C) or SIGTERM (a polite request to terminate) signals to our `quit` channel.
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	// Block until a signal is received on the `quit` channel.
	<-quit // This line will pause and wait until a signal is received on the `quit` channel.

	// Graceful shutdown
	// ELI5: Once we get the "stop" signal, we don't just crash. We try to finish up neatly.
	log.Println("Server shutting down...")
	// We create a "context" with a timeout. This is like saying, "Try to shut down within 30 seconds.
	// If it takes longer, we might have to force it."
	// `context.WithTimeout` creates a context that will be cancelled after the specified duration.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel() // `defer cancel()` ensures that resources used by the context are cleaned up.

	// Signal background services to stop
	// ELI5: We're sending the "time to close up shop" signal to our background embedding factory.
	// The `embeddingStopChan` was given to the background service when it started.
	// `close(channel)` is a special way to signal all listeners on that channel that no more
	// values will be sent and it's time to wrap up. The background service is designed to listen for this.
	log.Println("Signaling background embedding service to stop...")
	close(embeddingStopChan)
	// Note: StartEmbeddingCalculatorService is designed for graceful shutdown internally.
	// It will see the `embeddingStopChan` is closed and start its own cleanup.
	// We might want to add a timeout/wait here if main needs to ensure the background service
	// has fully stopped before the server itself stops, but for now, we just signal.
	// The background service uses WaitGroups to ensure its goroutines finish.

	// `srv.Shutdown(ctx)` attempts to gracefully shut down the HTTP server.
	// Now, tell the main web server (`srv`) to shut down gracefully.
	// It will try to finish handling any ongoing requests before stopping.
	if err := srv.Shutdown(ctx); err != nil { // Pass the timeout context.
		log.Fatalf("Server shutdown failed: %v", err) // If shutdown itself fails.
	}
	log.Println("Server stopped gracefully")
}

// writeError is a local helper for the panic recovery middleware.
// It's kept separate to avoid import cycles if apperror needed to import main or vice-versa.
// This function ensures that panic errors are also formatted using the `apperror` system.
func writeError(w http.ResponseWriter, appErr *apperror.AppError) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(appErr.StatusCode())
	// Encode the `AppError`'s `ErrorResponse` representation to JSON.
	if err := json.NewEncoder(w).Encode(appErr.ToResponse()); err != nil {
		// Fallback if JSON encoding fails
		http.Error(w, `{"error":"Failed to encode error response"}`, http.StatusInternalServerError)
	}
}
