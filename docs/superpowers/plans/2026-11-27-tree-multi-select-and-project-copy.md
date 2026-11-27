# 项目节点的复制菜单与侧栏多选（Ctrl+C 复制多个路径）实现计划

> **面向 AI 代理的工作者：** 必需子技能：使用 subagent-driven-development（推荐）或 executing-plans 逐任务实现此计划。
> 步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** 侧栏的「复制」这套动作补齐到**项目节点**，并给树加上**多选**：

1. 树里的**项目节点**右键补上：**复制文件夹**（单文件项目上是「复制文件」）、**复制文件名称**、**复制文件路径**、
   **在文件管理器中打开**——现在这个菜单只有展开/折叠两条。
2. **项目面板**（侧栏顶部那个下拉）的每一行同样补齐这几条：只开一个项目时树里**没有项目节点**，
   这个面板是那种情况下唯一能对「项目这个文件夹」做动作的地方。
3. 树里 **按住 Ctrl（macOS 上 Cmd 或 Ctrl 都行）+ 点**可以**多选**文件、目录、项目节点；普通点仍是单选，
   并且保留它原来的动作（点文档打开、点目录开合）。
4. **`Cmd/Ctrl+C`**：有选中的行就复制**这些行的绝对路径**（一行一个，可以直接贴给 agent）；
   没有选中的行时仍是老行为（复制当前文档的路径）。
5. 右键菜单**作用于整个选中集合**：右键的行在选中集合里时，「复制文件路径 / 名称 / 文件」覆盖所有选中项。

## 为什么现在拿不到（现状诊断）

- **项目节点没有那几条菜单**：`view.go` 的 `projectMenu`（第 379 行起）只有「展开/折叠此项目的全部目录」两条。
  文档/目录行的 `nodeMenu` 才有复制三兄弟。项目面板那一行（`projectSwitcher` 里的 `components.ContextMenu`）
  有「复制项目路径」，但没有「复制文件夹 / 复制文件名称」。
- **`Cmd/Ctrl+C` 与树无关**：`fileacts.go` 的 `handleShortcuts` 直接把 `Cmd+C` 交给 `copyDocPath()`，
  而它只认 `s.sel`（**当前显示的文档**）。树里点过哪个目录、右键过哪个项目，它一律不看；
  没有文档时（`s.sel` 为空）静默返回，所以「点一个文件夹，再 Ctrl+C」什么也不会发生。
- **只在焦点落进正文（WebView 渲染时）才与页面抢键**：本项目除正文的 WebView 渲染外**全部是原生 UI**——
  侧栏树、标签栏、标题行、项目面板都是 `ui` 画的，焦点在它们身上时 `Cmd/Ctrl+C` 走原生快捷键，
  与渲染模式无关（原生渲染下更不用说）。只有焦点在**正文页面内部**时，`Cmd+C` 才是页面自己的（复制选中文字），
  这一点**不是缺陷**，不写进「已知限制」；把「先点树选中行、再 `Cmd+C`」写清楚就够了。

## 架构

- **选择是一份内存态**：`State.treeSel []treeKey`，键是 `treeKey{project, path}`（`path == ""` 是项目节点）。
  用 `treeKey` 而不是绝对路径：重扫之后键仍稳定，剪枝也只需查 `treeNode(k)`。
- **点击路由只在一处**：`treesel.go` 的 `selectRow(k, mods)`。普通点 → 选择只剩这一行（然后照旧执行行自己的动作）；
  带修饰键 → **只**加/减这一行，**不**执行行自己的动作（不打开文档、不折叠目录、不切项目）。
  `treeRow` 的三种行（项目 / 目录 / 文档）、以及 `treeMark` 的箭头都走这一个入口。
- **选择跟着「用户刚点开的那份文档」走**：`openTab` 里把选择同步成该文档那一行（含「点的是已经显示着的标签」的早退分支）。
  这样「点目录 → 点标签 → Ctrl+C」不会复制到那个目录的路径。
- **复制只经 `System`**：文本框走 `CopyText`、文件/文件夹走 `CopyFiles`，与既有 `copyNode*` 同一套。
  多选时 `CopyText` 收到的是 `strings.Join(paths, "\n")`（不加引号、不去重以外的加工）。
- **菜单复用同一组动作**：`copyRows` / `copyRowNames` / `copyRowPaths` / `revealRow` 都吃 `[]treeKey`，
  单行是长度 1 的切片。菜单用 `menuTargets(k)` 决定吃哪些行。

## 技术栈

Go 1.27+、`github.com/egoist/mygo v0.10.0`（`Element.ClickModifiers()` 在**点击那一刻**的修饰键、
`ui.Modifiers` 的 `Ctrl` / `Super`、`ui.Cmd` 是平台快捷键修饰键（darwin = Super，其余 = Ctrl）、
`ui.Tester.ClickWith(mods, label)`、`ui.Tester.Menu()`）、
`github.com/oneo-me/onevra/source v0.2026.19`（主题色 `Theme.Selection` 已存在 = 主色 24% 洗淡，无需新颜色）。
**不引入新依赖。**

**规格：** `docs/superpowers/specs/2026-11-21-markdown-preview-design.md`。本计划不改既有目标，只**新增**一套
右键项与多选；执行完成后在规格里追加一节「侧栏多选与项目节点的复制（2026-11-27）」，并在本计划末尾追加「执行后的修订」。

## 本期不做

- **Shift 连选**（区间选择）、`Cmd/Ctrl+A` 全选、拖拽多选、把选中项拖进别的目录；
- 复制/剪切/粘贴**到**树里（改文件系统的动作），以及删除、重命名；
- 选择**落盘**（不写 `session.json`：选择是一次会话语境，重启后为空）；
- 目录（TOC）那一棵 `ui.Tree` 的多选；
- 项目节点的「移除项目…」（面板那一行已有，树里不加）；
- 多选时的排序规则（就按点选先后，不做「按树序重排」）。

## 全局约束

