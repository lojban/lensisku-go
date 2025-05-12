# Chapter 2: Project Structure and Configuration Management

In this chapter, we'll explore how to structure our Go rewrite of the Lojban dictionary application, focusing on project organization and configuration management. We'll use the original Rust project structure as a reference point while embracing Go's idioms and best practices.

## Project Structure: From Rust to Go

### Understanding the Original Rust Structure

The Rust project follows a common pattern with a `src/` directory containing multiple modules:

```
src/
├── main.rs           # Application entry point
├── config.rs         # Configuration management
├── server.rs         # HTTP server setup
├── auth/            # Authentication module
├── users/           # User management
├── language/        # Language-specific functionality
└── ... other modules
```

This structure uses Rust's module system, where each `.rs` file or directory implicitly defines a module. The `mod.rs` files in subdirectories serve as module entry points.

### Go Project Layout Best Practices

In Go, we'll adapt this structure to follow the [Standard Go Project Layout](https://github.com/golang-standards/project-layout). Here's our recommended structure:

```
.
├── cmd/
│   └── lojbanserver/
│       └── main.go       # Application entry point
├── internal/
│   ├── auth/            # Authentication package
│   ├── config/          # Configuration package
│   ├── server/          # Server setup and routing
│   ├── users/           # User management
│   └── language/        # Language-specific functionality
├── pkg/
│   ├── common/          # Shared utilities
│   └── models/          # Shared data models
└── api/
    └── openapi/         # OpenAPI/Swagger specifications
```

Key differences from Rust:

1. **Command Entry Points**: The `cmd/` directory contains our executable entry points. Each subdirectory is a separate executable.

2. **Internal vs. Public Code**: 
   - `internal/`: Private application code that won't be imported by other projects
   - `pkg/`: Public libraries that may be used by other projects
   
3. **Package Organization**: Instead of Rust's modules with `mod.rs`, Go uses directories with multiple `.go` files:

```go
internal/auth/
├── handler.go      // HTTP handlers
├── middleware.go   // Auth middleware
├── model.go        // Data models
└── service.go      // Business logic
```

## Configuration Management

### From `config.rs` to Go

Let's examine how to translate the Rust configuration management to idiomatic Go. Here's our approach:

```go
// internal/config/config.go

package config

import (
    "fmt"
    "os"
    "strconv"
    "time"
)

// AppConfig holds all application configuration
type AppConfig struct {
    DBPools DatabasePools
}

// DatabasePools holds different database connection pools
type DatabasePools struct {
    AppPool    *sql.DB
    ImportPool *sql.DB
}

// DBConfig holds database connection configuration
type DBConfig struct {
    Host     string
    Port     int
    User     string
    Password string
    DBName   string
    PoolSize int
}

// LoadEnvVars loads required environment variables
func LoadEnvVars(vars []string) ([]string, error) {
    var values []string
    var errors []string

    for _, v := range vars {
        if value, exists := os.LookupEnv(v); exists {
            values = append(values, value)
        } else {
            errors = append(errors, fmt.Sprintf("%s: not set", v))
        }
    }

    if len(errors) > 0 {
        return nil, fmt.Errorf("configuration errors: %v", errors)
    }
    return values, nil
}
```

### Key Differences from Rust

1. **Error Handling**: Instead of Rust's `Result` type, we use Go's multiple return values with error:

```go
// Go error handling
func CreateAppConfig() (*AppConfig, error) {
    dbPools, err := createDBPools()
    if err != nil {
        return nil, fmt.Errorf("failed to create DB pools: %w", err)
    }
    return &AppConfig{DBPools: dbPools}, nil
}
```

2. **Database Connection Pools**: Go's `database/sql` package provides built-in connection pooling:

```go
func createDBPool(cfg DBConfig) (*sql.DB, error) {
    db, err := sql.Open("postgres", fmt.Sprintf(
        "host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
        cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DBName,
    ))
    if err != nil {
        return nil, err
    }

    // Configure pool settings
    db.SetMaxOpenConns(cfg.PoolSize)
    db.SetMaxIdleConns(cfg.PoolSize)
    db.SetConnMaxLifetime(5 * time.Minute)

    return db, nil
}
```

3. **Configuration Loading**: Go offers several approaches:

```go
// Standard library approach
func loadConfig() (*DBConfig, error) {
    cfg := &DBConfig{
        Host: getEnvOrDefault("DB_HOST", "localhost"),
        Port: getEnvAsIntOrDefault("DB_PORT", 5432),
        User: os.Getenv("DB_USER"),
        Password: os.Getenv("DB_PASSWORD"),
    }
    return cfg, cfg.validate()
}

// Using external libraries (e.g., Viper)
func loadConfigWithViper() (*DBConfig, error) {
    v := viper.New()
    v.SetConfigFile(".env")
    v.AutomaticEnv()
    
    if err := v.ReadInConfig(); err != nil {
        return nil, err
    }
    
    var cfg DBConfig
    if err := v.Unmarshal(&cfg); err != nil {
        return nil, err
    }
    return &cfg, nil
}
```

## Dependency Management

Go modules (`go.mod`) serve a similar purpose to Rust's `Cargo.toml`:

```go
// go.mod
module github.com/yourusername/lojbanserver

go 1.21

require (
    github.com/lib/pq v1.10.9
    github.com/spf13/viper v1.18.2
    // ... other dependencies
)
```

Key differences from Rust's Cargo:

1. **Version Resolution**: 
   - Go: Uses minimum version selection (MVS)
   - Rust: Uses maximum version selection with SemVer

2. **Lock Files**:
   - Go: `go.sum` contains cryptographic hashes
   - Rust: `Cargo.lock` contains exact dependency tree

## Build System and Tools

Go provides a rich set of built-in tools:

```bash
# Build the project
go build ./cmd/lojbanserver

# Run tests
go test ./...

# Code formatting
go fmt ./...

# Code validation
go vet ./...

# Install linter
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Run linter
golangci-lint run
```

Additional tools to consider:
- `goimports`: Manages imports automatically
- `staticcheck`: Advanced static analysis
- `wire`: Compile-time dependency injection

## Key Takeaways

1. Go's project structure emphasizes clear separation between internal and public code
2. Configuration management is more straightforward with Go's standard library
3. Go's tooling is more integrated and standardized compared to Rust
4. Database connection pooling is handled differently, with more built-in functionality
5. Dependency management is simpler but less deterministic than Cargo

In the next chapter, we'll explore how to implement the HTTP server and routing layer, translating Rust's Actix-web patterns to Go's `net/http` or popular frameworks like Echo or Fiber.