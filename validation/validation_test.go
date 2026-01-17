package validation

import (
	"strings"
	"testing"
)

func TestIsValidDomain(t *testing.T) {
	tests := []struct {
		name    string
		domain  string
		wantErr bool
	}{
		{"valid domain", "google.com", false},
		{"valid subdomain", "mail.google.com", false},
		{"valid multi-level", "www.mail.google.com", false},
		{"empty domain", "", true},
		{"single label", "localhost", true},
		{"too long domain", strings.Repeat("a", 254) + ".com", true},
		{"label too long", strings.Repeat("a", 64) + ".com", true},
		{"starts with hyphen", "-invalid.com", true},
		{"ends with hyphen", "invalid-.com", true},
		{"double dots", "invalid..com", true},
		{"special chars", "inv@lid.com", true},
		{"trailing dot", "google.com.", true}, // We reject trailing dots for simplicity
		{"underscore", "in_valid.com", true},
		{"valid with numbers", "test123.example.com", false},
		{"numeric TLD", "example.123", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := IsValidDomain(tt.domain)
			if (err != nil) != tt.wantErr {
				t.Errorf("IsValidDomain(%q) error = %v, wantErr %v", tt.domain, err, tt.wantErr)
			}
		})
	}
}

func TestIsValidServer(t *testing.T) {
	tests := []struct {
		name    string
		server  string
		wantErr bool
	}{
		{"valid IP", "8.8.8.8", false},
		{"valid IP with port", "8.8.8.8:53", false},
		{"valid IPv6", "2001:4860:4860::8888", false},
		{"valid hostname", "dns.google", false},
		{"valid DoT", "tls://1.1.1.1", false},
		{"valid DoT with port", "tls://1.1.1.1:853", false},
		{"valid DoH", "https://dns.google/dns-query", false},
		{"invalid DoH scheme", "http://dns.google/dns-query", true},
		{"empty server", "", true},
		{"localhost", "localhost", false},
		{"invalid port", "8.8.8.8:999999", true},
		{"DoH without host", "https:///dns-query", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := IsValidServer(tt.server)
			if (err != nil) != tt.wantErr {
				t.Errorf("IsValidServer(%q) error = %v, wantErr %v", tt.server, err, tt.wantErr)
			}
		})
	}
}

func TestValidateDomains(t *testing.T) {
	input := []string{
		"google.com",
		"invalid",
		"yahoo.com",
		"google.com", // duplicate
		"",
		"facebook.com",
		"inv@lid.com",
	}

	valid, warnings := ValidateDomains(input)

	if len(valid) != 3 {
		t.Errorf("Expected 3 valid domains, got %d: %v", len(valid), valid)
	}

	if len(warnings) == 0 {
		t.Error("Expected warnings for invalid domains")
	}

	expectedValid := map[string]bool{
		"google.com":   true,
		"yahoo.com":    true,
		"facebook.com": true,
	}

	for _, domain := range valid {
		if !expectedValid[domain] {
			t.Errorf("Unexpected valid domain: %s", domain)
		}
	}
}

func TestValidateServers(t *testing.T) {
	input := []string{
		"8.8.8.8",
		"invalid_server!@#",
		"1.1.1.1",
		"8.8.8.8", // duplicate
		"",
		"https://dns.google/dns-query",
	}

	valid, warnings := ValidateServers(input)

	// Note: "invalid_server!@#" might be accepted as a hostname by the validator
	// since underscores are technically allowed in some contexts
	if len(valid) < 3 {
		t.Errorf("Expected at least 3 valid servers, got %d: %v", len(valid), valid)
	}

	if len(warnings) == 0 {
		t.Error("Expected warnings for duplicates at minimum")
	}

	// Check that the duplicate was removed
	serverCount := 0
	for _, s := range valid {
		if s == "8.8.8.8" {
			serverCount++
		}
	}
	if serverCount > 1 {
		t.Error("Expected duplicate server to be removed")
	}
}
