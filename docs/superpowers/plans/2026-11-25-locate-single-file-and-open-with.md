# 定位当前文档、独立文件、用此应用打开，以及大目录的性能 实现计划

> **面向 AI 代理的工作者：** 必需子技能：使用 subagent-driven-development（推荐）或 executing-plans 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** 一期做完两件事——**新功能**：（1）侧栏顶部「展开/折叠全部目录树」之后新增 **crosshair**，把当前标签的文档
在项目树里定位出来（展开祖先、滚进视野）；（2）项目树模糊搜索**只高亮当前候选**（树已是过滤视图，其余候选不再上色）；
（3）**独立文件**：底部工具栏新增「打开 Markdown 文件」，选中的 md 成为一个**单文件项目**（与目录项目同时在树里），
并且「用此应用打开 md」（命令行 / 文件关联 / 第二实例）也自动进树。**性能**：（4）项目树**异步扫描**（项目名后面挂
loading 图标）、**扫描上限 100 万条目，超过就丢弃并提示「目录过大无法打开，请缩小目录范围」**、
**监听上限同一常量**（超了停监听并提示，`刷新` 仍可用）、**目录/文件变动增量更新**（不再整棵重扫）、
（5）**树虚拟化**：`ui.Tree` → `ui.Outline`，每帧只构建可见行。

**架构：** 三块地基先落地，功能长在它们上面。

- **单文件项目**：`config.Project` 加 `File`（非空即单文件项目，`Root` 是它所在目录）。归属判断是纯函数
  `workspace.Locate`（已知项目里可见就复用，否则新建）。`workspace.Tree(p)` 是「项目 → 树」的唯一入口。
- **reload 管线**：`workspace.BuildTree` 移出窗口线程（goroutine + `Post` 回窗口线程），旧树保留、同一项目在扫时
  合并请求、关项目丢弃结果；扫描上限 `workspace.MaxEntries = 1_000_000`，超过返回 `ErrTreeTooLarge`，树不落地、
  只留一条提示并等下一次 `刷新`；`watch` 用同一常量，`mygo.ErrWatchLimit` 只提示一次、不再静默死掉。
  watcher 的 `ChangeSet` 直接**就地改内存树**（`workspace.InsertDoc` / `RemoveDoc` / `RenameDoc`），只有「不知道
  具体变了什么」才重扫。
- **树**：`ui.Tree` + `ui.TreeItem`（每帧构建全部节点）换成 `ui.Outline` + `OutlineState[treeKey]`（虚拟化，
  `Generation` 只在树变时重走）。行内容自绘（`ui.Row` + `OnClick` + `components.ContextMenu`），展开状态从
  `s.open map[string]*bool` 换成 `OutlineState.Open Selection[treeKey]`（按路径为键，重扫后自然保留），
  「定位 / 当前候选」都用 `List.ScrollIntoView(row)`，行号由 `rowOf(key)` 自己走一遍算出来（不碰 `List.Selected`）。

**技术栈：** Go 1.27+、`github.com/egoist/mygo v0.9.1`（`ui.Outline` / `ui.OutlineState[K]` / `Selection[K]` /
`ListState.ScrollIntoView` / `Visible`、`Element.OnClick`、`components.Spinner`、
`mygo.Dialog.Open`、`mygo.App.OnOpenFile` / `OnSecondInstance` / `RequestSingleInstanceLock`、
`mygo.WatchFiles` 的单文件模式与 `FileWatchOptions.MaxEntries`、`mygo.ErrWatchLimit`）、
`github.com/oneo-me/onevra/source v0.2026.18`（`components.IconButton` + `IconButtonPlain`、`components.Spinner` +
`SpinnerSpokes`、`lucide.Crosshair` / `FilePlus` / `FolderOpen` / `FileText` / `UnfoldVertical` / `FoldVertical`）。
**不引入新依赖**（含 `znkr.io/diff`，见 D20）。

**规格：** `docs/superpowers/specs/2026-11-21-markdown-preview-design.md`。本计划**新增**独立文件、定位与性能改造，
并**推翻**该规格的两条：2026-11-24 那节的「候选 / 当前候选两种底色」（改成只高亮当前候选）、
以及「树的 md 项超过约 3000 个时会变慢（`ui.Tree`）」这条已知限制（换成 `ui.Outline` 之后不再成立）。
执行后由任务 14 追加「### 定位、独立文件、大目录与虚拟化的树（新增，2026-11-25）」，并在
`docs/superpowers/plans/2026-11-24-tree-fold-and-file-search.md` 末尾的「执行后的修订」里记两笔（底色、树容器）。

**本期不做：** 把文件关联**注册**进系统（`mygo.json` 的 `fileAssociations` 已经写了 `.md` / `.markdown`，安装包会注册；
未打包的 exe 走系统「打开方式」）；命令行给**目录**时自动加项目；把独立文件并进某个目录项目；
多窗口；树的拖拽排序；**目录（TOC）那一棵 `ui.Tree`**（`native.go` 的 `toc` 仍是 `ui.Tree` + `s.openOf`，
标题数通常只有几十个，本计划不动它——见 D12 的坑）；`watch` 的 `HintsOnly` 降级（超上限就停监听，见 D6）。

## 实测数据（为什么必须动 reload 管线与树）

3600 个 md、嵌套 3~4 层，本机（Ryzen 9 7945HX，headless `ui.NewTester`）：

| 测的东西 | 结果 |
| --- | --- |
| `workspace.BuildTree` 扫一遍 | **81 ms**（跑在窗口线程上，打开 / 刷新 / 每次监听 TreeDirty 都跑） |
| 一帧，树里 3730 个节点 | **14.5 ms** |
| 一帧，树里 6 个节点 | **0.07 ms** |

结论：**卡死**来自同步扫盘（+ 连续写文件时一批接一批地重扫）；**持续发卡**来自 `ui.Tree` 每帧构建全部节点
（≈4 µs/节点，线性）。异步化只治前者，虚拟化才治后者，两个都要做。

## 全局约束

- 不引入第三方 markdown / GFM / 高亮 / web 框架依赖；**本计划不新增任何依赖**。
- **`internal/workspace`、`internal/launch` 不得 import `.../ui`**（前者只允许 `.../ui/markdown/parser`）：
  扫描上限、归属判断、树的增删改都必须是能无窗口测的纯函数。
- 视图状态归窗口线程：**唯一的读盘入口是扫描**（`workspace.Tree` / `workspace.ListDocs`），它只跑在扫描 goroutine 上；
  窗口线程只 `Post` 结果。构建期不读盘、不扫目录。
- 扫描 goroutine 的每一条回程都必须带**项目 ID + 代数**：项目被关掉 / 移除 / 又发起新扫描时，旧结果一律丢弃
  （`t.TempDir()` 在测试结束时被删，迟到的结果不许 panic）。
- 视图是 state 的纯函数：`View`（测试）与 shell 的 `Left` / `LeftHeader` / `LeftToolbar` / `Content` 路径一起改，
  `TestShellSlotsBuild` 不许回归。
- 可交互元素必须给 `label`（无障碍名，测试与读屏唯一的定位方式），新增 label 不得与现有重名：
  本计划新增 `在项目树中定位当前文档`、`打开 Markdown 文件`、`正在扫描 <项目名>`；
  树的行沿用文件名与项目名，项目行内按钮的 label 不动。
- 颜色只从 `AppTheme` 取，不写十六进制；本计划删一个颜色函数（`candidateRowTint`），不新增。
- 测试只用 `t.TempDir()`；不写平台专用代码，三平台可编译。
- **测试一律同步扫描**：`newState` / `newStateWith` / `twoProjects` 三个 helper 调 `SetSyncScan()`，其余测试不用管；
  只有讲异步的那个任务用可替换的扫描函数 + `waitFor`。
- 每个行为改动先有失败的测试；文档回写（README / AGENTS / CHANGELOG / 规格 / 上一份计划）**并入同一个提交**。
- 提交前 `go vet ./... && gofmt -l . && go test ./...` 必须干净；中文提交信息，前缀 `feat:` / `fix:` / `test:` / `docs:`。

## 设计决策（实施者按此执行，不要另立方案）

**数据模型**

| # | 决策 | 理由 |
| --- | --- | --- |
| D1 | **单文件项目** = `config.Project{File: <绝对路径>}`：`Root` 是文件所在目录、`Name` 是文件名，`File` 非空即单文件项目。`ProjectID(File)` 仍是哈希，与 `ProjectID(Root)` 不同，所以同一目录的目录项目和单文件项目**可以并存**。`config.Version` 不升（加字段、向后兼容，旧文件缺这个字段就是 `""`）。 | 复用 `docSel{项目ID, 相对路径}`、标签、监听、MCP、`mdview://` 白名单；不引入第二套身份。 |
| D2 | 用户确认：**项目树同时存在目录项目 + 独立文件**。多项目模式下项目节点对单文件项目**不存在**：那一条直接是文件行（否则会出现 `a.md › a.md`）。单文件项目不参与项目节点的展开/折叠。 | 用户原话「项目树能否同时存在项目+独立文件」——就是要并存。 |
| D3 | **打开一个文件时的归属**（纯函数 `workspace.Locate`）：① 它已是一个单文件项目 → 用它；② 它在某个已知项目里、`workspace.Ignored` 没挡住、是 md → 用那个项目（多个根都包含它时**取最深的根**）；③ 否则新建单文件项目。被忽略规则挡住的文件（`node_modules/x.md`、`.hidden/x.md`）走 ③，自己成项目后就看得见了。归属命中的项目**没打开**就顺手打开它（不另建条目）。 | 「能看见」是唯一标准：打开的文件必须在树里，否则定位按钮与标题行都会说谎。 |
| D4 | 标题行对单文件项目显示**文件绝对路径**（`shortPath`），路径过长仍由已有的 `TruncateMiddle` 省略中段。 | `项目名/相对路径` 对单文件项目是 `a.md/a.md`，没意义。 |
| D5 | MCP：`Workspace.ListDocs(projectID)` 对单文件项目只返回**一条**（`path` 是文件名，标题走同一条标题缓存）；`list_projects` 的 `root` 仍是文件所在目录，契约不改。 | 「项目里的文档」就是那一个；扫整目录会让 `search_docs` 越界。 |

