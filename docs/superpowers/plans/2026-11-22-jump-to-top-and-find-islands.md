# 目录跳转（顶到顶部 / 落到底部 + 平滑滚动）与搜索面板浮岛 实现计划

> **面向 AI 代理的工作者：** 必需子技能：使用 subagent-driven-development（推荐）或 executing-plans 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** 点击目录（以及正文里的 `#标题` 链接、搜索命中的第一行）时，把目标章节**平滑**滚到正文顶部；章节靠近文档末尾、顶不到顶部时，正文滚到**最底部**而不是停在半截。同时把悬浮搜索面板从"一整块卡片里叠三段"改成三块各自独立的浮岛（输入框 / 匹配行 / 上下文），块与块之间留空，匹配列表与上下文预览不再糊成一列。

**架构：** 跳转只留一条路。`State` 记一个待跳的块号 `jump`，原生正文在**有跳转的那一帧**把 `markdown.Options.Block` 挂上一个钩子：钩子在目标块自己的 Column 里插一个零高标记，并给标记调 `ScrollIntoView(ui.ScrollIntoViewOptions{Block: ui.ScrollStart, Animation: &ui.Smooth})`——mygo 的 reveal 会把目标块的顶边对到滚动容器的顶边、按需平滑过渡、并把结果钳制在 `MaxY`（于是"近末尾"自动变成"落到最底部"）。`markdown.Options.Anchor` / `AnchorVersion` / `AnchorBlock` 与 `find:` 锚点前缀全部退役；WebView 后端改用 `Page().Eval` 的 `scrollIntoView({behavior:'smooth',block:'start'})`，地址里不再塞 `#slug`，因此点目录不再重载页面。搜索面板的三段各自包一层 `findIsland`（圆角 + 边框 + 投影 + 留白），外层不再有整块背景，只用 `Gap` 把浮岛分开。

**技术栈：** Go 1.27+、`github.com/egoist/mygo v0.9.1`（`ui.ScrollState.ScrollIntoView` / `ScrollIntoViewOptions` / `ScrollStart` / `ui.Smooth`、`ui.Box`、`ui.Overlay`、`ui.ScrollBoth`、`markdown.Options.Block`、`mygo.Page.Eval`、`ui.Tester.Advance` / `SetPreferences`）、`github.com/oneo-me/onevra/source v0.2026.18`（`components.Text`、`components.IconButton` + `IconButtonCompact`、`icons/lucide`）。不新增依赖。

**规格：** `docs/superpowers/specs/2026-11-21-markdown-preview-design.md`（其中"原生模式 TOC 跳转 = `Options.Anchor`"与"webview TOC 跳转 = `LoadURL(#slug)`"两条被本计划取代，任务 4 回写）。需求来自用户 2026-11-22 的两条反馈与两张截图：① 目录点击后"当前章节能顶到顶部就顶到顶部，接近尾部顶不到就滚到最底部"，并且"增加平滑滚动，目前是直接跳过去的"；② 悬浮搜索"待选列表与 preview 预览没有明确的分割，容易造成视觉上的混淆"，要求采用 **IsLands** 的视觉效果。

## 设计决策（实施者按此执行，不要另立方案）

