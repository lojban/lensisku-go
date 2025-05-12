# Chapter 5: Implementing Core Business Logic - General Principles

This chapter explores how to effectively structure and implement business logic in Go, using our Rust project as a reference point for comparison. We'll examine architectural patterns, module organization, and concurrency approaches that leverage Go's strengths while maintaining clean separation of concerns.

## Structuring Business Logic in Go: Service Layer and Repository Pattern

In our Rust implementation, business modules like `auth`, `users`, and `collections` follow a clear separation of concerns with distinct files for controllers, services, and models. Let's translate this architecture to Go while embracing Go's idioms.

### Defining Interfaces for Business Logic Components

In Go, we express contracts through interfaces. Here's how we might structure our authentication service:

```go
// domain/auth/interfaces.go

type AuthService interface {
    Login(ctx context.Context, credentials LoginCredentials) (*AuthToken, error)
    Register(ctx context.Context, input RegistrationInput) (*User, error)
    VerifyEmail(ctx context.Context, token string) error
    // ... other auth operations
}

type AuthRepository interface {
    FindUserByEmail(ctx context.Context, email string) (*User, error)
    SaveUser(ctx context.Context, user *User) error
    // ... other data access methods
}
```

This interface-based approach provides several benefits:
1. Clear contract definition between components
2. Easy mocking for tests
3. Flexibility to change implementations without affecting dependent code

### Dependency Injection Patterns

Go favors explicit dependency injection over complex DI containers. Here's our recommended approach:

```go
// domain/auth/service.go

type authService struct {
    repo       AuthRepository
    mailClient MailClient
    config     *Config
}

func NewAuthService(repo AuthRepository, mailClient MailClient, config *Config) AuthService {
    return &authService{
        repo:       repo,
        mailClient: mailClient,
        config:     config,
    }
}
```

While DI containers like `uber-go/dig` or `google/wire` exist, manual dependency injection is often clearer and more maintainable for medium-sized projects. It makes dependencies explicit and keeps control flow easy to follow.

## Translating Rust Modules to Go Packages

Our Rust project uses a hierarchical module system with `mod.rs` files. Here's how we'll map this to Go's package structure:

```
domain/
├── auth/
│   ├── interfaces.go    // Contracts (equivalent to Rust traits)
│   ├── service.go       // Business logic implementation
│   ├── repository.go    // Data access implementation
│   ├── models.go        // Domain models
│   └── dto.go          // Data transfer objects
├── users/
│   ├── interfaces.go
│   ├── service.go
│   └── ...
└── collections/
    ├── interfaces.go
    ├── service.go
    └── ...
```

Key differences from Rust:
- No need for explicit module registration (no `mod.rs`)
- Visibility controlled by capitalization (exported vs. unexported)
- Package names match directory names
- One package per directory (flat package hierarchy)

## Data Transfer Objects (DTOs) vs. Domain Models

In Go, we separate our domain models from DTOs using distinct types:

```go
// domain/auth/models.go
type User struct {
    ID             uint64
    Email          string
    HashedPassword []byte
    Verified       bool
    // ... other domain-specific fields
}

// domain/auth/dto.go
type RegistrationInput struct {
    Email           string `json:"email" validate:"required,email"`
    Password        string `json:"password" validate:"required,min=8"`
    ConfirmPassword string `json:"confirm_password" validate:"required,eqfield=Password"`
}
```

For validation, we recommend using `go-playground/validator`:

```go
func (s *authService) Register(ctx context.Context, input RegistrationInput) (*User, error) {
    if err := s.validator.Struct(input); err != nil {
        return nil, fmt.Errorf("invalid input: %w", err)
    }
    
    // Transform DTO to domain model
    user := &User{
        Email: input.Email,
        // ... handle password hashing, etc.
    }
    
    // Continue with business logic
    return user, nil
}
```

## Concurrency Patterns for Business Logic

Go's concurrency primitives offer powerful tools for handling parallel operations. Here's how we can leverage them in our business logic:

### Goroutines and Channels for Parallel Operations

```go
type emailService struct {
    mailQueue chan *EmailJob
    workers   int
}

func NewEmailService(workers int) *emailService {
    svc := &emailService{
        mailQueue: make(chan *EmailJob, 100),
        workers:   workers,
    }
    svc.startWorkers()
    return svc
}

func (s *emailService) startWorkers() {
    for i := 0; i < s.workers; i++ {
        go s.worker()
    }
}

func (s *emailService) worker() {
    for job := range s.mailQueue {
        // Process email job
        s.processEmail(job)
    }
}
```

### Synchronization When Needed

While channels are great for communication, sometimes direct synchronization is clearer:

```go
type UserCache struct {
    mu    sync.RWMutex
    cache map[string]*User
}

func (c *UserCache) Get(id string) (*User, bool) {
    c.mu.RLock()
    defer c.mu.RUnlock()
    user, exists := c.cache[id]
    return user, exists
}

func (c *UserCache) Set(id string, user *User) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.cache[id] = user
}
```

Key concurrency patterns to consider:
1. Use channels for communication between goroutines
2. Use mutexes for protecting shared state
3. Use `sync.WaitGroup` for waiting on multiple goroutines
4. Consider using `context.Context` for cancellation

## Summary

When implementing business logic in Go:
- Use interfaces to define clear contracts between components
- Prefer explicit dependency injection
- Organize code into packages that match your domain concepts
- Separate DTOs from domain models
- Leverage Go's concurrency primitives appropriately

In the next chapter, we'll dive into implementing specific business logic components from our application, showing these patterns in action with real-world examples.