# 搜索面板：匹配列表的最小高度与可拖拽高度 实现计划

> **面向 AI 代理的工作者：** 必需子技能：使用 subagent-driven-development（推荐）或 executing-plans 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** 悬浮搜索面板里的**匹配列表岛**不再随命中数塌缩（哪怕只有一条命中也有最小高度），
并且可以用鼠标拖动它与「命中上下文」之间的那条带子拉高、拉低、双击复位；高度只在本次运行内记住。

**架构：** 高度是视图状态，归 `State`（`findState.listH`），不落 `config.json`（与 `zoom` 一致）。
匹配列表岛从 `MaxHeight` 改成显式的 `Height`，由 `findListHeightOf()` 把用户拖出来的值钳制在
`[findListMin, findListMax]`；两块数据岛之间插入一条 10 DIP 的拖拽带（`ui.Box`，`CursorResizeNS`），
用 mygo 的 `Dragged()`（**每帧增量**）改 `listH`，并保留 `Split` 那样的小过冲补偿，让「拖过头再拖回来」立刻响应。
面板列因此从 `Gap(t.Space(1.5))` 改成显式 `Margin` + 拖拽带，保证各岛间距不被拖拽带挤乱。

**技术栈：** Go 1.27+、`github.com/egoist/mygo v0.9.1`（`ui.Box`、`Element.Dragged` / `DoubleClicked` / `Hovered` /
`Cursor` / `Role` / `Label` / `Draw` / `Margin` / `Height`、`ui.CursorResizeNS`、`ui.RoleSplitter`、
`ui.NewTester` 的 `Press` / `Move` / `Release` / `Click`）、`github.com/oneo-me/onevra/source v0.2026.18`（`components.Text`）。
**不引入新依赖。**

**规格：** `docs/superpowers/specs/2026-11-21-markdown-preview-design.md` 的
「跳转与搜索面板（取代原第 5 条与第 6 条后半，2026-11-22）」一节——本计划只修改那一节里
「三块独立浮岛」的**尺寸行为**，不推翻任何目标；执行后在该文件末尾追加
「### 搜索面板的可调高度（新增，2026-11-23）」。

**本期不做：** 正文内命中高亮。用户 2026-11-23 决定走 mygo 引擎侧支持（`Options.Mark` + `MarkVersion`），
另见 `docs/superpowers/plans/2026-11-23-mygo-mark-ranges.md`——那份计划在 **mygo 仓库**执行，
本仓库**不落地** `<mark>` 注入方案，`find.go` 本期一行高亮代码都不加（现状保持：
面板内「命中上下文」已经把命中染主色，正文只滚动不高亮，README「已知限制」第 1 条不动）。

## 全局约束

- 不引入第三方 markdown / GFM / 高亮 / web 框架依赖（AGENTS 硬规则）。
- 颜色只从 `AppTheme` 取；本计划唯一用到的颜色是拖拽带 hover/拖动时的 `t.Accent`
  （`internal/view/theme.go`），不写死十六进制。
- 视图状态归窗口线程：拖动只在**构建期**改 `s.find.listH`，不读盘、不重新计算匹配、不 `Post`。
- 视图是 state 的纯函数：`View` 与 `Content` 两条拼装路径都要能构建；改布局后 `go test ./internal/view/` 全绿，
  `TestShellSlotsBuild` 不回归。
- 测试只用 `t.TempDir()`；不写平台专用代码；三平台可编译。
- 文案与无障碍名用中文，图标按钮/可交互元素必须给 `label`（这里是拖拽带的 `Label`）。
- 每个行为改动先写失败的测试；文档回写（README / CHANGELOG / AGENTS / 规格）**并入同一个提交**。
- 提交前 `go vet ./... && gofmt -l . && go test ./...` 必须干净；中文提交信息，前缀 `feat:` / `fix:` / `test:` / `docs:`。

## 设计决策（实施者按此执行，不要另立方案）

