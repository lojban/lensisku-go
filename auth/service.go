// Package auth is responsible for handling authentication and authorization logic.
// This includes user registration, login, token generation (JWT), and token validation.
// In a Nest.js analogy, this directory would correspond to an "AuthModule",
// containing services, controllers (handlers in Go), DTOs, and entities related to authentication.
package auth

// Standard library imports for context management, error handling, string manipulation, and time.
import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	// Third-party library for JWT handling. `jwt/v5` indicates version 5.
	"github.com/golang-jwt/jwt/v5"
	// PostgreSQL driver and utilities from the `jackc/pgx` suite.
	// `pgx` is a popular high-performance PostgreSQL driver for Go.
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	// Library for password hashing using bcrypt.
	"golang.org/x/crypto/bcrypt"

	// Internal application packages.
	// `apperror` provides custom error types for consistent error handling.
	"github.com/user/lensisku-go/apperror"
	// `config` provides access to application configuration values.
	"github.com/user/lensisku-go/config"
)

// Constants defining token types and a PostgreSQL error code.
const (
	// tokenTypeAccess is a string constant for access tokens.
	tokenTypeAccess = "access"
	// tokenTypeRefresh is a string constant for refresh tokens.
	tokenTypeRefresh = "refresh"
	// pgUniqueViolation is the PostgreSQL error code for unique constraint violations.
	pgUniqueViolation = "23505" // PostgreSQL unique violation error code
)

// AuthService provides authentication-related services.
type AuthService struct {
	dbPool     *pgxpool.Pool
	authConfig config.AuthConfig
	// In Go, dependencies are typically injected explicitly, often via constructor arguments.
	// This is analogous to constructor injection in Nest.js services.
	// `dbPool` provides database access, and `authConfig` provides authentication-specific settings.
}

// NewAuthService creates a new AuthService.
// This function acts as a constructor, a common pattern in Go for creating instances of structs.
// It takes its dependencies (`dbPool` and `authConfig`) as arguments.
// This manual dependency injection is a key difference from Nest.js's decorator-based DI system.
func NewAuthService(dbPool *pgxpool.Pool, authConfig config.AuthConfig) *AuthService {
	return &AuthService{
		dbPool:     dbPool,
		authConfig: authConfig,
	}
}

// CustomClaims embeds jwt.RegisteredClaims and adds custom fields.
// This struct defines the payload of our JWTs.
// Embedding `jwt.RegisteredClaims` includes standard claims like `iss` (issuer), `exp` (expiration time), etc.
type CustomClaims struct {
	UserID    int    `json:"user_id"`
	TokenType string `json:"token_type"` // "access" or "refresh"
	jwt.RegisteredClaims
}

// Register creates a new user.
// `ctx context.Context` is a standard Go pattern for passing request-scoped data, cancellation signals, and deadlines.
// `req RegisterRequest` is a Data Transfer Object (DTO) carrying the registration data.
func (s *AuthService) Register(ctx context.Context, req RegisterRequest) (*User, error) {
	// Hash the user's password using bcrypt. bcrypt is a strong, adaptive hashing algorithm.
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		// `fmt.Errorf` with `%w` wraps the original error, allowing `errors.Is` or `errors.As` to inspect the error chain.
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// Create a User struct (our domain model for a user).
	user := &User{
		Username: req.Username,
		// It's good practice to store emails in a consistent case, usually lowercase.
		Email:          strings.ToLower(req.Email),
		HashedPassword: string(hashedPassword),
	}

	// Call a private method to perform the database insertion.
	createdUser, err := s.createUser(ctx, user)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
			if strings.Contains(pgErr.ConstraintName, "username") {
				return nil, apperror.NewConflictError("username already exists", nil)
				// Returning a specific `apperror` type allows the handler to set the correct HTTP status code (e.g., 409 Conflict).
			}
			if strings.Contains(pgErr.ConstraintName, "email") {
				return nil, apperror.NewConflictError("email already exists", nil)
			}
		}
		// For other database errors, return a generic database error.
		return nil, apperror.NewDatabaseError("failed to create user", err)
	}
	return createdUser, nil
}

