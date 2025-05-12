# Chapter 12: Testing Your Go Application

When migrating from Rust to Go, you'll find that while both languages prioritize testing as a first-class concern, their approaches differ in implementation details and conventions. This chapter explores Go's testing methodologies, contrasting them with Rust's approaches where relevant, to help you adapt your testing strategies effectively.

## Go's Built-in Testing Package (`testing`)

Unlike Rust's attribute-based testing system (`#[cfg(test)]`), Go takes a more straightforward approach with its `testing` package. Tests in Go are regular functions that reside in files with names ending in `_test.go`. These files can exist alongside your production code or in a separate package with a `_test` suffix.

### Writing Unit Tests

Here's a basic example of a Go test:

```go
// user_service_test.go
package service

import "testing"

func TestCreateUser(t *testing.T) {
    srv := NewUserService(mockDB)
    user, err := srv.CreateUser("test@example.com", "password")
    
    if err != nil {
        t.Errorf("expected no error, got %v", err)
    }
    if user.Email != "test@example.com" {
        t.Errorf("expected email %q, got %q", "test@example.com", user.Email)
    }
}
```

This contrasts with Rust's approach:

```rust
// In Rust, you might write:
#[cfg(test)]
mod tests {
    use super::*;
    
    #[test]
    fn test_create_user() {
        let srv = UserService::new(mock_db());
        let user = srv.create_user("test@example.com", "password")
            .expect("should create user");
        assert_eq!(user.email, "test@example.com");
    }
}
```

#### Table-Driven Tests

A powerful idiom in Go testing is table-driven tests, which allow you to express multiple test cases concisely:

```go
func TestUserValidation(t *testing.T) {
    tests := []struct {
        name     string
        email    string
        password string
        wantErr  bool
    }{
        {
            name:     "valid credentials",
            email:    "user@example.com",
            password: "secure123",
            wantErr:  false,
        },
        {
            name:     "invalid email",
            email:    "not-an-email",
            password: "secure123",
            wantErr:  true,
        },
        // Add more test cases...
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            _, err := validateCredentials(tt.email, tt.password)
            if (err != nil) != tt.wantErr {
                t.Errorf("validateCredentials() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

### Test Helper Functions

Go provides a clean way to create helper functions for tests:

```go
func setupTestDB(t *testing.T) *sql.DB {
    t.Helper() // Marks this as a helper function
    db, err := sql.Open("postgres", "postgres://test:test@localhost:5432/testdb?sslmode=disable")
    if err != nil {
        t.Fatalf("failed to connect to test database: %v", err)
    }
    return db
}
```

### Mocking Dependencies

Go's interface system makes mocking straightforward without requiring external libraries (though they exist). Here's a comparison of approaches:

#### Manual Mocks with Interfaces

```go
// Define the interface
type UserRepository interface {
    CreateUser(email, password string) (*User, error)
    GetUserByID(id int) (*User, error)
}

// Create a mock implementation
type mockUserRepository struct {
    users map[int]*User
}

func (m *mockUserRepository) CreateUser(email, password string) (*User, error) {
    // Mock implementation
    return &User{ID: 1, Email: email}, nil
}
```

#### Using Mock Libraries

While manual mocks are common, libraries like `gomock` provide more sophisticated mocking capabilities:

```go
//go:generate mockgen -destination=mocks/mock_repository.go -package=mocks . UserRepository

func TestUserService_WithMockgen(t *testing.T) {
    ctrl := gomock.NewController(t)
    defer ctrl.Finish()

    mockRepo := mocks.NewMockUserRepository(ctrl)
    mockRepo.EXPECT().
        CreateUser(gomock.Any(), gomock.Any()).
        Return(&User{ID: 1}, nil)

    service := NewUserService(mockRepo)
    // Test the service...
}
```

## Integration Testing

Integration tests in Go often involve testing against real dependencies or their close approximations. Here's how you might test database interactions:

```go
func TestUserRepository_Integration(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping integration test")
    }

    pool, err := dockertest.NewPool("")
    if err != nil {
        t.Fatalf("Could not connect to docker: %v", err)
    }

    // Start a PostgreSQL container
    resource, err := pool.Run("postgres", "13", []string{
        "POSTGRES_PASSWORD=secret",
        "POSTGRES_DB=testdb",
    })
    if err != nil {
        t.Fatalf("Could not start resource: %v", err)
    }

    // Clean up after the test
    defer pool.Purge(resource)

    // Run tests...
}
```

## End-to-End (E2E) Testing

For web services, E2E tests often involve starting your HTTP server and making real requests:

```go
func TestAPI_E2E(t *testing.T) {
    app := NewApp()
    ts := httptest.NewServer(app.Handler())
    defer ts.Close()

    // Make HTTP requests to test endpoints
    resp, err := http.Post(
        ts.URL+"/api/users",
        "application/json",
        strings.NewReader(`{"email":"test@example.com","password":"secret"}`),
    )
    if err != nil {
        t.Fatalf("failed to make request: %v", err)
    }
    // Assert response...
}
```

## Benchmarking

Go's testing package includes built-in support for benchmarking:

```go
func BenchmarkUserValidation(b *testing.B) {
    email := "test@example.com"
    password := "secure123"
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        validateCredentials(email, password)
    }
}
```

Run benchmarks using:
```bash
go test -bench=. ./...
```

## Code Coverage

Go provides built-in code coverage analysis:

```bash
# Run tests with coverage
go test -cover ./...

# Generate coverage profile
go test -coverprofile=coverage.out ./...

# View coverage in browser
go tool cover -html=coverage.out
```

Unlike Rust's `grcov` or similar tools, Go's coverage tooling is built into the standard toolchain and requires no external dependencies.

## Best Practices and Tips

1. **Organization**:
   - Keep test files next to the code they test
   - Use clear, descriptive test names
   - Leverage table-driven tests for comprehensive test cases

2. **Test Isolation**:
   - Each test should run independently
   - Use subtests (`t.Run()`) to group related tests
   - Clean up resources in defer statements

3. **Mock vs. Real Dependencies**:
   - Prefer interfaces for external dependencies
   - Use real databases for integration tests when possible
   - Consider using `dockertest` for isolated integration testing

4. **Performance**:
   - Use benchmarks to catch performance regressions
   - Profile your tests if they're running slowly
   - Use `testing.Short()` to skip long-running tests

Remember that while Rust and Go have different testing idioms, the underlying principles remain the same. Focus on writing clear, maintainable tests that give you confidence in your code's correctness.