| # | 决策 | 理由 |
| --- | --- | --- |
| D1 | 高度是 `findState.listH`（新字段），语义是「用户拖出来的高度」，`0` 表示「没拖过」。读取一律经 `findListHeightOf()`：`0` → `findListHeight`（132，现值），否则 `min(max(listH, findListMin), findListMax)`。 | 用户原话「给一个最小高度 同时高度改为可拖拽拉伸」。把「没拖过」与「拖到 0」分开，才能既保留现值又让钳制只在一处。 |
| D2 | 常量：`findListHeight = 132`（默认，约 5 行）、`findListMin = 72`（最小，约 3 行，每行约 24 DIP）、`findListMax = 320`（最大，约 13 行）、`findGrip = 10`（拖拽带高度）。 | 132 是当前值，老用户观感不变；72 是「三条命中刚好看得全」的下限，再小就回到截图里那种「塌成一行」；320 让面板整体（132→320 加上下文岛 148 加输入行）不超过常见 700 DIP 高的窗口；10 是 `t.Space(2.5)`，比 6 DIP 的旧岛间距宽，因为这条带子要接鼠标（AGENTS 里有「命中区太小点不到」的前车之鉴）。 |
| D3 | 匹配列表岛用 `Height(s.findListHeightOf())`，**删掉** `MaxHeight(findListHeight)`。上下文岛保持 `MaxHeight(findPreviewSize)` 不变（内容自适应，本期不参与拖拽）。 | 「不再塌缩」只能用固定高度表达：`MaxHeight` 是「内容多高就多高，最多 132」。上下文岛本来就只有 5 行、天然不会塌，拖它也缺一个合理的下限，所以本期只拖列表。 |
| D4 | 面板列**不再用 `Gap`**，改成：输入岛无外边距 → 匹配列表岛 `Margin(t.Space(1.5), 0, 0, 0)` → 拖拽带 `Height(findGrip)` → 上下文岛无外边距。 | `Gap` 会给拖拽带也加间距，于是「带子」和「它替代的那段间距」叠成两份。用 Margin 表达，三处数字各自可见：输入↔列表仍是 `t.Space(1.5)`（6 DIP，不变），列表↔上下文变成 `findGrip`（10 DIP，变宽 4 DIP，因为带子住在这里）。 |
| D5 | 拖拽用 `Dragged()`：它返回的是**上一帧读到之后的增量**（`ui/runtime.go` 的 `forgetInput` 每帧把 `dragX/dragY` 清零）。所以每帧 `listH = clamp(上一帧的值 + dy + overshoot)`。 | 按「从按下起的累计位移」算会指数放大：每帧都把累计量再加一遍，列表会瞬间飞到上限。这是本计划最容易写错的一处。 |
| D6 | 小过冲补偿：`findState.dragOver float32`。`want := findListHeightOf() + dy + dragOver`；`listH = clamp(want)`；`dragOver = want - listH`；松开（`!held`）时 `dragOver = 0`。 | 没有它，拖到 320 之后继续下拉 200、再上推 200，要等指针先回到 320 的位置才开始变矮，手感是「粘住」。mygo 的 `Split` 用 `splitOvershoot` 解决同一问题（`ui/split.go`），照着做。 |
| D7 | 双击复位：`box.DoubleClicked()` → `listH = findListHeight`。 | 与 `ui.SplitReset` 的约定一致（mygo 的分隔条双击复位），也是一个不用看文档就能试出来的手势。 |
| D8 | 拖拽带静止时**不画**任何东西；`Hovered()` 或拖动中在带子正中画 1 DIP 的 `t.Accent` 细线。始终注册 `Draw` 回调。 | 面板是三块浮岛的观感，加一条常驻分隔线会退回「一整块卡片」。`Draw` 回调常挂是为了让它参与命中测试（mygo 的 `Split` 分隔条同理）。 |
| D9 | 拖拽带 `Role(ui.RoleSplitter)` + `Label("调整匹配列表高度")`。 | AGENTS 硬规则：可交互元素必须有读屏名；`RoleSplitter` 让读屏把它读成分隔条，也是测试里 `tt.Find(...)` 定位它的唯一办法。 |
| D10 | 高度**不落盘**：不写 `config.json`，窗口重开回到 132。 | `zoom`（`State.zoom`）也不落盘，同一个面板里的两个可调量保持同一套记忆范围；落盘要动 `config.Config` 与 `normalize`，本期不值。 |
| D11 | 每次打开面板不清高度；换文档不清高度；`openFind` / `closeFind` 都不碰 `listH`。 | 用户拖出来的高度是「这个面板的手感」，不是「这次搜索的参数」。 |

### 改完长什么样

```
 ┌──────────────────────────────────────────────────┐  ← 输入岛（不变）
 │ 🔍 自动定期检测              1/17  ↑  ↓  ✕        │
 └──────────────────────────────────────────────────┘
                        ↕ t.Space(1.5) = 6 DIP
 ┌──────────────────────────────────────────────────┐
 │ 13  **规格:…自动定期检测**                        │  ← 匹配列表岛：Height(listH)
 │ 27  …自动定期检测…                                │     默认 132 / 最小 72 / 最大 320
 │ …                                       ▐ scroll  │     只有一条命中时也是 132（不再塌成一行）
 └──────────────────────────────────────────────────┘
 ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━  ← 拖拽带 findGrip = 10 DIP
                                                        光标 ↕；hover/拖动时画 1 DIP accent 线
                                                        双击 = 回到 132
 ┌──────────────────────────────────────────────────┐
 │ 第 13 行上下文                                    │  ← 上下文岛（不变，MaxHeight 148）
 │ 11  **技术栈: Go 1.27.3…                          │
 │ 13  **规格: **用户 2026-11-20 的口头需求…         │
 └──────────────────────────────────────────────────┘
```

