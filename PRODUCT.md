# Product

## Platform

desktop

## Stack

Native GPUI Kit window. Rust owns the product domain, persistence, RealBrowser Chromium lifecycle, and platform integration through `browser-control`. The only browser runtime is the product-distributed Chromium kernel named **RealBrowser**.

## Users

The primary user is an authorized e-commerce operator who regularly works with multiple seller or store accounts on one macOS or Windows workstation. They need to recognize the right browser environment quickly, launch it safely, and preserve each environment's browsing state between sessions.

## Product Purpose

The product manages persistent, isolated Browser Identities for legitimate multi-account operations. Success means an operator can create, find, launch, stop, and recover multiple identities without mixing profile state or needing to manage browser data directories and command-line arguments manually.

## Positioning

The product uses a local-first Rust control plane around its own Chromium build: Rust is the authority for identity state, launch plans, storage, Persona compilation, and process ownership while the GPUI Kit shell renders projections and permitted actions. It does not discover, start, adopt, or fall back to a locally installed Google Chrome, and it does not take custody of website passwords, cookies, or 2FA secrets.

## Operating Context

- Desktop application for macOS and Windows.
- Frequent, high-density environment management from a searchable table.
- Human-operated seller workflows across general e-commerce platforms; Ozon Seller is a compatibility journey, not a product boundary.
- Each Browser Identity binds one persistent Browser Profile, one coherent Browser Persona, and one declared Network Egress.
- Every launch uses the verified product Chromium manifest and a dedicated, application-owned User Data root. Missing, mismatched, or tampered product kernels fail closed.

## Capabilities and Constraints

- MR-0 supports create, rename, list, start, stop, Persona configuration, restart reconciliation, and recoverable archive for Browser Identities.
- MR-0 supports Direct Network Egress and one fixed HTTP, HTTPS, or SOCKS5 proxy configured by host and port. Proxy launch fails loudly until the Persona WebRTC policy disables non-proxied UDP. It targets local or IP-allowlist/no-auth proxies; credentials, rotation and OS-level fail-closed enforcement remain unavailable.
- MR-0 persists Persona schema v5 with migration, a stable per-identity seed, and named surface groups. After locking the Profile, Rust writes a secret-free K0 `persona.json` and launches the product binary with `--realbrowser-persona-file`. Locale, window size, WebRTC policy, timezone and the coherent Viewport/Screen/DPR atom keep their typed Rust-owned execution paths.
- K1 changes only Canvas 2D readback copies in Blink: `getImageData`, `toDataURL`, `toBlob`, and OffscreenCanvas `convertToBlob`. Noise is seeded and idempotent per Browser Identity, is process-wide for top frames, iframes and dedicated workers, never writes back to the source canvas, and leaves native function identity intact. WebGL, Audio, TLS and JS hooks are explicitly outside K0+K1.
- Rust publishes the 28-field Persona capability catalogue consumed by the UI. `graphics.canvas` remains `Native` until the running product kernel passes the K1 observation matrix; only then is it projected as `CustomKernel`. All other unimplemented rendering/media/hardware surfaces remain honestly native or Profile-owned.
- The UI distinguishes `Native` and `Managed`, and `Direct` from `Proxy`. It does not claim proxy leak prevention, anonymity, undetectability, or fingerprint-surface effectiveness without a verified runtime.
- The shell never constructs executable paths, Chromium arguments, profile paths, Persona files, or environment variables. Rust normalizes input and assembles every launch plan.
- The desktop projection returns opaque identity IDs, stable operator codes, modes and status only; Profile roots, RealBrowser executable paths, application data roots and process IDs stay inside Rust.
- Operator errors expose stable product codes, field issues and fixed remediation text. Internal process, filesystem, Profile and storage diagnostics never become view-model text.
- Profile isolation uses a complete non-default User Data root per identity, a cross-process application lease, and platform process ownership. Restart reconciliation requires the recorded PID, the currently verified RealBrowser executable/version, and the exact User Data argument to match before adoption; a Google Chrome journal is never adopted.
- Stopping an identity terminates only its verified RealBrowser runtime. Managed launches disable Chromium background mode and use a dedicated Unix process group. The browser bundle, executable display name, visible product strings and icons are branded RealBrowser.
- Sensitive product-Chromium Profile contents remain opaque and browser-owned. Cookie import/export, password storage, 2FA custody, automation, and cloud/team synchronization are outside MR-0.
- Commercial distribution later requires signed builds, signed updates, platform-native verification of the Persona Runtime across every claimed frame/worker surface, and fail-closed Network Egress.

## Brand

- Product and browser name: **RealBrowser**. Bundles, executable presentation, and icons use this name.
- Interface language for the first build: Simplified Chinese.
- The window should feel as dense and status-forward as HubStudio, AdsPower, or Multilogin. Their branding, claims, and trade dress are not copied.
- Visual rules: [`DESIGN.md`](DESIGN.md).

## Product Principles

1. Rust owns truth; the GPUI Kit shell explains state and offers permitted actions.
2. Isolation is a product invariant, not a label: Profile, Persona, and Network Egress remain explicit and separate.
3. Default behavior is honest and recoverable: no hidden spoofing, fail-open network claim, destructive deletion, or silent state takeover.
4. Frequent actions remain fast at table density; advanced configuration reveals progressively without obscuring current state.
5. Every commercial claim must be backed by a platform-native test or remain visibly described as not yet available. Do not invent customer names, testimonials, benchmarks, certifications, or prices.

## Accessibility

Keyboard, focus, and contrast rules for the operator window are in [`DESIGN.md`](DESIGN.md).
