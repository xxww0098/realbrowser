# Electron + TypeScript 作为 RealBrowser 商业桌面栈

审阅日期：2026-08-15。资料范围限定为 Electron、Electron Forge 与 Electron GitHub 组织的官方文档、发布页和源码仓库。本文假定被管理的网站始终运行在**外部 stock Chrome** 中；Electron 只承载本地控制台 UI，绝不把卖家后台或其他不受信网页装进 `BrowserWindow`/`WebContentsView`。这一区分会显著改变 Electron 的安全判断。

文中“官方事实”均附第一方链接；“架构判断”是针对当前产品约束的推论；所有资源、兼容性和防护效果在完成签名安装包及 Windows 11 实测前均为“待验证”，没有使用或虚构体积、内存、启动时延等基准数字。

## 结论

**Electron + TypeScript 是很强的商业 UI 栈，但不是这个产品最合适的高权限浏览器控制内核。** 最有竞争力的 Electron 方案是：React/TypeScript 做控制台，Electron 仅做窗口、托盘、菜单、更新和一层很薄的桌面适配器；Chrome 进程树、Profile 路径、代理、Windows Job、网络 fail-closed、秘密存储和崩溃恢复交给同安装包内的签名 Rust sidecar。该组合的暂定商业适配度为 **7.5/10**。如果把这些高权限职责直接写进 Electron main process 和 npm/native-addon 依赖，适配度降为 **6.7/10**，不建议作为长期商业架构。

Electron 会成为三者中的优先选择，必须同时满足这些条件：

1. 团队最强能力是 TypeScript/React，产品竞争力主要来自复杂工作台、表格、筛选、模板、实时状态和高频 UX 迭代；
2. 接受安装包额外携带一套 Chromium + Node，并接受每八周处理一次 Electron 主版本升级；
3. 接受从第一天就维护一个版本化的本机 sidecar 协议，而不是让 renderer/main 直接拼 Chrome 参数、读写任意路径或操作防火墙；
4. Electron renderer 只加载随包本地 UI，电商网站只在外部 Chrome 中打开；
5. 商业发布有原生 Windows/macOS 构建、签名、notarization、自动更新、回滚和兼容性回归能力。

若最优先目标是最小安装体积、最少运行时、长期离线而不频繁升级，或把 Windows 低层系统控制直接放在桌面进程里，Electron 就不是自然首选。

## 版本与支持基线

