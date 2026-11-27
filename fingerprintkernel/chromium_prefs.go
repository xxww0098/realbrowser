package fingerprintkernel

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ChromiumPreferences constructs the minimal set of native Chromium C++
// Default/Preferences keys for this profile. These are consumed directly by
// Chromium's PrefService without any JavaScript prototype tampering.
func (fp FingerprintSpec) ChromiumPreferences(proxied bool) map[string]any {
	webrtcPol := fp.EffectiveWebRTCPolicy()

	webrtcMap := map[string]any{
		"ip_handling_policy":      string(webrtcPol),
		"multiple_routes_enabled": webrtcPol == WebRTCDefault || webrtcPol == WebRTCPublicAndPrivate,
		"nonproxied_udp_enabled":  webrtcPol != WebRTCDisableNonProxiedUDP,
	}

	patch := map[string]any{
		"webrtc":              webrtcMap,
		"enable_do_not_track": fp.EffectiveDoNotTrack() == DoNotTrackOn,
	}

	acceptLang := strings.TrimSpace(fp.AcceptLang)
	if acceptLang == "" {
		acceptLang = strings.TrimSpace(fp.Lang)
	}
	if acceptLang != "" {
		patch["intl"] = map[string]any{
			"accept_languages":   acceptLang,
			"selected_languages": acceptLang,
		}
	}

	// Suppress local DNS prefetching & speculative preconnect when proxied + DNS protection enabled.
	if proxied && fp.EffectiveDNSLeakMode() == DNSLeakAuto {
		patch["net"] = map[string]any{
			"network_prediction_options": 2, // NetworkPredictionOptions::kNever
		}
		patch["dns_prefetching"] = map[string]any{
			"enabled": false,
		}
	}

	// Content permissions (1 = allow, 2 = block; prompt omits the override key).
	contentSettings := map[string]any{}
	if geoMode := fp.Geolocation.EffectiveMode(); geoMode != PermissionPrompt {
		contentSettings["geolocation"] = permissionModeToInt(geoMode)
	}
	if notif := NormalizePermissionMode(string(fp.Permissions.Notifications)); notif != "" && notif != PermissionPrompt {
		contentSettings["notifications"] = permissionModeToInt(notif)
	}
	if media := NormalizePermissionMode(string(fp.Permissions.MediaDevices)); media != "" && media != PermissionPrompt {
		val := permissionModeToInt(media)
		contentSettings["media_stream_mic"] = val
		contentSettings["media_stream_camera"] = val
	}
	if sensors := NormalizePermissionMode(string(fp.Permissions.Sensors)); sensors != "" && sensors != PermissionPrompt {
		contentSettings["sensors"] = permissionModeToInt(sensors)
	}
	if hw := NormalizePermissionMode(string(fp.Permissions.HardwareGuard)); hw == PermissionBlock {
		contentSettings["usb_guard"] = 2
		contentSettings["hid_guard"] = 2
		contentSettings["bluetooth_guard"] = 2
		contentSettings["serial_guard"] = 2
	}

	profileMap := map[string]any{
		"exit_type":      "Normal",
		"exited_cleanly": true,
	}
	if len(contentSettings) > 0 {
		profileMap["default_content_setting_values"] = contentSettings
	}
	if fp.DisablePasswordPopup {
		patch["credentials_enable_service"] = false
		profileMap["password_manager_enabled"] = false
	}
	patch["profile"] = profileMap

	return patch
}

func permissionModeToInt(m PermissionMode) int {
	switch m {
	case PermissionAllow:
		return 1
	case PermissionBlock:
		return 2
	default:
		return 0
	}
}

