# 项目操作面板重做与多项目同时打开 实现计划

> **面向 AI 代理的工作者：** 必需子技能：使用 subagent-driven-development（推荐）或 executing-plans 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** 把「移除当前项目」从项目面板底部挪进项目行的右键菜单并加二次确认，新增「在文件管理器中打开」「复制项目路径」，并让面板可以同时勾选多个项目，树里多一层项目名。

**架构：** 配置新增 `open_ids`（哪些项目打开，`Version` 升到 2，旧文件迁移为全部打开）；`workspace` 的项目集合改成「打开的项目」；`State` 的 `tree *workspace.Node` 换成 `trees []projectTree`，`selected string` 换成 `docSel{project, path}`，这样任何项目的文档都能被选中；面板行用 `components.ContextMenu` 挂右键菜单，`System` 接口隔离桌面动作（打开目录 / 复制 / 确认），测试注入假实现。

**技术栈：** Go 1.27+、`github.com/egoist/mygo v0.9.1`（`ui`、`mygo.Dialog`、`mygo.Shell`、`mygo.Clipboard`）、`github.com/oneo-me/onevra/source v0.2026.18`（`components.ContextMenu`、`components.Menu`）。

**规格：** 无独立规格；本计划的「设计决策」一节即设计记录，架构背景见 `docs/superpowers/specs/2026-11-21-markdown-preview-design.md`。

## 设计决策（实施者按此执行，不要另立方案）

| # | 决策 | 理由 |
| --- | --- | --- |
| D1 | 面板每一行 = 一个项目，行尾的勾表示「打开」；**点这一行切换打开状态**（有勾 → 关闭，无勾 → 打开），面板不关。 | 用户要的是「选中多个项目 = 同时打开」；不再需要「切换活动项目」这个动作，所以整行只有一个含义。 |
| D2 | 「活动项目」不再是用户显式选择的状态，而是**推导**出来的：有选中文档时是文档所属项目，否则是第一个打开的项目。`cfg.ActiveID` 继续维护这个推导值（MCP 的默认项目靠它）。 | 少一个概念少一处状态；MCP 的 `list_projects` / 省略 `project` 参数的行为不变。 |
| D3 | 树里**只在打开项目数 ≥ 2 时**加项目名这一层；只有一个打开项目时保持现状（不加层）。 | 用户原话「打开多个项目时 tree 多一层项目名称树」；单项目加层是白占一行。 |
| D4 | 目录折叠状态按 `项目ID + ":" + 路径` 存，项目名节点的键是 `"project:" + 项目ID`。 | 两个项目都有 `docs/` 时不能互相影响折叠状态。 |
| D5 | 「移除项目」用**系统原生模态**（`mygo.Dialog.Message`，按钮「取消」「移除」，后一个是 destructive），不用应用内 `components.AlertDialog`。 | 应用内 Dialog 只能在某个 shell 槽位里构建，而 `LeftHeader` 在侧栏收起时根本不构建（`shell.go:444`）；原生模态三平台都画在最上层。 |
| D6 | 桌面动作走 `System` 接口（`Reveal` / `CopyText` / `Confirm`），`main` 装 `view.Desktop{}`，测试装假实现。`State.sys == nil` 时三个动作都是 no-op，确认返回「不确认」之外的空操作——即测试里不装 sys 时移除不会发生，测试必须先注入假实现。 | 测试不得依赖真实窗口/系统对话框。 |
| D7 | 右键菜单三项：`在文件管理器中打开`（`mygo.Shell.OpenPath(root)`）、`复制项目路径`（`mygo.Clipboard.WriteText(root)`）、分隔线、`移除项目…`。菜单作用于**被右键的那一行**，不是活动项目。 | 用户要求「移除改为右键操作面板中操作」+ 新增两个路径动作。 |
| D8 | 「打开项目」对话框允许一次选多个目录（`Multiple: true`），每个都打开。 | 多项目场景下逐个选太慢。 |
| D9 | `webview` 的 `mdview://` 只服务**打开的项目**（`Roots()` 从 `cfg.Projects` 改为 `cfg.OpenProjects()`）。 | 保持「只服务已打开项目」这条安全性质；关闭的项目不该再被取文件。 |

### 面板与树长什么样（验收画面）

```
侧栏头部 [📁 alpha ▾]                  ← 点开面板
  ┌──────────────────────────┐
  │ 📁 alpha              ✓ │        ← 有 ✓ = 打开；右键：打开/复制路径/移除…
  │ 📁 beta                 │        ← 无 ✓ = 未打开（点它打开）
  └──────────────────────────┘
树（两个项目打开时）：
  ▾ alpha
    ▾ docs
        architecture.md
  ▾ beta
        only.md
```

