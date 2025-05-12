# Chapter 7: Crafting the HTTP Server in Go

In this chapter, we'll explore how to implement a robust HTTP server in Go that matches the functionality of our Rust implementation. We'll examine both the standard library's `net/http` package and popular third-party frameworks, providing you with the knowledge to make informed decisions for your project.

## Go's `net/http` Package: The Foundation

Go's standard library provides a powerful and efficient HTTP server implementation through the `net/http` package. Unlike many other languages where external frameworks are necessary for basic web functionality, Go's standard library offers a complete solution for building production-ready web services.

Here's a basic example of a `net/http` server:

```go
package main

import (
    "log"
    "net/http"
)

func main() {
    http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte("Hello, World!"))
    })
    
    log.Printf("Starting server on :8080")
    if err := http.ListenAndServe(":8080", nil); err != nil {
        log.Fatalf("Server failed: %v", err)
    }
}
```

The standard library provides:
- Built-in routing with `http.HandleFunc` and `http.Handle`
- Request multiplexing via `http.ServeMux`
- Support for middleware through handler wrapping
- TLS support with `http.ListenAndServeTLS`
- Request parsing utilities for forms, multipart data, and JSON
- Response writing with status codes and headers

## Choosing a Web Framework

While `net/http` is powerful, larger applications often benefit from additional structure and features provided by web frameworks. Let's compare some popular options with Actix Web (our Rust framework):

### Popular Go Web Frameworks

1. **Chi** (`go-chi/chi`):
   - Lightweight and stdlib-compatible
   - Similar middleware system to Actix Web
   - Excellent for REST APIs
   ```go
   r := chi.NewRouter()
   r.Use(middleware.Logger)
   r.Get("/", homeHandler)
   ```

2. **Gin** (`gin-gonic/gin`):
   - High performance
   - Extensive middleware ecosystem
   - Built-in validation
   ```go
   r := gin.Default()
   r.GET("/", func(c *gin.Context) {
       c.JSON(200, gin.H{"message": "Hello"})
   })
   ```

3. **Echo**:
   - Minimalist but feature-rich
   - Great documentation
   - Built-in support for WebSocket
   ```go
   e := echo.New()
   e.Use(middleware.Logger())
   e.GET("/", handleHome)
   ```

4. **Fiber**:
   - Express.js-inspired
   - Built on `fasthttp`
   - Highly performant
   ```go
   app := fiber.New()
   app.Get("/", func(c *fiber.Ctx) error {
       return c.JSON(fiber.Map{"message": "Hello"})
   })
   ```

### Framework Comparison with Actix Web

| Feature | Actix Web | Chi | Gin | Echo |
|---------|-----------|-----|-----|------|
| Performance | Very High | High | High | High |
| Memory Usage | Low | Low | Low | Low |
| Learning Curve | Moderate | Low | Low | Low |
| Middleware System | Rich | Simple | Rich | Rich |
| Community Size | Large | Medium | Very Large | Large |

For our implementation, we'll use Chi due to its similarity to `net/http` and excellent middleware support.

## Translating `server.rs` to Go

Let's implement the equivalent of our Rust server in Go. We'll break this down into several components:

### 1. Server Configuration and State

```go
type AppState struct {
    DB           *sql.DB
    RedisCache   *RedisCache
    Parsers      map[int]*Parser
    PermCache    *PermissionCache
    RateLimiter  *RateLimiter
}

func NewAppState(config Config) (*AppState, error) {
    // Initialize shared state similar to Rust implementation
    state := &AppState{
        Parsers: make(map[int]*Parser),
    }
    
    // Initialize Redis cache with TTL
    cache, err := NewRedisCache(config.RedisURL, 10*time.Hour)
    if err != nil {
        return nil, fmt.Errorf("redis cache init: %w", err)
    }
    state.RedisCache = cache
    
    // Initialize other components...
    return state, nil
}
```

### 2. HTTP Server Setup and Routing

