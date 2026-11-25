# macOS 头部收纳：折叠按钮尾随、项目面板下沉 实现计划

> **面向 AI 代理的工作者：** 必需子技能：使用 subagent-driven-development（推荐）或 executing-plans 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** Windows / Linux 完全保持现状；macOS 上把侧栏折叠按钮移到标题栏**末尾**（onevra `LeftToggleTrailing`），同时把侧栏顶部的项目面板（三个树操作 + 切换项目）从标题栏槽位（`LeftHeader`）**下沉**为侧栏主体的第一行——标题栏在 macOS 上只留系统交通灯与折叠按钮。

**架构：** 平台判断只活在一处：`view.New` 里 `s.panelDown = runtime.GOOS == "darwin"`，导出 `PanelDown()` 给 main.go 复用为 `LeftToggleTrailing: state.PanelDown()`。view 侧提取面板的两行拼装（`panelItems` 共享子元素），`leftHeader`（标题栏槽位，macOS 上构建为空）与 `panelRow`（主体首行，高 40 DIP）共用它；`Left` 与 `View` 两条拼装路径按 `panelDown` 分叉。标签条不动：onevra 的 trailing 分支几何上不改变 `titleInset = 24`，`tabBarAt(c, -12)` 适配照旧。

**技术栈：** Go 1.27+、`github.com/egoist/mygo v0.10.0`、`github.com/oneo-me/onevra/source v0.2026.19`（`ShellOptions.LeftToggleTrailing`，PR #17）。

**规格：** 无独立规格；需求来自用户 2026-11-25 的两条指示（① windows/linux 保持现状；② macos 启用尾随折叠按钮 + 顶部 action 下沉为单独 Row、不再使用 `LeftHeader`）。背景：onevra PR #17 与其自测 `TestShellLeftToggleTrailing`（钉住「尾随 toggle 不移动 ContentTitle」）；标题槽 24 DIP 的由来与 −12 拉回适配见 `docs/superpowers/plans/2026-11-22-tabs-find-toc-actions-zoom.md` 修订 #7/#9，本计划不推翻它。

## 设计决策（实施者按此执行，不要另立方案）

| # | 决策 | 理由 |
| --- | --- | --- |
| D1 | `State.panelDown bool`（未导出字段）在 `New` 里赋值 `runtime.GOOS == "darwin"`；导出 `func (s *State) PanelDown() bool`。main.go 用它作 `LeftToggleTrailing: state.PanelDown()`，`LeftHeader` 仍无条件传 `state.LeftHeader`（panelDown 时构建为空）。 | 平台决策只有一个出处（view），main.go 保持声明式；不加 build tag，三平台都可编译。测试是同包白盒，直接改 `s.panelDown` 渲染两种形态。 |
| D2 | 提取 `func (s *State) panelItems(c *ui.Context)`（= 现有 `leftHeader` 的两个子元素：`treeBulkActions` + `projectSwitcher`）；`leftHeader` 改为用它；新增 `func (s *State) panelRow(c *ui.Context)` = `Row.FillWidth().Height(panelRowHeight).AlignItems(Center).Gap(Space(0.5)).Children(s.panelItems)`，`const panelRowHeight float32 = 40`。 | 两处拼装必须严格同构，否则 macOS 与 Windows 的面板内容会漂移；高度 40（图标 24 居中）是主体行的节奏，标题栏槽位原本是 48。 |
| D3 | `Left` 列：`[panelRow?][treeSearchRow?][fileTree]`；`View` 侧栏列：`panelDown ? panelRow : leftHeader` 打头。不加分隔线、面板行与树之间不加间距（截图定稿后允许 ±4 DIP 微调行高）。 | 搜索行贴着树（它过滤树）；面板是持久内容所以在最上。下沉后主体顶部不再是标题栏，行高按内容定。 |
| D4 | `LeftHeader` 在 `panelDown` 时直接 `return`（构建空）。onevra 的 trailing 分支不看 `LeftHeader != nil` 就建行（内部自行判空），折叠/动画走的 else 分支条件含 `leftOpen && leftProgress == 1`，空构建不触发任何分支副作用。 | 让 onevra 认为槽位存在但为空，与「槽位归 view 管」一致；行为已被 onevra 自测覆盖。 |
| D5 | `titleInsetGap` / `titleInsetWanted` / `tabBarAt` 不动；README「已知限制」与标签相关测试不动。 | onevra `TestShellLeftToggleTrailing` 钉住「尾随 toggle 不移动 ContentTitle」；三种模式（尾随/常规/折叠）下标签起点在窗口坐标里不变（`leftInset + leftWidth + 16`），我们的 −12 拉回继续成立。 |
| D6 | 文档回写并入行为提交：任务 1 提交带 AGENTS.md（结构表 + 坑），任务 2 提交带 AGENTS.md（改动指引）+ README + CHANGELOG。CHANGELOG 只写用户可见变化（macOS 一条）。 | AGENTS.md 文档回写硬规则；CHANGELOG 不给内部重构开条目。 |

## 文件结构

