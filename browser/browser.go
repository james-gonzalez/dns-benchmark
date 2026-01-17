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

	// Import sqlite3 driver for database/sql
	_ "github.com/mattn/go-sqlite3"
)

// GetDomains extracts unique domains from the specified browser's history
func GetDomains(browserName string, limit int) ([]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get user home dir: %v", err)
	}

	var historyPath string
	var query string

	switch strings.ToLower(browserName) {
	case "chrome":
		historyPath = filepath.Join(home, "Library", "Application Support", "Google", "Chrome", "Default", "History")
		query = "SELECT url FROM urls ORDER BY last_visit_time DESC LIMIT ?"
	case "brave":
		historyPath = filepath.Join(home, "Library", "Application Support", "BraveSoftware", "Brave-Browser", "Default", "History")
		query = "SELECT url FROM urls ORDER BY last_visit_time DESC LIMIT ?"
	case "safari":
		historyPath = filepath.Join(home, "Library", "Safari", "History.db")
		query = "SELECT url FROM history_items ORDER BY visit_count DESC LIMIT ?"
	case "firefox":
		// Firefox profiles have random strings in the path. We need to find the correct profile.
		profilesPath := filepath.Join(home, "Library", "Application Support", "Firefox", "Profiles")
		matches, err := filepath.Glob(filepath.Join(profilesPath, "*.default-release"))
		if err == nil && len(matches) == 0 {
			// Try fallback for non-release builds or older setups
			matches, err = filepath.Glob(filepath.Join(profilesPath, "*.default"))
		}
		if err != nil || len(matches) == 0 {
			return nil, fmt.Errorf("could not locate Firefox profile")
		}
		if len(matches) > 0 {
			historyPath = filepath.Join(matches[0], "places.sqlite")
		}
		query = "SELECT url FROM moz_places ORDER BY last_visit_date DESC LIMIT ?"
	default:
		return nil, fmt.Errorf("unsupported browser: %s (options: chrome, brave, safari, firefox)", browserName)
	}

	if historyPath == "" {
		return nil, fmt.Errorf("could not locate history file for %s", browserName)
	}

	// Copy database to a temp file to avoid locks
	tempFile, err := os.CreateTemp("", "dns-bench-history-*.db")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %v", err)
	}
	tempPath := tempFile.Name()

	// Close and ensure cleanup
	if err := tempFile.Close(); err != nil {
		return nil, fmt.Errorf("failed to close temp file: %v", err)
	}
	defer func() {
		if err := os.Remove(tempPath); err != nil {
			// Log but don't fail on cleanup error
			fmt.Fprintf(os.Stderr, "Warning: failed to remove temp file: %v\n", err)
		}
	}()

	if err := copyFile(historyPath, tempPath); err != nil {
		return nil, fmt.Errorf("failed to copy history file (browser might be open?): %v", err)
	}

	// Open database
	db, err := sql.Open("sqlite3", tempPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to close database: %v\n", err)
		}
	}()

	// Execute query
	rows, err := db.Query(query, limit*10) // Fetch more than needed to account for dupes/non-hostnames
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
		if err == nil && u.Hostname() != "" {
			host := u.Hostname()

			// Filter out localhost, IPs, and local names
			if host == "localhost" || strings.Contains(host, "127.0.0.1") {
				continue
			}
			if net.ParseIP(host) != nil {
				continue
			}
			if !strings.Contains(host, ".") {
				continue // likely a local hostname like "router" or "macbook"
			}

			if _, exists := domainSet[host]; !exists {
				domainSet[host] = struct{}{}
				domains = append(domains, host)
				if len(domains) >= limit {
					break
				}
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
