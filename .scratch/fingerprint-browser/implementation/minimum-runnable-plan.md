# Minimum Runnable Implementation Plan

Status: in implementation  
Decision date: 2026-08-15  
Locked stack: React + TypeScript + Tauri v2 + Tauri-free Rust `BrowserControl`

Implementation checkpoint (2026-08-15, superseding the earlier Stock Chrome slice): the only runtime is the product-distributed Chromium named RealBrowser. `browser-platform` verifies `realbrowser-kernel.json`, product id, version/major and executable SHA-256 before discovery, start and reconciliation; absence fails closed and no local Google Chrome path remains. After the Profile lease, Rust writes a secret-free K0 `persona.json` and launches with `--realbrowser-persona-file`. K1 changes only Canvas 2D readback copies in Blink and must pass the top-frame/iframe/dedicated-worker observation before `graphics.canvas` is projected as `CustomKernel`. WebGL, Audio, TLS and JS hooks remain out of scope.

Earlier installed-Chrome native evidence is historical only and does not satisfy this checkpoint. Current acceptance must use two RealBrowser parent processes from the product kernel directory, with no Google Chrome process launched by the product. The stable dev `.app` uses bundle id `com.realbrowser.desktop.tauri.dev`, `.dev/data`, and `.dev/kernel` (or `REALBROWSER_KERNEL_DIR`). Native Windows build/runtime/installer and the complete cross-platform §3 journey remain open until rerun against the product kernel.

The minimum operator UI has a Vitest/jsdom interaction lane through its fake desktop adapter. Regressions cover stable Host-owned display codes after list reordering and a stopped identity's `Direct → SOCKS5 proxy → saved Proxy` drawer journey. Native Computer Use configured `http://127.0.0.1:65534`, confirmed that Proxy launch with native WebRTC is refused with `代理模式需将 WebRTC 设为禁止直连 UDP`, then changed WebRTC through the Persona drawer and verified the real Chrome 151.0.7922.138 process contained `--force-webrtc-ip-handling-policy=disable_non_proxied_udp --proxy-server=http://127.0.0.1:65534 --disable-quic`. The identity was stopped and restored to `Direct / Native`. The same UI Stop path was rerun independently and verified that the managed parent, every process carrying its Profile root, and its LaunchServices/Dock registration disappeared while an ordinary Chrome parent remained live. A Rust coherence test covers the same `proxy_requires_non_proxied_udp` refusal.

The Tauri IPC contract removes Profile roots, Chrome executable paths, application data roots and process IDs from WebView DTOs. Rust projects `profileMode`, `personaMode`, versioned secret-free Network config, `egressMode`, browser version and new-identity defaults; React maps the closed values to concise Chinese labels. Serialization tests assert path/PID and proxy credential keys are absent. Native Computer Use confirmed `独立 / 原生 / 直连` and the saved `代理` projection.

The error boundary is also split: `ControlError` retains internal diagnostic strings for Rust-side ownership while `public_message()` returns fixed remediation text paired with the existing stable error code. `CommandFailure` no longer serializes `error.to_string()`. A third Tauri contract test injects a path/token-shaped runtime diagnostic and proves neither reaches IPC.

The current Persona Runtime pass adds `Target.setDiscoverTargets` plus recursive tab/page attachment. A native Chrome integration creates a second tab/page target and independently observes a managed IANA timezone plus Viewport, Screen and DPR, preventing the initial-page-only regression. The Rust schema accepts every named zone present in its bundled database, migrates the four legacy v2 tokens, removes the v3 display placeholders without enabling them, and rejects unknown or incoherent values before launch. Real Tauri Computer Use exercised `原生 → 1366×768 / 1920×1080 / 1.25× → 已配置 → 打开 → 已观测`, confirmed `运行观测 / 3 项一致`, stopped through the UI, and restored `原生 / 直连`. Capability coverage remains intentionally limited to the declared top-frame observation contract until iframe and worker acceptance exists.

Profile storage isolation now has a native regression independent of the UI and of Chrome's database formats. A loopback-only test origin writes `account=A` and `account=B` as persistent Cookies and localStorage through two concurrently running User Data roots, closes both CDP clients, stops both managed Chrome instances, restarts the same roots, and observes A only in Profile A and B only in Profile B. The application still treats Chrome-owned storage as opaque; the test exercises the website API rather than reading Cookie/Login Data files. Real named-account acceptance on a selected authorized site remains a separate manual gate.

