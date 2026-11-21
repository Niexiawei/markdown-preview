# 目录面板滚动、渲染分段器、标题栏复制路径与文件树右键菜单 实现计划

> **面向 AI 代理的工作者：** 必需子技能：使用 subagent-driven-development（推荐）或 executing-plans 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** 让正文目录（TOC）面板在内容超出时能上下、左右滚动；把标题栏的「原生 / WebView」换成主题的分段控件；在文档路径后加一个复制路径的图标按钮；给文件树每一项加右键菜单，并让 `Cmd/Ctrl+C` 复制当前文档的绝对路径。

**架构：** 全部落在 `internal/view`，不改 `workspace` / `config` / `mcp`。TOC 用 `ui.ScrollBoth` + `State.tocScroll`（与正文的 `docScroll` 同构），内容列用 `MinWidth` 顶住窗格宽度以保证短标题也整行可点；渲染开关改用 `components.Tabs(…, TabsSegmented, …)`，并让标题栏行 `FillWidth()`、路径文本 `Shrink(1)`，空间不够时路径省略而不是把开关挤出窗口；复制路径与文件树动作放进新文件 `internal/view/fileacts.go`，一律走既有的 `System` 接口（`CopyText` / `Reveal`），测试注入 `fakeSystem`。

**技术栈：** Go 1.27+、`github.com/egoist/mygo v0.9.1`（`ui.ScrollBoth`、`ui.ScrollState`、`ui.Tree`、`ui.Context.Shortcut`）、`github.com/oneo-me/onevra/source v0.2026.18`（`components.Tabs` / `TabsSegmented`、`components.IconButton` + `IconButtonCompact`、`components.ContextMenu`、`components.Menu`、`icons/lucide`）。

**规格：** 无独立规格；需求来自用户 2026-11-21 的四条反馈（附截图：目录面板与文件树被裁掉、标题栏两个独立按钮、文件名后没有复制入口）。本计划的「设计决策」一节即设计记录，架构背景见 `docs/superpowers/specs/2026-11-21-markdown-preview-design.md`。

## 设计决策（实施者按此执行，不要另立方案）

| # | 决策 | 理由 |
| --- | --- | --- |
| D1 | TOC 面板 = 固定的「目录」标题 + 一个 `ui.ScrollBoth`，滚动位置存 `State.tocScroll`（与正文的 `docScroll` 同构）。 | 用户原话「目录内容超出可视区域之后 y 轴与 x 轴需要可以滚动」；现结构是 `ui.Column(c).Fill()` 里直接放 `ui.Tree`，没有滚动容器，两向都被裁掉。 |
| D2 | 滚动内容列用 `MinWidth(窗格宽 - 2 * 8)`，不是 `Fill()`。 | `ScrollBoth` 给子元素的可用宽度是无限的（mygo `ui/layout.go` 在 `flagScrollX` 时把 `lw` 设为 `inf`），`Fill()` 的 100% 会落空；不给 `MinWidth` 时短标题的树行只有文字宽，点窗格右侧空白选不中该行（已实测：240 宽窗格里不加 `MinWidth` 时 x=220 点不到行）。 |
| D3 | 换文档时把 `tocScroll.X/Y` 归零；不做「每个文档记住各自的目录滚动位置」。 | 一份新目录停在上一个文档滚到的位置是纯粹的意外；按文档存位置需要与 `docScroll` 一套新状态，收益不足（YAGNI）。 |
| D4 | 渲染开关用 `components.Tabs(c, &idx, components.TabsSegmented, "原生", "WebView")`，删掉 `State.modeIdx`；每个文档的渲染模式（`renderMode map[docSel]RenderMode`）语义不变。 | 用户原话「还是使用分段器 而不是两个独立按钮」；Onevra 的 `TabsSegmented` 就是 macOS 式分段控件（凹槽 + 抬起滑块），再用自绘胶囊就是重复造轮子。 |
| D5 | 标题栏的行加 `FillWidth()`，路径文本改 `.SingleLine().TruncateMiddle().Shrink(1)`，去掉 `trimEllipsis(path, 90)` 的按字数截断。 | 实测：不加 `FillWidth` 时行宽由内容决定，窄窗口下开关被挤出窗口（280 宽时 `WebView` 段落在 284..300，看不见也点不到）；`FillWidth` + `Shrink(1)` 后由布局按可用宽度省略，`TruncateMiddle` 保住文件名的尾段（`alpha/docs/…/name.md`）。 |
| D6 | 复制路径的图标用 `components.IconButton(c, lucide.Copy(), "复制文件路径", components.IconButtonCompact)`，位置在路径文本之后、渲染开关之前，只在有文档时出现。 | 标题栏高 `sidebar.HeaderHeight` ≥ 48，`IconButtonCompact`（20 方块 / 14 图形）与 12px 路径文本、分段控件同一视觉密度；`IconButtonPlain`（32）在这一行比文字大一圈。（AGENTS.md 里「不要用 Compact」针对的是侧栏底部动作区，不是标题栏。） |
| D7 | 复制/打开动作一律加到 `System` 之外的新文件 `internal/view/fileacts.go`，只调用既有的 `System.CopyText` / `System.Reveal`，不加接口方法，也不在 view 里直接调 `mygo.Clipboard` / `mygo.Shell`。 | 接口已够用（`Reveal` 收绝对路径）；测试继续注入 `fakeSystem`，不碰真实剪贴板与文件管理器。 |
| D8 | 文件树菜单文案：所有项都是 `复制文件名称` / `复制文件路径`，目录额外一条分隔线 + `在文件管理器中打开`（复用项目菜单的同一文案，不用用户原话「文件浏览器」）。 | 同一个动作在一个应用里两个名字（项目菜单已有「在文件管理器中打开」）会让右键菜单看起来像两个功能。若用户坚持「文件浏览器」，改一处字符串 + 两个测试即可。 |
| D9 | `Cmd/Ctrl+C` 复制**当前显示文档**（`State.sel`）的绝对路径，与标题栏图标同一实现 `copyDocPath`；不做「复制树里获得键盘焦点的那一项」。 | 用户原话「ctrl + c 默认复制 文件路径」；树项的键盘焦点与「选中文档」在本应用里不是同一个概念（目录节点也能拿焦点、没有路径可谈），取当前文档是唯一无歧义的定义。 |
| D10 | 快捷键走 `c.Shortcut(ui.Cmd, ui.KeyC)`，在 `View` 与 `Content` 各调用一次 `handleShortcuts`。 | `ui.Cmd` 在 macOS 是 Command、其余平台是 Control；`Content` 是 shell 必然构建的槽位（侧栏收起也在），`View` 是测试用的并排布局。`rt.shortcut` 对同一次按键只交付一次，所以两处调用不会重复复制。 |

