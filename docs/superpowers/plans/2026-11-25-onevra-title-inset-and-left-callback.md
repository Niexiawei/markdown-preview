# onevra 标题槽内距选项与左栏折叠回调 实现计划

> **面向 AI 代理的工作者：** 必需子技能：使用 subagent-driven-development（推荐）或 executing-plans 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** 给 `sidebar.NewShell`（onevra）加两个能力：① 标题槽左右内距选项 `TitleInsetLeft` / `TitleInsetRight`（默认 24 / 8，非正值取默认）；② 左侧栏折叠状态变化回调 `OnLeftExpanded func(expanded bool)`。本应用（markdown-preview）随之消费：`TitleInsetLeft: 12` 取代 `tabBarAt` 的 −12 拉回，删掉 `titleInsetGap` / `titleInsetWanted` 常量（AGENTS.md 预言的「onevra 新增了间距选项」就此兑现）。

**架构：** fork 在 `D:\golang\onevra`（main，与 upstream 0.2026.19 同步，origin = Niexiawei/onevra）。onevra 侧只动 `ShellOptions`、`build` 的标题行与 `SetLeftExpanded`；应用侧经**未提交的** `replace` 指向 fork 本地验证，等 upstream 发版后才正式提交改造（`replace` 用本地路径，入库会让 CI 拉不到模块）。

**技术栈：** Go 1.27+、mygo v0.10.0（两侧一致）、onevra fork。

**规格：** 无独立规格；需求来自用户 2026-11-25 的指示（① 标题槽左右 padding 入口，默认 24/8；② 左栏折叠回调；③ fork 已建，后续走 PR 合入主库）。背景：本仓库 AGENTS.md「标题槽」条目与 `2026-11-22-tabs-find-toc-actions-zoom.md` 修订 #7/#9（−12 拉回的由来）；onevra fork 的 `AGENTS.md`（验证与文档规则）。

## 设计决策（实施者按此执行，不要另立方案）

| # | 决策 | 理由 |
| --- | --- | --- |
| D1 | 字段名 `TitleInsetLeft float32` / `TitleInsetRight float32`，文档照 fork 惯例写「non-positive values use 24 / 8」。 | fork 的 `LeftWidth` 等字段就是「non-positive values use X」约定；Left/Right 与既有 `LeftHeader` / `Right` 家族一致。 |
| D2 | **语义：`TitleInsetLeft` 是标题与侧栏头部内容之间的间隙**（默认 24 = 标题与主面对齐）。实现上：`shift = inset - 24`，三处 `titleInset` 公式**全部不动**（trailing=24、leading 行宽=clearance−24、折叠 floor=8），标题行改为 `Padding(0, insetRight, 0, titleInset+shift)`。展开两种分支下标题 X 都左移 `24−P`、面板末端不动（面板与标题的间隙恰为 P）；折叠/动画按同一差值平移，保持 toggle 避让。 | mygo 的 padding 原始相加不钳制（`layout.go` `contentX() = pad[3]+border[3]`），负值与今天应用侧 `Translate(-12)` 净几何完全一致（折叠态首个标签仍可能被行 Clip 切掉 4 DIP——与现状相同，非回归）。若把行宽也改成 `clearance−P`，面板会贴住标题（视觉回归），故行宽保持按默认 24 推导。 |
| D3 | `OnLeftExpanded func(expanded bool)`（true = 展开，false = 折叠）。在 `SetLeftExpanded` 里**值发生变化时**调用（先 `refresh()` 后回调）；构造期（`leftOpen: !options.LeftCollapsed`）不调用；nil 安全；在窗口线程触发；文档写明「回调内不要再调 `SetLeftExpanded`」（不做重入保护）。 | `SetLeftExpanded` 是 `leftOpen` 唯一的变更入口（fork `shell.go:237`），toggle 按钮也走它；词汇与既有 `LeftExpanded()` / `SetLeftExpanded` 一致。重入保护是 YAGNI，文档约定足够。 |
| D4 | 应用侧 `ContentTitle` 改为直接 `s.tabBar(c)`；`tabBarAt(c, pull)` 并回 `tabBar`（`Translate(pull, 0)` 删除）；`titleInsetGap` / `titleInsetWanted` 删除；main.go `ShellOptions` 加 `TitleInsetLeft: 12`；测试用 `View` 的标签行加 `Padding(0, 8, 0, 12)` 仿外壳（新增测试钉住首标签 X == 12）。 | AGENTS.md「标题槽」条目明确：onevra 新增了间距选项就改用它并删掉常量。⌄ 按钮不受影响（trailing padding 独立，仍为 8）。 |
| D5 | **提交门**：fork 在独立分支 `feat/title-inset-and-left-callback` 开发，本地提交视为已授权（用户 2026-11-25：「帮我在独立分支中开发」）；推送与 GitHub PR 由用户自行执行。应用的改造提交等 upstream 发版（预计 0.2026.20）后进行，`replace` 永不入库。 | fork AGENTS 的授权约束由用户的分支指示满足；本地路径 replace 入库会弄红 CI。 |
| D6 | fork 的 CHANGELOG（仓库根）Unreleased 加两条：标题槽内距选项、折叠回调。网站不加页（ShellOptions 字段不是新组件，PR #17 也未加页）。`OnLeftExpanded` 在应用侧**暂无消费方**（用户确认），仅组件能力 + fork 测试覆盖。 | fork 文档规则：「Update a document only when a fact its reader needs changes」。 |

