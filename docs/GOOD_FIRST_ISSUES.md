# Good First Issues

These issues are specifically designed for new contributors. Each one is self-contained,
well-scoped, and includes guidance on where to start.

## How to Contribute

1. Comment on the issue to claim it
2. Fork the repo and create a branch
3. Follow the [CONTRIBUTING.md](CONTRIBUTING.md) guide
4. Submit a PR — we review within 48 hours

## Suggested Starter Issues

### Documentation
- [ ] **Add JSDoc comments to all React components** — Components in `web/src/components/` need TypeScript JSDoc descriptions
- [ ] **Write CLI usage examples** — Add real-world examples for each `chronosctl` command in `docs/cli.md`
- [ ] **Create SDK quick-start guides** — Write a 5-minute guide for each SDK (Go, Python, TypeScript) in `docs/`

### Testing
- [ ] **Add Terraform acceptance tests for workflow resource** — Follow the pattern in `acceptance_test.go`
- [ ] **Add E2E test for job disable/enable cycle** — Extend `e2e/e2e_test.go`
- [ ] **Add Python SDK async client** — Add an async version of the client using `asyncio`/`aiohttp`

### Features
- [ ] **Add `chronosctl job export` command** — Export a job definition as YAML for use with `apply -f`
- [ ] **Add `--namespace` flag to `chronosctl job list`** — Filter jobs by namespace
- [ ] **Add execution count badge to Web UI job list** — Show total executions next to each job name

### Code Quality
- [ ] **Add golangci-lint rules for new packages** — Ensure new packages follow project linting standards
