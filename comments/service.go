// Package comments, as part of the comments module.
// This file, `service.go`, contains the business logic for comment-related operations.
// It acts as the "Service" layer in an MVC or similar architectural pattern,
// analogous to a Service class in Nest.js.
package comments

import (
	// `context` is used for managing request lifecycles, cancellation, and deadlines.
	"context"
	// `database/sql` provides generic SQL database access, used here for `sql.NullString` etc.
	"database/sql"
	"encoding/json"
	"fmt"
	"log" // todo: for basic logging, replace with a proper logger
	"os"
	// `strings` for string manipulation.
	"strings"
	"time"

	// `pgx` specific imports for PostgreSQL interaction.
	// `pgx.ErrNoRows` is a specific error for when a query returns no rows.
	"github.com/jackc/pgx/v5" // for pgx.ErrNoRows
	"github.com/jackc/pgx/v5/pgconn" // for pgconn.CommandTag
	"github.com/jackc/pgx/v5/pgxpool"
)

// CommentService defines the interface for comment-related operations.
// This is like a list of all the jobs the "comments manager" can do.
// Defining an interface for the service promotes loose coupling and testability.
// Handlers can depend on this interface rather than the concrete implementation.
// For example, "AddComment", "GetThreadComments", "ToggleLike", etc.
type CommentService interface {
	AddComment(params NewCommentRequest, userID int32) (*Comment, error)
	GetThreadComments(params ThreadQuery, currentUserID *int32) (*PaginatedCommentsResponse, error)
	ToggleLike(commentID int32, userID int32, like bool) error
	ToggleBookmark(commentID int32, userID int32, bookmark bool) error
	GetBookmarkedComments(userID int32, page int64, perPage int64, currentUserID *int32) (*PaginatedCommentsResponse, error)
	GetLikedComments(userID int32, page int64, perPage int64, currentUserID *int32) (*PaginatedCommentsResponse, error)
	GetUserComments(userID int32, page int64, perPage int64, currentUserID *int32) (*PaginatedCommentsResponse, error)
	CreateOpinion(userID int32, req CreateOpinionRequest) (*CommentOpinion, error)
	SetOpinionVote(userID int32, req OpinionVoteRequest) error
	GetCommentOpinions(commentID int32, userID *int32) ([]CommentOpinion, error)
	GetTrendingComments(timespan TrendingTimespan, currentUserID *int32, limit int32) ([]Comment, error)
	GetCommentStats(commentID int32) (*CommentStats, error)
	GetMostBookmarkedComments(page int64, perPage int64, currentUserID *int32) (*PaginatedCommentsResponse, error)
	GetTrendingHashtags(timespan TrendingTimespan, limit int32) ([]TrendingHashtag, error)
	GetCommentsByHashtag(tag string, userID *int32, page *int64, perPage *int64) (*PaginatedCommentsResponse, error)
	DeleteComment(commentID int32, userID int32) error
	ToggleReaction(commentID int32, userID int32, reaction string) (bool, error)
	SearchComments(params SearchCommentsQuery, currentUserID *int32) (*PaginatedCommentsResponse, error)
	GetMyReactions(userID int32, page int64, perPage int64) (*PaginatedCommentsResponse, error)
	GetReactions(commentID int32, currentUserID *int32, page *int64, pageSize *int32) (*ReactionSummary, error)
	ListThreads(page int64, perPage int64, sortBy string, sortOrder string) (*PaginatedCommentsResponse, error)
	ListComments(page int64, perPage int64, sortOrder string, currentUserID *int32) (*PaginatedCommentsResponse, error)
	GetLikeCount(commentID int32) (int64, error)
	// Internal helper, might not be exposed directly in the interface if only used internally
	// getCommentByID(tx pgx.Tx, commentID int32, userID *int32) (*Comment, error)
}

// commentServiceImpl is an implementation of CommentService.
// This is the actual "comments manager" who knows how to do all the jobs listed above.
// The `Impl` suffix is a common convention in Go for concrete implementations of interfaces.
// It needs a `db` (database connection) to store and retrieve comment information.
type commentServiceImpl struct {
	// `db` is a pointer to a `pgxpool.Pool`, representing the database connection pool.
	// This is a dependency injected via the constructor.
	db *pgxpool.Pool // This is like the filing cabinet where all comment data is stored.
}

// NewCommentService creates a new CommentService.
// This is the constructor function for `commentServiceImpl`.
// This is like hiring a new "comments manager" and giving them access to the filing cabinet (database).
func NewCommentService(db *pgxpool.Pool) CommentService {
	return &commentServiceImpl{db: db}
}

// This is a rule: comments can't be bigger than 5 Megabytes.
// Like saying a letter can't be heavier than a certain amount.
const maxCommentSize = 5 * 1024 * 1024 // 5MB limit