## 审查重点（Review Focus）

1. **只有一条命中**：匹配列表岛仍应有一段可读的高度，且**高度不再随命中数变化**，不能退回到「一行 + 横向滚动条」。
   钉住它的测试：任务 1 的 `TestFindListKeepsItsHeightWhateverTheCount`。
2. **拖动中的高度算法**：高度必须按「每帧增量」累加。写错成累计位移会让高度在几帧内撞到 320，
   用户感觉「一拖就满了」。钉住它的测试：任务 2 的 `TestDraggingTheDividerResizesTheMatchList`
   （拖 40 DIP 后必须正好是 `132+40`，不是 320）。
3. **拖到上限再往回拖**：拖到 320 后继续下拉 200、再上推 100，高度应立刻变成 220（而不是要等指针回到原位）。
   钉住它的测试：任务 2 的 `TestDraggingPastTheLimitComesBack`。
4. **三块浮岛的间距**：`Gap` 改 `Margin` 之后，输入↔列表仍是 6 DIP、列表↔上下文是拖拽带的 10 DIP，
   不能出现「带子占位 + 间距」双份留白。钉住它的测试：任务 2 的 `TestFindIslandsKeepTheirGaps`。
5. **拖动不影响搜索状态**：拖动后查询、`n/m`、当前命中、正文位置都不能变（拖拽带不是列表，误触也不能跳转）。
   钉住它的测试：任务 2 的 `TestDraggingKeepsTheChosenMatch` 与 `TestDraggingTheDividerDoesNotJump`。

---

## 任务

### 任务 1：匹配列表的最小与最大高度

**文件：**
- 修改：`internal/view/find.go`（常量块 `findPaneWidth…findPaneInner`、`findState`、新增 `findListHeightOf`、`findList`）
- 测试：`internal/view/find_test.go`（追加在 `TestFindCountSaysWhenItCapped` 之后）

- [ ] **步骤 1：编写失败的测试**

```go
// findListRect opens the search panel on a document and returns the box of the
// matches' island, as the last frame laid it out.
func findListRect(t *testing.T, body string) ui.Rect {
	t.Helper()
	s, root := newState(t)
	writeDoc(t, root, "a.md", body)
	s.Refresh()
	s.selectDoc("a.md")
	openFindOn(s, "目标")
	tt := ui.NewTester(func(c *ui.Context) { s.findPanel(c) }, 900, 700)
	tt.Frame()
	r, ok := tt.Find("匹配行")
	if !ok {
		t.Fatalf("texts = %q, want the matches' island", tt.Texts())
	}
	return r
}

// TestFindListKeepsItsHeightWhateverTheCount pins the floor: one match must not
// leave the panel a single row tall, as MaxHeight let it. The island takes the
// height asked for, not the height its rows happen to need.
func TestFindListKeepsItsHeightWhateverTheCount(t *testing.T) {
	one := findListRect(t, "甲\n乙 目标\n丙\n")
	many := findListRect(t, strings.Repeat("甲 目标\n", 30))
	if one.H != many.H {
		t.Errorf("one match H = %v, thirty = %v: the island still follows its rows", one.H, many.H)
	}
	if one.H < findListMin {
		t.Errorf("H = %v, want at least the floor %v", one.H, findListMin)
	}
}

// TestFindListHeightIsClampedToThePanel pins what the height may be: never
// asked, the default; dragged, within what the panel allows.
func TestFindListHeightIsClampedToThePanel(t *testing.T) {
	s := newStateWith(config.Config{Version: config.Version})
	if got := s.findListHeightOf(); got != findListHeight {
		t.Errorf("not dragged → %v, want the default %v", got, findListHeight)
	}
	s.find.listH = 10
	if got := s.findListHeightOf(); got != findListMin {
		t.Errorf("10 → %v, want the minimum %v", got, findListMin)
	}
	s.find.listH = 9999
	if got := s.findListHeightOf(); got != findListMax {
		t.Errorf("9999 → %v, want the maximum %v", got, findListMax)
	}
}
```

