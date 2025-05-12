# Chapter 9: API Documentation with OpenAPI/Swagger

In this chapter, we'll explore how to document our Go API using OpenAPI (formerly known as Swagger) specifications. We'll examine various tools and approaches available in the Go ecosystem and show how to translate our Rust project's API documentation setup into equivalent Go implementations.

## Introduction to OpenAPI Documentation in Go

API documentation is crucial for both internal development and external API consumers. The OpenAPI specification (OAS) provides a standardized way to describe RESTful APIs. In our Rust project, we used `utoipa` for generating OpenAPI documentation. Let's explore the Go equivalents and how to implement similar functionality.

## Popular Tools for OpenAPI in Go

Several mature tools exist in the Go ecosystem for working with OpenAPI specifications:

1. **swaggo/swag** - Most popular choice for annotation-based API documentation
   ```go
   // Example swaggo annotation
   // @Summary Create user
   // @Description Create a new user
   // @Tags users
   // @Accept json
   // @Produce json
   // @Param user body UserCreateRequest true "User creation request"
   // @Success 200 {object} UserResponse
   // @Router /users [post]
   func (h *Handler) CreateUser(c *gin.Context) {
       // Handler implementation
   }
   ```

2. **go-swagger** - Swiss army knife for OpenAPI, supporting both spec-first and code-first approaches
   ```go
   // go-swagger annotation example
   // swagger:route POST /users users createUser
   // Creates a new user.
   // responses:
   //   200: userResponse
   //   400: errorResponse
   func (h *Handler) CreateUser(w http.ResponseWriter, r *http.Request) {
       // Handler implementation
   }
   ```

3. **ogen** - Modern spec-first generator for clients and servers
   ```yaml
   # ogen uses OpenAPI spec files directly
   openapi: 3.0.3
   paths:
     /users:
       post:
         operationId: createUser
         requestBody:
           content:
             application/json:
               schema:
                 $ref: '#/components/schemas/UserCreateRequest'
   ```

For our project, we'll use `swaggo/swag` as it provides the closest parallel to our Rust project's annotation-based approach with `utoipa`.

## Translating the Rust Documentation Setup

Let's examine how to translate our Rust project's OpenAPI configuration to Go. First, recall our Rust setup:

```rust
// api_docs.rs in Rust
use utoipa::openapi::security::{ApiKey, ApiKeyValue, SecurityScheme};

pub struct ApiModifier;

impl utoipa::Modify for ApiModifier {
    fn modify(&self, openapi: &mut OpenApi) {
        if let Some(components) = &mut openapi.components {
            components.add_security_scheme(
                "bearer_auth",
                SecurityScheme::ApiKey(ApiKey::Header(ApiKeyValue::new("Authorization"))),
            );
        }
    }
}
```

Here's the equivalent Go implementation using `swaggo/swag`:

```go
// docs/docs.go
package docs

import "github.com/swaggo/swag"

func init() {
    swag.Register(swag.Name, &swag.Spec{
        Title:       "Lojban Dictionary API",
        Description: "API for the Lojban dictionary and related services",
        Version:     "1.0",
        SecurityDefinitions: map[string]map[string]any{
            "bearer_auth": {
                "type": "apiKey",
                "name": "Authorization",
                "in":   "header",
            },
        },
    })
}
```

For our API tags structure (which was in `openapi.rs` in Rust):

```go
// main.go or docs/docs.go
// @title Lojban Dictionary API
// @version 1.0
// @description API for the Lojban dictionary and related services

// @tag.name auth
// @tag.description Authentication endpoints

// @tag.name users
// @tag.description Users endpoints

// @tag.name comments
// @tag.description Discussions endpoints

// @tag.name language
// @tag.description Linguistics-related endpoints

// @tag.name muplis
// @tag.description Muplis search endpoints
// ... additional tags ...
```

## Documenting Data Transfer Objects (DTOs)

In our Rust project, we used component schemas in the OpenAPI definition. Here's how to document DTOs in Go:

```go
// dto/comments.go
// @Description Query parameters for listing comments
type ListCommentsQuery struct {
    // @Description Page number for pagination
    // @Example 1
    Page int `json:"page" example:"1"`
    
    // @Description Number of items per page
    // @Example 20
    PageSize int `json:"page_size" example:"20"`
    
    // @Description Optional thread ID filter
    ThreadID *string `json:"thread_id,omitempty"`
}

// dto/definitions.go
// @Description Query parameters for listing definitions
type ListDefinitionsQuery struct {
    // @Description Language code filter
    // @Example "en"
    Lang string `json:"lang" example:"en"`
    
    // @Description Search term
    // @Example "hello"
    Query string `json:"query" example:"hello"`
}
```

## Setting Up Swagger UI

Finally, let's implement the Swagger UI setup. In our Rust project, this was handled by `utoipa_swagger_ui`. Here's the Go equivalent using `swaggo/gin-swagger`:

```go
// main.go
package main

import (
    "github.com/gin-gonic/gin"
    swaggerFiles "github.com/swaggo/files"
    ginSwagger "github.com/swaggo/gin-swagger"
    _ "your-project/docs" // This is where the generated docs are
)

func setupSwagger(r *gin.Engine) {
    // Serve swagger documentation
    r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler,
        ginSwagger.PersistAuthorization(true),
        ginSwagger.DocExpansion("none"),
    ))
}
```

## Generating Documentation

With swaggo/swag, we need to generate the documentation files:

```bash
# Install swag CLI
go install github.com/swaggo/swag/cmd/swag@latest

# Generate documentation (run from project root)
swag init
```

This will create documentation files in the `docs/` directory, which will be served by the Swagger UI.

## Best Practices

1. **Keep Documentation Updated**: Always update API documentation when modifying endpoints
2. **Use Examples**: Provide clear examples in your documentation
3. **Document Error Responses**: Include possible error responses and their meanings
4. **Group Related Endpoints**: Use tags to organize endpoints logically
5. **Version Your API**: Include API version information in the documentation

## Summary

We've successfully translated our Rust project's OpenAPI documentation approach to Go, maintaining the same level of detail and functionality. The Go ecosystem offers several mature tools for API documentation, with `swaggo/swag` providing a familiar annotation-based approach similar to our original Rust implementation.

In the next chapter, we'll dive into implementing authentication and authorization mechanisms in our Go application.