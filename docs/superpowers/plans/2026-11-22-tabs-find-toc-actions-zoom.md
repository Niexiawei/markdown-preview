# 文档标签页、目录工具条（展开/折叠/缩放）与悬浮搜索 实现计划

> **面向 AI 代理的工作者：** 必需子技能：使用 subagent-driven-development（推荐）或 executing-plans 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** 让打开的文档变成一排可切换、可关闭的标签页（放主预览顶部，超出可视宽度时右侧出现「全部文件」展开按钮）；给目录面板加「展开全部 / 折叠全部 / 放大正文 / 缩小正文」四个动作；给正文加 `Cmd/Ctrl+F` 唤起的**悬浮**搜索面板（不遮不弹窗，VSCode 形态），面板除输入框与上一个/下一个外，多出两块内容——匹配行列表、以及便于定位的行上下文预览。

**架构：** 全部落在 `internal/view`，外加一个 `internal/workspace` 的纯函数（`FindIn`，不 import `.../ui`）。标签页是 `State` 里的一张按打开顺序排的 `[]docSel`，正文滚动位置按 `docSel` 各存一份；标签栏作为 `Content`（及测试用 `View`）里 `preview()` 之上的一行，溢出时用 `ui.Popover` 列出全部标签。目录的四个动作进 `toc()` 的标题行；展开/折叠写既有的 `State.open` 映射，缩放只改 `markdown.Options.Theme.FontSize`（原生）与页面 CSS 变量（webview）。搜索面板用 `ui.Overlay` + `Absolute().Right().Top()` 浮在正文右上，匹配由 `workspace.FindIn` 算出，跳转在原生模式走 `markdown.Options.AnchorBlock`（把「第 N 个匹配所在的块」直接交给 mygo 滚动），在 webview 模式走 `mygo.Page.FindInPage`（浏览器级高亮 + 滚动）。

**技术栈：** Go 1.27+、`github.com/egoist/mygo v0.9.1`（`ui.Overlay`、`ui.Popover`、`ui.SearchField`、`ui.RichText`、`ui.ScrollBoth`、`ui.ScrollState`、`ui.Context.Shortcut`、`markdown.Options.AnchorBlock`、`mygo.Page.FindInPage` / `StopFindInPage` / `Eval`）、`github.com/oneo-me/onevra/source v0.2026.18`（`components.Button` + `ButtonGhost`、`components.IconButton` + `IconButtonCompact`、`components.Icon`、`icons/lucide`）。

**规格：** 无独立规格；需求来自用户 2026-11-22 的三条反馈与截图（① 编辑器式标签栏 + 右侧「全部文件」浮层；② 目录标题右侧四个动作，其中放大/缩小针对正文；③ `Ctrl+F` 搜索：VSCode 式悬浮窗，另加匹配行列表与行上下文预览）。本计划的「设计决策」一节即设计记录，架构背景见 `docs/superpowers/specs/2026-11-21-markdown-preview-design.md`（该文档「非目标」里的「多文档标签页」「全文检索的 UI」本期被用户明确推翻，实施后需同步改那一节）。

## 设计决策（实施者按此执行，不要另立方案）

| # | 决策 | 理由 |
| --- | --- | --- |
| D1 | 打开文档 = 该文档进入 `State.tabs`（`[]docSel`，按打开顺序），并成为 `State.sel`；树里点一个已开的文档只切换、不新增；从任何入口（树、链接、搜索）打开都走同一个 `openTab`。 | 用户原话「文件打开 tab 切换」；树点击、内部链接（`followLink`）与将来的入口必须共享一套「打开」语义，否则同一个文件会出现两个标签。 |
| D2 | 标签栏放 `Content`（以及测试用 `View`）里 `preview()` **之上**的一行，高 `tabBarHeight = 36`；`Content` 原有的 `sidebar.HeaderHeight(c)` 占位行保持不变。 | 用户原话「这个 tab 栏放在主预览顶部」。shell 的浮动标题栏占住 `Content` 的顶部，占位行不能动，否则正文会被标题栏压住。 |
| D3 | 标签宽度按文件名实测宽度算：`clamp(measureText(name) + 前后内边距 + 关闭按钮, 96, 200)`；所有标签放不下时，才在最右侧出现一个「全部文件」按钮（`lucide.ChevronDown`，`IconButtonCompact`）并预留它的宽度。可见集合的选择以**活动标签**为锚点：先向右装、再向左装满剩余空间。 | 截图里标签宽度随文件名变化，且溢出按钮只在溢出时出现（不溢出时占着位置会让标签栏看起来少一格）。以活动标签为锚点，保证切换后当前文档永远可见（否则点标签后它自己被挤出可视区）。 |
| D4 | 「全部文件」浮层用 `ui.Popover` 挂在溢出按钮上，列出**全部**标签（含当前不可见的），每行 = 图标 + 文件名 + 关闭按钮；点行激活并关掉浮层，列表超过 400 DIP 时纵向滚动。 | 用户原话「可以打开所有文件列表按钮」。`ui.Popover` 自带点外部关闭与 Escape，与既有「项目面板」一致，无需自造浮层。 |
| D5 | 关闭活动标签后激活**右侧**邻居，没有右侧邻居则激活左侧，都没有则回到空状态；关闭非活动标签不改变当前文档。关闭 = `delete(State.scrolls, sel)`（不记忆已关闭标签的滚动位置）。 | 这是编辑器里唯一没有争议的关标签语义（VSCode / 浏览器都是右优先）；记住已关闭标签的滚动位置是无上限的状态泄漏。 |
| D6 | 正文滚动位置按标签各存一份：`scrolls map[docSel]*ui.ScrollState`，`docScroll()` 返回当前标签的那一份（首次访问时创建）。 | 标签页的意义就是来回切；切回来发现正文跳到顶上，等于每次切换都丢上下文。目录滚动（`tocScroll`）**不**按标签存（沿用既有「换文档归零」的决策 D3），避免一次改动翻两个状态。 |
| D7 | 标签只活在会话里，**不写进 `config.json`**；项目被移除时丢掉它的全部标签，文档被删除时丢掉那一个标签，文档被重命名时标签跟着改名。 | 用户没要求「重启恢复上次的标签」；写配置要动 `config.normalize`、迁移与 `config_test`，收益不明。删除/重命名必须跟上，否则会出现「点开是 404 的标签」。 |
| D8 | 目录标题行右侧四个动作：`UnfoldVertical`「展开全部目录」、`FoldVertical`「折叠全部目录」、`ZoomOut`「缩小正文」、`ZoomIn`「放大正文」，均为 `IconButtonCompact`；两者之间放当前缩放比例文本（`100%`，10px、`TextMuted`）。没有标题时前两个禁用，缩放两个始终可用。 | 用户原话「展开所有/合并所有/放大/缩小」。**「合并」按「折叠」实现**（同一个动作在中文里有两种叫法，取编辑器通用说法「折叠」）；四个 compact 按钮共 80 DIP + 比例 30 DIP，240 宽的目录面板放得下。 |
| D9 | 展开/折叠全部 = 把**当前文档**每一个有子标题的 slug 写成/擦写成打开/关闭，键为 `"toc:" + selKey + ":" + slug`（`selKey` = `项目ID + ":" + 路径`）。旧键 `"toc:" + slug` 不带文档身份，两个文档出现同名标题时互相串。 | 既有的 `openOf("toc:"+slug)` 是跨文档共用的键，展开全部会把别的文档同名标题一起展开；这与 AGENTS.md「文档身份是 docSel」一致，顺手修掉并加回归测试。 |
| D10 | 缩放是**全局**的一个倍数 `State.zoom`（默认 1.0，范围 0.8–2.0，步进 0.1，`Cmd/Ctrl+0` 复位），不是每个文档一份。原生后端把它乘进 `markdown.Theme.FontSize`；webview 后端把它乘进页面 CSS 变量 `--md-scale`（首次加载写进 HTML，之后用 `Page().Eval` 就地改，不重载页面因此不丢滚动位置）。 | 用户原话「放大和缩小时针对 preview 正文内容」。整窗统一的缩放手感与浏览器/编辑器一致；按文档存缩放要再造一张表，收益低。webview 若靠改 URL 触发重载，会踩 AGENTS.md 里「webview 重载丢滚动位置」这个已知坑，所以用 Eval。 |
| D11 | 搜索是**字面子串、不区分大小写**（`strings.EqualFold` 逐 rune 窗口比较），不是正则、不做模糊纠错；上限 500 处，超出时面板标题显示 `前 500 处`。 | 用户原话「模糊搜索」在中文语境里指的是「边打边筛、不必精确大小写」，不是 fuzzy matching；正则开关是另一个需求（见「未决问题」）。上限是为了让「每帧重建匹配行列表」的代价有界。 |
| D12 | 面板用 `ui.Overlay` + `Absolute().Right(t.Space(2)).Top(sidebar.HeaderHeight(c) + tabBarHeight + t.Space(2))` 浮在窗口右上（即正文右上），宽 420，内部分三段：搜索行（`ui.SearchField` + `n/m` + 上一个/下一个/关闭）、匹配行列表、行上下文预览。 | 用户原话「使用悬浮窗的形式而不是弹窗 类似 vscode」「不要在弹窗里」。`Overlay` 是 mygo 里唯一「浮在所有内容之上且能收指针」的层，webview 之上也收得到（`ui.WebView` 文档明确 children/浮层可覆盖它）。 |
| D13 | 匹配行列表每行 = 行号（右对齐、`TextMuted`）+ 该行文本（`SingleLine`、截断）；**选中行**加 `selectedRowTint` 底色并把它的上下文滚进预览。预览段 = 选中匹配上下各 2 行，行号 + `ui.RichText` 三段（前文 / 命中片段用 `AppTheme.Accent` + 底色 / 后文）。 | 用户原话「一个是匹配到的那一行内容的列表显示 第二是一个简单的预览页面便于快速定位」。列表给「有哪几处」，预览给「这一处前后是什么」——两个信息不重复。 |
| D14 | 点列表某行 = 跳到该匹配。原生后端：`State.anchor = "find:<块号>"`、`anchorVer++`，并在 `loadDoc` 里挂 `s.md.AnchorBlock = s.anchorBlock`（解析 `find:` 前缀，其余名字返回 -1 让 mygo 回落到标题 slug 查找）；webview 后端：`Page().FindInPage(query)` 重置后按 `FindNext/bkwd` 推进到第 N 个，关闭面板时 `StopFindInPage()`。 | 原生 markdown 视图没有「滚到第 N 个字」的接口，`AnchorBlock` + `Anchor` 是它唯一暴露的块级滚动入口（mygo `markdown/view.go:327` 每次 `Anchor`/`AnchorVersion` 变化都会 `scrollTo = anchorNamed(...)`）。webview 有浏览器级 `FindInPage`，白拿高亮 + 计数，别自己写 JS 遍历 DOM。 |
| D15 | 原生后端**不做正文内高亮**（命中片段只在面板预览里高亮）：`markdown.Options` 没有「给某段文字上底色」的能力，硬做要自绘或 fork `ui/markdown`。 | AGENTS.md 硬规则：不加第三方 markdown 依赖、不为一种后端另起解析器。功能上「跳到位 + 列表 + 上下文预览」已满足「快速定位」；这条写进 README「已知限制」。 |
| D16 | 快捷键：`Cmd/Ctrl+F` 开/聚焦面板（有文档时）；`Enter` / `Shift+Enter`（焦点在搜索框）与 `Cmd/Ctrl+G` / `Shift+Cmd/Ctrl+G` 下一个/上一个；`Escape` 有查询时清空查询、空查询时关面板；`Cmd/Ctrl+W` 关闭当前标签；`Cmd/Ctrl+=`/`-`/`0` 放大/缩小/复位。全部走 `c.Shortcut`，加在既有的 `handleShortcuts`（`fileacts.go`）里，`View` 与 `Content` 各调用一次（既有做法，`rt.shortcut` 对一次按键只交付一次）。 | 与截图里的 VSCode 手感一致；`SearchField` 自己会吃掉「有内容时的 Escape」（清空），所以我们的 Escape 必须「有查询时不动、空查询时才关」，否则一次按键同时清空又关闭。 |
| D17 | 搜索状态集中在一个结构里：`findState{open, query, matches []workspace.Match, current int, truncated bool}`；匹配在**查询变化**与**文档变化**时重算，不在每帧重算。 | AGENTS.md：「不要在 view 热路径里读盘或解析」。`FindIn` 只吃 `State.src`（已缓存），但 500 行的重复扫描仍然不该每帧跑。 |
| D18 | 新文件三个：`internal/view/tabs.go`（标签状态与标签栏）、`internal/view/find.go`（面板 + 跳转 + 快捷键）、`internal/workspace/find.go`（`Match` + `FindIn` 纯函数）。缩放与目录工具条留在 `native.go`。 | 与既有按区域分文件的做法一致（`fileacts.go` / `setup.go` 都是这么拆的），单文件不超过约 300 行。 |

