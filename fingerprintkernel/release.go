package fingerprintkernel

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// ReleaseAsset describes a single downloadable asset from a GitHub Release.
type ReleaseAsset struct {
	Name        string `json:"name"`
	DownloadURL string `json:"browser_download_url"`
	Size        int64  `json:"size"`
	Digest      string `json:"digest,omitempty"`       // GitHub Releases native digest (e.g. "sha256:<64hex>")
	SHA256      string `json:"sha256,omitempty"`       // optional expected 64-hex SHA-256 digest
	ChecksumURL string `json:"checksum_url,omitempty"` // optional sidecar .sha256 or SHA256SUMS URL
}

// ReleaseInfo summarizes the latest compatible official adryfish/fingerprint-chromium release.
type ReleaseInfo struct {
	TagName     string         `json:"tag_name"`
	Name        string         `json:"name"`
	HTMLURL     string         `json:"html_url"`
	PublishedAt string         `json:"published_at"`
	Body        string         `json:"body"`
	Draft       bool           `json:"draft,omitempty"`
	Prerelease  bool           `json:"prerelease,omitempty"`
	Assets      []ReleaseAsset `json:"assets"`
	Matched     *ReleaseAsset  `json:"matched,omitempty"`
	LatestTag   string         `json:"latest_tag,omitempty"` // populated when a newer release lacks a build for this OS/arch
}

var errNoReleases = errors.New("fpchromium: no releases found")

// FetchLatestRelease queries the official adryfish/fingerprint-chromium GitHub
// Releases API and selects the newest compatible release with a matching asset
// for the current OS/arch. When the latest upstream tag omits a platform build
// (e.g. Windows/Linux-only release while the latest macOS .dmg is in an earlier
// release), FetchLatestRelease scans recent releases and records LatestTag.
func (m *KernelManager) FetchLatestRelease(ctx context.Context) (*ReleaseInfo, error) {
	return m.fetchRelease(ctx, false)
}

// FetchInstallableRelease chooses the highest stable Chromium version with an archive the host can install.
func (m *KernelManager) FetchInstallableRelease(ctx context.Context) (*ReleaseInfo, error) {
	return m.fetchRelease(ctx, true)
}

func (m *KernelManager) fetchRelease(ctx context.Context, installable bool) (*ReleaseInfo, error) {

	// Custom LatestAPIURL (e.g. unit tests or self-hosted mirrors): query it first,
	// supporting either a single release object or a JSON array of releases.
	if m.LatestAPIURL != "" && m.LatestAPIURL != UpstreamLatestAPI {
		rels, err := m.fetchReleasePayload(ctx, m.LatestAPIURL)
		if err != nil {
			return nil, err
		}
		best, matched := m.pickRelease(rels, installable)
		if matched && !installable {
			return best, nil
		}
		// A mirror's latest publication can be a backport. Compare versions from
		// its own release list when available.
		listURL := ""
		if m.ReleasesAPIURL != "" && m.ReleasesAPIURL != UpstreamReleasesAPI {
			listURL = m.ReleasesAPIURL
		} else if strings.HasSuffix(m.LatestAPIURL, "/releases/latest") {
			listURL = strings.TrimSuffix(m.LatestAPIURL, "/latest") + "?per_page=20"
		}
		if listURL != "" && listURL != m.LatestAPIURL {
			if listRels, lerr := m.fetchReleasePayload(ctx, listURL); lerr == nil {
				best, _ = m.pickRelease(append(rels, listRels...), installable)
			} else if ctx.Err() != nil {
				return nil, ctx.Err()
			}
		}
		if best != nil {
			return best, nil
		}
		return nil, errNoReleases
	}

	// Default upstream mode: query /releases?per_page=10 in a single request so
	// platforms missing from the very newest tag (notably macOS .dmg) automatically
	// resolve to the newest compatible release.
	listURL := m.ReleasesAPIURL
	if listURL == "" {
		listURL = UpstreamReleasesAPI
	}
	rels, err := m.fetchReleasePayload(ctx, listURL)
	if err != nil {
		return nil, err
	}
	best, _ := m.pickRelease(rels, installable)
	if best == nil {
		return nil, errNoReleases
	}
	return best, nil
}