## 全局约束

- Go 模块名 `mdpreview`；不引入新依赖（不新增第三方包）。
- 不写平台专用代码（不加 `//go:build windows` 分支），三平台可编译。
- 测试一律用 `t.TempDir()`；不得读写真实用户目录，不得调用需要真实窗口的 `mygo.Dialog.*` / `mygo.Shell.*`（一律走 `System` 假实现）。
- `internal/workspace` 与 `internal/mcp` 不得 import `.../ui`（只有 `ui/markdown/parser` 可以）。
- 颜色只从 `AppTheme` 取（`c.Theme()`）；图标只用 `icons/lucide`；图标按钮必须给 `label`。
- 视图状态归窗口线程：`System.Confirm` 只在窗口线程调用（和现有 `Dialog.Open` 一样是模态阻塞），不要放进 `Post` 的回调里再等。
- 提交信息用中文，前缀 `feat:` / `fix:` / `test:` / `docs:` / `style:`；每个行为改动先有失败的测试。
- 提交前 `go vet ./... && gofmt -l .` 必须无输出。

## 文件结构

| 文件 | 职责 | 本计划里的改动 |
| --- | --- | --- |
| `internal/config/config.go` | 配置读写、项目增删、活动项目 | 加 `OpenIDs`、`IsOpen`/`SetOpen`/`OpenProjects`，`Version` → 2，`normalize` 做迁移与清理 |
| `internal/config/config_test.go` | 配置测试 | 加迁移、开关、活动项目不变量的测试 |
| `internal/workspace/project.go` | 项目集合 | `Projects()` 只返回打开的项目 |
| `internal/workspace/project_test.go` | 新建 | `Projects()` 过滤与 `Active()` 的测试 |
| `internal/view/system.go` | 新建：桌面动作接口 + `Desktop` 实现 | `System`、`ConfirmOptions`、`Desktop` |
| `internal/view/state.go` | 窗口状态 | `docSel`、`projectTree`、`trees`、`ToggleProject`、`SetSystem`、`revealProject`、`copyProjectPath`、`RemoveProject` 二次确认 |
| `internal/view/view.go` | 侧栏头部面板 / 树 | 面板行勾选 + 右键菜单；`fileTree`/`treeItems` 支持多项目 |
| `internal/view/native.go` | 原生渲染、链接跳转、TOC | 文档读写与链接改用 `s.sel` |
| `internal/view/preview.go` | 右栏分派与空状态 | 用 `trees` 判断空状态 |
| `internal/view/webview.go` | webview 与协议 | 渲染模式表按 `docSel` 存；`Roots()` 返回打开的项目 |
| `internal/view/watch.go` | 文件监听 | 变更按项目归属匹配选中文档 |
| `internal/view/view_test.go` / `watch_test.go` / `layout_test.go` | 视图测试 | 多项目、面板、菜单、确认的测试；`fakeSystem` |
| `main.go` | 组装 | `state.SetSystem(view.Desktop{})` |
| `AGENTS.md` / `README.md` | 项目说明 | 记录 `open_ids`、右键菜单、多项目树 |

## 审查重点（Review Focus）

1. **一个项目都没打开**（`open_ids: []`，或用户把唯一项目关掉）→ 右栏提示「打开一个目录开始预览 markdown」，侧栏提示「打开一个目录作为项目」，面板仍列出项目且右键仍可用，不 panic、不卡在旧文档上。测试：`TestNoOpenProjectShowsTheEmptyState`（任务 3）。
2. **两个项目里有同名文件**（都叫 `docs/a.md`）→ 点哪一个就加载哪一个，标题栏显示各自的项目名，切换渲染模式互不影响。测试：`TestSamePathInTwoProjectsLoadsItsOwnBody`（任务 3）。
3. **打开的项目目录被删掉** → 只丢那一个项目，其他项目的树、选中文档、监听都保留；不全清空、不每帧重复删。测试：`TestGoneProjectKeepsTheOthers`（任务 3）。
4. **旧配置或脏配置**（没有 `open_ids`、`open_ids` 指向已删除的项目、`active_id` 指向关闭的项目）→ 迁移后不丢项目、不显示不存在的项目、`active_id` 始终是打开的项目。测试：`TestLoadOpensEverythingWhenTheFileHasNoOpenIDs`、`TestOpenIDsOfRemovedProjectsAreDropped`、`TestClosingActiveProjectPicksAnother`（任务 1）。
5. **取消移除** → 项目、目录、监听都还在，选中文档不变。测试：`TestCancelledRemovalKeepsTheProject`（任务 6）。

