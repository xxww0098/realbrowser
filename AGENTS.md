## 核心规范文档

@docs/rust-engineering.md 是总工程规范。

# RealBrowser

Repository-wide rules for coding agents: an index plus the norms every change must respect. Keep details in their single source of truth.

## Read before changing code

- Product scope and non-goals: [`PRODUCT.md`](PRODUCT.md)
- Domain language and ownership: [`CONTEXT.md`](CONTEXT.md)
- Visual contract: [`DESIGN.md`](DESIGN.md)
- Rust and workspace rules: [`docs/rust-engineering.md`](docs/rust-engineering.md)
- Fingerprint surfaces and modification seams: [`docs/fingerprint-browser-principles.md`](docs/fingerprint-browser-principles.md)
- AdsPower UX/kernel comparison and chase order: [`docs/adspower-comparison.md`](docs/adspower-comparison.md)
- Kernel-level Persona path (if Chromium patches are justified): [`docs/kernel-level-persona.md`](docs/kernel-level-persona.md)
- Commands, dev runtime, verification, and Computer Use: [`README.md`](README.md)
- Current delivery sequence: [`.scratch/fingerprint-browser/implementation/minimum-runnable-plan.md`](.scratch/fingerprint-browser/implementation/minimum-runnable-plan.md)

## Documentation index

| Document | Sole responsibility |
| --- | --- |
| `README.md` | Development, build, verification, stable Dev shell, Computer Use |
| `PRODUCT.md` | Product scope, supported behavior, non-goals, commercial claims |
| `CONTEXT.md` | Domain vocabulary, ownership, and invariants |
| `DESIGN.md` | Visual language, layout, interaction, and accessibility contract |
| `docs/rust-engineering.md` | Rust workspace structure, dependencies, testing, and machine gates |
| `docs/fingerprint-browser-principles.md` | Browser fingerprint surfaces, modification seams, and coherence constraints |
| `docs/adspower-comparison.md` | AdsPower public surface vs RealBrowser, UX chase order, kernel decision |
| `docs/kernel-level-persona.md` | Product Chromium K0+K1 Persona contract and acceptance path |
| `.scratch/fingerprint-browser/implementation/minimum-runnable-plan.md` | Delivery sequence and acceptance evidence |

## First norms

- Rust owns business truth. React renders projections and invokes typed Tauri commands.
- Keep `realbrowser-desktop` a thin composition adapter. Browser lifecycle belongs in `browser-control`; OS process work belongs in `browser-platform`.
- A Browser Identity binds an isolated Profile, a coherent Persona, and one Network Egress policy. Do not weaken one boundary while changing another.
- Chrome-owned login state is opaque. Do not read, export, sync, back up, or migrate Cookies, Login Data, passwords, or tokens.
- Capability labels must describe what the runtime actually applies and observes. Never present planned, partial, or page-only coverage as applied browser-wide.
- Keep the crate graph flat and wide. Put shared dependency versions in the workspace root and obey the dependency direction in `docs/rust-engineering.md`.
- Preserve unrelated worktree changes. Scope edits, formatters, and generated files to the task.
- UI uses a single primary work area without redundant top-level page titles in main/detail workspaces (不再在详情/工作台页面设立顶层标题). Temporary dialogs and right drawers are allowed; permanent left/right content splits are not. Keep Chinese copy short, borders warm and subtle, and corners soft.

## Agent workflow

1. Read the source-of-truth documents for the layer being changed.
2. Trace the real path from React action through Tauri and Rust ownership to persisted/runtime state.
3. Implement the smallest complete vertical slice; avoid parallel truths and placeholder capability claims.
4. Run the checks documented in `README.md`. State clearly when evidence is source-only, cross-compiled, or native runtime proof.
5. For macOS UI or interaction changes, run the stable Dev shell and complete the Computer Use loop below.

## Real UI acceptance

1. Start `./dev.sh` and keep it running; wait until `./dev.sh status` reports `identity: ready`.
2. Use the `computer-use` skill against the development bundle reported by `./dev.sh status`.
3. Read the current accessibility state before choosing an element.
4. Perform one relevant, non-destructive user action, then immediately read state again.
5. Derive every subsequent element index from the newest state. Use a screenshot only when the accessibility tree is insufficient.
6. Restore temporary filters or test state and read state once more.

UI acceptance is complete only when the real Tauri window exposes the expected state transition after the action. A frontend build, DOM test, source inspection, or screenshot alone is supporting evidence, not real UI proof. Keep acceptance manager-local unless the task explicitly scopes interaction with an external Chrome page or account.

## Documentation homes

- Product behavior or scope changes: `PRODUCT.md`
- Domain terms, invariants, or ownership changes: `CONTEXT.md`
- Visual rules or interaction patterns: `DESIGN.md`
- Rust/module/dependency/testing rules: `docs/rust-engineering.md`
- Fingerprint surface mechanics and modification seams: `docs/fingerprint-browser-principles.md`
- AdsPower comparison and UX chase order: `docs/adspower-comparison.md`
- Commands, prerequisites, ports, and runtime operations: `README.md`
- Sequencing and readiness evidence: `.scratch/fingerprint-browser/implementation/minimum-runnable-plan.md`

One meaning has one home. Link to it instead of copying it into a second document.
