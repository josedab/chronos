// Package api provides the REST API router.
package api

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
)

// RouterConfig holds configuration for the API router.
type RouterConfig struct {
	// AllowedOrigins is the list of allowed CORS origins. Empty means all origins allowed.
	AllowedOrigins []string
	// AuthConfig holds authentication configuration.
	AuthConfig AuthConfig
	// RateLimiter is the rate limiter instance (optional).
	RateLimiter *RateLimiter
	// RBACConfig holds RBAC configuration (optional).
	RBACConfig RBACConfig
	// AuditConfig holds audit logging configuration (optional).
	AuditConfig AuditConfig
}

// NewRouter creates a new API router.
func NewRouter(handler *Handler, logger zerolog.Logger) *chi.Mux {
	return NewRouterWithConfig(handler, logger, RouterConfig{})
}

// NewRouterWithConfig creates a new API router with configuration.
func NewRouterWithConfig(handler *Handler, logger zerolog.Logger, config RouterConfig) *chi.Mux {
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(NewLoggingMiddleware(logger))
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	// Rate limiting (before other middleware for early rejection)
	if config.RateLimiter != nil {
		r.Use(NewRateLimitMiddleware(config.RateLimiter))
	}

	// Audit logging (after rate limiting to only log allowed requests)
	if config.AuditConfig.Enabled {
		r.Use(NewAuditMiddleware(config.AuditConfig))
	}

	// CORS
	r.Use(NewCORSMiddleware(config.AllowedOrigins))

	// Health check (no auth required)
	r.Get("/health", handler.HealthCheck)

	// Prometheus metrics (no auth required)
	r.Handle("/metrics", promhttp.Handler())

	// Permission factory for RBAC
	perms := NewPermissionMiddlewareFactory(config.RBACConfig)
	jobs := perms.Jobs()

	// API v1 (auth required for protected routes)
	r.Route("/api/v1", func(r chi.Router) {
		// Apply authentication middleware to all API routes
		r.Use(NewAuthMiddleware(config.AuthConfig))
		// Load RBAC user from API key
		r.Use(NewRBACMiddleware(config.RBACConfig))

		// Jobs
		r.Route("/jobs", func(r chi.Router) {
			r.With(jobs.Read()).Get("/", handler.ListJobs)
			r.With(jobs.Create()).Post("/", handler.CreateJob)

			r.Route("/{id}", func(r chi.Router) {
				r.With(jobs.Read()).Get("/", handler.GetJob)
				r.With(jobs.Update()).Put("/", handler.UpdateJob)
				r.With(jobs.Delete()).Delete("/", handler.DeleteJob)

				r.With(jobs.Trigger()).Post("/trigger", handler.TriggerJob)
				r.With(jobs.Enable()).Post("/enable", handler.EnableJob)
				r.With(jobs.Disable()).Post("/disable", handler.DisableJob)

				// Versioning
				r.With(jobs.Read()).Get("/versions", handler.ListJobVersions)
				r.With(jobs.Read()).Get("/versions/{version}", handler.GetJobVersion)
				r.With(jobs.Update()).Post("/rollback/{version}", handler.RollbackJob)

				// Executions
				r.Route("/executions", func(r chi.Router) {
					r.With(RequirePermission(config.RBACConfig, "execution:read")).Get("/", handler.ListExecutions)
					r.With(RequirePermission(config.RBACConfig, "execution:read")).Get("/{execId}", handler.GetExecution)
					r.With(RequirePermission(config.RBACConfig, "execution:replay")).Post("/{execId}/replay", handler.ReplayExecution)
				})
			})
		})

		// Cluster
		r.Route("/cluster", func(r chi.Router) {
			r.With(RequirePermission(config.RBACConfig, "admin:cluster")).Get("/status", handler.ClusterStatus)
		})
	})

	return r
}

// MountCloudRoutes mounts the cloud/SaaS control plane routes.
// Call this after creating the router to add cloud management endpoints.
func MountCloudRoutes(r *chi.Mux, cloudHandler http.Handler) {
	r.Mount("/api/v1/cloud", cloudHandler)
}

// MountMarketplaceRoutes mounts the marketplace routes.
// Call this after creating the router to add marketplace endpoints.
func MountMarketplaceRoutes(r *chi.Mux, marketplaceHandler http.Handler) {
	r.Mount("/api/v1/marketplace", marketplaceHandler)
}

// MountChatbotRoutes mounts Slack and Teams bot webhook routes.
// These routes are typically public (webhook endpoints) but verify signatures internally.
func MountChatbotRoutes(r *chi.Mux, slackHandler, teamsHandler http.HandlerFunc) {
	r.Route("/webhooks", func(r chi.Router) {
		if slackHandler != nil {
			r.Post("/slack/command", slackHandler)
			r.Post("/slack/interaction", slackHandler)
		}
		if teamsHandler != nil {
			r.Post("/teams/messages", teamsHandler)
		}
	})
}

// NewCORSMiddleware creates a CORS middleware with configurable origins.
func NewCORSMiddleware(allowedOrigins []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			
			// If no allowed origins specified, allow all (development mode)
			if len(allowedOrigins) == 0 {
				w.Header().Set("Access-Control-Allow-Origin", "*")
			} else {
				// Check if origin is in allowed list
				for _, allowed := range allowedOrigins {
					if origin == allowed || allowed == "*" {
						w.Header().Set("Access-Control-Allow-Origin", origin)
						break
					}
				}
			}
			
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Accept, Authorization, Content-Type, X-API-Key, X-Requested-With")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Max-Age", "86400")

			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// CORSMiddleware handles CORS headers (deprecated, use NewCORSMiddleware).
func CORSMiddleware(next http.Handler) http.Handler {
	return NewCORSMiddleware(nil)(next)
}