| # | 决策 | 理由 |
| --- | --- | --- |
| D1 | 原生跳转的唯一入口是 `State.jump int`（待跳的块号，0 = 没有）。目录、正文内 `#slug` 链接、搜索命中都设它，没有第二种滚动路径。 | 现在目录走 `md.Anchor`（mygo 里是 `ScrollIntoView()`，即 `ScrollNearest` + 无动画：就近露一点、一帧跳过去），搜索走 `AnchorBlock` + `find:` 前缀。两套都在 mygo 内部，拿不到"顶到顶部 + 平滑"。收成一套，顺带删掉 `find:` 前缀这块历史包袱。 |
| D2 | 滚动靠目标块自带的**零高标记**：`markdown.Options.Block(c, d, i)` 钩子在 `i == jump` 时 `ui.Box(c).Height(0).Width(0)`，对它调 `ScrollIntoView(ui.ScrollIntoViewOptions{Block: ui.ScrollStart, Animation: &ui.Smooth})`，然后 `return false` 让 view 照常画这个块。 | 已用探针实测（60 个 20 DIP 的行，容器 200 高）：标记在第 40 行 → 停在 `Y=800`（该行正好顶到容器顶边）；标记在第 58 行（目标 1160 > MaxY 1000）→ 停在 `Y=1000`，即最底部；有标记与无标记时 `MaxY` 完全相同（零高不参与布局）。真实 markdown 文档里同样成立：`## 第 30 节` 的块 → 标题落在 `Y=0`。mygo 的 `st.block` 会先建一个 `ui.Column`（块自己的盒子）再调 `Block` 钩子，所以标记天然落在"这个块的最顶上"，不会差一个 margin。 |
| D3 | `Options.Block` **只在有跳转的帧挂上**（`nativePreview` 里 `s.md.Block = nil`，`s.jump > 0` 时才设成 `s.jumpMarker`），跳完就摘。 | mygo 的 `viewState.block` 一旦看到 `o.Block != nil`，会给**每个块**多包一层 `ui.Column` 再调钩子。常挂等于给几千个块各加一个无用的布局节点，是每帧的固定开销；而这个钩子一年也用不了几次。 |
| D4 | `jumpMarker` 在目标块被建出来时**立刻**清掉 `s.jump`，并加一条保险：`i > s.jump` 时也清（块按索引递增建，越过就说明这次跳不到了，比如文档被换掉）。 | reveal 的登记发生在构建期、应用发生在布局之后，所以当场清状态不会取消这次滚动（探针已验证）。清掉之后下一帧就回到"没有钩子"的常态，不会因为一个失效块号永远挂着一个钩子。 |
| D5 | 换文档时 `s.jump` 与 `s.anchor` 一起清掉（`openTab` 清 `anchor`，`loadDoc` 清 `jump`）。表行的命中先按 mygo 的老规矩上提到它的 `Table` 块再跳。 | 块号是"这份文档的第几个块"，带到另一份文档里就是乱跳。表格的 row/cell 是表格自己画的，拿它们当标记会塞进单元格里，上提到 `Table` 才安全（mygo 的 `anchorNamed` 也是这么处理的）。 |
| D6 | `anchor` 的含义改成"目录高亮的那一项 / WebView 要滚到的标题"，**不再驱动原生滚动**；`anchorVer`、`s.md.Anchor`、`s.md.AnchorVersion`、`s.md.AnchorBlock`、`anchorBlock()`、`findAnchorPrefix` 全部删除。 | 跳转改由 `jump` 表达之后，这几个字段只剩"强制再滚一次"的用途（`AnchorVersion` 的唯一作用就是让同一个标题点第二次还能滚）。留着会让人以后误以为目录还是走它们。`anchor` 留着是因为目录项的 `Selected()` 与 WebView 跳转还要它。 |
| D7 | WebView 的标题跳转改成 `Page().Eval(headingScrollJS(slug))`，其中 `headingScrollJS` 返回 `document.getElementById(<slug>).scrollIntoView({behavior:'smooth',block:'start'})`；`loadWebDoc` 不再把 `#slug` 拼进地址。 | 现在的 `LoadURL(url + "#" + slug)` 是**整页重载**：既不是平滑滚动，还丢掉滚动位置（README 已列为已知限制）。页内 `scrollIntoView` 的 `block:'start'` + 浏览器自身的钳制，与原生行为逐条对齐（顶到顶部、近末尾落到底、平滑）。slug 不是合法 id（或页面还没加载完）时 Eval 什么都不做，不改状态栏以外的一切。 |
| D8 | 搜索面板由**一块**带背景/边框/投影的 `ui.Column` 变成**三块独立的浮岛**：输入框岛、匹配行岛、上下文岛；外层容器只剩 `Absolute().Right().Top().Width(findPaneWidth).Gap(...)`，没有背景。 | 用户原话"待选列表与 preview 预览没有明确的分割，容易造成视觉上的混淆"。一列行 + 一块浅底色的卡片，边界只有 4 DIP 的内边距和一点色差，确实读不出"这是两个东西"。浮岛把三者放到同一层级：各自圆角、各自投影，中间露出正文。 |
| D9 | 三块浮岛共用 `findIsland(c, pad)`：`FillWidth` + `Radius(t.Radius*1.5)` + `Background(t.Surface)` + `Border(1, t.Border)` + `Shadow(0, Space(0.5), Space(1.5), 0, t.Text.Alpha(0.10))` + `Padding(pad)`；岛间 `Gap(t.Space(1.5))`。 | 重复四次（含两处内层）的样式必须收进一个函数，否则以后调一处漏一处。颜色全部来自 `AppTheme`，不写死十六进制（AGENTS 硬规则）。 |
| D10 | 上下文岛顶部加一行说明 `第 N 行上下文`（10px、`TextMuted`），并给两个岛加无障碍名 `Label("匹配行")` / `Label("命中上下文")`。 | "哪块是列表、哪块是预览"光靠底色分不出来，得有一句话命名；`Label` 同时是测试里取岛矩形（`tt.Find`）的唯一办法，也是读屏软件念得出来的名字（AGENTS 要求图标按钮给 label，这里同理）。 |
| D11 | 不用 `plugins/glass` 的液态玻璃材质，用主题的 `Surface` + 边框 + 投影。 | 面板浮在正文上，而正文在 WebView 模式下是**原生 web view**：玻璃要采样底下的像素，跨原生视图的取样不保证正确，可能露出一块死色或旧帧。浮岛的"分离感"来自留白 + 投影，平涂已经够；要玻璃的话是 `findIsland` 里加一行 `Material(...)`，见「未决问题」。 |
| D12 | 平滑滚动尊重系统的"减少动态效果"：`ui.Smooth` 自带（`ReduceMotion` 时直接到位），本项目不另加开关，也不写自己的动画。 | mygo 的 `reveal` 里 `smooth := o.Animation != nil && !rt.preferences().ReduceMotion && !rt.instant()`，白拿；自己写插值会和滚动容器抢 `scrollY`。 |

### 改完长什么样（验收画面）