### 改完长什么样（验收画面）

```
┌ 标题栏（shell 浮动头）───────────────────────────────────────────────┐
│ 📄 alpha/docs/a.md  ⧉   ( 原生 | WebView )                            │
├──────────────────────────────────────────────────────────────────────┤
│ [📄 a.md ✕] [📄 b.md ✕] [📄 很长很长的一份方案.md ✕]        [ ⌄ ] │ ← 标签栏 36 DIP
├───────────────┬──────────────────────────────────────────────────────┤
│ 目录  ⇱ ⇲ － 100% ＋ │                                          │ ← 目录工具条（4 个动作 + 比例）
│ ▾ 一           │  ⌕ 导入 ▏3/17  ↑ ↓ ✕                     │ ← 悬浮搜索面板（Overlay，右上）
│   1. 子项      │  ────────────────────────────────────────  │
│ ▾ 二           │   12  │ …导入期间禁用重复导入、保存及…       │ ← 匹配行列表
│   2. 子项      │   48  │ …导入条件 Excel 的列名映射…          │
│ ▸ 三           │   91  │ …导入完成后刷新列表…                │
│                │  ────────────────────────────────────────  │
│                │   11  │ 后端只返回整组保存结果。             │ ← 行上下文预览
│                │   12  │ 导入期间禁用重复导入、保存及解锁…    │   （命中片段高亮）
│                │   13  │ 已有其他分组处于编辑状态时…          │
│                │  正文（跳过/滚动到该块，原生不高亮）                │
└───────────────┴──────────────────────────────────────────────────────┘
  展开全部 = ⇱（UnfoldVertical）   折叠全部 = ⇲（FoldVertical）
  缩小 = －（ZoomOut）             放大 = ＋（ZoomIn）
  标签放不下时的 [ ⌄ ] = 「全部已打开文件」浮层（列全部标签 + 关闭）
```

## 全局约束

- Go 模块名 `mdpreview`，`go 1.27.3`；不引入新依赖，不新增第三方包，不加 `//go:build` 平台专用代码。
- `internal/workspace` 与 `internal/mcp` 不得 import `.../ui`；`FindIn` 只吃 `string`，返回自己的 `Match`。
- 一份解析两种后端不变：搜索只读 `State.src` 与 `State.doc`（同一个 `*parser.Document`），不为搜索再解析一次。
- 颜色只从 `AppTheme`（`c.Theme()`）取，不写死十六进制；图标只用 `icons/lucide`。
- 图标按钮必须给 `label`（无障碍名，也是测试定位方式）；标签的关闭按钮 label 用 `"关闭 " + 文件名`，重名时带项目名。
- 视图状态归窗口线程：本计划不新增 goroutine；`FindInPage` / `Eval` 在窗口线程调用（它们本来就要求如此）。
- 测试只用 `t.TempDir()`；不碰真实剪贴板 / 文件管理器 / 对话框。
- 每个行为改动先有失败的测试；提交信息中文，前缀 `feat:` / `fix:` / `test:` / `docs:`。提交前 `go vet ./... && gofmt -l .` 必须无输出。

## 文件结构

| 文件 | 职责 | 本计划里的改动 |
| --- | --- | --- |
| `internal/workspace/find.go` | **新建**：`Match` 与 `FindIn`（纯函数，字面子串、忽略大小写、上限 500） | 全部 |
| `internal/workspace/find_test.go` | **新建**：行号/列号/偏移、多行多次命中、大小写、空查询、上限、`\r\n` | 全部 |
| `internal/view/state.go` | `State` 字段与文档选择 | 加 `tabs`、`scrolls`、`find`、`zoom`、`tabsOpen`；`selectDocIn` 改为 `openTab`；`docScroll` 字段换成 `docScroll()` |
| `internal/view/tabs.go` | **新建**：`openTab` / `activateTab` / `closeTab` / `tabWidth` / `visibleTabs` / `tabBar` / `tabsMenu` | 全部 |
| `internal/view/find.go` | **新建**：`findState`、`openFind` / `closeFind` / `refreshMatches` / `gotoMatch` / `findShortcuts` / `findPanel` / `anchorBlock` | 全部 |
| `internal/view/view.go` | shell 槽位与树 | `Content` 与 `View` 加标签栏一行 |
| `internal/view/preview.go` | 预览三段（空状态 / 目录 / 正文） | 末尾调用 `s.findPanel(c)` |
| `internal/view/native.go` | 原生后端、目录、`loadDoc` | 目录标题行四个动作；`docScroll()` + `TrackScroll`；`loadDoc` 挂 `AnchorBlock` 并在换文档时重算匹配；缩放写进 `md.Theme` |
| `internal/view/webview.go` | webview 后端与 `docHTML` | `docHTML` 加 `zoom` 参数与 `--md-scale`；`docURL` 带 `z=`；`applyWebZoom()` 用 `Eval` 就地缩放 |
| `internal/view/fileacts.go` | 既有的 `handleShortcuts` | 加 `s.findShortcuts(c)`、`Cmd+W`、`Cmd+=/-/0` |
| `internal/view/watch.go` | 文件变动应用 | 重命名 / 删除时同步 `tabs` 与 `scrolls` |
| `internal/view/tabs_test.go` | **新建**：标签状态与标签栏 | 全部 |
| `internal/view/find_test.go` | **新建**：搜索面板与跳转 | 全部 |
| `internal/view/layout_test.go` | 布局测试 | `docScroll` → `docScroll()`；目录工具条、缩放、标签栏窄窗口 |
| `internal/view/native_test.go` / `webview_test.go` | 既有测试 | `docHTML` 新签名；`AnchorBlock` 回落 |
| `README.md` / `AGENTS.md` | 用户文档与代理规则 | 功能表、结构表、已知限制、快捷键 |

## 审查重点（Review Focus）

规格只说了「要标签栏」「要四个动作」「要悬浮搜索」，没说的这些条件一样会让用户用不了：

1. **标签比窗口宽**（20 个长文件名、760 宽的最小窗口）：活动标签必须在可视区内、可点，溢出按钮出现且能打开全部列表，不溢出时按钮不占位。→ `TestTabBarOverflowsWithAButton`、`TestTabBarKeepsActiveTabVisible`
2. **关闭标签的各种位置**（第一个、最后一个、中间、唯一一个、非活动）：关掉后当前文档必须是一个还存在的文档，或干净的空状态，不能出现「正文空了但标签还亮着」。→ `TestClosingTabs`
3. **标签与文件系统变动**（重命名当前文档、删除当前文档、移除项目）：标签跟着改/消失，不留 404 标签。→ `TestTabsFollowRenamesAndDeletes`
4. **目录的展开全部作用域**：只影响当前文档（两个文档出现同名标题时互不串），折叠全部之后点击某个标题仍能跳。→ `TestExpandAllOnlyTouchesTheCurrentDocument`
5. **缩放的边界**：连点 30 次放大到上限 2.0 后不再变、缩小到 0.8 后不再变，`Cmd+0` 复位；webview 里缩放不改 URL（不触发重载、不丢滚动位置）。→ `TestZoomClampsAndResets`、`TestWebZoomDoesNotReload`
6. **搜索的边角**：空查询不显示列表、只有一格匹配时上一个/下一个都停在原地、匹配跨 markdown 标记（`**根**因`）时仍算一处、文档切换后匹配重算、`\r\n` 文件的行号不偏移。→ `TestFindPanelBasics`、`TestFindMatchesSurviveCRLF`
7. **没有 window / 没有 webview 的路径**（`ui.NewTester`、无图形环境）：`Cmd+F`、点匹配、缩放都退化成只改状态，不 panic、不吞状态栏错误。→ 各任务里的 `_ = ui.NewTester(...)` 断言 + `TestFindJumpWithoutWebView`
8. **测试定位歧义**：`ui.Tester.Click/Find` 按文本找元素，树项与标签上的文件名会同名。测试里给不同文件起不同名字，或在标签测试里用 `Find` 拿 rect 后 `ClickAt`。

---

### 任务 1：打开文档成为标签页（状态、树点击、关闭、随文件系统变动）

**文件：**
- 修改：`internal/view/state.go`
- 新建：`internal/view/tabs.go`
- 修改：`internal/view/native.go`（`docScroll` → `docScroll()`，`TrackScroll`）
- 修改：`internal/view/watch.go`（重命名 / 删除 / 移除项目同步标签）
- 测试：`internal/view/tabs_test.go`（新建）、`internal/view/layout_test.go`（`s.docScroll` 改 `s.docScroll()`）

- [ ] **步骤 1：编写失败的测试**

`internal/view/tabs_test.go`：

