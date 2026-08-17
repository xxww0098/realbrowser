# 指纹浏览器：原理、可观察面与改指纹方法

> 研究快照：2026-08-15。本文是技术参考，不是产品能力声明，也不承诺“不可检测”“绕过风控”或匿名。
>
> 产品范围、非目标与商业措辞见 [`PRODUCT.md`](../PRODUCT.md)。领域词汇（Browser Identity / Profile / Persona / Network Egress）见 [`CONTEXT.md`](../CONTEXT.md)。本仓库当前只启动产品 Chromium；K0+K1 能力边界见 [`crates/browser-persona`](../crates/browser-persona/src/lib.rs) 与 [`docs/kernel-level-persona.md`](kernel-level-persona.md)。

---

## 1. 文档在回答什么

网站识别访问者，不只靠 Cookie。它会把**网络层事实**、**HTTP 头**、**JavaScript 可读环境**和**渲染/音频硬件输出**拼成一个相对稳定的标识。这个标识就是浏览器指纹（browser fingerprint）。

**指纹浏览器**不是另一种搜索引擎，也不是隐私浏览器。它是一套本地控制平面：为每一个业务身份准备一份隔离的浏览状态、一份内部自洽的可观察特征（Persona），以及一条声明过的网络出口（Egress）。商业产品常把它叫 antidetect / fingerprint browser；本仓库把它叫 Fingerprint Browser，但明确避免“不可检测浏览器”这类说法。

本文分四层写清楚：

1. 指纹是什么、为什么能识别人。
2. 网站实际采集哪些面，每一面的熵从哪来。
3. 工程上有哪几条缝可以改这些面，各自覆盖到哪、漏在哪。
4. 改完之后为什么还经常被识破：不一致、覆盖不全、网络栈对不上。

改指纹的目标，对授权多账号运营来说，通常不是“每次都不一样”，而是：

- **隔离**：两个 Identity 的 Cookie、storage、Persona、出口互不串。
- **稳定**：同一个 Identity 跨重启看起来还是同一台设备。
- **自洽**：UA、Client Hints、时区、语言、IP 地理、GPU 字符串、屏幕几何说的是同一个故事。

Brave / Tor 走的是相反方向：让同一个人在不同站点、不同会话里看起来不一样，或让所有人看起来一样。两者都叫“对付指纹”，工程目标完全不同。

---

## 2. 浏览器指纹是什么

### 2.1 标准定义

