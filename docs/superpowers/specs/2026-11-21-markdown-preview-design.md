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
- ~~多文档标签页~~（后来实现：主预览顶部的标签栏，同一份计划）
- 工程范围的全文检索 UI（项目内跨文件搜索仍只作为 MCP 工具存在；**文档内**查找后来作为悬浮面板实现，
  见 `docs/superpowers/plans/2026-11-22-tabs-find-toc-actions-zoom.md`）
- 多窗口
- 非 md 文件预览（图片、代码、PDF）
- TOC 滚动跟随（scroll spy）——点击 TOC 能跳，但滚动正文不会高亮目录项
- 代码块语法高亮（原生 `ui/markdown` 的 `DefaultCodeBlock` 不带高亮；本期接受单色等宽）
- ~~应用打包~~（后来做了：`.github/workflows/build.yml` 出三平台的安装包）、签名、自动更新（签名与自动更新仍不做）

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
4. **TOC**：由 `parser.Document` 的顶层 `Heading` 块直接生成；slug 与 `ui/markdown.Slug` 逐字一致
   （小写、每个空格→`-`、保留 `-`/`_`/字母数字、不折叠不去尾；重复标题**不去重**）。
   理由见「实施后的变更」。
5. **原生模式 TOC 跳转**：`markdown.Options.Anchor` 设为该 slug、每点一次 `AnchorVersion++`。
6. **webview 模式**：注册自定义协议 `mdview://localhost/`，`/doc?p=<url-encoded 绝对路径>&rev=N`
   返回 `Document.HTML()` 套模板（CSS + `id` 锚点），`/img?p=...` 返回相对图片。
   TOC 跳转 = `LoadURL("mdview://localhost/doc?p=…&rev=N#slug")`，靠浏览器原生锚点滚动。
   不用 `Page.LoadHTML`，因为那样拿不到相对图片与锚点。
7. **两种模式的解析结果共用**：`workspace` 包对 (项目, 路径, mtime, size) 做缓存，返回同一个 `*parser.Document`；
   `ui/markdown` 也吃这份源字符串（它内部有自己的增量缓存）。
8. **渲染模式的作用域**：「当前文档」的状态，会话内记住（按项目内相对路径），不落盘。
   切换控件的位置后来从底部移到了标题栏，见「实施后的变更」。
9. **布局**：**已被 shell 取代**（见「实施后的变更」）。原计划是：窗口 1280×820，顶部 40 DIP 工具条，
   主体 `ui.Split`（侧栏 300 DIP），预览区再 `ui.Split`（TOC 240 DIP），底部 32 DIP 状态栏。
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
main.go                     组装：读配置 → 建 workspace → 建 sidebar shell → 文件监听 → MCP
internal/config/            配置读写（纯 IO，无 UI 依赖）
internal/workspace/         项目扫描、树、读取、解析缓存、标题缓存、TOC/slug、搜索（Index 实现）
internal/watch/             WatchFiles 封装成 ChangeSet 通道，可注入假事件源做测试
internal/mcp/               JSON-RPC server + tools（只依赖 workspace.Index 接口）
internal/view/              Onevra 主题（theme.go）+ 外壳各区域：侧栏顶栏、树、侧栏工具栏、
                            内容标题（含渲染开关）、预览（native / webview）、状态栏、设置面板
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
- webview 模式在缺 WebView2 Runtime 的机器上：标题栏的 WebView 分段禁用，并在状态栏说明原因（原生模式不受影响）。

## 测试策略

- **纯逻辑**（`config`、`workspace`、`watch`、`mcp`）：`go test ./...`，全部用 `t.TempDir()` 造目录树。
- **视图**：`ui` 有无窗口的 tester（`docs/ui/testing.md`），用它断言树的行、TOC 的行、标题栏分段、
  正文列宽度与滚动范围；shell 的各槽位（`Left`/`LeftHeader`/`LeftToolbar`/`ContentTitle`/`Content`）
  逐个构建，因为 `sidebar.NewShell` 会真的建窗口，测试里不能用。
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
| `sidebar.NewShell` 会真的建窗口，视图层没法整体 headless 测 | shell 各槽位单独导出，`TestShellSlotsBuild` 逐个构建；`View` 保留为并排近似布局 |
| 正文列宽依赖 `win.ContentSize()` 与侧栏宽度的算式 | 改动布局时必须同步 `bodyPaneWidth()`；`TestBodyColumnFollowsPaneWidth` 在两种宽度下断言不溢出 |
| 自绘项目下拉丢了系统菜单的键盘导航 | 无障碍名保留、Tab 可聚焦；若需要菜单键盘语义，换回 `ui.PopoverMenuButton`（但那个不支持图标） |

