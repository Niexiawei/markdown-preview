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
go build -o mp.exe .        # 出单个二进制（没有 exe 图标、没有安装包）
go tool mygo build -platform darwin/universal,windows/amd64,linux/amd64
                            # 出三平台安装包到 dist/<os>-<arch>/（dmg、setup.exe、tar.gz、deb、rpm）
go test ./...               # 全部测试（视图测试用 ui.NewTester，无窗口）
go test ./internal/view/ -run TestTOC -v
go vet ./... && gofmt -l .   # 提交前必跑：vet 无输出、gofmt 无输出
```

`mygo` 这个 CLI 由 `go.mod` 的 `tool` 指令钉在 mygo v0.9.1（`go tool mygo`），
所以本地与 CI 用同一套打包器；`dist/` 与 `build/` 都在 `.gitignore` 里。

Go 版本下限由 `go.mod` 的 `go 1.27.3` 决定（mygo v0.9.1 要求）；本机旧版本时 `GOTOOLCHAIN=auto` 会自动取工具链。

## 结构

| 路径 | 职责 |
| --- | --- |
| `main.go` | 组装：单实例锁 → 配置 → workspace → 窗口（shell） → 窗口图标 → 托盘 → 文件监听 → MCP |
| `internal/config` | 配置读写（项目、活动项目、渲染模式、MCP），原子写 `config.json` |
| `internal/workspace` | 扫描 / 读取 / 解析缓存 / 标题缓存 / TOC 与 slug / 搜索 / 正文内查找（`find.go` 的 `FindIn`）；`Index` 接口 |
| `internal/watch` | `mygo.WatchFiles` → `ChangeSet`（新增 / 删除 / 重命名 / 写入） |
| `internal/mcp` | `POST /mcp` JSON-RPC 2.0，四个只读工具 |
| `internal/notices` | 消息账本与 `notices.json`（最新在前、无上限、未读标记）；原子写，**不 import ui**，可无窗口测试 |
| `internal/view` | Onevra 主题（`theme.go`）与外壳各区域：侧栏顶栏（项目面板 + 右键菜单）、树、侧栏工具栏（含铃铛）、标签栏（`tabs.go`，占 shell 的标题槽位）、页面第一行的路径与渲染开关（`titleRow`）、预览、消息历史浮层（`notices.go`）、MCP 设置面板；`find.go` 是悬浮搜索（三块浮岛 + 两块数据岛各自的可拖拽高度 `findGripBand`）与跳转，`native.go` 是原生正文与跳转（`jumpToBlock` / `jumpMarker`），`zoom.go` 是正文缩放；`system.go` 是桌面动作接口 |
| `internal/tray` | 托盘：图标与两条菜单（打开主页面 / 退出）、关闭窗口即隐藏、`Show`（托盘与第二次启动共用）、窗口/任务栏图标。图片在 `assets/`，用 `go:embed` 打包；纯逻辑（菜单项、关闭决策、图标尺寸）无窗口可测 |
| `.github/workflows/build.yml` | CI：gofmt / vet / 测试（Ubuntu，带 Pango），再在 macOS / Ubuntu 上交叉编译三平台安装包；推 `v*` 标签时把产物挂到 Release |

依赖方向：`view → workspace`、`view → notices`、`mcp → workspace`、`workspace → config`。

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
- **关闭窗口 = 隐藏，不是退出**：这条规则只在 `internal/tray` 里落地（`Install` 先建托盘，成功后才接管
  `Window.OnClose`）。`keep.quitting` 必须由 `App.OnBeforeQuit` 置位：`App.Quit()` 是「逐个窗口 `close()`」，
  没有这条放行路径，「退出」会被自己的 `OnClose` 拦住，应用永远退不掉。
- **托盘建不起来就退回「关闭即退出」**：先 `NewTray`、成功后才装 `OnClose`；反过来会把用户锁死在没有窗口、
  也没有托盘可点的状态（日志里打 `tray:`）。
- **托盘与窗口图标只经 `internal/tray`**（`Install` / `WindowIcon` / `Show`），**不在 `internal/view` 里建**：
  view 的函数每帧都会跑，在那里 `NewTray` 会每帧建一个图标；view 的测试也不该需要托盘或窗口。
- **图标分三层，别混**：可执行文件/安装包的图标来自 `mygo.json` 的 `resources/icon.png`，**只有 `mygo build`**
  会把它写进 exe/bundle（`go build` 不会）；窗口/任务栏图标是运行时 `win.SetIcon`（`internal/tray/assets/window.png`，256）；
  托盘是 `TrayOptions.Icon` + `IconIsTemplate: true`（`assets/tray.png`，32，墨色剪影 + 透明），明暗由系统
  自己涂色，**不要**再按 `Dark()` 换图、也不要打包 `tray-on-dark`。
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
  `leftHeader`（项目面板：点一行开/关、右键打开/复制/移除）、`leftToolbar`（动作图标 + 铃铛）各就各位。
  改布局时 `View` 与 `Content` 两条拼装路径要一起改（`TestShellSlotsBuild`、`TestTabsSitAboveTheTitle`）。
- **改提示消息**：写入只调 `internal/view/notices.go` 的 `notify` / `notifyDone` / `notifyWarn` / `notifyErr`，
  不要直接在 view 里 `c.AddToast`（绕过了账本、去重与历史）；浮层与铃铛也在这个文件（`bellButton` /
  `noticePanel` / `noticeRow` / `noticeBadgeText` / `noticeTime`）。账本自己的行为（落盘、上限、未读）改
  `internal/notices`，并补 `TestMessagesSurviveARestart` 一类的测试；view 侧补 `TestTheHistory*`、`TestTheBadge*`。
- **改正文内查找**：匹配规则只改 `workspace.FindIn`（字面子串、忽略大小写、上限 500），补 `TestFindIn*`；
  面板与跳转在 `view/find.go`（三块浮岛 `findIsland` / 匹配列表 / 上下文预览 / `Cmd+F` 与 `Cmd+G`）；
  命中跳转只调 `jumpToBlock`，与目录、链接共用同一条路（见「已知的坑」里的 `State.jump`）。
- **改搜索面板的尺寸**：高度常量全在 `find.go` 顶部（列表默认 132 / 最小 72 / 最大 320，上下文默认 148 /
  最小 96 / 最大 420，拖拽带 `findGrip` 10），可拖拽的是 `findGripBand(c, which)`；读取一律经
  `findHeightOf(which)`（自己的上下限只在那里钳制一次），窗口给的余量由 `findRoom` / `findHeightMaxOf` 管，
  每帧的 `fitFindHeights` 决定真正显示多高（`shownList` / `shownPreview`）；补 `TestDragging*`、
  `TestDoubleClickingABandPutsTheHeightBack`、`TestTheBandsMoveWithTheArrowKeys`、
  `TestThePanelNeverGrowsPastTheWindow` 与 `TestFindIslandsKeepTheirGaps`。
- **改标签栏**：`view/tabs.go` 的 `openTab` / `closeTab` / `visibleTabs` / `tabBar`，补 `TestTabBar*` 与 `TestClosingTabs`。
- **加桌面动作（打开目录、复制、确认）**：扩 `view.System`（`system.go`）并在测试里扩 `fakeSystem`，
  不要在 view 里直接调 `mygo.Dialog` / `mygo.Shell` / `mygo.Clipboard`：`LeftHeader` 在侧栏收起时不构建，
  应用内弹窗画不出来，测试也跑不了真实对话框。
  改完跑 `TestShellSlotsBuild`，并在 `View` 里保持同样的拼装。
- **改托盘菜单**：只改 `internal/tray/tray.go` 的 `menuItems`（保持两项、无分隔线），并补
  `TestTrayMenuOpensTheWindowOrQuits`；改关闭行为只改 `keep.go`，`TestQuittingLetsTheWindowClose` 必须仍然通过。
- **换图标**：换 `internal/tray/assets/window.png`（256 方形）与 `assets/tray.png`（32 方形、墨色剪影 + 透明），
  同时换 `resources/icon.png`（1024 方形，`mygo build` 用）；`TestIconSizes` 会抓住尺寸不符。
- **改构建/发布**：只改 `.github/workflows/build.yml`；本地复现用 `go tool mygo build -platform <goos/goarch>`，
  产物在 `dist/<os>-<arch>/`（`mygo.json` 没设 `out`，默认就是 `dist`）。版本以 `mygo.json` 的 `version` 为准：
  发版先改它，再推 `v<同一版本>` 标签（workflow 会校验，不匹配就不构建）。
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
- **`App.SetName` 会改配置目录**：`Path(PathUserData)` 是 `UserConfigDir/<App.Name()>`，名字一变，
  `config.json` / `mcp.json` 就跑到新目录，看起来像「配置丢了」。要改显示名就改 `mygo.json`（打包名）或
  shell 的 `Title`，不要在 `main.go` 里 `SetName`。
- **「退出」退不掉 / 托盘留幽灵图标**：前者是 `keep.quitting` 没接上（见硬规则）；后者是漏了
  `App.OnWillQuit` 里的 `tray.Destroy()`，Windows 会把图标留到鼠标划过才消失。
- **`IconIsTemplate` 只认不透明像素**：托盘图必须是「实心剪影 + 透明底」；带底色方块或半透明渐变，
  在深色任务栏上会被涂成一团白方块。
- **CI 的测试任务需要 Pango**：原生 UI 在 Linux 上用 Pango 排版（mygo 用 purego 动态加载），
  缺了它 `ui.NewTester` 里的文字量宽会退化成 stub，标签/标题相关的断言会挂——所以 workflow 里那段
  `apt-get install libpango-*` 不能删。
- **产物没有代码签名**：macOS 是 ad-hoc 签名（Gatekeeper 拦）、Windows 没有签名（SmartScreen 警告）。
  要签名按 mygo 的 `docs/github-actions.md` 加证书与 `macos.signingIdentity` / `macos.notarize`；
  用 signtool 签名时 Windows 那一行要改成 `windows-latest`。
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
- **没有底部状态栏**：提示一律走 `notify*` → `internal/notices` 的账本 → 构建期 `updateToasts` 弹右下角 Toast /
  铃铛浮层看历史。不要在 view 里直接 `c.AddToast`（会绕过账本、去重与历史）；改 Toast 落位只改
  `view.ToastOptions()`。
- **浮层开着 = 已读**：`bellButton` 在开头就把未读标掉（`MarkSeen` 没有变化时不写盘），所以新消息到达时
  不会在正显示着的列表上冒出角标。
- **Toast 按 `noticeShown` 水位线去重**：一次点击会建两遍视图（同下面那条坑），不去重会弹两个、读屏还会念两遍。
  `New` 里把水位线对到账本最新一条，否则重启会把上次的旧消息全部重播一遍（历史要留，Toast 不要）。
- **`Content` 必须 `drain()`**：shell 只有它这一条构建路径，漏了它，`Post` 投递的 watcher 变更在真实窗口里
  永不生效（历史 bug，2026-11-24 修）。`View`（测试用）与 `Content` 的 `drain` → `handleShortcuts` →
  `updateToasts` 三步顺序要一致。
- **`view.New(…, "")` 表示无用户数据目录**：账本只在内存里，`Save` 是 no-op。`dir` 非空时是 `notices.json`
  所在目录（与 `config.json` 同目录）。测试一律传 `t.TempDir()`，不要在仓库根目录写出文件。
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
- **搜索面板是三块浮岛**（`findIsland`：输入框 / 匹配行 / 上下文），外层只给位置，**没有背景**，
  也**没有 `Gap`**：间距由「匹配列表岛的 6 DIP 上外边距」+「两条 `findGrip`（10 DIP）高的拖拽带
  （列表下、上下文下各一条）」构成，改间距要三处一起改，否则会出现「带子占位 + 间距」的双份留白。
  别再把它塞回一个带 Surface 的整块卡片里：那样列表最后一行和上下文第一行会读成一列。
  三块岛都给读屏名（`搜索输入` / `匹配行` / `命中上下文`，也是测试里 `tt.Find` 取岛矩形的唯一办法），
  两条带子也有（`调整匹配列表高度` / `调整命中上下文高度`）。上下文岛顶上那行 `第 N 行上下文` 是给人分清的。
  岛内宽度用 `findPaneInner`，别再用 `findPaneWidth - 2*Space(1)`。
- **拖拽带的高度算法**：`Dragged()` 返回的是**上一帧读到之后的增量**（`ui/runtime.go` 的 `forgetInput`
  每帧把 `dragX/dragY` 清零），所以每帧是把 `dy` 加到当前高度上；写成「从按下起的累计位移」会指数放大，
  一拖就撞上限。mygo 的 `ui.Split` 是同一套语义。
  另外 **`dy == 0` 的按下不能写高度**（`switch { case !held: …; case dy != 0: … }`）：否则点一下带子就会把
  「用户想要的高度」改写成「窗口现在能显示的高度」，窗口变高后不再恢复。
- **拖到极限要保留过冲**（`findState.listOver` / `previewOver`）：`want = 高度 + dy + 过冲`，
  `过冲 = want - 钳制后`，松手时归零。没有它，「拖过头再拖回来」要等指针先回到边界才开始变化，手感是粘住的；
  `ui/split.go` 的 `splitOvershoot` 解决的是同一个问题。
- **岛高量在岛上，不是滚动容器上**：`findList` / `findPreview` 给 `findIsland(...)` 设 `Height(shown*)`，
  里面的 `ScrollBoth` 用 `Grow(1)` 吃掉剩下的空间；这样 `岛.Y + 岛.H` 才等于下一条带子的位置（测试靠它）。
- **面板高度受窗口约束**：`findRoom` = 窗口高 − 面板顶部的 header/标题行 − 底部留白 − 输入岛高（`fieldH` 由上一帧
  `Bounds()` 量出）− 两条带子；`findHeightMaxOf` 让每条岛的上限再减去另一条岛**实际显示**的高度（`shown*`，
  不是它要的高度——否则窗口一矮，拖其中一条会把另一条直接压到地板）；`fitFindHeights`
  每帧兜底（窗口变矮时**上下文先让出高度**，再轮到列表），只改 `shown*`，不改用户要的 `listH` / `previewH`，
  所以窗口变高会自己恢复。窗口高来自 `State.SetContentHeight`（`main.go` 用 `win.ContentSize()` 接），
  测试里不设就用默认 700。
- **拖拽带能拿键盘**：`Focusable()` + 按下时 `Focus()` + `HandleInput` 里接 `InputKeyDown` 的 `↑`/`↓`
  （步长 `t.Space(2.5)`），与 `ui.Split` 的分隔条一致。`HandleInput` 在**下一帧构建之前**跑，
  那里**不能**用 `*ui.Context`（主题、shell 都拿不到），所以余量在构建期算好、闭包只捕数字（曾经的 panic 就在这里）。
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
- **`Options.Block` 必须常挂**（`s.md.Block = s.jumpMarker` 每帧都设）：mygo 一旦看到它非 nil，会给**每个块**
  多包一层 `ui.Column`，而这一层挡住了"块的边距"和"块之间的间距"的合并（`blocks()` 的 `Margin` 落在包裹层上），
  于是**同一份文档在挂与不挂时的高低差几十 DIP**。只在跳转那一帧挂，就会出现"按挂着时的布局量位置、按不挂时的布局显示"，
  目标被顶到画面外（用户报过的「新文档点目录，标题跑到可视区外」就是这个）。常挂的代价：每帧每块多一个节点
  （实测 583 块的文档 +3.5% 帧时间）、文档间距比 mygo 默认略松（0.4% 量级），换来"量位置与显示是同一套布局"。
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
