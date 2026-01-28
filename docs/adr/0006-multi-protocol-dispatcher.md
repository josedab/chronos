# ADR-0006: Multi-Protocol Dispatcher Architecture

## Status

Accepted

## Context

While HTTP webhooks (ADR-0004) serve as the primary execution mechanism, enterprise environments often require integration with diverse messaging and RPC infrastructure:

- **Message queues**: Kafka, RabbitMQ for event-driven architectures
- **Pub/sub systems**: NATS for lightweight, high-performance messaging
- **RPC frameworks**: gRPC for low-latency, type-safe service communication
- **Legacy systems**: Some internal services may not expose HTTP endpoints

Forcing all job targets to implement HTTP endpoints creates friction for teams with existing infrastructure investments.

Requirements:

- **Extensibility**: Support additional protocols without core changes
- **Unified semantics**: Consistent retry, timeout, and error handling across protocols
- **Configuration parity**: Similar job definitions regardless of dispatch method
- **Gradual adoption**: HTTP remains the default; other protocols opt-in

## Decision

We implemented a **multi-protocol dispatcher architecture** with pluggable dispatch backends:

```
internal/dispatcher/
├── dispatcher.go   # HTTP dispatcher (default)
├── grpc.go         # gRPC dispatcher
├── kafka.go        # Kafka publisher
├── nats.go         # NATS publisher
├── rabbitmq.go     # RabbitMQ publisher
└── multi.go        # Protocol router
```

Each dispatcher implements a common interface for job execution, and jobs specify their target protocol in the configuration.

### Protocol Selection

Jobs can specify alternate dispatch methods:

```yaml
# HTTP (default)
webhook:
  url: "https://api.example.com/job"
  method: POST

# gRPC
grpc:
  address: "jobs.example.com:9000"
  method: "JobService/Execute"
  
# Kafka
kafka:
  brokers: ["kafka-1:9092", "kafka-2:9092"]
  topic: "job-executions"
  
# NATS
nats:
  url: "nats://nats.example.com:4222"
  subject: "jobs.execute"
  
# RabbitMQ  
rabbitmq:
  url: "amqp://guest:guest@rabbitmq:5672/"
  exchange: "jobs"
  routing_key: "execute"
```

## Consequences

### Positive

- **Infrastructure flexibility**: Teams use their existing messaging infrastructure
- **Performance options**: gRPC for low-latency; Kafka for high-throughput fan-out
- **Decoupling**: Message queue targets enable async processing and buffering
- **Ecosystem integration**: Native integration with event-driven architectures
- **Protocol-appropriate semantics**: Each dispatcher handles protocol-specific concerns

### Negative

- **Increased complexity**: More code paths to maintain and test
- **Configuration surface**: Users must understand protocol-specific options
- **Dependency footprint**: Optional protocol support increases binary size
- **Inconsistent feedback**: Message queues provide weaker delivery guarantees than HTTP

### Protocol Comparison

| Protocol | Latency | Throughput | Feedback | Use Case |
|----------|---------|------------|----------|----------|
| HTTP | Low | Medium | Synchronous | General purpose |
| gRPC | Very Low | High | Synchronous | Internal services |
| Kafka | Medium | Very High | Async | Event streaming |
| NATS | Low | High | Async/Sync | Microservices |
| RabbitMQ | Medium | Medium | Async | Work queues |

### Execution Semantics by Protocol

**HTTP/gRPC (Synchronous)**:
- Execution waits for response
- Success/failure determined by response code
- Retries on transient failures

**Kafka/NATS/RabbitMQ (Asynchronous)**:
- Execution completes when message is published
- Success means message accepted by broker
- Consumer failures not tracked by Chronos

### gRPC Implementation

```go
// internal/dispatcher/grpc.go
type GRPCDispatcher struct {
    connections map[string]*grpc.ClientConn
    timeout     time.Duration
}

func (d *GRPCDispatcher) Execute(ctx context.Context, job *models.Job) (*models.ExecutionResult, error) {
    conn, err := d.getConnection(job.GRPC.Address)
    // Invoke method dynamically using reflection
    // Marshal job metadata as request
}
```

### Kafka Implementation

```go
// internal/dispatcher/kafka.go
type KafkaDispatcher struct {
    producer sarama.SyncProducer
}

func (d *KafkaDispatcher) Execute(ctx context.Context, job *models.Job) (*models.ExecutionResult, error) {
    msg := &sarama.ProducerMessage{
        Topic: job.Kafka.Topic,
        Key:   sarama.StringEncoder(job.ID),
        Value: sarama.ByteEncoder(payload),
        Headers: []sarama.RecordHeader{
            {Key: []byte("X-Chronos-Job-ID"), Value: []byte(job.ID)},
            {Key: []byte("X-Chronos-Execution-ID"), Value: []byte(execID)},
        },
    }
    _, _, err := d.producer.SendMessage(msg)
}
```

### Configuration

```yaml
# chronos.yaml
dispatcher:
  http:
    timeout: 30s
    max_idle_conns: 100
  grpc:
    timeout: 30s
    max_message_size: 4MB
  kafka:
    required_acks: all
    retry_max: 3
  nats:
    timeout: 10s
  rabbitmq:
    confirm_mode: true
```

## References

- [gRPC](https://grpc.io/)
- [Apache Kafka](https://kafka.apache.org/)
- [NATS](https://nats.io/)
- [RabbitMQ](https://www.rabbitmq.com/)
