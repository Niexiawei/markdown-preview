# AGENTS.md

本仓库是一个**纯 Go 的桌面 markdown 预览器**：多项目管理，左侧 md 文件树，右侧「文档目录 + 正文」，
正文可在原生渲染（`ui/markdown`）与 WebView 渲染（`ui.WebView`）之间切换；外部改动（agent / 编辑器）
实时跟随；另带一个默认关闭的本地 MCP server。

UI 框架是 `github.com/egoist/mygo v0.9.1`（原生 UI 路径，**不用** web 前端、不需要 bun）；
布局、主题与图标来自 `github.com/oneo-me/onevra/source v0.2026.18`（`shells/sidebar` 外壳、`theme.Onevra`、
`icons/lucide`、`components`）。设计与实现计划在 `docs/superpowers/`。

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
| `internal/workspace` | 扫描 / 读取 / 解析缓存 / 标题缓存 / TOC 与 slug / 搜索 / 正文内查找（`find.go` 的 `FindIn`）；`Index` 接口 |
| `internal/watch` | `mygo.WatchFiles` → `ChangeSet`（新增 / 删除 / 重命名 / 写入） |
| `internal/mcp` | `POST /mcp` JSON-RPC 2.0，四个只读工具 |
| `internal/view` | Onevra 主题（`theme.go`）与外壳各区域：侧栏顶栏（项目面板 + 右键菜单）、树、侧栏工具栏、标签栏（`tabs.go`，占 shell 的标题槽位）、页面第一行的路径与渲染开关（`titleRow`）、预览、状态栏、MCP 设置面板；`find.go` 是悬浮搜索（三块浮岛）与跳转，`native.go` 是原生正文与跳转（`jumpToBlock` / `jumpMarker`），`zoom.go` 是正文缩放；`system.go` 是桌面动作接口 |

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
- **窗口只用 `sidebar.NewShell` 创建**（`main.go`），不要退回 `mygo.NewWindow`：折叠侧栏、浮动标题栏、
  主题安装都由 shell 负责。要加内容就填它的槽位：`Left` / `LeftHeader` / `LeftToolbar` / `ContentTitle` / `Content`。
- **颜色从 `AppTheme` 取**（`internal/view/theme.go`）：不要在别处写死十六进制色值；需要主色就
  `c.Theme().Accent`，需要警示色就 `c.Theme().Warning`。
- **图标只用 `icons/lucide`**，用 `components.Icon` / `components.IconButton`；图标按钮必须给 `label`
  （无障碍名，也是测试定位的唯一方式，图标本身不是文本）。侧栏底部的动作按钮用
  `IconButtonPlain`（32 DIP 方块 / 20 DIP 图形），**不要**用 `IconButtonCompact`：用户反馈太小。
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
- **改跳转落点（顶部/居中/是否平滑）**：只改 `view/native.go` 的 `jumpMarker`（`ScrollIntoViewOptions`）与
  `view/webview.go` 的 `headingScrollJS`，两边要一致；不要在调用处各写一套滚动。
- **改渲染样式**：原生在 `markdown.Options`（`loadDoc` 里装配）；webview 在 `webview.go` 的 `docCSS`。
- **改标签栏/标题行/侧栏**：`tabs.go` 的 `tabBarAt`（标签、⌄ 列表）挂在 shell 的 `ContentTitle` 槽位；
  `view.go` 的 `titleRow` → `contentTitle`（路径 + 复制 + `modeToggle`）是 `Content` 的第一行；
  `leftHeader`（项目面板：点一行开/关、右键打开/复制/移除）、`leftToolbar`（动作图标）、`statusBar` 各就各位。
  改布局时 `View` 与 `Content` 两条拼装路径要一起改（`TestShellSlotsBuild`、`TestTabsSitAboveTheTitle`）。
- **改正文内查找**：匹配规则只改 `workspace.FindIn`（字面子串、忽略大小写、上限 500），补 `TestFindIn*`；
  面板与跳转在 `view/find.go`（三块浮岛 `findIsland` / 匹配列表 / 上下文预览 / `Cmd+F` 与 `Cmd+G`）；
  命中跳转只调 `jumpToBlock`，与目录、链接共用同一条路（见「已知的坑」里的 `State.jump`）。
