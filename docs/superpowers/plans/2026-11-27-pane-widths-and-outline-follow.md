# 窗格宽度记忆与目录跟随正文 实现计划

> **面向 AI 代理的工作者：** 必需子技能：使用 subagent-driven-development（推荐）或 executing-plans 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** 三件事——（1）侧栏宽度重启后还在；（2）每个文档（`docSel{project, path}`）记住自己的目录宽度，
切标签各回各的；（3）正文滚动时左侧目录高亮当前章节，并把那一行滚进目录的视野；原生与 WebView 两种后端都要跟随。

**架构：** 宽度进 `session.json`（`internal/session` 加两个字段：`sidebar` 与 `toc[]`），视图侧只在
「宽度停手 400 ms」时写一次盘；侧栏的真实宽度由 `main.go` 用闭包从 onevra shell 读出来交给 `State`（shell 不报事件）。
目录跟随是一条单向数据流：**滚动位置 → `State.anchor` → 目录行的 `Selected`**。原生用 markdown 的 `Block` 钩子
给每个标题插一个零尺寸标记，上一帧的布局位置 + 正文容器的矩形算出「顶端是哪个标题」；WebView 没有消息通道，
用后台 goroutine 每 200 ms `Page().Eval` 问页面一次（页面自己算「视口顶部之上最后一个标题」）。

**技术栈：** 只用标准库（`encoding/json` / `time` / `context`）与既有依赖 mygo v0.10.0、onevra。
`Page.Eval` 是 mygo 已有的 API（`webview.go` 的 `applyWebZoom` / `scrollWebTo` 已在用），不引新依赖。

**规格：** 需求来自用户 2026-11-27（「侧栏宽度记忆 / 目录宽度按文件记忆 / 正文滚动时目录跟随并高亮，
WebView 也要按显示范围顶部标题跟随」）。设计依据见下面「设计决定」。总设计文档
`docs/superpowers/specs/2026-11-21-markdown-preview-design.md` 的「非目标」里有一条
「TOC 滚动跟随（scroll spy）」——本计划推翻它，任务 5 回写。执行者两份都读。

## 全局约束

- 不引入第三方 markdown / GFM / 高亮 / web 框架依赖（本功能只用标准库）。
- `internal/session` 不得 import `.../ui`；`internal/workspace` / `internal/mcp` 的约束不变。
- 配置与状态一律写 `App.Path(PathUserData)` 下的 JSON；原子写（`session.Save` 已有）。
- 测试只用 `t.TempDir()`；`view.New(ws, cfg, "")` 表示无用户数据目录 —— `session.Save` 是 no-op。
- **视图状态归窗口线程**：宽度与 `anchor` 只在构建期或帧首 `drain()` 里改；后台 goroutine 只 `Post`。
  唯一跨线程的东西是 WebView 轮询 goroutine 里的本地变量（`last`），它不碰 `State` 的字段。
- **一次拖动只写一次盘**：写盘只在「宽度停手满 `widthQuiet`（400 ms）」时发生，另在退出时 flush 一次。
- 文档身份是 `docSel{project, path}`；目录宽度以它为键，不得只按 `path`。
- 平台无关代码，不加 `//go:build`；三平台可编译。
- 提交信息中文，前缀 `feat:` / `fix:` / `test:` / `docs:`；提交前 `go vet ./... && gofmt -l . && go test ./...` 干净；
  文档回写并入对应任务同一提交。

## 审查重点（Review Focus）

1. **`session.json` 被外部改坏、或写着奇怪的宽度**（缺失、`0`、负数、超大值）→ 窗口按默认宽度打开
   （侧栏 280 / 目录 240），不崩、不影响标签与折叠的恢复。钉住：任务 1 的坏文件与非法宽度用例、任务 2 的回落用例。
2. **拖动分隔条时写盘风暴** → 连续变化的帧里一次都不写，停手 400 ms 才写一次；退出前未落盘的那次在
   `OnWillQuit` 里补写。钉住：任务 2 的「静默期内不写、停手后写一次」用例。
3. **宽度串到别的文档 / 别的项目** → 两个项目里都有 `docs/a.md` 时，两个宽度互不影响；切标签各回各的。
   钉住：任务 2 的两个用例（同名路径分属两项目、切文档再切回）。
4. **跟随把用户的手抢走** → 只在 `anchor` 变化时滚目录一次（用户自己滚目录不会被每帧拉回）；
   跳转滑动期间不得被中途的滚动位置覆盖高亮；折叠着的分支不自动展开。
   钉住：任务 3 的 `TestAJumpKeepsItsHighlightWhileItGlides`、`TestTheOutlineLeavesFoldedBranchesFolded`、
   任务 3 的目录滚动断言。
5. **WebView 轮询在原生模式下乱改高亮 / 窗口销毁后还在跑** → `followTo` 只在 WebView 模式、且不在跳转静默期内生效；
   循环在 `IsDestroyed()` 时退出、`Eval` 带 2 s 超时不会卡死。钉住：任务 4 的两个 `followTo` 用例。

---

## 设计决定（执行者照此实现；用户可推翻）

- **D1 宽度进 `session.json`，不进 `config.json`。** config 是「有哪些项目、渲染模式、MCP」，
  session 是「上次窗口长什么样」；宽度属于后者，且与已有的 `tabs` / `tree_closed` 在同一份快照里，
  一次写盘写全。`session.Version` 升到 2：新增字段是纯增量，`Load` 不需要迁移代码（旧文件缺字段即零值）。
  这推翻了 `2026-11-25-session-restore.md` 的 D3（「本期的范围之外不持久化侧栏宽度」），任务 5 追加修订。
- **D2 目录宽度按 `docSel`（项目 ID + 路径）记，带 200 条上限与「最近用过的在尾」。** 键用
  `{"project","path"}` 对象，不拼字符串。上限 200 防止「一个项目里翻过上千个文档 → JSON 无限长」；
  淘汰最久未用的（切文档 / 改宽度时把该条移到尾部，保存时取尾部 200 条，并丢掉已关闭项目的条目）。
- **D3 侧栏宽度由 `main.go` 从 shell 读。** onevra 的 shell 只给 `LeftWidth()`、不发事件，所以 `State` 拿一个
  `func() float32`（与 `SetContentWidth` 同形）。启动时先用**默认 280** 建 shell（`LeftWidth` 同时是双击复位的目标，
  不能写成记忆值），建好后 `shell.SetLeftWidth(state.SidebarWidth())` 把记忆值放回去 —— 于是双击复位永远回 280。
