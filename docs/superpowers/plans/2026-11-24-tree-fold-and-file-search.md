# 项目树的全部展开/折叠 + 文件名模糊搜索 实现计划

> **面向 AI 代理的工作者：** 必需子技能：使用 subagent-driven-development（推荐）或 executing-plans 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** 侧栏的项目树补上两边：**一键展开 / 折叠所有目录**（侧栏顶部、项目选择按钮左边两个图标），
**每个项目也能单独展开 / 折叠**（项目节点与项目面板的右键菜单）；以及 **`Cmd/Ctrl+F` 按文件名模糊搜索树**
（焦点在侧栏时搜文件、否则搜正文；`Cmd/Ctrl+Shift+F` 从任何位置搜文件），
搜索时**树自己就是候选列表**：只显示命中的文件与它们的父目录，**候选与当前候选两种底色**，
`↑`/`↓` 切换候选并让树定位过去，`Enter`/点击才打开文档。

**架构：** 模糊匹配是 `internal/workspace` 里的纯函数（`FuzzyMatch`，子序列、大小写不敏感、不做正则），可无窗口测试。
视图侧新增 `internal/view/treesearch.go`：`treeSearchState` 保存查询、候选（树序）、候选的父目录集合与当前下标；
候选集合只在**查询变化 / 树重载**时重建（构建期只读内存里的 `s.trees`，不重扫目录）。
`fileTree` 在搜索时分叉成「只画命中的文件 + 祖先目录（构建期强制展开，不写 `s.open`）」，
底色由 `State.treeRowFill` 决定（候选淡、当前候选浓，都取自 `AppTheme`），当前候选用 `ScrollIntoView` 定位。
快捷键在 `handleShortcuts` 里统一分派（`searchShortcuts` 同时管正文查找与树搜索的 `Cmd+F` / `Esc`），
「现在焦点在哪个窗格」由侧栏与正文各自的 `HandleInput` 记在 `State.treeFocused`。

**技术栈：** Go 1.27+、`github.com/egoist/mygo v0.9.1`（`ui.Tree` / `ui.TreeItem`、`Element.Background` /
`ScrollIntoView` / `HandleInput` / `TrackScroll` / `Label` / `Disabled`、`ui.SearchField` 的 `Changed` / `Submitted` /
`AutoFocus`、`ui.ScrollState`、`ui.NewTester` 的 `Click` / `ClickAt` / `Key` / `ChooseMenuItem` / `Find` / `Texts`）、
`github.com/oneo-me/onevra/source v0.2026.18`（`components.IconButton` + `IconButtonPlain`、`components.Text`、
`lucide.UnfoldVertical` / `FoldVertical` / `X` / `ArrowUp` / `ArrowDown`）。**不引入新依赖。**

**规格：** `docs/superpowers/specs/2026-11-21-markdown-preview-design.md`。本计划**新增**两件事（侧栏树的批量折叠、
侧栏内的文件名搜索），不推翻该规格里的任何目标；执行后在该文件末尾追加
「### 项目树的批量折叠与文件名搜索（新增，2026-11-24）」，由任务 7 收尾。

**本期不做：** 目录工具条那三个按钮的任何改动（`setTOC` / `tocActions` 一行不动，树这边是新的一套 `Key`）；
候选按「匹配得分」排序（保持树序，见 D2）；候选**字符级**高亮（`ui.TreeItem` 的标签是纯字符串，要做得自己拼行，
本期只做整行底色，见 D4）；匹配目录名（只匹配文件名）；拼音 / 首字母缩写；搜索历史；把树的折叠状态落盘。

## 全局约束

- 不引入第三方 markdown / GFM / 高亮 / web 框架依赖（AGENTS 硬规则）；本计划不碰 markdown 渲染。
- **`internal/workspace` 不得 import `.../ui`**：`FuzzyMatch` 只收字符串、只返回 bool。
- 颜色只从 `AppTheme` 取（`t.Accent.Alpha(...)`），不写十六进制；两种底色的数值只放在 `internal/view/theme.go`。
- 视图状态归窗口线程：搜索只在构建期读 `s.trees` / `s.treeSearch`，不读盘、不重扫目录、不 `Post`。
- 视图是 state 的纯函数：`View`（测试）与 shell 的 `Left` / `LeftHeader` / `Content` 路径都要能构建，
  两边的拼装要一起改（`TestShellSlotsBuild` 不许回归）。
- 可交互元素必须给 `label`（无障碍名，也是测试与读屏唯一的定位方式）；**新增的 label 不得与现有重名**
  （树的两个是 `展开全部目录树` / `折叠全部目录树`，目录工具条的是 `展开全部目录` / `折叠全部目录`）。