### 改完长什么样（验收画面）

```
标题栏：[📄] lynkco08-cam/SENTINEL_PLAN.md [⧉] ( 原生 | WebView )
                                              ↑     ↑ 一个胶囊，选中项抬起
                                              └ 点它 = 复制绝对路径

目录面板（240 宽，纵向可滚、横向可滚）：
  目录
  ▾ 哨兵守护总方案
      范围
      执行顺序            ← 短标题也整行可点（MinWidth 顶住窗格宽）
  …

文件树：
  ▾ docs
      a.md     ← 右键：复制文件名称 / 复制文件路径
  ▾ tools      ← 右键：复制文件名称 / 复制文件路径 / ─── / 在文件管理器中打开
```

## 全局约束

- Go 模块名 `mdpreview`，`go 1.27.3`；不引入新依赖，不新增第三方包。
- 不写平台专用代码（不加 `//go:build windows` 之类），三平台保持可编译。
- `internal/workspace` 与 `internal/mcp` 不 import `.../ui`；本计划不改这两个包。
- 颜色只从 `AppTheme`（`c.Theme()`）取，不写死十六进制；图标只用 `icons/lucide`。
- 图标按钮必须给 `label`（无障碍名，也是测试定位的唯一方式）。
- 测试只用 `t.TempDir()`，不读写真实用户目录；不调用 `mygo.Clipboard` / `mygo.Shell` / `mygo.Dialog`（走 `fakeSystem`）。
- 视图状态归窗口线程；本计划不新增 goroutine、不改 `watch` / `Post`。
- 每个行为改动先有失败的测试；提交信息中文，前缀 `feat:` / `fix:` / `test:` / `docs:` / `style:`。
- 提交前 `go vet ./... && gofmt -l .` 必须无输出。

## 文件结构

| 文件 | 职责 | 本计划里的改动 |
| --- | --- | --- |
| `internal/view/native.go` | 原生渲染、`loadDoc`、TOC 构建与点击跳转 | 加 `tocPadding` 常量；`toc` 换成固定标题 + `ScrollBoth`；`loadDoc` 里归零 `tocScroll` |
| `internal/view/state.go` | `State` 字段与项目/文档选择 | 加 `tocScroll ui.ScrollState`；删 `modeIdx` 字段及其注释 |
| `internal/view/view.go` | 外壳各槽位与树 | `contentTitle` 加 `FillWidth` + 省略路径（任务 2）、复制图标（任务 3）；`modeToggle` 换 `components.Tabs`；`treeItems` 挂 `nodeMenu`；`View` / `Content` 调 `handleShortcuts` |
| `internal/view/fileacts.go` | **新建**：文件动作（复制名称/路径、在文件管理器中打开、`Cmd+C`） | `copyDocPath` / `copyNodeName` / `copyNodePath` / `revealNode` / `handleShortcuts` |
| `internal/view/webview.go` | WebView 后端 | 删掉 `setMode` 里的 `s.modeIdx = s.modeIndex()` |
| `internal/view/layout_test.go` | 布局与标题栏测试 | TOC 两向滚动、整行可点、分段器、窄窗口、复制路径图标 |
| `internal/view/view_test.go` | 全窗口与桌面动作测试 | 文件树三项菜单、无桌面时 no-op、`Cmd+C` |