**reload 管线**

| # | 决策 | 理由 |
| --- | --- | --- |
| D6 | **扫描上限 `workspace.MaxEntries = 1_000_000`**，扫描与监听共用这一个常量。扫描时按「访问到的条目数」（文件 + 目录，被忽略的子树不算）计数，超过就返回 `ErrTreeTooLarge`：**树不落地**、项目节点不出现、一条 `notifyWarn("目录过大无法打开，请缩小目录范围：" + root)`，并记 `s.tooLarge[id] = true`，**只有 `刷新` 才会重扫**。监听传 `MaxEntries: workspace.MaxEntries`；仍撞 `mygo.ErrWatchLimit`（晚到的错误也算）就停监听 + 一条 `notifyWarn("目录过大，已停止监听：<项目名>（刷新仍可用）")`，不降级 `HintsOnly`。 | 用户定：100 万，超了就丢弃并提示。不降级 `HintsOnly` 是为了保持「监听要么准、要么没有」这一条简单规则——粗粒度 hint 只会变成「不停重扫 100 万条目」。 |
| D7 | **异步扫描**：`reloadTree()` 不再扫盘，只负责「对齐要扫的项目」（开着的、没有在扫的、没有 `tooLarge` 的）→ 每个起一个 `scan(p)` goroutine → `s.Post` 回窗口线程 `applyTree(id, gen, tree, err)`。同一项目在扫时再来的请求记为 `scanAgain[id] = true`，扫完再来一次（**合并，不排队**）。`Refresh()` 清 `tooLarge` 与 `scanAgain` 并重扫。 | 81 ms 的扫盘不能留在窗口线程；一批文件变动会连续触发 TreeDirty，排队会越扫越落后。 |
| D8 | **旧树保留**：扫描期间 `s.trees` 保持上一次的结果，项目名后面挂 `components.Spinner(c, components.SpinnerSpokes)`（`Label("正在扫描 " + 项目名)`，约 `t.Space(1.5)`）；一个节点都还没有时，树区域显示 `正在扫描 <项目名>…`。 | 清空再填会让树闪一下、还会丢滚动位置；用户要的就是「项目后面一个 loading 图标」。 |
| D9 | 扫描结果与项目对齐：`applyTree` 按项目 ID **upsert** 到 `s.trees`，再按 `cfg.OpenProjects()` 的顺序**稳定排序**（结果回来的顺序不可控）。`ErrProjectGone` → 走现有的「项目目录不可用」路径（单文件项目文案是 `文件不可用：<File>`）；`ErrTreeTooLarge` → D6；其他错误 → 提示后保留旧树。 | 现在 `s.trees` 的顺序就是「项目添加顺序」，很多测试与多项目模式依赖它。 |
| D10 | `state.syncScan`：测试用的同步模式（`SetSyncScan()` 置位并立刻 `Refresh()`），`reloadTree` 在同步模式下就地扫描。**三个测试 helper 里各加一行**，其余测试照旧。 | 否则 40 多个测试要等 goroutine，全会变成 flaky。异步本身由专门的一个任务用可替换的 `scan` + `waitFor` 测。 |
| D11 | **增量更新**：`applyChanges` 对 `set.Docs` 里的每一条就地改内存树（`workspace.InsertDoc` / `RemoveDoc` / `RenameDoc`，纯函数），然后 `bumpTree()`（`Generation++` + 搜索候选重算）；只有 `set.TreeDirty && len(set.Docs) == 0` 才转成一次重扫（目录增删、hint、溢出）。 | 每次写入都重扫整棵树是最大的一笔浪费；watcher 已经告诉我们「变的是谁、怎么变的」。 |
| D12 | **不引入 `znkr.io/diff`**。 | 它是文本（行级 LCS）diff：我们要比的是目录树，而「变了什么」watcher 已经逐条给出（D11 用它就够了）；真正贵的扫盘与 markdown 重排版，diff 一分钱也省不下来。若将来需要「重扫后判断树到底变没变」，那是两个有序路径列表的归并比较，O(n) 十几行，不值得一个依赖。 |

**树（虚拟化）**

| # | 决策 | 理由 |
| --- | --- | --- |
| D13 | `ui.Tree` + `ui.TreeItem` → **`ui.Outline` + `ui.OutlineState[treeKey]`**，`treeKey{project, path}`（可比较；项目节点与单文件项目的根都是 `{project, ""}`，目录/文件是 `{project, 相对路径}`）。`roots()` 与 `children(key)` 都是 `State` 上的方法（`children` 从 `s.trees` 里按 `treeKey` 查节点，命中不了返回 `nil`），`Generation` 每次树变化 `+1`。 | 每帧 14.5 ms 的根因是「所有节点都建」；`Outline` 只建可见行，`Generation` 让它在树没变时连「走一遍开着的节点」都省掉。 |
| D14 | 展开状态：`s.treeOpen ui.OutlineState[treeKey].Open Selection[treeKey]`，**不再用树专用的 `s.open`**。新出现的目录**默认展开**：`applyTree` 里遍历新树，凡 `!s.treeSeen[key]` 的目录 `Open.Add` 并记进 `treeSeen`（用户折叠过的目录在重扫后保持折叠，因为 key 还在 `Open` 之外、`treeSeen` 里）。`setTreeOpen` 改成往 `Open` 里增删（`project:` 前缀的键换成 `{project, ""}`）。**`s.open` / `s.openOf` 保留**：目录（TOC）那一棵 `ui.Tree` 还在用它（`native.go` 的 `tocKey`），改的时候别顺手删。 | `Open` 以路径为键，重扫后天然跟着同一条路径；`s.open` 只剩 TOC 一个用户。 |
| D15 | 行内容自绘：`ui.Row(c)....OnClick(...)`（`Element.OnClick` 会把它变成可点击）+ `components.ContextMenu(c, row, …)`；缩进与箭头交给 `Outline`（`Indent` 用默认 `Space(4)`），行的选中底色仍是 `selectedRowTint`，当前候选是 `currentCandidateTint`，其余不上色。`ui.Scroll` + `TrackScroll(&s.treeScroll)` 去掉（`Outline` 自带滚动），`s.treeScroll` 删除，`treePress` 挂到 `Outline` 元素上（`Label("项目树")` 保留，测试靠它量位置）。 | `ui.TreeItem` 的全部能力（`Selected` / `Children` / `ScrollIntoView` / 可点击）在 `Outline` 里都要自己拼；这样一来自绘的行也能带项目行的两个行内小按钮。 |
| D16 | **行号**：`rowOf(roots, key) int` 自己按 `roots` + `children` + `Open.Has` 走一遍算出目标行号（-1 找不到），不依赖 `OutlineState.Rows()`（那是**上一帧**的快照）。定位与「当前候选」都用它，然后 `s.treeOpen.List.ScrollIntoView(row)`。 | 一次点击会建两遍视图，展开与滚动必须在**同一帧**里生效；`Rows()` 慢一帧就会滚到邻居。 |
| D17 | 搜索时的过滤视图就是**另一组 roots/children**：`roots()` 在 `searching()` 时只给「有候选的项目」，`children()` 只给「有候选的目录 + 候选文件」，且一律视为展开（`Open` 也照样满足要求：搜索期间 `treeOpen.Open` 不动，`children` 返回 `[]treeKey` 之后由 `Outline` 的 `Open.Has` 决定展开——所以搜索期间要在 `setTreeQuery` / `refreshCandidates` 里把这些键临时 `Open.Add`，关闭搜索时移除并恢复用户原来的展开）。`bumpTree()` 在查询变化时必须调用。 | `Outline` 没有「构建期强制展开」的入口；用 `Open` 临时展开、关闭时复原，与今天的 `matchedItems`（构建期临时指针）等价，但状态是可复原的。 |
| D18 | `s.treeFocused`、`Cmd+F` 分派、`.Label("项目树")` 的行为不变。 | 与搜索面板的约定（WebView 模式下 `Cmd+F` 一律搜正文）不重做。 |

**功能与接线**

