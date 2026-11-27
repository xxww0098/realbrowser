package profile

import (
	"encoding/json"
	"strconv"
	"strings"
)

// ApplyFingerprintConfig merges only the fields explicitly present in rawConfig
// (a Local API fingerprint_config map or JSON string, optionally containing an
// embedded fpchromium.fingerprint object) onto base, preserving all unspecified
// fields (including Seed, Platform, Brand, DisableSpoofing, and window geometry).
func ApplyFingerprintConfig(base FingerprintSpec, rawConfig any) FingerprintSpec {
	fp := base
	if len(base.DisableSpoofing) > 0 {
		fp.DisableSpoofing = append([]SpoofModule(nil), base.DisableSpoofing...)
	}
	fp.WindowX, fp.WindowY = clonePointer(base.WindowX), clonePointer(base.WindowY)
	fp.Geolocation.Latitude, fp.Geolocation.Longitude = clonePointer(base.Geolocation.Latitude), clonePointer(base.Geolocation.Longitude)
	fp.Screen = cloneScreen(base.Screen)

	var fpCfg map[string]any
	switch v := rawConfig.(type) {
	case map[string]any:
		fpCfg = v
	case string:
		if s := strings.TrimSpace(v); s != "" {
			_ = json.Unmarshal([]byte(s), &fpCfg)
		}
	}
	if len(fpCfg) == 0 {
		return fp
	}

	// 1. If an embedded native fpchromium.fingerprint object is present, merge its explicit keys first.
	if emb := mapObject(fpCfg, "fpchromium"); emb != nil {
		if embFP := mapObject(emb, "fingerprint"); len(embFP) > 0 {
			applyFingerprintMap(&fp, embFP)
		}
	}

	// 2. Apply top-level fingerprint_config keys.
	applyFingerprintMap(&fp, fpCfg)
	return fp
}

func clonePointer[T any](v *T) *T {
	if v == nil {
		return nil
	}
	c := *v
	return &c
}

func cloneScreen(screen *ScreenSpec) *ScreenSpec {
	c := clonePointer(screen)
	if c != nil {
		c.WindowX, c.WindowY = clonePointer(c.WindowX), clonePointer(c.WindowY)
	}
	return c
}

