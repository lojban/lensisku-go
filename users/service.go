// Package users, as part of the user profile management module.
// This file, `service.go`, contains the business logic for user profile operations.
// It acts as the "Service" layer, analogous to a Service class in Nest.js.
package users

import (
	"context"
	// `database/sql` is imported for types like `sql.NullString` to handle nullable database fields.
	"database/sql"
	"errors"
	"fmt"
	"strings"

	// `pgx` specific imports for PostgreSQL interaction.
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	// Internal application packages.
	"github.com/user/lensisku-go/apperror" // For standardized error handling.
	"github.com/user/lensisku-go/auth"     // For the `auth.User` model, reusing it here.
)

// UserService provides methods for user profile management.
// It encapsulates the core logic for fetching and updating user profiles.
type UserService struct {
	// `db` is a pointer to a `pgxpool.Pool`, representing the database connection pool.
	// This dependency is injected via the constructor.
	db *pgxpool.Pool
}

// NewUserService creates a new UserService.
// This is the constructor function for `UserService`.
func NewUserService(db *pgxpool.Pool) *UserService {
	return &UserService{db: db}
}

// GetUserProfile retrieves a user's profile by their ID.
func (s *UserService) GetUserProfile(userID int) (*UserProfileResponse, error) {
	query := `
		SELECT id, username, email, bio, created_at 
		FROM users 
		WHERE id = $1
	`
	// Reusing `auth.User` struct for scanning basic user data. This is acceptable if the fields match.
	// Alternatively, a dedicated `User` struct could be defined within the `users` package.
	var user auth.User // Reusing the auth.User model for scanning
	// `sql.NullString` is used for the `bio` field, as it can be NULL in the database.
	var bio sql.NullString // Handling nullable bio field

	// `s.db.QueryRow` executes the query and scans the result into the provided variables.
	err := s.db.QueryRow(context.Background(), query, userID).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&bio,
		&user.CreatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// If no user is found, return a `NotFoundError` from the `apperror` package.
			return nil, apperror.NewNotFoundError(fmt.Sprintf("user with ID %d not found", userID), nil)
		}
		// For other database errors, return a generic internal error.
		return nil, apperror.NewInternalError("Failed to get user profile", err)
	}

	response := &UserProfileResponse{
		// Map the scanned data to the `UserProfileResponse` DTO.
		ID:        user.ID,
		Username:  user.Username,
		Email:     user.Email,
		CreatedAt: user.CreatedAt,
	}
	if bio.Valid {
		// If `bio` is not NULL, assign its string value to the response.
		response.Bio = &bio.String
	}

	return response, nil
}

// UpdateUserProfile updates a user's profile.
func (s *UserService) UpdateUserProfile(userID int, req *UpdateUserProfileRequest) (*UserProfileResponse, error) {
	// 1. Check if user exists
	// Calling `GetUserProfile` serves as an existence check and reuses logic.
	_, err := s.GetUserProfile(userID) // This also checks for existence
	if err != nil {
		return nil, err // Will be NotFoundError or InternalServerError
	}

	// 2. Construct the UPDATE query dynamically based on provided fields
	var setClauses []string
	// `args` will hold the values for the query's placeholders ($1, $2, etc.).
	var args []interface{}
	argID := 1

	// Check if the email field is provided in the request for update.
	if req.Email != nil && *req.Email != "" {
		setClauses = append(setClauses, fmt.Sprintf("email = $%d", argID))
		args = append(args, *req.Email)
		argID++
	}
	// Check if the bio field is provided.
	if req.Bio != nil { // Allow setting bio to empty string or null
		setClauses = append(setClauses, fmt.Sprintf("bio = $%d", argID))
		// If `req.Bio` is a pointer to an empty string, `*req.Bio` will be `""`.
		// If `req.Bio` itself is `nil`, this block is skipped.
		args = append(args, *req.Bio) // If *req.Bio is "", it's an empty string. If req.Bio is nil, it won't be added.
		argID++
	}

	if len(setClauses) == 0 {
		// No fields to update, just return current profile
		return s.GetUserProfile(userID)
	}

	// Add the userID for the WHERE clause.
	args = append(args, userID) // For the WHERE clause

	query := fmt.Sprintf(`
		UPDATE users 
		SET %s 
		WHERE userid = $%d
		RETURNING userid as id, username, email, bio, created_at
	`, strings.Join(setClauses, ", "), argID)

	// Variables to scan the updated user data into.
	var updatedUser auth.User
	var updatedBio sql.NullString

	// Execute the update query and scan the returned (updated) row.
	err = s.db.QueryRow(context.Background(), query, args...).Scan(
		&updatedUser.ID,
		&updatedUser.Username,
		&updatedUser.Email,
		&updatedBio,
		&updatedUser.CreatedAt,
	)

	if err != nil {
		var pgErr *pgconn.PgError
		// Check for specific PostgreSQL errors, like unique constraint violations.
		if errors.As(err, &pgErr) {
			// Check for unique constraint violation on email (assuming constraint name is users_email_key)
			// You might need to adjust the constraint name based on your actual schema.
			if pgErr.Code == "23505" && strings.Contains(pgErr.ConstraintName, "email") { // 23505 is unique_violation
				return nil, apperror.NewConflictError(fmt.Sprintf("email '%s' already exists", *req.Email), nil)
			}
		}
		return nil, apperror.NewInternalError("Failed to update user profile", err)
	}

	// Construct and return the `UserProfileResponse` DTO.
	response := &UserProfileResponse{
		ID:        updatedUser.ID,
		Username:  updatedUser.Username,
		Email:     updatedUser.Email,
		CreatedAt: updatedUser.CreatedAt,
	}
	if updatedBio.Valid {
		response.Bio = &updatedBio.String
	}

	return response, nil
}

// Helper to get the actual user model if needed internally, not exposed.
// This function might be used by other methods within the `UserService` that need the full user model,
// including potentially sensitive fields like `HashedPassword`.
func (s *UserService) getUserModelByID(userID int) (*auth.User, error) {
	query := `SELECT userid as id, username, email, password as hashed_password, bio, created_at FROM users WHERE id = $1`
	var user auth.User
	var bio sql.NullString
	// Scan all relevant fields, including `HashedPassword`.
	err := s.db.QueryRow(context.Background(), query, userID).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.HashedPassword, // For internal use if needed, e.g. password checks
		&bio,
		&user.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperror.NewNotFoundError(fmt.Sprintf("user with ID %d not found", userID), nil)
		}
		return nil, apperror.NewInternalError("Failed to get user model", err)
	}
	if bio.Valid {
		// If you add Bio to auth.User model, assign it here.
		// For now, UserProfileResponse handles it.
	}
	return &user, nil
}
