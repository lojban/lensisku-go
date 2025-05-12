// Package auth, as previously noted, handles authentication.
// This file, `models.go`, defines data structures that represent entities or core concepts
// within the authentication domain. In this case, it defines the `User` struct.
package auth

import "time"

// User represents a user in the system.
// This struct is analogous to an "Entity" in ORM terms (like TypeORM in Nest.js)
// or a "Model" in MVC patterns. It defines the structure of user data as stored in the database
// and as used within the application's business logic.
type User struct {
	// `json:"id"` are struct tags. They provide metadata for encoding/decoding,
	// in this case, for JSON marshalling/unmarshalling. The `json:"-"` tag for HashedPassword
	// means this field will be ignored by the `encoding/json` package, preventing it from being exposed in API responses.
	ID             int       `json:"id"`
	Username       string    `json:"username"`
	Email          string    `json:"email"`
	HashedPassword string    `json:"-"` // Do not expose hashed password
	CreatedAt      time.Time `json:"created_at"`
	// `time.Time` is Go's standard type for representing time.
}
