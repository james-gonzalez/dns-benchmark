//go:build windows

package browser

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// resolveBrowser returns the history path and SQL query for the given browser
// on Windows.
//
// Chrome, Brave, Edge, and Opera all use the Chromium engine and store history
// at %LOCALAPPDATA%\<vendor>\<app>\User Data\Default\History.
// Firefox uses %APPDATA%\Mozilla\Firefox\Profiles\<profile>\places.sqlite.
func resolveBrowser(browserName string) (*browserConfig, error) {
	localAppData := os.Getenv("LOCALAPPDATA")
	appData := os.Getenv("APPDATA")

	if localAppData == "" || appData == "" {
		// Fall back to UserHomeDir-relative paths for non-standard setups
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get user home dir: %v", err)
		}
		if localAppData == "" {
			localAppData = filepath.Join(home, "AppData", "Local")
		}
		if appData == "" {
			appData = filepath.Join(home, "AppData", "Roaming")
		}
	}

	const chromiumQuery = "SELECT url FROM urls ORDER BY last_visit_time DESC LIMIT ?"
	const firefoxQuery = "SELECT url FROM moz_places ORDER BY last_visit_date DESC LIMIT ?"

	switch strings.ToLower(browserName) {
	case "chrome":
		return &browserConfig{
			historyPath: filepath.Join(localAppData, "Google", "Chrome", "User Data", "Default", "History"),
			query:       chromiumQuery,
		}, nil

	case "brave":
		return &browserConfig{
			historyPath: filepath.Join(localAppData, "BraveSoftware", "Brave-Browser", "User Data", "Default", "History"),
			query:       chromiumQuery,
		}, nil

	case "edge":
		return &browserConfig{
			historyPath: filepath.Join(localAppData, "Microsoft", "Edge", "User Data", "Default", "History"),
			query:       chromiumQuery,
		}, nil

	case "opera":
		return &browserConfig{
			historyPath: filepath.Join(appData, "Opera Software", "Opera Stable", "History"),
			query:       chromiumQuery,
		}, nil

	case "firefox":
		profilesPath := filepath.Join(appData, "Mozilla", "Firefox", "Profiles")
		path, err := findFirefoxProfile(profilesPath)
		if err != nil {
			return nil, err
		}
		return &browserConfig{historyPath: path, query: firefoxQuery}, nil

	default:
		return nil, fmt.Errorf("unsupported browser: %s (options: chrome, brave, edge, opera, firefox)", browserName)
	}
}