---

## 任务 1：配置记住哪些项目打开

**文件：**
- 修改：`internal/config/config.go`
- 测试：`internal/config/config_test.go`

- [ ] **步骤 1：写失败的测试**

在 `config_test.go` 追加（值都用 `t.TempDir()` 下的路径）：

```go
func TestLoadOpensEverythingWhenTheFileHasNoOpenIDs(t *testing.T) // 手写 v1 JSON（无 open_ids），Load 后 OpenProjects() 长度为 2、Version == 2
func TestSetOpenTogglesTheProject(t *testing.T)                   // SetOpen(id,false) → IsOpen false、OpenProjects 少一个；再 SetOpen(true) 回来
func TestOpenIDsOfRemovedProjectsAreDropped(t *testing.T)         // 手写 open_ids:["ghost","<alpha>"]，Load 后 OpenProjects() 只有 alpha
func TestClosingActiveProjectPicksAnother(t *testing.T)           // 两个打开项目、active=alpha，SetOpen(alpha,false) → ActiveID == beta.ID
func TestAddProjectOpensIt(t *testing.T)                          // AddProject 后 IsOpen(p.ID) 为 true
func TestRemoveProjectClosesIt(t *testing.T)                      // RemoveProject 后 OpenProjects 不含它、ActiveID 是打开的
```

- [ ] **步骤 2：运行测试验证失败**

运行：`go test ./internal/config/ -run 'OpenIDs|SetOpen|ClosingActive|AddProjectOpens|RemoveProjectCloses' -v`
预期：FAIL，`c.OpenProjects undefined`（编译错误也算失败）。

- [ ] **步骤 3：实现（`internal/config/config.go`）**

```go
// Version 2 adds OpenIDs, the projects the window shows.
const Version = 2

type Config struct {
	...
	OpenIDs []string `json:"open_ids"`
}

// IsOpen reports whether the project with id is open: shown in the window and
// watched for changes.
func (c Config) IsOpen(id string) bool

// SetOpen opens or closes the project with id, keeping ActiveID on an open
// project. It reports whether there is such a project.
func (c *Config) SetOpen(id string, open bool) bool

// OpenProjects returns the projects that are open, in the order they were
// added.
func (c Config) OpenProjects() []Project

// normalize applies the defaults and drops what the file no longer explains:
// a file without open_ids (every file before version 2) has every project
// open, applied once, and OpenIDs never names a project that is gone.
func (c *Config) normalize()
```

要点：
- `normalize` 用 `c.OpenIDs == nil` 判断「旧文件」（JSON 里没有这个字段是 nil，`[]` 是非 nil 空切片，两者不能混）；不是 nil 时按 `Projects` 过滤，保留顺序。最后若 `ActiveID` 不是打开的项目，改成第一个打开的项目（都没有则空）。
- `Load` 在现有默认值之后调用 `c.normalize()`；`Save` 之前也调用一次，保证写出去的文件自洽。
- `AddProject` 把新 id 追加到 `OpenIDs`（已有的不重复加）并按现状设 `ActiveID`。
- `RemoveProject` 同时从 `OpenIDs` 删掉，并按现状修正 `ActiveID`。

- [ ] **步骤 4：运行测试验证通过**

运行：`go test ./internal/config/ -v`
预期：PASS（含原有 `TestSaveLoadRoundTrip` 等）。

