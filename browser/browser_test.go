package browser

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetDomainsUnsupportedBrowser(t *testing.T) {
	_, err := GetDomains("unsupported-browser", 10)
	if err == nil {
		t.Error("Expected error for unsupported browser")
	}
	if err != nil && !contains(err.Error(), "unsupported browser") {
		t.Errorf("Expected unsupported browser error, got: %v", err)
	}
}

func TestGetDomainsInvalidPath(t *testing.T) {
	// Create a temporary directory to use as fake home
	tmpDir := t.TempDir()

	// Set HOME to temp directory
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	// Try to get domains from Safari (file won't exist)
	_, err := GetDomains("safari", 10)
	if err == nil {
		t.Error("Expected error when history file doesn't exist")
	}
}

func TestCopyFile(t *testing.T) {
	// Create a temporary source file
	tmpDir := t.TempDir()
	srcFile := filepath.Join(tmpDir, "source.txt")
	dstFile := filepath.Join(tmpDir, "dest.txt")

	content := []byte("test content for copy")
	if err := os.WriteFile(srcFile, content, 0600); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Test successful copy
	if err := copyFile(srcFile, dstFile); err != nil {
		t.Errorf("copyFile failed: %v", err)
	}

	// Verify content
	gotContent, err := os.ReadFile(dstFile)
	if err != nil {
		t.Fatalf("Failed to read destination file: %v", err)
	}

	if string(gotContent) != string(content) {
		t.Errorf("Content mismatch: got %q, want %q", gotContent, content)
	}
}

func TestCopyFileNonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	srcFile := filepath.Join(tmpDir, "nonexistent.txt")
	dstFile := filepath.Join(tmpDir, "dest.txt")

	err := copyFile(srcFile, dstFile)
	if err == nil {
		t.Error("Expected error when copying non-existent file")
	}
}

func TestCopyFileInvalidDestination(t *testing.T) {
	tmpDir := t.TempDir()
	srcFile := filepath.Join(tmpDir, "source.txt")

	content := []byte("test content")
	if err := os.WriteFile(srcFile, content, 0600); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Try to copy to invalid destination (directory that doesn't exist)
	invalidDst := filepath.Join(tmpDir, "nonexistent", "dest.txt")
	err := copyFile(srcFile, invalidDst)
	if err == nil {
		t.Error("Expected error when destination directory doesn't exist")
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