- **D4 停手 400 ms 才写盘**（`widthQuiet`）。写盘走既有的 `saveSession()`，它同时写标签、折叠与宽度；
  `State` 记 `widthsDirty` / `widthsChangedAt`，`trackWidths` 每帧（`View` 与 `Content` 各一行）看一眼。
  退出时 `FlushWidths()`（main 里 `mygo.App.OnWillQuit`，与 tray 的监听并列，监听器是列表所以互不影响）。
- **D5 当前章节 = 正文窗格顶端之上最后一个标题。** 原生：`Block` 钩子给每个 `parser.Heading` 插一个
  `ui.Box(0×0)` 并把 `{slug, element}` 记进 `headMarks`，下一帧用 `Bounds()` 比 `bodyPane.Bounds().Y + bodyJumpGap + followSlop(4)`。
  WebView：页面里用同样的规则（`getBoundingClientRect().top <= 24 * --md-scale`）。
- **D6 高亮复用一个状态 `State.anchor`。** 它就是目录行的 `Selected` 判据（现在只由点击设置）。
  滚动跟随与点击写同一个字段，目录因此只有一种"当前"概念。
- **D7 跳转后静默 400 ms**（`followHold`）：`gotoHeading` 记 `followUntil = frameNow + followHold`，
  这期间跟随不写 `anchor`，否则平滑滚动的中间帧会把高亮刷成路径上的邻居。`frameNow` 是当前帧的
  `c.Now()`（Tester 里是虚拟时钟，测试可以直接 `Advance` 过掉静默期）。
- **D8 目录只在 `anchor` 变化时滚一次**（`ScrollIntoView{Block: ScrollNearest}`），且只在那一行真的被构建出来时
  （折叠在分支里就不滚、不展开）。`tocShownAnchor` 记住已经跟到哪一行。
- **D9 WebView 用轮询，不用消息通道。** 内嵌 WebView 的 `Page` 没有 mygo 的 bridge（那是窗口页面的），
  所以后台 goroutine 每 200 ms（`followPoll`）`EvalContext(2s)` 一次，脚本返回 `[rev, slug]`：
  `rev` 来自 `docHTML` 已经写进页面的 `<meta name="rev">`，文档一换（同一个标题 slug）也会被当成新消息报一次。
  只在 `slug` 真的变了才 `Post`，避免空闲时每 200 ms 让窗口重画一帧。原生模式下 `followTo` 直接返回。
- **D10 `loadDoc` 清 `anchor` 与跟随的测量状态**（`anchor=""`、`bodyPane`、`headMarks`）：换了文档就该从「没有当前章节」
  开始，且旧文档的元素句柄不能留到新文档里用。这是顺手修掉的既有小毛病（换文档后同名 slug 仍高亮）。
- **D11 不做的事**：不记忆侧栏折叠状态（`LeftCollapsed` 仍不持久化）；不记忆标签滚动位置；
  目录跟随不做「用户滚目录时暂停跟随」的计时逻辑——只在 `anchor` 变化时滚一次就够了（见 D8）；
  WebView 的真实滚动事件仍拿不到，跟随靠轮询（D9），因此「页面滚到一半卡住」这类情况最多延迟 200 ms。

---

## 文件结构

- 修改 `internal/session/session.go` —— `Session` 加 `Sidebar`、`TOC`，新增 `DocWidth`，`Version` 升到 2。
- 修改 `internal/session/session_test.go` —— 宽度往返、缺字段的旧文件、坏宽度。
- 修改 `internal/view/state.go` —— `State` 的宽度/跟随字段、`SidebarWidth` / `SetSidebarWidth` / `trackWidths` /
  `tocWidthOf` / `noteTOCWidth` / `FlushWidths`、`restoreSession` 与 `saveSession` 的宽度部分、`New` 的默认值。
- 修改 `internal/view/view.go` —— `View` 与 `Content` 开头（`drain` 之后）各调一次 `trackWidths`；侧栏列宽保留测试用的 300。
- 修改 `internal/view/preview.go` —— `SplitReset(defaultTOCWidth)`。
- 修改 `internal/view/native.go` —— `jumpMarker` 改名 `blockHook` 并加标题标记；`followNative`；`gotoHeading` 设静默期；
  `toc()` 调 `followNative`、末尾记 `tocShownAnchor`；`tocItems` 在 `anchor` 变化时滚那一行；`loadDoc` 恢复/清理。
- 修改 `internal/view/webview.go` —— `webFollowJS`、`followPoll`、`startPageFollow`（在 `ensureWebView` 成功后启动）、
  `followTo`。
- 修改 `main.go` —— `LeftWidth: int(view.DefaultSidebarWidth)` + 建好后 `shell.SetLeftWidth(state.SidebarWidth())`；
  `state.SetSidebarWidth(func() float32 { return shell.LeftWidth() })`；`App.OnWillQuit` → `state.FlushWidths()`。
- 修改 `internal/view/widths_test.go`（新文件）/ `native_test.go` / `layout_test.go` / `webview_test.go` —— 本计划的钉子。
- 修改 `internal/view/screenshots_test.go` —— 新增 `12-outline-follow` 用例。
- 修改 `README.md` / `AGENTS.md` / `CHANGELOG.md` / `docs/superpowers/specs/2026-11-21-markdown-preview-design.md` /
  `docs/superpowers/plans/2026-11-25-session-restore.md`（末尾追加修订）—— 文档回写，并进对应任务。

依赖方向不变（`view → session` 已有；本计划没有新增包）。`main.go` 是唯一接触 shell 宽度的地方。

### 接口（任务间共用，签名照抄）

```go
// internal/session/session.go
const Version = 2

// DocWidth is the width the window remembers for one document's outline.
type DocWidth struct {
	Project string  `json:"project"`
	Path    string  `json:"path"`
	Width   float32 `json:"width"`
}

type Session struct {
	Version    int        `json:"version"`
	Tabs       []Ref      `json:"tabs"`
	Active     Ref        `json:"active"`
	TreeClosed []Ref      `json:"tree_closed"`
	Sidebar    float32    `json:"sidebar,omitempty"`
	TOC        []DocWidth `json:"toc,omitempty"`
}
```

