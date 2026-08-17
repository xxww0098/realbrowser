# Establish Chrome Control Constraints

Type: research
Status: resolved
Assignee: research_virtual_xchrome

## Question

What can a Windows Rust control plane safely and supportably control in stock Chrome or Chromium through command-line switches, separate user-data directories, managed Manifest V3 extensions, native messaging, DevTools Protocol, and operating-system process controls? Using official Chrome, Chromium, extension-platform, CDP, and Windows sources, establish constraints around Profile locking, extension injection timing, service workers and frames, proxy configuration and authentication, DNS/WebRTC/QUIC leakage, crash recovery, credential encryption, binary redistribution, updates, and enterprise policies. Mark unstable or unsupported mechanisms explicitly.

## Comments

- Planned research asset: [`../research/chrome-control-constraints.md`](../research/chrome-control-constraints.md)

## Answer

Resolved in [`Windows Rust 控制平面使用 stock Chrome/Chromium 的支持边界`](../research/chrome-control-constraints.md). The supported MVP seam is a Rust supervisor owning one non-default User Data directory and Windows Job per Browser Identity, a minimal managed MV3 extension bridged through Native Messaging, and a capability-limited, version-probed CDP subset. Chrome proxy configuration alone cannot enforce fail-closed egress, so a local proxy plus OS-level TCP/UDP/DNS controls is mandatory. Chrome profile secrets remain opaque and Chrome-owned; enterprise policies and extension force-installation are administrator-owned constraints, and normal installed Chrome should retain browser-update ownership.
