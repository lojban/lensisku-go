# Chapter 10: Handling Background Tasks and Asynchronous Operations

In this chapter, we'll explore how to implement background tasks and asynchronous operations in our Go implementation, drawing parallels with the Rust project's background task handling system. We'll learn how to leverage Go's powerful concurrency primitives and design patterns to create robust, efficient background processing capabilities.

## Understanding the Original Implementation

In the Rust project, background tasks are managed through the `background` module, specifically in `src/background/service.rs`. The main function `spawn_background_tasks` initializes several concurrent tasks:

1. Email import from Maildir
2. Embedding calculations (running hourly)
3. Muplis data updates (running daily)
4. New email checks (running daily)
5. Email notification processing
6. Dictionary export caching (running at midnight)

Each task runs in its own async context using Tokio's runtime, with proper error handling and logging.

## Implementing Background Jobs in Go

### Setting Up the Background Package

First, let's create a similar structure in our Go implementation:

```go
// pkg/background/service.go

package background

import (
    "context"
    "log"
    "sync"
    "time"

    "github.com/yourorg/lojban-mail/pkg/config"
    "github.com/yourorg/lojban-mail/pkg/db"
)

// BackgroundService manages all background tasks
type BackgroundService struct {
    db          *db.Pool
    mailDirPath string
    wg          sync.WaitGroup
    shutdown    chan struct{}
}

// New creates a new background service
func New(db *db.Pool, mailDirPath string) *BackgroundService {
    return &BackgroundService{
        db:          db,
        mailDirPath: mailDirPath,
        shutdown:    make(chan struct{}),
    }
}
```

### Implementing Worker Pools

Go's goroutines and channels make it easy to implement worker pools. Here's a generic worker pool implementation:

```go
// pkg/background/worker.go

package background

import "sync"

// WorkerPool manages a pool of workers
type WorkerPool struct {
    workers  int
    jobs     chan func()
    wg       sync.WaitGroup
    shutdown chan struct{}
}

func NewWorkerPool(workers int) *WorkerPool {
    return &WorkerPool{
        workers:  workers,
        jobs:     make(chan func()),
        shutdown: make(chan struct{}),
    }
}

func (p *WorkerPool) Start() {
    for i := 0; i < p.workers; i++ {
        p.wg.Add(1)
        go func() {
            defer p.wg.Done()
            for {
                select {
                case job, ok := <-p.jobs:
                    if !ok {
                        return
                    }
                    job()
                case <-p.shutdown:
                    return
                }
            }
        }()
    }
}
```

### Task Scheduling and Management

For scheduling tasks, we can use the excellent `robfig/cron` package, which provides a flexible cron-like scheduler:

```go
// pkg/background/scheduler.go

package background

import (
    "github.com/robfig/cron/v3"
    "log"
)

func (s *BackgroundService) setupScheduledTasks() {
    scheduler := cron.New(cron.WithSeconds())
    
    // Email checks - daily
    scheduler.AddFunc("0 0 0 * * *", func() {
        if err := s.checkNewEmails(); err != nil {
            log.Printf("Error checking emails: %v", err)
        }
    })

    // Embedding calculations - hourly
    scheduler.AddFunc("0 0 * * * *", func() {
        if err := s.calculateMissingEmbeddings(); err != nil {
            log.Printf("Error calculating embeddings: %v", err)
        }
    })

    scheduler.Start()
}
```

### Error Handling and Retries

Implementing robust error handling and retries is crucial for background tasks. Here's a pattern for resilient task execution:

```go
// pkg/background/retry.go

package background

import (
    "time"
    "errors"
)

type RetryConfig struct {
    MaxAttempts int
    InitialDelay time.Duration
    MaxDelay time.Duration
}

func withRetry(operation func() error, config RetryConfig) error {
    var lastErr error
    delay := config.InitialDelay

    for attempt := 0; attempt < config.MaxAttempts; attempt++ {
        if err := operation(); err == nil {
            return nil
        } else {
            lastErr = err
            time.Sleep(delay)
            delay *= 2
            if delay > config.MaxDelay {
                delay = config.MaxDelay
            }
        }
    }

    return errors.New("max retries exceeded: " + lastErr.Error())
}
```

### Main Service Implementation

Here's how we tie everything together:

```go
// pkg/background/service.go

func (s *BackgroundService) Start(ctx context.Context) error {
    // Initialize worker pool
    pool := NewWorkerPool(5)
    pool.Start()

    // Start scheduled tasks
    s.setupScheduledTasks()

    // Initial import
    go func() {
        if err := s.importMaildir(); err != nil {
            log.Printf("Error in initial maildir import: %v", err)
        }
    }()

    // Handle graceful shutdown
    <-ctx.Done()
    close(s.shutdown)
    pool.wg.Wait()
    return nil
}
```

## Integration with External Systems

For distributed task queues, we can integrate with systems like Redis or RabbitMQ:

```go
// pkg/background/queue/redis.go

package queue

import (
    "context"
    "encoding/json"
    "github.com/go-redis/redis/v8"
)

type RedisQueue struct {
    client *redis.Client
}

func (q *RedisQueue) EnqueueTask(ctx context.Context, task Task) error {
    payload, err := json.Marshal(task)
    if err != nil {
        return err
    }
    return q.client.LPush(ctx, "tasks", payload).Err()
}
```

## Comparison with Rust's Async Runtime

While Rust's Tokio provides an async runtime with features like `spawn`, `select`, and `JoinHandle`, Go's approach is different but equally powerful:

1. **Goroutines vs Tokio Tasks**:
   - Rust: `tokio::spawn(async move { ... })`
   - Go: `go func() { ... }()`

2. **Channel Operations**:
   - Rust: `tokio::sync::mpsc`
   - Go: Native `chan` type

3. **Timer Operations**:
   - Rust: `tokio::time::interval`
   - Go: `time.Ticker`

The main difference is that Go's concurrency primitives are built into the language and runtime, while Rust relies on external async runtimes like Tokio.

## Best Practices and Considerations

1. **Graceful Shutdown**:
   - Always implement proper shutdown handling
   - Use context cancellation for coordination
   - Wait for ongoing tasks to complete

2. **Resource Management**:
   - Monitor goroutine counts
   - Implement rate limiting
   - Use connection pools for databases

3. **Error Handling**:
   - Implement comprehensive logging
   - Use structured error types
   - Consider retry strategies

4. **Monitoring and Observability**:
   - Add metrics for task execution
   - Monitor queue lengths
   - Track execution times

## Summary

In this chapter, we've learned how to implement robust background processing in Go, translating the functionality from the Rust project while leveraging Go's native concurrency features. We've covered worker pools, task scheduling, error handling, and integration with external systems.

The Go implementation provides a clean, efficient way to handle background tasks while maintaining the reliability and functionality of the original Rust implementation. The use of goroutines and channels, combined with the `robfig/cron` package for scheduling, offers a powerful and idiomatic solution for background processing in Go.