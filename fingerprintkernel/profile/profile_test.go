package profile

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestProfileSaveRetainsUnknownFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".browserkernel.json")
	const initial = `{"Lang":"en-US","timezone":"UTC","future":{"counter":9007199254740993},"permissions":{"future_mode":"existing"}}`
	if err := os.WriteFile(path, []byte(initial), 0o600); err != nil {
		t.Fatal(err)
	}
	spec, err := ReadProfile(dir)
	if err != nil {
		t.Fatal(err)
	}
	spec.Lang, spec.Timezone = "", "Europe/Paris"
	if err := WriteProfile(dir, spec); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Lang        string
		Timezone    string
		Future      struct{ Counter int64 }
		Permissions map[string]string
	}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.Lang != "" || got.Timezone != "Europe/Paris" || got.Future.Counter != 9007199254740993 || got.Permissions["future_mode"] != "existing" {
		t.Fatalf("saved preferences: %s", data)
	}
}

func TestProfileEditsUseLatestFile(t *testing.T) {
	dir := t.TempDir()
	entered := make(chan struct{})
	release := make(chan struct{})
	first := make(chan error, 1)
	go func() {
		first <- EditProfile(t.Context(), dir, func(f *FingerprintSpec) error {
			close(entered)
			<-release
			f.Lang = "en-GB"
			return nil
		})
	}()
	select {
	case <-entered:
	case err := <-first:
		t.Fatalf("first edit failed before reading the profile: %v", err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()
	err := EditProfile(ctx, dir, func(f *FingerprintSpec) error { return errors.New("unexpected concurrent edit") })
	close(release)
	if firstErr := <-first; firstErr != nil || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("edits returned %v and %v", firstErr, err)
	}
	if err := EditProfile(t.Context(), dir, func(f *FingerprintSpec) error {
		f.Timezone = "UTC"
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	f, err := ReadProfile(dir)
	if err != nil || f.Lang != "en-GB" || f.Timezone != "UTC" {
		t.Fatalf("final profile: %+v, %v", f, err)
	}
}

func TestProfileEditFailurePreservesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".browserkernel.json")
	original := `{"lang":"en-GB","future_key":true}`
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	editErr := errors.New("discard edit")
	err := EditProfile(t.Context(), dir, func(f *FingerprintSpec) error {
		f.Lang = "de-DE"
		return editErr
	})
	if !errors.Is(err, editErr) {
		t.Fatalf("unexpected edit result: %v", err)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != original {
		t.Fatalf("discarded edit changed the file: %q, %v", data, err)
	}
}

func TestFingerprintPreferencesRoundTrip(t *testing.T) {
	dir := t.TempDir()
	f, err := ReadProfile(filepath.Join(dir, "new"))
	if err != nil || f.Lang != "" {
		t.Fatalf("legacy defaults: %+v, %v", f, err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".browserkernel.json"), []byte(`{"lang":"en-GB"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if f, err = ReadProfile(dir); err != nil || f.Lang != "en-GB" {
		t.Fatalf("existing profile: %+v, %v", f, err)
	}
	f.Timezone, f.WindowWidth, f.WindowHeight = "Europe/London", 720, 480
	f.ColorScheme = ColorSchemeDark
	if err = WriteProfile(dir, f); err != nil {
		t.Fatal(err)
	}
	got, err := ReadProfile(dir)
	if err != nil || got.Lang != "en-GB" || got.Timezone != f.Timezone || got.WindowWidth != 720 || got.ColorScheme != ColorSchemeDark {
		t.Fatalf("read preferences: %+v %v", got, err)
	}
	f.WindowHeight = -1
	if err = WriteProfile(dir, f); err == nil {
		t.Fatal("saved invalid dimensions")
	}
	if got, err = ReadProfile(dir); err != nil || got.WindowHeight != 480 {
		t.Fatalf("previous preferences lost: %+v %v", got, err)
	}
}
