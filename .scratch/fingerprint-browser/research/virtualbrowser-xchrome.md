# VirtualBrowser 与 XChrome：面向授权电商多店铺隔离浏览器的源码审阅

审阅日期：2026-08-13

## 结论

两者都不适合作为本产品的代码或发行物基础。

- **VirtualBrowser** 更接近完整产品：独立持久化环境、批量启动、配置表面、Playwright/CDP 示例和持续发行均已存在；但公开仓库只有 Vue 管理界面、欢迎页和自动化示例，没有 Chromium 修改、Native `chrome.send` handler、Local API server 或可复现发行链。其最严重的静态问题是管理 UI 从明文 HTTP 动态加载 JavaScript；同一 UI 又能接触 Cookie、代理口令和 Native bridge。它只能作为“产品能力清单”的参考，不能作为可审计信任根。
- **XChrome** 是 Windows WPF 控制平面，运行用户指定的 Chrome（或包内 Playwright Chromium），用独立 `--user-data-dir`、CDP、启动参数和 Win32 输入复制做多开与群控。这个方向比维护 Chromium fork 更接近本项目的推荐架构，但当前实现存在 Persona 内部不一致、代理错误时直连、明文配置、不可恢复删除、外部登录/更新依赖和无测试等问题；其 CC BY-NC 4.0 许可证也禁止商业使用。
- 对 RealBrowser 应采用的边界是：**Rust 拥有身份生命周期、进程、固定网络出口、密钥、审计与本机控制 API；官方 Chrome/Chromium 拥有网页运行时；Profile 目录是会话边界；Persona 只做最小、稳定、可解释且成组一致的设置。** 不导入随机化、批量养号或规避检测逻辑。

## 固定快照与证据等级