// AddComment creates a new comment.
// Corresponds to Rust's `add_comment` function.
// This is the detailed instruction manual for the "AddComment" job.
func (s *commentServiceImpl) AddComment(params NewCommentRequest, userID int32) (*Comment, error) {
	// Imagine we're doing several steps to add a comment, like writing on a form,
	// then putting it in an envelope, then mailing it.
	// A "transaction" (`tx`) means all these steps must succeed. If any step fails,
	// it's like we crumple up the form and throw it away – nothing gets saved (rolled back).
	// Database transactions ensure atomicity.
	ctx := context.Background()
	// `s.db.Begin(ctx)` starts a new database transaction.
	tx, err := s.db.Begin(ctx) // Start of the "all or nothing" process.
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err) // Problem starting? Can't do anything.
	}

	// The `defer` statement schedules a function call to be run when the surrounding function (`AddComment`) returns.
	// This is commonly used for cleanup tasks like closing resources or, in this case, handling transaction commit/rollback.
	// This `defer` block is like a special instruction: "No matter what happens in this function
	// (even if it crashes or finishes normally), do this cleanup stuff at the very end."
	defer func() {
		// `recover()` is used to regain control of a panicking goroutine.
		// If a panic occurs within `AddComment`, this `defer` function will execute, and `recover()` will catch the panic.
		if p := recover(); p != nil { // `recover` is for catching unexpected crashes (panics).
			_ = tx.Rollback(ctx) // If we crashed, undo everything (crumple the form).
			panic(p)          // Then, let the crash continue so we know something went very wrong.
		} else if err != nil { // If `err` is not nil, it means a known error happened.
			// `tx.Rollback(ctx)` discards all changes made during the transaction.
			_ = tx.Rollback(ctx) // A known error also means we undo everything.
		} else { // If no crash and no known error...
			err = tx.Commit(ctx) // ...then we try to save everything permanently (mail the envelope).
			// If even saving fails, `err` will catch that problem.
		}
	}() // This cleanup runs when we exit AddComment.

	var threadID int32 // A "thread" is like a conversation topic. We need to find or create one.

	// Scenario 1: Is this comment a reply to another comment?
	if params.ParentID != nil && *params.ParentID > 0 {
		// If yes, find the conversation topic (threadID) of the comment it's replying to.
		// `tx.QueryRow` executes a query expected to return at most one row.
		err = tx.QueryRow(ctx, "SELECT threadid FROM comments WHERE commentid = $1", params.ParentID).Scan(&threadID)
		if err != nil {
			// Couldn't find the parent comment's conversation? That's a problem.
			return nil, fmt.Errorf("failed to get thread ID from parent comment: %w", err)
		}
	// Scenario 2: Is this a brand new comment, not tied to any specific Lojban word, definition, etc.?
	// (i.e., a "free-standing" comment starting its own new topic)
	} else if (params.ValsiID == nil || *params.ValsiID == 0) &&
		(params.NatlangWordID == nil || *params.NatlangWordID == 0) &&
		(params.DefinitionID == nil || *params.DefinitionID == 0) {
		// If yes, create a brand new, generic conversation topic.
		err = tx.QueryRow(ctx, `
			INSERT INTO threads (valsiid, natlangwordid, definitionid)
			VALUES (0, 0, 0) /* 0 means not specific to any item */
			RETURNING threadid`).Scan(&threadID) // Get the ID of the new topic.
		if err != nil {
			return nil, fmt.Errorf("failed to create new free thread: %w", err)
		}
	// Scenario 3: This comment is about a specific Lojban word (Valsi), or a definition, etc.
	} else {
		// We need to find if there's already a conversation topic for this specific item.
		var valsiIDParam, natlangWordIDParam, definitionIDParam sql.NullInt32
		// `sql.NullInt32` (and similar types like `sql.NullString`) are used to handle nullable database columns.
		// They have a `Valid` boolean field indicating if the value is non-null.
		// These `sql.NullInt32` are special because the item IDs might be missing (nil).
		// If an ID is missing, we treat it as 0 for finding the thread.
		if params.ValsiID != nil {
			valsiIDParam = sql.NullInt32{Int32: *params.ValsiID, Valid: true}
		} else {
	           valsiIDParam = sql.NullInt32{Int32: 0, Valid: true} // Match 0 if NULL
	       }
		if params.NatlangWordID != nil {
			natlangWordIDParam = sql.NullInt32{Int32: *params.NatlangWordID, Valid: true}
		} else {
	           natlangWordIDParam = sql.NullInt32{Int32: 0, Valid: true} // Match 0 if NULL
	       }
		if params.DefinitionID != nil {
			definitionIDParam = sql.NullInt32{Int32: *params.DefinitionID, Valid: true}
		}

		// Try to find an existing conversation topic that matches all the provided IDs (or 0 if an ID is missing).
		err = tx.QueryRow(ctx, `
			SELECT threadid FROM threads
			WHERE (valsiid = $1 OR ($1 IS NULL AND valsiid = 0))
			AND (natlangwordid = $2 OR ($2 IS NULL AND natlangwordid = 0))
			AND (definitionid = $3 OR $3 IS NULL)`,
			valsiIDParam, natlangWordIDParam, definitionIDParam).Scan(&threadID)

		// `pgx.ErrNoRows` (or `sql.ErrNoRows` with `database/sql`) indicates that the query returned no results.
		if err == pgx.ErrNoRows { // `pgx.ErrNoRows` means no existing topic was found.
			// So, we create a new conversation topic for this specific item.
			var vID, nID, dID int32 // Get the actual IDs, or 0 if they were missing.
			if params.ValsiID != nil { vID = *params.ValsiID }
			if params.NatlangWordID != nil { nID = *params.NatlangWordID }
			if params.DefinitionID != nil { dID = *params.DefinitionID }

			err = tx.QueryRow(ctx, `
				INSERT INTO threads (valsiid, natlangwordid, definitionid)
				VALUES ($1, $2, $3)
				RETURNING threadid`, // Get the ID of this new topic.
				vID, nID, dID).Scan(&threadID)
			if err != nil {
				return nil, fmt.Errorf("failed to create new related thread: %w", err)
			}
		} else if err != nil { // Some other error happened while searching.
			return nil, fmt.Errorf("failed to find existing thread: %w", err)
		}
	} // Now we definitely have a `threadID` for our comment.

	// Each comment in a thread gets a number (1st comment, 2nd, etc.).
	// We find the biggest number so far in this thread and add 1.
	var commentNum int32
	err = tx.QueryRow(ctx, `
		SELECT COALESCE(MAX(commentnum), 0) + 1 as next_num
		FROM comments
		WHERE threadid = $1`, threadID).Scan(&commentNum)
	if err != nil {
		return nil, fmt.Errorf("failed to get next comment number: %w", err)
	}

	// The comment's content can be made of several parts (text, images, etc.).
	contentParts := params.Content
	// This loop cleans up trailing empty text parts from the comment content.
	// Clean up: if the user added empty text boxes at the end, remove them.
	for len(contentParts) > 0 {
		last := contentParts[len(contentParts)-1]
		if last.Type == "text" && last.Data == "" { // If last part is empty text...
			contentParts = contentParts[:len(contentParts)-1] // ...chop it off.
		} else {
			break // Otherwise, we're done cleaning.
		}
	}

	// Check if the comment is too big (remember the 5MB rule).
	var totalSize int
	for _, p := range contentParts { // Add up the size of all content parts.
		totalSize += len(p.Data)
	}
	if totalSize > maxCommentSize {
		return nil, fmt.Errorf("comment content exceeds the maximum size of %dMB", maxCommentSize/(1024*1024))
	}

	// If the user gave a "Subject" for the comment, add it as a special "header" part at the beginning.
	if params.Subject != "" {
		contentParts = append([]CommentContent{{Type: "header", Data: params.Subject}}, contentParts...)
	}

	// Computers store complex things like `contentParts` in a special text format called JSON.
	// We convert our `contentParts` into this JSON text.
	// `json.Marshal` serializes a Go data structure into a JSON byte slice.
	contentJSON, jsonErr := json.Marshal(contentParts)
	if jsonErr != nil {
		return nil, fmt.Errorf("failed to marshal content to JSON: %w", jsonErr)
	}

	// Now, we're ready to save the main comment information into the database!
	// Now, we're ready to save the main comment information into the database!
	// The `RETURNING commentid` clause in SQL allows us to get the ID of the newly inserted row.
	// `Scan` is used to read this returned ID into the `commentID` variable.
	var commentID int32 // This will be the unique ID for our new comment.
	err = tx.QueryRow(ctx, `
		INSERT INTO comments (threadid, parentid, userid, commentnum, time, subject, content)
		VALUES ($1, $2, $3, $4, $5, $6, $7) /* $1, $2... are placeholders for our values */
		RETURNING commentid`, // Tell the database to give us back the ID of the new comment.
		threadID, params.ParentID, userID, commentNum, time.Now().Unix(), params.Subject, contentJSON).Scan(&commentID)
	if err != nil {
		return nil, fmt.Errorf("failed to insert comment: %w", err)
	} // Our comment is now in the `comments` table!

	// --- Hashtags ---
	// If the comment has #hashtags, we need to find them and save them.
	// `strings.Builder` is an efficient way to build strings incrementally.
	var allTextContent strings.Builder // We'll put all text parts of the comment together here.
	for _, part := range params.Content { // Look at the original content parts from the user.
		if part.Type == "text" { // If it's a text part...
			allTextContent.WriteString(part.Data) // ...add its text.
			allTextContent.WriteString(" ")       // Add a space, just in case.
		}
	}
	// `ExtractHashtags` is a helper function (defined in `models.go`) to parse hashtags from text.
	hashtags := ExtractHashtags(allTextContent.String()) // A helper function finds all #words.

	for tag := range hashtags { // For each #hashtag found...
		var hashtagID int32
		// Try to add it to our list of all known hashtags.
		// If it's already there (`ON CONFLICT`), just make sure it's up-to-date.
		// Then get its unique ID.
		// `ON CONFLICT (tag) DO UPDATE SET tag = EXCLUDED.tag` is an "upsert" operation in PostgreSQL.
		err = tx.QueryRow(ctx, `
			INSERT INTO hashtags (tag)
			VALUES ($1)
			ON CONFLICT (tag) DO UPDATE
			SET tag = EXCLUDED.tag /* This ensures the casing or something could be updated if needed */
			RETURNING id`, tag).Scan(&hashtagID)
		if err != nil {
			return nil, fmt.Errorf("failed to insert/get hashtag ID for tag '%s': %w", tag, err)
		}

		// Now, link this comment to this hashtag in a separate table (`post_hashtags`).
		// If they are already linked (`ON CONFLICT`), do nothing.
		// `tx.Exec` is used for queries that don't return rows (like INSERT, UPDATE, DELETE without RETURNING).
		var cmdTag pgconn.CommandTag
		cmdTag, err = tx.Exec(ctx, `
			INSERT INTO post_hashtags (post_id, hashtag_id)
			VALUES ($1, $2)
			ON CONFLICT (post_id, hashtag_id) DO NOTHING`, commentID, hashtagID)
		if err != nil {
			return nil, fmt.Errorf("failed to link hashtag to comment: %w", err)
		}
		_ = cmdTag // Avoid unused variable error if not checking rows affected
	} // All hashtags are now processed.

	// --- Comment Counters ---
	// We keep track of how many reactions and replies each comment has.
	// For our new comment, initialize these counts to zero.
	_, err = tx.Exec(ctx, `
		INSERT INTO comment_counters (comment_id, total_reactions, total_replies)
		VALUES ($1, 0, 0)
		ON CONFLICT (comment_id) DO NOTHING`, commentID) // If counters already exist (shouldn't for new comment), do nothing.
	if err != nil {
		return nil, fmt.Errorf("failed to initialize comment counters: %w", err)
	}

	// If our new comment was a reply to a parent comment...
	if params.ParentID != nil && *params.ParentID > 0 {
		// ...we need to increase the `total_replies` count for that parent comment.
		_, err = tx.Exec(ctx, `
			INSERT INTO comment_counters (comment_id, total_reactions, total_replies)
			VALUES ($1, 0, 1) /* Try to insert with 1 reply */
			ON CONFLICT (comment_id) DO UPDATE /* If parent already has counters, update it */
			SET total_replies = comment_counters.total_replies + 1`, *params.ParentID)
		if err != nil {
			return nil, fmt.Errorf("failed to update parent comment reply count: %w", err)
		}
	}

	// --- Prepare the full comment to send back to the user ---
	// We just saved the basic comment. Now, get all its details (like username, reactions, etc.)
	// so we can show the complete, newly created comment to the user.
	// Calling an internal helper method that uses the same transaction `tx`.
	createdComment, err := s.getCommentByIDInternal(ctx, tx, commentID, &userID) // `getCommentByIDInternal` is a helper for this.
	if err != nil {
		return nil, fmt.Errorf("failed to fetch newly created comment: %w", err)
	}

	// --- Notifications ---
	// If the comment is about a Lojban word (Valsi), we might need to tell people who are
	// "subscribed" to that word that there's a new comment.
	var valsiWord sql.NullString          // To store the Lojban word itself (e.g., "broda").
	var valsiIDForNotification sql.NullInt32 // To store the ID of that Lojban word.

	// Only try to get valsi info if the comment is actually linked to a valsi.
	   if params.ValsiID != nil && *params.ValsiID > 0 {
		// Get the word and its ID from the database, based on the thread and valsi ID.
	       err = tx.QueryRow(ctx, `
	           SELECT v.word, v.valsiid
	           FROM threads t
	           JOIN valsi v ON t.valsiid = v.valsiid
	           WHERE t.threadid = $1 AND v.valsiid = $2`, threadID, *params.ValsiID).Scan(&valsiWord, &valsiIDForNotification)
	       
	       if err != nil && err != pgx.ErrNoRows { // If an error happened (but not "not found")...
	           log.Printf("Error fetching valsi for notification (threadID: %d, valsiID: %d): %v", threadID, *params.ValsiID, err)
	           // This might not be a critical error, so we just log it and continue.
	       } else if err == pgx.ErrNoRows { // If no valsi was found for this thread/valsi_id combo.
	            log.Printf("No valsi found for notification (threadID: %d, valsiID: %d)", threadID, *params.ValsiID)
	       }
	   }


	// If we successfully got the Lojban word and its ID...
	if valsiWord.Valid && valsiIDForNotification.Valid {
		// `os.Getenv` reads an environment variable, used here for frontend URL configuration.
		frontendURL := os.Getenv("FRONTEND_URL") // Get the website's main address (e.g., "https://lensisku.com").
		if frontendURL == "" {
			log.Println("FRONTEND_URL environment variable not set, skipping notification URL generation.")
		} else {
			var defID int32 // If the comment is also about a specific definition.
			if params.DefinitionID != nil {
				defID = *params.DefinitionID
			}
			// Create a direct link to this new comment on the website.
			notificationURL := fmt.Sprintf("%s/comments?valsi_id=%d&definition_id=%d", frontendURL, valsiIDForNotification.Int32, defID)
			
			// Tell the database to send out notifications to subscribers of this Lojban word.
			_, err = tx.Exec(ctx, `SELECT notify_valsi_subscribers($1, 'comment', $2, $3, $4)`,
				valsiIDForNotification.Int32, // The ID of the Lojban word.
				fmt.Sprintf("New comment on thread for %s", valsiWord.String), // Message for the notification.
				notificationURL, // Link to the comment.
				userID,          // Who posted the comment (so they don't get notified about their own comment).
			)
			if err != nil { // If sending notifications failed...
				log.Printf("Error sending notification for valsi_id %d: %v", valsiIDForNotification.Int32, err)
				// Again, just log it. The comment was still added successfully.
			}
		}
	}

	// Phew! Everything is done. The `defer` function at the top will now try to `Commit` all these changes.
	// If `Commit` is successful, `err` will be `nil`. If `Commit` fails, `err` will have that error.
	return createdComment, nil // Return the fully formed comment and any error from Commit (or earlier).
}


