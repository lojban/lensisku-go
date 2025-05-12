# Lensisku-Go Clone

This is a Go implementation of the Lensisku API server, providing authentication and dictionary services.

## Prerequisites

- Go 1.x installed
- PostgreSQL server running
- Git (for cloning the repository)

## Environment Setup

The application uses environment variables for configuration. Create a `.env` file in the project root with the following variables:

### Required Environment Variables

```env
DB_USER=your_db_user
DB_PASSWORD=your_db_password
DB_NAME=your_db_name
DB_APP_POOL_SIZE=10
DB_IMPORT_POOL_SIZE=5
JWT_SECRET=your_jwt_secret_key
```

### Optional Environment Variables (with defaults)

```env
DB_HOST=localhost
DB_PORT=5432
JWT_ACCESS_TOKEN_DURATION=15m
JWT_REFRESH_TOKEN_DURATION=168h
PORT=8080
```

Note: Make sure to add `.env` to your `.gitignore` file to avoid committing sensitive information.

### Environment Variable Details

- **Database Configuration:**
  - `DB_USER`: PostgreSQL database user
  - `DB_PASSWORD`: PostgreSQL database password
  - `DB_NAME`: Database name
  - `DB_HOST`: Database host (default: "localhost")
  - `DB_PORT`: Database port (default: 5432)
  - `DB_APP_POOL_SIZE`: Connection pool size for app queries (min: 5, max: 100)
  - `DB_IMPORT_POOL_SIZE`: Connection pool size for import operations (min: 5, max: 100)

- **JWT Configuration:**
  - `JWT_SECRET`: Secret key for signing JWT tokens
  - `JWT_ACCESS_TOKEN_DURATION`: Access token duration (default: 15 minutes)
  - `JWT_REFRESH_TOKEN_DURATION`: Refresh token duration (default: 7 days)

- **Server Configuration:**
  - `PORT`: HTTP server port (default: 8080)

## Running the Application

From the project directory:

```bash
go run main.go
```

The server will start on the configured port (default: 8080).

## Testing Endpoints

### User Registration

```bash
curl -X POST http://localhost:8080/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "username": "testuser",
    "email": "test@example.com",
    "password": "your_password"
  }'
```

### User Login

```bash
curl -X POST http://localhost:8080/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "username": "testuser",
    "password": "your_password"
  }'
```

On successful login, you'll receive an access token and refresh token in the response.

## API Documentation (Swagger)

The API documentation is available through Swagger UI, which provides an interactive interface to explore and test the API endpoints.

### Accessing Swagger UI

The Swagger UI is accessible at:
```
http://localhost:8080/swagger/
```

Replace the port number (8080) with your configured server port if you're using a different one.

### Swagger JSON

The raw Swagger JSON definition is available at:
```
http://localhost:8080/swagger/doc.json
```

### Updating Documentation

When making changes to API annotations in the code, you need to regenerate the Swagger documentation by running:
```bash
go run github.com/swaggo/swag/cmd/swag init
```

This will update the `docs/swagger.json` and `docs/swagger.yaml` files with the latest API specifications.

## Application Architecture and Concepts

This section provides an overview of the project's structure and core concepts, drawing comparisons to Nest.js where applicable.

### Directory Structure

The project follows a modular structure, organizing code by feature or domain. This is conceptually similar to modules in Nest.js.

-   **/auth**: Contains all logic related to authentication and authorization, including user registration, login, token generation (JWT), and validation.
    -   **Nest.js Analogy**: Corresponds to an `AuthModule` containing services, controllers, DTOs, and entities for authentication.
-   **/users**: Manages user profile information.
    -   **Nest.js Analogy**: Similar to a `UsersModule` for user-specific operations.
-   **/comments**: Handles all functionalities related to comments (creating, retrieving, managing likes, etc.).
    -   **Nest.js Analogy**: Akin to a `CommentsModule`.
-   **/config**: Responsible for loading and managing application configuration from environment variables.
    -   **Nest.js Analogy**: Similar to using `@nestjs/config` and a `ConfigService`.
-   **/db**: Manages database connectivity (using `pgxpool` for PostgreSQL) and schema migrations (using `golang-migrate`).
    -   **Nest.js Analogy**: Corresponds to a database module setup, like `TypeOrmModule` or `MongooseModule`, which provides database connection/ORM instances.
