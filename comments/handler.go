// Package comments is responsible for all functionalities related to comments.
// This includes creating, retrieving, and managing comments and their associated data (likes, reactions, etc.).
// It follows the modular structure seen in other parts of the application (e.g., `auth`, `users`),
// akin to a "CommentsModule" in Nest.js.
package comments

import (
	"encoding/json"
	"net/http"
	// `strings` provides utility functions for string manipulation.
	"strings"

	// `chi` is a lightweight, idiomatic and composable router for building HTTP services in Go.
	// It's used here for routing comment-related API endpoints.
	"github.com/go-chi/chi/v5"
)

// CommentHandler handles HTTP requests for comments.
// This struct acts as a "Controller" in MVC terms, or a Nest.js Controller.
// It receives HTTP requests, delegates business logic to the `CommentService`, and formulates HTTP responses.
// Think of it as the receptionist for all things related to comments.
// When a user wants to do something with a comment (like add one), this is the first stop.
type CommentHandler struct {
	// `service` is a dependency, an instance of `CommentService` containing the business logic.
	// This is manual dependency injection, common in Go.
	service CommentService // This is like the manager who knows how to actually do the comment work.
}

// NewCommentHandler creates a new CommentHandler.
// This is a constructor function, a common Go pattern for creating struct instances and injecting dependencies.
// This is like hiring a new receptionist and telling them who their manager is.
func NewCommentHandler(service CommentService) *CommentHandler {
	return &CommentHandler{service: service}
}

// RegisterRoutes registers the comment API routes with a `chi.Router`.
// This method sets up the specific endpoints (e.g., POST /comments) and maps them to handler methods.
// In Nest.js, this is analogous to defining routes with decorators like `@Post()` within a Controller class.
// This is like telling the building's main directory which office numbers (web paths)
// belong to the comments department and what actions can be done there.
// For example, if you go to "/comments" and send a POST request, you're trying to add a comment.
func (h *CommentHandler) RegisterRoutes(router chi.Router) {
	// We're creating a special section for comments under the main web address.
	// So, if the main address is "example.com/api", comments will be at "example.com/api/comments".
	// Note: Chi's sub-routers are typically mounted directly, not through a .Group() on the sub-router itself.
	// The grouping is done in main.go when calling r.Route("/api/v1/comments", ...)

	// This says: if someone sends a POST request to the base path of this sub-router ("/"),
	// (which would be `/api/v1/comments/` if mounted at `/api/v1/comments`)
	// call the `addComment` function.
	// A POST request is usually used when you want to create something new, like a new comment.
	router.Post("/", h.addComment)
	// ... other comment routes would be registered here ...
	// e.g., router.Get("/thread", h.getThread) // To get all comments in a discussion
	// router.Post("/like", h.toggleLike)    // To like or unlike a comment
}

// addComment handles the HTTP POST request to create a new comment.
// Corresponds to Rust's `add_comment` controller function.
// This function is called when a user tries to post a new comment.
// It's like filling out a form to submit a new comment.
func (h *CommentHandler) addComment(w http.ResponseWriter, r *http.Request) {
	// `w http.ResponseWriter` is used to write the HTTP response.
	// `r *http.Request` contains the incoming HTTP request details.
	// `r` is the incoming request, `w` is what we use to send a response.

	// `var req NewCommentRequest` declares a variable `req` of type `NewCommentRequest` (a DTO).
	var req NewCommentRequest // This is an empty, blank form for a "new comment".

	// We try to take the user's submitted information (from the request body, which is in JSON)
	// and fill our blank `req` form with it.
	// Go's standard `encoding/json` package is used for decoding.
	// It's good practice to limit the size of the request body.
	// r.Body = http.MaxBytesReader(w, r.Body, 1048576) // 1 MB limit example

	// Create a new JSON decoder for the request body.
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields() // Good practice: error if extra fields are sent.
	if err := decoder.Decode(&req); err != nil {
		// If something goes wrong (e.g., the user sent weird data that doesn't fit the form),
		// we tell them it's a "Bad Request" and show them the error.
		http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return // Stop here, don't do anything else.
	}

	// Now we need to know WHO is posting this comment.
	// In Chi, middleware typically puts values into the request's context.
	// Imagine when the user logged in, the security guard (auth middleware) put their User ID
	// into the request's `context.Context`. We're now retrieving it.
	// The key "userID" must match the key used by the authentication middleware.
	userIDVal := r.Context().Value("userID") // Replace "userID" with the actual key used by your auth middleware
	if userIDVal == nil {
		// If there's no userID in the context, it means they're not logged in or auth failed.
		http.Error(w, "User not authenticated", http.StatusUnauthorized)
		return // Stop.
	}

	// The value from context might be in a generic format, so we make sure it's an int32.
	// This is a type assertion in Go. `userIDVal.(int32)` attempts to assert that `userIDVal`
	// is of type `int32`. The `ok` variable will be true if the assertion succeeds.
	userID, ok := userIDVal.(int32)
	if !ok {
		// If the userID in context is weird (not an int32), something is wrong internally.
		http.Error(w, "Invalid user ID format in context", http.StatusInternalServerError)
		return // Stop.
	}

	// Now we have the comment details (`req`) and who wrote it (`userID`).
	// We ask the `service` (the manager) to actually add the comment.
	// This is the call to the business logic layer.
	comment, err := h.service.AddComment(req, userID)
	if err != nil {
		// If the manager (service) had a problem adding the comment...
		// We check if the error message says the comment was "too large".
		if strings.Contains(err.Error(), "exceeds the maximum size") {
			// If so, tell the user their comment is too big.
			http.Error(w, "Comment too large: "+err.Error(), http.StatusBadRequest)
		} else {
			// For any other problem, tell them something went wrong on our end.
			http.Error(w, "Failed to add comment: "+err.Error(), http.StatusInternalServerError)
		}
		return // Stop.
	}

	// If everything went well, the manager (`service`) gives us back the `comment` that was created.
	// We tell the user "Created" (HTTP status 201) and send them their new comment as JSON.
	// `w.Header().Set` sets response headers.
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	// `json.NewEncoder(w).Encode(comment)` serializes the `comment` struct to JSON and writes it to the response.
	json.NewEncoder(w).Encode(comment) // Encode the comment to JSON and write to response.
}

// --- Placeholder for other handlers ---

// Example:
// func (h *CommentHandler) getThread(c *gin.Context) {
// 	var query ThreadQuery
// 	if err := c.ShouldBindQuery(&query); err != nil {
// 		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid query parameters: " + err.Error()})
// 		return
// 	}
//
// 	// userID, _ := auth.GetUserIDFromContext(c) // Optional user ID
//   var currentUserID *int32
//   userIDVal, exists := c.Get("userID")
//   if exists {
//      uid, ok := userIDVal.(int32)
//      if ok {
//          currentUserID = &uid
//      }
//   }
//
// 	response, err := h.service.GetThreadComments(query, currentUserID)
// 	if err != nil {
// 		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get thread", "details": err.Error()})
// 		return
// 	}
// 	c.JSON(http.StatusOK, response)
// }

// Note: The actual implementation of auth.GetUserIDFromContext and apperror.HandleError
// would depend on how authentication and error handling are structured in the target Go project.
// The provided code uses placeholders or basic Gin responses for now.