| # | 决策 | 理由 |
| --- | --- | --- |
| D19 | 底部工具栏（方案 A）：`打开项目`（`FolderOpen`，目录、可多选，行为不变）→ 新增 `打开 Markdown 文件`（`FilePlus`，文件、`Filters: [{Name:"Markdown", Extensions:["md","markdown"]}]`、可多选）→ `刷新` → 弹簧 → 铃铛 → MCP。对话框仍直接调 `mygo.Dialog.Open`（与现有 `OpenProject` 一致，不扩 `view.System`）。 | mygo 的 `Dialog.Open` 是二选一（`Directory: true/false`），一次对话框选不了「目录或文件」；用户确认要两个图标。 |
| D20 | **crosshair**：侧栏顶部那一行（`treeBulkActions`）里、`折叠全部目录树` **之后**，`lucide.Crosshair()`，label `在项目树中定位当前文档`，`Disabled(s.sel.empty())`（用户确认放这一行）。点击：先 `closeTreeSearch()`（查询留着）→ 展开祖先目录（`Open.Add`）→ `rowOf` 算出目标行 → `List.ScrollIntoView(row)`。定位不到（被忽略规则挡住、刚被删）→ `notifyWarn("项目树里没有这个文件：" + 相对路径)`。 | 「在全部展开/收缩之后」只有这一行满足。 |
| D21 | 监听：`State.startWatch(p workspace.Project)`（不再收 `id, root`）——单文件项目用新的 `watch.WatchFile(ctx, p.File)`（mygo 会盯着它所在目录、只报这个文件，事件路径是 `"."`），目录项目用 `watch.Watch(ctx, p.Root)`（带 `MaxEntries`）。`watch.FoldFile(events, name)` 把 `"."` 翻成文件名。 | 盯着一个文件的整个目录（可能是 `~`）是灾难；mygo 的单文件模式正好只报它自己。 |
| D22 | 命令行：新的纯函数包 `internal/launch`，`Documents(args []string, wd string) []string`（md/markdown、大小写不敏感、存在且是普通文件、相对路径按 `wd` 补全、认 `file://` URL）。`main.go` 三处喂给 `State.OpenDocument`：`WhenReady` 里的 `os.Args[1:]`、`OnSecondInstance(args, wd)`、`OnOpenFile(path)`。`OpenDocument` **幂等**（同一个文件重复投递只选一次、只提示一次），末尾 `if s.win != nil { s.win.Invalidate() }`（它会在帧外被调用）。 | `packageFileExtensions` 只有 `mygo build` / `mygo dev` 会写进二进制，`go run . x.md` 与未打包的 exe **不会**触发 `OnOpenFile`；macOS 的「用此应用打开」只走 Apple Event（`OnOpenFile`）。两边都接，重复投递是幂等无害的。 |
| D23 | `OpenDocument` 开头 `filepath.EvalSymlinks`（失败退回 `filepath.Abs`）再比较身份；进了树之后**展开祖先**（新项目默认全展开本来就是 D14，已有项目里则 `revealDocInTree`）。 | Windows 上大小写/软链差异会变成两个条目。 |
| D24 | 只有一层搜索底色：删 `candidateRowTint` 与 `rowCandidate`/`treeRowFill` 枚举，换成 `isCurrentCandidate(projectID, path) bool`（用户截图 2）。 | 树已经是过滤视图，再给每个候选上色等于「整片都亮」。 |
| D25 | 不落盘的新状态：`treeOpen` / `treeSeen` / `tooLarge` / `scanAgain` / `searching` 的展开复原都只在本次运行内有效（与 `find`、`zoom`、`treeSearch` 一致）。`config.json` 只多一个 `file` 字段。 | 与现有约定一致。 |

## 审查重点（Review Focus）

1. **扫描迟到 / 重复 / 项目已被关掉**：结果不许落到已经关掉或已重扫的项目上，更不许 panic；关项目要能让下一次结果整个丢掉
   （`TestALateScanDoesNotResurrectAClosedProject`、`TestScanningAgainKeepsOneResult`，任务 5）。
2. **超大目录**：超过 100 万条目（以及监听撞上限）时**不显示半棵树**、不反复重扫、`刷新` 还救得回来，提示是中文且可读
   （`TestATooLargeDirectoryIsRefusedWithANotice`、`TestAWatchLimitStopsTheWatchAndSaysSo`，任务 5）。
3. **增量更新不许把树改坏**：新增 / 删除 / 改名一个文档之后，树里的节点、顺序、父目录、`Open` 状态、搜索候选都与
   「整棵重扫」的结果一致（`TestPatchingTheTreeMatchesARescan`，任务 6）。
4. **虚拟化保行为**：一棵有几千个节点的树，点击 / 展开 / 右键 / 搜索过滤 / 定位 / 折叠全部 都还按原样工作，
   并且每帧只建可见行（`TestTheTreeOnlyBuildsWhatShows`、`TestUnfoldingFoldingAndOpeningTheWholeTree`，任务 7）。
5. **「用此应用打开」落在已隐藏到托盘的实例上**：文件立刻成为标签并显示，而不是只把窗口叫到前面
   （`TestOpeningAFileShowsIt`、`TestOpeningTheSameFileTwiceIsOneEntry`，任务 9；接线在任务 13 手动验证）。

## 文件结构

- 修改 `internal/config/config.go`：`Project.File`、`(*Config).AddFileProject`。
- 新建 `internal/workspace/single.go`：`Tree(p)`、`Locate(projects, abs)`；改 `doc.go`（`ListDocs` 认单文件项目）。
- 修改 `internal/workspace/scan.go`：`MaxEntries`、`ErrTreeTooLarge`（扫描计数）。
- 新建 `internal/workspace/patch.go`：`InsertDoc` / `RemoveDoc` / `RenameDoc`（纯函数，就地改 `*Node`）。
- 新建 `internal/launch/launch.go`：`Documents(args, wd)`。
- 修改 `internal/watch/watch.go`：`FoldFile`、`WatchFile`、`Watch` 带 `MaxEntries`。
- 新建 `internal/view/scan.go`：`scan` 字段、`reloadTree`/`applyTree`/`bumpTree`、`tooLarge`/`scanAgain`、`SetSyncScan`。
- 新建 `internal/view/tree.go`：`treeKey`、`roots`、`children`、`rowOf`、`treeRows`（行内容）、`setTreeOpen`（迁过来）。
- 修改 `internal/view/state.go`（`OpenDocument`、`afterProjectsChanged`、`shortPath`、`treeOpen`/`treeSeen` 字段、`reloadTree` 的入口）、
  `internal/view/watch.go`（`startWatch(p)`）、`internal/view/view.go`（`leftToolbar`、树容器换 `Outline`、`treeBulkActions`）、
  `internal/view/treesearch.go`（过滤视图的 roots/children、`isCurrentCandidate`）、`internal/view/theme.go`（删 `candidateRowTint`）。
- 新建 `internal/view/locate.go`：`locateDoc`、`revealDocInTree`。
- 修改 `main.go`：`windowState`、启动参数、`OnSecondInstance`、`OnOpenFile`。
- 测试：`internal/config/config_test.go`、`internal/workspace/single_test.go`（新）、`scan_test.go`、`patch_test.go`（新）、
  `internal/launch/launch_test.go`（新）、`internal/watch/watch_test.go`、`internal/view/scan_test.go`（新）、
  `internal/view/tree_test.go`（新）、`internal/view/singlefile_test.go`（新）、`internal/view/locate_test.go`（新）、
  `internal/view/treesearch_test.go`、`internal/view/view_test.go`、`internal/view/shell_test.go`、`internal/view/bench_test.go`。

---

# 阶段 A：地基（数据、扫描、监听、增量）

### 任务 1：配置里的单文件项目

**文件：** 修改 `internal/config/config.go`；测试 `internal/config/config_test.go`

- [ ] **步骤 1：编写失败的测试**

```go
// TestAFileProjectIsOneDocument pins a single-document project: its file, the
// directory it sits in, its name, and that adding it twice keeps one entry.
func TestAFileProjectIsOneDocument(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "notes.md")
	if err := os.WriteFile(file, []byte("# N\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var c Config
	p, isNew := c.AddFileProject(file)
	switch {
	case !isNew:
		t.Fatal("isNew = false, want the file added")
	case p.File != file:
		t.Errorf("File = %q, want %q", p.File, file)
	case p.Root != dir:
		t.Errorf("Root = %q, want %q", p.Root, dir)
	case p.Name != "notes.md":
		t.Errorf("Name = %q, want notes.md", p.Name)
	case !c.IsOpen(p.ID):
		t.Error("the file project is not open, want it open")
	case p.ID == ProjectID(dir):
		t.Error("the file project's id is the directory project's, want them apart")
	}
	if again, isNew := c.AddFileProject(file); isNew || again.ID != p.ID {
		t.Errorf("AddFileProject twice = %q, %v, want the same one", again.ID, isNew)
	}
	// The file survives a save and a load.
	if err := c.Save(t.TempDir()); err != nil {
		t.Fatal(err)
	}
}
```

- [ ] **步骤 2：运行测试验证失败** — `go test ./internal/config/ -run TestAFileProjectIsOneDocument -v`，预期 FAIL：`c.AddFileProject undefined`

- [ ] **步骤 3：实现** — `Project` 加 `File string \`json:"file,omitempty"\``（注释写清「非空即单文件项目，`Root` 是它的目录」）；
  `AddFileProject(path string) (Project, bool)`：`id := ProjectID(path)`，已存在就返回，否则 `Root = filepath.Dir(path)`、
  `Name = filepath.Base(path)`，追加到 `Projects` / `OpenIDs`，`ActiveID` 为空时置上。`normalize` 不动。

- [ ] **步骤 4：运行测试验证通过** — `go test ./internal/config/ -v`，预期 PASS

- [ ] **步骤 5：Commit** — `git add internal/config/ && git commit -m "feat: 配置支持单文件项目（Project.File）"`

---

### 任务 2：workspace 认单文件项目，并判定文件归属

**文件：** 创建 `internal/workspace/single.go`、`single_test.go`；修改 `internal/workspace/doc.go`（`ListDocs` 开头分叉）

- [ ] **步骤 1：编写失败的测试**