## 实施后的变更（写回）

实现并合并之后，下面这些「我定的」条目被替换或被细化。它们是**现状**，上面的正文只在没被这里
覆盖时才是准确的。

### 布局与外观（取代原第 9 条）

- 窗口只由 `sidebar.NewShell`（`github.com/oneo-me/onevra/source v0.2026.18`）创建，不再用
  `mygo.NewWindow` + `ui.Split` 拼三栏。默认 1280×820、最小 760×520、侧栏 280（最小 200），
  窗口位置与最大化状态由 `StateKey: "markdown-preview.main"` 记住。
- 槽位映射：`Left` = md 文件树；`LeftHeader` = 项目切换；`LeftToolbar` = 打开项目 / 刷新 / MCP；
  `ContentTitle` = 文档路径 + 渲染开关；`Content` = 目录 + 正文 + 状态栏。
  折叠侧栏、浮动标题栏、拖拽分隔都由 shell 负责。
- 主题：`internal/view/theme.go` 的 `AppTheme` = `theme.Onevra(c)` + 本应用调色板
  （主色 `#0052d9`、悬停 `#618dff`、按下 `#003cab`、警告 `#e37318`），由 shell 每帧安装；
  侧栏选中行与已开启的 MCP 按钮用 `Accent.Alpha(0.12)` 的淡色。
- 图标：一律用 `onevra/source/icons/lucide` + `components.Icon` / `components.IconButton`。
  侧栏底部的动作按钮用 `IconButtonPlain`（32 DIP 方块 / 20 DIP 图形）；**不要**退回
  `IconButtonCompact`（20/14），用户反馈太小。
- 项目切换从系统菜单（`ui.PopoverMenuButton`）改成 `ui.Popover` + `components.Button` 行
  （Folder 图标、当前项 Check + 淡主色底、末尾分隔 + Trash「移除当前项目」）。
  **代价：丢掉了菜单的键盘导航语义**；无障碍名仍在。

### 渲染开关的位置与尺寸（细化原第 8 条）

- 从底部状态栏移到标题栏，紧跟在文档路径之后（`contentTitle` → `modeToggle`）。
- 自绘两段而不是 `ui.Segmented`：外框高 `max(20, Rem(1.25))`、内段 18、字号 11，
  目的是不把 `sidebar.HeaderHeight` 撑高。
- 状态栏只剩一行提示文字（高度 24）。**（2026-11-24 起状态栏整个移除，提示改走 Toast 与铃铛历史，见文末「提示改为 Toast 与铃铛历史」。）**

### 正文宽度与横向滚动（新增）

- 正文列**不用** `WidthPercent`，也不用 `MaxWidth`：`ScrollBoth` 里的百分比不解析成视口宽度
  （会退化成按内容撑开，导致文字不换行、内容溢出）。宽度由
  `State.SetContentWidth`（`main.go` 用 `win.ContentSize()` 减去侧栏宽度）减去目录宽度算出。
- 正文容器是 `ui.ScrollBoth`：窗口拉伸、侧栏折叠、拖动目录分隔条都会按新宽度重排；
  比窗格宽的图片等由这条横向滚动承载。
- **表格**保持 mygo 的行为（决策 A，用户确认）：表格被拉伸到窗格宽度，超出的部分在
  **表格自带**的 `ScrollHorizontal` 里滚，滚动条在表格底部而不是窗口底部。不做「整篇变宽」，
  也不自绘表格。
- 列宽里额外减去 2 DIP，抵消宽度取整带来的 1px 溢出（否则每个文档都会多出一条无用的横向滚动条）。

### slug 规则修正（取代原第 4 条）

- `workspace.Slug` 是 `ui/markdown.Slug` 的逐字复制：小写、每个空格→`-`、保留 `-`/`_`/字母数字、
  不折叠连续空格、不去首尾 `-`；**重复标题不去重**（都指向第一个同 slug 的标题）。