## 审查重点（Review Focus）

规格只说了「要能滚」「要分段器」「要复制」，没说的这些条件一样会让用户用不了：

1. **窄窗口 + 很长路径**（760 的最小窗口、标题栏里 60+ 字符的路径）：渲染开关必须整段留在窗口内且可点，路径自己省略。→ `TestTitleFitsNarrowWindow`（任务 2）
2. **超长标题 / 上百条标题**（一份 60 节、每节名带 120 个汉字的文档）：目录面板上下左右都能滚，且短标题的树行仍然占满窗格宽度、点右侧空白也能选中。→ `TestTOCScrollsBothWays`、`TestTOCRowsFillThePane`（任务 1）
3. **目录在扫描之后被删 / 无权限**：右键「在文件管理器中打开」失败时只在状态栏说明，不 panic、不吞掉。→ `TestTreeDirMenuReportsARevealFailure`（任务 4）
4. **没有选中文档时按下 Cmd/Ctrl+C**：什么都不复制（不写空串、不误解成项目路径），标题栏也不出现复制图标。→ `TestCmdCCopiesNothingWithoutADocument`（任务 4）
5. **没有桌面实现（`State.sys == nil`，测试或早期初始化）**：右键菜单、复制图标、快捷键都退化成 no-op，不 panic。→ `TestTreeMenuWithoutDesktopDoesNothing`（任务 4）

---

### 任务 1：目录面板双向滚动

**文件：**
- 修改：`internal/view/native.go:120-136`（`toc`）、`internal/view/native.go:16-45`（`loadDoc`）
- 修改：`internal/view/state.go:100-104`（`tocW` / `docScroll` 旁）
- 测试：`internal/view/layout_test.go`

- [ ] **步骤 1：编写失败的测试**

在 `layout_test.go` 末尾加两个测试（需要给 import 加 `fmt`、`strings`）：

```go
// TestTOCScrollsBothWays pins that the outline scrolls both ways: a document
// with many headings and one long heading is cut neither at the bottom nor at
// the right edge.
func TestTOCScrollsBothWays(t *testing.T) {
	s, root := newState(t)
	var b strings.Builder
	b.WriteString("# " + strings.Repeat("很长", 60) + "\n\n")
	for i := range 60 {
		fmt.Fprintf(&b, "## 小节 %02d\n\n正文\n", i)
	}
	writeDoc(t, root, "a.md", b.String())
	s.Refresh()
	s.selectDoc("a.md")

	_ = ui.NewTester(func(c *ui.Context) { s.toc(c) }, 240, 300)
	if s.tocScroll.MaxY <= 0 {
		t.Errorf("MaxY = %v, want the outline to scroll down", s.tocScroll.MaxY)
	}
	if s.tocScroll.MaxX <= 0 {
		t.Errorf("MaxX = %v, want the long heading to scroll sideways", s.tocScroll.MaxX)
	}
}

// TestTOCRowsFillThePane pins that a short outline's rows still take the whole
// pane: a click at the pane's right edge chooses the heading there.
func TestTOCRowsFillThePane(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "a.md", "# 短标题\n\n## 范围\n\n## 执行顺序\n")
	s.Refresh()
	s.selectDoc("a.md")

	tt := ui.NewTester(func(c *ui.Context) { s.toc(c) }, 240, 300)
	rect, ok := tt.Find("范围")
	if !ok {
		t.Fatalf("texts = %q, want the heading", tt.Texts())
	}
	tt.ClickAt(220, rect.Y+rect.H/2)
	if s.anchor != "范围" {
		t.Errorf("anchor = %q, want the heading clicked at the pane's edge", s.anchor)
	}
}
```

- [ ] **步骤 2：运行测试验证失败**

运行：`go test ./internal/view -run 'TestTOCScrollsBothWays|TestTOCRowsFillThePane' -v`
预期：FAIL —— `TestTOCScrollsBothWays` 报 `MaxY = 0`（此时 `tocScroll` 还没被任何容器接管），`TestTOCRowsFillThePane` 通过（它是防回归断言，见步骤 4 说明）。

- [ ] **步骤 3：实现**

`state.go`：在 `docScroll` 旁加字段（注释一样写清单位与用途）：

```go
	// tocScroll is how far the document's outline scrolls, both ways.
	tocScroll ui.ScrollState
```

`native.go`：加常量并把 `toc` 改成「固定标题 + 双向滚动 + 窗格宽的树」：

