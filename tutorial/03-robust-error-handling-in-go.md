# Chapter 3: Robust Error Handling in Go

This chapter explores how to implement robust error handling in Go, using our Rust codebase as a reference point. We'll examine how to translate Rust's powerful error handling patterns into idiomatic Go code while maintaining the same level of reliability and expressiveness.

## Go's `error` Interface: The Idiomatic Approach

At the heart of Go's error handling is the simple yet powerful `error` interface:

```go
type error interface {
    Error() string
}
```

Unlike Rust's `Result<T, E>` enum, Go uses return values for error handling. While this approach may seem more verbose at first, it encourages explicit error handling and makes control flow immediately visible in the code.

## Translating Rust's Error Types to Go

Our Rust codebase uses a comprehensive error enum (`AppError`) defined in `error.rs`. Let's examine how to translate this pattern to Go:

```go
// AppError represents application-specific errors
type AppError struct {
    Kind    ErrorKind
    Message string
    Err     error  // Wrapped error
}

// ErrorKind represents different types of errors
type ErrorKind int

const (
    ErrorDatabase ErrorKind = iota
    ErrorMigration
    ErrorIO
    ErrorConfig
    ErrorAuth
    ErrorNotFound
    ErrorExternalService
    ErrorValidation
    ErrorBadRequest
    ErrorUnauthorized
    ErrorInternal
    ErrorJSON
    ErrorRedis
    ErrorJWT
)

// Error satisfies the error interface
func (e *AppError) Error() string {
    if e.Err != nil {
        return fmt.Sprintf("%s: %v", e.Message, e.Err)
    }
    return e.Message
}
```

### Creating Custom Error Types

In Go, we implement the `error` interface to create custom error types. This approach offers flexibility similar to Rust's error enums:

```go
// Constructor functions for different error types
func NewDatabaseError(msg string, err error) *AppError {
    return &AppError{
        Kind:    ErrorDatabase,
        Message: fmt.Sprintf("Database error: %s", msg),
        Err:     err,
    }
}

func NewConfigError(errors []string) *AppError {
    return &AppError{
        Kind:    ErrorConfig,
        Message: fmt.Sprintf("Configuration errors: %s", strings.Join(errors, ", ")),
    }
}
```

### Error Wrapping and Context

Go 1.13 introduced error wrapping with `fmt.Errorf` and the `%w` verb, providing functionality similar to Rust's error wrapping:

```go
// Wrapping errors with additional context
if err := db.Query(); err != nil {
    return fmt.Errorf("failed to execute query: %w", err)
}
```

Using `errors.Is` and `errors.As` for error checking:

```go
func handleError(err error) {
    // Check if the error is of a specific type
    var appErr *AppError
    if errors.As(err, &appErr) {
        switch appErr.Kind {
        case ErrorDatabase:
            // Handle database error
        case ErrorAuth:
            // Handle authentication error
        }
    }

    // Check for specific error instances
    if errors.Is(err, sql.ErrNoRows) {
        // Handle not found case
    }
}
```

## Error Handling Strategies

### Returning Errors vs. Panicking

In Go, we generally prefer returning errors over panicking. However, there are legitimate use cases for both:

```go
// Prefer returning errors for expected error conditions
func loadConfig(path string) (*Config, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, fmt.Errorf("failed to read config file: %w", err)
    }
    // ...
}

// Use panic for truly exceptional conditions that shouldn't occur
func mustLoadConfig(path string) *Config {
    config, err := loadConfig(path)
    if err != nil {
        panic(fmt.Sprintf("failed to load required config: %v", err))
    }
    return config
}
```

When using panics, consider implementing recovery middleware:

```go
func RecoveryMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        defer func() {
            if err := recover(); err != nil {
                log.Printf("panic recovered: %v", err)
                http.Error(w, "Internal Server Error", http.StatusInternalServerError)
            }
        }()
        next.ServeHTTP(w, r)
    })
}
```

## Centralized vs. Localized Error Handling

### Centralized Error Handling

Implement a central error handler for consistent error responses:

```go
func handleError(err error) *APIResponse {
    var appErr *AppError
    if errors.As(err, &appErr) {
        switch appErr.Kind {
        case ErrorNotFound:
            return NewErrorResponse(http.StatusNotFound, appErr.Message)
        case ErrorValidation:
            return NewErrorResponse(http.StatusBadRequest, appErr.Message)
        case ErrorUnauthorized:
            return NewErrorResponse(http.StatusUnauthorized, appErr.Message)
        }
    }
    // Default to internal server error
    return NewErrorResponse(http.StatusInternalServerError, "Internal server error")
}
```

### Localized Error Handling

Some errors are better handled close to their source:

```go
func (s *Service) CreateUser(ctx context.Context, user *User) error {
    if err := user.Validate(); err != nil {
        // Handle validation errors locally
        return NewValidationError("invalid user data: %v", err)
    }
    
    if err := s.db.Create(ctx, user); err != nil {
        // Propagate database errors up
        return fmt.Errorf("failed to create user: %w", err)
    }
    
    return nil
}
```

## Integrating with Logging Libraries

Using structured logging with errors:

```go
func (s *Service) ProcessOrder(ctx context.Context, order *Order) error {
    logger := log.With().
        Str("order_id", order.ID).
        Str("user_id", order.UserID).
        Logger()

    if err := s.validateOrder(order); err != nil {
        logger.Error().Err(err).Msg("order validation failed")
        return NewValidationError("invalid order: %v", err)
    }

    if err := s.processPayment(order); err != nil {
        logger.Error().Err(err).
            Str("payment_method", order.PaymentMethod).
            Msg("payment processing failed")
        return NewExternalServiceError("payment failed: %v", err)
    }

    logger.Info().Msg("order processed successfully")
    return nil
}
```

This approach provides:
- Contextual information with each error
- Structured logs for easier parsing and analysis
- Consistent error handling across the application

By following these patterns, we maintain the robustness of our Rust error handling while embracing Go's idioms. The result is a system that's both reliable and maintainable, with clear error handling paths and meaningful error context for debugging.