# ADR-0005: RESTful API with chi Router

## Status

Accepted

## Context

Chronos requires a management API for:

- **Job management**: CRUD operations, enabling/disabling, manual triggering
- **Execution queries**: Viewing history, logs, and status
- **Cluster operations**: Health checks, leader status, node management
- **Metrics**: Prometheus-compatible metrics endpoint

API requirements:

- **RESTful design**: Familiar patterns for easy integration
- **Middleware support**: Authentication, rate limiting, logging, CORS
- **Performance**: Low overhead for high-frequency polling
- **OpenAPI compatibility**: Machine-readable API specification

We evaluated several Go HTTP routers:

1. **net/http (stdlib)**: Minimal dependencies but lacks routing patterns and middleware
2. **Gorilla Mux**: Feature-rich but maintainer stepped back; now community-maintained
3. **Gin**: Popular but heavy; includes features we don't need (HTML rendering)
4. **Echo**: Good performance but larger API surface
5. **chi**: Lightweight, stdlib-compatible, excellent middleware composition

## Decision

We chose **go-chi/chi** as the HTTP router for the Chronos API.

Key characteristics:
- 100% compatible with `net/http`
- Composable middleware stack
- URL parameter extraction (`/jobs/{id}`)
- Subrouter support for API versioning
- No external dependencies beyond stdlib

```go
// internal/api/router.go
func NewRouterWithConfig(handler *Handler, logger zerolog.Logger, config RouterConfig) *chi.Mux {
    r := chi.NewRouter()

    // Middleware stack
    r.Use(middleware.RequestID)
    r.Use(middleware.RealIP)
    r.Use(NewLoggingMiddleware(logger))
    r.Use(middleware.Recoverer)
    r.Use(middleware.Timeout(60 * time.Second))

    // API routes
    r.Route("/api/v1", func(r chi.Router) {
        r.Use(NewAuthMiddleware(config.AuthConfig))
        r.Route("/jobs", func(r chi.Router) {
            r.Get("/", handler.ListJobs)
            r.Post("/", handler.CreateJob)
            r.Route("/{id}", func(r chi.Router) {
                r.Get("/", handler.GetJob)
                r.Put("/", handler.UpdateJob)
                r.Delete("/", handler.DeleteJob)
            })
        })
    })

    return r
}
```

## Consequences

### Positive

- **stdlib compatibility**: Handlers are standard `http.HandlerFunc`; easy testing and migration
- **Minimal footprint**: chi adds ~2000 lines of code; tiny binary size impact
- **Excellent middleware**: Built-in middleware for common patterns; easy to write custom middleware
- **Context-based**: URL parameters via `chi.URLParam(r, "id")` using standard context
- **Subrouter composition**: Clean organization of versioned API endpoints
- **Active maintenance**: Well-maintained with regular releases

### Negative

- **Less "batteries included"**: No built-in validation, binding, or response helpers (we add our own)
- **Manual OpenAPI sync**: API spec must be maintained separately (we use openapi.yaml)
- **Learning curve**: Middleware composition patterns require understanding

### Middleware Stack

Our middleware executes in this order:

1. **RequestID**: Adds unique request ID for tracing
2. **RealIP**: Extracts client IP from X-Forwarded-For
3. **Logging**: Structured request/response logging with zerolog
4. **Recoverer**: Panic recovery to prevent crashes
5. **Timeout**: 60-second request timeout
6. **RateLimit**: Per-IP/per-API-key rate limiting (optional)
7. **Audit**: Audit logging for compliance (optional)
8. **CORS**: Cross-origin resource sharing headers
9. **Auth**: API key or Bearer token validation
10. **RBAC**: Permission checks based on user roles

### API Versioning

APIs are versioned via URL path (`/api/v1/`), allowing:
- Breaking changes in future versions (`/api/v2/`)
- Parallel support for multiple versions during migration
- Clear deprecation path for old versions

### CORS Configuration

```go
func NewCORSMiddleware(allowedOrigins []string) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
            w.Header().Set("Access-Control-Allow-Headers", "Accept, Authorization, Content-Type, X-API-Key")
            // ...
        })
    }
}
```

### Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/health` | Health check (no auth) |
| GET | `/metrics` | Prometheus metrics (no auth) |
| GET | `/api/v1/jobs` | List all jobs |
| POST | `/api/v1/jobs` | Create a job |
| GET | `/api/v1/jobs/{id}` | Get job details |
| PUT | `/api/v1/jobs/{id}` | Update a job |
| DELETE | `/api/v1/jobs/{id}` | Delete a job |
| POST | `/api/v1/jobs/{id}/trigger` | Manual trigger |
| GET | `/api/v1/cluster/status` | Cluster status |

## References

- [go-chi/chi](https://github.com/go-chi/chi)
- [chi Documentation](https://go-chi.io/)
- [RESTful API Design](https://restfulapi.net/)
