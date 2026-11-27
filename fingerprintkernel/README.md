# 指纹核心

RealBrowser 维护的独立 Go 模块，供 RealBrowser 的 Go 桌面工程和图来共用。两端都导入 `github.com/xxww0098/realbrowser/fingerprintkernel`。本地联调时，Go 桌面工程引用 `./fingerprintkernel`，图来引用相邻的 `../realbrowser/fingerprintkernel`。模块拥有独立的 `go.mod`，只依赖 Go 标准库、XZ 解压库和系统文件锁。两个应用分别提供自己的数据目录。

模块可以在 RealBrowser 仓库使用 `fingerprintkernel/vX.Y.Z` 标签独立发布。图来在 `go.mod` 锁定模块版本，CI 按版本标签或 Go 伪版本对应的提交检出。原 `browserkernel` 模块的 `v0.1.0` 仍可按原路径获取。迁移时将 Go 导入路径的最后一段及包名改为 `fingerprintkernel`，现有指纹配置可以直接沿用。

| 用途 | 接口 |
| --- | --- |
| 查询发行信息与当前平台的可安装更新 | `KernelManager.FetchLatestRelease`、`FetchInstallableRelease`、`CheckUpdate` |
| 下载、校验、安装和切换 | `KernelManager.Update`、`ActivateRelease`、`ActiveRelease` |
| Node 宿主的镜像测速、分段下载和续传 | `downloader/download-watch.mjs` 的 `downloadArchive` |
| 使用锁定的首次安装包 | `PinnedRelease`、`PinnedBuild.Asset`、`PinnedBuild.Unpack` |
| 编辑和持久化指纹详情 | `FingerprintSpec`、`ImportFingerprintJSON`、`ExportFingerprintJSON`、`ReadProfile`、`EditProfile`、`WriteProfile` |
| 启动、等待、终止子进程和登记活动实例 | `StartProcess`、`Process`、`Registry` |

有版本选择界面时，先获取发行版，再把选中的 `ReleaseInfo` 交给 `ActivateRelease`。后台自动升级可以直接调用 `Update`。检查和自动升级都选择当前平台可安装、带 SHA-256 校验信息的最高稳定版本，跳过空的 XZ 归档。自动选包时，文件名须标明匹配的处理器架构。macOS 也接受标明 `universal` 或 `universal2` 的构建。模块内已锁定的构建可通过平台、版本、文件名和内置摘要确认。

更新器校验下载内容后才写入 `active.json`。版本保存在同一根目录的 `versions/` 下，已有进程可继续使用旧文件。已有安装的版本号可作为 `current` 参数传入，更新器会比较版本并避免降级。

`ReadProfile` 读取资料目录中的 `.browserkernel.json`。修改语言、时区等单项偏好时，使用 `EditProfile(ctx, dir, edit)`。它持有同一把文件锁读取最新配置并保存，回调返回错误会保留原文件。`WriteProfile` 可整份替换当前版本定义的字段。两种写法都会保留磁盘中较新版本写入的扩展字段，包括已知对象中的扩展字段。使用 SQLite 的宿主可直接保存同一个模型或使用其 JSON 接口。

在本目录运行 `go test -race ./...` 可验证下载、解压、指纹资料与版本切换。运行 `node --test downloader/*.test.mjs` 可单独验证多镜像传输。

要验证受管浏览器能渲染正文，可将 `REALBROWSER_TEST_BINARY` 设为浏览器可执行文件的绝对路径，在本目录运行 `go test -count=1 -run '^TestManagedChromiumRendersLocalContent$' .`。macOS 还需将 `REALBROWSER_TEST_LAUNCHER` 设为临时资料目录专用的 `chromium-test` 启动器路径。测试通过本机 HTTP 服务分别打开动态正文和 403 错误页，每次使用独立临时资料目录。