- [ ] **步骤 5：Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat: 配置记住哪些项目打开（open_ids，版本 2）"
```

---

## 任务 2：workspace 的项目集合 = 打开的项目

**文件：**
- 修改：`internal/workspace/project.go`
- 测试：创建 `internal/workspace/project_test.go`

- [ ] **步骤 1：写失败的测试**

```go
func TestProjectsListsOnlyOpenOnes(t *testing.T) // cfg: alpha 打开、beta 关闭（SetOpen(beta,false)）→ w.Projects() 只有 alpha
func TestActiveIsAnOpenProject(t *testing.T)     // Active() 返回的项目 IsOpen 为 true；全关时 ok == false
```

- [ ] **步骤 2：运行测试验证失败**

运行：`go test ./internal/workspace/ -run 'TestProjectsListsOnlyOpenOnes|TestActiveIsAnOpenProject' -v`
预期：FAIL（`Projects` 仍返回两个 / `Active` 仍返回关闭的项目）。

- [ ] **步骤 3：实现**

`Projects()` 在锁内返回 `w.cfg.OpenProjects()` 的副本。`Active()` 保持不变（`cfg.ActiveID` 已被任务 1 的不变量保证是打开的项目），只在 `!w.cfg.IsOpen(id)` 时返回 `false` 兜底。

- [ ] **步骤 4：运行测试验证通过**

运行：`go test ./internal/workspace/ -v`
预期：PASS。

- [ ] **步骤 5：Commit**

```bash
git add internal/workspace/project.go internal/workspace/project_test.go
git commit -m "feat: workspace 的项目集合只含打开的项目"
```

---

## 任务 3：多项目树与文档归属

这是本计划的核心：文档不再「属于活动项目」，而是自己带着项目。

**文件：**
- 修改：`internal/view/state.go`、`internal/view/view.go`、`internal/view/native.go`、`internal/view/preview.go`、`internal/view/webview.go`、`internal/view/watch.go`
- 测试：`internal/view/view_test.go`、`internal/view/watch_test.go`

- [ ] **步骤 1：写失败的测试**

在 `view_test.go` 追加一个建两个项目的辅助函数（两个目录都 `os.MkdirAll`，都通过 `cfg.AddProject` 打开）：

```go
// twoProjects returns a window with alpha and beta open, and their roots.
func twoProjects(t *testing.T) (*State, string, string)

func TestTreeGroupsByProjectWhenSeveralAreOpen(t *testing.T) // alpha/docs/a.md + beta/only.md → Texts 含 "alpha","beta","a.md","only.md"；len(s.Trees()) == 2
func TestSingleOpenProjectHasNoProjectLevel(t *testing.T)    // 一个打开项目 + notes/a.md → s.Trees() 长度为 1，且树里没有名为 "alpha" 的节点（用 s.Trees()[0].root.Children 断言顶层是 "notes"）
func TestNoOpenProjectShowsTheEmptyState(t *testing.T)       // 两个项目都 SetOpen(false) 后 New(...) → 文本含 "打开一个目录开始预览 markdown"，且 s.Selected() == ""
func TestSamePathInTwoProjectsLoadsItsOwnBody(t *testing.T)  // 两个项目各有 docs/a.md，内容分别含 "ALPHA" / "BETA"；左键点第二个 a.md → s.SelectedProject().ID == beta.ID、s.src 含 "BETA"、标题含 "beta/docs/a.md"
func TestDirectoriesOfTwoProjectsOpenApart(t *testing.T)     // 两个项目都有 docs 目录；把 alpha 的 docs 折叠（*s.openOf(alpha.ID+":docs") = false）后，beta 的 a.md 仍在文本里
func TestGoneProjectKeepsTheOthers(t *testing.T)             // 删掉 alpha 目录后 s.Refresh() → Projects() 只剩 beta，beta 的文档仍在树里，选中文档（beta 的）保留
func TestSelectingDocSyncsWorkspace(t *testing.T)            // 选中 beta 的文档后 ws.Active().ID == beta.ID（MCP 的默认项目跟着走）
```

在 `watch_test.go` 改写 `TestApplyChangesIgnoresOtherProjects` 之外再补：

```go
func TestApplyChangesOnlyTouchesItsOwnProjectsDoc(t *testing.T) // 选中 alpha/a.md，另一个项目的监听报告删除 a.md → 选中文档还在
```

同时**改写**这两个现有测试（行为变了）：
- `TestChoosingProjectSwitchesIt` → 删掉（所选行为被 `TestSamePathInTwoProjectsLoadsItsOwnBody` 取代；面板勾选在任务 4 测）。
- `TestSelectProjectSyncsWorkspace` → 并入上面的 `TestSelectingDocSyncsWorkspace`。
- `TestTreeKeepsOpenStateAcrossRefresh` → 折叠键改成 `s.Trees()[0].project.ID + ":notes"`。

- [ ] **步骤 2：运行测试验证失败**

运行：`go test ./internal/view/ -run 'TreeGroupsByProject|SingleOpenProject|NoOpenProject|SamePathInTwoProjects|DirectoriesOfTwoProjects|GoneProjectKeeps|SelectingDocSyncs' -v`
预期：FAIL，`s.Trees undefined`。

- [ ] **步骤 3：改 `state.go`**

同时加一个小查询（任务 5、6 用）：`func (s *State) project(id string) (workspace.Project, bool)`（在 `s.cfg.Projects` 里按 id 找）。

```go
// docSel names the document the window shows: the project it belongs to, and
// its path in it.
type docSel struct {
	project string
	path    string
}