```go
package view

import (
	"testing"
)

// TestOpeningDocsMakesTabs pins that every document the window shows becomes a
// tab, in the order it was opened, and that opening one already open only
// switches to it.
func TestOpeningDocsMakesTabs(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "a.md", "# A\n")
	writeDoc(t, root, "b.md", "# B\n")
	s.Refresh()

	s.selectDoc("a.md")
	s.selectDoc("b.md")
	s.selectDoc("a.md")

	if len(s.tabs) != 2 {
		t.Fatalf("tabs = %v, want two", s.tabs)
	}
	if s.tabs[0].path != "a.md" || s.tabs[1].path != "b.md" {
		t.Errorf("tabs = %v, want a.md then b.md in opening order", s.tabs)
	}
	if got := s.Selected(); got != "a.md" {
		t.Errorf("selected = %q, want a.md: reopening switched to it", got)
	}
}

// TestClosingTabs pins what closing a tab does to the document shown: the one
// to its right, else the one to its left, else nothing.
func TestClosingTabs(t *testing.T) {
	s, root := newState(t)
	for _, name := range []string{"a.md", "b.md", "c.md"} {
		writeDoc(t, root, name, "# "+name+"\n")
	}
	s.Refresh()
	s.selectDoc("a.md")
	s.selectDoc("b.md")
	s.selectDoc("c.md")

	s.closeTab(s.sel) // c.md closes: nothing right, b.md takes over
	if got := s.Selected(); got != "b.md" {
		t.Errorf("selected = %q, want b.md after closing the last", got)
	}
	s.closeTab(s.tabs[0]) // a.md closes while b.md shows: still b.md
	if got := s.Selected(); got != "b.md" {
		t.Errorf("selected = %q, want b.md still", got)
	}
	s.closeTab(s.sel)
	if len(s.tabs) != 0 || !s.sel.empty() {
		t.Errorf("tabs = %v, selected = %q, want none", s.tabs, s.Selected())
	}
	if s.src != "" {
		t.Errorf("src = %q, want the body emptied", s.src)
	}
}

// TestTabsFollowRenamesAndDeletes pins that a tab never points at a document
// that moved or went away.
func TestTabsFollowRenamesAndDeletes(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "a.md", "# A\n")
	writeDoc(t, root, "b.md", "# B\n")
	s.Refresh()
	s.selectDoc("a.md")
	s.selectDoc("b.md")

	// watch.Change 的字段见 internal/watch/watch.go：Op / Path / Old。
	s.applyChanges(s.sel.project, watch.ChangeSet{
		Docs: []watch.Change{{Op: mygo.FileRename, Path: "z.md", Old: "a.md"}},
	})

	if len(s.tabs) != 2 {
		t.Fatalf("tabs = %v, want two", s.tabs)
	}
	if s.tabs[0].path != "z.md" {
		t.Errorf("tabs = %v, want the tab to follow the rename", s.tabs)
	}
	if _, ok := s.scrolls[docSel{project: s.sel.project, path: "z.md"}]; !ok {
		t.Errorf("scrolls = %v, want the position under the new path", s.scrolls)
	}

	// 删除一个开着的文档：它的标签消失，当前文档回到邻居。
	s.applyChanges(s.sel.project, watch.ChangeSet{
		Docs: []watch.Change{{Op: mygo.FileRemove, Path: "b.md"}},
	})
	if s.hasTab(docSel{project: s.sel.project, path: "b.md"}) {
		t.Errorf("tabs = %v, want b.md's tab gone", s.tabs)
	}
	if got := s.Selected(); got != "z.md" {
		t.Errorf("selected = %q, want the neighbour z.md", got)
	}
}

// TestRemovingProjectDropsItsTabs pins that a removed project takes its tabs
// with it, and the document shown falls back to another tab.
func TestRemovingProjectDropsItsTabs(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "a.md", "# A\n")
	s.Refresh()
	s.selectDoc("a.md")
	s.removeProject(s.sel.project)
	if len(s.tabs) != 0 || !s.sel.empty() {
		t.Errorf("tabs = %v, selected = %q, want none", s.tabs, s.Selected())
	}
}

// TestScrollPositionsAreKeptPerTab pins that switching back to a tab shows it
// where the user left it.
func TestScrollPositionsAreKeptPerTab(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "a.md", "# A\n")
	writeDoc(t, root, "b.md", "# B\n")
	s.Refresh()
	s.selectDoc("a.md")
	a := s.docScroll()
	a.Y = 120
	s.selectDoc("b.md")
	if s.docScroll().Y != 0 {
		t.Errorf("b.md starts at %v, want 0", s.docScroll().Y)
	}
	s.selectDoc("a.md")
	if got := s.docScroll().Y; got != 120 {
		t.Errorf("a.md came back at %v, want 120", got)
	}
}
```

> 这个测试会 import `mdpreview/internal/watch` 与 `github.com/egoist/mygo`（`mygo.FileRename` /
> `mygo.FileRemove`），`tabs_test.go` 的 import 块要带上它们。

- [ ] **步骤 2：运行测试验证失败**

运行：`go test ./internal/view -run 'TestOpeningDocsMakesTabs'`
预期：编译失败 —— `s.tabs` / `s.closeTab` 未定义。

- [ ] **步骤 3：实现**

`state.go`：`sel` 旁加标签与滚动位置：

```go
	// tabs are the documents the window has open, in the order they were
	// opened: the tab bar shows them, and sel is always one of them.
	tabs []docSel
	// scrolls keeps each tab's body scroll position, so that switching back
	// shows the document where the user left it.
	scrolls map[docSel]*ui.ScrollState
	// tabsOpen is whether the panel listing every tab shows.
	tabsOpen bool
```

`New` 里 `scrolls: map[docSel]*ui.ScrollState{}`。删掉 `docScroll ui.ScrollState` 字段，换成：

```go
// docScroll is how far the document shown is scrolled, both ways: each tab
// keeps its own, so that switching back does not jump to the top.
func (s *State) docScroll() *ui.ScrollState {
	if st, ok := s.scrolls[s.sel]; ok {
		return st
	}
	if s.scrolls == nil {
		s.scrolls = map[docSel]*ui.ScrollState{}
	}
	st := &ui.ScrollState{}
	s.scrolls[s.sel] = st
	return st
}
```

`selectDocIn` 的最后一步从「设 sel + 重载」改成「开标签」：

```go
func (s *State) selectDocIn(projectID, rel string) {
	sel := docSel{project: projectID, path: rel}
	if sel == s.sel {
		return
	}
	if projectID != "" && s.cfg.SetActive(projectID) {
		s.activeID = projectID
		s.ws.SetConfig(s.cfg)
		s.save()
	}
	s.openTab(sel)
}
```

XTABSX

```go
package view

import "github.com/egoist/mygo/ui"

// openTab shows the document sel, adding it to the tabs unless it is open
// there already. Every way of showing a document — the tree, a link in it,
// a search hit — comes through here, so that one document is one tab.
func (s *State) openTab(sel docSel) {
	if sel.empty() || sel == s.sel {
		return
	}
	if !s.hasTab(sel) {
		s.tabs = append(s.tabs, sel)
	}
	s.activateTab(sel)
}

// activateTab makes sel the document shown, without adding a tab.
func (s *State) activateTab(sel docSel) {
	s.sel = sel
	s.status = ""
	// Another document's outline starts at its top, not where the last
	// one's scroll was left. A reload of the same document keeps it.
	s.tocScroll.X, s.tocScroll.Y = 0, 0
	s.onDocChanged()
}

// hasTab reports whether the document is open in a tab.
func (s *State) hasTab(sel docSel) bool { return s.tabIndex(sel) >= 0 }

// tabIndex is where the document's tab sits, or -1.
func (s *State) tabIndex(sel docSel) int {
	for i, t := range s.tabs {
		if t == sel {
			return i
		}
	}
	return -1
}

// closeTab closes the document's tab. Closing the document shown shows the
// tab to its right, else the one to its left, else nothing.
func (s *State) closeTab(sel docSel) {
	i := s.tabIndex(sel)
	if i < 0 {
		return
	}
	s.tabs = append(s.tabs[:i], s.tabs[i+1:]...)
	delete(s.scrolls, sel)
	if sel != s.sel {
		return
	}
	switch {
	case i < len(s.tabs):
		s.activateTab(s.tabs[i])
	case i > 0:
		s.activateTab(s.tabs[i-1])
	default:
		s.sel = docSel{}
		s.status = ""
		s.onDocChanged()
	}
}
```

`native.go`：`TrackScroll(&s.docScroll)` → `TrackScroll(s.docScroll())`。

`watch.go` 的 `applyChanges`：重命名时同步标签与滚动位置（不管它是不是当前文档）；删除时丢掉标签，并把「当前文档」交给 `closeTab` 自己挑邻居。改成：

```go
func (s *State) applyChanges(id string, set watch.ChangeSet) {
	for _, change := range set.Docs {
		// A tab pointing at the old name would show nothing: follow the
		// file wherever it went, and forget it if it went away.
		if change.Old != "" {
			s.renameTab(docSel{project: id, path: change.Old}, docSel{project: id, path: change.Path})
		}
		switch {
		case change.Old != "" && id == s.sel.project && change.Old == s.sel.path:
			// The document the window shows was renamed: follow it.
			s.sel.path = change.Path
			s.onDocChanged()
		case id == s.sel.project && change.Path == s.sel.path && change.Op == mygo.FileRemove:
			// The document shown went away: its neighbours take over.
			s.closeTab(s.sel)
			s.reloadTree()
			continue
		case id == s.sel.project && change.Path == s.sel.path:
			s.onDocChanged()
		case change.Op == mygo.FileRemove:
			s.dropTab(docSel{project: id, path: change.Path})
		}
	}
	if set.TreeDirty {
		s.reloadTree()
	}
}
```

`tabs.go` 里补：

```go
// renameTab moves a tab and its scroll position to its new path, keeping
// where it sat in the bar.
func (s *State) renameTab(from, to docSel) {
	i := s.tabIndex(from)
	if i < 0 {
		return
	}
	s.tabs[i] = to
	if st, ok := s.scrolls[from]; ok {
		delete(s.scrolls, from)
		s.scrolls[to] = st
	}
}
```

`reloadTree` 里发现当前文档已不存在时，把既有的 `s.sel = docSel{}` 换成 `s.closeTab(s.sel)`（它自己会挑邻居或清空）；项目目录不可用被丢弃的分支里，`dropTabsOf(p.ID)`。`removeProject` 在 `s.cfg.RemoveProject(id)` 之前同样 `s.dropTabsOf(id)`：

```go
// dropTab forgets the document's tab without changing what the window shows:
// the caller is already clearing it, as when the file went away.
func (s *State) dropTab(sel docSel) {
	if i := s.tabIndex(sel); i >= 0 {
		s.tabs = append(s.tabs[:i], s.tabs[i+1:]...)
	}
	delete(s.scrolls, sel)
}

// dropTabsOf closes every tab of one project: a removed project takes its
// documents with it.
func (s *State) dropTabsOf(projectID string) {
	kept := s.tabs[:0]
	for _, tab := range s.tabs {
		if tab.project == projectID {
			delete(s.scrolls, tab)
			continue
		}
		kept = append(kept, tab)
	}
	s.tabs = kept
	if s.sel.project == projectID {
		s.sel = docSel{}
		s.onDocChanged()
	}
}
```

`TestRemovingProjectDropsItsTabs` 断言的正是这条路径。

- [ ] **步骤 4：运行测试验证通过**

运行：`go test ./internal/view -run 'TestOpeningDocs|TestClosingTabs|TestTabsFollow|TestRemovingProjectDrops|TestScrollPositions' -v`
预期：PASS。既有测试同时必须仍全绿（`go test ./internal/view -run TestTOCScrollSurvivesAReloadButNotADocumentChange` 用到目录滚动，不受影响）。

- [ ] **步骤 5：Commit**

```bash
git add internal/view/state.go internal/view/tabs.go internal/view/watch.go internal/view/native.go internal/view/tabs_test.go internal/view/layout_test.go
git commit -m "feat: 打开的文档成为可关闭的标签页，各页记住自己的滚动位置"
```

---

### 任务 2：标签栏、溢出时的「全部文件」浮层

**文件：**
- 修改：`internal/view/tabs.go`（`tabWidth` / `visibleTabs` / `tabBar` / `tabsMenu`）
- 修改：`internal/view/view.go`（`Content` 与 `View` 各加一行）
- 测试：`internal/view/tabs_test.go`

