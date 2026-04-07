package browser

import (
	"database/sql"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	// Import sqlite driver for database/sql (pure Go, no CGO required)
	_ "modernc.org/sqlite"
)

// browserConfig holds the resolved path and query for a browser
type browserConfig struct {
	historyPath string
	query       string
}

// GetDomains extracts unique domains from the specified browser's history
func GetDomains(browserName string, limit int) ([]string, error) {
	cfg, err := resolveBrowser(browserName)
	if err != nil {
		return nil, err
	}

	if cfg.historyPath == "" {
		return nil, fmt.Errorf("could not locate history file for %s", browserName)
	}

	// Copy database to a temp file to avoid locks
	tempFile, err := os.CreateTemp("", "dns-bench-history-*.db")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %v", err)
	}
	tempPath := tempFile.Name()

	if err := tempFile.Close(); err != nil {
		return nil, fmt.Errorf("failed to close temp file: %v", err)
	}
	defer func() {
		if err := os.Remove(tempPath); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to remove temp file: %v\n", err)
		}
	}()

	if err := copyFile(cfg.historyPath, tempPath); err != nil {
		return nil, fmt.Errorf("failed to copy history file (browser might be open?): %v", err)
	}

	db, err := sql.Open("sqlite", tempPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to close database: %v\n", err)
		}
	}()

	// Fetch more than needed to account for duplicates and non-hostname URLs
	rows, err := db.Query(cfg.query, limit*10)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %v", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to close rows: %v\n", err)
		}
	}()

	domainSet := make(map[string]struct{})
	var domains []string

	for rows.Next() {
		var rawURL string
		if err := rows.Scan(&rawURL); err != nil {
			continue
		}

		u, err := url.Parse(rawURL)
		if err != nil || u.Hostname() == "" {
			continue
		}
		host := u.Hostname()

		if host == "localhost" || strings.Contains(host, "127.0.0.1") {
			continue
		}
		if net.ParseIP(host) != nil {
			continue
		}
		if !strings.Contains(host, ".") {
			continue
		}

		if _, exists := domainSet[host]; !exists {
			domainSet[host] = struct{}{}
			domains = append(domains, host)
			if len(domains) >= limit {
				break
			}
		}
	}

	return domains, nil
}

func copyFile(src, dst string) error {
	sourceFileStat, err := os.Stat(src)
	if err != nil {
		return err
	}

	if !sourceFileStat.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file", src)
	}

	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() {
		if err := source.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to close source file: %v\n", err)
		}
	}()

	destination, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() {
		if err := destination.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to close destination file: %v\n", err)
		}
	}()

	_, err = io.Copy(destination, source)
	return err
}

// findFirefoxProfile returns the path to the most likely Firefox places.sqlite
// by globbing for *.default-release then *.default profiles.
func findFirefoxProfile(profilesPath string) (string, error) {
	for _, pattern := range []string{"*.default-release", "*.default"} {
		matches, err := filepath.Glob(filepath.Join(profilesPath, pattern))
		if err == nil && len(matches) > 0 {
			return filepath.Join(matches[0], "places.sqlite"), nil
		}
	}
	return "", fmt.Errorf("could not locate Firefox profile in %s", profilesPath)
}