// Login authenticates a user and returns tokens.
func (s *AuthService) Login(ctx context.Context, req LoginRequest) (*TokenResponse, error) {
	// Retrieve the user by their login identifier (username or email).
	user, err := s.getUserByLogin(ctx, req.Login)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// If user not found, return an "invalid credentials" error. Avoid revealing whether the username or password was wrong.
			return nil, apperror.NewUnauthorizedError("invalid credentials", nil)
		}
		// Log the original database error for debugging purposes
		log.Printf("Database error in Login when trying to getUserByLogin: %v", err)
		return nil, apperror.NewDatabaseError("failed to get user", err)
	}

	// Compare the provided password with the stored hashed password.
	// `bcrypt.CompareHashAndPassword` handles the comparison securely.
	err = bcrypt.CompareHashAndPassword([]byte(user.HashedPassword), []byte(req.Password))
	// If passwords don't match, `err` will be `bcrypt.ErrMismatchedHashAndPassword`.
	if err != nil {
		return nil, apperror.NewUnauthorizedError("invalid credentials", nil)
	}

	return s.generateTokens(user.ID)
}

// RefreshToken generates new tokens based on a refresh token.
func (s *AuthService) RefreshToken(ctx context.Context, refreshTokenString string) (*TokenResponse, error) {
	// Validate the incoming refresh token.
	claims, err := s.validateToken(refreshTokenString, tokenTypeRefresh)
	if err != nil {
		return nil, apperror.NewUnauthorizedError(fmt.Sprintf("invalid refresh token: %s", err.Error()), err)
	}

	// Optionally: Check if refresh token is revoked (if implementing revocation list)

	// Generate a new access token.
	newAccessToken, newAccessExpiresAt, err := s.generateSpecificToken(claims.UserID, tokenTypeAccess, s.authConfig.AccessTokenDuration)
	if err != nil {
		return nil, fmt.Errorf("failed to generate new access token: %w", err)
	}

	// For simplicity, we can return the same refresh token or generate a new one.
	// Let's return the same one if it's still valid for a reasonable duration,
	// or generate a new one if its expiry is too close or if we want to rotate them.
	// For this implementation, we'll generate a new access token and return the existing valid refresh token.
	// A more robust solution might involve rotating refresh tokens.

	return &TokenResponse{
		// The new access token.
		AccessToken:  newAccessToken,
		RefreshToken: refreshTokenString, // Or generate a new one
		TokenType:    "Bearer",
		ExpiresIn:    newAccessExpiresAt.Unix(),
	}, nil
}

// generateTokens is a helper function to create both access and refresh tokens for a user.
func (s *AuthService) generateTokens(userID int) (*TokenResponse, error) {
	// Generate the access token.
	accessToken, accessExpiresAt, err := s.generateSpecificToken(userID, tokenTypeAccess, s.authConfig.AccessTokenDuration)
	if err != nil {
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	// Generate the refresh token.
	refreshToken, _, err := s.generateSpecificToken(userID, tokenTypeRefresh, s.authConfig.RefreshTokenDuration)
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	return &TokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
		// `ExpiresIn` typically refers to the access token's expiration.
		ExpiresIn: accessExpiresAt.Unix(),
	}, nil
}

// generateSpecificToken creates a JWT with specified claims, type, and duration.
func (s *AuthService) generateSpecificToken(userID int, tokenType string, duration time.Duration) (string, time.Time, error) {
	expirationTime := time.Now().Add(duration)
	// Define the custom claims for the token.
	claims := &CustomClaims{
		UserID:    userID,
		TokenType: tokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "lensisku",                // Optional: identify the issuer
			Subject:   fmt.Sprintf("%d", userID), // Optional: subject of the token
		},
	}

	// Create a new token object with the specified signing method (HS256) and claims.
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	// Sign the token with the secret key from the auth configuration.
	tokenString, err := token.SignedString([]byte(s.authConfig.JWTSecret))
	if err != nil {
		return "", time.Time{}, fmt.Errorf("failed to sign token: %w", err)
	}
	// Return the signed token string and its expiration time.
	return tokenString, expirationTime, nil
}

