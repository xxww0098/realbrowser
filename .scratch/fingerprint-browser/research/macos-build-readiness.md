# React + TypeScript + Tauri v2: macOS build readiness

Research snapshot: 2026-08-15. Scope: the locked React/TypeScript + Tauri v2 desktop stack, with the managed browsing runtime remaining an external Chrome/Chromium process. This is a build and distribution readiness review, not runtime proof of Browser Identity isolation or fingerprint behavior.

## Verdict

**The stack can support a commercial macOS build, including Intel + Apple Silicon universal distribution. This repository cannot currently be built, and public macOS distribution is not ready.**

The distinction matters:

| Gate | Technical feasibility | Current repository evidence |
| --- | --- | --- |
| Local development on Apple Silicon | Supported | **Blocked:** no application source, package manifest, Tauri config, or Tauri CLI dependency |
| Unsigned/ad-hoc ARM64 `.app` | Supported | **Blocked:** same missing project inputs |
| Universal Intel + Apple Silicon `.app`/DMG | Supported | **Blocked:** `x86_64-apple-darwin` is not installed and no project exists |
| Public direct-download release | Supported through Developer ID + notarization | **Blocked:** no Developer ID identity, notarization credentials, entitlements/configuration, CI, or built artifact |
| Auto-update | Supported by the Tauri updater | **Unproven:** ambient updater variables exist, but there is no product-specific key/config/artifact/update manifest |

