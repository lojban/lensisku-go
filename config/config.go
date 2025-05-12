// Package config provides configuration management for the lensisku application.
// It handles loading and validation of configuration values from environment variables,
// with support for required variables, default values, and collective error reporting.
// This is a crucial part of any application, allowing it to be configured for different
// environments (dev, staging, prod) without code changes.
// In Nest.js, the `@nestjs/config` module serves a similar purpose, often integrating
// with `.env` files and providing a `ConfigService`.
package config

import (
	"fmt"
	// `os` package provides operating system functionalities, like reading environment variables.
	"os"
	"strconv"
	"strings"
	"time"
)

// DatabasePools holds configuration for different database connection pools.
// This struct groups configurations for multiple database pools if needed.
type DatabasePools struct {
	AppPool    *PoolConfig
	ImportPool *PoolConfig
}

// PoolConfig represents configuration for a single database connection pool.
// PoolConfig represents configuration for a single database connection pool.
type PoolConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
	MaxSize  int
}

// AuthConfig holds authentication-related configuration.
type AuthConfig struct {
	JWTSecret            string        // Secret key for signing JWTs
	AccessTokenDuration  time.Duration // Duration for access tokens
	RefreshTokenDuration time.Duration // Duration for refresh tokens
}

// ServerConfig holds server-related configuration.
// For settings like the HTTP server port.
type ServerConfig struct {
	Port string // Port for the HTTP server
}

// AppConfig is the top-level configuration structure for the application.
type AppConfig struct {
	DBPools *DatabasePools
	Auth    *AuthConfig
	Server  *ServerConfig
}

// Helper function to get a required environment variable.
// Appends an error to the errors slice if the variable is not set.
// This promotes a "fail fast" approach for critical missing configurations.
func getRequiredEnv(key string, errors *[]string) string {
	value, exists := os.LookupEnv(key)
	if !exists {
		*errors = append(*errors, fmt.Sprintf("missing required environment variable: %s", key))
		return "" // Return empty string, error is collected
	}
	return value
}