- **不新增依赖**；不引入第三方 markdown / 高亮 / web 框架；不改 `internal/workspace`、`internal/mcp`。
- **颜色只从 `AppTheme` 取**：选中行用主题已有的 `t.Selection`，当前文档仍是 `t.Accent`；
  **不要**新写十六进制、也不要新加 `theme.go` 的函数。
- **视图状态归窗口线程**：选择只在事件回调（`OnClick` / 菜单 / 快捷键）里改，构建期只读。
- **`View` 与 shell 的槽位两条拼装路径一起改**：行在 `treeBody` 里，两条路径共用；改完 `TestShellSlotsBuild` 必须绿。
- **菜单标签是测试与读屏唯一的定位方式**：新增标签 `复制文件名称` / `复制文件路径` / `复制文件夹` 已存在
  （`nodeMenu`），项目节点的同名项与之**不同时出现**，可以重名；**不要**造 `复制项目文件夹` 这类新变体。
- 注释一律用**英文**、写「为什么」；用户可见文案是中文（既有风格）。
- 测试只用 `t.TempDir()`；不写平台专用代码（不加 `//go:build`），三平台可编译。
  修饰键判断用 `ui.Ctrl|ui.Cmd` 做掩码，**不要**写 `runtime.GOOS == "darwin"`。
- **测试点树上的项目节点用「非活动项目」的名字**（`twoProjects` 里是 `beta`）：面板按钮显示的是**活动项目**
  的名字（`alpha`），`Tester` 按文本找元素，同名的两个标签里它会先碰到按钮，于是点到「切换项目」而不是树行。
  按名字取行用 `treeKeyOfProject`（见任务 2 的测试块）。
- 每个行为改动**先有失败的测试**；文档回写（README / AGENTS / CHANGELOG / 规格 / 本计划）**并入同一批提交**。
- 提交信息中文，前缀 `feat:` / `fix:` / `test:` / `docs:`；提交前 `go vet ./... && gofmt -l . && go test ./...` 干净。

## 设计决策（执行者照此实现；用户可推翻）

- **D1 选择键是 `treeKey`**：`{project, path}`。不用绝对路径（重扫、改名、Windows 短名都会漂），
  不用 `docSel`（它只能指文档，目录与项目节点指不了）。
- **D2 选择是保序切片**：`[]treeKey` 按**点选先后**记，复制出来的路径就是这个顺序；再次点同一行是**取消选择**（不换位置）。
  排序不重排——用户点出来的顺序就是他要粘贴的顺序。
- **D3 普通点 = 单选，并保留原动作**：点目录→开合、点文档→打开、点项目节点→开合，同时选择变成那一行。
  **修饰键点 = 只改选择**，行自己的动作一概不跑（否则按住 Ctrl 选目录会把它全部折叠，选文档会开标签）。
- **D4 修饰键 = `ui.Ctrl | ui.Cmd`**：Windows/Linux 上就是 Ctrl；macOS 上是 Ctrl 或 Cmd（`ui.Cmd` = Super）。
  Windows 的 Win 键 + 点也落进这个掩码——无害（它没有别的行为），不值得为它分叉。
- **D5 选中的行与「当前文档」是两件事**：行底色 `t.Selection`（24% 主色）；当前文档仍是整行 `t.Accent`，
  两个都成立时**以当前文档为准**（保持既有的「当前文档最显眼」）。搜索打开时当前候选的底色优先级最高（既有行为不变）。
- **D6 `Cmd/Ctrl+C` 的先后**：有选中行 → 复制它们的绝对路径；没有 → `copyDocPath()`（老行为，既有测试不动）。
- **D7 一条路径也走「选中」这条路**：单选一个目录时 `Ctrl+C` 复制的是**目录**路径，不是当前文档——
  这正是用户报的那个缺口。
- **D8 文案**：1 项沿用 `已复制文件路径：<abs>`；多项是 `已复制 N 个路径`（不把 N 行路径塞进 Toast）。
  `复制文件夹` 多项是 `已复制 N 个文件`（`CopyFiles` 收的是文件/文件夹列表）。失败沿用 `无法复制文件路径：…` / `无法复制：…`。
- **D9 名字取磁盘上的名字，不取显示名**：项目可以改名（`AddProject(root, name)`），但「文件名称」应当是
  `filepath.Base(p.Root)`（目录项目）或 `filepath.Base(p.File)`（单文件项目）。文档/目录行沿用 `n.Name`。
- **D10 选择不落盘**；`New` 之后为空（`restoreSession` 不走 `openTab`，所以不会凭空选中活动标签那一行）。
- **D11 剪枝时机**：`afterTreesChanged`（掉已不存在的键：`treeNode(k)` 为 false）、`dropTree`（关掉的项目一行不留）、
  `dropTabsOf`（移除项目）、`closeTab`（被关掉的那份文档的行也去掉）。**折叠目录不清选择**：行被折起来看不见，
  但 `Ctrl+C` 仍复制它——与「菜单作用于选中集合」一致。
- **D12 菜单的顺序统一成**：复制三兄弟 → 分隔 → 打开/其他 → 分隔 → 展开/折叠（项目节点）。
  项目节点的展开/折叠**从第 1、2 位移到末尾**——标签不变，既有测试不受影响。
- **D13 菜单目标**：`menuTargets(k)`：`k` 在选中集合里且集合大小 > 1 → 整个集合（保序）；
  否则只有 `[k]`（右键一个没选中的行，只对那一行生效，与文件管理器一致）。

## 文件结构

- 创建 `internal/view/treesel.go` —— 选择模型（`selectRow` / `isRowSelected` / `clearTreeSel` / `syncTreeSelToDoc` /
  `pruneTreeSel` / `dropTreeSelOf` / `menuTargets`）与复制动作（`copyRows` / `copyRowNames` / `copyRowPaths` /
  `revealRow` / `copyTreeSelection` / `rowName` / `absOfKey` / `isFileKey`）。
