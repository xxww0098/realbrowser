# RealBrowser

Agent index. Details live in one document each. Link to that document; do not copy it.

## When to read

| Change touches | Read |
| --- | --- |
| What the product does or refuses | [`PRODUCT.md`](PRODUCT.md) |
| Names for identity, profile, persona, egress | [`CONTEXT.md`](CONTEXT.md) |
| Layout, color, copy, dialogs, drawers | [`DESIGN.md`](DESIGN.md) |
| Commands, toolchain, kernel build, tests | [`README.md`](README.md) |
| Crate layout, dependencies, test shape, `xtask` | [`docs/rust-engineering.md`](docs/rust-engineering.md) |
| A fingerprint surface or how a seam can lie | [`docs/fingerprint-browser-principles.md`](docs/fingerprint-browser-principles.md) |
| AdsPower parity or chase order | [`docs/adspower-comparison.md`](docs/adspower-comparison.md) |
| Product Chromium, K0 file, or K1 Canvas | [`docs/kernel-level-persona.md`](docs/kernel-level-persona.md) |

`docs/fingerprint-browser-principles.md` and `docs/adspower-comparison.md` are references, not capability claims. `docs/rust-engineering.md` is the engineering handbook; the checks this repo actually runs are `cargo run -p xtask -- structure` and `cargo run -p xtask -- ci`.

## Norms

- Rust owns identity, persona, network, launch plans, and process lifetime. The GPUI Kit shell renders projections and calls `browser-control` in process.
- `realbrowser-desktop` stays a thin adapter. Lifecycle belongs in `browser-control`. OS process work belongs in `browser-platform`.
- One Browser Identity binds one Profile, one coherent Persona, and one Network Egress. Do not weaken one boundary to change another.
- Profile roots, executable paths, data roots, and process IDs stay inside Rust. The shell shows opaque ids, operator codes, modes, and status.
- Cookies, Login Data, passwords, and tokens stay opaque. Do not read, export, sync, back up, or migrate them.
- A capability label matches what the running kernel applies and observes. Planned or page-only coverage is not "applied". `graphics.canvas` is `CustomKernel` only while a passing product runtime is active.
- Shared dependency versions live in the workspace root. Keep the crate graph wide and obey the direction enforced by `xtask`.
- The window is one work surface. No persistent left/right content split and no extra page title on a workspace. Dialogs and a temporary drawer are allowed. Chinese labels stay short. See `DESIGN.md`.
- Scope edits to the task. Do not reformat or regenerate unrelated files.

## Workflow

1. Read the row above for the layer you are changing.
2. Follow the call from the GPUI action into the crate that owns the state.
3. Ship the smallest complete slice. Do not add a second model or a placeholder capability.
4. From the repo root, run `cargo run -p xtask -- ci`. Say whether the evidence is source-only, cross-compiled, or a native kernel run.
5. A UI change is accepted only when the GPUI window shows the state change. `./dev.sh` opens that window and stores data under `.dev/data`. `./dev.sh status` prints the shell and data root; it does not probe identities. A projection test is supporting evidence, not UI proof.

Linux without a display can run `cargo run -p xtask -- ci`. Linking the window also needs the GUI libraries in `README.md` and `cargo build -p realbrowser-desktop --features shell`. The product Chromium kernel and ignored kernel tests stay on a machine that can launch that kernel. A Linux test run is not native UI proof and is not a reason to mark Canvas as `CustomKernel`.
