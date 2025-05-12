// Package apperror defines a centralized system for application-specific errors.
// This approach promotes consistent error handling and responses across the application.
// It's similar in concept to Nest.js's Exception Filters, where you can catch specific
// error types and customize the HTTP response.
package apperror

import (
	"errors"
	"fmt"
	// `net/http` is used for HTTP status codes.
	"net/http"
)

// ErrorType is an enumeration (using `iota`) for different categories of application errors.
// ErrorType defines the type of application error
type ErrorType int

const (
	// UnknownError is for unspecified errors
	UnknownError ErrorType = iota
	// DatabaseError represents an error originating from the database
	DatabaseError
	// ConfigError represents an error related to application configuration
	ConfigError
	// AuthError represents an authentication error (e.g. invalid credentials)
	AuthError
	// UnauthorizedError represents an authorization error (e.g. insufficient permissions)
	UnauthorizedError
	// NotFoundError represents a resource not found error
	NotFoundError
	// ValidationError represents an input validation error
	ValidationError
	// BadRequestError represents a generic bad request
	BadRequestError
	// InternalError represents a generic internal server error
	InternalError
	// ExternalServiceError represents an error from an external service
	ExternalServiceError
	// MigrationError represents an error during database migrations
	MigrationError
	// ConflictError represents a conflict, e.g., resource already exists
	ConflictError
)

// AppError is a custom error type for the application
// It embeds the standard `error` interface implicitly by having an `Error()` method.
// It also allows wrapping an underlying error (`Err`) for more detailed debugging.
type AppError struct {
	Type    ErrorType
	Message string
	Err     error // Underlying error
}

