# browserkernel

RealBrowser 维护的独立 Go 模块，供 RealBrowser 的 Go 桌面工程和图来共用。两端都导入 `github.com/xxww0098/realbrowser/browserkernel`。本地联调时，Go 桌面工程引用 `./browserkernel`，图来引用相邻的 `../realbrowser/browserkernel`。模块拥有独立的 `go.mod`，只依赖 Go 标准库、XZ 解压库和系统文件锁。两个应用分别提供自己的数据目录。

模块可以在 RealBrowser 仓库使用 `browserkernel/vX.Y.Z` 标签独立发布。图来在 `go.mod` 选择模块版本，源码联调使用相邻目录，CI 默认按版本标签检出。

| 用途 | 接口 |
| --- | --- |
| 查询当前平台的稳定版本与更新 | `KernelManager.FetchLatestRelease`、`CheckUpdate` |
| 下载、校验、安装和切换 | `KernelManager.Update`、`ActivateRelease`、`ActiveRelease` |
| Node 宿主的镜像测速、分段下载和续传 | `downloader/download-watch.mjs` 的 `downloadArchive` |
| 使用锁定的首次安装包 | `PinnedRelease`、`PinnedBuild.Asset`、`PinnedBuild.Unpack` |
| 编辑和持久化指纹详情 | `FingerprintSpec`、`ImportFingerprintJSON`、`ExportFingerprintJSON`、`ReadProfile`、`WriteProfile` |
| 启动、等待、终止子进程和登记活动实例 | `StartProcess`、`Process`、`Registry` |

有版本选择界面时，先获取发行版，再把选中的 `ReleaseInfo` 交给 `ActivateRelease`。后台自动升级可以直接调用 `Update`。更新要求发行包具有 SHA-256，校验成功才写入 `active.json`。版本保存在同一根目录的 `versions/` 下，已有进程可继续使用旧文件。已有安装的版本号可作为 `current` 参数传入，更新器会比较版本并避免降级。

`ReadProfile` 和 `WriteProfile` 读写指定资料目录中的 `.browserkernel.json`。编辑部分字段时，先读取当前记录，再修改并保存。使用 SQLite 的宿主可直接保存同一个模型或使用其 JSON 接口。

在本目录运行 `go test -race ./...` 可验证下载、解压、指纹资料与版本切换。运行 `node --test downloader/*.test.mjs` 可单独验证多镜像传输。
