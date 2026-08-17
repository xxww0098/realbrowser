# Windows Rust 控制平面使用 stock Chrome/Chromium 的支持边界

审阅日期：2026-08-13。本文只依据 Chrome/Chromium、Chrome Enterprise 和 Microsoft 官方资料形成静态架构约束；没有运行 Chrome、任何真实 Seller Platform 或代理，因此不包含生产验证声明。

## 决策摘要

可支持的 MVP 形态是：**Rust supervisor + 每个 Browser Identity 独占的非默认 User Data 目录 + stock Chrome/Chromium + 最小 MV3 扩展/Native Messaging + 小而版本探测的 CDP 子集 + Windows Job Object + 系统级出站封锁。**

控制平面可以可靠拥有 Profile 目录、进程树、启动/退出、代理租约、扩展消息和有限 CDP 操作；不能支持性地承诺“覆盖所有指纹面”、worker 全域注入、仅靠 Chrome 代理设置防直连、稳定的 tip-of-tree CDP、随意读取/迁移 Chrome secrets，或用普通未托管 Windows 强制静默安装扩展。

## 支持矩阵

| 缝 | 官方可证能力 | 必须接受的约束 | 产品决定 |
| --- | --- | --- | --- |
| User Data / Profile | `--user-data-dir` 是 Chromium 文档化覆盖入口；User Data 包含 cookies、history、bookmarks 和每安装 Local State，每个 profile 是其子目录。[Chromium User Data Directory](https://chromium.googlesource.com/chromium/src.git/+/HEAD/docs/user_data_dir.md) | 两个运行实例不能共享同一 User Data。Windows `ProcessSingleton` 对同目录建锁、通知既有进程，并在无法安全取得锁时返回 `PROFILE_IN_USE`，避免损坏。[ProcessSingleton](https://chromium.googlesource.com/chromium/src/+/master/chrome/browser/process_singleton_win.cc) | 一 Identity 一**完整 User Data 目录**，不是同一 User Data 下多个 `--profile-directory`；Rust 另加应用级排他锁，遇到 Profile-in-use 失败关闭，不接管/杀死未知 Chrome。 |
| 进程生命周期 | Chrome 是多进程；Windows Job Object 可把进程组作为单元管理，子进程默认继承 job，`JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE` 可在 supervisor 消失时清理整树。[Chromium 多进程架构](https://www.chromium.org/developers/design-documents/multi-process-architecture/) [Microsoft Job Objects](https://learn.microsoft.com/windows/win32/procthread/job-objects) | Job 可能嵌套或与既有 job 冲突；强杀不能代替优雅关闭。浏览器主进程退出也不是“数据已安全落盘”的业务证据。 | 用 suspended `CreateProcess` → assign Job → resume，正常路径请求 `Browser.close` 并等待，超时才终止 Job；落盘后 reconcile Profile/进程状态。具体 Chrome-in-job 兼容性列为 Windows 原型验证项。 |
| MV3 页面注入 | 静态 content script 可选 `document_start`，默认在 `ISOLATED` world；可显式 `all_frames`、`match_about_blank`、`match_origin_as_fallback`。静态脚本在同一生命周期阶段早于动态注册脚本。[Content scripts](https://developer.chrome.com/docs/extensions/develop/concepts/content-scripts) | `MAIN` world 与页面共享环境，页面可干扰且扩展 CSP 不保护该代码；URL match/host permission 与受限页面仍适用。所有 frame 必须逐类声明，不能从 top frame 推断覆盖。 | 默认 isolated world，只把确需影响页面主世界的极小 shim 放 MAIN；固定扩展版本、最小 host permissions、明确 frame 测试矩阵。`document_start` 是“该扩展的早期页面 hook”，不是在网络/解析/所有浏览器脚本之前的绝对保证。 |
| Service workers / workers | MV3 extension service worker 是事件驱动后台：通常空闲 30 秒终止，单事件约 5 分钟、`fetch()` 约 30 秒；全局变量会丢失，必须持久化状态。Native Messaging 长连接可延长其寿命。[Extension service-worker lifecycle](https://developer.chrome.com/docs/extensions/develop/concepts/service-workers/lifecycle) | Content scripts 运行于网页，不是 dedicated/shared/web service worker 的执行上下文；官方扩展 seam 没有“向任意站点 worker 注入 content script”的对称能力。 | 不把 Persona 正确性依赖于常驻 extension worker或内存状态；事件幂等、状态落 `chrome.storage`/Rust。MVP 不承诺覆盖站点 workers 中的 Canvas/WebGL/Audio/Navigator；这类要求若成为必要条件，必须单独研究而不能偷偷升级为 Chromium fork。 |
| Native Messaging | Chrome 用 manifest + 精确 `allowed_origins` 将扩展 ID 绑定原生 host；Windows 由 HKCU/HKLM registry 注册。协议是 length-prefixed UTF-8 JSON，host→Chrome 单条上限 1 MiB、Chrome→host 64 MiB；content script 必须经扩展页/service worker 转发。[Native Messaging](https://developer.chrome.com/docs/extensions/develop/concepts/native-messaging) | Chrome 会启动独立 host 进程；`sendNativeMessage` 每次起一个进程，`connectNative` 随 port 生命周期。扩展 ID、manifest 路径、stdout 协议和安装权限都是信任边界。 | 这是扩展↔Rust 的首选稳定 seam。固定扩展 ID，只列一个 origin；消息做 schema/version、size、nonce、Identity 授权；host stdout 只发协议，日志走 stderr。不要让网页消息直接成为原生命令。 |
| CDP | CDP 能 instrument/control Chromium；`--remote-debugging-port=0` 会把 endpoint 写入 Profile 的 `DevToolsActivePort`。但 tip-of-tree 会频繁变化且可随时破坏兼容，stable 1.3 只是 Chrome 64 的较小子集且没有整体向后兼容保证。[CDP](https://chromedevtools.github.io/devtools-protocol/) | 从 Chrome 136 起，普通 Chrome 对默认数据目录忽略 `--remote-debugging-port/pipe`；必须同时使用非默认 `--user-data-dir`。Google 明确指出 CDP 被用于窃取 cookies，并建议自动化用 Chrome for Testing。[Chrome 136 变更](https://developer.chrome.com/blog/remote-debugging-port)；企业 `RemoteDebuggingAllowed=false` 会完全禁止该 seam。[Policy](https://chromeenterprise.google/policies/remote-debugging-allowed/) | CDP 是高权限 capability，不是公开 Local API。优先 pipe；若用 port，选随机端口、只从 `DevToolsActivePort` 发现、绝不对 LAN 暴露，并由 Rust 独占。只封装版本探测过的 Target/Page/Browser 小子集，未知/Experimental 命令 fail closed。 |
| Chrome proxy | `chrome.proxy` 提供 direct/auto/PAC/fixed/system 模式；企业 `ProxySettings` 可配置 fixed/PAC、bypass，并用 `ProxyPacMandatory` 阻止 PAC 无效时回落直连，同时会让 Chrome 忽略命令行代理选项。[Extension proxy API](https://developer.chrome.com/docs/extensions/reference/api/proxy) [ProxySettings policy](https://chromeenterprise.google/policies/proxy-settings/) | policy、扩展、用户设置和命令行有优先级；ongoing requests 不一定即时切换。bypass、PAC `DIRECT`、其他协议和浏览器外进程都使“设置了 proxy”不等于“不会直连”。 | 每 Identity 固定连接本机 forward proxy；Rust 先鉴权/探测实际出口再启动。Windows Firewall/WFP 只允许该 Chrome Job 对本地代理与必要本地 IPC 通信，阻断直接公网，这是 fail-closed 的真正边界。 |
| 代理认证 | MV3 可用 `webRequest` + `webRequestAuthProvider` 在 `onAuthRequired` 同步/异步提供 HTTP auth；一般 MV3 不再拥有 `webRequestBlocking`，policy-installed 扩展例外。[webRequest](https://developer.chrome.com/docs/extensions/reference/api/webRequest) | 只能看到获 host permission 的可见请求；无效凭据会循环；它不是任意网络流量拦截器。 | 更优方案是在 Rust 本地代理消化上游 HTTP/SOCKS 凭据，Chrome 只连无秘密的 loopback endpoint。扩展 auth 仅作明确限定的兼容 fallback，带重试上限。 |
| DNS / WebRTC / QUIC | `DnsOverHttpsMode=automatic` 可回退不安全 DNS，`secure` 失败关闭但仍有 DoH server hostname 等特殊查询；`WebRtcIPHandling=disable_non_proxied_udp` 禁止非代理 UDP；`QuicAllowed=false` 禁 QUIC且需重启。[DoH policy](https://chromeenterprise.google/policies/dns-over-https-mode/) [WebRTC policy](https://chromeenterprise.google/policies/web-rtc-ip-handling/) [QUIC policy](https://chromeenterprise.google/policies/quic-allowed/) | 这些 policy 有 browser/profile scope 差异、动态应用差异，也不约束其他进程。WebRTC 的严格模式仍可能使用代理 UDP/TCP；它不是匿名性保证。 | 对受管 Chrome 设 strict WebRTC、禁 QUIC并选择明确 DNS 模式；但把 DNS/UDP/TCP 泄漏防护放在本地代理 + OS egress ACL，并用 DNS/WebRTC/QUIC 独立探针验收。仅静态 policy 不能宣布“无泄漏”。 |
| Windows secrets | Windows DPAPI 通常只允许同登录凭据、同机器解密；Chrome 说明 Windows 原先以 DPAPI 保护，Chrome 127 起以 App-Bound Encryption 先迁移 cookies，并绑定请求应用身份。[Microsoft DPAPI](https://learn.microsoft.com/windows/win32/api/dpapi/nf-dpapi-cryptprotectdata) [Chrome App-Bound Encryption](https://security.googleblog.com/2024/07/improving-security-of-chrome-cookies-on.html) | DPAPI 不防同用户恶意进程；App-Bound 的覆盖范围按 Chrome 版本演进。Chrome 136 又说明非默认 User Data 使用不同 encryption key。Profile schema、key和迁移不是控制平面 API。 | Profile 整体视为 Chrome 拥有的 opaque secret container：Rust 不查询 Cookie/Login Data、不复制、不降级 encryption、不传 `--password-store=basic` 一类非产品契约开关。Rust 自身代理秘密用 Windows Credential Manager/DPAPI，并仅存 reference。 |
| 扩展部署 / enterprise policy | `ExtensionInstallForcelist` 能静默、不可由用户关闭地安装，并隐式授予权限；Windows 外部 Web Store 扩展只有 AD/Azure AD joined 或 Chrome Enterprise Core enrolled 时才可强装。[ExtensionInstallForcelist](https://chromeenterprise.google/policies/extension-install-forcelist/) | policy 可能由组织管理员禁止 remote debugging、覆盖 proxy；部分是 browser-level、需重启或不能由 Cloud user policy 设置。应用不能假装自己拥有机器 policy。 | 支持两种明确模式：自用机器由用户安装签名 Web Store 扩展；受管企业机器由管理员部署 policy。启动时读取 `chrome://policy` 等效状态/可观测结果，发现冲突就解释并拒绝降级，不改写组织 policy。 |
| 浏览器分发与更新 | Regular Chrome 自动更新；Chrome for Testing 是面向自动化/测试、仅可信内容、**无自动更新**的版本化产物。[Chrome for Testing](https://developer.chrome.com/blog/chrome-for-testing) | CfT 不应被误当长期卖家日常浏览器；固定二进制意味着应用承担安全更新时限。公开 Chromium 也不同于 Chrome。Google Chrome 的商业再分发权没有被本次技术材料证明。 | MVP 优先发现用户/管理员安装的 Stable Chrome，由 Google/企业更新系统拥有 binary；Rust 在启动前做最小/最大兼容门槛并拒绝过旧版本。若未来内嵌 Chromium/CfT，另开法律、签名、SBOM、codec、patch SLA 和 updater 研究票。 |

## 由此形成的控制契约

1. **Identity 原子边界**：UUID、绝对 User Data path、应用锁、Chrome binary/version、Job、proxy lease、扩展 ID/version 和 CDP lease 必须同属一个聚合；任一不一致便不启动。
2. **启动状态机**：解析并验证 Chrome → 锁 Profile → 建本地代理与 OS egress ACL → 探测出口/DNS → suspended launch + Job → 等待 Native Messaging/CDP readiness → 校验版本与扩展 → 才显示“可用”。任何失败都清理 Job/lease，绝不回落 default Profile、direct/system proxy 或无扩展模式。
3. **关闭与恢复**：先撤销新工作和 CDP lease，优雅关闭 Chrome，等待进程树/文件句柄退出，再撤代理/ACL/锁。异常重启时从 OS 进程、Job/lease journal 和 Profile lock 三方 reconcile；不通过删除 Chromium lock file“修复”。
4. **扩展职责最小化**：页面级提示、必要的站点集成和 Native Messaging；不保存网站凭据、不导出 cookies、不充当网络防火墙、不宣称 worker/所有 frame 全覆盖。
5. **CDP 职责最小化**：健康检查、窗口/页面生命周期和显式授权的人工辅助；不提供原始 endpoint 给任意本机调用者，不以 CDP 读取 cookies/storage。
6. **网络强属性在 OS 层**：Chrome proxy/policy 是配置层，Windows egress ACL + 本地 proxy 才是防直连 enforcement。验收必须分别覆盖 TCP、UDP/QUIC、DNS/DoH、WebRTC、代理认证失败和代理运行中断线。

## 明确不支持或不稳定

- tip-of-tree/Experimental CDP 作为稳定产品 API；依赖固定调试端口或公开 endpoint。
- 多个 Chrome 实例共享同一 User Data，或用多个 `profile-directory` 代替进程级隔离。
- `MAIN` world 大面积 monkey-patch、对 web workers 的“全覆盖”承诺、把 extension service worker 当常驻 daemon。
- 仅靠 `--proxy-server`、PAC 或扩展 proxy 声称 fail closed；代理失败自动直连。
- 读取、编辑、迁移或导出 Chrome Cookie/Login Data；用非文档化开关削弱 secret storage。
- 未受管 Windows 上静默强装外部扩展，或覆盖组织已有 enterprise policy。
- 将 Chrome for Testing/固定 Chromium 当作无需持续安全更新的生产浏览器。

## 需要原型或后续研究的缺口

1. Windows 11 上 stock Stable Chrome 加入 nested Job Object、优雅关闭与崩溃恢复的实际行为。
2. 所选 Chrome 版本的 remote-debugging-pipe Rust 客户端兼容性，以及 policy 禁用时的可诊断错误。
3. 签名扩展在所选 Seller Platform 的 CSP、跨域 frame、`about:blank`/`blob:`、站点 service worker 和登录跳转矩阵；不能由文档推导兼容。
4. Windows Firewall/WFP 能否精确约束整个 Chrome Job 且允许 loopback proxy、更新器与 DNS 的最小规则；需以抓包证明。
5. 自定义 User Data 下 Chrome App-Bound Encryption、Profile 备份/恢复和 Windows roaming 的具体支持范围。
6. Google Chrome 再分发/商标许可；若不使用用户已安装 Chrome，必须法律复核并单独决定更新所有权。
