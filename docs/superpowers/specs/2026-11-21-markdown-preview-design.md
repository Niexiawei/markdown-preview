# Markdown Preview 设计文档

日期：2026-11-21
状态：待批准

## 意图

我（使用者）经常让 agent 直接改仓库里的 `.md`（方案、笔记、计划）。改完我需要**立刻**看到效果，
而不是切到编辑器再切回来。同时将来希望 agent 能自己检索这些 md（"上次那个方案里怎么写的导出流程"），
所以现在就要把文档访问层留出干净的入口，而不是等接入时再去掏 UI 代码。

## 目标（本期）

1. **多项目管理**：打开一个目录成为一个项目；记住项目列表；顶部切换当前项目。
2. **文件树**：当前项目内递归列出 `.md` / `.markdown`，目录可折叠，点击即预览。
3. **预览**：右半区再分成左右两栏——左是文档目录（TOC），右是正文。
4. **双渲染后端**：正文既可走原生 `ui/markdown`，也可走 `ui.WebView`；底部状态栏用分段控件切换。
5. **实时跟随变更**：外部（agent / 编辑器）**新增、删除、重命名、写入** md 时，树与当前正文自动跟上。
6. **MCP 骨架**：本地 MCP server（HTTP），4 个只读工具，默认关闭，设置里可开。

## 非目标（本期明确不做）

- 编辑 md、保存写回（本应用是只读预览器）
- 全文检索的 UI（搜索只作为 MCP 工具存在）
- 多文档标签页 / 多窗口
- 非 md 文件预览（图片、代码、PDF）
- TOC 滚动跟随（scroll spy）——点击 TOC 能跳，但滚动正文不会高亮目录项
- 代码块语法高亮（原生 `ui/markdown` 的 `DefaultCodeBlock` 不带高亮；本期接受单色等宽）
- 应用打包、签名、自动更新

## 反目标（技术上能做，但做了就等于失败）

- **两套 markdown 解析器**。原生与 webview 必须来自同一份 `parser.Document`，否则同一文档两种模式
  渲染结果不一致，会被当成 bug 反复排查。
- **把项目列表/偏好放 `localStorage`**。webview 模式的 origin 与原生状态不同源，`mygo dev` 与打包后
  也不同源；配置必须落在 Go 侧的 `App.Path(PathUserData)`。
- **在视图构建里做磁盘 IO / 解析**。视图每帧都会跑，读文件必须发生在事件回调或后台 goroutine 里。
- **MCP server 默认监听**。必须是默认关闭 + 仅 `127.0.0.1` + 必须带 token。

## 约束

- Go 1.27.1（本机），`github.com/egoist/mygo v0.9.1`（`go.mod` 要求 go 1.27.3，`GOTOOLCHAIN=auto`
  会自动取工具链；若下载不到，需手工升级 Go）
- 不引入任何第三方 markdown / GFM 库：解析与转发 HTML 都用 `ui/markdown/parser`
- 主开发平台 Windows，但不写平台专用代码，保持三平台可 `go build`
- 测试不得触碰真实用户目录（用 `t.TempDir()` + `App.SetPath(PathUserData, ...)`）

## 你定下的（按原样写入）

| 决定 | 内容 |
| --- | --- |
| UI 形态 | 纯 Go 原生 UI（`ui` 包），不用 web 前端、不装 bun |
| 正文渲染 | 原生 `ui/markdown` 与 `ui.WebView` 两种都要；底部状态栏分段切换（参考 VS Code 状态栏「Markdown Plus \| Markdown 拆分编辑器」） |
| 变更跟随 | 文件监听 + 自动重载，必须覆盖新增 / 删除 / 重命名 |
| MCP | 本期就实现本地 MCP server 骨架（不是只留接口） |

## 我定的（builder 决定，可推翻）

1. **项目 = 一个绝对路径目录**；显示名取目录 basename，重名时追加 ` (2)`。
2. **配置**：`<PathUserData>/config.json`，结构 `{version:1, projects:[{id,name,root}], activeId, renderMode}`。
   写入走"临时文件 + rename"原子替换。`id` 用路径的稳定哈希，避免重命名根目录后配置错乱。