```go
// internal/view/state.go

// DefaultSidebarWidth is the width the sidebar opens at, and the width a
// double click on its divider puts back: main.go passes it to the shell and
// then restores the width the last run remembered.
const DefaultSidebarWidth float32 = 280

const (
	defaultTOCWidth     float32 = 240
	maxTOCWidths                = 200
	widthQuiet                  = 400 * time.Millisecond
	followHold                  = 400 * time.Millisecond
	followSlop          float32 = 4
)

// headingMark is where a heading of the body sat in the last frame's layout.
type headingMark struct {
	slug string
	el   ui.Element
}

func (s *State) SidebarWidth() float32            // 记住的宽度，没记过就是 DefaultSidebarWidth
func (s *State) SetSidebarWidth(fn func() float32) // main.go 把它接到 shell.LeftWidth
func (s *State) FlushWidths()                      // 退出时把没落盘的宽度写下去
func (s *State) trackWidths(c *ui.Context)         // 每帧一眼：变化 → 标脏；停手满 widthQuiet → saveSession

// tocWidthOf returns the width remembered for sel, and whether it was
// remembered at all: a document nobody resized opens at defaultTOCWidth.
func (s *State) tocWidthOf(sel docSel) (float32, bool)

// noteTOCWidth records w for sel, most recently used last.
func (s *State) noteTOCWidth(sel docSel, w float32)

func (s *State) followNative(c *ui.Context)                                     // native.go
func (s *State) blockHook(c *ui.Context, d *parser.Document, i int) bool         // native.go，原 jumpMarker
func (s *State) followTo(slug string)                                           // webview.go，窗口线程
func (s *State) startPageFollow()                                               // webview.go，goroutine
```

`State` 新增字段（名字照抄，测试会读）：

```go
	sidebarWidth    float32
	sidebarOf       func() float32
	widths          []session.DocWidth // 最近用过的在尾
	seenTOC         float32
	widthsDirty     bool
	widthsChangedAt time.Time
	frameNow        time.Time
	followUntil     time.Time
	headMarks       []headingMark
	nextMarks       []headingMark
	bodyPane        ui.Element
	tocShownAnchor  string
	pageFollow      bool // the web view's poll loop is running
```

删掉现在没人读的 `State.sidebarW` 字段（`View` 的侧栏列宽是测试用的字面量 300，留在 `view.go` 里并加注释）。

---

### 任务 1：`internal/session` 记住两个宽度

**文件：**
- 修改：`internal/session/session.go`
- 测试：`internal/session/session_test.go`

- [ ] **步骤 1：写失败的测试**

在 `session_test.go` 追加：

```go
func TestWidthsRoundTrips(t *testing.T) {
	dir := t.TempDir()
	in := Session{Version: Version,
		Sidebar: 312,
		TOC: []DocWidth{
			{Project: "a", Path: "docs/x.md", Width: 180},
			{Project: "b", Path: "docs/x.md", Width: 320},
		},
	}
	if err := Save(dir, in); err != nil {
		t.Fatal(err)
	}
	out, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if out.Sidebar != 312 || !slices.Equal(out.TOC, in.TOC) {
		t.Errorf("widths = %v %+v, want 312 %+v", out.Sidebar, out.TOC, in.TOC)
	}
}

func TestAFileWithoutWidthsLeavesThemEmpty(t *testing.T) {
	dir := t.TempDir()
	old := `{"version":1,"tabs":[{"project":"a","path":"x.md"}],"active":{},"tree_closed":[]}`
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if out.Sidebar != 0 || len(out.TOC) != 0 || len(out.Tabs) != 1 {
		t.Errorf("session = %+v, want the tabs kept and no widths", out)
	}
}
```

- [ ] **步骤 2：运行测试验证失败**

运行：`go test ./internal/session -run 'TestWidths|TestAFileWithout' -v`
预期：FAIL，编译错误 `unknown field Sidebar in struct literal` / `undefined: DocWidth`。

- [ ] **步骤 3：在 `internal/session/session.go` 中实现**

`Version = 2`，加 `DocWidth`（字段与 JSON 名照「接口」一节），`Session` 加 `Sidebar float32 \`json:"sidebar,omitempty"\``
与 `TOC []DocWidth \`json:"toc,omitempty"\``。`Load` / `Save` 不动（纯增量字段，无迁移）。

- [ ] **步骤 4：运行测试验证通过**

运行：`go test ./internal/session -v`
预期：PASS（含既有的四个用例）。

- [ ] **步骤 5：文档回写并入本任务**

`AGENTS.md` 结构表里 `internal/session` 那一行改成「会话快照 `session.json`（标签页与活动标签、树里被折叠的目录、
侧栏与每个文档的目录宽度）」。`README.md`「配置文件与数据存放位置」里 `session.json` 的说明同步补一句。

- [ ] **步骤 6：Commit**

```bash
git add internal/session AGENTS.md README.md
git commit -m "feat: 会话快照记住侧栏与每个文档的目录宽度"
```

---

### 任务 2：视图侧记下并恢复两个宽度

**文件：**
- 修改：`internal/view/state.go`、`internal/view/preview.go`、`internal/view/view.go`、`internal/view/native.go:loadDoc`
- 修改：`main.go`
- 测试：`internal/view/widths_test.go`（新文件）

- [ ] **步骤 1：写失败的测试**

新建 `internal/view/widths_test.go`：

```go
package view

import (
	"testing"
	"time"

	"mdpreview/internal/session"
	"mdpreview/internal/workspace"
)

func TestTheSidebarWidthSurvivesARestart(t *testing.T) {
	s, _ := newState(t)
	if got := s.SidebarWidth(); got != DefaultSidebarWidth {
		t.Errorf("SidebarWidth = %v, want the default %v", got, DefaultSidebarWidth)
	}
	s.SetSidebarWidth(func() float32 { return 312 })
	tt := newTester(s)
	tt.Frame()
	tt.Advance(time.Second) // 停手满静默期
	tt.Frame()

	again := New(workspace.New(s.cfg), s.cfg, s.cfgDir)
	if got := again.SidebarWidth(); got != 312 {
		t.Errorf("after a restart SidebarWidth = %v, want 312", got)
	}
}

func TestAWidthUnderThePointerIsWrittenOnceItStops(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "a.md", "# A\n\n## 甲\n")
	s.Refresh()
	s.selectDoc("a.md")
	tt := newTester(s)
	tt.Frame()

	s.tocW = 300
	tt.Frame()
	tt.Advance(100 * time.Millisecond) // 还在动（没到静默期）
	tt.Frame()
	if got := readSession(t, s).TOC; len(got) != 0 {
		t.Errorf("toc widths = %+v, want nothing written yet", got)
	}
	tt.Advance(500 * time.Millisecond)
	tt.Frame()
	if got := readSession(t, s).TOC; len(got) != 1 || got[0].Width != 300 {
		t.Errorf("toc widths = %+v, want the one 300", got)
	}
}

func TestTheOutlineWidthComesBackWithItsDocument(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "a.md", "# A\n\n## 甲\n")
	writeDoc(t, root, "b.md", "# B\n")
	s.Refresh()
	s.selectDoc("a.md")
	tt := newTester(s)
	s.tocW = 320
	tt.Frame()
	tt.Advance(time.Second)
	tt.Frame()

	s.selectDoc("b.md")
	if got := s.tocW; got != defaultTOCWidth {
		t.Errorf("tocW = %v on the other document, want %v", got, defaultTOCWidth)
	}
	s.selectDoc("a.md")
	if got := s.tocW; got != 320 {
		t.Errorf("tocW = %v back on a.md, want 320", got)
	}
}

func TestAnOutlineWidthBelongsToItsProject(t *testing.T) {
	s, alpha, beta := twoProjects(t)
	writeDoc(t, alpha, "docs/a.md", "# A\n")
	writeDoc(t, beta, "docs/a.md", "# A\n")
	s.Refresh()
	s.selectDocIn(s.Trees()[0].project.ID, "docs/a.md")
	tt := newTester(s)
	s.tocW = 300
	tt.Frame()
	tt.Advance(time.Second)
	tt.Frame()

	s.selectDocIn(s.Trees()[1].project.ID, "docs/a.md")
	if got := s.tocW; got != defaultTOCWidth {
		t.Errorf("tocW = %v in the other project, want %v", got, defaultTOCWidth)
	}
}

// readSession reads what the window wrote, for the tests about writing.
func readSession(t *testing.T, s *State) session.Session {
	t.Helper()
	sess, err := session.Load(s.cfgDir)
	if err != nil {
		t.Fatalf("read session: %v", err)
	}
	return sess
}
```