```go
func TestTheTreeOfAFileProjectIsOneNode(t *testing.T)       // Tree(p) → 一个非目录节点，Path 是文件名
func TestTheTreeOfAFileProjectWhoseFileIsGone(t *testing.T) // 删掉文件后 Tree(p) 返回 ErrProjectGone
func TestListDocsOfAFileProject(t *testing.T)               // ListDocs(id) 只一条，Path == "notes.md"，Title 取一级标题
func TestLocateFindsTheKnownProjectHoldingAFile(t *testing.T) // 目录项目 + 它的子目录项目都包含它 → 取更深的那个
func TestLocateFindsAFileProjectItself(t *testing.T)         // 已存在的单文件项目 → 它自己，rel == 文件名
func TestLocateSkipsWhatTheTreeWouldNotShow(t *testing.T)    // node_modules/x.md、非 md、目录 → ok == false
```

- [ ] **步骤 2：运行测试验证失败** — `go test ./internal/workspace/ -run 'TestTheTreeOfAFileProject|TestListDocsOfAFileProject|TestLocate' -v`，预期 FAIL：`Tree undefined`、`Locate undefined`

- [ ] **步骤 3：实现**

`single.go`：
- `func Tree(p Project) (*Node, error)`：`p.File == ""` → `BuildTree(p.Root)`；否则 `os.Stat(p.File)`，不存在或不是普通文件 →
  `ErrProjectGone`，否则 `&Node{Name: filepath.Base(p.File), Path: filepath.Base(p.File)}`。
- `func Locate(projects []Project, abs string) (id, rel string, ok bool)`：单文件项目的 `File` 与 `abs` 相等即命中（`rel` = 文件名）；
  目录项目用已有的 `within(root, abs)`，`rel = filepath.ToSlash(filepath.Rel(root, abs))`，要求 `IsMarkdown(abs)` 且
  `!Ignored(rel, false)`；多个命中取 `len(root)` 最长的那个。

`doc.go` 的 `ListDocs(projectID)`：取到项目后 `p.File != ""` 就只造一条 `Doc{ProjectID, Path: base, ModTime, Size}`，
标题走 `w.titleOf(projectID, p.Root, &doc)`。

- [ ] **步骤 4：运行测试验证通过** — `go test ./internal/workspace/ -v`，预期 PASS

- [ ] **步骤 5：Commit** — `git commit -m "feat: workspace 支持单文件项目，并按项目判定文件归属"`

---

### 任务 3：扫描上限（100 万条目，超了就丢弃）

**文件：** 修改 `internal/workspace/scan.go`；测试 `internal/workspace/scan_test.go`

- [ ] **步骤 1：编写失败的测试**

```go
// TestTooManyEntriesStopsTheScan pins the ceiling: a directory with more entries
// than the limit is refused rather than read whole.
func TestTooManyEntriesStopsTheScan(t *testing.T) {
	root := t.TempDir()
	for i := range 10 {
		writeFile(t, root, "d"+strconv.Itoa(i%3)+"/f"+strconv.Itoa(i)+".md", "# t\n")
	}
	_, err := listDocs(root, 5)
	if !errors.Is(err, ErrTreeTooLarge) {
		t.Fatalf("err = %v, want ErrTreeTooLarge", err)
	}
	if _, err := listDocs(root, 1000); err != nil {
		t.Fatalf("err = %v, want the tree read under the limit", err)
	}
}

func TestTheEntryLimitIsAMillion(t *testing.T) // MaxEntries == 1_000_000
```

（`listDocs(root, max)` 是 `ListDocs` 的内部分支，测试直接调它。）

- [ ] **步骤 2：运行测试验证失败** — `go test ./internal/workspace/ -run 'TestTooManyEntries|TestTheEntryLimitIsAMillion' -v`，预期 FAIL：`undefined: listDocs`、`MaxEntries`

- [ ] **步骤 3：实现** — `scan.go` 加 `const MaxEntries = 1_000_000` 与 `var ErrTreeTooLarge = errors.New("workspace: project has too many entries")`；
  `ListDocs(root) = listDocs(root, MaxEntries)`；`listDocs` 在 `WalkDir` 的回调里对**每个访问到的条目**（被忽略的子树除外）计数，
  超过 `max` 就 `return ErrTreeTooLarge`。`BuildTree` 与 MCP 因此共用同一条上限。

- [ ] **步骤 4：运行测试验证通过** — `go test ./internal/workspace/ -v`，预期 PASS

- [ ] **步骤 5：Commit** — `git commit -m "feat: 扫描上限 100 万条目，超过返回 ErrTreeTooLarge"`

---

### 任务 4：命令行里的 md 文件、只监听一个文件、监听上限

**文件：** 创建 `internal/launch/launch.go`、`launch_test.go`；修改 `internal/watch/watch.go`、`watch_test.go`

- [ ] **步骤 1：编写失败的测试**

```go
// internal/launch
func TestDocumentsPicksMarkdownArguments(t *testing.T)                      // a.md、B.MARKDOWN 收下；c.txt 丢掉
func TestDocumentsJoinsRelativeArgumentsToTheWorkingDirectory(t *testing.T) // "notes/a.md" + wd → 绝对路径
func TestDocumentsTakesFileURLs(t *testing.T)                               // file:///…/a.md → 本地路径
func TestDocumentsIgnoresWhatIsNotAFile(t *testing.T)                       // 不存在、目录、无扩展名 → 丢掉，不 panic

// internal/watch
// TestFoldFileTurnsTheWatchedFileIntoADocument pins mygo's single-file watch:
// its events name the file itself as ".", which the window must read as the
// document it opened.
func TestFoldFileTurnsTheWatchedFileIntoADocument(t *testing.T) // "." → 文件名；"." 带 IsDir → TreeDirty
func TestTheWatchKeepsAMillionEntries(t *testing.T)             // watch.options(…) 里 MaxEntries == workspace.MaxEntries
```

- [ ] **步骤 2：运行测试验证失败** — `go test ./internal/launch/ ./internal/watch/ -v`，预期 FAIL：`Documents undefined`、`FoldFile undefined`

- [ ] **步骤 3：实现**

`launch.Documents(args []string, wd string) []string`：`file://` 换成本地路径 → 相对路径 `filepath.Join(wd, p)` →
`workspace.IsMarkdown(p)` 且 `os.Stat` 是普通文件才收（顺序 = 参数顺序）。

`watch`：
- `FoldFile(events []mygo.FileEvent, name string) ChangeSet`：`Path == "."` 且非目录 → `Change{Op, Path: name}`；目录事件 → `TreeDirty`；其余交 `Fold`。
- `WatchFile(ctx, path string) (Source, error)`：`mygo.WatchFiles(ctx, path, mygo.FileWatchOptions{})`（**不传 `Ignore`**：用户点名的文件就要盯）。
- `Watch`：`mygo.FileWatchOptions{Recursive: true, Ignore: workspace.Ignored, MaxEntries: workspace.MaxEntries}`；
  抽一个 `func watchOptions(root string, single bool) mygo.FileWatchOptions` 以便测试断言 `MaxEntries`。
- `watcher` 加 `name string` 字段，`Next` 按 `name != ""` 走 `FoldFile`。

- [ ] **步骤 4：运行测试验证通过** — `go test ./internal/launch/ ./internal/watch/ -v`，预期 PASS

- [ ] **步骤 5：Commit** — `git commit -m "feat: 命令行 md 文件、单文件监听与监听上限"`

---

### 任务 5：异步扫描、loading 图标、上限提示

**文件：** 创建 `internal/view/scan.go`、`internal/view/scan_test.go`；修改 `internal/view/state.go`（字段、`reloadTree` 入口、
三个测试 helper 之外的调用点）、`internal/view/watch.go`（`startWatch(p)` + 提示文案）、`internal/view/view.go`（spinner 挂点、
空态文案）、`internal/view/view_test.go` / `shell_test.go` / `treesearch_test.go` 里的 helper

- [ ] **步骤 1：编写失败的测试**

```go
// newState etc. gain s.SetSyncScan(); the async ones below replace the scanner.
func TestTheTreeLoadsOnAGoroutine(t *testing.T)          // 起扫描后 s.scanning[id] 为真、旧树还在、s.trees 未变；释放后结果落到 s.trees
func TestTheProjectShowsASpinnerWhileScanning(t *testing.T) // tt.Access("正在扫描 alpha") 在扫时存在、扫完就没了
func TestALateScanDoesNotResurrectAClosedProject(t *testing.T) // 扫描在飞时 ToggleProject(id,false) → 结果丢弃，树里没有它
func TestScanningAgainKeepsOneResult(t *testing.T)        // 扫描中再请求两次 → scanAgain 为真，最多再扫一次，最终只有一个结果
func TestATooLargeDirectoryIsRefusedWithANotice(t *testing.T) // 假扫描返回 ErrTreeTooLarge → 树里没有这个项目、一条「目录过大无法打开，请缩小目录范围」、s.tooLarge[id] 为真
func TestRefreshingATooLargeDirectoryTriesAgain(t *testing.T) // Refresh 清 tooLarge 并重扫
func TestAWatchLimitStopsTheWatchAndSaysSo(t *testing.T)  // watchFailure(p, mygo.ErrWatchLimit) → 「目录过大，已停止监听：…（刷新仍可用）」
```

`waitFor(t, func() bool)` 帮助函数（2 秒超时、每次 `s.drain()`）；假扫描用可替换字段：

```go
// scan reads a project's tree. The app reads it on a goroutine; a test that
// wants to hold one mid-flight replaces it.
scan func(p workspace.Project) (*workspace.Node, error)
```

- [ ] **步骤 2：运行测试验证失败** — `go test ./internal/view/ -run 'TestTheTreeLoadsOnAGoroutine|TestTheProjectShowsASpinner|TestALateScan|TestScanningAgain|TestATooLarge|TestAWatchLimit' -v`，预期 FAIL：`s.scan undefined`

