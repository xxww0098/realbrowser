# AdsPower 实现对照与追上顺序

> 研究快照：2026-08-15。本文对照 **AdsPower 公开一手材料** 与 **RealBrowser 当前 MR-0 实现**。
>
> “AdsPower 有”只表示厂商文档/API/博客声明了该表面，**不是**源码可证、也不是防关联效果证明。产品范围与非目标仍以 [`PRODUCT.md`](../PRODUCT.md) 为准。领域词以 [`CONTEXT.md`](../CONTEXT.md) 为准。指纹机制见 [`docs/fingerprint-browser-principles.md`](fingerprint-browser-principles.md)。
>
> **2026-08-15 决策覆盖：** 早期“继续 Stock Chrome”的建议已失效。当前唯一运行时是产品发行、清单和 SHA-256 绑定、名称与 icon 均为 RealBrowser 的 Chromium；本轮只实施 K0 `persona.json` 与 K1 Canvas 2D，缺失或观测失败即 fail closed。下文 Stock Chrome 内容只保留为方案比较和历史研究。

---

## 1. 这篇文档要回答什么

用户要的是两件事：

1. **使用体验**：AdsPower 操作员每天点的那些东西，我们缺什么，按什么顺序补。
2. **自研浏览器**：SunBrowser / FlowerBrowser 到底是什么，我们要不要做、什么时候做、不做什么。

结论先写在前面：

- **追上体验**主要发生在控制平面：环境表、创建流、分组标签、代理库、批量、Cookie 可移植、扩展、窗口工作台。这些不需要自研内核。
- **当前一致性路径已经锁定：** RealBrowser 产品 Chromium 保持原生 TLS / HTTP/2，只在 Blink C++ 的 Canvas 2D 回读副本实施种子化 K1；WebGL、Audio、TLS 与 JS hook 不做。
- **内核范围必须保持窄。** K0+K1 有固定补丁、构建脚本和 frame/worker 观测；这不授权扩展到 AdsPower 声称的其他内核表面。

---

## 2. AdsPower 是什么（公开架构）

AdsPower 是闭源桌面客户端 + 可选云同步/团队 SaaS + 本机 Local API。操作员核心对象叫 **Profile（环境）**。公开结构可以还原成：

```text
AdsPower 控制面（桌面客户端 / 网页端 / Local API / RPA / 同步器）
        │
        ├── 每环境一份隔离缓存（Cookie、站点存储、书签……厂商称 siloed）
        ├── 一份 fingerprint_config（时区/语言/UA/Canvas/WebGL/……）
        ├── 一条绑定代理
        └── 一个内核进程
                ├── SunBrowser  = 厂商声称的 Chromium 定制构建
                └── FlowerBrowser = 厂商声称的 Firefox 定制构建
```