// WriteChromiumPreferences atomically deep-merges native Chromium privacy & anti-leak
// preferences into <userDataDir>/Default/Preferences and ensures the "First Run"
// sentinel file exists so bookmarks, cookies, extensions, and existing user settings
// are preserved losslessly. Managed override keys reset back to defaults (e.g.
// PermissionPrompt, DisablePasswordPopup=false, or DNSLeakSystem) are cleanly cleared.
func (fp FingerprintSpec) WriteChromiumPreferences(ctx context.Context, userDataDir string, proxied bool) error {
	if strings.TrimSpace(userDataDir) == "" {
		return nil
	}
	if err := fp.Validate(); err != nil {
		return err
	}
	defaultDir := filepath.Join(userDataDir, "Default")
	if err := os.MkdirAll(defaultDir, 0o755); err != nil {
		return fmt.Errorf("fpchromium: create Default profile dir: %w", err)
	}

	lock, err := acquireFileLock(ctx, filepath.Join(defaultDir, ".fingerprintkernel-prefs.lock"), nil)
	if err != nil {
		return err
	}
	defer ReleaseInstallLock(lock)

	// Ensure First Run sentinel exists (additional safeguard against first-run dialogs).
	firstRunPath := filepath.Join(userDataDir, "First Run")
	if _, err := os.Stat(firstRunPath); os.IsNotExist(err) {
		_ = os.WriteFile(firstRunPath, nil, 0o644)
	}

	prefsPath := filepath.Join(defaultDir, "Preferences")
	existing := map[string]any{}
	raw, err := os.ReadFile(prefsPath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if json.Valid(raw) {
		var parsed map[string]any
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.UseNumber()
		if decoder.Decode(&parsed) == nil && parsed != nil {
			existing = parsed
		}
	}

	fp.reconcileClearedManagedPrefs(existing, proxied)
	deepMergeMap(existing, fp.ChromiumPreferences(proxied))

	encoded, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return fmt.Errorf("fpchromium: marshal Preferences: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := writeAtomic(defaultDir, "Preferences", encoded); err != nil {
		return fmt.Errorf("fpchromium: commit Preferences: %w", err)
	}
	return nil
}

// reconcileClearedManagedPrefs deletes previously written managed keys when a
// profile setting is switched back to its default/off state, preventing stale
// overrides from lingering across profile edits.
func (fp FingerprintSpec) reconcileClearedManagedPrefs(existing map[string]any, proxied bool) {
	if !(proxied && fp.EffectiveDNSLeakMode() == DNSLeakAuto) {
		deleteNestedKey(existing, "net", "network_prediction_options")
		deleteNestedKey(existing, "dns_prefetching", "enabled")
	}
	if !fp.DisablePasswordPopup {
		delete(existing, "credentials_enable_service")
		deleteNestedKey(existing, "profile", "password_manager_enabled")
	}
	if fp.Geolocation.EffectiveMode() == PermissionPrompt {
		deleteNestedKey(existing, "profile", "default_content_setting_values", "geolocation")
	}
	if notif := NormalizePermissionMode(string(fp.Permissions.Notifications)); notif == "" || notif == PermissionPrompt {
		deleteNestedKey(existing, "profile", "default_content_setting_values", "notifications")
	}
	if media := NormalizePermissionMode(string(fp.Permissions.MediaDevices)); media == "" || media == PermissionPrompt {
		deleteNestedKey(existing, "profile", "default_content_setting_values", "media_stream_mic")
		deleteNestedKey(existing, "profile", "default_content_setting_values", "media_stream_camera")
	}
	if sensors := NormalizePermissionMode(string(fp.Permissions.Sensors)); sensors == "" || sensors == PermissionPrompt {
		deleteNestedKey(existing, "profile", "default_content_setting_values", "sensors")
	}
	if hw := NormalizePermissionMode(string(fp.Permissions.HardwareGuard)); hw != PermissionBlock {
		deleteNestedKey(existing, "profile", "default_content_setting_values", "usb_guard")
		deleteNestedKey(existing, "profile", "default_content_setting_values", "hid_guard")
		deleteNestedKey(existing, "profile", "default_content_setting_values", "bluetooth_guard")
		deleteNestedKey(existing, "profile", "default_content_setting_values", "serial_guard")
	}
}

func deleteNestedKey(root map[string]any, keys ...string) {
	if root == nil || len(keys) == 0 {
		return
	}
	cur := root
	for i := 0; i < len(keys)-1; i++ {
		next, ok := cur[keys[i]].(map[string]any)
		if !ok || next == nil {
			return
		}
		cur = next
	}
	delete(cur, keys[len(keys)-1])
}

func deepMergeMap(dst, src map[string]any) {
	for k, sv := range src {
		if sm, ok := sv.(map[string]any); ok {
			if dm, exists := dst[k].(map[string]any); exists && dm != nil {
				deepMergeMap(dm, sm)
				continue
			}
			cp := make(map[string]any, len(sm))
			deepMergeMap(cp, sm)
			dst[k] = cp
			continue
		}
		dst[k] = sv
	}
}