截至 2026-08-15，官方发布页显示最新稳定补丁为 **Electron 43.4.0**，内含 Chromium `150.0.7871.224` 和 Node `24.18.1`；受支持的稳定主版本为 43、42、41，44 仍是 beta。不能把 beta 或 nightly 当商业基线。[Electron Release Status](https://releases.electronjs.org/)

Electron 每八周发布一个稳定主版本，官方仅支持最近三个稳定主版本；最新线接收全部修复，次新线按资源接收大多数修复，最老线只接收安全修复。[Electron Releases](https://www.electronjs.org/docs/latest/tutorial/electron-timelines) Electron 43 已明确结束 40.x 支持，并宣布 43 是 Windows x86/Linux ARMv7 预编译包的最后一个系列；这对 Windows 11 x64/arm64 的首版没有阻碍，但说明商业产品不能长期钉死旧版。[Electron 43 release](https://www.electronjs.org/blog/electron-43-0)

**架构判断：** Electron 的升级节奏是持续运营成本，而不是一次性选型成本。RealBrowser 应固定到 43.x 的具体补丁而非宽泛 semver，建立“新补丁快速吸收、主版本逐个升级”的回归流水线。好处是卖家网站不运行在 Electron 内，所以 Electron 的 Chromium 升级主要影响本地 UI，而外部 Chrome 兼容矩阵可单独演进；坏处是 main/preload、Forge、native addon 和安装器仍需持续回归。

## 正确的 seam：Electron 是桌面适配器，不是 BrowserControl 本体

Electron 官方进程模型包含一个拥有完整 Node.js 能力的 main process、每个窗口对应的 renderer，以及介于两者之间的 preload；renderer 默认不能直接使用 Node，特权动作经 IPC 委托给 main。[Process Model](https://www.electronjs.org/docs/latest/tutorial/process-model) 对本产品最深、最小的模块应放在以下 seam：

```text
React renderer
  -> typed preload intentions
  -> ElectronDesktopAdapter (main)
  -> authenticated local RPC
  -> BrowserControl sidecar
  -> Chrome / profiles / proxy / OS enforcement
```

`BrowserControl` 的 Interface 只应表达产品意图，例如：

- `listIdentities()`、`createIdentity(draft)`、`archiveIdentity(id)`；
- `startIdentity(id)`、`stopIdentity(id)`、`reconcileRuntime()`；
- `testProxy(id)`、`subscribeRuntimeEvents()`；
- 返回稳定的领域错误，如 `ProfileLocked`、`ProxyUnavailable`、`PolicyDenied`、`ChromeVersionUnsupported`。

它不应暴露 `spawn(executable, args, env)`、`readFile(path)`、任意 shell 命令、原始代理凭据、裸 CDP 端口或“执行任意 JavaScript”。这些只是实现细节，一旦进入 Interface，renderer 被攻破就等于获得本机控制能力。生产的 `SidecarBrowserControl` 与测试用 `FakeBrowserControl` 是两个真正的 Adapter；调用者和测试都只穿过同一 seam，Chrome 参数、路径规范化、进程树归属和恢复逻辑因此集中在一个深模块内。

**架构判断：** 如果 Electron main 自己实现全部 BrowserControl，它会同时承担窗口生命周期、IPC 权限检查、SQLite/秘密、进程监管、代理和系统策略，Interface 会退化成大量浅 IPC channel。sidecar 让 Electron main 保持为桌面 Adapter，也把 native 崩溃、系统权限和 Chrome 监管从 UI 故障域中分离。

## 进程模型与安全姿态

### 推荐责任分配

| 区域 | 应拥有的能力 | 明确禁止 |
| --- | --- | --- |
| Renderer | 本地 React UI、表单校验、视图状态 | Node、文件系统、shell、Chrome/CDP、代理秘密 |
| Preload | 少量具名、类型化的 intention 方法 | 直接暴露 `ipcRenderer`、通用 `send`、任意 callback event 对象 |
| Electron main | 窗口/托盘/菜单、更新 UI、调用 sidecar Adapter | 解析任意 shell 字符串、保存网站凭据、加载电商网页 |
| Sidecar | 身份生命周期、路径/锁、Chrome 监管、代理与 OS 策略、秘密句柄 | 向 UI 返回明文密码/Cookie/令牌或裸 CDP |
| 外部 Chrome | 不受信网站、站点登录态、扩展和网页沙箱 | 反向控制 Electron 或跨 Profile 访问 |

Electron 官方明确指出它不是普通浏览器：Node、Chromium、Electron 和 npm 依赖共同形成安全结果，加载不受信远程内容会放大风险。[Security](https://www.electronjs.org/docs/latest/tutorial/security) 由于本方案只让 renderer 加载随包本地 UI，Electron 最危险的远程网页场景可以被设计掉；这不是“Electron 天然安全”，而是主动缩小攻击面。

生产窗口必须满足：

- `nodeIntegration: false`、`contextIsolation: true`、`sandbox: true`；context isolation 自 Electron 12 默认启用，renderer sandbox 自 Electron 20 默认启用，但仍应显式配置并在测试中断言。[Context Isolation](https://www.electronjs.org/docs/latest/tutorial/context-isolation) [Process Sandboxing](https://www.electronjs.org/docs/latest/tutorial/sandbox)
- 使用受限自定义协议加载本地 UI，不使用拥有额外历史权限的 `file://`；设置 `default-src 'self'` 起步的严格 CSP，阻止任意导航、窗口创建和远程脚本。Electron 官方安全清单也要求限制导航、新窗口、权限、外链和不安全内容。[Security checklist](https://www.electronjs.org/docs/latest/tutorial/security)
- preload 每个方法对应一个业务意图并校验输入/输出；不能把 `ipcRenderer` 原样挂到 `window`。官方教程明确将这种做法称为强攻击面。[Using Preload Scripts](https://www.electronjs.org/docs/latest/tutorial/tutorial-preload)
- main 对每条 IPC 校验 `senderFrame`，再做运行时 schema、长度、枚举、身份状态和授权检查。Electron 官方指出 iframe/child window 也可能发 IPC，要求默认验证 sender。[IPC sender validation](https://www.electronjs.org/docs/latest/tutorial/security#17-validate-the-sender-of-all-ipc-messages)
- `shell.openExternal` 只接受解析后、协议与 host allowlist 命中的 URL；UI 不接收服务端下发的可执行 HTML/JS。

### Electron 到 sidecar 的本地 IPC

这是产品新增的高价值攻击面，Electron 官方 IPC 隔离并不会自动保护它。建议使用当前用户 ACL 限制的 Windows Named Pipe（macOS/Linux 用本地 socket），不监听 TCP；采用长度前缀消息、严格 schema、协议版本、随机会话 nonce、请求 id、超时和最大消息尺寸。sidecar 应拒绝任意路径、任意可执行文件、任意端口和任意环境变量，只接受身份 id 与领域命令。

**待验证：** 同用户恶意进程、旧版 UI、新版 sidecar、sidecar 重启、重复请求和管道劫持场景必须进入威胁测试。单靠“只在 localhost”不构成认证。

## Fuses、ASAR 与商业代码保护

Electron Fuses 是打包时修改二进制的开关，官方说明其结果应在签名前固定，再由 OS 代码签名保护。[Electron Fuses](https://www.electronjs.org/docs/latest/tutorial/fuses) 商业构建至少应使用 `strictlyRequireAllFuses: true` 并明确：

- 关闭 `RunAsNode`；
- 关闭 `EnableNodeOptionsEnvironmentVariable`；
- 关闭 `EnableNodeCliInspectArguments`；
- 开启 `EnableEmbeddedAsarIntegrityValidation`；
- 开启 `OnlyLoadAppFromAsar`；
- 若 UI 不使用 `file://`，关闭 `GrantFileProtocolExtraPrivileges`；
- cookie encryption 只保护 Electron 自身 session；本产品不应把网站登录态放进 Electron session。

Fuses 默认值并不全部是商业安全默认值：`runAsNode`、`NODE_OPTIONS` 和 CLI inspect 默认开启，而 ASAR integrity、only-load-ASAR 默认关闭。必须把 fuse 读取结果作为发布验收证据，不能只相信 Forge 配置。[Official fuse list](https://www.electronjs.org/docs/latest/tutorial/fuses#current-fuses)

ASAR 只是 Electron 的简单归档格式，官方只称它能“conceal your source code from cursory inspection”；Node 仍可把它当虚拟目录读取。[ASAR Archives](https://www.electronjs.org/docs/latest/tutorial/asar-archives) 因此：

- ASAR **不是加密、混淆或知识产权保险箱**；前端商业逻辑应假定可被提取；
- integrity + only-load-ASAR 能阻止未经验证的包内代码替换，但不能阻止合法进程中的逻辑缺陷或 renderer 获得过宽能力；
- native `.node`、sidecar 可执行文件及需要真实路径的文件必须在 ASAR 外。官方文档指出 `process.dlopen`、部分 child-process API 会临时解包，并可能触发杀毒软件；显式 `asar.unpacked` 仍需单独签名和清单校验。[ASAR limitations](https://www.electronjs.org/docs/latest/tutorial/asar-archives#limitations-of-the-node-api)

**架构判断：** 若 Persona 生成规则、许可证校验或策略引擎需要提高逆向门槛，把它们放进 sidecar 比放入 TypeScript/ASAR 更合适，但 native binary 同样不能被宣传为不可逆向。授权、签名、服务端许可与运行时完整性应是分层控制。

## Windows 原生集成与 native modules

Electron 对商业桌面“表层原生体验”支持很好：官方提供 tray、原生 menu/dialog、全局快捷键、开机启动、通知，以及 Windows JumpList、缩略图工具栏、overlay 和任务栏进度等能力。[Tray](https://www.electronjs.org/docs/latest/tutorial/tray) [Windows Taskbar](https://www.electronjs.org/docs/latest/tutorial/windows-taskbar) [app login items](https://www.electronjs.org/docs/latest/api/app#appsetloginitemsettingssettings) 这些足以实现 Profile 快速启动、运行状态托盘、紧急停止入口和升级提示。

但 Windows Job Object、WFP/Firewall、服务安装、进程令牌、ACL、DPAPI 细粒度策略等深层能力不是 Electron 的高层强项。Electron 支持 C++、Rust 等 native Node addon，理论上可调用全部 OS 能力；官方同时说明 addon 使用 Electron 的 ABI，需要针对 Electron 版本与平台/架构重编译，Electron 升级后通常要 rebuild，Windows 还涉及 delay-load hook。[Native Code and Electron](https://www.electronjs.org/docs/latest/tutorial/native-code-and-electron) [Native Node Modules](https://www.electronjs.org/docs/latest/tutorial/using-native-node-modules)

这形成三个商业风险：

1. Electron 主版本、Node ABI、CPU 架构与 native binary 形成发布矩阵；
2. native 崩溃可带走 main process，扩大 UI 与控制面的共同故障域；
3. unpacked DLL/`.node` 增加签名、杀毒误报和供应链核对工作。

**推荐：** 不为 BrowserControl 写大量 native addon；改用签名 Rust sidecar。Electron main 可以直接 spawn sidecar 或监督其生命周期，但所有参数由固定安装路径和结构化配置生成，绝不经 shell。若只需 Node 隔离而没有独立原生核心，Electron 官方 `utilityProcess` 可承载 CPU 密集或易崩溃 Node 模块；它并不能替代 Rust/Go sidecar 的独立 ABI、OS 能力和版本协议。[utilityProcess](https://www.electronjs.org/docs/latest/api/utility-process)

如果 fail-closed 网络策略需要管理员权限或系统服务，Squirrel.Windows 的“no-admin”安装体验无法凭空完成这一部署；需要明确的提权步骤、MSI/MSIX/专用 bootstrapper 或单独受控 helper。官方 Forge 把 Squirrel 描述为无管理员安装，也提供面向企业分发的 WiX MSI；后者体验更传统但适合组织策略。[Squirrel.Windows](https://www.electronforge.io/config/makers/squirrel.windows) [WiX MSI](https://www.electronforge.io/config/makers/wix-msi) 具体 WFP/Firewall 部署仍须 Windows 11 原型验证。

## 资源与分发影响

Electron 官方定义就是在应用二进制中嵌入 Chromium 与 Node。[Electron introduction](https://www.electronjs.org/docs/latest/) 在当前架构中，用户还会另外运行多个 stock Chrome Profile，因此 Electron shell 与外部 Chrome 是两套独立 Chromium 进程体系。

**可确定的方向性结论：**

- Electron 的安装/更新载荷与空闲进程面天然包含自身 Chromium/Node，不能像系统 WebView 壳那样复用已安装 WebView；
- 不应为每个 Browser Identity 建一个 Electron `BrowserWindow` 或 `WebContentsView`。一个主控制台窗口加少量辅助窗口即可，所有卖家网页都在外部 Chrome；
- Electron 43 官方宣称改进了 main 启动 snapshot、V8 bytecode cache、preload 启动数据和 Windows/Linux ThinLTO，但这不等于 RealBrowser 已达到任何启动或内存指标。[Electron 43 performance changes](https://www.electronjs.org/blog/electron-43-0#improved-app-startup-performance)
- 当同时运行 20 个外部 Chrome 窗口时，Chrome 很可能是主要资源消费者，但在没有同机测量前不能据此忽略 Electron 的 idle RSS、GPU/utility process、安装体积和更新流量。

商业选型前必须用同一 UI 原型、同一数据量和同一机器记录：签名安装包大小、首次/增量更新下载量、冷/热启动、空闲 RSS/进程数、1/10/20 个外部 Chrome Profile 时的 UI 响应、后台状态事件吞吐、崩溃后回收时间。本文没有这些运行证据，资源项评分因此是保守暂定值。

## 打包、签名、notarization 与自动更新

Electron 官方推荐 Electron Forge 完成打包和分发。[Application Packaging](https://www.electronjs.org/docs/latest/tutorial/application-distribution) Forge 可生成平台安装格式并通过 publisher 上传到 S3 等静态存储，不依赖 GitHub 才能发布。[Forge Makers](https://www.electronforge.io/config/makers) [Forge Publishers](https://www.electronforge.io/config/publishers)

商业发行仍需原生构建链：

- macOS 应代码签名并提交 Apple notarization；Electron/Forge 官方文档将二者列为直接分发的必要发布步骤，并要求 macOS/Xcode/开发者证书。[Electron Code Signing](https://www.electronjs.org/docs/latest/tutorial/code-signing) [Forge macOS signing](https://www.electronforge.io/guides/code-signing/code-signing-macos)
- Windows 应签 Authenticode；Forge 支持传统证书与 Windows 签名工具，并可签安装器。证书/云签名资格、HSM 与时间戳服务属于运营依赖，不能把“Forge 配了字段”当成签名链已证明。[Forge Windows signing](https://www.electronforge.io/guides/code-signing/code-signing-windows)
- Electron 内置 `autoUpdater`，官方支持 Squirrel 路径，也支持指向静态对象存储上的平台特定元数据；macOS 自动更新要求已签名应用。[Updating Applications](https://www.electronjs.org/docs/latest/tutorial/updates) [Forge Auto Update](https://www.electronforge.io/advanced/auto-update)

sidecar 会让发布链多一个必须整体处理的可执行物。建议 UI 与 sidecar 同版本发布、启动时互相握手协议版本、更新时原子替换、保留可回滚上一版，并在外层应用签名前完成 sidecar 签名/清单固定。不得让新版 UI 静默调用不兼容旧 sidecar，也不得允许下载后的 sidecar 绕过产品更新签名链。以上为架构要求，仍需对 Forge maker/updater 的实际产物做破坏性更新与回滚测试。

## 崩溃、日志、测试与调试

Electron `crashReporter` 使用 Crashpad，可上传 Electron main/renderer 崩溃到自有 endpoint 或托管平台；dump 暂存在应用数据目录。[crashReporter](https://www.electronjs.org/docs/latest/api/crash-reporter) 它不会自动解释外部 Chrome、代理进程或 Rust/Go sidecar 的失败。因此建议：

- Electron crash dump、sidecar crash artifact 和 Chrome runtime event 分开归属，用匿名运行 id 关联；
- 上传默认关闭或获得明确同意，单独做保留期、删除与访问控制；
- 任何通道都不得记录 Cookie、密码、令牌、代理口令、2FA、网页正文或完整 CDP payload；Profile 路径、命令行和 URL 也需字段级清洗；
- sidecar 异常不能让 Chrome 进程成为无主进程；重启后通过 `reconcileRuntime()` 从 OS 事实恢复，而不是相信 UI 缓存。

开发体验是 Electron 的明显优势：renderer 可用 Chromium DevTools，main 可通过 V8 inspector/VS Code 调试。[Application Debugging](https://www.electronjs.org/docs/latest/tutorial/application-debugging) Electron 官方也给出 Playwright E2E 方案，可启动 app、访问 main 并操纵窗口，但明确称 Playwright 的 Electron 支持为 experimental。[Automated Testing](https://www.electronjs.org/docs/latest/tutorial/automated-testing) Linux headless CI 还需显示驱动/Xvfb。[Testing on Headless CI](https://www.electronjs.org/docs/latest/tutorial/testing-on-headless-ci)

推荐测试层次：

1. React/领域 reducer 快速单元测试；
2. preload Interface 的 schema 与权限测试，恶意 renderer 输入作为负例；
3. `FakeBrowserControl` 上的 UI 集成测试；
4. production sidecar Adapter 的契约测试与故障注入；
5. Playwright Electron 控制台 E2E；
6. Windows 11 签名安装包 + 真实 Chrome 的 Profile 锁、代理中断、20 窗口、崩溃恢复、升级/降级验证。

生产 fuses 关闭 inspector 后，开发调试路径与生产二进制应明确分离；不能为了现场排障重新开放通用 Node inspector。

## 生态、招聘与开发效率

官方可证的事实是 Electron 允许用一套 JavaScript/HTML/CSS 代码创建 Windows、macOS、Linux 应用，renderer 可使用标准 web 工具链，main 可使用 Node/npm；官方 Forge 覆盖打包、maker、publisher、fuses 和 native rebuild。[Electron](https://www.electronjs.org/docs/latest/) [Forge Configuration](https://www.electronforge.io/config/configuration) 这对数据密集型商业控制台意味着 React 组件、表格、表单、可访问性、i18n 和设计系统都能直接复用 web 能力。

“TypeScript/React 更好招聘”与“Electron 一定开发更快”不是 Electron 官方文档能够证明的事实。针对本项目的合理推论是：若现有团队本来就能高质量交付 React/TypeScript，Electron 的学习面主要集中在 main/preload/IPC/发行而不是新 UI 语言，原型和产品迭代通常更直接；若团队缺乏 Electron 安全与原生发行经验，普通 web 经验不会自动覆盖 IPC、签名、更新、native addon 和进程监管风险。

sidecar 会削弱“单语言全栈”的卖点，但保留 UI 生产力，并把高风险系统代码放进更合适的实现。Rust sidecar 更适合需要内存安全和低层 Windows 控制的长期内核；Go sidecar 也能简化进程/网络编排与单文件分发，但两者都引入协议、双构建链、符号/崩溃采集和整体签名工作。这里是架构推论，不是语言性能基准。

## 许可证、SBOM 与供应链

Electron 源码采用 MIT 许可证，可用于商业软件，但 MIT notice 仍需随发行保留。[Electron LICENSE](https://github.com/electron/electron/blob/main/LICENSE) Electron 官方发布物还包含 Chromium/第三方 notices；官方 Linux 打包示例展示 `LICENSE` 和 `LICENSES.chromium.html` 随产品目录分发。[Snapcraft Guide](https://www.electronjs.org/docs/latest/tutorial/snapcraft) 这不替代对本应用 npm 包、native module、sidecar crates/modules、扩展和安装器依赖的逐项许可证审阅，也不构成法律意见。

Electron 官方安全指南明确把 Electron、Chromium、Node、npm 依赖和应用代码共同视为安全结果，并要求审慎评估依赖、及时升级。[Security responsibility](https://www.electronjs.org/docs/latest/tutorial/security#security-is-everyones-responsibility) 与 Tauri/Wails 相比，Electron 会自然带来 Chromium + Node + npm/Forge 的较大依赖图；sidecar 又增加 Rust crates 或 Go modules，但可反向删掉高风险 native Node addon。

从已审阅的 Electron/Forge 官方文档，不能证明工具链会自动生成满足商业审计的完整应用 SBOM。发布流水线应独立生成并归档：

- Electron、Chromium、Node 的精确版本和官方 checksum；Electron 安装器支持校验官方 `SHASUMS256.txt`，自建镜像也应保持校验。[Advanced Installation](https://www.electronjs.org/docs/latest/tutorial/installation)
- 锁文件解析出的 production npm 依赖、Forge/maker/updater 依赖；
- sidecar 及其 crates/modules、native DLL/driver/helper；
- 自有扩展版本与权限；
- 所有随包 notice、许可证、签名证书指纹和构建来源。

供应链最低门槛应包括精确版本锁定、依赖最小化、禁止未经审阅的 install/postinstall 行为、隔离签名密钥、可重建构建环境、产物 hash/签名验证以及更新 feed 的回放/降级防护。这些是商业控制要求，不应误写成 Electron 默认提供的能力。

## 商业适配度评分

分数是针对“外部 Chrome + Windows 11 首发 + 通用指纹浏览器控制台”的架构判断，不是通用框架排名，也不是运行基准。

| 维度 | 权重 | 纯 Electron/TS 核心 | Electron UI + 签名 sidecar | 判断 |
| --- | ---: | ---: | ---: | --- |
| 产品/UI 交付速度 | 20% | 9.0 | 8.5 | React/TS 工作台效率高；sidecar 协议增加少量前期成本 |
| 特权隔离与安全 | 20% | 5.5 | 8.0 | main + npm 直接拥有系统权力风险高；sidecar 可缩小 renderer/main 权限和故障域 |
| Windows 深层控制 | 15% | 6.0 | 8.5 | addon 可做但 ABI/崩溃/签名耦合；原生 sidecar 更适配 Job/WFP/ACL |
| 打包、签名、更新 | 10% | 8.0 | 7.5 | Forge 链成熟；sidecar 要整体签名、版本握手、原子更新 |
| 体积与空闲资源 | 10% | 5.0 | 4.5 | 都携带 Electron Chromium/Node，sidecar 再增一进程；未实测 |
| 版本与长期维护 | 10% | 6.0 | 6.5 | 八周 cadence 较重；sidecar 降低 native-addon ABI 耦合但增加双栈 |
| 测试、调试、可观测 | 5% | 8.0 | 8.0 | DevTools/inspector/Crashpad/Playwright 可用；真实 Chrome 与 sidecar 仍需独立测试 |
| 许可证、SBOM、供应链 | 10% | 6.0 | 6.5 | 依赖面大；sidecar 可减少 native npm，但 SBOM/notice 仍需自建 |
| **加权结果** | **100%** | **6.7/10** | **约 7.5/10** | sidecar 方案可进入最终技术栈决策；纯 Electron 核心不推荐 |

## 最终建议与决策门槛

**建议保留 Electron 作为候选，但只以“Electron + React/TypeScript UI + Rust sidecar”进入最终三栈比较。** 不要把 Electron 自带 Chromium误认为 managed browsing runtime，也不要为了保持“全 TypeScript”把网络 fail-closed、Profile 锁、秘密和 Chrome 监管塞进 main process。

Electron 是最佳选择的决定性优势：复杂商业工作台迭代快、web UI 能力完整、桌面表层集成充足、Forge/签名/更新/Crashpad 路径清楚。决定性风险：重复携带 Chromium、八周升级、Node/npm 与 IPC 攻击面、ASAR 不保护源码、深层 Windows 能力需要 addon/sidecar、sidecar 后失去单栈简洁性。

进入实现前只需做一个有明确退出条件的签名原型：

1. 一个本地 React 控制台、严格 sandbox/context isolation/CSP 和全部生产 fuses；
2. 一个最小 sidecar，实现 `start/stop/reconcile/testProxy`，不返回裸 CDP；
3. Windows Signed Squirrel 与 MSI 各一份，验证安装、提权 helper、更新、回滚和 sidecar 版本不匹配；
4. 测量安装/更新大小、启动/idle、20 个真实 Chrome 窗口下的 UI 响应；
5. 用恶意 renderer 输入证明无法获得文件、shell、秘密、任意 Chrome 参数或 sidecar 管道能力；
6. 杀死 Electron、sidecar、Chrome 与代理，逐一证明不会串 Profile、遗留无主进程或 fail-open 直连。

若原型达成安全与资源门槛，并且团队的 React 交付速度显著优于另外两个同等 UI 原型，Electron 就是合理的商业选择；若资源/更新载荷不可接受，或 sidecar 协议使 Electron 只剩一层昂贵 WebView 壳，则应选能直接复用原生核心且运行时更少的方案。
