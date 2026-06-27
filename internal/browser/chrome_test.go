package browser

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectProfileDirectoryUsesSingleClonedProfile(t *testing.T) {
	dir := t.TempDir()
	mustWritePreferences(t, filepath.Join(dir, "Default", "Preferences"))
	mustWritePreferences(t, filepath.Join(dir, "Profile 1", "Preferences"))

	if got := DetectProfileDirectory(dir); got != "Profile 1" {
		t.Fatalf("DetectProfileDirectory() = %q, want Profile 1", got)
	}
}

func TestDetectProfileDirectoryRequiresExactlyOneClonedProfile(t *testing.T) {
	dir := t.TempDir()
	mustWritePreferences(t, filepath.Join(dir, "Profile 1", "Preferences"))
	mustWritePreferences(t, filepath.Join(dir, "Profile 2", "Preferences"))

	if got := DetectProfileDirectory(dir); got != "" {
		t.Fatalf("DetectProfileDirectory() = %q, want empty profile directory", got)
	}
}

func mustWritePreferences(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
}
