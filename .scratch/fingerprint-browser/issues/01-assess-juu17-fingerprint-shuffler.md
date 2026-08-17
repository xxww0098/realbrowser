# Assess Juu17 Browser Fingerprint Shuffler

Type: research
Status: resolved

## Question

What does `juu17/browser-fingerprint-shuffler` actually implement, under what license and trust boundary, and which ideas or code—if any—are suitable for an authorized e-commerce Browser Persona? Establish from the repository, commit history, extension manifest, source, tests, and first-party author material its injection timing, seed scope, covered surfaces, frame and worker coverage, determinism, internal consistency, permissions, update path, detectable failure modes, and compatibility risks. Separate verified behavior from README claims and conclude with reuse, reimplement, or reject criteria; do not treat anti-detection claims as a product goal.

## Comments

- Planned research asset: [`../research/juu17-fingerprint-shuffler.md`](../research/juu17-fingerprint-shuffler.md)

## Answer

Resolved in [`Juu17 Browser Fingerprint Shuffler 源码审阅`](../research/juu17-fingerprint-shuffler.md). At pinned commit `02ae61b`, only the asynchronously injected WebGL page script can affect ordinary top-frame page JavaScript; the Canvas, Audio, and Navigator hooks remain in Chrome's default isolated content-script world, while subframes and site workers are uncovered. A shared stateful PRNG, canvas mutation, changed AudioBuffer semantics, call-order-dependent WebGL values, and silent seed replacement prevent this from being a stable, coherent Browser Persona. Retain only the persistent root-seed and per-origin derivation concepts, independently reimplement them as versioned per-surface pure functions, and default to stock Chrome truth until frame, worker, timing, and selected Seller Platform compatibility tests pass.
