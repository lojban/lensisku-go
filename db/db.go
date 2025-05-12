// Package db provides database connectivity and migration functionality for the lensisku application.
// It handles establishing database connections, managing connection pools, enabling required
// PostgreSQL extensions, and running database migrations.
// This package centralizes database concerns, similar to how a database module (e.g., TypeORMModule)
// would be configured in Nest.js, providing a connection or pool to the rest of the application.
package db

import (
	"context"
	"fmt"
	// `time` is used for setting timeouts and connection pool configurations.
	"time"

	// `golang-migrate` is a popular library for database migrations in Go.
	// It supports various database drivers and migration source formats (like SQL files).
	"github.com/golang-migrate/migrate/v4"
	// `_ "github.com/golang-migrate/migrate/v4/source/file"` imports the file source driver for golang-migrate.
	// The underscore `_` means the package is imported for its side effects (registering the driver).
	_ "github.com/golang-migrate/migrate/v4/source/file" // For file-based migrations
	// `_ "github.com/lib/pq"` imports the `lib/pq` PostgreSQL driver. This specific import is often
	// required by `golang-migrate`'s `postgres` database driver when using DSNs, as `migrate`
	// might internally use `database/sql` with `lib/pq`.
	_ "github.com/lib/pq"                               // driver for database/sql, needed by migrate's postgres driver with DSN
	// `pgxpool` is part of the `jackc/pgx` suite, providing a robust connection pool for PostgreSQL.
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/user/lensisku-go/apperror"
	"github.com/user/lensisku-go/config"
)
// NewDBPools establishes connections to PostgreSQL databases using the provided configuration.
// It returns two database pools - one for regular application queries and one for import operations.
//
// The function uses pgx/v5 driver which provides better performance than lib/pq.
// It configures connection pools with appropriate settings based on the configuration,
// including max connections, connection lifetime, and idle connection management.
func NewDBPools(cfg *config.DatabasePools) (*pgxpool.Pool, *pgxpool.Pool, error) {
	// This function demonstrates creating multiple database pools, potentially for different
	// purposes or even different databases, based on the application's needs.

	// Create the application database pool
	appPool, err := createPgxPool(cfg.AppPool)
	if err != nil {
		// If pool creation fails, wrap the error with `apperror` for consistent error handling.
		return nil, nil, apperror.NewDatabaseError("failed to create application pool", err)
	}

	// Create the import database pool
	importPool, err := createPgxPool(cfg.ImportPool)
	if err != nil {
		// If the second pool creation fails, ensure the first pool is closed to release resources.
		if appPool != nil {
			appPool.Close() // Clean up the app pool if import pool creation fails
		}
		return nil, nil, apperror.NewDatabaseError("failed to create import pool", err)
	}

	return appPool, importPool, nil
}

// createPgxPool establishes a single pgxpool connection pool.
// This helper function encapsulates the logic for creating and configuring one `pgxpool.Pool`.
func createPgxPool(cfg *config.PoolConfig) (*pgxpool.Pool, error) {
	dsn := fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=disable&pool_max_conns=%d&pool_max_conn_idle_time=%s&pool_max_conn_lifetime=%s",
		cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.DBName,
		cfg.MaxSize,
		(10 * time.Minute).String(), // Example: pool_max_conn_idle_time
		(30 * time.Minute).String(), // Example: pool_max_conn_lifetime
	)

	// `pgxpool.ParseConfig` parses the DSN string into a `pgxpool.Config` struct.
	poolConfig, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, apperror.NewDatabaseError(fmt.Sprintf("error parsing DSN for database %s", cfg.DBName), err)
	}

	// Further configure the pool settings directly on the `poolConfig` struct.
	// pgxpool.Config allows more fine-grained control if needed, e.g., HealthCheckPeriod
	poolConfig.MaxConns = int32(cfg.MaxSize)
	poolConfig.MaxConnIdleTime = 10 * time.Minute
	poolConfig.MaxConnLifetime = 30 * time.Minute
	// poolConfig.MinConns = int32(cfg.MaxSize / 4) // Example: set min connections

	// Use a context with a timeout for the pool creation process.
	// This prevents indefinite blocking if the database is unreachable.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second) // Increased timeout for pool creation
	// `defer cancel()` ensures the context's resources are released when `createPgxPool` returns.
	defer cancel()

	// `pgxpool.NewWithConfig` creates the connection pool using the configured settings.
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, apperror.NewDatabaseError(fmt.Sprintf("error creating pgxpool for database %s", cfg.DBName), err)
	}

	// It's good practice to ping the database to verify the connection after creating the pool.
	// Verify the connection by pinging
	pingCtx, pingCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer pingCancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close() // Clean up on connection failure
		return nil, apperror.NewDatabaseError(fmt.Sprintf("error connecting to the database %s with pgxpool", cfg.DBName), err)
	}

	return pool, nil
}

