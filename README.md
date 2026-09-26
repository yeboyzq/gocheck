# gocheck

`gocheck` 检查 Go 项目 `go.mod` 中所有**直接依赖**的可用更新，包括 Go v2+ 大版本模块更新，并尽可能复用本机 Go 私有仓库配置。

## 功能特性

- **直接依赖精准识别**：直接以目标项目 `go.mod` 的非 `// indirect` require 为来源，不把间接依赖误报为直接依赖。
- **大版本更新提示**：探测 `/v2` 到 `/v10` 等独立模块路径，并用 `major` 标记破坏性升级。
- **私有仓库支持**：优先使用本机 `GOPROXY`、`GOPRIVATE`、`GONOSUMDB` 和认证配置；查询失败时回退到 Git。
- **只读检查**：不运行 `go get`、`go mod tidy` 或 `go mod download`，不修改目标项目的 `go.mod` / `go.sum`。
- **并发查询**：默认 8 个并发检查，可通过参数调整。
- **机器可读输出**：内置 JSON 输出，便于脚本和 CI 使用。

## 安装

### go install

```bash
go install github.com/yeboyzq/gocheck@latest
```

安装后二进制位于 `$(go env GOPATH)/bin/gocheck`。请确认该目录已在 `PATH` 中。

### 源码编译

```bash
git clone https://github.com/yeboyzq/gocheck.git
cd gocheck
go build .
```

## 使用

在目标 Go 项目目录执行：

```bash
gocheck
```

或指定项目目录：

```bash
gocheck --dir /path/to/your/go/project
```

### 参数

| 参数 | 说明 | 默认值 |
|---|---|---|
| `--dir` | 目标 Go 项目目录 | 当前目录 |
| `--all` | 显示全部直接依赖；默认仅显示有更新或查询失败的依赖 | `false` |
| `--json` | 输出 JSON | `false` |
| `--concurrency` | 并发检查数量，必须为正整数 | `8` |

pflag 也接受 `-dir`、`-all` 这类单横线写法；文档统一推荐 `--dir`。

每次外部 `go list` / Git 查询的超时时间为 10 秒。

## 项目结构

```text
.
├── main.go                 # 模块根入口，保证 go install github.com/yeboyzq/gocheck@latest 可用
├── app/
│   ├── main.go             # 应用入口
│   ├── cmd/                # Cobra 命令定义
│   ├── modules/            # 依赖解析、版本查询、Git 回退与输出模块
│   └── utils/              # 外部命令执行等通用工具
├── .github/workflows/       # CI 与 Release 自动化
└── go.mod
```

仓库根入口是薄封装，实际命令逻辑位于 `app/cmd`。这样目录保持清晰的分层，同时满足 Go module 根包必须存在 main 包的 `go install` 约束。

### 表格输出

```text
PACKAGE                    | CURRENT | LATEST | NOTE
example.com/module         | v1.9.1  | v1.10.0
example.com/other          | v1.7.0  | v1.8.1
example.com/versioned      | v9.3.0  | v9.6.1
example.com/breaking       | v1.2.3  | v2.0.0 | major
example.com/preview        | v2.0.0-rc.1 | v2.0.0-rc.2 | pre-release
example.com/local-replace  | local   |        | replace
```

查询失败的依赖会始终显示，并在 `NOTE` 中包含 `error`，避免静默漏报。

如果上游只有预发布版本，`go list @latest` 返回的预发布版本会正常参与比较，并标记 `pre-release`；这不算查询失败。默认输出仍只包含有更新的依赖，可用 `--all` 查看已经是最新的预发布依赖。

### JSON 输出

```bash
gocheck --json --all
```

```json
{
  "dependencies": [
    {
      "path": "github.com/example/module",
      "current": "v1.2.3",
      "latest": "v2.0.0",
      "update_available": true,
      "major": true,
      "replace": false,
      "pre_release": true,
      "error": ""
    }
  ]
}
```

默认 JSON 只包含有更新或查询失败的依赖；`--all` 包含全部直接依赖。

### 退出码

- `0`：扫描完成，且所有依赖查询成功。
- `1`：参数错误、目标项目缺少/无法解析 `go.mod`，或至少一个依赖查询失败。

表格和 JSON 写入标准输出；错误信息写入标准错误。

## 私有仓库配置

gocheck 会继承用户现有 Go 环境配置。使用私有模块时，可按需配置：

```bash
export GOPRIVATE=example.company/*
export GOPROXY=https://your-proxy.example.com,direct
```

如需 Git 回退查询，请确保 Git 具备非交互认证能力，例如 SSH key、credential helper 或 `insteadOf` 配置。gocheck 会禁用 Git 交互式密码提示。

### 查询策略

1. 在独立临时空模块中执行 `go list -m -mod=readonly -json <module>@latest`，避免修改目标项目。
2. 若 `go list` 失败，先解析模块路径的 `go-import` 元数据；再尝试从模块路径推导 Git 仓库地址。
3. 使用 `git ls-remote --tags --refs` 获取远程 tag，并选择同主版本下最新的正式 semver tag。
4. v1 仅支持 Git 回退；Mercurial、Subversion 和 Bazaar 会返回不支持。

## 发布

模块路径为 `github.com/yeboyzq/gocheck`，主包位于仓库根目录，因此可直接安装：

```bash
go install github.com/yeboyzq/gocheck@latest
```

发布正式版时推送语义化版本 tag：

```bash
git tag v0.1.0
git push origin v0.1.0
```

也可以直接在 GitHub 网页上创建 Release 并新建 `v0.1.0` tag。该 tag 出现在仓库后，同样会触发本项目的 Release workflow。

CI 在 push 和 pull request 时运行测试、vet 和构建。推送 `v*` tag 后，Release workflow 会构建 Linux、macOS、Windows 的 amd64/arm64 产物并创建 GitHub Release。

如果 tag 对应的 GitHub Release 已经存在，例如你先在网页上创建了 Release，workflow 不会重复创建同名 Release，而是把构建产物上传到已有 Release 并覆盖同名文件。

### 验证 Go 模块索引

tag 推送到 GitHub 后，在项目根目录执行：

```bash
./verify-module-index.sh v0.1.0
```

脚本会完成：

1. 验证 GitHub tag 是否存在；
2. 主动请求 Go 模块代理拉取该版本；
3. 校验 `.info`、`.mod` 和版本列表；
4. 在干净临时环境中执行 `go install github.com/yeboyzq/gocheck@<version>`；
5. 运行安装后的 `gocheck --help` 做冒烟测试；
6. 请求并验证 `pkg.go.dev` 版本页面是否已收录。

`pkg.go.dev` 收录可能存在延迟。默认最多等待 300 秒；如需调整：

```bash
PKG_GO_DEV_TIMEOUT_SECONDS=600 ./verify-module-index.sh v0.1.0
```

脚本默认使用官方模块代理：

```text
https://proxy.golang.org
```

如果你的网络无法访问官方代理，可以改用可访问的代理做拉取验证：

```bash
PROXY_BASE=https://goproxy.cn ./verify-module-index.sh v1.0.0
```

注意：`goproxy.cn` 只能证明该代理可以拉取模块，不能替代 `proxy.golang.org` 与 `pkg.go.dev` 的官方索引验证。如果本机无法访问这两个站点，建议在 GitHub Actions 或其他可访问网络中执行完整脚本。

## 环境要求

- 检查目标项目时需要本机安装 Go。
- 仅当私有模块查询进入 Git 回退路径时，需要本机安装 Git。

## 许可证

[Mulan PSL v2](LICENSE)