## 1. Purpose

Build the smallest end-to-end vertical slice that proves the selected desktop stack and the core Browser Identity lifecycle with the verified RealBrowser product Chromium on native macOS and Windows hosts.

This plan deliberately distinguishes:

- **MR-0, minimum runnable engineering slice:** a persistent multi-profile RealBrowser manager with a configurable, capability-gated Persona envelope and Direct or fixed no-auth/IP-allowlist Network Egress. K0+K1 are real; fields without an execution and observation backend remain visible but read-only.
- **Commercial MVP:** MR-0 plus a verified Persona Runtime, fail-closed proxy, extension trust, secret lifecycle, signed release/update, threat-model and capacity gates.

The plan references [`listing-fields.md`](/Volumes/Acasis/Code/REPO/ozon/ozon/ozon-pod/docs/listing-fields.md) for its production/assembly discipline, not for Ozon-specific fields or serialization.

## 2. What is borrowed from `listing-fields.md`

| Listing discipline | Browser-control translation |
| --- | --- |
| Field + value + origin | `IdentityFieldDefinition` + typed `IdentityDraft` value + `FieldValueOrigin` |
| Five producers | MR-0 has only `Manual`, `Generated`, and `SystemDetected`; later Template/Proxy-derived values add explicit origin kinds |
| `normalize_and_validate` is canonical | Rust alone converts `IdentityDraft` into `NormalizedIdentitySnapshot`; React never reimplements readiness |
| One payload assembler | Rust alone converts the normalized snapshot into `LaunchPlan`; React never sends executable paths, arguments or environment variables |
| Draft revision/CAS | Every configuration write carries `expected_revision`; conflicting UI writes fail explicitly |
| Workflow is projection, listing is truth | React/Tauri DTOs are projections; SQLite records and Rust state-machine rules are authoritative |
| Golden final payload | Golden fixtures assert normalized snapshots and platform launch plans, not just nearby helper functions |

The resulting pipeline is:

```text
produce                           normalize                         assemble                       execute
Manual / Generated / Detected -> IdentityDraft + origins -> NormalizedIdentitySnapshot -> LaunchPlan -> BrowserSession
                                      Rust authority                  Rust authority            platform Adapter
```

## 3. MR-0 definition of runnable

MR-0 is complete only when an operator can perform this journey on native Windows 11 x64 and macOS Apple Silicon builds:

1. Start the React/Tauri desktop application.
2. Create two Browser Identities named `Store A` and `Store B`.
3. See that each identity is assigned a different application-owned, non-default full RealBrowser User Data root.
4. Configure Store B language, timezone, window size, Viewport/Screen/DPR and WebRTC policy; persist the revision, compile launch mappings, and observe the managed runtime fields.
5. Configure Store B with a fixed HTTP, HTTPS, or SOCKS5 proxy host and port plus `WebRTC: 禁止直连 UDP`; persist it and observe the complete Rust-assembled RealBrowser argument.
6. Start both identities in the verified RealBrowser product Chromium; missing product kernel fails closed.
7. Log into the same test/e-commerce site with two different authorized accounts.
8. Attempting to start either identity again fails with a stable `AlreadyRunning` error and does not start a second process on the same Profile.
9. A Persona field without an available backend cannot be configured as managed; the UI shows its actual native/Profile capability instead of creating an unapplied value.
10. Stop both identities gracefully, close the application, restart it, and relaunch both identities.
11. Each Profile retains only its own login state; no cookies/storage/history cross into the other identity.
12. A forced application/RealBrowser termination is reconciled on next launch without deleting Chromium lock files or taking ownership of a Google Chrome or unknown process.
13. Both native unsigned development bundles build from committed lockfiles and pass their platform verification lane.

MR-0 explicitly shows `Native` or `Managed`, a backend label per Persona field, and `Egress: Direct` or `Proxy`. It makes no claim of fingerprint-surface effectiveness, proxy leak prevention, anonymity, undetectability, or compatibility beyond tested browser/platform/runtime versions.

## 4. MR-0 scope

### Included

