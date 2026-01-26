# Contributing to Chronos

Thank you for your interest in contributing to Chronos! This document provides guidelines and instructions for contributing.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
- [Development Setup](#development-setup)
- [Making Changes](#making-changes)
- [Pull Request Process](#pull-request-process)
- [Code Style](#code-style)
- [Testing](#testing)
- [Documentation](#documentation)

## Code of Conduct

Please be respectful and constructive in all interactions. We are committed to providing a welcoming and inclusive experience for everyone.

## Getting Started

### Prerequisites

- **Go 1.22+** - [Download Go](https://golang.org/dl/)
- **Node.js 18+** - For Web UI development
- **Docker** - Optional, for containerized builds
- **Make** - For running build commands

### Development Setup

1. **Fork and clone the repository**
   ```bash
   git clone https://github.com/YOUR-USERNAME/chronos.git
   cd chronos
   ```

2. **Download dependencies**
   ```bash
   make deps
   ```

3. **Verify your setup**
   ```bash
   make test
   make build
   ```

4. **Run locally**
   ```bash
   cp chronos.yaml.example chronos.yaml
   make run
   ```

## Making Changes

### Finding Something to Work On

- Check [open issues](https://github.com/chronos/chronos/issues) for bugs or feature requests
- Look for issues labeled `good first issue` for beginner-friendly tasks
- Feel free to ask questions in issue comments before starting work

### Creating a Branch

```bash
git checkout -b feature/your-feature-name
# or
git checkout -b fix/issue-description
```

### Commit Messages

We follow [Conventional Commits](https://www.conventionalcommits.org/):

```
feat: add new webhook retry option
fix: resolve scheduling race condition
docs: update API documentation
test: add scheduler integration tests
chore: update dependencies
```

## Pull Request Process

1. **Ensure all tests pass**
   ```bash
   make test
   ```

2. **Run the linter**
   ```bash
   make lint
   ```

3. **Update documentation** if you changed behavior or added features

4. **Create your pull request**
   - Provide a clear description of the changes
   - Reference any related issues
   - Include screenshots for UI changes

5. **Address review feedback**
   - Respond to all comments
   - Push additional commits to address feedback
   - Request re-review when ready

### PR Checklist

- [ ] Tests pass locally (`make test`)
- [ ] Linter passes (`make lint`)
- [ ] Documentation updated (if applicable)
- [ ] Commit messages follow conventional commits
- [ ] Branch is up to date with main

## Code Style

### Go

- Follow [Effective Go](https://go.dev/doc/effective_go)
- Run `gofmt` before committing (automatic with most editors)
- Use meaningful variable and function names
- Add godoc comments for all exported types and functions
- Handle errors explicitly - don't ignore them

### TypeScript (Web UI)

- Follow the existing patterns in `web/src`
- Use TypeScript types - avoid `any`
- Run `npm run lint` before committing

### Comments

- Comment **why**, not **what**
- Keep comments up to date with code changes
- Use TODO comments for future improvements: `// TODO: description`

## Testing

### Running Tests

```bash
# Run all tests
make test

# Run tests with coverage
make test-coverage

# Run specific package tests
go test -v ./internal/scheduler/...

# Run tests matching a pattern
go test -v -run TestScheduler ./...
```

### Writing Tests

- Write table-driven tests where appropriate
- Test edge cases and error conditions
- Use subtests for related test cases
- Mock external dependencies

Example:
```go
func TestSomething(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        expected string
        wantErr  bool
    }{
        {"valid input", "test", "TEST", false},
        {"empty input", "", "", true},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result, err := Something(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("unexpected error: %v", err)
            }
            if result != tt.expected {
                t.Errorf("got %s, want %s", result, tt.expected)
            }
        })
    }
}
```

## Documentation

### Where to Document

- **README.md** - Overview, quick start, basic usage
- **docs/api.md** - API reference
- **Code comments** - Implementation details
- **Godoc** - Package and function documentation

### Godoc Guidelines

```go
// Package scheduler provides the job scheduling engine.
//
// The scheduler is responsible for tracking job schedules,
// determining when jobs should run, and dispatching executions.
package scheduler

// Scheduler manages job scheduling and execution.
// It runs as a background process, checking for due jobs
// at regular intervals defined by TickInterval.
type Scheduler struct {
    // ...
}

// New creates a new Scheduler with the given configuration.
// If cfg is nil, DefaultConfig() is used.
func New(store *storage.Store, disp *dispatcher.Dispatcher, logger zerolog.Logger, cfg *Config) *Scheduler {
    // ...
}
```

## Questions?

If you have questions about contributing, feel free to:

- Open an issue with your question
- Ask in the pull request comments
- Check existing issues for similar questions

Thank you for contributing to Chronos!
