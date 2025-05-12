// Package auth, as part of the authentication module.
// This file, `middleware.go`, defines HTTP middleware related to authentication.
// Middleware are functions that process HTTP requests before they reach the main handler,
// or after the handler has processed them. They are used for cross-cutting concerns.
// In Nest.js, middleware (`@nestjs/common/interfaces/nest-middleware.interface.ts`) and
// guards (`@nestjs/common/interfaces/can-activate.interface.ts`) serve similar purposes.
package auth

import (
	"context"
	"fmt"
	"net/http"
	// `strings` for string manipulation (e.g., splitting the Authorization header).
	"strings"

	// `jwt` library for JWT parsing and validation.
	"github.com/golang-jwt/jwt/v5"
	// Internal packages for application errors and configuration.
	"github.com/user/lensisku-go/apperror"
	"github.com/user/lensisku-go/config"
)

// ContextKey is a type used for context keys to avoid collisions.
// Using a custom string type for context keys is a Go idiom to prevent key collisions
// between different packages.
type ContextKey string

const (
	// UserIDKey is the key used to store the userID in the request context.
	// This constant defines the key under which the authenticated user's ID will be stored.
	UserIDKey ContextKey = "userID"
)

// Claims represents the JWT claims.
// This struct defines the expected structure of the JWT payload (claims).
// It embeds `jwt.RegisteredClaims` for standard claims (like `exp`, `iat`) and adds custom claims.
type Claims struct {
	UserID int `json:"user_id"`
	jwt.RegisteredClaims
}

// JWTMiddleware creates a new JWT authentication middleware.
// It verifies the token from the Authorization header and adds userID to the context.
// This function is a higher-order function: it takes configuration and returns the actual middleware function.
// The returned middleware conforms to the standard Go `func(next http.Handler) http.Handler` pattern.
// This is analogous to a Nest.js Guard that implements `CanActivate`.
func JWTMiddleware(cfg *config.AuthConfig) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				WriteError(w, r, apperror.NewUnauthorizedError("Authorization header is missing", nil))
				return
			}

			// The Authorization header should be in the format "Bearer {token}".
			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
				WriteError(w, r, apperror.NewUnauthorizedError("Authorization header format must be Bearer {token}", nil))
				return
			}

			// Extract the token string.
			tokenString := parts[1]
			claims := &Claims{}

			// Parse and validate the token.
			// The key function provides the secret key used for verifying the token's signature.
			token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
				if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
				}
				return []byte(cfg.JWTSecret), nil
			})

			if err != nil {
				// Handle specific JWT parsing errors.
				if err == jwt.ErrSignatureInvalid {
					WriteError(w, r, apperror.NewUnauthorizedError("Invalid token signature", nil))
					return
				}
				WriteError(w, r, apperror.NewUnauthorizedError(fmt.Sprintf("Invalid token: %v", err), err))
				return
			}

			// Check if the token itself is valid (e.g., not expired, signature correct).
			if !token.Valid {
				WriteError(w, r, apperror.NewUnauthorizedError("Invalid token", nil))
				return
			}

			// Validate custom claims, e.g., ensure UserID is present.
			if claims.UserID == 0 { // Or any other validation for UserID
				WriteError(w, r, apperror.NewUnauthorizedError("Invalid token: user_id claim is missing or invalid", nil))
				return
			}

			// If the token is valid, add the UserID to the request's context.
			// This makes the UserID available to subsequent handlers in the chain.
			// Add userID to context
			ctx := context.WithValue(r.Context(), UserIDKey, claims.UserID)
			// Call the next handler in the chain with the modified context.
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetUserIDFromContext retrieves the userID from the request context.
// This is a helper function for handlers to easily access the UserID set by the middleware.
// Returns 0 and false if userID is not found or not an int.
func GetUserIDFromContext(ctx context.Context) (int, bool) {
	userID, ok := ctx.Value(UserIDKey).(int)
	return userID, ok
}