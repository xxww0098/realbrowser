package browser

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

	"github.com/xxww0098/realbrowser/fingerprintkernel/engine"
	"github.com/xxww0098/realbrowser/fingerprintkernel/process"
	fpprofile "github.com/xxww0098/realbrowser/fingerprintkernel/profile"
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

	profile := t.TempDir()
	if runtime.GOOS == "darwin" {
		// Let the test launcher verify and claim its empty directory before
		// the kernel writes the test profile's preferences there.
		probe := exec.CommandContext(t.Context(), launcher, browser, "--user-data-dir="+profile, "--use-mock-keychain", "--version")
		if out, err := probe.CombinedOutput(); err != nil {
			t.Fatalf("initialize isolated browser profile: %s: %v", out, err)
		}
	}
	if err := fpprofile.EditProfile(t.Context(), profile, func(f *fpprofile.FingerprintSpec) error {
		f.Headless = true
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	installation := New(t.TempDir())
	installation.Base = engine.InstalledRelease{Executable: browser}
	var proc *process.Process
	var err error
	if launcher == "" {
		proc, err = installation.Open(t.Context(), profile, server.URL+"/catalog")
	} else {
		cmd, prepErr := installation.openCommand(t.Context(), profile, []string{server.URL + "/catalog"})
		if prepErr != nil {
			t.Fatal(prepErr)
		}
		// The macOS test launcher accepts the executable first, then verifies
		// that the final command uses the isolated profile and mock keychain.
		args := []string{launcher, cmd.Path, "--disable-background-networking", "--remote-debugging-address=127.0.0.1"}
		if runtime.GOOS == "darwin" {
			args = append(args, "--use-mock-keychain")
		}
		cmd.Path, cmd.Args = launcher, append(args, cmd.Args[1:]...)
		proc, err = process.StartProcess(cmd, process.DetachedBrowser)
	}
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
			if err := proc.Terminate(process.DefaultKillGrace); err != nil {
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
