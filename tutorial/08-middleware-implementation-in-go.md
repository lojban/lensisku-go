# Chapter 8: Middleware Implementation in Go

In this chapter, we'll explore how to implement middleware in Go web applications, focusing on translating the middleware patterns from our Rust project. We'll cover both standard middleware concepts and specific implementations that mirror our Rust application's functionality.

## Middleware Concepts in Go Web Applications

In Go web applications, middleware is typically implemented using the `http.Handler` interface. The standard pattern looks like this:

```go
type Middleware func(http.Handler) http.Handler

func MyMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Pre-processing logic
        next.ServeHTTP(w, r)
        // Post-processing logic
    })
}
```

This pattern allows you to wrap handlers and execute code before and after request processing. Let's look at common middleware implementations:

### Logging Middleware

Similar to Actix Web's `Logger`, here's a basic logging middleware in Go:

```go
func LoggingMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        
        // Create a custom response writer to capture status code
        rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}
        
        // Process request
        next.ServeHTTP(rw, r)
        
        // Log after processing
        duration := time.Since(start)
        log.Printf(
            "%s %s %d %v",
            r.Method,
            r.URL.Path,
            rw.status,
            duration,
        )
    })
}
```

### CORS Middleware

Translating our Actix CORS setup to Go:

```go
func CORSMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Mirror our Rust CORS configuration
        w.Header().Set("Access-Control-Allow-Origin", "*")
        w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE")
        w.Header().Set("Access-Control-Allow-Headers", 
            "Authorization, Accept, Content-Type")
        w.Header().Set("Access-Control-Max-Age", "3600")

        // Handle preflight requests
        if r.Method == "OPTIONS" {
            w.WriteHeader(http.StatusOK)
            return
        }

        next.ServeHTTP(w, r)
    })
}
```

### Request ID Middleware

A simple request ID middleware for request tracing:

```go
func RequestIDMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        requestID := uuid.New().String()
        w.Header().Set("X-Request-ID", requestID)
        
        // Add request ID to context
        ctx := context.WithValue(r.Context(), "request_id", requestID)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

### Recovery Middleware

Handling panics gracefully:

```go
func RecoveryMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        defer func() {
            if err := recover(); err != nil {
                log.Printf("panic: %v", err)
                http.Error(w, "Internal Server Error", 
                    http.StatusInternalServerError)
            }
        }()
        
        next.ServeHTTP(w, r)
    })
}
```

## Translating Actix Web Middleware Concepts

### Request Lifecycle and Middleware Chaining

In Go, middleware is chained by wrapping handlers:

```go
func SetupMiddleware(handler http.Handler) http.Handler {
    // Chain middleware from outside to inside
    handler = LoggingMiddleware(handler)
    handler = CORSMiddleware(handler)
    handler = RequestIDMiddleware(handler)
    handler = RecoveryMiddleware(handler)
    return handler
}
```

### Redis Cache Implementation

Translating our `RedisCache` middleware from Rust:

```go
type RedisCache struct {
    client  *redis.Client
    ttl     time.Duration
}

func NewRedisCache(redisURL string, ttl time.Duration) (*RedisCache, error) {
    opts, err := redis.ParseURL(redisURL)
    if err != nil {
        return nil, fmt.Errorf("parse redis URL: %w", err)
    }
    
    client := redis.NewClient(opts)
    return &RedisCache{
        client: client,
        ttl:    ttl,
    }, nil
}

func (rc *RedisCache) Middleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        key := r.URL.Path // Simple key based on path
        
        // Try to get from cache
        if cached, err := rc.client.Get(r.Context(), key).Result(); err == nil {
            w.Write([]byte(cached))
            return
        }
        
        // Create response recorder
        rec := httptest.NewRecorder()
        next.ServeHTTP(rec, r)
        
        // Cache the response
        if rec.Code == http.StatusOK {
            rc.client.Set(r.Context(), key, rec.Body.String(), rc.ttl)
        }
        
        // Copy response to original writer
        for k, v := range rec.Header() {
            w.Header()[k] = v
        }
        w.WriteHeader(rec.Code)
        rec.Body.WriteTo(w)
    })
}
```

### Rate Limiting Implementation

Translating our rate limiters from Rust:

```go
type RateLimiter struct {
    client *redis.Client
    key    string
    limit  rate.Limit
    burst  int
}

func NewRateLimiter(redisURL, key string, rps float64, burst int) (*RateLimiter, error) {
    opts, err := redis.ParseURL(redisURL)
    if err != nil {
        return nil, fmt.Errorf("parse redis URL: %w", err)
    }
    
    return &RateLimiter{
        client: redis.NewClient(opts),
        key:    key,
        limit:  rate.Limit(rps),
        burst:  burst,
    }, nil
}

func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
    limiter := rate.NewLimiter(rl.limit, rl.burst)
    
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Check rate limit
        if !limiter.Allow() {
            http.Error(w, "Rate limit exceeded", 
                http.StatusTooManyRequests)
            return
        }
        
        next.ServeHTTP(w, r)
    })
}

// Specialized limiters
func NewPasswordResetLimiter(redisURL string) (*RateLimiter, error) {
    return NewRateLimiter(redisURL, "password_reset", 1, 3)
}

func NewEmailConfirmationLimiter(redisURL string) (*RateLimiter, error) {
    return NewRateLimiter(redisURL, "email_confirmation", 1, 5)
}
```

These implementations provide similar functionality to our Rust project's limiters, using Redis for distributed rate limiting.

## Integration with Go Web Frameworks

While the examples above use the standard `net/http` package, many Go web frameworks provide their own middleware systems. Here's how to adapt our middleware for popular frameworks:

### Echo Framework

```go
func LoggingMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
    return func(c echo.Context) error {
        start := time.Now()
        err := next(c)
        duration := time.Since(start)
        
        log.Printf(
            "%s %s %d %v",
            c.Request().Method,
            c.Path(),
            c.Response().Status,
            duration,
        )
        
        return err
    }
}
```

### Gin Framework

```go
func LoggingMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        start := time.Now()
        c.Next()
        duration := time.Since(start)
        
        log.Printf(
            "%s %s %d %v",
            c.Request.Method,
            c.Request.URL.Path,
            c.Writer.Status(),
            duration,
        )
    }
}
```

## Best Practices and Considerations

1. **Order Matters**: Consider the order of middleware carefully. For example:
   - Recovery middleware should be first to catch panics in other middleware
   - CORS middleware should be early to handle preflight requests
   - Logging middleware is typically last to capture timing information

2. **Context Usage**: Use `context.Context` to pass data between middleware:
   ```go
   ctx := context.WithValue(r.Context(), "user_id", userID)
   r = r.WithContext(ctx)
   ```

3. **Error Handling**: Implement consistent error handling across middleware:
   ```go
   if err != nil {
       log.Printf("middleware error: %v", err)
       http.Error(w, "Internal Server Error", 
           http.StatusInternalServerError)
       return
   }
   ```

4. **Performance**: Consider caching strategies and middleware overhead:
   - Use in-memory caches for frequently accessed data
   - Implement request coalescing for concurrent similar requests
   - Monitor middleware execution time

By following these patterns and implementations, you can create robust and maintainable middleware in your Go web applications that provides similar functionality to our Rust project.