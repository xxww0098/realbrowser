# Wayfinder Map: General-Purpose Fingerprint Browser

Label: wayfinder:map

## Destination

Produce a decision-complete, implementation-ready product and architecture specification for a Windows 11, local-first, general-purpose Fingerprint Browser suited to authorized e-commerce operations. Its React/TypeScript UI and Tauri v2 desktop adapter call a Tauri-free Rust control plane that manages stock Chrome or Chromium Browser Identities and the standard local feature set expected of this product category; the route is clear when an implementation team can build and verify 200 stored identities and 20 concurrent windows without making unresolved product, trust, engine, data, network, automation, security, compatibility, or release decisions.

## Notes

- This map plans the product; implementation is a separate effort.
- Use the ubiquitous language in [`CONTEXT.md`](../../CONTEXT.md). Invoke `grilling` and `domain-modeling` for human decisions, `research` for primary-source investigations, and `prototype` where behavior needs a concrete artifact.
- The standard local feature baseline includes Browser Identity lifecycle, groups/tags/search, Identity Templates, Persona configuration, proxy library and binding, controlled Cookie portability, extension management, startup pages/bookmarks, batch operations, window arrangement/synchronization, and a capability-scoped Local API/automation surface.
- The MVP product boundary is a Rust control plane around stock Chrome or Chromium. A new Rust browser engine and a maintained Chromium fork remain outside this MVP; unsupported fingerprint surfaces must be disclosed rather than simulated inconsistently.
- Local-first and single-user describe the first deliverable, not the entire product category. Team roles, encrypted cloud synchronization, and hosted automation are later product tiers whose contracts depend on the local data and security decisions.
- Seller Platform behavior must remain outside the core identity, process, Persona, network, and storage models. Compatibility is proven through Platform Acceptance Journeys.
- The default acceptance scenario is two authorized Browser Identities logged into different Store Accounts on a selected Seller Platform at the same time, visiting catalog and order pages, surviving restart without state crossover, and retaining their declared Network Egress. Ozon Seller is an optional reference platform, not the product boundary.
- Research assets live under [`research/`](./research/) because this directory is not currently a Git repository and cannot provide throwaway research branches.
- Security and policy boundary: isolation, stability, compatibility, and operator control are goals; bypassing platform controls is not.
- The commercial desktop stack is locked to React + TypeScript + Tauri v2 around a deep, Tauri-free Rust `BrowserControl` Module. Electron and Wails remain research comparisons, not parallel implementation targets.

## Decisions so far

- [Assess Juu17 Browser Fingerprint Shuffler](./issues/01-assess-juu17-fingerprint-shuffler.md) — Direct reuse is rejected: most hooks cannot affect page-world JavaScript and the noise model is stateful and inconsistent; retain only independently reimplemented, versioned seed-derivation concepts.
- [Assess OpenBrowser](./issues/02-assess-openbrowser.md) — Treat it as a pattern catalogue, not a base: reimplement its useful path/lock/ephemeral-CDP ideas while rejecting its opaque kernel, plaintext secrets, Cookie backup, broad automation, fail-open proxy, and updater trust.
- [Assess VirtualBrowser and XChrome](./issues/03-assess-virtualbrowser-and-xchrome.md) — Neither project is an acceptable code or binary trust base; independently implement the sound local Chrome + per-Profile supervisor shape with fail-closed egress, OS-backed secrets, recoverable lifecycle, and a capability-limited API.
- [Establish Chrome Control Constraints](./issues/04-establish-chrome-control-constraints.md) — Use one non-default User Data directory and Windows Job per identity, minimal MV3 plus Native Messaging, and a small version-probed CDP subset; enforce TCP/UDP/DNS egress below Chrome because browser proxy settings cannot prove fail-closed behavior.
- [Lock the Desktop Stack and Cross-Platform Build Baseline](./issues/17-lock-desktop-stack-and-build-baseline.md) — Use React/TypeScript for presentation, Tauri v2 as a thin replaceable desktop adapter, and a Tauri-free Rust `BrowserControl` core; framework feasibility exists on macOS and Windows, but neither repository build is proven until a real project and native signed Release artifacts exist.

## Not yet specified

- Concrete Rust crate contents, process topology, and UI-to-`BrowserControl` protocols remain fog until the browser integration, Browser Persona, and data-lifecycle decisions are resolved; the desktop framework and language boundary are no longer fog.
- Detailed operator screens and recovery interactions remain fog until Browser Identity lifecycle, the Seller Platform acceptance matrix, Network Egress behavior, and storage semantics are resolved.
- Exact installer contents, browser/app update choreography, and the executable verification matrix remain fog until trust ownership and security invariants are resolved.
- The boundary between the local product, team/cloud collaboration, and hosted automation tiers remains fog until Cookie portability, secret ownership, API authority, and audit semantics are resolved.

## Implementation route (not started)

- [Minimum Runnable Implementation Plan](./implementation/minimum-runnable-plan.md) — MR-0 is a deliberately truthful engineering slice: React/TypeScript/Tauri v2, a deep Tauri-free Rust `BrowserControl`, two persistent isolated stock-Chrome identities, Native Persona, Direct Network Egress, real crash reconciliation, and native macOS ARM64/Windows 11 x64 evidence. It is not yet the commercial Fingerprint Browser MVP.
- [Build the Minimum Runnable Vertical Slice](./issues/18-build-minimum-runnable-vertical-slice.md) — open implementation ticket for MR-0; Persona, proxy, extension, Cookie, API, capacity and signed-release work remain with tickets 05–16.

## Out of scope

- Buyer-account automation, bulk account creation, account trading, ban evasion, CAPTCHA bypass, platform-control evasion, credential harvesting, and claims of being undetectable.
- Covert credential collection, an application-owned website password or 2FA vault, and any session transfer without explicit operator authorization.
- Blind duplication of a live Profile directory or session. Identity Templates are in scope; controlled Cookie portability and encrypted backup semantics must be decided explicitly.
- Uncontrolled public-proxy collection or silent fallback from a configured proxy to a direct connection. Explicit provider-backed rotation may only exist under the Network Egress contract.
- Unqualified claims of compatibility with every Seller Platform; each supported platform requires its own Platform Acceptance Journeys and evidence.
- Platform-specific logic inside the core Browser Identity, Persona, network, or storage model.
- A new Rust browser engine, a maintained Chromium fork, cloud-phone infrastructure, and public or commercial distribution during the MVP.