- **改标签栏**：`view/tabs.go` 的 `openTab` / `closeTab` / `visibleTabs` / `tabBar`，补 `TestTabBar*` 与 `TestClosingTabs`。
- **加桌面动作（打开目录、复制、确认）**：扩 `view.System`（`system.go`）并在测试里扩 `fakeSystem`，
  不要在 view 里直接调 `mygo.Dialog` / `mygo.Shell` / `mygo.Clipboard`：`LeftHeader` 在侧栏收起时不构建，
  应用内弹窗画不出来，测试也跑不了真实对话框。
  改完跑 `TestShellSlotsBuild`，并在 `View` 里保持同样的拼装。
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
- **表格的横向滚动条在表格底部**，不在窗口底部：mygo 的 markdown 视图把表格包在它自己的
  `ScrollHorizontal` 里并 `AlignSelf(Stretch)`，所以表格铺满窗格、在内部滚。这是与用户确认过的
  决策 A；想做“窗口级 x 滚动条”就要 `markdown.Options.Block` 里自绘表格（两百行上下），别偷偷改。
- **正文宽度是算出来的**：`State.SetContentWidth` 由 `main.go` 用 `win.ContentSize()` 减去侧栏宽度提供，
  `bodyPaneWidth()` 再减去目录宽度；body 的列用这个宽度（而不是 `WidthPercent`），因为 `ScrollBoth`
  里百分比不解析成视口宽度。改布局时要同步这个算式，否则正文会溢出或换行错。
- **`cfg.OpenIDs` 的 `nil` 与 `[]` 语义不同**：`nil`（旧文件、没有这个字段）表示「全部打开」并迁移一次，
  `[]` 表示「一个都不打开」。`config.normalize` 负责迁移、丢掉已不存在项目的 id，并保证 `ActiveID` 始终是打开的项目。
- **文档身份是 `docSel{project, path}`**：树、正文、渲染模式表、监听都以它为准；只有一个 `path` 时两个项目里的
  `docs/a.md` 会互相串。目录折叠键是 `项目ID + ":" + 路径`，项目节点的键是 `"project:" + 项目ID`。
- **树只在打开项目数 ≥ 2 时多一层项目名**：`fileTree` 里按 `len(s.trees)` 分叉，改布局时别让单项目也长出这层。
- **状态栏只放提示文字**（`statusBar`，高 24）：渲染开关、路径都不在这里，它们在标签栏下面那一行（`titleRow`）。
- **标题槽的 24 DIP 首部间距是 onevra 写死的**（`shells/sidebar/shell.go` 里 `titleInset = 24`，`ShellOptions`
  没有对应字段）。本项目要 12，所以 `ContentTitle` 调 `tabBarAt(c, -12)`，只把**标签条**（不含 ⌄ 按钮）往左拉，
  ⌄ 留在标题槽自带的右侧内边距上——整行平移会让右边多出 16 的空档（用户报过「移动太过头」）。
  **标签条自己不加左右内边距**（留白交给标题槽，否则是双层留白）。两个常量 `titleInsetGap`（24，onevra 的）
  与 `titleInsetWanted`（本项目的 12）在 `view.go`；用 `Translate` 而不是负 `Margin`，因为后者会改布局宽度、
  把标签条量宽量错。onevra 升级后若新增了间距选项，改用那个选项并删掉常量。
- **标签栏在 shell 的 `ContentTitle` 槽位**（标题栏那 48 DIP），文档路径 + 复制 + 渲染开关是 `Content` 的第一行
  （`titleRow`，高 `titleRowHeight`）。别再把这些塞回彼此的槽位：用户要求 tab 在文件名字那一行之上；
  `View` 与 `Content` 的拼装要保持一致（`TestTabsSitAboveTheTitle`）。
