package driver

import (
	"net/http"
	"testing"

	kernel "github.com/xxww0098/realbrowser/fingerprintkernel"
)

func TestDriverBrowserConfiguresPrivateTransport(t *testing.T) {
	b, err := Browser(t.TempDir(), kernel.InstalledRelease{}, "http://127.0.0.1:3128")
	if err != nil {
		t.Fatal(err)
	}
	transport := b.Manager.HTTPClient.Transport.(*http.Transport)
	proxy, err := transport.Proxy(&http.Request{})
	if err != nil || proxy.Host != "127.0.0.1:3128" || transport == http.DefaultTransport {
		t.Fatalf("transport %v, %v", proxy, err)
	}
	if b.Manager.AcceptRelease("v150.0.7871.186") || !b.Manager.AcceptRelease("151.0.0.1") {
		t.Fatal("driver compatibility policy lost")
	}
	if _, err := Browser(t.TempDir(), kernel.InstalledRelease{}, "://"); err == nil {
		t.Fatal("accepted invalid proxy")
	}
}
