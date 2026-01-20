# H3

H3 is a lightweight, high-performance Go HTTP framework built on Go 1.22+ enhanced routing features.

[中文](README_CN.md) | English

## Features

- 🚀 **Standard Library Based** - Uses Go 1.22+ `http.ServeMux` enhanced routing
- 🧩 **Component Architecture** - Modular application structure through Component pattern
- 🔄 **Lifecycle Management** - Servlet interface supports component startup and shutdown lifecycle
- 🔌 **Middleware Support** - Onion model middleware chain, supports global and route-level middleware
- 📊 **Response Wrapping** - Automatically captures HTTP status code, response size, and write status
- ⚡ **Graceful Shutdown** - Built-in graceful shutdown support
- 🎯 **Type Safe** - Fully type-safe with zero reflection

## Installation

```bash
go get github.com/h3go/h3
```

**Requirements**: Go 1.25.5 or higher

## Quick Start

```go
package main

import (
    "net/http"
    "github.com/h3go/h3"
)

func main() {
    // Create router
    mux := h3.NewMux()
    
    // Register routes
    mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte("Hello, H3!"))
    })
    
    mux.HandleFunc("GET /users/{id}", func(w http.ResponseWriter, r *http.Request) {
        id := r.PathValue("id")
        w.Write([]byte("User ID: " + id))
    })
    
    // Start server
    server := h3.NewServer(":8080", mux)
    server.Start()
}
```

## Core Concepts

### 1. Mux (Router)

Mux is H3's core router, wrapping Go 1.22+ `http.ServeMux`:

```go
mux := h3.NewMux()

// Register handlers
mux.Handle("GET /api/users", usersHandler)
mux.HandleFunc("POST /api/users", createUser)

// Mount sub-router
apiMux := h3.NewMux()
apiMux.HandleFunc("GET /status", getStatus)
mux.Mount("/api", apiMux)
```

### 2. Component

Component is an independently registerable routing module for organizing large applications:

```go
// Create users module
usersComponent := h3.NewComponent("/users")
usersComponent.Mux().HandleFunc("GET /", listUsers)
usersComponent.Mux().HandleFunc("GET /{id}", getUser)
usersComponent.Mux().HandleFunc("POST /", createUser)

// Create admin module
adminComponent := h3.NewComponent("/admin")
adminComponent.Mux().HandleFunc("GET /dashboard", dashboard)

// Register to server
server := h3.NewServer(":8080", h3.NewMux())
server.Register(usersComponent)
server.Register(adminComponent)
server.Start()
```

### 3. Response (Response Wrapper)

Response automatically wraps `http.ResponseWriter` to capture response information:

```go
mux.Use(func(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        rw := h3.NewResponse(w)
        next.ServeHTTP(rw, r)
        
        // Log response information
        log.Printf("Status: %d, Size: %d bytes, Committed: %v",
            rw.Status(), rw.Size(), rw.Committed())
    })
})
```

Response interface supports advanced features:

```go
// HTTP/2 Server Push
rw.Push("/static/style.css", nil)

// Streaming response (SSE)
fmt.Fprintf(rw, "data: %s\n\n", message)
rw.Flush()

// WebSocket upgrade
conn, buf, err := rw.Hijack()
```

### 4. Servlet (Service Component)

Servlet is an optional interface for managing component lifecycle. Components implementing this interface can automatically initialize and cleanup resources during server startup and shutdown:

```go
type DatabaseComponent struct {
    *h3.Component
    db *sql.DB
}

func (c *DatabaseComponent) Start(ctx context.Context) error {
    // Connect to database on server startup
    db, err := sql.Open("postgres", "connection-string")
    if err != nil {
        return err
    }
    c.db = db
    return db.PingContext(ctx)
}

func (c *DatabaseComponent) Stop() error {
    // Disconnect database on server shutdown
    if c.db != nil {
        return c.db.Close()
    }
    return nil
}

// Register component implementing Servlet
server := h3.NewServer(":8080", h3.NewMux())
server.Register(dbComponent) // Start is called automatically
// ... server running
server.Stop(ctx)             // Stop is called automatically
```

**Servlet Features**:
- ✅ Automatic lifecycle management
- ✅ Start is called before HTTP server starts
- ✅ Stop is called in reverse registration order (LIFO)
- ✅ Start failure prevents server startup
- ✅ Stop is idempotent, can be safely called multiple times