// getDSN constructs a DSN string from PoolConfig, suitable for golang-migrate.
func getDSN(cfg *config.PoolConfig) string {
	// `golang-migrate`'s `postgres` driver (which often uses `lib/pq` under the hood)
	// typically expects a DSN in a slightly different format than `pgx`.
	// Note: golang-migrate's postgres driver typically uses lib/pq format DSN
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=disable",
		cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.DBName,
	)
}

// EnableExtensions enables required PostgreSQL extensions for the lensisku application.
// It currently enables pg_trgm for text search functionality and vector for embeddings support.
// It takes the importPool *sql.DB as an argument.
func EnableExtensions(importPool *pgxpool.Pool) error {
	// Define a slice of strings containing the names of extensions to enable.
	extensions := []string{"pg_trgm", "vector"}

	for _, ext := range extensions {
		// `CREATE EXTENSION IF NOT EXISTS` is idempotent; it won't error if the extension already exists.
		query := fmt.Sprintf("CREATE EXTENSION IF NOT EXISTS %s;", ext)

		// Execute the query with a timeout.
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		// defer cancel() // Defer inside loop creates issues, cancel explicitly or at end
		// `cancel()` should be called after each `Exec` in a loop, or the context might expire prematurely for later iterations.

		_, err := importPool.Exec(ctx, query)
		cancel() // Cancel after exec
		if err != nil {
			return apperror.NewDatabaseError(fmt.Sprintf("failed to create extension %s", ext), err)
		}
	}

	return nil
}

// RunMigrations applies any pending database migrations from the specified migrations directory.
// It uses golang-migrate to handle migration versioning and execution.
//
// The migrations directory should contain SQL files named in the format:
// V{version}__{description}.sql (e.g., V1__create_users.sql)
// The function signature is RunMigrations(cfg *config.PoolConfig, migrationsPath string) error.
// It now takes PoolConfig to construct DSN for migrations, as pgxpool.Pool is not directly usable by golang-migrate's postgres driver.
func RunMigrations(cfg *config.PoolConfig, migrationsPath string) error {
	// Get the DSN suitable for `golang-migrate`.
	dsn := getDSN(cfg) // Use the DSN for migrations

	// Open a new sql.DB connection specifically for migrations using the DSN
	// golang-migrate's postgres driver expects a *sql.DB instance or a DSN it can use with lib/pq.
	// Since we're moving the main app to pgxpool, we'll use DSN for migrations.
	// Ensure "github.com/lib/pq" is imported (usually via migrate's postgres driver).
	m, err := migrate.New(
		// `file://` specifies that migrations are read from the local filesystem.
		"file://"+migrationsPath,
		dsn, // Pass DSN directly
	)
	if err != nil {
		return apperror.NewDatabaseError("failed to create migrator", err)
	}
	// `defer m.Close()` ensures that resources used by the `migrate` instance (like database connections
	// and file handles) are released when `RunMigrations` returns.
	// It's important to close the source and database instance that migrate creates.
	// m.Close() returns two errors, one for source and one for database.
	// We are only concerned about logging them if they occur.
	defer func() {
		if srcErr, dbErr := m.Close(); srcErr != nil || dbErr != nil {
			// Log or handle these errors appropriately. For now, just print.
			// In a real app, you'd use your logger.
			if srcErr != nil {
				fmt.Printf("Warning: error closing migration source: %v\n", srcErr)
			}
			if dbErr != nil {
				fmt.Printf("Warning: error closing migration database instance: %v\n", dbErr)
			}
		}
	}()

	// Apply all pending migrations
	// `m.Up()` applies all available "up" migrations.
	// `migrate.ErrNoChange` is returned if there are no new migrations to apply, which is not an actual error.
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return apperror.NewDatabaseError("failed to run migrations", err)
	}

	return nil
}