package profile

import (
	"context"
	"strings"
)

// DisplayPreferences contains the editable language, timezone and window
// settings used by desktop browser hosts. Empty fields restore defaults.
type DisplayPreferences struct {
	Lang         string `json:"lang"`
	Timezone     string `json:"timezone"`
	WindowWidth  int    `json:"windowWidth"`
	WindowHeight int    `json:"windowHeight"`
	ColorScheme  string `json:"colorScheme"`
}

// ReadDisplayPreferences reads the editable display settings for a profile.
func ReadDisplayPreferences(dir string) (DisplayPreferences, error) {
	f, err := ReadProfile(dir)
	if err != nil {
		return DisplayPreferences{}, err
	}
	return DisplayPreferences{Lang: f.Lang, Timezone: f.Timezone, WindowWidth: f.WindowWidth, WindowHeight: f.WindowHeight, ColorScheme: string(f.ColorScheme)}, nil
}

// Save updates just the display fields under the profile lock. Validation
// and serialization use the same path as all other profile edits.
func (p DisplayPreferences) Save(ctx context.Context, dir string) error {
	return EditProfile(ctx, dir, func(f *FingerprintSpec) error {
		f.Lang, f.Timezone = strings.TrimSpace(p.Lang), strings.TrimSpace(p.Timezone)
		f.WindowWidth, f.WindowHeight = p.WindowWidth, p.WindowHeight
		f.ColorScheme = ColorSchemeMode(p.ColorScheme)
		return nil
	})
}