**Common Use Cases**:
- Database connection pool initialization and cleanup
- Message queue connection management
- Background task startup and shutdown
- Scheduled task management
- Cache system initialization

### 5. Middleware

Middleware uses the standard `func(http.Handler) http.Handler` signature:

```go
// Custom middleware
func Logger(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        next.ServeHTTP(w, r)
        log.Printf("%s %s - %v", r.Method, r.URL.Path, time.Since(start))
    })
}

// Use middleware
mux.Use(Logger)
```

Middleware execution follows the onion model:

```
Request → M1 → M2 → M3 → Handler → M3 → M2 → M1 → Response
```

## Complete Examples

### Modular Application

```go
package main

import (
    "encoding/json"
    "net/http"
    "github.com/h3go/h3"
)

// Users module
func NewUsersComponent() h3.Component {
    c := h3.NewComponent("/users")
    mux := c.Mux()
    
    // List users
    mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
        users := []map[string]string{
            {"id": "1", "name": "Alice"},
            {"id": "2", "name": "Bob"},
        }
        json.NewEncoder(w).Encode(users)
    })
    
    // User details
    mux.HandleFunc("GET /{id}", func(w http.ResponseWriter, r *http.Request) {
        id := r.PathValue("id")
        user := map[string]string{"id": id, "name": "User " + id}
        json.NewEncoder(w).Encode(user)
    })
    
    return c
}

// Admin module
func NewAdminComponent() h3.Component {
    c := h3.NewComponent("/admin")
    mux := c.Mux()
    
    // Admin-specific middleware
    mux.Use(func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // Permission check
            token := r.Header.Get("Authorization")
            if token == "" {
                http.Error(w, "Unauthorized", http.StatusUnauthorized)
                return
            }
            next.ServeHTTP(w, r)
        })
    })
    
    mux.HandleFunc("GET /dashboard", func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte("Admin Dashboard"))
    })
    
    return c
}

func main() {
    // Create root router
    mux := h3.NewMux()
    
    // Root route
    mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte("Welcome to H3!"))
    })
    
    // Create server and register components
    server := h3.NewServer(":8080", mux)
    server.Register(NewUsersComponent())
    server.Register(NewAdminComponent())
    
    // Start server
    server.Start()
}
```

### Graceful Shutdown

```go
func main() {
    mux := h3.NewMux()
    mux.HandleFunc("GET /", handler)
    
    server := h3.NewServer(":8080", mux)
    
    // Start in goroutine
    go server.Start()
    
    // Wait for signal
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
    <-sigChan
    
    // Graceful shutdown
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    
    if err := server.Stop(ctx); err != nil {
        log.Printf("Server shutdown error: %v", err)
    }
}
```

## Routing Patterns

H3 uses Go 1.22+ routing pattern syntax:

```go
mux.HandleFunc("GET /users", listUsers)              // Exact match
mux.HandleFunc("GET /users/{id}", getUser)           // Path parameter
mux.HandleFunc("GET /files/{path...}", serveFile)    // Wildcard
mux.HandleFunc("POST /users", createUser)            // Method match
mux.HandleFunc("/about", about)                      // All methods
```

Access path parameters:

```go
func handler(w http.ResponseWriter, r *http.Request) {
    id := r.PathValue("id")
    path := r.PathValue("path")
}
```

## Performance

H3 is directly based on the standard library's `http.ServeMux`, with performance close to native Go HTTP server:

- Zero reflection
- Minimal memory allocation
- Efficient route matching
- Lightweight middleware chain

## Testing

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Verbose test output
go test -v ./...
```

Current test coverage: **95.4%**

## Framework Comparison

| Feature | H3 | Chi | Echo | Gin |
|---------|-----|-----|------|-----|
| Standard Library Based | ✅ | ✅ | ❌ | ❌ |
| Go 1.22+ Routing | ✅ | ❌ | ❌ | ❌ |
| Zero Reflection | ✅ | ✅ | ❌ | ❌ |
| Component Architecture | ✅ | ❌ | ❌ | ❌ |
| Graceful Shutdown | ✅ | ✅ | ✅ | ✅ |
| Middleware | ✅ | ✅ | ✅ | ✅ |

## License

MIT License

## Contributing

Issues and Pull Requests are welcome!
