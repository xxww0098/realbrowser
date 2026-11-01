# RealBrowser

Local-first browser identity manager. The operator window is a native GPUI Kit shell. Identity, persona, network, and process ownership stay in Rust.

What the product does, and what it will not claim, is [`PRODUCT.md`](PRODUCT.md). Domain words are [`CONTEXT.md`](CONTEXT.md).

Authenticated proxies, rotation, fail-closed OS egress, cloud sync, cookie import/export, automation, and undetectability claims are not included.

## Develop

Prerequisites: the pinned Rust toolchain from `rust-toolchain.toml`, Linux GUI libraries (`libfontconfig-dev`, `libwayland-dev`, `libxkbcommon-dev`, `libxkbcommon-x11-dev`, `libvulkan1`, `libssl-dev`, `libasound2-dev`), and a packaged RealBrowser product kernel under `.dev/kernel` (or `REALBROWSER_KERNEL_DIR`). Google Chrome is neither required nor used.

```bash
cargo run -p realbrowser-desktop --features shell
./dev.sh
```

`./dev.sh` runs the GPUI shell and keeps local state under `.dev/data` via `REALBROWSER_DEV_DATA_ROOT`.

Building the macOS product kernel requires the full `/Applications/Xcode.app`; Command Line Tools alone are insufficient. The normal build lane syncs a complete Chromium checkout before applying the pinned K0+K1 patch. A previously verified official source archive can be built without dependency Git metadata only when it already contains matching macOS GN, Ninja, Clang, and Rust host tools:

```bash
REALBROWSER_CHROMIUM_SRC=/absolute/path/to/chromium/src \
REALBROWSER_CHROMIUM_OFFLINE_SOURCE_ARCHIVE=1 \
tools/build-realbrowser-chromium-macos.sh
```

Offline archive mode still requires the exact pinned Chromium commit, rejects sparse source, validates every host executable as macOS-native, and validates the installed Clang/Rust revisions before building. It is not a Stock Chrome or incomplete-source fallback.

## Verify

```bash
cargo run -p xtask -- structure
cargo run -p xtask -- ci
cargo build -p realbrowser-desktop --features shell
```

`cargo run -p xtask -- ci` runs workspace structure checks, formatting, Clippy with warnings denied, and Rust tests. The GPUI shell is feature-gated (`shell`) so hermetic tests do not need a display server. Linking the window requires the GUI libraries above.

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
| `realbrowser-desktop` | GPUI Kit window and in-process composition. Projections only; no profile paths or PIDs |

Engineering rules are maintained in [`docs/rust-engineering.md`](docs/rust-engineering.md). Product truth lives in [`PRODUCT.md`](PRODUCT.md); domain language lives in [`CONTEXT.md`](CONTEXT.md).