```
搜索面板 = 三块浮岛，浮在正文右上；块与块之间留 6 DIP，露出正文档

目录                                ┌ ⌕ onnx ────────── 4/56 ─ ↑ ↓ ✕ ┐  ← 岛 1：输入框
▾ 一                                └──────────────────────────────────┘
  1. 子项                           ┌ 匹配行（自己滚）─────────────────┐  ← 岛 2：命中列表
▾ 二                                │  80 │ 推理引擎 Rust 直接调 ONNX… │
  2. 子项                           │ 138 │ 云端推理部署 Go 同进程…    │
▸ 三                                └──────────────────────────────────┘
                                    ┌ 第 138 行上下文（自己滚）────────┐  ← 岛 3：上下文
                                    │ 136 │ ----                       │
                                    │ 137 │ 车端模型档位 默认 `s@640`   │
                                    │ 138 │ 云端推理部署 Go 同进程…    │ ← 命中片段主色
                                    └──────────────────────────────────┘
```

点击目录「二」之后正文的样子（原生后端）：

```
┌ 路径 a.md ─ 复制 ─ 原生|WebView ┐
│ 二            ← 这个标题贴在正文顶部（Y=0）
│ 2. 子项
│ 正文…
```

## 全局约束

- 不新增依赖、不新增第三方包、不加 `//go:build` 平台专用代码；`internal/workspace` 与 `internal/mcp` 不 import `.../ui`。
- 颜色只从 `AppTheme`（`c.Theme()`）取，不写死十六进制；图标只用 `icons/lucide`；图标按钮必须给 `label`。
- 视图状态归窗口线程；本计划不新增 goroutine，不新增热路径（跳转是事件驱动的，`blockOfHeading` 只在点击时走一遍块表）。
- 测试只用 `t.TempDir()`；不碰真实用户目录、不碰真实 WebView（无窗口时走 `s.wv == nil` 的降级分支）。
- 每个行为改动**先有失败的测试**；文档回写（README / AGENTS / CHANGELOG / 规格）并入同一个提交，不单开一次"补文档"。
- 提交信息中文，前缀 `feat:` / `fix:` / `test:` / `docs:` / `style:`。提交前 `go vet ./... && gofmt -l . && go test ./...` 必须干净。

## 文件结构

| 文件 | 职责 | 本计划里的改动 |
| --- | --- | --- |
| `internal/view/state.go` | `State` 字段与文档选择 | 加 `jump int`；删 `anchorVer`；改 `anchor` 的注释；`openTab` 清 `anchor` |
| `internal/view/native.go` | 原生后端、目录、`loadDoc` | `loadDoc` 不再挂 `Anchor` / `AnchorVersion` / `AnchorBlock`，并清 `jump`；`gotoHeading` 改走 `jumpToBlock`；新增 `jumpToBlock` / `blockOfHeading` / `jumpMarker`；`nativePreview` 按需挂 `md.Block` |
| `internal/view/find.go` | 悬浮搜索、跳转、面板 | `gotoMatch` 改走 `jumpToBlock`；删 `findAnchorPrefix` / `anchorBlock`；面板改成三岛 + `findIsland` / `findCaption` |
| `internal/view/webview.go` | WebView 后端 | 新增 `scrollWebTo` / `headingScrollJS`；`loadWebDoc` 不再拼 `#slug` |
| `internal/view/native_test.go` | 目录跳转 | `TestTOCClickSetsAnchor` 改为断言 `jump`；新增顶到顶部 / 近末尾到底 / 平滑 / 减少动态效果四例 |
| `internal/view/find_test.go` | 搜索面板与跳转 | 删除 `anchorBlock` 两例；跳转断言改 `jump`；新增浮岛间距与命名两例 |
| `internal/view/webview_test.go` | 页面地址与 JS | 新增 `headingScrollJS` 单测；`loadWebDoc` 不再带 `#` |
| `internal/view/layout_test.go` | 布局 | `TestTOCRowsFillThePane` 里 `anchor` 断言保持；无其他改动（确认跑绿） |
| `README.md` / `AGENTS.md` / `CHANGELOG.md` / 规格 | 文档回写 | 见各任务最后一步与任务 4 |

## 审查重点（Review Focus）

1. **章节在文档两端**：第一个标题（正文已在顶部）不能滚出负数；最后一个标题（或比一屏还高的最后一段）必须停在 `MaxY`（最底部），而不是"标题刚露一行"的半截位置。→ 任务 1 的 `TestTOCJumpPutsTheHeadingAtTheTop` / `TestTOCJumpNearTheEndLandsAtTheBottom`。
2. **平滑途中改主意**：动画还在走时点另一个标题（或搜索的"下一个"连点），要转向新目标，不能钉死在旧动画上、也不能越滚越远。→ 任务 1 的 `TestSecondJumpTurnsTowardTheNewTarget`。
3. **系统要求减少动态效果**：`ReduceMotion` 打开时直接到位（一帧内 `Y` 就是终值），没有动画、没有残留的待跳状态。→ 任务 1 的 `TestJumpWithoutMotionWhenAsked`。
4. **待跳的块号跨文档泄漏**：点了目录后马上切标签 / 文件被外部改写重载，新文档不能自己滚到别人的第 12 块。→ 任务 1 的 `TestJumpDoesNotCarryToAnotherDocument`。
5. **搜索面板在窄窗口与长列表下**：三块浮岛不重叠、不超出面板宽度、各自能滚；没有匹配时只剩输入框岛（不留两个空岛）。→ 任务 3 的 `TestFindPanelShowsIslandsApart` / `TestFindPanelWithoutMatchesKeepsOneIsland`。