```go
// tocPadding is the room around the outline, in DIPs.
const tocPadding float32 = 8

func (s *State) toc(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Fill().Padding(tocPadding).Children(func() {
		ui.Text(c, "目录").FontSize(11).TextColor(t.TextMuted)
		if len(s.headings) == 0 {
			return
		}
		// Both ways: an outline deeper than the pane scrolls down, and a
		// heading wider than it scrolls sideways instead of being cut. The
		// column takes at least the pane's own width, so that a short
		// heading's row still stretches across it and a click past its text
		// chooses it.
		ui.ScrollBoth(c).Grow(1).TrackScroll(&s.tocScroll).Children(func() {
			ui.Column(c).MinWidth(max(0, s.tocW-2*tocPadding)).Gap(t.Space(0.25)).Children(func() {
				ui.Tree(c, func() { s.tocItems(c, buildTOC(s.headings)) })
			})
		})
	})
}
```

`loadDoc`（`native.go`，`s.doc, s.headings, s.src = …` 之后）加两行，注释说明是按文档重置：

```go
	// A document's outline starts at its top, not where the last one's
	// scroll was left.
	s.tocScroll.X, s.tocScroll.Y = 0, 0
```

- [ ] **步骤 4：运行测试验证通过**

运行：`go test ./internal/view -run 'TestTOC|TestBodyColumnFollowsPaneWidth' -v`
预期：PASS。`TestTOCRowsFillThePane` 在这里验证的是 `MinWidth` 的必要性：把 `MinWidth(…)` 临时删掉再跑它必须 FAIL（x=220 落在树行右边）。`TestTOCScrollsBothWays` 实测应为 `MaxX ≈ 1486`、`MaxY ≈ 1438`。

- [ ] **步骤 5：Commit**

```bash
git add internal/view/native.go internal/view/state.go internal/view/layout_test.go
git commit -m "fix: 目录面板在两个方向上滚动，内容不足一屏时仍整行可点"
```

---

### 任务 2：渲染开关换成主题分段器，标题栏不再被长路径撑破

**文件：**
- 修改：`internal/view/view.go:146-188`（`contentTitle`、`modeToggle`）
- 修改：`internal/view/state.go:68-70`（删 `modeIdx`）
- 修改：`internal/view/webview.go:222-228`（`setMode`）
- 测试：`internal/view/layout_test.go`

- [ ] **步骤 1：编写失败的测试**

把 `layout_test.go` 里已有的 `TestModeToggleSitsWithTheDocumentName` 的点击目标改成 `"WebView"` / `"原生"`，再加两个测试：

```go
// TestModeToggleIsOneSegmentedControl pins the switch as the theme's segmented
// control: the two backends are segments of one track, not two loose buttons
// that name themselves.
func TestModeToggleIsOneSegmentedControl(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "notes/a.md", "# A\n")
	s.Refresh()
	s.selectDoc("notes/a.md")

	tt := ui.NewTester(func(c *ui.Context) { s.ContentTitle(c) }, 900, 48)
	if tt.HasText("渲染方式：原生") {
		t.Errorf("texts = %q, want no per-segment label: the segmented control names itself", tt.Texts())
	}
	if err := tt.Click("WebView"); err != nil {
		t.Fatalf("Click WebView: %v", err)
	}
	if got := s.mode(); got != config.RenderWebView {
		t.Errorf("mode = %q, want webview", got)
	}
	if err := tt.Click("原生"); err != nil {
		t.Fatalf("Click 原生: %v", err)
	}
	if got := s.mode(); got != config.RenderNative {
		t.Errorf("mode = %q, want native", got)
	}
}

// TestTitleFitsNarrowWindow pins that a long path gives way instead of pushing
// the mode switch out of the window: at the narrowest useful title bar the
// WebView segment still ends inside it, and still works.
func TestTitleFitsNarrowWindow(t *testing.T) {
	s, root := newState(t)
	doc := "docs/2026-11-21-a-really-long-document-name-for-the-title-bar.md"
	writeDoc(t, root, doc, "# A\n")
	s.Refresh()
	s.selectDoc(doc)

	tt := ui.NewTester(func(c *ui.Context) { s.ContentTitle(c) }, 280, 48)
	rect, ok := tt.Find("WebView")
	if !ok {
		t.Fatalf("texts = %q, want the mode switch", tt.Texts())
	}
	if rect.X+rect.W > 280 {
		t.Errorf("the WebView segment ends at %v, past the window's 280", rect.X+rect.W)
	}
	if err := tt.Click("WebView"); err != nil {
		t.Fatalf("Click: %v", err)
	}
	if got := s.mode(); got != config.RenderWebView {
		t.Errorf("mode = %q, want webview", got)
	}
}
```

- [ ] **步骤 2：运行测试验证失败**

运行：`go test ./internal/view -run 'TestModeToggle|TestTitleFitsNarrowWindow' -v`
预期：FAIL —— 两个新测试都因为 `渲染方式：` 前缀还在（`HasText` 为真）而失败，`TestTitleFitsNarrowWindow` 另外报 `WebView` 段落在窗口外；旧的 `TestModeToggleSitsWithTheDocumentName` 因点不到 `"WebView"`（label 是 `渲染方式：WebView`）而失败。