- 创建 `internal/view/treesel_test.go` —— 选择与复制的新测试（任务 1、2、6）。
- 修改 `internal/view/state.go` —— `State` 加 `treeSel []treeKey`。
- 修改 `internal/view/tabs.go` —— `openTab` 同步选择（含早退分支）；`closeTab` / `dropTabsOf` 剪枝。
- 修改 `internal/view/scan.go` —— `afterTreesChanged` 里 `pruneTreeSel()`；`dropTree` 里 `dropTreeSelOf(id)`。
- 修改 `internal/view/tree.go` —— `treeRow` 的点击路由走 `selectRow`、选中底色；`treeMark` 的箭头也走 `selectRow`。
- 修改 `internal/view/view.go` —— `projectMenu` 补齐复制四兄弟；项目面板行菜单补两条；`nodeMenu` 改成吃 `menuTargets`。
- 修改 `internal/view/fileacts.go` —— `handleShortcuts` 的 `Cmd+C` 先试选择（`copyTreeSelection`）。
- 修改 `internal/view/screenshots_test.go` —— 加一条「树里选中三行」的截图用例。
- 修改 `README.md` / `AGENTS.md` / `CHANGELOG.md` / 规格 —— 文档回写（并入最后一个任务的提交）。

依赖方向不变：`view → workspace`、`view → system`。

### 接口（任务间共用，签名照抄）

```go
// internal/view/treesel.go

// selectRow is what a click on a tree row does to the selection. A plain click
// makes the row the only one selected, and the row's own action runs after it.
// A click with the multi-select modifier (Ctrl everywhere, Command too on
// macOS) adds the row or takes it out, and does nothing else: folding a
// directory or opening a document while the user is picking several rows would
// undo the pick.
func (s *State) selectRow(k treeKey, mods ui.Modifiers)

// isRowSelected reports whether the row is one of the picked ones.
func (s *State) isRowSelected(k treeKey) bool

// clearTreeSel forgets every picked row.
func (s *State) clearTreeSel()

// syncTreeSelToDoc makes the document's row the selection: opening a document
// is picking it, so that a later Cmd+C copies what the user just opened.
func (s *State) syncTreeSelToDoc(sel docSel)

// pruneTreeSel drops the rows the tree no longer has.
func (s *State) pruneTreeSel()

// dropTreeSelOf drops the rows of a project the window closed or removed.
func (s *State) dropTreeSelOf(projectID string)

// menuTargets are the rows a context menu acts on: every picked row when the
// row it opened on is one of them, else the row alone.
func (s *State) menuTargets(k treeKey) []treeKey

// copyRows puts entries themselves on the clipboard, as the file manager does.
func (s *State) copyRows(keys []treeKey)

// copyRowNames puts the entries' names on the clipboard, one per line.
func (s *State) copyRowNames(keys []treeKey)

// copyRowPaths puts the entries' absolute paths on the clipboard, one per line.
func (s *State) copyRowPaths(keys []treeKey)

// copyTreeSelection copies the picked rows' paths, and reports whether there
// was any row to copy: Cmd+C falls back to the document when there is none.
func (s *State) copyTreeSelection() bool

// revealRow shows a row in the file manager.
func (s *State) revealRow(k treeKey)

// absOfKey is a row's absolute path: the project's directory for a project
// node, the file itself for a single-file project's node.
func (s *State) absOfKey(k treeKey) (string, error)

// rowName is the name on disk of what a row shows.
func (s *State) rowName(k treeKey) (string, bool)

// isFileKey reports whether a row is a file rather than a directory: a project
// node of a single file is the file.
func (s *State) isFileKey(k treeKey) bool
```

```go
// internal/view/state.go
type State struct {
    …
    // treeSel is the rows of the sidebar picked with the pointer, in the order
    // they were picked: Cmd+C copies their paths, and a context menu opened on
    // one of them acts on all of them. Empty when the user has picked none.
    treeSel []treeKey
}
```

---

### 任务 1：选择模型（状态、路由、剪枝）

**文件：**
- 创建：`internal/view/treesel.go`、`internal/view/treesel_test.go`
- 修改：`internal/view/state.go`、`internal/view/tabs.go`、`internal/view/scan.go`

- [ ] **步骤 1：编写失败的测试**

```go
// treesel_test.go
func TestAPlainClickMakesTheRowTheOnlyOneSelected(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "docs/a.md", "# A\n")
	writeDoc(t, root, "docs/b.md", "# B\n")
	s.Refresh()

	tt := newTester(s)
	if err := tt.Click("a.md"); err != nil { t.Fatalf("Click: %v", err) }
	if err := tt.Click("docs"); err != nil { t.Fatalf("Click: %v", err) }
	if want := []treeKey{{project: s.trees[0].project.ID, path: "docs"}}; !slices.Equal(s.treeSel, want) {
		t.Errorf("treeSel = %v, want %v", s.treeSel, want)
	}
}

func TestCtrlClickPicksSeveralRowsAndDoesNotOpenThem(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "docs/a.md", "# A\n")
	writeDoc(t, root, "docs/b.md", "# B\n")
	s.Refresh()
	s.selectDoc("docs/a.md")

	tt := newTester(s)
	if err := tt.ClickWith(ui.Ctrl, "b.md"); err != nil { t.Fatalf("ClickWith: %v", err) }
	if s.sel.path != "docs/a.md" {
		t.Errorf("sel = %+v, want the document shown unchanged", s.sel)
	}
	if len(s.treeSel) != 1 || s.treeSel[0].path != "docs/b.md" {
		t.Errorf("treeSel = %v, want b.md picked", s.treeSel)
	}
	// A second click with the modifier takes it out again.
	if err := tt.ClickWith(ui.Ctrl, "b.md"); err != nil { t.Fatalf("ClickWith: %v", err) }
	if len(s.treeSel) != 0 {
		t.Errorf("treeSel = %v, want nothing picked", s.treeSel)
	}
}

func TestCtrlClickDoesNotFoldADirectory(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "docs/a.md", "# A\n")
	s.Refresh()
	key := treeKey{project: s.trees[0].project.ID, path: "docs"}
	if !s.treeOpen.Open.Has(key) {
		t.Fatal("the directory should start open")
	}
	tt := newTester(s)
	if err := tt.ClickWith(ui.Ctrl, "docs"); err != nil { t.Fatalf("ClickWith: %v", err) }
	if !s.treeOpen.Open.Has(key) {
		t.Error("a Ctrl-click folded the directory; it should only pick it")
	}
}

func TestOpeningADocumentMovesTheSelection(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "docs/a.md", "# A\n")
	writeDoc(t, root, "docs/b.md", "# B\n")
	s.Refresh()

	tt := newTester(s)
	if err := tt.Click("docs"); err != nil { t.Fatalf("Click: %v", err) }   // pick the directory
	if err := tt.Click("b.md"); err != nil { t.Fatalf("Click: %v", err) }   // open a document
	if want := []treeKey{{project: s.trees[0].project.ID, path: "docs/b.md"}}; !slices.Equal(s.treeSel, want) {
		t.Errorf("treeSel = %v, want the document just opened", s.treeSel)
	}
}

func TestPickedRowsGoWhenTheirProjectGoes(t *testing.T) {
	s, alphaRoot, _ := twoProjects(t)
	writeDoc(t, alphaRoot, "docs/a.md", "# A\n")
	s.Refresh()
	s.selectRow(treeKey{project: s.trees[0].project.ID, path: "docs"}, 0)
	if len(s.treeSel) != 1 { t.Fatalf("treeSel = %v, want one row", s.treeSel) }
	s.dropTreeSelOf(s.trees[0].project.ID)
	if len(s.treeSel) != 0 { t.Errorf("treeSel = %v, want nothing picked", s.treeSel) }
}

func TestPickedRowsGoWhenTheFileGoes(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "docs/a.md", "# A\n")
	s.Refresh()
	s.selectRow(treeKey{project: s.trees[0].project.ID, path: "docs/a.md"}, 0)
	if err := os.Remove(filepath.Join(root, "docs", "a.md")); err != nil { t.Fatal(err) }
	s.Refresh()
	if len(s.treeSel) != 0 { t.Errorf("treeSel = %v, want the gone row dropped", s.treeSel) }
}
```

