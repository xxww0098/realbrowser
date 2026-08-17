# React/TypeScript + Tauri v2 cross-platform build readiness

Snapshot: 2026-08-15. This review locks the desktop stack and distinguishes framework support, repository build evidence, and public-distribution evidence.

## Implementation update

The original planning-only snapshot is superseded. The pinned React/TypeScript/Tauri workspace and eight Rust members now exist; `cargo xtask ci`, the native macOS debug `.app`, stable Computer Use dev shell and Tauri-free Windows MSVC source checks pass. Native Windows remains unproven because `xxww-win` is unreachable, and a full desktop macOS-to-MSVC check stops in bundled SQLite's C build without the Windows SDK. The release/signing conclusions below are unchanged.

## Outcome

| Target | Framework support | Current checkout | Recommended commercial lane |
| --- | --- | --- | --- |
| macOS Apple Silicon development | Supported | **Native dev passing** | Native macOS runner; unsigned ARM64 app passes |
| macOS Intel + Apple Silicon | Supported through `universal-apple-darwin` | **Blocked**: no project and Intel Rust target absent | Universal Developer ID-signed, notarized, stapled DMG |
| Windows 11 x64 | Supported through MSVC + WebView2 | **Workspace exists; native lane unavailable** | Native Windows x64 Release, NSIS primary; MSI enterprise lane |
| Windows 11 ARM64 | Supported | **Deferred** | Separate native artifact, helper binaries, feed and real ARM64 acceptance |

No framework-level blocker was found. macOS development now passes; Windows and public distribution do not. Signing configuration, updater keys, native Windows artifacts and clean-host release evidence are still absent.

## Locked build policy