- [ ] **步骤 3：实现**

`view.go` 的 `contentTitle`：行加 `FillWidth()`，路径文本改成由布局截断（**任务 3 会在这两行之间插入复制图标**）：

```go
	ui.Row(c).FillWidth().Height(sidebar.HeaderHeight(c)).Padding(0, t.Space(2)).AlignItems(ui.Center).Gap(t.Space(1)).Children(func() {
		if path := s.shortPath(); path != "" {
			components.Icon(c, lucide.FileText()).Size(t.Rem(0.875), t.Rem(0.875)).TextColor(t.TextMuted)
			// The path gives the rest of the bar its room: a long one ends in
			// an ellipsis in its middle, keeping the file's name, rather than
			// pushing the mode switch out of the window.
			components.Text(c, path).FontSize(12).TextColor(t.TextMuted).SingleLine().TruncateMiddle().Shrink(1)
			s.modeToggle(c)
		}
	})
```

`view.go` 的 `modeToggle` 整个换成（`modeToggle` 上方的注释一并改为「主题的分段控件」）：

```go
func (s *State) modeToggle(c *ui.Context) {
	idx := s.modeIndex()
	parts := components.Tabs(c, &idx, components.TabsSegmented, "原生", "WebView")
	// A window that could not make a web view keeps that segment out of the
	// pointer's way while the document asks for it.
	if !s.webOK && s.win != nil && s.mode() == config.RenderWebView {
		parts.Tab(1).Disabled(true)
	}
	if idx != s.modeIndex() {
		modes := []config.RenderMode{config.RenderNative, config.RenderWebView}
		s.setMode(modes[idx])
		s.onDocChanged()
	}
}
```

`state.go`：删掉 `modeIdx int` 字段与它上面的注释（`// modeIdx is the segment the status bar shows, which a click writes.`）。
`webview.go`：`setMode` 里删掉 `s.modeIdx = s.modeIndex()`，函数体只剩 `renderMode` 的那三行。

- [ ] **步骤 4：运行测试验证通过**

运行：`go test ./internal/view -v`
预期：PASS（全包，含 `TestShellSlotsBuild`）。实测布局：280 宽时路径缩到约 124、分段器整体落在 220..278。

- [ ] **步骤 5：Commit**

```bash
git add internal/view/view.go internal/view/state.go internal/view/webview.go internal/view/layout_test.go
git commit -m "feat: 渲染开关改用主题分段控件，长路径不再把开关挤出标题栏"
```

---

### 任务 3：标题栏「复制文件路径」图标

**文件：**
- 修改：`internal/view/view.go`（`contentTitle`，在两个 `components.Text` / `s.modeToggle` 之间）
- 创建：`internal/view/fileacts.go`
- 测试：`internal/view/layout_test.go`

- [ ] **步骤 1：编写失败的测试**

```go
// TestTitleCopiesTheDocumentPath pins the button after the document's name:
// one click puts the document's absolute path in the clipboard and says so.
func TestTitleCopiesTheDocumentPath(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "notes/a.md", "# A\n")
	s.Refresh()
	s.selectDoc("notes/a.md")
	sys := &fakeSystem{}
	s.SetSystem(sys)

	tt := ui.NewTester(func(c *ui.Context) { s.ContentTitle(c) }, 900, 48)
	if err := tt.Click("复制文件路径"); err != nil {
		t.Fatalf("Click: %v", err)
	}
	if want := filepath.Join(root, "notes", "a.md"); sys.copied != want {
		t.Errorf("copied = %q, want %q", sys.copied, want)
	}
	if !strings.Contains(s.Status(), "已复制文件路径") {
		t.Errorf("status = %q, want the copy reported", s.Status())
	}
}
```

- [ ] **步骤 2：运行测试验证失败**

运行：`go test ./internal/view -run TestTitleCopiesTheDocumentPath -v`
预期：FAIL，`Click: ui: no target labeled "复制文件路径"`。

- [ ] **步骤 3：实现**

新建 `internal/view/fileacts.go`（包注释风格与 `system.go` 一致，只放文件级动作）：

```go
package view

import (
	"github.com/egoist/mygo/ui"

	"mdpreview/internal/workspace"
)

// copyDocPath puts the absolute path of the document shown in the clipboard,
// as the title bar's button and Cmd+C do.
func (s *State) copyDocPath() { /* 见下 */ }
```

`copyDocPath` 的签名与语义（实现者照写）：

```go
// copyDocPath puts the absolute path of the document shown in the clipboard,
// as the title bar's button and Cmd+C do.
func (s *State) copyDocPath() {
	p, ok := s.SelectedProject()
	if !ok || s.sel.empty() || s.sys == nil {
		return
	}
	abs, err := s.ws.AbsOf(p.ID, s.sel.path)
	if err != nil {
		s.status = "无法复制文件路径：" + err.Error()
		return
	}
	s.sys.CopyText(abs)
	s.status = "已复制文件路径：" + abs
}
```

