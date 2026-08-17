# Juu17 Browser Fingerprint Shuffler 源码审阅

审阅日期：2026-08-13  
固定快照：[`02ae61bed7404e6c2d150d6eb343b2c057dedc58`](https://github.com/juu17/browser-fingerprint-shuffler/commit/02ae61bed7404e6c2d150d6eb343b2c057dedc58)，提交时间 2026-06-21 UTC  
证据边界：源码与一方资料静态审阅；未安装扩展、未运行检测页、未登录任何真实 Seller Platform。

## 结论

不要把该扩展直接用于真实 Seller Platform，也不要把它当作“页面看到稳定混淆指纹”的证据。可以借鉴的只有两个小概念：**每个 Browser Identity 持有持久随机种子**，以及**从该种子派生有版本的 per-origin 值**。实现应独立重写并先定义一致性契约。

固定源码存在决定性语义缺口：Manifest 没有声明 `world` 或 `all_frames`，所以 Chrome 默认把 content script 放在 `ISOLATED` world 且仅注入顶层 frame；因此 Canvas、Audio 与 Navigator 原型修改只影响扩展自己的 JavaScript 世界，不影响普通网页脚本。只有 WebGL 另外插入了 page-world 文件，但它要等待异步 `chrome.storage.local`、一次“before”采样和 DOM script 加载，网页可以在补丁安装前读取原始值。没有 site worker 覆盖。

即使忽略覆盖缺口，噪声也不是稳定、内部一致的 Persona：一个共享、可变 PRNG 被各 API 调用按调用顺序消耗；Canvas 会改写调用方画布；Audio `getChannelData()` 改成返回副本；WebGL 对所有数值参数固定加 2，并让 vendor 后缀随读取次序变化。这些行为既改变网页语义，也可能让同一页面内的重复读取不一致。

## 一方声明与仓库边界

作者在 [2025-12-08 的 X 主帖](https://x.com/Juu17__/status/1998032917164097666) 中声明其方案是纯净 Chrome、不同 User Data 目录和自写扩展；这证明方案描述来自作者，但不能证明隔离、兼容性或不读取凭据。仓库目前只有两个提交；固定快照没有 tag、release、CI 或自动化测试。根许可证为 [ISC](https://github.com/juu17/browser-fingerprint-shuffler/blob/02ae61bed7404e6c2d150d6eb343b2c057dedc58/LICENSE)，法律上允许宽松复用，但许可证兼容不等于技术适用。

[`manifest.json`](https://github.com/juu17/browser-fingerprint-shuffler/blob/02ae61bed7404e6c2d150d6eb343b2c057dedc58/manifest.json) 是 Manifest V3，权限只有 `storage`，content script 匹配 `<all_urls>` 并在 `document_start` 运行；两个 page script 被声明为所有 URL 可访问的 web-accessible resources。它没有 background/service worker、网络权限或打包更新配置。当前源码没有外传数据的调用路径，但“不会盗密码”仍需对每个固定版本、分发物和更新渠道建立供应链证据，不能从作者声明推出。

## 实际执行模型

Chrome 官方说明静态 content script 默认运行在 [`ISOLATED` world](https://developer.chrome.com/docs/extensions/develop/concepts/content-scripts#work_in_isolated_worlds)，对该世界原型的改动不会改变页面自己的 JavaScript 环境；`world` 缺省值也是 `ISOLATED`。官方文档同时说明，只有设置 `all_frames: true` 才覆盖匹配的子 frame；本 Manifest 未设置它，也未设置 `match_about_blank` 或 `match_origin_as_fallback`。

| 表面 | 固定源码实际行为 | 对网页的实际覆盖判断 |
| --- | --- | --- |
| 种子 | [`salts.js`](https://github.com/juu17/browser-fingerprint-shuffler/blob/02ae61bed7404e6c2d150d6eb343b2c057dedc58/core/salts.js) 在扩展 `storage.local` 保存 128-bit salt；读取/写入失败时生成只存内存的新 salt。[`bootstrap.js`](https://github.com/juu17/browser-fingerprint-shuffler/blob/02ae61bed7404e6c2d150d6eb343b2c057dedc58/content/bootstrap.js) 默认用 `location.origin` 派生 seed。 | 存储正常时可跨重启稳定；失败时静默变值。是扩展/Browser Profile 范围的 salt，不是由 Rust 显式拥有、版本化和审计的 Browser Persona。 |
| Canvas | [`hooks_canvas.js`](https://github.com/juu17/browser-fingerprint-shuffler/blob/02ae61bed7404e6c2d150d6eb343b2c057dedc58/content/hooks_canvas.js) 改写 `getImageData`、`toDataURL`、`toBlob`，并在序列化时把噪声像素写回原画布。 | 只改 ISOLATED world；普通页面原型不受影响。即使迁到 MAIN，写回会改变页面数据，重复调用还继续消耗 PRNG，破坏幂等性。 |
| Audio | [`hooks_audio.js`](https://github.com/juu17/browser-fingerprint-shuffler/blob/02ae61bed7404e6c2d150d6eb343b2c057dedc58/content/hooks_audio.js) 把 `AudioBuffer.getChannelData()` 的 live view 换成带噪副本。 | 只改 ISOLATED world；普通页面不受影响。若迁到 MAIN，会改变 Web Audio API 的可修改共享缓冲语义，可能破坏站点。 |
| Navigator | [`hooks_navigator.js`](https://github.com/juu17/browser-fingerprint-shuffler/blob/02ae61bed7404e6c2d150d6eb343b2c057dedc58/content/hooks_navigator.js) 在 `window.navigator` 实例上定义 CPU、内存、语言 getter。 | 只改 ISOLATED world；普通页面不受影响。也没有同步 HTTP headers、UA Client Hints、locale、时区、字体、设备或系统事实。 |
| WebGL | [`hooks_webgl.js`](https://github.com/juu17/browser-fingerprint-shuffler/blob/02ae61bed7404e6c2d150d6eb343b2c057dedc58/content/hooks_webgl.js) 既改 ISOLATED 原型，又通过 DOM 加载 [`webgl_page_patch.js`](https://github.com/juu17/browser-fingerprint-shuffler/blob/02ae61bed7404e6c2d150d6eb343b2c057dedc58/content/webgl_page_patch.js) 改 MAIN world；所有数值参数固定 `+ jitter`，vendor 字符串拼接伪后缀，扩展列表反转。 | 这是唯一能影响普通顶层页面脚本的表面，但安装异步、有抢跑窗口；数值能力可能变成浏览器无法兑现的值，vendor 每次读取依赖 PRNG 次序。 |
| Frames / workers | Manifest 没有 `all_frames` 等字段，也没有 worker instrumentation。 | 仅匹配页面的顶层 frame；不覆盖普通跨域/相关子 frame、dedicated/shared/service workers。 |

## “确定性”为什么不等于“稳定 Persona”

[`content_main.js`](https://github.com/juu17/browser-fingerprint-shuffler/blob/02ae61bed7404e6c2d150d6eb343b2c057dedc58/content/content_main.js) 先等待 storage bootstrap，然后在安装 hooks **之前**运行异步 fingerprint sampler，再依次安装 hooks。虽然 content script 声明为 `document_start`，这段异步流程仍给页面代码留下读取窗口。

一个 seed 只创建一个共享 PRNG。Canvas 像素数、Audio buffer 长度、WebGL/Navigator 调用先后都会推进同一个状态。因此输出取决于“页面先调用了什么、调用多少次”，而不是稳定的 `(persona version, origin, surface, property, input)`。这会造成：

1. 同源两个页面因调用路径不同而读到不同 vendor 后缀或噪声序列；
2. 同一属性重复读取可能变化；
3. 一个表面的额外调用会改变另一个表面的结果；
4. storage 故障时 seed 只在当前页面内存中存在，重载即可改变。

合格的 Browser Persona 应为每个表面和属性做独立派生，输出是纯函数，并对重复读取、frame、navigation、restart 和版本迁移建立测试。

## 安全、兼容性与供应链判断

- 正面：源码短、无第三方依赖、权限只有 `storage`，当前固定版本没有 Cookie API、native messaging 或网络外传实现，ISC 许可证清晰。
- 风险：`<all_urls>` 与全站 web-accessible resources 扩大攻击和兼容面；page-world 脚本可被页面观察/干扰；大量异常被吞掉，实际失效时用户只看到“网页没坏”；固定版本仍会无条件打印 before/after 摘要。
- 供应链：没有签名包、release、CI、测试或更新策略。手工加载 unpacked 扩展让源码可见，但无法自动建立“当前磁盘代码就是审阅提交”的证据。
- 验证：仓库的 `test_fingerprint*.js` 是自采样和 console 输出，不是有断言的测试；而且 Canvas/Audio/Navigator sampler 与补丁处于同一 isolated world，所以它可以显示“Before/After 变化”，却不能证明网页世界看到变化。

## 对 RealBrowser 的决定输入

**可吸收概念：**

1. 每个 Browser Identity 生成一次随机 root seed，明确保存、轮换和恢复语义；
2. 从 `(persona schema version, identity seed, origin, surface, property)` 独立派生确定值；
3. 以功能开关逐表面禁用，并把兼容性优先于变化数量；
4. 固定扩展版本、来源和 hash，保持权限最小。

**明确拒绝：**

1. 直接加载该扩展作为 MVP Persona；
2. 用 isolated-world 自测输出或检测站分数证明网页指纹已改变；
3. 共享有状态 PRNG、对 Canvas 原数据写回、改变 `getChannelData()` 语义、WebGL 所有数值统一偏移；
4. 声称覆盖所有 frame/worker 或声称“不可检测”；
5. storage 失败后静默生成临时身份。

**后续验证门槛：** 只有在受控扩展原型对 MAIN/ISOLATED、顶层/同源/跨域/相关 frame、dedicated/shared/service worker、早期 inline script，以及所选 Seller Platform 的登录跳转与商品/订单页面建立红绿测试后，才能决定是否对任何表面做页面级调整。默认策略应是保留 stock Chrome 真值。