- [ ] **步骤 3：实现**

`scan.go`：
```go
// scanGen is the counter the results are stamped with: a result whose stamp is
// not the current one is dropped.
type scanGen map[string]int
```
- 字段：`scan func(workspace.Project) (*workspace.Node, error)`、`syncScan bool`、`scanning map[string]bool`、`scanAgain map[string]bool`、
  `scanGen map[string]int`、`tooLarge map[string]bool`、`watched map[string]bool`（监听失败过的项目）。
- `SetSyncScan()`：`s.syncScan = true; s.Refresh()`（供测试 helper 用）。
- `reloadTree()`：只做「对齐」——按 `cfg.OpenProjects()` 的顺序，对每个「没有在扫、没有 `tooLarge`、树里没有或请求了重扫」的项目启动扫描；
  同步模式就地 `applyTree`。**扫盘、报「项目目录不可用」、丢弃死标签这些都不在这里了**（跟着 `applyTree` 走）。
- `startScan(p workspace.Project)`：`gen := s.scanGen[p.ID]+1`；`s.scanning[p.ID] = true`；`go func(){ tree, err := s.scan(p); s.Post(func(){ s.applyTree(p, gen, tree, err) }) }()`。
- `applyTree(p, gen, tree, err)`：`if s.scanGen[p.ID] != gen || !s.cfg.IsOpen(p.ID) { return }`（迟到的、被关掉的丢弃）；
  然后分派：`nil` → upsert + 稳定排序 + 默认展开新目录 + `bumpTree()` + 重算候选 + 丢掉已不存在的标签（原来 `reloadTree` 的尾巴搬到这里）；
  `ErrProjectGone` → 现有的移除项目路径（单文件项目文案 `文件不可用：<File>`）；`ErrTreeTooLarge` → `s.tooLarge[p.ID] = true` + 提示 + 从 `s.trees` 里去掉它；
  其他错误 → 提示 + 保留旧树。关掉的项目顺手清掉 `treeSeen` 里它那一堆键（否则改来改去会无限长）。最后 `if s.scanAgain[p.ID] { delete(...); s.startScan(p) }`。
- `bumpTree()`：`s.treeOpen.Generation++`，`if s.treeSearch.open { s.refreshCandidates() }`。
- `watchFailure(p workspace.Project, err error) string`：纯函数，`errors.Is(err, mygo.ErrWatchLimit)` → `"目录过大，已停止监听：" + p.Name + "（刷新仍可用）"`，
  否则 `"无法监听这个项目：" + err.Error()`；`startWatch` 用它 + `notifyErr`，并记 `s.watched[id] = false`（避免每帧重试）。

`view.go`：项目行的 `Children` 里，`if s.scanning[tr.project.ID] { components.Spinner(c, components.SpinnerSpokes).Label("正在扫描 " + tr.project.Name) }`；
树区域没有节点且在扫时显示 `正在扫描 <项目名>…`；`fileTree` 的空态文案改成 `打开一个目录或 Markdown 文件`。

三个测试 helper（`newState`、`newStateWith`、`twoProjects`）加 `s.SetSyncScan()`。

- [ ] **步骤 4：运行测试验证通过** — `go test ./internal/view/ -v`，预期 PASS（既有测试在同步模式下照旧）

- [ ] **步骤 5：Commit** — `git commit -m "feat: 项目树异步扫描、项目名后 loading 图标与目录过大提示"`

---

### 任务 6：目录 / 文件变动的增量更新

**文件：** 创建 `internal/workspace/patch.go`、`patch_test.go`；修改 `internal/view/watch.go`（`applyChanges`）、`internal/workspace/scan.go`（抽出 `lessNode`）

- [ ] **步骤 1：编写失败的测试**

```go
// internal/workspace
func TestInsertDocCreatesTheDirectoriesAboveIt(t *testing.T) // docs/a.md 进树：docs 是目录节点，插在有序位置
func TestInsertDocKeepsTheOrder(t *testing.T)                // 目录在前、名字小写序，插进去之后与 sortTree 的结果一致
func TestRemoveDocDropsTheEmptyDirectories(t *testing.T)     // 删掉唯一文件后 docs 也消失
func TestRenameDocMovesTheNode(t *testing.T)                 // 改名后旧路径不在、新路径在、顺序仍对

// internal/view
// TestPatchingTheTreeMatchesARescan drives the same changes through
// applyChanges and through a full rescan, and compares the trees.
func TestPatchingTheTreeMatchesARescan(t *testing.T)
func TestAWriteDoesNotRescanTheTree(t *testing.T)       // 假扫描计次：写入一个已存在的文件 → 扫描次数不变
func TestANewFileAddsOneNode(t *testing.T)              // 假扫描计次：新增一个 md → 只补一个节点，不重扫
func TestADeadDirectoryRescans(t *testing.T)            // 只有 TreeDirty（无 Docs）→ 重扫一次
```

- [ ] **步骤 2：运行测试验证失败** — `go test ./internal/workspace/ -run 'TestInsertDoc|TestRemoveDoc|TestRenameDoc' ./internal/view/ -run 'TestPatchingTheTree|TestAWriteDoesNotRescan' -v`，预期 FAIL

- [ ] **步骤 3：实现** — `scan.go` 把 `sortTree` 的比较抽成 `lessNode(a, b *Node) bool`；
  `patch.go` 三个就地改树的函数（插入时按 `lessNode` 找到位置；删除时递归清掉空目录；改名 = 删 + 插，保留原有子节点）；
  `applyChanges`：`for _, change := range set.Docs { 按 Op patch }` + `bumpTree()`；`set.TreeDirty && len(set.Docs) == 0` → `s.reloadTree()`。

- [ ] **步骤 4：运行测试验证通过** — `go test ./internal/workspace/ ./internal/view/ -v`，预期 PASS

- [ ] **步骤 5：Commit** — `git commit -m "feat: 目录与文件变动就地更新项目树，不再整棵重扫"`

---

# 阶段 B：树虚拟化（`ui.Outline`）

### 任务 7：把项目树换成 `ui.Outline`

**文件：** 创建 `internal/view/tree.go`、`internal/view/tree_test.go`；修改 `internal/view/view.go`（`fileTree`、`treeItems`、`nodeMenu`、`projectActions`、`projectMenu`）、
`internal/view/state.go`（`treeOpen` / `treeSeen` / `treeScroll` 字段、`setTreeOpen`、`openOf` 的注释）、`internal/view/treesearch.go`（过滤视图搬到 `roots`/`children`）
及受影响的测试（`view_test.go`、`shell_test.go`、`treesearch_test.go`）

- [ ] **步骤 1：编写失败的测试**

```go
func TestTheTreeOnlyBuildsWhatShows(t *testing.T)     // 2000 个 md、全部展开：s.treeOpen.Rows() 为 2000+，而 tt.Frame() 只建了可见的那些（用假行计数：在 row builder 里 s.built++）
func TestUnfoldingFoldingAndOpeningTheWholeTree(t *testing.T) // 展开全部 / 折叠全部 / 按项目，行为与旧的 setTreeOpen 一致
func TestTheTreeKeepsWhatTheUserFoldedAcrossAReload(t *testing.T) // 折叠 docs → 重扫 → 仍然折叠（Open 里没有它、treeSeen 里有它）
func TestTheNewDirectoriesOpenByThemselves(t *testing.T)          // 新出现的目录默认展开
func TestAProjectNodeOpensAndClosesItsWholeProject(t *testing.T)   // 多项目模式下项目节点的箭头
func TestTheTreeRowsKeepTheirMenus(t *testing.T)                   // 文件行右键（复制文件/名称/路径）、目录行（+在文件管理器中打开）、项目行（折叠/展开）
```

并把既有测试按新容器调整（**只改实现细节，不断言行为的地方别动**）：
`TestTreeKeepsOpenStateAcrossRefresh`（改用 `treeOpen.Open`）、`TestDirectoriesOfTwoProjectsOpenApart`、`TestSingleOpenProjectHasNoProjectLevel`、
`TestTreeGroupsByProjectWhenSeveralAreOpen`、`TestClickingDocShowsIt`、`TestUnfoldingAndFoldingEveryDirectoryOfTheTree`、
`TestTheProjectMenuFoldsOneProjectOnly`、`TestTheProjectPanelFoldsOneProject`、`TestTheProjectRowFoldsOnlyItsOwnProject`、
`TestTheTreeScrollerIsNamed`（`Label("项目树")` 挂到 `Outline` 元素上）、`TestShellSlotsBuild`。

- [ ] **步骤 2：运行测试验证失败** — `go test ./internal/view/ -run 'TestTheTreeOnlyBuildsWhatShows|TestUnfoldingFoldingAndOpening|TestTheTreeKeepsWhatTheUserFolded' -v`，预期 FAIL：`s.treeOpen undefined`

- [ ] **步骤 3：实现**

`tree.go`：
```go
// treeKey names a row of the sidebar's tree: the project, and the path in it.
// The path is "" for a project node, and the file's path for a document.
type treeKey struct {
	project string
	path    string
}
```
- `State.treeOpen ui.OutlineState[treeKey]`（`Indent` 留零 = 主题 `Space(4)`）、`State.treeSeen map[treeKey]bool`；
  `State.treeScroll` 删除（`Outline` 自带滚动）。