func (d docSel) empty() bool { return d.path == "" }

// projectTree is one open project's tree, as the sidebar shows it.
type projectTree struct {
	project workspace.Project
	root    *workspace.Node
}
```

- `State.tree` → `trees []projectTree`；`State.selected` → `sel docSel`（`renderMode` 改成 `map[docSel]config.RenderMode`）。
- `Trees() []projectTree` 替代 `Tree()`；`Selected() string` 返回 `s.sel.path`；新增 `SelectedProject() (workspace.Project, bool)`。
- `selectDoc(rel string)` 保持签名（对活动项目操作，供 TOC 链接与现有测试用），内部转调新方法：

```go
// selectDocIn shows a document of the project with projectID, saving which
// project that makes active.
func (s *State) selectDocIn(projectID, rel string)
```

`selectDocIn` 在项目变化时更新 `cfg.ActiveID`、`ws.SetConfig` 并 `save()`（同一项目内换文档不写盘）。
- `reloadTree()` 遍历 `cfg.OpenProjects()` 逐个 `workspace.BuildTree`；某个项目返回错误时按现有 `gone` 规则**只**移除它、把 `status` 设成「项目目录不可用：<root>」，其余项目照常建树；最后校验 `s.sel`（`ws.Stat(sel.project, sel.path)` 失败就清成 `docSel{}`）。
- 新增 `hasDocs() bool`（任一打开项目的树非空），空状态判断用 `s.sel.empty()`。
- `shortPath()` 用 `s.SelectedProject()` 的名字；view 的 `SetActive`、`SelectProject` 删掉（不再有显式切换；`config.SetActive` 留着，MCP 与配置测试仍在用）。

- [ ] **步骤 4：改 `view.go` 的树**

```go
func (s *State) fileTree(c *ui.Context)           // 空状态用 len(s.trees) == 0 / !s.hasDocs()
func (s *State) treeItems(c *ui.Context, tr projectTree) // 目录键用 tr.project.ID+":"+child.Path；文件用 s.selectDocIn(tr.project.ID, child.Path)
```

`fileTree` 里：`len(s.trees) == 1` 时直接 `s.treeItems(c, s.trees[0])`；否则每个项目先 `ui.TreeItem(c, tr.project.Name, s.openOf("project:"+tr.project.ID), func(){ s.treeItems(c, tr) })`。

- [ ] **步骤 5：改 `native.go` / `preview.go` / `webview.go` / `watch.go`**

- `native.go`：`s.selected` → `s.sel.path`，项目一律取 `s.SelectedProject()`；`followLink` 的同项目路径解析与 `gotoHeading` 不变；正文上方那行路径用 `s.sel.path`。
- `preview.go`：`case len(s.trees) == 0` / `case s.sel.empty() && s.hasDocs()` / `case s.sel.empty()`。
- `webview.go`：`mode()` / `setMode` 的键换成 `s.sel`；`loadWebDoc` 里取项目用 `s.SelectedProject()`；`Roots()` 返回 `s.cfg.OpenProjects()`。
- `watch.go`：`applyChanges` 的匹配加上项目归属：

```go
case change.Old != "" && id == s.sel.project && change.Old == s.sel.path: // 跟随重命名
case id == s.sel.project && change.Path == s.sel.path:                    // 重载或删除
```

- [ ] **步骤 6：运行测试验证通过**

运行：`go test ./internal/view/ -v`
预期：PASS。若 `TestShellSlotsBuild` 因树多一层失败，检查它用的是单项目（应该不变）。

- [ ] **步骤 7：Commit**

```bash
git add internal/view/state.go internal/view/view.go internal/view/native.go \
  internal/view/preview.go internal/view/webview.go internal/view/watch.go \
  internal/view/view_test.go internal/view/watch_test.go