- [ ] **步骤 2：运行测试验证失败**

运行：`go test ./internal/view -run 'TestTheSidebarWidth|TestAWidthUnder|TestTheOutlineWidthComes|TestAnOutlineWidth' -v`
预期：FAIL，编译错误（`s.SetSidebarWidth` / `s.SidebarWidth` / `defaultTOCWidth` / `s.noteTOCWidth` 未定义）。

- [ ] **步骤 3：实现 `State` 的宽度状态**

在 `state.go`：
- 加「接口」一节的常量、`DefaultSidebarWidth`、`headingMark`（本任务先建类型，任务 3 才用它的元素），以及
  「接口」一节列出的字段；`New` 里初始化 `sidebarWidth: DefaultSidebarWidth`；删掉 `sidebarW` 字段。
- `restoreSession` 开头加：`if sess.Sidebar > 0 { s.sidebarWidth = sess.Sidebar }`、`s.widths = sess.TOC`
  （`sess.TOC` 里宽度 ≤ 0 或 > 4000 的条目直接丢掉）。
- 实现 `SidebarWidth`（返回 `s.sidebarWidth`，`<= 0` 时回 `DefaultSidebarWidth`）、`SetSidebarWidth`、
  `FlushWidths`（`if s.widthsDirty { s.saveSession() }`）。
- 实现 `tocWidthOf`（遍历 `s.widths` 找 `project`+`path` 都对上的那条，返回 `(宽度, true)`；`Width <= 0`
  视为没记过、返回 `(defaultTOCWidth, false)`）、`noteTOCWidth`（有就改并移到尾；没有就追加；
  超过 `maxTOCWidths` 从头截）。
- 实现 `trackWidths(c)`（见「设计决定 D4」）：

```go
func (s *State) trackWidths(c *ui.Context) {
	s.frameNow = c.Now()
	if s.sidebarOf != nil {
		if w := s.sidebarOf(); w != s.sidebarWidth && w > 0 {
			s.sidebarWidth = w
			s.widthsDirty, s.widthsChangedAt = true, c.Now()
		}
	}
	if !s.sel.empty() && s.tocW != s.seenTOC {
		s.seenTOC = s.tocW
		s.noteTOCWidth(s.sel, s.tocW)
		s.widthsDirty, s.widthsChangedAt = true, c.Now()
	}
	if s.widthsDirty && c.Now().Sub(s.widthsChangedAt) >= widthQuiet {
		s.saveSession()
	}
}
```

- `saveSession` 尾部改成：按 `s.cfg.IsOpen` 过滤 `s.widths`、只留尾部 `maxTOCWidths` 条，连同
  `Sidebar: s.sidebarWidth` 一起交给 `session.Save`，最后 `s.widthsDirty = false`。

- [ ] **步骤 4：实现宽度的恢复与回落**

- `native.go` 的 `loadDoc`：开头（`s.sel.empty()` 判断之前）加 `s.anchor = ""`、`s.bodyPane = ui.Element{}`、
  `s.headMarks = nil`；`s.sel.empty()` 分支里 `s.tocW = defaultTOCWidth`、`s.seenTOC = s.tocW`；
  读到文档后：

```go
if w, ok := s.tocWidthOf(s.sel); ok {
	s.tocW = w
	s.noteTOCWidth(s.sel, w) // 用过的排到最后，保存时截断的永远是别人
} else {
	s.tocW = defaultTOCWidth // 没记过就不写进快照，别让每个翻过的文档都留一条 240
}
s.seenTOC = s.tocW
```
- `preview.go` 的 `SplitReset(240)` 改成 `SplitReset(defaultTOCWidth)`。
- `view.go`：`View` 与 `Content` 在 `drain` / `handleShortcuts` / `updateToasts` 之后各加一行 `s.trackWidths(c)`；
  `View` 的侧栏列宽保留 `Width(300)`，注释写明「测试用的近似宽度，真实宽度由 shell 决定」。

- [ ] **步骤 5：接线 `main.go`**

`sidebar.ShellOptions` 的 `LeftWidth: int(view.DefaultSidebarWidth)`（默认值，也是双击复位目标）；
`NewShell` 之后立刻 `shell.SetLeftWidth(state.SidebarWidth())`（记忆值）；
`state.SetContentHeight` 旁边加 `state.SetSidebarWidth(func() float32 { return shell.LeftWidth() })`；
`state.StartWatching()` 附近加 `mygo.App.OnWillQuit(func(*mygo.QuitEvent) { state.FlushWidths() })`。

- [ ] **步骤 6：运行测试验证通过**

运行：`go test ./internal/view -run 'TestTheSidebarWidth|TestAWidthUnder|TestTheOutlineWidthComes|TestAnOutlineWidth' -v`
预期：PASS。再跑 `go test ./internal/view`：既有用例全绿（`TestTabsStartAtTheShellInset` 的 316 不受影响）。

- [ ] **步骤 7：文档回写并入本任务**

`CHANGELOG.md`「未发布 / 新增」加两条（侧栏宽度记忆、每个文档记住自己的目录宽度）。
`AGENTS.md`：「改会话记忆」那条指引补上两个宽度与 `trackWidths` 的写盘规则（停手 400 ms、退出 flush、
`setTreeKeyOpen` 那条禁写盘的规则不变）；「已知的坑」加一条「onevra 的 shell 只给 `LeftWidth()` 不发事件，
所以宽度每帧读、变化才写；`LeftWidth` 选项同时是双击复位目标，记忆值必须用 `SetLeftWidth` 放回去」。