## 文件结构

**fork（D:\golang\onevra，命令在 `source/` 下跑）：**
- 修改 `source/shells/sidebar/shell.go`：`ShellOptions` 三个新字段、`build` 标题行的 inset/shift 与 Padding、`SetLeftExpanded` 回调。
- 修改 `source/shells/sidebar/shell_title_test.go`：新选项的几何测试（扩展既有 `TestShellContentTitle` 的基准）。
- 修改 `source/shells/sidebar/shell_left_header_test.go`：trailing + 选项组合不破坏「trailing 不移动标题」。
- 修改 `source/shells/sidebar/shell_state_test.go` 或新增用例：回调的触发时机。
- 修改 `CHANGELOG.md`：Unreleased 两条。

**应用（D:\golang\markdown-preview）：**
- 修改 `go.mod`：本地验证期加 `replace github.com/oneo-me/onevra/source => ../onevra/source`（不入库）；正式提交时删掉并 bump 版本。
- 修改 `main.go`：`TitleInsetLeft: 12`。
- 修改 `internal/view/view.go`：`ContentTitle` 直调 `tabBar`；删两常量；`View` 标签行加仿外壳 padding。
- 修改 `internal/view/tabs.go`：`tabBarAt` 并回 `tabBar`。
- 测试：`internal/view` 新增/扩展一条首标签 X 的测试；`AGENTS.md`「标题槽」条目改写；`CHANGELOG.md` 修复条目（折叠态 4 DIP）。

## 全局约束

- fork：Go 命令在 `source/` 下跑；先扩展既有测试；不建 worktree；提交/推送须用户授权；CHANGELOG 不写验证细节。
- 应用：不加 build tag、三平台可编译（`GOOS=darwin go build ./...` 作编译门）；测试只用 `t.TempDir()` 与 `ui.NewTester`；文档回写并入同一提交；提交前 `go vet ./... && gofmt -l . && go test ./...` 干净。
- 平台无关：新选项与回调不做 GOOS 分支（macOS 的行为差异全部由既有 `LeftToggleTrailing` 分支承担）。

## 审查重点（Review Focus）

1. **默认路径零回归**：不设新选项时，fork 既有几何（`TestShellContentTitle` 的 X==236、`TestShellLeftToggleTrailing` 的标题不动）必须原样通过——`shift=0` 下所有公式与今天逐字节等价。
2. **折叠态的负 padding**：`titleInset+shift = 8−12 = −4` 依赖 mygo 不钳制 padding——fork 测试钉住实际值；若实测被钳到 0，折叠态标题在 40（比今天的 36 右 4 DIP），作为发现记录并让用户裁决，不静默接受。
3. **⌄ 按钮不动**：leading shift 不得影响 trailing（应用侧 ⌄ 仍在标题槽右缘内距 8 处）——fork 测试断言 trailing 边距，应用侧由既有 tabs 测试覆盖。
4. **回调时序**：同值 `SetLeftExpanded` 不触发；toggle 点击触发一次；构造与窗口状态恢复不触发——fork 测试钉住计数与取值序列。
5. **`LeftToggleTrailing` 组合**：macOS 分支（trailing toggle）下 `TitleInsetLeft: 12` 的标题 X 必须与 leading 分支相等——fork 测试扩展既有 trailing 用例。
6. **replace 不入库**：任务 4 的一切停留在本地工作区；`git status` 里 go.mod 的 replace 出现在任何提交里都是事故。

