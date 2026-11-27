package fingerprintkernel_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	kernel "github.com/xxww0098/realbrowser/fingerprintkernel"
)

func TestSharedProfileEditor(t *testing.T) {
	dir := t.TempDir()
	initial := kernel.DefaultFingerprint(kernel.PlatformLinux, kernel.BrandChrome)
	changed := kernel.ApplyFingerprintConfig(initial, map[string]any{"language": []string{"de-DE", "de"}, "screen_resolution": "1600x900"})
	if changed.Seed != initial.Seed || changed.Lang != "de-DE" || changed.WindowWidth != 1600 || changed.WindowHeight != 900 {
		t.Fatalf("partial edit lost fields: %+v", changed)
	}
	if err := kernel.WriteProfile(dir, changed); err != nil {
		t.Fatal(err)
	}
	p, err := kernel.ReadDisplayPreferences(dir)
	if err != nil {
		t.Fatal(err)
	}
	p.Lang, p.Timezone, p.ColorScheme = " ja-JP ", " Asia/Tokyo ", "dark"
	if err := p.Save(t.Context(), dir); err != nil {
		t.Fatal(err)
	}
	saved, err := kernel.ReadProfile(dir)
	if err != nil || saved.Seed != initial.Seed || saved.Lang != "ja-JP" || saved.Timezone != "Asia/Tokyo" {
		t.Fatalf("display edit: %+v %v", saved, err)
	}
	p.Timezone = "invalid/timezone"
	if err := p.Save(t.Context(), dir); err == nil {
		t.Fatal("invalid edit was saved")
	}
	saved, err = kernel.ReadProfile(dir)
	if err != nil || saved.Timezone != "Asia/Tokyo" {
		t.Fatalf("invalid edit changed profile: %+v %v", saved, err)
	}

	prefsDir := filepath.Join(dir, "Default")
	if err := os.MkdirAll(prefsDir, 0700); err != nil {
		t.Fatal(err)
	}
	prefsPath := filepath.Join(prefsDir, "Preferences")
	if err := os.WriteFile(prefsPath, []byte(`{"extension":{"wide":9007199254740993}}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := saved.WriteChromiumPreferences(t.Context(), dir, false); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(prefsPath)
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Extension struct{ Wide json.Number }
		Intl      struct {
			AcceptLanguages string `json:"accept_languages"`
		}
	}
	if err := json.Unmarshal(raw, &doc); err != nil || doc.Extension.Wide.String() != "9007199254740993" || doc.Intl.AcceptLanguages == "" {
		t.Fatalf("native preference merge: %s, %v", raw, err)
	}
}

func TestSharedProfileEditorUpgradesLegacyScreen(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".browserkernel.json"), []byte(`{"seed":42,"future":{"value":9007199254740993}}`), 0600); err != nil {
		t.Fatal(err)
	}
	ready, err := kernel.EnsureProfile(t.Context(), dir, kernel.EnsureOptions{})
	if err != nil {
		t.Fatal(err)
	}
	saved, err := kernel.ReadProfile(dir)
	if err != nil || saved.Screen == nil || ready.Screen == nil || *saved.Screen != *ready.Screen || saved.Seed != 42 {
		t.Fatalf("incomplete persisted upgrade: %+v %v", saved, err)
	}
	x, y := 20, 45
	saved.Screen.WindowX, saved.Screen.WindowY = &x, &y
	detached := kernel.ApplyFingerprintConfig(saved, map[string]any{"screen": map[string]any{"window_x": 30}})
	if detached.Screen == nil || detached.Screen.WindowX == nil || *detached.Screen.WindowX != 30 || *saved.Screen.WindowX != 20 {
		t.Fatalf("screen patch changed source or lost a field: %+v", detached.Screen)
	}
	if got := kernel.ApplyFingerprintConfig(saved, map[string]any{"screen": nil}); got.Screen != nil {
		t.Fatal("screen override was not cleared")
	}
}