func applyFingerprintMap(fp *FingerprintSpec, m map[string]any) {
	if len(m) == 0 {
		return
	}
	hasKey := func(k string) bool {
		v, ok := m[k]
		return ok && v != nil
	}

	if seedStr := firstString(m, "fingerprint_seed", "seed"); seedStr != "" {
		if n, err := strconv.ParseUint(strings.TrimSpace(seedStr), 10, 32); err == nil && n > 0 {
			fp.Seed = uint32(n)
		}
	}

	ua := strings.TrimSpace(mapString(m, "ua"))

	var plat Platform
	if hasKey("platform") {
		plat = NormalizePlatform(mapString(m, "platform"))
	} else if ua != "" {
		plat = InferPlatformFromUA(ua)
	}
	if plat != "" {
		changed := plat != fp.Platform
		fp.Platform = plat
		if changed && !hasKey("platform_version") {
			fp.PlatformVersion = DefaultPlatformVersion(plat)
		}
	}
	if hasKey("platform_version") {
		fp.PlatformVersion = strings.TrimSpace(mapString(m, "platform_version"))
	}

	var brand Brand
	if hasKey("brand") {
		brand = NormalizeBrand(mapString(m, "brand"))
	} else if ua != "" {
		brand = InferBrandFromUA(ua)
	}
	if brand != "" {
		changed := brand != fp.Brand
		fp.Brand = brand
		if changed && !hasKey("brand_version") {
			fp.BrandVersion = ""
		}
	}
	if hasKey("brand_version") {
		fp.BrandVersion = strings.TrimSpace(mapString(m, "brand_version"))
	}

	if hasKey("hardware_concurrency") {
		hc := strings.TrimSpace(mapString(m, "hardware_concurrency"))
		if hc == "default" || hc == "0" {
			fp.HardwareConcurrency = 0
		} else if n, err := strconv.Atoi(hc); err == nil && n > 0 && n <= 128 {
			fp.HardwareConcurrency = n
		}
	}

	hasAcceptLang := strings.TrimSpace(mapString(m, "accept_lang")) != ""
	if langs := mapStringSlice(m, "language"); len(langs) > 0 {
		fp.Lang = langs[0]
		if len(langs) == 1 && !hasAcceptLang {
			fp.AcceptLang = DefaultAcceptLang(langs[0])
		} else {
			fp.AcceptLang = strings.Join(langs, ",")
		}
	} else if hasKey("lang") {
		if l := strings.TrimSpace(mapString(m, "lang")); l != "" {
			changed := !strings.EqualFold(fp.Lang, l)
			fp.Lang = l
			if !hasAcceptLang && (changed || strings.TrimSpace(fp.AcceptLang) == "") {
				fp.AcceptLang = DefaultAcceptLang(l)
			}
		}
	}
	if hasAcceptLang {
		fp.AcceptLang = strings.TrimSpace(mapString(m, "accept_lang"))
	}

	if hasKey("timezone") {
		if tz := strings.TrimSpace(mapString(m, "timezone")); tz != "" {
			fp.Timezone = strings.ReplaceAll(tz, " ", "_")
		}
	}

	if hasKey("disable_spoofing") {
		rawMods := mapStringSlice(m, "disable_spoofing")
		mods := make([]SpoofModule, 0, len(rawMods))
		for _, s := range rawMods {
			mods = append(mods, SpoofModule(s))
		}
		fp.DisableSpoofing = CanonicalSpoofModules(mods)
	}

	legacyToggles := []struct {
		key string
		mod SpoofModule
	}{
		{"canvas", SpoofCanvas},
		{"audio", SpoofAudio},
		{"client_rects", SpoofClientRects},
		{"webgl", SpoofGPU},
		{"fonts", SpoofFont},
	}
	for _, tg := range legacyToggles {
		if !hasKey(tg.key) {
			continue
		}
		v := strings.ToLower(strings.TrimSpace(mapString(m, tg.key)))
		switch v {
		case "0", "false":
			fp.SetSpoofingEnabled(tg.mod, false)
		case "1", "true":
			fp.SetSpoofingEnabled(tg.mod, true)
		}
	}

	if hasKey("webrtc") {
		if w := strings.ToLower(strings.TrimSpace(mapString(m, "webrtc"))); w != "" {
			if pol := NormalizeWebRTCPolicy(w); pol != "" {
				fp.SetWebRTCPolicy(pol)
			} else {
				fp.DisableNonProxiedUDP = w != "real"
			}
		}
	}
	if hasKey("disable_non_proxied_udp") {
		if v, ok := parseLooseBool(m["disable_non_proxied_udp"]); ok {
			fp.DisableNonProxiedUDP = v
			if !hasKey("webrtc_policy") {
				if v {
					fp.WebRTCPolicy = WebRTCDisableNonProxiedUDP
				} else {
					fp.WebRTCPolicy = WebRTCDefault
				}
			}
		}
	}
	if hasKey("webrtc_policy") {
		if pol := NormalizeWebRTCPolicy(mapString(m, "webrtc_policy")); pol != "" {
			fp.SetWebRTCPolicy(pol)
		}
	}
	if hasKey("dns_leak_mode") {
		if dm := NormalizeDNSLeakMode(mapString(m, "dns_leak_mode")); dm != "" {
			fp.DNSLeakMode = dm
		}
	}
	if hasKey("quic_mode") {
		if qm := NormalizeQUICMode(mapString(m, "quic_mode")); qm != "" {
			fp.QUICMode = qm
		}
	} else if hasKey("disable_quic") {
		if v, ok := parseLooseBool(m["disable_quic"]); ok {
			if v {
				fp.QUICMode = QUICDisable
			} else {
				fp.QUICMode = QUICAllow
			}
		}
	}
	if hasKey("automation_mode") {
		if am := NormalizeAutomationMode(mapString(m, "automation_mode")); am != "" {
			fp.AutomationMode = am
		}
	}
	if hasKey("do_not_track") {
		if dnt := NormalizeDoNotTrack(mapString(m, "do_not_track")); dnt != "" {
			fp.DoNotTrack = dnt
		}
	}
	if hasKey("gpu_mode") {
		if gm := NormalizeGPUMode(mapString(m, "gpu_mode")); gm != "" {
			fp.GPUMode = gm
		}
	}
	if hasKey("color_scheme") {
		if cs := NormalizeColorScheme(mapString(m, "color_scheme")); cs != "" {
			fp.ColorScheme = cs
		}
	}
	if hasKey("disable_pings") {
		if v, ok := parseLooseBool(m["disable_pings"]); ok {
			fp.DisablePings = v
		}
	}
	if hasKey("mute_audio") {
		if v, ok := parseLooseBool(m["mute_audio"]); ok {
			fp.MuteAudio = v
		}
	}
	if hasKey("disable_password_popup") {
		if v, ok := parseLooseBool(m["disable_password_popup"]); ok {
			fp.DisablePasswordPopup = v
		}
	}
	if geo := mapObject(m, "geolocation"); geo != nil {
		if mode := NormalizePermissionMode(mapString(geo, "mode")); mode != "" {
			fp.Geolocation.Mode = mode
		}
		rawLat, hasLat := geo["latitude"]
		rawLon, hasLon := geo["longitude"]
		if (hasLat && (rawLat == nil || strings.TrimSpace(mapString(geo, "latitude")) == "")) ||
			(hasLon && (rawLon == nil || strings.TrimSpace(mapString(geo, "longitude")) == "")) {
			fp.Geolocation.Latitude = nil
			fp.Geolocation.Longitude = nil
		} else {
			if latStr := strings.TrimSpace(mapString(geo, "latitude")); latStr != "" {
				if lat, err := strconv.ParseFloat(latStr, 64); err == nil && lat >= -90 && lat <= 90 {
					fp.Geolocation.Latitude = &lat
				}
			}
			if lonStr := strings.TrimSpace(mapString(geo, "longitude")); lonStr != "" {
				if lon, err := strconv.ParseFloat(lonStr, 64); err == nil && lon >= -180 && lon <= 180 {
					fp.Geolocation.Longitude = &lon
				}
			}
		}
		if accStr := strings.TrimSpace(mapString(geo, "accuracy")); accStr != "" {
			if acc, err := strconv.ParseFloat(accStr, 64); err == nil && acc >= 0 {
				fp.Geolocation.Accuracy = acc
			}
		}
	}
	if hasKey("location") {
		if mode := NormalizePermissionMode(mapString(m, "location")); mode != "" {
			fp.Geolocation.Mode = mode
		}
	}
	if hasKey("latitude") && hasKey("longitude") {
		lat, err1 := strconv.ParseFloat(strings.TrimSpace(mapString(m, "latitude")), 64)
		lon, err2 := strconv.ParseFloat(strings.TrimSpace(mapString(m, "longitude")), 64)
		if err1 == nil && err2 == nil && lat >= -90 && lat <= 90 && lon >= -180 && lon <= 180 {
			fp.Geolocation.Latitude = &lat
			fp.Geolocation.Longitude = &lon
		}
	}
	if perms := mapObject(m, "permissions"); perms != nil {
		if v := NormalizePermissionMode(mapString(perms, "notifications")); v != "" {
			fp.Permissions.Notifications = v
		}
		if v := NormalizePermissionMode(mapString(perms, "media_devices")); v != "" {
			fp.Permissions.MediaDevices = v
		}
		if v := NormalizePermissionMode(mapString(perms, "sensors")); v != "" {
			fp.Permissions.Sensors = v
		}
		if v := NormalizePermissionMode(mapString(perms, "hardware_guard")); v == PermissionPrompt || v == PermissionBlock {
			fp.Permissions.HardwareGuard = v
		}
	}

	if v, set := m["screen"]; set {
		if v == nil {
			fp.Screen = nil
		} else if patch := mapObject(m, "screen"); patch != nil {
			screen := cloneScreen(fp.Screen)
			raw, err := json.Marshal(patch)
			if err == nil && json.Unmarshal(raw, &screen) == nil && screen != nil && screen.Validate() == nil {
				fp.Screen = screen
			}
		}
	}
	if hasKey("screen_resolution") {
		if w, h := parseResolution(mapString(m, "screen_resolution")); w > 0 && h > 0 {
			fp.WindowWidth, fp.WindowHeight = w, h
		}
	}
	if hasKey("window_width") {
		if w, err := strconv.Atoi(strings.TrimSpace(mapString(m, "window_width"))); err == nil && w >= 0 {
			fp.WindowWidth = w
		}
	}
	if hasKey("window_height") {
		if h, err := strconv.Atoi(strings.TrimSpace(mapString(m, "window_height"))); err == nil && h >= 0 {
			fp.WindowHeight = h
		}
	}
	if hasKey("window_x") {
		if x, err := strconv.Atoi(strings.TrimSpace(mapString(m, "window_x"))); err == nil {
			fp.WindowX = &x
		}
	}
	if hasKey("window_y") {
		if y, err := strconv.Atoi(strings.TrimSpace(mapString(m, "window_y"))); err == nil {
			fp.WindowY = &y
		}
	}
	if hasKey("device_scale_factor") || hasKey("device_scale") {
		rawScale := strings.TrimSpace(firstString(m, "device_scale_factor", "device_scale"))
		if rawScale == "" || rawScale == "0" {
			fp.DeviceScaleFactor = 0
		} else if s, err := strconv.ParseFloat(rawScale, 64); err == nil && s >= 0.5 && s <= 4.0 {
			fp.DeviceScaleFactor = s
		}
	}

	if hasKey("headless") {
		h := strings.ToLower(strings.TrimSpace(mapString(m, "headless")))
		if h == "1" || h == "2" || h == "true" {
			fp.Headless = true
		} else if h == "0" || h == "false" {
			fp.Headless = false
		}
	}

	if hasKey("user_agent") {
		fp.UserAgent = strings.TrimSpace(mapString(m, "user_agent"))
	}
}

