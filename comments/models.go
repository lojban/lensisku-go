// Package comments, as part of the comments module.
// This file, `models.go`, defines data structures (structs) that represent
// entities (like `Comment`, `Thread`) and Data Transfer Objects (DTOs like `NewCommentRequest`)
// specific to the comments feature.
// This is analogous to having entities and DTOs within a feature module in Nest.js.
package comments

import (
	// `regexp` for regular expression operations, used here for hashtag extraction.
	"regexp"
	"strings"
	"time"
	// `unicode/utf8` for UTF-8 string manipulation, like counting runes (characters).
	"unicode/utf8"
)

// CommentContent represents a part of a comment's content, supporting different types (e.g., text, image).
// Corresponds to Rust's `CommentContent` in `models.rs` and `ContentPart` in `dto.rs`.
// Think of a comment like a Lego creation. Each `CommentContent` is one Lego brick.
// This struct allows for rich content in comments, not just plain text.
// It has a `Type` (e.g., "text", "image_url", "video_url") and `Data` (the actual text or URL).
type CommentContent struct {
	Type string `json:"type"` // What kind of brick is it? (e.g., "text", "image")
	Data string `json:"data"` // What's on the brick? (e.g., "Hello world!", "http://example.com/cat.jpg")
}

// ReactionResponse represents a summary of a specific reaction type on a comment.
// Corresponds to Rust's `ReactionResponse` in `dto.rs`.
// When you see 👍 (15) on a comment, this struct holds that info.
// Used to show aggregated reaction counts.
type ReactionResponse struct {
	Reaction string `json:"reaction"` // The emoji itself, like "👍" or "😂".
	Count    int64  `json:"count"`    // How many people used this reaction.
	Reacted  bool   `json:"reacted"`  // Did *you* (the person looking) make this reaction? True or false.
}

// Comment represents a comment in a thread.
// Corresponds to Rust's `Comment` in `models.rs`.
// This is the main blueprint for what a comment looks like and all the information it holds.
// This struct is a core entity for the comments feature.
// It's like a profile card for a single comment.
type Comment struct {
	// --- What is this comment about? ---
	// Pointer types (`*int32`) are used for fields that can be nullable in the database
	// or optional in JSON. `omitempty` in the JSON tag means the field will be omitted
	// from the JSON output if its value is the zero value for its type (e.g., nil for pointers).
	ValsiID              *int32           `json:"valsi_id,omitempty"`      // If about a Lojban word, its ID. `*int32` means it might be missing (nil).
	DefinitionID         *int32           `json:"definition_id,omitempty"` // If about a specific definition, its ID.
	
	// --- Basic Comment Info ---
	CommentID            int32            `json:"comment_id"`    // The unique ID number for *this* comment.
	ThreadID             int32            `json:"thread_id"`     // Which conversation (thread) does this comment belong to?
	ParentID             *int32           `json:"parent_id,omitempty"` // If this is a reply, what's the ID of the comment it's replying to?
	UserID               int32            `json:"user_id"`       // Who wrote this comment? (Their ID number).
	CommentNum           int32            `json:"comment_num"`   // In its thread/reply chain, is this the 1st, 2nd, 3rd comment?
	Time                 int32            `json:"time"`          // When was it posted? (Unix timestamp: seconds since a long time ago).
	Subject              string           `json:"subject"`       // The title or subject line of the comment.
	Content              []CommentContent `json:"content"`       // The actual stuff in the comment (text, images), made of `CommentContent` bricks.
	
	// --- Author Info ---
	Username             *string          `json:"username,omitempty"` // The author's display name.
	Realname             *string          `json:"realname,omitempty"` // The author's real name (if they provided it).

	// --- Thread Context (often for displaying lists of threads) ---
	LastCommentUsername  *string          `json:"last_comment_username,omitempty"` // In a list of threads, who made the *latest* reply in this one?
	FirstCommentSubject  *string          `json:"first_comment_subject,omitempty"`  // In a list of threads, what was the subject of the *first* comment?
	FirstCommentContent  []CommentContent `json:"first_comment_content,omitempty"` // And what was its content?
	ValsiWord            *string          `json:"valsi_word,omitempty"`            // If thread is about a Lojban word, what's the word? (e.g., "broda")
	Definition           *string          `json:"definition,omitempty"`            // If thread is about a definition, what's its text?

	// --- Stats & User Interactions ---
	TotalReactions       int64            `json:"total_reactions"` // How many reactions (likes, hearts, etc.) in total?
	TotalReplies         int64            `json:"total_replies"`   // How many direct replies does this comment have?
	IsLiked              *bool            `json:"is_liked,omitempty"`    // Did *you* (the current viewer) "like" this specific comment?
	IsBookmarked         *bool            `json:"is_bookmarked,omitempty"` // Did *you* bookmark it?
	Reactions            []ReactionResponse `json:"reactions,omitempty"` // A list of all reaction types and their counts (e.g., 👍:15, ❤️:3).
	
	// --- Reply Context ---
	ParentContent        []CommentContent `json:"parent_content,omitempty"` // If this is a reply, what was the content of the comment it replied to?
}

