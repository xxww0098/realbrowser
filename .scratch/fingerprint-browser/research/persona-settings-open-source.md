# Persona / 指纹设置：开源实现与 MR-0 建议

> 研究快照：2026-08-15。目标是稳定、隔离、可解释的浏览器 Persona；不以规避网站风控或“不可检测”为产品承诺。

## 结论

当前 `browser-persona` 已经列出了较完整的表面，但模型过于扁平：`SurfacePolicy::{Native, Stable}` 同时承担“用户意图、实现方式、运行能力”三件事，无法表达字段值、覆盖范围和启动后的真实状态。MR-0 不应继续增加同类布尔式字段，而应拆成：

1. `PersonaSpec`：用户保存的意图；
2. `PersonaExecutionPlan`：按当前 Chrome 与后端能力编译出的启动参数、CDP 操作、策略和明确拒绝项；
3. `PersonaObservation`：启动后实际观测值、覆盖范围和失败原因。

实现边界必须公开：Stock Chrome 启动参数只覆盖 profile、代理、窗口和少量策略；UA/UA-CH、时区、语言、地理位置、设备指标宜通过 CDP；MV3 主世界注入可覆盖部分页面 JavaScript 表面，但存在 frame/worker/执行时序和被页面干扰的边界；图形、音频、字体、WebRTC/网络栈的深层一致性需要定制浏览器内核。Chrome 官方分别记录了 [用户数据目录](https://chromium.googlesource.com/chromium/src/+/HEAD/docs/user_data_dir.md)、[代理启动配置](https://www.chromium.org/developers/design-documents/network-settings/)、[CDP Emulation 能力](https://chromedevtools.github.io/devtools-protocol/tot/Emulation/) 与 [扩展脚本执行世界](https://developer.chrome.com/docs/extensions/develop/concepts/content-scripts)。

## 已核实项目

| 项目 | 真实角色 | 可借鉴内容 | 不应误读 |
|---|---|---|---|
| browser-fingerprint-shuffler | MV3、`document_start`、全 frame 的页面注入扩展 | origin 稳定 seed、Canvas/WebGL/Audio/Navigator 分模块 | 不是 Chromium 内核能力；页面主世界、worker 与网络层不能由这些 hook 完整覆盖。源码：[manifest](https://github.com/juu17/browser-fingerprint-shuffler/blob/02ae61bed7404e6c2d150d6eb343b2c057dedc58/manifest.json#L1-L46)、[seed bootstrap](https://github.com/juu17/browser-fingerprint-shuffler/blob/02ae61bed7404e6c2d150d6eb343b2c057dedc58/content/bootstrap.js#L1-L35)、[Canvas](https://github.com/juu17/browser-fingerprint-shuffler/blob/02ae61bed7404e6c2d150d6eb343b2c057dedc58/content/hooks_canvas.js#L1-L62)、[WebGL](https://github.com/juu17/browser-fingerprint-shuffler/blob/02ae61bed7404e6c2d150d6eb343b2c057dedc58/content/hooks_webgl.js#L67-L93) |
| OpenBrowser | Electron 管理端 + CDP/脚本 + 独立第三方内核路径 | profile/privacy 分组、桌面/移动设备 Persona、静态/动态字段拆分 | 其完整能力依赖独立 Wayfern/OpenBrowser 内核，不是 stock Chrome 字段清单。源码：[profile schema](https://github.com/lyu0805/OpenBrowser/blob/cb9842a8b0d63475d96f7dd3b9948b949996c501/Browserapp/engine.js#L350-L503)、[device persona](https://github.com/lyu0805/OpenBrowser/blob/cb9842a8b0d63475d96f7dd3b9948b949996c501/Browserapp/automation/device-personas.js#L19-L271)、[内核要求](https://github.com/lyu0805/OpenBrowser/blob/cb9842a8b0d63475d96f7dd3b9948b949996c501/Browserapp/engine.js#L654-L684)、[启动/CDP](https://github.com/lyu0805/OpenBrowser/blob/cb9842a8b0d63475d96f7dd3b9948b949996c501/Browserapp/engine.js#L1814-L1930) |
| VirtualBrowser | Web 管理 UI + native bridge | 表单分组、环境管理交互 | 公开仓库的 native 后端不完整，UI 字段不能当作可运行证明。源码：[表单](https://github.com/Virtual-Browser/VirtualBrowser/blob/d47736b5d66fc5f641b57f56df2942aa9162d7e8/server/src/views/browser/index.vue#L1217-L1340)、[native bridge](https://github.com/Virtual-Browser/VirtualBrowser/blob/d47736b5d66fc5f641b57f56df2942aa9162d7e8/server/src/api/native.js#L15-L127)、[外部 automation binary](https://github.com/Virtual-Browser/VirtualBrowser/blob/d47736b5d66fc5f641b57f56df2942aa9162d7e8/automation/README.md#L21-L25) |
| BrowserForge | 一致性 Persona/headers 数据生成器 | 约束式数据模型、UA 与 headers 联动、屏幕/硬件/语言等相关采样 | 注入器已被项目标为 deprecated，适合借鉴模型，不应作为新运行时基础。源码：[生成模型](https://github.com/daijro/browserforge/blob/be7a953b36b83c00012963dc9cdb87ca5c1d948a/browserforge/fingerprints/generator.py#L23-L100)、[UA/headers 联动](https://github.com/daijro/browserforge/blob/be7a953b36b83c00012963dc9cdb87ca5c1d948a/browserforge/fingerprints/generator.py#L130-L210)、[弃用说明](https://github.com/daijro/browserforge/blob/be7a953b36b83c00012963dc9cdb87ca5c1d948a/README.md#L521-L526) |
| Camoufox | 带 C++ patch 的定制 Firefox | typed properties、内核读取配置、跨表面一致性思路 | 不是 Chrome/MV3 路径；能力来自源码补丁。源码：[定制内核说明](https://github.com/daijro/camoufox/blob/7add1ef554bbbca237b9eaf1c1e0610d116c2f21/README.md#L217-L235)、[typed properties](https://github.com/daijro/camoufox/blob/7add1ef554bbbca237b9eaf1c1e0610d116c2f21/settings/properties.json#L1-L108)、[C++ MaskConfig](https://github.com/daijro/camoufox/blob/7add1ef554bbbca237b9eaf1c1e0610d116c2f21/additions/camoucfg/MaskConfig.hpp#L1-L94) |
| Simprint | Tauri/Rust 管理端 + 专用浏览器运行时 | TS/Rust 双端 typed config、硬件 profile 一致生成、environment ID 传递 | Rust 主要负责序列化和启动，字段生效依赖专用内核/IPC。源码：[字段类型](https://github.com/Simprint/simprint/blob/8d24b350ef716af16f819ee7b503c0a58181b184/plugins/pages/create-window/src/types/index.ts#L84-L126)、[一致硬件 profile](https://github.com/Simprint/simprint/blob/8d24b350ef716af16f819ee7b503c0a58181b184/plugins/pages/create-window/src/utils/fingerprint-generator.ts#L155-L348)、[Rust config](https://github.com/Simprint/simprint/blob/8d24b350ef716af16f819ee7b503c0a58181b184/src-tauri/src/infrastructure/runtime/api.rs#L8-L58)、[专用启动参数](https://github.com/Simprint/simprint/blob/8d24b350ef716af16f819ee7b503c0a58181b184/src-tauri/crates/runtime/src/services/environment/kernel/launcher.rs#L343-L384) |
| Pota Browser | Rust Chromium profile/代理/CDP 管理器 | stock Chromium 最小真实边界 | 项目明确说 Canvas/WebGL/Audio 深层修改需要 C++ Chromium patch。源码：[实际模型](https://github.com/snaberino/pota-browser/blob/b23f8e084617a011701aed8432ad64bdc46993ac/src/chromium/chromium_manager.rs#L17-L29)、[实际启动参数](https://github.com/snaberino/pota-browser/blob/b23f8e084617a011701aed8432ad64bdc46993ac/src/chromium/chromium_manager.rs#L166-L200)、[边界声明](https://github.com/snaberino/pota-browser/blob/b23f8e084617a011701aed8432ad64bdc46993ac/readme.md#L54-L66) |
| rustcloak | Rust/Tauri 管理器 + CloakBrowser 定制引擎 | 小而清晰的 profile/seed/proxy/language/timezone 模型 | 深层能力属于外部 CloakBrowser 引擎；本仓库不是 stock Chrome 实现。源码：[模型](https://github.com/izzipizzy/rustcloak/blob/1fd49ab50de4f087d720e8a0cd756bc419b3c97c/crates/core/src/model.rs#L26-L59)、[引擎启动](https://github.com/izzipizzy/rustcloak/blob/1fd49ab50de4f087d720e8a0cd756bc419b3c97c/crates/core/src/launch.rs#L4-L43) |

未找到能够唯一确认用户所称 `fingerprint-browser` 的仓库，因此不把同名或近似仓库纳入证据。

## 字段矩阵与真实实现边界

图例：`可靠` 表示该层有对应官方接口；`有限` 表示仅能覆盖部分可观察面；`—` 表示不应由该层承诺。

| 分组 / 字段 | Stock 启动参数/策略 | CDP | MV3 主世界注入 | 定制内核 | MR-0 决策 |
|---|---:|---:|---:|---:|---|
| profile / cookies / storage 隔离 | 可靠 | — | — | 可 | 必做：每 Identity 独立 `--user-data-dir`；Chrome 136 起 remote debugging 也要求非默认目录，[官方说明](https://developer.chrome.com/blog/remote-debugging-port) |
| proxy | 可靠 | 有限 | — | 可 | 属于 Egress，不放 Persona；使用 Chrome 代理配置，[官方设计](https://www.chromium.org/developers/design-documents/network-settings/) |
| 窗口大小 | 可靠 | `setDeviceMetricsOverride` | 有限 | 可 | 保存 window 与 viewport 的区别；MR-0 启动参数生效，观测验证，[CDP](https://chromedevtools.github.io/devtools-protocol/tot/Emulation/#method-setDeviceMetricsOverride) |
| UA + UA-CH + platform | 粗略 | 可靠/部分 experimental | 有限 | 可 | 一组原子配置；优先保持 Native，Custom 时由 `setUserAgentOverride` 同时设置 metadata，[CDP](https://chromedevtools.github.io/devtools-protocol/tot/Emulation/#method-setUserAgentOverride) |
| locale / languages / Accept-Language | `--lang` 只覆盖一部分 | 可设置 locale、UA override 的 acceptLanguage | 有限 | 可 | 拆为 `primary_locale` + 有序 languages；同时验证 HTTP header 与 JS，[CDP](https://chromedevtools.github.io/devtools-protocol/tot/Emulation/#method-setLocaleOverride) |
| timezone | 不应以跨平台 `TZ` 当作完整保证 | 可靠 | 有限 | 可 | 改为 CDP `setTimezoneOverride`；启动后观测，[CDP](https://chromedevtools.github.io/devtools-protocol/tot/Emulation/#method-setTimezoneOverride) |
| geolocation + permission | — | 坐标可靠；权限另管 | 页面 API 可有限包装 | 可 | 用 tagged enum 表达 denied/prompt/coordinates/egress；不可再用 `Stable`，[CDP](https://chromedevtools.github.io/devtools-protocol/tot/Emulation/#method-setGeolocationOverride) |
| screen / viewport / DPR / touch | 窗口有限 | 可靠/部分 experimental | 有限 | 可 | Stage 1 CDP；保持数值约束，[CDP](https://chromedevtools.github.io/devtools-protocol/tot/Emulation/) |
| hardwareConcurrency | — | experimental | 有限 | 可 | Stage 2 或 `Native`；CDP 有专用接口但需版本/平台验收，[CDP](https://chromedevtools.github.io/devtools-protocol/tot/Emulation/#method-setHardwareConcurrencyOverride) |
| deviceMemory | — | — | 有限 | 可 | MR-0 `Native`，不声称可用 |
| Canvas | — | — | 有限 | 可靠 | MR-0 `Native`；扩展阶段只能标“页面有限覆盖”，Juu17 的做法是包装 `getImageData/toDataURL/toBlob`，[源码](https://github.com/juu17/browser-fingerprint-shuffler/blob/02ae61bed7404e6c2d150d6eb343b2c057dedc58/content/hooks_canvas.js#L1-L62) |
| WebGL image / vendor / renderer | — | — | 有限 | 可靠 | 拆成 image 与 metadata；vendor+renderer 为原子对；MR-0 `Native`，[源码示例](https://github.com/juu17/browser-fingerprint-shuffler/blob/02ae61bed7404e6c2d150d6eb343b2c057dedc58/content/hooks_webgl.js#L67-L93) |
| WebGPU | — | — | 极有限 | 可靠 | MR-0 `Native`；不提供 `Stable` 假选项 |
| Audio | — | — | 有限 | 可靠 | MR-0 `Native`；Juu17 只包装部分 AudioBuffer/Analyser API，[源码](https://github.com/juu17/browser-fingerprint-shuffler/blob/02ae61bed7404e6c2d150d6eb343b2c057dedc58/content/hooks_audio.js#L1-L31) |
| fonts / ClientRects | — | — | 有限 | 可靠 | MR-0 `Native`；Stage 2 分字段、分 coverage |
| media devices / speech voices | — | — | 有限，且受权限/设备状态影响 | 可 | MR-0 为 Native + permission policy；不可生成虚假设备列表 |
| plugins / battery | — | — | 有限 | 可 | Chrome 现代行为优先 Native；从 MR-0 UI 主表移到高级能力状态 |
| DNT / GPC | 策略/首选项可能覆盖 | — | 有限 | 可 | 用户隐私偏好，使用 `Off/On`，绝不随机或 `Stable` |
| WebRTC IP policy | flag/企业策略可部分控制 | — | — | 可 | 属于 Network/Privacy；用可验证策略值，[Chrome policy](https://chromeenterprise.google/policies/web-rtc-ip-handling/) |
| QUIC / DNS | 策略/网络配置 | Network domain 仅观测/控制部分 | — | 可 | 属于 Egress，不放 Persona；SOCKS/DNS 边界见 [Chromium 文档](https://new.chromium.org/developers/design-documents/network-stack/socks-proxy/) |
| TLS/SSL 指纹 | — | — | — | 可靠 | 从 Persona 可编辑字段删除；stock Chrome 网络栈为事实，不承诺任意 TLS Persona |

### 为什么 MV3 只能标“有限”

Chrome content script 默认为 isolated world；若使用 `MAIN` world 才能影响页面看到的对象，但会与页面共享执行环境，页面也能干扰或识别修改。`document_start` 与 `all_frames` 能减小时序/frame 漏洞，却不等于 worker、网络 header 和浏览器内部实现全覆盖。依据：[content scripts](https://developer.chrome.com/docs/extensions/develop/concepts/content-scripts)、[`ExecutionWorld`](https://developer.chrome.com/docs/extensions/reference/api/scripting#type-ExecutionWorld)。

因此能力状态至少要包含 `top_frame / all_frames / dedicated_worker / shared_worker / service_worker / network` 六个 coverage 位；没有自动附着与实测之前，不得显示“已应用”。

## 对当前 `browser-persona` 的具体增删建议

当前代码是 [`crates/browser-persona/src/lib.rs`](../../../crates/browser-persona/src/lib.rs)：顶层含 locale/timezone/window/WebRTC，加上 17 个统一的 `SurfacePolicy`。建议如下。

### MR-0：现在就改

**删除/降级**

- 删除通用 `SurfacePolicy::Stable`。稳定性是 seed 与派生算法属性，不是 Canvas、TLS、权限、DNT 共用的“值”。
- 从可编辑 Persona 中移除 `tls`；放进只读 `EngineObservation.network_stack = StockChrome`。
- 将 `headers` 移除为大杂烩；拆成 `user_agent`、`languages`，其余 headers 由浏览器/网络栈原生生成。
- 将 `navigator`、`hardware` 拆成具名字段，不能让 UI 显示“Navigator 稳定”却不知道设置了什么。
- `plugins`、`battery` 从 MR-0 主设置移入“高级 / 原生”，不提供不可执行选择。
- `webrtc` 移到 `NetworkPrivacySpec`；proxy、DNS、QUIC 同属 Egress/网络，不混进 Persona。
- 停止把进程环境变量 `TZ` 视为 Persona 已应用；跨平台以 CDP timezone override + 页面观测为准。

**新增/改型**

```rust
#[derive(Clone, Debug, Serialize, Deserialize)]
#[serde(tag = "mode", rename_all = "snake_case")]
pub enum ValuePolicy<T> {
    Native,
    Preset { preset_id: String },
    Custom { value: T },
}

pub struct PersonaSpecV2 {
    pub schema_version: u16,
    pub persona_id: Uuid,
    pub revision: u64,
    pub seed: [u8; 32],
    pub browser: BrowserIdentitySpec,
    pub region: RegionSpec,
    pub display: DisplaySpec,
    pub graphics: GraphicsSpec,
    pub media: MediaSpec,
    pub privacy: PrivacyPreferenceSpec,
}

pub struct RegionSpec {
    pub languages: ValuePolicy<Vec<LanguageTag>>,
    pub timezone: ValuePolicy<IanaTimezone>,
    pub geolocation: GeolocationPolicy,
}

pub struct DisplaySpec {
    pub window: WindowSize,
    pub viewport: ValuePolicy<Viewport>,
    pub screen: ValuePolicy<ScreenMetrics>,
}

pub struct PersonaExecutionPlan {
    pub launch_args: Vec<LaunchArg>,
    pub managed_policies: Vec<ManagedPolicy>,
    pub cdp_actions: Vec<CdpAction>,
    pub extension_actions: Vec<ExtensionAction>,
    pub fields: Vec<FieldApplication>,
}

pub struct FieldApplication {
    pub field: PersonaField,
    pub backend: BackendKind,
    pub support: SupportLevel,
    pub coverage: Coverage,
}
```

MR-0 `GraphicsSpec`/`MediaSpec` 可以把所有字段配齐，但每个字段默认且只能选择 `Native`；UI 同时显示后端能力。这样“设置项齐全”不等于伪造“能力齐全”。真正可执行的 MR-0 字段应限定为：独立 profile、window、语言、时区、可选 UA/UA-CH、地理位置/权限、WebRTC policy；CDP 接口以官方 [Emulation domain](https://chromedevtools.github.io/devtools-protocol/tot/Emulation/) 为准。

**必须建立的一致性约束**

- Chrome engine major、UA major、UA-CH brands/platform 必须一致。
- `languages[0]`、`navigator.language(s)`、HTTP `Accept-Language` 必须一致。
- timezone、geolocation 选择“随出口”时，只能来自同一次已验证 egress snapshot。
- screen/available/viewport/window/DPR 数值必须可实现；`avail <= total`，DPR 大于零。
- OS、fonts、voices、touch、hardware、GPU preset 必须来自同一平台族。
- WebGL vendor/renderer 是不可拆的组合。
- seed 按 `schema_version/persona_id/surface/property` 域分离派生；默认重启不旋转，按 origin 稳定只能是显式模式。
- DNT/GPC 是用户选择，不参与随机生成。

### 阶段 2：运行时能力

- 增加签名 MV3 扩展 adapter；Canvas、WebGL、Audio、ClientRects、fonts、media/voices 分别声明 `Mv3MainWorld` 与精确 coverage，不使用一个总开关。
- CDP controller 对新 target 自动 attach/reapply；未覆盖 worker 时状态为 `BestEffort`，不是 `Applied`。
- preset 绑定 engine family/major；升级 Chromium 后先运行兼容性测试再迁移。
- 只有决定维护定制 Chromium 后才开放 `CustomKernel` 模式；adapter 通过同一个 capability contract 编译，不能让 UI 猜后端。
- BrowserForge 可用于 coherent preset 生成思路；其项目已弃用注入器，因此只借鉴约束和数据模型，[一手说明](https://github.com/daijro/browserforge/blob/be7a953b36b83c00012963dc9cdb87ca5c1d948a/README.md#L521-L526)。

## UI 分组

继续使用弹窗/侧边抽屉，不做永久左右分栏。文案保持短：

1. **基础**：预设、系统、浏览器版本、seed（高级只读/重建）。
2. **地区**：语言、时区、地理位置。
3. **屏幕与硬件**：窗口、viewport、screen、DPR、CPU/内存/touch。
4. **图形**：Canvas、WebGL 图像、GPU vendor/renderer、WebGPU。
5. **媒体与隐私**：Audio、fonts、media devices、voices、ClientRects、权限、DNT/GPC。
6. **网络**：只展示已绑定 Egress、WebRTC/QUIC/DNS 状态；编辑跳转到网络抽屉。

每行只需字段名、当前值、能力徽标：`内核`、`CDP`、`扩展·有限`、`原生`、`不可用`。不放教学段落；一致性错误用一行短提示并禁用保存。OpenBrowser 的字段分组可作信息架构参考，但它的独立内核要求必须同时参考，[源码](https://github.com/lyu0805/OpenBrowser/blob/cb9842a8b0d63475d96f7dd3b9948b949996c501/Browserapp/engine.js#L654-L684)。

## 可验证验收

### Schema / 编译器

- v1→v2 迁移：现有 `Native` 保持 Native；任何旧 `Stable` 不静默降级，迁移成 `UnavailableMigration` 并要求用户确认。
- JSON round-trip、未知 schema version 拒绝、typed newtype 范围测试。
- property-based tests 覆盖上面所有一致性约束。
- `PersonaSpec + RuntimeCapabilities -> ExecutionPlan` 快照测试；不可用字段 fail closed。
- seed 派生：同 persona/版本/字段结果稳定，不同字段不共享有状态 PRNG。

### 真实浏览器

- 两个 Identity 使用不同 `user-data-dir`，并发启动无锁冲突，cookies/local storage/history 不串；目录语义依据 [Chromium 官方文档](https://chromium.googlesource.com/chromium/src/+/HEAD/docs/user_data_dir.md)。
- 重启同一 Identity，所有声明稳定的观测值一致；更换 Identity 不要求“全部不同”，只要求符合 preset 与 seed 契约。
- 观测页同时采集顶层、同源 iframe、跨源 iframe、dedicated/shared/service worker；按 `Coverage` 比较，而不是只测顶层页面。
- UA/UA-CH/platform、语言/Accept-Language、timezone、geo、screen/viewport 做 JS + CDP + 网络 header 交叉验证。
- 每创建一个新 target 都验证 CDP override 已重放；失败时启动结果标 `Degraded/Failed`，不显示“已应用”。
- 禁用扩展后所有 MV3 字段必须回到 Native；不能残留或误报。
- proxy/WebRTC/DNS/QUIC 单独做 egress 验收，断开代理必须 fail closed，不能只以启动参数存在作为通过。
- Windows 与 macOS 分别用固定 Chrome major 做 smoke test；升级 major 时重新运行 capability/observation 套件。

## Rust 工程建议

- Rust 2024 edition；公开 adapter 枚举可 `#[non_exhaustive]`，crate 内部仍保持 exhaustive match。
- `serde` + `schemars` 生成前后端一致 schema；前端类型由 schema 生成，不再手写第二份 `PersonaConfig`。
- `uuid`、`semver`、`url`、`unic-langid`/`language-tags`、`chrono-tz` 负责领域类型，不保存任意字符串。
- `blake3` 做域分离派生，`rand_chacha` 做可复现采样；seed 使用 256 bit，不继续使用可碰撞的 `u32`。
- `proptest` 验证组合约束，`insta` 固化 execution plan，`thiserror` 保持可机器识别错误码。
- 启动参数使用 typed `LaunchArg`，敏感 proxy credential 不进入日志、错误文案或普通 JSON Persona。

## 许可证与复用风险

| 项目 | 许可证证据 | 商业软件建议 |
|---|---|---|
| browser-fingerprint-shuffler | [ISC](https://github.com/juu17/browser-fingerprint-shuffler/blob/02ae61bed7404e6c2d150d6eb343b2c057dedc58/LICENSE#L1-L13) | 可复用需保留声明；实现覆盖弱，优先重写并建立测试 |
| BrowserForge | [Apache-2.0](https://github.com/daijro/browserforge/blob/be7a953b36b83c00012963dc9cdb87ca5c1d948a/LICENSE#L1-L5) | 可借鉴/复用时保留 LICENSE、NOTICE 与变更说明；不要采用已弃用 injector |
| Camoufox | [MPL-2.0](https://github.com/daijro/camoufox/blob/7add1ef554bbbca237b9eaf1c1e0610d116c2f21/LICENSE#L1-L22) | 有文件级源码义务且为 Firefox patch；当前只借鉴架构/模型 |
| OpenBrowser | [根 MIT](https://github.com/lyu0805/OpenBrowser/blob/cb9842a8b0d63475d96f7dd3b9948b949996c501/LICENSE)，但含 [AGPL 第三方模块](https://github.com/lyu0805/OpenBrowser/blob/cb9842a8b0d63475d96f7dd3b9948b949996c501/Browserapp/THIRD-PARTY-NOTICES.md#L1-L26) 与第三方内核 | 不把根许可证等同于整个分发物；逐文件/二进制审计后再复用 |
| Simprint | [AGPL-3.0](https://github.com/Simprint/simprint/blob/8d24b350ef716af16f819ee7b503c0a58181b184/LICENSE) | 闭源商业产品不要复制代码，除非满足 AGPL 或取得商业许可；可独立实现概念 |
| VirtualBrowser | [BSD-3-Clause](https://github.com/Virtual-Browser/VirtualBrowser/blob/d47736b5d66fc5f641b57f56df2942aa9162d7e8/LICENSE) | 可参考 UI，但公开仓库缺少核心 native backend，不能据此声明运行能力 |
| Pota Browser | 仓库快照没有许可证文件 | 在权利澄清前不复制代码，仅引用事实 |
| rustcloak | 仓库快照没有许可证文件，README 的 MIT 字样不足以替代明确授权 | 在许可证与 CloakBrowser 引擎条款澄清前不复制代码 |

许可证结论只用于工程分流，不替代法律意见。当前最安全路径是：独立实现 typed schema、capability compiler 与验收工具；仅从 permissive 项目吸收抽象思路，并保存第三方来源清单。

## 推荐落地顺序

1. 先把当前 v1 扁平模型迁移成 `Spec / ExecutionPlan / Observation`，删除 `TLS Stable` 等假能力。
2. 用 CDP 补齐 locale/timezone/UA/geo/device metrics，建立 new-target 重放与 observation 页面。
3. UI 展示所有字段，但只开放 runtime capability 真正支持的 mode。
4. 再做受控 MV3 扩展，并以 frame/worker 覆盖矩阵验收。
5. 最后评估是否值得维护定制 Chromium；此前 Canvas/WebGL/Audio 等保持 Native，不阻塞正常电商多开。
