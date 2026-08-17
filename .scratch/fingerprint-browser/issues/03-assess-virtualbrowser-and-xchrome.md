# Assess VirtualBrowser and XChrome

Type: research
Status: resolved
Assignee: research_virtual_xchrome

## Question

How do `Virtual-Browser/VirtualBrowser` and `chanawudi/XChrome` differ in engine ownership, Profile isolation, Browser Persona handling, proxy behavior, group control, local or remote APIs, credential boundaries, persistence, update trust, tests, releases, licenses, and maintenance health? Verify against each repository and first-party material, distinguish implemented behavior from marketing claims, and extract architectural lessons without importing evasion-oriented behavior.

## Comments

- Planned research asset: [`../research/virtualbrowser-xchrome.md`](../research/virtualbrowser-xchrome.md)

## Answer

Resolved in [`VirtualBrowser 与 XChrome：面向授权电商多店铺隔离浏览器的源码审阅`](../research/virtualbrowser-xchrome.md). VirtualBrowser is a more complete but non-reproducible product surface whose published repository omits its engine and Native/API implementation and whose privileged management UI loads remote JavaScript over HTTP; XChrome demonstrates the useful Chrome + per-Profile + local control-plane shape, but its current CDP Persona, proxy, secret, deletion, update, testing, and CC BY-NC boundaries make it unsuitable for reuse. The product should independently implement a Rust supervisor around supported Chrome/Chromium, fail-closed egress, stable minimal Persona, OS-backed secrets, recoverable lifecycle, and a capability-limited local API.