- 原因：`markdown.Options.Anchor` 按 mygo 的 `Slug` 匹配标题，自造规则会让 TOC 点击跳不到。

### 实现上的其他定论

- 视图状态不加锁：状态归窗口线程，watcher 用 `State.Post(fn)` 投递，帧首 `drain()` 应用后
  `win.Invalidate()`。（原计划的「mutex + Invalidate」作废。）
- 树默认全展开，只记折叠（`open` 指针表），因为 `ui.TreeItem` 直接写 `*bool`。
- `Workspace.ListDocs` 的标题按 `(mtime, size)` 缓存（`titleEntry`），避免每次树重建都读一遍文档开头。
- MCP：通知（无 `id`）一律 202 空体、不回响应；`SelectProject` 要把新配置同步进 `workspace`
  （否则 MCP 的「当前项目」停在旧值）。
- 测试补充：`TestAppThemeUsesPalette`（四个色值）、`TestShellSlotsBuild`（逐个构建 shell 槽位）、
  `TestBodyColumnFollowsPaneWidth`（600/1200 下都不溢出）、`TestModeToggleSitsWithTheDocumentName`、
  `TestStatusBarDropsTheModeSwitch`。

### 跳转与搜索面板（取代原第 5 条与第 6 条后半，2026-11-22）

- **原生 TOC / 正文内 `#标题` / 搜索命中的跳转**不再用 `markdown.Options.Anchor` + `AnchorVersion`（mygo 里那
  是 `ScrollNearest`、无动画：一帧跳过去，且只保证目标露出来一点）。改为 `State.jump`（待跳的块号）+
  `markdown.Options.Block` 钩子：钩子在目标块自己的 Column 里建一个零高标记，对它调
  `ScrollIntoView({Block: ScrollStart, Animation: ui.Smooth})`。于是「目标顶到顶部、近末尾落到底部（滚动容器把
  位置钳制在 `MaxY`）、平滑、尊重系统的减少动态效果」都由 mygo 的 reveal 一次给全。跳转在顶上留 16 DIP
  （`bodyJumpGap`）：reveal 对齐的是滚动容器的边框而不是内边距，所以这 16 是正文滚动容器的透明上边框；
  webview 用 `scroll-margin-top: calc(16px * var(--md-scale))` 对齐同一数值；`find:` 锚点前缀与
  `anchorBlock` 退役，`anchor` 只剩「目录高亮的那一项 / WebView 要滚到的标题」这一个含义。
- **webview 的 TOC 跳转**不再 `LoadURL(url + "#slug")`（那是整页重载：既不平滑，又丢滚动位置）。改为
  `Page().Eval` 的 `scrollIntoView({behavior:'smooth', block:'start'})`，页内滚动，地址里不带 `#`；
  页面的钳制与原生一致（到页尾落到底部）。
- **搜索面板从"一整块卡片里叠三段"改成三块独立的浮岛**（`findIsland`：输入框 / 匹配行 / 上下文），
  外层只给位置与 `Gap`、没有背景：块与块之间露出正文，列表与上下文不再读成一列；上下文岛顶上写明
  `第 N 行上下文`，两块数据的岛各有无障碍名（`匹配行` / `命中上下文`）。

### 托盘与窗口生命周期（新增，2026-11-23）

- **关闭窗口 = 隐藏窗口，不退出**：`internal/tray` 的 `keep` 在 `Window.OnClose` 里 `PreventDefault()` +
  `Hide()`，只有 `App.OnBeforeQuit` 置过 `quitting` 时才放行。`App.Quit()`（托盘「退出」、macOS 的 `Cmd+Q`）
  因此能真正结束时进程——`prepareQuit` 会逐个窗口 `close()`，没有这条放行路径会被自己的 `OnClose` 拦住。
  隐藏期间文件监听与 MCP server 都不中断，标签、滚动位置、渲染模式留在内存里（同一份视图）。
- **托盘只有两项**：`打开主页面`（`Restore` + `Show` + `Focus`，与第二次启动共用 `tray.Show`）、
  `退出`（`App.Quit`）。菜单挂在 `TrayOptions.Menu` 上，三平台一致：Linux 的 AppIndicator 只支持菜单，
  macOS 设了菜单左键点击即开菜单，不需要平台分支。macOS 另接 `OnActivate`：没有可见窗口时点 Dock 图标把窗口叫回来。