- 修改 `internal/view/state.go`：`panelDown` 字段初始化（`New`）+ 导出 `PanelDown()`（字段声明放 `view.go` 的 State 或 state.go，跟随现有字段归属）。
- 修改 `internal/view/view.go`：`LeftHeader` 判空返回、`panelItems` / `panelRow` 提取、`Left` 与 `View` 按 `panelDown` 分叉。
- 创建 `internal/view/panel_test.go`：下沉形态的四条测试。
- 修改 `internal/view/screenshots_test.go`：加一个 shot case 看下沉后的样子。
- 修改 `main.go`：`LeftToggleTrailing: state.PanelDown()` + 注释。
- 修改 `AGENTS.md`、`README.md`、`CHANGELOG.md`：文档回写。

## 全局约束

- 平台分支只许 `runtime.GOOS` 运行时判断，不加 `//go:build`，三平台保持可编译。
- 不新增第三方依赖；图标只用 `icons/lucide`；颜色取 `AppTheme`；按钮必须带 label。
- 测试只用 `t.TempDir()` 与 `ui.NewTester`，不建真实窗口。
- 提交前 `go vet ./... && gofmt -l . && go test ./...` 干净；中文提交信息，前缀 `feat:` / `docs:`。
- macOS 实机部分（交通灯间距、动画中的按钮跳变）写进提交信息「手动验证」，不假装测过。

## 审查重点（Review Focus）

1. macOS 展开/折叠动画中折叠按钮跳回标题栏左端（onevra 设计：折叠与动画期间保持 leading）——期待：按钮不与交通灯重叠（onevra 的 `leftInset` 处理），面板始终在主体首行不随动画移动。钉住：`TestTheHeaderSlotStaysEmptyWhenThePanelSinks` + onevra 自测 `TestShellLeftToggleTrailing`。
2. 侧栏拖到最小宽 200 DIP 时下沉面板仍摆得下——期待：三个树操作与切换按钮不重叠、不溢出列宽。钉住：`TestTheSunkPanelFitsANarrowSidebar`。
3. 标签条起点在尾随/常规/折叠三种模式下不变——期待：第一个标签的 X 与现状一致（−12 拉回仍生效）。钉住：既有 `TestTabsTuckUnderTheSidebarHeader` + onevra `TestShellLeftToggleTrailing`（本仓库无法渲染真 shell，几何由 onevra 测试守）。
4. Windows / Linux 逐像素保持现状——期待：`LeftHeader` 照常渲染面板，`Left`/`View` 不出现面板行。钉住：`TestThePanelStaysInTheHeaderOutsideMacOS` + 既有 `TestShellSlotsBuild`。
5. 项目下拉（`projectsOpen`）的锚点随按钮下沉后仍可开合——期待：点「切换项目」开列表、再点关列表，位置跟随新行。钉住：`TestThePanelSinksIntoTheSidebarBodyOnMacOS` 末段的点击断言。

---

### 任务 1：view 层——panelDown 与下沉面板行

**文件：**
- 修改：`internal/view/state.go`（`New` 里初始化）
- 修改：`internal/view/view.go`（字段声明、`PanelDown`、`LeftHeader`、`panelItems`/`panelRow`、`Left`、`View`）
- 测试：`internal/view/panel_test.go`（新建）

- [ ] **步骤 1：编写失败的测试**

新建 `internal/view/panel_test.go`，四条测试（复用 `newState` / `newTester` / `writeDoc` 夹具）：

```go
func TestThePanelSinksIntoTheSidebarBodyOnMacOS(t *testing.T) {
	// s := newState(t)；writeDoc(t, root, "docs/notes.md", "# Notes\n")；s.Refresh()
	// s.panelDown = true
	// tt := newTester(s)
	// HasText("切换项目")；r := tt.Find("切换项目")；doc := tt.Find("notes.md")
	// 断言 r.Y + r.H <= doc.Y（面板整行在树之上）
	// tt.Click("切换项目") 后 s.projectsOpen == true（下拉锚点随行下沉仍可开合）
}

func TestTheHeaderSlotStaysEmptyWhenThePanelSinks(t *testing.T) {
	// s.panelDown = true
	// tt := ui.NewTester(func(c *ui.Context) { s.LeftHeader(c) }, 280, 48)
	// tt.Texts() 既不含 "切换项目" 也不含 "展开全部目录树"
}

func TestThePanelStaysInTheHeaderOutsideMacOS(t *testing.T) {
	// 默认 s（panelDown == false）
	// ui.NewTester(s.LeftHeader, 280, 48) HasText("切换项目")
	// ui.NewTester(func(c *ui.Context) { s.Left(c) }, 280, 600) 的 Texts() 不含 "切换项目"
}

func TestTheSunkPanelFitsANarrowSidebar(t *testing.T) {
	// s.panelDown = true
	// tt := ui.NewTester(func(c *ui.Context) { s.Left(c) }, 200, 600)
	// HasText("切换项目")
	// bulk := tt.Find("展开全部目录树")；btn := tt.Find("切换项目")
	// 断言 bulk.X + bulk.W <= btn.X 且 btn.X + btn.W <= 200（不重叠、不溢出）
}
```