---

### 任务 1：原生跳转只有一条路——目录、正文内链接与搜索命中都顶到顶部，近末尾落到底部，平滑过去

**文件：**
- 修改：`internal/view/state.go`（`jump` 字段、删 `anchorVer`、`openTab` 清 `anchor`）
- 修改：`internal/view/native.go`（`loadDoc`、`gotoHeading`、`jumpToBlock`、`blockOfHeading`、`jumpMarker`、`nativePreview`）
- 修改：`internal/view/find.go`（`gotoMatch` 改走 `jumpToBlock`；删 `findAnchorPrefix` / `anchorBlock`）
- 修改：`internal/view/webview.go`（`loadWebDoc` 去掉 `findAnchorPrefix` 判断）
- 测试：`internal/view/native_test.go`、`internal/view/find_test.go`
- 文档：`README.md`（用法里的目录一行）、`AGENTS.md`（坑：跳转只有一条路）、`CHANGELOG.md`（未发布 / 变更）

目录、正文内 `#标题` 链接、搜索命中三处跳转**一次**收到 `State.jump` 上：`anchorVer` / `md.Anchor*` / `find:`
前缀删除后，`find.go` 与 `webview.go` 必须同步改，否则中间状态编译不过。

- [ ] **步骤 1：编写失败的测试**

`internal/view/native_test.go`：

```go
// sections is a document of n sections, each a heading "第 i 节" and a dozen
// lines that start with "P<i> 正文": the heading is what a test clicks in the
// outline, and the line is where the body shows it. A dozen lines a section
// makes the body several screens tall while the outline keeps only n rows, so
// that every heading stays clickable.
func sections(n int) string {
	var b strings.Builder
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&b, "## 第 %d 节

", i)
		for line := 1; line <= 12; line++ {
			fmt.Fprintf(&b, "P%d 正文，第 %d 行

", i, line)
		}
	}
	return b.String()
}

func TestTOCJumpPutsTheHeadingAtTheTop(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "a.md", sections(8))
	s.Refresh()
	s.selectDoc("a.md")
	tt := newTester(s)
	if err := tt.Click("第 6 节"); err != nil {
		t.Fatalf("Click: %v", err)
	}
	tt.Frame()
	tt.Advance(time.Second)
	tt.Frame()
	if s.docScroll().Y <= 0 {
		t.Fatalf("Y = %v, want the body scrolled", s.docScroll().Y)
	}
	rect, ok := tt.Find("P6 正文，第 1 行")
	if !ok {
		t.Fatalf("texts = %q, want the body under the heading", tt.Texts())
	}
	if rect.Y > 200 {
		t.Errorf("body at Y=%v, want it near the pane's top", rect.Y)
	}
}

// TestTOCJumpNearTheEndLandsAtTheBottom: 点 "第 8 节"（它的正文短，顶不到顶部），断言 Y 与 MaxY
// 相差不到 0.5（浮点容差），且 "P8 正文，第 1 行" 仍在画面内（不要滚过正文）。

// TestTOCJumpGlides: 点 "第 6 节"，这一帧 Y 仍是 0 且 tt.FramePending() 为 true；
// Advance(time.Second) + Frame 之后 Y 到位、FramePending 为 false。

// TestJumpWithoutMotionWhenAsked: tt.SetPreferences(ui.Preferences{ReduceMotion: true})，
// 点 "第 6 节" 后第一帧 Y 就已是终值（与平滑用例的终值一致）。
```

同时把 `TestTOCClickSetsAnchor` 换成只建目录树的版本（不建正文，跳过的块号才留得住）：

```go
func TestTOCClickSetsTheBlockToJumpTo(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "a.md", "# 一\n\n## 二\n\n正文一\n")
	s.Refresh()
	s.selectDoc("a.md")
	// Only the outline: the body would build the block and take the jump.
	tt := ui.NewTester(func(c *ui.Context) { s.toc(c) }, 240, 400)
	if err := tt.Click("二"); err != nil {
		t.Fatalf("Click: %v", err)
	}
	if s.anchor != "二" {
		t.Errorf("anchor = %q, want 二", s.anchor)
	}
	// The outline asks for the heading's block; the body's own build of that
	// block is what consumes it.
	if want := blockOfHeading(s.doc, "二"); want == 0 || s.jump != want {
		t.Errorf("jump = %d, want the heading's block %d", s.jump, want)
	}
}
```

- [ ] **步骤 2：编写搜索跳转的测试（同一个任务，同一条路）**

`internal/view/find_test.go`：

