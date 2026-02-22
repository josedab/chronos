# Launch Campaign Materials

## Hacker News: Show HN

**Title:** Show HN: Chronos – Distributed cron with zero dependencies, Raft consensus, and triple IaC

**Body:**
Hi HN,

I've been building Chronos, a distributed cron system that runs as a single binary — no etcd, no PostgreSQL, no Redis. It embeds BadgerDB for storage and uses HashiCorp Raft for leader election.

What makes it different from K8s CronJobs or Airflow:

- **Zero dependencies** — Single binary. Works on bare metal, Docker, K8s, or edge devices.
- **Raft consensus** — Automatic failover in < 2 seconds (E2E tested with 3-node cluster).
- **Webhook security** — HMAC-SHA256 signing + mTLS + response assertions (JSON path matchers, timing SLAs).
- **Canary execution** — Shadow mode for safe webhook URL changes with response comparison.
- **Triple IaC** — Terraform provider + Pulumi SDK + K8s Operator (unique in this space).
- **Migration wizard** — Import from crontab, K8s CronJobs, or Airflow DAGs.

Tech details: 102K LOC Go, 1,037 test functions, 23 benchmarks, 48 internal packages, zero `go vet` issues.

GitHub: https://github.com/chronos/chronos
Docs: https://chronos.github.io/chronos
Benchmarks: https://github.com/chronos/chronos/blob/main/docs/BENCHMARKS.md

Would love feedback on the architecture and feature set.

---

## Reddit r/golang

**Title:** Chronos: A distributed cron system in Go with zero external dependencies

**Body:**
Just released Chronos v0.1.0 — a distributed cron system built entirely in Go.

Highlights:
- Single binary with embedded BadgerDB (no external databases)
- Raft consensus with E2E-tested failover
- Multi-protocol dispatch (HTTP, gRPC, Kafka, NATS, RabbitMQ)
- HMAC webhook signing + mTLS
- Terraform provider + Pulumi SDK + K8s Operator
- 1,037 tests, 23 benchmarks, zero `go vet` violations

100K+ LOC of Go, 48 internal packages, clean architecture with zero circular dependencies.

https://github.com/chronos/chronos

---

## Reddit r/devops

**Title:** Tired of managing Airflow for simple cron jobs? We built a zero-dependency alternative

**Body:**
If you've ever set up Airflow just to run some cron jobs and thought "this is overkill," Chronos might be for you.

It's a distributed cron system that runs as a single binary — no databases, no message queues, no Kubernetes required. Just download and run.

Features that DevOps teams care about:
- Automatic failover with Raft consensus (< 2s recovery)
- HMAC-signed webhooks for security
- Migration wizard to import existing crontabs and K8s CronJobs
- Terraform provider for managing jobs as code
- Alert engine with Slack/PagerDuty integration
- Cost attribution per job/namespace

https://github.com/chronos/chronos

---

## Twitter/X Thread

1/ 🚀 Introducing Chronos — a distributed cron system with zero dependencies.

Single binary. No etcd. No PostgreSQL. No Redis. Just download and run.

Built in Go. 102K LOC. 1,037 tests. Open source (Apache 2.0).

🧵 Thread...

2/ What makes it different?

✅ Raft consensus with < 2s failover (E2E tested)
✅ HMAC-signed webhooks + mTLS
✅ Response assertions (JSON path, body, timing SLAs)
✅ Canary/shadow execution for safe rollouts
✅ Triple IaC: Terraform + Pulumi + K8s Operator

3/ We built a migration wizard that imports from:
- Unix crontab
- K8s CronJobs
- Airflow DAGs
- AWS EventBridge
- Temporal
- GitHub Actions

Switching to Chronos takes minutes, not days.

4/ Developer experience matters:
- SDKs for Go, Python, and TypeScript
- Web UI with failure heatmaps and SLO tracking
- `chronosctl apply -f` for GitOps
- GitHub Action for CI/CD deployment
- Docker playground to try it in 30 seconds

5/ Try it now:

```
docker run -d -p 8080:8080 ghcr.io/chronos/chronos:0.1.0
```

GitHub: github.com/chronos/chronos
Docs: chronos.github.io/chronos

Star ⭐ if distributed cron without the complexity sounds useful!
