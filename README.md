# RealBrowser

Local-first browser identity manager built with React, TypeScript, Tauri v2, and a multi-crate Rust control plane.

## Current slice

- Isolated RealBrowser Chromium User Data root per Browser Identity.
- Create, list, rename, start, stop, and recoverable archive.
- Cross-process Profile lease with canonical-root and symlink escape checks.
- SQLite metadata with revision checks and a runtime reconciliation journal.
- Restart recovery adopts only the recorded PID whose executable, product-kernel manifest/version and User Data argument still match. Local Google Chrome is never discovered, launched, adopted or used as a fallback.
- Managed RealBrowser starts with background mode disabled. On Unix, each launch owns a process group; Stop gives the product Chromium parent a bounded graceful exit for state flush, then force-stops the remaining group if needed.
- Stable Rust-owned operator codes remain attached to an identity across sorting, rename, and archive/restore.
- Tauri sends capability/status projections to React without exposing Profile roots, browser executable paths, application data paths, or process IDs.
- Stable error codes cross Tauri IPC, while filesystem, process and storage diagnostics remain Rust-only behind fixed user remediation messages.
- Versioned Persona v5 with migration, a stable per-identity seed, and 28 Rust-owned region, device, graphics, media, and privacy capabilities. Rust writes the K0 secret-free `persona.json` and supplies only `--realbrowser-persona-file` to the product binary.
- K1 Canvas 2D readback noise lives in Blink C++, is seeded/idempotent, changes copies only, and covers `getImageData`, `toDataURL`, `toBlob`, iframe and dedicated-worker OffscreenCanvas. WebGL, Audio, TLS and JS hooks are unchanged.
- Typed product-Chromium execution plan for locale, window size, WebRTC, timezone and coherent Viewport/Screen/DPR. Every start observes the Canvas K1 matrix; `graphics.canvas` becomes `CustomKernel` only while a passing product runtime is active.
- Versioned Network Egress configuration for Direct, HTTP, HTTPS, and SOCKS5 no-auth/IP-allowlist proxies, compiled only by Rust into Chromium arguments; Proxy launch requires the Persona WebRTC non-proxied-UDP guard.

Authenticated proxies, rotation, fail-closed OS egress, cloud sync, cookie import/export, automation, and undetectability claims are not included.

## Develop

Prerequisites: pnpm, the pinned Rust toolchain from `rust-toolchain.toml`, and a packaged RealBrowser product kernel under `.dev/kernel` (or `REALBROWSER_KERNEL_DIR`). Google Chrome is neither required nor used.

```bash
pnpm install --frozen-lockfile
pnpm --filter @realbrowser/desktop dev
tools/build-realbrowser-chromium-macos.sh
./dev.sh
```

Building the macOS product kernel requires the full `/Applications/Xcode.app`; Command Line Tools alone are insufficient. The normal build lane syncs a complete Chromium checkout before applying the pinned K0+K1 patch. A previously verified official source archive can be built without dependency Git metadata only when it already contains matching macOS GN, Ninja, Clang, and Rust host tools:

```bash
REALBROWSER_CHROMIUM_SRC=/absolute/path/to/chromium/src \
REALBROWSER_CHROMIUM_OFFLINE_SOURCE_ARCHIVE=1 \
tools/build-realbrowser-chromium-macos.sh
```

Offline archive mode still requires the exact pinned Chromium commit, rejects sparse source, validates every host executable as macOS-native, and validates the installed Clang/Rust revisions before building. It is not a Stock Chrome or incomplete-source fallback.

Port `1431` is used by the frontend development server. Direct frontend-only work can use `pnpm --filter @realbrowser/desktop dev`; use `./dev.sh` for the real Tauri shell.

On macOS, `./dev.sh` preserves Vite HMR and the Tauri Rust watcher while wrapping each rebuilt executable in `.dev/macos/RealBrowser Dev.app`. The development-only bundle id is `com.realbrowser.desktop.tauri.dev`, and its local state is isolated under `.dev/data` rather than the release application data directory.

### Computer Use

Keep `./dev.sh` running in its terminal, then wait for the stable application identity:

```bash
./dev.sh status
```

Proceed only when the result contains `identity: ready`. Target `com.realbrowser.desktop.tauri.dev`; the release bundle id is never a development target. For every interaction, read the current accessibility state, perform one action, and read state again before selecting another element. Reacquire a full state or screenshot when the accessibility diff is insufficient. Restore temporary filters or test state before finishing.

Real UI proof requires an observed state transition in the Tauri window. Source inspection, a frontend build, DOM tests, and screenshots alone do not satisfy this gate.

## Verify

```bash
cargo xtask structure
cargo xtask ci
pnpm --filter @realbrowser/desktop tauri build --debug --bundles app
```

`cargo xtask ci` runs workspace structure checks, formatting, Clippy with warnings denied, Rust tests, TypeScript checks, React/Vitest interaction tests, and the production frontend build.

Cursor Cloud on Ubuntu is this hermetic lane: `pnpm install --frozen-lockfile` then `cargo xtask ci`. Native macOS UI, the product kernel, and ignored kernel-launching tests stay on a Mac; see the Cursor Cloud section in [`AGENTS.md`](AGENTS.md).

The native macOS acceptance tests are intentionally ignored by hermetic CI because they launch the packaged RealBrowser product Chromium:

```bash
cargo test -p realbrowser-desktop --test kernel_canvas -- --ignored --nocapture
cargo test -p realbrowser-desktop --test persona_runtime -- --ignored --nocapture
cargo test -p realbrowser-desktop --test profile_state_isolation -- --ignored --nocapture
cargo test -p browser-platform -- --ignored --nocapture
```

The Profile-state test serves a loopback-only test page, writes distinct persistent Cookie/localStorage account markers into two User Data roots, stops both RealBrowser instances, restarts them, and proves that A and B retain only their own state. It does not read the product Chromium's on-disk Cookie/Login Data databases.

The current macOS development bundle is produced at `target/debug/bundle/macos/RealBrowser.app`. The Network, Profile, Persona, Persona Runtime, Platform, Control, and Core crates pass `cargo check --target x86_64-pc-windows-msvc`; this is a source-level portability check, not a substitute for the required native Windows installer and runtime lane. A complete desktop cross-check from macOS currently stops in bundled SQLite's C build because the native MSVC/Windows SDK toolchain is absent.

## Rust boundaries

| Crate | Owns |
| --- | --- |
| `browser-persona` | Persona schema, K0 file contract, validation and capability gate |
| `browser-persona-runtime` | Loopback CDP attachment, managed-field application, observation and new-target replay |
| `browser-network` | Network Egress schema, proxy validation and Chromium argument compilation |
| `browser-profile` | Canonical Profile roots and exclusive application leases |
| `browser-core` | Identity records and ports |
| `browser-control` | Use cases, revision rules, launch-plan assembly |
| `browser-storage` | SQLite adapter and migrations |
| `browser-platform` | Product-kernel manifest verification and process lifecycle |
| `realbrowser-desktop` | Thin Tauri commands and composition |

Engineering rules are maintained in [`docs/rust-engineering.md`](docs/rust-engineering.md). Product truth lives in [`PRODUCT.md`](PRODUCT.md); domain language lives in [`CONTEXT.md`](CONTEXT.md).
