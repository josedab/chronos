# Architecture Decision Records

This directory contains Architecture Decision Records (ADRs) for Chronos.

ADRs document the significant architectural decisions made during the design and evolution of this system. They provide context for why things are built the way they are, helping new team members understand the reasoning behind key choices.

## ADR Index

| ADR | Title | Status |
|-----|-------|--------|
| [ADR-0001](0001-go-as-primary-language.md) | Go as Primary Implementation Language | Accepted |
| [ADR-0002](0002-raft-consensus-for-coordination.md) | Raft Consensus for Distributed Coordination | Accepted |
| [ADR-0003](0003-badgerdb-embedded-storage.md) | BadgerDB as Embedded Storage Engine | Accepted |
| [ADR-0004](0004-http-webhook-execution-model.md) | HTTP Webhook-First Job Execution Model | Accepted |
| [ADR-0005](0005-restful-api-with-chi.md) | RESTful API with chi Router | Accepted |
| [ADR-0006](0006-multi-protocol-dispatcher.md) | Multi-Protocol Dispatcher Architecture | Accepted |
| [ADR-0007](0007-dag-workflow-orchestration.md) | DAG-Based Workflow Orchestration | Accepted |
| [ADR-0008](0008-multi-tenancy-tiered-quotas.md) | Multi-Tenancy with Tiered Quotas | Accepted |
| [ADR-0009](0009-rbac-with-teams.md) | Role-Based Access Control with Teams | Accepted |
| [ADR-0010](0010-cross-region-federation.md) | Cross-Region Federation for Global Deployment | Accepted |
| [ADR-0011](0011-policy-as-code-governance.md) | Policy-as-Code Governance Engine | Accepted |

## ADR Format

Each ADR follows this structure:

- **Title**: Short descriptive name
- **Status**: Accepted, Superseded, or Deprecated
- **Context**: What prompted this decision?
- **Decision**: What was decided?
- **Consequences**: Tradeoffs, implications, what this enabled/prevented

## Contributing

When making significant architectural changes, please add a new ADR documenting:
1. The problem or opportunity
2. Options considered
3. The decision made and why
4. Expected consequences

Use the next available number (e.g., `0012-your-decision.md`).

## References

- [ADR GitHub Organization](https://adr.github.io/)
- [Michael Nygard's article on ADRs](https://cognitect.com/blog/2011/11/15/documenting-architecture-decisions)