`view.go` 的 `contentTitle` 在路径文本与 `s.modeToggle(c)` 之间加（图标只在有文档时出现，因为整块都在 `path != ""` 里）：

```go
			if components.IconButton(c, lucide.Copy(), "复制文件路径", components.IconButtonCompact).Clicked() {
				s.copyDocPath()
			}
```

- [ ] **步骤 4：运行测试验证通过**

运行：`go test ./internal/view -run 'TestTitle|TestShellSlotsBuild' -v`
预期：PASS。

- [ ] **步骤 5：Commit**

```bash
git add internal/view/fileacts.go internal/view/view.go internal/view/layout_test.go
git commit -m "feat: 标题栏文件名后加复制路径按钮"
```

---

### 任务 4：文件树右键菜单与 Cmd/Ctrl+C

**文件：**
- 修改：`internal/view/view.go`（`treeItems` 加 `nodeMenu`；`View`、`Content` 首行调 `handleShortcuts`）
- 修改：`internal/view/fileacts.go`（`copyNodeName` / `copyNodePath` / `revealNode` / `handleShortcuts`）
- 测试：`internal/view/view_test.go`

- [ ] **步骤 1：编写失败的测试**

在 `view_test.go` 里 `fakeSystem` 之后加（`ui`、`os`、`path/filepath`、`strings` 已在 import 里，另需给 import 块加标准库 `slices`）：

```go
// TestTreeFileMenuCopiesTheName pins the file's two entries: its name, and its
// absolute path.
func TestTreeFileMenuCopiesTheName(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "notes/a.md", "# A\n")
	s.Refresh()
	sys := &fakeSystem{}
	s.SetSystem(sys)

	tt := newTester(s)
	if err := tt.RightClick("a.md"); err != nil {
		t.Fatalf("RightClick: %v", err)
	}
	if got, want := tt.Menu(), []string{"复制文件名称", "复制文件路径"}; !slices.Equal(got, want) {
		t.Fatalf("menu = %q, want %q", got, want)
	}
	if err := tt.Click("复制文件名称"); err != nil {
		t.Fatalf("Click: %v", err)
	}
	if sys.copied != "a.md" {
		t.Errorf("copied = %q, want a.md", sys.copied)
	}
	if !strings.Contains(s.Status(), "已复制文件名称") {
		t.Errorf("status = %q, want the copy reported", s.Status())
	}
}

// TestTreeDirMenuCopiesThePathAndRevealsTheFolder pins the directory's three
// entries, the path being the directory's own.
func TestTreeDirMenuCopiesThePathAndRevealsTheFolder(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "notes/a.md", "# A\n")
	s.Refresh()
	sys := &fakeSystem{}
	s.SetSystem(sys)

	tt := newTester(s)
	if err := tt.RightClick("notes"); err != nil {
		t.Fatalf("RightClick: %v", err)
	}
	if got, want := tt.Menu(), []string{"复制文件名称", "复制文件路径", "-", "在文件管理器中打开"}; !slices.Equal(got, want) {
		t.Fatalf("menu = %q, want %q", got, want)
	}
	if err := tt.Click("复制文件路径"); err != nil {
		t.Fatalf("Click: %v", err)
	}
	if want := filepath.Join(root, "notes"); sys.copied != want {
		t.Errorf("copied = %q, want %q", sys.copied, want)
	}
	if err := tt.RightClick("notes"); err != nil {
		t.Fatalf("RightClick again: %v", err)
	}
	if err := tt.Click("在文件管理器中打开"); err != nil {
		t.Fatalf("Click: %v", err)
	}
	if want := filepath.Join(root, "notes"); sys.revealed != want {
		t.Errorf("revealed = %q, want %q", sys.revealed, want)
	}
}

// TestTreeDirMenuReportsARevealFailure pins that a file manager that cannot
// open the directory says so in the status bar instead of failing silently.
func TestTreeDirMenuReportsARevealFailure(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "notes/a.md", "# A\n")
	s.Refresh()
	s.SetSystem(&fakeSystem{revealErr: os.ErrNotExist})

	tt := newTester(s)
	if err := tt.RightClick("notes"); err != nil {
		t.Fatalf("RightClick: %v", err)
	}
	if err := tt.Click("在文件管理器中打开"); err != nil {
		t.Fatalf("Click: %v", err)
	}
	if !strings.Contains(s.Status(), "文件管理器") {
		t.Errorf("status = %q, want the failure reported", s.Status())
	}
}

// TestTreeMenuWithoutDesktopDoesNothing pins that a tree without a System to
// copy through is a no-op, not a panic.
func TestTreeMenuWithoutDesktopDoesNothing(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "notes/a.md", "# A\n")
	s.Refresh()

	tt := newTester(s)
	if err := tt.RightClick("a.md"); err != nil {
		t.Fatalf("RightClick: %v", err)
	}
	if err := tt.Click("复制文件路径"); err != nil {
		t.Fatalf("Click: %v", err)
	}
	if s.Status() != "" {
		t.Errorf("status = %q, want nothing claimed to have been copied", s.Status())
	}
}

// TestCmdCCopiesTheDocumentPath pins Cmd/Ctrl+C: the document shown, its
// absolute path.
func TestCmdCCopiesTheDocumentPath(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "notes/a.md", "# A\n")
	s.Refresh()
	s.selectDoc("notes/a.md")
	sys := &fakeSystem{}
	s.SetSystem(sys)

	tt := newTester(s)
	tt.Key(ui.Cmd, ui.KeyC)
	if want := filepath.Join(root, "notes", "a.md"); sys.copied != want {
		t.Errorf("copied = %q, want %q", sys.copied, want)
	}
	if !strings.Contains(s.Status(), "已复制文件路径") {
		t.Errorf("status = %q, want the copy reported", s.Status())
	}
}

// TestCmdCCopiesNothingWithoutADocument pins that the shortcut does not claim
// to copy a path when no document is shown.
func TestCmdCCopiesNothingWithoutADocument(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "notes/a.md", "# A\n")
	s.Refresh()
	sys := &fakeSystem{}
	s.SetSystem(sys)

	tt := newTester(s)
	tt.Key(ui.Cmd, ui.KeyC)
	if sys.copied != "" {
		t.Errorf("copied = %q, want nothing copied", sys.copied)
	}
	if s.Status() != "" {
		t.Errorf("status = %q, want nothing claimed to have been copied", s.Status())
	}
}
```