// validateToken parses and validates a JWT string.
// It checks the signature, expiration, and expected token type.
func (s *AuthService) validateToken(tokenString string, expectedTokenType string) (*CustomClaims, error) {
	claims := &CustomClaims{}
	// Parse the token string. The key function (`func(token *jwt.Token) (interface{}, error)`)
	// is used to provide the secret key for verification.
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			// Ensure the token's signing method is HMAC, as expected.
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(s.authConfig.JWTSecret), nil
	})

	if err != nil {
		return nil, err
	}

	// Check if the token is valid after parsing (e.g., signature is correct, not expired based on 'exp' claim).
	if !token.Valid {
		return nil, errors.New("token is invalid")
	}

	// Verify that the token type matches the expected type (e.g., "access" or "refresh").
	if claims.TokenType != expectedTokenType {
		return nil, fmt.Errorf("invalid token type: expected %s, got %s", expectedTokenType, claims.TokenType)
	}

	// Check if token is expired (though jwt.ParseWithClaims should handle this based on 'exp' claim)
	if time.Now().Unix() > claims.ExpiresAt.Unix() {
		return nil, errors.New("token has expired")
	}

	return claims, nil
}

// --- Database Helper Functions ---
// These functions encapsulate direct database interactions for the AuthService.
// This separation of concerns is good practice, keeping SQL queries localized.
// In a more complex application, these might reside in a separate "repository" or "data access" layer/package.

func (s *AuthService) createUser(ctx context.Context, user *User) (*User, error) {
	query := `INSERT INTO users (username, email, password) 
              VALUES ($1, $2, $3) 
              RETURNING id, created_at`
	// `s.dbPool.QueryRow` executes the query and expects a single row in return.
	err := s.dbPool.QueryRow(ctx, query, user.Username, user.Email, user.HashedPassword).Scan(&user.ID, &user.CreatedAt)
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (s *AuthService) getUserByLogin(ctx context.Context, login string) (*User, error) {
	// This function allows login with either username or email.
	// Try finding by username first, then by email if it looks like an email
	var user User
	var query string
	var arg interface{}

	if strings.Contains(login, "@") { // Simple check for email format
		query = `SELECT userid as id, username, email, password as hashed_password, created_at FROM users WHERE email = $1`
		arg = strings.ToLower(login)
	} else {
		query = `SELECT userid as id, username, email, password as hashed_password, created_at FROM users WHERE username = $1`
		arg = login
	}

	// Execute the query and scan the results into the `user` struct.
	err := s.dbPool.QueryRow(ctx, query, arg).Scan(&user.ID, &user.Username, &user.Email, &user.HashedPassword, &user.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// If not found by one method, try the other if applicable (e.g., if login was not an email, try it as email)
			// For simplicity now, we just return the error. A more robust check could be added.
			if !strings.Contains(login, "@") { // If it wasn't an email, try searching by email
				query = `SELECT userid as id, username, email, password as hashed_password, created_at FROM users WHERE email = $1`
				errEmail := s.dbPool.QueryRow(ctx, query, strings.ToLower(login)).Scan(&user.ID, &user.Username, &user.Email, &user.HashedPassword, &user.CreatedAt)
				if errEmail != nil { // If still not found or another error occurred
					return nil, errEmail // Return the error from the email search
				}
				return &user, nil
			}
		}
		return nil, err
	}
	return &user, nil
}

// GetUserByUsername retrieves a user by their username.
func (s *AuthService) GetUserByUsername(ctx context.Context, username string) (*User, error) {
	// This function is more specific, only searching by username.
	var user User
	query := `SELECT id, username, email, password as hashed_password, created_at FROM users WHERE username = $1`
	err := s.dbPool.QueryRow(ctx, query, username).Scan(&user.ID, &user.Username, &user.Email, &user.HashedPassword, &user.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperror.NewNotFoundError(fmt.Sprintf("user with username '%s' not found", username), nil)
		}
		return nil, apperror.NewDatabaseError("failed to get user by username", err)
	}
	return &user, nil
}

// GetUserByEmail retrieves a user by their email address.
func (s *AuthService) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	// This function is specific to searching by email.
	var user User
	query := `SELECT id, username, email, password as hashed_password, created_at FROM users WHERE email = $1`
	err := s.dbPool.QueryRow(ctx, query, strings.ToLower(email)).Scan(&user.ID, &user.Username, &user.Email, &user.HashedPassword, &user.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperror.NewNotFoundError(fmt.Sprintf("user with email '%s' not found", email), nil)
		}
		return nil, apperror.NewDatabaseError("failed to get user by email", err)
	}
	return &user, nil
}