3. **树忽略规则**（扫描与监听共用同一函数）：跳过 `.git`、`node_modules`、`vendor`、`dist`、`build`、
   `.mygo`、以及任何以 `.` 开头的目录；只收录后缀 `.md` / `.markdown`（大小写不敏感）。
4. **TOC**：由 `parser.Document` 的 `Heading` 块直接生成；slug 用 GitHub 规则（小写、空格→`-`、
   去标点、CJK 保留），重复 slug 追加 `-1`、`-2`。
5. **原生模式 TOC 跳转**：`markdown.Options.Anchor` 设为该 slug、每点一次 `AnchorVersion++`。
6. **webview 模式**：注册自定义协议 `mdview://localhost/`，`/doc?p=<url-encoded 绝对路径>&rev=N`
   返回 `Document.HTML()` 套模板（CSS + `id` 锚点），`/img?p=...` 返回相对图片。
   TOC 跳转 = `LoadURL("mdview://localhost/doc?p=…&rev=N#slug")`，靠浏览器原生锚点滚动。
   不用 `Page.LoadHTML`，因为那样拿不到相对图片与锚点。
7. **两种模式的解析结果共用**：`workspace` 包对 (项目, 路径, mtime, size) 做缓存，返回同一个 `*parser.Document`；
   `ui/markdown` 也吃这份源字符串（它内部有自己的增量缓存）。
8. **渲染模式的作用域**：底部切换是"当前文档"的状态，会话内记住（按项目内相对路径），不落盘。
9. **布局**：窗口 1280×820。顶部 40 DIP 工具条（项目下拉 + 「打开项目…」+ 「刷新」）；
   主体 `ui.Split`（侧栏 300 DIP）；预览区再 `ui.Split`（TOC 240 DIP）；底部 32 DIP 状态栏。
10. **文件监听**：每个打开过的项目一个 `mygo.WatchFiles(ctx, root, FileWatchOptions{Recursive:true, Ignore:…})`，
    生命周期跟随项目被移除/应用退出；事件合并后（watcher 自带 50ms debounce）→ 重建树；
    若当前文档受影响则重读并重新解析。
11. **重命名跟随**：`FileRename` 带 `OldPath`；当前文档正好是 `OldPath` 时，当前选中切到新路径。
12. **MCP 传输**：单端点 `POST http://127.0.0.1:<port>/mcp`，JSON-RPC 2.0，响应 `application/json`
    （不使用 SSE / 长连接）。实现 `initialize`、`notifications/initialized`、`tools/list`、`tools/call`，
    其余方法返回 `-32601`。鉴权 `Authorization: Bearer <token>`，token 与端口写在 `<PathUserData>/mcp.json`。
    端口固定 8765，被占用则回退到系统分配端口并更新 `mcp.json`。
13. **不做的**：scroll spy、代码高亮、编辑器、多标签、全文索引（搜索用逐文件大小写不敏感子串扫描）。

## 架构

```
main.go                     组装：读配置 → 建 workspace → 建 view state → 绑 MCP → 开窗口
internal/config/            配置读写（纯 IO，无 UI 依赖）
internal/workspace/         项目扫描、树、读取、解析缓存、TOC/slug、DocIndex 实现
internal/watch/             WatchFiles 封装成 ChangeSet 通道，可注入假事件源做测试
internal/mcp/               JSON-RPC server + tools（只依赖 workspace.Index 接口）
internal/view/              原生 UI：state、工具条、树、TOC、预览（native / webview）、状态栏
```

依赖方向：`view → workspace`、`mcp → workspace`、`workspace → config`、`config → (mygo 只有 App.Path)`。
`workspace` 与 `mcp` 都不 import `ui`，这样 MCP 侧可以单测到，不需要图形环境。

### workspace 对外接口（MCP 与 UI 共用）