W3C 的指纹指南把浏览器指纹定义为：站点通过配置项或其他可观察特征，识别或再识别访问用户、用户代理或设备的能力。[Mitigating Browser Fingerprinting in Web Specifications](https://w3c.github.io/fingerprinting-guidance/)

它还区分了四类机制：

| 类型 | 含义 | 例子 |
| --- | --- | --- |
| 被动指纹 | 不在客户端执行代码，只看请求本身 | IP、TLS ClientHello、User-Agent、Accept-Language、HTTP/2 SETTINGS |
| 主动指纹 | 在页面里跑 JS / CSS，探测本地环境 | Canvas、WebGL、Audio、字体、`navigator.*`、屏幕 |
| Cookie 类状态 | 先写入再读回，用于再识别 | Cookie、localStorage、IndexedDB、Evercookie 一类复活手法 |
| 瞬时事件关联 | 用几乎同时发生的硬件/环境事件把两个会话绑在一起 | 媒体设备热插拔、设备姿态变化、渲染资源争用 |

被动指纹在页面脚本执行之前就已经发生。只改 JavaScript 原型，改不到 TLS 和 HTTP/2。

### 2.2 为什么组合起来能认出人

单独一项很少唯一。说“我用中文”或“我用 Windows”都不够。把几十个半标识叠在一起，笛卡尔积会迅速变稀。这是 Brave 对指纹的公开解释：法语用户很多，Linux 用户也很多，同时是法语 + Linux + 某分辨率 + 某 GPU 的人就少得多。[Brave Fingerprinting defenses 2.0](https://brave.com/privacy-updates/4-fingerprinting-defenses-2.0/)

学术起点和可引用数字：

- Mayer（2009）在 1,328 个实例上指出 Web 2.0 特征可以去匿名化。
- Eckersley / EFF Panopticlick（PETS 2010）：约 50 万次观测里，**83.6%** 的指纹即时唯一；再算上匿名集大小为 2 的 5.3%，绝大多数访客无法藏进人群。启用 Flash 或 Java 的子集里，唯一率升到 **94.2%**。作者同时警告：样本偏隐私关注用户，不能直接外推到全网。[How Unique Is Your Web Browser?](https://www.freehaven.net/anonbib/papers/pets2010/p1-eckersley.pdf) [EFF 综述](https://www.eff.org/deeplinks/2010/05/every-browser-unique-results-fom-panopticlick)
- Laperdrix 等 *Beauty and the Beast*（IEEE S&P 2016 / AmIUnique）：118,934 个指纹、17 个属性；桌面约 **89–90%** 唯一，移动约 **81%** 唯一。[HAL PDF](https://inria.hal.science/hal-01285470v2/document) 综述见 [ACM TWEB 2020](https://dl.acm.org/doi/10.1145/3386040)。
- Gómez-Boix、Laperdrix、Baudry（WWW 2018, *Hiding in the Crowd*）：在法国大型新闻站收集 **2,067,942** 个指纹，用与 AmIUnique **相同的 17 个属性** 时，唯一率只有 **33.6%**。时区熵因用户地理集中而塌缩。说明唯一率是**人群属性**，不是浏览器属性：实验室/自选样本和真实站点流量差一个数量级。[WWW 2018](https://dl.acm.org/doi/10.1145/3178876.3186097)
- Vastel 等 FP-STALKER（IEEE S&P 2018）：98,598 个指纹 / 1,905 个浏览器实例。约 **50% 在 5 天内变化，80% 在 10 天内变化**；规则 + 学习仍能把演化串起来，平均可跟 **54.48 天**，约 **26%** 超过 100 天。产业识别器因此很少死磕一个总哈希，而是做带权重的矢量比对。[FP-STALKER](https://inria.hal.science/hal-01652021/document)
- Cao、Li、Wijmans（NDSS 2017）：31 个 WebGL 渲染任务，在 1,903 台设备上 **>99%** 可唯一识别，且可跨浏览器对准同一台机器的 OS/硬件。[论文](https://yinzhicao.org/TrackingFree/crossbrowsertracking_NDSS17.pdf)

对指纹浏览器的直接推论：不必追求“宇宙唯一的 Canvas”。在真实流量里，普通办公本画像大量重复是正常的；更危险的是**自相矛盾**和**同一台工作站上本该撞车的面被改得过于稀有**。

产业侧（FingerprintJS 一类）强调两件事同时成立才有用：

- **熵够高**：组合后能把访客从人群里分开。
- **足够稳**：一次小改（清 Cookie、换 IP、升补丁）不应让标识彻底崩掉。好的识别器会给不同信号不同权重，而不是做一个脆弱的总哈希。[Fingerprint.com, Browser Fingerprinting Techniques](https://fingerprint.com/blog/browser-fingerprinting-techniques/)

### 2.3 指纹不是 Cookie，也不是“硬件序列号”

| | Cookie / storage | 浏览器指纹 |
| --- | --- | --- |
| 存在哪里 | 浏览器为该源存的状态 | 每次访问现场测出来的特征矢量 |
| 用户能否清掉 | 能 | 清 Cookie 通常清不掉 |
| 隐身模式 | 会话结束就没了 | 多数硬件/引擎面仍然在 |
| 跨站 | 受 SameSite / 分区存储限制 | 同一套硬件在不同源上往往测出相近结果 |
| 失败方式 | 被删、被拦、过期 | 环境一变哈希就变；乱改会自相矛盾 |

Canvas / WebGL / Audio 的哈希看起来像硬件序列号，其实是**渲染管线 + 驱动 + 字体栅格 + 浮点实现**的副作用。同一块 GPU 在不同 OS / 浏览器大版本上，哈希可以不同；不同机器如果驱动栈足够接近，哈希也可以撞车。Multilogin 自己也写：Canvas 哈希不必每个 profile 都独一无二，共享结果本身不证明两个 profile 被关联。[Multilogin Fingerprint section](https://multilogin.com/help/en_US/profile-settings-fingerprint-section)

### 2.4 网站拿指纹做什么

公开声明的用途包括：反欺诈、识别回访、把多次会话绑到同一设备、区分自动化流量。GoLogin、AdsPower、Fingerprint.com 都把这些写成产品叙事。[GoLogin: How browser fingerprints work](https://gologin.com/docs/how-browser-fingerprints-work)

对授权多店铺运营，真正要处理的产品问题是：

- 同一台工作站上的多个店铺账号，不能共享一份 Chrome Profile。
- 出口 IP、时区、语言如果明显对不上，平台会把它当成环境异常，而不是“指纹魔法失败”。
- 随机抖动每一次启动的 Canvas，往往比不改更像机器人。

---

## 3. 指纹浏览器的产品原理

### 3.1 三个边界必须分开

本仓库的领域模型把一个对外 Identity 拆成三件不可互相替代的东西：

```
Browser Identity
├── Browser Profile     隔离的浏览状态（Cookie、站点存储、偏好）
├── Browser Persona     网站能看到的、内部自洽的特征集合
└── Network Egress      声明过的出口：直连或某一条代理
```

削弱其中一个去“补”另一个，会立刻制造可检测裂缝。例如：

- 只换 User-Agent，不换 UA-CH / `navigator.userAgentData`。
- 只挂代理，不关非代理 UDP，WebRTC ICE 仍报真实地址。
- 只在页面主世界 hook Canvas，worker / OffscreenCanvas 仍泄漏真值。
- 声称 Windows GPU，却跑在 macOS 字体和 Core Text 栅格上。

### 3.2 三种完全不同的“反指纹”策略

| 策略 | 代表 | 目标 | 对多账号产品的含义 |
| --- | --- | --- | --- |
| 匿名集 / 人人一样 | Tor Browser、Firefox `resistFingerprinting` | 让所有用户返回同一套值 | 所有 Identity 撞车，店铺之间反而更像同一人 |
| 站点级噪声 / farbling | Brave | 同一用户在不同 eTLD+1、不同会话上看起来不同 | 同一店铺下次访问哈希变了，像换设备 |
| 稳定 Persona | AdsPower / Multilogin / GoLogin 一类，以及本仓库的设计 | 每个 Identity 像一台真实、稳定、自洽的设备 | 这是指纹浏览器该做的事 |

Brave 把 farbling 定义为：用**每会话、每 eTLD+1 种子**对半标识输出做确定性微扰。同一站点在同一会话里读到的值稳定，换站点或换会话就变。[Brave Farbling](https://brave.com/privacy-updates/4-fingerprinting-defenses-2.0/)

指纹浏览器如果照抄 Brave 的“每站点换种子”，会破坏账号连续性。正确的默认是：**每个 Identity 一颗根种子，按表面/字段做域分离派生，重启不旋转**；“按 origin 变化”只能是显式模式。

### 3.3 商业产品实际卖的是什么

公开文档能核实的共同结构：

1. **独立 Profile 目录**：每个环境一份 User Data，Cookie 和站点存储天然分开。
2. **一组可编辑的 Persona 字段**：UA、语言、时区、分辨率、WebRTC、Canvas/WebGL/Audio 噪声、字体、媒体设备。
3. **绑定代理**：并试图让时区 / 语言 / WebRTC IP 跟着出口走。
4. **内核或补丁声明**：AdsPower 称 SunBrowser / FlowerBrowser 是真实浏览器内核并随官方版本更新；GoLogin 称 Orbita 基于 Chromium；Multilogin 提供 Mimic（Chromium 系）和 Stealthfox（Firefox 系）。这些是厂商声明，不是本仓库的运行时证明。[AdsPower fingerprint help](https://help.adspower.com/docs/browser_fingerprint) [GoLogin profile settings](https://support.gologin.com/en/articles/14810056-profile-fingerprint-settings) [Multilogin Fingerprint section](https://multilogin.com/help/en_US/profile-settings-fingerprint-section)

厂商文档反复强调的运营规则，和技术事实一致：

- 账号创建后不要频繁改硬件相关参数。
- 时区、语言尽量与代理地理位置匹配。
- 一个 profile 绑一条专用代理。
- 随机乱改比不改更容易露出破绽。

---

## 4. 改指纹的五条工程缝

改一个可观察值，必须先问：它是在哪一层产生的？只能在产生它的那一层改，或者在更底层改。

```
网站服务器
    ▲
    │  被动：IP / TLS / HTTP/2 / HTTP 头
    │
浏览器网络栈（Cronet / BoringSSL / H2 / QUIC）
    ▲
    │  进程级：启动参数、企业策略、代理
    │
Chromium / Blink 渲染与 JS 运行时
    ▲
    │  CDP Emulation、Preferences
    │
页面 JS 世界（MAIN） / Worker / Worklet
    ▲
    │  MV3 content script、prototype hook
    │
扩展隔离世界（默认 ISOLATED）  ← 改这里，页面通常看不见
```

### 4.1 缝 A：独立 User Data / Profile

这是唯一对 Cookie 和站点存储真正可靠的隔离。

- Chromium 用 `--user-data-dir` 指定完整用户数据根，而不是同一根下再开一个 `--profile-directory` 就宣称“隔离完成”。[Chromium user data dir](https://chromium.googlesource.com/chromium/src/+/HEAD/docs/user_data_dir.md)
- 两个实例不能共享同一 User Data。
- Chrome 136 起，对**默认**数据目录不再尊重 `--remote-debugging-port` / `--remote-debugging-pipe`；要开远程调试必须同时给非默认 `--user-data-dir`。[Chrome 136 remote debugging](https://developer.chrome.com/blog/remote-debugging-port)

Profile 解决的是**状态隔离**，不是 Persona。两个目录里的 Chrome 如果跑在同一台机器上，Canvas / WebGL / Audio / 字体默认仍然相同。

### 4.2 缝 B：启动参数与企业策略

Stock Chrome 真正文档化、进程级生效的开关很少，但这些很少的开关很硬：

| 开关 / 策略 | 改变什么 | 不改变什么 |
| --- | --- | --- |
| `--user-data-dir=` | Profile 根 | Persona |
| `--proxy-server=` / `--proxy-bypass-list=` | 浏览器代理配置 | OS 其他进程、WebRTC UDP、QUIC 是否走同一条路 |
| `--lang=` | 一部分 UI / 应用 locale | **不完整**设置 `Accept-Language`（Chromium 已知缺口，[crbug 40651045](https://issues.chromium.org/40651045)） |
| `--accept-lang=` | 收窄 HTTP `Accept-Language` 与 JS 语言列表 | 必须和 `--lang` 一起用，否则出现“界面英语、头还是主机语言” |
| `--window-size=` | 初始窗口 | `screen.*`、DPR、CSS `device-width` |
| `--user-agent=` | 粗粒度 UA 字符串 | UA-CH 头、`navigator.userAgentData`；容易和真实内核版本打架 |
| `--force-webrtc-ip-handling-policy=` | 模仿企业 WebRTC IP 策略 | 不是“伪装成代理 IP”，只是限制 ICE 用哪些地址 |
| `WebRtcIPHandling=disable_non_proxied_udp` | 禁止非代理 UDP | 仍可能走代理 UDP/TCP；不是匿名保证 |
| `QuicAllowed=false` | 关掉 QUIC / HTTP3 | 需重启；不修 TLS 指纹 |
| `--disable-quic` | 同上，命令行侧 | 同上 |

完整开关列表由 Chromium 源码生成，Peter Beverloo 的页面是常用镜像。[Chromium command-line switches](https://peter.sh/experiments/chromium-command-line-switches/) WebRTC 策略见 [Chrome Enterprise WebRtcIPHandling](https://chromeenterprise.google/policies/web-rtc-ip-handling/)。

结论：**启动参数能做隔离、窗口、代理和少量策略，不能做 Canvas / WebGL / Audio / 字体 / TLS。**

### 4.3 缝 C：Chrome DevTools Protocol（CDP）

CDP 的 `Emulation` 域是 Stock Chrome 上最完整的“官方改环境”接口。关键方法（tip-of-tree 文档，部分标 Experimental）：

| 方法 | 作用 | 注意 |
| --- | --- | --- |
| `Emulation.setUserAgentOverride` | 覆盖 UA、`Accept-Language`、`navigator.platform` | **必须同时设 `userAgentMetadata`，Client Hints 才会按覆盖值发送** |
| `Emulation.setTimezoneOverride` | 覆盖 IANA 时区 | 空字符串恢复主机时区 |
| `Emulation.setLocaleOverride` | 覆盖 ICU locale（如 `en_US`） | Experimental |
| `Emulation.setGeolocationOverride` | 覆盖经纬度 / 精度；省略则模拟不可用 | 权限是另一件事 |
| `Emulation.setDeviceMetricsOverride` | 覆盖 `screen` / `inner` 尺寸、DPR、mobile 行为 | 部分字段 Experimental |
| `Emulation.setTouchEmulationEnabled` | 触摸点 | 与 mobile UA 必须一起看 |
| `Emulation.setHardwareConcurrencyOverride` | 覆盖 `navigator.hardwareConcurrency` | Experimental |
| `Emulation.setAutomationOverride` | 覆盖自动化标记 | Experimental；不是隐身保证 |
| `Page.addScriptToEvaluateOnNewDocument` | 每个新文档先跑一段脚本 | 仍是页面世界；worker 要另说 |

官方原文写得很干脆：`userAgentMetadata` must be set for Client Hint headers to be sent。[CDP Emulation](https://chromedevtools.github.io/devtools-protocol/tot/Emulation/)

CDP 的结构性限制：

- Chrome 136 后必须非默认 User Data 才开得了。
- tip-of-tree 协议不稳定；Experimental 命令不能当产品契约。
- 每个新 Target（新标签、新 iframe 进程、部分 worker）都要重新 attach 并重放 override。
- CDP 是高权限调试面，不是公开 Local API。Google 明确说过它被用来偷 Cookie。

### 4.4 缝 D：MV3 扩展 / 页面脚本注入

Content script 默认跑在 **ISOLATED** 世界：和页面共享 DOM，但不共享 JavaScript 堆。在隔离世界里改 `HTMLCanvasElement.prototype.toDataURL`，**普通网页脚本读不到**。[Chrome content scripts](https://developer.chrome.com/docs/extensions/develop/concepts/content-scripts)

要影响页面观察到的 API，必须显式：

```json
{
  "content_scripts": [{
    "matches": ["<all_urls>"],
    "js": ["hooks.js"],
    "run_at": "document_start",
    "all_frames": true,
    "match_about_blank": true,
    "world": "MAIN"
  }]
}
```

即便如此，仍有硬边界：

| 覆盖位 | 默认 content script | 声明 `all_frames` + `MAIN` | 仍然做不到 |
| --- | --- | --- | --- |
| 顶层页面 | 是 | 是 | 注入前的同步脚本竞态 |
| 同源 / 跨源 iframe | 否 | 部分 | `about:blank` / `blob:` / `srcdoc` 要额外声明 |
| Dedicated / Shared Worker | 否 | 否 | 没有“给任意站点 worker 注入 content script”的对称 API |
| Service Worker | 否 | 否 | 站点自己的 SW 不在扩展 content script 世界里 |
| OffscreenCanvas 在 worker | 否 | 否 | 内核或 CDP 目标附着 |
| HTTP 头 / TLS / H2 | 否 | 否 | 网络栈 |

本仓库对 `juu17/browser-fingerprint-shuffler` 的源码审阅已经证实：Manifest 不声明 `world` / `all_frames` 时，Canvas / Audio / Navigator hook 只影响扩展自己的 JS 世界；只有 WebGL 另插了 page-world 文件，还要等异步 `chrome.storage`，页面可以在补丁装上之前读到真值。[`.scratch/fingerprint-browser/research/juu17-fingerprint-shuffler.md`](../.scratch/fingerprint-browser/research/juu17-fingerprint-shuffler.md)

MAIN world 的另一面：页面可以检测原型是否被包装（`toString` 异常、堆栈、描述符）。`iframe.contentWindow.HTMLCanvasElement` 是一份**未打补丁的新对象**；CreepJS 专门为此做了 iframe / prototype 测页。

### 4.5 缝 E：定制内核 / Chromium 补丁

只有这一层能同时改：

- Canvas / WebGL / WebGPU 的像素回读（在 Blink 光栅之后、JS 拿到数据之前）。
- 字体枚举与 `measureText` / ClientRects。
- TLS ClientHello、HTTP/2 SETTINGS、JA3/JA4。
- 所有 frame 和 worker 的同一套值。

开源和商业实现都把这一点写在表面上：

- Pota Browser 明确说 Canvas / WebGL / Audio 深层修改需要 C++ Chromium patch。
- Camoufox 用 Firefox C++ patch 读 typed properties。
- BotBrowser 声称在渲染管线而不是 JS 注入上对 Canvas 加确定性噪声，并用 profile 内置字体让跨 OS 输出一致。[BotBrowser Canvas](https://github.com/botswin/BotBrowser/blob/main/docs/guides/fingerprint/CANVAS.md)
- AdsPower 公开文案把“真实内核 + 跟官方大版本走”当作 TLS / H2 / Canvas 一致性的来源。这是营销主张，但方向与工程事实相符：网络栈指纹来自内核，不来自扩展。

维护 Chromium fork 的代价是：跟上游、编编编、编解码器、签名、安全补丁 SLA。本仓库 MR-0 明确不走这条路。

---

## 5. 表面目录：网站看什么，工程怎么改

图例：

- **可靠**：该层有官方或内核接口，语义清楚。
- **有限**：能改一部分观察点，覆盖或一致性不足。
- **—**：不该由该层承诺。

### 5.1 网络与传输：JS 永远改不到

#### IP 与出口

网站最先看到的是 TCP/UDP 源地址、HTTP 代理链、有时还有 `X-Forwarded-For`。出口属于 Network Egress，不属于 Persona。

改法：

- Chrome `--proxy-server=http://host:port` 或 SOCKS5。官方网络设置见 [Chromium network settings](https://www.chromium.org/developers/design-documents/network-settings/)，SOCKS 见 [SOCKS proxy](https://www.chromium.org/developers/design-documents/network-stack/socks-proxy/)。
- 本地 forward proxy 消化上游凭据，Chrome 只连 loopback。这是本仓库倾向的模型。
- OS 层 egress ACL（macOS pf / Windows WFP）才是 fail-closed：Chrome 进程树不准直连公网。

只设 `--proxy-server` **不等于**不会泄漏。bypass 列表、PAC 回落 `DIRECT`、WebRTC UDP、QUIC、浏览器外进程，都会绕开。

#### WebRTC ICE

WebRTC 为了打洞会收集 ICE candidate，里面可以出现：

- 局域网地址（`192.168.*`、`10.*`、`fd00:`）
- 主机公网地址（和代理出口不是同一个）
- mDNS 候选

这四个取值来自 [RFC 8828 §5.2](https://www.rfc-editor.org/rfc/rfc8828.html#section-5.2)，Chrome / Edge 企业策略沿用同一组名字。另有按 URL 生效的 `WebRtcIPHandlingUrl`。Stock Chrome 可验证的控制是 **IP handling policy**，不是“把 ICE 改写成任意假 IP”：

| 策略值 | 行为 |
| --- | --- |
| `default` | 默认，可能暴露本地接口 |
| `default_public_and_private_interfaces` | 公网 + 私网 |
| `default_public_interface_only` | 只用默认公网接口 |
| `disable_non_proxied_udp` | 除非代理支持 UDP，否则只用 TCP 联系对端 |

命令行对应 `--force-webrtc-ip-handling-policy=disable_non_proxied_udp`。本仓库 MR-0 在启用代理时要求 Persona 的 WebRTC 策略关掉非代理 UDP，否则启动失败。

商业产品额外提供“Replace / Masked / Forward”：声称 ICE 里出现的 IP 等于代理 IP，或把 STUN 流量转发到 Google 公共服务器。这需要内核或网络栈拦截，Stock Chrome 做不到“伪造 ICE 地址还保持通话可用”。AdsPower 自己也区分 Disabled（拦流量但不拆 API，让网站以为是防火墙）和 Disabled UDP。[AdsPower WebRTC](https://help.adspower.com/docs/browser_fingerprint)

#### TLS：JA3 / JA4

TLS 握手的 ClientHello 在加密应用数据之前明文发送。JA3 取以下字段的十进制值，用 `,` 和 `-` 拼起来再 MD5：

```
SSLVersion,Cipher,SSLExtension,EllipticCurve,EllipticCurvePointFormat
```

并忽略 GREASE，以免 Chrome 每次握手都换哈希。[Salesforce JA3](https://github.com/salesforce/ja3)

JA4 由同一作者后续提出：对扩展排序，加入 TCP/QUIC、TLS 版本、ALPN 等，现代浏览器随机打乱扩展顺序后仍然可聚类。[Cloudflare JA3/JA4](https://developers.cloudflare.com/bots/additional-configurations/ja3-ja4-fingerprint/) [FoxIO JA4](https://github.com/FoxIO-LLC/ja4)

含义：

- Python / Go / Node 的 HTTP 客户端，JA3 通常不像 Chrome。
- **用用户已安装的 Stable Chrome，TLS 指纹天然就是 Chrome。** 这对指纹浏览器是优势，不是缺陷。
- 想“假装成另一个浏览器的 TLS”必须换 TLS 栈。JS、CDP、扩展都做不到。
- 本仓库因此把 TLS 从可编辑 Persona 字段删除：它是引擎/网络事实。

#### HTTP/2 / HTTP/3

Akamai 提出的被动 H2 指纹常见格式：

```
SETTINGS|WINDOW_UPDATE|PRIORITY|PSEUDO_HEADER_ORDER
```

例如 Chrome 一类客户端会送固定的 SETTINGS 对、一个约 `15663105` 的 `WINDOW_UPDATE`，以及 `:method, :authority, :scheme, :path` 的伪头顺序。CDN 和 Cloudflare 用它核对“自称 Chrome 的客户端，握手是不是也像 Chrome”。

QUIC / HTTP3 另有一套传输指纹。企业策略 `QuicAllowed=false` 或 `--disable-quic` 可以让流量退回 TCP+H2，减少一条面，但不能把 H2 指纹改成任意值。

| 面 | 启动参数 | CDP | MV3 | 定制内核 |
| --- | --- | --- | --- | --- |
| 出口 IP | 可靠（代理配置） | 有限 | — | 可 |
| WebRTC 本地 IP | 策略有限 | — | — | 可伪造/对齐 |
| TLS JA3/JA4 | —（用真 Chrome 即真 Chrome） | — | — | 可靠 |
| HTTP/2 指纹 | — | — | — | 可靠 |
| DNS / DoH | 策略有限 | 观测为主 | — | 可 |

### 5.2 HTTP 头与 User-Agent Client Hints

Chrome 89 起默认启用 UA-CH。每次请求默认带：

```
Sec-CH-UA: "Chromium";v="93", "Google Chrome";v="93", "Not;A Brand";v="99"
Sec-CH-UA-Mobile: ?0
Sec-CH-UA-Platform: "macOS"
```

服务器用 `Accept-CH` 再要高熵字段：`Sec-CH-UA-Full-Version-List`、`Sec-CH-UA-Platform-Version`、`Sec-CH-UA-Arch`、`Sec-CH-UA-Model`、`Sec-CH-UA-Bitness`。JS 侧是 `navigator.userAgentData.brands / mobile / platform` 和 `getHighEntropyValues()`。[Chrome UA-CH](https://developer.chrome.com/docs/privacy-security/user-agent-client-hints) [WICG UA-CH](https://wicg.github.io/ua-client-hints/)

同时，Chrome 把传统 `User-Agent` 字符串降分辨率（User-Agent reduction）：桌面不再细报 OS 小版本，移动设备型号被收成通用值。站点若还只解析 UA 字符串，会漏掉 CH 里的真值；站点若两边都读，只改其中一个就会穿帮。

**正确改法（原子组）：**

1. CDP `Emulation.setUserAgentOverride`，同时传：
   - `userAgent`
   - `acceptLanguage`
   - `platform`（即 `navigator.platform`）
   - `userAgentMetadata`（brands、fullVersionList、platform、platformVersion、architecture、model、mobile、bitness）
2. 内核 major、UA major、CH brands 必须一致。不要出现“UA 写 Chrome 146、CH 写 Chrome 120、进程其实是 146”。
3. 不要只用 `--user-agent=`。它不更新 CH。

相关头还有 `Accept-Language`、`Accept-Encoding`、`DNT`、`Sec-GPC`、`Sec-Fetch-*`。后两者是隐私偏好，应是用户选择，不应随机，也不该叫“稳定指纹”。

### 5.3 Navigator 与 JS 环境

常见采集：

```text
navigator.userAgent
navigator.platform          // Win32 / MacIntel / Linux x86_64
navigator.language / languages
navigator.hardwareConcurrency
navigator.deviceMemory      // 粗粒度 GiB
navigator.maxTouchPoints
navigator.vendor
navigator.plugins / mimeTypes   // 现代 Chrome 已基本空壳
navigator.webdriver         // WebDriver 控制时为 true
navigator.userAgentData
navigator.connection
navigator.pdfViewerEnabled
window.chrome               // Chrome 有，Firefox 无
```

Multilogin 把 UA、Platform、HardwareConcurrency、OSCPU 绑成一组，并警告：macOS UA 配 Windows platform 会自己打自己。[Multilogin Navigator](https://multilogin.com/help/en_US/profile-settings-fingerprint-section)

| 字段 | 启动参数 | CDP | MV3 MAIN | 内核 |
| --- | --- | --- | --- | --- |
| UA + UA-CH + platform | 粗略 | 可靠（带 metadata） | 有限（改得到 JS，改不到请求头） | 可 |
| languages | `--lang` 一部分 | `acceptLanguage` + locale override | 有限 | 可 |
| hardwareConcurrency | — | Experimental override | 有限 | 可 |
| deviceMemory | — | — | 有限 | 可 |
| plugins / mimeTypes | — | — | 有限 | 可 |
| webdriver | 避免用 WebDriver | `setAutomationOverride` | 有限 | 可 |
| `window.chrome` | 用真 Chrome 即存在 | — | 可补/可露馅 | 可 |

`navigator.webdriver` 是自动化最显眼的布尔值。Playwright / Puppeteer 默认会留下 CDP 伪影（`Runtime.enable` 的副作用、console 补丁、headless 特征）。用真人操作的 Stock Chrome 窗口，这一项默认就是普通用户。为“隐身”去 hook `webdriver`，本身就会留下原型包装痕迹。

### 5.4 屏幕、窗口、DPR

站点会读：

```text
screen.width / height
screen.availWidth / availHeight
screen.colorDepth / pixelDepth
window.innerWidth / innerHeight
window.outerWidth / outerHeight
window.devicePixelRatio
matchMedia('(pointer: coarse)')
CSS @media (device-width)
```

约束：

- `avail* <= screen*`
- 窗口不能大于声明的屏幕（多显示器例外要自圆其说）
- DPR 必须是该设备族真实会出现的值（`1` / `1.25` / `1.5` / `2` / `3`）
- 移动 UA 却是桌面式 overlay 滚动条、无触摸点，是常见矛盾

改法：`--window-size` 只改窗口；要改 `screen.*` 和 CSS `device-width`，用 CDP `setDeviceMetricsOverride`。Persona 里应同时保存 **window** 与 **viewport / screen**，不要当成同一个数。

Multilogin 的实用建议：选一个不超过你所有工作机物理屏的分辨率。Profile 设 1920×1080、显示器只有 1366×768，窗口会超出可用区域。[Multilogin Screen](https://multilogin.com/help/en_US/profile-settings-fingerprint-section)

### 5.5 时区、Locale、地理位置

这是最容易和出口 IP 对不上的一组。

| 观察点 | API |
| --- | --- |
| 时区名 | `Intl.DateTimeFormat().resolvedOptions().timeZone` |
| 偏移 | `new Date().getTimezoneOffset()` |
| 语言 | `navigator.languages`、`Accept-Language`、`Intl` |
| 坐标 | `navigator.geolocation`（要权限） |
| 区域格式 | `Intl.NumberFormat` / `DateTimeFormat` |

**不要**把进程环境变量 `TZ` 当成跨平台 Persona 已应用。Windows 上尤其不可靠。

正确改法：

1. CDP `setTimezoneOverride("Europe/London")`
2. 进程级同时给 `--lang=` 和 `--accept-lang=`，或企业策略 `ForcedLanguages` / `ApplicationLocaleValue`；再加 CDP `setLocaleOverride("en_GB")` 与 UA override 的 `acceptLanguage`。只设 `--lang` 会出现 UI 语言和 HTTP 头分裂。
3. 启动后在**顶层文档**观测 `Intl` 与 `Date`，并对新 Target 重放
4. 地理位置用 `setGeolocationOverride`；权限用 Profile / Preferences，不要假装“Stable”

商业产品默认“Based on IP / Masked”：时区和坐标跟着代理地理走，坐标再加一点偏移，避免所有 profile 钉在同一个机房经纬度。语言则不必等于代理国家——它是用户偏好，但完全离谱（莫斯科出口 + 仅 `ja-JP`）会被纳入风险模型。

### 5.6 Canvas 2D

原理：页面在隐藏 canvas 上画渐变、文字（常用特定字体和 Unicode）、emoji，然后：

```text
canvas.toDataURL()
canvas.toBlob()
ctx.getImageData()
OffscreenCanvas.convertToBlob()
ctx.measureText() / isPointInPath()
```

把像素或度量做成哈希。该方法由 Mowery 与 Shacham 在 W2SP 2012 的 *Pixel Perfect: Fingerprinting Canvas in HTML5* 系统提出：同一段绘制指令在不同 GPU / 驱动 / OS 文本栅格上会产生稳定的亚像素差异。[论文 PDF](https://hovav.net/ucsd/dist/canvas.pdf)

差异来自：

- GPU / 驱动 / OS 文本栅格（ClearType、Core Text、FreeType）
- 抗锯齿、子像素、颜色管理、色域（sRGB / `display-p3`）
- 浏览器 Skia 实现细节

Princeton Web Census 曾在 Top 100 万站点中发现大量 Canvas 脚本；它至今仍是主动指纹的主力之一。Fingerprint.com 的公开说明与此一致。[Canvas fingerprinting](https://fingerprint.com/blog/browser-fingerprinting-techniques/)

**改法分层：**

1. **Native**：不改。同一台机器上的所有 Identity 哈希相同。隔离仍靠 Profile。对“我只是不要串号”往往够用；对“每个 Identity 必须像不同硬件”不够。
2. **持久噪声（推荐的 Persona 语义）**：由 Identity 根种子派生像素扰动。同一 Identity 每次 `toDataURL()` 得到**同一**结果；不同 Identity 不同。Brave 的 default farbling、Multilogin Noise、BotBrowser `--bot-noise-seed` 都属于这一族。关键是**幂等**：不能每调用一次就消耗有状态 PRNG。
3. **JS hook**：包装 `getImageData` / `toDataURL` / `toBlob`。必须 MAIN world、`document_start`、`all_frames`，且不要把噪声写回调用方画布（Juu17 写回原画布，会破坏页面语义）。Worker / OffscreenCanvas / 时序仍漏。
4. **内核回读拦截**：在 Blink 把像素交给 JS 之前改。能覆盖 worker。BotBrowser 公开声称走这条路。
5. **Disabled**：直接拆 API。图表、验证码、编辑器会坏，也极度显眼。Multilogin 标成遗留、不建议日常使用。

检测方常见对策：连读两次看是否变化；对比 iframe 干净原型；看 `toString`；同时采 WebGL 图像看两者是否像同一 GPU。

### 5.7 WebGL / WebGL2 / WebGPU

WebGL 提供两套指纹：

1. **元数据**：`getParameter(VENDOR/RENDERER)`，以及 `WEBGL_debug_renderer_info` 的 `UNMASKED_VENDOR_WEBGL` / `UNMASKED_RENDERER_WEBGL`（例如 `NVIDIA GeForce RTX 2080 Ti` + Direct3D 后端）。再加扩展列表、最大纹理、着色器精度、各种 limit。
2. **图像**：用着色器画隐藏场景，`readPixels` 后哈希。比 2D canvas 更吃 GPU 管线。

WebGPU 是下一代面：`adapter.info` 的 vendor / device / architecture，以及 GPU 计算输出。AdsPower 提供 Based on WebGL / Real / Disabled；Multilogin 把 WebGL+WebGPU metadata 和 WebGL graphics（像素）分开。[AdsPower WebGPU](https://help.adspower.com/docs/browser_fingerprint)

**硬约束：vendor 与 renderer 是原子对。** 不能 Intel vendor 配 NVIDIA renderer。还要和 UA 的 OS、是否 Apple Silicon 说得通。Multilogin 特别注明：Apple Silicon 上 Chrome 仍可能报 `Macintosh; Intel Mac OS X 10_15_7` 和 `MacIntel`，这是兼容性 token，不自动和 M 系列 GPU 冲突。

| 子面 | JS hook | 内核 | 注意 |
| --- | --- | --- | --- |
| vendor / renderer 字符串 | 有限（`getParameter` / `getExtension`） | 可靠 | 必须成对；iframe 里的新 context 也要盖到 |
| 扩展列表 / limits | 有限 | 可靠 | 乱减扩展会和真实 GPU 矛盾 |
| 渲染图像哈希 | 有限（`readPixels` / `toDataURL`） | 可靠 | 噪声应种子化；换主机 GPU 时“滤镜相同、底片不同” |
| WebGPU adapter.info | 极有限 | 可靠 | 与 WebGL metadata 对齐 |

“Noise”加在像素上，**不会**自动改 UNMASKED_RENDERER。只改字符串不改图像，或只改图像不改字符串，都是 Pixelscan / CreepJS 一类检查器会抓的不一致。

### 5.8 AudioContext

经典算法（FingerprintJS 公开过实现）：

1. 建 `OfflineAudioContext(1, 5000, 44100)`
2. 三角波振荡器 1000 Hz
3. 接 `DynamicsCompressor`（threshold / knee / ratio 等固定参数）
4. `startRendering()`，对 `getChannelData(0)` 求和得到一个浮点数

Fingerprint.com 写明：差异主要来自**浏览器引擎的浮点/SIMD 实现**，不是声卡型号。同一台电脑上 Chrome / Safari / Firefox 会得到不同常数，且在普通模式下稳定；Safari 17 隐私浏览会注入噪声。[Audio fingerprinting](https://fingerprint.com/blog/audio-fingerprinting/)

改法与 Canvas 同构：种子化、幂等地微扰 `AudioBuffer.getChannelData` / `AnalyserNode`；或在内核改。Juu17 把 `getChannelData()` 改成返回副本再加噪声，既改变语义，又按调用次序消耗 PRNG——这是反面教材。

Audio 哈希撞车很常见。不要为了“每个 profile 不同”去制造不可能出现的浮点值。

### 5.9 字体与 ClientRects

现代 Chrome 不再给出完整系统字体列表。站点改用：

- `document.fonts.check("12px SomeFont")`
- 用 `measureText` 或隐藏 DOM 量字符宽高
- `getClientRects()` / `Range.getClientRects()` / SVG `getBBox()` 看亚像素布局
- 滚动条宽度（`innerWidth - clientWidth`）

Fifield 与 Egelman（FC 2015）证明：同一 Unicode 字形、同一 CSS 字体族，在不同 OS / 浏览器上的 **bounding box 不同**；约 43 个字符就够用，样本里约 **34%** 仅凭字体度量唯一。[Fingerprinting Web Users Through Font Metrics](https://doi.org/10.1007/978-3-662-47854-7_7)

字体集强烈绑定 OS：Windows 有 Segoe UI，macOS 有 San Francisco / PingFang，Linux 发行版各不相同。给 Windows Persona 配一套只有 macOS 才有的字体，或反过来，是高置信矛盾。

改法：

- Native：主机真实字体。同一工作站上所有 Identity 相同。
- 内核 / 内置字体库：按 Persona 的 OS 暴露一套常见字体，并让栅格也走这套字体（BotBrowser 声称 profile 自带字体，跨主机 Canvas 才能一致）。
- JS 拦截 `document.fonts` / `measureText`：有限，且容易和真实绘制对不上。

ClientRects 对 DPI、缩放、字体、滚动条宽度敏感。乱加噪声会让页面布局检测自己打自己。

### 5.10 媒体设备、语音、传感器、电池

`navigator.mediaDevices.enumerateDevices()` 在未授权时通常只给模糊项；授权后出现真实 `deviceId` 和 label（如 `FaceTime HD Camera`）。`speechSynthesis.getVoices()` 暴露系统语音包，也高度绑定 OS / 语言。

Battery Status 在桌面 Chrome 已基本无区分度；再去伪造电量，收益低、痕迹高。

Permissions API（`geolocation` / `notifications` / `camera`）是 Profile 状态，不是 Persona 随机数。

商业产品允许 Masked / Custom 设备个数。可信的笔记本画像通常是 1 摄 + 1 麦 + 1 出。不要给 iPhone UA 报 4 个 USB 摄像头。

### 5.11 存储、扩展、行为

这些经常被误叫成“指纹”，工程上要分开：

| 机制 | 它是什么 | 和 Persona 的关系 |
| --- | --- | --- |
| Cookie / localStorage / IndexedDB | 站点写入的状态 | Profile 隔离负责；不要读、不要导 |
| Service Worker / Cache Storage | 站点自己的持久逻辑 | 随 Profile；Persona 覆盖不到 SW 里的 JS |
| 已装扩展 | 可通过 CSS / web accessible resources / 特定行为探测 | 管理器自己的扩展也是指纹；要固定、最小权限 |
| 鼠标轨迹、击键、焦点 | 行为生物特征 | 同步器 / 群控最容易在这里露馅；本仓库不把它当 Persona 字段 |

Chrome 拥有的登录态（Cookies、Login Data、密码、token）在本产品里是不透明的。改指纹文档不包含任何读取或迁移这些文件的方法。

### 5.12 次要但仍会被采集的面

主表面改完之后，检测脚本还会顺手读这些。它们单独熵不高，却经常用来做**一致性旁证**：

| 面 | 观察方式 | 改法与态度 |
| --- | --- | --- |
| CSS 媒体特征 | `prefers-color-scheme`、`prefers-reduced-motion`、`forced-colors`、`hover` / `pointer`、对比度 | CDP `setEmulatedMedia` 可改一部分；应跟 OS / 是否移动设备一致 |
| 键盘布局 | `navigator.keyboard.getLayoutMap()`（要权限）、`Intl` / 输入事件 `code` vs `key` | 与语言、地理同一区域；不要随手伪造 |
| 语音包 | `speechSynthesis.getVoices()` | 绑定 OS + 语言；Windows 与 macOS 列表完全不同 |
| 编解码器 | `MediaSource.isTypeSupported`、`video.canPlayType`、Audio/Video codec 字符串 | 来自内核；Stock Chrome 保持原生最安全 |
| HDR / 色域 | `matchMedia('(dynamic-range: high)')`、`colorGamut`、`display-p3` canvas | 跟屏幕 DPR / GPU 一起看 |
| Emoji / 系统 UI 字体 | Canvas 画 emoji、私有 Unicode | 跨 OS 最容易穿帮；内核内置字体库才能稳定 |
| Apple Pay / WebAuthn / Widevine | `ApplePaySession`、`PublicKeyCredential`、EME CDM 名称 | 平台专有；桌面 Windows Persona 不应出现 Apple Pay |
| WebRTC SDP | 编解码偏好、`ice-options`、fingerprint 行 | 内核/WebRTC 栈；JS 改 SDP 很脆 |
| 数学/SVG 文本度量 | MathML、SVG `getBBox` | 与 ClientRects / 字体同一族 |
| 自动化残余 | `cdc_` 前缀、`__playwright`、`__puppeteer`、DevTools 协议探测 | 不要用这些栈开店；真人 Stock Chrome 默认没有 |

这些面的产品策略应当是：**能保持 Native 就保持 Native**；只有当主 Persona 已经换了 OS 族，才需要一起编进同一套 preset。单独给它们加噪声，收益低、破绽高。

---

## 6. 怎么设计一套可落地的改指纹系统

### 6.1 先编译，再应用，再观测

不要让 UI 上的一个开关同时表示“用户想改”“运行时能改”“已经改上了”。本仓库研究结论是拆成三份：

```text
PersonaSpec          用户保存的意图（含根种子、schema 版本）
    ↓ 对照 RuntimeCapabilities
PersonaExecutionPlan 启动参数 + CDP 动作 + 扩展动作 + 明确拒绝项
    ↓ 启动并观测
PersonaObservation   实际读到的值、覆盖位、失败原因
```

字段能力至少要能表达：

```text
backend:     Native | LaunchArgument | Cdp | Mv3MainWorld | CustomKernel
confidence:  Native | MappedUnverified | Observed | NotApplied
coverage:    top_frame | all_frames | dedicated_worker | shared_worker
             | service_worker | network
```

没有实测覆盖之前，UI 不得显示“已应用”。页面级 hook 最多标“扩展·有限”。

### 6.2 种子与幂等

```text
root_seed          256-bit，每 Identity 一颗，持久化
derived(field)     BLAKE3(root_seed, schema_version, persona_id, surface, property)
noise(pixels)      由 derived(field) 决定，对同一输入永远同一输出
```

禁止：

- 各 API 共享一个可变 PRNG，按调用顺序吐随机数。
- 存储失败时静默换盐。
- 默认按 origin 换种子（那是 Brave 的反追踪，不是店铺身份）。

### 6.3 一致性约束（保存时就应拒绝）

这些是检测器最爱的裂缝，也是生成器必须写成机器可检查的规则：

1. Chrome 内核 major = UA major = UA-CH brands / fullVersionList。
2. `languages[0]`、`navigator.language`、`Accept-Language` 主语言一致。
3. 时区、地理“随出口”时，只能来自同一次已验证的 egress 快照。
4. `screen.avail* <= screen.*`，DPR > 0，窗口可被该屏幕放下。
5. OS、字体、voices、touch、CPU、GPU preset 属于同一平台族。
6. WebGL vendor / renderer 不可拆；WebGPU metadata 跟着同一 GPU。
7. 移动 UA ⇒ 有合理 `maxTouchPoints`、mobile CH、匹配的 viewport。
8. 代理开启 ⇒ WebRTC 不得停留在会泄漏非代理 UDP 的策略。
9. DNT / GPC 是用户选择，不参与随机。
10. 不要用真实主机的稀有字体 / 稀有 GPU 字符串，去配一个声称是普通办公本的 Persona。

BrowserForge 这类生成器的价值在于**约束采样**，不在于它的已弃用注入器。Camoufox 的 typed properties 也是同一思路：内核读一份自洽配置，而不是 UI 上五十个互不相干的文本框。

### 6.4 推荐的应用顺序

启动一个 Identity 时，顺序比“改了多少字段”更重要：

1. 锁 Profile 目录，确认无第二实例。
2. 拉起本地代理，探测真实出口 IP / 粗地理；失败则拒绝启动。
3. 编译 Persona：把“随出口”的时区、语言、坐标写成具体值。
4. 用启动参数打开：User Data、窗口、`--lang`、代理、WebRTC 策略、禁用后台模式。
5. 等浏览器就绪后 attach CDP（pipe 优先），按 Target 重放 UA-CH、时区、locale、geo、device metrics。
6. 若启用了 MV3 Persona Runtime：确认 MAIN / all_frames 已注册，再打开业务页。
7. 用观测页交叉核对：JS 值、请求头、ICE、时区。不一致则标 Degraded，而不是绿灯。

### 6.5 RealBrowser K0+K1 现在真正执行的

对照本仓库 28 项能力目录（2026-08 快照）：

| 已映射并要求观测 | 本轮仍为 Native / 不可用 |
| --- | --- |
| 每 Identity 独立 User Data + secret-free `persona.json` | UA / UA-CH（保持内核原生，不暗示已伪造） |
| `--window-size` | WebGL / WebGPU / ClientRects |
| `--lang` | Audio / 字体 / 媒体设备 / 语音 |
| WebRTC IP policy 启动参数 | hardwareConcurrency / deviceMemory / touch |
| 时区：Persona Runtime + 顶层观测 | plugins / battery |
| 代理：host:port，无凭据 | TLS（只读引擎事实） |
| Canvas 2D：Blink 副本微扰 + 顶层/iframe/dedicated worker 观测 | |
| | 地理 CDP 尚未作为可编辑已应用项 |

这不是功能残缺，而是诚实。把 Native 标成 Managed，比少几个开关更危险。

---

## 7. 覆盖范围：改到了页面，不等于改到了浏览器

同一 Persona 必须在这些执行上下文里讲同一个故事：

```text
顶层窗口
  ├─ 同源 iframe
  ├─ 跨源 iframe
  ├─ about:blank / srcdoc / blob:
  ├─ Dedicated Worker
  ├─ Shared Worker
  ├─ Service Worker
  ├─ Paint Worklet / Audio Worklet
  └─ OffscreenCanvas（窗口或 worker）
```

验收时不要只打开 BrowserLeaks 的顶层页。至少：

- 顶层读一次 Canvas / WebGL / UA / 时区。
- 同源 iframe 再读一次，必须相同。
- 跨源 iframe 再读一次。
- Worker 里 `importScripts` 后读 Navigator / OffscreenCanvas（若声称覆盖）。
- 用 `fetch` 看请求头是否与 JS 一致。
- 用 WebRTC 页看 ICE 是否还在报局域网地址。

CDP override 必须在 **Target.create** / attached 时重放。只在第一个标签设时区，第二个标签仍是主机时区，这是真实发生过的类缺陷。

---

## 8. 检测方在对什么

公开检查器（BrowserLeaks、CreepJS、Pixelscan、FingerprintJS demo、iphey）和商业风控，看的不是“哈希漂不漂亮”，而是：

### 8.1 自相矛盾

- UA 说 iPhone，WebGL 报 NVIDIA。
- CH platform 是 Windows，`navigator.platform` 是 `MacIntel`。
- 代理 IP 在法兰克福，时区是 `Asia/Shanghai`。
- `hardwareConcurrency = 32` 配移动设备。
- Canvas 噪声每次调用都变，或 iframe 与顶层不同。

GoLogin 自己的排障表几乎就是这份清单：OS mismatch、WebGL mismatch、Timezone mismatch、Canvas hash changed。[GoLogin verify](https://support.gologin.com/en/articles/14810056-profile-fingerprint-settings)

### 8.2 自动化与注入痕迹

- `navigator.webdriver === true`
- 原型 `toString` 不是 `[native code]`
- iframe 里的干净 `HTMLCanvasElement.prototype` 与顶层不一致
- 存在 Playwright / Puppeteer 的典型补丁对象
- Headless 特征（缺插件、缺 window chrome、WebGL 软件渲染 SwiftShader 却声称独立显卡）

CreepJS 把自己的目标写得很清楚：检测并忽略 JS 篡改、收集不一致、使用难伪造的 API。公开测项包括 contentWindow、CSS 系统/计算样式、JS Math、引擎 console 错误、emoji/DomRect、SVG、Audio、Canvas（image/blob/paint/text/emoji）、TextMetrics、WebGL / GPU、Fonts、Voices、Screen，以及 **Resistance（已知伪装模式）** 和 Device-of-Timezone。[CreepJS README](https://github.com/abrahamjuliot/creepjs)

Pixelscan 的打分算法**未公开**。其一手说明只写检查 UA 完整性、OS 一致性、Canvas/WebGL/渲染信号，不要把第三方博客当成它的方法论。[pixelscan.net](https://pixelscan.net/manifest)

### 8.3 网络栈与自称不符

- JA3/JA4 属于 Python TLS 或旧 Chrome，UA 却是最新 Chrome
- HTTP/2 SETTINGS 不像该大版本
- 开了 HTTP/3 但 QUIC 指纹不像 Chrome
- TLS 是 Chrome，JS 环境却被改得像 Firefox

**用真 Chrome 做运行时，这一项天然过关。** 用 HTTP 客户端去“模拟浏览器”，这一项天然不过关。

### 8.4 群体异常

风控很少只看单次哈希。它看：

- 同一出口 IP 上出现几十个“独一无二”的 GPU
- 同一 Canvas 哈希配完全不同的 UA
- 账号刚注册，设备画像却像一台用了三年的老机器，或反过来
- 行为：瞬时填写、完美直线鼠标、无焦点变化

Persona 再自洽，也替代不了正常使用节奏。

---

## 9. 开源与商业实现对照（只记已核实事实）

| 项目 | 实际机制 | 可借鉴 | 不可误读 |
| --- | --- | --- | --- |
| Brave farbling | 内核级、每会话每 eTLD+1 种子微扰 Canvas / Audio 等 | 确定性噪声、分表面 | 目标是反追踪，不是稳定店铺身份 |
| Tor / RFP | 统一输出，扩大匿名集 | “少即是多” | 多账号会撞车 |
| juu17 shuffler | MV3，多数 hook 在 ISOLATED | 根种子、按 origin 派生的**概念** | 对页面 JS 基本无效；噪声有状态 |
| OpenBrowser | Electron + CDP + 独立内核路径 | 字段分组、设备 Persona | 完整能力依赖其内核，不是 Stock Chrome 清单 |
| VirtualBrowser | Web UI + native bridge | 交互分组 | 公开仓库缺引擎，字段≠已实现 |
| XChrome | Chrome + 每 Profile + 本地控制面 | 拓扑形状 | CDP Persona / 许可证不适合复用 |
| BrowserForge | 约束式指纹数据生成 | UA 与头、屏幕、硬件联动采样 | 注入器已弃用 |
| Camoufox | 打过补丁的 Firefox | 内核读 typed config | 不是 Chrome 路径 |
| Pota Browser | Stock Chromium 管理器 | 诚实边界：深层图形要 C++ patch | 不要把它的 README 当成“已能改 Canvas” |
| BotBrowser | 定制 Chromium + profile + noise seed | 管线级噪声、跨表面同一颗种子 | 第三方 fork，信任与许可另审 |
| AdsPower / Multilogin / GoLogin | 厂商内核 + 控制面 | 字段分组、默认 Masked、强调一致性 | 营销句不能当本仓库能力 |

AdsPower 一手材料可再精确一层（仍是厂商声明，不是本仓库证据）：SunBrowser = Chromium/Chrome，FlowerBrowser = Firefox；公开主张“选 Chrome X 就跑 Chrome X”，因而 TLS / HTTP/2 / Canvas 跟自称版本对齐；WebRTC 的 Forward 是强制走 Google 公共 STUN，Replace 是把 ICE 改成代理 IP，Disabled 拦流量但不拆 API。Local API 里 `canvas` 0=主机 / 1=噪声，`webgl` 0=主机 / 2=自定义 / 3=仅创建时随机匹配。[AdsPower help](https://help.adspower.com/docs/browser_fingerprint) [Local API](https://localapi-doc-en.adspower.com/docs/Awy6Dg)

Multilogin：Mimic 是仍在更新的 Chrome 内核，Stealthfox 是冻结在 v146 的遗留 Firefox 内核；创建后浏览器类型不可改。他们**没有**公开 Blink / BoringSSL 补丁点。Canvas 默认 Real，Noise 是相对主机 GPU 的滤镜，换机器哈希会变。[Mimic vs Stealthfox](https://multilogin.com/help/en_US/how-to-use-mimic-and-stealthfox)

---

## 10. 失败模式（改指纹最常见的死法）

1. **改了隔离世界，以为改了网站。** 扩展默认 ISOLATED，页面读到的仍是真值。
2. **只改 JS，不改头。** `navigator.userAgent` 是 A，`User-Agent` / `Sec-CH-UA` 是 B。
3. **有状态噪声。** 同一页面读两次 Canvas 得两个哈希。
4. **把噪声写回画布。** 破坏页面功能，也暴露 hook。
5. **Worker 未覆盖。** 站点把探测放到 Worker / OffscreenCanvas。
6. **时序竞态。** 页面在 `document_start` 脚本跑完前同步读取。
7. **代理开了，WebRTC 没关非代理 UDP。** ICE 直接报真实公网 / 局域网。
8. **时区跟着操作系统，IP 跟着代理。**
9. **移动 UA + 桌面 GPU + 桌面屏幕 + 无触摸。**
10. **声称改了 TLS。** Stock Chrome 改不了；乱接第三方 TLS 库只会让 JA3 不再像 Chrome。
11. **同一工作站 50 个 Identity 共用一个出口、一套 Canvas 真值，却在文案上写“每套环境独一无二”。**
12. **为了过检测器关掉 WebGL / Canvas。** 正常用户几乎不会这么做，缺失本身就是信号。

---

## 11. 和本仓库的对应关系

本文是机制说明。产品当前承诺以 [`PRODUCT.md`](../PRODUCT.md) 为准。对照如下，避免把研究写成功能清单：

| 机制 | RealBrowser 当前态度 |
| --- | --- |
| Profile 隔离 | 必做：每 Identity 完整非默认 User Data + 应用锁 + 进程所有权 |
| Persona | schema v5 + 稳定种子；可编辑项仅限运行时真正映射的字段 |
| 浏览器运行时 | 只认带清单和哈希的 RealBrowser Chromium；缺失即 fail closed，不发现/启动/调和 Google Chrome |
| 语言 / 窗口 / WebRTC 策略 | 编进产品 Chromium 启动参数 |
| 时区 | Rust 校验并发布完整 IANA 目录；Persona Runtime 在初始顶层 frame 观测，并对新 tab/page 重放 |
| Viewport / Screen / DPR | Rust 作为一个原子配置校验；Persona Runtime 应用、观测并对新 tab/page 重放 |
| Canvas 2D | K1 在 `getImageData` / `toDataURL` / `toBlob` / OffscreenCanvas 回读副本上做种子化幂等微扰；观测后才是 `CustomKernel` |
| UA / WebGL / WebGPU / Audio / 字体 / 电池 / 插件 | 本轮显式 Native；不做 JS hook |
| TLS | 只读引擎事实，不是可编辑字段 |
| 代理 | Direct，或一条固定 HTTP/HTTPS/SOCKS5；无凭据、无轮换；代理时必须关掉非代理 UDP |
| Cookie / 密码 / 2FA | 产品 Chromium 不透明拥有；不读、不导、不迁移 |
| 不可检测 / 反关联 | 明确非目标 |

K2 以后不属于本轮。WebGL、Audio、TLS 或 JS hook 若要进入产品，必须另开范围、覆盖矩阵与原生验收；不能从 K1 的 Canvas 证据外推。

---

## 12. 术语对照

| 本文用语 | 本仓库正式词 | 不要说成 |
| --- | --- | --- |
| 指纹浏览器 | Fingerprint Browser | 反检测浏览器、Ozon 浏览器 |
| 一套环境 | Browser Identity | 账号、实例、指纹 |
| 数据目录 | Browser Profile | 窗口、标签 |
| 可观察特征集 | Browser Persona | 随机指纹、噪声档 |
| 改指纹实现 | Persona Runtime | 指纹插件、stealth 补丁 |
| 出口 | Network Egress | IP 身份、动态 IP |
| 字段是否真的生效 | Persona Capability | 打开即可、尽力伪装 |

---

## 13. 主要来源

学术与标准：

- [W3C Mitigating Browser Fingerprinting in Web Specifications](https://w3c.github.io/fingerprinting-guidance/)
- [Eckersley, How Unique Is Your Web Browser?, PETS 2010](https://www.freehaven.net/anonbib/papers/pets2010/p1-eckersley.pdf)
- [Mowery & Shacham, Pixel Perfect: Fingerprinting Canvas in HTML5, W2SP 2012](https://hovav.net/ucsd/dist/canvas.pdf)
- [Laperdrix et al., Beauty and the Beast / AmIUnique, IEEE S&P 2016](https://inria.hal.science/hal-01285470v2/document)
- [Laperdrix et al., Browser Fingerprinting: A Survey, ACM TWEB 2020](https://dl.acm.org/doi/10.1145/3386040)
- [Gómez-Boix et al., Hiding in the Crowd, WWW 2018](https://dl.acm.org/doi/10.1145/3178876.3186097)
- [Vastel et al., FP-STALKER, IEEE S&P 2018](https://inria.hal.science/hal-01652021/document)
- [Cao, Li, Wijmans, Cross-Browser Fingerprinting, NDSS 2017](https://yinzhicao.org/TrackingFree/crossbrowsertracking_NDSS17.pdf)
- [Fifield & Egelman, Fingerprinting Web Users Through Font Metrics, FC 2015](https://doi.org/10.1007/978-3-662-47854-7_7)
- [RFC 8828 WebRTC IP Handling](https://www.rfc-editor.org/rfc/rfc8828.html#section-5.2)
- [CreepJS](https://github.com/abrahamjuliot/creepjs)
- [WICG User-Agent Client Hints](https://wicg.github.io/ua-client-hints/)
- [RFC 8942 Client Hints](https://datatracker.ietf.org/doc/html/rfc8942)

浏览器与协议一手：

- [Chrome User-Agent Client Hints](https://developer.chrome.com/docs/privacy-security/user-agent-client-hints)
- [CDP Emulation domain](https://chromedevtools.github.io/devtools-protocol/tot/Emulation/)
- [Chrome content scripts / ExecutionWorld](https://developer.chrome.com/docs/extensions/develop/concepts/content-scripts)
- [Chrome 136 remote debugging + user-data-dir](https://developer.chrome.com/blog/remote-debugging-port)
- [Chromium user data directory](https://chromium.googlesource.com/chromium/src/+/HEAD/docs/user_data_dir.md)
- [Chromium network settings](https://www.chromium.org/developers/design-documents/network-settings/)
- [Chrome Enterprise WebRtcIPHandling](https://chromeenterprise.google/policies/web-rtc-ip-handling/)
- [Chromium command-line switches](https://peter.sh/experiments/chromium-command-line-switches/)
- [Brave Fingerprinting defenses 2.0 / Farbling](https://brave.com/privacy-updates/4-fingerprinting-defenses-2.0/)

网络栈指纹：

- [Salesforce JA3](https://github.com/salesforce/ja3)
- [FoxIO JA4](https://github.com/FoxIO-LLC/ja4)
- [Cloudflare JA3/JA4 fingerprints](https://developers.cloudflare.com/bots/additional-configurations/ja3-ja4-fingerprint/)
- Akamai HTTP/2 client fingerprint（SETTINGS / WINDOW_UPDATE / PRIORITY / 伪头顺序）

产业与实现（当厂商声明读，不当本仓库证据）：

- [Fingerprint.com technique overview](https://fingerprint.com/blog/browser-fingerprinting-techniques/)
- [Fingerprint.com audio fingerprinting](https://fingerprint.com/blog/audio-fingerprinting/)
- [AdsPower: What is browser fingerprint](https://help.adspower.com/docs/browser_fingerprint)
- [Multilogin Fingerprint section](https://multilogin.com/help/en_US/profile-settings-fingerprint-section)
- [GoLogin: How browser fingerprints work](https://gologin.com/docs/how-browser-fingerprints-work)
- [GoLogin profile fingerprint settings](https://support.gologin.com/en/articles/14810056-profile-fingerprint-settings)
- [BotBrowser Canvas guide](https://github.com/botswin/BotBrowser/blob/main/docs/guides/fingerprint/CANVAS.md)

本仓库已有审阅：

- [`.scratch/fingerprint-browser/research/persona-settings-open-source.md`](../.scratch/fingerprint-browser/research/persona-settings-open-source.md)
- [`.scratch/fingerprint-browser/research/juu17-fingerprint-shuffler.md`](../.scratch/fingerprint-browser/research/juu17-fingerprint-shuffler.md)
- [`.scratch/fingerprint-browser/research/chrome-control-constraints.md`](../.scratch/fingerprint-browser/research/chrome-control-constraints.md)
- [`.scratch/fingerprint-browser/research/virtualbrowser-xchrome.md`](../.scratch/fingerprint-browser/research/virtualbrowser-xchrome.md)
