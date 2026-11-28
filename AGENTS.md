# AGENTS.md

本仓库是一个**纯 Go 的桌面 markdown 预览器**：多项目管理，左侧 md 文件树，右侧「文档目录 + 正文」，
正文可在原生渲染（`ui/markdown`）与 WebView 渲染（`ui.WebView`）之间切换；外部改动（agent / 编辑器）
实时跟随；另带一个默认关闭的本地 MCP server。

UI 框架是 `github.com/egoist/mygo v0.10.0`（原生 UI 路径，**不用** web 前端、不需要 bun）；
布局、主题与图标来自 `github.com/oneo-me/onevra/source v0.2026.19`（`shells/sidebar` 外壳、`theme.Onevra`、
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
MDPREVIEW_SHOT_DIR=/tmp/shots go test ./internal/view -run TestScreenshots
                            # 把界面状态渲染成 PNG，改完界面自己看一眼（见「GUI 验证」）
MDPREVIEW_PANEL_DOWN=1 go run .
                            # 在 Windows/Linux 上预览 macOS 的头部形态（面板下沉、折叠按钮尾随）
```

`mygo` 这个 CLI 由 `go.mod` 的 `tool` 指令钉在 mygo v0.10.0（`go tool mygo`），
所以本地与 CI 用同一套打包器；`dist/` 与 `build/` 都在 `.gitignore` 里。

Go 版本下限由 `go.mod` 的 `go 1.27.3` 决定（mygo v0.10.0 要求）；本机旧版本时 `GOTOOLCHAIN=auto` 会自动取工具链。

## GUI 验证（硬规则）

GUI 要真实窗口；**无头环境里验证视图一律用 `ui.NewTester`，不要 `go run .`**（它会弹窗、读写真用户目录、
而且没法断言）。三层，从下往上：

1. **视图断言**（`go test ./internal/view/`）：`ui.NewTester(s.View, 1000, 700)` + `HasText` / `Find` /
   `Access` / `Texts` / `Click`。**不依赖像素**：只断言文字在不在、控件的位置与尺寸关系、无障碍名与状态。
   新组件、新面板、新交互都要有这一层测试。
2. **看图**（人工核对，不是断言）：`MDPREVIEW_SHOT_DIR=<目录> go test ./internal/view -run TestScreenshots`，
   用例表是 `internal/view/screenshots_test.go` 的 `shotCases()`，每个界面状态渲染成一张 PNG。
   不设环境变量就 SKIP，CI 不写文件。**改完界面必须自己看一遍**：文字都在不等于布局是对的——
   底色铺没铺满整行、箭头与图标是否同列、loading 大小、弹层有没有盖住东西，只有图能回答。
   加了新面板 / 新状态就加一条用例（`build` + 可选 `view`/`act`/`settle`）。
3. **真窗口**（只有人能做的部分）：拖动分隔条、右键菜单、系统对话框、托盘、文件关联。
   这些写进提交信息或 PR 的「手动验证」里，不要假装测过。

写截图或异步界面的测试时记住：**Tester 的时钟是虚拟的**，`Frame()` 不会让扫描 goroutine 落地——
要 `tt.Advance(20*time.Millisecond)` + `time.Sleep(2*time.Millisecond)` 再来几帧（`shotCase.settle` 就是干这个的）。

## 结构

| 路径 | 职责 |
| --- | --- |
| `main.go` | 组装：单实例锁 → 配置 → workspace → 窗口（shell） → 窗口图标 → 托盘 → 文件监听 → 启动参数里的 md → MCP；`windowState` 供第二实例 / `OnOpenFile` 用 |
| `internal/config` | 配置读写（项目、活动项目、渲染模式、MCP），原子写 `config.json`；`Project.File` 非空即单文件项目（`Root` 是它所在目录） |
| `internal/workspace` | 扫描（上限 `MaxEntries` = 100 万）/ 读取 / 解析缓存 / 标题缓存 / TOC 与 slug / 搜索 / 正文内查找（`find.go` 的 `FindIn`）/ 文件名模糊匹配（`fuzzy.go` 的 `FuzzyMatch`）/ 单文件项目与归属（`single.go` 的 `Tree`、`Locate`）/ 树的就地增删改（`patch.go` 的 `InsertDoc` / `RemoveDoc` / `RenameDoc`）；`Index` 接口 |
| `internal/launch` | 命令行 / 系统递来的参数里挑出 md 文件（`Documents`，纯函数，不 import ui） |
| `internal/watch` | `mygo.WatchFiles` → `ChangeSet`（新增 / 删除 / 重命名 / 写入）；单文件监听（`WatchFile` + `FoldFile`）、监听上限 |
| `internal/mcp` | `POST /mcp` JSON-RPC 2.0，四个只读工具 |
| `internal/notices` | 消息账本与 `notices.json`（最新在前、无上限、未读标记）；原子写，**不 import ui**，可无窗口测试 |
| `internal/session` | 会话快照 `session.json`（标签页与活动标签、树里被折叠的目录、侧栏与每个文档的目录宽度）；原子写，**不 import ui**，可无窗口测试 |
| `internal/view` | Onevra 主题（`theme.go`）与外壳各区域：侧栏顶栏（项目面板 + 右键菜单；macOS 上面板下沉为 `panelRow`，槽位留空）、树（`tree.go` 的 `treeKey` / `roots` / `children` / `rowOf` / 自绘行，`scan.go` 的异步扫描与上限，`locate.go` 的 crosshair 定位）、选择与按行复制（`treesel.go` 的 `treeSel` / `selectRow` / `menuTargets` / `copyRow*`）、侧栏工具栏（含铃铛）、标签栏（`tabs.go`，占 shell 的标题槽位）、页面第一行的路径与渲染开关（`titleRow`）、预览、消息历史浮层（`notices.go`）、MCP 设置面板；`find.go` 是悬浮搜索（三块浮岛 + 两块数据岛各自的可拖拽高度 `findGripBand`）与跳转，`native.go` 是原生正文与跳转（`jumpToBlock` / `jumpMarker`），`zoom.go` 是正文缩放；`system.go` 是桌面动作接口 |
| `internal/tray` | 托盘：图标与两条菜单（打开主页面 / 退出）、关闭窗口即隐藏、`Show`（托盘与第二次启动共用）、窗口/任务栏图标。图片在 `assets/`，用 `go:embed` 打包；纯逻辑（菜单项、关闭决策、图标尺寸）无窗口可测 |
| `.github/workflows/build.yml` | CI：gofmt / vet / 测试（Ubuntu，带 Pango），再在 macOS / Ubuntu 上交叉编译三平台安装包；推 `v*` 标签时把产物挂到 Release |

依赖方向：`view → workspace`、`view → notices`、`view → session`、`view → config`、`mcp → workspace`、`workspace → config`、
`launch → workspace`（只用 `IsMarkdown`）、`watch → workspace`。

## 硬规则

- **不引入第三方 markdown / GFM / 高亮 / web 框架依赖。** 解析与 HTML 都用 `ui/markdown/parser`
  （`parser.Parse` + `Document.HTML()`）。
- **`internal/workspace` 与 `internal/mcp` 不得 import `.../ui`**（只允许 `.../ui/markdown/parser`）。
  这样 MCP 能用 `httptest` 测，不需要图形环境。
- **一份解析，两种后端。** 原生喂源字符串给 `markdown.View`，webview 喂同一个 `*parser.Document` 的
  `HTML()`；绝不为 webview 另引解析器，否则两模式渲染会不一致。
- **视图状态归窗口线程。** 视图是 state 的纯函数；其他 goroutine 只通过 `State.Post(fn)` 投递，
  帧首 `drain()` 应用，然后 `win.Invalidate()` 唤醒。不要在 view 热路径里读盘或解析。
- **扫盘只跑在扫描 goroutine 上**（`view/scan.go` 的 `loadTree` → `s.scan(workspace.Tree)` → `Post(applyTree)`）：
  窗口线程只应用结果。每一条回程都带**项目 ID + 代数**（`scanGen`），项目被关掉 / 又发起新扫描时旧结果一律丢弃。
  测试用 `SetSyncScan()` 走同步路径，别在别处偷偷同步扫。
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
  `c.Theme().Accent`，需要警示色就 `c.Theme().Warning`。**`t.Selection` 由 `AppTheme` 覆盖成主色的洗淡版**：
  onevra 的调色板给它留的是**橙色**（`RGBA(230,101,43,0.22)`），不覆盖的话多选底色会是橙的；
  `t.Selection` 同时是正文里选中文字的高亮，所以这一条也让它跟着主色。
- **图标只用 `icons/lucide`**，用 `components.Icon` / `components.IconButton`；图标按钮必须给 `label`
  （无障碍名，也是测试定位的唯一方式，图标本身不是文本）。侧栏底部的动作按钮用
  `IconButtonPlain`（32 DIP 方块 / 20 DIP 图形），**不要**用 `IconButtonCompact`：用户反馈太小。
  例外只有侧栏**顶部**那一行的三个树操作（`treeBulkActions`）：`IconButtonCompact` + `Size(treeBulkSize, treeBulkSize)`
  （24），用户 2026-11-25 明确要求比底部的小一号。
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
  **滚动条在窗格边缘，不在内边距里**：`toc` 的列不再加 `Padding`（只有表头那一行加），
  内边距给了滚动容器里的列（`Padding(0, tocPadding, tocPadding, tocPadding)`）。两个后果：
  （1）`MinWidth` 是**含内边距的边框盒**，所以那里写的是 `s.tocW`（整个窗格宽）而不是
  `s.tocW-2*tocPadding`——写小了会让行缩到 232 以内，`TestTOCRowsFillThePane`（点窗格右缘选标题）就红；
  （2）滚动容器带无障碍名「目录列表」，这是测试量它位置（`X==0`、`W==窗格宽`）的唯一办法。
- **改跳转落点（顶部/居中/是否平滑）**：只改 `view/native.go` 的 `jumpMarker`（`ScrollIntoViewOptions`）与
  `view/webview.go` 的 `headingScrollJS`，两边要一致；不要在调用处各写一套滚动。
- **改渲染样式**：原生在 `markdown.Options`（`loadDoc` 里装配）；webview 在 `webview.go` 的 `docCSS`。
- **改标签栏/标题行/侧栏**：`tabs.go` 的 `tabBar`（标签、⌄ 列表）挂在 shell 的 `ContentTitle` 槽位（内距由外壳的 `TitleInsetLeft/Right` 提供，取值见 `view` 的共享常量）；
  `view.go` 的 `titleRow` → `contentTitle`（路径 + 复制 + `modeToggle`）是 `Content` 的第一行；
  `leftHeader`（项目面板：点一行开/关、右键打开/复制/移除；macOS 上它让位给 `panelRow`——主体首行，
  两者共用 `panelItems`，改面板内容两处一起变，测试见 `panel_test.go`）、`leftToolbar`（动作图标 + 铃铛）各就各位。
  改布局时 `View` 与 `Content` 两条拼装路径要一起改（`TestShellSlotsBuild`、`TestTabsSitAboveTheTitle`）。
- **改树的展开 / 折叠**：批量只改 `tree.go` 的 `setTreeOpen`（往 `s.treeOpen.Open` 里增删 `treeKey`；
  **多项目时折叠连项目节点一起收**（收完只剩项目名，与单项目窗口「收成顶层」一致；2026-11-25 用户按方案 b 定稿，
  推翻了 2026-11-24 的「折叠不动项目节点」），展开把节点与目录一起开）；
  单行开合是 `setTreeKeyOpen` / `toggleTreeKey`；
  三个入口是 `view.go` 的 `treeBulkActions`（`leftHeader`，项目选择按钮左边）、项目节点的 `projectActions`
  （项目名后面那两个行内小按钮，label 带项目名以便区分）与 `projectMenu` + 项目面板每行的右键菜单。
  label 不得与目录工具条重名（树是 `展开全部目录树`，目录是 `展开全部目录`），重名会让 `tt.Find`、读屏与快捷键提示都分不清。
- **改多选与按行复制**：底色是 `treeRow` 里自绘的一层（见「已知的坑」里那条：选中行比行矮一点，
  避免多选连成一块；颜色是 `AppTheme` 的 `t.Selection`，它被覆盖成主色的洗淡版）。选择模型与复制动作只在 `internal/view/treesel.go`（`selectRow` 的普通点=单选、
  修饰键=加/减且不执行行的动作；`menuTargets` 决定右键菜单作用于整组还是那一行；`copyRows` / `copyRowNames` /
  `copyRowPaths` 吃 `[]treeKey`，路径一行一个）。行的入口是 `tree.go` 的 `treeRow` 与 `treeMark`（**两个都调
  `pickRow`**，否则按住 Ctrl 点箭头会顺手折叠目录）；`Cmd+C` 的先后在 `fileacts.go` 的 `handleShortcuts`
  （有选中走 `copyTreeSelection`，没有才 `copyDocPath`）。剪枝在 `afterTreesChanged` / `dropTree` / `dropTabsOf` /
  `closeTab`，**折叠目录不清选择**。补 `TestCtrl*`、`TestCmdCCopies*`、`TestTheTreeMenuActsOn*`、
  `TestPickedRows*`，并在 `screenshots_test.go` 的 `11-multi-select` 里看一眼底色。
- **改会话记忆（重启恢复什么）**：改 `internal/view/state.go` 的 `restoreSession`（恢复）与 `saveSession`（快照），
补 `TestRestart*`、`TestTabChangesAreWrittenBack` / `TestFoldChangesAreWrittenBack`。写盘点 = 五个标签变更点
（`openTab` / `closeTab` / `dropTab` / `dropTabsOf` / `renameTab`）+ `toggleTreeKey` / `setTreeOpen` / `locateDoc`；
**`setTreeKeyOpen` 里绝不写盘**（`setTreeOpen` 循环调它上万次，写盘只在入口尾部一次）。恢复必须在 `New` 里
`reloadTree` **之前**（closed 键要先进 `treeSeen`，否则扫描落地时 `noteNewDirectories` 把折叠目录全部展开）；
活动标签无效就空着（不擅自挑一个）。
- **改提示消息**：写入只调 `internal/view/notices.go` 的 `notify` / `notifyDone` / `notifyWarn` / `notifyErr`，
  不要直接在 view 里 `c.AddToast`（绕过了账本、去重与历史）；浮层与铃铛也在这个文件（`bellButton` /
  `noticePanel` / `noticeRow` / `noticeBadgeText` / `noticeTime`）。账本自己的行为（落盘、上限、未读）改
  `internal/notices`，并补 `TestMessagesSurviveARestart` 一类的测试；view 侧补 `TestTheHistory*`、`TestTheBadge*`。
  **浮层的尺寸常量在 `notices.go` 顶上**：宽度 `noticePanelWidth` 360 DIP，高度是**窗口内容高度的比例**
  （`noticePanelMinShare` 0.5 与 `noticePanelMaxShare` 0.7，靠 `MinHeight` / `MaxHeight` 压在面板列上，
  列表用 `Grow(1)` 吃剩下的，超过上限就内部滚动）。比例而不是 DIP，就是为了 resize 时跟着变；
  面板列带无障碍名「消息历史」，这是测试量它高度的唯一办法。
  两个测试陷阱：（1）`TestTheHistory*` 里的 400/560 是**字面量**，不要改成引用常量，否则改常量就不再失败；
  （2）量高度要**直接建 `s.noticePanel`**，不要通过 `s.View` 量：测试用的并排布局里侧栏不一定撑满高度，
  弹层的可用空间会小于真实壳层，量出来的高度会被弹层位置钳住（曾经量到 401 而不是 560）。
- **改正文内查找**：匹配规则只改 `workspace.FindIn`（字面子串、忽略大小写、上限 500），补 `TestFindIn*`；
  面板与跳转在 `view/find.go`（三块浮岛 `findIsland` / 匹配列表 / 上下文预览 / `Cmd+F` 与 `Cmd+G`）；
  命中跳转只调 `jumpToBlock`，与目录、链接共用同一条路（见「已知的坑」里的 `State.jump`）。
- **改文件名搜索**：匹配规则只改 `workspace.FuzzyMatch`（子序列、大小写不敏感、空白查询=不匹配、**不编译正则**），
  补 `TestFuzzyMatch*`；候选集合只改 `treesearch.go` 的 `refreshCandidates`（树序、不扫盘、在 `reloadTree` 后重算），
  补 `TestCandidates*`；过滤视图是 `tree.go` 的 `roots` / `children`（搜索时只给有候选的项目、有候选的目录与候选本身），
  临时展开记在 `treeSearch.opened` 并在 `closeTreeSearch` / 清空查询时折回；
  只有当前候选上一层底色（`isCurrentCandidate` + `theme.go` 的 `currentCandidateTint`）；
  快捷键路由与 `Esc` / `↑` / `↓` 在 `searchShortcuts`，补 `TestCmdF*`、`TestShiftCmdF*`、`TestArrows*`、`TestEscape*`。
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
- **改树的容器 / 行**：树是 `tree.go` 的 `treeBody`（`ui.Outline` + `OutlineState[treeKey]`），行是自绘的
  `ui.Row`（点击、右键菜单、选中底色都在那里）。**只有目录（TOC）那一棵还是 `ui.Tree`**（`native.go` 的 `toc`，
  用 `s.openOf` / `s.open`），两套展开状态别混。树的虚拟化靠 `treeOpen.Generation`：任何改动 `children()` 结果的操作
  都必须 `bumpTree()`（或至少 `treeOpen.Generation++`），否则 Outline 会继续用它缓存的旧行。
- **改扫描的上限 / 提示**：`workspace.MaxEntries`（100 万）是扫描与监听共用的那个数字，改它要同时想清楚两处；
  扫描拒绝时视图 `stopWatch` 并提示「目录过大无法打开，请缩小目录范围」，监听撞上限时提示「目录过大，已停止监听：…（刷新仍可用）」
  （`view/scan.go` 的 `watchFailure`）。

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
  `config.json` / `notices.json` / `window-state.json` / `mcp.json` 就跑到新目录，看起来像「配置丢了」。
  要改显示名就改 `mygo.json`（打包名）或 shell 的 `Title`，不要在 `main.go` 里 `SetName`。
- **未打包时目录名就是 exe 文件名**：`App.Name()` 没有 bundle 名时取可执行文件名，所以
  `go build -o mdpreview.exe .` 用 `…\AppData\Roaming\mdpreview\`，而本文件推荐过的 `go build -o mp.exe .`
  会另起一个空的 `…\Roaming\mp\`（本机两者都存在过）。调试时记住自己用的是哪个名字，否则会以为历史/配置没写盘。
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
- **`packageFileExtensions` 只有 `mygo build` / `mygo dev` 会写进二进制**：`go run . x.md` 与未打包的 exe
  **不会**触发 `OnOpenFile`（exe 也不注册文件关联），所以 `main.go` 必须自己解析 `os.Args[1:]`；
  macOS 的「用此应用打开」只走 `OnOpenFile`，两边都要接，`State.OpenDocument` 幂等所以重复投递无害。
- **单文件项目要 `watch.WatchFile`**：`watch.Watch(root)` 会盯住整个目录，而独立文件可能就在 `~` 下。
  mygo 的单文件监听把事件路径报成 `"."`，`watch.FoldFile` 把它翻成文件名；删掉/换掉目录时它报 `"."` + `IsDir` 并结束（`io.EOF`）。
- **`workspace.Tree(p)` ≠ `BuildTree(p.Root)`**：单文件项目的树就是那一个文件节点，写错会去扫它所在的整个目录。
- **树的展开状态分两套**：侧栏树是 `s.treeOpen.Open`（`treeKey{project, path}`，`path` 为 `""` 是项目节点），
  目录（TOC）那棵 `ui.Tree` 仍是 `s.open` / `s.openOf`（键 `"projectID:slug"`）。`setOpen` 已删，别复活；
  `openOf` 只给 TOC 用。
- **折叠记忆是显式 closed 集，不从 `Open` 推导**（`s.treeClosed`，写进 `session.json`）：树搜索的临时展开
  （`openForSearch` / `closeSearchOpen`）直接动 `Open`、不碰 `treeClosed`，快照才能在搜索开着时照常写而不把
  用户折叠的目录抹掉。恢复时 closed 键要**同时**进 `treeSeen`（否则 `noteNewDirectories` 把它们当新目录重新展开）。
  删除目录不清它的键（与 `treeSeen` 同语义：重建同名目录保持折叠）；`dropTree` 随 `treeSeen` 一起清掉整个项目的键。
- **扫描结果必须带项目 ID + 代数**：`applyTree` 先比 `scanGen`，再比 `cfg.IsOpen`；被关掉的项目、被更新的扫描取代的结果一律丢弃。
  一次点击/一次变动可能会连发几次扫描请求，`scanAgain` 把它们合并成一次。
- **`OutlineState.Rows()` 是上一帧的快照**：算行号一律用 `rowOf(roots, key)`（它自己按 `Open` 走一遍），
  否则「展开祖先 + 滚过去」在同一帧里会滚到邻居。`ListState.Visible()` 的 `last` 是**包含**的。
- **`ui.Outline` 的一行不是你建的**：缩进与箭头归 Outline，行的内容 `row(item)` 只是它的子元素；
  行底色因此画在内容上（`Padding` 之内），比旧的 `ui.TreeItem` 略窄。点击、右键菜单、项目行的两个小按钮都要自己装。
- **行的样子照 mygo 的 `ui.TreeItem` 来**：`treeBody` 每帧把 `s.treeOpen.Prefix` 设成**空函数**——Prefix 非 nil
  时 `ui.Outline` 的行没有内边距、没有 gap、也不画箭头，只剩「一个居中的容器」，所以行是自己画的整条
  （`Height(Space(7))` + `Radius` + `Padding(0, Space(1))`）：悬停 `t.SurfaceHover`、当前文档 `t.Accent` 底 +
  `t.AccentText` 字，底色因此**铺满整行**（这块以前栽过：底色只盖文字那一块，因为颜色画在 Outline 的内边距内侧）。
  **唯一的例外是多选**：选中的行用 `row.Draw` 自绘一层比行矮 `Space(0.5)` 的圆角填色（`p.Fill`），
  上下各留一点缝，否则相邻的选中行会连成一大块（用户 2026-11-27 提的）；悬停与当前文档仍是整行高。
  别改成「再包一层内容容器再给它上底色」：右键菜单与 `OnClick` 挂在行上时会收不到（菜单打不开、
  行也点不动），而 `IgnoreInput` 又会让 `tt.Click` 找不到行里的文字。
  `Hovered()` 在构建期读，它会让 mygo 为悬停变化重建一帧（`hoverBuilt`），不要改成读上一帧的 `st`。
- **箭头与文件图标都在 `treeMark` 里自绘**（`treeMark` 的 box = `Size(Space(4))`，与 `ui.TreeItem` 的 arrow box 同尺寸）：
  目录/项目行画三角（`p.StrokePath`）并 `OnClick` 切换，文件夹在名字前，文档的图标就在 arrow box 里。
  项目节点的文件夹图标是 `Rem(1.25)` + `AlignSelf(Center)` + `MarginEnd(Space(1))`（比名字大一号，别贴在一起）。
  代价：mygo 的 `OutlineRow.Arrow()`（带 Alt/Option「全部展开」）不能用，左/右键与读屏的展开仍由 Outline 自己接。
  行本身也有 `OnClick`，靠「最内层可点击元素赢得点击」保证只切换一次（`TestClickingTheArrowOfADirectoryOpensItOnce`）。
- **loading 用 `plugins/loading.Snake`**（`loading.Options{Size: loading.DefaultSize}` = 20 DIP，
  `MarginStart(Space(0.5))`）：`components.Spinner` 的默认直径是 `Space(8)`（32），放在树的行里太大。
  它要**紧跟名字**：行的名字 `Text` **不要 `Grow(1)`**，行尾放 `ui.Spacer(c)` 把 `projectActions` 的按钮推到右边——
  名字一 Grow，loading 就被挤到名字与按钮的中间。
- **任何树的变化都要 `bumpTree()`**：`OutlineState` 会把上一帧的行走一遍的结果缓存起来，只有 `Generation` /
  `Open` 的 stamp / roots 之一变了才重走。扫描结果落地只换内容（roots 与 Open 都不变），漏掉 `bumpTree` 就会出现
  「项目没有子项、没有箭头，点一下才有」——`afterTreesChanged` 里必须调它。
- **树要立刻显示新项目**：`loadTree` 先 `showPlaceholder`（`projectTree{project}`、`root` 为 nil），
  行由 `treeRow` 的项目分支画（名字 + `scanning` 时的 spinner）；`reloadTree` 用 `root != nil` 判断「已读过」，
  别把占位当成读完了。文件监听也在 goroutine 里建（`watchProject` 字段 + `watches[id]` 先占位），
  否则 `mygo.WatchFiles` 的首次遍历会把窗口冻住几秒。
- **树的焦点跟踪用 `ObserveInput`，不是 `HandleInput`**：`HandleInput` 会顶掉 Outline 自己的输入处理，
  `ObserveInput` 只是旁听（`treePress` 只认 `InputPointerDown`）。
- **`workspace.Locate` 两侧都先 `EvalSymlinks`**：Windows 的 8.3 短名（`NIEXIA~1`）、符号链接会让
  「项目根」「文件路径」字面不同，不解析就会把项目内的文件当成独立文件。`OpenDocument` 自己也解析一次。
- **浮层开着 = 已读**：`bellButton` 在开头就把未读标掉（`MarkSeen` 没有变化时不写盘），所以新消息到达时
  不会在正显示着的列表上冒出角标。
- **Toast 按 `noticeShown` 水位线去重**：一次点击会建两遍视图（同下面那条坑），不去重会弹两个、读屏还会念两遍。
  `New` 里把水位线对到账本最新一条，否则重启会把上次的旧消息全部重播一遍（历史要留，Toast 不要）。
- **`Content` 必须 `drain()`**：shell 只有它这一条构建路径，漏了它，`Post` 投递的 watcher 变更在真实窗口里
  永不生效（历史 bug，2026-11-24 修）。`View`（测试用）与 `Content` 的 `drain` → `handleShortcuts` →
  `updateToasts` 三步顺序要一致。
- **`view.New(…, "")` 表示无用户数据目录**：账本只在内存里，`Save` 是 no-op。`dir` 非空时是 `notices.json`
  所在目录（与 `config.json` 同目录）。测试一律传 `t.TempDir()`，不要在仓库根目录写出文件。
- **macOS 的头部是「面板下沉 + 按钮尾随」，两件事必须同源**：`State.panelDown`（`New` 里 `runtime.GOOS == "darwin"`，`MDPREVIEW_PANEL_DOWN=1` 可在任何机器上预览）同时驱动面板位置与 main.go 的 `LeftToggleTrailing: state.PanelDown()`。面板内容只有一份：`panelItems` 同时喂标题栏槽位的 `leftHeader`（macOS 上构建为空）与主体首行的 `panelRow`（高 `panelRowHeight` = 40），改面板内容两处一起变，`Left` 与 `View` 两条拼装路径都按 `panelDown` 分叉。折叠与展开动画期间 onevra 把折叠按钮放回标题栏左端（交通灯之后），动画结束才跳到右端——这是 onevra 的设计，不是 bug。标签条起点在两种模式下不变（onevra 的 trailing 分支与 leading 分支都按 `TitleInsetLeft` 偏移）。面板测试显式设置 `s.panelDown`，不依赖 GOOS（darwin 机器上跑测试也要绿）。
- **标题槽的内距来自 onevra 的 `TitleInsetLeft` / `TitleInsetRight`**（默认 24 / 8，非正值取默认；偏移随展开进度渐变——折叠/动画中保持外壳原公式，展开后全量，标题不会压进 toggle）。本项目的取值是 `view.go` 导出的 `TitleInsetLeft`（16）/ `TitleInsetRight`（8）：`main.go` 传给 `ShellOptions`，`View` 的仿垫行用同一组常量——**单一来源，别在两处写字面量**（改了外壳忘仿垫、测试又抓不到，就是漂移）。`tabBar` 不再自己管内距：`tabBarAt` / `Translate(-12)` / `titleInsetGap` / `titleInsetWanted` 已删。**标签条自己不加左右内边距**（留白交给标题槽，否则是双层留白）。钉位测试 `TestTabsStartAtTheShellInset` 用字面量 316（300 侧栏 + 16），改常量它会红，逼你有意识地更新预期。
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
- **滚动条一律贴窗格/岛的边缘，不放在内边距里**（用户两次提的同一个问题）：目录树、正文、搜索面板两块数据岛
  都是「外层不加 padding，滚动容器铺满，内边距给滚动内容」：目录树的 `toc` 自己拼，正文看 `nativePreview`
  （padding 在内容列上，`bodyJumpGap` 的透明上边框仍在滚动容器上），搜索面板用 `findScrollPane`。
  原因是滚动条的命中区在滚动容器的内边距盒里：padding 在外层时滚动条会压在文字上，窗格边缘又正好是命中区。
  两个量尺寸的坑：`MinWidth`/`Width` 是**含内边距的边框盒**（目录树写 `s.tocW`，正文列写 `bodyPaneWidth-2`，
  搜索面板写 `findPaneInner+2*findIslandPad`）；滚动容器要带读屏名（`目录列表` / `匹配行列表` /
  `命中上下文列表`）才能在测试里量到它的位置。正文那处改不了 Tester 里可断言的量（滚动条不是带名字的节点，
  文本几何也没变），靠 `TestBodyColumnFollowsPaneWidth` 保证不退化。
- **`Cmd+F` 按焦点分派，而焦点是上一帧的**：`State.treeFocused` 由侧栏树与正文滚动容器的 `HandleInput`
  （`InputPointerDown`）记，读它的是 `handleShortcuts`（构建之前）；点完窗格要等一帧再按快捷键。
  两个滚动容器都返回 false，不吞事件；`openTreeSearch` / `openFind` 会直接置位，因为那时没有指针事件。
- **WebView 里的点击拿不到**：`platform.WebViewPress` 只把焦点给 webview 节点、不调用它的 `HandleInput`，
  所以「最后一次按在哪个窗格」在 WebView 模式下永远是侧栏（用户从树里点开的文档）。因此 `searchShortcuts`
  在 WebView 模式下让 `Cmd+F` 一律搜正文（搜文件用 `Shift+Cmd+F`），这是写进 README 的约定，不要改成猜。
- **mygo 的快捷键要求修饰键完全相等**：`c.Shortcut(ui.Cmd, ui.KeyF)` 在按住 Shift 时**不会**匹配
  （`routeKey` 比的是 `mods == k.mods`），带 Shift 的组合必须单独注册一次（`ui.Cmd|ui.Shift`）。
  `Shift+Cmd+G`（上一个匹配，`find.go`）曾经因为写成 `Shortcut(ui.Cmd, ui.KeyG)` + 看 `Modifiers()` 而不生效，
  已改成 `Cmd|Shift` / `Cmd` 两次注册；以后加带 Shift 的组合照这个写。
- **占位文字（Placeholder）不能用 `tt.Texts()` 断言**：它是画出来的，不是文本节点；要断言就读无障碍信息：
  `tt.Access("<字段的无障碍名>").Placeholder`（两个搜索框的中文占位就是这么钉的）。
- **搜索框拿键盘要一直要到它真的拿到**：`ui.Tree` 本身是个焦点组，树搜索打开的那一帧会把键盘抢走（那时没有别的东西持焦点），
  单次 `AutoFocus` 会输掉这场竞争，字段就收不到敲字。用 `s.treeSearch.needFocus` 每帧 `Focus()`，`= !field.Focused()` 才停。
- **搜索框上的 `↑` / `↓` 用窗口级 `Shortcut` 接**：`Element.HandleInput` 挂在 `SearchField` 外层拿不到按键——焦点在它里面
  那个输入框节点上，`focusHandler` 只看焦点节点自己。所以在 `searchShortcuts` 里用 `c.Shortcut(0, ui.KeyUp/KeyDown)`；
  树行自己有 `Shortcut`，行上有焦点时它先拿到（那是树自己的焦点移动），这是有意的。
- **`View` 的侧栏列要 `FillHeight()`**：`View` 是测试用的并排近似布局，侧栏列不给高度时 `Grow(1)` 不生效，
  树的滚动容器被内容撑高，`s.treeScroll.MaxY` 恒为 0（定位与滚动类断言全假）。真实 shell 的 `Left` 槽位是有界的，
  这个漏洞只出现在测试路径上。
- **搜索面板是三块浮岛**（`findIsland`：输入框 / 匹配行 / 上下文），外层只给位置，**没有背景**，
  也**没有 `Gap`**：间距由「匹配列表岛的 6 DIP 上外边距」+「两条 `findGrip`（10 DIP）高的拖拽带
  （列表下、上下文下各一条）」构成，改间距要三处一起改，否则会出现「带子占位 + 间距」的双份留白。
  别再把它塞回一个带 Surface 的整块卡片里：那样列表最后一行和上下文第一行会读成一列。
  三块岛都给读屏名（`搜索输入` / `匹配行` / `命中上下文`，也是测试里 `tt.Find` 取岛矩形的唯一办法），
  两条带子也有（`调整匹配列表高度` / `调整命中上下文高度`）；两块数据岛的滚动容器另各有名字
  （`匹配行列表` / `命中上下文列表`）。上下文岛顶上那行 `第 N 行上下文` 是给人分清的。
  岛内宽度用 `findPaneInner`，别再用 `findPaneWidth - 2*Space(1)`；滚动内容的列（`findScrollPane`）
  自己带 padding，`MinWidth` 要写成 `findPaneInner+2*findIslandPad`（边框盒含 padding）。
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
  所以滚动容器上挂了 `HandleInput`：`InputScroll` 且没按 Shift 时 `tabScroll.X += ev.DY` 并返回 true。
  **符号别搞反**：mygo 的容器是 `scrollBy` 里的 `x + dx` / `y + dy`，也就是 **`DY` 为正 = 向下（向前）滚**；
  写成 `-=` 会「往上滚、标签往前走」（用户 2026-11-27 报的就是这个），`Shift`+滚轮那条路是外壳自己按
  `dx, dy = dy, 0` 转的，两边必须同向。
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
  所以这 16 是正文滚动容器的**透明上边框**（内容列的上内边距相应减去 16，静止时的样子不变；
  注意内边距在**内容列**上，不在滚动容器上，见上面那条）。要改这个数就改
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
- **多选靠 `ClickModifiers()` 读「这次点击」的修饰键**：它读的是点击那一刻记在行上的状态，所以在
  `OnClick` 回调里读（不要提前到构建期存进变量）；修饰键的掩码是 `ui.Ctrl|ui.Cmd`（macOS 上 Cmd 也进得来，
  Windows 的 Win 键顺带落进来，无害），**不要**写 `runtime.GOOS` 分支。
- **修饰键点击绝不能执行行自己的动作**：`pickRow` 返回 true 就 `return`。否则按住 Ctrl 选目录会把它折叠、
  选文档会开标签，用户刚选的那几行会在他眼前消失。
- **选中行在 `openTab` 的早退分支之前就要同步**（`syncTreeSelToDoc`）：点的是已经显示着的那份文档时
  `openTab` 会提前返回，不同步的话「点目录 → 点已打开的标签 → Ctrl+C」会复制到那个目录的路径。
  选择**不落盘**（`session.json` 里没有它）：它是一次会话语境，重启后为空，此时 `Cmd+C` 回落到当前文档路径。
- **测试点项目节点要用「非活动项目」的名字**（`twoProjects` 里是 `beta`）：项目面板的按钮显示的是**活动项目**
  的名字，同名标签有两个，`Tester` 按文本找元素会先碰上按钮（点成「切换项目」）。同理，单文件项目的名字在
  标签栏、标题行、面板按钮上都有一份——要右键它的树行/面板行，就用带前缀的 `文件：<名字>` 或先换活动项目。
- **本项目除正文的 WebView 渲染外全是原生 UI**：树、标签栏、标题行、项目面板都是 `ui` 画的，焦点在它们身上时
  `Cmd/Ctrl+C` 走原生快捷键（`routeKey` 落到窗口的 `Shortcut`），与当前是哪种渲染模式无关；只有焦点在正文页面里时
  `Cmd+C` 是页面自己的（复制选中文字）。别因为「WebView 模式下复制不出来」去改页面或快捷键。
- **`tt.Menu()` 对这个应用的右键菜单返回 nil**：`components.ContextMenu` 画的是窗口内菜单，Tester 的
  `Menu()` 只认平台菜单。断言菜单内容用 `tt.HasText(...)`（或 `tt.Texts()`），点菜单项照旧用 `tt.Click`。
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
