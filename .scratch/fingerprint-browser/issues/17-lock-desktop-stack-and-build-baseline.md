# Lock the Desktop Stack and Cross-Platform Build Baseline

Type: decision
Status: resolved
Resolved: 2026-08-15

## Question

Which desktop application stack is the commercial baseline, and what can currently be claimed about macOS and Windows builds?

## Answer

The desktop stack is locked to **React + TypeScript + Tauri v2**, with a deep, Tauri-free Rust `BrowserControl` Module.

This locks the architecture family, not floating dependency ranges. The scaffold must pin the selected Tauri core/CLI/plugins, React, TypeScript, Node, pnpm, and Rust versions in manifests, lockfiles, and `rust-toolchain.toml`; those exact dependency versions become authoritative only when the application project exists and its native build matrix passes.

- React/TypeScript owns presentation and permitted user intentions only.
- Tauri owns the replaceable desktop adapter: application windows, tray, typed IPC, bundled assets, packaging, signing hooks, and updater integration.
- Rust owns Browser Identity lifecycle, profile/process/network/storage/secret invariants, Chrome integration, recovery, and the capability-limited Local API.
- `browser-domain` and `browser-control` must not depend on `tauri::*`, WebView labels, frontend event names, or plugin DTOs.
- Seller Platform pages remain in external stock Chrome/Chromium, never in the privileged Tauri WebView.
- A privileged Windows network helper/service, if WFP enforcement proves necessary, is a separately signed Adapter; the desktop process remains a standard-user process.

Electron and Wails are no longer parallel implementation targets. Electron + Rust sidecar remains only a documented contingency if a same-function signed prototype later proves that Tauri itself, rather than Windows/Chrome mechanics, misses a release gate.

## Build evidence at decision time

The stack is **architecturally buildable** on macOS and Windows, but this workspace is **not yet a buildable application**: it contains planning/research Markdown only and has no `package.json`, lockfile, `Cargo.toml`, `src-tauri`, frontend source, Tauri configuration, icons, entitlements, installer configuration, signing identity, updater key, or CI workflow.

Therefore the valid claim on 2026-08-15 is:

- macOS framework feasibility: supported; repository build: not attempted because no project exists; public distribution: blocked on project creation, signing identity, universal-target setup, and notarization proof.
- Windows framework feasibility: supported; repository build: not attempted because no project exists; public distribution: blocked on project creation, a native Windows MSVC build/signing host, installer/helper design, and signed update proof.

Static platform research lives in:

- [`../research/cross-platform-build-readiness.md`](../research/cross-platform-build-readiness.md)
- [`../research/macos-build-readiness.md`](../research/macos-build-readiness.md)
- [`../research/windows-build-readiness.md`](../research/windows-build-readiness.md)
- [`../research/commercial-desktop-stack-decision.md`](../research/commercial-desktop-stack-decision.md)

No platform may be called “build passing” until its native Release artifact is built and the artifact verification commands in those reports pass.

The release matrix is Windows 11 x64 first, macOS Universal through direct Developer ID distribution second, and Windows ARM64 only after an independent native acceptance lane. A compiling macOS shell does not imply parity with Windows Job/WFP/DPAPI behavior; macOS process, Keychain, Native Messaging, and fail-closed egress adapters require their own decisions and runtime proof.

## Reopen conditions

Reopen this decision only if a native prototype shows a Tauri-specific blocker in owned local UI, packaging, accessibility, updater, or crash recovery that cannot be fixed without violating the deep `BrowserControl` boundary. Failures in Chrome, Job Objects, WFP, proxy enforcement, Profile handling, or browser compatibility do not justify switching the desktop shell.