- `roots() []treeKey`、`children(k treeKey) []treeKey`（在 `s.trees` 里按 ID 找项目、按路径找节点；文件或无此节点返回 `nil`）。
- `rowKeyOf(projectID, path string) treeKey`、`rowOf(roots []treeKey, want treeKey) int`（按 `roots` + `children` + `Open.Has` 走，-1 找不到）。
- `treeBody(c)`：`ui.Outline(c, &s.treeOpen, s.roots(), s.children, func(k treeKey){ s.treeRow(c, k) }).Label("项目树").Grow(1).HandleInput(s.treePress)`。
- `treeRow(c, k)`：`ui.Row(c)` + `OnClick`（打开文档 / 切换项目节点的展开）+ 选中底色（`selectedRowTint`）+ `components.ContextMenu`；
  目录/文件/项目节点三条分支，项目节点后面仍是那两枚 `IconButtonCompact`（label 不变）与 spinner。
- `applyTree` 里对每个新目录 `Open.Add` 并记 `treeSeen`；`setTreeOpen(projectID, open)` 改成对 `Open` 增删 + `Generation++`。
- `view.go`：`fileTree` 的 `ui.Scroll` + `ui.Tree` 整段换成 `s.treeBody(c)`（空态、搜索空态的分支保留）。

- [ ] **步骤 4：运行测试验证通过** — `go test ./internal/view/ -v`，预期 PASS

- [ ] **步骤 5：Commit** — `git commit -m "feat: 项目树改用 ui.Outline，每帧只构建可见行"`

---

### 任务 8：搜索过滤视图与定位滚动收口到新容器

**文件：** 修改 `internal/view/treesearch.go`（`roots`/`children` 的搜索分支、`reveal` 改 `List.ScrollIntoView`）、
`internal/view/treesearch_test.go`、`internal/view/bench_test.go`

- [ ] **步骤 1：编写失败的测试**

```go
func TestTheSearchShowsOnlyWhatMatchesInTheOutline(t *testing.T) // 搜索时 rows 只有命中文件与其祖先目录；关闭搜索后复原成用户原来的展开
func TestTheSearchRevealsTheCurrentCandidate(t *testing.T)       // ↑↓ 之后 first,last := s.treeOpen.List.Visible()；当前候选的行号在 [first,last] 里
```

并把 `TestSteppingTheCurrentCandidateScrollsTheTree` 从 `s.treeScroll.Y > 0` 改成 `List.Visible()` 的断言
（`s.treeScroll` 已被删）。

- [ ] **步骤 2：运行测试验证失败** — `go test ./internal/view/ -run 'TestTheSearchShowsOnlyWhatMatchesInTheOutline|TestTheSearchRevealsTheCurrentCandidate|TestSteppingTheCurrentCandidate' -v`，预期 FAIL

- [ ] **步骤 3：实现** — `roots()`/`children()` 在 `searching()` 时走候选集合（有候选的项目 → 有候选的目录 → 候选文件），
  并在 `setTreeQuery` / `refreshCandidates` 里把这些键临时 `Open.Add`（记在 `s.treeSearchOpened []treeKey`，`closeTreeSearch` 时移除并复原）；
  `refreshCandidates` 之后 `bumpTree()`；候选定位改成 `s.treeOpen.List.ScrollIntoView(rowOf(...))`。
  `bench_test.go` 的 `BenchmarkCandidateLookup` 改成调新名字（见任务 12 的 `isCurrentCandidate`；本任务先按老名字留着，任务 12 一起改）。

- [ ] **步骤 4：运行测试验证通过** — `go test ./internal/view/ -v`，预期 PASS

- [ ] **步骤 5：Commit** — `git commit -m "feat: 搜索过滤视图与候选定位走 ui.Outline"`

---

# 阶段 C：功能

### 任务 9：独立文件进树，以及 `State.OpenDocument`

**文件：** 修改 `internal/view/state.go`（`OpenDocument`、`afterProjectsChanged`、`shortPath`）、`internal/view/watch.go`（`startWatch(p)`）、
`internal/view/tree.go`（单文件项目的行）、`internal/view/view.go`（项目面板的图标/文案）；测试 `internal/view/singlefile_test.go`（新建）

- [ ] **步骤 1：编写失败的测试**

```go
func TestOpeningAFileAddsItAsItsOwnEntry(t *testing.T)       // OpenDocument(path) → 配置 2 条（目录项目 + 单文件），正文是它
func TestOpeningAFileShowsIt(t *testing.T)                   // 树里有文件名这一行，正文是它的标题
func TestOpeningTheSameFileTwiceIsOneEntry(t *testing.T)     // 调两次 → 配置仍 2 条，一个标签，一条提示
func TestTheTreeShowsProjectsAndLooseFilesTogether(t *testing.T) // 多项目模式：项目节点 + 独立文件行同时出现
func TestOpeningAFileOfAnOpenProjectSelectsItThere(t *testing.T)  // 文件在已打开项目里 → 不新增条目
func TestOpeningAFileOfAClosedProjectOpensIt(t *testing.T)       // 项目关着 → 打开它并选中
func TestOpeningAFileThatIsNotMarkdownIsRefused(t *testing.T)    // .txt / 不存在 / 目录 → 不新增条目，一条提示
func TestTheTitleRowShowsALooseFilesPath(t *testing.T)           // shortPath() == 绝对路径
func TestAGoneFileProjectIsDropped(t *testing.T)                 // 文件删掉后重扫 → 条目消失 + 「文件不可用」
func TestAWriteToALooseFileRefreshesTheBody(t *testing.T)        // 外部写入 → 正文变了（增量更新 + 缓存失效）
func TestTheSameFileInAProjectAndOnItsOwn(t *testing.T)          // 两处各一个标签，关一个另一个还在
```

- [ ] **步骤 2：运行测试验证失败** — `go test ./internal/view/ -run 'TestOpeningAFile|TestTheTreeShowsProjectsAndLooseFilesTogether|TestTheSameFileInAProject' -v`，预期 FAIL：`s.OpenDocument undefined`

- [ ] **步骤 3：实现**

- `OpenDocument(path string)`：`filepath.Abs` → `EvalSymlinks`（失败退回 abs）→ `workspace.IsMarkdown` + `os.Stat` 校验
  （失败 `notifyErr("无法打开：" + abs)`）→ `workspace.Locate(s.cfg.Projects, abs)` 命中则
  （没打开就 `cfg.SetOpen` + `s.startWatch(p)`）`s.afterProjectsChanged()` + `s.selectDocIn(id, rel)`；
  否则 `p, _ := s.cfg.AddFileProject(abs)` → `s.startWatch(p)` → `s.afterProjectsChanged()` → `s.selectDocIn(p.ID, filepath.Base(abs))`。
  开头 `if sel == s.sel { return }`（幂等），末尾 `notifyDone("已打开 " + filepath.Base(abs))` + `if s.win != nil { s.win.Invalidate() }`。
- `afterProjectsChanged()`：`activeID = cfg.ActiveID`、`ws.SetConfig`、`save()`、`reloadTree()`（`AddProjects` 的尾巴改成调它）。
- `shortPath()`：`p.File != ""` → `p.File`。
- `tree.go`：单文件项目的根（`{project, ""}`）不再当项目节点画 —— `roots()` 对它直接给文件键 `{project, base}`；
  多项目模式下也是文件行本身（行内容与普通文件行共用 `treeRow` 的文件分支）。
- `startWatch(p workspace.Project)`：`p.File != ""` → `watch.WatchFile(ctx, p.File)`，否则 `watch.Watch(ctx, p.Root)`；
  三处调用点跟着改；失败消息走任务 5 的 `watchFailure`。
- `projectSwitcher`：单文件项目用 `lucide.FileText()`、无障碍名 `文件：<名字>`（目录项目维持 `项目：<名字>`）。

- [ ] **步骤 4：运行测试验证通过** — `go test ./internal/view/ -v`，预期 PASS

- [ ] **步骤 5：Commit** — `git commit -m "feat: 打开一个 md 文件成为一个独立文件条目"`

---

### 任务 10：底部工具栏的「打开 Markdown 文件」

**文件：** 修改 `internal/view/view.go`（`leftToolbar`）、`internal/view/state.go`（`OpenDocumentFile`）；测试 `internal/view/view_test.go`、`shell_test.go`

- [ ] **步骤 1：编写失败的测试**

```go
// TestTheToolbarOpensAProjectOrADocument pins the two entries of the sidebar's
// footer, side by side, before the reload button.
func TestTheToolbarOpensAProjectOrADocument(t *testing.T) {
	s, _ := newState(t)
	tt := ui.NewTester(func(c *ui.Context) { s.LeftToolbar(c) }, 300, 48)
	open, ok := tt.Find("打开项目")
	if !ok {
		t.Fatalf("no 打开项目; texts = %q", tt.Texts())
	}
	doc, ok := tt.Find("打开 Markdown 文件")
	if !ok {
		t.Fatalf("no 打开 Markdown 文件; texts = %q", tt.Texts())
	}
	reload, _ := tt.Find("刷新")
	if doc.X < open.X || doc.X+doc.W > reload.X+1 {
		t.Errorf("打开 Markdown 文件 at X %v..%v, want it between %v and %v", doc.X, doc.X+doc.W, open.X, reload.X)
	}
}
```

`TestShellSlotsBuild` 的 `LeftToolbar` 期望值加上 `"打开 Markdown 文件"`。

- [ ] **步骤 2：运行测试验证失败** — `go test ./internal/view/ -run 'TestTheToolbarOpensAProjectOrADocument|TestShellSlotsBuild' -v`，预期 FAIL

- [ ] **步骤 3：实现** — `leftToolbar` 在「打开项目」之后插
  `components.IconButton(c, lucide.FilePlus(), "打开 Markdown 文件", components.IconButtonPlain)` → `s.OpenDocumentFile()`；
  `OpenDocumentFile()` 用 `mygo.Dialog.Open(mygo.OpenDialogOptions{Title: "打开 Markdown 文件", Multiple: true, Filters: []mygo.FileFilter{{Name: "Markdown", Extensions: []string{"md", "markdown"}}}, Parent: s.win})`，
  逐个 `s.OpenDocument(path)`。