- [ ] **步骤 2：运行测试验证失败**

运行：`go test ./internal/view/ -run TestThePanel -v` 与 `-run TestTheSunkPanel -v`
预期：FAIL，编译错误 `s.panelDown undefined`。

- [ ] **步骤 3：实现**

`state.go` 的 `New` 末尾（返回前）：`s.panelDown = runtime.GOOS == "darwin"`（补 `runtime` import）。
`view.go`：

- `State` 增加 `panelDown bool`（注释说明：macOS 把面板沉进侧栏主体、标题栏只留尾随折叠按钮；测试直接改它渲染两种形态）与导出方法 `func (s *State) PanelDown() bool { return s.panelDown }`（注释说明 main.go 用它作 `LeftToggleTrailing`，两处行为同源）。
- `leftHeader` 拆成：`panelItems`（两个子元素的调用，签名 `func (s *State) panelItems(c *ui.Context)`）+ `leftHeader`（原 Row 属性不变，`Children(s.panelItems)`；开头加 `if s.panelDown { return }`）。
- 新增 `panelRow`（D2 的属性与 `panelRowHeight = 40`）。
- `Left` 列首加 `if s.panelDown { s.panelRow(c) }`；`View` 侧栏列把 `s.leftHeader(c)` 改为 `if s.panelDown { s.panelRow(c) } else { s.leftHeader(c) }`。

- [ ] **步骤 4：运行测试验证通过**

运行：`go test ./internal/view/ -v`（全包，既有 `TestShellSlotsBuild` / 标签 / 树测试不得红）
预期：PASS。

- [ ] **步骤 5：加截图用例并自己看图**

`screenshots_test.go` 的 `shotCases()` 在 `09-multi-project` 后加：

```go
{
	name: "10-sunk-panel",
	build: func(t *testing.T) *State {
		s := shotTree(t)
		s.panelDown = true
		return s
	},
},
```

运行：`MDPREVIEW_SHOT_DIR=/tmp/shots go test ./internal/view -run TestScreenshots`
人工核对 PNG：面板行高 40 是否顺眼、与树的节奏、行内图标是否同现在标题栏时一致（折叠按钮不在 View 里，属正常）。不顺眼就调 `panelRowHeight`（±4 DIP 内）重跑本步与步骤 4。

- [ ] **步骤 6：Commit**

```bash
git add internal/view/state.go internal/view/view.go internal/view/panel_test.go internal/view/screenshots_test.go AGENTS.md
git commit -m "feat: macOS 上项目面板下沉为侧栏首行，标题栏槽位留空"
```

（AGENTS.md 本次提交只改结构表 `internal/view` 行的描述与「坑」里新增一条：`panelDown` 同源决策、动画中按钮回左端是 onevra 设计、标签条几何不变的依据。）

### 任务 2：main.go 接线尾随折叠按钮 + 文档收尾

**文件：**
- 修改：`main.go`（shell 选项）
- 修改：`AGENTS.md`（改动指引）、`README.md`（若提及头部/面板结构）、`CHANGELOG.md`

- [ ] **步骤 1：main.go 启用尾随按钮**

`sidebar.ShellOptions` 里加（`LeftMinWidth` 之后）：

```go
// macOS sinks the project panel into the sidebar's body (view), leaving the
// header to the traffic lights and the collapse toggle, which onevra parks
// at the header's trailing edge. Elsewhere the panel stays in the header
// beside the leading toggle, as it always has.
LeftToggleTrailing: state.PanelDown(),
```

- [ ] **步骤 2：验证**

运行：`go build ./... && go vet ./... && gofmt -l . && go test ./...`
预期：全绿、无输出。

- [ ] **步骤 3：文档回写**

- AGENTS.md「改动指引」的「改标签栏/标题行/侧栏」条目：补一句 panelDown 的两条拼装路径（`LeftHeader` 与 `panelRow`）必须同源（`panelItems`）。
- README：`grep -n "标题栏\|头部\|项目面板" README.md`，若有与现状冲突的描述则同步（预计只有结构一段的一句话）。
- CHANGELOG「未发布 → 变更」加一条：`macOS：侧栏顶部的项目面板移入侧栏主体第一行，标题栏只留折叠按钮且移到右端；Windows / Linux 布局不变。`

- [ ] **步骤 4：Commit（含手动验证清单）**

```bash
git add main.go AGENTS.md README.md CHANGELOG.md
git commit -m "feat: macOS 折叠按钮移到标题栏右端（LeftToggleTrailing）

手动验证（macOS 实机）：
- 展开侧栏：折叠按钮在标题栏右端、项目面板在主体第一行、不与交通灯重叠
- 折叠侧栏与展开动画中：按钮回标题栏左端（onevra 设计），动画结束跳回右端
- 标签条起点与 Windows 一致
Windows/Linux：布局与现状一致（测试钉住）。"
```

- [ ] **步骤 5：确认 darwin 路径可编译**

运行：`GOOS=darwin go build ./...`（纯编译检查，不打包；三平台安装包由 CI 的 `go tool mygo build` 负责）
预期：编译无错误。