---

### 任务 1：fork——TitleInsetLeft / TitleInsetRight

**准备：** 在 `D:\golang\onevra` 建独立分支：`git checkout -b feat/title-inset-and-left-callback`（用户将自行推送并向 upstream 发 PR；后续任务都在该分支上进行，任务 3 的 replace 读的是检出的工作树，任务 3 期间 fork 必须停留在该分支的检出状态）。

**文件：**
- 修改：`D:\golang\onevra\source\shells\sidebar\shell.go`
- 测试：`D:\golang\onevra\source\shells\sidebar\shell_title_test.go`、`shell_left_header_test.go`

- [ ] **步骤 1：编写失败的测试**

在 `shell_title_test.go` 追加（fixture 与既有 `TestShellContentTitle` 相同：默认 `LeftWidth`、`TitleBar{Height: 48, Left: 80}`，基准 X==236）：

```go
func TestShellTitleInsetOptions(t *testing.T) {
	// 相对断言，不硬编码平台差异：base 与 shifted 用同一几何，只差内距。
	rect := func(insetLeft, insetRight float32) ui.Rect {
		s, _ := fixture()
		s.options.TitleInsetLeft, s.options.TitleInsetRight = insetLeft, insetRight
		var title ui.Rect
		s.options.ContentTitle = func(c *ui.Context) {
			ui.Text(c, "Current document").SingleLine().Grow(1).MinWidth(0).DrawOver(
				func(_ *ui.Painter, r ui.Rect) { title = r })
		}
		tt := ui.NewTester(s.view, 820, 600)
		tt.SetTitleBar(ui.TitleBar{Height: 48, Left: 80})
		tt.SetPreferences(ui.Preferences{ReduceMotion: true})
		return title
	}
	base := rect(0, 0) // 非正值取默认 24/8，与既有 TestShellContentTitle 的 236 同基准
	shifted := rect(12, 20)
	if shifted.X != base.X-12 {
		t.Fatal("TitleInsetLeft did not move the title by 24-12", base, shifted)
	}
	if shifted.X+shifted.W != base.X+base.W-12 {
		t.Fatal("TitleInsetRight did not narrow the trailing room", base, shifted)
	}
	collapsed := func(insetLeft float32) ui.Rect {
		s, _ := fixture()
		s.options.TitleInsetLeft = insetLeft
		var title ui.Rect
		s.options.ContentTitle = func(c *ui.Context) {
			ui.Text(c, "Current document").SingleLine().Grow(1).MinWidth(0).DrawOver(
				func(_ *ui.Painter, r ui.Rect) { title = r })
		}
		tt := ui.NewTester(s.view, 820, 600)
		tt.SetTitleBar(ui.TitleBar{Height: 48, Left: 80})
		tt.SetPreferences(ui.Preferences{ReduceMotion: true})
		s.SetLeftExpanded(false)
		tt.Frame()
		return title
	}
	baseC, shiftedC := collapsed(0), collapsed(12)
	if shiftedC.X != baseC.X-12 {
		t.Fatal("collapsed title did not shift by 24-12", baseC, shiftedC)
	}
}
```

在 `shell_left_header_test.go` **新增**一条组合测试：`TitleInsetLeft: 12` + `LeftHeader` + `LeftToggleTrailing: true`，断言展开态标题 X 与 leading 分支相等（`LeftWidth: 252` 几何下同为 256）。

- [ ] **步骤 2：运行测试验证失败**

运行（在 `source/` 下）：`go test ./shells/sidebar/ -run TestShellTitleInset -v`
预期：FAIL——`TitleInsetLeft did not move the title`（选项尚不存在，编译失败也算 FAIL）。折叠态断言按实测钉住（预期基准 −12；若 mygo 钳制负 padding 则实测基准 −8，记 Ruling 并报告用户——折叠态会比今天右 4 DIP）。

- [ ] **步骤 3：实现**