```go
// TestFindJumpPutsTheMatchAtTheTop: 命中的段落顶到正文顶部，且 state 里不留锚点。
func TestFindJumpPutsTheMatchAtTheTop(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "a.md", sections(8))
	s.Refresh()
	s.selectDoc("a.md")
	openFindOn(s, "P8 正文，第 1 行")
	tt := newTester(s)
	s.gotoMatch(0)
	tt.Frame()
	tt.Advance(time.Second)
	tt.Frame()
	if s.jump != 0 {
		t.Errorf("jump = %d, want it taken by the block's build", s.jump)
	}
	if s.docScroll().Y <= 0 {
		t.Fatalf("Y = %v, want the body scrolled to the hit", s.docScroll().Y)
	}
}

// TestSecondJumpTurnsTowardTheNewTarget: 第一次跳还没走完就换目标，后一次覆盖前一次。
// TestJumpDoesNotCarryToAnotherDocument: 设了 jump 再切文档，新文档不自已滚。
```

删掉两例：`TestAnchorBlockLeavesHeadingsAlone`、`TestFindAnchorDoesNotCarryToAnotherDocument`（后者由 `TestJumpDoesNotCarryToAnotherDocument` 取代）；`TestFindJumpScrollsTheNativeBody` 改成断言 `s.jump == blockAt(s.doc, s.find.matches[0].Offset)`。

- [ ] **步骤 3：运行测试验证失败**

运行：`go test ./internal/view/ -run 'TestTOCJump|TestTOCClickSetsTheBlock|TestFindJump|TestJumpDoesNot|TestSecondJump' -v`
预期：FAIL / 编译错误——`s.jump`、`blockOfHeading`、`sections` 都还不存在，且现有点击后 `s.docScroll().Y` 一直是 0。

- [ ] **步骤 4：在 `internal/view/state.go` 加 `jump`、删 `anchorVer`**

```go
	// anchor is the heading the outline marks as current: the one last jumped
	// to. The native body keeps its place by jump, not by this.
	anchor string
	// jump is the block of the document shown that the native body brings to
	// the top of its pane, 0 for none. The outline, a link to a heading and a
	// search hit ask for one; the markdown view builds that block with a
	// marker that scrolls the body there (native.go's jumpMarker).
	jump int
```

`openTab`（`tabs.go`）在切换文档时清掉高亮：与清滚动位置的地方保持一致，加 `s.anchor = ""`。

- [ ] **步骤 5：在 `internal/view/native.go` 实现跳转**

```go
// jumpToBlock brings block b of the document shown to the top of the body's
// pane, gliding there. Where the document ends before the block reaches the
// top, the body stops at its end: the scroll container keeps the place within
// what it has.
func (s *State) jumpToBlock(b int) { … 表格行/单元格上提到 Table；越界或 d == nil 直接返回；s.jump = b … }

// blockOfHeading returns the block of d that holds the heading of slug, 0 when
// the document has none. It is how a heading, which the outline names by its
// slug, becomes the block the body scrolls to.
func blockOfHeading(d *parser.Document, slug string) int { … 逐块找 Kind==parser.Heading，比 markdown.Slug(parser.PlainText(d.Inlines(i))) == strings.ToLower(slug) … }

// jumpMarker is the markdown view's block hook: for the block the body is to
// bring to the top it builds a marker of no size at that block's own start and
// asks the pane to line the marker up with its top, gliding; the block itself is
// the view's to build, so it reports false.
func (s *State) jumpMarker(c *ui.Context, d *parser.Document, i int) bool { … }
```

`gotoHeading` 改成（WebView 分支暂时什么都不做，任务 2 在那里接上页内平滑滚动）：

```go
func (s *State) gotoHeading(slug string) {
	slug = strings.TrimPrefix(strings.TrimSpace(slug), "#")
	if slug == "" || s.doc == nil {
		return
	}
	s.anchor = slug
	if s.mode() == config.RenderWebView {
		// 任务 2 在这里接上页内平滑滚动。
		return
	}
	s.jumpToBlock(blockOfHeading(s.doc, slug))
}
```

`loadDoc` 里删掉这三行与锚点回落逻辑，改为清 `jump`：

```go
	s.md.Anchor = s.anchor
	s.md.AnchorVersion = s.anchorVer
	s.md.AnchorBlock = s.anchorBlock
```

```go
	// A jump names a block of the document it was asked in: another document
	// opens where its own tab was left.
	s.jump = 0
```

`nativePreview` 在设 `s.md.Theme` 的地方按需挂钩子：

```go
	// The hook wraps every block in a box of its own, so it is hung only on
	// the frames a jump is on its way.
	s.md.Block = nil
	if s.jump > 0 {
		s.md.Block = s.jumpMarker
	}
```

- [ ] **步骤 6：在 `internal/view/find.go` 把命中跳转接到同一条路**

`gotoMatch` 的原生分支换成：

```go
	// The anchor is the block, not the match: a scroll is all the native
	// view does, and one block is as close as it gets.
	s.jumpToBlock(blockAt(s.doc, s.find.matches[i].Offset))
```

删掉 `findAnchorPrefix` 常量、`anchorBlock` 方法、以及 `gotoMatch` 里对 `s.anchor` / `s.anchorVer` / `s.md.Anchor` 的赋值。
再改 `internal/view/webview.go` 的 `loadWebDoc`：前缀常量没了，地址拼 `#slug` 的判断改成 `if s.anchor != ""`（任务 2 把这段整体换成 `Page().Eval`）。

- [ ] **步骤 7：运行测试验证通过**