// getCommentByIDInternal fetches a single comment by its ID using an existing transaction.
// This is an internal helper.
// Think of this as a private assistant for the `AddComment` manager (and other managers).
// It knows how to look up all the details of one specific comment from the database.
// It needs `tx` (the ongoing "all or nothing" process) to make sure it reads consistent data.
// `currentUserID` is to check if the person looking at the comment has liked or bookmarked it.
// The `Internal` suffix suggests it's not part of the public `CommentService` interface.
func (s *commentServiceImpl) getCommentByIDInternal(ctx context.Context, tx pgx.Tx, commentID int32, currentUserID *int32) (*Comment, error) {
	// This `commentRow` is a temporary container to hold all the bits of information
	// we get from the database for a comment. Some bits might be special, like JSON text.
	var commentRow struct {
		Comment             // Embeds the main Comment structure.
		ContentJSON         []byte         `db:"content_json"`        // The comment's main text/images, as raw JSON.
		ParentContentJSON   sql.NullString `db:"parent_content_json"` // If it's a reply, the parent's content.
		ValsiWordFromDB     sql.NullString `db:"valsi_word_from_db"`  // Lojban word, if any.
		DefinitionFromDB    sql.NullString `db:"definition_from_db"`  // Definition text, if any.
		FirstCommentSubjectFromDB sql.NullString `db:"first_comment_subject_from_db"` // Subject of the first comment in the thread.
		FirstCommentContentJSON sql.NullString `db:"first_comment_content_json"`    // Content of the first comment.
		LastCommentUsernameFromDB sql.NullString `db:"last_comment_username_from_db"` // User who made the latest reply.
	}

	// This is a big SQL query – a set of instructions for the database
	// to find and gather all the pieces of information for the comment.
	// It joins (connects) data from multiple tables:
	// `comments` (main comment data), `users` (who wrote it),
	// `comment_counters` (likes/replies), `comment_likes` (did current user like it?),
	// `comment_bookmarks` (did current user bookmark it?), and `threads` (what's it about?).
	// SQL `JOIN` clauses are used to combine data from these related tables.
	// `COALESCE` is used to provide default values for potentially NULL results (e.g., 0 for counts).
	query := `
		SELECT
			c.commentid,
			c.threadid,
			c.parentid,
			c.userid,
			c.commentnum,
			c.time,
			c.subject,
			c.content AS content_json, /* Get the raw JSON content */
			u.username,
			u.realname,
			COALESCE(cc.total_reactions, 0) as total_reactions, /* How many reactions in total? Default to 0 */
			COALESCE(cc.total_replies, 0) as total_replies,     /* How many replies? Default to 0 */
			CASE WHEN cl.user_id IS NOT NULL THEN true ELSE false END as is_liked,      /* Did the current user like this? */
			CASE WHEN cb.user_id IS NOT NULL THEN true ELSE false END as is_bookmarked, /* Did the current user bookmark this? */
			pc.content AS parent_content_json, /* If it's a reply, get parent's content as JSON */
			t.valsiid,      /* What Lojban word (ID) is this thread about? */
			t.definitionid  /* What definition (ID) is this thread about? */
			/* Other fields like valsi_word, definition text are fetched later if needed */
		FROM comments c
		JOIN users u ON c.userid = u.userid /* Link comment to its author */
		LEFT JOIN comment_counters cc ON c.commentid = cc.comment_id /* Link to its like/reply counts */
		LEFT JOIN comment_likes cl ON c.commentid = cl.comment_id AND cl.user_id = $2 /* Check if current user liked it */
		LEFT JOIN comment_bookmarks cb ON c.commentid = cb.comment_id AND cb.user_id = $2 /* Check if current user bookmarked it */
		LEFT JOIN comments pc ON c.parentid = pc.commentid /* If it's a reply, link to parent comment */
		LEFT JOIN threads t ON c.threadid = t.threadid /* Link comment to its conversation topic */
		WHERE c.commentid = $1 /* We only want the comment with this specific ID */`

	// Ask the database to run the query and put the results into `commentRow`.
	// `$1` is `commentID`, `$2` is `currentUserID`.
	// `Scan` populates the fields of `commentRow` from the query result. The order of fields in `Scan` must match the order of columns in the `SELECT` statement.
	// For pgx, we scan into the fields of the struct directly.
	err := tx.QueryRow(ctx, query, commentID, currentUserID).Scan(
		&commentRow.CommentID, // c.commentid
		&commentRow.ThreadID,
		&commentRow.ParentID,
		&commentRow.UserID,
		&commentRow.CommentNum,
		&commentRow.Time,
		&commentRow.Subject,
		&commentRow.ContentJSON,         // c.content AS content_json
		&commentRow.Username,            // u.username
		&commentRow.Realname,            // u.realname
		&commentRow.TotalReactions,      // COALESCE(cc.total_reactions, 0)
		&commentRow.TotalReplies,        // COALESCE(cc.total_replies, 0)
		&commentRow.IsLiked,             // CASE WHEN cl.user_id IS NOT NULL
		&commentRow.IsBookmarked,        // CASE WHEN cb.user_id IS NOT NULL
		&commentRow.ParentContentJSON,   // pc.content AS parent_content_json
		&commentRow.Comment.ValsiID,     // t.valsiid - directly into embedded struct
		&commentRow.Comment.DefinitionID,// t.definitionid - directly into embedded struct
	)

	if err != nil {
		if err == pgx.ErrNoRows { // If the database says "sorry, no comment with that ID"...
			return nil, fmt.Errorf("comment with ID %d not found", commentID)
		}
		// Some other database error.
		return nil, fmt.Errorf("error fetching comment by ID %d: %w", commentID, err)
	}

	// We got the raw data. Now, put it into a nice, final `Comment` structure.
	var finalComment Comment = commentRow.Comment // Start with the basic fields.
	finalComment.CommentID = commentRow.CommentID
	finalComment.ThreadID = commentRow.ThreadID
	finalComment.ParentID = commentRow.ParentID
	finalComment.UserID = commentRow.UserID
	finalComment.CommentNum = commentRow.CommentNum
	finalComment.Time = commentRow.Time
	finalComment.Subject = commentRow.Subject
	finalComment.Username = commentRow.Username
	finalComment.Realname = commentRow.Realname
	finalComment.TotalReactions = commentRow.TotalReactions
	finalComment.TotalReplies = commentRow.TotalReplies
	finalComment.IsLiked = commentRow.IsLiked
	finalComment.IsBookmarked = commentRow.IsBookmarked


	// The `ContentJSON` was raw text. We need to "unmarshal" it back into structured `CommentContent` parts.
	// `json.Unmarshal` parses JSON data (byte slice) into a Go data structure.
	if err := json.Unmarshal(commentRow.ContentJSON, &finalComment.Content); err != nil {
		return nil, fmt.Errorf("error unmarshalling comment content for comment ID %d: %w", commentID, err)
	}
	// Same for the parent comment's content, if it exists.
	if commentRow.ParentContentJSON.Valid { // `.Valid` checks if there was a parent content.
		if err := json.Unmarshal([]byte(commentRow.ParentContentJSON.String), &finalComment.ParentContent); err != nil {
			return nil, fmt.Errorf("error unmarshalling parent comment content for comment ID %d: %w", commentID, err)
		}
	}
	
	// The main query already got `ValsiID` and `DefinitionID` from the `threads` table.
	   // finalComment.ValsiID = commentRow.ValsiID // Already set via embedded struct scan
	   // finalComment.DefinitionID = commentRow.DefinitionID // Already set

	// Now, let's get all the reactions (like 👍, ❤️, 😂) for this comment.
	// Calls another internal helper to fetch reaction details.
	reactions, err := s.fetchReactionsInternal(ctx, tx, []int32{commentID}, currentUserID) // Another helper does this.
	if err != nil {
		return nil, fmt.Errorf("error fetching reactions for comment ID %d: %w", commentID, err)
	}
	if r, ok := reactions[commentID]; ok { // If reactions were found for this comment...
		finalComment.Reactions = r // ...add them to our `finalComment`.
	} else {
		finalComment.Reactions = []ReactionResponse{} // Otherwise, it's an empty list of reactions.
	}
	
	// If this comment is tied to a Lojban word (ValsiID exists)...
	   if finalComment.ValsiID != nil && *finalComment.ValsiID > 0 {
	       var valsiWord string
		// ...look up the actual word (e.g., "broda") from the `valsi` table.
	       err := tx.QueryRow(ctx, "SELECT word FROM valsi WHERE valsiid = $1", *finalComment.ValsiID).Scan(&valsiWord)
	       if err == nil { // If found...
	           finalComment.ValsiWord = &valsiWord // ...add it to our `finalComment`.
	       } else if err != pgx.ErrNoRows { // If some other error (not "not found")...
	           log.Printf("Error fetching valsi word for valsi_id %d: %v", *finalComment.ValsiID, err)
	       }
	   }

	   // If this comment is tied to a specific definition (DefinitionID exists)...
	   if finalComment.DefinitionID != nil && *finalComment.DefinitionID > 0 {
	       var definitionText string
		// ...look up the text of that definition from the `definitions` table.
	       err := tx.QueryRow(ctx, "SELECT definition FROM definitions WHERE definitionid = $1", *finalComment.DefinitionID).Scan(&definitionText)
	       if err == nil { // If found...
	           finalComment.Definition = &definitionText // ...add it to our `finalComment`.
	       } else if err != pgx.ErrNoRows { // If some other error...
	           log.Printf("Error fetching definition for definition_id %d: %v", *finalComment.DefinitionID, err)
	       }
	   }

	// The `finalComment` is now fully assembled!
	return &finalComment, nil
}


