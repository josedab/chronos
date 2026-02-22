# Release Checklist

## Pre-Release

- [ ] All tests pass: `go test ./internal/... ./pkg/... -count=1`
- [ ] E2E tests pass: `cd e2e && go test -timeout 120s`
- [ ] Terraform provider tests pass: `cd terraform-provider-chronos && go test ./...`
- [ ] Frontend tests pass: `cd web && npx vitest run`
- [ ] Python SDK tests pass: `cd sdks/python && python -m pytest tests/`
- [ ] Build succeeds: `go build ./...`
- [ ] Docker image builds: `docker build -f deployments/docker/Dockerfile .`
- [ ] CHANGELOG.md updated with version and date
- [ ] README.md is curated (stable features only, experimental in docs/EXPERIMENTAL.md)
- [ ] goreleaser dry-run succeeds: `goreleaser check`

## Release

- [ ] Tag the release: `git tag -a v0.1.0 -m "Release v0.1.0"`
- [ ] Push tag: `git push origin v0.1.0`
- [ ] Verify goreleaser builds complete in CI
- [ ] Verify Docker image is pushed to ghcr.io/chronos/chronos
- [ ] Verify GitHub Release is created with binaries

## Post-Release

- [ ] Publish Terraform provider: tag `terraform-provider-v0.1.0`
- [ ] Publish Python SDK to PyPI: `cd sdks/python && python -m build && twine upload dist/*`
- [ ] Publish JS SDK to npm: `cd sdks/javascript && npm publish`
- [ ] Announce on Hacker News ("Show HN: Chronos — Distributed cron with zero dependencies")
- [ ] Post on Reddit r/golang, r/devops, r/kubernetes
- [ ] Post on Twitter/X with architecture diagram
- [ ] Submit to awesome-go list
- [ ] Submit Terraform provider to registry.terraform.io
- [ ] Submit Helm chart to Artifact Hub