git commit -m "feat: 树按项目分组，文档跟着自己的项目"
```

---

## 任务 4：面板勾选多个项目

**文件：**
- 修改：`internal/view/state.go`（`ToggleProject`）、`internal/view/view.go`（`leftHeader`）
- 测试：`internal/view/view_test.go`

- [ ] **步骤 1：写失败的测试**

```go
func TestPanelTogglesProjectOpen(t *testing.T)     // 两个项目打开："切换项目" → 点 "beta" → cfg 里 beta 关闭（s.Trees() 长度 1，文本里没有 "only.md"），面板仍开着；再点 "beta" → 又打开
func TestPanelListsClosedProjectsToo(t *testing.T) // beta 先 SetOpen(false) 再 New；点开面板后文本含 "beta"（关掉的项目仍在列表里可再打开）
func TestClosingProjectStopsWatching(t *testing.T) // s.watchReady = true 后关闭 beta → s.watches 里没有 beta.ID
```

- [ ] **步骤 2：运行测试验证失败**

运行：`go test ./internal/view/ -run 'TestPanelTogglesProjectOpen|TestPanelListsClosedProjectsToo|TestClosingProjectStopsWatching' -v`
预期：FAIL（点 beta 目前是切换活动项目并关闭面板）。

- [ ] **步骤 3：实现 `State.ToggleProject`**

```go
// ToggleProject opens or closes a project: the open ones are the projects the
// tree shows, the window watches, and the web view serves.
func (s *State) ToggleProject(id string)
```

要点：`open := !s.cfg.IsOpen(id)`；`s.cfg.SetOpen(id, open)`；打开时 `startWatch`，关闭时 `stopWatch`，且若 `s.sel.project == id` 就清空选中并 `onDocChanged()`；然后 `ws.SetConfig`、`save()`、`reloadTree()`。不关面板。

删掉 `SelectProject`（任务 3 已经删了 `SetActive`）。

- [ ] **步骤 4：改 `leftHeader` 面板行**

每行保持现在的 `components.Button(c, "", components.ButtonGhost).Label(p.Name)` + 文件夹图标 + 名字 + 尾部图标，改为：
- 名字后面用 `lucide.Check()`（`t.Accent`）表示打开，未打开不画（不占位也行，保持行高一致用 `ui.Box` 占位）；
- `if s.cfg.IsOpen(p.ID) { row.Background(selectedRowTint(t)) }`（保留现有选中底色）；
- `row.Clicked()` → `s.ToggleProject(p.ID)`，**不再**关闭面板；
- 空列表文案保持「还没有项目」；
- 删掉面板底部的分隔线与「移除当前项目」按钮（任务 6 会把「移除」放进右键菜单）。

- [ ] **步骤 5：运行测试验证通过**

运行：`go test ./internal/view/ -v`
预期：PASS（`TestToolbarShowsProjectNames` 仍应通过：两个项目都在面板里）。

- [ ] **步骤 6：Commit**

```bash
git add internal/view/state.go internal/view/view.go internal/view/view_test.go
git commit -m "feat: 项目面板可同时勾选打开多个项目"
```

---

## 任务 5：右键菜单：打开/复制路径、移除

**文件：**
- 创建：`internal/view/system.go`
- 修改：`internal/view/state.go`、`internal/view/view.go`
- 测试：`internal/view/view_test.go`（`fakeSystem` 定义在这里）

- [ ] **步骤 1：写失败的测试**

```go
// fakeSystem records what the window asked of the desktop.
type fakeSystem struct {
	revealed, copied string
	asked            bool
	confirm          bool
	revealErr        error
}

func (f *fakeSystem) Reveal(path string) error { f.revealed = path; return f.revealErr }
func (f *fakeSystem) CopyText(text string)     { f.copied = text }
func (f *fakeSystem) Confirm(options ConfirmOptions) bool { f.asked = true; return f.confirm }

func TestProjectMenuCopiesThePath(t *testing.T)        // 注入 fake，"切换项目" → RightClick("beta") → Click("复制项目路径") → fake.copied == betaRoot，Status 含 "已复制"
func TestProjectMenuRevealsTheFolder(t *testing.T)     // 同上点 "在文件管理器中打开" → fake.revealed == betaRoot
func TestProjectMenuActsOnTheClickedProject(t *testing.T) // 活动项目是 alpha，右键 beta 并复制 → 复制的是 beta 的路径
func TestProjectMenuRevealFailureIsReported(t *testing.T) // fake.revealErr 非空 → Status 含 "文件管理器"
```

- [ ] **步骤 2：运行测试验证失败**

运行：`go test ./internal/view/ -run 'TestProjectMenu' -v`
预期：FAIL，`fakeSystem` 未实现 `System`（编译错误）或菜单项不存在。

- [ ] **步骤 3：写 `internal/view/system.go`**

```go
// System is what the window asks of the desktop beyond drawing it: revealing a
// project in the file manager, copying a path, and asking before a project is
// removed. main supplies Desktop; the tests supply a fake.
type System interface {
	Reveal(path string) error
	CopyText(text string)
	Confirm(options ConfirmOptions) bool
}