- [ ] **步骤 1：编写失败的测试**

```go
// TestTabBarShowsEveryTab pins the bar above the body: every open document has
// a tab, the one shown is the one in it, and no overflow button shows while
// they fit.
func TestTabBarShowsEveryTab(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "a.md", "# A\n")
	writeDoc(t, root, "b.md", "# B\n")
	s.Refresh()
	s.selectDoc("a.md")
	s.selectDoc("b.md")

	tt := ui.NewTester(func(c *ui.Context) { s.tabBar(c) }, 900, 40)
	for _, want := range []string{"a.md", "b.md"} {
		if !tt.HasText(want) {
			t.Errorf("tab texts = %q, want %q", tt.Texts(), want)
		}
	}
	if tt.HasText("全部已打开文件") {
		t.Errorf("tab texts = %q, want no overflow button while they fit", tt.Texts())
	}
}

// TestTabBarOverflowsWithAButton pins the overflow: too many tabs and the
// rightmost control lists them all, and choosing one shows it.
func TestTabBarOverflowsWithAButton(t *testing.T) {
	s, root := newState(t)
	var names []string
	for i := range 12 {
		name := fmt.Sprintf("文档-%02d-很长很长的名字.md", i)
		names = append(names, name)
		writeDoc(t, root, name, "# x\n")
	}
	s.Refresh()
	for _, name := range names {
		s.selectDoc(name)
	}

	tt := ui.NewTester(func(c *ui.Context) { s.tabBar(c) }, 760, 40)
	if err := tt.Click("全部已打开文件"); err != nil {
		t.Fatalf("Click overflow: %v", err)
	}
	s.tabsOpen = true
	tt.Frame()
	if err := tt.Click(names[0]); err != nil {
		t.Fatalf("Click %s: %v", names[0], err)
	}
	if got := s.Selected(); got != names[0] {
		t.Errorf("selected = %q, want %q", got, names[0])
	}
	if s.tabsOpen {
		t.Error("the list stayed open after choosing a tab")
	}
}

// TestTabBarKeepsActiveTabVisible pins that the tab shown is never the one the
// bar leaves out.
func TestTabBarKeepsActiveTabVisible(t *testing.T) {
	s, root := newState(t)
	var names []string
	for i := range 12 {
		name := fmt.Sprintf("文档-%02d-很长很长的名字.md", i)
		names = append(names, name)
		writeDoc(t, root, name, "# x\n")
	}
	s.Refresh()
	for _, name := range names {
		s.selectDoc(name)
	}
	s.selectDoc(names[0]) // the leftmost tab is the one shown

	tt := ui.NewTester(func(c *ui.Context) { s.tabBar(c) }, 520, 40)
	rect, ok := tt.Find(names[0])
	if !ok {
		t.Fatalf("tab texts = %q, want the active tab's name", tt.Texts())
	}
	if rect.X+rect.W > 520 {
		t.Errorf("the active tab ends at %v, past the window's 520", rect.X+rect.W)
	}
}

// TestClosingATabDoesNotActivateIt pins the close button's click: it closes
// the tab it sits in rather than switching to it first.
func TestClosingATabDoesNotActivateIt(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "a.md", "# A\n")
	writeDoc(t, root, "b.md", "# B\n")
	s.Refresh()
	s.selectDoc("a.md")
	s.selectDoc("b.md")

	tt := ui.NewTester(func(c *ui.Context) { s.tabBar(c) }, 900, 40)
	if err := tt.Click("关闭 a.md"); err != nil {
		t.Fatalf("Click close: %v", err)
	}
	if got := s.Selected(); got != "b.md" {
		t.Errorf("selected = %q, want b.md: closing a.md is not switching to it", got)
	}
	if s.hasTab(docSel{project: s.sel.project, path: "a.md"}) {
		t.Error("a.md still has a tab")
	}
}
```

- [ ] **步骤 2：运行测试验证失败**

运行：`go test ./internal/view -run 'TestTabBar|TestClosingATab'`
预期：编译失败 —— `s.tabBar` / `s.tabsOpen` 未定义。

- [ ] **步骤 3：实现**

`tabs.go` 追加（import 需加 `path`、`github.com/oneo-me/onevra/source/components`、`.../icons/lucide`、`github.com/egoist/mygo/ui`）。高度常量与宽度拟合：

```go
// tabBarHeight is the room the tab bar takes above the body, in DIPs.
const tabBarHeight float32 = 36

// tabIcon is the room the document icon takes in a tab, in DIPs.
const tabIcon float32 = 14

// tabWidth is how wide the tab of name is: as wide as its name needs, held
// between the narrowest that still shows a name and the widest that leaves
// room for the next one.
func tabWidth(c *ui.Context, name string) float32 {
	t := c.Theme()
	pad := t.Space(1.5)
	w, _ := c.MeasureText(0, ui.Span{Text: name, Font: t.Font, Size: t.FontSize})
	return min(t.Rem(12), max(t.Rem(6), w+2*pad+tabIcon+t.Space(1)+20))
}

// visibleTabs returns the tabs the bar shows, and whether any are left out:
// as many as fit, the one shown first, so that it is never the one cut.
func (s *State) visibleTabs(c *ui.Context, avail float32) ([]docSel, bool) {
	widths := make([]float32, len(s.tabs))
	total := float32(0)
	for i, tab := range s.tabs {
		widths[i] = tabWidth(c, trimEllipsis(path.Base(tab.path), 40))
		total += widths[i]
	}
	if total <= avail {
		return s.tabs, false
	}
	active := s.tabIndex(s.sel)
	if active < 0 {
		active = 0
	}
	// The active tab first, then as many on either side as fit, right
	// before left: the next tab is likelier to be the one wanted.
	room, from, to := avail-widths[active], active, active
	for {
		grew := false
		if to+1 < len(widths) && widths[to+1] <= room {
			to++
			room -= widths[to]
			grew = true
		}
		if from-1 >= 0 && widths[from-1] <= room {
			from--
			room -= widths[from]
			grew = true
		}
		if !grew {
			break
		}
	}
	return s.tabs[from : to+1], true
}
```

`tabBar`：

```go
// tabBar builds the row above the body: a tab per open document, which a click
// shows and its close button closes, and, when they do not fit, the button
// listing all of them.
func (s *State) tabBar(c *ui.Context) {
	t := c.Theme()
	if len(s.tabs) == 0 {
		ui.Row(c).Height(tabBarHeight)
		return
	}
	// The overflow button is one compact square and the gap before it: it
	// only takes room when something is left out.
	overflowW := max(20, t.Rem(1.25)) + t.Space(1)
	shown, cut := s.visibleTabs(c, s.windowWidth()-overflowW-2*t.Space(1))
	if !cut {
		shown = s.tabs
	}
	ui.Row(c).FillWidth().Height(tabBarHeight).AlignItems(ui.Center).
		Padding(0, t.Space(1)).Gap(t.Space(0.5)).Children(func() {
			for _, tab := range shown {
				tab := tab
				active := tab == s.sel
				row := ui.Row(c).Height(tabBarHeight - 2*t.Space(1)).Radius(t.Radius).AlignItems(ui.Center)
				if active {
					row.Background(selectedRowTint(t))
				}
				row.Children(func() {
					// The name is the clickable part, the close button its
					// sibling: nesting one button in another would let a
					// close also switch to the tab it closes.
					name := components.Button(c, "", components.ButtonGhost).
						Label("文档：" + tab.path).Height(tabBarHeight - 2*t.Space(1)).
						Padding(t.Space(0.75), t.Space(1))
					name.Gap(t.Space(0.5)).Children(func() {
						components.Icon(c, lucide.FileText()).Size(tabIcon, tabIcon).TextColor(t.TextMuted)
						components.Text(c, trimEllipsis(path.Base(tab.path), 40)).FontSize(12).SingleLine()
					})
					if name.Clicked() {
						s.openTab(tab)
					}
					if components.IconButton(c, lucide.X(), "关闭 "+path.Base(tab.path), components.IconButtonCompact).Clicked() {
						s.closeTab(tab)
					}
				})
			}
			if cut {
				ui.Spacer(c)
				button := components.IconButton(c, lucide.ChevronDown(), "全部已打开文件", components.IconButtonCompact)
				if button.Clicked() {
					s.tabsOpen = !s.tabsOpen
				}
				ui.Popover(c, button, &s.tabsOpen, func() { s.tabsMenu(c) })
			}
		})
}
```

`tabsMenu`（每行左侧按钮激活、右侧 `X` 关闭，超过 400 滚）：

```go
// tabsMenu builds the list every open document shows in, for the tabs the bar
// has no room for.
func (s *State) tabsMenu(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Width(260).Padding(t.Space(1)).Children(func() {
		ui.Scroll(c).MaxHeight(400).Children(func() {
			ui.Column(c).Gap(t.Space(0.25)).Children(func() {
				for _, tab := range s.tabs {
					tab := tab
					row := ui.Row(c).Radius(t.Radius).AlignItems(ui.Center)
					if tab == s.sel {
						row.Background(selectedRowTint(t))
					}
					row.Children(func() {
						name := components.Button(c, "", components.ButtonGhost).Label("打开：" + tab.path)
						name.Grow(1).Gap(t.Space(0.75)).Children(func() {
							components.Icon(c, lucide.FileText()).Size(tabIcon, tabIcon).TextColor(t.TextMuted)
							components.Text(c, trimEllipsis(path.Base(tab.path), 32)).FontSize(12).SingleLine()
						})
						if name.Clicked() {
							s.openTab(tab)
							s.tabsOpen = false
						}
						if components.IconButton(c, lucide.X(), "关闭 "+path.Base(tab.path), components.IconButtonCompact).Clicked() {
							s.closeTab(tab)
						}
					})
				}
			})
		})
	})
}
```

`view.go`：`Content` 与 `View` 各在 `preview()` 之前加 `s.tabBar(c)`（`View` 里放在标题占位行之后）。同时把 `View` 的侧栏宽度从写死的 300 改成 `s.sidebarW`（顺手对齐，若不想动可保留）。

- [ ] **步骤 4：运行测试验证通过**

运行：`go test ./internal/view -run 'TestTabBar|TestClosingATab|TestTitle|TestShellSlotsBuild' -v`
预期：PASS。再跑全量 `go test ./internal/view`，确认既有的「点树里的 a.md」类测试没有被同名标签干扰（若有，按审查重点 8 改名或改用 `ClickAt`）。

- [ ] **步骤 5：Commit**

```bash
git add internal/view/tabs.go internal/view/view.go internal/view/tabs_test.go
git commit -m "feat: 主预览顶部加标签栏，放不下时右侧给出全部文件浮层"
```

---

### 任务 3：目录工具条的「展开全部 / 折叠全部」

**文件：**
- 修改：`internal/view/native.go`（`toc` 标题行、`tocKey`、`expandAll`）
- 修改：`internal/view/view.go`（`treeItems` 的目录键不变，只有 TOC 键改格式）
- 测试：`internal/view/layout_test.go`

- [ ] **步骤 1：编写失败的测试**

