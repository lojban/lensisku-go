// Package auth, as part of the authentication module.
// This file, `context.go`, deals with utilities for managing authentication-related data
// within the Go `context.Context`. The context is a standard way in Go to carry request-scoped
// values, cancellation signals, and deadlines across API boundaries and between goroutines.

// In Go, a context.Context (often just called ctx) is an object that carries deadlines, cancellation signals, and other request-scoped values across API boundaries and between goroutines (Go's lightweight threads). You'll see it as the first parameter in many functions, especially those involved in I/O, network requests, or long-running computations. The file /home/user/lojban/lensisku/archive/lensisku-go/auth/context.go uses it to pass around authentication claims.

// What are "Cancellation Signals"? A cancellation signal is a way for one part of your program to tell another part (or multiple parts) that the work it's doing is no longer needed and it should stop. Imagine you make an HTTP request to a server. The server starts processing it, perhaps by querying a database. If the client who made the request hangs up or their connection times out, the server ideally shouldn't continue wasting resources on that request. The context.Context provides a mechanism to signal this cancellation.
package auth

import (
	"context"
)

// `contextKey` is a custom type for context keys. Using a custom type prevents collisions
// with context keys defined in other packages. It's a common Go idiom.
type contextKey string

const (
	// `claimsContextKey` is the specific key used to store authentication claims in the context.
	claimsContextKey contextKey = "auth_claims"
)

// NewContextWithClaims creates a new context with CustomClaims
// It takes an existing parent context and returns a new child context containing the claims.
// This is the standard way to add values to a context.
func NewContextWithClaims(ctx context.Context, claims *CustomClaims) context.Context {
	return context.WithValue(ctx, claimsContextKey, claims)
}

// ClaimsFromContext extracts CustomClaims from context
// It retrieves the claims stored by `NewContextWithClaims`.
// The second return value (`bool`) indicates if the claims were found and of the correct type.
func ClaimsFromContext(ctx context.Context) (*CustomClaims, bool) {
	claims, ok := ctx.Value(claimsContextKey).(*CustomClaims)
	return claims, ok
}

// RequireAuth returns a function that checks if the user has required roles
func RequireAuth(roles ...string) func(ctx context.Context) error {
	// This function uses a higher-order function pattern: it returns another function.
	// The returned function performs the actual authorization check.
	// `roles ...string` is a variadic parameter, allowing zero or more role strings.
	return func(ctx context.Context) error {
		_, ok := ClaimsFromContext(ctx)
		if !ok {
			return ErrNoAuthContext
		}

		if len(roles) == 0 {
			return nil // No specific roles required
		}

		// TODO: Implement role checking once roles are added to CustomClaims
		// For now, if roles are specified, we deny access as roles are not yet in claims.
		// If no roles are specified, access is allowed.
		if len(roles) > 0 {
			// This part needs to be implemented correctly when roles are in CustomClaims
			// For now, let's assume if specific roles are required, and we can't check them,
			// it's insufficient permissions.
			// A proper implementation would iterate through claims.Roles once available.
			return ErrInsufficientPermissions
		}
		// If no specific roles are required by the endpoint, allow access.
		return nil
	}
}

// Common authentication errors
// These are predefined error variables for common authentication/authorization issues.
// Using predefined error variables allows for easy comparison using `errors.Is`.
var (
	ErrNoAuthContext           = &authError{message: "no authentication context found"}
	ErrInsufficientPermissions = &authError{message: "insufficient permissions"}
)

// `authError` is a custom error type specific to this package's authorization checks.
// authError implements the error interface for auth-specific errors
type authError struct {
	message string
}

func (e *authError) Error() string {
	return e.message
}
