# Windows 11 Build and Distribution Readiness

Research snapshot: 2026-08-15. Scope: the locked React + TypeScript + Tauri v2 desktop stack, with an external Chrome/Chromium runtime and a Tauri-free Rust `BrowserControl` core. Sources are limited to first-party Tauri, Microsoft, Rust, and Node documentation.

## Implementation update

The planning-only repository state described by the original snapshot has been superseded. The checkout now contains the pinned React/TypeScript/Tauri workspace, lockfiles, eight Rust workspace members, SQLite storage, stock-Chrome lifecycle code, icons, a restrictive capability manifest and a working native macOS development bundle. `cargo xtask ci` passes, and the Tauri-free Core/Profile/Persona/Platform/Control crates pass the `x86_64-pc-windows-msvc` source check.

Native Windows evidence is still absent. The configured `xxww-win` SSH host timed out again on 2026-08-15. Adding the full Tauri/storage application to the macOS cross-check reaches `libsqlite3-sys` and fails because the native MSVC/Windows SDK C toolchain is unavailable; that is not treated as a Windows product failure or as Windows build evidence. The native commands and clean-host gates below remain authoritative.

## Verdict

| Claim | Status | Evidence required before changing the status |
| --- | --- | --- |
| Tauri v2 can produce a Windows 11 x64 application | **Framework-supported** | Tauri officially supports the Windows MSVC target and Windows 11 uses WebView2. |
| This workspace has a Windows development build | **Not proven natively** | Run the pinned workspace on a reachable native Windows 11/MSVC host. |
| This workspace produces an unsigned Windows installer | **Blocked on native lane** | Build and launch NSIS/MSI on a native Windows MSVC host. |
| This product can be publicly distributed on Windows | **Blocked** | Native Release build, Authenticode signing and timestamping, updater signing, Windows 11 install/upgrade/uninstall tests, helper/service design, and artifact verification must all pass. |

The stack decision remains viable and the application workspace now exists. There is still no native Windows artifact, installer, signature or Windows runtime result to audit.

## What is feasible

### Development and native compilation