`shell.go` 的 `ShellOptions` 追加两个字段（D1 文案）；`build` 里标题行之前：

```go
inset := float32(24)
if s.options.TitleInsetLeft > 0 { inset = s.options.TitleInsetLeft }
insetRight := float32(8)
if s.options.TitleInsetRight > 0 { insetRight = s.options.TitleInsetRight }
shift := inset - 24
```

三处 `titleInset` 赋值不动；标题行 `.Padding(0, insetRight, 0, titleInset+shift)`。

- [ ] **步骤 4：运行测试验证通过**

运行（在 `source/` 下）：`go test ./shells/sidebar/`（整包；既有 `TestShellContentTitle`、`TestShellLeftToggleTrailing` 不得红）
预期：PASS。折叠态差异（若有）已按步骤 2 的说明钉住并记账。

- [ ] **步骤 5：格式与整包验证**

运行（在 `source/` 下）：`gofmt -l . && go vet ./... && go test ./...`
预期：无输出、全绿。

- [ ] **步骤 6：Commit（待用户授权）**

```bash
git commit -m "feat: add ShellOptions.TitleInsetLeft/Right for the content title's padding"
```

### 任务 2：fork——OnLeftExpanded 回调

**文件：**
- 修改：`D:\golang\onevra\source\shells\sidebar\shell.go`
- 测试：`D:\golang\onevra\source\shells\sidebar\shell_state_test.go`

- [ ] **步骤 1：编写失败的测试**

```go
func TestShellLeftExpandedCallback(t *testing.T) {
	s, _ := fixture()
	var calls []bool
	s.options.OnLeftExpanded = func(expanded bool) { calls = append(calls, expanded) }
	tt := ui.NewTester(s.view, 820, 600)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	if len(calls) != 0 {
		t.Fatal("construction notified the callback", calls)
	}
	s.SetLeftExpanded(false) // 与初值相同：不触发
	s.SetLeftExpanded(true)
	s.SetLeftExpanded(false)
	if len(calls) != 2 || calls[0] != true || calls[1] != false {
		t.Fatal("callback sequence wrong", calls)
	}
	if err := tt.Click("Toggle sidebar"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if len(calls) != 3 || calls[2] != true {
		t.Fatal("toggle click did not notify", calls)
	}
}

func TestShellLeftExpandedCallbackNil(t *testing.T) {
	s, _ := fixture() // OnLeftExpanded 为 nil：折叠/展开不 panic
	tt := ui.NewTester(s.view, 820, 600)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	s.SetLeftExpanded(false)
	s.SetLeftExpanded(true)
	tt.Frame()
	if !s.LeftExpanded() {
		t.Fatal("nil callback broke the toggle")
	}
}
```

- [ ] **步骤 2：运行测试验证失败**

运行（在 `source/` 下）：`go test ./shells/sidebar/ -run TestShellLeftExpandedCallback -v`
预期：FAIL，编译错误 `s.options.OnLeftExpanded undefined`。

- [ ] **步骤 3：实现**

`ShellOptions` 追加字段（D3 文案）；`SetLeftExpanded`：

```go
func (s *Shell) SetLeftExpanded(open bool) {
	changed := open != s.leftOpen
	s.leftOpen = open
	s.refresh()
	if changed && s.options.OnLeftExpanded != nil {
		s.options.OnLeftExpanded(open)
	}
}
```

- [ ] **步骤 4：运行测试验证通过**

运行（在 `source/` 下）：`go test ./shells/sidebar/`（整包）
预期：PASS。

- [ ] **步骤 5：格式与整包验证**

运行（在 `source/` 下）：`gofmt -l . && go vet ./... && go test ./...`
预期：无输出、全绿。

- [ ] **步骤 6：CHANGELOG + Commit（待用户授权）**

`CHANGELOG.md` Unreleased 加两条（任务 1、2 各一条，用户视角一句话）；提交：

```bash
git commit -m "feat: notify the left sidebar's expanded state via ShellOptions.OnLeftExpanded"
```

推送 fork 分支（如 `feat/title-inset-and-left-callback`）由用户执行并向 upstream 发 PR；PR 描述附两个选项的用法示例。

### 任务 3：应用——本地验证（不入库）