- [ ] **步骤 8：Commit**

```bash
git add internal/view main.go CHANGELOG.md AGENTS.md
git commit -m "feat: 记住侧栏与每个文档的目录宽度"
```

---

### 任务 3：原生正文滚动 → 目录高亮并跟到当前行

**文件：**
- 修改：`internal/view/native.go`（`jumpMarker` → `blockHook`、`followNative`、`gotoHeading`、`toc`、`tocItems`）
- 修改：`internal/view/state.go`（`headerMark` 若任务 2 未建齐则在此补齐；`tocShownAnchor` 初始化）
- 测试：`internal/view/native_test.go`、`internal/view/layout_test.go`

- [ ] **步骤 1：写失败的测试**

在 `native_test.go` 追加（`sections(n)` 已在该文件里）：

```go
// TestTheOutlineFollowsTheBodyScroll pins the follow: the section at the top
// of the body is the outline's current one, without a click.
func TestTheOutlineFollowsTheBodyScroll(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "a.md", sections(8))
	s.Refresh()
	s.selectDoc("a.md")

	tt := newTester(s)
	tt.Frame() // 一帧之后标记与正文容器才有位置
	pane := s.bodyPane.Bounds()
	if pane.H == 0 || len(s.headMarks) < 5 {
		t.Fatalf("pane %v, marks %d: the body did not lay out", pane, len(s.headMarks))
	}
	// 把第 5 节的标记顶到窗格顶边（bodyJumpGap 之下）。
	s.docScroll().Y = s.headMarks[4].el.Bounds().Y - (pane.Y + bodyJumpGap) + 1
	tt.Frame()
	tt.Frame()

	if s.anchor != "第 5 节" {
		t.Errorf("anchor = %q, want 第 5 节", s.anchor)
	}
}

func TestScrollingBackToTheTopLeavesTheFirstHeadingCurrent(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "a.md", sections(8))
	s.Refresh()
	s.selectDoc("a.md")

	tt := newTester(s)
	tt.Frame()
	s.docScroll().Y = 2000
	tt.Frame()
	tt.Frame()
	if s.anchor == "第 1 节" || s.anchor == "" {
		t.Fatalf("anchor = %q, want a later section while scrolled", s.anchor)
	}
	s.docScroll().Y = 0
	tt.Frame()
	tt.Frame()
	if s.anchor != "第 1 节" {
		t.Errorf("anchor = %q back at the top, want 第 1 节", s.anchor)
	}
}

// TestAJumpKeepsItsHighlightWhileItGlides pins the hold: the frames between
// the click and the landing must not repaint the outline with the sections the
// body is passing.
func TestAJumpKeepsItsHighlightWhileItGlides(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "a.md", sections(8))
	s.Refresh()
	s.selectDoc("a.md")

	tt := newTester(s)
	if err := tt.Click("第 6 节"); err != nil {
		t.Fatalf("Click: %v", err)
	}
	tt.Frame()
	if s.anchor != "第 6 节" {
		t.Errorf("anchor = %q right after the click, want 第 6 节", s.anchor)
	}
}

// TestTheOutlineKeepsTheCurrentHeadingInSight pins that the outline scrolls to
// the current row of a long document, rather than marking a row nobody sees.
func TestTheOutlineKeepsTheCurrentHeadingInSight(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "a.md", sections(40))
	s.Refresh()
	s.selectDoc("a.md")

	tt := newTester(s) // 1000x700
	tt.Frame()
	pane := s.bodyPane.Bounds()
	if len(s.headMarks) < 30 {
		t.Fatalf("marks = %d, want the whole outline measured", len(s.headMarks))
	}
	s.docScroll().Y = s.headMarks[29].el.Bounds().Y - (pane.Y + bodyJumpGap) + 1
	for range 3 {
		tt.Frame()
	}
	if s.anchor != "第 30 节" {
		t.Fatalf("anchor = %q, want 第 30 节", s.anchor)
	}
	if s.tocScroll.Y <= 0 {
		t.Fatal("the outline did not scroll to the current row")
	}
	list, ok := tt.Find("目录列表")
	if !ok {
		t.Fatalf("texts = %q, want the outline's scroll container", tt.Texts())
	}
	row, ok := tt.Find("第 30 节")
	if !ok {
		t.Fatalf("texts = %q, want the current row built", tt.Texts())
	}
	if row.Y < list.Y || row.Y+row.H > list.Y+list.H {
		t.Errorf("row at %v, outline %v: the current row is not in sight", row, list)
	}
}
```

- [ ] **步骤 2：运行测试验证失败**

运行：`go test ./internal/view -run 'TestTheOutlineFollows|TestScrollingBack|TestAJumpKeeps|TestTheOutlineKeeps' -v`
预期：FAIL：`anchor = ""`（跟随不存在）或 `pane.H == 0, marks 0`（标记不存在）。

- [ ] **步骤 3：实现标题标记**

`native.go`：`jumpMarker` 改名 `blockHook`，签名不变，行为变成：
- 跳转目标块照旧插 `0×0` 的 `ui.Box` + `ScrollIntoView`；
- 另对 `d.Block(i).Kind == parser.Heading` 插一个 `ui.Box(c).Width(0).Height(0)`，把
  `headingMark{slug: markdown.Slug(parser.PlainText(d.Inlines(i))), el: el}` 追加到 `s.nextMarks`；
- 末尾仍 `return false`。

`nativePreview` 里：装配 `s.md.Block = s.blockHook` 之前 `s.nextMarks = s.nextMarks[:0]`；把正文容器换成
`s.bodyPane = ui.ScrollBoth(c)...`（保存句柄）；正文构建完之后 `s.headMarks, s.nextMarks = s.nextMarks, s.headMarks[:0]`。
保持 `s.md.Block` 常挂（注释照旧说明为什么）。

- [ ] **步骤 4：实现跟随与静默期**

`native.go` 加 `followNative(c)`（D5/D7）：

```go
func (s *State) followNative(c *ui.Context) {
	if s.mode() != config.RenderNative {
		return
	}
	if s.jump > 0 || !s.jumpSeen.IsZero() || c.Now().Before(s.followUntil) {
		return
	}
	pane := s.bodyPane.Bounds()
	if pane.H == 0 || len(s.headMarks) == 0 {
		return
	}
	top := pane.Y + bodyJumpGap
	slug := ""
	for _, m := range s.headMarks {
		if m.el.Bounds().Y > top+followSlop {
			break
		}
		slug = m.slug
	}
	if slug != "" {
		s.anchor = slug
	}
}
```