- **单实例**：`App.RequestSingleInstanceLock` + `OnSecondInstance` → `tray.Show`。关闭即隐藏之后，
  再次启动不能再开第二个进程/第二个托盘图标。
- **托盘建不起来就退回「关闭即退出」**：`Install` 先 `NewTray`，成功后才接管 `OnClose`；反过来会让用户
  既没有窗口也没有托盘可点。失败只记日志（`tray:`）。
- **图标三层**：`mygo.json` 的 `resources/icon.png`（1024）由 `mygo build` 生成 exe/bundle 图标（`go build` 不带）；
  窗口/任务栏图标运行时 `win.SetIcon`（`internal/tray/assets/window.png`，embed 的 256）；
  托盘 `IconIsTemplate: true` + `assets/tray.png`（32，墨色剪影 + 透明），明暗由系统涂色，
  因此**不打包**图标套件里的 `tray-on-dark`，也不按 `Dark()` 换图。

### 搜索面板的可调高度（新增，2026-11-23）

- **两块数据岛各自可调，且各有自己的高度**：匹配列表岛（默认 `findListHeight=132`，`findListMin=72`、
  `findListMax=320`）与上下文预览岛（默认 `findPreviewHeight=148`，`findPreviewMin=96`、`findPreviewMax=420`）。
  以前列表是 `MaxHeight(132)`（只有一条命中时塌成一行 + 横向滚动条），上下文是 `MaxHeight(148)`、内容自适应。
- **拖拽**：每块岛**下面**各一条 `findGrip=10` DIP 的带子（`findGripBand`，`CursorResizeNS`、`RoleSplitter`、
  读屏名「调整匹配列表高度」/「调整命中上下文高度」），拖动改上方岛的高度；**双击复位**；指针离开或松手后
  带子不画东西（hover/拖动时在带子正中画 1 DIP 主色线），面板仍是三块浮岛而不是一张带分隔线的卡片。
  带子按「上一帧的增量」累加（`Dragged()`），保留拖过极限后的过冲（`listOver`/`previewOver`），
  且 `dy == 0` 的按下不写高度（否则会把「用户要的高度」改成「窗口能显示的高度」）。
- **键盘**：带子可聚焦（按一下就拿到焦点），`↑`/`↓` 以 `t.Space(2.5)` 为步长移动它，与 `ui.Split` 的分隔条一致。
- **总高度不超窗口**：`findRoom` 扣掉面板顶部的 header 与标题行、底部留白、输入岛与两条带子；每条岛的上限
  再减去另一条岛的高度（`findHeightMaxOf`）；`fitFindHeights` 每帧兜底——窗口变矮时**上下文先让出高度**、
  再轮到列表，只改显示高度（`shownList`/`shownPreview`），用户拖出来的值保留，窗口变高会自己恢复。
  窗口高度来自 `State.SetContentHeight`（`main.go` 用 `win.ContentSize()` 接），测试不设时用 700。
- **布局**：面板列不再用 `Gap`——输入↔列表 6 DIP（列表岛的 `Margin`），列表↔上下文、上下文↔面板底各是带子的 10 DIP。
- **高度只在本次运行内记住**（与正文缩放 `zoom` 一致，不写 `config.json`）；换文档、关面板都不重置。
- **正文高亮不在本期**：原生模式下搜索仍不高亮正文（见「已知限制」）。走 mygo 引擎侧的
  `Options.Mark` 区间方案，见 `docs/superpowers/plans/2026-11-23-mygo-mark-ranges.md`。

### 提示改为 Toast 与铃铛历史（新增，2026-11-24）

- **底部信息栏整个移除**（`statusBar` 不再存在）：提示改弹出在**窗口右下角**的 Toast，几秒后自动消失，
  按类型带图标与颜色（信息 / 成功 / 警告 / 失败）。正文与目录因此吃满原来那 24 DIP，一直画到窗口底边。
- **消息账本是 `internal/notices`**（纯逻辑 + JSON，**不 import ui**，可无窗口测试）：消息为
  `{ID, Kind, Text, At, Seen}`，**最新在前、不设条数上限**；ID 单调递增、清空后也不复用（它同时是
  「这条要不要弹 Toast」的水位线）。