**文件：** 修改 `go.mod`（仅本地）、`main.go`、`internal/view/view.go`、`internal/view/tabs.go`；测试 `internal/view/view_test.go` 或 `tabs_test.go`。**本任务一切改动停留在本地工作区，`git status` 的 go.mod replace 不得出现在任何提交里（审查重点 6）。**

- [ ] **步骤 1：接上 fork 并写失败的测试**

`go.mod` 加 `replace github.com/oneo-me/onevra/source => ../onevra/source`；新测试：`View` 渲染后 `tt.Find` 首个标签的 X == 12（仿外壳 `Padding(0, 8, 0, 12)` 之前应为 FAIL——当前首标签在 0）。

- [ ] **步骤 2：运行测试验证失败**

运行：`go test ./internal/view/ -run TestTabs -v`
预期：新增测试 FAIL（首标签 X 为 0 或 −12）。

- [ ] **步骤 3：实现**

- `main.go`：`ShellOptions` 加 `TitleInsetLeft: 12,`（注释说明取代 tabBarAt 的拉回）。
- `view.go`：`ContentTitle` 改 `s.tabBar(c)`；删 `titleInsetGap` / `titleInsetWanted` 常量；`View` 的标签行 `.Children` 前加 `Padding(0, 8, 0, 12)`（仿外壳）。
- `tabs.go`：`tabBarAt` 并回 `tabBar`（删 `pull` 参数与 `Translate(pull, 0)`），文件头注释同步。

- [ ] **步骤 4：运行测试验证通过**

运行：`go vet ./... && gofmt -l . && go test ./...`
预期：全绿（含既有标签、`TestShellSlotsBuild`）。

- [ ] **步骤 5：截图看图**

`MDPREVIEW_SHOT_DIR=/tmp/shots-inset go test ./internal/view -run TestScreenshots`，人工核对标签条起点与 ⌄ 位置和改造前一致（`02-tree` 等用例）；`GOOS=darwin go build ./...` 编译门。

- [ ] **步骤 6：保持未提交，向用户报告截图**

### 任务 4：应用——upstream 发版后正式提交

**前置：** 用户向 upstream 发的 PR 已合并且 onevra 发布了含两个能力的新版本（预计 0.2026.20）。未发版则本任务挂起。

- [ ] **步骤 1：** `go.mod` 删 `replace`，`go get github.com/oneo-me/onevra/source@v<新版本>`；任务 3 的其余改动原样保留。
- [ ] **步骤 2：** `go vet ./... && gofmt -l . && go test ./...` 全绿。
- [ ] **步骤 3：** 文档回写：AGENTS.md「标题槽」条目改写（间距来自 `TitleInsetLeft: 12`，常量已删）；`README.md` 若有 −12 相关描述同步；`CHANGELOG.md` 修复节：折叠侧栏时首个标签不再压到折叠按钮（若折叠态 X 与改造前确有差异，以实测为准）。
- [ ] **步骤 4：** Commit：`feat: 标题槽内距改用 onevra 的 TitleInsetLeft，删掉 −12 拉回`。

## 执行后的修订

（执行过程中追加。）

- **标题槽 shift 改为随展开进度渐变**（用户 2026-11-25 实机反馈：折叠态标题压进 toggle −4 DIP 不能要）：fork 的 `titleInset += (inset - 24) * leftProgress`——折叠/动画中保持外壳原公式（无偏移），展开后全量偏移，动画中平滑渐变；回调无需参与（外壳自己知道状态）。应用侧 `TitleInsetLeft` 由 12 改 **16**（展开态标题在 leftWidth+8，折叠态回到 toggle 后 8 DIP），View 仿垫同步、测试钉 316。fork 提交 `872742f`。
- **16/8 提为 view 导出常量 `TitleInsetLeft` / `TitleInsetRight`**（用户问「到底谁在显示 tab」时暴露的漂移隐患）：`main.go` 的 shell 选项与 `View` 的仿垫同源，单一来源。
- **文档回写提前备好**（用户指示）：AGENTS.md「标题槽」条目改写（选项语义 + 共享常量 + 删除旧适配）、「macOS 头部」与「改标签栏/标题行/侧栏」两处同步、CHANGELOG 修复条目；这些改动**随任务 4 一并提交**（文档先行提交会描述尚未入库的代码），任务 4 时记得同步 bump AGENTS.md 头部的 onevra 版本号。