（`ui.Rect` 是 `tt.Find` 的返回类型。第一条用「一条命中与三十条命中的高度相同」表达「高度不再随内容变化」，
比写死 132 更稳——主题的边框是否计入 `Height` 不必在这里钉死。）

- [ ] **步骤 2：运行测试验证失败**

运行：`go test ./internal/view/ -run 'TestFindList' -v`
预期：FAIL，`undefined: findListMin`（以及 `s.findListHeightOf undefined`）。

- [ ] **步骤 3：在 `internal/view/find.go` 中实现**

常量块改成（保留原有注释风格，`findListHeight` 从「上限」变成「默认」）：

```go
const (
	findPaneWidth   float32 = 420
	findListHeight  float32 = 132 // the default: about five rows
	findListMin     float32 = 72  // the floor: about three rows, ~24 DIP each
	findListMax     float32 = 320 // the ceiling: about thirteen rows
	findGrip        float32 = 10  // the drag band between the two data islands
	findPreviewSize float32 = 148
	findContext     int     = 2
	findIslandPad   float32 = 6
	findPaneInner   = findPaneWidth - 2*findIslandPad - 2
)
```

`findState` 加字段：

```go
	// listH is how tall the matches' island is, as the user dragged it: 0
	// until then, and for as long as the window lives (findListHeightOf
	// reads it, within what the panel allows).
	listH float32
```

新增方法（放在 `findable` 附近）：

```go
// findListHeightOf returns how tall the matches' island is: the height the
// user dragged it to, within what the panel allows, and the default until
// the divider is dragged.
func (s *State) findListHeightOf() float32 {
	if s.find.listH == 0 {
		return findListHeight
	}
	return min(max(s.find.listH, findListMin), findListMax)
}
```

`findList` 里的滚动容器由 `MaxHeight(findListHeight)` 改成 `Height(s.findListHeightOf())`。

- [ ] **步骤 4：运行测试验证通过**

运行：`go test ./internal/view/ -run 'TestFindList' -v`
预期：PASS（两条）。

- [ ] **步骤 5：Commit**

```bash
git add internal/view/find.go internal/view/find_test.go
git commit -m "feat: 搜索面板的匹配列表有最小与最大高度"
```

### 任务 2：拖动分隔条改高度

**文件：**
- 修改：`internal/view/find.go`（`findState.dragOver`、新增 `findDivider`、`findPanel` 的列布局、`findList`/`findPreview` 的外边距）
- 测试：`internal/view/find_test.go`（追加）

- [ ] **步骤 1：编写失败的测试**

