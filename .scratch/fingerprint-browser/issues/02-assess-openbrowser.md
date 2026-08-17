# Assess OpenBrowser

Type: research
Status: resolved
Assignee: research_openbrowser

## Question

What architecture and security model does `lyu0805/OpenBrowser` ship today, and which decisions are relevant to this product? Verify from source and first-party documentation its implementation languages, Chrome or Chromium ownership, Profile isolation, Browser Persona mechanism, proxy handling, Local API and MCP exposure, credential access, persistence, update mechanism, tests, release artifacts, license, and maintenance health. Identify reusable patterns, unsafe assumptions, and claims that cannot be proven statically.

## Comments

- Planned research asset: [`../research/openbrowser.md`](../research/openbrowser.md)

## Answer

Resolved in [`OpenBrowser architecture and security assessment`](../research/openbrowser.md). At pinned commit `cb9842a` OpenBrowser offers reusable User Data root, path-validation, locking, ephemeral-CDP and stable/dynamic-persona patterns, but is not a suitable base: its Electron control plane trusts bundled third-party kernels, persists sensitive profile/proxy/platform state in plaintext JSON, can export and back up cookies, exposes broad Local API/MCP/RPA/CDP authority, permits fail-open egress policies, and downloads updates without an in-app signature/hash gate. Reimplement the isolation invariants in Rust around an attestable official Chrome/Chromium runtime; keep sessions opaque, secrets vault-backed, proxy launch fail-closed, automation disabled by default, and require Windows plus selected Seller Platform runtime validation.
