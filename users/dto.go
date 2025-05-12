// Package users, as part of the user profile management module.
// This file, `dto.go`, defines Data Transfer Objects (DTOs) for the users module.
// DTOs are simple objects used to transfer data between layers, especially between
// handlers (controllers) and services, and for API request/response bodies.
// This is very similar to DTOs in Nest.js, often used with validation decorators.
package users

import "time"

// UserProfileResponse represents the data returned for a user profile.
// @Description User profile information (This is a Swagger annotation)
// Struct tags like `json:"id"` control how the struct fields are serialized to/from JSON.
// @Description User profile information
type UserProfileResponse struct {
	// The ID of the user
	// example: 1
	ID int `json:"id"`
	// The username of the user
	// example: "johndoe"
	Username string `json:"username"`
	// The email address of the user
	// example: "johndoe@example.com"
	Email string `json:"email"`
	// A short biography of the user
	// example: "Lojban enthusiast and software developer."
	// `*string` (pointer to string) allows the `bio` field to be `nil` (null in JSON) if not set.
	Bio *string `json:"bio,omitempty"` // Pointer to allow null/omitted
	// The time the user was created
	// example: "2023-01-15T10:30:00Z"
	CreatedAt time.Time `json:"created_at"`
}

// UpdateUserProfileRequest represents the data for updating a user profile.
// @Description Request body for updating user profile
type UpdateUserProfileRequest struct {
	// The new email address for the user.
	// example: "john.doe.new@example.com"
	// Using pointers (`*string`) allows for partial updates: if a field is `nil`, it means
	// the client doesn't intend to update that field. `omitempty` in the JSON tag
	// means the field will not be included in the JSON output if it's nil (for responses) or empty (for requests, depending on marshaller).
	Email *string `json:"email,omitempty"` // Pointer to allow partial updates
	// The new biography for the user.
	// example: "Updated bio: Still a Lojban enthusiast, now also learning Klingon."
	Bio *string `json:"bio,omitempty"` // Pointer to allow partial updates
}