`toc(c)` 的第一行调 `s.followNative(c)`；`toc` 末尾（标题列表构建完之后）`s.tocShownAnchor = s.anchor`。
`gotoHeading` 里 `s.anchor = slug` 之后加 `s.followUntil = s.frameNow.Add(followHold)`。

- [ ] **步骤 5：实现「只滚当前那一行」**

`tocItems` 的两个分支里，在 `item := ...` 之后、`Clicked()` 之前加：

```go
if s.anchor != "" && n.h.Slug == s.anchor && s.anchor != s.tocShownAnchor {
	item.ScrollIntoView(ui.ScrollIntoViewOptions{Block: ui.ScrollNearest})
	s.tocShownAnchor = s.anchor
}
```

（带子节点的分支与叶子分支各一处；`Selected(...)` 不动。）

- [ ] **步骤 6：运行测试验证通过**

运行：`go test ./internal/view -run 'TestTheOutlineFollows|TestScrollingBack|TestAJumpKeeps|TestTheOutlineKeeps|TestTOC|TestJump' -v`
预期：PASS。再跑 `go test ./internal/view`：既有 `TestTOCJump*`、`TestJumpWithoutMotionWhenAsked`、
`TestTOCRowsFillThePane`、`TestCollapseAllFoldsTheOutline` 仍绿。

- [ ] **步骤 7：文档回写并入本任务**

`README.md`「用法」的目录那一段补「滚动正文时目录跟着高亮当前章节，并自动滚到那一行（原生与 WebView 都跟随）」。
`CHANGELOG.md`「未发布 / 新增」加一条。`AGENTS.md`：
- 「已知的坑」加两条：①`followNative` 读的是**上一帧**的布局（`Bounds()`），所以跳转要 `followHold` 静默期，
  否则平滑滚动的中间帧会把高亮刷成路上的邻居；②目录只在 `anchor` 变化时 `ScrollIntoView` 一次，
  每帧都调会把用户手动滚走的目录拉回来；折叠分支不展开、不滚。
- 「改动指引」加一条「改目录跟随」：算法只在 `native.go` 的 `followNative` / `webview.go` 的 `webFollowJS`，
  两边同一个规则（顶端之上最后一个标题），改一处必须改另一处。

- [ ] **步骤 8：Commit**

```bash
git add internal/view README.md AGENTS.md CHANGELOG.md
git commit -m "feat: 正文滚动时目录跟随当前章节并高亮"
```

---

### 任务 4：WebView 也跟随（页面顶部标题）

**文件：**
- 修改：`internal/view/webview.go`
- 测试：`internal/view/webview_test.go`

- [ ] **步骤 1：写失败的测试**

在 `webview_test.go` 追加（这层只钉 Go 侧的策略，页面脚本本身由真窗口手动验证）：

```go
// TestThePageTellsTheOutlineWhereTheReaderIs pins the WebView path: the page
// reports the heading at its top, and that becomes the outline's current one.
func TestThePageTellsTheOutlineWhereTheReaderIs(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "a.md", "# 一\n\n## 二\n\n正文\n")
	s.Refresh()
	s.selectDoc("a.md")
	s.setMode(config.RenderWebView)

	s.followTo("二")
	if s.anchor != "二" {
		t.Errorf("anchor = %q, want the heading the page reported", s.anchor)
	}
}

// TestThePageCannotMoveTheNativeOutline pins the guard: the page of a web view
// nobody shows must not repaint the outline of the native body.
func TestThePageCannotMoveTheNativeOutline(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "a.md", "# 一\n\n## 二\n")
	s.Refresh()
	s.selectDoc("a.md") // 原生

	s.followTo("二")
	if s.anchor != "" {
		t.Errorf("anchor = %q, want the native body's own state", s.anchor)
	}
}

// TestAPageReportDoesNotCutAJumpShort pins the hold for the page too.
func TestAPageReportDoesNotCutAJumpShort(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "a.md", "# 一\n\n## 二\n")
	s.Refresh()
	s.selectDoc("a.md")
	s.setMode(config.RenderWebView)
	s.frameNow = time.Now()
	s.followUntil = s.frameNow.Add(followHold)

	s.followTo("二")
	if s.anchor != "" {
		t.Errorf("anchor = %q during the hold, want it left alone", s.anchor)
	}
}
```

- [ ] **步骤 2：运行测试验证失败**

运行：`go test ./internal/view -run 'TestThePageTells|TestThePageCannot|TestAPageReport' -v`
预期：FAIL，编译错误 `s.followTo undefined`。

- [ ] **步骤 3：实现页面轮询与 `followTo`**

`webview.go` 加：

```go
// followPoll is how often the page is asked where the reader is: often enough
// that a scroll is followed by the next blink, rare enough to cost nothing.
const followPoll = 200 * time.Millisecond

// webFollowJS is the page's answer to where the reader is: the id of the last
// heading whose top is above the page's own top edge, and the revision of the
// document it belongs to, which changes with the document. It is the rule
// followNative applies to the native body.
const webFollowJS = `(() => {
  const scale = parseFloat(getComputedStyle(document.documentElement).getPropertyValue('--md-scale')) || 1;
  const top = 24 * scale;
  const heads = document.querySelectorAll('article h1,article h2,article h3,article h4,article h5,article h6');
  let slug = '';
  for (const h of heads) {
    if (h.getBoundingClientRect().top <= top) slug = h.id; else break;
  }
  const meta = document.querySelector('meta[name="rev"]');
  return [meta ? meta.getAttribute('content') : '', slug];
})()`
```

`startPageFollow()`：`s.wv == nil` 时不启动；用一个 `State` 字段 `pageFollow bool` 防止重复启动；
goroutine 每 `followPoll` 用 `EvalContext`（`context.WithTimeout(context.Background(), 2*time.Second)`）
取一次结果，`[]any` 里 `[rev, slug]`；键 `rev + "\x00" + slug` 与本地 `last` 相同就跳过；
`slug == ""` 跳过；否则 `last = key` 并 `s.Post(func() { s.followTo(slug) })`；`wv.IsDestroyed()` 时 `return`。

`ensureWebView` 首次建成功之后调 `s.startPageFollow()`。
`followTo`（窗口线程）：`slug == ""`、`s.mode() != config.RenderWebView`、`s.frameNow.Before(s.followUntil)`
三者任一就返回；否则 `s.anchor = slug`。

- [ ] **步骤 4：运行测试验证通过**

运行：`go test ./internal/view -run 'TestThePage|TestDocHTML|TestHeadingScroll|TestDocURL' -v`
预期：PASS（`webview_test.go` 需要新加 `"time"` 的 import）。