// ConfirmOptions is what to ask before an action the window cannot undo.
type ConfirmOptions struct {
	Title, Message, ConfirmLabel string
}

// Desktop is the System the app runs with: the system's file manager, the
// clipboard, and a modal message box.
type Desktop struct{}

func (Desktop) Reveal(path string) error { return mygo.Shell.OpenPath(path) }
func (Desktop) CopyText(text string)     { mygo.Clipboard.WriteText(text) }
func (Desktop) Confirm(o ConfirmOptions) bool {
	// Dialog.Message with Buttons {"取消", o.ConfirmLabel}, DefaultButton 0,
	// CancelButton 0, DestructiveButtons []int{1}, Type MessageQuestion;
	// true only when Button == 1 and err == nil.
}
```

`mygo.Clipboard.WriteText` / `mygo.Shell.OpenPath` / `mygo.Dialog.Message` 的签名见 mygo v0.9.1（`clipboard.go`、`modules.go`、`dialog.go`）。

- [ ] **步骤 4：状态侧**

```go
// SetSystem gives the window the desktop it acts through.
func (s *State) SetSystem(sys System)

func (s *State) revealProject(p workspace.Project) // s.sys == nil → return；失败 status = "无法打开文件管理器：" + err
func (s *State) copyProjectPath(p workspace.Project) // s.sys == nil → return；成功 status = "已复制项目路径：" + p.Root
```

- [ ] **步骤 5：面板行挂菜单（`view.go`）**

行元素要先按键稳定（`c.Key("project:"+p.ID)` 或构造后 `row.Key(...)`），然后在**同一帧内、建完行之后**：

```go
components.ContextMenu(c, row, func(m *components.Menu) {
	if m.Item("在文件管理器中打开").Chosen() { s.revealProject(p) }
	if m.Item("复制项目路径").Chosen() { s.copyProjectPath(p) }
	m.Separator()
	if m.Item("移除项目…").Chosen() { s.RemoveProject(p.ID) } // 任务 6 让它先确认
})
```

- [ ] **步骤 6：运行测试验证通过**

运行：`go test ./internal/view/ -v`
预期：PASS。

- [ ] **步骤 7：Commit**

```bash
git add internal/view/system.go internal/view/state.go internal/view/view.go internal/view/view_test.go
git commit -m "feat: 项目右键菜单支持在文件管理器中打开与复制路径"
```

---

## 任务 6：移除项目要二次确认

**文件：**
- 修改：`internal/view/state.go`
- 测试：`internal/view/view_test.go`

- [ ] **步骤 1：写失败的测试**

```go
func TestRemovingAProjectAsksFirst(t *testing.T)        // fake.confirm = false；右键 "beta" → "移除项目…" → fake.asked 为 true，Projects() 仍有 beta，面板仍列出 beta
func TestCancelledRemovalKeepsTheProject(t *testing.T)   // 同上，且选中 beta 的文档时它仍是选中文档
func TestConfirmedRemovalDropsTheProject(t *testing.T)  // fake.confirm = true → Projects() 不含 beta、s.watches 不含 beta.ID，面板不再有 beta
func TestRemovalWithoutSystemDoesNothing(t *testing.T)   // 不注入 sys，直接 s.RemoveProject(beta.ID) → 项目还在（宁可不删也不无提示地删）
```

注意：`TestRemoveProjectFallsBackToNext` 与 `TestGoneProjectIsDropped` 里直接调 `s.RemoveProject(...)` 的用例要改成注入 `fakeSystem{confirm: true}`，或在测试里调用内部的 `s.removeProject(id)`（本任务把原 `RemoveProject` 的实现改名为私有 `removeProject`）。

- [ ] **步骤 2：运行测试验证失败**

运行：`go test ./internal/view/ -run 'TestRemovingAProject|TestCancelledRemoval|TestConfirmedRemoval|TestRemovalWithoutSystem' -v`
预期：FAIL（现在直接删）。

- [ ] **步骤 3：实现**

```go
// RemoveProject removes a project from the window, after the desktop confirms:
// the window cannot undo it.
func (s *State) RemoveProject(id string) {
	p, ok := s.project(id)
	if !ok || s.sys == nil {
		return
	}
	if !s.sys.Confirm(ConfirmOptions{
		Title:        "移除项目？",
		Message:      "从列表中移除 “" + p.Name + "”？项目里的文件不会被删除。",
		ConfirmLabel: "移除",
	}) {
		s.status = "已取消移除"
		return
	}
	s.removeProject(id)
}
```

`removeProject` 是原 `RemoveProject` 的实现（停监听、`cfg.RemoveProject`、修 `ActiveID`、清选中、`save`、`reloadTree`）。

- [ ] **步骤 4：运行测试验证通过**

运行：`go test ./internal/view/ -v`
预期：PASS。

- [ ] **步骤 5：Commit**

```bash
git add internal/view/state.go internal/view/view_test.go
git commit -m "feat: 移除项目前二次确认"
```

---

## 任务 7：打开项目支持多选、main 装配、文档

**文件：**
- 修改：`internal/view/state.go`（`AddProjects`、`OpenProject`）、`main.go`、`AGENTS.md`、`README.md`
- 测试：`internal/view/view_test.go`

- [ ] **步骤 1：写失败的测试**

```go
func TestAddProjectsOpensAll(t *testing.T) // AddProjects([]{alphaRoot, betaRoot}) → 两个项目都打开、Trees() 长度 2、ActiveProject().ID == beta.ID
```

- [ ] **步骤 2：运行测试验证失败**

运行：`go test ./internal/view/ -run TestAddProjectsOpensAll -v`
预期：FAIL，`AddProjects undefined`。

- [ ] **步骤 3：实现**

```go
// OpenProject asks for directories and opens each of them.
func (s *State) OpenProject() // Multiple: true，然后 s.AddProjects(paths)