运行：`go test ./internal/view/ -run 'TestTOCJump|TestTOCClickSetsTheBlock|TestFindJump|TestJumpDoesNot|TestSecondJump' -v`
预期：PASS（含 `TestTOCJumpNearTheEndLandsAtTheBottom` 的 `Y == MaxY`、`TestJumpWithoutMotionWhenAsked` 一帧到位）。

再跑一次全量：`go test ./internal/view/`
预期：全绿（`find:` 前缀已经没了，`grep -rn "find:" internal/` 为空）。

- [ ] **步骤 8：文档回写并提交**

`README.md` 目录那一行加"点击平滑滚到该标题顶部；标题靠近文档末尾、顶不到顶部时正文滚到最底部"，
搜索那段补一句"命中跳转与目录一致"。
`AGENTS.md`「已知的坑」加"跳转只有一条路（`State.jump` + `jumpMarker`），不要再给 `md.Anchor` 赋值"、
"近末尾落到最底部是 `ScrollIntoView` 的钳制"、"`Options.Block` 会让 mygo 给每个块多包一层 Column，只在有跳转的帧挂"
三条，并把"原生搜索只能滚到块"那条改成新机制。
`CHANGELOG.md`「未发布 / 变更」加两条（目录跳转与搜索跳转）。

```bash
git add internal/view/ README.md AGENTS.md CHANGELOG.md
git commit -m "feat: 目录/链接/搜索跳转统一为平滑顶到顶部，近末尾落到底部"
```

---

### 任务 2：WebView 的标题跳转改为页内平滑滚动，不再重载页面

**文件：**
- 修改：`internal/view/webview.go`（`scrollWebTo`、`headingScrollJS`、`loadWebDoc`）
- 测试：`internal/view/webview_test.go`、`internal/view/native_test.go`（没有 WebView 时的降级）
- 文档：`README.md`（已知限制那条）、`CHANGELOG.md`

- [ ] **步骤 1：编写失败的测试**

```go
func TestHeadingScrollJS(t *testing.T) {
	js := headingScrollJS("二-install-3")
	for _, want := range []string{`document.getElementById("二-install-3")`, "scrollIntoView", "behavior:'smooth'", "block:'start'"} {
		if !strings.Contains(js, want) { t.Errorf("js = %s, want %q", js, want) }
	}
}

func TestWebDocURLHasNoHeadingHash(t *testing.T) {
	… s.anchor = "二"; 断言 s.docURL() 不含 "二"，且 loadWebDoc 在 s.wv == nil 时不改 wvURL …
}

func TestTOCJumpWithoutAWebViewSaysSo(t *testing.T) {
	… setMode(RenderWebView)（无窗口，s.wv == nil），gotoHeading("二")，断言 s.status 提到 WebView、s.jump == 0 …
}
```

- [ ] **步骤 2：运行测试验证失败**

运行：`go test ./internal/view/ -run 'TestHeadingScrollJS|TestWebDocURL|TestTOCJumpWithoutAWebView' -v`
预期：FAIL / 编译错误——`headingScrollJS` 不存在，`docURL` 里现在也没有 anchor（这一条可能一上来就过，保留它防回归）。

- [ ] **步骤 3：实现 `internal/view/webview.go`**

```go
// scrollWebTo glides the page to the heading of slug, as the native body glides
// to the block: the page's own scrollIntoView lines the heading up with the
// top and stops at the page's end.
func (s *State) scrollWebTo(slug string) { … s.wv == nil → s.status = "WebView 不可用，未滚动到标题"；否则 Eval，出错写 status … }

// headingScrollJS is the page's scroll to the heading of slug: the browser's
// smooth scroll, which stops at the end of the page as the native one stops at
// the end of the document.
func headingScrollJS(slug string) string { … strconv.Quote(slug)，不用手拼引号 … }
```

`loadWebDoc` 删掉拼 `#` 的两段（连同 `findAnchorPrefix` 的注释），只按 `docURL()` 比较与加载。

- [ ] **步骤 4：运行测试验证通过**

运行：`go test ./internal/view/ -v`
预期：全部 PASS。

- [ ] **步骤 5：文档回写并提交**

`README.md`「已知限制」把"WebView 模式在文档变化或用目录跳转时会重新加载页面，滚动位置回到顶部"改成"WebView 模式在**文档内容变化**时会重新加载页面，滚动位置回到顶部；用目录跳转不再重载（页内平滑滚动）"。
`CHANGELOG.md`「未发布 / 修复」加一条。

```bash
git add internal/view/ README.md CHANGELOG.md
git commit -m "feat: WebView 目录跳转改为页内平滑滚动，不再重载丢位置"
```

---

### 任务 3：搜索面板改成三块浮岛

**文件：**
- 修改：`internal/view/find.go`（`findPanel`、`findField`、`findList`、`findPreview`、新增 `findIsland`）
- 测试：`internal/view/find_test.go`
- 文档：`README.md`（搜索那一段）、`AGENTS.md`（坑）、`CHANGELOG.md`

- [ ] **步骤 1：编写失败的测试**