- [ ] **步骤 2：运行测试验证失败**

```sh
go test ./internal/view -run 'TestAPlainClick|TestCtrlClick|TestOpeningADocumentMoves|TestPickedRowsGo' -v
```
期望：编译错误（`s.treeSel`、`selectRow`、`dropTreeSelOf` 未定义）。

- [ ] **步骤 3：实现**

`state.go`：加字段（见上面的接口块）。

`treesel.go`：

```go
// multiClick is the modifier that picks rows instead of acting on them:
// Control everywhere, and Command too on macOS, where ui.Cmd is Super. It is a
// var, not a const: ui.Cmd is decided at init, from the platform.
var multiClick = ui.Ctrl | ui.Cmd

func (s *State) selectRow(k treeKey, mods ui.Modifiers) {
	if mods&multiClick != 0 {
		for i, picked := range s.treeSel {
			if picked == k {
				s.treeSel = append(s.treeSel[:i], s.treeSel[i+1:]...)
				return
			}
		}
		s.treeSel = append(s.treeSel, k)
		return
	}
	s.treeSel = []treeKey{k}
}
```

`index` 线性找就够了：选择通常几个到几十个。

`tabs.go` 的 `openTab` 开头（在 `sel.empty()` 之后、`sel == s.sel` 早退之前）：

```go
s.syncTreeSelToDoc(sel)
```
`syncTreeSelToDoc`：`s.treeSel = []treeKey{{project: sel.project, path: sel.path}}`（`sel.empty()` 时清空）。

`closeTab` 里，`sel` 被关掉时把它的行从选择里去掉；`dropTabsOf` 里 `s.dropTreeSelOf(projectID)`。

`scan.go`：`afterTreesChanged` 开头 `s.pruneTreeSel()`；`dropTree` 尾部 `s.dropTreeSelOf(id)`。

- [ ] **步骤 4：运行测试验证通过**

```sh
go test ./internal/view -run 'TestAPlainClick|TestCtrlClick|TestOpeningADocumentMoves|TestPickedRowsGo' -v
```

- [ ] **步骤 5：提交**

```sh
git add internal/view/treesel.go internal/view/treesel_test.go internal/view/state.go internal/view/tabs.go internal/view/scan.go
git commit -m "feat: 侧栏树的行选择（普通点单选、Ctrl 点多选）"
```

---

### 任务 2：行的点击路由与选中底色

**文件：** 修改 `internal/view/tree.go`；测试加在 `internal/view/treesel_test.go`

- [ ] **步骤 1：编写失败的测试**

```go
func TestCtrlClickPicksAProjectNodeToo(t *testing.T) {
	s, _, betaRoot := twoProjects(t)
	writeDoc(t, betaRoot, "docs/a.md", "# A\n")
	s.Refresh()
	key := treeKeyOfProject(s, "beta")
	tt := newTester(s)
	if err := tt.ClickWith(ui.Ctrl, "beta"); err != nil { t.Fatalf("ClickWith: %v", err) }
	if want := []treeKey{key}; !slices.Equal(s.treeSel, want) {
		t.Errorf("treeSel = %v, want the project node picked", s.treeSel)
	}
}

// treeKeyOfProject is a project's own row, by the name it shows.
func treeKeyOfProject(s *State, name string) treeKey {
	for _, tr := range s.trees {
		if tr.project.Name == name {
			return treeKey{project: tr.project.ID}
		}
	}
	return treeKey{}
}

// The tests click "beta", never "alpha": the panel button shows the *active*
// project's name, so "alpha" names two labels and Tester would hit the button
// first. "beta" names one label in the window: the tree's row.
func TestAPlainClickOnAProjectNodeStillFoldsIt(t *testing.T) {
	s, _, _ := twoProjects(t)
	s.Refresh()
	key := treeKeyOfProject(s, "beta")
	if !s.treeOpen.Open.Has(key) { t.Fatal("the project node should start open") }
	tt := newTester(s)
	if err := tt.Click("beta"); err != nil { t.Fatalf("Click: %v", err) }
	if s.treeOpen.Open.Has(key) { t.Error("a plain click should have folded the project node") }
}
```