```go
// findDividerY returns where the drag band sits between the two data islands,
// as the last frame laid them out: the top of the band.
func findDividerY(t *testing.T, tt *ui.Tester) float32 {
	t.Helper()
	list, ok := tt.Find("匹配行")
	if !ok {
		t.Fatalf("texts = %q, want the matches' island", tt.Texts())
	}
	return list.Y + list.H
}

// TestDraggingTheDividerResizesTheMatchList pins the gesture: 40 DIPs down is
// 40 more for the matches' island, not the ceiling.
func TestDraggingTheDividerResizesTheMatchList(t *testing.T) {
	s, tt := newFindPanel(t, "甲 目标\n乙 目标\n丙\n")
	y := findDividerY(t, tt)
	tt.Press(200, y+2)
	tt.Move(200, y+42)
	tt.Release(200, y+42)
	if got := s.findListHeightOf(); got != findListHeight+40 {
		t.Errorf("listH = %v, want %v", got, findListHeight+40)
	}
}

// TestDraggingTheDividerStopsAtTheLimits pins both ends.
func TestDraggingTheDividerStopsAtTheLimits(t *testing.T) {
	s, tt := newFindPanel(t, "甲 目标\n乙 目标\n丙\n")
	y := findDividerY(t, tt)
	tt.Press(200, y+2)
	tt.Move(200, y-400)
	tt.Release(200, y-400)
	if got := s.findListHeightOf(); got != findListMin {
		t.Errorf("dragged far up → %v, want %v", got, findListMin)
	}
	tt.Press(200, findDividerY(t, tt)+2)
	tt.Move(200, findDividerY(t, tt)+900)
	tt.Release(200, findDividerY(t, tt)+900)
	if got := s.findListHeightOf(); got != findListMax {
		t.Errorf("dragged far down → %v, want %v", got, findListMax)
	}
}

// TestDraggingPastTheLimitComesBack pins the overshoot: past the ceiling, the
// pointer has to come back over the divider before the island shrinks.
func TestDraggingPastTheLimitComesBack(t *testing.T) {
	s, tt := newFindPanel(t, "甲 目标\n乙 目标\n丙\n")
	y := findDividerY(t, tt)
	tt.Press(200, y+2)
	tt.Move(200, y+600) // clamped at findListMax
	tt.Move(200, findDividerY(t, tt)+2-100)
	tt.Release(200, findDividerY(t, tt)+2-100)
	if got := s.findListHeightOf(); got != findListMax-100 {
		t.Errorf("listH = %v, want %v", got, findListMax-100)
	}
}

// TestDoubleClickingTheDividerPutsTheHeightBack pins the reset gesture.
func TestDoubleClickingTheDividerPutsTheHeightBack(t *testing.T) {
	s, tt := newFindPanel(t, "甲 目标\n乙 目标\n丙\n")
	s.find.listH = 300
	tt.Frame()
	tt.ClickAt(200, findDividerY(t, tt)+2)
	tt.ClickAt(200, findDividerY(t, tt)+2)
	if got := s.findListHeightOf(); got != findListHeight {
		t.Errorf("listH = %v, want the default back: %v", got, findListHeight)
	}
}

// TestFindIslandsKeepTheirGaps pins the layout: the band replaced the gap, it
// did not add one, so no island sits further from its neighbour than before.
// The two DIPs of slack are the border of an island, which may or may not count
// in its box.
func TestFindIslandsKeepTheirGaps(t *testing.T) {
	_, tt := newFindPanel(t, "甲 目标\n乙 目标\n丙\n")
	in, ok := tt.Find("搜索输入")
	if !ok {
		t.Fatalf("texts = %q, want the input island", tt.Texts())
	}
	list, ok := tt.Find("匹配行")
	if !ok {
		t.Fatalf("texts = %q, want the matches' island", tt.Texts())
	}
	preview, ok := tt.Find("命中上下文")
	if !ok {
		t.Fatalf("texts = %q, want the preview island", tt.Texts())
	}
	if gap := list.Y - (in.Y + in.H); gap < 4 || gap > 8 {
		t.Errorf("input→matches gap = %v, want the theme's 6", gap)
	}
	if gap := preview.Y - (list.Y + list.H); gap < findGrip-2 || gap > findGrip+2 {
		t.Errorf("matches→preview gap = %v, want the band's %v", gap, findGrip)
	}
}

// TestDraggingKeepsTheChosenMatch pins that a drag is no search: the query,
// the count and the chosen match stay as they were.
func TestDraggingKeepsTheChosenMatch(t *testing.T) {
	s, tt := newFindPanel(t, "甲 目标\n乙 目标\n丙 目标\n")
	s.findNext(1)
	tt.Frame()
	before := s.find.current
	y := findDividerY(t, tt)
	tt.Press(200, y+2)
	tt.Move(200, y+52)
	tt.Release(200, y+52)
	if s.find.current != before || s.find.query != "目标" {
		t.Errorf("current = %d, query = %q: a drag changed the search", s.find.current, s.find.query)
	}
}
```

其中测试辅助（放在 `find_test.go` 顶部，`openFindOn` 旁边）：

```go
// newFindPanel opens the search panel on a document and returns the window with
// its panel built, as a user who searched for 目标 leaves it.
func newFindPanel(t *testing.T, body string) (*State, *ui.Tester) {
	t.Helper()
	s, root := newState(t)
	writeDoc(t, root, "a.md", body)
	s.Refresh()
	s.selectDoc("a.md")
	openFindOn(s, "目标")
	tt := ui.NewTester(func(c *ui.Context) { s.findPanel(c) }, 900, 700)
	tt.Frame()
	return s, tt
}
```

- [ ] **步骤 2：运行测试验证失败**

运行：`go test ./internal/view/ -run 'TestDragging|TestDoubleClicking|TestFindIslands' -v`
预期：FAIL——「拖动没改高度」（`listH = 132, want 172`，此时拖拽带还不存在，指针落在两块岛之间的空隙上）
与「列表→上下文的间距还是旧的 6」（`gap = 6, want the band's 10`）。

- [ ] **步骤 3：在 `internal/view/find.go` 中实现**

常量块里的 `findGrip` 已在任务 1 加好（若任务 1 只加了三个高度常量，就把它补上）：

```go
	// findGrip is the band between the matches and the preview that drags the
	// matches' island taller or shorter, in DIPs. It is wider than the room
	// between the islands was, since it has to be caught with a pointer.
	findGrip float32 = 10
```

`findState` 加字段：

```go
	// dragOver is how far past a limit the pointer dragged the divider, so that
	// coming back moves it at once rather than once the pointer is over it
	// again, as ui.Split keeps its own (splitOvershoot).
	dragOver float32
```