Tauri requires Microsoft C++ Build Tools with the **Desktop development with C++** workload, WebView2, Rust, and Node LTS for a JavaScript frontend. Tauri recommends the MSVC Rust toolchain; Rust itself lists `x86_64-pc-windows-msvc` and `aarch64-pc-windows-msvc` as Tier 1 targets with host tools. The MSVC linker, Windows import libraries, and SDK come from Visual Studio/Build Tools. [Tauri Windows prerequisites](https://v2.tauri.app/start/prerequisites/) [Rust Windows MSVC target support](https://doc.rust-lang.org/stable/rustc/platform-support/windows-msvc.html) [Microsoft C++ Build Tools workload](https://learn.microsoft.com/en-us/visualstudio/install/workload-component-id-vs-build-tools?view=visualstudio) [Rustup MSVC prerequisites](https://rust-lang.github.io/rustup/installation/windows-msvc.html)

Commercial teams must also select a Visual Studio/Build Tools edition and license appropriate to their organization. Rustup explicitly warns that the automatically offered Visual Studio Community edition may not be appropriate for proprietary enterprise use.

Recommended target policy:

- Ship `x86_64-pc-windows-msvc` first. It covers the primary Windows 11 market and minimizes the initial native dependency matrix.
- Add `aarch64-pc-windows-msvc` only after a separate native ARM64 artifact and Windows-on-ARM runtime suite passes. Tauri supports building it when the VS C++ ARM64 tools and Rust target are installed. The generated app is native ARM64, although Tauri documents that the NSIS installer itself runs as x86 under emulation. [Tauri Windows ARM build](https://v2.tauri.app/distribute/windows-installer/#building-for-32-bit-or-arm)
- Do not add i686 to the product matrix unless a real commercial requirement appears.

### WebView2

The Tauri management UI uses WebView2; the managed seller sessions still run in external Chrome/Chromium. Windows 11 includes the Evergreen WebView2 Runtime, but Microsoft still recommends checking that the Runtime exists and deploying it when missing. Tauri installers can download the bootstrapper, embed it, include the offline installer, or package a fixed runtime. `skip` can produce an application that simply fails on a machine without the Runtime and is not acceptable for public distribution. [Microsoft WebView2 distribution](https://learn.microsoft.com/en-us/microsoft-edge/webview2/concepts/distribution) [Tauri WebView2 installer modes](https://v2.tauri.app/distribute/windows-installer/#webview2-installation-options)

Recommended release choices:

- Internet-connected consumer installer: `downloadBootstrapper` or `embedBootstrapper`, with an actual clean-machine test.
- Explicit offline/managed-enterprise installer: `offlineInstaller`, accepting the larger artifact.
- Do not use `fixedRuntime` by default; Evergreen transfers Chromium security servicing to Microsoft and avoids carrying another browser runtime in the release.
- Add WebView2 Stable and Preview compatibility smoke tests. Evergreen can move independently of the application.

### Installer formats

Tauri produces NSIS `-setup.exe` and WiX v3 `.msi` installers. MSI bundles can only be built on Windows. MSI creation also requires the optional VBSCRIPT feature, which Microsoft is deprecating and which may be disabled on future Windows installations. Tauri can cross-build an NSIS package from macOS/Linux, but describes that route as less tested and a last resort; Rust likewise says non-Windows-to-MSVC cross-compilation may be possible but is unsupported. Therefore a native Windows release runner is a product requirement, not an optimization. [Tauri Windows installers](https://v2.tauri.app/distribute/windows-installer/) [Rust MSVC cross-compilation support](https://doc.rust-lang.org/stable/rustc/platform-support/windows-msvc.html)

Recommended packaging baseline:

- Use NSIS as the primary direct-download installer and build it on native Windows.
- Produce MSI only for enterprise customers that require it; keep it in a separate native-Windows job and test the VBSCRIPT prerequisite.
- A per-user main application should run `asInvoker`. If a machine-wide service/WFP helper is required, install only that component through a deliberate elevated installation step; do not run the React/Tauri desktop process as administrator.

## Hard blockers for this repository

1. **No reachable native Windows build/runtime evidence exists.** A native Windows MSVC runner must build and test every shipping Windows artifact. A macOS cross result is only an early source probe; it cannot produce MSI or exercise Windows 11, WebView2, registry, Job Objects, WFP, or signing end to end.
2. **No Authenticode signing identity is configured.** An unsigned installer can be built for local testing, but public direct download needs a trusted publisher identity, SHA-256 signing, and timestamping. A self-signed certificate is only a test mechanism.
3. **No Tauri updater key or release feed exists.** Authenticode and the Tauri updater signature solve different problems. The updater requires its own signing key and does not allow update signature verification to be disabled. Losing that private key prevents future updates to existing installations. [Tauri updater signing](https://v2.tauri.app/plugin/updater/#signing-updates)
4. **No architecture-specific external binaries exist.** Every future native host/helper/service must be built for, and named with, its target triple such as `helper-x86_64-pc-windows-msvc.exe` and later `helper-aarch64-pc-windows-msvc.exe`. Tauri requires the `-$TARGET_TRIPLE` suffix for `bundle.externalBin`. [Tauri external binaries](https://v2.tauri.app/develop/sidecar/)
5. **The privileged Windows component is undecided.** If fail-closed egress needs WFP, the project must decide whether built-in user-mode WFP filters are sufficient. A Windows service needs privileged installation through the Service Control Manager. A custom WFP callout requires a kernel driver, the WDK, and Microsoft driver-signing/certification work; that is a much larger commercial gate and should be avoided unless the user-mode API cannot enforce the contract. [Microsoft WFP architecture](https://learn.microsoft.com/en-us/windows/win32/fwp/windows-filtering-platform-architecture-overview) [Installing a Windows service](https://learn.microsoft.com/en-us/windows/win32/services/installing-a-service) [Microsoft driver-signing options](https://learn.microsoft.com/en-us/windows-hardware/drivers/dashboard/driver-signing-offerings)

## Signing and public distribution

Tauri can invoke SignTool through its Windows signing configuration or a custom signing command. Configure SHA-256 and an RFC 3161 timestamp, and verify the final files rather than trusting a successful bundler log. Sign all executable trust boundaries: the desktop EXE, Native Messaging host, sidecars/helpers/services, installer, and updater bundle as applicable. [Tauri Windows code signing](https://v2.tauri.app/distribute/sign/windows/) [Microsoft SignTool](https://learn.microsoft.com/en-us/windows/win32/seccrypto/signtool)

For direct download, a valid OV/EV or supported managed signing identity displays a verified publisher, but it does **not** guarantee that a new release avoids Microsoft Defender SmartScreen. Microsoft states that even a newly signed binary can show an unrecognized-app warning until file/publisher reputation develops, and EV certificates no longer receive automatic positive reputation. Microsoft Store distribution avoids the download warning because Store apps are signed by Microsoft, but Store suitability for an application that manages external browsers and an optional system service must be evaluated separately. [Microsoft SmartScreen reputation](https://learn.microsoft.com/en-us/windows/apps/package-and-deploy/smartscreen-reputation)

The updater additionally needs:

- `bundle.createUpdaterArtifacts: true`;
- an updater public key embedded in `tauri.conf.json`;
- the private key supplied only to protected release CI via `TAURI_SIGNING_PRIVATE_KEY` and optional password;
- HTTPS endpoints and complete `windows-x86_64`, and later `windows-aarch64`, feed entries;
- interruption, rollback, downgrade-policy, and old-client upgrade tests.

The Tauri updater's default Windows `passive` mode is appropriate for a visible commercial update. Its `quiet` mode cannot request elevation and therefore does not work for a per-machine installation unless the application is already elevated; Tauri does not recommend that mode. [Tauri updater Windows install modes](https://v2.tauri.app/plugin/updater/#installmode-on-windows)

## Native Messaging and privileged helper boundary

Microsoft's Chromium-extension documentation confirms that a Native Messaging host is a separate executable described by a JSON manifest with a `stdio` transport and an `allowed_origins` allowlist. On Windows the installer registers the manifest path under `HKCU` or `HKLM`; the documented search order includes Microsoft Edge, Chromium, and Google Chrome registry locations. A per-user `HKCU` registration can avoid elevation; a machine-wide `HKLM` registration belongs in the elevated installer. [Microsoft Edge Native Messaging](https://learn.microsoft.com/en-us/microsoft-edge/extensions/developer-guide/native-messaging)

Release gates for this host:

- exact extension IDs in `allowed_origins`; no wildcard origin;
- versioned, bounded JSON protocol over stdin/stdout;
- signed target-specific host binary;
- install/upgrade/uninstall registration tests for every supported browser channel;
- no proxy secret, raw CDP endpoint, arbitrary filesystem path, command line, or environment-variable capability exposed to the extension.

Windows Job Objects themselves are ordinary securable kernel objects and are suitable for managing a Chrome process tree as a unit. WFP is different: its Base Filtering Engine enforces access control, and application-identity rules are available at ALE layers. Keep the normal Tauri process unprivileged and place only SCM/WFP operations in a narrow signed service/helper with authenticated, ACL-restricted IPC. Microsoft recommends that most desktop applications run `asInvoker` and that privileged work be minimized. [Microsoft Job Objects](https://learn.microsoft.com/en-us/windows/win32/procthread/job-objects) [WFP application-layer enforcement](https://learn.microsoft.com/en-us/windows/win32/fwp/application-layer-enforcement--ale-) [Running with administrator privileges](https://learn.microsoft.com/en-us/windows/win32/secbp/running-with-administrator-privileges)

## Required native Windows validation

Run the following from a clean Windows 11 x64 development/CI host after scaffolding. These are gates, not evidence that currently exists.

### Toolchain inventory

```powershell
node --version
npm --version
rustc -Vv
cargo --version
rustup show
rustup target list --installed
where.exe cl
where.exe link
where.exe signtool
```

Expected minimum shape: Node LTS, a pinned stable Rust toolchain, `x86_64-pc-windows-msvc`, MSVC v143/x64 build tools, and a Windows 11 SDK. Pin Node, Rust, Tauri CLI, package-manager version, and all lockfiles in source control; do not let release CI float to arbitrary latest versions.

### Repository and compile gates

```powershell
Test-Path package.json
Test-Path src-tauri\Cargo.toml
Test-Path src-tauri\tauri.conf.json
npm ci
npm run build
cargo fmt --manifest-path src-tauri\Cargo.toml -- --check
cargo clippy --manifest-path src-tauri\Cargo.toml --target x86_64-pc-windows-msvc --all-targets --all-features -- -D warnings
cargo test --manifest-path src-tauri\Cargo.toml --target x86_64-pc-windows-msvc
npm run tauri build -- --target x86_64-pc-windows-msvc --bundles nsis,msi
```

If using another package manager, replace `npm ci` with its frozen-lockfile equivalent. MSI must remain a native-Windows-only gate. For an early macOS compile probe, Tauri documents `cargo-xwin` plus NSIS, but that output must not replace the commands above.

### External binary, signature, and installer gates

```powershell
Test-Path src-tauri\binaries\native-host-x86_64-pc-windows-msvc.exe
Test-Path src-tauri\binaries\network-helper-x86_64-pc-windows-msvc.exe

Get-ChildItem src-tauri\target\x86_64-pc-windows-msvc\release\bundle -Recurse -Include *.exe,*.msi |
  ForEach-Object { Get-AuthenticodeSignature -LiteralPath $_.FullName } |
  Format-Table Path, Status, StatusMessage, SignerCertificate

signtool verify /pa /all /v .\path\to\fingerprint-browser.exe
signtool verify /pa /all /v .\path\to\fingerprint-browser-setup.exe
```

Every shipping PE and installer must report a valid, trusted, timestamped signature. `Get-AuthenticodeSignature` is the official Windows PowerShell inspection API. [Microsoft `Get-AuthenticodeSignature`](https://learn.microsoft.com/en-us/powershell/module/microsoft.powershell.security/get-authenticodesignature)

Then test on clean Windows 11 VMs, not just the build runner:

1. standard-user online installation with WebView2 already installed;
2. missing/stale WebView2 handling and offline installation policy;
3. first launch, uninstall, reinstall, repair, and paths containing spaces/non-ASCII characters;
4. signed update from N-1, update interruption at each component boundary, restart, recovery, and rollback;
5. Browser Identity creation and concurrent launch/stop/reconcile of the target Chrome process count;
6. Native Messaging registration and extension-ID rejection tests;
7. main desktop process remains non-elevated; the helper alone receives elevation;
8. helper/service install, start, crash, update, rollback, and uninstall leave no orphan service or WFP rule;
9. `netsh wfp show state` plus independent packet capture proves the intended egress policy and removal behavior. Microsoft documents `netsh wfp` as the Windows 11 WFP troubleshooting interface. [Microsoft `netsh wfp`](https://learn.microsoft.com/en-us/windows-server/administration/windows-commands/netsh-wfp)

### ARM64 gate, when enabled

```powershell
rustup target add aarch64-pc-windows-msvc
npm run tauri build -- --target aarch64-pc-windows-msvc --bundles nsis
Test-Path src-tauri\binaries\native-host-aarch64-pc-windows-msvc.exe
Test-Path src-tauri\binaries\network-helper-aarch64-pc-windows-msvc.exe
```

Compilation on x64 Windows is not enough. Install and run the result on native Windows 11 ARM64, verify WebView2 architecture, external Chrome architecture selection, Native Messaging, Job containment, updater selection (`windows-aarch64`), signing, and any service/WFP adapter.

## Release decision

Proceed with React + TypeScript + Tauri v2. Create the project and make native Windows x64 the first authoritative Release lane. The shortest credible path is:

1. scaffold and pin a minimal React/Tauri workspace;
2. obtain a clean native Windows 11/MSVC development and CI host;
3. build unsigned x64 NSIS and MSI artifacts;
4. add the target-specific Native Messaging host and ordinary-process/Job adapter;
5. decide user-mode WFP service versus no WFP before freezing the installer topology;
6. add Authenticode and independent Tauri updater signing;
7. pass clean-machine install/update/uninstall and network-failure tests;
8. only then describe Windows as “build passing” or “publicly distributable.”

No framework-level Windows blocker was found. All current blockers are missing implementation/release evidence or unresolved privileged-component design, and none justifies changing the locked desktop stack.