- [ ] **步骤 4：运行测试验证通过** — `go test ./internal/view/ -v`，预期 PASS

- [ ] **步骤 5：Commit** — `git commit -m "feat: 侧栏底部新增「打开 Markdown 文件」"`

---

### 任务 11：crosshair —— 把当前文档定位到项目树

**文件：** 创建 `internal/view/locate.go`、`locate_test.go`；修改 `internal/view/view.go`（`treeBulkActions`）、`internal/view/view_test.go`

- [ ] **步骤 1：编写失败的测试**

```go
func TestTheCrosshairBringsTheDocumentIntoView(t *testing.T)     // 40 个文件、选中最后一个 → Click 后 first,last := List.Visible()，目标行在 [first,last] 里；祖先目录 Open.Has 为真
func TestTheCrosshairIsDisabledWithoutADocument(t *testing.T)    // tt.Access("在项目树中定位当前文档").Disabled
func TestLocatingClosesTheTreeSearch(t *testing.T)               // 搜索开着 → 定位后 s.treeSearch.open == false，query 留着，树复原成用户原来的展开
func TestLocatingAFileTheTreeDoesNotShowIsReported(t *testing.T) // 手工塞一个不在树里的 docSel → 一条「项目树里没有这个文件」，不 panic
func TestLocatingALooseFileWorks(t *testing.T)                   // 单文件项目的那一行也能被定位
```

并把 `TestBulkActionsSitLeftOfTheProjectSwitcher` 的标签表加上 `"在项目树中定位当前文档"`。

- [ ] **步骤 2：运行测试验证失败** — `go test ./internal/view/ -run 'TestTheCrosshair|TestLocating' -v`，预期 FAIL：`no 在项目树中定位当前文档`

- [ ] **步骤 3：实现** — `locate.go`：

```go
// locateDoc brings the document shown into the tree's view: the search folds
// away, the directories above it unfold, and the tree slides to its row.
func (s *State) locateDoc() {
	if s.sel.empty() {
		return
	}
	if s.treeSearch.open {
		s.closeTreeSearch()
	}
	if !s.revealDocInTree(s.sel) {
		s.notifyWarn("项目树里没有这个文件：" + s.sel.path)
		return
	}
	if row := s.rowOf(s.roots(), treeKey{project: s.sel.project, path: s.sel.path}); row >= 0 {
		s.treeOpen.List.ScrollIntoView(row)
	}
}

// revealDocInTree opens what sits above the document sel in the tree. It
// reports whether the tree holds it at all.
func (s *State) revealDocInTree(sel docSel) bool
```

（**不调** `List.Choose`：选中行是自绘的 `selectedRowTint`，不靠 `List.Selected`；多设一个选择只会让键盘行为变模糊。）

`revealDocInTree`：找到 `sel.project` 的树 → 逐段沿 `workspace.Node` 走，每段目录 `Open.Add(treeKey{sel.project, 该目录路径})`；
多项目模式还要 `Open.Add(treeKey{sel.project, ""})`；末段存在即成功（单文件项目直接成功）。改完 `bumpTree()`。

`view.go` 的 `treeBulkActions` 末尾：

```go
if components.IconButton(c, lucide.Crosshair(), "在项目树中定位当前文档", components.IconButtonPlain).
	Disabled(s.sel.empty()).Clicked() {
	s.locateDoc()
}
```

- [ ] **步骤 4：运行测试验证通过** — `go test ./internal/view/ -v`，预期 PASS

- [ ] **步骤 5：Commit** — `git commit -m "feat: 侧栏顶部新增 crosshair，把当前文档定位到项目树"`

---

### 任务 12：搜索只高亮当前候选

**文件：** 修改 `internal/view/treesearch.go`、`internal/view/theme.go`、`internal/view/tree.go`（行底色）；
测试 `internal/view/treesearch_test.go`、`internal/view/bench_test.go`

- [ ] **步骤 1：编写失败的测试**

把 `TestCandidatesAndTheOneChosenAreTwoDifferentFills` 换成：

```go
// TestOnlyTheCurrentCandidateIsFilled pins the single wash the search keeps: the
// tree already shows nothing but matches, so the candidate the keyboard is on is
// the one row that reads differently.
func TestOnlyTheCurrentCandidateIsFilled(t *testing.T) {
	s, root := newState(t)
	for _, name := range []string{"a_plan.md", "b_plan.md", "c_plan.md", "other.md"} {
		writeDoc(t, root, name, "# "+name+"\n")
	}
	s.Refresh()
	openTreeSearchOn(s, "plan")
	s.treeSearch.current = 1
	id := s.Projects()[0].ID
	switch {
	case !s.isCurrentCandidate(id, "b_plan.md"):
		t.Error("b_plan.md is not the current candidate, want it")
	case s.isCurrentCandidate(id, "a_plan.md"):
		t.Error("a_plan.md reads as the current candidate, want a plain row")
	case s.isCurrentCandidate(id, "other.md"):
		t.Error("other.md is not a candidate at all, want no fill")
	}
}
```

`bench_test.go` 的 `BenchmarkCandidateLookup` 改成调 `s.isCurrentCandidate(...)`。

- [ ] **步骤 2：运行测试验证失败** — `go test ./internal/view/ -run TestOnlyTheCurrentCandidateIsFilled -v`，预期 FAIL：`s.isCurrentCandidate undefined`

- [ ] **步骤 3：实现** — 删 `treeRowFill` / `rowNone` / `rowCandidate` / `rowCurrent` 与 `candidateRowTint`，加

```go
// isCurrentCandidate reports whether the file is the candidate the keyboard is
// on: the one row the search fills.
func (s *State) isCurrentCandidate(projectID, path string) bool {
	return s.searching() && s.candidateIndex(projectID, path) == s.treeSearch.current
}
```

`treeRow` 的文件分支里只在 `isCurrentCandidate` 时上 `currentCandidateTint`（其余不上色，`selectedRowTint` 只在非搜索时用）。

- [ ] **步骤 4：运行测试验证通过** — `go test ./internal/view/ -v`，预期 PASS

- [ ] **步骤 5：Commit** — `git commit -m "feat: 树搜索只高亮当前候选，其余候选不再上色"`

---

# 阶段 D：接线与文档

### 任务 13：把「用此应用打开」接到窗口

**文件：** 修改 `main.go`

- [ ] **步骤 1：接线**

```go
// windowState is the window's state, for the handlers that arrive outside a
// frame: a second launch's command line, and a file opened with the app.
var windowState *view.State
```

- `WhenReady` 里 `state.StartWatching()` 之后：`openDocuments(launch.Documents(os.Args[1:], wd))`（`wd, _ := os.Getwd()`）。
- `OnSecondInstance(func(args []string, wd string) { openDocuments(launch.Documents(args, wd)); tray.Show(mainWindow) })`。
- `WhenReady` 里加 `mygo.App.OnOpenFile(func(path string) { openDocuments([]string{path}); tray.Show(mainWindow) })`。
- `openDocuments(paths []string)`：`for _, p := range paths { if windowState != nil { windowState.OpenDocument(p) } }`。

- [ ] **步骤 2：跑一遍全部测试与构建** — `go vet ./... && gofmt -l . && go test ./... && go build -o mp.exe .`，预期 vet / gofmt 无输出、测试 PASS、构建通过

- [ ] **步骤 3：手动验证**

```bash
./mp.exe "D:/some/notes/demo.md"   # 侧栏出现 demo.md 这一行（独立文件），正文是它
./mp.exe "D:/some/notes/demo.md"   # 第二个进程立刻退出，第一个把窗口叫到前面，仍只有一个标签
./mp.exe "D:/some/huge-repo"       # （不支持目录参数）只把窗口叫到前面，不崩
```

另测：打开一个**大目录**（>1 万条目）作为项目 → 打开期间项目名后面有 spinner、窗口不卡；超过 100 万条目的目录 → 一条
「目录过大无法打开，请缩小目录范围」。记下结果（这是唯一不能自动化的一段）。

- [ ] **步骤 4：Commit** — `git commit -m "feat: 命令行与文件关联打开 md 文件时自动进项目树"`

---

### 任务 14：文档回写

**文件：** `README.md`、`AGENTS.md`、`CHANGELOG.md`、`docs/superpowers/specs/2026-11-21-markdown-preview-design.md`、
`docs/superpowers/plans/2026-11-24-tree-fold-and-file-search.md`

- [ ] **步骤 1：README**

- 用法：侧栏底部现在是「打开项目（目录，可多选）/ 打开 Markdown 文件（`md`、`markdown`，可多选）/ 刷新 / 铃铛 / MCP」；
  打开的文件若已在某个打开的项目里就直接选中，否则成为**独立文件**条目（与项目并列、不套项目节点），标题行显示它的绝对路径；
  命令行、系统「打开方式」、托盘里的第二实例都走同一条路。
- 用法：侧栏顶部「展开全部目录树 / 折叠全部目录树」右边新增 **crosshair**：把当前标签的文档在树里展开并滚进视野。
- 用法的文件名搜索那段：删掉「候选一层淡主色」，改成**只有当前候选高亮**。
- 新增一段「大目录」：扫描在后台跑，项目名后面有 loading 图标，树在扫完前显示上一次的内容；
  **超过 100 万条目的目录会被拒绝**（提示「目录过大无法打开，请缩小目录范围」），`刷新` 可重试；监听上限同一数字，
  超了就停止监听并提示（`刷新` 仍可用）；目录/文件变动就地更新，不再整棵重扫。
