# OpenBrowser architecture and security assessment

Research date: 2026-08-13  
Repository: [`lyu0805/OpenBrowser`](https://github.com/lyu0805/OpenBrowser)  
Pinned source snapshot: [`cb9842a8b0d63475d96f7dd3b9948b949996c501`](https://github.com/lyu0805/OpenBrowser/commit/cb9842a8b0d63475d96f7dd3b9948b949996c501), authored 2026-08-05  
Comparison release: [`v1.0.4`](https://github.com/lyu0805/OpenBrowser/releases/tag/v1.0.4), commit `5d93bdf67ee9e2a029e4945270f88f0a89716455`, published 2026-07-26 UTC

## Executive decision

OpenBrowser is useful as a **pattern catalogue**, not as the base or dependency for the planned Windows Rust control plane. Its strongest reusable ideas are one validated User Data root per identity, a per-profile launch lock, a random loopback CDP port, pre-navigation setup, and explicit separation between a stable persona layer and an exit-IP-derived layer. Its product and trust boundaries do not match ours:

- it is an Electron/Node/JavaScript application, not Rust;
- Windows packages run a third-party Wayfern Chromium-derived kernel, not an official user-owned Chrome instance;
- profiles, proxy credentials, platform passwords/TOTP and exported cookies can be stored in plaintext application JSON, and optional backup can sync them;
- the Local API/MCP/RPA surface has profile lifecycle and arbitrary page-JavaScript powers, while the underlying CDP endpoint is separately reachable on loopback without API-key authentication;
- proxy readiness can be configured to fall back to direct or continue, and an unchecked proxy can start with only a warning;
- application updates are downloaded from a mutable GitHub release channel without an in-app digest/signature verification step; Windows signing is absent from the build script and macOS defaults to ad-hoc signing.

For the authorized e-commerce seller product, reuse the invariants and tests after independently implementing them in Rust. Do not reuse the credential/session model, remote-control breadth, fail-open proxy policies, bundled kernel supply chain, fingerprint claims, or updater.

## Evidence model and limits

“Code-proven” below means the behavior is directly represented in the pinned source. It does **not** mean it worked in a packaged binary or against any Seller Platform. “Project claim” means first-party README/release documentation without equivalent runtime proof. “Unverified” means it needs execution, packet capture, binary inspection, or an authorized Store Account.

This review inspected the pinned Git tree, manifests, source, tests, workflow, tags, release metadata/assets and first-party documentation. It did **not** download the roughly 0.27–1.21 GB release artifacts, fetch Git LFS kernel payloads, run `npm ci`, execute self-tests, install the application, inspect signatures, capture traffic, or log into any Seller Platform. The checked-out kernel files were LFS pointers; consequently, the proprietary/third-party browser binaries and their behavior were not source-audited.

## 1. Implementation and browser ownership

### Code-proven

The control application is Electron 43.1.1 plus Node/CommonJS JavaScript, HTML and CSS. Small native helpers are C#, with Python, PowerShell and shell build/patch scripts. There is no Rust application layer. The package manifest declares only the aliased Electron host and `rcedit` as development dependencies and exposes numerous Node self-test scripts ([`package.json`](https://github.com/lyu0805/OpenBrowser/blob/cb9842a8b0d63475d96f7dd3b9948b949996c501/Browserapp/package.json#L1-L46)).

The browser is a separately spawned executable controlled with command-line flags and CDP. The engine prefers an “independent kernel,” disables implicit system-browser fallback, and only permits installed Chrome/Edge after explicit manual policy selection ([`engine.js`](https://github.com/lyu0805/OpenBrowser/blob/cb9842a8b0d63475d96f7dd3b9948b949996c501/Browserapp/engine.js#L162-L223), [`engine.js`](https://github.com/lyu0805/OpenBrowser/blob/cb9842a8b0d63475d96f7dd3b9948b949996c501/Browserapp/engine.js#L654-L685)). Runtime kernel auto-download is disabled: the manager resolves packaged/local seeds or an explicitly selected custom binary ([`browser-kernel.js`](https://github.com/lyu0805/OpenBrowser/blob/cb9842a8b0d63475d96f7dd3b9948b949996c501/Browserapp/automation/browser-kernel.js#L1-L8), [`browser-kernel.js`](https://github.com/lyu0805/OpenBrowser/blob/cb9842a8b0d63475d96f7dd3b9948b949996c501/Browserapp/automation/browser-kernel.js#L1210-L1304)).

Kernel ownership varies by package: Windows x64 and macOS arm64 package Wayfern, macOS x64 packages an `OpenBrowser 148` kernel, and Ubuntu x64 prepares Chrome for Testing. The packaging code makes those platform choices explicitly ([`package-portable.js`](https://github.com/lyu0805/OpenBrowser/blob/cb9842a8b0d63475d96f7dd3b9948b949996c501/Browserapp/scripts/package-portable.js#L116-L137)). The project attributes Wayfern/Donut Browser in its README and checks kernel binaries into Git LFS paths; it does not contain Chromium source sufficient to reproduce/audit those binaries ([README](https://github.com/lyu0805/OpenBrowser/blob/cb9842a8b0d63475d96f7dd3b9948b949996c501/README.md#L142-L151)).

### Implication

This is not the desired “Rust control plane + official Chrome/Chromium we can update and attest” model. On Windows, browser-runtime correctness and security inherit an opaque third-party binary and its terms. A fresh Rust implementation should own a pinned official Chrome for Testing/Chromium acquisition and verification policy, or use an installed official Chrome only through an explicit supported contract.

## 2. Profile isolation and lifecycle

### Code-proven

Each identity maps to `{profileDataRoot}/{validatedProfileId}` and launches Chromium with that directory as `--user-data-dir`, always using its `Default` subprofile. Cache and crash directories are also placed below the identity root. CDP asks Chromium for an ephemeral port with `--remote-debugging-port=0` ([`engine.js`](https://github.com/lyu0805/OpenBrowser/blob/cb9842a8b0d63475d96f7dd3b9948b949996c501/Browserapp/engine.js#L1814-L1860), [`engine.js`](https://github.com/lyu0805/OpenBrowser/blob/cb9842a8b0d63475d96f7dd3b9948b949996c501/Browserapp/engine.js#L1897-L1913)).

The isolation module:

- restricts IDs to `[A-Za-z0-9_-]{1,64}`;
- rejects the filesystem root, system Chrome/Edge/Chromium/Brave/Vivaldi data roots, and roots that contain or are contained by them;
- resolves real paths and rejects a symlink/junction at the configured root or profile root;
- requires the exact `{dataRoot}/{profileId}` path;
- guards destructive child paths against escape and intermediate symlinks;
- serializes launches using an exclusive per-profile lock with a random owner token, and refuses to steal a lock whose PID is alive;
- audits duplicate User Data roots and CDP ports among running profiles.

These behaviors are directly visible in [`isolation.js`](https://github.com/lyu0805/OpenBrowser/blob/cb9842a8b0d63475d96f7dd3b9948b949996c501/Browserapp/automation/isolation.js#L8-L112), [`isolation.js`](https://github.com/lyu0805/OpenBrowser/blob/cb9842a8b0d63475d96f7dd3b9948b949996c501/Browserapp/automation/isolation.js#L123-L237), and [`isolation.js`](https://github.com/lyu0805/OpenBrowser/blob/cb9842a8b0d63475d96f7dd3b9948b949996c501/Browserapp/automation/isolation.js#L239-L353). Deletion first stops a profile, revalidates its exact root and retries Windows lock-related removal errors ([`engine.js`](https://github.com/lyu0805/OpenBrowser/blob/cb9842a8b0d63475d96f7dd3b9948b949996c501/Browserapp/engine.js#L2420-L2452)).

### Gaps and unverified properties

This is strong **logical browser-storage separation**, not a secret-security boundary. All identities run under one OS account, one control process, one Local API key and one extension-management authority. A process with access to the user account, engine state, profile files or CDP ports can cross identities. No per-profile OS user, sandbox token, ACL boundary, encrypted vault or brokered capability is established.

The audit checks collisions in the in-memory running set; it is not proof that a second application instance or unrelated Chrome process cannot open the same root. Crash recovery, 20 concurrent windows, Windows antivirus/file-lock behavior and actual Cookie/storage non-bleed were not exercised.

## 3. Browser Persona and fingerprint mechanism

### Code-proven

The persona is deterministic per profile: `SHA-256(profile.id)` seeds User-Agent, CPU, memory, screen, Canvas/WebGL/Audio/client-rect marks, media devices, battery, voices, device name and WebRTC local-IP values. A launch-only random seed is used only when refresh-on-start is selected and site-stability mode is off ([`fingerprint.js`](https://github.com/lyu0805/OpenBrowser/blob/cb9842a8b0d63475d96f7dd3b9948b949996c501/Browserapp/automation/fingerprint.js#L592-L650)). Network-derived timezone, geolocation and public WebRTC address are kept as a dynamic layer, separate from stable fields ([`fingerprint.js`](https://github.com/lyu0805/OpenBrowser/blob/cb9842a8b0d63475d96f7dd3b9948b949996c501/Browserapp/automation/fingerprint.js#L784-L817), [`fingerprint.js`](https://github.com/lyu0805/OpenBrowser/blob/cb9842a8b0d63475d96f7dd3b9948b949996c501/Browserapp/automation/fingerprint.js#L836-L945)). The identity is profile-stable, not per-origin.

On the stock-browser path the engine applies UA/Client Hints, language, timezone and geolocation through CDP, then registers a document-start script and also evaluates it in the current document. The script wraps JavaScript-visible Navigator, Screen, Canvas, WebGL, Audio, media-device, voice, battery, WebGPU and related surfaces ([`fingerprint.js`](https://github.com/lyu0805/OpenBrowser/blob/cb9842a8b0d63475d96f7dd3b9948b949996c501/Browserapp/automation/fingerprint.js#L1-L18), [`fingerprint.js`](https://github.com/lyu0805/OpenBrowser/blob/cb9842a8b0d63475d96f7dd3b9948b949996c501/Browserapp/automation/fingerprint.js#L2169-L2287)). New page/iframe/worker targets are auto-attached and paused for injection, with a polling/watch fallback ([`engine.js`](https://github.com/lyu0805/OpenBrowser/blob/cb9842a8b0d63475d96f7dd3b9948b949996c501/Browserapp/engine.js#L1053-L1137)).

An opt-in device-persona mode bundles CPU, RAM, display, DPR and GPU strings from hard-coded Windows/macOS/Linux combinations. It is deliberately off by default so upgrades do not silently mutate existing identities ([`device-personas.js`](https://github.com/lyu0805/OpenBrowser/blob/cb9842a8b0d63475d96f7dd3b9948b949996c501/Browserapp/automation/device-personas.js#L3-L19), [`fingerprint.js`](https://github.com/lyu0805/OpenBrowser/blob/cb9842a8b0d63475d96f7dd3b9948b949996c501/Browserapp/automation/fingerprint.js#L698-L728)). On the special macOS-x64 OpenBrowser 148 path, the same identity is also written to a kernel `init.json`; JS pixel noise is then suppressed to avoid double-noising ([`kernel-init-sync.js`](https://github.com/lyu0805/OpenBrowser/blob/cb9842a8b0d63475d96f7dd3b9948b949996c501/Browserapp/automation/kernel-init-sync.js#L1-L7), [`kernel-init-sync.js`](https://github.com/lyu0805/OpenBrowser/blob/cb9842a8b0d63475d96f7dd3b9948b949996c501/Browserapp/automation/kernel-init-sync.js#L428-L495)).

### Unsafe assumptions / unverified claims

- JavaScript/CDP patching is not proof of a coherent browser identity. The module itself says MAC/device-name/file-protocol and full TLS/JA3 surfaces require the kernel, and its font code acknowledges that measurement-based font detection still sees host fonts on stock Chromium ([`fingerprint.js`](https://github.com/lyu0805/OpenBrowser/blob/cb9842a8b0d63475d96f7dd3b9948b949996c501/Browserapp/automation/fingerprint.js#L12-L17), [`fingerprint.js`](https://github.com/lyu0805/OpenBrowser/blob/cb9842a8b0d63475d96f7dd3b9948b949996c501/Browserapp/automation/fingerprint.js#L1320-L1326)).
- Default mode samples hardware axes independently; coherent device personas are opt-in. Hard-coded combinations are plausible assertions without first-party hardware measurement provenance.
- Injection failures can soft-fail and the browser can remain open. Repeated wrappers, cross-realm objects, early scripts, workers, extensions, browser UI, native APIs and kernel behavior need runtime proof.
- There is no static or runtime evidence here that any Seller Platform accepts these personas, that they reduce account risk, or that “anti-detection” is achievable. The project disclaimer correctly disclaims anonymity, uniqueness and site compatibility ([DISCLAIMER](https://github.com/lyu0805/OpenBrowser/blob/cb9842a8b0d63475d96f7dd3b9948b949996c501/DISCLAIMER.md)).

Our product should prefer a small, invariant-driven persona contract aligned with the actual Windows host/browser and fixed network exit. Do not promise spoofing or transplant this patch surface wholesale.

## 4. Proxy and egress behavior

### Code-proven

Profiles accept HTTP, HTTPS, SOCKS4 and SOCKS5 syntax. Authenticated HTTP/HTTPS/SOCKS5 proxies are hidden from Chrome behind a random loopback bridge; the bridge forwards upstream credentials and returns failures rather than directly dialing the destination ([`proxy-forwarder.js`](https://github.com/lyu0805/OpenBrowser/blob/cb9842a8b0d63475d96f7dd3b9948b949996c501/Browserapp/proxy-forwarder.js#L16-L40), [`proxy-forwarder.js`](https://github.com/lyu0805/OpenBrowser/blob/cb9842a8b0d63475d96f7dd3b9948b949996c501/Browserapp/proxy-forwarder.js#L315-L351), [`proxy-forwarder.js`](https://github.com/lyu0805/OpenBrowser/blob/cb9842a8b0d63475d96f7dd3b9948b949996c501/Browserapp/proxy-forwarder.js#L392-L484)). The bridge binds to `127.0.0.1` on an ephemeral port. Chrome receives `--proxy-server`, a loopback bypass, and `--disable-quic` when proxied ([`engine.js`](https://github.com/lyu0805/OpenBrowser/blob/cb9842a8b0d63475d96f7dd3b9948b949996c501/Browserapp/engine.js#L1957-L1974), [`engine.js`](https://github.com/lyu0805/OpenBrowser/blob/cb9842a8b0d63475d96f7dd3b9948b949996c501/Browserapp/engine.js#L2011-L2014)).

Proxy checks query multiple public IP/geo services through the configured proxy, persist exit metadata, and can derive locale/timezone from it. Primary, backup and dynamically extracted proxy endpoints are supported ([`engine.js`](https://github.com/lyu0805/OpenBrowser/blob/cb9842a8b0d63475d96f7dd3b9948b949996c501/Browserapp/engine.js#L2457-L2588), [`proxy-forwarder.js`](https://github.com/lyu0805/OpenBrowser/blob/cb9842a8b0d63475d96f7dd3b9948b949996c501/Browserapp/proxy-forwarder.js#L1065-L1095)).

### Contract-breaking behavior

Fail-closed is not unconditional. A failed check may use configured `direct` or `continue` policy. More importantly, if the profile requests proxy mode but startup checking was not enabled and no prior exit IP exists, it emits `proxy-unchecked` and starts anyway ([`engine.js`](https://github.com/lyu0805/OpenBrowser/blob/cb9842a8b0d63475d96f7dd3b9948b949996c501/Browserapp/engine.js#L2628-L2715)). Direct mode follows the system proxy by default unless `systemProxy=off`, so “Direct” is not necessarily a known physical egress.

No packet capture proves DNS, WebRTC, crash reporter, extension, browser component or background traffic cannot bypass the intended exit. The Node TLS “Chrome” profile explicitly cannot reproduce GREASE/extension ordering without a custom dialer ([`proxy-forwarder.js`](https://github.com/lyu0805/OpenBrowser/blob/cb9842a8b0d63475d96f7dd3b9948b949996c501/Browserapp/proxy-forwarder.js#L51-L63)). Our design must eliminate direct/continue, require an authenticated egress probe before every launch, bind the verified endpoint immutably to the identity, and add packet-level no-bypass tests.

## 5. Local API, MCP, CDP and authority

### Code-proven

The HTTP API always binds to `127.0.0.1:50325` by default and always has a key: a 32-byte random base64url key is generated for each process unless `OPENBROWSER_API_KEY` is supplied. Every route, including status, checks `api-key`, `x-api-key` or bearer auth using timing-safe comparison. Browser-origin requests are denied unless explicitly allowlisted; non-browser clients without an `Origin` header are accepted with the key. Bodies are capped at 1 MiB ([`local-api-server.js`](https://github.com/lyu0805/OpenBrowser/blob/cb9842a8b0d63475d96f7dd3b9948b949996c501/Browserapp/automation/local-api-server.js#L7-L21), [`local-api-server.js`](https://github.com/lyu0805/OpenBrowser/blob/cb9842a8b0d63475d96f7dd3b9948b949996c501/Browserapp/automation/local-api-server.js#L61-L155)).

The API can list/create/delete/start/stop profiles, returns running debug ports, manages proxies/extensions/window sync, reads fingerprints/isolation audits, and runs RPA steps ([`local-api-server.js`](https://github.com/lyu0805/OpenBrowser/blob/cb9842a8b0d63475d96f7dd3b9948b949996c501/Browserapp/automation/local-api-server.js#L175-L335), [`local-api-server.js`](https://github.com/lyu0805/OpenBrowser/blob/cb9842a8b0d63475d96f7dd3b9948b949996c501/Browserapp/automation/local-api-server.js#L337-L465)). The separate stdio MCP server merely forwards a subset of these calls using an environment-provided API key; it adds no authorization or confirmation layer ([`mcp-server.js`](https://github.com/lyu0805/OpenBrowser/blob/cb9842a8b0d63475d96f7dd3b9948b949996c501/Browserapp/automation/mcp-server.js#L1-L38), [`mcp-server.js`](https://github.com/lyu0805/OpenBrowser/blob/cb9842a8b0d63475d96f7dd3b9948b949996c501/Browserapp/automation/mcp-server.js#L61-L207)). RPA accepts `evaluate/javascript` and sends caller-provided expressions to `Runtime.evaluate` in the logged-in page ([`rpa-engine.js`](https://github.com/lyu0805/OpenBrowser/blob/cb9842a8b0d63475d96f7dd3b9948b949996c501/Browserapp/automation/rpa-engine.js#L960-L981)).

Chromium itself exposes a random loopback CDP endpoint and allows localhost origins, but CDP has no OpenBrowser API-key layer. The Local API reveals the port to authenticated clients; any same-user local process that discovers `DevToolsActivePort` or enumerates loopback can attempt direct CDP access. This is effectively full browser/session authority.

### Fit decision

Do not ship Local API/MCP in MVP. Later, use a Rust broker with per-capability, per-profile, short-lived tokens; never return raw CDP ports; require explicit operator consent for session-sensitive actions; remove arbitrary JavaScript evaluation; audit every invocation; and keep it disabled by default.

## 6. Credentials, sessions and persistence

### Code-proven

The main process persists its complete profile objects to `openbrowser-engine.json`. Those objects include up to 500 KB of exported Cookie JSON, proxy URLs with credentials, platform username/password/TOTP, and persistence/backup flags. The source explicitly says these secrets live in main-process state rather than renderer localStorage, but the write is ordinary JSON with no OS-vault encryption or restrictive file mode specified ([`engine.js`](https://github.com/lyu0805/OpenBrowser/blob/cb9842a8b0d63475d96f7dd3b9948b949996c501/Browserapp/engine.js#L230-L290), [`engine.js`](https://github.com/lyu0805/OpenBrowser/blob/cb9842a8b0d63475d96f7dd3b9948b949996c501/Browserapp/engine.js#L374-L421)). The proxy library separately stores username, password and raw authenticated URL in plaintext JSON ([`proxy-store.js`](https://github.com/lyu0805/OpenBrowser/blob/cb9842a8b0d63475d96f7dd3b9948b949996c501/Browserapp/automation/proxy-store.js#L16-L59), [`proxy-store.js`](https://github.com/lyu0805/OpenBrowser/blob/cb9842a8b0d63475d96f7dd3b9948b949996c501/Browserapp/automation/proxy-store.js#L62-L88)). Cloud provider passwords/tokens and the backup passphrase are likewise part of plaintext local settings.

The engine can import and export all profile cookies through CDP. On close it exports cookies when cloud backup is enabled, stores them back in the profile object, and persists them ([`engine.js`](https://github.com/lyu0805/OpenBrowser/blob/cb9842a8b0d63475d96f7dd3b9948b949996c501/Browserapp/engine.js#L826-L861), [`engine.js`](https://github.com/lyu0805/OpenBrowser/blob/cb9842a8b0d63475d96f7dd3b9948b949996c501/Browserapp/engine.js#L2345-L2363)). Backup payloads explicitly include cookies, passwords, TOTP, proxies, fingerprint/preferences and optional browser data. Encryption is optional: with no passphrase the gzip payload is uploaded unencrypted; with a passphrase it uses scrypt and AES-256-GCM ([`cloud-sync.js`](https://github.com/lyu0805/OpenBrowser/blob/cb9842a8b0d63475d96f7dd3b9948b949996c501/Browserapp/automation/cloud-sync.js#L3-L8), [`cloud-sync.js`](https://github.com/lyu0805/OpenBrowser/blob/cb9842a8b0d63475d96f7dd3b9948b949996c501/Browserapp/automation/cloud-sync.js#L38-L70), [`cloud-sync.js`](https://github.com/lyu0805/OpenBrowser/blob/cb9842a8b0d63475d96f7dd3b9948b949996c501/Browserapp/automation/cloud-sync.js#L73-L99)).

This directly conflicts with our locked boundary that the application never reads, exports or syncs site passwords, cookies or login tokens. Our Rust application should treat the Chrome User Data directory as opaque; store only proxy credentials/API secrets in Windows Credential Manager/DPAPI through a narrow secret-service interface; never model seller credentials/TOTP; never expose a Cookie API; and exclude profile cloning/cloud backup from MVP.

## 7. Update and distribution trust

### Code-proven

The application polls the repository’s GitHub releases at startup and every six hours, resolves a platform-specific exact asset name, restricts download URLs to HTTPS GitHub asset hosts, caps downloads at 1 GiB, writes a random-named file into Downloads, and asks the OS to open it ([`main.js`](https://github.com/lyu0805/OpenBrowser/blob/cb9842a8b0d63475d96f7dd3b9948b949996c501/Browserapp/main.js#L45-L57), [`main.js`](https://github.com/lyu0805/OpenBrowser/blob/cb9842a8b0d63475d96f7dd3b9948b949996c501/Browserapp/main.js#L287-L373), [`main.js`](https://github.com/lyu0805/OpenBrowser/blob/cb9842a8b0d63475d96f7dd3b9948b949996c501/Browserapp/main.js#L375-L445)). It does not compare an expected SHA-256, verify an artifact signature, bind release metadata to a commit, or perform an authenticated atomic updater install.

The Windows packaging path brands Electron and produces a portable zip/optional NSIS installer, but contains no Authenticode signing step. macOS signing defaults to `codesign --sign -` (ad hoc), has no notarization step, and its launcher clears quarantine; a Developer ID is only an optional environment override ([`package-portable.js`](https://github.com/lyu0805/OpenBrowser/blob/cb9842a8b0d63475d96f7dd3b9948b949996c501/Browserapp/scripts/package-portable.js#L443-L516), [`package-portable.js`](https://github.com/lyu0805/OpenBrowser/blob/cb9842a8b0d63475d96f7dd3b9948b949996c501/Browserapp/scripts/package-portable.js#L529-L552), [`package-portable.js`](https://github.com/lyu0805/OpenBrowser/blob/cb9842a8b0d63475d96f7dd3b9948b949996c501/Browserapp/scripts/package-portable.js#L681-L730)).

The release workflow is manual (`workflow_dispatch`), uses tag text supplied by the operator, publishes with `gh release upload --clobber`, and does not generate provenance/SBOM or sign the artifacts. It runs selected self-tests before upload, but GitHub Actions dependencies are tag-pinned rather than commit-SHA-pinned ([workflow](https://github.com/lyu0805/OpenBrowser/blob/cb9842a8b0d63475d96f7dd3b9948b949996c501/.github/workflows/build-installers.yml#L1-L31), [`publish-release.sh`](https://github.com/lyu0805/OpenBrowser/blob/cb9842a8b0d63475d96f7dd3b9948b949996c501/Browserapp/scripts/publish-release.sh#L1-L18)).

Our updater must be a separate decision: signed Windows artifacts, immutable manifest containing version/commit/size/hash, signature verification before execution, anti-rollback, atomic install/recovery and a controlled release gate. Never trust release filename and transport alone.

## 8. Tests, releases, maintenance and installation

### First-party evidence

- The repository contains many focused Node “selftests” for isolation, fingerprint stability, proxy parsing/forwarding, API, cloud, kernel policy and UI wiring. The package manifest enumerates them, but the release workflow only runs `selftest`, automation, protocol, isolation and kernel suites before packaging ([`package.json`](https://github.com/lyu0805/OpenBrowser/blob/cb9842a8b0d63475d96f7dd3b9948b949996c501/Browserapp/package.json#L8-L41), [workflow](https://github.com/lyu0805/OpenBrowser/blob/cb9842a8b0d63475d96f7dd3b9948b949996c501/.github/workflows/build-installers.yml#L153-L162)). There is no always-on push/PR test workflow in the pinned tree.
- The latest public release is v1.0.4. It predates the pinned head by 11 commits. Its four public assets are Windows x64 EXE/ZIP and macOS arm64/x64 DMGs; the current README/workflow advertises Ubuntu, but v1.0.4 has no Linux asset. Thus head documentation and the available release are not the same product snapshot ([v1.0.4 release](https://github.com/lyu0805/OpenBrowser/releases/tag/v1.0.4), [commit history](https://github.com/lyu0805/OpenBrowser/commits/cb9842a8b0d63475d96f7dd3b9948b949996c501/)).
- The repository began on 2026-07-19 and reached the pinned head on 2026-08-05 with 104 commits and five tags. Activity is recent, but the history is very young and contributions are highly concentrated. This is active experimentation, not evidence of long-term maintenance.
- Source installation is documented as Node LTS, `npm ci --include=dev`, `npm run selftest`, `npm start`; packaging is platform-native and kernel-specific ([README](https://github.com/lyu0805/OpenBrowser/blob/cb9842a8b0d63475d96f7dd3b9948b949996c501/README.md#L55-L101)). Node’s lockfile helps reproduce JS dependencies, but browser kernels are Git LFS/third-party inputs and platform packaging is not hermetic. This review did not execute that path.

### License

The repository root declares MIT ([LICENSE](https://github.com/lyu0805/OpenBrowser/blob/cb9842a8b0d63475d96f7dd3b9948b949996c501/LICENSE)). However, first-party third-party notices say the Windows native input-mirroring code adapts `chrome-power-app` under AGPL-3.0 and includes the corresponding source/license ([THIRD-PARTY-NOTICES](https://github.com/lyu0805/OpenBrowser/blob/cb9842a8b0d63475d96f7dd3b9948b949996c501/Browserapp/THIRD-PARTY-NOTICES.md#L1-L16)). Wayfern has separate terms. Therefore “the repo is MIT” is not a safe blanket conclusion for copying or distributing all components. Independently reimplement required behavior and obtain legal review before reusing any native helper or kernel material.

## Reusable patterns for the Rust design

1. Make `BrowserIdentityId` a strict opaque identifier and derive exactly one canonical User Data root beneath an application-owned base.
2. Canonicalize and reject system-browser roots, filesystem roots, symlinks/junctions, alternate Windows casing and path escapes both at configuration time and immediately before every destructive operation.
3. Hold an OS-level exclusive per-identity lease across browser lifetime; include owner PID/start time/random token and handle stale ownership defensively.
4. Ask Chrome for an ephemeral loopback CDP port, discover it through `DevToolsActivePort`, and never expose it outside a narrow broker.
5. Start on `about:blank`, install required policy/locale settings before navigating to a Seller Platform, and make setup failure fatal rather than a soft warning.
6. Separate immutable identity attributes from verified egress-derived attributes. Changing either should be an explicit versioned identity transition, never incidental randomization.
7. Probe the configured proxy through the same transport before launch and retain observed IP/timezone/country as evidence, while independently packet-testing no-bypass behavior.
8. Use atomic state-file replacement and bounded, allowlisted destructive operations; port the isolation tests as Rust unit/property tests and Windows integration tests.

## Required departures for the e-commerce seller product

- Use Rust as the control plane and a pinned, attestable official browser runtime; do not bundle or auto-fetch Wayfern/OpenBrowser kernels.
- Keep seller authentication solely in the opaque Chrome profile. Remove credential/TOTP fields, Cookie import/export, profile cloning, browser-data backup and cloud sync.
- Put proxy secrets and app secrets in Windows Credential Manager/DPAPI, with non-secret metadata in the local database and restrictive ACLs on every profile/state directory.
- Make fixed-proxy readiness an unconditional launch gate. Remove direct/continue fallback and forbid system-proxy inheritance for identities that require a proxy.
- Keep Local API/MCP off in MVP. If later enabled, use least-privilege capabilities and never expose CDP or arbitrary JavaScript.
- Define a conservative Windows persona tied to the actual Chrome version/host and test internal consistency. Treat compatibility as a test target, not an “undetectable” promise.
- Require signed, hash-bound, rollback-safe releases and verify the complete Windows install/update chain.
- Acceptance must include authorized login and selected Platform Acceptance Journeys on each claimed Seller Platform, cross-profile storage tests, proxy-loss tests, crash/recovery tests, 20-window capacity tests, log-secret scans, and packet capture. None of these is proven by OpenBrowser’s source or selftests.

## Bottom line

OpenBrowser demonstrates that a small desktop control plane can orchestrate many isolated Chromium User Data directories and layer per-profile configuration on top. It also demonstrates why the control plane must be treated as a high-value credential and session broker. For our product, copy the **shape of the isolation invariants**, not its code or security boundary. The pivotal architecture remains Rust orchestration around official Chrome/Chromium, opaque browser-owned sessions, vault-backed proxy secrets, fail-closed egress, no default automation surface, and evidence-driven Seller Platform acceptance.