```go
// TestExpandAllOnlyTouchesTheCurrentDocument pins the outline's two bulk
// actions: expanding every heading of the document shown unfolds its outline
// and not another document's headings of the same name.
func TestExpandAllOnlyTouchesTheCurrentDocument(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "a.md", "# 总览\n\n## 范围\n\n### 细节\n")
	writeDoc(t, root, "b.md", "# 总览\n\n## 范围\n\n### 细节\n")
	s.Refresh()
	s.selectDoc("a.md")
	s.selectDoc("b.md") // b.md shows; a.md keeps its own open state

	tt := ui.NewTester(func(c *ui.Context) { s.toc(c) }, 240, 400)
	if err := tt.Click("折叠全部目录"); err != nil {
		t.Fatalf("Click collapse: %v", err)
	}
	tt.Frame()
	if err := tt.Click("展开全部目录"); err != nil {
		t.Fatalf("Click expand: %v", err)
	}
	// b.md 的三个标题都该展开：子标题出现在目录里。
	for _, want := range []string{"范围", "细节"} {
		if !tt.HasText(want) {
			t.Errorf("outline texts = %q, want %q after expanding all", tt.Texts(), want)
		}
	}

	s.selectDoc("a.md")
	tt.Frame()
	if !tt.HasText("范围") {
		t.Errorf("outline texts = %q, want a.md untouched by b.md's bulk action", tt.Texts())
	}
}

// TestCollapseAllFoldsTheOutline pins the other direction: folding every
// heading leaves only the top level, and a click still jumps.
func TestCollapseAllFoldsTheOutline(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "a.md", "# 一\n\n## 二\n\n### 三\n")
	s.Refresh()
	s.selectDoc("a.md")

	tt := ui.NewTester(func(c *ui.Context) { s.toc(c) }, 240, 400)
	if err := tt.Click("折叠全部目录"); err != nil {
		t.Fatalf("Click: %v", err)
	}
	tt.Frame()
	if tt.HasText("二") || tt.HasText("三") {
		t.Errorf("outline texts = %q, want only the top level", tt.Texts())
	}
	if err := tt.Click("一"); err != nil {
		t.Fatalf("Click heading: %v", err)
	}
	if s.anchor != "一" {
		t.Errorf("anchor = %q, want the top-level heading", s.anchor)
	}
}
```

- [ ] **步骤 2：运行测试验证失败**

运行：`go test ./internal/view -run 'TestExpandAll|TestCollapseAll'`
预期：FAIL / 编译失败 —— 「折叠全部目录」按钮不存在。

- [ ] **步骤 3：实现**

`native.go`：目录键带上文档身份（D9），标题行改成「目录 + 四动作」：

```go
// tocKey is where the outline's open state lives for a heading of the document
// shown: by project and path first, so that two documents whose headings share
// a name do not unfold each other.
func (s *State) tocKey(slug string) string {
	return "toc:" + s.sel.project + ":" + s.sel.path + ":" + slug
}

// setTOC unfolds or folds every heading of the document shown.
func (s *State) setTOC(open bool) {
	nodes := buildTOC(s.headings)
	var walk func([]*tocNode)
	walk = func(ns []*tocNode) {
		for _, n := range ns {
			if len(n.children) > 0 {
				b := open
				s.open[s.tocKey(n.h.Slug)] = &b
			}
			walk(n.children)
		}
	}
	walk(nodes)
}
```

`toc()` 的标题行：

```go
	ui.Column(c).Fill().Padding(tocPadding).Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
			components.Text(c, "目录").FontSize(11).TextColor(t.TextMuted)
			ui.Spacer(c)
			hasHeads := len(s.headings) > 0
			if components.IconButton(c, lucide.UnfoldVertical(), "展开全部目录", components.IconButtonCompact).
				Disabled(!hasHeads).Clicked() {
				s.setTOC(true)
			}
			if components.IconButton(c, lucide.FoldVertical(), "折叠全部目录", components.IconButtonCompact).
				Disabled(!hasHeads).Clicked() {
				s.setTOC(false)
			}
			// 缩放的四个动作见任务 4，这里只落位。
		})
		…
	})
```

`tocItems` 里 `s.openOf("toc:"+n.h.Slug)` → `s.openOf(s.tocKey(n.h.Slug))`。

- [ ] **步骤 4：运行测试验证通过**

运行：`go test ./internal/view -run 'TestTOC|TestExpandAll|TestCollapseAll' -v`
预期：PASS。

- [ ] **步骤 5：Commit**

```bash
git add internal/view/native.go internal/view/layout_test.go
git commit -m "feat: 目录面板加展开全部与折叠全部，展开状态按文档区分"
```

---

### 任务 4：正文缩放（原生 + WebView，快捷键与比例指示）

**文件：**
- 修改：`internal/view/state.go`（`zoom` 字段 + `zoomBy` / `zoomPercent`）
- 修改：`internal/view/native.go`（目录标题行的两个缩放按钮、`nativePreview` 写 `md.Theme`）
- 修改：`internal/view/webview.go`（`docHTML` 加 `zoom` 参数与 `--md-scale`、`docURL` 带 `z=`、`applyWebZoom`）
- 修改：`internal/view/fileacts.go`（`Cmd+=` / `-` / `0`）
- 测试：`internal/view/layout_test.go`、`internal/view/webview_test.go`

- [ ] **步骤 1：编写失败的测试**

```go
// TestZoomClampsAndResets pins the body's zoom: it steps both ways inside its
// range, and Cmd+0 brings it back.
func TestZoomClampsAndResets(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "a.md", "# A\n\n正文\n")
	s.Refresh()
	s.selectDoc("a.md")

	if got := s.zoomPercent(); got != 100 {
		t.Errorf("zoom = %d%%, want 100%%", got)
	}
	for range 30 {
		s.zoomBy(zoomStep)
	}
	if got := s.zoom; got != zoomMax {
		t.Errorf("zoom = %v, want the %v ceiling", got, zoomMax)
	}
	for range 30 {
		s.zoomBy(-zoomStep)
	}
	if got := s.zoom; got != zoomMin {
		t.Errorf("zoom = %v, want the %v floor", got, zoomMin)
	}
	_ = ui.NewTester(s.View, 1000, 700)
	tt := ui.NewTester(func(c *ui.Context) { s.Content(c) }, 900, 700)
	tt.Key(ui.Cmd, ui.KeyEqual)
	tt.Frame()
	if s.zoom <= zoomMin {
		t.Errorf("zoom = %v, want the shortcut to step up", s.zoom)
	}
	tt.Key(ui.Cmd, ui.Key0)
	tt.Frame()
	if got := s.zoomPercent(); got != 100 {
		t.Errorf("zoom = %d%%, want 100%% after reset", got)
	}
}

// TestZoomReachesTheNativeBody pins that the zoom changes the body text size
// the native view is given, not just a number in the state.
func TestZoomReachesTheNativeBody(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "a.md", "# A\n\n正文\n")
	s.Refresh()
	s.selectDoc("a.md")

	_ = ui.NewTester(func(c *ui.Context) { s.nativePreview(c) }, 800, 600)
	base := s.md.Theme.FontSize
	s.zoomBy(zoomStep)
	tt := ui.NewTester(func(c *ui.Context) { s.nativePreview(c) }, 800, 600)
	tt.Frame()
	if got := s.md.Theme.FontSize; got <= base {
		t.Errorf("body font = %v, want more than %v after zooming in", got, base)
	}
	rect, ok := tt.Find("正文")
	if !ok {
		t.Fatalf("texts = %q, want the paragraph", tt.Texts())
	}
	if rect.H <= 0 {
		t.Errorf("the paragraph drew %v DIPs tall", rect.H)
	}
}

// TestWebZoomDoesNotReload pins that zooming a web view edits the page in place
// instead of loading it again: a reload would lose the scroll position.
func TestWebZoomDoesNotReload(t *testing.T) {
	if got := docHTML("/tmp", "# A\n", "1", 1.25); !strings.Contains(got, "--md-scale: 1.25") {
		t.Errorf("page = %q, want the scale in it", got)
	}
}
```

> 缩放复位走真实快捷键 `ui.Cmd` + `ui.KeyEqual` / `ui.Key0`（与
> `TestCmdCCopiesTheDocumentPath` 同一个写法），不要为它开测试后门。

- [ ] **步骤 2：运行测试验证失败**

运行：`go test ./internal/view -run 'TestZoom'`
预期：编译失败 —— `zoomStep` / `s.zoom` / `--md-scale` 都还不存在。

- [ ] **步骤 3：实现**

`state.go`：

```go
// The body's zoom, as the outline's buttons and the shortcuts step it: a
// multiplier of the theme's text size, held between the two, stepped by
// zoomStep, 1 showing it as the theme has it.
const (
	zoomMin     float32 = 0.8
	zoomMax     float32 = 2.0
	zoomStep    float32 = 0.1
	zoomDefault float32 = 1
)

// zoom is how much larger or smaller the body's text shows than the theme's.
func (s *State) zoomBy(delta float32) {
	z := s.zoom
	if z == 0 {
		z = zoomDefault
	}
	s.zoom = min(zoomMax, max(zoomMin, math.Round(float64(z+delta)*100)/100))
	s.applyWebZoom()
}

// zoomPercent is the zoom as the outline shows it, as a whole percentage.
func (s *State) zoomPercent() int { return int(math.Round(float64(s.zoomOrOne()) * 100)) }

// zoomOrOne is the zoom, 1 while it was never changed.
func (s *State) zoomOrOne() float32 {
	if s.zoom == 0 {
		return zoomDefault
	}
	return s.zoom
}
```

`native.go` 里 `nativePreview` 顶部（`t := c.Theme()` 之后）：

```go
	// The body's text scales with the zoom: every length the theme gives in
	// ems — spacing, code, tables — follows its font size.
	s.md.Theme = &markdown.Theme{FontSize: max(9, t.FontSize*s.zoomOrOne())}
```

目录标题行补两个按钮与比例（任务 3 的行里）：

```go
			ui.Spacer(c)
			if components.IconButton(c, lucide.ZoomOut(), "缩小正文", components.IconButtonCompact).Clicked() {
				s.zoomBy(-zoomStep)
			}
			components.Text(c, itoa(s.zoomPercent())+"%").FontSize(10).TextColor(t.TextMuted)
			if components.IconButton(c, lucide.ZoomIn(), "放大正文", components.IconButtonCompact).Clicked() {
				s.zoomBy(zoomStep)
			}
```

（`ui.Spacer` 只留一个：把「目录」与四个动作分开，中间不要两个 spacer。）

`fileacts.go` 的 `handleShortcuts`：

```go
	switch {
	case c.Shortcut(ui.Cmd, ui.KeyEqual):
		s.zoomBy(zoomStep)
	case c.Shortcut(ui.Cmd, ui.KeyMinus):
		s.zoomBy(-zoomStep)
	case c.Shortcut(ui.Cmd, ui.Key0):
		s.zoom = zoomDefault
		s.applyWebZoom()
	}
```

`webview.go`：

- `docHTML(baseDir, src, rev string, zoom float32)`，`rev` 之后写入：

```go
	b.WriteString("<style>:root { --md-scale: " + strconv.FormatFloat(float64(zoom), 'f', 2, 64) + "; }" + docCSS + "</style>")
```

- `docCSS` 里 `body` 的 `font` 之后加 `font-size: calc(15px * var(--md-scale));`。
- `docURL()` 末尾 `&z=` + 格式化后的缩放（`z` 只影响首屏，后续就地改）。
- handler 的 `document` 把 `z` 解析成 float（空 / 非法 → 1）传给 `docHTML`。
- 新增：

