# Rust + Tauri v2 for a commercial fingerprint browser

Research snapshot: **2026-08-15**. Scope: Tauri is only the desktop adapter/control-panel shell; managed browsing happens in separate stock Chrome/Chromium processes. Sources are first-party Tauri, Rust, Microsoft, and Apple material. “Fact” below means documented behavior; “inference” is an architecture recommendation for this product; “runtime gap” requires a native build or production-like test.

## Verdict

**Recommendation: choose Rust + Tauri v2 if the product-defining code is the BrowserControl control plane.** It is the strongest structural fit for a Windows-first commercial product whose hard problems are process ownership, profile isolation, fail-closed egress, native secret storage, recovery, and a capability-limited Local API. Use React/TypeScript for the operator UI, but keep Tauri as a replaceable adapter around a deep, Tauri-free Rust module.

This is not a recommendation to render seller sites inside Tauri. The control panel should contain bundled, owned UI only; every untrusted/e-commerce page stays in external Chrome. That choice removes most system-WebView compatibility questions from the product's actual browsing identity and materially reduces the privileged-WebView attack surface.

Tauri is a poor fit if the company primarily has Electron expertise, must ship a very large plugin-heavy UI immediately, cannot staff Rust/Windows systems engineering, or needs one identical embedded renderer across operating systems. Those conditions can outweigh its architectural advantages.

## Pinned baseline