// AddProjects opens the directories at roots, the last one active, and reports
// how many it added.
func (s *State) AddProjects(roots []string) int
```

`AddProject(root)` 变成 `AddProjects([]string{root})` 的包装（保留给现有测试）。每个新项目：`cfg.AddProject`、`cfg.SetOpen(id, true)`、`startWatch`、`ActiveID = 最后加的那个`；最后 `ws.SetConfig`、`save`、`reloadTree`。`gone` 只在真正新增项目时清空。

- [ ] **步骤 4：`main.go` 装配**

在 `state.SetWindow(win)` 之后加：

```go
state.SetSystem(view.Desktop{})
```

- [ ] **步骤 5：运行测试验证通过**

运行：`go test ./... && go vet ./... && gofmt -l .`
预期：全部 PASS，vet 与 gofmt 无输出。

- [ ] **步骤 6：手动验收（会开窗口）**

运行：`go run .`
逐条确认：
1. 打开两个目录（对话框可多选），树里出现两个项目名节点，各带自己的文件；
2. 点面板行勾选框能关掉一个项目，树里那一层消失，另一个项目的折叠状态不变；
3. 右键项目行 → 复制项目路径（能粘贴出目录）→ 在文件管理器中打开（资源管理器定位到目录）；
4. 右键「移除项目…」→ 出现系统对话框，取消后项目还在、监听还在，确认后消失；
5. 关掉所有项目 → 右栏显示「打开一个目录开始预览 markdown」；
6. 用外部编辑器改另一个项目里的文件 → 没有选中它时树照常刷新，选中它时正文跟随。

- [ ] **步骤 7：更新文档**

`AGENTS.md`：
- 「结构」表里 `internal/view` 一行补一句：面板勾选打开的项目、右键菜单（打开/复制/确认移除）、多项目树多一层项目名。
- 「已知的坑」加两条：`cfg.OpenIDs` 的 `nil` 与 `[]` 语义不同（迁移只在 nil 时发生）；`docSel` 是文档身份，别再退回只认路径；应用内 `components.Dialog` 在侧栏槽位不可靠，确认一律走 `System.Confirm`。
- 「改动指引」加一条：加桌面动作时扩 `System` 接口 + `fakeSystem`，不要在 view 里直接调 `mygo.Dialog`/`mygo.Shell`。

`README.md`：功能列表补「多项目同时打开（树按项目分组）」「项目右键菜单：在文件管理器中打开 / 复制路径 / 移除（二次确认）」，并把 MCP 的 `list_projects` 描述改为「已打开的项目」。

- [ ] **步骤 8：Commit**

```bash
git add internal/view/state.go internal/view/view_test.go main.go AGENTS.md README.md
git commit -m "feat: 一次打开多个项目；装配桌面动作并更新文档"
```

---

## 收尾检查

- [ ] `go test ./...` 全绿
- [ ] `go vet ./... && gofmt -l .` 无输出
- [ ] 审查重点 5 条各自都有对应测试存在并通过
- [ ] `git log --oneline` 显示 7 个中文提交