- [ ] **步骤 2：运行测试验证失败**

```sh
go test ./internal/view -run 'TestCtrlClickPicksAProjectNode|TestAPlainClickOnAProjectNode' -v
```

- [ ] **步骤 3：实现**

`treeRow`：在 `row.Children(...)` 之后、按行种类分派之前，读出这次点击的修饰键，并**在行的三个点击入口里统一调用**：

```go
mods := row.ClickModifiers()
picked := mods&multiClick != 0
```
- 项目 / 目录：`row.OnClick(func() { s.selectRow(k, mods); if !picked { s.toggleTreeKey(k) } })`
- 文档：`row.OnClick(func() { s.selectRow(k, mods); if picked { return }; if i := s.candidateIndex(...); i >= 0 { s.gotoCandidate(i) }; s.selectDocIn(k.project, k.path) })`
- `treeMark` 的箭头盒子：`box.OnClick(func() { s.selectRow(k, mods); if !picked { s.toggleTreeKey(k) } })`
  （`ClickModifiers()` 在同一个构建周期里读到的就是这次点击的键；箭头本身也要能参与选择，否则 Ctrl 点箭头会意外折叠）。

底色（`switch` 里，候选 > 当前文档 > 选中 > 悬停）：

```go
case s.isRowSelected(k):
	row.Background(t.Selection)
```

- [ ] **步骤 4：运行测试验证通过**

```sh
go test ./internal/view -run 'TestCtrlClick|TestAPlainClick|TestTree|TestTOC' -v
```

- [ ] **步骤 5：提交**

```sh
git add internal/view/tree.go internal/view/treesel_test.go
git commit -m "feat: 树的行按住 Ctrl 多选，选中行上底色"
```

---

### 任务 3：`Cmd/Ctrl+C` 复制选中的行

**文件：** 修改 `internal/view/treesel.go`、`internal/view/fileacts.go`

- [ ] **步骤 1：编写失败的测试**

```go
func TestCmdCCopiesEveryPickedPath(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "docs/a.md", "# A\n")
	writeDoc(t, root, "b.md", "# B\n")
	s.Refresh()
	sys := &fakeSystem{}
	s.SetSystem(sys)
	s.selectRow(treeKey{project: s.trees[0].project.ID, path: "docs/a.md"}, ui.Ctrl)
	s.selectRow(treeKey{project: s.trees[0].project.ID, path: "b.md"}, ui.Ctrl)

	tt := newTester(s)
	tt.Key(ui.Cmd, ui.KeyC)

	want := filepath.Join(root, "docs", "a.md") + "\n" + filepath.Join(root, "b.md")
	if sys.copied != want { t.Errorf("copied = %q, want %q", sys.copied, want) }
	if !strings.Contains(s.Status(), "已复制 2 个路径") {
		t.Errorf("status = %q, want the count reported", s.Status())
	}
}

func TestCmdCCopiesADirectoryTheUserPicked(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "docs/a.md", "# A\n")
	s.Refresh()
	s.selectDoc("docs/a.md")
	sys := &fakeSystem{}
	s.SetSystem(sys)

	tt := newTester(s)
	if err := tt.Click("docs"); err != nil { t.Fatalf("Click: %v", err) }
	tt.Key(ui.Cmd, ui.KeyC)
	if want := filepath.Join(root, "docs"); sys.copied != want {
		t.Errorf("copied = %q, want the directory picked, not the document shown", sys.copied)
	}
}

func TestCmdCCopiesAProjectNodesPath(t *testing.T) {
	s, _, betaRoot := twoProjects(t)
	s.Refresh()
	sys := &fakeSystem{}
	s.SetSystem(sys)
	tt := newTester(s)
	if err := tt.Click("beta"); err != nil { t.Fatalf("Click: %v", err) }
	tt.Key(ui.Cmd, ui.KeyC)
	if sys.copied != betaRoot { t.Errorf("copied = %q, want %q", sys.copied, betaRoot) }
}
```

（`TestCmdCCopiesTheDocumentPath` 与 `TestCmdCCopiesNothingWithoutADocument` 必须**仍然通过**——没有选中行时行为不变。）

- [ ] **步骤 2：运行测试验证失败**

```sh
go test ./internal/view -run 'TestCmdCCopies' -v
```

- [ ] **步骤 3：实现**

`treesel.go` 的 `copyTreeSelection`：

```go
func (s *State) copyTreeSelection() bool {
	if s.sys == nil || len(s.treeSel) == 0 {
		return false
	}
	paths := make([]string, 0, len(s.treeSel))
	for _, k := range s.treeSel {
		abs, err := s.absOfKey(k)
		if err != nil {
			s.notifyErr("无法复制文件路径：" + err.Error())
			return true
		}
		paths = append(paths, abs)
	}
	s.sys.CopyText(strings.Join(paths, "\n"))
	if len(paths) == 1 {
		s.notifyDone("已复制文件路径：" + paths[0])
		return true
	}
	s.notifyDone(fmt.Sprintf("已复制 %d 个路径", len(paths)))
	return true
}
```

`fileacts.go` 的 `handleShortcuts`：

```go
case c.Shortcut(ui.Cmd, ui.KeyC):
	if !s.copyTreeSelection() {
		s.copyDocPath()
	}
```

`absOfKey`：`k.path == ""` 时是项目根（`p.Root`），单文件项目的根节点是文件本身
（`tr.project.File != ""`，`p.File`）；其余走 `s.ws.AbsOf(k.project, k.path)`。

- [ ] **步骤 4：运行测试验证通过**

```sh
go test ./internal/view -run 'TestCmdC' -v
```

- [ ] **步骤 5：提交**

```sh
git add internal/view/treesel.go internal/view/fileacts.go internal/view/treesel_test.go
git commit -m "feat: Ctrl+C 复制选中的行（可多行路径）"
```

---

### 任务 4：项目节点与项目面板的复制菜单

**文件：** 修改 `internal/view/view.go`、`internal/view/treesel.go`、`internal/view/fileacts.go`