- [ ] **步骤 5：真窗口手动验证（写进提交信息）**

`go run .`，打开一份长 md，切到 WebView：滚动正文，目录高亮跟着走、并滚到当前行；
点目录跳转时高亮不闪回；切回生先后各滚一次，两边规则一致。

- [ ] **步骤 6：文档回写并入本任务**

`README.md`「已知限制」加一条：「WebView 的滚动位置靠每 200 ms 轮询页面得到（内嵌 WebView 没有消息通道），
所以跟随最多慢 0.2 秒；页面还没加载完时不动。」
`AGENTS.md`「已知的坑」加一条 `webFollowJS` / `startPageFollow` 的说明：脚本返回 `[rev, slug]`、
用 `rev` 区分文档（同一个标题 slug 换文档也要重报）、只在变化时 `Post`（否则空闲时每 200 ms 唤醒一帧）、
`Eval` 必须带超时、循环在 `IsDestroyed()` 退出。`CHANGELOG.md` 那条跟随的条目补「WebView 也一样」。
`AGENTS.md` 的 `mdview://` 安全那条不动（本计划没有新增读取入口）。

- [ ] **步骤 7：Commit**

```bash
git add internal/view README.md AGENTS.md CHANGELOG.md
git commit -m "feat: WebView 正文滚动时目录跟随页面顶部标题"
```

---

### 任务 5：截图用例、规格/旧计划回写与收尾

**文件：**
- 修改：`internal/view/screenshots_test.go`
- 修改：`docs/superpowers/specs/2026-11-21-markdown-preview-design.md`
- 修改：`docs/superpowers/plans/2026-11-25-session-restore.md`（末尾追加）
- 修改：`CHANGELOG.md`（收尾校对）

- [ ] **步骤 1：加截图用例**

`shotCases()` 末尾加：

```go
{
	name: "12-outline-follow",
	build: func(t *testing.T) *State {
		s := shotTree(t)
		writeDoc(t, s.Trees()[0].project.Root, "docs/long.md", sections(40))
		s.Refresh()
		s.selectDoc("docs/long.md")
		return s
	},
	settle: 2,
	act: func(t *testing.T, s *State, tt *ui.Tester) {
		// 滚到中段，让目录高亮与目录自己的滚动都看得见。
		tt.Scroll(700, 400, 0, 2200)
		for range 3 {
			tt.Advance(20 * time.Millisecond)
			tt.Frame()
		}
	},
},
```

- [ ] **步骤 2：看图**

运行：`MDPREVIEW_SHOT_DIR=/tmp/shots go test ./internal/view -run TestScreenshots -v`
预期：`12-outline-follow.png` 生成；打开确认三件事：当前章节那行是主色底（与点选目录一致）、
它的底色铺满整行、目录自己的滚动条位置跟着走了（当前行在视野内而不是被切在边上）。

- [ ] **步骤 3：规格回写**

`docs/superpowers/specs/2026-11-21-markdown-preview-design.md`「非目标（本期明确不做）」里
`- TOC 滚动跟随（scroll spy）——点击 TOC 能跳，但滚动正文不会高亮目录项` 改成删除线 + 说明：

```markdown
- ~~TOC 滚动跟随（scroll spy）~~（2026-11-27 实现：正文滚动时目录高亮当前章节并滚到那一行，
  原生与 WebView 都跟随；见 `docs/superpowers/plans/2026-11-27-pane-widths-and-outline-follow.md`）
```

- [ ] **步骤 4：旧计划追加修订**

`docs/superpowers/plans/2026-11-25-session-restore.md` 末尾追加（只追加，不改写原来的任务步骤）：

```markdown
## 执行后的修订（2026-11-27）

- **D3 的半条被推翻**：侧栏宽度与每个文档的目录宽度现在也进 `session.json`（`sidebar` 与 `toc` 两个字段，
  `Version` 升到 2）。原因：onevra 的 shell 只给 `LeftWidth()`、不持久化也不发事件，`StateKey` 只管窗口位置，
  所以「用户每次都要重新拖」成了实际抱怨。实现见
  `docs/superpowers/plans/2026-11-27-pane-widths-and-outline-follow.md`。
- D3 里其余几条仍然成立：目录窗格的展开状态、标签的正文滚动位置、搜索面板与消息面板的开合、树搜索查询
  依旧不持久化。
```

- [ ] **步骤 5：CHANGELOG 校对**

确认三条新条目都在「未发布 / 新增」里、措辞是用户视角（不写实现细节），`session.json` 那条提到
「侧栏与目录的宽度也一起记住」。`README.md` 的「配置文件与数据存放位置」里 `session.json` 的字段列全。

- [ ] **步骤 6：全量验证**

运行：`go vet ./... && gofmt -l . && go test ./...`
预期：`vet` 无输出、`gofmt` 无输出、`go test` 全 PASS。
再跑一次 `MDPREVIEW_SHOT_DIR=/tmp/shots go test ./internal/view -run TestScreenshots`，确认十三个用例都出图。

- [ ] **步骤 7：Commit**

```bash
git add internal/view/screenshots_test.go docs/superpowers CHANGELOG.md README.md
git commit -m "test: 目录跟随的截图用例与规格回写"
```

---

## 自检记录

1. **规格覆盖度**：三个需求各自有任务——侧栏宽度（任务 1 + 2）、按文档记目录宽度（任务 1 + 2）、
   目录跟随并高亮（任务 3 原生 / 任务 4 WebView）。WebView 的「按显示范围顶部标题」由 `webFollowJS` 的
   `rect.top <= 24 * scale` 规则落地（任务 4），与原生 `followNative` 同规则（任务 3，D5）。
   规格（总设计文档）里被推翻的「TOC 滚动跟随」非目标由任务 5 步骤 3 回写；旧计划 D3 由任务 5 步骤 4 追加修订。
2. **步骤扫描**：每个测试步骤给了名字与断言，每个代码步骤给了文件、签名与「设计决定」里定死的取值
   （280 / 240 / 400 ms / 200 ms / 200 条 / 4 DIP / 24 CSS px）。没有「处理边界情况」「适当验证」这类空步骤；
   函数体只在算法不被签名与测试决定时才写（`trackWidths`、`followNative`、`webFollowJS`）。
3. **类型一致性**：`DocWidth`（session）、`headingMark`（view）、`SidebarWidth` / `SetSidebarWidth` / `FlushWidths` /
   `trackWidths` / `tocWidthOf` / `noteTOCWidth` / `followNative` / `blockHook` / `followTo` / `startPageFollow`
   在接口一节定义、任务 2–4 使用，名字一致。`DefaultSidebarWidth`（导出，main 用）与 `defaultTOCWidth`
   （包内）区分大小写，别混。