- **落盘与重启恢复**：写在用户数据目录的 `notices.json`（与 `config.json` 同目录，原子写：临时文件 + `Rename`），
  重启后历史与**未读标记**原样恢复。没有用户数据目录时（测试路径）账本只在内存里，不写盘。
  读不出来（文件坏了）退化成空历史，应用照常打开。
- **不重播旧消息**：启动时把「已发过的最后一条 ID」对到账本最新一条，上次的历史只进列表、不再弹 Toast。
- **铃铛与角标**：铃铛在左侧工具栏底部（`ui.Spacer` 之后、MCP 之前）。有未读时铃铛图标变主色，
  右上角挂一个**红色数字角标**（`t.Danger` 底 + `t.AccentText` 字，最多 `99`，超过显示 `99+`）；
  角标不吃点击（`IgnoreInput`）、不进无障碍树（条数已写进铃铛的无障碍名）。
- **历史浮层**（`ui.Popover`，同项目面板与 MCP 面板）：顶部一行「消息」+「清空全部」，下面是可滚动的列表
  （每条：类型图标 + 全文 + 时间；今天只显示 `HH:MM`，同年跨天 `MM-DD HH:MM`，跨年带年份），
  行尾 ✕ 只删这一条；没有消息时只显示「暂无消息」，不出现清空按钮。打开浮层即把全部标为已读，
  浮层开着时新到的消息也立刻算已读（不会在正显示着的列表上冒出角标）。
  浮层的高度是**窗口可视高度的比例**（2026-11-24 用户要求「再高一点，占可视区域 50%；resize 时超过 70% 就跟着变」）：
  至少 **50%**（`noticePanelMinShare`，空历史也是这么高，不会随消息到来从一条细条往下长），
  最多 **70%**（`noticePanelMaxShare`），再长就在列表内部滚动；两者都是比例，所以窗口拉大拉小时跟着变。
- **每条消息是一张卡片**（`Surface` 底 + `Border` 描边 + 圆角 + 内边距，悬停换 `SurfaceHover`），
  卡片之间留 4 DIP：早先只有一行行文字，相邻两条读起来分不开。
- **去重**：一次点击会构建两遍视图，Toast 按 ID 水位线去重，同一条只弹一次、读屏只播报一次。
- **顺带修掉的前置 bug**：shell 用的构建路径（`Content`）一直没有应用其它 goroutine 投递的变更
  （`drain`），外部改动实时跟随与 watcher 消息都因此失效；现在每帧先应用再构建。
- **不设条数上限**是用户 2026-11-24 的决定：历史会一直累积，浮层的「清空全部」/ 单条删除是唯一收缩手段。

### 滚动条的位置：贴边缘，不在内边距里（新增，2026-11-24）

- **滚动条贴着窗格边缘，不在内边距里**：`toc` 的外层列不再统一加 `tocPadding`，只有表头那一行加；
  滚动容器铺满窗格，内边距移到滚动内容上（`Padding(0, tocPadding, tocPadding, tocPadding)`）。
  用户原话：「preview 目录树 滚动条能否放在 padding 层之外 目前在 padding 层里面 容易误触」——
  原来的布局让竖滚动条压住标题文字、窗格边缘又正好是滚动条的命中区。
- 行仍然铺满窗格（`MinWidth` 是**含内边距的边框盒**，所以写 `s.tocW`），点窗格右缘照样能选到标题
  （`TestTOCRowsFillThePane`）；滚动容器的位置由 `TestTheOutlineScrollsAtThePanesEdge` 钉住。
- **正文与搜索面板也是同一条规则**（用户 2026-11-24 追加：正文「是的」，搜索浮窗「也有相同问题」）：
  正文的滚动容器铺满窗格、`bodyPadding` 给里面的内容列（`bodyJumpGap` 的透明上边框仍留在滚动容器上，
  跳跃落点不变）；搜索面板的 `findIsland` 不再带内边距，两块数据岛改由 `findScrollPane` 给滚动容器 +
  带 padding 的内容列，输入岛自己保留 `findIslandPad`。
- 三处都遵守同一个算术：`Width` / `MinWidth` 是**含内边距的边框盒**，所以内容列要写「窗格/岛宽」而不是
  「宽度 − 2×内边距」；滚动容器带读屏名（`目录列表` / `匹配行列表` / `命中上下文列表`），测试据此量它的位置。

