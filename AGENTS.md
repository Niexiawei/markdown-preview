# AGENTS.md

本仓库是一个**纯 Go 的桌面 markdown 预览器**：多项目管理，左侧 md 文件树，右侧「文档目录 + 正文」，
正文可在原生渲染（`ui/markdown`）与 WebView 渲染（`ui.WebView`）之间切换；外部改动（agent / 编辑器）
实时跟随；另带一个默认关闭的本地 MCP server。

UI 框架是 `github.com/egoist/mygo v0.9.1`（原生 UI 路径，**不用** web 前端、不需要 bun）。
设计与实现计划在 `docs/superpowers/`。

## 命令

```sh
go run .                    # 开发运行（会打开窗口）
go build -o mp.exe .        # 出单个二进制
go test ./...               # 全部测试（视图测试用 ui.NewTester，无窗口）
go test ./internal/view/ -run TestTOC -v
go vet ./... && gofmt -l .   # 提交前必跑：vet 无输出、gofmt 无输出
```

Go 版本下限由 `go.mod` 的 `go 1.27.3` 决定（mygo v0.9.1 要求）；本机旧版本时 `GOTOOLCHAIN=auto` 会自动取工具链。

## 结构

| 路径 | 职责 |
| --- | --- |
| `main.go` | 组装：配置 → workspace → 窗口 → 文件监听 → MCP |
| `internal/config` | 配置读写（项目、活动项目、渲染模式、MCP），原子写 `config.json` |
| `internal/workspace` | 扫描 / 读取 / 解析缓存 / 标题缓存 / TOC 与 slug / 搜索；`Index` 接口 |
| `internal/watch` | `mygo.WatchFiles` → `ChangeSet`（新增 / 删除 / 重命名 / 写入） |
| `internal/mcp` | `POST /mcp` JSON-RPC 2.0，四个只读工具 |
| `internal/view` | 原生 UI：工具条、树、目录、两种预览、状态栏、MCP 设置面板 |

依赖方向：`view → workspace`、`mcp → workspace`、`workspace → config`。

## 硬规则

- **不引入第三方 markdown / GFM / 高亮 / web 框架依赖。** 解析与 HTML 都用 `ui/markdown/parser`
  （`parser.Parse` + `Document.HTML()`）。
- **`internal/workspace` 与 `internal/mcp` 不得 import `.../ui`**（只允许 `.../ui/markdown/parser`）。
  这样 MCP 能用 `httptest` 测，不需要图形环境。
- **一份解析，两种后端。** 原生喂源字符串给 `markdown.View`，webview 喂同一个 `*parser.Document` 的
  `HTML()`；绝不为 webview 另引解析器，否则两模式渲染会不一致。
- **视图状态归窗口线程。** 视图是 state 的纯函数；其他 goroutine 只通过 `State.Post(fn)` 投递，
  帧首 `drain()` 应用，然后 `win.Invalidate()` 唤醒。不要在 view 热路径里读盘或解析。
- **配置与状态不落 `localStorage`**，一律写 `App.Path(PathUserData)` 下的 JSON。
- **MCP server 默认关闭**；只监听 `127.0.0.1`（`Start` 会拒绝非 loopback 地址）；无
  `Authorization: Bearer <token>` 一律 401。
- **测试只用 `t.TempDir()`**，不得读写真实用户目录。
- 不写平台专用代码（不加 `//go:build windows` 之类），三平台保持可编译。

## 改动指引

- **加一个 MCP 工具**：在 `internal/mcp/tools.go` 的 `tools()` 里加一项（schema + run），
  并在 `server_test.go` 里断言它是只读（`readOnlyHint: true`）且 `inputSchema.type == "object"`；
  同时更新 README 的表格。
- **改忽略规则**：只改 `workspace.Ignored`（扫描与监听共用它），并补 `TestIgnoredRules`。
- **改 TOC 缩进/交互**：`view/native.go` 的 `buildTOC` + `tocItems`。
- **改渲染样式**：原生在 `markdown.Options`（`loadDoc` 里装配）；webview 在 `webview.go` 的 `docCSS`。
- **换掉 `ui.Tree`**：md 项超过约 3000 时 `ui.Tree` 会构建全部节点，应换成 `ui.Outline`。

## 已知的坑

- **slug 必须与 mygo 一致。** `workspace.Slug` 是 `ui/markdown.Slug` 的逐字复制（小写、空格逐个→`-`、
  保留 `-`/`_`/字母数字、不折叠不去尾、重复标题不去重），因为 `Options.Anchor` 按它匹配标题。
  自造规则会让 TOC 点击跳不到。
- **`Options.Anchor` + `AnchorVersion` 要成对变**：同一个标题点第二次时只有 `AnchorVersion` 递增才会再滚。
- **webview 是 make once**：不在视图函数里创建，首次切到 webview 时 `win.NewWebView` 并缓存，视图只
  `ui.WebView(c, wv)`。切回原生只停止构建，不 `Destroy`。
- **`mdview://` 只服务已打开项目内的文件**：`mdViewHandler.allowed` 会 `EvalSymlinks` 再判前缀，
  任何越界一律 403；新增读取入口要走同一判断。
- **缓存失效按 `(mtime, size)`**（`docEntry` / `titleEntry`）：改了读取逻辑要一起改失效条件，
  否则会出现「文件变了但标题/正文不变」。
- **Windows 文件名限制**：测试里不要造含 `?` `*` 的文件名（`os.WriteFile` 会直接报错）；
  `os.Symlink` 需要管理员权限，相关用例会 `t.Skip`。
- **webview 重载丢滚动位置**（原生模式保留），已写进 README「已知限制」。

## 提交

中文提交信息，前缀 `feat:` / `fix:` / `test:` / `docs:` / `style:`。每个行为改动都先有失败的测试。
