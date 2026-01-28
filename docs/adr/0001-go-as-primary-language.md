# ADR-0001: Go as Primary Implementation Language

## Status

Accepted

## Context

Chronos is a distributed cron system that requires:

- **High concurrency**: Managing thousands of scheduled jobs with parallel execution
- **Low latency**: Sub-second scheduling precision for time-critical workloads
- **Operational simplicity**: Easy deployment across diverse infrastructure
- **Strong networking**: Native support for HTTP servers, gRPC, and network protocols
- **Distributed systems ecosystem**: Access to battle-tested libraries for consensus, coordination, and storage

The team evaluated several languages:

1. **Java/Kotlin**: Rich ecosystem but JVM startup time and memory overhead problematic for lightweight deployments
2. **Rust**: Excellent performance but steeper learning curve and less mature distributed systems libraries
3. **Python**: Rapid development but GIL limits true parallelism; runtime dependency management
4. **Node.js**: Good async model but single-threaded; less suitable for CPU-bound coordination work
5. **Go**: Purpose-built for networked services with goroutines, channels, and single-binary deployment

## Decision

We chose **Go** as the primary implementation language for all Chronos components.

Key factors:

- **Goroutines and channels** provide lightweight concurrency primitives ideal for managing thousands of concurrent job executions and Raft state machine operations
- **Single binary compilation** eliminates runtime dependencies, simplifying Docker images and Kubernetes deployments
- **Static typing with fast compilation** enables rapid iteration while catching errors at compile time
- **Mature distributed systems ecosystem**: HashiCorp's Raft, BadgerDB, gRPC, and Prometheus client libraries are all Go-native
- **Strong standard library** for HTTP servers, JSON handling, and cryptography reduces external dependencies
- **Cross-compilation** simplifies building for Linux, macOS, and Windows from a single codebase

## Consequences

### Positive

- **Zero-dependency deployment**: The `chronos` binary runs anywhere without installing runtimes
- **Predictable performance**: No garbage collection pauses typical of JVM; Go's GC is optimized for low latency
- **Developer productivity**: Go's simplicity and tooling (`go fmt`, `go test`, `go mod`) accelerate development
- **Ecosystem alignment**: First-class support for Kubernetes controllers, Prometheus metrics, and cloud SDKs
- **Hiring**: Go is widely adopted for infrastructure tooling, ensuring a talent pool familiar with the patterns

### Negative

- **Verbose error handling**: Go's explicit `if err != nil` pattern increases code volume compared to exceptions
- **Limited generics** (prior to Go 1.18): Some abstractions required code generation or interface{}; now mitigated
- **No runtime plugins**: Extensibility requires recompilation rather than dynamic loading (addressed via webhooks and gRPC)

### Tradeoffs Accepted

- We chose operational simplicity over raw performance (Rust) or rapid prototyping (Python)
- The explicit nature of Go code prioritizes maintainability over brevity
- Single-binary deployment constrains plugin architectures but simplifies the operational model

## References

- [Go at Google: Language Design in the Service of Software Engineering](https://go.dev/talks/2012/splash.article)
- [HashiCorp's use of Go](https://www.hashicorp.com/resources/why-we-use-go)
