# 商业 Fingerprint Browser 桌面技术栈决策

研究快照：2026-08-15。**决策状态：已由用户锁定为 React + TypeScript + Tauri v2。** 产品边界是 Windows 11 首发、local-first、管理外部 stock Chrome/Chromium Browser Identities 的商业桌面软件；Tauri、Wails 或 Electron 只承载管理台，不承载电商网站。本文综合三个框架专项报告、官方一手文档与四个 Rust 项目源码核验。分数是针对本产品的架构判断，不是通用框架排行榜或性能基准。

## 决策

**推荐基线：Tauri v2 + React/TypeScript + Tauri-free Rust BrowserControl core。**

具体不是“全 Rust UI”，而是：

- React/TypeScript 负责工作台、表格、筛选、模板、状态和 i18n；
- Tauri 只负责本地窗口、托盘、IPC、安装和更新适配；
- Rust deep Module 负责 Browser Identity、Profile 锁、Chrome 进程树、本地代理、Network Egress、Windows Job、秘密、恢复与有限 CDP/Native Messaging；
- 需要管理员权限的 WFP/Firewall 能力放进单独签名、最小权限的 Windows helper/service，桌面 UI 始终按普通用户运行；
- 所有网站只在外部 Chrome/Chromium 中打开，绝不放进 Tauri/Electron/Wails 的特权 WebView。

推荐顺序：

1. **Rust + Tauri v2：默认选择。** 产品核心与 Windows 原生适配在同一 Rust 信任链内，不需要为了 UI 壳额外引入 native addon 或 sidecar RPC。
2. **Electron + React/TypeScript + 签名 Rust sidecar：条件性备选。** 只在团队的 Electron/React 交付优势足够大、能抵消双运行时和双构建链时选择。
3. **Go + Wails：Go 团队的可行备选。** Wails v2 当前稳定但商业发布能力需要自建；功能更合适的 v3 仍是 beta，不应作为无条件 GA 基线。
4. **纯 Electron/TypeScript 高权限核心：拒绝。** Windows Job、WFP、秘密与 Chrome 生命周期最终仍会逼出 native addon/sidecar，同时 main process、npm 和 IPC 获得过宽权力。

## 为什么这个产品会改变一般桌面框架排名

Electron 的典型优势是随应用携带一套确定版本的 Chromium + Node。但本产品真正被网站观察、存放 Cookie、运行扩展和使用代理的浏览器是**另一套外部 Chrome/Chromium**。Electron 自带 Chromium 只渲染管理台，因此：

- 它不会改善 Browser Persona、Profile Isolation 或 Seller Platform 兼容性；
- 它会与最多 20 个外部 Chrome 窗口同时存在，增加一套 Chromium/Node 的发布和安全更新责任；
- 深层 Windows 控制仍需 native addon 或独立 Rust/Go sidecar。