func parseLooseBool(v any) (bool, bool) {
	switch t := v.(type) {
	case bool:
		return t, true
	case string:
		switch strings.ToLower(strings.TrimSpace(t)) {
		case "1", "true":
			return true, true
		case "0", "false":
			return false, true
		}
	case float64:
		if t == 1 {
			return true, true
		}
		if t == 0 {
			return false, true
		}
	case int:
		if t == 1 {
			return true, true
		}
		if t == 0 {
			return false, true
		}
	}
	return false, false
}

func parseResolution(s string) (int, int) {
	s = strings.TrimSpace(s)
	sep := "_"
	if strings.Contains(s, "x") {
		sep = "x"
	} else if strings.Contains(s, "X") {
		sep = "X"
	} else if strings.Contains(s, ",") {
		sep = ","
	}
	parts := strings.Split(s, sep)
	if len(parts) != 2 {
		return 0, 0
	}
	w, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
	h, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err1 != nil || err2 != nil || w <= 0 || h <= 0 {
		return 0, 0
	}
	return w, h
}

func firstString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v := mapString(m, k); v != "" {
			return v
		}
	}
	return ""
}

func mapString(m map[string]any, k string) string {
	if m == nil {
		return ""
	}
	v, ok := m[k]
	if !ok || v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case float64:
		if t == float64(int64(t)) {
			return strconv.FormatInt(int64(t), 10)
		}
		return strconv.FormatFloat(t, 'f', -1, 64)
	case int:
		return strconv.Itoa(t)
	case int32:
		return strconv.FormatInt(int64(t), 10)
	case int64:
		return strconv.FormatInt(t, 10)
	case uint:
		return strconv.FormatUint(uint64(t), 10)
	case uint32:
		return strconv.FormatUint(uint64(t), 10)
	case uint64:
		return strconv.FormatUint(t, 10)
	case bool:
		if t {
			return "true"
		}
		return "false"
	case json.Number:
		return t.String()
	default:
		return ""
	}
}

func mapObject(m map[string]any, k string) map[string]any {
	if m == nil {
		return nil
	}
	switch v := m[k].(type) {
	case map[string]any:
		return v
	case string:
		if v == "" {
			return nil
		}
		var out map[string]any
		if json.Unmarshal([]byte(v), &out) == nil {
			return out
		}
	}
	return nil
}

func mapStringSlice(m map[string]any, k string) []string {
	if m == nil {
		return nil
	}
	var out []string
	switch v := m[k].(type) {
	case []string:
		return append([]string(nil), v...)
	case []any:
		for _, e := range v {
			if s, ok := e.(string); ok && strings.TrimSpace(s) != "" {
				out = append(out, strings.TrimSpace(s))
			}
		}
	case string:
		v = strings.TrimSpace(v)
		if v == "" {
			return nil
		}
		if strings.HasPrefix(v, "[") {
			var arr []string
			if json.Unmarshal([]byte(v), &arr) == nil {
				return arr
			}
		}
		for _, p := range strings.Split(v, ",") {
			if t := strings.TrimSpace(p); t != "" {
				out = append(out, t)
			}
		}
	}
	return out
}