func (m *KernelManager) pickRelease(releases []ReleaseInfo, installable bool) (*ReleaseInfo, bool) {
	goos, goarch := m.goos(), m.goarch()
	var candidates []ReleaseInfo
	versions := make(map[string][]int)
	for _, r := range releases {
		if m.AcceptRelease != nil && !m.AcceptRelease(r.TagName) {
			continue
		}
		if installable {
			version, err := versionParts(r.TagName)
			if err != nil {
				continue
			}
			versions[r.TagName] = version
		}
		r.Matched = nil
		if installable || m.ArchiveSuffix != "" {
			assets := r.Assets
			r.Assets = nil
			for _, a := range assets {
				if (installable && !CanInstallArchive(goos, a.Name)) || !strings.HasSuffix(strings.ToLower(a.Name), m.ArchiveSuffix) {
					continue
				}
				if a.ChecksumURL == "" {
					if cs := FindChecksumAsset(assets, a.Name); cs != nil {
						a.ChecksumURL = cs.DownloadURL
					}
				}
				if installable {
					var ok bool
					if a, ok = a.forAutomaticUpdate(goos, goarch, r.TagName, r.Body); !ok {
						continue
					}
				}
				r.Assets = append(r.Assets, a)
			}
		}
		candidates = append(candidates, r)
	}
	if installable {
		slices.SortStableFunc(candidates, func(a, b ReleaseInfo) int {
			return slices.Compare(versions[b.TagName], versions[a.TagName])
		})
	}
	return pickCompatibleRelease(candidates, goos, goarch)
}

func (a ReleaseAsset) forAutomaticUpdate(goos, goarch, tag, body string) (ReleaseAsset, bool) {
	if a.DownloadURL == "" || a.emptyXZArchive() {
		return a, false
	}
	pinned, ok := PinnedRelease(goos, goarch)
	if ok && strings.TrimPrefix(tag, "v") == PinnedVersion && a.Name == pinned.File {
		if a.SHA256 != "" && NormalizeSHA256(a.SHA256) != pinned.SHA256 {
			return a, false
		}
		if digest := NormalizeSHA256(a.Digest); digest != "" && digest != pinned.SHA256 {
			return a, false
		}
		a.SHA256 = pinned.SHA256
	} else if !matchesUpdateArchitecture(strings.ToLower(a.Name), goos, goarch) {
		return a, false
	}
	if !a.hasChecksum() && a.SHA256 == "" {
		a.SHA256, _ = parseReleaseBodySHA256(body, a.Name)
	}
	return a, a.hasChecksum()
}

func (a ReleaseAsset) emptyXZArchive() bool {
	// An XZ stream of at most 32 bytes cannot contain a data block.
	return a.Size > 0 && a.Size <= 32 && strings.HasSuffix(strings.ToLower(a.Name), ".tar.xz")
}

func matchesUpdateArchitecture(name, goos, goarch string) bool {
	arm64, x64 := releaseArchitectures(name)
	if arm64 || x64 {
		return (goarch == "arm64" && arm64) || (goarch == "amd64" && x64)
	}
	if goos == "darwin" && (goarch == "arm64" || goarch == "amd64") {
		parts := strings.FieldsFunc(name, func(c rune) bool { return c == '-' || c == '_' || c == '.' })
		if slices.Contains(parts, "universal") || slices.Contains(parts, "universal2") {
			return true
		}
	}
	return false
}

func releaseArchitectures(lowerName string) (arm64, x64 bool) {
	arm64 = strings.Contains(lowerName, "arm64") || strings.Contains(lowerName, "aarch64")
	x64 = strings.Contains(lowerName, "x64") || strings.Contains(lowerName, "x86_64") || strings.Contains(lowerName, "amd64") || strings.Contains(lowerName, "win64")
	return arm64, x64
}

func (a ReleaseAsset) hasChecksum() bool {
	if strings.TrimSpace(a.SHA256) != "" {
		return NormalizeSHA256(a.SHA256) != ""
	}
	return NormalizeSHA256(a.Digest) != "" || strings.TrimSpace(a.ChecksumURL) != ""
}