`findPanel` 的列改成（去 `Gap`，用 `Margin` + 拖拽带）：

```go
	ui.Column(c).Absolute().
		Right(t.Space(2)).
		Top(sidebar.HeaderHeight(c) + titleRowHeight + t.Space(2)).
		Width(findPaneWidth).
		Children(func() {
			s.findField(c)
			s.findList(c)
			s.findDivider(c)
			s.findPreview(c)
		})
```

`findList` 的岛加 `Margin(t.Space(1.5), 0, 0, 0)`（在 `findIsland(c).Label("匹配行")` 之后链式调用）。

`findField` 的输入岛加 `.Label("搜索输入")`：它是容器，加名字只为让测试量得到三块岛的间距（与 `匹配行` /
`命中上下文` 同理），输入框自己的 `Label("在文档中查找")` 不变。

新增 `findDivider`：

```go
// findDivider is the band between the matches and the preview: the pointer
// drags it to resize the matches' island, and a double click puts it back to
// its default. It is drawn only while the pointer is over it or dragging it, so
// that the panel stays three islands rather than one card with a rule through
// it.
//
// Dragged reports what the pointer moved since the frame before, not since the
// press: adding it to the height each frame is what makes the island follow the
// pointer. The overshoot kept in dragOver is what makes coming back from a
// limit move the island at once.
func (s *State) findDivider(c *ui.Context) {
	t := c.Theme()
	band := ui.Box(c).FillWidth().Height(findGrip).
		Cursor(ui.CursorResizeNS).Role(ui.RoleSplitter).Label("调整匹配列表高度")
	_, dy, held := band.Dragged()
	if held {
		want := s.findListHeightOf() + dy + s.find.dragOver
		s.find.listH = min(max(want, findListMin), findListMax)
		s.find.dragOver = want - s.find.listH
	} else {
		s.find.dragOver = 0
	}
	if band.DoubleClicked() {
		s.find.listH = findListHeight
		s.find.dragOver = 0
	}
	band.Draw(func(p *ui.Painter, r ui.Rect) {
		if !held && !band.Hovered() {
			return
		}
		p.Fill(ui.Rect{X: r.X, Y: r.Y + r.H/2, W: r.W, H: 1}, t.Accent, 0)
	})
}
```

（`ui.Rect` 的字段是 `X/Y/W/H`、`Painter.Fill(rect, color, radius)`——与 `ui/split.go` 的 `div.Draw` 同款用法。）

- [ ] **步骤 4：运行测试验证通过**

运行：`go test ./internal/view/ -run 'TestDragging|TestDoubleClicking|TestFindIslands' -v`
预期：PASS（六条）。

若 `tt.Press` 打不中拖拽带（高度一直不变），说明空 `ui.Box` 没有参与命中：给 `band` 加
`Background(ui.Transparent)` 再跑一次。**不要**改成把拖拽带塞进列表岛里做绝对定位（那会让命中区跟着滚动内容走）。

- [ ] **步骤 5：跑全量测试**

运行：`go test ./internal/view/ -v`
预期：全绿；`TestShellSlotsBuild`、`TestFind*` 无回归。若某条既有测试断言了 `findListHeight` 是上限
（例如量 1 条命中时岛只有一行高），按新行为改写它，并在提交信息里写明。

- [ ] **步骤 6：Commit**

```bash
git add internal/view/find.go internal/view/find_test.go
git commit -m "feat: 搜索面板可以拖动调整匹配列表高度"
```

### 任务 3：文档回写（**并入同一个提交**，不单开「补文档」）

- [ ] `README.md`
  - 「用法」里悬浮搜索那一条（`Cmd/Ctrl+F` 那段）补一句：匹配列表可以**拖动它与上下文之间的分隔条**调整高度
    （默认约 5 行、最小约 3 行、最大约 13 行），**双击分隔条复位**；高度只在本次运行内记住。
  - 「已知限制」**不动**：原生模式搜索不高亮正文这条是本期遗留（见下方「后续」）。
- [ ] `CHANGELOG.md` 的「未发布」
  - **变更**：悬浮搜索的匹配列表不再随命中数塌缩（只有一条命中时也保持可读高度），并可以拖动它与
    「命中上下文」之间的分隔条拉高/拉低，双击复位；高度只在本次运行内记住。
