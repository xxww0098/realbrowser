# 内核级 Persona：RealBrowser K0+K1

> 实施决策：2026-08-15。K0+K1 已获准，本文是产品 Chromium 的实现合同。
>
> RealBrowser 只发现、启动和调和自己发行的 Chromium。删除 Stock Chrome 后备路径；产品内核缺失、清单/哈希/major 不匹配或 K1 观测失败都 fail closed。当前范围不含 WebGL、Audio、TLS 或 JS hook。

---

## 1. 先把目标说清楚

AdsPower 的「内核级」不是 Rust 里改几个字段，而是：

1. **自己发行一份 Chromium 构建**（RealBrowser），不是用户的 Google Chrome。
2. 在 **Blink / 网络栈的 C++** 里改可观察输出。
3. 启动时把该 Identity 的 Persona 读进渲染进程，所有 frame / worker 走同一套值。
4. 换大版本时连二进制一起换，避免 UA 写 150、V8/TLS 却像 140。

你要复制的是这个形状，不是他们的闭源补丁。公开可参照、且语义更接近「稳定 Persona」的开源先例：

| 先例 | 引擎 | 可学的 | 不要照抄的 |
| --- | --- | --- | --- |
| [Brave farbling](https://brave.com/privacy-updates/4-fingerprinting-defenses-2.0/) | Chromium C++ | 在 Blink 回读点改、确定性种子 | 每会话 / 每 eTLD+1 换种子（反追踪，破坏店铺连续性） |
| [Bromite canvas noise patch](https://github.com/bromite/bromite) | Chromium patch | `getImageData` / `toBlob` 回读处改像素 | 随机、非幂等 |
| [Camoufox MaskConfig](https://github.com/daijro/camoufox) | Firefox C++ | 内核读一份 typed JSON，各表面从同一配置派生 | Firefox 路径；电商后台是 Chrome 世界 |

本仓库正确的语义是：**每 Identity 一颗根种子，按表面域分离派生，重启不旋转**。不是 Brave，也不是每次启动随机。

---

## 2. 已通过的启动闸门

本轮决策已经选择维护产品 Chromium。代价仍需持续承担：

- 月更合入与安全补丁
- 签名 / 分发 / 编解码器许可
- 和本机 Chrome 已经免费得到的真 TLS / 真 H2

本轮锁定的闸门：

1. 固定官方 Chromium Stable tag；不从 Google Chrome 二进制或品牌分支派生。
2. 产物名称与 icon 都是 **RealBrowser**，由产品清单和 SHA-256 绑定。
3. Persona 仍走 `PersonaSpec → ExecutionPlan → Observation`；内核只是 `ExecutionBackend::CustomKernel`。
4. K0+K1 的真机矩阵未通过前，`graphics.canvas` 继续显示 Native。

---

## 3. 目标架构

```text
RealBrowser（Rust）
  PersonaSpec + seed
        │ 编译
        ▼
  KernelPersonaFile（JSON，无秘密）
        │ --realbrowser-persona-file=...
        │ 或单独 IPC / memory-mapped，启动即注入每个渲染进程
        ▼
  自有 Chromium 构建（仅 Chromium 一族）
        ├── Browser 进程：读文件、校验 schema、下发给 Renderer
        └── Renderer / Blink：
              K1 只在 Canvas 2D C++ 回读点改副本，JS 看到的仍是 native function
        └── WebGL / Audio / TLS / JS hook：本轮不改
```

原则：

- **Rust 仍拥有真相。** 内核是执行后端，不自己发明 Persona。
- **一份配置驱动所有表面。** 学 Camoufox，不要在 Blink 里散落 20 个命令行开关。
- **默认不改 TLS。** AdsPower 最硬的一致性其实来自「真内核版本」；乱改 ClientHello 会立刻不像 Chrome。
- **噪声只加在回读副本上**，不写回页面正在用的画布。
- **幂等：** 同一 Persona + 同一画布内容 → 永远同一哈希。禁止有状态 PRNG。

---

## 4. 工程上实际要建的东西

### 4.1 一份可启动的 Chromium 树

不要从 Google Chrome 分支切。用：

- 官方 [Chromium](https://www.chromium.org/developers/how-tos/get-the-code/) 对应 Stable 大版本的 tag，或
- [Chrome for Testing](https://developer.chrome.com/blog/chrome-for-testing) 的源码/二进制当基线（无自动更新，更新归你）

仓库布局建议（与本仓库隔离，不要塞进 `crates/`）：

```text
chromium-persona/          独立 git，子模块或 depot_tools 树
  patches/                 相对上游的 git format-patch
    0001-persona-file.patch
    0002-canvas-readback.patch
    ...
  persona/                 你加的小目录（C++ 配置解析 + 噪声）
tools/chromium-sync.sh     拉上游 tag、apply patches、gn/ninja
```

只跟 **一个大版本**。AdsPower 同时供 10 个大版本，那是他们的内核团队规模，不是起步形态。

### 4.2 配置协议（Rust ↔ 内核）

内核启动参数只加一个：

```text
--realbrowser-persona-file=/abs/path/persona.json
```

文件由 Rust 在锁 Profile 之后写出，权限仅当前用户，进程退出可留（便于崩溃复现）但不得含代理密码。示意：

```json
{
  "schema_version": 1,
  "persona_id": "…",
  "seed": "<32-byte hex>",
  "engine_major": 150,
  "surfaces": {
    "canvas": { "mode": "seeded_noise" }
  }
}
```

K0 schema 只接受本轮定义的 Canvas `seeded_noise`。出现未知字段、未知 schema、非绝对 Persona 文件路径或 `engine_major` 对不上当前二进制时，**拒绝启动**，不要静默降级。

种子派生（与现有 Persona 研究一致）：

```text
derived(surface, property) = BLAKE3(seed, schema, persona_id, surface, property)
```

Canvas 噪声用 `derived("graphics.canvas", "readback")` 做 HMAC 选像素，对同一输入图像永远同一输出。

### 4.3 补丁落点（只改回读，不改绘制）

这是「内核级」的技术含义。公开 Chromium 隐私补丁（Brave / Bromite）都走回读路径，而不是重写 Skia 画布。

| 表面 | 改哪里（概念） | 正确行为 | 错误行为 |
| --- | --- | --- | --- |
| Canvas 2D | Blink 里 `getImageData` / `toDataURL` / `toBlob` / OffscreenCanvas `convertToBlob` 把像素交给 JS 之前 | 复制 buffer，按种子微扰少量通道，返回副本 | hook JS 原型；写回原 canvas；每次调用换随机数 |
| WebGL 图像 | `readPixels`、把 WebGL 画到 2D canvas 再导出 | 同上，种子与 Canvas 域分离但同 Persona | 只改 `UNMASKED_RENDERER` 不改像素 |
| WebGL 元数据 | `getParameter` / `WEBGL_debug_renderer_info` | vendor+renderer 原子对，且和声称 OS 同族 | Intel + NVIDIA 混搭 |
| Audio | `AudioBuffer::getChannelData`、OfflineAudioContext 渲染完成 | 对副本做种子化 LSB 扰动 | 改变 `getChannelData` 语义（返回非视图） |
| ClientRects / measureText | 布局度量出口 | 极小、稳定的亚像素扰动或 Native | 大噪声导致点击/碰撞错位 |
| 字体列表 | Font cache / `document.fonts` | 按 Persona OS 暴露常见集合；缺字要有真实 fallback | JS 撒谎「有 Segoe」但栅格是 PingFang |
| hardwareConcurrency 等 | `Navigator` 的 C++ 绑定 | 与 CDP 同一套值，内核一次写死 | JS 改得到、Worker 里仍是主机值 |
| WebRTC ICE | 已有策略即可 | `disable_non_proxied_udp` | 在 Stock 上假装 Replace 成任意 IP |
| TLS / H2 | **默认不打补丁** | 保持该 Chromium 大版本的真 ClientHello | 做成可编辑 JA3 |

Worker / OffscreenCanvas / 跨源 iframe 必须走同一份 Persona，因为它们都在 Renderer 里读同一份进程级配置。这是内核相对 MV3 的唯一决定性优势。

不要一上来改网络栈。真 Chromium 150 的 JA3 就是产品资产。

### 4.4 和现有 Rust 控制面怎么接

`browser-control` 只有产品内核这一种浏览器后端，没有 Stock Chrome fallback。启动时：

1. 校验内核文件哈希 / 签名 / `engine_major`
2. 写 `persona.json`
3. 额外参数：`--user-data-dir`（仍然每 Identity 一份）、`--realbrowser-persona-file`、现有代理 / WebRTC / 窗口
4. 观测页交叉读顶层 + iframe + dedicated worker/OffscreenCanvas；对不上立即关闭本次进程，UI 不得显示 `CustomKernel`

28 项能力目录里，只有内核真正实现并测过的字段，`backend` 才能从 `Native` 变成 `CustomKernel`。其余继续只读。

### 4.5 构建、更新、验证

| 工作 | 最低标准 |
| --- | --- |
| 构建 | `gn gen` + 官方参数；关 Google API key 相关；要有组件更新策略（自己的或关闭） |
| 补丁 | `git format-patch` 对固定上游 tag；升级 = 换 tag + 重放补丁 + 解冲突 |
| 回归 | 每个表面：同 Persona 两次启动哈希相同；两 Persona 在声称要分的面上不同；iframe/worker 一致；`toString` 仍是 `[native code]` |
| 兼容 | 固定卖家后台走一遍登录（授权账号）；图表/验证码/文件上传不能坏 |
| 发布 | 签名构建、签名更新、SBOM；不能让用户去编 40GB Chromium |

团队规模心理预期：跟一个 Desktop 大版本，至少一名熟悉 Blink 的人持续合入。这不是「一个周末加开关」。

---

## 5. 分阶段交付（内核内部也要切）

即使决定做内核，也不要一次打完 AdsPower 字段表。

| 阶段 | 内核改什么 | 验收 |
| --- | --- | --- |
| K0 | 能启动的自有 Chromium + 读 `persona.json` + 拒绝 schema 不匹配 | 两个 Identity 仍是 Native 硬件面，但进程已是你的二进制 |
| K1 | Canvas 2D 回读种子噪声，含 OffscreenCanvas | 同机两 Identity Canvas 哈希稳定且不同；CreepJS 原型检测不过度异常 |
| K2 | WebGL `readPixels` + vendor/renderer 原子对 | 图像与元数据同族；iframe/worker 一致 |
| K3 | Audio 回读 | 幂等；不改变 Analyser 时序到页面可感知 |
| K4 | 字体 / ClientRects（很脆） | 布局测试 + 卖家页 |
| K5 | 仅当平台验收要求：TLS 微调（禁旧 cipher），仍不是任意 JA3 | 与声称 Chrome major 的 JA4 仍聚类为 Chrome |

K0 是唯一启动路径。产品内核不存在时停止在 `BrowserUnavailable`，不尝试查找本机浏览器。

---

## 6. 明确不要做的

- 在 JS 里包装原型，却在 UI 上写「内核级」。
- 把 Google Chrome 打补丁再分发（商标与许可）。
- 同时维护 Firefox 第二内核。
- 假 iOS/Android UA 配桌面 GPU。
- 可编辑 JA3 / 任意 ClientHello。
- 每站点、每会话换种子（Brave 语义）。
- 无观测就标「已应用」。
- 为了过检测器关掉 WebGL/Canvas。

---

## 7. 和本仓库现状的接法

本仓库保存 Rust 协议、产品内核清单/打包工具和固定上游 tag 的补丁；Chromium 工作树与构建输出位于仓库外。当前只交付 **K0+K1**。K2 以后必须单独立项。

完成证据必须来自同一台真机的两个 Identity：两个父进程均指向 RealBrowser 产品目录且版本输出以 `RealBrowser` 开头；同一 Identity 重启 Canvas 哈希不变，两个 Identity 不同；iframe、dedicated worker 与顶层一致；原生 API `toString` 仍含 `[native code]`。通过前能力目录保持 `graphics.canvas = Native`。