### 项目树的批量折叠与文件名搜索（新增，2026-11-24）

用户原话：「给项目树也增加 收缩全部/展开全部」「同时每个项目 也可以单独收缩全部/展开全部」「同时给项目树也增加
ctrl + f 模糊搜索文件名称 类似现有 preview 的搜索 但是不需要候选列表与简单预览 可以上下切换候选值 同时 tree 需要
定位到当前候选值 候选值与选中的候选值都需要高亮显示 使用两种不同的背景色」。

- **全部展开 / 折叠**：侧栏顶部、**项目选择按钮左边**两个图标（`展开全部目录树` / `折叠全部目录树`），
  一次管所有已打开的项目；没有打开的项目时禁用。展开状态只在本次运行内有效（与目录树的旧行为一致，不落盘）。
- **每个项目单独**：`展开此项目的全部目录` / `折叠此项目的全部目录` 在**项目面板每行的右键菜单**里
  （只开一个项目时也能用），开了多个项目时树顶上那个**项目节点的右键菜单**里也有同样两条。
- **快捷键按焦点分派**：`Cmd/Ctrl+F` 在**焦点位于侧栏**时搜文件名、否则搜正文；
  `Shift+Cmd/Ctrl+F` 从任何位置都搜文件名。正文查找的快捷键与行为一条都不变。
  「焦点在侧栏」= 最后一次按下发生在侧栏树或搜索框（`State.treeFocused`，由两个滚动容器的 `HandleInput` 维护）。
- **树自己就是候选列表**：搜索打开时树上方出现一行搜索框，树只显示**命中的文件 + 它们所在的目录 + 项目节点**；
  祖先目录在搜索期间强制展开（不写 `s.open`），关掉搜索后恢复用户原来的折叠状态。列表与预览面板一个都不加。
- **模糊匹配只看文件名**：大小写不敏感的**子序列**（`plan` 命中 `RELEASE_PLAN.md`、`p_l_a_n.md`），
  不做正则、不做拼音、不打分排序；候选按树里的顺序（项目顺序 → 目录在前、各自按名）。空白查询 = 不搜索。
  输入时不重扫目录，只过滤内存里已经扫好的树。
- **两种底色**：所有候选一层淡主色（`t.Accent.Alpha(0.08)`），当前候选一层浓主色（`t.Accent.Alpha(0.22)`）；
  搜索期间不再用「选中行」那层色，免得三种底色混在一起、也免得「当前候选」与「已打开文档」分不开。
- **上下切换与定位**：`↑` / `↓` 换候选（**不打开文档**，用户明确要求），当前候选那一行由树 `ScrollIntoView`
  滚进视野；`Enter` 或点一下候选才打开，搜索保持开着以便接着跳。搜索框右边是计数（`3/12`，没有命中是「无匹配」，
  树里显示「没有匹配的文件」）。
- **`Esc`**：先清空查询（树恢复完整），空查询时再收起搜索框；与正文查找的 `Esc` 不互相干扰
  （树搜索开着时正文查找的 `Esc` 不生效）。
- **两个搜索面板不同时占键盘**：打开树搜索会收起正文查找，打开正文查找会收起树搜索（树搜索的查询留着）。
- **不落盘**：搜索查询、当前候选、焦点、滚动位置都只在本次运行内有效。
- **每个项目的行内按钮**（2026-11-24 用户验收时追加：「项目 后增加 全部展开/全部折叠」）：开了多个项目时，
  项目名后面直接有两个小图标（展开/折叠**这个项目**的全部目录），不用先右键；项目面板每行的右键与项目节点的
  右键菜单保留同样两条。按钮的无障碍名带项目名（`展开项目 <名字> 的全部目录`），读屏与测试才能分清是哪一个。
- **搜索框的占位文字是中文**（同日追加）：树搜索 `搜索文件名`、正文查找 `在文档中查找`，不再是 mygo 自带的 `Search`。
- **折叠与展开的范围**（同日追加）：`折叠` 只收起目录，**项目节点保持打开**（否则项目里只有根目录文件时，
  折叠会看起来像把整个项目关掉）；`展开` 连同项目节点一起展开，所以不会有文件被留在关着的节点里。