- [ ] `AGENTS.md`
  - 「改动指引」的「改正文内查找」一条补：面板尺寸在 `find.go` 的常量块（`findListHeight`/`findListMin`/
    `findListMax`/`findGrip`），拖拽带是 `findDivider`，补 `TestDragging*` 与 `TestFindIslandsKeepTheirGaps`。
  - 「已知的坑」加三条：
    1. **面板列没有 `Gap`**：间距由两处 `Margin` + 一条 `findGrip` 高的拖拽带构成，改间距要一起改这三处，
       否则会出现「带子占位 + 间距」的双份留白。
    2. **`Dragged()` 是每帧增量**（`ui/runtime.go` 的 `forgetInput` 每帧清零），不要按「从按下起的累计位移」
       写高度，否则一拖就撞上限。mygo 的 `Split` 是同一套语义。
    3. **拖到极限要保留过冲**（`findState.dragOver`），否则「拖过头再拖回来」会粘住；`Split` 的
       `splitOvershoot` 做的是同一件事。
  - 「结构」表里 `internal/view` 那一行补一句：`find.go` 里匹配列表的高度可拖拽（`findDivider`）。
- [ ] `docs/superpowers/specs/2026-11-21-markdown-preview-design.md`：在「实施后的变更（写回）」末尾追加
  「### 搜索面板的可调高度（新增，2026-11-23）」：最小/默认/最大高度、拖拽带取代旧 `Gap`、双击复位、
  高度只在会话内、上下文岛不参与拖拽。
- [ ] 本计划末尾追加「执行后的修订」（只追加，不改写上面的任务步骤）：实际做了什么、与计划的偏离、
  以及 `TestFindIslandsKeepTheirGaps` 最终钉住的数字。

### 任务 4：验收

- [ ] 自动化：

```sh
go vet ./... && gofmt -l . && go test ./...
```

预期：vet 无输出、gofmt 无输出、测试全绿。

- [ ] 手工（`go run .`，至少 Windows）：
  - `Cmd+F` 搜一个只出现一次的短语 → 匹配列表岛仍是约 5 行高，不再是一行 + 横向滚动条。
  - 把指针放到两块数据岛之间 → 光标变成上下箭头，出现一条主色细线；按住下拉 → 列表变高、上下文岛被推下去；
    松开后高度留在新位置。
  - 一直上拖 → 停在最小高度（还能看到约 3 行 + 滚动条）；一直下拖 → 停在最大高度。
  - 拖到最大后再继续下拉一段，然后上推 → 高度立刻开始变小（不粘住）。
  - 双击分隔条 → 回到默认高度。
  - 拖动分隔条时不应跳转正文、不应改 `n/m`、不应清空查询。
  - 命中很多时列表内滚动正常，横向长行仍可横滚；`Esc` 关闭再 `Cmd+F` 打开，高度还是上次拖的位置。

## 未决问题（本计划按括号里的选择执行，用户可推翻）

1. 高度要不要**跨会话记住**（本计划：不记，跟 `zoom` 一致；要记就加 `config.Config.FindHeight` +
   `normalize` 的钳制 + 一条 `TestConfigFindHeightClamped`）。
2. 拖拽带 10 DIP 让两岛间距从 6 变 10（本计划接受；若要保持 6，把 `findGrip` 改成 `t.Space(1.5)`，
   代价是命中区变窄，并把 `TestFindIslandsKeepTheirGaps` 的上界改小）。
3. 最大高度固定 320，**不按窗口高度自适应**（本计划接受；小窗口下面板底边可能超出窗口，需另开「按窗口高
   夹紧」的改动，那时得给 `State` 补一个 `SetContentHeight` 钩子，像 `SetContentWidth` 那样）。
4. 上下文岛要不要也参与拖拽（本计划：不参与；它的高度由内容与 148 上限决定）。
5. 内容很少时要不要「按内容自适应、但不低于最小高度」（本计划：固定高度，拖多少是多少；
   自适应会让「拖动」在内容不足时看起来没反应）。

## 后续（本期不做，等 mygo）

正文内命中高亮走 **mygo 引擎侧**：`Options.Mark []MarkRange` + `MarkVersion uint64`，
见 `docs/superpowers/plans/2026-11-23-mygo-mark-ranges.md`（在 mygo 仓库执行）。
PR 合并、我们升级依赖之后，另写一份本仓库的计划：`refreshMatches` / `gotoMatch` 里把
`s.find.matches` 变成区间交给 `s.md.Mark`，删掉 README「已知限制」里「原生渲染模式下搜索不高亮正文」一条。

## 执行后的修订（2026-11-23）

1. **任务 1、2 各一个提交，任务 3 一个 `docs:` 提交**（与本仓库既有的 `feat:` → `docs:` 流程一致）：
   `feat: 搜索面板的匹配列表有最小与最大高度`、`feat: 搜索面板可以拖动调整匹配列表高度`、
   `docs: 搜索面板的可调高度写回 README/AGENTS/CHANGELOG/规格`。