- React/TypeScript control panel rendered from bundled local assets.
- Tauri v2 desktop adapter with a minimal capability allowlist and restrictive CSP.
- Product Chromium manifest/hash/version verification and fail-closed availability reporting.
- Browser Identity create, rename, list, start, stop and recoverable archive.
- One complete User Data root per Browser Identity.
- Application-level Profile lease plus platform process containment.
- SQLite metadata, revision CAS and runtime reconciliation journal.
- Persistent Persona configuration for locale, timezone, window size, coherent Viewport/Screen/DPR and WebRTC, plus a capability catalogue for Canvas, WebGL, WebGPU, Audio, Navigator, fonts, media devices and hardware.
- Product Chromium launch compilation for locale, window size and WebRTC, plus K0 `persona.json`.
- K1 seeded/idempotent copy-only Canvas 2D readback for top frame, iframe and dedicated-worker OffscreenCanvas, with native function identity.
- Fail-loud validation for persisted values whose execution backend is unavailable.
- Direct or one fixed HTTP/HTTPS/SOCKS5 no-auth/IP-allowlist proxy, visibly declared and compiled by Rust; Proxy start requires the non-proxied-UDP WebRTC policy.
- Optional `http://` or `https://` startup URL.
- Structured, secret-free diagnostics and stable product error codes.
- Native macOS ARM64 and Windows 11 x64 unsigned build lanes.

### Excluded until later tickets resolve

- Browser-side application of WebGL/WebGPU/Audio/Navigator/font/media-device/hardware policies across frames and workers.
- Proxy credentials, rotation, geographic derivation, connectivity testing and fail-closed WFP/macOS network enforcement.
- First-party MV3 extension, Native Messaging, and CDP management of fields beyond the current timezone/display top-frame contract.
- Cookie import/export, Profile cloning, Profile backup/migration and website-password/2FA storage.
- Identity Templates, batch start/stop, window arrangement/synchronization and 20-window capacity.
- Local API, MCP, RPA or generic automation.
- Team/cloud synchronization, licensing/billing and public distribution.
- Any claim of being undetectable, and any kernel surface beyond K0+K1.

These exclusions are intentional. They keep MR-0 runnable without pre-deciding open Persona, Network Egress, extension, secret and release contracts.

## 5. Authoritative model

### 5.1 Field definitions

MR-0 uses a closed, Rust-owned field schema. React receives labels/editor metadata but does not decide validity.

| Field | Type/editor | Required | Producer/origin | Rule |
| --- | --- | --- | --- | --- |
| `name` | trimmed text | yes | `Manual` | 1–80 Unicode scalar values; unique name is not required |
| `startup_url` | optional URL | no | `Manual` | only `http`/`https`; empty means `about:blank` |
| `product_kernel` | read-only product id | yes at launch | `SystemDetected` | must be `com.realbrowser.browser` from the verified manifest |
| `browser_version` | read-only text | yes at launch | `SystemDetected` | re-verified from the product manifest, executable hash and `RealBrowser --version` before every launch |
| `profile_root` | read-only canonical path | yes | `Generated` | derived from identity UUID under the product data root |
| `persona` | structured config | yes | `Manual` + `Generated` | Rust validates and compiles supported launch mappings; runtime-only surfaces are fail-loud |
| `network` | versioned config | yes | `Manual` + `Generated` | Direct, or HTTP/HTTPS/SOCKS5 host + port; credentials are not accepted |
| `egress_mode` | enum | yes | `Generated` | derived by Rust as `Direct` or `Proxy` |

`FieldValueOriginKind` initially contains exactly `Manual`, `Generated`, and `SystemDetected`. Do not add placeholder enum variants for features that do not exist. Migrations introduce later kinds when Template or proxy-derived writes become real.

### 5.2 Durable records

```rust
BrowserIdentityRecord {
    id,
    revision,
    name,
    startup_url,
    lifecycle: Active | Archived,
    field_origins,
    created_at,
    updated_at,
}

RuntimeProjection {
    identity_id,
    state: Stopped | Starting | Running | Stopping | Failed,
    session_id,
    browser_pid,
    observed_browser_version,
    started_at,
    last_error,
}
```

Persistent Identity lifecycle and transient runtime state are separate. `Running` is never trusted from SQLite alone: startup reconciliation derives it from the process/container facts, application lease and journal.

### 5.3 Normalized launch input