// This is a pre-built tool (a "regular expression") that's good at finding hashtags like #example or #Lojban.
// It looks for a '#' followed by one or more word characters (letters, numbers, underscore).
// `regexp.MustCompile` compiles the regular expression. If the expression is invalid, it panics,
// which is acceptable for global regex variables defined at package level, as it indicates a programmer error.
var hashtagRegex = regexp.MustCompile(`#(\w+)`)

// ExtractHashtags extracts unique hashtags from a given text content.
// Imagine you give this function a sentence: "I love #Lojban and #coding. #Lojban is fun!"
// It will find all the #hashtags.
// This is a utility function specific to comment processing.
func ExtractHashtags(content string) map[string]struct{} {
	// `FindAllStringSubmatch` uses our `hashtagRegex` tool to find all occurrences.
	// For "#Lojban", `match` would be `["#Lojban", "Lojban"]`. We want the part without the '#'.
	matches := hashtagRegex.FindAllStringSubmatch(content, -1) // -1 means find all.
	
	// We use a `map[string]struct{}` to store the hashtags. This is a clever way
	// to get a list of *unique* items because map keys must be unique.
	// `struct{}` is an empty struct, which takes up zero memory, making it an efficient choice for set values.
	// The `struct{}` part means we don't care about the value, only the key (the hashtag itself).
	hashtags := make(map[string]struct{})
	
	for _, match := range matches { // Go through each found hashtag.
		if len(match) > 1 { // Make sure we actually got the part after '#' (match[1]).
			// Convert to lowercase so #Lojban and #lojban are treated as the same.
			hashtags[strings.ToLower(match[1])] = struct{}{} // Add it to our set of unique hashtags.
		}
	}
	// The function returns the set of unique, lowercase hashtags.
	// For our example, it would return {"lojban", "coding"}.
	return hashtags
}

// Thread represents a comment thread, typically associated with a valsi, definition, or natlang word.
// Corresponds to Rust's `Thread` in `models.rs`.
// This entity defines a conversation thread to which comments belong.
type Thread struct {
	ThreadID      int32   `json:"thread_id"`
	ValsiID       *int32  `json:"valsi_id,omitempty"`
	NatlangWordID *int32  `json:"natlang_word_id,omitempty"`
	DefinitionID  *int32  `json:"definition_id,omitempty"`
	Valsi         *string `json:"valsi,omitempty"`
	NatlangWord   *string `json:"natlang_word,omitempty"`
	Tag           *string `json:"tag,omitempty"` // Could be used for categorization
}

// CommentLike represents a "like" action on a comment by a user.
// Corresponds to Rust's `CommentLike` in `models.rs`.
// This entity maps to a `comment_likes` table, recording individual like actions.
type CommentLike struct {
	UserID    int32     `json:"user_id"`
	CommentID int32     `json:"comment_id"`
	CreatedAt time.Time `json:"created_at"`
}

// CommentOpinion represents an opinion expressed on a comment.
// Corresponds to Rust's `CommentOpinion` in `models.rs`.
// Represents a structured opinion or poll-like feature on comments.
type CommentOpinion struct {
	ID        int64     `json:"id"`
	Opinion   string    `json:"opinion"`
	CommentID int32     `json:"comment_id"`
	UserID    int32     `json:"user_id"`
	Votes     int32     `json:"votes"`
	Voted     bool      `json:"voted"` // If the current user voted for this opinion
	CreatedAt time.Time `json:"created_at"`
}

// `opinionMaxLen` defines a constraint on opinion length.
const opinionMaxLen = 12 // Max grapheme clusters

// ParseOpinion validates and normalizes an opinion string.
// This function encapsulates validation logic for opinions.
func ParseOpinion(content string) *string {
	lowerContent := strings.ToLower(content)
	if lowerContent == "" || utf8.RuneCountInString(lowerContent) > opinionMaxLen {
		return nil
	}
	return &lowerContent
}

