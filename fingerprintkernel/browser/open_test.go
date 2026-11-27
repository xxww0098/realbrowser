package browser

import (
	"os"
	"path/filepath"
	"testing"
)

func TestManagedBrowserOpenRejectsInvalidStartupURL(t *testing.T) {
	b := New(t.TempDir())
	profile := filepath.Join(t.TempDir(), "new-profile")
	if _, err := b.Open(t.Context(), profile, "--user-data-dir=other"); err == nil {
		t.Fatal("browser treated a switch as a startup URL")
	}
	if _, err := os.Stat(profile); !os.IsNotExist(err) {
		t.Fatalf("invalid request initialized a profile: %v", err)
	}
}