`normalize_and_validate` is a pure Rust function:

```text
IdentityDraft + detected BrowserInventory + product DataRoot
    -> Accepted(NormalizedIdentitySnapshot)
    -> Rejected(Vec<FieldIssue>)
```

The accepted snapshot contains the canonical UUID, revision, trimmed values, canonical Profile path, verified RealBrowser path/version, Persona, normalized Direct/Proxy Network config and startup URL. `FieldIssue` contains stable `code`, field key and remediation; React renders it without reproducing the rule.

### 5.4 One launch-plan assembler

`plan_launch(snapshot)` is the only function allowed to create a RealBrowser executable/argument/environment plan. It hides:

- browser path and signature/version policy;
- `--user-data-dir` construction;
- safe fixed launch switches;
- environment scrubbing;
- containment setup and readiness timeout;
- startup URL;
- platform-specific process metadata.

No Tauri command or TypeScript code may accept or return arbitrary executable paths, argument arrays, environment variables, raw handles or CDP endpoints.

## 6. Deep Module and seams

The external `BrowserControl` Module exposes two operations and one read-only event projection:

```rust
trait BrowserControl {
    fn execute(&self, command: ControlCommand) -> Result<ControlResult, ControlError>;
    fn query(&self, query: ControlQuery) -> Result<ControlView, ControlError>;
    fn events(&self, after: EventCursor) -> Result<Vec<ControlEvent>, ControlError>;
}
```

`ControlCommand` is a closed enum for Create, Rename, Start, Stop, Archive and Reconcile. `ControlQuery` covers identity list/detail and runtime status. The Interface includes these invariants:

- configuration mutations use `expected_revision`;
- Start is idempotently rejected when an identity already owns a runtime lease;
- Start does not return `Running` until Profile ownership and platform containment exist;
- Stop reports graceful versus forced termination;
- Archive is rejected while running and never deletes Profile data;
- errors are stable codes with redacted diagnostic detail;
- events are ordered per identity and carry an application sequence.

The implementation hides SQLite, filesystem layout, product-kernel verification, launch-plan assembly, process handles, locks, journal reconciliation and platform cleanup.

Real seams and Adapters:

| Seam | Production Adapters | Test Adapter |
| --- | --- | --- |
| `IdentityRepository` | SQLite | in-memory repository |
| `PlatformRuntime` | Windows Job/RealBrowser; macOS process/RealBrowser | deterministic fake runtime |
| `ProductKernelResolver` | packaged `realbrowser-kernel.json` plus executable hash/version verification | fixed product manifest |
| `Clock/IdSource` | system clock/UUID | deterministic clock/IDs |

Filesystem behavior is local-substitutable and tested with temporary directories rather than exposed as a public port. `browser-network` is a pure schema/normalization/compiler Module; no SecretStore or proxy-credential Adapter exists until authenticated proxy support has a second real implementation.

## 7. Workspace shape

```text
Cargo.toml                         # Rust workspace
rust-toolchain.toml                # pinned Rust
package.json / pnpm-lock.yaml      # pinned frontend tooling
pnpm-workspace.yaml

apps/desktop/
  package.json
  src/                             # React/TypeScript presentation
  src-tauri/
    Cargo.toml
    tauri.conf.json
    capabilities/
    src/                           # thin Tauri Adapter/composition root

crates/browser-control/
  src/domain/                      # records, commands, issues, state rules
  src/normalize.rs
  src/plan.rs
  src/repository/                  # SQLite + in-memory Adapter
  src/control.rs                   # deep Module implementation

crates/browser-platform/
  src/windows/                     # product Chromium verification, Job, shutdown
  src/macos/                       # product Chromium verification, process containment
  src/fake.rs

crates/browser-persona/
  src/lib.rs                        # Persona schema, K0 file contract, validation and capability gate

crates/browser-persona-runtime/
  src/lib.rs                        # Loopback CDP attach, observation and target replay

crates/browser-network/
  src/lib.rs                        # Network schema, proxy validation and Chromium argument compiler

fixtures/browser-control/
  valid-native-direct.json
  invalid-startup-url.json
  launch-plan-windows-x64.json
  launch-plan-macos-arm64.json
```