- 子序列匹配**不得**用正则 / `strings.Contains`：查询里的 `.` `*` `[` `\` 就是普通字符。
- 测试只用 `t.TempDir()`，不得读写真实用户目录；不写平台专用代码，三平台可编译。
- 每个行为改动先有失败的测试；文档回写（README / CHANGELOG / AGENTS / 规格）**并入同一个提交**。
- 提交前 `go vet ./... && gofmt -l . && go test ./...` 必须干净；中文提交信息，前缀 `feat:` / `fix:` / `test:` / `docs:`。

## 设计决策（实施者按此执行，不要另立方案）

| # | 决策 | 理由 |
| --- | --- | --- |
| D1 | **快捷键按窗格分派**：`Cmd/Ctrl+F` 在「焦点在侧栏」时打开树搜索，否则打开正文查找；`Cmd/Ctrl+Shift+F` 任何位置都打开树搜索。焦点不是猜出来的：`State.treeFocused` 由侧栏树的滚动容器 `HandleInput`（`InputPointerDown` → `true`）与正文滚动容器 `HandleInput`（`InputPointerDown` → `false`）维护，`openTreeSearch` 置 `true`、`openFind` 置 `false`。 | 用户选定方案 A：正文查找是既有主功能，不能被抢键。mygo 的 `FocusWithin()` 只在构建期可读、且点正文未必移动焦点，所以用「最后一次按下发生在哪个窗格」这个可断言的事实。 |
| D2 | 模糊匹配 = **文件名上的子序列**，大小写不敏感，按 rune 比较：`FuzzyMatch(name, query) bool`；空白查询返回 `false`（由调用方处理「不过滤」）。候选**不打分、不排序**，按树里的顺序（项目顺序 → 扫描顺序）。 | 用户原话「模糊搜索文件名称」。树序是用户看得懂的稳定顺序：敲字时候选不会跳来跳去。打分排序是另一个功能（见「本期不做」）。 |
| D3 | 搜索时**只画**「命中的文件 + 它们的祖先目录 + 项目节点」；祖先目录用**构建期的临时 `*bool`**（永远 true）强制展开，**不写 `s.open`**。关掉搜索后树原样恢复（用户手动折叠的目录不会因为搜过一次就全开）。 | 搜索要能定位到候选，祖先必须开着；但「搜一次 = 全展开」是副作用，用户没要求。临时指针在 `TreeItem` 里只读不写（`c.Key` 保证节点身份），所以不污染状态。 |
| D4 | 底色分两层：**状态**由 `treeRowFill(projectID, path) treeRowFill` 决定（`rowNone` / `rowCandidate` / `rowCurrent`，纯状态、可无 Context 测试），**颜色**在 `theme.go`：`candidateRowTint(t) = t.Accent.Alpha(0.08)`、`currentCandidateTint(t) = t.Accent.Alpha(0.22)`。搜索期间树项**不再 `Selected`**：底色只有「候选 / 当前候选」两种，已打开文档不额外标。 | 用户原话「候选值与选中的候选值都需要高亮显示 使用两种不同的背景色」。若同时用 `Selected` + 两种底色，会出现三种色、且「当前候选」与「已打开文档」分不清。字符级高亮要自绘行，见「本期不做」。 |
| D5 | 定位：当前候选那一行 `ScrollIntoView()`（`ScrollNearest`、无动画），由 `treeSearch.reveal` 置位触发。清除照 `State.jump` / `jumpSeen` 的规矩：**用掉的那一帧不清，下一帧才清**。 | 一次点击会构建两遍视图（AGENTS 已知坑），构建里立刻清会让请求在第二遍构建前消失。`ScrollNearest` 只保证「露出来」而不是「顶到上面」，键盘连按不会把树弹来弹去。 |
| D6 | 树滚动容器加 `TrackScroll(&s.treeScroll)` 与 `Label("项目树")`。 | 测试只能靠读屏名 `tt.Find("项目树")` 量位置、靠 `s.treeScroll.Y` 断言「真的定位过去了」；目录列表与搜索面板都是这个做法。 |
| D7 | `↑`/`↓` 只在**搜索框**上切换候选（`SearchField.HandleInput`；mygo 在 `deliver` 阶段把按键先交给 `HandleInput`，早于输入框自己移动光标），`Enter` 用 `field.Submitted()`，**切换候选不打开文档**；`Enter` 或点击候选才 `selectDocIn`，并且**搜索保持打开**（可以接着跳别的文件，`Esc` 才收）。 | 用户明确要求「上下切换候选时不打开 只有 Enter/点击 才打开」。保持打开是为了连续切换文件这个主用法。 |
| D8 | `Esc`：树搜索打开时先清查询（树恢复完整），空查询时关闭搜索框；`findShortcuts` 的 `Esc` 只在树搜索**未打开**时生效。 | 与正文查找的 `Esc` 顺序一致（先清空、再关面板），两个面板的键不打架。 |
| D9 | 打开一个面板就收起另一个：`openTreeSearch` 调 `closeFind()`，`openFind` 关掉树搜索（`query` 留着，只是收起）。 | 两个面板都在时 `Esc` / `↑↓` 归属不清；树搜索在侧栏、正文查找在正文，视觉上可以共存，但键盘只有一个。 |
| D10 | 全局两个按钮在 `leftHeader` 里、**项目选择按钮的左边**：`components.IconButton(c, lucide.UnfoldVertical(), "展开全部目录树", components.IconButtonPlain)` 与 `lucide.FoldVertical()` + `折叠全部目录树`。没有打开的项目时 `Disabled(true)`。 | 用户选定「放在 项目选择 左边」。用 `IconButtonPlain` 与侧栏底部、目录工具条一致（AGENTS 硬规则：不要 `IconButtonCompact`）。 |
| D11 | 按项目的两条进右键菜单：项目节点（多项目模式才有该节点）的右键菜单、以及项目面板每行的右键菜单，文案 `展开此项目的全部目录` / `折叠此项目的全部目录`。 | 用户原话「每个项目也可以单独收缩全部/展开全部」，截图红框指的正是项目节点；单项目时没有该节点，所以项目面板也必须有一条路。 |
| D12 | `setTreeOpen(projectID string, open bool)`（`""` = 所有打开的项目）写 `s.open`：多项目模式的 `"project:"+ID`，以及每个目录的 `ID+":"+path`。写入走新的 `setOpen(key, open)`：**已存在就 `*p = open` 就地改**，不存在才 `b := open; s.open[key] = &b`。 | 指针是 mygo `TreeItem` 手里的展开状态；换成新指针会让「这一帧点折叠」与布局读到的状态不同步。 |
| D13 | 候选集合只在三处重建：查询变化、`reloadTree()` 之后（文件增删/重命名、项目开关）、`openTreeSearch` 时。**输入时不重扫目录**（`workspace.BuildTree` 一次都不调）。 | 大项目下每敲一个字符重扫是灾难；树本来就在 `s.trees` 里，搜索是纯内存过滤。 |
| D14 | 数据结构：`treeSearchState{candidates []docSel, dirs map[string]bool}`（`dirs` 的键 `projectID:path`，值是「这里下面有候选」）；当前候选 `current int`，越界一律 `min(current, len-1)` 且有 `len == 0` 的守卫。 | 树按节点画，需要「这个目录下面有没有候选」的 O(1) 查询；`docSel` 已经是文档身份（AGENTS：文档身份是 `docSel{project, path}`）。 |
| D15 | 空态与计数：`searching()` 且候选为空时树区域显示 `没有匹配的文件`，计数显示 `无匹配`；有候选时计数是 `3/12`（当前/总数）。 | 与正文查找的 `findCount()` 一致，用户不用学两套。 |
| D16 | 都不落盘：`treeFocused`、`treeSearch`、`treeScroll` 只在本次运行内有效（与 `find`、`zoom` 一致）；折叠结果也不落盘（`s.open` 今天就不落盘）。 | 落盘要动 `config.Config` 与 `normalize`，不是本期的需求。 |

### 改完长什么样

```
┌───────────────────────────────┬──────────────────────────────────────┐
│ ⇱  ⇲  [ 📁 alpha      ⌄ ]     │  标签条 …                            │
│ ┌ 在项目树中查找 ──── 1/3  ✕ ┐│                                      │
│ │ ⌕ plan                     ││  正文（不变）                        │
│ └────────────────────────────┘│                                      │
│  ▾ alpha                      │                                      │
│    ▾ docs                     │                                      │
│      RELEASE_PLAN.md   ← 候选（淡主色）                               │
│      SENTINEL_PLAN.md  ← 当前候选（浓主色）                           │
│    ▾ src                      │                                      │
│      plan.go.md        ← 候选（淡主色）                               │
│                               │                                      │
│ [📂 打开项目] [⟳]      [🔔] [⋮]│                                     │
└───────────────────────────────┴──────────────────────────────────────┘
   ↑ ⇱ = 展开全部目录树   ⇲ = 折叠全部目录树（在项目选择左边）