// Helper function to get an optional environment variable with a default string value.
// Provides sensible defaults if an optional configuration is not explicitly set.
func getOptionalEnv(key string, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

// Helper function to get an optional environment variable parsed as an int.
// Uses defaultValue if not set or if parsing fails. Appends an error if parsing fails.
// Includes type conversion and error handling.
func getOptionalEnvInt(key string, defaultValue int, errors *[]string) int {
	valueStr, exists := os.LookupEnv(key)
	if !exists {
		return defaultValue
	}
	valueInt, err := strconv.Atoi(valueStr)
	if err != nil {
		*errors = append(*errors, fmt.Sprintf("invalid value for %s: expected integer, got '%s': %v", key, valueStr, err))
		return defaultValue // Return default, error is collected
	}
	return valueInt
}

// Helper function to get an optional environment variable parsed as time.Duration.
// Uses defaultValue if not set or if parsing fails. Appends an error if parsing fails.
// `time.ParseDuration` expects a string like "15m", "1h30s".
func getOptionalEnvDuration(key string, defaultValue time.Duration, errors *[]string) time.Duration {
	valueStr, exists := os.LookupEnv(key)
	if !exists {
		return defaultValue
	}
	valueDuration, err := time.ParseDuration(valueStr)
	if err != nil {
		*errors = append(*errors, fmt.Sprintf("invalid value for %s: expected duration string, got '%s': %v", key, valueStr, err))
		return defaultValue // Return default, error is collected
	}
	return valueDuration
}

// parseAndValidatePoolSize converts a string value to an integer, validates and clamps it.
// Appends an error to the errors slice if parsing or validation fails.
// This function ensures pool sizes are within reasonable bounds.
func parseAndValidatePoolSize(valueStr string, varName string, errors *[]string) int {
	if valueStr == "" { // Should be caught by getRequiredEnv if it's a required var
		*errors = append(*errors, fmt.Sprintf("missing value for pool size: %s", varName))
		return 5 // Default to min clamp value on missing, though getRequiredEnv should prevent this path for required.
	}
	size, err := strconv.Atoi(valueStr)
	if err != nil {
		*errors = append(*errors, fmt.Sprintf("invalid pool size for %s: expected integer, got '%s': %v", varName, valueStr, err))
		return 5 // Default to min clamp value on error
	}

	// Clamp the pool size between 5 and 100
	if size < 5 {
		*errors = append(*errors, fmt.Sprintf("pool size for %s (%d) is less than minimum 5, clamping to 5", varName, size))
		size = 5
	}
	if size > 100 {
		*errors = append(*errors, fmt.Sprintf("pool size for %s (%d) is greater than maximum 100, clamping to 100", varName, size))
		size = 100
	}
	return size
}

// LoadConfig creates and returns an AppConfig by reading and validating environment variables.
// It collects all errors encountered during loading and returns a single error if any exist.
// This is the main function of the package, orchestrating the loading of all configurations.
func LoadConfig() (*AppConfig, error) {
	// `errors` slice collects all validation/parsing errors during config loading.
	var errors []string

	// Database Configuration
	// Load individual database settings using the helper functions.
	dbUser := getRequiredEnv("DB_USER", &errors)
	dbPassword := getRequiredEnv("DB_PASSWORD", &errors)
	dbName := getRequiredEnv("DB_NAME", &errors)
	dbHost := getOptionalEnv("DB_HOST", "localhost")
	dbPort := getOptionalEnvInt("DB_PORT", 5432, &errors)

	dbAppPoolSizeStr := getRequiredEnv("DB_APP_POOL_SIZE", &errors)
	dbImportPoolSizeStr := getRequiredEnv("DB_IMPORT_POOL_SIZE", &errors)

	var appPoolSize, importPoolSize int
	// Only parse if the string was successfully retrieved (i.e., no "missing required" error for them yet)
	if dbAppPoolSizeStr != "" {
		appPoolSize = parseAndValidatePoolSize(dbAppPoolSizeStr, "DB_APP_POOL_SIZE", &errors)
	} else {
		// Ensure a value is set if getRequiredEnv added an error but returned empty
		// This path is mostly defensive as getRequiredEnv already logs the missing error.
		appPoolSize = 5 // Default to min clamp
	}
	if dbImportPoolSizeStr != "" {
		importPoolSize = parseAndValidatePoolSize(dbImportPoolSizeStr, "DB_IMPORT_POOL_SIZE", &errors)
	} else {
		importPoolSize = 5 // Default to min clamp
	}

	// Populate the DatabasePools struct.
	dbPools := &DatabasePools{
		AppPool: &PoolConfig{
			Host:     dbHost,
			Port:     dbPort,
			User:     dbUser,
			Password: dbPassword,
			DBName:   dbName,
			MaxSize:  appPoolSize,
		},
		ImportPool: &PoolConfig{
			Host:     dbHost,
			Port:     dbPort,
			User:     dbUser,
			Password: dbPassword,
			DBName:   dbName,
			MaxSize:  importPoolSize,
		},
	}

	// Auth Configuration
	jwtSecret := getRequiredEnv("JWT_SECRET", &errors)
	accessTokenDuration := getOptionalEnvDuration("JWT_ACCESS_TOKEN_DURATION", 15*time.Minute, &errors)
	refreshTokenDuration := getOptionalEnvDuration("JWT_REFRESH_TOKEN_DURATION", 168*time.Hour, &errors) // 7 days

	// Populate the AuthConfig struct.
	authConfig := &AuthConfig{
		JWTSecret:            jwtSecret,
		AccessTokenDuration:  accessTokenDuration,
		RefreshTokenDuration: refreshTokenDuration,
	}

	// Server Configuration
	serverPort := getOptionalEnv("PORT", "8080")
	serverConfig := &ServerConfig{
		// Note: Server port is typically a string because it's used directly in `net.Listen` (e.g., ":8080").
		Port: serverPort,
	}

	// If any errors were collected during loading, return a single aggregated error message.
	if len(errors) > 0 {
		return nil, fmt.Errorf("configuration errors:\n- %s", strings.Join(errors, "\n- "))
	}

	// Return the fully populated AppConfig.
	return &AppConfig{
		DBPools: dbPools,
		Auth:    authConfig,
		Server:  serverConfig,
	}, nil
}
