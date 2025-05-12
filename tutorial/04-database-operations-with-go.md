# Chapter 4: Database Operations with Go

In this chapter, we'll explore how to implement database operations in Go, translating our Rust application's database layer while maintaining its robustness and functionality. We'll cover connection management, query execution, migrations, and handling PostgreSQL-specific features.

## Interfacing with Databases in Go: The `database/sql` Package

Go's standard library provides the `database/sql` package as a generic interface to SQL databases. Unlike Rust's ecosystem where we used `deadpool_postgres`, Go takes a different approach to database connectivity.

### Choosing and Using Database Drivers

In our Rust implementation, we use PostgreSQL with `deadpool_postgres`. For Go, we have several options for PostgreSQL drivers:

```go
// Using pq (traditional choice)
import _ "github.com/lib/pq"

// Using pgx (modern, recommended choice)
import _ "github.com/jackc/pgx/v5/stdlib"
```

The `pgx` driver is recommended for new projects as it offers better performance and more PostgreSQL-specific features. Here's how we can set up a database connection:

```go
import (
    "database/sql"
    _ "github.com/jackc/pgx/v5/stdlib"
)

func NewDB(config *Config) (*sql.DB, error) {
    db, err := sql.Open("pgx", config.DatabaseURL)
    if err != nil {
        return nil, fmt.Errorf("error opening database: %w", err)
    }

    // Configure connection pool
    db.SetMaxOpenConns(25)
    db.SetMaxIdleConns(5)
    db.SetConnMaxLifetime(5 * time.Minute)

    return db, nil
}
```

### Connection Pooling Mechanisms

In our Rust code, we use `deadpool_postgres` for connection pooling:

```rust
// Rust implementation (from config.rs)
pub struct DatabasePools {
    pub app_pool: Pool,
    pub import_pool: Pool,
}
```

Go's `database/sql` package includes built-in connection pooling. While not as explicitly configured as Rust's `deadpool`, it's production-ready and well-tested:

```go
type DatabasePools struct {
    AppPool    *sql.DB
    ImportPool *sql.DB
}

func NewDatabasePools(config *Config) (*DatabasePools, error) {
    appPool, err := NewDB(&config.App)
    if err != nil {
        return nil, fmt.Errorf("failed to create app pool: %w", err)
    }

    importPool, err := NewDB(&config.Import)
    if err != nil {
        return nil, fmt.Errorf("failed to create import pool: %w", err)
    }

    return &DatabasePools{
        AppPool:    appPool,
        ImportPool: importPool,
    }, nil
}
```

## Translating `db.rs`

Let's look at how to translate our Rust database operations to idiomatic Go code.

### Executing Raw SQL Queries and Managing Transactions

Here's how we translate the `get_message_count` function from Rust to Go:

```rust
// Rust implementation (from db.rs)
pub async fn get_message_count(pool: &Pool) -> AppResult<i64> {
    let conn = pool.get().await.map_err(|e| AppError::Database(e.to_string()))?;
    let row = conn.query_one("SELECT COUNT(*) FROM messages", &[])
        .await
        .map_err(|e| AppError::Database(e.to_string()))?;
    row.try_get(0).map_err(|e| AppError::Database(e.to_string()))
}
```

```go
// Go implementation
func GetMessageCount(db *sql.DB) (int64, error) {
    var count int64
    err := db.QueryRow("SELECT COUNT(*) FROM messages").Scan(&count)
    if err != nil {
        return 0, fmt.Errorf("error getting message count: %w", err)
    }
    return count, nil
}
```

For enabling extensions, we can translate the Rust code as follows:

```rust
// Rust implementation (from db.rs)
pub async fn enable_extensions(pool: &Pool) -> AppResult<()> {
    let client = pool.get().await.map_err(|e| AppError::Database(e.to_string()))?;
    client.execute("CREATE EXTENSION IF NOT EXISTS pg_trgm", &[])
        .await.map_err(|e| AppError::Database(e.to_string()))?;
    client.execute("CREATE EXTENSION IF NOT EXISTS vector", &[])
        .await.map_err(|e| AppError::Database(e.to_string()))?;
    Ok(())
}
```

```go
// Go implementation
func EnableExtensions(db *sql.DB) error {
    extensions := []string{
        "CREATE EXTENSION IF NOT EXISTS pg_trgm",
        "CREATE EXTENSION IF NOT EXISTS vector",
    }

    for _, ext := range extensions {
        if _, err := db.Exec(ext); err != nil {
            return fmt.Errorf("error enabling extension: %w", err)
        }
    }
    return nil
}
```

### Mapping Database Rows to Go Structs

While Rust's `tokio-postgres` requires manual row scanning, Go provides several options:

1. Manual scanning (built-in):
```go
type Message struct {
    ID      int64
    Content string
    UserID  int64
}

func GetMessage(db *sql.DB, id int64) (*Message, error) {
    var msg Message
    err := db.QueryRow("SELECT id, content, user_id FROM messages WHERE id = $1", id).
        Scan(&msg.ID, &msg.Content, &msg.UserID)
    if err != nil {
        return nil, fmt.Errorf("error getting message: %w", err)
    }
    return &msg, nil
}
```

