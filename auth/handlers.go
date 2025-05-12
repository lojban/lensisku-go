// Package auth, as part of the authentication module.
// This file, `handlers.go`, is responsible for handling HTTP requests related to authentication.
// It acts as the "Controller" layer in an MVC or similar architectural pattern, analogous to
// a Controller class in Nest.js (e.g., `AuthController`).
package auth

import (
	"encoding/json"
	"net/http"
	// `apperror` provides standardized error types and responses.
	"github.com/user/lensisku-go/apperror"
)

// Handlers wraps the AuthService to provide HTTP handlers
// It holds a reference to the `AuthService`, which contains the business logic for authentication.
// This is a form of dependency injection, where the service is provided to the handler.
type Handlers struct {
service *AuthService
}

// NewHandlers creates a new Handlers instance
// This is a constructor function, a common Go pattern for creating struct instances and injecting dependencies.
func NewHandlers(service *AuthService) *Handlers {
return &Handlers{service: service}
}

// The `godoc` comments (like `@Summary`, `@Tags`, etc.) are annotations used by tools like
// `swaggo/swag` to generate OpenAPI/Swagger documentation. This is similar to how Nest.js
// uses decorators from `@nestjs/swagger` for API documentation.

// HandleRegister godoc
// @Summary User Registration
// Defines the API endpoint for user registration.
// @Description Registers a new user in the system.
// @Tags Auth
// @Accept json
// @Produce json
// @Param registerBody body auth.RegisterRequest true "User registration details"
// @Success 201 {object} auth.User "User created successfully"
// @Failure 400 {object} apperror.ErrorResponse "Bad Request - Invalid input or missing fields"
// @Failure 409 {object} apperror.ErrorResponse "Conflict - User already exists (username or email)"
// @Failure 500 {object} apperror.ErrorResponse "Internal Server Error"
// @Router /auth/register [post]
// `HandleRegister` returns an `http.HandlerFunc`, which is a function type that Go's `net/http`
// package (and routers like `chi`) can use to handle HTTP requests.
func (h *Handlers) HandleRegister() http.HandlerFunc {
	// The returned function is a closure, capturing the `h *Handlers` receiver (which includes `h.service`).
	return func(w http.ResponseWriter, r *http.Request) {
	// `w http.ResponseWriter` is used to write the HTTP response.
	// `r *http.Request` contains the incoming HTTP request details.

	// Declare a variable `req` of type `RegisterRequest` (our DTO for registration).
	var req RegisterRequest
	// Decode the JSON request body into the `req` struct.
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, r, apperror.NewBadRequestError("invalid request body: "+err.Error(), nil))
		return
	}
	// `defer r.Body.Close()` ensures the request body is closed after the handler finishes.
	defer r.Body.Close()

	// Perform basic validation on the request DTO.
	// Basic validation (can be expanded with a validation library if needed)
	if req.Username == "" || req.Email == "" || req.Password == "" {
		WriteError(w, r, apperror.NewBadRequestError("username, email, and password are required", nil))
		return
	}

	// Call the `Register` method on the `AuthService` to perform the business logic.
	user, err := h.service.Register(r.Context(), req)
	if err != nil {
		WriteError(w, r, err)
		return
	}

	// For registration, typically return 201 Created with the user object (excluding password)
	// or a success message. Here, we return the created user object.
	user.HashedPassword = "" // Ensure hashed password is not sent in response
	// `writeJSON` is a helper function to send a JSON response with a specific status code.
	writeJSON(w, http.StatusCreated, user)
}
}