同一批测试里还要断言文件菜单**没有**「在文件管理器中打开」——它已经由 `TestTreeFileMenuCopiesTheName` 的 `tt.Menu()` 全列表断言覆盖，不再重复。

- [ ] **步骤 2：运行测试验证失败**

运行：`go test ./internal/view -run 'TestTree|TestCmdC' -v`
预期：FAIL —— 右键后 `menu = []`（树项还没挂菜单），`Click: no target`；`TestCmdC*` 因为还没注册快捷键而 `copied = ""`（`TestCmdCCopiesNothingWithoutADocument`、`TestTreeMenuWithoutDesktopDoesNothing` 这两个是 no-op 守卫，此时通过）。

- [ ] **步骤 3：实现**

`view.go` 的 `treeItems`：目录与文件都挂同一个 `nodeMenu`：

```go
func (s *State) treeItems(c *ui.Context, tr projectTree) {
	for _, child := range tr.root.Children {
		child := child
		if child.IsDir {
			item := ui.TreeItem(c, child.Name, s.openOf(tr.project.ID+":"+child.Path), func() {
				s.treeItems(c, tr.childOf(child))
			})
			s.nodeMenu(c, item, tr, child)
			continue
		}
		selected := s.sel.project == tr.project.ID && child.Path == s.sel.path
		item := ui.TreeItem(c, child.Name, nil, nil).Selected(selected)
		if item.Clicked() {
			s.selectDocIn(tr.project.ID, child.Path)
		}
		s.nodeMenu(c, item, tr, child)
	}
}

// nodeMenu gives a tree entry its right click: copying its name or its path,
// and for a directory, showing it in the file manager.
func (s *State) nodeMenu(c *ui.Context, item ui.Element, tr projectTree, n *workspace.Node) {
	components.ContextMenu(c, item, func(m *components.Menu) {
		if m.Item("复制文件名称").Chosen() {
			s.copyNodeName(n)
		}
		if m.Item("复制文件路径").Chosen() {
			s.copyNodePath(tr, n)
		}
		if !n.IsDir {
			return
		}
		m.Separator()
		if m.Item("在文件管理器中打开").Chosen() {
			s.revealNode(tr, n)
		}
	})
}
```

`view.go` 的 `View` 与 `Content` 首行各加 `s.handleShortcuts(c)`（在 `s.drain()` / 构建之前）：

```go
func (s *State) View(c *ui.Context) {
	s.drain()
	s.handleShortcuts(c)
	…
}

func (s *State) Content(c *ui.Context) {
	s.handleShortcuts(c)
	…
}
```

`fileacts.go` 追加（签名与语义，实现者照写）：