```

## 审查重点（Review Focus）

1. **查询里有正则元字符**（`.` `*` `[` `\` `(`）：必须当普通字符做子序列比较，不能崩、不能当通配符。
   测试：`TestFuzzyMatchTreatsPunctuationLiterally`（任务 1）。
2. **候选的文档被删掉 / 项目被关掉后按 Enter**：不能让 `candidates[current]` 越界 panic，也不能打开一个
   不在项目里的路径。测试：`TestEnterWithNoCandidatesDoesNothing`（任务 6）、`TestCandidatesClampTheCurrent`
   与 `TestClosingAProjectDropsItsCandidates`（任务 3）。
3. **敲字时重扫目录**：3000 个文件的项目里每敲一个字符都走一遍磁盘，会卡到输入不跟手。
   测试：`TestSearchingUsesTheTreeInMemory`（任务 3）。
4. **两个面板同时开着**：`Esc` / `↑` / `↓` 归属不清，可能一次清两个查询。测试：
   `TestOpeningOnePanelClosesTheOther`（任务 5）、`TestEscapeClosesOnlyTheTreeSearch`（任务 6）。
5. **`Cmd+F` 分派读到的是上一帧的焦点**：点一下树再按 `Cmd+F` 必须搜文件、点一下正文再按必须查正文；
   不能串。测试：`TestCmdFInTheTreeSearchesFiles`、`TestCmdFInTheBodySearchesTheDocument`（任务 5）。

## 文件结构

| 文件 | 职责 |
| --- | --- |
| `internal/workspace/fuzzy.go`（新建） | 文件名的子序列匹配（纯函数，无 ui 依赖） |
| `internal/workspace/fuzzy_test.go`（新建） | 上面那个函数的行为 |
| `internal/view/treesearch.go`（新建） | 树搜索的状态、候选重建、搜索框那一行、过滤后的树、底色状态、按键 |
| `internal/view/treesearch_test.go`（新建） | 树搜索的视图行为（任务 3–6） |
| `internal/view/state.go`（修改） | `treeFocused` / `treeScroll` / `treeSearch` 三个字段；`setOpen` / `setTreeOpen`；`reloadTree` 之后刷新候选 |
| `internal/view/view.go`（修改） | `leftHeader` 加两个图标按钮；`fileTree` 分叉（过滤/完整）；`Left` 与 `View` 都加搜索框那一行；树滚动容器加 `Label` / `TrackScroll` / `HandleInput` |
| `internal/view/theme.go`（修改） | `candidateRowTint` / `currentCandidateTint` |
| `internal/view/find.go`（修改） | `openFind` 收起树搜索并把 `treeFocused` 置假；`findShortcuts` 交出 `Cmd+F` |
| `internal/view/fileacts.go`（修改） | `handleShortcuts` 调新的 `searchShortcuts` |
| `internal/view/native.go`、`webview.go`（修改） | 正文滚动容器上挂 `HandleInput` 把 `treeFocused` 置假 |
| `internal/view/view_test.go`（修改） | 全局/按项目展开折叠的测试（挨着现有树测试） |
| `README.md` / `CHANGELOG.md` / `AGENTS.md` / 规格 | 与各自的行为改动同一个提交 |

---

### 任务 1：文件名的模糊匹配（纯函数）

**文件：**
- 创建：`internal/workspace/fuzzy.go`
- 测试：`internal/workspace/fuzzy_test.go`
- 修改：`AGENTS.md`（结构表里 `internal/workspace` 那一行补上 `fuzzy.go`）

- [ ] **步骤 1：编写失败的测试**

```go
func TestFuzzyMatchTakesALetterSubsequence(t *testing.T)   // "plan" 命中 "RELEASE_PLAN.md"、"p_l_a_n.md"
func TestFuzzyMatchIgnoresCase(t *testing.T)               // "Plan" 命中 "release_plan.md"
func TestFuzzyMatchKeepsTheOrder(t *testing.T)             // "ml" 不命中 "lm.md"，命中 "m_l.md"
func TestFuzzyMatchRejectsAMissingLetter(t *testing.T)     // "pnx" 不命中 "plan.md"
func TestFuzzyMatchTreatsPunctuationLiterally(t *testing.T) // 查询 "." / "*" / "[" / "\\" 只按字符比较，不命中不含它的名字
func TestFuzzyMatchOfAnEmptyQueryIsFalse(t *testing.T)      // "" 一律 false，由调用方表示「不过滤」
```

- [ ] **步骤 2：运行测试验证失败**

运行：`go test ./internal/workspace/ -run TestFuzzyMatch -v`
预期：FAIL，`undefined: FuzzyMatch`。

- [ ] **步骤 3：在 `internal/workspace/fuzzy.go` 中实现 `FuzzyMatch(name, query string) bool`**

做法：两边 `strings.ToLower` 后各转 `[]rune`，指针扫 `name`，`query` 的每个 rune 依次找到就前进，
扫完时 `query` 的指针走到了末尾即命中；空 `query` 直接 `false`。**不编译正则、不做大小写折叠以外的规范化、不去空格**。

- [ ] **步骤 4：运行测试验证通过**

运行：`go test ./internal/workspace/ -run TestFuzzyMatch -v` 与 `go test ./internal/workspace/`
预期：PASS。

- [ ] **步骤 5：Commit**

```bash
git add internal/workspace/fuzzy.go internal/workspace/fuzzy_test.go AGENTS.md
git commit -m "feat: 项目树文件名模糊匹配的纯函数"
```

---

### 任务 2：全部展开 / 折叠，以及按项目展开 / 折叠

**文件：**
- 修改：`internal/view/state.go`（`setOpen`、`setTreeOpen`）
- 修改：`internal/view/view.go`（`leftHeader` 的两个按钮、项目节点的右键菜单）
- 测试：`internal/view/view_test.go`
- 修改：`README.md`（用法）、`CHANGELOG.md`（新增一条）、`AGENTS.md`（改动指引：改树的批量折叠只改 `setTreeOpen` 与 `leftHeader`）

- [ ] **步骤 1：编写失败的测试**

```go
func TestUnfoldingAndFoldingEveryDirectoryOfTheTree(t *testing.T)
    // notes/ 下有 a.md、draft/ 下有 b.md：
    // build s.View；tt.Click("折叠全部目录树") → !tt.HasText("a.md") && !tt.HasText("b.md")
    // tt.Click("展开全部目录树") → 两个都在
func TestBulkActionsSitLeftOfTheProjectSwitcher(t *testing.T)
    // build s.LeftHeader；tt.Find("展开全部目录树").X + W <= tt.Find("切换项目").X，三者 Y 相同
func TestTheProjectMenuFoldsOneProjectOnly(t *testing.T)
    // 两个项目各有一份 docs/a.md；build s.View
    // tt.RightClick("beta")（项目节点）→ tt.ChooseMenuItem("折叠此项目的全部目录")
    // → beta 的项目节点关掉了，alpha 的 a.md 还在
func TestTheProjectPanelFoldsOneProject(t *testing.T)
    // 只开一个项目时没有项目节点：tt.Click("切换项目") → tt.RightClick("项目：alpha")
    // → tt.ChooseMenuItem("折叠此项目的全部目录") → 树里没有 a.md
```

- [ ] **步骤 2：运行测试验证失败**

运行：`go test ./internal/view/ -run 'TestUnfolding|TestBulkActions|TestTheProjectMenu|TestTheProjectPanel' -v`
预期：FAIL（找不到 `展开全部目录树`）。

- [ ] **步骤 3：实现**

- `state.go`：`func (s *State) setOpen(key string, open bool)`（就地改指针，见 D12）与
  `func (s *State) setTreeOpen(projectID string, open bool)`；后者遍历 `s.trees`，`len(s.trees) > 1` 时
  连 `"project:"+tr.project.ID` 一起设置，再深度优先遍历 `tr.root` 的每个 `IsDir` 节点设置 `ID+":"+path`。
- `view.go` 的 `leftHeader`：外层改成一个 `ui.Row`（`FillWidth`、`AlignItems(ui.Center)`、`Gap(t.Space(0.5))`），
  第一件事是新的 `func (s *State) treeBulkActions(c *ui.Context)`（两个 `IconButtonPlain`，
  `Disabled(len(s.trees) == 0)`，点了分别 `setTreeOpen("", true/false)`），然后才是现有的项目按钮
  （加 `.Grow(1)`）与 `ui.Popover`。
- `view.go` 的 `fileTree` 多项目分支：`ui.TreeItem` 之后调新的 `s.projectMenu(c, item, tr)`，菜单两项即 D11 的文案
  （`m.Separator()` 放在它们之后）；项目面板每行的 `components.ContextMenu` 里也在「移除项目…」之前插入同样两项，
  用 `m.Separator()` 分开。

- [ ] **步骤 4：运行测试验证通过**

运行：`go test ./internal/view/ -run 'TestUnfolding|TestBulkActions|TestTheProjectMenu|TestTheProjectPanel' -v`，
再跑 `go test ./internal/view/` 与 `go test ./...`。
预期：PASS；`TestShellSlotsBuild` 仍然通过。

- [ ] **步骤 5：Commit**

```bash
git add internal/view/state.go internal/view/view.go internal/view/view_test.go README.md CHANGELOG.md AGENTS.md
git commit -m "feat: 项目树全部展开折叠，并按项目单独展开折叠"
```

---

### 任务 3：树搜索的状态与候选（无视图的部分）

**文件：**
- 创建：`internal/view/treesearch.go`
- 修改：`internal/view/state.go`（三个字段、`reloadTree` 末尾刷新候选）
- 测试：`internal/view/treesearch_test.go`

- [ ] **步骤 1：编写失败的测试**

```go
func TestCandidatesComeInTreeOrder(t *testing.T)
    // beta/ 与 alpha/ 各两个 plan 文件：setTreeQuery("plan") 之后 candidates 按「项目顺序 + 扫描顺序」，
    // 元素是 docSel{project, path}
func TestCandidatesMarkTheirDirectories(t *testing.T)
    // 命中 docs/deep/PLAN.md：dirs 里有 "ID:docs" 与 "ID:docs/deep"，没有不相干的目录
func TestClearingTheQueryShowsTheWholeTree(t *testing.T)
    // setTreeQuery("")：candidates 为空、searching() 为假
func TestSearchingUsesTheTreeInMemory(t *testing.T)
    // Refresh 之后把磁盘上的 a.md 删掉（不再 Refresh），setTreeQuery("a")：候选里仍有 a.md（没有重扫）
func TestClosingAProjectDropsItsCandidates(t *testing.T)
    // 两个项目都命中；s.ToggleProject(beta.ID) 之后候选里不再有 beta 的任何文件
func TestCandidatesClampTheCurrent(t *testing.T)
    // current 指向最后一个候选，删文件再 s.Refresh()：current 落在 [0, len-1]
```

- [ ] **步骤 2：运行测试验证失败**

运行：`go test ./internal/view/ -run 'TestCandidates|TestClearingTheQuery|TestSearchingUsesTheTree' -v`
预期：FAIL，`undefined: treeSearchState`。

- [ ] **步骤 3：实现**

`internal/view/treesearch.go` 里：
- `type treeSearchState struct { open, needFocus, reveal bool; query string; candidates []docSel; dirs map[string]bool; current int; revealSeen time.Time }`
- `func (s *State) searching() bool`（`open && strings.TrimSpace(query) != ""`）
- `func (s *State) refreshCandidates()`：深度优先走 `s.trees`，命中 `workspace.FuzzyMatch(node.Name, s.treeSearch.query)`
  的文件按树序追加到 `candidates`，并把每一层的祖先目录写进 `dirs`（键 `ID+":"+path`）；
  末尾 `current = min(current, max(0, len(candidates)-1))`。
- `func (s *State) setTreeQuery(q string)`：`query = q`、`current = 0`、`refreshCandidates()`
- `func (s *State) openTreeSearch()`（`open = true`、`needFocus = true`、`current = 0`、`refreshCandidates()`）与
  `func (s *State) closeTreeSearch()`（`open = false`，`query` 留着）
- `func (s *State) candidateIndex(projectID, path string) int`（线性扫 `candidates`）

`state.go`：加字段 `treeFocused bool`、`treeScroll ui.ScrollState`、`treeSearch treeSearchState`；
`reloadTree()` 末尾加 `if s.treeSearch.open { s.refreshCandidates() }`。任务 5 再给 `openTreeSearch` / `openFind`
补面板互斥与焦点（本任务不涉及）。

- [ ] **步骤 4：运行测试验证通过**

运行：`go test ./internal/view/ -run 'TestCandidates|TestClearingTheQuery|TestSearchingUsesTheTree' -v` 与 `go test ./internal/view/`
预期：PASS。

- [ ] **步骤 5：Commit**

```bash
git add internal/view/treesearch.go internal/view/state.go internal/view/treesearch_test.go
git commit -m "feat: 项目树搜索的候选集合"
```

---

### 任务 4：搜索框、过滤后的树、两种底色与定位

**文件：**
- 修改：`internal/view/treesearch.go`（`treeSearchRow`、`treeMatched`、`treeRowFill`、`treeSearchCount`）
- 修改：`internal/view/view.go`（`fileTree` 分叉、`Left`/`View` 加搜索框、滚动容器三件套）
- 修改：`internal/view/theme.go`（两个底色）
- 测试：`internal/view/treesearch_test.go`

- [ ] **步骤 1：编写失败的测试**

```go
func TestSearchingTheTreeShowsOnlyMatchingFiles(t *testing.T)
    // docs/PLAN.md、docs/notes.md、README.md；s.treeSearch.open = true; s.setTreeQuery("plan")
    // build s.View：HasText("PLAN.md") 为真，HasText("notes.md") / HasText("README.md") 为假
func TestCandidatesAndTheOneChosenAreTwoDifferentFills(t *testing.T)
    // 三个候选、current 在中间：treeRowFill(…PLAN.md) == rowCandidate，
    // treeRowFill(当前那个) == rowCurrent；非候选 == rowNone
    // 另在一个 tester 的构建里断言 candidateRowTint(c.Theme()) != currentCandidateTint(c.Theme())
    // （两种底色必须真的不同，否则「两种高亮」只有一层）
func TestSteppingTheCurrentCandidateScrollsTheTree(t *testing.T)
    // 40 个命中、tester 300 高：current 指到最后一个 + reveal = true → build → s.treeScroll.Y > 0
func TestTheSearchRowCountsItsCandidates(t *testing.T)
    // 3 个命中、current 0 → HasText("1/3")；查询 "zzz" → HasText("无匹配") 且 HasText("没有匹配的文件")
func TestTheTreeScrollerIsNamed(t *testing.T)
    // tt.Find("项目树") 存在，W > 200（侧栏内宽），Y 在搜索框之下
```

- [ ] **步骤 2：运行测试验证失败**

运行：`go test ./internal/view/ -run 'TestSearchingTheTree|TestCandidatesAndTheOne|TestSteppingTheCurrent|TestTheSearchRow|TestTheTreeScroller' -v`
预期：FAIL（树没有过滤、没有 `项目树` 这个 label）。

- [ ] **步骤 3：实现**

- `theme.go`：`candidateRowTint(t) = t.Accent.Alpha(0.08)`、`currentCandidateTint(t) = t.Accent.Alpha(0.22)`。
- `treesearch.go`：
  - `type treeRowFill int` + `rowNone` / `rowCandidate` / `rowCurrent`；
    `func (s *State) treeRowFill(projectID, path string) treeRowFill`（非搜索或非候选 → `rowNone`；
    当前候选 → `rowCurrent`；其余候选 → `rowCandidate`）。
  - `treeSearchRow(c)`：`ui.Row`（`Padding(t.Space(0.5), t.Space(1))`）里 `ui.SearchField(c, &s.treeSearch.query)`、
    `.Label("在项目树中查找")`、`.Grow(1)`、`needFocus` 时 `.AutoFocus()`；`Changed()` → `setTreeQuery(s.treeSearch.query)`；
    `components.Text(c, s.treeSearchCount())`；一个 ✕ `IconButtonCompact`（`关闭树搜索`）调 `closeTreeSearch`。
    **这里不接 ↑↓/Enter**（任务 6）。
  - `treeMatched(c)`：项目节点（多项目）与目录节点都用构建期 `var open = true` 强制展开，
    `dirs[ID+":"+path]` 不在的目录跳过；文件节点 `candidateIndex` 为 -1 的跳过，其余按
    `treeRowFill` 取 `candidateRowTint` / `currentCandidateTint` 并 `.Background(col)`（不调 `Selected`），
    当前候选那一个在 `treeSearch.reveal` 置位时 `.ScrollIntoView()`。
    `reveal` 的清除放在 `fileTree` 开头，照 `State.jump` / `jumpSeen` 的规矩（D5）。
  - `treeSearchCount()`：`无匹配` / `3/12`。
- `view.go`：`fileTree` 的 `ui.Tree` 里 `if s.searching() { s.treeMatched(c); return }`；
  `searching() && len(candidates) == 0` 时树的空态是 `没有匹配的文件`；
  滚动容器改成 `ui.Scroll(c).Grow(1).Label("项目树").TrackScroll(&s.treeScroll)`；
  `Left` 与 `View` 都在滚动容器**之前**加 `if s.treeSearch.open { s.treeSearchRow(c) }`。

- [ ] **步骤 4：运行测试验证通过**

运行：`go test ./internal/view/ -run 'TestSearchingTheTree|TestCandidatesAndTheOne|TestSteppingTheCurrent|TestTheSearchRow|TestTheTreeScroller' -v`
与 `go test ./internal/view/`
预期：PASS。

- [ ] **步骤 5：Commit**

```bash
git add internal/view/treesearch.go internal/view/treesearch_test.go internal/view/view.go internal/view/theme.go
git commit -m "feat: 项目树的搜索框、过滤视图与两种候选底色"
```

---

### 任务 5：快捷键分派与「焦点在哪个窗格」

**文件：**
- 修改：`internal/view/treesearch.go`（`searchShortcuts`、`openTreeSearch` 的焦点与互斥）
- 修改：`internal/view/fileacts.go`（`handleShortcuts` 改调 `searchShortcuts`）
- 修改：`internal/view/find.go`（`openFind` 收起树搜索并置 `treeFocused = false`；`findShortcuts` 交出 `Cmd+F`）
- 修改：`internal/view/view.go`（树滚动容器 `HandleInput` 置 `treeFocused = true`）
- 修改：`internal/view/native.go`、`internal/view/webview.go`（正文滚动容器 `HandleInput` 置 `treeFocused = false`）
- 测试：`internal/view/treesearch_test.go`
- 修改：`README.md`（快捷键表加一行 `Cmd/Ctrl+Shift+F`，并说明 `Cmd/Ctrl+F` 按焦点分派）、
  `AGENTS.md`（坑：`treeFocused` 是上一帧的、label 不许重名）

- [ ] **步骤 1：编写失败的测试**

```go
func TestCmdFInTheTreeSearchesFiles(t *testing.T)
    // 打开文档后 tt.ClickAt(侧栏里的一行) → tt.Frame() → tt.Key(ui.Cmd, ui.KeyF)
    // → s.treeSearch.open 为真、s.find.open 为假
func TestCmdFInTheBodySearchesTheDocument(t *testing.T)
    // tt.ClickAt(700, 400)（正文）→ tt.Frame() → tt.Key(ui.Cmd, ui.KeyF)
    // → s.find.open 为真、s.treeSearch.open 为假
func TestShiftCmdFOpensTheTreeSearchFromAnywhere(t *testing.T)
    // 新 state、没有任何点击：tt.Key(ui.Cmd|ui.Shift, ui.KeyF) → s.treeSearch.open 为真
func TestOpeningOnePanelClosesTheOther(t *testing.T)
    // 先 Cmd+F 开正文查找，再 Cmd+Shift+F：s.find.open 为假；反过来同样
```

- [ ] **步骤 2：运行测试验证失败**

运行：`go test ./internal/view/ -run 'TestCmdF|TestShiftCmdF|TestOpeningOne' -v`
预期：FAIL（`Cmd+F` 仍然只开正文查找）。

- [ ] **步骤 3：实现**

- `searchShortcuts(c)`（`treesearch.go`）在 `handleShortcuts` 的最前面被调用（取代 `s.findShortcuts(c)` 里的 `Cmd+F`）：
  `c.Shortcut(ui.Cmd, ui.KeyF)` 时，`c.Modifiers()&ui.Shift != 0 || s.treeFocused || s.treeSearch.open` →
  `openTreeSearch()`，否则 `openFind()`；其余一律交给 `findShortcuts(c)`。
- `openTreeSearch()` 补三行：`treeFocused = true`、`closeFind()`（D9）；`openFind()` 补三行：
  `treeFocused = false`、`treeSearch.open = false`（`query` 留着）。
- 两个滚动容器的 `HandleInput`：树 → `InputPointerDown` 时 `treeFocused = true`；正文（原生与 webview）→
  `InputPointerDown` 时 `treeFocused = false`；都返回 false，不吃事件。

- [ ] **步骤 4：运行测试验证通过**

运行：`go test ./internal/view/ -run 'TestCmdF|TestShiftCmdF|TestOpeningOne' -v`，再跑 `go test ./internal/view/`、
`go test ./...`、`go vet ./...`、`gofmt -l .`
预期：全绿（`gofmt -l .` 无输出）；现有 `TestFindPanelOpensWithCmdF` 等正文查找的测试仍然通过。

- [ ] **步骤 5：Commit**

```bash
git add internal/view README.md AGENTS.md
git commit -m "feat: Cmd+F 按焦点分派到项目树搜索或正文查找"
```

---

### 任务 6：搜索框的按键（↑↓ / Enter / Esc）

**文件：**
- 修改：`internal/view/treesearch.go`（`moveCandidate`、`openCandidate`、搜索框的 `HandleInput`、`Esc` 分支）
- 修改：`internal/view/find.go`（`findShortcuts` 的 `Esc` 只在树搜索未开时生效）
- 测试：`internal/view/treesearch_test.go`
- 修改：`README.md`（用法：搜索树怎么用）、`CHANGELOG.md`（新增一条）、`AGENTS.md`（坑：`reveal` 下一帧清、搜索不重扫目录）

- [ ] **步骤 1：编写失败的测试**

```go
func TestArrowsStepTheCandidatesAndOpenNothing(t *testing.T)
    // 打开树搜索、查询 "plan"、记下 s.Selected()；tt.Key(0, ui.KeyDown) 两次
    // → s.treeSearch.current == 2，s.Selected() 没变，s.treeScroll.Y 变了（定位）
func TestEnterOpensTheCurrentCandidate(t *testing.T)
    // 同上但 tt.Key(0, ui.KeyEnter) → s.Selected() 是第 current 个候选，搜索仍开着
func TestEnterWithNoCandidatesDoesNothing(t *testing.T)
    // 查询 "zzz"（无候选）：tt.Key(0, ui.KeyEnter) 既不 panic 也不改 s.Selected()
func TestEscapeClearsTheQueryThenClosesTheTreeSearch(t *testing.T)
    // Esc → query 为空、树恢复完整、面板仍开；再 Esc → 面板关
func TestEscapeClosesOnlyTheTreeSearch(t *testing.T)
    // 树搜索开着、正文查找也曾经开着时按 Esc：s.find.query 不被清空
```

- [ ] **步骤 2：运行测试验证失败**

运行：`go test ./internal/view/ -run 'TestArrows|TestEnter|TestEscape' -v`
预期：FAIL（`↑`/`↓` 在搜索框里没有反应，`Enter` 不打开）。

- [ ] **步骤 3：实现**

- `moveCandidate(n int)`：`current = (current + n + len) % len`（`len == 0` 直接返回）、`reveal = true`；
  `openCandidate()`：`len(candidates) == 0` 直接返回，否则 `selectDocIn(cell.project, cell.path)`，搜索保持打开（D7）。
- `treeSearchRow` 的输入框：`.HandleInput` 接 `InputKeyDown` 的 `KeyUp` / `KeyDown`（`moveCandidate(∓1)`、返回 true），
  其余返回 false；`field.Submitted()` → `openCandidate()`。
- `searchShortcuts` 加 `s.treeSearch.open && c.Shortcut(0, ui.KeyEscape)`：有查询就 `setTreeQuery("")`，
  没有就 `closeTreeSearch()`，然后 return；`findShortcuts` 的 `Esc` 分支加 `!s.treeSearch.open` 条件。

- [ ] **步骤 4：运行测试验证通过**

运行：`go test ./internal/view/ -run 'TestArrows|TestEnter|TestEscape' -v`，再跑 `go test ./...`、`go vet ./...`、`gofmt -l .`
预期：全绿。

- [ ] **步骤 5：Commit**

```bash
git add internal/view internal/view/treesearch_test.go README.md CHANGELOG.md AGENTS.md
git commit -m "feat: 树搜索的上下切换、回车打开与 Esc"
```

---

### 任务 7：规格与计划回写

**文件：**
- 修改：`docs/superpowers/specs/2026-11-21-markdown-preview-design.md`（末尾追加一节）
- 修改：`docs/superpowers/plans/2026-11-24-tree-fold-and-file-search.md`（本文件末尾「执行后的修订」）

- [ ] **步骤 1：在规格末尾追加**

`### 项目树的批量折叠与文件名搜索（新增，2026-11-24）`：侧栏顶部（项目选择左边）两个按钮展开/折叠所有目录；
项目节点与项目面板的右键菜单可单独展开/折叠一个项目；`Cmd/Ctrl+F` 按焦点分派（侧栏→文件名模糊搜索，
正文→正文查找），`Cmd/Ctrl+Shift+F` 总是搜文件；搜索时树只显示命中文件与祖先目录、候选两种底色、
`↑↓` 换候选并定位、`Enter`/点击才打开、`Esc` 先清后收；正文查找的键与行为不变。

- [ ] **步骤 2：在本计划末尾追加「执行后的修订」**

按实际落地的差异逐条写（只追加，不改写上面的任务步骤）：实际改了哪些文件、哪些决策在执行中被改掉
（例如焦点记法、底色数值）、执行中冒出来的坑（写进 AGENTS.md 的那几条要在这里点名）。

- [ ] **步骤 3：Commit**

```bash
git add docs/superpowers/specs/2026-11-21-markdown-preview-design.md docs/superpowers/plans/2026-11-24-tree-fold-and-file-search.md
git commit -m "docs: 项目树批量折叠与文件名搜索的规格与计划回写"
```
