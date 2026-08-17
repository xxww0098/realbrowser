# HubStudio 公开能力面与 RealBrowser 可行性差距

审阅日期：2026-08-15。入口为用户给出的官方注册页，但未使用推荐参数，未注册、登录、下载或运行客户端。本报告只核对 HubStudio 官网、官方帮助中心和官方 API 文档；“支持”表示厂商公开声明或接口表面存在，不表示源码可证、运行已证或具备防关联/反检测效果。

## 结论

**可以做出具备常规指纹浏览器功能、并适合授权电商运营的产品，但不应把“完整复刻 HubStudio”当作当前 MVP。** RealBrowser 现有路线（Windows 11、Rust 控制平面、stock Chrome、每身份独立 User Data、本地单用户）足以覆盖环境列表、Persona、代理库、Cookie 可移植性、扩展、批量启停、窗口工作台和受控 Local API/自动化。它还可以把 HubStudio 公开材料没有证明的网络 fail-closed、秘密不落日志、崩溃恢复做得更强。

完整对齐 HubStudio 仍会增加三个独立产品层：团队权限与云端缓存构成 SaaS；跨团队环境迁移和远程自动化构成托管控制面；自有版本化内核构成浏览器发行。Cookie 可移植性、本地自动化和窗口同步现在属于标准功能范围，但必须重新实现为显式授权、可审计且不暴露裸浏览器权限的本地能力。

## 官方公开能力面（声明不等于实现证明）