2. Using `sqlx` for tagged struct scanning:
```go
import "github.com/jmoiron/sqlx"

type Message struct {
    ID      int64  `db:"id"`
    Content string `db:"content"`
    UserID  int64  `db:"user_id"`
}

func GetMessage(db *sqlx.DB, id int64) (*Message, error) {
    var msg Message
    err := db.Get(&msg, "SELECT id, content, user_id FROM messages WHERE id = $1", id)
    if err != nil {
        return nil, fmt.Errorf("error getting message: %w", err)
    }
    return &msg, nil
}
```

### ORMs and Query Builders

While our Rust implementation uses raw SQL, Go offers several ORM options:

1. GORM - Full-featured ORM:
```go
import "gorm.io/gorm"

type Message struct {
    ID      int64  `gorm:"primaryKey"`
    Content string
    UserID  int64
}

func GetMessage(db *gorm.DB, id int64) (*Message, error) {
    var msg Message
    result := db.First(&msg, id)
    return &msg, result.Error
}
```

2. SQLBoiler - Type-safe query builder:
```go
// Generated code from SQLBoiler
message, err := models.FindMessage(ctx, db, id)
```

3. `sqlc` - SQL-first approach with type-safe generated code:
```sql
-- queries.sql
-- name: GetMessage :one
SELECT id, content, user_id FROM messages WHERE id = $1;
```

Generated Go code:
```go
func (q *Queries) GetMessage(ctx context.Context, id int64) (Message, error) {
    // Type-safe generated implementation
}
```

## Database Migrations in a Go Project

Our Rust project uses `refinery` for migrations:

```rust
// Rust implementation (from db.rs)
mod embedded {
    use refinery::embed_migrations;
    embed_migrations!("./migrations");
}

pub async fn run_migrations(pool: &Pool) -> AppResult<()> {
    let mut conn = pool.get().await.map_err(|e| AppError::Database(e.to_string()))?;
    embedded::migrations::runner()
        .run_async(&mut **conn)
        .await
        .map_err(|e| {
            error!("Error running migrations: {:?}", e);
            AppError::Migration(e.to_string())
        })?;
    info!("Migrations ran successfully");
    Ok(())
}
```

For Go, we can use `golang-migrate/migrate`:

```go
import (
    "github.com/golang-migrate/migrate/v4"
    _ "github.com/golang-migrate/migrate/v4/database/postgres"
    _ "github.com/golang-migrate/migrate/v4/source/file"
)

func RunMigrations(dbURL string) error {
    m, err := migrate.New(
        "file://migrations",
        dbURL,
    )
    if err != nil {
        return fmt.Errorf("error creating migrator: %w", err)
    }
    
    if err := m.Up(); err != nil && err != migrate.ErrNoChange {
        return fmt.Errorf("error running migrations: %w", err)
    }
    
    log.Println("Migrations ran successfully")
    return nil
}
```

The migration files structure remains the same:
```
migrations/
├── V2__lensisku.sql
├── V12__user_created_at.sql
// ... more migration files
```

## Handling Database-Specific Types and Extensions

Our Rust code uses PostgreSQL-specific features like `pg_trgm` and the `vector` extension. Here's how we handle these in Go:

1. JSONB support:
```go
import "encoding/json"

type DataWithJSON struct {
    ID   int64
    Data json.RawMessage
}

func SaveJSON(db *sql.DB, d *DataWithJSON) error {
    _, err := db.Exec(
        "INSERT INTO data_table (id, data) VALUES ($1, $2)",
        d.ID, d.Data,
    )
    return err
}
```

2. Custom types and extensions:
```go
import "github.com/lib/pq"

// Array support
type TaggedItem struct {
    ID   int64
    Tags pq.StringArray `db:"tags"`
}

// Vector type (using pgvector)
type Vector []float32

func (v Vector) Value() (driver.Value, error) {
    // Convert vector to PostgreSQL vector format
    return fmt.Sprintf("[%s]", strings.Join(strings.Fields(fmt.Sprint(v)), ",")), nil
}

func (v *Vector) Scan(src interface{}) error {
    // Parse PostgreSQL vector format into Go slice
    // Implementation details...
}
```

Using these types with queries:
```go
func SearchSimilar(db *sql.DB, embedding Vector) ([]Item, error) {
    rows, err := db.Query(
        "SELECT id, content FROM items ORDER BY embedding <-> $1 LIMIT 10",
        embedding,
    )
    // Process results...
}
```

This chapter demonstrates how to maintain the functionality of our Rust database layer while following Go idioms and best practices. The Go ecosystem provides robust tools for database operations, with options ranging from low-level drivers to full-featured ORMs. Choose the approach that best fits your project's needs while maintaining type safety and performance.