- **标签条是横向滚动容器**（`ui.ScrollHorizontal` + `State.tabScroll`），不丢标签：`visibleTabs` 那套"挑几个显示"
  已经删掉，`tabsWidth` 只用来判断要不要出 ⌄ 按钮。切标签后由 `revealTab` 把活动标签滚进视野：目标是**当前标签完整可见**
  （滚到需要的位置）且**左边缘落在某个标签起点**（否则会出现「空白 + 孤零零的 ✕」），够不着时退到 `MaxX`，两端用
  `tabFade` 渐隐。滚动容器的 `MaxX` **慢一帧**：刚打开的标签还没撑开它，所以
  `revealTab` 返回 "settled" 表示这个位置真的落得下，落不下就 `tabPending` 下一帧再要一次
  （否则会差一个标签、选中的那个永远切着边）；用户滚轮一动就清掉 `tabPending`，让位给用户。
  它必须**在构建滚动容器之前**
  设 `tabScroll.X`（容器构建后再设会被忽略），并且首帧（`Bounds` 为 0 / `MaxX` 为 0）不要记 `tabShown`，否则永远不滚。
- **搜索面板是三块浮岛**（`findIsland`：输入框 / 匹配行 / 上下文），外层只给位置和 `Gap`，**没有背景**。
  别再把它塞回一个带 Surface 的整块卡片里：那样列表最后一行和上下文第一行会读成一列。
  两块数据的岛给 `Label("匹配行")` / `Label("命中上下文")`（读屏名，也是测试里 `tt.Find` 取岛矩形的唯一办法），
  上下文岛顶上那行 `第 N 行上下文` 是给人分清的。岛内宽度用 `findPaneInner`，别再用 `findPaneWidth - 2*Space(1)`。
- **「全部已打开文件」列表给滚动条留了右边距**（`Padding(0, Space(3), 0, 0)`）：列表的竖向滚动条压在最右边，
  不给留出来时关标签的 ✕ 会被它盖住（用户报过点不到）。
- **标签条的滚动条只有 2 DIP**：`ScrollbarStyle{ThumbWidth: 2, TrackWidth: 2}`。默认是 `拇指 + 6` 的命中区，
  压在标签底部会吃掉点击（用户报过误触）。改这里要同时想清楚命中区。
- **鼠标滚轮滚动标签条**要自己接：mygo 的滚动容器只把 **Shift**+滚轮转成横向，普通滚轮会漏到外层去滚页面。
  所以滚动容器上挂了 `HandleInput`：`InputScroll` 且没按 Shift 时 `tabScroll.X -= ev.DY` 并返回 true
  （`DY` 为负是向下滚，与 mygo 内部 `scrollY -= dy` 一致）。
- **标签宽度要按「上一帧的槽宽」算，不是 `windowWidth()`**：shell 在标题栏里给窗口按钮留了位置，
  用内容宽度算会让最后几个标签和溢出按钮压到窗口按钮底下。`tabBar` 用 `row.Bounds().W`（首帧回落到
  `windowWidth()`），`tabWidth` / `tabTextWidth` / `fitText` 三者共用同一套尺寸，名字超长就省略，
  具体宽度写死在标签行上（`Width` + `Clip`）——否则文字会画出自己的框、和邻居叠在一起。
- **标签与滚动位置都以 `docSel` 为键**（`State.tabs` / `State.scrolls`）：只改路径不改项目会串到另一个项目里的同名文件。
- **正文跳转只有一条路**：`State.jump`（块号）+ `nativePreview` 按需挂的 `markdown.Options.Block` 钩子
  （`jumpMarker`）。钩子在目标块自己的 Column 里插一个 `0×0` 的 `ui.Box`，对它调
  `ScrollIntoView({Block: ui.ScrollStart, Animation: &ui.Smooth})`——mygo 的 reveal 于是把该块顶边对到
  滚动容器的顶边、平滑过渡、并钳制在 `MaxY`。**不要再给 `md.Anchor` / `AnchorVersion` 赋值**：那是
  `ScrollNearest` + 无动画（一帧跳过去，且只保证"露一点"）。`find:` 锚点前缀已删。
- **跳转在顶上留 16 DIP**（`bodyJumpGap`）：`ScrollIntoView` 对齐的是滚动容器的**边框**、不算内边距，
  所以这 16 是正文滚动容器的**透明上边框**（内边距相应减去 16，静止时的样子不变）。要改这个数就改
  `native.go` 的 `bodyJumpGap`，并把 webview 的 `scroll-margin-top`（`docCSS`）一起改，两个后端才一致。