- [ ] **步骤 1：编写失败的测试**

```go
func TestProjectNodeMenuCopiesTheFolderNameAndPath(t *testing.T) {
	s, _, betaRoot := twoProjects(t)
	writeDoc(t, betaRoot, "docs/a.md", "# A\n")
	s.Refresh()
	sys := &fakeSystem{}
	s.SetSystem(sys)
	tt := newTester(s)

	if err := tt.RightClick("beta"); err != nil { t.Fatalf("RightClick: %v", err) }
	for _, want := range []string{"复制文件夹", "复制文件名称", "复制文件路径", "在文件管理器中打开",
		"展开此项目的全部目录", "折叠此项目的全部目录"} {
		if !slices.Contains(tt.Menu(), want) {
			t.Fatalf("menu = %q, want %q in it", tt.Menu(), want)
		}
	}
	if err := tt.Click("复制文件夹"); err != nil { t.Fatalf("Click: %v", err) }
	if want := []string{betaRoot}; !slices.Equal(sys.files, want) {
		t.Errorf("files = %v, want %v", sys.files, want)
	}
	if err := tt.RightClick("beta"); err != nil { t.Fatalf("RightClick: %v", err) }
	if err := tt.Click("复制文件名称"); err != nil { t.Fatalf("Click: %v", err) }
	if want := filepath.Base(betaRoot); sys.copied != want {
		t.Errorf("copied = %q, want the folder's own name %q", sys.copied, want)
	}
}

func TestTheProjectPanelCopiesTheFolderOfALoneProject(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "docs/a.md", "# A\n")
	s.Refresh()
	sys := &fakeSystem{}
	s.SetSystem(sys)
	tt := newTester(s)
	openPanel(t, tt)
	if err := tt.RightClick("项目：alpha"); err != nil { t.Fatalf("RightClick: %v", err) }
	if err := tt.Click("复制文件夹"); err != nil { t.Fatalf("Click: %v", err) }
	if want := []string{root}; !slices.Equal(sys.files, want) {
		t.Errorf("files = %v, want %v", sys.files, want)
	}
}
```

- [ ] **步骤 2：运行测试验证失败**

```sh
go test ./internal/view -run 'TestProjectNodeMenu|TestTheProjectPanelCopies' -v
```

- [ ] **步骤 3：实现**

`projectMenu`（`view.go`）变成：

```go
components.ContextMenu(c, item, func(m *components.Menu) {
	keys := s.menuTargets(treeKey{project: tr.project.ID})
	copyFile := "复制文件夹"
	if s.isFileKey(treeKey{project: tr.project.ID}) {
		copyFile = "复制文件"
	}
	if m.Item(copyFile).Chosen() { s.copyRows(keys) }
	m.Separator()
	if m.Item("复制文件名称").Chosen() { s.copyRowNames(keys) }
	if m.Item("复制文件路径").Chosen() { s.copyRowPaths(keys) }
	m.Separator()
	if m.Item("在文件管理器中打开").Chosen() { s.revealRow(keys[0]) }
	m.Separator()
	if m.Item("展开此项目的全部目录").Chosen() { s.setTreeOpen(tr.project.ID, true) }
	if m.Item("折叠此项目的全部目录").Chosen() { s.setTreeOpen(tr.project.ID, false) }
})
```

项目面板那一行（`projectSwitcher` 的 `ContextMenu`）在「复制项目路径」前面加同样的两条，
用 `treeKey{project: p.ID}` 走同一组动作（这样单文件项目在面板上也是「复制文件」）。
「复制项目路径」保留原样（`copyProjectPath`），它和「复制文件路径」对项目而言取的是同一个目录。

`nodeMenu` 也改成吃 `menuTargets`（任务 5 的测试钉住）。

- [ ] **步骤 4：运行测试验证通过**

```sh
go test ./internal/view -run 'TestProjectMenu|TestProjectNodeMenu|TestTheProjectPanel|TestTreeMenu' -v
```

- [ ] **步骤 5：提交**

```sh
git add internal/view/view.go internal/view/treesel.go internal/view/treesel_test.go
git commit -m "feat: 项目节点与项目面板补齐复制文件夹/名称/路径"
```

---

### 任务 5：菜单作用于整个选中集合

**文件：** 修改 `internal/view/view.go`（`nodeMenu`）、`internal/view/treesel.go`

- [ ] **步骤 1：编写失败的测试**

```go
func TestTheTreeMenuActsOnEveryPickedRow(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "docs/a.md", "# A\n")
	writeDoc(t, root, "docs/b.md", "# B\n")
	s.Refresh()
	sys := &fakeSystem{}
	s.SetSystem(sys)
	id := s.trees[0].project.ID
	s.selectRow(treeKey{project: id, path: "docs/a.md"}, ui.Ctrl)
	s.selectRow(treeKey{project: id, path: "docs/b.md"}, ui.Ctrl)

	tt := newTester(s)
	if err := tt.RightClick("b.md"); err != nil { t.Fatalf("RightClick: %v", err) }
	if err := tt.Click("复制文件路径"); err != nil { t.Fatalf("Click: %v", err) }
	want := filepath.Join(root, "docs", "a.md") + "\n" + filepath.Join(root, "docs", "b.md")
	if sys.copied != want { t.Errorf("copied = %q, want both rows in the order picked", sys.copied) }
}

func TestTheMenuOfARowOutsideThePickCopiesOnlyThatRow(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "docs/a.md", "# A\n")
	writeDoc(t, root, "docs/b.md", "# B\n")
	s.Refresh()
	sys := &fakeSystem{}
	s.SetSystem(sys)
	s.selectRow(treeKey{project: s.trees[0].project.ID, path: "docs/a.md"}, 0)

	tt := newTester(s)
	if err := tt.RightClick("b.md"); err != nil { t.Fatalf("RightClick: %v", err) }
	if err := tt.Click("复制文件路径"); err != nil { t.Fatalf("Click: %v", err) }
	if want := filepath.Join(root, "docs", "b.md"); sys.copied != want {
		t.Errorf("copied = %q, want only the row the menu opened on", sys.copied)
	}
}
```