| 能力 | 官方当前公开表面 | 可验证性与 RealBrowser 差距 |
| --- | --- | --- |
| 环境/Profile | 官网称每个账号拥有独立环境、支持同机多账号和“指纹不变”的环境转移；帮助文档支持批量创建、打开、关闭、分组、筛选和回收站恢复。[产品页](https://www.hubstudio.cn/index.html) [环境管理](https://support-orig.hubstudio.cn/645e/cf62) [批量操作](https://support-orig.hubstudio.cn/645e/2751) | 公开页未披露 User Data 路径、锁、进程树和隔离测试，故只能记录为产品声明。RealBrowser 已明确一身份一完整 User Data、一进程树、一出口，能用源码和 Windows 测试证明。 |
| 指纹/Persona | 官方列出 UA、语言、时区、地理位置、分辨率、WebRTC、字体、Canvas、WebGL/WebGPU、AudioContext、语音、媒体设备、CPU、内存、DNT、电池、端口扫描等配置。[指纹说明](https://support-orig.hubstudio.cn/4632/6300) | 文档描述配置选项，没有公开注入位置、worker/frame 覆盖、跨 API 一致性或测试。不能由检测页截图或字段数量推出有效性。RealBrowser 可先支持真实引擎派生且可一致性验证的最小 Persona；全面运行时改写不属于 stock Chrome 可支持承诺。 |
| 代理/网络 | 用户自备代理；支持 SOCKS5、HTTP、HTTPS、SSH、IPv4/IPv6、账号口令、自定义静态代理或代理 API 提取、代理检测和 IP 变化提醒；也明确允许“不使用代理”直连。[新建环境](https://support-orig.hubstudio.cn/645e/2879) [代理说明](https://support-orig.hubstudio.cn/7794/e409) | 官方公开材料没有证明 Chrome 进程在代理失效、DNS、WebRTC、QUIC 或 UDP 情况下仍不能直连。RealBrowser 的本地 forward proxy + Windows 出站约束是更强、但必须抓包原型验证的属性。 |
| Cookie、账号与秘密 | 创建环境可录入 Cookie 和 2FA 密钥；API 可导入/导出 Cookie；平台账号 API 可按权限返回密码；环境转移会携带账号和 Cookie，云端缓存也可包含 Cookie/站点存储。[新建环境](https://support-orig.hubstudio.cn/645e/2879) [环境 API](https://support-orig.hubstudio.cn/0379/7beb/fbb0/8e65) [平台账号 API](https://support-orig.hubstudio.cn/0379/7beb/fbb0/e11b) [环境转移与清理](https://support-orig.hubstudio.cn/645e/cf62) | Cookie 的显式、单 Profile 导入导出属于常规功能范围，但必须加预览、审计、加密临时数据和清理。网站密码/2FA vault、后台批量导出和默认云上传仍不进入 MVP。 |
| 自动化/RPA/API | Local API 可读写环境、启停浏览器并返回调试端口，官方称可接 Selenium、Puppeteer、Playwright；CLI 支持 1–99 worker。Linux 指南还展示 Ubuntu 24.04 AppImage、Local API 和 CDP 远程自动化，并警告多数 API 端点无内建鉴权、CDP 可读取 Cookie/注入脚本。[API 总览](https://api-docs.hubstudio.cn/) [HTTP/CLI 模式](https://support-orig.hubstudio.cn/0379/7beb/935c/3d48) [Linux 指南](https://api-docs.hubstudio.cn/8945881m0) | Local API 和常用自动化框架接入属于标准功能范围；RealBrowser 应通过认证、短期 capability 和 Rust broker 提供，不能返回裸 CDP、默认开放任意 JavaScript 或演变成无审计的托管 RPA。 |
| 团队、云与迁移 | 官方支持 Boss/管理员/经理/成员及自定义用户组，按环境授权；可在多电脑和团队成员间同步插件数据、LocalStorage/IndexedDB，并同步标签页、允许同一环境多人多开；环境还可跨团队转移。[团队权限](https://support-orig.hubstudio.cn/9d74/2d26) [偏好与云同步](https://support-orig.hubstudio.cn/373a/0b7c) [环境转移](https://support-orig.hubstudio.cn/645e/cf62) | 这些能力需要服务端身份、授权、加密、冲突和审计体系。当前地图已明确排除团队、云同步、远程控制和跨机器 Profile 迁移。 |
| 扩展 | 官方团队扩展可从 Web Store URL 或 CRX/ZIP 导入，默认全局安装，也可按环境、分组、平台分配，并有每日自动更新开关。[扩展管理](https://support-orig.hubstudio.cn/645e/2946) | stock Chrome 可加载自有/用户批准扩展，但未受管 Windows 上的静默强装和可信更新受到平台政策限制。RealBrowser 应坚持签名/固定来源、权限提示、显式 allowlist 和可回滚更新，而非默认全局安装。 |
| 窗口同步 | Windows 同步器可排列窗口，把主控窗口的鼠标、键盘、标签页和插件操作复制到被控窗口，并提供多种仿真文本；官方 Q&A 说明它按坐标定位，建议同步 9–15 个环境。[同步器](https://support-orig.hubstudio.cn/0379/synchronizer/336f) [同步器 Q&A](https://support-orig.hubstudio.cn/0379/synchronizer/196a) | 窗口排列和可选择的同步器属于标准功能范围；输入广播必须显式选择目标、预览差异、支持紧急停止并审计，不能默认向全部运行窗口扇出。 |
| 内核与更新 | 文档称有 ChroBrowser/FireBrowser、可选择并下载内核，更新中心提示下载新版；API 返回专用浏览器路径、WebDriver 和 CDP 端口。[新建环境](https://support-orig.hubstudio.cn/645e/2879) [内核下载](https://support-orig.hubstudio.cn/7794/21e9) [浏览器 API](https://support-orig.hubstudio.cn/0379/7beb/fbb0/6964) | 官方文档对“当前内核”自相矛盾（仍写 Chrome 100，同时出现 131/133 参数），且未公开补丁集、构建链或更新 SLA，因此不能确认其内核实现。RealBrowser MVP 应继续由用户/企业安装的 Stable Chrome 拥有安全更新。 |
| 主机系统与套餐 | 官方材料同时描述 Windows 7+ 客户端、Arm/x64 macOS 安装包，以及 Ubuntu 24.04 无头 AppImage；macOS FAQ 又说明同步器/窗口排列是 Windows 特性。免费版公开为无限环境但每日打开 20 次，VIP 才含 API、窗口同步和批量操作。[Windows 入门](https://support-orig.hubstudio.cn/373a/0bfa) [Mac FAQ](https://support-orig.hubstudio.cn/7794/a06d) [Linux 指南](https://api-docs.hubstudio.cn/8945881m0) [价格](https://www.hubstudio.cn/pricing/index.html) | “环境模拟的 OS”与客户端宿主 OS 不能混为一谈，且官方页面存在年代差异。RealBrowser 只承诺 Windows 11，避免首版跨平台和兼容矩阵膨胀。 |

## RealBrowser 可行性矩阵

| 类别 | 能力 | 结论 |
| --- | --- | --- |
| **stock Chrome + Rust 可实现** | 独立 User Data、身份元数据/搜索/分组、Identity Templates、批量启停、进程归属、代理库与启动前探测、窗口排列、受控 Cookie 可移植性、登录态持久化、扩展管理、能力受限的本机 API/自动化、Windows 11 安装/UI | 这是当前地图的标准本地产品范围；不需要 Chromium fork。 |
| **需要原型或 OS enforcement** | Chrome Job/崩溃恢复；20 窗口容量；代理中断时 TCP/UDP/DNS/WebRTC/QUIC 全部 fail-closed；locale/timezone/geo/viewport/headers 的跨上下文一致性；未受管 Windows 的扩展安装/升级行为 | 在 [Chrome/Rust 控制边界](./chrome-control-constraints.md) 已列为 Windows 实测项，不能只靠启动参数或文档宣布完成。 |
| **要求自维护 Chromium/fork 才能可靠承诺** | 在 main world、跨域/特殊 frame、dedicated/shared/service workers 中一致覆盖 Canvas、WebGL/WebGPU、Audio、字体、媒体设备等，并与请求头、真实内核和 OS 事实保持一致；类似 HubStudio 的专有 ChroBrowser 发行与版本矩阵 | 这会引入持续合并安全补丁、签名、分发、编解码器/许可、兼容和多平台回归成本；当前地图已明确排除。公开 HubStudio 文档也不足以证明它实际达成这些属性。 |
| **后续独立产品层** | 团队 SaaS、加密云同步、跨团队/跨机器迁移、托管 RPA、云手机、自有浏览器内核发行 | 这些不是本地标准功能的简单开关，需要分别设计服务端授权、密钥、冲突、审计、更新和运维边界。 |

仍然刻意排除：网站密码/2FA 托管、无授权批量会话搬运、裸 CDP、默认任意 JavaScript、无选择地向全部窗口广播输入、公共代理抓取、静默直连回退，以及规避平台控制或“不可检测”的功能宣传。

## 对路线图的影响

HubStudio 不推翻现有架构，反而确认了环境管理、代理绑定、窗口工作台和扩展管理是用户可感知的主要产品面。下一步仍应先解决 [Choose the Browser Integration Architecture](../issues/05-choose-browser-integration-architecture.md)，随后定义 Persona 与 fail-closed egress；等身份生命周期和 Seller Platform 验收矩阵明确后，再用原型验证 200 个存档身份、20 个活动窗口的操作台。

产品定位应写成“**授权店铺的本地隔离浏览器管理器**”，而不是“HubStudio 克隆”或“无法被检测的指纹浏览器”。前者用 stock Chrome/Rust 能做并能验证；后两者要么扩大到完整 SaaS/自有内核，要么是公开资料无法支持的效果承诺。