// fetchReactionsInternal fetches reactions for a list of comment IDs using an existing transaction.
// This is another private assistant. It's good at finding all reactions (like 👍, ❤️)
// for one or more comments.
// `commentIDs` is a list of comments we're interested in.
// `tx pgx.Tx` indicates this function must be called within an existing database transaction.
// `currentUserID` helps us know if the person looking has already reacted.
func (s *commentServiceImpl) fetchReactionsInternal(ctx context.Context, tx pgx.Tx, commentIDs []int32, currentUserID *int32) (map[int32][]ReactionResponse, error) {
	if len(commentIDs) == 0 { // If no comments were asked for, nothing to do.
		return make(map[int32][]ReactionResponse), nil // Return an empty map.
	}

	// This SQL query is a bit tricky. For each comment ID in our list:
	// 1. Group reactions by type (e.g., all 👍 together, all ❤️ together).
	// 2. Count how many of each type there are.
	// 3. Check if the `currentUserID` made one of those reactions (`reacted`).
	// pgx uses $1, $2 for placeholders. We'll use ANY($1) for the list.
	// `ANY($1)` is a PostgreSQL operator to compare a value against an array of values.
	// `BOOL_OR` is an aggregate function that returns true if any input value is true.
	// `ANY($1)` is a PostgreSQL operator to compare a value against an array of values.
	// `BOOL_OR` is an aggregate function that returns true if any input value is true.
	query := `
		SELECT
			cr.comment_id,
			cr.reaction, /* The type of reaction, e.g., "👍" */
			COUNT(*) as count, /* How many of this reaction type */
			COALESCE(BOOL_OR(cr.user_id = $2), false) as reacted /* Did the current user make this type of reaction? */
		FROM comment_reactions cr
		WHERE cr.comment_id = ANY($1) /* For all comments in our list */
		GROUP BY cr.comment_id, cr.reaction /* Group by comment and reaction type */
		ORDER BY cr.comment_id, count DESC, cr.reaction /* Order them nicely */`
	


	// `tx.Query` is used when a query can return multiple rows.
	rows, err := tx.Query(ctx, query, commentIDs, currentUserID)
	if err != nil {
		return nil, fmt.Errorf("error executing fetchReactions query: %w", err)
	}
	// `defer rows.Close()` ensures the `pgx.Rows` result set is closed, freeing database resources.
	// This is crucial to prevent connection leaks.
	// `defer rows.Close()` ensures the `pgx.Rows` result set is closed, freeing database resources.
	defer rows.Close()

	// Now, organize the results into a map.
	// The map's key will be the `commentID`, and the value will be a list of its reactions.
	// `make(map[int32][]ReactionResponse)` initializes an empty map.
	reactionsMap := make(map[int32][]ReactionResponse)
	// `rows.Next()` advances to the next row in the result set. It returns false when there are no more rows or an error occurs.
	for rows.Next() { // For each reaction result we got...
		var commentID int32
		var reaction string
		var count int64
		var reacted bool
		if err := rows.Scan(&commentID, &reaction, &count, &reacted); err != nil {
			// Error scanning a row; important to handle.
			// Error scanning a row; important to handle.
			return nil, fmt.Errorf("error scanning reaction row: %w", err)
		}
		reactionsMap[commentID] = append(reactionsMap[commentID], ReactionResponse{
			Reaction: reaction,
			Count:    count,
			Reacted:  reacted,
		})
	}
	// `rows.Err()` checks for any errors that occurred during row iteration (e.g., network issues).
	// `rows.Err()` checks for any errors that occurred during row iteration (e.g., network issues).
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating reaction rows: %w", err)
	}

	// Just in case some comments had NO reactions, make sure they still have an empty list in our map.
	for _, id := range commentIDs {
		if _, ok := reactionsMap[id]; !ok { // If a comment ID isn't in the map yet...
			reactionsMap[id] = []ReactionResponse{} // ...add it with an empty list of reactions.
		}
	}
	return reactionsMap, nil // All done! Return the map of reactions.
}