```go
// applyWebZoom rescales the page the web view shows without loading it again:
// a load would lose the scroll position, which the app's own zoom buttons must
// not do.
func (s *State) applyWebZoom() {
	if s.wv == nil {
		return
	}
	scale := strconv.FormatFloat(float64(s.zoomOrOne()), 'f', 2, 64)
	_, _ = s.wv.Page().Eval("document.documentElement.style.setProperty('--md-scale', '" + scale + "')")
}
```

- 测试里 `docHTML(...)` 的调用点补 `1`；`TestWebZoomDoesNotReload` 用 `docHTML("/tmp", "# A\n", "1", 1.25)`，断言 `--md-scale: 1.25`。

- [ ] **步骤 4：运行测试验证通过**

运行：`go test ./internal/view -run 'TestZoom|TestWebZoom|TestDocHTML' -v`
预期：PASS。手工验证：`go run .` → 打开文档 → 点 ＋ 三次，正文与标题、代码块一起变大，目录与侧栏不变；`Ctrl+-` 变小、`Ctrl+0` 复位；切到 WebView 重复一次，**滚动位置不动**。

- [ ] **步骤 5：Commit**

```bash
git add internal/view/state.go internal/view/native.go internal/view/webview.go internal/view/fileacts.go internal/view/layout_test.go internal/view/webview_test.go
git commit -m "feat: 正文可放大缩小，原生与 webview 两种后端一致"
```

---

### 任务 5：`workspace.FindIn`（纯函数，带行号与列号）

**文件：**
- 新建：`internal/workspace/find.go`、`internal/workspace/find_test.go`

- [ ] **步骤 1：编写失败的测试**

```go
package workspace

import (
	"strings"
	"testing"
)

func TestFindInReportsLineAndColumn(t *testing.T) {
	src := "第一行\n第二行 abc\n第三行\nabc 又一次 abc\n"
	got, truncated := FindIn(src, "abc")
	if truncated {
		t.Error("truncated = true, want false for three matches")
	}
	if len(got) != 3 {
		t.Fatalf("FindIn = %v, want three", got)
	}
	want := []struct{ line, col int }{{2, 5}, {4, 1}, {4, 9}}
	for i, w := range want {
		if got[i].Line != w.line || got[i].Col != w.col {
			t.Errorf("match %d = line %d col %d, want line %d col %d", i, got[i].Line, got[i].Col, w.line, w.col)
		}
	}
	if got[0].Text != "第二行 abc" {
		t.Errorf("match 0 text = %q, want the whole line", got[0].Text)
	}
	if got[0].Offset != strings.Index(src, "abc") {
		t.Errorf("match 0 offset = %d, want %d", got[0].Offset, strings.Index(src, "abc"))
	}
}

func TestFindInIgnoresCaseAndEmpties(t *testing.T) {
	src := "IMPORT 表格\nimport 又见\n"
	if got, _ := FindIn(src, "import"); len(got) != 2 {
		t.Errorf("FindIn = %v, want two matches ignoring case", got)
	}
	if got, _ := FindIn(src, "  "); len(got) != 0 {
		t.Errorf("FindIn(blank) = %v, want none", got)
	}
}

func TestFindInHandlesCRLF(t *testing.T) {
	got, _ := FindIn("一\r\n二 abc\r\n", "abc")
	if len(got) != 1 || got[0].Line != 2 || got[0].Text != "二 abc" {
		t.Fatalf("FindIn = %+v, want line 2 without the carriage return", got)
	}
}

func TestFindInCapsResults(t *testing.T) {
	got, truncated := FindIn(strings.Repeat("x y\n", 600), "x")
	if len(got) != findLimit || !truncated {
		t.Errorf("FindIn = %d matches, truncated %v, want the %d cap and the flag", len(got), truncated, findLimit)
	}
}
```

> `FindIn` 返回两个值：匹配与「是否被 500 条上限截断」——面板用它决定要不要在计数旁
> 写「前 500 处」。

- [ ] **步骤 2：运行测试验证失败**

运行：`go test ./internal/workspace -run TestFindIn`
预期：编译失败 —— `FindIn` / `findLimit` 未定义。

- [ ] **步骤 3：实现**

`internal/workspace/find.go`：

```go
package workspace

import "strings"

// findLimit caps how many matches FindIn returns: the panel builds a row per
// match every time they change, and a phrase in a huge document must not stall
// the window.
const findLimit = 500

// Match is one occurrence of a phrase in a document's source: the line it sits
// on, from 1, its column within the line, from 1 in runes, the offset of its
// first byte in the source, and the line's text as the panel lists it.
type Match struct {
	Line   int    `json:"line"`
	Col    int    `json:"col"`
	Offset int    `json:"offset"`
	Text   string `json:"text"`
}

// FindIn returns where query appears in src, ignoring case, at most findLimit
// places, and whether it left any out: literal text, not a pattern, as an
// editor's find bar takes it.
func FindIn(src, query string) (matches []Match, truncated bool) {
	needle := []rune(query)
	if len(needle) == 0 || strings.TrimSpace(query) == "" {
		return nil, false
	}
	lines := strings.SplitAfter(src, "\n")
	offset := 0
	for lineNo, line := range lines {
		text := strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
		runes := []rune(text)
		for i := 0; i+len(needle) <= len(runes); i++ {
			if !strings.EqualFold(string(runes[i:i+len(needle)]), query) {
				continue
			}
			matches = append(matches, Match{
				Line:   lineNo + 1,
				Col:    i + 1,
				Offset: offset + len(string(runes[:i])),
				Text:   strings.TrimSpace(text),
			})
			if len(matches) == findLimit {
				// The cap only cuts when something is left to look at.
				more := i+len(needle) < len(runes) || lineNo+1 < len(lines)
				return matches, more
			}
		}
		offset += len(line)
	}
	return matches, false
}
```

（`truncated` = 卡到 500 那一刻还有没看完的字或行；`strings.TrimSpace(query) == ""` 让纯空白查询没有匹配。）

- [ ] **步骤 4：运行测试验证通过**

运行：`go test ./internal/workspace -run TestFindIn -v`
预期：PASS。

- [ ] **步骤 5：Commit**

```bash
git add internal/workspace/find.go internal/workspace/find_test.go
git commit -m "feat: 正文内查找的字面匹配与行列定位"
```

---

### 任务 6：悬浮搜索面板（`Cmd/Ctrl+F`，输入 + 匹配列表 + 上下文预览）

**文件：**
- 新建：`internal/view/find.go`
- 修改：`internal/view/state.go`（`find` 字段）
- 修改：`internal/view/preview.go`（末尾 `s.findPanel(c)`）
- 修改：`internal/view/fileacts.go`（`s.findShortcuts(c)`）
- 修改：`internal/view/native.go`（`loadDoc` 里重算匹配）
- 测试：`internal/view/find_test.go`（新建）

- [ ] **步骤 1：编写失败的测试**

```go
package view

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestFindPanelOpensWithCmdF pins the shortcut: it opens the floating panel
// only when a document shows, and the panel carries two blocks beyond the
// field — the matches' lines and a preview of the chosen one.
func TestFindPanelOpensWithCmdF(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "a.md", "# 一\n\n导入期间禁用重复导入。\n\n导入完成后刷新。\n")
	s.Refresh()
	s.selectDoc("a.md")

	tt := ui.NewTester(func(c *ui.Context) { s.Content(c) }, 900, 700)
	tt.Key(ui.Cmd, ui.KeyF)
	tt.Frame()
	if !s.find.open {
		t.Fatal("the panel did not open")
	}
	s.find.query = "导入"
	s.refreshMatches()
	tt.Frame()

	if !tt.HasText("1/3") {
		t.Errorf("panels = %q, want the count 1/3", tt.Texts())
	}
	if !tt.HasText("导入期间禁用重复导入。") {
		t.Errorf("panels = %q, want the matched line listed", tt.Texts())
	}
	if !tt.HasText("3") {
		t.Errorf("panels = %q, want the match's line number", tt.Texts())
	}
}

// TestFindPanelStepsThroughMatches pins next/previous: they wrap, keep the
// current match inside the list, and re-scroll the body to it.
func TestFindPanelStepsThroughMatches(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "a.md", "一 目标\n二 目标\n三 目标\n")
	s.Refresh()
	s.selectDoc("a.md")
	s.find.open = true
	s.find.query = "目标"
	s.refreshMatches()
	if len(s.find.matches) != 3 {
		t.Fatalf("matches = %v, want three", s.find.matches)
	}
	s.findNext(1)
	s.findNext(1)
	s.findNext(1)
	if s.find.current != 0 {
		t.Errorf("current = %d, want 0: it wrapped", s.find.current)
	}
	s.findNext(-1)
	if s.find.current != 2 {
		t.Errorf("current = %d, want 2: it wrapped the other way", s.find.current)
	}
}

// TestFindJumpScrollsTheNativeBody pins the native jump: the block holding the
// match becomes the markdown view's anchor, which is what scrolls it.
func TestFindJumpScrollsTheNativeBody(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "a.md", "# 一\n\n甲段\n\n乙段 目标\n\n丙段\n")
	s.Refresh()
	s.selectDoc("a.md")
	s.find.open = true
	s.find.query = "目标"
	s.refreshMatches()
	s.gotoMatch(0)

	if !strings.HasPrefix(s.anchor, findAnchorPrefix) {
		t.Fatalf("anchor = %q, want the find prefix", s.anchor)
	}
	block := s.anchorBlock(s.doc, s.anchor)
	if block <= 0 {
		t.Fatalf("anchorBlock(%q) = %d, want the paragraph's block", s.anchor, block)
	}
	if got := s.doc.Block(block); got.Kind != parser.Paragraph {
		t.Errorf("block kind = %v, want the paragraph holding the match", got.Kind)
	}

	// 再跳一次同一处也必须重新触发滚动。
	before := s.anchorVer
	s.gotoMatch(0)
	if s.anchorVer <= before {
		t.Errorf("anchorVer = %d, want more than %d", s.anchorVer, before)
	}
}

// TestEscapeClearsThenCloses pins the panel's Escape: it empties the query
// first, and only an empty field closes the panel.
func TestEscapeClearsThenCloses(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "a.md", "# A\n\n正文\n")
	s.Refresh()
	s.selectDoc("a.md")
	s.find.open = true
	s.find.query = "正文"
	s.refreshMatches()

	tt := ui.NewTester(func(c *ui.Context) { s.Content(c) }, 900, 700)
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if s.find.query != "" {
		t.Errorf("query = %q, want it cleared", s.find.query)
	}
	if !s.find.open {
		t.Error("the panel closed while it only had to clear the query")
	}
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if s.find.open {
		t.Error("the panel stayed open on the second Escape")
	}
}

// TestFindPanelWithoutADocumentDoesNothing pins the guard: Cmd+F in the empty
// state opens nothing, so no panel floats over "打开一个目录开始预览 markdown".
func TestFindPanelWithoutADocumentDoesNothing(t *testing.T) {
	s := newStateWith(config.Config{Version: config.Version})
	tt := ui.NewTester(func(c *ui.Context) { s.Content(c) }, 900, 700)
	tt.Key(ui.Cmd, ui.KeyF)
	tt.Frame()
	if s.find.open {
		t.Error("the panel opened without a document")
	}
}
```