- Desktop stack: React + TypeScript + Tauri v2.
- Core boundary: `browser-domain` and `browser-control` are Rust crates with no `tauri::*` dependency.
- Tier 1 release target: Windows 11 x64, built and tested on native Windows/MSVC.
- macOS distribution target: Universal `arm64 + x86_64`, direct Developer ID distribution through a signed/notarized DMG; Mac App Store sandbox is outside the first release.
- Windows ARM64 is added only after a separate build/runtime acceptance lane passes.
- Cross-compilation can be an early compile probe, never the release authority. Tauri states that MSI can only be produced on Windows and describes macOS/Linux-to-Windows NSIS cross-builds as less tested and last-resort.[Tauri Windows installer](https://v2.tauri.app/distribute/windows-installer/)
- External Chrome/Chromium remains independently installed and updated. It is not bundled into or re-signed with the Tauri application.

## Current machine evidence

The current macOS host has:

- macOS 26.6.1 on Apple Silicon;
- Node 22.23.2, npm 10.9.8 and pnpm 11.20.0;
- Rust/Cargo 1.97.1 for `aarch64-apple-darwin`;
- Apple Clang 21, Command Line Tools, macOS SDK, `codesign`, `notarytool` and `stapler`.

It does not currently have:

- a Tauri project or project-pinned `@tauri-apps/cli`;
- `x86_64-apple-darwin` for a universal build;
- a valid macOS code-signing identity (`security find-identity` returned zero);
- a selected full Xcode installation; CLT is sufficient in principle for desktop development, but a pinned native Xcode runner is the safer release baseline.[Tauri prerequisites](https://v2.tauri.app/start/prerequisites/)

The configured `xxww-win` SSH build host timed out on 2026-08-15, so no Windows Node/Rust/MSVC/WebView2/signing inventory or build result was available. A native Windows runner remains a hard evidence gap.

## macOS build boundary

Tauri can produce macOS app bundles and DMGs and supports a universal target. Apple requires every compiled item—not only the top-level app—to contain the promised architectures, including helpers, daemons and command-line tools.[Apple universal binaries](https://developer.apple.com/documentation/Apple-Silicon/building-a-universal-macos-binary)

The direct-download commercial artifact must include:

- a universal top-level Tauri executable;
- universal Native Messaging host and any sidecar/helper;
- Hardened Runtime with minimal entitlements;
- Developer ID Application signing of nested code from the inside out;
- notarization and a stapled ticket;
- final `codesign`, `spctl`, `stapler`, `hdiutil` and clean-download Gatekeeper validation.

A free Apple account or ad-hoc signature is development evidence only. Tauri documents that public direct distribution needs an appropriate Developer ID identity and notarization credentials.[Tauri macOS signing](https://v2.tauri.app/distribute/sign/macos/) [Apple notarization](https://developer.apple.com/documentation/security/notarizing-macos-software-before-distribution)

Full product parity on macOS is a separate gate. The Windows design currently names Job Objects, WFP and DPAPI; macOS needs explicit adapters for process containment/recovery, Keychain, Chrome/Native Messaging paths, and fail-closed Network Egress. A macOS bundle compiling does not prove those properties.

## Windows build boundary

Tauri requires Microsoft C++ Build Tools with Desktop development with C++, WebView2, an MSVC Rust toolchain and Node for a React frontend. Windows 11 supplies WebView2, but the installer still needs a defined missing/offline-runtime policy.[Tauri Windows prerequisites](https://v2.tauri.app/start/prerequisites/) [Microsoft WebView2 distribution](https://learn.microsoft.com/en-us/microsoft-edge/webview2/concepts/distribution)

The first native lane should produce:

- `x86_64-pc-windows-msvc` application and tests;
- NSIS direct-download installer;
- optional MSI only in a native Windows enterprise job;
- target-suffixed Native Messaging host and network helper;
- Authenticode-signed/timestamped executable boundaries and installer;
- separately signed Tauri updater artifacts and a protected product-specific updater key;
- clean Windows 11 install/update/rollback/uninstall evidence.

The normal Tauri desktop process remains `asInvoker`. If WFP enforcement requires privilege, install a narrow signed service/helper through an explicit elevated step. Do not run the React/Tauri process as administrator. Avoid a kernel callout driver unless user-mode WFP cannot meet a packet-capture-backed contract.

## Shared release topology

```text
apps/desktop-tauri
  -> bundled local React/TypeScript assets
  -> thin typed commands/events

crates/browser-control (Tauri-free)
  -> platform process adapter
  -> platform secret adapter
  -> platform Chrome/native-messaging adapter
  -> platform egress adapter
  -> SQLite/recovery journal

release set
  -> desktop app
  -> native host/helper for target triple
  -> first-party extension
  -> schema/migrations
  -> OS code signature
  -> independent Tauri updater signature
```

Tauri requires bundled external binaries to exist under target-triple-suffixed names. Every target therefore needs its own helper artifacts and verification; a single helper copied across architectures is not acceptable.[Tauri external binaries](https://v2.tauri.app/develop/sidecar/)

## First evidence milestones

1. Commit a minimal pinned React/TypeScript/Tauri v2 workspace and lockfiles.
2. On the current Mac, pass frontend build, Rust tests, Tauri dev and unsigned ARM64 `.app`.
3. Install the Intel Rust target and pass a universal app build; inspect all nested Mach-O slices.
4. Provision a native Windows 11/MSVC runner and pass frontend, Rust tests, x64 app, NSIS and MSI builds.
5. Add target-specific Native Messaging hosts; validate registration, origin allowlist, upgrade and removal.
6. Add macOS Developer ID/notarization and Windows Authenticode; independently add product-specific Tauri update signing.
7. Test downloaded signed artifacts on clean native machines, including update interruption and rollback.
8. Only after platform BrowserControl adapters and runtime acceptance pass may the product claim functional support rather than build support.

Detailed platform commands and sources:

- [`macos-build-readiness.md`](./macos-build-readiness.md)
- [`windows-build-readiness.md`](./windows-build-readiness.md)

## Verdict

Keep the stack locked. The correct present status is **cross-platform framework feasible, repository not yet buildable, public distribution not ready**. The next implementation milestone is a minimal pinned skeleton and native unsigned builds; signing, updater, helper/service and real Chrome acceptance follow as separate gates.