```go
// TestFindPanelShowsIslandsApart pins the panel's shape: the matches and the
// context are panes of their own, apart from one another and from the field, so
// that one does not read as a run of the other's rows.
func TestFindPanelShowsIslandsApart(t *testing.T) {
	… 文档 "甲 目标\n乙\n丙\n"，openFindOn(s, "目标")，tt 建 s.findPanel
	// SearchField 的 label；若它不是 tt.Find 认的那种 label，就改成输入框所在行里唯一的文本。
	field, ok := tt.Find("在文档中查找")
	list, ok := tt.Find("匹配行")            // 岛的 label
	prev, ok := tt.Find("命中上下文")        // 岛的 label
	if prev.Y < list.Y+list.H { t.Errorf("上下文岛 %v 压在匹配行岛 %v 上", prev, list) }
	if gap := prev.Y - (list.Y + list.H); gap < 5 { t.Errorf("两岛间距 = %v, want t.Space(1.5) 也就是 6 DIP 的留白", gap) }
	if list.X != prev.X || list.W != prev.W { t.Errorf("两岛未对齐：%v / %v", list, prev) }
	if list.Y < field.Y+field.H { t.Errorf("匹配行岛 %v 压在输入框岛 %v 上", list, field) }
	if !tt.HasText("第 1 行上下文") { t.Errorf("texts = %q, want the context named", tt.Texts()) }
}

// TestFindPanelWithoutMatchesKeepsOneIsland pins the empty half: no matches
// shows the field alone, not two empty panes.
func TestFindPanelWithoutMatchesKeepsOneIsland(t *testing.T) { … 查一个不存在的词，断言 HasText("匹配行") == false … }
```

- [ ] **步骤 2：运行测试验证失败**

运行：`go test ./internal/view/ -run 'TestFindPanelShowsIslandsApart|TestFindPanelWithoutMatches' -v`
预期：FAIL——`tt.Find("匹配行")` 找不到（还没有 label），间距断言拿不到矩形。

- [ ] **步骤 3：在 `internal/view/find.go` 实现浮岛**

```go
// findIsland fills one pane of the search panel: a face of its own, rounded,
// bordered and lifted off the page, with pad DIPs of room inside. The panes are
// apart from one another and from the page, so that the list and the context do
// not read as one run of rows.
func findIsland(c *ui.Context, pad float32) ui.Element { … }

// findCaption puts a pane's name above what it holds.
func findCaption(c *ui.Context, text string) { … Text(text).FontSize(10).TextColor(t.TextMuted) … }
```

`findPanel` 的外层只剩位置与间距：

```go
ui.Column(c).Absolute().Right(t.Space(2)).Top(...).Width(findPaneWidth).
	Gap(t.Space(1.5)).Children(func() {
	s.findField(c)
	s.findList(c)
	s.findPreview(c)
})
```

`findField` 内容放进 `findIsland(c, t.Space(0.75))`；`findList` 找到匹配时放进 `findIsland(...)` 并 `.Label("匹配行")`，列表本身仍是 `ui.ScrollBoth(c).MaxHeight(findListHeight)`；`findPreview` 放进 `findIsland(...)` 并 `.Label("命中上下文")`，先用 `findCaption` 写 `第 N 行上下文`，再是两行号开头的上下文行（命中行仍是主色底）。内层列宽统一改成 `findPaneWidth - 2*findIslandPad - 2`，用一个常量 `findIslandPad` 免得三处各算一遍。

- [ ] **步骤 4：运行测试验证通过**

运行：`go test ./internal/view/ -run 'TestFind' -v`，再 `go test ./internal/view/`
预期：全部 PASS（含既有的 `TestFindPanelOpensWithCmdF`、`TestFindListRowJumpsToItsMatch`）。

- [ ] **步骤 5：文档回写并提交**

`README.md` 悬浮搜索那段加"面板是三块独立的浮岛：输入框 / 匹配行 / 上下文，块与块之间留空，上下文那块顶部写明第几行"。
`AGENTS.md` 的坑里加一条（三块浮岛、`findIsland`、`Label` 的唯一性）。
`CHANGELOG.md`「未发布 / 变更」加一条。

```bash
git add internal/view/ README.md AGENTS.md CHANGELOG.md
git commit -m "style: 搜索面板改成三块浮岛，匹配行与上下文分开"
```

---

### 任务 4：规格回写与全量校验

**文件：**
- 修改：`docs/superpowers/specs/2026-11-21-markdown-preview-design.md`
- 修改：`AGENTS.md`（结构表、坑、改动指引）
- 修改：`CHANGELOG.md`（确认三条变更在「未发布」下齐全）

- [ ] **步骤 1：规格回写**

在 `docs/superpowers/specs/2026-11-21-markdown-preview-design.md` 的「实施后的变更」末尾加两条（只追加，不改写原文）：

```markdown
### 跳转与搜索面板（user 2026-11-22）

- **原生 TOC / 正文内 `#标题` / 搜索命中的跳转**不再用 `markdown.Options.Anchor` + `AnchorVersion`
  （mygo 里那是 `ScrollNearest`、无动画）：改成 `State.jump` + `Options.Block` 钩子建零高标记，
  `ScrollIntoView({Block: ScrollStart, Animation: ui.Smooth})`。于是"目标顶到顶部、近末尾落到底部、平滑"
  都由 mygo 的 reveal 一次给全，`find:` 锚点前缀退役。取代原第 5 条。