// CommentWithOpinions combines a comment with its associated opinions.
// Corresponds to Rust's `CommentWithOpinions` in `models.rs`.
// This is a DTO used for API responses that need to include both comment and its opinions.
type CommentWithOpinions struct {
	Comment  Comment          `json:"comment"`
	Opinions []CommentOpinion `json:"opinions"`
}

// CommentReaction represents a specific reaction instance by a user on a comment.
// Corresponds to Rust's `CommentReaction` in `models.rs`.
// This entity maps to a `comment_reactions` table, recording individual reaction instances.
type CommentReaction struct {
	ID        int32     `json:"id"`
	CommentID int32     `json:"comment_id"`
	UserID    int32     `json:"user_id"`
	Reaction  string    `json:"reaction"`
	CreatedAt time.Time `json:"created_at"`
}

// TrendingTimespan defines periods for trending calculations.
// Corresponds to Rust's `TrendingTimespan` enum in `models.rs`.
// Using a custom string type (`TrendingTimespan`) with constants provides type safety
// and readability for defining time spans.
type TrendingTimespan string

const (
	LastDay   TrendingTimespan = "LastDay"
	LastWeek  TrendingTimespan = "LastWeek"
	LastMonth TrendingTimespan = "LastMonth"
	LastYear  TrendingTimespan = "LastYear"
	AllTime   TrendingTimespan = "AllTime"
)

// FreeThread represents a comment thread in a list view, often for "free discussions" not tied to specific items.
// Corresponds to Rust's `FreeThread` in `models.rs`.
// This struct seems tailored for displaying a list of threads, possibly with summary information.
type FreeThread struct {
	ThreadID            int32            `json:"thread_id"`
	ValsiID             *int32           `json:"valsi_id,omitempty"` // Renamed from valsiid for consistency
	DefinitionID        *int32           `json:"definition_id,omitempty"` // Renamed from definitionid
	ValsiWord           *string          `json:"valsi_word,omitempty"`
	Definition          *string          `json:"definition,omitempty"`
	LastCommentID       int32            `json:"last_comment_id"`
	LastCommentTime     int32            `json:"last_comment_time"` // Unix timestamp
	LastCommentSubject  string           `json:"last_comment_subject"`
	LastCommentContent  []CommentContent `json:"last_comment_content"`
	FirstCommentSubject string           `json:"first_comment_subject"`
	FirstCommentContent []CommentContent `json:"first_comment_content"`
	TotalComments       int64            `json:"total_comments"`
	LastCommentUsername *string          `json:"last_comment_username,omitempty"`
	Username            string           `json:"username"` // Username of the original poster of the thread's first comment
	Realname            *string          `json:"realname,omitempty"` // Real name of the OP
	IsLiked             *bool            `json:"is_liked,omitempty"`    // If current user liked the first comment of this thread
	IsBookmarked        *bool            `json:"is_bookmarked,omitempty"` // If current user bookmarked the first comment
	UserID              int32            `json:"user_id"` // User ID of the OP
	CommentNum          int32            `json:"comment_num"` // Comment number of the first comment
	ParentID            *int32           `json:"parent_id,omitempty"` // Parent ID of the first comment (should be null for thread starters)
	TotalReactions      int64            `json:"total_reactions"` // Total reactions on the first comment
	Reactions           []ReactionResponse `json:"reactions,omitempty"` // Reactions on the first comment
}

// --- DTOs from dto.rs ---

// NewCommentRequest is used to create a new comment.
// Corresponds to Rust's `NewCommentRequest` in `dto.rs`.
// This DTO defines the expected structure of a request to create a new comment.
type NewCommentRequest struct {
	ValsiID       *int32           `json:"valsi_id,omitempty"`
	NatlangWordID *int32           `json:"natlang_word_id,omitempty"`
	DefinitionID  *int32           `json:"definition_id,omitempty"`
	ParentID      *int32           `json:"parent_id,omitempty"` // nil or 0 for top-level comments
	Subject       string           `json:"subject"`
	Content       []CommentContent `json:"content"`
}

// CommentActionRequest is used for liking/unliking or bookmarking/unbookmarking a comment.
// Corresponds to Rust's `CommentActionRequest` in `dto.rs`.
// A DTO for actions like liking or bookmarking.
type CommentActionRequest struct {
	CommentID int32 `json:"comment_id"`
	Action    bool  `json:"action"` // true to like/bookmark, false to unlike/unbookmark
}

// CommentResponse wraps a Comment model with additional stats for API responses.
// Corresponds to Rust's `CommentResponse` in `dto.rs`.
type CommentResponse struct {
	// Embedding the `Comment` struct provides all its fields.
	Comment      Comment `json:"comment"`
	Likes        int64   `json:"likes"` // Note: Rust version has total_reactions in Comment model, this might be redundant or specific to "like" type. Assuming this is total likes.
	Replies      int64   `json:"replies"`
	IsLiked      bool    `json:"is_liked"`
	IsBookmarked bool    `json:"is_bookmarked"`
}