Tauri documents native macOS application bundles and DMG output, and its CLI supports the virtual `universal-apple-darwin` target when both Rust targets are installed. Apple defines a universal binary as containing native Intel and Apple Silicon slices. [Tauri macOS app bundle](https://v2.tauri.app/distribute/macos-application-bundle/) [Tauri CLI](https://v2.tauri.app/reference/cli/) [Apple universal binaries](https://developer.apple.com/documentation/Apple-Silicon/building-a-universal-macos-binary)

## Evidence from this checkout and machine

Read-only inspection produced the following snapshot:

- Repository contains planning Markdown only. It has no `package.json`, lockfile, `src-tauri/Cargo.toml`, `tauri.conf.json`, frontend source, or CI workflow. The directory is not currently a Git checkout.
- Host is Apple Silicon (`arm64`), macOS 26.6.1.
- Apple Command Line Tools and macOS SDK are available at `/Library/Developer/CommandLineTools`; Apple clang 21 is available. Full Xcode is not selected, so `xcodebuild -version` fails.
- Rust `1.97.1`/Cargo `1.97.1` are installed for `aarch64-apple-darwin`. The Intel target is not installed.
- Node `22.23.2`, npm `10.9.8`, and pnpm `11.20.0` are installed. Node 22 and 24 are currently LTS; a new commercial project should pin one supported LTS line rather than depend on the developer machine's ambient version. [Node release policy](https://nodejs.org/en/about/previous-releases)
- `cargo tauri` is not installed, and `pnpm exec tauri` has no workspace package to resolve. Prefer a project-pinned `@tauri-apps/cli` over a globally installed CLI.
- `codesign`, `notarytool`, and `stapler` are present, but `security find-identity -v -p codesigning` reports **0 valid identities**. Apple signing/notarization environment variables are unset.
- Tauri updater signing variables happen to exist in the ambient shell. Their ownership and compatibility with this new product are unknown; they are **not evidence** that this product has an update signing key.
- Installed Google Chrome is a signed, notarized universal app (`x86_64 arm64`). This proves only that the local machine has a usable external browser candidate; the application does not yet discover, validate, launch, or monitor it.

Tauri requires Xcode or, for desktop-only development, the Command Line Tools; it also recommends a Node LTS line when a JavaScript frontend is used. Therefore the current CLT installation is enough in principle for a desktop compile, while a pinned full Xcode image remains the safer release-runner baseline. [Tauri prerequisites](https://v2.tauri.app/start/prerequisites/)

## Recommended macOS distribution shape

Use **direct Developer ID distribution as a signed and notarized universal DMG**, not the Mac App Store, for the first commercial release.

Reasons:

- The product must launch an external Chrome app, manage non-default User Data roots, write product-owned profile data, and may register a Native Messaging helper. The Mac App Store requires App Sandbox; direct Developer ID distribution requires Hardened Runtime for notarization but does not require App Sandbox. [Apple distribution comparison](https://developer.apple.com/macos/distribution/) [Apple distribution preparation](https://developer.apple.com/documentation/xcode/preparing-your-app-for-distribution)
- Apple documents that a sandboxed app cannot run programs outside its app bundle, sandbox container, or app-group containers merely through user-selected file access. Making external browser management fit the App Sandbox would require a separate design and is not an MVP packaging change. [Apple sandbox file access](https://developer.apple.com/documentation/security/accessing-files-from-the-macos-app-sandbox)
- Tauri's updater fits developer-managed, direct distribution. Mac App Store releases instead use Apple's update channel.

The first release should therefore use:

```text
Universal notarized Fingerprint Browser.app
  Contents/MacOS/Fingerprint Browser        arm64 + x86_64
  Contents/MacOS/native-host (if shipped)   arm64 + x86_64
  Contents/Resources/...                    static UI/resources
  minimal entitlements
  Hardened Runtime
wrapped by a signed/notarized/stapled DMG
```

Do not bundle or re-sign the user's Chrome application in the MVP. Treat Chrome as an independently installed and signed external product. The manager should resolve an absolute app executable path and never assume the GUI app inherits the user's shell `$PATH`; Tauri explicitly warns that macOS GUI apps do not inherit shell-dotfile paths. [Tauri DMG guide](https://v2.tauri.app/distribute/dmg/)

## Architecture and universal-binary requirements

Rust supports both macOS targets; `aarch64-apple-darwin` is Tier 1 and `x86_64-apple-darwin` is Tier 2 with host tools. Rust respects `MACOSX_DEPLOYMENT_TARGET`, so the Cargo build target and Tauri `minimumSystemVersion` must be intentionally aligned. [Rust macOS targets](https://doc.rust-lang.org/rustc/platform-support/apple-darwin.html)

For a universal release:

1. Install both Rust targets.
2. Build with Tauri's virtual `universal-apple-darwin` target.
3. Verify **every compiled item**, not just the top-level Tauri executable. Apple explicitly includes apps, plug-ins, frameworks, libraries, command-line tools, daemons, and agents in the universal requirement.
4. Test both slices. Apple notes that an Apple Silicon machine can run the Intel slice under Rosetta, but that does not replace a real supported-Intel smoke test if Intel is a customer promise.

Tauri's `externalBin` mechanism expects target-suffixed binaries such as `native-host-aarch64-apple-darwin`. It does not make an architecture-incompatible helper acceptable; a universal release must supply and verify a universal helper or have a deliberately tested per-architecture packaging scheme. [Tauri external binaries](https://v2.tauri.app/develop/sidecar/)

The same rule applies to a Native Messaging host. If Chrome launches a separate compiled host, that host is a shipping executable: it must run on both promised architectures and must be included in the code-signing/notarization verification. Registration, stable absolute path behavior, app relocation, update replacement, and a running-host upgrade are separate integration tests; Tauri's bundler does not prove those Chrome-specific behaviors.

## Signing, entitlements, and notarization

There are three materially different outputs:

### Development build

`tauri dev` and debug artifacts can run locally without a commercial identity. They prove frontend/Rust compilation and local launch only.

### Unsigned or ad-hoc bundle

Tauri supports ad-hoc identity `-`. It can help test ARM bundles, but Tauri warns that users may still need to whitelist the application in Privacy & Security. It is not a public-release substitute. [Tauri macOS signing](https://v2.tauri.app/distribute/sign/macos/)

### Public direct-download bundle

The release needs all of the following:

- paid Apple Developer Program membership and a **Developer ID Application** identity;
- Hardened Runtime enabled;
- secure timestamp;
- valid signatures for the app and all nested executable code, including sidecars/helpers;
- notarization using `notarytool` or Tauri's notarization integration;
- a stapled notarization ticket;
- Gatekeeper assessment of the final shipped artifact.

Apple's notary service rejects the wrong certificate class, missing Hardened Runtime, unsigned nested code, or absent secure timestamps. A free Apple account cannot notarize a Tauri app. [Apple Developer ID](https://developer.apple.com/support/developer-id/) [Apple notarization requirements](https://developer.apple.com/documentation/security/notarizing-macos-software-before-distribution) [Tauri signing and notarization](https://v2.tauri.app/distribute/sign/macos/)

Start with **minimal entitlements**. React/Vite static UI and a normal Rust process do not inherently require JIT, unsigned executable memory, DYLD environment variables, or disabled library validation. Add an entitlement only for a demonstrated runtime need. Tauri can merge a product `Info.plist` and apply an `Entitlements.plist`; Apple applies entitlements at signing time. [Tauri application bundle and entitlements](https://v2.tauri.app/distribute/macos-application-bundle/) [Apple Hardened Runtime](https://developer.apple.com/documentation/security/hardened-runtime)

Do not use `codesign --deep` as the signing strategy. Apple recommends signing nested components from the inside out; `--deep` is appropriate for verification and can apply the wrong entitlement set during signing. [Apple distribution-signed code](https://developer.apple.com/documentation/xcode/creating-distribution-signed-code-for-the-mac/)

## Updater readiness

Tauri's updater supports macOS and refuses to install unsigned updater artifacts; signature verification cannot be disabled. Apple code signing and Tauri update signing are two separate trust layers and require separate key custody. On macOS, Tauri creates `.app.tar.gz` plus `.sig` updater artifacts. [Tauri updater](https://v2.tauri.app/plugin/updater/)

Required product decisions before enabling it:

- generate a new product-specific Tauri signing key; never silently reuse an ambient key;
- store the private key in release-secret custody and embed only its public key;
- define a universal update target consistently (for example a single `macos-universal` feed key);
- version the Tauri app, Rust schema, native host, extension, and any sidecar as one compatible release set;
- test interrupted download, interrupted replacement, running Chrome/native-host processes, rollback policy, and profile-data migration independently of updater signature verification.

Loss of the Tauri updater private key prevents publishing updates accepted by existing installations, according to Tauri's documentation. Key backup and rotation design are therefore a commercial release gate.

## CI and runtime-test boundary

The macOS release job must run on macOS. Tauri says macOS application bundles are built on a Mac, and its signing guide imports the `.p12` certificate into a temporary CI keychain before invoking the Tauri build. Signing also requires an Apple device under Tauri's documented constraints. [Tauri application bundle](https://v2.tauri.app/distribute/macos-application-bundle/) [Tauri CI signing example](https://v2.tauri.app/distribute/sign/macos/)

A CI compile is not enough. Separate evidence is required for:

- minimum supported macOS and current macOS, because Tauri uses the system WKWebView and WebKit updates follow OS updates; [Tauri WebView versions](https://v2.tauri.app/reference/webview-versions/)
- Apple Silicon native launch;
- Intel native launch if Intel is promised, plus an Intel-slice Rosetta smoke test on Apple Silicon;
- discovery and launch of supported Chrome/Chromium channels;
- profile paths containing spaces and non-ASCII characters;
- app moved outside `/Applications` and then restored;
- Native Messaging host registration, extension-origin restriction, host crash, and app/native-host version mismatch;
- notarized artifact downloaded through the real release channel on a clean Mac so quarantine/Gatekeeper behavior is exercised.

Headless CI cannot by itself certify Chrome windows, macOS privacy prompts, Finder DMG installation, or Native Messaging lifecycle. Those need a signed-release acceptance lane on real macOS hosts.

## Hard blockers

Current hard blockers, in dependency order:

1. **No project exists:** scaffold and commit the React/TypeScript/Tauri v2 workspace, lockfile, Rust toolchain, and Tauri configuration.
2. **No reproducible toolchain contract:** pin Node LTS, pnpm, Rust, both macOS Rust targets, and the release Xcode image.
3. **No Intel target:** install `x86_64-apple-darwin`; then prove a universal build and inspect all nested Mach-O slices.
4. **No Apple identity:** provision Developer ID Application signing material and notarization API credentials in isolated release-secret custody.
5. **No entitlements/bundle contract:** choose bundle identifier, minimum macOS, direct-DMG channel, Hardened Runtime, and minimal entitlements.
6. **No helper packaging contract:** decide whether Native Messaging uses a bundled universal helper, and define its stable path, signature, version negotiation, registration, and removal.
7. **No release CI:** build, sign, notarize, staple, verify, and publish on macOS with fail-closed secret checks.
8. **No runtime acceptance evidence:** test the final downloaded artifact on Apple Silicon and Intel/customer-supported OS versions.

## Verification commands after scaffolding

These commands are gates, not evidence today. Replace `$APP_NAME` with the final bundle name and run them on a clean macOS release runner.

### Toolchain and source gates

```bash
node --version
pnpm --version
rustc -Vv
cargo -V
xcodebuild -version
xcrun --sdk macosx --show-sdk-path
rustup target add aarch64-apple-darwin x86_64-apple-darwin
rustup target list --installed
pnpm install --frozen-lockfile
pnpm exec tauri info
pnpm run build
cargo check --workspace --all-targets
cargo test --workspace
```

### Native and universal build gates

```bash
pnpm exec tauri build --no-sign --bundles app
pnpm exec tauri build --no-sign --target universal-apple-darwin --bundles app

APP_PATH="src-tauri/target/universal-apple-darwin/release/bundle/macos/$APP_NAME.app"
file "$APP_PATH/Contents/MacOS/$APP_NAME"
lipo -archs "$APP_PATH/Contents/MacOS/$APP_NAME"
find "$APP_PATH" -type f -perm -111 -exec file {} \;
```

The top-level output must report both `arm64` and `x86_64`. Every bundled Mach-O sidecar/helper/library must be inspected as well.

### Signed public-release gates

```bash
security find-identity -v -p codesigning
pnpm exec tauri build --target universal-apple-darwin --bundles app,dmg

APP_PATH="src-tauri/target/universal-apple-darwin/release/bundle/macos/$APP_NAME.app"
DMG_PATH="src-tauri/target/universal-apple-darwin/release/bundle/dmg/$APP_NAME.dmg"

codesign --verify -vvv --deep --strict "$APP_PATH"
codesign -dvv "$APP_PATH" 2>&1
spctl -vvv --assess --type exec "$APP_PATH"
xcrun stapler validate "$APP_PATH"
hdiutil verify "$DMG_PATH"
xcrun stapler validate "$DMG_PATH"
```

Apple documents `codesign -vvv --deep --strict` for recursive notarization-style verification and `spctl --assess --type exec` for Gatekeeper policy assessment. These must succeed on the final artifact, not merely the pre-bundle executable. [Apple notarization diagnostics](https://developer.apple.com/documentation/security/resolving-common-notarization-issues)

## Decision

**Keep React + TypeScript + Tauri v2 locked.** No macOS framework-level blocker has been found. The current state is nevertheless **planning-ready, not build-ready**. The first honest macOS milestone is a committed skeleton that produces an unsigned ARM64 `.app`; the second is a universal app with verified nested slices; the commercial gate is a downloaded, Developer ID-signed, notarized, stapled universal DMG tested with external Chrome and the chosen Native Messaging helper lifecycle.
