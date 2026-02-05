// Package dispatcher handles job execution via HTTP webhooks and other protocols.
//
// The dispatcher is responsible for executing scheduled jobs by sending requests
// to configured endpoints. It supports HTTP webhooks as the primary execution method,
// with additional protocols available for specialized use cases.
//
// # Architecture
//
// The dispatcher maintains:
//
//   - HTTP client with connection pooling for efficient webhook calls
//   - Running execution tracker for concurrency management
//   - Circuit breakers for endpoint resilience
//   - Semaphore for limiting concurrent executions
//
// # HTTP Webhook Execution
//
// The primary execution method is HTTP webhooks. For each job execution:
//
//  1. Build HTTP request from job configuration (URL, method, headers, body)
//  2. Apply authentication (Bearer token, API key, Basic auth)
//  3. Send request with configured timeout
//  4. Capture response status, body, and duration
//  5. Determine success/failure based on status codes
//
// # Multi-Protocol Support
//
// In addition to HTTP, the dispatcher supports (via [MultiDispatcher]):
//
//   - gRPC: For high-performance service-to-service communication
//   - Kafka: For event-driven job execution via message queues
//   - NATS: For lightweight pub/sub messaging
//   - RabbitMQ: For robust message queue integration
//
// # Circuit Breaker Pattern
//
// The dispatcher implements circuit breakers per-endpoint to prevent cascade
// failures. When an endpoint fails repeatedly:
//
//   - Closed state: Normal operation, requests pass through
//   - Open state: Requests fail fast without calling endpoint
//   - Half-open state: Single request allowed to test recovery
//
// Configuration options:
//
//   - FailureThreshold: Failures before opening circuit (default: 5)
//   - SuccessThreshold: Successes to close circuit (default: 2)
//   - Timeout: Time before half-open transition (default: 30s)
//
// # Concurrency Control
//
// The dispatcher limits concurrent executions via a semaphore to prevent
// resource exhaustion. The MaxConcurrent configuration option controls this
// limit (default: 100 concurrent executions).
//
// # Usage
//
// Creating a dispatcher:
//
//	cfg := &dispatcher.Config{
//	    NodeID:        "chronos-1",
//	    Timeout:       30 * time.Second,
//	    MaxConcurrent: 100,
//	    CircuitBreaker: &dispatcher.CircuitBreakerConfig{
//	        FailureThreshold: 5,
//	        SuccessThreshold: 2,
//	        Timeout:          30 * time.Second,
//	    },
//	}
//	disp := dispatcher.New(cfg)
//
// Executing a job:
//
//	result := disp.Execute(ctx, job, execution)
//	if result.Success {
//	    log.Printf("Job completed in %v", result.Duration)
//	} else {
//	    log.Printf("Job failed: %s", result.Error)
//	}
//
// # Metrics
//
// The dispatcher exposes metrics for monitoring:
//
//   - chronos_dispatcher_executions_total: Total executions attempted
//   - chronos_dispatcher_executions_success: Successful executions
//   - chronos_dispatcher_executions_failed: Failed executions
//   - chronos_dispatcher_circuit_breaker_state: Circuit breaker state per endpoint
//
// # Thread Safety
//
// The dispatcher is fully thread-safe. It uses mutexes for state protection
// and atomic counters for metrics.
package dispatcher
