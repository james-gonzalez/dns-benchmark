# Developer Guide & Agent Instructions

This repository contains `dns-bench`, a high-performance DNS benchmarking utility written in Go.
This document serves as a guide for AI agents and developers working on this codebase.

## 1. Build, Lint, and Test Commands

### Build
To build the application, use the standard Go build command:

```bash
# Build the binary
go build -o dns-bench

# Cross-compile (example for Linux AMD64)
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o dns-bench-linux-amd64
```

### Linting
Ensure code quality by running the following before committing:

```bash
# Standard Go vet
go vet ./...

# Format code (must run this)
gofmt -s -w .
```

### Testing
There are currently no existing test files (`*_test.go`). When adding new features, **you must add corresponding tests**.

```bash
# Run all tests (once added)
go test ./... -v

# Run a specific test
go test -run TestName ./... -v

# Run tests with race detection (recommended for concurrency code)
go test -race ./...
```

## 2. Code Style & Conventions

Adhere strictly to standard Go conventions (Effective Go).

### Formatting & Imports
- **Formatting:** Always run `gofmt`.
- **Imports:** Group imports in this order:
  1. Standard library (`"fmt"`, `"time"`)
  2. Third-party packages (`"github.com/miekg/dns"`)
  3. Local packages (`"dns-bench/benchmark"`)

```go
import (
    "fmt"
    "time"

    "github.com/miekg/dns"

    "dns-bench/benchmark"
)
```

### Naming Conventions
- **Exported:** `PascalCase` (e.g., `BenchmarkConfig`, `Run`).
- **Private:** `camelCase` (e.g., `calculateStats`, `measureDoH`).
- **Variables:** Short, descriptive names are preferred for small scopes (e.g., `res` for result, `err` for error, `wg` for WaitGroup).
- **Acronyms:** Keep acronyms consistent case (e.g., `ServeHTTP`, not `ServeHttp`).

### Error Handling
- **Propagate Errors:** Return errors to the caller rather than panicking. Only `main()` should handle top-level exits.
- **Check Errors:** Always check `if err != nil`.
- **Context:** Wrap errors with useful context if helpful, but keep it simple.

```go
if err != nil {
    return fmt.Errorf("failed to parse YAML: %w", err)
}
```

### Types & Structs
- Use **structs** for configuration and data passing (e.g., `BenchmarkConfig`, `Result`).
- Prefer `time.Duration` over `int` or `float64` for time values.

## 3. Architecture Overview

### Structure
- **`main.go`**: Entry point. Handles CLI flags, reads input files (CSV/YAML), calls the benchmark engine, and renders reports (CLI table, CSV, HTML).
- **`benchmark/`**: Core logic package.
  - **`Run(config)`**: Orchestrates the benchmark using a worker pool pattern.
  - **`Client`**: Handles the actual DNS queries (UDP, DoT, DoH).
- **`browser/`**: Handles extraction of history from web browsers (requires CGO/sqlite).

### Concurrency Model
The project uses a worker pool model:
1.  **Job Channel:** `jobs` channel accepts `Job` structs (server + domain).
2.  **Workers:** `Concurrency` flag determines the number of goroutines consuming from `jobs`.
3.  **Results:** Workers push `Result` objects to a `results` channel.
4.  **Synchronization:** `sync.WaitGroup` ensures all workers finish before closing the results channel.

## 4. Agent Guidelines

- **No Assumptions:** Do not assume dependencies exist. Check `go.mod`.
- **CGO Considerations:** The `browser` package uses `go-sqlite3` which requires CGO. If modifying build scripts, remember `CGO_ENABLED` implications.
- **Safety:** When adding file operations, verify paths first.
- **Tests:** Since test coverage is low/non-existent, PROACTIVELY write tests for any new logic you implement.
