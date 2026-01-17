// Package validation provides input validation utilities for DNS benchmark
package validation

import (
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strings"
)

const (
	maxDomainLength = 253
	maxLabelLength  = 63
)

var (
	// Domain name regex: allows letters, numbers, hyphens, and dots
	domainRegex = regexp.MustCompile(`^([a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,}$`)
)

// IsValidDomain checks if a domain name is valid according to DNS standards
func IsValidDomain(domain string) error {
	if domain == "" {
		return fmt.Errorf("domain cannot be empty")
	}

	// Check total length
	if len(domain) > maxDomainLength {
		return fmt.Errorf("domain exceeds maximum length of %d characters", maxDomainLength)
	}

	// Check each label
	labels := strings.Split(domain, ".")
	if len(labels) < 2 {
		return fmt.Errorf("domain must have at least two labels (e.g., example.com)")
	}

	for _, label := range labels {
		if len(label) == 0 {
			return fmt.Errorf("domain contains empty label")
		}
		if len(label) > maxLabelLength {
			return fmt.Errorf("domain label '%s' exceeds maximum length of %d", label, maxLabelLength)
		}
		if label[0] == '-' || label[len(label)-1] == '-' {
			return fmt.Errorf("domain label '%s' cannot start or end with hyphen", label)
		}
	}

	// Basic regex validation
	if !domainRegex.MatchString(domain) {
		return fmt.Errorf("invalid domain format: %s", domain)
	}

	return nil
}

// IsValidServer checks if a server address is valid
func IsValidServer(server string) error {
	if server == "" {
		return fmt.Errorf("server cannot be empty")
	}

	// Handle DoH (HTTPS)
	if strings.HasPrefix(server, "https://") {
		u, err := url.Parse(server)
		if err != nil {
			return fmt.Errorf("invalid DoH URL: %w", err)
		}
		if u.Scheme != "https" {
			return fmt.Errorf("DoH URL must use https scheme")
		}
		if u.Host == "" {
			return fmt.Errorf("DoH URL must have a host")
		}
		return nil
	}

	// Handle DoT (TLS)
	if strings.HasPrefix(server, "tls://") {
		host := strings.TrimPrefix(server, "tls://")
		return validateHostPort(host, 853)
	}

	// Handle standard UDP/TCP
	return validateHostPort(server, 53)
}

// validateHostPort validates a host:port or just host string
func validateHostPort(hostPort string, _ int) error {
	host, port, err := net.SplitHostPort(hostPort)
	if err != nil {
		// No port specified, just validate host
		host = hostPort
		port = ""
	}

	// Validate host is either valid IP or domain
	if ip := net.ParseIP(host); ip != nil {
		// Valid IP address
		if port != "" {
			// Validate port range
			portNum, err := net.LookupPort("tcp", port)
			if err != nil || portNum < 1 || portNum > 65535 {
				return fmt.Errorf("invalid port: %s", port)
			}
		}
		return nil
	}

	// Check if it's a valid domain/hostname
	if host == "localhost" {
		return nil
	}

	// For hostnames, do basic validation
	if len(host) == 0 {
		return fmt.Errorf("host cannot be empty")
	}
	if len(host) > maxDomainLength {
		return fmt.Errorf("host exceeds maximum length")
	}

	// If port is specified, validate it
	if port != "" {
		portNum, err := net.LookupPort("tcp", port)
		if err != nil || portNum < 1 || portNum > 65535 {
			return fmt.Errorf("invalid port: %s", port)
		}
	}

	return nil
}

// ValidateDomains validates a list of domains and returns only valid ones with warnings
func ValidateDomains(domains []string) ([]string, []string) {
	valid := make([]string, 0, len(domains))
	warnings := make([]string, 0)

	seen := make(map[string]bool)
	for _, domain := range domains {
		domain = strings.TrimSpace(strings.ToLower(domain))
		if domain == "" {
			continue
		}

		// Check for duplicates
		if seen[domain] {
			warnings = append(warnings, fmt.Sprintf("duplicate domain ignored: %s", domain))
			continue
		}
		seen[domain] = true

		// Validate domain
		if err := IsValidDomain(domain); err != nil {
			warnings = append(warnings, fmt.Sprintf("invalid domain '%s': %v", domain, err))
			continue
		}

		valid = append(valid, domain)
	}

	return valid, warnings
}

// ValidateServers validates a list of servers and returns only valid ones with warnings
func ValidateServers(servers []string) ([]string, []string) {
	valid := make([]string, 0, len(servers))
	warnings := make([]string, 0)

	seen := make(map[string]bool)
	for _, server := range servers {
		server = strings.TrimSpace(server)
		if server == "" {
			continue
		}

		// Check for duplicates
		if seen[server] {
			warnings = append(warnings, fmt.Sprintf("duplicate server ignored: %s", server))
			continue
		}
		seen[server] = true

		// Validate server
		if err := IsValidServer(server); err != nil {
			warnings = append(warnings, fmt.Sprintf("invalid server '%s': %v", server, err))
			continue
		}

		valid = append(valid, server)
	}

	return valid, warnings
}
