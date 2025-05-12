// Package auth provides authentication and authorization functionality
// This file, `dto.go` (Data Transfer Object), defines structures used for
// transferring data in API requests and responses related to authentication.
// These are similar to DTOs in Nest.js, often used with validation pipes/decorators.
package auth

// RegisterRequest represents the registration request payload
// It contains the fields necessary for a new user to register.
// Struct tags `json:"..."` define how these fields map to JSON keys.
// `example:"..."` tags are for Swagger/OpenAPI documentation.
type RegisterRequest struct {
	Username string `json:"username" example:"newuser"`
	Email    string `json:"email" example:"user@example.com"`
	Password string `json:"password" example:"strongpassword123"`
}

// LoginRequest represents the login request payload
// Contains fields for a user to log in.
type LoginRequest struct {
	Login    string `json:"login" example:"user@example.com"` // Can be username or email
	Password string `json:"password" example:"strongpassword123"`
}

// TokenResponse represents the authentication token response
// This structure is returned to the client upon successful login or token refresh.
type TokenResponse struct {
	AccessToken  string `json:"access_token" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."`
	RefreshToken string `json:"refresh_token" example:"def50200..."`
	// TokenType and ExpiresIn are common fields in OAuth2-like token responses.
	// TokenType and ExpiresIn can be kept or removed; for now, let's keep them as they are common.
	// If they cause issues with Rust compatibility, they can be removed.
	TokenType string `json:"token_type" example:"Bearer"` // Typically "Bearer" for JWTs.
	ExpiresIn int64  `json:"expires_in" example:"3600"` // Expiration time of the access token in seconds.
}

// RefreshTokenRequest represents the token refresh request payload
// Used when a client wants to obtain a new access token using a refresh token.
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" example:"def50200..."`
}

// TokenClaims represents the custom claims in the JWT token.
// This will be used internally for generating and validating tokens.
type TokenClaims struct {
	// These are custom claims specific to this application, embedded within the JWT.
	// `UserID` is essential for identifying the authenticated user.
	UserID    int64  `json:"user_id"`
	TokenType string `json:"token_type"` // "access" or "refresh"
	// We can add more claims like username/email if needed for the access token,
	// but UserID is the primary identifier.
	// For now, keeping it minimal as per the direct requirements for token generation.
	// jwt.RegisteredClaims will be embedded for standard claims like exp, iat, nbf.
}