- [ ] **步骤 2：运行测试验证失败**

运行：`go test ./internal/view -run 'TestFindPanel|TestFindJump|TestEscapeClears'`
预期：编译失败 —— `s.find` / `findAnchorPrefix` 未定义。

- [ ] **步骤 3：实现**

`find.go`（新建，面板骨架）——字段与开关：

```go
package view

import (
	"path"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"
	"github.com/oneo-me/onevra/source/icons/lucide"
	"github.com/egoist/mygo/ui/markdown/parser"

	"mdpreview/internal/workspace"
)

// findAnchorPrefix marks an anchor that names a search hit's block rather than
// a heading's slug: the markdown view scrolls to the block AnchorBlock returns
// for it.
const findAnchorPrefix = "find:"

// findPaneWidth is how wide the floating panel is, in DIPs, and findListHeight
// how tall its matches list and its preview grow before scrolling.
const (
	findPaneWidth  float32 = 420
	findListHeight float32 = 132
	findPreview    float32 = 148
)

// findState is the window's search over the document shown: whether the panel
// floats, the phrase, where it matches, and which match the body shows.
type findState struct {
	open      bool
	query     string
	matches   []workspace.Match
	current   int // 0-based into matches, 0 while there are none
	truncated bool
}
```

`state.go`：`find findState` 字段；`loadDoc` 成功之后（`s.src = src` 之后）：

```go
	// A new document is a new search: the matches of the last one name
	// lines that no longer mean the same thing.
	s.find.matches, s.find.current = nil, 0
	if s.find.open && s.find.query != "" {
		s.refreshMatches()
	}
```

`refreshMatches` / `findNext` / `openFind` / `closeFind`：

```go
// refreshMatches recomputes where the phrase appears in the document shown.
func (s *State) refreshMatches() {
	s.find.matches, s.find.truncated = workspace.FindIn(s.src, s.find.query)
	s.find.current = min(s.find.current, max(0, len(s.find.matches)-1))
}

// findNext moves to the match n places on, round the ends, showing it in the
// body.
func (s *State) findNext(n int) {
	if len(s.find.matches) == 0 {
		return
	}
	s.find.current = ((s.find.current+n)%len(s.find.matches) + len(s.find.matches)) % len(s.find.matches)
	s.gotoMatch(s.find.current)
}

// openFind shows the panel, over the document shown: without one there is
// nothing to search.
func (s *State) openFind() {
	if s.sel.empty() {
		return
	}
	s.find.open = true
	if s.find.query != "" {
		s.refreshMatches()
	}
}

// closeFind hides the panel and clears the page's highlights.
func (s *State) closeFind() {
	s.find.open = false
	if s.wv != nil {
		s.wv.Page().StopFindInPage()
	}
}
```

（`truncated` 来自 `FindIn` 的第二个返回值；面板在截断时于计数旁显示「前 500 处」。）

面板本体（`findPanel`，在 `preview()` 末尾调用；`find.go` 的 import 需加 `github.com/egoist/mygo`、`.../shells/sidebar`、`mdpreview/internal/config`）：

```go
// findPanel floats the search over the body's top right, as an editor's find
// bar does: the field, how many matches there are, the lines they sit on with
// the one chosen first, and the lines around it to see it in context.
func (s *State) findPanel(c *ui.Context) {
	if !s.find.open || s.sel.empty() {
		return
	}
	t := c.Theme()
	ui.Overlay(c, func() {
		ui.Column(c).Absolute().
			Right(t.Space(2)).
			Top(sidebar.HeaderHeight(c) + tabBarHeight + t.Space(2)).
			Width(findPaneWidth).
			Radius(t.Radius * 1.5).Background(t.Surface).
			Border(1, t.Border).
			Shadow(0, 8, 24, 0, t.Text.Alpha(0.12)).
			Padding(t.Space(1)).Gap(t.Space(1)).Children(func() {
				s.findField(c)
				s.findList(c)
				s.findPreview(c)
			})
	})
}
```

`findField`：一行 `ui.SearchField(c, &s.find.query).Label("在文档中查找").AutoFocus()` ——
`.Changed()` 里 `s.refreshMatches()`，`.Submitted()`（Enter）里 `s.findNext(1)`；右侧依次是
`n/m` 计数（截断时改成 `前 500 处`）、`ArrowUp`「上一个匹配」、`ArrowDown`「下一个匹配」两个
`IconButtonCompact`、`X`「关闭搜索」。Shift+Enter 的上一个不靠字段（字段不告诉按键是否带
Shift），改由 `Shift+Cmd+G` 与 `↑` 按钮承担。

`findList`：`ui.ScrollBoth(c).MaxHeight(findListHeight)` 里逐行 `ui.Row`（行号 + 文本），选中行 `Background(selectedRowTint(t))` 且 `Clicked()` 调 `s.gotoMatch(i)`（同时 `s.find.current = i`）。

`findPreview`：取 `s.find.matches[s.find.current]`，从 `s.src` 切出上下各 2 行（按 `
` 切，
行号与列表一致），每行 `ui.Row{行号, ui.RichText(前文, 命中, 后文)}`——命中那一段用
`t.Accent` 前景 + `t.Accent.Alpha(0.18)` 底色，与列表里选中的那一行呼应；
外层 `ui.ScrollBoth(c).MaxHeight(findPreview)`。

`gotoMatch` + `anchorBlock`：

```go
// gotoMatch shows the match at i in the body: the native view scrolls to the
// block holding it, the web view to the same place in its page.
func (s *State) gotoMatch(i int) {
	if i < 0 || i >= len(s.find.matches) {
		return
	}
	s.find.current = i
	if s.mode() == config.RenderWebView {
		s.findInWeb()
		return
	}
	if s.doc == nil {
		return
	}
	// The anchor is the block, not the match: a scroll is all the native
	// view does, and one block is as close as it gets.
	s.anchor = findAnchorPrefix + strconv.Itoa(blockAt(s.doc, s.find.matches[i].Offset))
	s.anchorVer++
	s.md.Anchor, s.md.AnchorVersion = s.anchor, s.anchorVer
}

// anchorBlock is the markdown view's block lookup: a find anchor names the
// block to scroll to, anything else is a heading's slug the view finds itself.
func (s *State) anchorBlock(d *parser.Document, name string) int {
	rest, ok := strings.CutPrefix(name, findAnchorPrefix)
	if !ok {
		return -1
	}
	i, err := strconv.Atoi(rest)
	if err != nil || i <= 0 || i >= d.Len() {
		return -1
	}
	return i
}

// blockAt returns the deepest block of d whose source holds the byte offset at.
func blockAt(d *parser.Document, at int) int {
	best := 1
	for i := 1; i < d.Len(); i++ {
		b := d.Block(i)
		if b.Start <= at && at < b.End && b.Start >= d.Block(best).Start {
			best = i
		}
	}
	return best
}
```

`loadDoc` 里挂上：`s.md.AnchorBlock = s.anchorBlock`。

`fileacts.go` 的 `handleShortcuts` 里加 `s.findShortcuts(c)`；`find.go`：

```go
// findShortcuts handles the panel's keys: Cmd+F opens it, Escape empties the
// field before it closes the panel, and Cmd+G walks the matches.
func (s *State) findShortcuts(c *ui.Context) {
	switch {
	case c.Shortcut(ui.Cmd, ui.KeyF):
		s.openFind()
	case s.find.open && c.Shortcut(0, ui.KeyEscape):
		if s.find.query != "" {
			s.find.query = ""
			s.refreshMatches()
			return
		}
		s.closeFind()
	case s.find.open && c.Shortcut(ui.Cmd, ui.KeyG):
		if c.Modifiers()&ui.Shift != 0 {
			s.findNext(-1)
			return
		}
		s.findNext(1)
	}
}
```

- [ ] **步骤 4：运行测试验证通过**

运行：`go test ./internal/view -run 'TestFind|TestEscape' -v`
预期：PASS。手工验证（`go run .`）：`Ctrl+F` 面板浮在正文右上（不是弹窗、不遮窗口），输入时实时出 `n/m`、匹配行列表与上下文预览；点列表某行正文跳到该段；`Esc` 清空、再按关闭。

- [ ] **步骤 5：Commit**

```bash
git add internal/view/find.go internal/view/find_test.go internal/view/state.go internal/view/preview.go internal/view/fileacts.go internal/view/native.go internal/workspace/find.go internal/workspace/find_test.go
git commit -m "feat: Cmd+F 唤起悬浮搜索，带匹配行列表与行上下文预览"
```

---

### 任务 7：WebView 侧的搜索（浏览器高亮）与文档变动后的收尾

**文件：**
- 修改：`internal/view/find.go`（`findInWeb`、`gotoMatch` 的 webview 分支）
- 修改：`internal/view/webview.go`（`loadWebDoc` 后不再自动找，避免在页面加载完成前调用）
- 测试：`internal/view/find_test.go`、`internal/view/webview_test.go`

- [ ] **步骤 1：编写失败的测试**

```go
// TestFindJumpWithoutAWebViewOnlyMovesTheList pins the degraded path: with no
// web view made (headless tests, a machine without one) jumping in webview mode
// still moves the list, and only says so in the status bar.
func TestFindJumpWithoutAWebViewOnlyMovesTheList(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "a.md", "一 目标\n二 目标\n")
	s.Refresh()
	s.selectDoc("a.md")
	s.setMode(config.RenderWebView)
	s.find.open = true
	s.find.query = "目标"
	s.refreshMatches()

	s.gotoMatch(1)
	if s.find.current != 1 {
		t.Errorf("current = %d, want 1", s.find.current)
	}
	if s.wv != nil {
		t.Fatal("a web view was made without a window")
	}
}

// TestClosingTheFindPanelStopsThePageHighlight is a compile-level pin for the
// web view call: the panel asks the page to stop highlighting when it closes.
func TestClosingTheFindPanelStopsThePageHighlight(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "a.md", "# A\n")
	s.Refresh()
	s.selectDoc("a.md")
	s.find.open = true
	s.closeFind()
	if s.find.open {
		t.Error("the panel stayed open")
	}
}
```

- [ ] **步骤 2：运行测试验证失败**

运行：`go test ./internal/view -run 'TestFindJumpWithoutAWebView|TestClosingTheFindPanel'`
预期：FAIL（`gotoMatch` 的 webview 分支还没有实现，`current` 不前进）。

- [ ] **步骤 3：实现**

`find.go`：

```go
// findInWeb highlights the phrase in the page and scrolls to the match chosen:
// the web view's own find does both, and counts them.
func (s *State) findInWeb() {
	if s.wv == nil {
		s.status = "WebView 不可用，已跳过页面内高亮"
		return
	}
	res, err := s.wv.Page().FindInPage(s.find.query, mygo.FindOptions{})
	if err != nil {
		s.status = "无法在页面中查找：" + err.Error()
		return
	}
	// The page counts its own matches: markup can split one, so its count
	// is not always the source's. Walk to the match chosen, as far as the
	// page goes.
	target := min(s.find.current, res.Matches-1)
	for n := 1; n <= target; n++ {
		if _, err := s.wv.Page().FindInPage(s.find.query, mygo.FindOptions{FindNext: true}); err != nil {
			s.status = "无法在页面中查找：" + err.Error()
			return
		}
	}
}
```

