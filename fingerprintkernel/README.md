# fingerprintkernel

`fingerprintkernel` 是 RealBrowser 维护的 Go 指纹核心。Go 应用只需提供一个可写的内核安装目录和每个环境各自的浏览器资料目录（profile），就能通过它下载和启动指纹浏览器。应用通过同一个包读取和编辑指纹，模块负责校验安装包、保存配置并将配置应用到下次启动。

## 安装

需要 Go 1.27.3 或更高版本。在 Go 项目中运行以下命令即可引入本文示例使用的版本。如果从空目录试用，先在该目录运行 `go mod init example.com/fingerprint-demo`。

```sh
go get github.com/xxww0098/realbrowser/fingerprintkernel@v0.0.0-20261127150039-05a134703390
```

`go get` 安装 Go 依赖。浏览器本体在首次调用 `Open` 或 `Ensure` 时准备。

## 快速开始

将下面的程序保存为 `main.go`，在项目目录运行 `go run .`。程序会打开一个空白页，关闭浏览器后返回进程的退出码。

```go
package main

import (
	"context"
	"log"
	"os"
	"path/filepath"

	"github.com/xxww0098/realbrowser/fingerprintkernel"
)

func main() {
	browser := fingerprintkernel.ManagedBrowser{Dir: filepath.Join("data", "kernel")}
	profileDir := filepath.Join("data", "profiles", "demo")
	process, err := browser.Open(context.Background(), profileDir, "about:blank")
	if err != nil {
		log.Fatal(err)
	}
	os.Exit(process.Wait())
}
```

`ManagedBrowser.Dir` 可以由多个环境共用。对同一环境重复传入相同的 `profileDir`，会复用已保存的指纹和浏览器资料。创建另一个环境时，只需换一个资料目录。`Open` 可以接收零个或多个启动 URL，每个 URL 都须是绝对地址。

`Open` 成功后返回 `*fingerprintkernel.Process`。示例中的 `Wait()` 会阻塞到浏览器退出。桌面应用可以向 `process.OnExit` 传入 `func(code int)` 回调接收退出通知，也可以调用 `process.Terminate(fingerprintkernel.DefaultKillGrace)` 关闭进程。传给 `Open` 的 `context.Context` 控制准备和启动阶段，启动后的进程由调用方管理。

## 读取和编辑指纹

调用 `fingerprintkernel.ReadProfile(profileDir)` 可取得该环境的 `FingerprintSpec`。资料文件不存在时，返回零值和 `nil`。若要在首次启动前生成并保存完整指纹，调用 `fingerprintkernel.EnsureProfile(ctx, profileDir, fingerprintkernel.EnsureOptions{})`。已有指纹会被复用，这一步可以离线完成。

例如，在下次打开该环境时使用中文界面和上海时区，可以只修改这两个字段。

```go
func setLocale(ctx context.Context, profileDir string) error {
	return fingerprintkernel.EditProfile(ctx, profileDir, func(f *fingerprintkernel.FingerprintSpec) error {
		f.Lang = "zh-CN"
		f.Timezone = "Asia/Shanghai"
		return nil
	})
}
```

`EditProfile` 在资料目录的写锁内读取最新配置，校验后一次性写回。回调返回错误或校验失败时，原配置保留。时区使用 IANA 名称，窗口宽高需要成对设置。

| 使用场景 | 公开接口 |
| --- | --- |
| 设置面板只编辑语言、时区、窗口大小和配色 | 用 `ReadDisplayPreferences(profileDir)` 读出当前值，修改后调用 `prefs.Save(ctx, profileDir)` |
| 从预设创建指纹 | 从 `CommonFingerprintPresets` 选择模板，调用 `preset.Fingerprint()`，再用 `WriteProfile(profileDir, spec)` 保存 |
| 将指纹作为 JSON 导入或导出 | `ImportFingerprintJSON(raw)`、`ExportFingerprintJSON(spec)` |
| 将指纹保存在应用自己的数据库中 | 在同一事务内读出原始 JSON，用 `ImportFingerprintJSON` 解析并修改，再用 `MergeFingerprintJSON(original, spec)` 合并后写回，保留原有扩展字段 |

## 下载和管理浏览器内核

只需打开浏览器时，快速开始中的 `Open` 已包含安装步骤。有下载进度或版本选择界面的应用，可以单独调用这些接口。

| 使用场景 | 调用 | 返回结果 |
| --- | --- | --- |
| 查看本机是否已有可用内核 | `browser.Installed()` | `(InstalledRelease, bool)`，只读取本机状态 |
| 提前下载，或离线复用已有安装 | `browser.Ensure(ctx, progress)` | `InstalledRelease` 的 `Version` 和 `Executable` 给出版本和可执行文件路径 |
| 安装当前平台兼容的最新稳定内核 | `browser.Update(ctx, progress)` | 下次启动使用的 `InstalledRelease`，同版本复用已有安装 |

`Ensure` 和 `Update` 除了 `InstalledRelease` 还会返回 `error`。可选的 `progress` 是 `func(done, total int64)`，以字节为单位，`total <= 0` 表示总大小未知。不显示进度时传 `nil`。

首次在线安装优先使用模块锁定的 fingerprint-chromium 构建，目前覆盖 macOS（Apple Silicon）、Windows（x64）和 Linux（x86-64）。其他平台会尝试选择兼容的稳定发行包，找不到时返回错误。模块通过 SHA-256 校验并安装成功后才切换当前版本，失败时可继续使用原有安装。[预置构建清单](./pinned.go)

离线首次安装时，在 `Open` 或 `Ensure` 前设置 `browser.Pinned.Archive = archivePath`，指向与预置构建匹配的归档。如果应用已有验证过的浏览器，可以设置 `browser.Base = fingerprintkernel.InstalledRelease{Version: version, Executable: exePath}` 来复用它。

如果界面需要先展示版本再安装，可设置 `browser.Manager = fingerprintkernel.NewKernelManager("", "")`，然后调用 `browser.Manager.AvailableUpdate(ctx, currentVersion)`。首次安装时 `currentVersion` 可传空字符串。返回 `nil, nil` 表示没有可用更新。有候选版本时，用 `release.TagName` 展示版本号，用户确认后将同一份 `release` 传给 `browser.Install(ctx, release, progress)`。

完整的类型、方法签名和其他启动选项可在调用方项目中通过 `go doc` 查看。

```sh
go doc github.com/xxww0098/realbrowser/fingerprintkernel.ManagedBrowser
go doc github.com/xxww0098/realbrowser/fingerprintkernel.FingerprintSpec
```