-   **/apperror**: Defines custom error types and a centralized system for consistent error handling across the application.
    -   **Nest.js Analogy**: Conceptually similar to Nest.js's Exception Filters, which catch specific error types and customize HTTP responses.
-   **/background**: Contains services and tasks that run in the background, independently of direct HTTP requests (e.g., `EmbeddingCalculatorService`).
    -   **Nest.js Analogy**: Similar to using `@nestjs/schedule` for cron jobs or integrating with message queues (like BullMQ) for task processing.
-   **/jbovlaste**: Appears to handle specific domain logic related to "jbovlaste", possibly involving Server-Sent Events (SSE) for real-time communication via a `Broadcaster`.
    -   **Nest.js Analogy**: Could be part of a module handling real-time updates, perhaps using SSE, WebSockets (Gateways), or integrating with a message broker.
-   **/docs**: Contains auto-generated Swagger/OpenAPI documentation files.
    -   **Nest.js Analogy**: Similar to the output generated by `@nestjs/swagger` based on decorators in controllers and DTOs.
-   **main.go**: The main entry point of the application. It initializes configurations, database connections, services, handlers, sets up the HTTP router (Chi) and middleware, and starts the HTTP server. It also handles graceful shutdown.
    -   **Nest.js Analogy**: Similar to `main.ts` where the Nest application instance is created, modules are configured, middleware is applied, and the application is bootstrapped.

### Core Concepts

#### 1. Handlers (Controllers)

-   **In this Go Project**:
    -   Handlers are typically structs with methods that accept `http.ResponseWriter` and `*http.Request` as arguments. They are responsible for parsing incoming HTTP requests, validating input (often by decoding into DTOs), calling appropriate service methods to execute business logic, and formatting the HTTP response (e.g., writing JSON data or errors).
    -   They are found in files like `auth/handlers.go`, `users/handlers.go`.
    -   Routing is defined in `main.go` using the Chi router, mapping URL paths and HTTP methods to these handler methods.
-   **Nest.js Analogy**:
    -   Controllers are classes decorated with `@Controller('base-path')`. Methods within these classes are decorated with HTTP method decorators (e.g., `@Get()`, `@Post('/:id')`) to define routes.
    -   They use DTOs (often with validation pipes) for request payloads and inject services to delegate business logic.

#### 2. DTOs (Data Transfer Objects)

-   **In this Go Project**:
    -   DTOs are Go structs used to define the structure of data for API request bodies and response payloads. Examples include `auth.RegisterRequest` or `users.UserProfileResponse`.
    -   They are defined in files like `auth/dto.go`, `users/dto.go`.
    -   JSON serialization and deserialization are controlled by struct tags (e.g., `json:"username"`). Validation is often performed manually within handlers or service layers, or could be integrated with third-party validation libraries.
-   **Nest.js Analogy**:
    -   DTOs are typically classes. They are heavily used with `class-validator` and `class-transformer` for automatic request payload validation (via ValidationPipes) and response serialization.

#### 3. Services

-   **In this Go Project**:
    -   Services are structs that encapsulate the core business logic of the application (e.g., `auth.AuthService`, `users.UserService`). They are responsible for operations like interacting with the database (via the injected database pool), performing calculations, and enforcing business rules.
    -   They are defined in files like `auth/service.go`, `users/service.go`.
    -   Dependencies (like database pools or other services) are manually injected, typically through constructor functions (e.g., `NewAuthService(dbPool, authConfig)`).
-   **Nest.js Analogy**:
    -   Services are classes decorated with `@Injectable()`. They contain the business logic and are managed by Nest's Dependency Injection (DI) container. Services are injected into controllers or other services using constructor injection.

#### 4. Database Connection and Interaction

-   **In this Go Project**:
    -   The `db` package (`db/db.go`) is responsible for establishing and managing database connections. It uses `jackc/pgx/v5` (specifically `pgxpool` for connection pooling) to interact with the PostgreSQL database.
    -   Configuration for database connections (host, port, user, password, pool size) is loaded via the `config` package.
    -   The initialized database pool (`*pgxpool.Pool`) is then passed (injected) into service structs that require database access.
    -   Database schema migrations are handled using the `golang-migrate` library, with migration files typically stored in a `/migrations` directory (though currently disabled in `main.go`).
