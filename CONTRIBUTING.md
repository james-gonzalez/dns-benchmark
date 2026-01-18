# Contributing to DNS Benchmark

Thank you for considering contributing to dns-bench! We welcome contributions from the community.

## Code of Conduct

Please be respectful and constructive in all interactions. We aim to maintain a welcoming and inclusive environment.

## How to Contribute

### Reporting Bugs

Before creating bug reports, please check existing issues to avoid duplicates. When creating a bug report, include:

- Clear, descriptive title
- Steps to reproduce the behavior
- Expected vs actual behavior
- Environment details (OS, Go version, etc.)
- Any relevant logs or error messages

### Suggesting Enhancements

Enhancement suggestions are welcome! Please provide:

- Clear description of the feature
- Use cases and benefits
- Possible implementation approach (optional)

### Pull Requests

1. **Fork the repository** and create your branch from `main`
2. **Follow the code style** (see below)
3. **Add tests** for any new functionality
4. **Update documentation** as needed (README, AGENTS.md, etc.)
5. **Run tests and linting** before submitting
6. **Write clear commit messages** following Conventional Commits format

## Development Setup

### Prerequisites

- Go 1.21 or later
- Git
- golangci-lint (for linting)

### Building

```bash
go build -o dns-bench
```

### Running Tests

```bash
# Run all tests
go test ./... -v

# Run tests with race detection
go test -race ./...

# Run only fast tests (skip network tests)
go test -short ./...
```

### Linting

```bash
# Install golangci-lint (if not already installed)
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Run linter
golangci-lint run
```

### Formatting

Always format your code before committing:

```bash
gofmt -s -w .
```

## Code Style Guidelines

### General

- Follow standard Go conventions (Effective Go)
- Use meaningful variable and function names
- Keep functions focused and small
- Add comments for exported functions and complex logic

### Naming

- **Exported identifiers**: `PascalCase`
- **Unexported identifiers**: `camelCase`
- **Acronyms**: Maintain case consistency (e.g., `ServeHTTP`, not `ServeHttp`)

### Imports

Group imports in this order:
1. Standard library
2. Third-party packages
3. Local packages

```go
import (
    "fmt"
    "time"

    "github.com/miekg/dns"

    "dns-bench/benchmark"
)
```

### Error Handling

- Always check errors: `if err != nil`
- Wrap errors with context: `fmt.Errorf("operation failed: %w", err)`
- Return errors instead of panicking (except in `main()`)

### Testing

- Write table-driven tests when appropriate
- Use descriptive test names
- Test edge cases and error conditions
- Use `-short` flag for network-dependent tests

Example:
```go
func TestFeature(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping network test in short mode")
    }
    // test code
}
```

### Concurrency

- Use `sync.WaitGroup` for goroutine synchronization
- Close channels from the sender side
- Use `context.Context` for cancellation
- Avoid shared mutable state

## Commit Message Format

We use [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>(<scope>): <description>

[optional body]

[optional footer]
```

**Types:**
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `style`: Formatting, missing semicolons, etc.
- `refactor`: Code restructuring
- `test`: Adding or updating tests
- `chore`: Maintenance tasks

**Examples:**
```
feat(benchmark): add DNS-over-QUIC support
fix(validation): handle IPv6 addresses correctly
docs(readme): update installation instructions
test(benchmark): add tests for DoH protocol
```

## Project Structure

```
dns-bench/
├── main.go              # CLI entry point
├── benchmark/           # Core benchmarking logic
├── browser/             # Browser history integration
├── validation/          # Input validation
├── scripts/             # Helper scripts
└── AGENTS.md           # Developer guide
```

## Testing Guidelines

- **Unit tests**: Test individual functions in isolation
- **Integration tests**: Test component interactions
- **Network tests**: Mark with `if testing.Short()` skip
- **Coverage**: Aim for >80% coverage on new code

## Documentation

- Update README.md for user-facing changes
- Update AGENTS.md for developer-facing changes
- Add GoDoc comments for exported identifiers
- Include examples for complex features

## Review Process

1. All PRs require at least one review
2. CI checks must pass (tests, linting)
3. Maintain or improve code coverage
4. Address review feedback promptly

## Release Process

Releases are automated using semantic-release:
- Commits to `main` trigger version analysis
- Version bumped based on commit types
- GitHub release created with changelog
- Binaries built and attached via GoReleaser

## Questions?

Feel free to open an issue for questions or join discussions in existing issues.

Thank you for contributing!