`browser-persona` and `browser-network` are small leaf crates because their schemas and compilers are shared by control, storage and IPC. Keep the remaining DAG wide and avoid one-crate-per-field fragmentation.

## 8. Ordered implementation milestones

### M0 — Reproducible cross-platform skeleton

Deliver:

- scaffold React/TypeScript/Tauri v2 under `apps/desktop`;
- pin Node LTS, pnpm, Rust and exact Tauri core/CLI/plugin versions;
- commit frontend and Cargo lockfiles;
- local bundled assets, strict CSP and only the minimal Tauri core permissions;
- one `SystemInfo` command to prove typed React↔Rust invocation;
- native macOS ARM64 and Windows 11 x64 unsigned build jobs.

Exit gate:

- `pnpm install --frozen-lockfile`, frontend build, Rust format/clippy/test and unsigned Tauri bundle pass on native hosts;
- the WebView cannot navigate to arbitrary remote content or call shell/filesystem/SQL plugins.

### M1 — Pure domain, normalization and golden launch plans

Deliver:

- types in §5;
- Rust-owned fixed field definitions;
- `normalize_and_validate` and stable `FieldIssue` codes;
- platform-neutral `LaunchPlan` plus one platform assembler per target;
- state-transition and CAS rules;
- golden fixtures for accepted/rejected snapshots and both platform launch plans.

Exit gate:

- golden assertions compare complete normalized snapshots and launch plans;
- TypeScript contains no RealBrowser argument, executable path or validation rule;
- tests use fake inventory/runtime through the same `BrowserControl` Interface.

### M2 — Durable Identity store and Profile ownership

Deliver:

- SQLite migrations for identities, field origins, revisions and runtime journal;
- canonical product data root and UUID-derived Profile roots;
- atomic create and recoverable archive;
- application Profile lease that refuses symlink/reparse/path escape and duplicate ownership;
- startup reconciliation from journal + OS/process facts; never delete Chromium lock files.

Exit gate:

- concurrent updates return `RevisionConflict`;
- partial create/restart tests leave either a complete identity or a recoverable error, never a shared Profile root;
- archive while running fails; archive preserves Profile data.

### M3 — Native RealBrowser lifecycle and K0+K1

Deliver:

- fixed product Chromium tag, RealBrowser name/icon resources and reproducible patch/build lane;
- manifest/product-id/version/major/SHA-256 verification with no system-browser fallback;
- secret-free `persona.json` generation and fail-closed `--realbrowser-persona-file` startup;
- Blink Canvas 2D copy-only seeded readback plus top/iframe/dedicated-worker observation;
- normalized launch plan execution with one full User Data root per identity;
- Windows suspended launch → Job assignment → resume;
- macOS process containment/ownership Adapter with equivalent observable lifecycle semantics;
- graceful stop, bounded forced stop and crash/exit classification;
- structured redacted runtime events.

Exit gate:

- two real RealBrowser identities run concurrently on each claimed platform;
- same-Identity restart Canvas hashes are stable, two identities differ, and iframe/worker match top;
- duplicate launch is rejected;
- killing RealBrowser, the Tauri process and the machine/session at defined points produces the expected next-start reconciliation;
- a Google Chrome or unknown process is never killed or adopted.

### M4 — Minimum operator UI

Deliver:

- Profiles page: name, Persona, Egress, RealBrowser version, runtime state and actions;
- create/rename dialog with Host-returned Field Issues;
- start/stop/archive actions with optimistic UI prohibited for authoritative state;
- identity details with generated Profile identity and redacted diagnostics;
- activation refresh so keep-alive views query Rust truth when revisited.

Exit gate:

- React tests use a fake desktop Adapter;
- packaged UI journey creates and controls two identities entirely through the `BrowserControl` seam;
- refreshing/reopening the UI cannot invent a stale Running state.

### M5 — Native acceptance and MR-0 handoff

Deliver:

- signed-off two-identity journey in §3 on macOS ARM64 and Windows 11 x64;
- final unsigned development artifacts and hashes;
- exact RealBrowser/platform versions and test-machine facts;
- crash/reconcile evidence and secret-free logs;
- known-limitations page showing Persona backend coverage and that fixed Proxy is not fail-closed or authenticated.

Exit gate:

- every §3 step has retained evidence;
- no unsupported fingerprint/proxy claim appears in UI, README or release notes;
- issues discovered in platform mechanics are routed to the owning Module, not patched in React/Tauri callers.