```go
type Project struct{ ID, Name, Root string }
type Doc struct {
	ProjectID string
	Path      string // 相对项目根，使用 / 分隔
	Title     string // 首个 H1，否则文件名
	ModTime   time.Time
	Size      int64
}
type Heading struct{ Level int; Text, Slug string }

type Index interface {
	Projects() []Project
	ListDocs(projectID string) ([]Doc, error)
	ReadMarkdown(projectID, path string) (string, error)
	Parse(projectID, path string) (*parser.Document, []Heading, error)
	Search(projectID, query string, limit int) ([]Hit, error) // Hit{Path, Line, Text}
}
```

### MCP 工具契约（骨架期就固定下来，后续接 agent 不改）

| tool | 入参 | 出参 |
| --- | --- | --- |
| `list_projects` | `{}` | `[{id,name,root,active}]` |
| `list_docs` | `{project?}` | `[{path,title,size,mod_time}]` |
| `read_doc` | `{path, project?}` | `{path, content}` |
| `search_docs` | `{query, project?, limit?}` | `[{path,line,text}]` |

`project` 省略时用当前活动项目。全部只读；未知 tool 名返回 `-32602`。

### 构建者检查后补充的决定

1. **窗口与 WebView 生命周期**：`win := mygo.NewWindow(...)` 在 `App.WhenReady` 内创建，state 持有 `win`。
   webview **不在视图函数里创建**（mygo 要求 "make once, the view only shows it"）：首次切到 webview 模式时
   `win.NewWebView(mygo.WebViewOptions{URL: docURL})` 并缓存在 state；之后视图只是 `ui.WebView(c, app.wv)`。
   切回原生模式时只停止构建该元素，页面保留（不 `Destroy`）。
2. **外部事件唤醒视图**：视图是 state 的纯函数；watch goroutine 在锁内改 state 后调 `win.Invalidate()`。
3. **树展开状态**：`map[string]bool` 以项目内相对目录路径为 key（不是索引），重建树时不清空。
4. **视图里不做解析**：`*parser.Document` 与 `HTML()` 字符串按 `(path, mtime, size)` 缓存在 state，
   事件到达时才重新解析。
5. **依赖注入**：`*workspace.Workspace` 实现 `Index`；`mcp.New(index Index, opts...)` 只接收接口，便于单测塞假实现。

## 关键交互与空状态

- 没有任何项目：主区显示一句提示 + 「打开项目…」按钮。
- 项目里没有 md：侧栏显示"这个目录下没有 markdown 文件"。
- 当前文档被删除：正文清空并提示"文件已被删除"，树同步移除该项。
- 项目根目录被删除或移动：该项目标记为失效，自动切到下一个可用项目；全部失效则回到空状态。
- webview 模式在缺 WebView2 Runtime 的机器上：分段控件第二项禁用，并在状态栏说明原因（原生模式不受影响）。

## 测试策略

- **纯逻辑**（`config`、`workspace`、`watch`、`mcp`）：`go test ./...`，全部用 `t.TempDir()` 造目录树。
- **视图**：`ui` 有无窗口的 tester（`docs/ui/testing.md`），用它断言树的行、TOC 的行、分段控件选中项。
- **MCP**：`httptest` 直接打 `/mcp`，断言 `initialize` → `tools/list` → `tools/call` 三段握手与 401/404 分支。
- **手工验收**（由我本人过一遍）：多项目来回切、外部改文件、删除/重命名、两模式切换同一文档、
  MCP 用 `curl` 打一次 `tools/call`。

## 风险

| 风险 | 处置 |
| --- | --- |
| 本机 Go 1.27.1 < mygo 要求的 1.27.3 | 先跑一次 `go build ./...` 看工具链是否自动拉取；失败则提示升级 |
| `ui.Tree` 会构建全部节点，超大仓库树变慢 | md-only + 忽略规则通常 < 2000 项；超过 3000 项改为 `ui.Outline`（本期只记录阈值，不实现） |
| Windows 上 webview 依赖 WebView2 Runtime | 启动时探测，缺失则禁用切换（原生模式是主路径，不影响可用性） |
| 协议处理器里读任意路径（`mdview://localhost/img?p=C:\...`） | 只允许"已打开项目根目录下的文件"，越界返回 403 |
| 文件监听在编辑器"原子保存"下丢事件 | `WatchFiles` 本身按目录监听文件替换；再加"手动刷新"按钮兜底 |
