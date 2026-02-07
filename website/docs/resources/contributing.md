---
sidebar_position: 5
title: Contributing
description: How to contribute to Chronos development
---

# Contributing to Chronos

We love contributions! Chronos is an open-source project and we welcome contributions of all kinds: bug fixes, features, documentation, and feedback.

## Quick Start

```bash
# 1. Fork and clone
git clone https://github.com/YOUR-USERNAME/chronos.git
cd chronos

# 2. Install dependencies
make deps

# 3. Run tests to verify setup
make test

# 4. Start developing
make run
```

## Ways to Contribute

### 🐛 Report Bugs

Found a bug? [Open an issue](https://github.com/chronos/chronos/issues/new?template=bug_report.md) with:
- Clear description of the problem
- Steps to reproduce
- Expected vs actual behavior
- Chronos version and environment details

### 💡 Suggest Features

Have an idea? [Start a discussion](https://github.com/chronos/chronos/discussions/new?category=ideas) with:
- Use case description
- Proposed solution
- Alternatives you considered

### 📝 Improve Documentation

Documentation improvements are highly valued:
- Fix typos or unclear explanations
- Add examples or tutorials
- Translate documentation

### 🔧 Submit Code

Ready to code? Here's the process:

1. **Find or create an issue** - Check [good first issues](https://github.com/chronos/chronos/labels/good%20first%20issue)
2. **Fork and branch** - Create a feature branch
3. **Make changes** - Write code and tests
4. **Submit PR** - Open a pull request

## Development Setup

### Prerequisites

| Tool | Version | Purpose |
|------|---------|---------|
| Go | 1.22+ | Core development |
| Node.js | 18+ | Web UI development |
| Docker | Latest | Containerized builds |
| Make | Any | Build automation |

### Building from Source

```bash
# Clone your fork
git clone https://github.com/YOUR-USERNAME/chronos.git
cd chronos

# Download Go dependencies
make deps

# Build the binary
make build

# Build with all targets (includes CLI, operator)
make all

# Build Docker image
make docker-build
```

### Running Locally

```bash
# Copy example configuration
cp chronos.yaml.example chronos.yaml

# Start Chronos
make run

# Or run directly
./bin/chronos --config chronos.yaml
```

### Running Tests

```bash
# Run all tests
make test

# Run with coverage report
make test-coverage

# Run specific package
go test -v ./internal/scheduler/...

# Run with race detection
go test -race ./...

# Run integration tests
make test-integration

# Run end-to-end tests
make test-e2e
```

### Linting

```bash
# Run linter
make lint

# Auto-fix some issues
make lint-fix
```

## Code Guidelines

### Project Structure

```
chronos/
├── cmd/                    # Application entrypoints
│   ├── chronos/           # Main server
│   └── chronosctl/        # CLI tool
├── internal/              # Private packages
│   ├── api/              # HTTP API handlers
│   ├── config/           # Configuration
│   ├── dispatcher/       # Job execution
│   ├── raft/             # Consensus layer
│   ├── scheduler/        # Scheduling engine
│   └── storage/          # Data persistence
├── pkg/                   # Public packages (SDK)
├── api/                   # OpenAPI specs
├── web/                   # Web UI source
├── deployments/          # Deployment configs
└── docs/                  # Additional documentation
```

### Go Style Guide

We follow [Effective Go](https://go.dev/doc/effective_go) and use:

- `gofmt` for formatting (automatic)
- `golangci-lint` for static analysis
- Meaningful names over comments
- Explicit error handling

**Example:**

```go
// Job represents a scheduled task with its execution configuration.
type Job struct {
    ID        string    `json:"id"`
    Name      string    `json:"name"`
    Schedule  string    `json:"schedule"`
    Enabled   bool      `json:"enabled"`
    CreatedAt time.Time `json:"created_at"`
}

// Validate checks if the job configuration is valid.
// It returns an error describing any validation failures.
func (j *Job) Validate() error {
    if j.Name == "" {
        return errors.New("job name is required")
    }
    if _, err := cron.Parse(j.Schedule); err != nil {
        return fmt.Errorf("invalid schedule: %w", err)
    }
    return nil
}
```

### Commit Messages

We use [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>(<scope>): <subject>

<body>

<footer>
```

**Types:**
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation only
- `test`: Adding tests
- `refactor`: Code change that neither fixes a bug nor adds a feature
- `perf`: Performance improvement
- `chore`: Maintenance tasks

**Examples:**

```
feat(scheduler): add support for @every interval syntax

fix(api): return 404 for missing jobs instead of 500

docs(readme): update installation instructions

test(dispatcher): add webhook retry tests
```

### Testing Standards

Write tests for all new code:

```go
func TestScheduler_NextRun(t *testing.T) {
    tests := []struct {
        name     string
        schedule string
        from     time.Time
        want     time.Time
        wantErr  bool
    }{
        {
            name:     "every hour",
            schedule: "0 * * * *",
            from:     time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC),
            want:     time.Date(2026, 1, 15, 11, 0, 0, 0, time.UTC),
        },
        {
            name:     "invalid schedule",
            schedule: "invalid",
            wantErr:  true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := NextRun(tt.schedule, tt.from)
            if (err != nil) != tt.wantErr {
                t.Errorf("NextRun() error = %v, wantErr %v", err, tt.wantErr)
                return
            }
            if !got.Equal(tt.want) {
                t.Errorf("NextRun() = %v, want %v", got, tt.want)
            }
        })
    }
}
```

## Pull Request Process

### Before Submitting

- [ ] Tests pass: `make test`
- [ ] Linter passes: `make lint`
- [ ] Documentation updated (if needed)
- [ ] Commit messages follow conventions
- [ ] Branch is rebased on latest `main`

### PR Template

When opening a PR, include:

```markdown
## Summary
Brief description of changes

## Related Issue
Fixes #123

## Type of Change
- [ ] Bug fix
- [ ] New feature
- [ ] Documentation update
- [ ] Performance improvement
- [ ] Refactoring

## Testing
Describe how you tested the changes

## Checklist
- [ ] Tests added/updated
- [ ] Documentation updated
- [ ] CHANGELOG updated (for features/fixes)
```

### Review Process

1. **Automated checks** - CI runs tests and linting
2. **Code review** - Maintainers review the code
3. **Feedback** - Address any comments
4. **Approval** - At least one maintainer approves
5. **Merge** - Squash and merge into `main`

## Release Process

Releases follow [Semantic Versioning](https://semver.org/):

- **MAJOR** (v2.0.0): Breaking changes
- **MINOR** (v1.1.0): New features, backward compatible
- **PATCH** (v1.0.1): Bug fixes, backward compatible

### Creating a Release

Maintainers create releases:

```bash
# Update CHANGELOG.md
# Commit and tag
git tag v1.2.0
git push origin v1.2.0

# GoReleaser handles the rest
```

## Community

### Getting Help

- **GitHub Discussions**: [Ask questions](https://github.com/chronos/chronos/discussions)
- **Discord**: [Real-time chat](https://discord.gg/chronos)
- **Twitter**: [@chronos_cron](https://twitter.com/chronos_cron)

### Code of Conduct

We are committed to providing a welcoming and inclusive experience for everyone. Please:

- Be respectful and constructive
- Use welcoming and inclusive language
- Accept constructive criticism gracefully
- Focus on what is best for the community

See our full [Code of Conduct](https://github.com/chronos/chronos/blob/main/CODE_OF_CONDUCT.md).

## Recognition

Contributors are recognized in:

- [CHANGELOG.md](https://github.com/chronos/chronos/blob/main/CHANGELOG.md)
- [GitHub contributors page](https://github.com/chronos/chronos/graphs/contributors)
- Release notes

## License

By contributing, you agree that your contributions will be licensed under the [Apache 2.0 License](https://github.com/chronos/chronos/blob/main/LICENSE).

---

Thank you for contributing to Chronos! 🎉
