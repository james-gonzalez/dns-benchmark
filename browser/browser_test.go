package browser

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// ── resolveBrowser tests ──────────────────────────────────────────────────────

func TestResolveBrowserUnsupported(t *testing.T) {
	_, err := resolveBrowser("netscape")
	if err == nil {
		t.Fatal("expected error for unsupported browser, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported browser") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestResolveBrowserChrome(t *testing.T) {
	cfg, err := resolveBrowser("chrome")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.historyPath == "" {
		t.Error("expected non-empty historyPath")
	}
	if cfg.query == "" {
		t.Error("expected non-empty query")
	}
	assertChromiumPath(t, cfg.historyPath, "Chrome")
}

func TestResolveBrowserBrave(t *testing.T) {
	cfg, err := resolveBrowser("brave")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertChromiumPath(t, cfg.historyPath, "Brave")
}

func TestResolveBrowserEdge(t *testing.T) {
	cfg, err := resolveBrowser("edge")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertChromiumPath(t, cfg.historyPath, "Edge")
}

func TestResolveBrowserFirefox(t *testing.T) {
	// Firefox requires a real profile directory — just check the error path
	// when no profile exists (the common test environment case).
	_, err := resolveBrowser("firefox")
	// Either succeeds (profile found) or fails with a profile-not-found message.
	if err != nil && !strings.Contains(err.Error(), "Firefox") && !strings.Contains(err.Error(), "profile") {
		t.Errorf("unexpected error for firefox: %v", err)
	}
}

// assertChromiumPath checks that the resolved path contains the expected vendor
// string and ends with "History", regardless of OS.
func assertChromiumPath(t *testing.T, path, vendor string) {
	t.Helper()
	if !strings.Contains(path, vendor) {
		t.Errorf("expected path to contain %q, got: %s", vendor, path)
	}
	if filepath.Base(path) != "History" {
		t.Errorf("expected path to end with 'History', got: %s", filepath.Base(path))
	}
}

// ── GetDomains integration tests ─────────────────────────────────────────────

func TestGetDomainsUnsupportedBrowser(t *testing.T) {
	_, err := GetDomains("unsupported-browser", 10)
	if err == nil {
		t.Fatal("expected error for unsupported browser")
	}
	if !strings.Contains(err.Error(), "unsupported browser") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestGetDomainsInvalidPath(t *testing.T) {
	// Point HOME / USERPROFILE at an empty temp dir so no history file exists.
	tmpDir := t.TempDir()

	if runtime.GOOS == "windows" {
		t.Setenv("LOCALAPPDATA", tmpDir)
		t.Setenv("APPDATA", tmpDir)
	} else {
		t.Setenv("HOME", tmpDir)
	}

	// Safari only exists on non-Windows; use chrome on Windows.
	browser := "safari"
	if runtime.GOOS == "windows" {
		browser = "chrome"
	}

	_, err := GetDomains(browser, 10)
	if err == nil {
		t.Error("expected error when history file doesn't exist")
	}
}

// ── findFirefoxProfile tests ──────────────────────────────────────────────────

func TestFindFirefoxProfileDefaultRelease(t *testing.T) {
	tmpDir := t.TempDir()
	profileDir := filepath.Join(tmpDir, "abc123.default-release")
	if err := os.MkdirAll(profileDir, 0755); err != nil {
		t.Fatal(err)
	}
	// Create a fake places.sqlite
	if err := os.WriteFile(filepath.Join(profileDir, "places.sqlite"), []byte{}, 0600); err != nil {
		t.Fatal(err)
	}

	path, err := findFirefoxProfile(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasSuffix(path, "places.sqlite") {
		t.Errorf("expected path ending in places.sqlite, got: %s", path)
	}
}

func TestFindFirefoxProfileDefault(t *testing.T) {
	tmpDir := t.TempDir()
	profileDir := filepath.Join(tmpDir, "xyz789.default")
	if err := os.MkdirAll(profileDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(profileDir, "places.sqlite"), []byte{}, 0600); err != nil {
		t.Fatal(err)
	}

	path, err := findFirefoxProfile(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasSuffix(path, "places.sqlite") {
		t.Errorf("expected path ending in places.sqlite, got: %s", path)
	}
}

func TestFindFirefoxProfileNotFound(t *testing.T) {
	tmpDir := t.TempDir() // empty — no profiles
	_, err := findFirefoxProfile(tmpDir)
	if err == nil {
		t.Error("expected error when no Firefox profile exists")
	}
}

// ── copyFile tests ────────────────────────────────────────────────────────────

func TestCopyFile(t *testing.T) {
	tmpDir := t.TempDir()
	srcFile := filepath.Join(tmpDir, "source.txt")
	dstFile := filepath.Join(tmpDir, "dest.txt")

	content := []byte("test content for copy")
	if err := os.WriteFile(srcFile, content, 0600); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	if err := copyFile(srcFile, dstFile); err != nil {
		t.Fatalf("copyFile failed: %v", err)
	}

	got, err := os.ReadFile(dstFile)
	if err != nil {
		t.Fatalf("failed to read destination file: %v", err)
	}
	if string(got) != string(content) {
		t.Errorf("content mismatch: got %q, want %q", got, content)
	}
}

func TestCopyFileNonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	err := copyFile(filepath.Join(tmpDir, "nonexistent.txt"), filepath.Join(tmpDir, "dest.txt"))
	if err == nil {
		t.Error("expected error when copying non-existent file")
	}
}

func TestCopyFileInvalidDestination(t *testing.T) {
	tmpDir := t.TempDir()
	srcFile := filepath.Join(tmpDir, "source.txt")
	if err := os.WriteFile(srcFile, []byte("data"), 0600); err != nil {
		t.Fatal(err)
	}

	err := copyFile(srcFile, filepath.Join(tmpDir, "nonexistent-dir", "dest.txt"))
	if err == nil {
		t.Error("expected error when destination directory doesn't exist")
	}
}