Tauri/Wails 使用 Windows WebView2 的版本漂移仍要测试，但影响的是我们拥有的本地管理 UI，不是实际浏览身份。Windows 11 自带 WebView2，Evergreen runtime 会独立更新；对保守、纯本地资产的控制台，这是可接受的取舍。[Tauri WebView versions](https://v2.tauri.app/reference/webview-versions/) [Microsoft WebView2 distribution](https://learn.microsoft.com/en-us/microsoft-edge/webview2/concepts/distribution)

## 同一权重下的比较

权重专门面向本项目：产品核心/Windows 原生适配 25%，信任边界与安全 20%，商业发布与更新 15%，UI/团队交付效率 15%，资源与更新负担 10%，测试/可观测 8%，框架稳定性与支持 7%。

| 方案 | 加权判断 | 主要收益 | 决定性代价 | 商业结论 |
| --- | ---: | --- | --- | --- |
| **Tauri v2 + Rust core + React/TS** | **8.1/10** | 核心和 Windows adapter 同语言；Tauri capability/permission；系统 WebView；签名 updater | Rust/Win32 人才、系统 WebView 矩阵、原生 crash reporting 需自建 | **推荐**，通过 Windows PoC 后锁栈 |
| Electron + React/TS + Rust sidecar | 7.7/10 | 最成熟的 Web/桌面生态、调试和打包体验 | 重复 Chromium；八周主版本节奏；双栈、sidecar RPC、整体签名/回滚 | 团队 Electron 优势显著时备选 |
| Wails v3 beta + Go | 7.5/10（内部原型） | Go 控制平面合适；系统 WebView；v3 service/multi-window/updater 形态更好 | 仍是 prerelease，安全支持和兼容性不能当 GA 承诺 | 只做精确锁版本的内部验证 |
| 纯 Electron/TypeScript core | 6.8/10 | 早期 UI/原型最快，生态最大 | 高权限 main/npm；Job/WFP 需要 addon；重复运行时与持续 Chromium 升级 | 不作为长期架构 |
| Wails v2 + Go | 6.6/10 | 稳定、安全策略明确支持；Go 原生/网络能力好 | updater、crash、原生商业发布链更多自建；v2→v3 迁移 | Go-heavy 团队可选，不是默认 |

这些数值不是实测结果。最终资源、启动、20 窗口响应和升级可靠性必须由同功能签名 Release 原型决定。

## 三条技术路线的深度判断

### 1. Rust + Tauri v2

Tauri 的核心优势不是营销意义上的“小”，而是它允许桌面 Adapter 直接调用同进程 Rust application/core，同时把 WebView 权限收敛到明确的 commands、capabilities、permissions 和 scopes。官方也明确说明这些机制只能减少前端被攻破后的影响，不能弥补不安全的 Rust command 或过宽配置。[Tauri capabilities](https://v2.tauri.app/security/capabilities/) [Runtime Authority](https://v2.tauri.app/security/runtime-authority/) [Tauri architecture](https://v2.tauri.app/concept/architecture/)

商业上可取的结构：

```text
React/TypeScript control UI
        |
        | small typed intents; no path/shell/SQL/secret/raw CDP
        v
apps/desktop-tauri          <- replaceable Adapter
        |
        v
crates/browser-control      <- deep Module, no tauri::*
        |
        +-- browser-domain
        +-- storage-sqlite
        +-- secrets-windows
        +-- platform-windows (Job/process/window)
        +-- chrome-runtime (profile/MV3/native messaging/CDP subset)
        +-- local-proxy + egress-policy
        +-- signed privileged network helper
```

Tauri updater 强制校验更新签名，Windows 支持 MSI/NSIS 和代码签名，但这只是原语；应用、Native Messaging host、扩展、helper、schema 必须作为一个版本化 release set 更新并可恢复。[Tauri updater](https://v2.tauri.app/plugin/updater/) [Windows installer](https://v2.tauri.app/distribute/windows-installer/) [Windows signing](https://v2.tauri.app/distribute/sign/windows/)

主要风险：

- React WebView 仍应被视为不可信 presentation；不能给它通用 shell、filesystem、SQL、HTTP 或 updater 权限；
- CSP 不是默认魔法，必须只加载随包资产并显式配置；[Tauri CSP](https://v2.tauri.app/security/csp/)
- Evergreen WebView2 会独立更新，需 Stable/Preview 前向兼容测试；
- Tauri 没有替我们解决 Job、WFP、代理直连、Chrome secrets 或 Profile 恢复；
- 官方日志插件不是完整 native minidump/symbolication 方案，崩溃采集和隐私清洗仍需产品拥有；
- Rust + Windows internals 招聘、FFI review、编译时间和 CI 是真实成本。

### 2. TypeScript + Electron

Electron 的商业成熟度最高：main/renderer/preload 进程模型清楚，Forge、原生签名、Squirrel/MSIX 更新、Crashpad 和 Chromium DevTools 路径成熟。[Electron process model](https://www.electronjs.org/docs/latest/tutorial/process-model) [Electron updates](https://www.electronjs.org/docs/latest/tutorial/updates) [autoUpdater](https://www.electronjs.org/docs/latest/api/auto-updater/)

但官方安全指南同时明确：Electron 把 Chromium、Node、Electron、npm 依赖和应用代码共同放进安全结果，厂商必须持续升级随包 Chromium/Node；renderer IPC 必须验证 sender、保持 sandbox/context isolation，不能加载不受信远程内容。[Electron security](https://www.electronjs.org/docs/latest/tutorial/security) [Electron sandbox](https://www.electronjs.org/docs/latest/tutorial/sandbox)

对本产品，合理 Electron 拓扑只能是：

```text
React renderer -> typed preload -> thin Electron main
                                -> authenticated named pipe
                                -> signed Rust BrowserControl sidecar
```

这仍可成立，但必须接受：

- 安装和运行时同时存在 Electron Chromium/Node 与外部 Chrome；
- Electron 主版本、Forge、sidecar、扩展、helper 有联合版本矩阵；
- sidecar protocol、ACL、nonce、重试幂等、原子更新和崩溃恢复全部是新增产品工作；
- ASAR 不是代码加密，fuses/ASAR integrity/OS signing 只能做完整性分层；
- 如果最后 80% 的高价值代码都在 Rust sidecar，Electron 只剩 UI 壳，必须证明其 UI 交付优势值得这层成本。

因此 Electron 是“组织能力驱动”的第二名，不是结构上的第一名。

### 3. Go + Wails

Go 本身很适合 supervisor、本地代理、SQLite、named pipe 和长期运行服务。`golang.org/x/sys/windows` 已暴露 Job Object 和 Win32 primitives；WFP 仍需高风险 ABI/权限原型，与选择哪种 WebView 框架无关。[x/sys/windows](https://pkg.go.dev/golang.org/x/sys/windows) [Microsoft WFP](https://learn.microsoft.com/en-us/windows/win32/fwp/about-windows-filtering-platform)

Wails 的当前版本门是决定性问题：

- v2.14.0 是稳定线并受官方安全策略支持；
- v3.0.0-beta.8 提供更好的 service model、多窗口、签名 updater 和发布结构，但官方仍标记 Beta；[Wails v3 status](https://v3.wails.io/status/)
- v2 现在商发需要产品自有签名 updater、crash 方案和完整原生测试；
- v3 适合内部原型，不能把 beta 的路线图当商业 GA 承诺。

Wails 也必须只是 Adapter。将 Wails runtime context、绑定 DTO 或 WebView event 放入 BrowserControl 会让未来 v2→v3 或框架替换触碰领域内核。其社区规模和已验证商业支持路径小于 Electron/Tauri；若公司已经有强 Go/Windows 团队，它可以超过 Electron，否则不值得为桌面壳引入第三种组织方向。

## 用户提供的 Rust 项目：能证明什么，不能证明什么

### `Leon-Wo/fingerprint-browser`

[仓库固定快照](https://github.com/Leon-Wo/fingerprint-browser/tree/49a29059351d593483bb0520010d7c72d50752a4) 确实是 Tauri + React + Rust MVP：Profile CRUD、外部 Chromium 启停、CDP endpoint、检测页面回归和测试骨架都存在。它证明“React 控制台 + Rust launcher + 外部 Chromium”能快速形成 MVP。

但它不是商业基线：当前代码只发现 macOS 浏览器；Profile/代理密码保存在 WebView `localStorage`；前端可把 `launch_args` 和环境变量交给特权 Rust command；CDP endpoint 返回 UI；README 仍把更强的 fingerprint injection 和自动恢复列为 planned；没有 GitHub release，也没有仓库许可证文件。应借鉴界面与检测回归思路，不能复用其秘密、IPC 或运行契约。

### `Simprint/simprint`

[仓库固定快照](https://github.com/Simprint/simprint/tree/8d24b350ef716af16f819ee7b503c0a58181b184) 是活跃的 Tauri/Rust/TypeScript 产品型代码库，覆盖环境、代理、同步、多窗口、Local API/MCP、服务端和发布编排。它最有价值的是产品面、release orchestration 和“桌面 Adapter + 运行时服务”的规模化案例。

限制同样明确：仓库是 AGPL-3.0，README 为不遵守 AGPL 的用途提供另行商业许可路径；README 还说明 `simprint-runtime` 和 `simprint-browser-kernel` 等核心组件仍在准备进一步开放。它不能作为“浏览器内核已完整可审计”的证据，也不能直接搬进闭源商业产品。只做 clean-room 的产品/模块边界参考。

### `snaberino/pota-browser`

[仓库固定快照](https://github.com/snaberino/pota-browser/tree/b23f8e084617a011701aed8432ad64bdc46993ac) 是 Rust + `eframe`，不是 Tauri。它有外部 Chromium、多 Profile、自定义 browser path 和“Chrome 只连 loopback、本地代理再连上游代理”的正确方向；README 也诚实承认 launch flags/CDP 不足以覆盖深层 fingerprint surfaces。

它仍是早期实验：没有 release、没有许可证文件、没有商业安装/更新/秘密/Job/WFP/恢复证据。可参考 local forward-proxy seam，不可当产品 base。

### `izzipizzy/rustcloak`

[仓库固定快照](https://github.com/izzipizzy/rustcloak/tree/1fd49ab50de4f087d720e8a0cd756bc419b3c97c) 是真实的 Tauri + Svelte + Rust manager，且把纯 Rust core 放在独立 crate 中；每 Profile 独立 User Data、seed、proxy，能下载并校验 CloakBrowser engine。这是四个案例里最接近推荐 Module/Adapter 形态的原型。

但当前只声明 macOS，项目历史和 release 证据很薄，README 虽写 MIT、仓库却没有标准 LICENSE 文件。更关键的是它依赖 CloakBrowser proprietary binary：CloakBrowser 仓库的 wrapper 是 MIT，但 binary license 禁止再分发/打包，并明确规定第三方客户可控制浏览器能力时需要单独 OEM/SaaS license。[CloakBrowser binary license](https://github.com/CloakHQ/CloakBrowser/blob/main/BINARY-LICENSE.md) 因此只能借鉴 core/adapter、checksum 与 engine capability negotiation，不能把该 engine 当我们商业产品的默认依赖。

## 从这些案例得到的共同结论

四个项目支持的是下面这条结论：**Rust/Tauri 很适合做 Chromium manager。** 它们没有替我们证明下面这些商业属性：

- Profile secret、proxy credential、Cookie 和更新密钥的完整生命周期；
- Chrome 真实多进程树的 Job containment 与崩溃恢复；
- proxy/auth/DNS/UDP/QUIC/WebRTC 失败时不会直连；
- 全 frame/worker/headers/TLS/GPU 等 Persona 一致性；
- signed installer、原子多组件 updater、rollback、SBOM、崩溃符号；
- 200 stored / 20 active 的总产品资源和长期稳定性；
- Chrome/Chromium、extension 和任何 patched engine 的商业再分发权。

所以正确用法是“借 pattern，不继承 trust”：重新实现领域模型、路径/锁、秘密、网络 enforcement、运行状态机和 release chain；每个外部 engine 都通过版本化 capability Adapter 接入。

## 建议落地栈

首发实现建议：

- Desktop shell：Tauri v2，精确锁 core/plugin 版本；
- UI：React + TypeScript + Vite，所有资产随包，strict TypeScript，严格 CSP；
- Core：Rust workspace，`browser-domain` + `browser-control`，不依赖 `tauri::*`；
- Async/runtime：Tokio 仅用于 I/O 和 supervisor，领域状态机保持同步、可测；
- Metadata：Rust-owned SQLite + migrations + launch/recovery journal，React 不直接执行 SQL；
- Windows：Microsoft `windows`/`windows-sys` bindings；Job/DPAPI/CredMan/named pipe/WFP 分 Adapter；
- Network：每 Identity 的 loopback forward proxy + OS egress enforcement，失败关闭；
- Browser：用户/管理员安装的 Stable Chrome 优先；一 Identity 一完整 non-default User Data；
- Integration：最小 MV3 + Native Messaging；CDP 仅内部、随机/pipe、版本探测、能力限定；
- Release：native Windows build、Authenticode、Tauri signature updater、immutable manifest、分阶段 rollout、回滚与 SBOM；
- Observability：Rust structured events、Windows dump/symbol、Chrome/helper exit classification；默认不记录 Cookie、密码、代理口令、页面正文或裸 CDP。

## 锁栈前的退出条件原型

不要先写完整 UI。用一个 2–3 周 Windows Release PoC 同时淘汰架构风险：

1. 签名 Tauri app 创建 200 个 metadata identities，并稳定启停 20 个真实 Chrome User Data roots；
2. suspended launch → assign Job → resume，证明优雅关闭、强制关闭、Tauri crash 和重启 reconcile；
3. 本地代理断线、错误密码、DNS/UDP/QUIC/WebRTC 故障注入，抓包证明无 direct fallback；
4. 恶意 WebView 输入无法获得 shell、任意路径、SQLite、secret、launch args 或裸 CDP；
5. WebView2 Stable/Preview、DPI/RDP、睡眠恢复、UI crash 通过；
6. 签名安装、升级中断、schema/extension/native-host/helper 版本错配和 rollback 通过；
7. 记录冷/热启动、idle、20 Chrome 下 UI 响应、handle/process/disk/update 大小；再与同功能 Electron + Rust sidecar PoC 比较。

如果 Tauri 原型失败在 UI 壳能力或团队交付速度，而 Electron 原型明显达标，再切 Electron + Rust sidecar。若失败发生在 Job/WFP/Chrome/Profile 机制，换 Electron/Wails 不会解决根因，应修 BrowserControl/Windows Adapter。

## 最终结论

**商业首选不是“Rust 做所有东西”，而是“Tauri/React 做薄桌面壳，Rust 做可独立测试和替换 Adapter 的产品内核”。** 这同时保留 Web UI 生产力、降低重复运行时和 IPC 复杂度，并让真正困难的 Windows/Chrome 可靠性集中在一个 deep Module。用户列出的项目证明该形态有现实可行性；它们的许可证、秘密、内核完整性和发布缺口也证明我们不应 fork 任意一个作为商业底座。