创建环境的官方五段表单：[Create a profile](https://help.adspower.com/docs/creating_browser_profiles)

| 段 | 操作员填什么 |
| --- | --- |
| General | 名称、Sun/Flower、模拟 OS、UA、分组、标签、Cookie 粘贴、备注 |
| Proxy | 自定义 `IP:port:user:pass`、已存代理库、随机分配、Check proxy |
| Platform | 平台账号（可填登录页自动打开并回填）、启动 URL；单环境最多 10 个平台账号 |
| Fingerprint | 默认自动生成；右侧 Overview；一键 New Fingerprint；Preferences 模板 |
| Advanced | 按环境装扩展、云同步、浏览器设置、付费的启动时 Random fingerprint |

三步创建路径被刻意压短：**新建 → 填代理并检测 → 打开**。指纹默认自动生成，不强迫操作员先懂 Canvas。

RealBrowser 今天对应的是：一张环境表 + 新建对话框（名称/启动 URL）+ Persona 抽屉 + 网络抽屉 + 归档。没有分组、标签、备注、代理库、Cookie 粘贴、扩展、批量、同步器。

---

## 3. 自研浏览器：SunBrowser / FlowerBrowser

### 3.1 厂商自己怎么说

一手材料一致的部分：

| 名称 | 声称基于 | 角色 |
| --- | --- | --- |
| **SunBrowser** | Chromium / Chrome | 主内核，跟官方大版本走 |
| **FlowerBrowser** | Firefox | 第二引擎，用来混指纹、吃 Firefox 生态 |

来源：[创建环境](https://help.adspower.com/docs/creating_browser_profiles)、[内核更新说明](https://www.adspower.com/blog/adspower-sunbrowser-kernel-version-update-chrome-127)、[一致性博客](https://www.adspower.com/blog/science-of-browser-consistency)、[内核级改指纹](https://www.adspower.com/blog/how-adspower-builds-browser-fingerprints)。

公开版本线（厂商月报，不是我们测过的二进制）：

| 时间 | SunBrowser | FlowerBrowser |
| --- | --- | --- |
| 2022-08 | Chrome 102 | 发布 |
| 2025-05 | Chrome 136 | — |
| 2026-05 | Chrome 148 | — |
| 2026-06 | Chrome 149 | Firefox 150 + Linux 模拟 |
| 2026-07 | Chrome 150（Win/mac/Linux） | — |

[July 2026](https://www.adspower.com/blog/what-is-new-in-adspower-browser-july-2026) [June 2026](https://www.adspower.com/blog/what-is-new-in-adspower-browser-june-2026)

FlowerBrowser **仍在维护**，但产品面更窄：扩展、缓存管理、Cookie Robot、同步器、Disabled UDP 都是 **SunBrowser only**。它不是“死内核”，对授权电商也几乎没有增量。

创建后 **不能换引擎、不能改模拟 OS**；降内核厂商自己也不推荐（丢数据/风控）。[创建页](https://help.adspower.com/docs/creating_browser_profiles)

Local API 用 `browser_kernel_config` 选内核：

```json
{ "type": "chrome", "version": "ua_auto" }
```

`type` 为 `chrome` 或 `firefox`；`version` 可以是具体大版本或 `ua_auto`（智能匹配已下载内核）。[fingerprint_config](https://localapi-doc-en.adspower.com/docs/Awy6Dg)

他们还提供内核下载/列表接口（`download-kernel`、`get-kernel-list`），说明内核是**客户端按需下载的独立二进制**，不是用户本机的 Google Chrome。

### 3.2 他们声称改在哪一层

[How AdsPower Builds Browser Fingerprints at the Kernel Level](https://www.adspower.com/blog/how-adspower-builds-browser-fingerprints) 把业界拆成三档，并自称第三档：

1. 配置级：UA、分辨率、语言、时区。
2. JS 注入：hook Canvas / WebGL / Audio，留原型痕迹。
3. **内核级：在 Chromium C++ 源码里改，编译进浏览器。运行时不注入脚本、不改原型。**

该文列出在内核改的面：Canvas、WebGL、GPU 参数、AudioContext、字体列表与栅格、CPU/内存、屏幕、ClientRects、**TLS/SSL 握手**。

换版本时“不是只换 UA，连内核一起换”，用来避免 UA 写 135、V8/TLS 却像 129。

[一致性博客](https://www.adspower.com/blog/science-of-browser-consistency) 补充：

- 2025 年宣称 14 次大版本内核更新（营销数字）。
- “不是套皮，是真正的浏览器进程。”
- 移动模拟（NMS）声称连 Canvas/WebGL/触摸/DPR/TLS 都按 Android/iOS 对齐。
- 强调 SOC 2 Type II、云加密——这是 SaaS 信任叙事，与内核实现无关。

### 3.3 公开材料证明不了什么

必须写死，避免把营销写成我们的技术债：

- **没有**源码、补丁集、构建脚本、SBOM。
- **没有** frame / worker / OffscreenCanvas 覆盖测试。
- **没有** “我们改了 Blink 的哪一个文件 / BoringSSL 的哪一次握手”。
- TLS 可编辑在 Local API 里只暴露为 `tls_switch` + 一组 **cipher suite 十六进制列表**，不是完整 JA3/JA4 编辑器。默认是关的。
- FlowerBrowser 仍有版本号，但扩展/同步器/缓存/Disabled UDP 都不给它。第二引擎对授权电商后台几乎没有增量。API 里 `browser_kernel_config.version` 示例仍停在 111 / Firefox 100，**文档滞后于 2026 真实内核线**。
- 分享页承认：噪声模式的环境换一台电脑会有“轻微差异”。说明他们的 Canvas/WebGL 噪声至少有一部分是 **相对主机 GPU 的滤镜**，不是跨机器可复现的纯种子 Persona。[Profile Sharing](https://help.adspower.com/docs/Profile_Sharing)
- “选 Chrome X 就跑 Chrome X”如果为真，**TLS/H2 一致性来自真内核，不是来自魔法**。用户已安装的 Stable Chrome 在这一点上同样成立——只是大版本不能像他们那样在 120/130/140 之间任选。

### 3.4 RealBrowser 怎么办

分三条路，不要混成一个“做内核”开关。

| 路线 | 做什么 | 得到什么 | 代价 | 建议 |
| --- | --- | --- | --- | --- |
| **A. 继续 Stock Chrome** | 发现并启动用户/企业已装的 Stable Chrome；一 Identity 一 User Data | 真 Chrome 的 TLS、H2、Canvas 真值、自动安全更新 | 无法提供本轮 Identity 级 Canvas K1，且重新引入本机浏览器所有权歧义 | **已拒绝；不得保留后备路径** |
| **B. Persona Runtime** | CDP 重放已支持的时区/屏幕原子组与运行观测 | 受支持字段可观测 Managed | 不能替代 Blink Canvas 的全 frame/worker 回读实现 | **保留既有受限职责，不做 JS 图形 hook** |
| **C. 产品 Chromium K0+K1** | 固定上游 tag、产品清单/哈希、K0 Persona 文件、Blink Canvas 2D K1 | 产品进程所有权与顶层/iframe/dedicated worker 一致的 Canvas 回读 | 需维护补丁、构建、签名和安全更新 | **当前已选；只限 K0+K1** |

更细的决策规则：

1. **当前只做 C 的 K0+K1。** MR-0 只接受带产品清单、哈希和观测证据的 RealBrowser Chromium。[`minimum-runnable-plan.md`](../.scratch/fingerprint-browser/implementation/minimum-runnable-plan.md)
2. **不要做 FlowerBrowser。** 电商卖家后台是 Chromium 世界。第二引擎只增加更新矩阵。
3. **不要做“假移动 UA”，也不要做云手机。** AdsPower 自己区分：它是浏览器环境，不是远程 Android（他们拿 DuoPlus 当对照）。授权桌面卖家工作流不需要 iPhone Persona。[cloud phone vs](https://www.adspower.com/blog/cloud-phone-vs-antidetect-browser)
4. **产品 Chromium 继续保持 UA major = 真内核 major，且本轮不改 TLS。** K1 只解决同机不同 Identity 的 Canvas 2D 回读差异。
5. 若将来必须选旧内核（某平台只认 Chrome 120），优先评估 **Chrome for Testing** 或官方版本化 Chromium 二进制，而不是从零维护 fork。CfT **无自动更新**，要自建补丁 SLA。[Chrome for Testing](https://developer.chrome.com/blog/chrome-for-testing)
6. 若将来做 C，必须先有：法律再分发结论、签名更新器、与上游差的补丁清单、每个声称表面的 frame/worker 观测套件。做不到就继续标 Native。

一句话：**当前内核只交付产品所有权、K0 Persona 和 K1 Canvas；其余体验走控制面，其余内核表面不顺带扩张。**

---

## 4. 使用体验对照（逐项追上）

图例：

- **已有**：MR-0 已在原生路径上做到，或已有诚实只读表面。
- **可追上**：RealBrowser 产品 Chromium + Rust 控制面能做，且不违反产品原则。
- **有限**：能做一部分，必须标能力/覆盖，不能写成 AdsPower 同款。
- **不做 / 后置**：违反不透明登录态、团队 SaaS、或需要自研内核才能诚实承诺。

### 4.1 环境工作台（操作员每天盯的那张表）

| # | AdsPower 体验 | 一手依据 | RealBrowser 现在 | 追上方式 | 优先级 |
| --- | --- | --- | --- | --- | --- |
| 1 | 环境表：打开/编辑/分组/标签/代理/平台账号一览 | 创建页 + 客户端主表 | 编号、名称、Profile、Persona、出口、RealBrowser、更新、启停 | 加备注、分组、标签列；操作仍是打开/停止/Persona/网络 | P0 |
| 2 | 搜索 + 筛选（分组、标签、运行中、平台） | 主表交互 | 搜名称/编号/网址；运行中过滤；无分组标签 | 分组/标签过滤，不引入永久左右栏 | P0 |
| 3 | 新建：三步（代理检测即可开） | [创建](https://help.adspower.com/docs/creating_browser_profiles) | 对话框：名称 + 启动 URL，其余进抽屉 | 新建可同时选出口；指纹保持默认 Native，不弹 20 个开关 | P0 |
| 4 | 快速批量创建 N 个环境 | 创建页 Quick Create | 无 | 从 Identity Template 批量；不复制 Cookie | P1 |
| 5 | 分组（文件夹语义） | 创建页 / API `group` | 无 | Rust 拥有 Group 聚合；表上筛选 | P0 |
| 6 | 多标签 | 创建页 | 无 | 多对多标签，短中文 | P0 |
| 7 | 备注 | 创建页 | 无 | 纯文本备注，不放秘密 | P0 |
| 8 | 回收站 / Trash 恢复 | [定价页 Restore in Trash](https://www.adspower.com/pricing) | **归档 + 恢复**已有 | 文案可更接近“回收站”，语义已够 | — |
| 9 | 一键 New Fingerprint | 创建页 Overview | 有稳定种子，无“换一套”按钮 | 仅对将来 Managed 面重建种子；Native 面不要假装换了 | P2 |
| 10 | Preferences 模板 | [Settings](https://help.adspower.com/docs/personal_settings) | 无 | Identity Template：语言/窗口/WebRTC/默认代理类型 | P1 |
| 11 | 批量改指纹/代理/分组 | 定价 Batch management | 无 | 选中行批量；CAS 修订；失败逐条 | P1 |
| 12 | 批量打开/关闭 | 主表 | 单个启停 | 批量启停 + 容量上限 | P1 |
| 13 | 编号稳定、可复制 | 客户端 | 已有 Rust 拥有的操作员编号 | 保持 | — |

### 4.2 代理与网络

| # | AdsPower 体验 | RealBrowser 现在 | 追上方式 | 优先级 |
| --- | --- | --- | --- | --- |
| 14 | 代理库（保存、标签、随机分配） | 每 Identity 内联 host:port，无库 | `ProxyLease` 库，Identity 绑定引用 | P0 |
| 15 | `host:port:user:pass` 一串粘贴 | 无凭据 | 凭据进 OS 密钥库，IPC 不回传 | P1 |
| 16 | Check proxy（出口 IP/地区） | 无连通性测试 | 启动前探测；失败拒绝启动 | P0 |
| 17 | HTTP / HTTPS / SOCKS5 / SSH | 已有前三种，无认证 | 补认证；SSH 隧道后置 | P1 |
| 18 | 时区/语言/地理“跟随 IP” | 时区可管；语言启动参数；地理不可用 | 探测出口后写入同一快照；不一致禁止保存 | P1 |
| 19 | WebRTC：Forward / Replace / Real / Disabled / Disabled UDP | 仅策略级 `disable_non_proxied_udp` 等 | 代理模式继续强制非代理 UDP；Replace/Forward 需内核或本地 STUN 中继，标有限 | P1 / 有限 |
| 20 | 禁 QUIC | 代理启动已编 `--disable-quic` | 保持；直连是否禁 QUIC单独产品决定 | — |
| 21 | DNS 防泄漏文案 | 无 OS fail-closed | 先文档诚实；WFP/pf 是商业 MVP 网络门禁 | P2 |

AdsPower WebRTC 枚举（API）：`forward`、`proxy`（Replace）、`local`（真实）、`disabled`、以及仅 Chrome 内核的 Disabled UDP。[fingerprint_config](https://localapi-doc-en.adspower.com/docs/Awy6Dg) 帮助中心还写 Forward = 强制 Google 公共 STUN。[指纹说明](https://help.adspower.com/docs/browser_fingerprint)

我们**不要**在 RealBrowser 上做未实现的 Replace 还显示“已应用”。能诚实交付的是：代理 + `disable_non_proxied_udp` + 观测 ICE。

### 4.3 Persona / 指纹字段

AdsPower Local API `fingerprint_config` 全表对照（[Awy6Dg](https://localapi-doc-en.adspower.com/docs/Awy6Dg)）：

| AdsPower 字段 | 默认 | RealBrowser 27 项目录 | 追上策略 |
| --- | --- | --- | --- |
| `automatic_timezone` / `timezone` | 跟 IP | **已配置 + 已观测**（顶层/新 tab） | 补“跟已验证出口” |
| `language_switch` / `language` | 跟 IP | `--lang` 已映射 | 补 `--accept-lang` + 跟出口 |
| `page_language` | 可跟 language | 无（UI 语言是产品中文） | 不追；显示语言不是指纹 |
| `ua` / `random_ua` | 随机 UA 库 | **Native**，不可编 | CDP UA-CH 原子组后再开放 Custom |
| `browser_kernel_config` | chrome + ua_auto | 验证产品 RealBrowser 清单/哈希/版本 | 不追多内核；只显示真实产品版本 |
| `screen_resolution` | 跟主机 / 随机 / 自定义 | 窗口启动参数；screen Native | CDP device metrics 后可管窗口+屏幕 |
| `webrtc` | disabled | 启动策略 | 见上 |
| `location*` | 跟 IP / 询问 | 能力目录标 CDP 不可用 | CDP geo + 权限 |
| `canvas` | 1 噪声 | K1 观测前 Native；通过后 CustomKernel | 只认顶层/iframe/dedicated-worker 真机矩阵 |
| `webgl_image` | 1 噪声 | Native | 同上 |
| `webgl` + `webgl_config` | 3 随机匹配 | Native | vendor/renderer 原子对；内核或有限 hook |
| `webgpu` | 跟 WebGL / 真 / 关 | Native | 跟 WebGL 元数据绑定 |
| `audio` | 1 噪声 | Native | Runtime |
| `client_rects` | 1 噪声 | Native | Runtime |
| `fonts` | `["all"]` | Native | 不提供假字体列表除非内核内置字体 |
| `hardware_concurrency` | 4 | Native | CDP experimental，验收后 |
| `device_memory` | 8 | Native | 无官方 CDP，保持 Native |
| `media_devices*` | 噪声 | Native | 不生成假设备表 |
| `speech_switch` | 替换 | Native | 跟 OS/语言 |
| `mac_address_config` | 匹配值 | 无 | **不追**。页面几乎读不到真实 MAC；伪造收益低 |
| `device_name*` | 遮罩 | 无 | 低优先级 |
| `scan_port_type` | 开 | 无 | 后置 |
| `do_not_track` | default | Native | Off/On，不随机 |
| `flash` | block | 无 | 不追（Flash 已死） |
| `gpu` 硬件加速 | 跟随本地设置 | 无 | 可作启动偏好，须提示影响硬件面 |
| `tls_switch` / `tls` | 关 | TLS 只读引擎事实 | **不做成可编辑 Persona** |

AdsPower 的产品技巧是：**默认全自动，高级才展开。** 我们追上体验时也应如此——新建环境不要把 27 个 Native 字段摊开当表单。主路径只暴露：名称、分组、代理、启动 URL、语言/时区/窗口。其余进 Persona 抽屉，不能编的就显示原生。

### 4.4 Cookie、账号、扩展

| # | AdsPower | RealBrowser | 决定 |
| --- | --- | --- | --- |
| 22 | 创建时粘贴 Cookie（JSON / Netscape / Name=Value，可 Merge） | Chrome 登录态不透明；**禁止读盘** | 允许**显式、单 Identity、预览+确认**的 Cookie **导入**（走 CDP/扩展 API，不读 Login Data 文件）；导出同权。密码/2FA 仍不做 |
| 23 | 平台账号 + 自动打开登录页并回填 | 仅启动 URL | 启动 URL 已有；**不托管网站密码**。平台只作标签/备注 |
| 24 | 环境分享携带 Cookie/密码/指纹 | 无 | 不做云分享 |
| 25 | 按环境自动装扩展分类 | 无 | 用户批准的扩展 allowlist；不静默强装 |
| 26 | 云同步缓存 / 多设备 | 本地 SQLite | 后置独立产品层 |

Cookie 可移植是授权运营的真实需求（换机、备份自己的店）。实现约束见产品原则：预览、审计、加密临时文件、用完即删；Rust 不解析 Chrome Cookie SQLite。

### 4.5 自动化与窗口

| # | AdsPower | RealBrowser | 决定 |
| --- | --- | --- | --- |
| 27 | 同步器：主控窗口鼠标/键盘/开标签广播到被控 | 无 | P2。必须显式勾选目标、紧急停止、审计。默认不全员广播 |
| 28 | 窗口排列 | 无 | P1。排列不等于同步 |
| 29 | RPA 无代码机器人 | 无 | 后置 |
| 30 | Local API：启停、CRUD、返回 `debug_port` / puppeteer ws / chromedriver | 无 | P2。经鉴权 + capability；**不把裸 CDP 交给任意本机调用者** |
| 31 | MCP Server | 无 | 后置 |
| 32 | 从 Multilogin / Dolphin / GoLogin 迁环境 | 无 | 后置；迁入也不得读对方密码库当卖点 |

AdsPower 打开浏览器后直接返回调试端口和 WebDriver 路径，且 `cdp_mask` **默认开启**（试图遮 CDP 探测）。[Open Browser](https://localapi-doc-en.adspower.com/docs/FFMFMf) 这对自动化很爽，对本产品是高权限泄漏面。追上“能接 Playwright”时，应由 Rust broker 签发短时 capability，而不是把 `ws://127.0.0.1:.../devtools/browser/...` 印在文档首页，更不要默认宣称“已隐藏自动化”。

### 4.6 团队 / 云 / 套餐

AdsPower 定价把批量、回收站、同步器、RPA、Local API 速率做成付费门。[Pricing](https://www.adspower.com/pricing)

这些是 **SaaS 商业层**，不是指纹技术。RealBrowser 当前是本地单操作员。团队权限、云缓存、跨机器 Profile 迁移单独开产品层，不进体验追上的前两波。

---

## 5. 体验差距的本质

AdsPower 让人觉得“能用”，不是因为操作员理解了 JA3，而是：

1. **表密度**：打开、代理、分组、标签、备注、平台都在一行里。
2. **创建短**：代理过了就能开；指纹有默认值。
3. **代理是一等公民**：库、检测、跟随 IP。
4. **批量是一等公民**：创建、改、开、关。
5. **内核被藏起来**：操作员选 Chrome 136，客户端自己下二进制。

RealBrowser 今天已经比 AdsPower **更硬**的部分：

- 一 Identity 一完整 User Data，源码可证。
- 能力目录诚实：不能编的就是 Native。
- 代理启动强制 WebRTC 非代理 UDP，否则拒绝。
- 崩溃调和不删 Chromium 锁、不接管陌生进程。
- Cookie 文件不读。
- 时区有观测，不是只写启动参数。

追上时要抄的是 **1–4 的信息架构**，不是 5 的闭源内核，更不是“不可检测”文案。

---

## 6. 推荐追上顺序（按操作员痛感）

每一波都必须：Rust 拥有真相、UI 只投影、能力标签诚实、Computer Use 能走出状态变化。

### 第 1 波：像一张能干活的环境表（纯控制面）

- 分组、标签、备注。
- 新建对话框带上出口（直连/已有代理）。
- 代理库 + 保存 + 绑定 + 启动前检测。
- 表列：分组、标签、出口摘要、备注。
- 搜索覆盖这些字段。

**不做什么：** 不把 WebGL、Audio、TLS 或 JS hook 顺带包装成已应用能力。

### 第 2 波：少点几次完成同样的事

- Identity Template（语言/窗口/WebRTC/默认代理类型）。
- 批量创建、批量启停、批量改分组/标签。
- 代理凭据进 OS 密钥库。
- 时区/语言“跟随已验证出口”。

### 第 3 波：店之间的合法搬运

- 显式 Cookie 导入/导出（预览、审计、单 Identity）。
- 扩展 allowlist（用户批准，按 Identity 启用）。
- 窗口排列（不同步输入）。

### 第 4 波：工作台加速

- 同步器：显式目标、紧急停止、审计日志。
- 能力受限 Local API（启停、列表、不回裸路径/Cookie）。

### 第 5 波：既有 Persona Runtime 的受限扩展

- UA + UA-CH 原子组（CDP，带观测）。
- 屏幕 / DPR / geo。
- 若卖家验收要求硬件面不同：按表面声明 coverage，先 Canvas 再 WebGL 元数据。测不过就保持 Native。

### 第 6 波：K0+K1 之外的内核扩展（独立立项，可拒绝）

触发条件（全部满足才开）：

1. 选定 Seller Platform 验收证明当前 RealBrowser K0+K1 的其他硬件面确实不够。
2. 法律允许分发 Chromium 构建。
3. 有签名更新与安全补丁 SLA。
4. 补丁集可审查，每个声称表面有 frame/worker 证据。

交付物如果做：仍只做 **Chromium 一族、跟 Stable 大版本**，不做 Firefox，不做移动内核。在此之前，UI 只写真实产品名和版本：“RealBrowser x.y”。

---

## 7. 明确不要为了对齐 AdsPower 而做的事

- 在 Stock Chrome 上提供 Canvas/WebGL“已应用噪声”。
- 可编辑 TLS / 任意 JA3。
- 假 iOS/Android UA 配桌面 GPU。
- 网站密码、2FA 托管、环境云分享带密码。
- 默认打开的裸 CDP / chromedriver 路径。
- 默认向全部窗口广播键鼠。
- FlowerBrowser。
- 把 SOC 2、14 次内核更新、NMS 写成我们的能力。
- 复制 AdsPower 品牌、独特视觉或“防关联/反检测”话术。商业参照只定信息架构和密度。

---

## 8. 和本仓库文档的关系

| 问题 | 家 |
| --- | --- |
| 产品做不做、怎么对外说 | [`PRODUCT.md`](../PRODUCT.md) |
| Identity / Profile / Persona / Egress | [`CONTEXT.md`](../CONTEXT.md) |
| 指纹机制与改法 | [`docs/fingerprint-browser-principles.md`](fingerprint-browser-principles.md) |
| MR-0 已交付与排除项 | [`.scratch/fingerprint-browser/implementation/minimum-runnable-plan.md`](../.scratch/fingerprint-browser/implementation/minimum-runnable-plan.md) |
| HubStudio 差距（同类对照） | [`.scratch/fingerprint-browser/research/hubstudio-capability-gap.md`](../.scratch/fingerprint-browser/research/hubstudio-capability-gap.md) |
| **AdsPower 体验对照与内核决策** | 本文 |

追上第 1 波不需要改 PRODUCT 非目标。第 3 波 Cookie 可移植、第 4 波 Local API 必须先改 PRODUCT 再写代码。第 6 波指的是 K0+K1 之外的扩展，必须先单独研究票，不能从本文直接开工。

---

## 9. 来源

AdsPower 一手：

- [Create a profile](https://help.adspower.com/docs/creating_browser_profiles)
- [What is browser fingerprint](https://help.adspower.com/docs/browser_fingerprint)
- [Preferences / Local settings](https://help.adspower.com/docs/personal_settings)
- [Synchronizer](https://help.adspower.com/docs/synchronizer)
- [Profile Sharing](https://help.adspower.com/docs/Profile_Sharing)
- [Local API fingerprint_config](https://localapi-doc-en.adspower.com/docs/Awy6Dg)
- [Local API Open Browser](https://localapi-doc-en.adspower.com/docs/FFMFMf)
- [How AdsPower Builds Browser Fingerprints at the Kernel Level](https://www.adspower.com/blog/how-adspower-builds-browser-fingerprints)
- [Browser Consistency & Kernel Mismatch](https://www.adspower.com/blog/science-of-browser-consistency)
- [SunBrowser kernel update (Chrome 136)](https://www.adspower.com/blog/adspower-sunbrowser-kernel-version-update-chrome-127)
- [Pricing](https://www.adspower.com/pricing)
- [Bulk Create](https://help.adspower.com/docs/Bulk-Create)
- [Trash](https://help.adspower.com/docs/trash)
- [Proxy list](https://help.adspower.com/docs/proxy_list)
- [Cache Data](https://help.adspower.com/docs/Cache-Data)
- [Extensions](https://help.adspower.com/docs/extensions)
- [July 2026 功能更新（Chrome 150）](https://www.adspower.com/blog/what-is-new-in-adspower-browser-july-2026)
- [June 2026 功能更新（Firefox 150）](https://www.adspower.com/blog/what-is-new-in-adspower-browser-june-2026)

本仓库：

- [`PRODUCT.md`](../PRODUCT.md)
- [`CONTEXT.md`](../CONTEXT.md)
- [`crates/browser-persona/src/lib.rs`](../crates/browser-persona/src/lib.rs)（27 项能力目录）
- [`.scratch/fingerprint-browser/implementation/minimum-runnable-plan.md`](../.scratch/fingerprint-browser/implementation/minimum-runnable-plan.md)