-   **Nest.js Analogy**:
    -   Database integration is commonly managed through dedicated modules like `@nestjs/typeorm` (for TypeORM) or `@nestjs/mongoose` (for Mongoose). These modules handle connection setup based on configuration and make ORM repositories or database connection objects available for injection into services. Migrations are often handled by the ORM's built-in mechanisms.

#### 5. Modularity (Packages)

-   **In this Go Project**:
    -   The application is organized into packages (directories), where each package groups related functionality (e.g., `auth` for authentication, `comments` for comment logic). This promotes separation of concerns.
    -   There isn't an explicit module system with decorators like in Nest.js, but the package structure serves a similar organizational purpose. Dependencies between packages are managed through Go's import system.
-   **Nest.js Analogy**:
    -   Applications are built from modules, defined by classes decorated with `@Module()`. Modules encapsulate controllers, services, providers, and can import other modules, creating a clear dependency graph managed by the Nest DI container.

#### 6. Middleware

-   **In this Go Project**:
    -   Middleware are functions that process HTTP requests before they reach the main handler or after the handler has processed them. They are used for cross-cutting concerns like logging, panic recovery, CORS handling, and authentication.
    -   The Chi router provides its own set of common middleware (e.g., `middleware.Logger`, `middleware.Recoverer`). Custom middleware, like `auth.JWTMiddleware`, is also implemented.
    -   Middleware is applied either globally to all routes or to specific route groups in `main.go` using `r.Use(...)`.
-   **Nest.js Analogy**:
    -   Nest.js has a robust middleware system. Middleware can be simple functions or classes implementing the `NestMiddleware` interface. They can be applied globally, to specific modules, or to individual routes.
    -   Additionally, Nest.js offers Guards (for authorization, implementing `CanActivate`), Interceptors (for transforming request/response data, logging, caching, implementing `NestInterceptor`), and Pipes (for data transformation and validation, implementing `PipeTransform`), which cover a broader range of request-processing scenarios.

#### 7. Configuration Management

-   **In this Go Project**:
    -   The `config` package (`config/config.go`) centralizes configuration loading. It reads environment variables (using `github.com/joho/godotenv` to load `.env` files during development) and populates a typed `AppConfig` struct.
    -   Helper functions within the package handle required vs. optional variables, default values, and type parsing (e.g., string to int or time.Duration).
-   **Nest.js Analogy**:
    -   The `@nestjs/config` module is widely used. It provides a `ConfigService` that can be injected into other services or modules to access configuration variables loaded from environment variables, `.env` files, or other sources. It supports schema validation for configuration.

#### 8. Error Handling

-   **In this Go Project**:
    -   The `apperror` package (`apperror/apperror.go`) defines a custom `AppError` struct and a set of predefined error types (e.g., `NotFoundError`, `AuthError`). This allows for standardized error creation and handling.
    -   Services return these custom errors. Handlers (or a centralized error handling middleware/utility like `auth.WriteError`) then convert these `AppError` instances into appropriate HTTP status codes and JSON error responses.
-   **Nest.js Analogy**:
    -   Nest.js uses Exception Filters. These are classes decorated with `@Catch()` that can catch specific types of exceptions (or all exceptions) thrown during request processing. They allow developers to customize the error response sent to the client. Nest provides a base exception filter and allows for custom implementations.

#### 9. Background Tasks

-   **In this Go Project**:
    -   The `background` package demonstrates how to run tasks asynchronously from the main HTTP request-response cycle. The `EmbeddingCalculatorService` is an example, using goroutines, channels, and `sync.WaitGroup` for concurrent processing and graceful shutdown.
    -   This is suitable for long-running operations, periodic jobs, or offloading work that doesn't need to block the user's request.
-   **Nest.js Analogy**:
    -   Nest.js offers several ways to handle background tasks:
        -   `@nestjs/schedule`: For cron-like scheduled jobs (e.g., running a task every hour).
        -   Queueing systems: Integrating with libraries like BullMQ for robust, distributed task queues, allowing tasks to be processed by separate worker processes.
        -   Asynchronous services: Standard JavaScript/TypeScript async/await patterns can be used within services for non-blocking I/O operations.

This comparison should help in understanding the Go project's structure and design patterns, especially for those familiar with Nest.js. While the implementation details differ due to language and framework paradigms, many core architectural principles are shared.