func (m *KernelManager) fetchReleasePayload(ctx context.Context, apiURL string) ([]ReleaseInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "fingerprint-chromium-manager")

	cl := m.HTTPClient
	if cl == nil {
		cl = &http.Client{Timeout: 20 * time.Second}
	}
	resp, err := cl.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fpchromium: query releases: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fpchromium: github releases API returned HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, fmt.Errorf("fpchromium: read release JSON: %w", err)
	}
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return nil, errors.New("fpchromium: empty release JSON")
	}
	if trimmed[0] == '[' {
		var list []ReleaseInfo
		if err := json.Unmarshal(trimmed, &list); err != nil {
			return nil, fmt.Errorf("fpchromium: decode releases JSON: %w", err)
		}
		return list, nil
	}
	var single ReleaseInfo
	if err := json.Unmarshal(trimmed, &single); err != nil {
		return nil, fmt.Errorf("fpchromium: decode release JSON: %w", err)
	}
	return []ReleaseInfo{single}, nil
}

func pickCompatibleRelease(releases []ReleaseInfo, goos, goarch string) (*ReleaseInfo, bool) {
	var (
		latestTag string
		fallback  *ReleaseInfo
	)
	for i := range releases {
		r := releases[i]
		if r.Draft || r.Prerelease {
			continue
		}
		if r.HTMLURL == "" {
			r.HTMLURL = UpstreamReleasesPage
		}
		if latestTag == "" {
			latestTag = r.TagName
		}
		if fallback == nil {
			cp := r
			fallback = &cp
		}
		if matched := SelectReleaseAsset(r.Assets, goos, goarch); matched != nil {
			if matched.SHA256 == "" && matched.ChecksumURL == "" && strings.TrimSpace(r.Body) != "" {
				if dig, ok := parseReleaseBodySHA256(r.Body, matched.Name); ok {
					matched.SHA256 = dig
				}
			}
			r.Matched = matched
			if latestTag != "" && latestTag != r.TagName {
				r.LatestTag = latestTag
			}
			return &r, true
		}
	}
	return fallback, false
}

// SelectReleaseAsset chooses the best pre-compiled binary asset for (goos, goarch).
func SelectReleaseAsset(assets []ReleaseAsset, goos, goarch string) *ReleaseAsset {
	if len(assets) == 0 {
		return nil
	}
	score := func(name string) int {
		n := strings.ToLower(name)
		// Skip checksums, signatures, metadata, source archives, debug symbols.
		if strings.HasSuffix(n, ".sha256") || strings.HasSuffix(n, ".sha512") ||
			strings.HasSuffix(n, ".sig") || strings.HasSuffix(n, ".asc") ||
			strings.HasSuffix(n, ".zsync") || strings.HasSuffix(n, ".blockmap") ||
			strings.HasSuffix(n, ".txt") || strings.HasSuffix(n, ".json") ||
			strings.Contains(n, "chromedriver") || strings.Contains(n, "source") {
			return -1
		}
		isArm, isX64 := releaseArchitectures(n)
		if goarch == "amd64" && isArm && !isX64 || goarch == "arm64" && isX64 && !isArm {
			return -1
		}
		s := 0
		switch goos {
		case "darwin":
			if !strings.Contains(n, "macos") && !strings.Contains(n, "darwin") && !strings.Contains(n, "mac") && !strings.HasSuffix(n, ".dmg") {
				return -1
			}
			s += 10
			if strings.HasSuffix(n, ".dmg") {
				s += 5
			} else if strings.HasSuffix(n, ".zip") {
				s += 4
			}
			if goarch == "arm64" {
				if strings.Contains(n, "arm64") || strings.Contains(n, "aarch64") {
					s += 8
				} else if strings.Contains(n, "x64") || strings.Contains(n, "x86_64") || strings.Contains(n, "amd64") {
					s -= 4
				}
			} else {
				if strings.Contains(n, "x64") || strings.Contains(n, "x86_64") || strings.Contains(n, "amd64") {
					s += 8
				} else if strings.Contains(n, "arm64") {
					s -= 4
				}
			}
		case "windows":
			if !strings.Contains(n, "win") && !strings.HasSuffix(n, ".zip") && !strings.HasSuffix(n, ".exe") {
				return -1
			}
			if strings.Contains(n, "macos") || strings.Contains(n, "darwin") || strings.Contains(n, "linux") ||
				strings.HasSuffix(n, ".dmg") || strings.HasSuffix(n, ".tar.xz") || strings.HasSuffix(n, ".appimage") {
				return -1
			}
			s += 10
			if strings.HasSuffix(n, ".zip") {
				s += 6
			}
			if strings.Contains(n, "x64") || strings.Contains(n, "amd64") || strings.Contains(n, "win64") {
				s += 5
			}
		case "linux":
			if !strings.Contains(n, "linux") && !strings.HasSuffix(n, ".tar.xz") && !strings.HasSuffix(n, ".appimage") {
				return -1
			}
			s += 10
			if strings.HasSuffix(n, ".appimage") {
				s += 7
			} else if strings.HasSuffix(n, ".tar.xz") {
				s += 5
			} else if strings.HasSuffix(n, ".zip") {
				s += 3
			}
			if goarch == "arm64" && isArm {
				s += 6
			} else if goarch != "arm64" && isX64 {
				s += 6
			}
		default:
			return -1
		}
		return s
	}

	bestIdx := -1
	bestScore := -1
	for i, a := range assets {
		if sc := score(a.Name); sc > bestScore {
			bestScore = sc
			bestIdx = i
		}
	}
	if bestIdx < 0 {
		return nil
	}
	cp := assets[bestIdx]
	if cp.SHA256 == "" && cp.Digest != "" {
		cp.SHA256 = NormalizeSHA256(cp.Digest)
	}
	if cp.ChecksumURL == "" {
		if cs := FindChecksumAsset(assets, cp.Name); cs != nil {
			cp.ChecksumURL = cs.DownloadURL
		}
	}
	return &cp
}