```go
// copyNodeName puts a tree entry's name in the clipboard.
func (s *State) copyNodeName(n *workspace.Node) {
	if s.sys == nil {
		return
	}
	s.sys.CopyText(n.Name)
	s.status = "已复制文件名称：" + n.Name
}

// copyNodePath puts a tree entry's absolute path in the clipboard.
func (s *State) copyNodePath(tr projectTree, n *workspace.Node) {
	if s.sys == nil {
		return
	}
	abs, err := s.absOf(tr, n)
	if err != nil {
		s.status = "无法复制文件路径：" + err.Error()
		return
	}
	s.sys.CopyText(abs)
	s.status = "已复制文件路径：" + abs
}

// revealNode shows a directory of the tree in the file manager.
func (s *State) revealNode(tr projectTree, n *workspace.Node) {
	if s.sys == nil {
		return
	}
	abs, err := s.absOf(tr, n)
	if err != nil {
		s.status = "无法打开文件管理器：" + err.Error()
		return
	}
	if err := s.sys.Reveal(abs); err != nil {
		s.status = "无法打开文件管理器：" + err.Error()
		return
	}
	s.status = "已在文件管理器中打开：" + abs
}

// absOf is a tree entry's absolute path on disk.
func (s *State) absOf(tr projectTree, n *workspace.Node) (string, error) {
	return s.ws.AbsOf(tr.project.ID, n.Path)
}

// handleShortcuts copies the document shown for the platform's Cmd+C, which
// the title bar's button does too.
func (s *State) handleShortcuts(c *ui.Context) {
	if c.Shortcut(ui.Cmd, ui.KeyC) {
		s.copyDocPath()
	}
}
```

`fileacts.go` 的 import 块只要 `github.com/egoist/mygo/ui` 与 `mdpreview/internal/workspace`。`copyNodeName` / `copyNodePath` / `revealNode` / `copyDocPath` 在 `s.sys == nil` 时**先返回、不写 status**，`TestTreeMenuWithoutDesktopDoesNothing` 与 `TestCmdCCopiesNothingWithoutADocument` 靠这一点断言「没有声称复制过」。

- [ ] **步骤 4：运行测试验证通过**

运行：`go test ./internal/view -v && go test ./...`
预期：PASS。`TestCmdCCopiesNothingWithoutADocument` 能通过的前提是 `copyDocPath` 在 `s.sel.empty()` 时提前返回、不写 `status`。

- [ ] **步骤 5：Commit**

```bash
git add internal/view/view.go internal/view/fileacts.go internal/view/view_test.go
git commit -m "feat: 文件树右键复制名称/路径并在此打开，Cmd+C 复制文档路径"
```

---

## 收尾

- [ ] `go vet ./... && gofmt -l .` 无输出。
- [ ] `go test ./...` 全绿（含 `TestShellSlotsBuild`，证明 shell 各槽位与 `View` 两条路径都同步）。
- [ ] `go run .` 手动验证四项：目录面板上下左右滚；标题栏是一个分段器；点复制图标后状态栏报「已复制文件路径」；树上右键目录/文件菜单项与 `Ctrl+V` 粘贴结果一致，`Ctrl+C` 复制当前文档绝对路径。
- [ ] README「已知限制」不需要改（本次不引入新的限制）；如用户要求把 `在文件管理器中打开` 改成 `在文件浏览器中打开`，只改 `nodeMenu` 里一个字符串 + `TestTreeDirMenuCopiesThePathAndRevealsTheFolder` 的期望列表。

---

## 执行备注（事后）

实现在 `feat/toc-scroll-title-actions` 上按本计划完成（6 个提交）。三处计划本身的偏差，已在当时裁决并落地：

1. **任务 2 步骤 2 的预期不成立**：旧分段按钮的可见文字本来就是 `WebView` / `原生`，tester 点文本即可命中，所以改完点击目标后 `TestModeToggleSitsWithTheDocumentName` 直接通过。保留该测试（断言模式真的切换），不再为它造 RED。
2. **`TestTitleFitsNarrowWindow` 的原断言恒真**：tester 报告的文本矩形按窗口裁过，`rect.X+rect.W > 280` 永远不成立。改为「窄窗口下 `WebView` 段的可见宽度必须等于宽窗口下的自然宽度」（16.2 vs 45.3 DIP，差距足够明显）。
3. **`tt.Menu()` 看不到 Onevra 的菜单**：tester 的 `Menu()` 只读系统菜单，而 `components.ContextMenu` 在窗口内自绘、条目是真实元素。改为 `tt.HasText(...)` 逐项断言，并单独断言文件菜单里**没有**「在文件管理器中打开」。

最终审查（作者自审）另外修了两处：

4. **目录滚动位置原来在任何重载时归零**（外部保存、切渲染后端都会让目录跳回顶部）。归零动作从 `loadDoc` 移到 `selectDocIn`：换文档才归零，同一文档重载保留位置（`TestTOCScrollSurvivesAReloadButNotADocumentChange`）。
5. **标题栏图标与树菜单共用「复制文件路径」这个无障碍名**：读屏与 tester 都无法区分，实测点树菜单项会命中标题栏图标。图标改名为「复制文档路径」（`TestTreeMenuCopiesTheClickedFileWhileADocumentShows`）。

另外 README 记录了新的树右键菜单、复制路径与 `Cmd/Ctrl+C`（计划的「收尾」里写的是「README 不需要改」，实际需要）。
