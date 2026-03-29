//go:build !windows

package browser

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// resolveBrowser returns the history path and SQL query for the given browser
// on macOS / Linux.
func resolveBrowser(browserName string) (*browserConfig, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get user home dir: %v", err)
	}

	const chromiumQuery = "SELECT url FROM urls ORDER BY last_visit_time DESC LIMIT ?"
	const firefoxQuery = "SELECT url FROM moz_places ORDER BY last_visit_date DESC LIMIT ?"

	switch strings.ToLower(browserName) {
	case "chrome":
		return &browserConfig{
			historyPath: filepath.Join(home, "Library", "Application Support", "Google", "Chrome", "Default", "History"),
			query:       chromiumQuery,
		}, nil

	case "brave":
		return &browserConfig{
			historyPath: filepath.Join(home, "Library", "Application Support", "BraveSoftware", "Brave-Browser", "Default", "History"),
			query:       chromiumQuery,
		}, nil

	case "edge":
		return &browserConfig{
			historyPath: filepath.Join(home, "Library", "Application Support", "Microsoft Edge", "Default", "History"),
			query:       chromiumQuery,
		}, nil

	case "safari":
		return &browserConfig{
			historyPath: filepath.Join(home, "Library", "Safari", "History.db"),
			query:       "SELECT url FROM history_items ORDER BY visit_count DESC LIMIT ?",
		}, nil

	case "firefox":
		profilesPath := filepath.Join(home, "Library", "Application Support", "Firefox", "Profiles")
		path, err := findFirefoxProfile(profilesPath)
		if err != nil {
			return nil, err
		}
		return &browserConfig{historyPath: path, query: firefoxQuery}, nil

	default:
		return nil, fmt.Errorf("unsupported browser: %s (options: chrome, brave, edge, safari, firefox)", browserName)
	}
}