```go
func startServer(state *AppState) error {
    r := chi.NewRouter()
    
    // Middleware setup
    r.Use(middleware.Logger)
    r.Use(middleware.Recoverer)
    r.Use(cors.Handler(cors.Options{
        AllowedOrigins:   []string{"*"},
        AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
        AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
        ExposedHeaders:   []string{"Link"},
        AllowCredentials: true,
        MaxAge:          300,
    }))
    
    // Mount routes with state
    r.Mount("/auth", authRouter(state))
    r.Mount("/users", usersRouter(state))
    r.Mount("/language", languageRouter(state))
    // ... mount other routes
    
    srv := &http.Server{
        Addr:         ":8080",
        Handler:      r,
        ReadTimeout:  15 * time.Second,
        WriteTimeout: 15 * time.Second,
        IdleTimeout:  60 * time.Second,
    }
    
    // Start server with graceful shutdown
    return serveWithGracefulShutdown(srv)
}
```

### 3. Request Handling and Response Generation

```go
func handleCreateUser(state *AppState) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        // Parse JSON request
        var input CreateUserInput
        if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
            respondWithError(w, http.StatusBadRequest, "Invalid request payload")
            return
        }
        
        // Handle business logic
        user, err := state.UserService.CreateUser(r.Context(), input)
        if err != nil {
            // Handle different error types appropriately
            respondWithError(w, http.StatusInternalServerError, err.Error())
            return
        }
        
        // Respond with JSON
        respondWithJSON(w, http.StatusCreated, user)
    }
}
```

### 4. Graceful Shutdown Implementation

```go
func serveWithGracefulShutdown(srv *http.Server) error {
    // Channel to listen for errors coming from the listener.
    serverErrors := make(chan error, 1)
    
    // Start the server
    go func() {
        log.Printf("Server is starting on %s", srv.Addr)
        serverErrors <- srv.ListenAndServe()
    }()
    
    // Channel for graceful shutdown
    shutdown := make(chan os.Signal, 1)
    signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)
    
    // Blocking select
    select {
    case err := <-serverErrors:
        return fmt.Errorf("server error: %w", err)
        
    case sig := <-shutdown:
        log.Printf("Starting shutdown: %v", sig)
        ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
        defer cancel()
        
        // Gracefully shutdown the server
        if err := srv.Shutdown(ctx); err != nil {
            srv.Close()
            return fmt.Errorf("could not stop server gracefully: %w", err)
        }
    }
    
    return nil
}
```

## Key Differences from Rust Implementation

1. **Concurrency Model**: 
   - Go uses goroutines and channels instead of Rust's async/await
   - Worker management is handled by Go's runtime automatically

2. **Error Handling**:
   - Go uses explicit error returns rather than Rust's Result type
   - Error handling is more procedural in Go

3. **Memory Safety**:
   - Go's garbage collector handles memory management
   - No need for explicit lifetime annotations or ownership rules

4. **Middleware Implementation**:
   - Go middleware is typically implemented as function chaining
   - Simpler but less powerful than Rust's actor model

## Best Practices for Go HTTP Servers

1. **Use Context for Timeouts and Cancellation**
   ```go
   ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
   defer cancel()
   ```

2. **Implement Graceful Shutdown**
   - Always handle SIGTERM/SIGINT signals
   - Give ongoing requests time to complete
   - Close resources properly

3. **Structured Logging**
   ```go
   log.With().
       Str("handler", "createUser").
       Err(err).
       Msg("Failed to create user")
   ```

4. **Panic Recovery**
   - Use middleware to recover from panics
   - Log recovered panics for debugging

5. **Rate Limiting**
   - Implement rate limiting at the middleware level
   - Use Redis or in-memory stores for distributed setups

By following these patterns and practices, you can create a robust, performant HTTP server in Go that matches or exceeds the capabilities of the original Rust implementation while maintaining Go's simplicity and readability.

Remember that Go's standard library is often sufficient for many use cases, but frameworks can provide additional structure and features for larger applications. Choose based on your specific needs and constraints.