- 已知限制：**删掉**「树的 md 项超过约 3000 个时会变慢」这条（换成 `ui.Outline`）；补「独立文件只显示它自己，
  不扫描同级文件」与「监听到上限时该项目不再自动跟随」。

- [ ] **步骤 2：AGENTS.md**

- 结构表：加 `internal/launch`（命令行参数里的 md 文件，纯函数）；`internal/config` 提 `Project.File`；
  `internal/view` 提 `scan.go`（异步扫描 + 上限）与 `tree.go`（`treeKey` / `roots` / `children` / `rowOf` / 自绘行）与 `locate.go`。
- 硬规则／坑加这些：（1）`packageFileExtensions` 只有 `mygo build`/`mygo dev` 会写，`go run .` 下 `OnOpenFile` 不触发，
  所以必须自己解析 `os.Args`；（2）单文件项目要 `watch.WatchFile`，用目录监听会盯着整个目录；
  （3）`workspace.Tree(p)` 不能换成 `BuildTree(p.Root)`；（4）**树的展开状态在 `treeOpen.Open`（`treeKey`）里，
  目录（TOC）那一棵 `ui.Tree` 仍用 `s.open` / `s.openOf`，两者别混**；（5）扫描结果必须带项目 ID + 代数，
  迟到的结果一律丢弃；（6）`OutlineState.Rows()` 是**上一帧**的快照，算行号一律用 `rowOf`；
  （7）扫描上限与监听上限是同一个常量 `workspace.MaxEntries`，改数字要同时想清楚这两处。
- 删掉所有「两种底色」「`ui.Tree` 超过 3000 个会慢」的提法。

- [ ] **步骤 3：CHANGELOG**

「未发布」下：**新增**（crosshair 定位、打开 Markdown 文件 / 独立文件、用此应用打开 md、大目录后台扫描与 loading 图标）、
**变更**（树搜索只高亮当前候选；项目树改为虚拟化，几千个文件也不卡；目录/文件变动就地更新）、
**已知限制**（超过 100 万条目的目录打不开；监听撞上限的项目不再自动跟随；独立文件只显示它自己）。

- [ ] **步骤 4：规格与上一份计划**

规格末尾追加 `### 定位、独立文件、大目录与虚拟化的树（新增，2026-11-25）`，并**标注推翻**两处：2026-11-24 那节的
「两种底色」、已知限制里的「`ui.Tree` 超过约 3000 个会慢」；「目标」里「打开一个目录成为一个项目」补成「目录或一个 md 文件」。
`docs/superpowers/plans/2026-11-24-tree-fold-and-file-search.md` 的「执行后的修订」追加两条：
D4 的两种底色改成一层、树容器从 `ui.Tree` 换成 `ui.Outline`（`s.open` 只留给 TOC）。

- [ ] **步骤 5：全绿并提交** — `go vet ./... && gofmt -l . && go test ./...`，预期无输出、PASS；
  `git add README.md AGENTS.md CHANGELOG.md docs/superpowers/ && git commit -m "docs: 回写定位、独立文件、大目录与树虚拟化"`

---

## 执行后的修订（只追加，不改写上面的任务步骤）

### 用户验收反馈（2026-11-25，执行完之后）

1. **树行的 hover / 选中要对齐 mygo 的 `ui.Tree`**（截图对比：树里选中的文档只是淡淡一层，与 hover 几乎一样，
   而目录（TOC）里选中项是蓝底反色字）。实现：`tree.go` 的 `treeRowBox`（`Height(Space(7))` + `Radius` + 内边距，
   照 `ui.TreeItem` 的量），悬停 `t.SurfaceHover`、当前文档 `t.Accent` + `t.AccentText`。
   `selectedRowTint` 只剩项目面板在用一个地方（面板行不是树的项）。
2. **独立文件那一行要有图标**：`treeRow` 对单文件项目的行画 `lucide.FileText`（选中时用 `AccentText`），
   项目节点顺带画 `lucide.Folder`，两者才分得清。
3. **打开大目录（Downloads）时卡住、树里也不显示这个项目**（截图：面板里已勾上，树里没有 Downloads，
   popup 里看着像在加载）。两个原因：
   - `startWatch` 在窗口线程上同步跑 `mygo.WatchFiles`，它要把整个目录读一遍建快照——几万条目就是几秒的白屏。
     现在改成 `watchProject` 字段（测试可替换）+ goroutine，`watches[id]` 先登记 cancel，关项目能取消；
     失败时 `ctx.Err() != nil` 说明是窗口自己取消的，不报警。
   - 扫描是异步的，但 `s.trees` 里没有占位，新项目**在扫描结束前根本不存在**于树中。
     现在 `loadTree` 先 `showPlaceholder`（`root` 为 nil 的 `projectTree`），项目行立刻出现、后面跟 spinner
     （`Label("正在扫描 <项目名>")`，树项的无障碍名是它内容拼起来的，所以 `Access` 要按包含判断）；
     `reloadTree` 改成用 `root != nil` 判断「已读过」，`fileTree` 在没有文档但又正在扫时显示「正在扫描 …」。
   测试：`TestOpeningAProjectShowsItBeforeTheScanEnds`、`TestOpeningAProjectDoesNotWaitForItsWatch`。
   顺带修了 `TestSteppingTheCurrentCandidateScrollsTheTree`：它以前拿 `plan_39.md` 当最后一个候选，
   字符串序里其实不是，行高变大之后正好差一行才暴露出来。

### 第二轮反馈（2026-11-25，同一晚）

4. **loading 太大** → 换 mygo `plugins/loading` 的 `Snake`，`Options{Size: t.Space(3)}`（12 DIP），
   `projectSwitcher` 与树的项目行都用它。
5. **扫描完成后项目没有箭头、要点一下才能展开** → 根因是 `ui.Outline` 缓存了「走一遍开着的项」的结果，
   而扫描结果落地只换内容（roots 与 `Open` 都没变），`afterTreesChanged` 又没 `bumpTree()`。现在
   `afterTreesChanged` 第一件事就是 `bumpTree()`；`TestAProjectShowsItsEntriesWhenTheScanLands` 先看扫描中
   （没有子项、`Expandable` 为假），再看扫完（内容与 `Expandable` 都有，不用点）。
6. **hover / 选中要整行** → `treeBody` 把 `s.treeOpen.Prefix` 设成空函数，Outline 的行就没有内边距与箭头了，
   行由 `treeRow` 自己画整条（高 `Space(7)`、圆角、`Padding(0, Space(1))`），底色铺满；缩进按
   `strings.Count(path, "/") + 1` 自己算（`treeDepth`）。
7. **文件图标放进 arrow box** → 箭头也改成自绘（`treeMark`：目录/项目行画三角 + 点击切换，文档行画 `FileText`），
   两者在同一列。代价是丢掉 `OutlineRow.Arrow()` 的 Alt/Option「全部展开」，键盘与读屏的展开仍在。
   `TestClickingTheArrowOfADirectoryOpensItOnce` 钉住「点箭头只切换一次」。
8. **「折叠全部」保留一级展开** → 用户选**方案 b**：折叠连**项目节点**一起收起（收完只剩项目名），
   与单项目窗口「收成顶层」一致；推翻上一期「折叠不动项目节点」。改的是 `setTreeOpen`：多项目时节点跟着
   `open` 一起写；`TestFoldingEveryTreeClosesTheProjects` 钉住这个，`TestTheProjectRowFoldsOnlyItsOwnProject` /
   `TestTheProjectMenuFoldsOneProjectOnly` 的断言改成「节点也关了」。
   顺带发现并修掉：项目行的 `branch` 只对非项目行算，项目节点因此**从来没有展开三角**（收起来时看不出能再打开）。

### 第三轮反馈（2026-11-25，方案 b 与 GUI 验证）

9. **GUI 验证要能看图**：按 `D:/golang/tailcat_tool` 的做法加了 `internal/view/screenshots_test.go`
   （`MDPREVIEW_SHOT_DIR=<目录> go test ./internal/view -run TestScreenshots`，不设就 SKIP，用例表 `shotCases()`），
   并把规范写进 AGENTS.md 的「GUI 验证（硬规则）」：无头环境用 `ui.NewTester`（不要 `go run`）、断言不依赖像素、
   改完界面必须看图、Tester 的时钟是虚拟的（`Advance` + `Sleep` 才让异步扫描落地）。
   本轮就是靠这 9 张图核对的行底色、箭头/图标同列、loading 大小与折叠后的样子。
10. **loading 定稿 20 DIP**（`loading.DefaultSize`）：12 太小、原来的 32 太大。
11. **loading 要跟在项目名后面**：以前它落在名字与右边两个按钮的中间——因为名字 `Text` 带 `Grow(1)`，
    和 `projectActions` 里的 `Spacer` 平分了剩余空间。现在名字不 Grow，行尾自己放 `ui.Spacer(c)`，
    loading 紧贴名字（`MarginStart(Space(0.5))`）。
12. **项目文件夹图标放大**：`Rem(1.25)`（20）+ `AlignSelf(ui.Center)` + `MarginEnd(Space(1))`，
    以前 `Rem(1)` 且只靠行的 2 DIP gap，贴着名字。
13. **顶部三个树操作按钮缩小**：`treeBulkActions` 从 `IconButtonPlain`（32）改成
    `IconButtonCompact` + `Size(treeBulkSize, treeBulkSize)`（24）；底部的动作按钮仍按上一期的规矩保持 32。
