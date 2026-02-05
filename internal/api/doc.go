// Package api provides the REST API for Chronos.
//
// This package implements the HTTP API server including routing, middleware,
// and handlers for all Chronos operations. It uses the [chi] router for
// HTTP routing and middleware composition.
//
// # Architecture
//
// The API layer consists of:
//
//   - [Router]: HTTP route definitions and middleware stack
//   - [Handler]: Request handlers for each endpoint
//   - Middleware: Authentication, RBAC, rate limiting, audit logging
//
// # Endpoints
//
// Core job management endpoints:
//
//   - GET    /health                      Health check
//   - GET    /metrics                     Prometheus metrics
//   - GET    /api/v1/jobs                 List jobs
//   - POST   /api/v1/jobs                 Create job
//   - GET    /api/v1/jobs/{id}            Get job
//   - PUT    /api/v1/jobs/{id}            Update job
//   - DELETE /api/v1/jobs/{id}            Delete job
//   - POST   /api/v1/jobs/{id}/trigger    Trigger execution
//   - POST   /api/v1/jobs/{id}/enable     Enable job
//   - POST   /api/v1/jobs/{id}/disable    Disable job
//   - GET    /api/v1/jobs/{id}/executions List executions
//   - GET    /api/v1/cluster/status       Cluster status
//
// # Middleware Stack
//
// Requests pass through the following middleware (in order):
//
//  1. RequestID: Assigns unique ID for request tracing
//  2. RealIP: Extracts client IP from X-Forwarded-For
//  3. Logger: Structured request/response logging
//  4. Recoverer: Panic recovery with stack trace
//  5. Timeout: Request timeout (default: 60s)
//  6. RateLimiter: Token bucket rate limiting (optional)
//  7. Audit: Audit logging for compliance (optional)
//  8. CORS: Cross-origin resource sharing
//  9. Auth: API key / JWT authentication
//  10. RBAC: Role-based access control
//  11. Policy: Policy-as-code enforcement
//
// # Authentication
//
// The API supports multiple authentication methods:
//
//   - API Keys: Static keys configured in chronos.yaml
//   - JWT Tokens: Bearer tokens with configurable issuer/secret
//   - Basic Auth: Username/password (for simple deployments)
//
// Configuration:
//
//	auth:
//	  type: api_key
//	  api_keys:
//	    - key: "sk_live_xxx"
//	      name: "production"
//	      roles: ["admin"]
//
// # Rate Limiting
//
// Token bucket rate limiting protects against abuse:
//
//   - Configurable requests per second and burst size
//   - Per-client tracking (by IP or API key)
//   - Returns 429 Too Many Requests when exceeded
//
// # RBAC (Role-Based Access Control)
//
// The API enforces permissions based on user roles:
//
//   - admin: Full access to all operations
//   - operator: Manage jobs, view executions
//   - developer: Create/manage own jobs
//   - viewer: Read-only access
//
// Permissions are checked at the route level via middleware.
//
// # Response Format
//
// All responses follow a consistent JSON structure:
//
//	{
//	    "success": true,
//	    "data": { ... },
//	    "error": null
//	}
//
// Error responses:
//
//	{
//	    "success": false,
//	    "data": null,
//	    "error": {
//	        "code": "NOT_FOUND",
//	        "message": "job not found"
//	    }
//	}
//
// # Usage
//
// Creating and starting the API server:
//
//	store := storage.NewBadgerStore(cfg.DataDir)
//	handler := api.NewHandler(store, scheduler, logger)
//	router := api.NewRouterWithConfig(handler, logger, api.RouterConfig{
//	    AuthConfig: api.AuthConfig{
//	        Type:    "api_key",
//	        APIKeys: []api.APIKey{{Key: "secret", Roles: []string{"admin"}}},
//	    },
//	    RateLimiter: api.NewRateLimiter(100, 10), // 100 req/s, burst 10
//	})
//
//	server := &http.Server{
//	    Addr:    ":8080",
//	    Handler: router,
//	}
//	server.ListenAndServe()
//
// # Thread Safety
//
// The API handlers are stateless and thread-safe. All shared state is
// accessed through the storage layer, which provides its own synchronization.
//
// [chi]: https://github.com/go-chi/chi
package api
