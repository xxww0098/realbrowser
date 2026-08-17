# Kernel Persona K0+K1

Type: implementation  
Status: open  
Depends on: 05, 07

## Goal

Make **self-owned Chromium** the **only** browser runtime. Remove discovery, launch, and reconciliation of the user’s installed Google Chrome.

Rust writes a secret-free `persona.json` and starts the product Chromium with `--realbrowser-persona-file=<json>`. Blink applies **seeded, idempotent Canvas 2D readback noise** in C++ (including OffscreenCanvas) so two Identities on one machine get **stable, different** canvas hashes. JS prototypes stay native.

## Read first

- [`docs/kernel-level-persona.md`](../../../docs/kernel-level-persona.md) — K0/K1 architecture
- [`docs/fingerprint-browser-principles.md`](../../../docs/fingerprint-browser-principles.md) — seed / coverage / honesty
- [`CONTEXT.md`](../../../CONTEXT.md) — Persona vs Profile vs Egress
- [`PRODUCT.md`](../../../PRODUCT.md) — no undetectability claims
- `BrowserControl` launch plan + 27-field capability catalogue (replace Stock Chrome seams; do not keep them as fallback)

## In scope

1. **Runtime cutover:** one pinned Chromium tag, product-owned binary. Delete Stock Chrome executable discovery, `--` launch of `Google Chrome`, and any “use installed browser” path. Restart reconciliation only adopts processes of **this** binary + the Identity’s User Data root.
2. **K0:** patch reads `persona.json`; schema / `engine_major` mismatch **refuses to start**.
3. **K1:** copy-then-perturb on `getImageData` / `toDataURL` / `toBlob` / `OffscreenCanvas.convertToBlob`. Noise = `BLAKE3(seed, schema, persona_id, "graphics.canvas", "readback")`. Same input → same pixels across restarts. Do **not** write noise back onto the page canvas.
4. Rust: single backend. Write `persona.json`, assemble launch plan. `graphics.canvas` → `CustomKernel` **only after** observation. All other graphics / media / TLS fields stay Native.
5. Tests: two Identities, two seeds → two stable canvas hashes; iframe + dedicated worker match top frame; `toString` still `[native code]`.

## Out of scope

WebGL, Audio, fonts, ClientRects, UA spoof, TLS/JA3, Firefox, mobile UA, JS prototype hooks billed as kernel, capability labels that say “applied” without observation, keeping a Stock Chrome fallback.

## Done when

Native macOS or Windows: the app starts two Identities on the product Chromium only (no Google Chrome process). Observation page records canvas hashes. Restart: hashes unchanged per Identity, different across Identities, workers agree. Starting with no product Chromium binary fails closed. Evidence is a real process + observation log.
