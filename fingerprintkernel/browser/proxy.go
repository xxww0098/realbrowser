package browser

import (
	"errors"
	"fmt"
	"strings"
)

// ProxySpec configures outbound proxy routing for a profile.
// When Username or Password is set, RealBrowser starts a local loopback
// ProxyBridge so Chromium's --proxy-server never prompts for credentials.
type ProxySpec struct {
	Mode       string   `json:"mode"` // "direct" | "http" | "https" | "socks5"
	Host       string   `json:"host,omitempty"`
	Port       int      `json:"port,omitempty"`
	Username   string   `json:"username,omitempty"`
	Password   string   `json:"password,omitempty"`
	BypassList []string `json:"bypass_list,omitempty"`
	ProxyID    string   `json:"proxy_id,omitempty"` // optional reference to saved proxy record
}

// NormalizedMode returns canonical proxy mode ("direct", "http", "https", "socks5").
func (p ProxySpec) NormalizedMode() string {
	switch strings.ToLower(strings.TrimSpace(p.Mode)) {
	case "", "direct", "none", "no_proxy":
		return "direct"
	case "http":
		return "http"
	case "https":
		return "https"
	case "socks5", "socks":
		return "socks5"
	default:
		return strings.ToLower(strings.TrimSpace(p.Mode))
	}
}

// IsDirect reports whether traffic goes direct without a proxy.
func (p ProxySpec) IsDirect() bool {
	return p.NormalizedMode() == "direct"
}

// NeedsLocalBridge reports whether Chromium requires a local loopback proxy
// bridge to supply upstream proxy authentication credentials.
func (p ProxySpec) NeedsLocalBridge() bool {
	return !p.IsDirect() && (p.Username != "" || p.Password != "")
}

// ServerURL returns scheme://host:port (without credentials) for direct proxy usage.
func (p ProxySpec) ServerURL() string {
	if p.IsDirect() || p.Host == "" || p.Port <= 0 {
		return ""
	}
	host := p.Host
	if strings.Contains(host, ":") && !strings.HasPrefix(host, "[") {
		host = "[" + host + "]"
	}
	return fmt.Sprintf("%s://%s:%d", p.NormalizedMode(), host, p.Port)
}

// Summary returns a compact display label for UI tables.
func (p ProxySpec) Summary() string {
	if p.IsDirect() || p.Host == "" {
		return "直连"
	}
	auth := ""
	if p.Username != "" {
		auth = " (认证)"
	}
	return fmt.Sprintf("%s %s:%d%s", strings.ToUpper(p.NormalizedMode()), p.Host, p.Port, auth)
}

// Validate verifies proxy configuration fields.
func (p ProxySpec) Validate() error {
	mode := p.NormalizedMode()
	if mode == "direct" {
		return nil
	}
	if mode != "http" && mode != "https" && mode != "socks5" {
		return fmt.Errorf("fpchromium: unsupported proxy mode %q", p.Mode)
	}
	if strings.TrimSpace(p.Host) == "" {
		return errors.New("fpchromium: proxy host is required")
	}
	if p.Port < 1 || p.Port > 65535 {
		return fmt.Errorf("fpchromium: proxy port %d out of range [1, 65535]", p.Port)
	}
	return nil
}