// NormalizeSHA256 extracts a canonical lowercase 64-hex SHA-256 digest from either
// a bare hex string or a GitHub API digest string ("sha256:<64hex>"). Returns "" if invalid.
func NormalizeSHA256(raw string) string {
	s := strings.TrimSpace(raw)
	if len(s) > 7 && strings.EqualFold(s[:7], "sha256:") {
		s = strings.TrimSpace(s[7:])
	}
	if !isHex64(s) {
		return ""
	}
	return strings.ToLower(s)
}

// parseReleaseBodySHA256 extracts a SHA-256 hash explicitly paired with assetName
// in a GitHub release notes body (never falling back to an unlabeled hash).
func parseReleaseBodySHA256(body, assetName string) (string, bool) {
	wantBase := checksumBaseName(assetName)
	if wantBase == "" {
		return "", false
	}
	// Strip lightweight markdown formatting (backticks, bold, table pipes) per line.
	cleaned := strings.NewReplacer("`", " ", "|", " ", "**", " ").Replace(body)
	for _, line := range strings.Split(cleaned, "\n") {
		if !strings.Contains(strings.ToLower(line), strings.ToLower(wantBase)) {
			continue
		}
		if dig, err := ParseSHA256Checksum(line, wantBase); err == nil && dig != "" {
			return dig, true
		}
		// Also support "<assetName>: sha256:<hex>" or "<assetName> <hex>" inline lines.
		for _, tok := range strings.Fields(line) {
			tok = strings.Trim(tok, "()[]<>,;:\"'")
			if dig := NormalizeSHA256(tok); dig != "" {
				return dig, true
			}
		}
	}
	return "", false
}

// FindChecksumAsset locates a matching published SHA-256 checksum asset for assetName,
// preferring an exact per-asset sidecar (<name>.sha256) and falling back to a release
// manifest (SHA256SUMS / checksums.txt).
func FindChecksumAsset(assets []ReleaseAsset, assetName string) *ReleaseAsset {
	base := strings.ToLower(checksumBaseName(assetName))
	if base == "" {
		return nil
	}
	exactTargets := []string{
		base + ".sha256",
		base + ".sha256sum",
		base + ".sha256sums",
		base + ".sha256.txt",
	}
	for _, want := range exactTargets {
		for i := range assets {
			if strings.ToLower(strings.TrimSpace(assets[i].Name)) == want && assets[i].DownloadURL != "" {
				cp := assets[i]
				return &cp
			}
		}
	}
	manifestNames := []string{
		"sha256sums",
		"sha256sums.txt",
		"checksums.sha256",
		"checksums.txt",
		"sha256.txt",
	}
	for _, want := range manifestNames {
		for i := range assets {
			if strings.ToLower(strings.TrimSpace(assets[i].Name)) == want && assets[i].DownloadURL != "" {
				cp := assets[i]
				return &cp
			}
		}
	}
	return nil
}

