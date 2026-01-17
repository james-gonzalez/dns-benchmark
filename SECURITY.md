# Security Policy

## Supported Versions

We release patches for security vulnerabilities for the following versions:

| Version | Supported          |
| ------- | ------------------ |
| Latest  | :white_check_mark: |
| < Latest| :x:                |

We recommend always using the latest version for security and feature updates.

## Reporting a Vulnerability

We take security vulnerabilities seriously. If you discover a security issue, please report it responsibly.

### How to Report

**Please DO NOT open a public GitHub issue for security vulnerabilities.**

Instead, please report security issues via email to the project maintainer. You can find contact information in the repository settings or commit history.

### What to Include

When reporting a vulnerability, please include:

1. **Description**: Clear description of the vulnerability
2. **Impact**: Potential impact and severity assessment
3. **Reproduction**: Step-by-step instructions to reproduce
4. **Environment**: Version, OS, Go version, etc.
5. **Proof of Concept**: Code or commands demonstrating the issue (if applicable)
6. **Suggested Fix**: If you have ideas for remediation (optional)

### Response Timeline

- **Acknowledgment**: Within 48 hours of report
- **Initial Assessment**: Within 7 days
- **Status Updates**: Every 14 days until resolved
- **Fix Release**: Depends on severity (Critical: <7 days, High: <30 days, Medium: <90 days)

## Security Considerations

### Data Privacy

**Browser History Access**: The tool can access browser history databases for domain extraction. Users should be aware:
- History data is only read, never modified
- Data is processed locally and not transmitted
- Temporary copies are cleaned up after use
- macOS requires Full Disk Access permissions

**Recommendation**: Use the `scripts/export_history.sh` helper to manually extract domains if you're concerned about granting permissions.

### Network Security

**TLS Verification**: By default, the benchmark uses `InsecureSkipVerify: true` for TLS connections to support benchmarking servers with IP addresses or self-signed certificates.

**Security Implication**: This means TLS certificate validation is bypassed, making the tool vulnerable to man-in-the-middle attacks during benchmarking.

**Mitigation**: This is acceptable for benchmarking purposes, but do not use this tool for production DNS resolution. Future versions may add a flag to enable strict TLS verification.

### Input Validation

The tool validates:
- Domain names (length, format, characters)
- Server addresses (IP, hostname, URL format)
- Port numbers (range 1-65535)

Invalid inputs are rejected with warnings.

### Dependencies

We regularly update dependencies to patch known vulnerabilities. Run `go mod tidy` and `go get -u ./...` to update dependencies.

To check for known vulnerabilities:
```bash
go install golang.org/x/vuln/cmd/govulncheck@latest
govulncheck ./...
```

## Known Security Limitations

### 1. CGO Dependency (go-sqlite3)

The `browser` package uses CGO for SQLite database access. This introduces potential security risks:
- Memory safety issues in C code
- Platform-specific vulnerabilities
- Harder to audit than pure Go

**Mitigation**: CGO is only required for browser history features. Cross-compiled binaries (`CGO_ENABLED=0`) disable this feature entirely.

### 2. No Rate Limiting

The tool can generate high volumes of DNS queries, which could:
- Trigger rate limiting on DNS servers
- Appear as a denial-of-service attack
- Violate terms of service for public resolvers

**Recommendation**: Use responsibly with appropriate concurrency limits.

### 3. DNS Spoofing

The tool does not implement DNSSEC validation, making it vulnerable to DNS spoofing/cache poisoning attacks.

**Impact**: Benchmark results could be affected by malicious DNS responses.

### 4. File System Access

The tool reads/writes files for:
- Configuration (YAML, CSV, TXT)
- Results export (CSV, HTML)
- Browser history (SQLite databases)

**Recommendation**: Only run with trusted input files and in directories you control.

## Security Best Practices

When using dns-bench:

1. **Run as non-root user** - No elevated privileges needed
2. **Validate input files** - Don't trust untrusted YAML/CSV files
3. **Review output** - Check exported CSV/HTML for sensitive data before sharing
4. **Limit concurrency** - Use reasonable values to avoid overwhelming servers
5. **Respect rate limits** - Be mindful of DNS server policies
6. **Use latest version** - Security fixes are only backported in rare cases

## Disclosure Policy

After a security fix is released:

1. We will publish a security advisory on GitHub
2. Credit will be given to the reporter (unless they prefer anonymity)
3. CVE will be requested for high/critical severity issues
4. A detailed postmortem may be published for significant issues

## Security Tooling

We use the following tools to maintain security:

- **golangci-lint**: Static analysis with gosec
- **govulncheck**: Dependency vulnerability scanning
- **GitHub Dependabot**: Automated dependency updates
- **CodeQL**: Automated code scanning (if enabled)

## Contact

For security concerns, please contact the project maintainers directly rather than opening public issues.

Thank you for helping keep dns-bench secure!
