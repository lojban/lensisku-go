// Package users encapsulates all functionality related to user profile management.
// This follows a modular design, similar to feature modules in Nest.js.
// This file, `handlers.go`, is responsible for handling HTTP requests related to users.
// It acts as the "Controller" layer in an MVC or similar architectural pattern.
package users

import (
	"encoding/json"
	"net/http"

	// `apperror` provides standardized error types and responses.
	"github.com/user/lensisku-go/apperror"
	// `auth` package provides authentication utilities, like extracting user ID from context.
	"github.com/user/lensisku-go/auth"
)

// UserHandlers provides HTTP handlers for user profile management.
// It holds a reference to the `UserService`, which contains the business logic.
// This is a form of dependency injection, where the service is provided to the handler.
// In Nest.js, this would be analogous to a Controller class injecting a Service class.
type UserHandlers struct {
	// `service` is a pointer to a `UserService` instance. Using a pointer is common for struct fields
	// that represent dependencies or services, allowing shared state or methods to be accessed.
	service *UserService
}

// NewUserHandlers creates new UserHandlers.
func NewUserHandlers(service *UserService) *UserHandlers {
	return &UserHandlers{service: service}
}

// HandleGetUserProfile godoc
// @Summary Get current user's profile
// @Description Retrieves the profile information for the currently authenticated user.
// @Tags users
// @Produce json
// @Security BearerAuth
// @Success 200 {object} UserProfileResponse "Successfully retrieved user profile"
// @Failure 401 {object} apperror.ErrorResponse "Unauthorized - Invalid or missing token"
// @Failure 404 {object} apperror.ErrorResponse "Not Found - User not found"
// @Failure 500 {object} apperror.ErrorResponse "Internal Server Error"
// @Router /users/me [get]
// `HandleGetUserProfile` returns an `http.HandlerFunc`, a standard Go type for HTTP handlers.
// This pattern allows methods to be easily converted to the type expected by HTTP routers like `chi`.
func (h *UserHandlers) HandleGetUserProfile() http.HandlerFunc {
	// The returned function is a closure, capturing the `h *UserHandlers` receiver.
	// This allows the handler function to access `h.service`.
	return func(w http.ResponseWriter, r *http.Request) {
		// `auth.GetUserIDFromContext` retrieves the user ID set by the authentication middleware.
		// `r.Context()` provides access to request-scoped context.
		userID, ok := auth.GetUserIDFromContext(r.Context())
		if !ok {
			// If user ID is not found, it indicates an issue with authentication or middleware setup.
			auth.WriteError(w, r, apperror.NewUnauthorizedError("User ID not found in context, middleware issue?", nil))
			return
		}

		// Call the service layer to fetch the user profile.
		profile, err := h.service.GetUserProfile(userID)
		if err != nil {
			// The service layer is expected to return `apperror` types, which `auth.WriteError` can handle.
			auth.WriteError(w, r, err) // service layer should return apperror types
			return
		}

		// Set response headers and status code, then encode the profile as JSON.
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(profile)
	}
}

// HandleUpdateUserProfile godoc
// @Summary Update current user's profile
// @Description Updates the profile information (e.g., email, bio) for the currently authenticated user.
// @Tags users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param userProfile body UpdateUserProfileRequest true "User profile data to update"
// @Success 200 {object} UserProfileResponse "Successfully updated user profile"
// @Failure 400 {object} apperror.ErrorResponse "Bad Request - Invalid input data"
// @Failure 401 {object} apperror.ErrorResponse "Unauthorized - Invalid or missing token"
// @Failure 404 {object} apperror.ErrorResponse "Not Found - User not found"
// @Failure 409 {object} apperror.ErrorResponse "Conflict - e.g., email already exists"
// @Failure 500 {object} apperror.ErrorResponse "Internal Server Error"
// @Router /users/me [put]
// `HandleUpdateUserProfile` follows the same pattern as `HandleGetUserProfile`.
func (h *UserHandlers) HandleUpdateUserProfile() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Extract user ID from context.
		userID, ok := auth.GetUserIDFromContext(r.Context())
		if !ok {
			auth.WriteError(w, r, apperror.NewUnauthorizedError("User ID not found in context, middleware issue?", nil))
			return
		}

		// Decode the JSON request body into `UpdateUserProfileRequest` DTO.
		var req UpdateUserProfileRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			auth.WriteError(w, r, apperror.NewBadRequestError("Invalid request payload", err))
			return
		}
		// `defer r.Body.Close()` ensures the request body is closed after the function finishes,
		// which is important for resource management.
		defer r.Body.Close()

		// Perform basic validation on the request DTO.
		// Basic validation (more can be added)
		if req.Email == nil && req.Bio == nil {
			auth.WriteError(w, r, apperror.NewBadRequestError("No fields provided for update", nil))
			return
		}
		// Example: Validate email format if provided
		// if req.Email != nil && !isValidEmail(*req.Email) {
		//    apperror.HandleError(w, apperror.NewBadRequestError("Invalid email format", nil))
		//    return
		// }

		// Call the service layer to update the user profile.
		updatedProfile, err := h.service.UpdateUserProfile(userID, &req)
		if err != nil {
			auth.WriteError(w, r, err) // service layer should return apperror types
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(updatedProfile)
	}
}