## 9. Test matrix

### Hermetic

- Field origin replacement/removal.
- Name and startup URL normalization.
- Profile path derivation and path-escape rejection.
- CAS conflict and allowed state transitions.
- Complete Windows/macOS LaunchPlan goldens.
- Redaction of paths, URLs and runtime errors.
- Fake-runtime create/start/stop/archive/reconcile Interface tests.

### Native integration

- RealBrowser product kernel missing, unsupported version, tampered hash and changed binary between planning and launch.
- Two distinct User Data roots and duplicate-lock rejection.
- Paths with spaces and non-ASCII characters.
- Graceful RealBrowser close, forced close, RealBrowser crash and app crash.
- Windows Job child containment; macOS process ownership/cleanup.
- SQLite power-loss/restart reconciliation.

### UI/package

- Create/rename validation and stable errors.
- Starting/Running/Stopping/Failed projections.
- Window close/reopen and keep-alive activation refresh.
- macOS ARM64 `.app` and Windows x64 installer launch on clean hosts.

### Real acceptance

- Two authorized test accounts on one selected e-commerce platform.
- Persistent session continuity after application restart.
- No state crossover proved by site login, local storage and Profile-root inspection.

## 10. Build and release gates for MR-0

MR-0 uses unsigned development artifacts only. The authoritative platform commands and later commercial signing gates are maintained in:

- [`macos-build-readiness.md`](../research/macos-build-readiness.md)
- [`windows-build-readiness.md`](../research/windows-build-readiness.md)
- [`cross-platform-build-readiness.md`](../research/cross-platform-build-readiness.md)

Windows x64 is the Tier-1 native lane. macOS ARM64 is the second native lane; Universal macOS becomes a later commercial packaging gate after the Intel target and helpers exist. Cross-compilation is never accepted as native runtime proof.

## 11. Security invariants even in MR-0

- Tauri WebView loads bundled UI only and is treated as untrusted presentation.
- No generic shell, filesystem, SQL, HTTP, arbitrary path, executable, launch argument or environment command reaches React.
- Profile roots are application-owned canonical paths and unique per Identity.
- Website state remains product-Chromium-owned opaque data; Rust does not read Cookie/Login Data.
- The ephemeral CDP endpoint is derived only from the managed Profile's `DevToolsActivePort`, is accepted only on loopback, and never crosses Tauri IPC.
- Logs exclude website content, cookies, tokens, passwords and complete URLs/query strings.
- Direct and fixed Proxy Egress are explicit; MR-0 never labels Proxy as authenticated, rotating, anonymous, isolated-network or fail-closed.
- All destructive actions are recoverable archive operations; physical Profile deletion is deferred.

## 12. Work that starts only after MR-0

MR-1 begins by resolving the existing Wayfinder decisions rather than adding features ad hoc:

1. Ticket 05: extend the current Rust-owned CDP seam with exact MV3/Native Messaging ownership and broader target coverage.
2. Ticket 07: supported Browser Persona surfaces and consistency/version contract.
3. Ticket 08: proxy credentials plus fail-closed Network Egress on Windows and the separately designed macOS contract.
4. Ticket 09: secrets, controlled Cookie portability and deletion/recovery semantics.
5. Ticket 10: extension provenance, permissions and update/rollback.
6. Ticket 14: threat model and release-blocking properties.
7. Ticket 15: signed installer/updater and multi-artifact compatibility.
8. Ticket 16: 200 stored/20 active capacity and real Platform Acceptance gates.

Only after MR-1 can the application be evaluated as a commercial Fingerprint Browser MVP.

## 13. Definition of done

MR-0 is done when the two-identity journey passes on native macOS ARM64 and Windows 11 x64 through one deep `BrowserControl` Interface, and both processes are the branded RealBrowser product Chromium. K1 additionally requires stable same-Identity restart hashes, distinct A/B hashes, matching top/iframe/dedicated-worker results, copy-only readback and native function identity. Launch-plan goldens, durable isolated Profile roots, deterministic crash reconciliation and truthful Persona plus Direct/Proxy UI labels remain required.

It is not done merely because a Tauri window opens, a Chromium source patch applies, or unit tests pass. It is also not permission to publish or market the application.