4. **审查重点**：五条各有钉子——①任务 1 的坏文件/缺字段用例 + 任务 2 的回落断言（`<= 0` 回默认）；
   ②任务 2 的 `TestAWidthUnderThePointerIsWrittenOnceItStops` + `FlushWidths`（任务 2 步骤 3/5）；
   ③任务 2 的 `TestTheOutlineWidthComesBackWithItsDocument` / `TestAnOutlineWidthBelongsToItsProject`；
   ④任务 3 的 `TestAJumpKeepsItsHighlightWhileItGlides` / `TestTheOutlineKeepsTheCurrentHeadingInSight`，
   折叠分支不展开由 D8 + `tocShownAnchor` 保证（既有 `TestCollapseAllFoldsTheOutline` 不得红）；
   ⑤任务 4 的 `TestThePageCannotMoveTheNativeOutline` / `TestAPageReportDoesNotCutAJumpShort`
   + `startPageFollow` 的 `IsDestroyed` 退出与 2 s 超时。
5. **比例**：本计划约 400 行，三个功能各约 100 行代码 + 测试；没有把实现代码抄进来（`webFollowJS` 是唯一
   必须逐字给出的字符串，因为规则与阈值在页面里）。

## 执行后的修订（执行时追加，不改写上面的任务步骤）

- **标题位置存数字，不存 `ui.Element`**（推翻任务 3 的 `headingMark{slug, el}`）：MyGo 每个 pass 结束就作废
  上一 pass 的 Element，跨 pass 读 `.Bounds()` 会 panic（`ui: element from pass 1 used in pass 2`）。
  现在的形状是 `headingMark{slug, y}` + `State.blockMark`（本 pass 的 Element，只在本 pass 读一次）+
  `State.bodyBox`：正文构建完的同一 pass 把位置量成数字，下一帧的 `toc` 只比数字。
- **「窗格顶边之上没有标题」取第一个标题**（`headMarks[0]` / 页面报空时取 `s.headings[0]`）：文档顶端的
  第一个标题本来就落在路径行 + 内边距之下，否则滚回顶部时高亮会停在中途的章节（任务 3 的测试抓到的）。
- **测试比较 `workspace.Slug(...)`，不比较中文标题**：`State.anchor` 一直是 slug（目录 `Selected` 的判据），
  计划里的测试写法写成标题文字了。
- **`tocWidthOf` 返回 `(float32, bool)`**：`loadDoc` 要区分「记过 240」与「没记过」，否则每个翻过的文档都会
  在快照里留一条 240，与 D2 的「不囤垃圾」相反。
- **页面脚本的 `rev` / `slug` 用 `/` 连接**做去重键（原稿写 ` `）：`rev` 是数字，slug 里没有 `/`，
  足够区分，也不必在源码里写控制字符。
- **截图用例 `12-outline-follow` 滚到文档底部**（`tt.Scroll(..., 0, 20000)`）：滚到中段时当前章节往往还在目录
  视野内，看不出「目录自己跟着滚」。看图确认：当前行是主色底、铺满整行，目录已滚到 `第 17 节` 起、当前
  `第 39 节` 在视野底部边上。
- **`followHold` 是 600 ms，不是计划的 400 ms**（最终审查抓到）：mygo 的 `ui.Smooth` 是 500 ms 的弹簧，
  400 ms 的静默期在滑行最后约 100 ms 就会过期，高亮会在落点前的章节上闪一下。新增
  `TestTheHoldOutlastsTheGlide`（断言静默期 ≥ `ui.Smooth.Spring.Duration`）钉住它。
- **测试视图里的正文横向滚动条是既有现象，不是本次改动**：`View`（测试布局）的侧栏写死 300，
  而 `bodyPaneWidth()` 用 `windowWidth()-tocW`（不减侧栏），长文档因此比窗格宽；真实 shell 里
  `SetContentWidth` 已经减掉侧栏宽度，不会有这条滚动条。

## 执行后的修订（用户 2026-11-28 报的问题，已修）

- **点靠末尾的章节时高亮会跳回上一节**：跳转把目标顶不到页面顶部（正文被 `MaxY` 钳住）时，跟随按「窗格顶边之上最后一个标题」
  会选中上一节，静默期一过就跳一次。修法 = 给跳转选中的章节一个 pin：`State.pinSlug` / `pinY` / `pinSeen` / `pinDeadline`，
  `pinned(y, landed)` 在落点之前一直当它是当前章节，落点之后正文一动（>0.5 DIP）就释放；`jumpLanded` 认「标记到了顶边」
  或「正文已在 `MaxY`」两种落点。WebView 侧同一套（页面在静默期之后的第一条报告就是落点，`webFollowJS` 顺手多报
  `window.scrollY`，去重键带上它，这样「标题没变但页面动了」也能报上来）。新增
  `TestAJumpThatCannotReachTheTopKeepsItsHeading`、`TestAPinnedHeadingLetsGoWhenTheBodyIsDragged`、
  `TestAWheelAtTheEndKeepsThePinnedHeading`、`TestThePageCannotTakeTheOutlineOffAClammedJump`。

## 执行后的修订（WebView 的回跳，用户 2026-11-28 用 `data-verify-plan.md` 复现）

- **WebView 里点靠末尾的章节仍会回跳**：页面的 `scrollIntoView({behavior:'smooth'})` 距离越长滑得越久，
  可能超过 600 ms 静默期，于是「还在滑」的报告被当成落点：先 capture 在途中，下一条报告位置一变就释放，
  跟随按页面顶边的章节重算，落到上一节。修法 = 让页面自己报落点：`headingScrollJS` 把目标写进
  `window.__mdTarget`（16×scale 的 `scroll-margin-top`，钳到最大滚动），`webFollowJS` 每 200 ms 报
  `[rev, slug, scrollY, state]`（1 落点 / 0 滑行 / -1 读者接管），`followTo` 把 state 交给 `pinned`。
  页面的 wheel / touchstart / keydown（`pageInputJS`）清掉目标并报 -1，pin 立刻松开；
  `jumpPatience` 从 1 s 放宽到 3 s（页面的滑动可能比原生慢，耐心期只该在「被打断」时才用完）。
  新增 `TestAPageReportOfItsGlideKeepsThePinnedHeading`、`TestAPageTakeOverLetsTheOutlineFollow`、
  `TestHeadingScrollJSMarksItsTarget`、`TestDocHTMLDropsTheTargetWhenTheReaderScrolls`；
  另用用户的 `data-verify-plan.md`（12 个标题）在原生路径上手动跑过：点最后三个标题（其中两个落在 `MaxY`）高亮都不再回跳。