- **"近末尾落到底部"是 reveal 钳制出来的**，不是算出来的：`ScrollStart` 会要一个超过 `MaxY` 的位置，
  容器把它夹到 `MaxY`。不要自己读 `MaxY` 再 `ScrollTo`，那会与钳制、与用户滚动抢。
- **`Options.Block` 不能常挂**：mygo 的 `viewState.block` 一旦看到它非 nil，会给**每个块**多包一层
  `ui.Column` 再调钩子，几千块就是几千个多余节点。只在 `s.jump > 0` 的帧挂（`nativePreview`）。
- **一次点击会建两遍视图**：mygo 处理完构建期的事件（点目录）后会重建这一帧，`c.reveal` 在重建前被清空，
  所以「建标记 + `ScrollIntoView`」如果只在第一遍发生，这次滚动会被丢掉。`jump` 因此不在钩子里清，
  而是记 `jumpSeen = c.Now()`，等到**下一帧**（`jumpSeen != c.Now()`）才清——同一帧的两遍构建都会挂上钩子，
  第二遍的请求才活到布局。别把这个改成"钩子里立刻清 `jump`"，那会让跳转在点击时静默失效。
- **标签栏宽度是算出来的**（`tabWidth` + `visibleTabs`）：可见集合以活动标签为锚点、先右后左装满；不溢出时不预留
  溢出按钮的宽度。改布局要同步 `Content` 与 `View` 两处。
- **`Cmd+F` 的 Escape 有先后**：有查询先清空、空查询才关面板（`SearchField` 自己也会清空，所以顺序不能交给它）。
- **测试看不到 shell**：`sidebar.NewShell` 会真的建窗口，测试里不能用。`View` 是给测试用的并排近似布局，
  shell 的各槽位各自导出（`Left`/`LeftHeader`/`LeftToolbar`/`ContentTitle`/`Content`），
  `TestShellSlotsBuild` 逐个构建它们——改布局时保持这两条路径同步。

## 文档回写规则（硬规则）

1. **功能改了，文档跟着改，在同一次提交里**。用户可见的行为一变，就同时改：
   `README.md`（用法 / 快捷键 / 已知限制 / 结构）、`AGENTS.md`（结构表、坑、改动指引），
   以及相应 `docs/superpowers/` 下的规格或计划。实现细节与踩过的坑写进 AGENTS.md，不要写进 README。
2. **计划执行完之后补的功能，也要回写**。执行一份计划的过程中或之后追加的东西（用户新提的需求、
   执行中改的主意），除了当次实现之外，必须：
   - 追加到该计划末尾的「执行后的修订」小节（计划是**历史记录**：只追加，不改写原来的任务步骤）；
   - 若该计划的规格（`docs/superpowers/specs/`）里有被推翻的目标或非目标，同步改那份规格；
   - 若补的功能超出原计划的「目标」，按第 3 条进 CHANGELOG。
3. **功能变更在合适的时机写进 `CHANGELOG.md`**：
   - 「合适的时机」= 行为定稿、测试全绿、就在那一次提交里；不要攒到最后，也不要等发版才回想。
   - 落在四个小节之一：**新增** / **变更** / **修复** / **已知限制**（用户视角的一句话，不写实现细节）。
   - 未发版时写在「未发布」小节；发版时把「未发布」改成版本号 + 日期，再新开一个空的「未发布」。
   - 已知限制变了（例如某个限制消失了）要一起改掉，不能只往「新增」里加。
4. `CHANGELOG.md` 全是“用户会看到什么”，不给内部重构、重命名、测试补强单开条目（除非它们改变了行为）；
   内部的事写进 AGENTS.md 的坑与指引。

## 提交

中文提交信息，前缀 `feat:` / `fix:` / `test:` / `docs:` / `style:`。每个行为改动都先有失败的测试；
文档回写（README / AGENTS / CHANGELOG / 计划与规格）**并入同一个提交**，不单开一次“补文档”。
提交前 `go vet ./... && gofmt -l . && go test ./...` 必须干净。