| 项目 | 固定版本 | 仓库所含内容 | 本次证据等级 |
| --- | --- | --- | --- |
| VirtualBrowser | [`d47736b5d66fc5f641b57f56df2942aa9162d7e8`](https://github.com/Virtual-Browser/VirtualBrowser/commit/d47736b5d66fc5f641b57f56df2942aa9162d7e8)，提交日期 2026-05-18 | `server/` Vue 2 管理 UI、`worker/` Vue 3 欢迎页、`automation/` 示例；没有浏览器内核、Native handler 或安装器构建源码 | 源码静态证明 + GitHub 一方发布资料；未运行 Windows/macOS 二进制 |
| XChrome | [`0c2e210a04d1937ad14ca54f41b2b215c73a6fa9`](https://github.com/chanawudi/XChrome/commit/0c2e210a04d1937ad14ca54f41b2b215c73a6fa9)，提交日期 2025-04-03 | .NET 8/WPF 控制平面、SQLite、CDP/Playwright 路径、Win32 群控和本地代理转接；没有 Chrome/Chromium 内核源码 | 源码静态证明；未在 Windows 编译或运行 |

“已实现”在本文中只表示固定提交里存在可达代码路径，不等同于生产运行证明。README、截图和更新日志中的能力若缺少相应实现，均标为“声称”。

## 对比矩阵

| 维度 | VirtualBrowser | XChrome | 对本产品的决定 |
| --- | --- | --- | --- |
| 引擎所有权 | README 称基于 Chromium，自动化实际启动专用 `VirtualBrowser.exe`；公开仓库没有内核或 Native bridge 实现，无法复现发布物。[README](https://github.com/Virtual-Browser/VirtualBrowser/blob/d47736b5d66fc5f641b57f56df2942aa9162d7e8/README_EN.md#L7-L19) [自动化入口](https://github.com/Virtual-Browser/VirtualBrowser/blob/d47736b5d66fc5f641b57f56df2942aa9162d7e8/automation/index.js#L7-L15) | README 称使用本机 Chrome；当前默认路径要求用户配置 Chrome，代码也保留包内 Playwright Chromium fallback。控制层拥有进程和 CDP，不拥有内核。[README](https://github.com/chanawudi/XChrome/blob/0c2e210a04d1937ad14ca54f41b2b215c73a6fa9/README.md#L4-L11) [引擎选择](https://github.com/chanawudi/XChrome/blob/0c2e210a04d1937ad14ca54f41b2b215c73a6fa9/cs/zchrome/ZChromeManager.cs#L277-L288) | 采用 Rust 控制平面 + 受支持 Chrome/Chromium；不 fork 内核作为 MVP 前提。 |
| Profile 隔离 | 示例把 `workerId` 映射到 `%LOCALAPPDATA%\\VirtualBrowser\\Workers\\<id>` 的 persistent context，并传 `--worker-id`；Native 是否严格隔离、删除哪些目录无法由公开源码确认。[示例](https://github.com/Virtual-Browser/VirtualBrowser/blob/d47736b5d66fc5f641b57f56df2942aa9162d7e8/automation/README.md#L15-L30) | 每条数据库记录映射到默认 `chrome_data/<id>` 或任意自定义目录，启动时传独立 `--user-data-dir`；这是明确可读的隔离机制。[映射](https://github.com/chanawudi/XChrome/blob/0c2e210a04d1937ad14ca54f41b2b215c73a6fa9/pages/CManager.xaml.cs#L243-L272) [启动参数](https://github.com/chanawudi/XChrome/blob/0c2e210a04d1937ad14ca54f41b2b215c73a6fa9/cs/zchrome/ZChromeClient.cs#L215-L240) | 一身份一目录、一进程树、一固定出口；启动前验证目录不重用，运行时加锁，删除进回收站。 |
| Browser Persona | UI 会在创建时生成并持久化 UA、语言/时区、地理位置、Canvas/WebGL/Audio 等大量随机字段，但实际应用这些字段的内核代码不公开，覆盖面、一致性和 worker/frame 行为不可证。[默认配置](https://github.com/Virtual-Browser/VirtualBrowser/blob/d47736b5d66fc5f641b57f56df2942aa9162d7e8/server/src/views/browser/index.vue#L1217-L1340) | 可达路径用 CDP 为新 target 应用时区、触摸、地理位置、UA、headers 和 document-start 脚本；语言脚本直接覆盖 `navigator`。但随机 UA 生成 108–119，可能与真实 Chrome 不符；分辨率函数只发送 `mobile`，且配置构造未填宽高，README 的“分辨率”能力未由当前路径证明。[随机生成](https://github.com/chanawudi/XChrome/blob/0c2e210a04d1937ad14ca54f41b2b215c73a6fa9/api/XChrome.cs#L69-L150) [target 应用](https://github.com/chanawudi/XChrome/blob/0c2e210a04d1937ad14ca54f41b2b215c73a6fa9/cs/zchrome/ZChromeClient.cs#L711-L745) [设备指标缺口](https://github.com/chanawudi/XChrome/blob/0c2e210a04d1937ad14ca54f41b2b215c73a6fa9/cs/zchrome/ZChromeClient.cs#L497-L508) | 不随机拼装“看起来像别的设备”的字段。只允许由真实引擎能力派生、跨重启稳定、可做一致性断言的 Persona；缺少一致性证明就保持默认。 |
| 代理与防直连 | 支持默认、无代理、自定义 HTTP/SOCKS 和代理 API；配置、测试与启动是分离的。启动时代理 API 刷新没有 `await`，随后立即 `launchBrowser`，因此旧出口/直连竞态在源码上成立。[配置序列化](https://github.com/Virtual-Browser/VirtualBrowser/blob/d47736b5d66fc5f641b57f56df2942aa9162d7e8/server/src/views/browser/index.vue#L1466-L1479) [启动竞态](https://github.com/Virtual-Browser/VirtualBrowser/blob/d47736b5d66fc5f641b57f56df2942aa9162d7e8/server/src/views/browser/index.vue#L1542-L1547) | 无认证 HTTP 直接用 Chrome 参数；SOCKS5 或带认证代理经本地转接。解析失败只弹错误，仍继续无代理参数启动，明确 fail-open。[代理构建](https://github.com/chanawudi/XChrome/blob/0c2e210a04d1937ad14ca54f41b2b215c73a6fa9/cs/zchrome/ZChromeClient.cs#L181-L211) | `resolve -> authenticate -> probe actual egress -> launch` 必须是同一失败即终止的状态机；运行中代理死亡要停网/停 Profile，绝不能回落直连。 |
| 分组与群控 | 有组织分组、筛选、批量创建/启动/删除；分组只保存在管理 UI `localStorage`，未见同步鼠标/键盘群控。[分组存储](https://github.com/Virtual-Browser/VirtualBrowser/blob/d47736b5d66fc5f641b57f56df2942aa9162d7e8/server/src/api/native.js#L132-L165) [批量启动](https://github.com/Virtual-Browser/VirtualBrowser/blob/d47736b5d66fc5f641b57f56df2942aa9162d7e8/server/src/views/browser/index.vue#L1197-L1215) | 可选择一个正在运行的主控，并将点击、滚轮、按键和鼠标移动复制给**所有**其他运行实例；数据库“分组”不限制群控目标。[选择主控](https://github.com/chanawudi/XChrome/blob/0c2e210a04d1937ad14ca54f41b2b215c73a6fa9/pages/CManager.xaml.cs#L724-L761) [全运行实例扇出](https://github.com/chanawudi/XChrome/blob/0c2e210a04d1937ad14ca54f41b2b215c73a6fa9/cs/zchrome/ManagerControler.cs#L24-L72) | MVP 只做显式选择后的批量启动/停止/排列；不复制网页输入。以后若做运营批处理，应是有审计、逐店确认的高层动作。 |
| 本地/远程 API | 示例向 `http://localhost:9000/api/launchBrowser` POST id，再拿 debugging port 通过 CDP 连接；示例没有鉴权。Local API server 实现不在仓库，无法证明只绑定 loopback、鉴权、授权或会话回收。[Node 示例](https://github.com/Virtual-Browser/VirtualBrowser/blob/d47736b5d66fc5f641b57f56df2942aa9162d7e8/automation/test-api.js#L3-L33) | `api/XChrome.cs` 只是进程内批量创建 helper；README 仍把脚本和 API 标成“开发中”。可达网络 listener 是欢迎页，不是产品 API。[helper](https://github.com/chanawudi/XChrome/blob/0c2e210a04d1937ad14ca54f41b2b215c73a6fa9/api/XChrome.cs#L15-L67) [欢迎页 listener](https://github.com/chanawudi/XChrome/blob/0c2e210a04d1937ad14ca54f41b2b215c73a6fa9/cs/zchrome/WelComePage.cs#L13-L69) | 本机 API 默认关闭；启用时只绑定 loopback/本地 IPC，随机启动密钥、最小方法集、Profile 级授权、速率限制和可撤销 CDP lease。 |
| Cookie、口令与会话边界 | UI 明确支持 Cookie 导入；整条环境可导出为 JSON，因此 Cookie、代理用户名/密码和代理 API URL会进入明文文件。浏览器列表还进入 `localStorage`，bridge 调用参数会写 console。[Cookie 解析](https://github.com/Virtual-Browser/VirtualBrowser/blob/d47736b5d66fc5f641b57f56df2942aa9162d7e8/server/src/views/browser/index.vue#L810-L867) [导出](https://github.com/Virtual-Browser/VirtualBrowser/blob/d47736b5d66fc5f641b57f56df2942aa9162d7e8/server/src/views/browser/index.vue#L1570-L1616) [存储与日志](https://github.com/Virtual-Browser/VirtualBrowser/blob/d47736b5d66fc5f641b57f56df2942aa9162d7e8/server/src/api/native.js#L15-L32) | 网站会话由 Profile 保存；数据库 schema 有 `cookie` 字段但可达写入均置空。代理完整字符串和 Persona 明文保存在工作目录 SQLite；发行构建的“登录码”也明文落盘并作为 query 参数发给作者服务。[SQLite](https://github.com/chanawudi/XChrome/blob/0c2e210a04d1937ad14ca54f41b2b215c73a6fa9/cs/db/MyDb.cs#L21-L38) [登录码](https://github.com/chanawudi/XChrome/blob/0c2e210a04d1937ad14ca54f41b2b215c73a6fa9/forms/Login.xaml.cs#L100-L138) | 应用不读写/导入/导出网站 Cookie 或密码。代理口令进 OS keychain；SQLite 只存 secret reference。Profile 目录以 OS ACL 保护，日志严格脱敏。 |
| 持久化与删除 | 管理配置由 Native store + `localStorage` 双写；删除先调用缺失的 Native handler，再删列表，因此目录删除与异常恢复不可审计。[生命周期 bridge](https://github.com/Virtual-Browser/VirtualBrowser/blob/d47736b5d66fc5f641b57f56df2942aa9162d7e8/server/src/api/native.js#L68-L119) | SQLite 元数据 + Profile 目录。删除直接删数据库记录和默认目录且明确不可恢复；若身份使用自定义 `datapath`，删除代码仍只看默认目录，留下失去索引的敏感 Profile。[删除](https://github.com/chanawudi/XChrome/blob/0c2e210a04d1937ad14ca54f41b2b215c73a6fa9/pages/CManager.xaml.cs#L650-L706) | 两阶段归档/可恢复删除；元数据与目录绑定必须事务化并带稳定 UUID；异常退出后先 reconcile，再允许启动。 |
| 更新与发行信任 | GitHub 有持续安装器发布，最新可见 2.3.1 于 2026-07-30 发布；但 2.3.0/2.3.1 tag 都只是同一个源码 commit，仓库无安装器构建链。更严重的是 UI 通过 **HTTP** 注入远端更新脚本，而该页能访问 secrets 与 Native bridge。[Releases](https://github.com/Virtual-Browser/VirtualBrowser/releases/tag/2.3.1) [HTTP 更新脚本](https://github.com/Virtual-Browser/VirtualBrowser/blob/d47736b5d66fc5f641b57f56df2942aa9162d7e8/server/src/views/browser/index.vue#L1140-L1169) [动态注入](https://github.com/Virtual-Browser/VirtualBrowser/blob/d47736b5d66fc5f641b57f56df2942aa9162d7e8/server/src/utils/index.js#L439-L442) | 启动时从作者域名 HTTPS XML 交给 AutoUpdater.NET；源码未给签名/哈希固定策略。GitHub 没有 Releases/tags，README 让用户从官网取完整包；源与二进制无法建立可复现对应关系。[更新调用](https://github.com/chanawudi/XChrome/blob/0c2e210a04d1937ad14ca54f41b2b215c73a6fa9/MainWindow.xaml.cs#L138-L180) [安装说明](https://github.com/chanawudi/XChrome/blob/0c2e210a04d1937ad14ca54f41b2b215c73a6fa9/README.md#L29-L43) | 可复现构建、签名清单、HTTPS + 签名验证、降级防护、人工批准更新；管理 UI 永不执行远端脚本。 |
| 测试、发布、维护 | 三个 package manifest 只有 build/lint 或运行 demo；`test-api.*` 是示例，不含断言。源码提交稀疏，但安装器在 2025–2026 持续发布，说明存在仓库外开发/构建面。[automation scripts](https://github.com/Virtual-Browser/VirtualBrowser/blob/d47736b5d66fc5f641b57f56df2942aa9162d7e8/automation/package.json#L1-L14) [发布历史](https://github.com/Virtual-Browser/VirtualBrowser/releases) | 无 test project/CI；名为 `TestAndGoAsync` 的可达函数立即返回 true。31 个提交集中在 2025-02 至 2025-04，此后固定分支无源码提交，也无 GitHub Releases。[伪测试](https://github.com/chanawudi/XChrome/blob/0c2e210a04d1937ad14ca54f41b2b215c73a6fa9/cs/Test.cs#L17-L42) [提交历史](https://github.com/chanawudi/XChrome/commits/0c2e210a04d1937ad14ca54f41b2b215c73a6fa9/) | 隔离、出口、恢复、秘密和真实 Seller Platform 旅程都必须有自动化/人工证据；不能用 README 或指纹检测站截图代替。 |
| 许可证 | 根仓库为 BSD-3-Clause，可宽松使用 UI 源码；但未公开的引擎/安装器实现不因这个文件自动变得可复用。[LICENSE](https://github.com/Virtual-Browser/VirtualBrowser/blob/d47736b5d66fc5f641b57f56df2942aa9162d7e8/LICENSE#L1-L27) | CC BY-NC 4.0，要求署名并禁止商业用途；不适合商业或未来可能商业化产品直接复用。[LICENSE](https://github.com/chanawudi/XChrome/blob/0c2e210a04d1937ad14ca54f41b2b215c73a6fa9/LICENSE#L1-L19) | 只学习架构事实；MVP 自行实现。引入任何第三方代码前做逐文件许可证清单。 |

## 关键判断

### 1. “开源指纹浏览器”不等于浏览器信任根开源

VirtualBrowser 的核心 Persona 应用和 Profile Native handler 不在仓库；XChrome 虽公开控制层，却依赖用户 Chrome、包内 Chromium、CDP 和生成扩展。两者都无法从固定源码构建出用户下载的完整可信运行时。因此不能基于 star、截图或“开源”字样推断其不读取凭据，也不能把安装器放进 Store Account 信任边界。

### 2. Profile 目录隔离是正确的底座，但不是完整安全属性

两个项目都把身份映射到独立持久化目录，这个模式值得保留。产品规格还必须补足：目录唯一性、并发锁、进程树归属、代理绑定、OS ACL、扩展白名单、崩溃恢复、备份/删除策略以及“应用层永不打开 Cookie 数据库”的可测试约束。仅仅改变 User Data 路径无法证明出口、扩展、下载目录、密钥或日志不串线。

### 3. Persona 越广，内部矛盾面越大

VirtualBrowser 暴露大量随机字段但核心实现不可审计；XChrome 已显示典型矛盾：随机 UA 可落后真实引擎十多个主版本，移动/触摸/分辨率字段并未形成一个完整设备模型，语言由 JS 覆盖而 HTTP headers、CDP locale 和系统字体又可能不同。对授权卖家业务，更稳妥的目标是“每个身份稳定、可解释、兼容”，不是“变化越多越好”。

### 4. 出口必须是强绑定，不是一个可选表单字段

VirtualBrowser 的异步刷新竞态和 XChrome 的解析失败直连都违反首版硬门槛。Rust supervisor 应在 Chrome 进程创建前拿到已验证的 egress lease；没有 lease 就没有进程。Chrome 还应受 OS 防火墙/本地转发层约束，使应用 bug 也无法绕过固定出口。

### 5. Local API 与 CDP 是高权限控制面

VirtualBrowser 示例直接返回调试端口，XChrome 为每个实例打开 remote debugging port。两者均未给出足够的调用者认证、租约或审计证据。RealBrowser 不应把裸 CDP endpoint 当产品 API；Rust 应代理有限动作，短期租出调试能力时绑定 Profile、调用者、时限和撤销事件。

## 可安全吸收与明确拒绝

可吸收的模式：

1. 稳定 UUID 对应独立 Chrome User Data 目录。
2. Rust supervisor 显式拥有启动、停止、运行状态和异常恢复。
3. 管理 UI 支持创建、重命名、归档、代理绑定、批量启动/停止和窗口排列。
4. 自动化通过本机受鉴权控制面取得短期、Profile 级 capability；底层可使用 CDP，但不默认暴露。
5. Persona schema 版本化，并把真实引擎版本、locale/timezone/viewport 等一致性检查作为启动 gate。

明确拒绝：

1. 随机拼接 UA、设备、Canvas/WebGL/Audio 等字段，或把检测站得分作为成功标准。
2. Cookie 导入导出、Profile 克隆、明文代理密码、远端脚本、裸调试端口。
3. 代理失败后回落直连。
4. 将鼠标/键盘输入广播给所有店铺窗口；这既容易误操作，也不符合“逐店授权、可审计”的运营模型。
5. 采用任一项目的预编译浏览器或更新服务作为信任根。

## 后续票据应携带的约束

- 浏览器集成架构：优先官方 Chrome/Chromium + Rust supervisor；只有经研究证明 CDP/企业策略/受控扩展无法满足**必要且合规**的能力时，才重新评估 fork。
- Browser Identity：Profile、proxy lease、Persona version、extension policy、进程树和 secret refs 必须作为一个不可拆分聚合管理。
- 网络出口：启动和运行时均 fail closed；通过真实出口探针验证，不只检查 TCP 可达。
- 本地控制 API：默认关闭、loopback/IPC、每次启动随机凭证、方法白名单、Profile 级授权和完整审计。
- 更新：签名发行物、签名 manifest、回滚/降级保护、明确人工批准；UI 不执行任何远端 JavaScript。
- 验收：除 hermetic tests 外，必须在 Windows 11 上用获授权的 Store Account 完成选定 Seller Platform 的真实人工路径，并分别记录“代码可证”与“运行已证”。

## 未完成的运行验证

本次没有下载或执行任一第三方安装器，也没有在 Windows 上启动 Profile、抓包、探测 CDP bind address、检查 Chrome credential encryption、验证任何 Seller Platform 兼容性或复现 20 窗口容量。VirtualBrowser 缺失核心源码，故即使运行也只能证明特定二进制的观测行为，不能建立源到产物的信任；XChrome 固定提交也没有足够的测试/发行材料建立生产声明。上述项目方功能表均应继续视为声明，直到针对固定二进制完成独立动态验证。
