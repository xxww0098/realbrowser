package fingerprintkernel

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestManagedChromiumRendersLocalContent(t *testing.T) {
	browser, launcher := os.Getenv("REALBROWSER_TEST_BINARY"), os.Getenv("REALBROWSER_TEST_LAUNCHER")
	if browser == "" && launcher == "" {
		t.Skip("set REALBROWSER_TEST_BINARY to run the native rendering test")
	}
	if !filepath.IsAbs(browser) || (launcher != "" && !filepath.IsAbs(launcher)) {
		t.Fatal("test browser and launcher paths must be absolute")
	}
	if runtime.GOOS == "darwin" && launcher == "" {
		t.Fatal("set REALBROWSER_TEST_LAUNCHER to the temporary-profile Chromium launcher on macOS")
	}

	rendered := make(chan string, 2)
	mux := http.NewServeMux()
	mux.HandleFunc("/rendered", func(w http.ResponseWriter, r *http.Request) {
		select {
		case rendered <- r.URL.Query().Get("text"):
		case <-r.Context().Done():
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		switch r.URL.Path {
		case "/catalog":
			fmt.Fprint(w, "<main></main><script>document.querySelector('main').textContent = 'Catalog ready'</script>")
		case "/restricted":
			w.WriteHeader(http.StatusForbidden)
			fmt.Fprint(w, "<main>Access unavailable</main>")
		default:
			http.NotFound(w, r)
			return
		}
		fmt.Fprint(w, `<script>
			const main = document.querySelector("main");
			if (main.offsetHeight > 0) {
				fetch("/rendered?text=" + encodeURIComponent(main.innerText)).then(() => {
					if (location.pathname === "/catalog") location.assign("/restricted");
				});
			}
		</script>`)
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	args := []string{
		"--user-data-dir=" + t.TempDir(), "--headless=new", "--no-first-run",
		"--no-default-browser-check", "--disable-background-networking",
		"--remote-debugging-address=127.0.0.1", "--remote-debugging-port=0",
	}
	if runtime.GOOS == "darwin" {
		args = append(args, "--use-mock-keychain")
	}
	executable := browser
	if launcher != "" {
		executable, args = launcher, append([]string{browser}, args...)
	}
	cmd := exec.Command(executable, append(args, server.URL+"/catalog")...)
	proc, err := StartProcess(cmd, DetachedBrowser)
	if err != nil {
		t.Fatal(err)
	}
	exited := make(chan struct{})
	proc.OnExit(func(int) { close(exited) })
	t.Cleanup(func() {
		select {
		case <-exited:
			return
		default:
			if err := proc.Terminate(DefaultKillGrace); err != nil {
				t.Error(err)
			}
		}
	})
	deadline := time.NewTimer(30 * time.Second)
	defer deadline.Stop()
	for _, want := range []string{"Catalog ready", "Access unavailable"} {
		select {
		case got := <-rendered:
			if got != want {
				t.Fatalf("rendered %q, want %q", got, want)
			}
		case <-exited:
			t.Fatalf("browser exited %d before rendering %q", proc.Wait(), want)
		case <-deadline.C:
			t.Fatalf("timed out waiting for rendered text %q", want)
		}
	}
}