// Error returns the string representation of the error, satisfying the `error` interface.
// This method is automatically called when an `AppError` is treated as a standard `error`.
func (e *AppError) Error() string {
	if e.Err != nil {
		// If there's an underlying error, include its message.
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

// Unwrap returns the underlying error. This is part of Go's error wrapping convention (Go 1.13+),
// allowing `errors.Is` and `errors.As` to inspect the chain of wrapped errors.
// Unwrap returns the underlying error
func (e *AppError) Unwrap() error {
	return e.Err
}

// StatusCode returns the HTTP status code appropriate for the error type
func (e *AppError) StatusCode() int {
	// This switch statement maps our custom `ErrorType` to standard HTTP status codes.
	switch e.Type {
	case DatabaseError:
		return http.StatusInternalServerError
	case ConfigError:
		return http.StatusInternalServerError
	case AuthError:
		return http.StatusUnauthorized
	case UnauthorizedError:
		// Distinguishing between 401 (authentication - not logged in) and 403 (authorization - logged in but no permission).
		// HTTP 403 Forbidden is typically used for authorization issues (valid token, but no permission)
		// HTTP 401 Unauthorized is for authentication issues (no/invalid token)
		// Here, `UnauthorizedError` is mapped to 403, implying it's for authorization. `AuthError` is for 401.
		return http.StatusForbidden
	case NotFoundError:
		return http.StatusNotFound
	case ValidationError:
		return http.StatusBadRequest
	case BadRequestError:
		return http.StatusBadRequest
	case InternalError:
		return http.StatusInternalServerError
	case ExternalServiceError:
		return http.StatusBadGateway
	case MigrationError:
		return http.StatusInternalServerError
	case ConflictError:
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}

// NewAppError creates a new AppError. This is a generic constructor.
// It's useful for creating errors with types not covered by specific constructors
// or when the error type is determined dynamically.
// This function acts as a factory for `AppError` instances.
func NewAppError(errType ErrorType, message string, underlyingError error) *AppError {
	return &AppError{
		Type:    errType,
		Message: message,
		Err:     underlyingError,
	}
}

// Constructor functions for specific error types
// These provide a more readable and type-safe way to create common `AppError` types.
// For example, `NewDatabaseError("message", err)` is clearer than `NewAppError(DatabaseError, "message", err)`.

// NewDatabaseError creates a new DatabaseError
func NewDatabaseError(message string, underlyingError error) *AppError {
	return NewAppError(DatabaseError, message, underlyingError)
}

// NewConfigError creates a new ConfigError
func NewConfigError(message string, underlyingError error) *AppError {
	return NewAppError(ConfigError, message, underlyingError)
}

// NewAuthError creates a new AuthError (for authentication issues)
func NewAuthError(message string, underlyingError error) *AppError {
	return NewAppError(AuthError, message, underlyingError)
}

// NewUnauthorizedError creates a new UnauthorizedError (for authorization issues)
func NewUnauthorizedError(message string, underlyingError error) *AppError {
	return NewAppError(UnauthorizedError, message, underlyingError)
}

// NewNotFoundError creates a new NotFoundError
func NewNotFoundError(message string, underlyingError error) *AppError {
	return NewAppError(NotFoundError, message, underlyingError)
}

// NewValidationError creates a new ValidationError
func NewValidationError(message string, underlyingError error) *AppError {
	return NewAppError(ValidationError, message, underlyingError)
}

// NewBadRequestError creates a new BadRequestError
func NewBadRequestError(message string, underlyingError error) *AppError {
	return NewAppError(BadRequestError, message, underlyingError)
}

// NewInternalError creates a new InternalError
func NewInternalError(message string, underlyingError error) *AppError {
	return NewAppError(InternalError, message, underlyingError)
}

// NewExternalServiceError creates a new ExternalServiceError
func NewExternalServiceError(message string, underlyingError error) *AppError {
	return NewAppError(ExternalServiceError, message, underlyingError)
}

// NewMigrationError creates a new MigrationError
func NewMigrationError(message string, underlyingError error) *AppError {
	return NewAppError(MigrationError, message, underlyingError)
}

// NewConflictError creates a new ConflictError
func NewConflictError(message string, underlyingError error) *AppError {
	return NewAppError(ConflictError, message, underlyingError)
}

// ErrorResponse represents a generic error response payload for API clients.
type ErrorResponse struct {
	// `example` is a struct tag often used by Swagger/OpenAPI documentation generators.
	Error string `json:"error" example:"A description of the error"`
}

// ToResponse converts an AppError to an ErrorResponse suitable for API responses.
// This ensures that all API error responses have a consistent JSON structure.
func (e *AppError) ToResponse() ErrorResponse {
	// Only the user-facing `Message` is included in the response, not the underlying `Err` details.
	return ErrorResponse{Error: e.Message}
}

// FromError attempts to convert a generic error to an *AppError.
// It returns the *AppError and true if successful, otherwise nil and false.
func FromError(err error) (*AppError, bool) {
	if err == nil {
		return nil, false
	}
	ae, ok := err.(*AppError)
	return ae, ok
}

// Helper functions to check error types
// These functions use `errors.As` to check if an error in a chain is of a specific `AppError` type.
// This is more robust than direct type assertion (`err.(*AppError)`) when errors might be wrapped.

// IsNotFound checks if an error is a NotFound error
func IsNotFound(err error) bool {
	var appErr *AppError
	return errors.As(err, &appErr) && appErr.Type == NotFoundError
}

// IsAuthError checks if an error is an AuthError (authentication problem)
func IsAuthError(err error) bool {
	var appErr *AppError
	return errors.As(err, &appErr) && appErr.Type == AuthError
}

// IsUnauthorizedError checks if an error is an UnauthorizedError (authorization problem)
func IsUnauthorizedError(err error) bool {
	var appErr *AppError
	return errors.As(err, &appErr) && appErr.Type == UnauthorizedError
}

// IsValidationError checks if an error is a Validation error
func IsValidationError(err error) bool {
	var appErr *AppError
	return errors.As(err, &appErr) && appErr.Type == ValidationError
}

// IsConflictError checks if an error is a Conflict error
func IsConflictError(err error) bool {
	var appErr *AppError
	return errors.As(err, &appErr) && appErr.Type == ConflictError
}