- **webview 的 TOC 跳转**不再 `LoadURL(url + "#slug")`（整页重载）：改成 `Page().Eval` 的
  `scrollIntoView({behavior:'smooth', block:'start'})`，地址里不带 `#`。取代原第 6 条后半。
```

- [ ] **步骤 2：AGENTS.md 回写**

「结构」表：`find.go` 补"三块浮岛（`findIsland`）"；`native.go` 补"跳转（`jumpToBlock` / `jumpMarker`）"。
「已知的坑」加三条、改一条：

- 跳转只有一条路：`State.jump` + `jumpMarker`。**不要**再给 `md.Anchor` / `AnchorVersion` 赋值。
- "近末尾落到底部"是 `ScrollIntoView` 对滚动范围的钳制，不要自己算 `MaxY` 再 `ScrollTo`。
- `Options.Block` 会让 mygo 给每个块多包一层 Column，所以只在有跳转的帧挂（`nativePreview`）。
- 原"原生搜索只能滚到块"改成：搜索与目录共用 `jumpToBlock`，块仍然是最小粒度（`blockAt`）。
- 搜索面板是三块浮岛，别再塞回一个带背景的外层；块间必须留 `Gap`，否则会退回"一列糊在一起的行"。

- [ ] **步骤 3：全量校验**

```bash
go vet ./... && gofmt -l . && go test ./...
```
预期：vet 无输出、gofmt 无输出、`ok mdpreview/...` 全部通过。

- [ ] **步骤 4：提交**

```bash
git add AGENTS.md docs/superpowers/
git commit -m "docs: 回写跳转与搜索浮岛的结构、坑与规格"
```

---

## 未决问题（不阻塞实施，留给你拍板）

1. **IsLands 的具体材质**：本计划按"三块独立圆角卡片（平涂 + 投影）"实现（D11）。若你要的是 Liquid Glass 那种质感，只需在 `findIsland` 里加
   `Material(glass.Glass{Tint: t.Surface.Alpha(0.92)})`（`github.com/egoist/mygo/plugins/glass`，onevra 的 Popover / Dialog 就是这套）。
   风险写在 D11：面板会浮在 WebView 之上，玻璃采样跨原生视图不保证正确，需要实机看一眼。
2. **搜索跳转是否也要"顶到顶部"**：本计划把它和目录统一了（D1）。若你更希望搜索"就近滚动、只保证命中可见"，把 `gotoMatch` 的调用换成
   `s.jumpToBlock` 的一个 `ScrollNearest` 变体即可（多一个 `jumpNearest bool`），代价是多一条分支。
3. **滚动跟随（scroll spy）**：滚动正文时高亮目录当前章节，本计划不做（规格原「非目标」仍成立）。

## 执行后的修订（2026-11-22）

计划是历史记录，下面是执行时的偏离（账本里有对应的 `Ruling:` 行，账本已随工作区删除，内容以本节为准）：

1. **`s.jump` 不在钩子里清**（原 D4 作废一半）：一次点击会让 mygo 重建这一帧（`rt.consumed`），重建前
   `c.reveal` 被清空，第一遍注册的滚动请求会被丢掉，于是"钩子里立刻清 `jump`"会让点击静默失效。改成
   钩子只记 `jumpSeen = c.Now()`，**下一帧**才清 `jump`——同一帧的两遍构建都挂上钩子，第二遍的请求活到布局。
   代价是多一个 `time.Time` 字段；两帧同一时间戳时会多挂一帧（同一目标，不会位移）。
2. **`TestTOCJumpGlides` 的断言放宽**：原计划断言"一秒后 `FramePending` 为 false"，实测滚动条淡出动画仍在要帧、
   弹簧也只是渐近到位。改成"再 Advance 一秒，位置变化不超过 1 DIP"。
3. **搜索跳转与目录跳转并成一个任务**：删 `anchorVer` / `md.Anchor*` 会让 `find.go`、`webview.go` 编译不过，
   任务 1 必须连 `find:` 前缀一起退役（计划正文已按此改写）。
4. **WebView 没有页面时回落到原生跳转**：`webPreview` 在 `ensureWebView` 失败时会用原生正文，所以
   `gotoHeading` 只在 `mode == WebView && s.wv != nil` 时走 `Page().Eval`，否则照常 `jumpToBlock`。
   计划里"只写状态栏"的说法作废。
5. **上下文岛不再套内层凹槽**：浮岛本身是卡片，再套 `t.Background` 就成三层底色；改成岛顶一行
   `第 N 行上下文` 做区分（渲染成 PNG 目检过）。
6. **跳转在顶上留 16 DIP**（用户验收追加）：原计划是"顶边对齐窗格顶边"，实机上标题贴着边缘太紧。现在正文滚动容器
   有一个 16 DIP 的**透明上边框**（`bodyJumpGap`），内边距相应减 16（静止时样子不变），reveal 对齐边框于是正好留出
   16；webview 侧加 `scroll-margin-top: calc(16px * var(--md-scale))`。回归测试
   `TestTOCJumpLeavesRoomAboveTheHeading`、`TestDocHTMLLeavesRoomAboveHeadings`。