// HandleLogin godoc
// @Summary User Login
// @Description Logs in an existing user and returns access and refresh tokens.
// @Tags Auth
// @Accept json
// @Produce json
// @Param loginBody body auth.LoginRequest true "User login credentials"
// @Success 200 {object} auth.TokenResponse "Login successful, tokens provided"
// @Failure 400 {object} apperror.ErrorResponse "Bad Request - Invalid input or missing fields"
// @Failure 401 {object} apperror.ErrorResponse "Unauthorized - Invalid credentials"
// @Failure 500 {object} apperror.ErrorResponse "Internal Server Error"
// @Router /auth/login [post]
// `HandleLogin` follows the same pattern as `HandleRegister`.
func (h *Handlers) HandleLogin() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
	// Decode the login request DTO.
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, r, apperror.NewBadRequestError("invalid request body: "+err.Error(), nil))
		return
	}
	defer r.Body.Close()

	// Basic validation.
	if req.Login == "" || req.Password == "" {
		WriteError(w, r, apperror.NewBadRequestError("login and password are required", nil))
		return
	}

	// Call the `Login` method on the `AuthService`.
	resp, err := h.service.Login(r.Context(), req)
	if err != nil {
		WriteError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, resp)
}
}

// HandleRefreshToken godoc
// @Summary Refresh Access Token
// @Description Provides a new access token and refresh token using a valid refresh token.
// @Tags Auth
// @Accept json
// @Produce json
// @Param refreshBody body auth.RefreshTokenRequest true "Refresh token details"
// @Success 200 {object} auth.TokenResponse "Tokens refreshed successfully"
// @Failure 400 {object} apperror.ErrorResponse "Bad Request - Invalid input or missing refresh token"
// @Failure 401 {object} apperror.ErrorResponse "Unauthorized - Invalid or expired refresh token"
// @Failure 500 {object} apperror.ErrorResponse "Internal Server Error"
// @Router /auth/refresh [post]
// `@Security BearerAuth` indicates that this endpoint requires Bearer token authentication,
// though for a refresh token endpoint, the refresh token itself is usually sent in the body,
// and the endpoint might not require an access token in the Authorization header.
// This might be a documentation nuance or specific implementation detail.
// @Security BearerAuth
func (h *Handlers) HandleRefreshToken() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
	// Decode the refresh token request DTO.
	var req RefreshTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, r, apperror.NewBadRequestError("invalid request body: "+err.Error(), nil))
		return
	}
	defer r.Body.Close()
	if req.RefreshToken == "" {
		WriteError(w, r, apperror.NewBadRequestError("refresh_token is required", nil))
		return
	}
	// Call the `RefreshToken` method on the `AuthService`.
	resp, err := h.service.RefreshToken(r.Context(), req.RefreshToken)
	if err != nil {
		WriteError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, resp)
}
}

// Helper functions for writing responses
// These helpers centralize response writing logic.

// `writeJSON` serializes `data` to JSON and writes it to the `http.ResponseWriter` with the given `status`.
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
w.Header().Set("Content-Type", "application/json")
w.WriteHeader(status)
// Check if data is nil to avoid writing "null" as the response body if no data is intended.
if data != nil { // Avoid writing nil, which can result in "null" response body
	if err := json.NewEncoder(w).Encode(data); err != nil {
		// Log this error, as it's a server-side issue if encoding fails
		// For now, we can't do much more in the response itself
		http.Error(w, `{"error":"failed to encode response"}`, http.StatusInternalServerError)
	}
}
}

// WriteError uses the apperror system to write standardized error responses.
// It now accepts the *http.Request to potentially log more context if needed.
// This function converts any error into a standardized `apperror.ErrorResponse`.
func WriteError(w http.ResponseWriter, r *http.Request, err error) {
// `apperror.FromError` attempts to convert the given `err` into an `*apperror.AppError`.
appErr, ok := apperror.FromError(err)
if !ok {
	// If the error is not already an `*apperror.AppError` (e.g., it's a standard Go error),
	// wrap it in a generic `InternalError`. This ensures all errors are handled consistently.
	appErr = apperror.NewInternalError("an unexpected error occurred: " + err.Error(), err)
}

// Log the error internally (especially for 5xx errors)
// Placeholder for more sophisticated logging. In a production app, a structured logger would be used.
// log.Printf("Error processing request %s %s: %v", r.Method, r.URL.Path, appErr.LogError())

// Use `writeJSON` to send the standardized error response.
writeJSON(w, appErr.StatusCode(), appErr.ToResponse())
}