// ThreadResponse is an API response for a thread, containing comments and total count.
// Corresponds to Rust's `ThreadResponse` in `dto.rs`.
type ThreadResponse struct {
	// A slice of `CommentResponse` for the comments in the thread.
	Comments []CommentResponse `json:"comments"`
	Total    int64             `json:"total"`
}

// CreateOpinionRequest is used to create a new opinion on a comment.
// Corresponds to Rust's `CreateOpinionRequest` in `dto.rs`.
type CreateOpinionRequest struct {
	// Fields required to create an opinion.
	CommentID int32  `json:"comment_id"`
	Opinion   string `json:"opinion"`
}

// OpinionVoteRequest is used to vote on an existing opinion.
// Corresponds to Rust's `OpinionVoteRequest` in `dto.rs`.
type OpinionVoteRequest struct {
	// Fields required to cast a vote on an opinion.
	OpinionID int64 `json:"opinion_id"`
	CommentID int32 `json:"comment_id"` // Often included for context or validation
	Vote      bool  `json:"vote"`       // true for upvote, false for downvote/remove vote
}

// CommentStats provides various statistics for a comment.
// Corresponds to Rust's `CommentStats` in `dto.rs`.
type CommentStats struct {
	// Aggregated statistics for a comment.
	TotalLikes        int64     `json:"total_likes"`
	TotalBookmarks    int64     `json:"total_bookmarks"`
	TotalReplies      int64     `json:"total_replies"`
	TotalOpinions     int64     `json:"total_opinions"`
	TotalReactions    int64     `json:"total_reactions"`
	LastActivityAt    time.Time `json:"last_activity_at"`
}

// TrendingHashtag represents a hashtag and its usage statistics.
// Corresponds to Rust's `TrendingHashtag` in `dto.rs`.
type TrendingHashtag struct {
	// Information about a trending hashtag.
	Tag         string    `json:"tag"`
	UsageCount  int64     `json:"usage_count"`
	LastUsed    time.Time `json:"last_used"`
}

// ReactionRequest is used to add or remove a reaction to/from a comment.
// Corresponds to Rust's `ReactionRequest` in `dto.rs`.
type ReactionRequest struct {
	// DTO for adding or removing a reaction.
	CommentID int32  `json:"comment_id"`
	Reaction  string `json:"reaction"`
}

// PaginatedReactions is a response structure for paginated reaction lists.
// Corresponds to Rust's `PaginatedReactions` in `dto.rs`.
type PaginatedReactions struct {
	// Structure for returning a paginated list of reactions.
	Reactions      []ReactionResponse `json:"reactions"`
	TotalReactions int64              `json:"total_reactions"`
	TotalPages     int64              `json:"total_pages"`
	CurrentPage    int64              `json:"current_page"`
	PageSize       int32              `json:"page_size"`
}

// ReactionSummary provides a summary of reactions, including paginated results.
// Corresponds to Rust's `ReactionSummary` in `dto.rs`.
type ReactionSummary struct {
	// A more comprehensive summary of reactions, including pagination details.
	Reactions              PaginatedReactions `json:"reactions"`
	TotalDistinctReactions int64              `json:"total_distinct_reactions"`
}

// ReactionPaginationQuery defines query parameters for paginating reactions.
// Corresponds to Rust's `ReactionPaginationQuery` in `dto.rs`.
type ReactionPaginationQuery struct {
	// Query parameters for requesting paginated reactions.
	// `form:"page"` tags are often used by libraries like Gorilla/Schema to decode form data into structs.
	Page     *int64 `json:"page,omitempty" form:"page"`         // Default 1
	PageSize *int32 `json:"page_size,omitempty" form:"page_size"` // Default 10
}

// PaginatedCommentsResponse is a generic response for paginated comments.
// Corresponds to Rust's `PaginatedCommentsResponse` in `dto.rs`.
type PaginatedCommentsResponse struct {
	// Standard structure for returning a paginated list of comments.
	Comments []Comment `json:"comments"`
	Total    int64     `json:"total"`
	Page     int64     `json:"page"`
	PerPage  int64     `json:"per_page"`
}

