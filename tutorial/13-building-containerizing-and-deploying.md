# Chapter 13: Building, Containerizing, and Deploying the Go Application

In this chapter, we'll explore how to prepare our Go application for production deployment. We'll cover building optimized binaries, containerization with Docker, various deployment strategies, and implementing monitoring solutions. This knowledge will help you deploy and maintain your Go application reliably in production environments.

## Building Go Binaries

Go's build system provides powerful features for creating optimized, production-ready binaries. Unlike our Rust project that uses Cargo, Go uses the `go build` command with various flags to control the build process.

### Static Linking and Small Executables

One of Go's strengths is its ability to produce statically linked binaries that can run without external dependencies. Here's how to create an optimized binary:

```bash
# Basic build
go build -o server

# Build with optimizations
go build -ldflags="-w -s" -o server
```

The `-ldflags` flags:
- `-w`: Removes DWARF debugging information
- `-s`: Removes symbol table and debugging information

To make the binary even smaller, you can use UPX (Ultimate Packer for eXecutables):

```bash
upx --best --lzma server
```

### Cross-Compilation

Go excels at cross-compilation. Unlike Rust which requires additional toolchain setup, Go can cross-compile with just environment variables:

```bash
# Build for Linux on any platform
GOOS=linux GOARCH=amd64 go build -o server-linux

# Build for Windows
GOOS=windows GOARCH=amd64 go build -o server.exe

# Build for macOS
GOOS=darwin GOARCH=amd64 go build -o server-mac
```

### Build Tags

Build tags allow conditional compilation based on various factors. Create a file `production.go`:

```go
//go:build production
// +build production

package main

const environment = "production"
```

And a corresponding `development.go`:

```go
//go:build !production
// +build !production

package main

const environment = "development"
```

Build with tags:

```bash
go build -tags production
```

## Containerization with Docker

Docker is crucial for modern deployment workflows. Let's create an efficient Dockerfile using multi-stage builds.

### Writing Efficient Dockerfiles

Here's an example of a multi-stage Dockerfile for our application:

```dockerfile
# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o server

# Final stage
FROM alpine:3.18

WORKDIR /app
COPY --from=builder /app/server .
COPY config.yaml .

# Create non-root user
RUN adduser -D appuser
USER appuser

EXPOSE 8080
CMD ["./server"]
```

This Dockerfile:
1. Uses a multi-stage build to keep the final image small
2. Builds a statically linked binary
3. Runs as a non-root user for security
4. Only includes necessary files in the final image

### Managing Configuration

For containerized environments, consider these approaches for configuration:

1. Environment Variables:
```go
type Config struct {
    Port        string `env:"PORT" envDefault:"8080"`
    DatabaseURL string `env:"DATABASE_URL,required"`
}
```

2. Configuration Files:
- Mount as Docker volumes
- Include in the image (for non-sensitive data)
- Use Docker secrets for sensitive information

## Deployment Strategies

### Traditional VMs

For VM deployment:
1. Build the binary locally
2. Transfer to the VM
3. Set up systemd service:

```ini
[Unit]
Description=Mail Service
After=network.target

[Service]
Type=simple
User=appuser
WorkingDirectory=/opt/mail
ExecStart=/opt/mail/server
Restart=always

[Install]
WantedBy=multi-user.target
```

### Platform as a Service (PaaS)

For Heroku deployment, create a `Procfile`:

```
web: ./server
```

For Google App Engine, create `app.yaml`:

```yaml
runtime: go121
main: ./server

env_variables:
  PORT: "8080"
```

### Kubernetes Deployment

Basic Kubernetes manifests:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: mail-service
spec:
  replicas: 3
  selector:
    matchLabels:
      app: mail-service
  template:
    metadata:
      labels:
        app: mail-service
    spec:
      containers:
      - name: mail-service
        image: mail-service:latest
        ports:
        - containerPort: 8080
        env:
        - name: DATABASE_URL
          valueFrom:
            secretKeyRef:
              name: db-secrets
              key: url
```

### Serverless Deployment

For AWS Lambda, use the AWS Lambda Go runtime:

```go
package main

import (
    "github.com/aws/aws-lambda-go/lambda"
)

func handleRequest(event APIGatewayEvent) (Response, error) {
    // Handle request
    return Response{
        StatusCode: 200,
        Body:       "Hello from Lambda!",
    }, nil
}

func main() {
    lambda.Start(handleRequest)
}
```

## Monitoring and Observability

### Metrics with Prometheus

Add Prometheus metrics to your application:

```go
import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
)

var (
    requestsTotal = promauto.NewCounter(prometheus.CounterOpts{
        Name: "mail_http_requests_total",
        Help: "Total number of HTTP requests",
    })
)

// In your HTTP handler
func handler(w http.ResponseWriter, r *http.Request) {
    requestsTotal.Inc()
    // Handle request
}
```

### Distributed Tracing

Implement OpenTelemetry tracing:

```go
import (
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/trace"
)

func processRequest(ctx context.Context) error {
    tr := otel.Tracer("mail-service")
    ctx, span := tr.Start(ctx, "process-request")
    defer span.End()

    // Add attributes
    span.SetAttributes(attribute.String("user.id", userID))

    // Process request
    return nil
}
```

### Health Checks

Implement health check endpoints:

```go
func healthCheck(w http.ResponseWriter, r *http.Request) {
    status := struct {
        Status    string `json:"status"`
        Timestamp string `json:"timestamp"`
    }{
        Status:    "healthy",
        Timestamp: time.Now().UTC().Format(time.RFC3339),
    }
    json.NewEncoder(w).Encode(status)
}
```

Add Kubernetes liveness and readiness probes:

```yaml
livenessProbe:
  httpGet:
    path: /health
    port: 8080
  initialDelaySeconds: 3
  periodSeconds: 3
readinessProbe:
  httpGet:
    path: /ready
    port: 8080
  initialDelaySeconds: 5
  periodSeconds: 5
```

## Comparison with Rust Project

Our Rust project uses similar containerization approaches but with some key differences:

1. Build Process:
   - Rust uses Cargo for dependency management and building
   - Go uses modules and the built-in build system
   - Both support cross-compilation, but Go's approach is simpler

2. Docker Setup:
   - Both use multi-stage builds for optimization
   - Rust typically requires more build dependencies
   - Go's static linking makes containers simpler

3. Configuration:
   - Both projects use environment variables and configuration files
   - Both support runtime configuration through Docker environments

The principles of containerization and deployment remain similar between both languages, but Go's simpler tooling and built-in static linking can make the process more straightforward.