- [ ] **步骤 2：运行测试验证失败**

```sh
go test ./internal/view -run 'TestTheTreeMenuActs|TestTheMenuOfARowOutside' -v
```

- [ ] **步骤 3：实现**

`nodeMenu`：

```go
func (s *State) nodeMenu(c *ui.Context, item ui.Element, tr projectTree, n *workspace.Node) {
	k := treeKey{project: tr.project.ID, path: n.Path}
	components.ContextMenu(c, item, func(m *components.Menu) {
		keys := s.menuTargets(k)
		copyFile := "复制文件"
		if n.IsDir { copyFile = "复制文件夹" }
		if m.Item(copyFile).Chosen() { s.copyRows(keys) }
		m.Separator()
		if m.Item("复制文件名称").Chosen() { s.copyRowNames(keys) }
		if m.Item("复制文件路径").Chosen() { s.copyRowPaths(keys) }
		if !n.IsDir { return }
		m.Separator()
		if m.Item("在文件管理器中打开").Chosen() { s.revealRow(k) }
	})
}
```
（菜单标签仍按**右键的那一行**给：右键目录说「复制文件夹」，即使集合里混着文件——`copyRows` 收的是路径列表，
`CopyFiles` 两者都收。文案上不区分，别为它造两套菜单。）

`menuTargets` / `copyRows` / `copyRowNames` / `copyRowPaths` 见接口块；多选文案按 D8。
`copyRows` 失败沿用 `无法复制：…`（既有 `copyNodeFile` 的文案）。

- [ ] **步骤 4：运行测试验证通过**

```sh
go test ./internal/view -run 'TestTreeMenu|TestTheTreeMenu|TestTheMenuOfARow' -v
```

- [ ] **步骤 5：提交**

```sh
git add internal/view/view.go internal/view/treesel.go internal/view/treesel_test.go
git commit -m "feat: 树的右键复制作用于整个选中集合"
```

---

### 任务 6：截图用例（人工核对底色与顺序）

**文件：** 修改 `internal/view/screenshots_test.go`；图看 `MDPREVIEW_SHOT_DIR`

- [ ] **步骤 1：加一条用例**

在 `shotCases()` 里加：

```go
{
	name: "tree-multi-select",
	build: func(t *testing.T) *State {
		s, root := newState(t)
		writeDoc(t, root, "docs/a.md", "# A\n")
		writeDoc(t, root, "docs/b.md", "# B\n")
		writeDoc(t, root, "docs/c.md", "# C\n")
		s.Refresh()
		id := s.trees[0].project.ID
		s.selectRow(treeKey{project: id, path: "docs"}, ui.Ctrl)
		s.selectRow(treeKey{project: id, path: "docs/a.md"}, ui.Ctrl)
		s.selectRow(treeKey{project: id, path: "docs/c.md"}, ui.Ctrl)
		return s
	},
	settle: 3,
},
```

- [ ] **步骤 2：看图**

```sh
MDPREVIEW_SHOT_DIR=/tmp/shots go test ./internal/view -run TestScreenshots
```
人工确认：三行底色铺满整行、与「当前文档」的整行主色能区分、与鼠标悬停不同、文字对比度够。

- [ ] **步骤 3：提交**

```sh
git add internal/view/screenshots_test.go
git commit -m "test: 多选状态的截图用例"
```

---

### 任务 7：真窗口手动验证

**不做假装的验证**：这一条要人来做，结果写进提交信息或 PR 的「手动验证」。

- [ ] **步骤 1：** `go run .` → 打开一个目录项目，**按住 Ctrl 点三行**（目录 + 两个文档）：
  三行都有底色，目录**没有**被折叠，文档**没有**被打开。
- [ ] **步骤 2：** 按 `Ctrl+C`，贴进 agent / 终端：三行绝对路径、一行一个、顺序与点选一致。
- [ ] **步骤 3：** 右键项目名 → 看到复制三兄弟 → 「复制文件夹」后去资源管理器/访达**粘贴**，落下一个文件夹。
- [ ] **步骤 4：** 只开一个项目时，项目面板那一行右键 → 「复制文件夹」同样能粘贴。
- [ ] **步骤 5：** 切到 **WebView 渲染**，**先点一下树里的行**（焦点回到原生侧栏），再按 `Ctrl+C`：
  仍要复制选中行的路径。只有焦点在**正文页面里**时 `Ctrl+C` 才是页面自己的（复制选中文字）——
  这是预期行为，不要为此改页面或快捷键；README 里把「先点树，再 `Ctrl+C`」写一句即可。

---

### 任务 8：文档回写与全量验证

**文件：** `README.md`、`AGENTS.md`、`CHANGELOG.md`、
`docs/superpowers/specs/2026-11-21-markdown-preview-design.md`

- [ ] **步骤 1：README**
  - 「用法」里项目面板那一段：右键一行新增 **复制文件夹**（单文件项目是「复制文件」）、**复制文件名称**。
  - 多项目树那一段：项目名右键新增复制这四条；树里**按住 `Ctrl`（macOS 上 `Cmd` 也行）点多行**可以多选，
    选中行上底色；右键菜单对整组生效。
  - 「快捷键」表：`Cmd+C` 改为「复制选中的行（一行一个绝对路径）；没有选中时复制当前文档的路径」，
    并注明「焦点在正文（WebView 渲染的页面）里时，`Cmd+C` 是页面自己的」——本项目除正文之外的界面都是原生 UI，
    选完行直接按就行。
  - 如有任务 7 的结论，写进「已知限制」。- [ ] **步骤 2：AGENTS.md**
  - 结构表 `internal/view` 那一行加 `treesel.go`（选择模型与按行复制）。
  - 「改动指引」加一条：**改多选与按行复制** → `treesel.go` 的 `selectRow` / `menuTargets` / `copyRow*`，
    行的入口在 `tree.go` 的 `treeRow` 与 `treeMark`（两个都调 `selectRow`），剪枝在 `afterTreesChanged` /
    `dropTree` / `dropTabsOf` / `closeTab`；补 `TestCtrlClick*`、`TestCmdCCopies*`、`TestTheTreeMenuActsOn*`。
  - 「已知的坑」加五条：（1）`ClickModifiers()` 是**这次点击**的修饰键，别在读它之前重排行的构建顺序；
    （2）修饰键点击**不能**跑行自己的动作，否则选目录会折叠、选文档会开标签；（3）`openTab` 早退分支之前就要
    同步选择，否则「点目录→点已打开的标签→Ctrl+C」会复制到目录（并说明选择不落盘）；
    （4）测试点项目节点用**非活动项目**的名字（活动项目的名字在面板按钮上也有一份，`Tester` 会先点到按钮）；
    （5）本项目除正文的 WebView 渲染外**全是原生 UI**：`Cmd+C` 到不到原生快捷键看的是**焦点**（在侧栏/标签上就到），
    不是「当前是哪种渲染模式」；焦点在正文页面里时 `Cmd+C` 是页面自己的，不要因此去改页面或快捷键。