func checksumBaseName(name string) string {
	s := strings.TrimSpace(strings.ReplaceAll(name, `\`, "/"))
	if s == "" {
		return ""
	}
	b := path.Base(s)
	if b == "." || b == "/" {
		return ""
	}
	return b
}

// ParseSHA256Checksum extracts the expected 64-character hex SHA-256 digest for
// assetName from a sidecar .sha256 file or a multi-line SHA256SUMS manifest
// (supporting bare hex, GNU "<hash>  <file>", and BSD "SHA256 (<file>) = <hash>").
func ParseSHA256Checksum(raw, assetName string) (string, error) {
	wantBase := checksumBaseName(assetName)
	type entry struct {
		digest string
		file   string
	}
	var entries []entry
	for _, line := range strings.Split(raw, "\n") {
		s := strings.TrimSpace(line)
		if s == "" || strings.HasPrefix(s, "#") {
			continue
		}
		// BSD style: SHA256 (filename) = <64hex>
		if strings.HasPrefix(strings.ToUpper(s), "SHA256") {
			if lp := strings.IndexByte(s, '('); lp >= 0 {
				if rp := strings.IndexByte(s[lp+1:], ')'); rp >= 0 {
					fname := strings.TrimSpace(s[lp+1 : lp+1+rp])
					rest := s[lp+1+rp+1:]
					if eq := strings.IndexByte(rest, '='); eq >= 0 {
						dig := strings.TrimSpace(rest[eq+1:])
						if isHex64(dig) {
							entries = append(entries, entry{digest: strings.ToLower(dig), file: fname})
							continue
						}
					}
				}
			}
		}
		// GNU / bare style: <64hex> [*filename]
		fields := strings.Fields(s)
		if len(fields) == 0 || !isHex64(fields[0]) {
			continue
		}
		fname := ""
		if len(fields) >= 2 {
			fname = strings.TrimPrefix(strings.TrimSpace(strings.TrimPrefix(s, fields[0])), "*")
			fname = strings.TrimSpace(fname)
		}
		entries = append(entries, entry{digest: strings.ToLower(fields[0]), file: fname})
	}
	if len(entries) == 0 {
		return "", errors.New("fpchromium: no valid SHA-256 digest found in checksum payload")
	}
	if wantBase != "" {
		// 1. Exact base filename match.
		for _, e := range entries {
			if e.file != "" && checksumBaseName(e.file) == wantBase {
				return e.digest, nil
			}
		}
		// 2. Case-insensitive base filename match.
		for _, e := range entries {
			if e.file != "" && strings.EqualFold(checksumBaseName(e.file), wantBase) {
				return e.digest, nil
			}
		}
	}
	// 3. Single bare hash (no filename specified in sidecar).
	if len(entries) == 1 && entries[0].file == "" {
		return entries[0].digest, nil
	}
	return "", fmt.Errorf("fpchromium: no SHA-256 entry matching %q in checksum manifest", wantBase)
}

func isHex64(s string) bool {
	if len(s) != 64 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

func fetchExpectedSHA256(ctx context.Context, cl *http.Client, checksumURL, assetName string) (string, error) {
	reqCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, checksumURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "fingerprint-chromium-manager")
	if cl == nil {
		cl = http.DefaultClient
	}
	resp, err := cl.Do(req)
	if err != nil {
		return "", fmt.Errorf("fpchromium: fetch checksum %s: %w", checksumURL, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fpchromium: fetch checksum returned HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("fpchromium: read checksum: %w", err)
	}
	return ParseSHA256Checksum(string(body), assetName)
}

func (a ReleaseAsset) expectedSHA256(ctx context.Context, cl *http.Client) (string, error) {
	if raw := strings.TrimSpace(a.SHA256); raw != "" {
		digest := NormalizeSHA256(raw)
		if digest == "" {
			return "", fmt.Errorf("fpchromium: invalid expected sha256 %q", a.SHA256)
		}
		return digest, nil
	}
	if digest := NormalizeSHA256(a.Digest); digest != "" {
		return digest, nil
	}
	if url := strings.TrimSpace(a.ChecksumURL); url != "" {
		return fetchExpectedSHA256(ctx, cl, url, filepath.Base(strings.TrimSpace(a.Name)))
	}
	return "", nil
}

// DownloadRelease streams a ReleaseAsset into destDir/<asset.Name> using atomic
// temp-file rename, verifies SHA-256 whenever asset.SHA256 or asset.ChecksumURL is
// set, and invokes onProgress(downloaded, total) periodically.
// Unlike FetchLatestRelease, downloads never impose a short total http.Client.Timeout
// (150-250 MB Chromium archives rely on ctx cancellation instead).
func (m *KernelManager) DownloadRelease(ctx context.Context, asset ReleaseAsset, destDir string, onProgress func(done, total int64)) (string, error) {
	baseName := filepath.Base(strings.TrimSpace(asset.Name))
	if asset.DownloadURL == "" || baseName == "" || baseName == "." || baseName == ".." {
		return "", errors.New("fpchromium: invalid release asset")
	}
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return "", fmt.Errorf("fpchromium: create download dir: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, asset.DownloadURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "fingerprint-chromium-manager")

	// Clone client with Timeout=0 so large release archives don't abort after 20s.
	cl := &http.Client{}
	if m.HTTPClient != nil {
		cl.Transport = m.HTTPClient.Transport
		cl.CheckRedirect = m.HTTPClient.CheckRedirect
		cl.Jar = m.HTTPClient.Jar
	}

	expectedSHA, err := asset.expectedSHA256(ctx, cl)
	if err != nil {
		return "", err
	}

	resp, err := cl.Do(req)
	if err != nil {
		return "", fmt.Errorf("fpchromium: download %s: %w", baseName, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fpchromium: download %s returned HTTP %d", baseName, resp.StatusCode)
	}

	total := resp.ContentLength
	if total <= 0 {
		total = asset.Size
	}

	asset.SHA256 = expectedSHA
	return asset.SaveDownload(ctx, destDir, func(path string) error {
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_TRUNC, 0o600)
		if err != nil {
			return err
		}
		var done int64
		buf := make([]byte, 64*1024)
		for {
			n, rerr := resp.Body.Read(buf)
			if n > 0 {
				if _, werr := f.Write(buf[:n]); werr != nil {
					_ = f.Close()
					return werr
				}
				done += int64(n)
				if onProgress != nil {
					onProgress(done, total)
				}
			}
			if rerr == io.EOF {
				break
			}
			if rerr != nil {
				_ = f.Close()
				return rerr
			}
		}
		if err := f.Close(); err != nil {
			return err
		}
		return nil
	})
}

// SaveDownload stages a host transport's output and verifies it before publication.
// A failed mirror or a cancelled download leaves the existing archive intact.
func (a ReleaseAsset) SaveDownload(ctx context.Context, dir string, fetch func(path string) error) (string, error) {
	name := strings.TrimSpace(a.Name)
	if name == "" || name == "." || !filepath.IsLocal(name) || strings.ContainsAny(name, "/\\:") {
		return "", fmt.Errorf("fingerprintkernel: invalid release filename %q", name)
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	tmp, err := os.CreateTemp(dir, ".download-*.part")
	if err != nil {
		return "", err
	}
	path := tmp.Name()
	defer os.Remove(path)
	if err = tmp.Close(); err != nil {
		return "", err
	}
	if err = fetch(path); err != nil {
		return "", err
	}
	if a.SHA256 != "" {
		if err := VerifyFileSHA256(ctx, path, a.SHA256); err != nil {
			return "", err
		}
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if strings.HasSuffix(strings.ToLower(name), ".appimage") {
		if err := os.Chmod(path, 0o755); err != nil {
			return "", err
		}
	}
	final := filepath.Join(dir, name)
	if err := os.Rename(path, final); err != nil {
		return "", err
	}
	return final, nil
}