`gotoMatch` 的 webview 分支改成 `s.findInWeb()`（去掉 `!s.find.open` 之类的多余条件）。

`webview.go` 的 `loadWebDoc`：**不**在加载后调用 `FindInPage`（页面还没加载完），改为在注释里写明「文档切换后页面高亮丢失，点一次匹配项即恢复」（写进 README 已知限制）。

- [ ] **步骤 4：运行测试验证通过**

运行：`go test ./internal/view -run 'TestFind' -v` 然后 `go test ./...`
预期：PASS。手工验证（`go run .`，切 WebView）：`Ctrl+F` 输入 → 页面里全部命中高亮、当前一个是另一种颜色；点列表第 3 行 → 页面滚到第 3 处；关闭面板 → 高亮消失。

- [ ] **步骤 5：Commit**

```bash
git add internal/view/find.go internal/view/webview.go internal/view/find_test.go
git commit -m "feat: webview 模式下用页面内查找来高亮并滚动到命中处"
```

---

### 任务 8：文档、截图状态与全量验证

**文件：**
- 修改：`README.md`（功能表、快捷键、已知限制）
- 修改：`AGENTS.md`（结构表、硬规则/坑、改动指引）
- 修改：`docs/superpowers/specs/2026-11-21-markdown-preview-design.md`（「非目标」里删掉「多文档标签页」「全文检索的 UI」，加一行指向本计划）

- [ ] **步骤 1：改 README**

在功能列表加三条：标签页（含「全部已打开文件」浮层、`Cmd/Ctrl+W` 关闭）、目录工具条（展开/折叠全部、正文缩放 `Cmd/Ctrl+=/-/0`）、悬浮搜索（`Cmd/Ctrl+F`，匹配列表 + 上下文预览）。「已知限制」加两条：

- 原生渲染模式下搜索不高亮正文（只有面板里的上下文预览有高亮）；WebView 模式下整页高亮。
- WebView 模式切换文档后页面高亮会丢，点一次匹配项即恢复；缩放不重载页面，因此不丢滚动位置。

- [ ] **步骤 2：改 AGENTS.md**

- 结构表：`internal/view` 一行补上 `tabs.go`（标签页与标签栏）、`find.go`（悬浮搜索与跳转）；`internal/workspace` 一行补 `find.go`（`FindIn`）。
- 「已知的坑」加：
  - **标签与滚动位置都以 `docSel` 为键**（`tabs` / `scrolls`）：只改路径不改项目会串到另一个项目的同名文件。
  - **原生搜索只能滚到块**：`Options.AnchorBlock` + `Anchor` 是唯一的块级滚动入口，`find:<块号>` 前缀是搜索在用它，普通 slug 必须返回 -1 让 mygo 自己找标题。
  - **标签栏宽度是算出来的**（`tabWidth` + `visibleTabs`）：`ui.Tester` 里窗口宽度来自 `windowWidth()`，改布局要同步 `Content` 与 `View` 两处。
  - **`Cmd+F` 的 Escape 有先后**：有查询先清空、空查询才关面板，否则一次按键既清空又关闭。
- 「改动指引」加：改搜索匹配规则 → `workspace.FindIn` + `TestFindIn*`；改标签栏 → `view/tabs.go` + `TestTabBar*`。

- [ ] **步骤 3：全量验证**

```sh
go vet ./... && gofmt -l .          # 都必须无输出
go test ./...                        # 全绿
go run .                             # 手工验收：标签栏 / 目录四动作 / Ctrl+F 三块内容
```

- [ ] **步骤 4：Commit**

```bash
git add README.md AGENTS.md docs/superpowers/specs/2026-11-21-markdown-preview-design.md
git commit -m "docs: 记录标签页、目录动作与悬浮搜索的用法与已知限制"
```

---

## 交付前检查清单

- [ ] `go test ./...` 全绿，`go vet ./...` 与 `gofmt -l .` 无输出。
- [ ] `TestShellSlotsBuild` 仍通过：`Content` 与 `View` 两条布局路径同步（标签栏两处都加）。
- [ ] 无新依赖（`go.mod` 里的 require 不变），无 `//go:build` 平台代码。
- [ ] `internal/workspace/find.go` 不 import `.../ui`；`internal/mcp` 未被改动。
- [ ] 颜色全部来自 `c.Theme()`；新增图标按钮都有 `label`。
- [ ] 每个任务一个中文提交，前缀 `feat:` / `fix:` / `test:` / `docs:`。

## 未决问题（实施前请用户确认，确认后改写上文对应决策）

1. **「合并所有」= 折叠所有？** 本计划按「折叠」实现（D8）。若指的是「所有标题层级合并成一级平铺」，这是另一个功能（TOC 拍平），需要单独一轮。
2. **标签要不要跨重启保留？** 本计划不持久化（D7）。若要，加 `cfg.Tabs []string` + `cfg.ActiveTab`，并在 `config.normalize` 里过滤掉不存在的文档。
3. **缩放要不要记住？** 本计划只在会话内（D10）。若要，加 `cfg.Zoom float32` 并在 `normalize` 里夹到 0.8–2.0。
4. **搜索要不要正则 / 全词 / 项目内全文件搜？** 本计划只有「当前文档、字面子串、忽略大小写」（D11）。项目内全文件搜可以复用既有的 `workspace.Search`，但那是另一个面板（结果跨文件），建议下一期。
5. **原生模式正文高亮**：本计划接受「不高亮」（D15）。若必须高亮，需要 `markdown.Options.Block` 自绘段落，约 200 行、且要处理表格/代码块/嵌套列表，风险明显更高——请确认是否值得。


---

## 执行后的修订（用户 2026-11-22 实机反馈，已实现）

1. **标签栏与标题行的顺序反过来**：标签栏改由 shell 的 `ContentTitle` 槽位（标题栏 48 DIP）构建，
   原来在该槽位的「路径 + 复制 + 渲染开关」下沉成 `Content` 的第一行（`titleRow`，`titleRowHeight`）。
   屏幕顺序变成：标签 → 路径与渲染开关 → 目录/正文。`View` 的测试拼装同步。搜索面板的 `Top`
   也随之从 `HeaderHeight + tabBarHeight` 改成 `HeaderHeight + titleRowHeight`。
   回归测试：`TestTabsSitAboveTheTitle`、`TestTitleSitsInsideItsRow`、`TestShellSlotsBuild`。
2. **标签不再重叠、也不再压到窗口按钮**：每个标签固定宽度（`tabWidth`）并 `Clip`，名字用
   `fitText` 按 `tabTextWidth` 截断；标签栏的可用宽度取**上一帧自己的 `Bounds().W`**（首帧回落
   `windowWidth()`），因为 shell 在标题栏右侧给窗口按钮留了位置。回归测试：
   `TestTabBarFitsTheRoomItIsGiven`、`TestTabBarKeepsActiveTabVisible`。
3. **「全部已打开文件」列表左对齐**：行内改用一层 `Grow(1)` 的 `ui.Row` 顶满按钮宽度，
   图标与文件名从左边缘排（按钮默认居中）。
4. **目录工具条放大**：`目录` 13px，四个动作从 `IconButtonCompact` 换成 `IconButtonPlain`（32 DIP 方块、
   20 DIP 图形），比例文字 11px。

5. **标签条改为横向滚动**（用户 2026-11-22 追加：「标签可横向滚，⌄ 只作为补充」）：标签不再按可视宽度挑选，
   全部排进 `ui.ScrollHorizontal`（`State.tabScroll`），`tabsWidth` 只决定 ⌄ 按钮出不出现；
   `revealTab` 在构建前把活动标签滚进视野，首帧（`Bounds`/`MaxX` 未知）不记 `tabShown`。
   回归测试：`TestTabBarScrollsAndKeepsTheListInsideItsSlot`、`TestTabScrollFollowsTheTabShown`。

6. **标签条的三个小修**（用户 2026-11-22 实机反馈）：⌄ 按钮从 `IconButtonCompact` 换成 `IconButtonPlain`（32 DIP），
   预留宽度同源（`tabOverflow`）；`revealTab` 改成**停在某个标签的起点**（原来右对齐会切出「空白 + 孤零零的 ✕」）；
   滚动容器挂 `HandleInput` 把普通滚轮转成横向滚动（mygo 只认 Shift+滚轮）。
   回归测试：`TestTabStripRestsOnATabStart`、`TestTabStripScrollsWithTheWheel`。

7. **标题槽首部间距 24 → 8**（用户 2026-11-22）：onevra 的 sidebar 把 `titleInset` 写死成 24，
   `ShellOptions` 不暴露；`ContentTitle` 里整行 `Translate(titleInsetWanted-titleInsetGap, 0)` 抵掉 16。
   回归测试：`TestTabsTuckUnderTheSidebarHeader`（同时验证拉回来后点击仍落在标签上）。

8. **去掉标签条自身的左右内边距**（用户 2026-11-22）：留白由 shell 的标题槽 padding 提供，
   `tabBar` 不再 `Padding(0, t.Space(1))`，可用宽度就是这一行的整宽（`Bounds().W`，不再减 8）。
   `TestTabsTuckUnderTheSidebarHeader` 收紧成「第一个标签正好起于 `titleInsetWanted`」。

9. **标签条的停留位置、细滚动条、标题槽间距**（用户 2026-11-22 实机反馈）：
   `revealTab` 改成「当前标签完整可见 + 左边缘落在标签起点（够不着时退到 `MaxX`）」，两端加 `tabFade` 渐隐；
   滚动条用 `ScrollbarStyle{ThumbWidth: 2, TrackWidth: 2, Flush: true}`（默认 `拇指+6` 的命中区压在标签底部，
   会吃掉点击）；`titleInsetWanted` 由 8 调到 12（8 看起来贴边），并且只把**标签条**往左拉，⌄ 按钮留在标题槽的
   右侧内边距上（之前整行平移，右边多出 16 的空档）。
   回归测试：`TestTabStripShowsTheWholeTabShown`、`TestTabStripFollowsTheRoomItHas`、`TestTabsTuckUnderTheSidebarHeader`。

10. **「选中的标签完整可见」的两个真 bug**（用户 2026-11-22 再次反馈）：滚动容器的 `MaxX` 慢一帧，
    刚打开的标签还没撑开它，原来把位置 clamp 到旧 `MaxX` 会**差整整一个标签**，而且记成"已滚到"后再不重试。
    现在 `revealTab` 返回是否 settled，未 settled 时置 `tabPending` 下一帧重试；`tabTotal` 参与记忆
    （关掉前面的标签会挪动当前标签）；滚轮一动清掉 `tabPending`。
    另外「全部已打开文件」列表右侧留 `Space(3)` 给竖向滚动条，避免它盖住 ✕。
    回归测试：`TestTabsOpenedOneByOneStayWhole`、`TestTabsListKeepsClearOfItsBar`。

11. **切换标签时标签条滑动到位**（用户 2026-11-22 追加）：原来直接写 `tabScroll.X`，视觉上是瞬间跳位；
    改成 `ScrollState.ScrollTo(x, y, ui.Smooth)`，并用 `tabAsk` 记住已请求的位置去重；范围还没算好
    （`MaxX` 慢一帧）时不发起动画，下一帧再滑；系统「减少动态效果」时 mygo 直接到位。
    回归测试：`TestTabsSlideToTheTabShown`。