// PaginatedUserCommentsResponse is for paginated comments by a specific user.
// Corresponds to Rust's `PaginatedUserCommentsResponse` in `dto.rs`.
// Note: Structure is identical to PaginatedCommentsResponse, could be aliased or kept separate for semantic distinction.
type PaginatedUserCommentsResponse struct {
	// Specific DTO for paginated comments by a user, though structurally same as `PaginatedCommentsResponse`.
	Comments []Comment `json:"comments"`
	Total    int64     `json:"total"`
	Page     int64     `json:"page"`
	PerPage  int64     `json:"per_page"`
}

// FreeThreadQuery defines query parameters for listing free threads.
// Corresponds to Rust's `FreeThreadQuery` in `dto.rs`.
type FreeThreadQuery struct {
	// Query parameters for fetching a list of "free threads".
	Page       *int64  `json:"page,omitempty" form:"page"`             // Default 1
	PerPage    *int64  `json:"per_page,omitempty" form:"per_page"`       // Default 20
	SortBy     *string `json:"sort_by,omitempty" form:"sort_by"`       // Default "time", example "subject"
	SortOrder  *string `json:"sort_order,omitempty" form:"sort_order"`   // Default "desc", example "asc"
}

// ThreadQuery defines query parameters for fetching a specific thread's comments.
// Corresponds to Rust's `ThreadQuery` in `dto.rs`.
type ThreadQuery struct {
	// Query parameters for fetching comments within a specific thread.
	ValsiID       *int32 `json:"valsi_id,omitempty" form:"valsi_id"`
	NatlangWordID *int32 `json:"natlang_word_id,omitempty" form:"natlang_word_id"`
	DefinitionID  *int32 `json:"definition_id,omitempty" form:"definition_id"`
	CommentID     *int32 `json:"comment_id,omitempty" form:"comment_id"` // To find thread by a comment within it
	ScrollTo      *int32 `json:"scroll_to,omitempty" form:"scroll_to"`   // Comment ID to scroll to in the view
	ThreadID      *int32 `json:"thread_id,omitempty" form:"thread_id"`
	Page          *int64 `json:"page,omitempty" form:"page"`             // Default 1
	PerPage       *int64 `json:"per_page,omitempty" form:"per_page"`       // Default 20
}

// TrendingQuery defines parameters for fetching trending items (e.g., hashtags).
// Corresponds to Rust's `TrendingQuery` in `dto.rs`.
type TrendingQuery struct {
	// Query parameters for fetching trending items.
	Timespan *string `json:"timespan,omitempty" form:"timespan"` // Default "LastWeek", example "week"
	Limit    *int32  `json:"limit,omitempty" form:"limit"`       // Default 10
}

// PaginationQuery is a generic set of pagination parameters.
// Corresponds to Rust's `PaginationQuery` in `dto.rs`.
type PaginationQuery struct {
	// Generic pagination query parameters, reusable across different listing endpoints.
	Page    *int64 `json:"page,omitempty" form:"page"`       // Default 1
	PerPage *int64 `json:"per_page,omitempty" form:"per_page"` // Default 20
}

// SearchCommentsQuery defines parameters for searching comments.
// Corresponds to Rust's `SearchCommentsQuery` in `dto.rs`.
type SearchCommentsQuery struct {
	// Query parameters for searching comments with various filters and sorting options.
	Page         *int64  `json:"page,omitempty" form:"page"`                 // Default 1
	PerPage      *int64  `json:"per_page,omitempty" form:"per_page"`           // Default 20
	Search       *string `json:"search,omitempty" form:"search"`
	SortBy       *string `json:"sort_by,omitempty" form:"sort_by"`           // Default "time"
	SortOrder    *string `json:"sort_order,omitempty" form:"sort_order"`       // Default "desc"
	Username     *string `json:"username,omitempty" form:"username"`
	ValsiID      *int32  `json:"valsi_id,omitempty" form:"valsi_id"`
	DefinitionID *int32  `json:"definition_id,omitempty" form:"definition_id"`
}

// ListCommentsQuery defines parameters for listing comments (e.g., all comments by a user).
// Corresponds to Rust's `ListCommentsQuery` in `dto.rs`.
type ListCommentsQuery struct {
	// Query parameters for listing comments, typically with pagination and sorting.
	Page      *int64  `json:"page,omitempty" form:"page"`             // Default 1
	PerPage   *int64  `json:"per_page,omitempty" form:"per_page"`       // Default 20
	SortOrder *string `json:"sort_order,omitempty" form:"sort_order"`   // Default "desc", example "asc"
}

// Note: Rust's `NewCommentParams`, `SearchCommentsParams`, `ThreadParams` are internal service parameters
// often including a database pool. These will be defined in the Go service layer as needed,
// rather than as general DTOs in this file.