// Placeholder for other CommentService methods
// These methods are part of the `CommentService` interface but are not yet implemented.
// These methods are part of the `CommentService` interface but are not yet implemented.
func (s *commentServiceImpl) GetThreadComments(params ThreadQuery, currentUserID *int32) (*PaginatedCommentsResponse, error) {
	// TODO: Implement
	return nil, fmt.Errorf("GetThreadComments not implemented")
}

func (s *commentServiceImpl) ToggleLike(commentID int32, userID int32, like bool) error {
	// TODO: Implement
	return fmt.Errorf("ToggleLike not implemented")
}

func (s *commentServiceImpl) ToggleBookmark(commentID int32, userID int32, bookmark bool) error {
	// TODO: Implement
	return fmt.Errorf("ToggleBookmark not implemented")
}
func (s *commentServiceImpl) GetBookmarkedComments(userID int32, page int64, perPage int64, currentUserID *int32) (*PaginatedCommentsResponse, error) {
	// TODO: Implement
	return nil, fmt.Errorf("GetBookmarkedComments not implemented")
}
func (s *commentServiceImpl) GetLikedComments(userID int32, page int64, perPage int64, currentUserID *int32) (*PaginatedCommentsResponse, error) {
	// TODO: Implement
	return nil, fmt.Errorf("GetLikedComments not implemented")
}
func (s *commentServiceImpl) GetUserComments(userID int32, page int64, perPage int64, currentUserID *int32) (*PaginatedCommentsResponse, error) {
	// TODO: Implement
	return nil, fmt.Errorf("GetUserComments not implemented")
}
func (s *commentServiceImpl) CreateOpinion(userID int32, req CreateOpinionRequest) (*CommentOpinion, error) {
	// TODO: Implement
	return nil, fmt.Errorf("CreateOpinion not implemented")
}
func (s *commentServiceImpl) SetOpinionVote(userID int32, req OpinionVoteRequest) error {
	// TODO: Implement
	return fmt.Errorf("SetOpinionVote not implemented")
}
func (s *commentServiceImpl) GetCommentOpinions(commentID int32, userID *int32) ([]CommentOpinion, error) {
	// TODO: Implement
	return nil, fmt.Errorf("GetCommentOpinions not implemented")
}
func (s *commentServiceImpl) GetTrendingComments(timespan TrendingTimespan, currentUserID *int32, limit int32) ([]Comment, error) {
	// TODO: Implement
	return nil, fmt.Errorf("GetTrendingComments not implemented")
}
func (s *commentServiceImpl) GetCommentStats(commentID int32) (*CommentStats, error) {
	// TODO: Implement
	return nil, fmt.Errorf("GetCommentStats not implemented")
}
func (s *commentServiceImpl) GetMostBookmarkedComments(page int64, perPage int64, currentUserID *int32) (*PaginatedCommentsResponse, error) {
	// TODO: Implement
	return nil, fmt.Errorf("GetMostBookmarkedComments not implemented")
}
func (s *commentServiceImpl) GetTrendingHashtags(timespan TrendingTimespan, limit int32) ([]TrendingHashtag, error) {
	// TODO: Implement
	return nil, fmt.Errorf("GetTrendingHashtags not implemented")
}
func (s *commentServiceImpl) GetCommentsByHashtag(tag string, userID *int32, page *int64, perPage *int64) (*PaginatedCommentsResponse, error) {
	// TODO: Implement
	return nil, fmt.Errorf("GetCommentsByHashtag not implemented")
}
func (s *commentServiceImpl) DeleteComment(commentID int32, userID int32) error {
	// TODO: Implement
	return fmt.Errorf("DeleteComment not implemented")
}
func (s *commentServiceImpl) ToggleReaction(commentID int32, userID int32, reaction string) (bool, error) {
	// TODO: Implement
	return false, fmt.Errorf("ToggleReaction not implemented")
}
func (s *commentServiceImpl) SearchComments(params SearchCommentsQuery, currentUserID *int32) (*PaginatedCommentsResponse, error) {
	// TODO: Implement
	return nil, fmt.Errorf("SearchComments not implemented")
}
func (s *commentServiceImpl) GetMyReactions(userID int32, page int64, perPage int64) (*PaginatedCommentsResponse, error) {
	// TODO: Implement
	return nil, fmt.Errorf("GetMyReactions not implemented")
}
func (s *commentServiceImpl) GetReactions(commentID int32, currentUserID *int32, page *int64, pageSize *int32) (*ReactionSummary, error) {
	// TODO: Implement
	return nil, fmt.Errorf("GetReactions not implemented")
}
func (s *commentServiceImpl) ListThreads(page int64, perPage int64, sortBy string, sortOrder string) (*PaginatedCommentsResponse, error) {
	// TODO: Implement
	return nil, fmt.Errorf("ListThreads not implemented")
}
func (s *commentServiceImpl) ListComments(page int64, perPage int64, sortOrder string, currentUserID *int32) (*PaginatedCommentsResponse, error) {
	// TODO: Implement
	return nil, fmt.Errorf("ListComments not implemented")
}
func (s *commentServiceImpl) GetLikeCount(commentID int32) (int64, error) {
	// TODO: Implement
	return 0, fmt.Errorf("GetLikeCount not implemented")
}

// Ensure all interface methods are implemented (compile-time check)
// This is a common Go idiom to verify at compile time that `commentServiceImpl`
// correctly implements the `CommentService` interface. If not, the compilation will fail.
var _ CommentService = (*commentServiceImpl)(nil)
