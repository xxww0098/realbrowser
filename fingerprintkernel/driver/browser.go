package driver

import (
	"errors"
	"net/http"
	"net/url"
	"runtime"
	"strings"

	"github.com/xxww0098/realbrowser/fingerprintkernel/browser"
	"github.com/xxww0098/realbrowser/fingerprintkernel/engine"
)

// Browser uses the archive format and releases compatible with the private
// desktop driver. Its private HTTP transport uses the host proxy by default.
func Browser(dir string, base engine.InstalledRelease, proxy string) (browser.ManagedBrowser, error) {
	mgr := engine.NewKernelManager("", "")
	// This upstream build crashes when the driver's renderer reads a canvas.
	mgr.AcceptRelease = func(tag string) bool { return strings.TrimPrefix(tag, "v") != "150.0.7871.186" }
	if runtime.GOOS == "linux" {
		mgr.ArchiveSuffix = ".tar.xz"
	}
	transport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		transport = &http.Transport{Proxy: http.ProxyFromEnvironment, ForceAttemptHTTP2: true}
	}
	clientTransport := transport.Clone()
	if proxy != "" {
		u, err := url.Parse(proxy)
		if err != nil || u.Host == "" {
			return browser.ManagedBrowser{}, errors.New("invalid browser download proxy")
		}
		clientTransport.Proxy = http.ProxyURL(u)
	}
	mgr.HTTPClient.Transport = clientTransport
	b := browser.New(dir)
	b.Base, b.Manager = base, mgr
	return b, nil
}