2. **计划里的测试坐标有误，已改**：任务 2 的用例原本按 `tt.Press(200, band)` 写，但面板浮在正文右侧
   （900 宽窗口里占 472..892），x=200 落在正文上，拖拽带收不到 press，表现为「高度一路不变」。
   改为按最后一帧的布局取带子正中：`findDividerPoint(t, tt)` 返回
   `list.X + list.W/2, list.Y + list.H + findGrip/2`，五个用例都用它。
3. **`Height` 量的是滚动容器，不是岛**：`Height(findListHeightOf())` 加在 `ScrollBoth` 上，岛的外框还要加
   `2*findIslandPad + 2`（实测 132 → 146）。所以 `TestFindListKeepsItsHeightWhateverTheCount` 断言
   「一条命中与三十条命中的高度相同」而不是写死 132；其余断言都走 `findListHeightOf()`。
4. **`TestFindIslandsKeepTheirGaps` 最终钉住的数字**：输入↔列表 4~8 DIP（实测 8，即主题的
   `t.Space(1.5)=6` 加岛边框）、列表↔上下文 `findGrip±2`（实测 10）。既有的
   `TestFindPanelShowsIslandsApart`（`gap < 5` 判失败）不用改，仍是第二道守卫。
5. **新增一个只给测试用的读屏名**：输入岛 `.Label("搜索输入")`，与 `匹配行` / `命中上下文` 同理，
   为的是能量到三块岛的间距；输入框自己的 `Label("在文档中查找")` 不变。
6. **没有偏离的项**：拖拽按每帧增量累加 + `dragOver` 过冲、双击复位、`RoleSplitter` + 中文读屏名、
   hover/拖动才画 1 DIP 主色线、高度不落盘、上下文岛不参与拖拽——均按计划 D1~D11 实现。
7. **手工验收未做**（本会话没有图形界面）：自动化 `go vet ./... && gofmt -l . && go test ./...` 全绿；
   计划「任务 4」里的手工项（真实窗口里的拖动、光标形状、双击复位、深浅主题下的线色）留给用户确认。

### 追加需求（2026-11-23，用户看过第一期后追加）

用户在看过第一期效果后追加三条，并要求把「延后的 Minor」一起修完：

1. **上下文预览岛也要能拖**（原计划只有列表）。`findDivider` 于是变成 `findGripBand(c, which)`：
   两条带子（列表下、上下文下各一条），各有默认/最小/最大高度——上下文 148 / 96 / 420。
   上下文岛因此从「内容自适应 + `MaxHeight(148)`」变成显式高度，内部滚动用 `Grow(1)` 吃掉剩下的空间。
2. **输入框 + 列表 + 上下文的总高度不能超过窗口**：新增 `State.SetContentHeight`
   （`main.go` 用 `win.ContentSize()` 接）+ `findRoom` + `findHeightMaxOf` + 每帧的 `fitFindHeights`。
   窗口变矮时**上下文先让出高度**、再轮到列表，只改显示高度（`shownList`/`shownPreview`），
   用户拖出来的值保留，窗口变高会自己恢复。`TestThePanelNeverGrowsPastTheWindow` 用 520 高的窗口
   把两个岛都拖到底，断言上下文岛的下边不出窗口。
3. **延后的两件 Minor 一起修**：带子可聚焦、`↑`/`↓` 调整（`TestTheBandsMoveWithTheArrowKeys`）；
   最大高度的窗口自适应（就是第 2 条）。
4. **两处实现上的教训**（已写进 AGENTS 的坑）：
   - `HandleInput` 在下一帧构建之前跑，那里**拿不到 `*ui.Context`**（主题、shell 都没有）：余量必须在构建期算好、
     闭包只捕数字；直接在闭包里用 `c.Theme()` 会 panic。
   - **`dy == 0` 的按下不能写高度**，否则点一下带子就把「用户想要的高度」改写成「窗口现在能显示的高度」，
     窗口变高后不恢复（`TestDoubleClickingABandPutsTheHeightBack` 抓到过）。
5. **高度量在岛上了**（`Height(shown*)` + 内层 `Grow(1)`）：`岛.Y + 岛.H` 因此等于下一条带子的位置，
   `dragBand` / `TestFindIslandsKeepTheirGaps` 都靠它。这条推翻了上一轮「`Height` 量在滚动容器上」的说法。
6. 仍然**没有**做的：高度不落盘（会话内，与 `zoom` 一致）；带子只响应鼠标拖动与 `↑`/`↓`，
   没有 Home/End/PageUp 之类的跳跃。