- The latest Tauri core release visible on the official repository is **Tauri 2.11.5**, released 2026-07-01; production should start at 2.11.5 or later, not float on `2` ([official release](https://github.com/tauri-apps/tauri/releases/tag/tauri-v2.11.5)). This matters because the 2026 origin-confusion advisory affected `2.0` through `2.11.0` and was patched in `2.11.1` ([GHSA-7gmj-67g7-phm9](https://github.com/tauri-apps/tauri/security/advisories/GHSA-7gmj-67g7-phm9)).
- The current Rust stable point release is **1.97.1**, released 2026-07-16 ([Rust release announcement](https://blog.rust-lang.org/2026/07/16/Rust-1.97.1/)). Pin it in `rust-toolchain.toml`; update deliberately after the native matrix passes.
- Tauri and its official plugins are released independently. Commit `Cargo.lock` and the frontend lockfile; do not assume “Tauri v2” implies one coordinated plugin version. Cargo's `--locked` mode fails if resolution would change ([Cargo metadata](https://doc.rust-lang.org/cargo/commands/cargo-metadata.html)).

## Commercial scorecard for this product

Scores are reasoned product-fit judgments, not measured benchmarks.

| Axis | Score | Why |
| --- | ---: | --- |
| Fit with a Rust BrowserControl core | **9/10** | Rust core and OS adapters stay in one language; no FFI bridge is required between shell backend and product core. |
| Process/network/native integration | **9/10** | Microsoft-maintained `windows`/`windows-sys` bindings expose Win32/COM/WinRT; Tauri does not constrain direct Job Object, WFP, DPAPI, named-pipe, or installer work ([windows-rs](https://github.com/microsoft/windows-rs)). |
| Security controllability | **8/10** | Capabilities, permissions, scopes, CSP, and central runtime authority are useful controls, but core/plugin code has full OS authority and an XSS/IPC mistake can still become native compromise. |
| UI delivery/productivity | **8/10** | React/TypeScript works normally and can hot-reload; Rust changes still recompile and restart the native process. |
| Packaging/signing/updating | **8/10** | NSIS/MSI, DMG/app bundle, signing hooks, and a mandatory-signature updater are provided; certificate custody, notarization, rollout, rollback, and native-host choreography remain ours. |
| Testability | **8/10** | A Tauri-free core is easy to test; Tauri has a mock runtime and current WebdriverIO integration across desktop platforms, but Job/WFP/Chrome/update behavior still needs real Windows hosts. |
| Runtime/package efficiency | **8/10** | The shell dynamically uses the OS WebView rather than shipping another renderer. No total-process claim is made: 20 external Chrome windows will dominate resources. |
| Renderer predictability | **6/10** | WebView2 Evergreen and WKWebView evolve outside the app release; this is manageable for a small owned control UI, not suitable as the managed browsing engine. |
| Crash reporting/observability | **6/10** | First-party file logging exists, but the reviewed official feature set has no first-party native minidump upload/symbolication product; the listed Sentry integration is community-owned. |
| Ecosystem/support risk | **7/10** | Core and official plugins are active and security-reviewed, but the ecosystem is smaller than Electron's and community plugins must be treated as third-party supply-chain code. |
| Hiring/organizational fit | **7/10** | React hiring is easy; Rust plus Windows process/network/security expertise is scarcer. This is a qualitative staffing risk, not a performance defect. |

**Overall fit: 8/10, conditional on a Rust-capable systems team and a deliberately thin Tauri layer.**

## The seam: keep Tauri out of the domain

The deep module should offer a small interface that hides Chrome discovery, launch arguments, profile locks, proxy startup, OS policy, reconciliation, and durable state transitions:

```text
React/TypeScript control panel (untrusted presentation)
        |
        | small, typed invoke/query DTOs; no secrets, paths, shell, SQL, or raw CDP
        v
desktop-tauri adapter
        |
        | maps IPC DTOs <-> application commands/results
        v
BrowserControl module (deep interface; no tauri::* types)
        |
        +-- IdentityRepository adapter ------ SQLite
        +-- SecretStore adapter ------------- DPAPI/CredMan; Keychain later
        +-- BrowserRuntime adapter ---------- stock Chrome + MV3/native host
        +-- ProcessPolicy adapter ----------- Windows Job Objects
        +-- EgressPolicy adapter ------------ local proxy + WFP/firewall
        +-- Clock/EventSink adapters -------- tests/observability
```

Suggested workspace shape:

```text
crates/browser-domain/          # Identity, Persona, NetworkEgress, lifecycle invariants
crates/browser-control/         # deep module; Command/Query interface and reconciliation
crates/control-contract/        # versioned serde DTOs shared by UI/CLI/local API
crates/storage-sqlite/          # migrations, transactions, recovery journal
crates/secrets-windows/         # DPAPI or Credential Manager adapter
crates/platform-windows/        # Job/WFP/firewall/process/window adapters
crates/chrome-runtime/          # launch, profile lock, extension/native-host/CDP subset
crates/local-proxy/             # egress engine and health proof
apps/desktop-tauri/             # Tauri commands, capabilities, React assets, updater
apps/realbrowser-cli/           # second adapter; admin/recovery and hermetic testing
```

`browser-domain` and `browser-control` must not depend on `tauri`, `tauri::AppHandle`, WebView labels, JSON field casing, UI event names, or plugin types. Tauri commands should not mirror every repository method. Prefer a few application-level operations such as `create_identity`, `start_identity`, `stop_identity`, `archive_identity`, `test_egress`, `list_identities`, and `subscribe_status`; validate identity and authorization again in Rust.

The CLI/local-automation surface supplies the second real adapter at the BrowserControl seam. If later requirements say managed browsers must survive UI-core crashes or desktop updates, move the same module into a per-user `browserd` and put a versioned, per-user-ACL named-pipe adapter at the seam. Windows named-pipe security descriptors control both ends; the default descriptor is too broad for this use, so explicitly restrict it to the logon SID ([Microsoft named-pipe security](https://learn.microsoft.com/en-us/windows/win32/ipc/named-pipe-security-and-access-rights)). Do **not** introduce the daemon merely for fashion: it adds installation, upgrade, crash, and protocol-version complexity.

## Process, security, and frontend trust

**Facts.** Tauri uses a Rust Core process and OS-provided WebView processes, routes IPC through Core, and recommends keeping secrets/business logic out of the frontend ([process model](https://v2.tauri.app/concept/process-model/)). IPC is asynchronous message passing with commands and events ([IPC](https://v2.tauri.app/concept/inter-process-communication/)). The runtime authority checks origin, capability, permission, and applicable scopes before invoking a command ([runtime authority](https://v2.tauri.app/security/runtime-authority/)). Capabilities grant permissions to selected windows/webviews; overlapping capabilities merge their authority ([capabilities](https://v2.tauri.app/security/capabilities/)). CSP is opt-in configuration, and Tauri explicitly advises against remote scripts/content ([CSP](https://v2.tauri.app/security/csp/)).

**Product rules (inference).**

- Treat the React WebView as untrusted presentation. It receives redacted summaries and opaque identity IDs, never proxy passwords, Cookie values, raw secret references, arbitrary filesystem paths, shell commands, or CDP endpoints.
- Bundle all UI assets. No remote URLs, CDN scripts, arbitrary navigation, or seller-platform content in a privileged Tauri WebView. Set a restrictive CSP and explicit capability allowlist; avoid wildcard windows and remote capability URLs.
- Do not expose `shell`, generic filesystem, SQL execute, HTTP, or updater installation directly to JavaScript. Rust-owned commands call internal modules. The official SQL plugin is intentionally a frontend-to-SQL interface and `sql:allow-execute` permits writes ([SQL plugin](https://v2.tauri.app/plugin/sql/)); that is the wrong trust direction here.
- A Tauri security release is an urgent product dependency. The 2026 advisory proves that local-origin classification can be security-critical even with capabilities; pinning and rapid patch response are part of the commercial operating model, not optional hygiene.
- Tauri's external-binary support can package target-triple-specific sidecars and scope their execution ([sidecars](https://v2.tauri.app/develop/sidecar/)). Nevertheless, Chrome, native-messaging host, and proxy processes must be launched by Rust-owned BrowserControl logic, never by a frontend shell permission.

## System WebView and resource implications

**Fact.** Tauri dynamically links the platform WebView: WebView2 on Windows, WKWebView on macOS, and WebKitGTK on Linux; it does not bundle those libraries in the application binary ([Tauri process model](https://v2.tauri.app/concept/process-model/)). WebView2 Evergreen is shared and updates automatically; applications cannot require one exact Evergreen version, admins can delay updates, and Microsoft recommends forward-compatibility tests and feature detection. A Fixed Version runtime transfers update ownership to the vendor and adds more than 250 MB to the package ([Microsoft WebView2 distribution](https://learn.microsoft.com/en-us/microsoft-edge/webview2/concepts/distribution)). Apple release notes explicitly cover WKWebView changes across Safari/OS releases ([Safari release notes](https://developer.apple.com/documentation/safari-release-notes)).

**Recommendation.** Use Evergreen WebView2 for the Windows control panel, feature-detect any non-basic API, record its version in diagnostics, and test preview/stable rings. Do not spend package size and security-servicing capacity on Fixed Version unless an enterprise customer proves an offline/locked-runtime requirement. On macOS, test every supported OS; do not claim renderer parity. These UI runtimes have no bearing on the external Chrome persona.

**Resource inference.** Tauri avoids shipping and running a second embedded Chromium/Node stack, so it should reduce shell packaging/runtime overhead relative to Electron. No numeric memory/CPU claim is justified without a product prototype. At the target of 20 concurrent Chrome windows, profile content and Chrome processes dominate; benchmark full scenarios, not an empty window.

## Windows-native control is a Rust concern, not a Tauri feature

- Job Objects can contain a Chrome process tree; child processes join by default, and `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE` can terminate the associated tree ([Job Objects](https://learn.microsoft.com/en-us/windows/win32/procthread/job-objects)). Implement this behind `ProcessPolicy`, with tests for breakaway, pre-existing jobs, crash recovery, PID reuse, and graceful-versus-forced shutdown.
- WFP provides filtering hooks and allow/block decisions across Windows networking layers; ALE can classify the first packet of a connection ([WFP overview](https://learn.microsoft.com/en-us/windows/win32/fwp/about-windows-filtering-platform)). Put it behind `EgressPolicy`. Whether user-mode filters alone can prove per-identity TCP/UDP/DNS fail-closed behavior, what elevation is needed, and how filters survive crashes are **runtime gaps**. Tauri neither solves nor blocks them.
- The local proxy should own proxy authentication, DNS behavior, connection proof, and redacted telemetry. Decide in-process versus isolated worker from a crash/fail-closed prototype. A sidecar is packaging, not containment or correct lifecycle by itself.
- Native Messaging and minimal, version-probed CDP belong in `chrome-runtime`; Tauri must never pass those raw capabilities to React.

## State and secrets

Use a Rust-owned SQLite connection behind `IdentityRepository`; include migrations, exclusive identity leases, a transition journal, and reconciliation after unclean exit. The WebView asks for domain operations, not SQL. The official plugin proves SQLite/migration support exists, but its JavaScript-facing shape is not required here ([Tauri SQL](https://v2.tauri.app/plugin/sql/)).

For Windows MVP proxy credentials/API tokens, prefer an injected OS adapter:

- DPAPI normally limits decryption to the same user credentials and machine and provides integrity protection ([`CryptProtectData`](https://learn.microsoft.com/en-us/windows/win32/api/dpapi/nf-dpapi-cryptprotectdata)); or use Credential Manager for application-defined generic credentials ([Credentials Management](https://learn.microsoft.com/en-us/windows/win32/secauthn/credentials-management)). Choose after testing backup/account-reset and support behavior.
- On macOS, Keychain is the corresponding encrypted small-secret store ([Apple Keychain Services](https://developer.apple.com/documentation/security/keychain-services)).
- Tauri Stronghold is a first-party encrypted-vault option ([Stronghold plugin](https://v2.tauri.app/plugin/stronghold/)), but it creates a separate vault/password/recovery lifecycle and can be called from JavaScript if permissions are granted. Use it only if cross-platform vault semantics are consciously preferred over OS stores; never expose it to the WebView.
- Chrome's site passwords/Cookies remain inside each Chrome User Data directory. BrowserControl should not duplicate them into SQLite, Stronghold, logs, crash reports, or the frontend.

## Packaging, signing, updating, and supply chain

Tauri builds Windows MSI (WiX) or NSIS setup executables; its docs note native Windows MSI builds and caveats for cross-building ([Windows installer](https://v2.tauri.app/distribute/windows-installer/)). For a commercial Windows-first release, build, sign, install, update, and smoke-test on a native Windows release host. Tauri supports a custom signing command and Azure Artifact Signing ([Windows code signing](https://v2.tauri.app/distribute/sign/windows/)). On macOS, use Developer ID/Application signing and notarization; ad-hoc signing is not public commercial distribution ([macOS signing](https://v2.tauri.app/distribute/sign/macos/)).

The updater requires a signature and cannot disable verification. Its public key is embedded/configured in the app and loss of the private key prevents further updates to installed users ([Tauri updater](https://v2.tauri.app/plugin/updater/)). This is a strong primitive, not a complete release system. The product still needs offline root-key custody, key-rotation/recovery design, immutable artifact identity, staged channels, downgrade rules, atomic coordination of app/native host/proxy/extension/schema versions, and a rollback/recovery test. The UI may request an update check; Rust owns download/install policy.

Tauri code is MIT or MIT/Apache-2.0 where applicable ([repository](https://github.com/tauri-apps/tauri)). That is commercially friendly but does not clear transitive crates, npm packages, extensions, sidecars, codecs, or redistributed runtimes. Commit both lockfiles; inventory enabled Cargo features and target-specific graphs (`cargo metadata`, `cargo tree --format "{p} {l}"` supports license fields), generate a release SBOM for both ecosystems, preserve notices/source obligations, scan advisories, and attach the SBOM/provenance to each signed artifact ([Cargo tree](https://doc.rust-lang.org/cargo/commands/cargo-tree.html)). Tauri publishes information on human-reviewed releases and automated security audits, but the application vendor remains responsible for its dependency closure ([ecosystem security](https://v2.tauri.app/security/ecosystem/)).

## Reliability, observability, testing, and debugging

The official log plugin can write/rotate/filter logs ([logging](https://v2.tauri.app/plugin/logging/)). It is not crash capture. The official feature catalogue labels native crash/minidump Sentry support as a **community** plugin, not an official feature ([plugin catalogue](https://v2.tauri.app/plugin/)). Therefore provide product-owned structured events, correlation by opaque Identity ID, secret/URL/body redaction, panic hooks, Windows crash dumps and symbols, WebView process-failure signals, Chrome exit/crash classification, proxy/WFP state, and opt-in upload policy. Keep operational logs useful without recording Cookies, tokens, proxy passwords, full URLs/query strings, or page content.

Testing layers:

1. Test `browser-control` through its public interface with fake clock, repositories, process, egress, secret, and event adapters; observable state transitions are the test surface.
2. Run native Windows integration tests for SQLite crash recovery, profile locks, Job containment, proxy/DNS/UDP leaks, WFP install/remove/reboot behavior, DPAPI/CredMan, and updater interruption. These cannot be proven by Tauri mocks.
3. Tauri's mock runtime supports native-free unit/integration tests; current official docs also describe WebdriverIO Tauri testing on Windows/Linux/macOS and direct `tauri-driver` only on Windows/Linux ([tests](https://v2.tauri.app/develop/tests/), [WebDriver](https://v2.tauri.app/develop/tests/webdriver/)). Run packaged control-panel journeys on each supported native OS and WebView ring.
4. Separately run real external-Chrome journeys: create/start/stop/restart identities, verify profile isolation and declared egress, exercise extension/native-host protocol, crash the app/proxy/Chrome independently, and update across schema/protocol versions.

Rust's ownership model provides compile-time memory-safety guarantees without a garbage collector, and its type system catches many concurrency errors at compile time ([ownership](https://doc.rust-lang.org/book/ch04-00-understanding-ownership.html), [concurrency](https://doc.rust-lang.org/book/ch16-00-concurrency.html)). These are valuable for a concurrent supervisor, but they do not prove correct Win32 handles, firewall rules, Chrome semantics, unsafe FFI, or security policy. Code review and native fault-injection remain mandatory.

## Productivity and support risks

- **Compile loop:** TypeScript UI changes can hot-reload; Rust changes rebuild/restart the core. Cargo uses incremental artifacts for development and exposes timings/caches, but no compile-speed promise should be made before measuring this workspace ([Cargo build cache](https://doc.rust-lang.org/cargo/reference/build-cache.html)). Keep Tauri adapter small, split stable crates, and measure cold/warm developer and CI builds.
- **Two-language staffing:** React work remains mainstream, while core engineers need Rust plus Windows internals. The payoff is locality: the hard control-plane logic is not duplicated across a TS host and native helpers.
- **Plugin policy:** Prefer Tauri core and reviewed official plugins; wrap each behind an internal adapter. Avoid community plugins for privileged paths unless source, maintenance, permissions, platform behavior, and update cadence are audited. Vendor or replace small critical integrations when commercial support requires it.
- **Support model:** Tauri is community/open-source infrastructure rather than a full commercial desktop platform SLA. Paid services mentioned in docs are optional third parties. Budget ownership for framework triage, upstream tracking, and emergency rebuilds.

## Decision gates before commitment

Proceed with Tauri only after a short Windows proof establishes all of the following:

1. Packaged/signed app can launch and reconcile two, then 20, independent stock-Chrome identities without profile or process crossover.
2. Job containment and shutdown semantics hold under Chrome's real multiprocess behavior.
3. Proxy failure produces no TCP/UDP/DNS direct egress; WFP/firewall setup, elevation, cleanup, reboot, and uninstall behavior are acceptable.
4. The WebView has no remote navigation and cannot access secrets, SQL, shell, arbitrary files, raw CDP, or proxy credentials through IPC.
5. Evergreen WebView2 stable/preview compatibility, WebView crash recovery, and accessibility meet control-panel needs.
6. Native Windows installer signing and a signed updater exercise upgrade, interrupted update, schema migration, extension/native-host version skew, and rollback/recovery.
7. Cold/warm build time, installer size, idle control-panel footprint, and full 20-Chrome scenario are measured on target hardware; no framework-only benchmark substitutes for this.

If these gates pass, Tauri should remain the desktop adapter. If they fail, the failure will almost certainly be in Windows/Chrome product mechanics rather than the WebView shell; because the domain and BrowserControl module contain no Tauri types, switching the adapter remains tractable.