- [ ] **步骤 3：CHANGELOG.md**（「未发布」的 **新增**）
  - 「侧栏多选与复制路径」：按住 Ctrl 点选多行（文件/目录/项目），`Ctrl+C` 复制它们的绝对路径；
    项目节点与项目面板新增复制文件夹、复制文件名称、复制文件路径、在文件管理器中打开。
  - 变更：项目节点的展开/折叠移到了菜单末尾（前面让给复制）。
- [ ] **步骤 4：规格与计划回写**
  - 规格追加「侧栏多选与项目节点的复制（2026-11-27）」一节（用户可见行为，不写实现细节）。
  - 本计划末尾追加「执行后的修订」，记下实施中偏离的地方（如增删的菜单项、文案、修饰键范围）。
- [ ] **步骤 5：全量验证**

```sh
go vet ./... && gofmt -l . && go test ./...
MDPREVIEW_SHOT_DIR=/tmp/shots go test ./internal/view -run TestScreenshots
```
期望：vet 无输出、gofmt 无输出、测试全绿。

- [ ] **步骤 6：提交**

```sh
git add README.md AGENTS.md CHANGELOG.md docs/superpowers
git commit -m "docs: 侧栏多选与项目复制菜单（README / AGENTS / CHANGELOG / 规格）"
```

---

## 验收清单

- [ ] 项目节点右键有：复制文件夹（单文件项目为「复制文件」）、复制文件名称、复制文件路径、在文件管理器中打开，
      以及原来的展开/折叠两条。
- [ ] 只开一个项目时，项目面板那一行右键有同样的复制项，「复制项目路径」保持原样。
- [ ] 按住 Ctrl（macOS 上 Cmd 也行）点文件 / 目录 / 项目节点：加选、再点取消；行的原动作**不**发生。
- [ ] 普通点仍是单选，并保留原动作；点标签、从搜索打开文档时选择跟着那份文档走。
- [ ] 选中行有底色，当前文档行仍是整行主色，两者可分辨。
- [ ] `Cmd/Ctrl+C`：有选中 → 复制它们一行一个的绝对路径（顺序 = 点选顺序）；没有 → 复制当前文档路径（老行为）。
- [ ] 右键菜单在右键的行属于选中集合时作用于整组。
- [ ] 文件被删、项目被关/移除时，选择里对应的行自动消失。
- [ ] 选择不写盘（`session.json` 里没有它），重启后为空。
- [ ] 除正文外全是原生 UI：树 / 标签 / 标题行 / 项目面板的焦点上 `Cmd/Ctrl+C` 走原生快捷键，两种渲染模式下都成立；
      只有焦点在正文页面（WebView 渲染）里时 `Cmd+C` 归页面（复制选中文字），这是预期。
- [ ] `go vet ./... && gofmt -l . && go test ./...` 干净；`TestScreenshots` 的 `tree-multi-select` 已人工看过。

## 执行后的修订（2026-11-27，实施时记下，不回改上面的任务）

- **测试归属重排**：计划把四个「点击」类测试放在任务 1，但它们要等任务 2 的行路由才可能通过。
  执行时任务 1 = 选择模型 + 同步/剪枝（测试直接调 `selectRow`），任务 2 = 点击路由与底色（那几个点击测试）。
  测试一条没少。
- **`tt.Menu()` 用不了**：`components.ContextMenu` 画的是窗口内菜单，`ui.Tester.Menu()` 对它返回 nil。
  菜单项的断言改用 `tt.HasText(...)`；点击菜单项照旧 `tt.Click`。
- **面板行菜单的顺序**没按 D12 把复制整块提到最前，而是在「在文件管理器中打开」之后、「复制项目路径」之前插入两条，
  少动已发布的菜单顺序；树里项目节点的顺序按 D12 执行（复制三条 → 打开 → 展开/折叠）。
- **单文件项目的复制项**：树里那种节点走的是 `nodeMenu`（本来就有复制文件），新增代码落在**项目面板**那一行，
  所以计划里的 `TestProjectNodeOfOneFileOffersTheFile` 改成 `TestTheProjectPanelCopiesAProjectOfOneFile`。
- **一条测试改为口径统一的写法**：`TestCtrlClickPicksSeveralRowsAndDoesNotOpenThem` 原先假设选择是空的，
  但「显示中的文档自己也在选择里」是 D 条的口径，于是拆成「不显示文档时 Ctrl 点只选不打开」与
  `TestCtrlClickAddsToTheDocumentShown` 两条。
- **`ClickModifiers()` 在读的时候取**：写在 `OnClick` 回调里（`pickRow`），不要提前到构建期存进变量。
  行与 `treeMark` 的箭头都走 `pickRow`，否则 Ctrl 点箭头会顺手折叠目录。
- **真窗口验证（任务 7）留给人工**：Ctrl+点多选、`Ctrl+C` 贴进 agent、右键「复制文件夹」后到资源管理器粘贴、
  WebView 渲染下先点树再 `Ctrl+C`——这几条没有在无头环境里做。
