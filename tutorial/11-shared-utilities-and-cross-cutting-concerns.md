# Chapter 11: Shared Utilities and Cross-Cutting Concerns

In this chapter, we'll explore how to organize and implement shared utilities and cross-cutting concerns in our Go project, translating and improving upon our Rust implementation's approaches. We'll cover utility functions, notification systems, and logging - all crucial aspects that support the core business logic of our application.

## Organizing Shared Utilities in Go

When translating Rust's `utils.rs` module to Go, we need to consider Go's idiomatic package organization. While Rust uses modules with `mod.rs` files, Go favors a flatter package structure with clear boundaries.

### Package Organization

There are two common approaches for organizing utility code in Go:

1. `pkg/utils/` - For utilities that might be useful across different projects
2. `internal/common/` - For project-specific utilities that shouldn't be imported by other projects

For our project, we'll use the `internal/common` approach since our utilities are specific to this application:

```
internal/
├── common/
│   ├── stringutil/     # String manipulation utilities
│   ├── timeutil/      # Date/time handling
│   └── transform/     # Data transformations
```

### Example Utility Implementation

Here's how we might implement some common utilities in Go:

```go
// internal/common/stringutil/cleanup.go
package stringutil

import (
    "strings"
    "unicode"
)

// CleanupSpaces removes redundant whitespace and trims the string
func CleanupSpaces(s string) string {
    return strings.Join(strings.Fields(s), " ")
}

// NormalizeWord prepares a word for consistent comparison
func NormalizeWord(s string) string {
    return strings.Map(func(r rune) rune {
        if unicode.IsSpace(r) || unicode.IsPunct(r) {
            return -1
        }
        return unicode.ToLower(r)
    }, s)
}
```

Notice how Go's utility functions tend to be:
- Single-purpose and well-named
- Using clear parameter and return types
- Documented with clear comments
- Grouped by functionality in separate packages

## Implementing Notification Systems

In our Rust project, notifications were handled through `notifications/service.rs`. Let's implement an equivalent system in Go that handles email notifications and provides extensibility for other notification types.

### Notification Service Interface

First, let's define a notification interface:

```go
// internal/notification/service.go
package notification

type Service interface {
    SendEmail(to string, subject string, body string) error
    SendWebSocketMessage(userID string, message interface{}) error
}
```

### Email Service Implementation

Here's an example implementation using the popular `gomail` package:

```go
// internal/notification/email.go
package notification

import (
    "github.com/go-mail/mail"
)

type EmailConfig struct {
    Host     string
    Port     int
    Username string
    Password string
}

type EmailService struct {
    dialer *mail.Dialer
}

func NewEmailService(config EmailConfig) (*EmailService, error) {
    dialer := mail.NewDialer(config.Host, config.Port, config.Username, config.Password)
    
    // Test connection
    if err := dialer.DialAndVerify(); err != nil {
        return nil, fmt.Errorf("failed to connect to email server: %w", err)
    }
    
    return &EmailService{dialer: dialer}, nil
}

func (s *EmailService) SendEmail(to, subject, body string) error {
    msg := mail.NewMessage()
    msg.SetHeader("To", to)
    msg.SetHeader("Subject", subject)
    msg.SetBody("text/html", body)
    
    return s.dialer.DialAndSend(msg)
}
```

## Logging Best Practices in Go

While Rust uses the `log` crate with `env_logger`, Go has several excellent logging libraries. As of Go 1.21+, the standard library includes `log/slog` for structured logging. Let's explore how to implement logging in our application.

### Using slog (Go 1.21+)

```go
// internal/logger/logger.go
package logger

import (
    "log/slog"
    "os"
)

func InitLogger(env string) {
    var handler slog.Handler
    
    if env == "production" {
        // JSON format for production
        handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
            Level: slog.LevelInfo,
        })
    } else {
        // Text format for development
        handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
            Level: slog.LevelDebug,
        })
    }
    
    logger := slog.New(handler)
    slog.SetDefault(logger)
}
```

Using the logger in your application:

```go
// cmd/api/main.go
package main

import (
    "log/slog"
    "your/app/internal/logger"
    "your/app/internal/notification"
)

func main() {
    logger.InitLogger(os.Getenv("GO_ENV"))
    
    emailSvc, err := notification.NewEmailService(notification.EmailConfig{
        // config details...
    })
    if err != nil {
        slog.Error("failed to initialize email service",
            "error", err,
            "impact", "notifications will not be sent")
    } else {
        slog.Info("email service initialized successfully")
    }
    
    // Rest of the application setup...
}
```

### Structured Logging Benefits

Go's structured logging (whether using `slog` or alternatives like `zerolog` or `zap`) offers several advantages over Rust's `env_logger`:

1. Better performance through efficient allocation
2. Structured data that's easily parseable
3. Built-in support for logging levels and contexts
4. Simple configuration without macro complexity

Example of structured logging with context:

```go
slog.Info("user action completed",
    "user_id", user.ID,
    "action", "profile_update",
    "changes", len(updates),
    "duration_ms", time.Since(start).Milliseconds(),
)
```

## Best Practices for Cross-Cutting Concerns

When implementing shared utilities and cross-cutting concerns:

1. **Package Organization**
   - Keep utilities focused and well-organized
   - Use `internal/` for project-specific code
   - Create clear package boundaries

2. **Interface Design**
   - Define clear interfaces for services
   - Make interfaces small and focused
   - Use composition over inheritance

3. **Error Handling**
   - Return explicit errors from utilities
   - Wrap errors with context
   - Log errors at the appropriate level

4. **Configuration**
   - Make services configurable
   - Use dependency injection
   - Support testing and mocking

5. **Documentation**
   - Document all exported functions and types
   - Include usage examples
   - Explain any non-obvious behaviors

## Conclusion

When transitioning from Rust to Go, organizing shared utilities and cross-cutting concerns requires adapting to Go's idioms and best practices. By leveraging Go's package system, interfaces, and modern logging capabilities, we can create maintainable and efficient utility code that supports our application's core functionality.

Remember that Go emphasizes simplicity and explicitness over Rust's more complex type system and macro capabilities. This often leads to more straightforward implementations that are easier to understand and maintain, even if they might be slightly more verbose.