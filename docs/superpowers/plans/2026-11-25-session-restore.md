# 会话恢复（顶部标签页 + 侧栏树折叠）实现计划

> **面向 AI 代理的工作者：** 必需子技能：使用 subagent-driven-development（推荐）或 executing-plans 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** 重启后恢复上次的会话现场——顶部标签页与活动标签、侧栏树里被折叠的目录；项目的开合与活动项目本就由 `config.json`（`open_ids`/`active_id`）保留，本次只补视图层的回归测试。

**架构：** 新包 `internal/session` 只管一个 JSON 快照文件 `session.json`（读、原子写、不 import ui）。视图状态仍是窗口线程的内存态（`s.tabs`/`s.sel`/`treeOpen.Open`/`treeSeen`），在每次「标签或树折叠变化」时把快照写下去；`view.New` 启动时读回来恢复。折叠记忆采用**显式 closed 集**（`treeClosed`），与 `treeOpen.Open`（含搜索的临时展开）解耦，保证任何时刻快照都正确。

**技术栈：** Go 标准库（`encoding/json`），原子写复用 `internal/config` 的「临时文件 + 改名」模式；不引新依赖。

**规格：** 无独立规格文档。需求来自用户 2026-11-25 的请求（「记忆项目树的打开状态 / 项目的打开状态 / 文件的打开状态（顶部 tab）」）；设计依据记录在下面的「设计决定」，执行者两处都读。

## 全局约束

- 不引入第三方 markdown / GFM / 高亮 / web 框架依赖；本功能只用标准库。
- `internal/session` 不 import `.../ui`（与 `internal/notices` 同级要求），可无窗口测试。
- 配置与状态一律写 `App.Path(PathUserData)` 下的 JSON；写盘走「临时文件 + 改名」原子写。
- 测试只用 `t.TempDir()`；`view.New(ws, cfg, "")` 表示无用户数据目录——`session.Load` 返回空会话、`session.Save` 是 no-op。
- 视图状态归窗口线程：session 保存只发生在用户动作与既有回程上，**一次动作最多一次写盘**（`setTreeKeyOpen` 里绝不写盘）。
- 树与标签的路径统一 `/` 分隔（workspace 树的 `Path`），`session.Ref.Path` 与 `docSel.path` 同形；单文件项目的 `Path` 是文件名。
- 树的任何改动 `children()` 结果的操作都要 `bumpTree()`；恢复逻辑不得绕过既有回程。
- 提交信息中文，前缀 `feat:` / `test:` / `docs:`；提交前 `go vet ./... && gofmt -l . && go test ./...` 干净；文档回写并入行为改动同一提交。

## 审查重点（Review Focus）

1. **session.json 被外部改坏 / 写到一半** → `Load` 返回空会话 + err，应用照常以默认现场启动，不崩、不影响 `config.json`。钉住：任务 1 的坏文件测试 + 任务 2 的「坏文件不阻断恢复」。
2. **上次会话后文件被删 / 项目被关或移除** → 恢复的 tab 被既有 `afterTreesChanged` 剪枝静默丢弃；活动文件没了沿用「文件已被删除」提示；closed 键指向已删目录时保持该键折叠（与内存语义一致）。钉住：任务 2 的剪枝测试、任务 4 的僵尸键测试。
3. **树搜索的临时展开绝不能落盘** → 用户折叠过的目录在搜索开着时被一次标签操作写盘，折叠记忆不能被抹掉；`openForSearch`/`closeSearchOpen` 直接动 `Open`、不碰 `treeClosed`，快照读 `treeClosed` 不从 `Open` 推导。钉住：任务 5 的「搜索期间保存」测试。
4. **单文件项目与多项目的项目节点**（`treeKey{project, path: ""}`）→ 标签与折叠键同样恢复；单项目窗口没有项目节点，不得给它写键。钉住：任务 4 的单文件与多项目节点测试。
5. **大树上「折叠全部」** → `setTreeOpen` 走几万次 `setTreeKeyOpen` 只在尾部写盘一次；写盘是快照的原子写（closed 集大也只是几 MB 一次性写）。钉住：任务 5 的「写盘次数」测试（用 `SetSyncScan` + 计数替身不可行时，以「尾部一次调用」的代码结构 + `setTreeOpen` 测试钉住）。

## 设计决定（执行者照此实现；用户可推翻）

- **D1 独立快照文件**：`session.json` + `internal/session` 包，不并入 `config.json`——config 是用户偏好（项目、渲染模式、MCP），session 是上次现场；树折叠集合混进 config 会搅乱它的 `Version` 迁移，还会让 `config.Save` 的调用点背上折叠集合的体积。
- **D2 只持久化「被折叠的目录」**：现有语义是「扫到的目录自动展开」（`noteNewDirectories`：未见过的键 → 标 `treeSeen` + 加 `Open`），折叠记忆 = `treeSeen` 有而 `Open` 无。恢复时把 closed 键标进 `treeSeen` 即可完整重现；closed 集通常只有几个键，文件小。不持久化「展开的」目录。
- **D3 范围之外不持久化**：目录窗格（TOC）的展开状态（`s.open`）、每个标签的正文滚动位置（`s.scrolls`）、搜索面板、消息面板开合、树搜索查询、侧栏宽度（mygo shell 的 `StateKey` 已管窗口位置）。
- **D4 恢复是静默的**：不弹「已打开 xxx」；恢复不走 `OpenDocument`/`selectDocIn`，直接设 `s.tabs`/`s.sel` 后 `onDocChanged()`。文件缺失的提示沿用既有剪枝路径。
- **D5 活动标签无效则右侧空白**：`Active` 不在恢复的 `Tabs` 里时 `sel` 为空，不擅自挑一个标签。
- **D6 键形用对象数组**：`treeKey{project, path}` 序列化成 `{"project":…,"path":…}`，不做 `"id:path"` 字符串拼接，免 escape 顾虑。

## 文件结构

- 创建 `internal/session/session.go` —— 会话快照的结构与读写，唯一的持久化新面。
- 创建 `internal/session/session_test.go` —— Load/Save 的行为钉子。
- 修改 `internal/view/state.go` —— `State` 加 `treeClosed map[treeKey]bool`；`New` 里 `session.Load` → `restoreSession` → `reloadTree` → 活动文档加载；新增 `saveSession` / `restoreSession`。
- 修改 `internal/view/tabs.go` —— 五个标签变更点（`openTab` / `closeTab` / `dropTab` / `dropTabsOf` / `renameTab`）尾部 `saveSession()`。
- 修改 `internal/view/tree.go` —— `setTreeKeyOpen` 维护 `treeClosed`；`toggleTreeKey` / `setTreeOpen` 尾部 `saveSession()`。
- 修改 `internal/view/scan.go` —— `dropTree` 随 `treeSeen` 一起清掉该项目的 `treeClosed`。
- 修改 `internal/view/locate.go` —— crosshair 展开祖先链后 `saveSession()` 一次。
- 修改 `README.md` / `AGENTS.md` / `CHANGELOG.md` —— 文档回写（并入任务 5 的提交）。

依赖方向新增：`view → session`（session 无依赖、不 import ui）；`main.go` 无改动（`dir` 已传入 `view.New`）。

### 接口（任务间共用，签名照抄）

```go
// internal/session/session.go
const Version = 1
const FileName = "session.json"

// Ref 是一条会话记忆的身份：标签页是「项目 + 项目内路径」，
// 树折叠键是「项目 + 目录路径（项目节点为 ""）」。
type Ref struct {
    Project string `json:"project"`
    Path    string `json:"path"`
}

type Session struct {
    Version    int    `json:"version"`
    Tabs       []Ref  `json:"tabs"`        // 标签栏顺序
    Active     Ref    `json:"active"`      // 活动标签；空表示无
    TreeClosed []Ref  `json:"tree_closed"` // 被折叠的目录（含项目节点）
}

// Load 读 dir 下的 session.json。dir 为 ""、没有文件、文件读不了或 JSON 坏，
// 都给 Version=Version 的空会话；后两者带回 err 供调用方记日志。
func Load(dir string) (Session, error)

// Save 把 s 原子写进 dir（临时文件 + 改名）。dir 为 "" 时不写、返回 nil。
func Save(dir string, s Session) error
```

```go
// internal/view/state.go 新增
treeClosed map[treeKey]bool // 用户折叠过的树键；与 treeOpen.Open 解耦

func (s *State) saveSession()      // 组装快照（tabs、active、closed）并 Save
func (s *State) restoreSession(sess session.Session)
```

---

### 任务 1：`internal/session` 快照读写

**文件：**
- 创建：`internal/session/session.go`
- 测试：`internal/session/session_test.go`

- [ ] **步骤 1：编写失败的测试**

```go
func TestSaveThenLoadRoundTrips(t *testing.T) {
    dir := t.TempDir()
    in := Session{Version: Version,
        Tabs:  []Ref{{Project: "a", Path: "docs/x.md"}, {Project: "a", Path: "y.md"}},
        Active: Ref{Project: "a", Path: "y.md"},
        TreeClosed: []Ref{{Project: "a", Path: "docs"}, {Project: "b", Path: ""}},
    }
    if err := Save(dir, in); err != nil { t.Fatal(err) }
    out, err := Load(dir)
    if err != nil { t.Fatal(err) }
    if out.Version != Version || !slices.Equal(out.Tabs, in.Tabs) ||
        out.Active != in.Active || !slices.Equal(out.TreeClosed, in.TreeClosed) {
        t.Errorf("round trip = %+v, want %+v", out, in)
    }
}

func TestMissingEmptyOrBrokenFilesGiveAnEmptySession(t *testing.T) {
    dir := t.TempDir()
    if s, err := Load(dir); err != nil || s.Version != Version ||
        len(s.Tabs) != 0 || len(s.TreeClosed) != 0 {
        t.Errorf("missing file = %+v, %v; want empty session, no error", s, err)
    }
    if err := os.WriteFile(filepath.Join(dir, FileName), []byte("{oops"), 0o644); err != nil { t.Fatal(err) }
    if s, err := Load(dir); err == nil || s.Version != Version {
        t.Errorf("broken file = %+v, %v; want empty session and an error", s, err)
    }
}

func TestEmptyDirSavesNothingAndLoadsNothing(t *testing.T) {
    if err := Save("", Session{Version: Version}); err != nil {
        t.Errorf("Save(\"\") = %v, want no-op nil", err)
    }
    if s, err := Load(""); err != nil || s.Version != Version {
        t.Errorf("Load(\"\") = %+v, %v; want empty session", s, err)
    }
}

func TestSaveLeavesNoTempFilesBehind(t *testing.T) {
    dir := t.TempDir()
    if err := Save(dir, Session{Version: Version}); err != nil { t.Fatal(err) }
    entries, _ := os.ReadDir(dir)
    if len(entries) != 1 || entries[0].Name() != FileName {
        t.Errorf("dir holds %v; want only %s", names(entries), FileName)
    }
}
```

- [ ] **步骤 2：运行测试验证失败**

运行：`go test ./internal/session/ -v`
预期：FAIL，报错 `package mdpreview/internal/session` 无 `Save`/`Load`（编译失败即算失败）。

- [ ] **步骤 3：实现 `internal/session/session.go`**

按「接口」块的签名。`Save` 照抄 `internal/config` 的 `Save`：`MkdirAll` → `CreateTemp(dir, FileName+".*.tmp")` → 写 → `Sync` → `Rename`，`defer os.Remove(name)`。`Load` 照 `internal/notices` 的 `Load` 语义：`ErrNotExist`、`dir == ""`、JSON 坏都给空会话。包注释说明它是「上一次窗口的现场快照， beside the configuration」，与 notices 同风格。

- [ ] **步骤 4：运行测试验证通过**

运行：`go test ./internal/session/ -v`
预期：PASS。

- [ ] **步骤 5：Commit**

```bash
git add internal/session/
git commit -m "feat: 会话快照 session.json 的读写（internal/session）"
```

### 任务 2：启动时恢复标签页、活动标签与项目开合

**文件：**
- 修改：`internal/view/state.go`（`State` 加 `treeClosed` 字段与初始化、`restoreSession`、`New` 的恢复与活动文档加载）
- 测试：`internal/view/view_test.go`（或就近的 `tabs_test.go`，与既有标签测试同文件即可）

本任务只做「读」：测试用 `session.Save` 预置快照（任务 1 已就绪），不依赖任务 3 的自动写回。标签剪枝完全依赖既有 `afterTreesChanged`（`ws.Stat` 走磁盘，不依赖树落地）。

- [ ] **步骤 1：编写失败的测试**

```go
func TestRestartRestoresTheTabsAndTheActiveOne(t *testing.T) {
    root, dir := t.TempDir(), t.TempDir()
    var cfg config.Config
    p, _ := cfg.AddProject(root, "alpha")
    cfg.ActiveID = p.ID
    writeDoc(t, root, "a.md", "# A\n")
    writeDoc(t, root, "b.md", "# B\n")
    err := session.Save(dir, session.Session{Version: session.Version,
        Tabs:   []session.Ref{{Project: p.ID, Path: "a.md"}, {Project: p.ID, Path: "b.md"}},
        Active: session.Ref{Project: p.ID, Path: "b.md"},
    })
    if err != nil { t.Fatal(err) }

    s := New(workspace.New(cfg), cfg, dir)
    s.SetSyncScan()
    tt := ui.NewTester(s.View, 1000, 700)
    if len(s.tabs) != 2 || s.tabs[0].path != "a.md" || s.tabs[1].path != "b.md" {
        t.Errorf("tabs = %v, want a.md then b.md", s.tabs)
    }
    if s.sel.path != "b.md" {
        t.Errorf("sel = %q, want b.md", s.sel.path)
    }
    if !tt.HasText("B") {
        // 活动文档已加载：正文是 # B。
        t.Errorf("texts = %q, want the restored document's body", tt.Texts())
    }
}

func TestRestartDropsTabsOfClosedProjects(t *testing.T) {
    // 快照里的 tab 指向一个存在但未打开的项目：恢复时丢弃。
    root, other, dir := t.TempDir(), t.TempDir(), t.TempDir()
    var cfg config.Config
    p, _ := cfg.AddProject(root, "alpha")
    q, _ := cfg.AddProject(other, "beta")
    cfg.OpenIDs = []string{p.ID} // beta 关着
    cfg.ActiveID = p.ID
    writeDoc(t, other, "q.md", "# Q\n")
    _ = session.Save(dir, session.Session{Version: session.Version,
        Tabs:   []session.Ref{{Project: q.ID, Path: "q.md"}},
        Active: session.Ref{Project: q.ID, Path: "q.md"},
    })
    s := New(workspace.New(cfg), cfg, dir)
    s.SetSyncScan()
    if len(s.tabs) != 0 || !s.sel.empty() {
        t.Errorf("tabs = %v, sel = %v; want none restored", s.tabs, s.sel)
    }
}

func TestRestartPrunesTabsWhoseFileIsGone(t *testing.T) {
    // 快照里的文件已删除：既有剪枝把它拿掉，活动标签没了就回到空状态。
    root, dir := t.TempDir(), t.TempDir()
    var cfg config.Config
    p, _ := cfg.AddProject(root, "alpha")
    cfg.ActiveID = p.ID
    writeDoc(t, root, "a.md", "# A\n") // b.md 不写
    _ = session.Save(dir, session.Session{Version: session.Version,
        Tabs:   []session.Ref{{Project: p.ID, Path: "a.md"}, {Project: p.ID, Path: "b.md"}},
        Active: session.Ref{Project: p.ID, Path: "b.md"},
    })
    s := New(workspace.New(cfg), cfg, dir)
    s.SetSyncScan()
    if len(s.tabs) != 1 || s.tabs[0].path != "a.md" || !s.sel.empty() {
        t.Errorf("tabs = %v, sel = %v; want only a.md, nothing active", s.tabs, s.sel)
    }
}

func TestRestartRestoresTheOpenProjects(t *testing.T) {
    // 回归钉子：项目的开合与活动项目由 config.json 承担，视图层必须对齐它。
    rootA, rootB := t.TempDir(), t.TempDir()
    var cfg config.Config
    a, _ := cfg.AddProject(rootA, "alpha")
    b, _ := cfg.AddProject(rootB, "beta")
    cfg.OpenIDs = []string{a.ID}
    cfg.ActiveID = b.ID
    cfg.normalize() // ActiveID 必须落在打开的项目上
    s := New(workspace.New(cfg), cfg, "")
    s.SetSyncScan()
    if len(s.trees) != 1 || s.trees[0].project.ID != a.ID {
        t.Errorf("trees = %v, want only alpha", s.trees)
    }
}
```

- [ ] **步骤 2：运行测试验证失败**

运行：`go test ./internal/view/ -run 'TestRestart' -v`
预期：FAIL——`New` 不读 session，`s.tabs` 为空。

- [ ] **步骤 3：实现恢复**

在 `internal/view/state.go`：

1. `State` 加字段 `treeClosed map[treeKey]bool`；`New` 里初始化为 `map[treeKey]bool{}`（与 `treeSeen` 并排）。
2. `New` 里、**`s.reloadTree()` 之前**（否则 `noteNewDirectories` 会把恢复的折叠目录全部重新展开）：`sess, _ := session.Load(dir)` → `s.restoreSession(sess)`。
3. `restoreSession` 按接口语义实现：closed 键过滤 `cfg.IsOpen` 后**同时**写 `s.treeSeen[k]=true` 与 `s.treeClosed[k]=true`；tabs 过滤（`Project != "" && Path != "" && cfg.IsOpen`，`hasTab` 去重）后按快照顺序追加；`Active` 必须是恢复出的某个 tab 才设 `s.sel`（D5）；若 `s.sel.project != s.cfg.ActiveID` 则 `cfg.SetActive` 并同步 `s.activeID`。
4. `New` 末尾 `s.reloadTree()` 之后：`if !s.sel.empty() { s.onDocChanged() }`（D4：静默，不走 `OpenDocument`/`selectDocIn`）。测试里 `SetSyncScan → Refresh` 会再加载一次，无害。

- [ ] **步骤 4：运行测试验证通过**

运行：`go test ./internal/view/ -run 'TestRestart' -v && go test ./internal/view/`
预期：新测试 PASS，既有测试无回归。

- [ ] **步骤 5：Commit**

```bash
git add internal/view/state.go internal/view/view_test.go
git commit -m "feat: 启动时恢复上次的标签页与活动标签"
```

### 任务 3：标签变化写回快照

**文件：**
- 修改：`internal/view/state.go`（`saveSession`）
- 修改：`internal/view/tabs.go`（五个变更点尾部调用）
- 测试：`internal/view/tabs_test.go`

- [ ] **步骤 1：编写失败的测试**

```go
func TestTabChangesAreWrittenBack(t *testing.T) {
    root, dir := t.TempDir(), t.TempDir()
    var cfg config.Config
    p, _ := cfg.AddProject(root, "alpha")
    cfg.ActiveID = p.ID
    writeDoc(t, root, "a.md", "# A\n")
    writeDoc(t, root, "b.md", "# B\n")
    s := New(workspace.New(cfg), cfg, dir)
    s.SetSyncScan()
    s.selectDocIn(p.ID, "a.md")
    s.selectDocIn(p.ID, "b.md") // b 活动且是第二个标签
    s.closeTab(docSel{project: p.ID, path: "a.md"})

    out, err := session.Load(dir)
    if err != nil { t.Fatal(err) }
    if len(out.Tabs) != 1 || out.Tabs[0] != (session.Ref{Project: p.ID, Path: "b.md"}) {
        t.Errorf("session tabs = %v, want only b.md", out.Tabs)
    }
    if out.Active != (session.Ref{Project: p.ID, Path: "b.md"}) {
        t.Errorf("session active = %v, want b.md", out.Active)
    }

    // 重启闭环：只剩一个标签。
    s2 := New(workspace.New(cfg), cfg, dir)
    s2.SetSyncScan()
    if len(s2.tabs) != 1 || s2.sel.path != "b.md" {
        t.Errorf("restart tabs = %v, sel = %v; want b.md active", s2.tabs, s2.sel)
    }
}

func TestDroppingTabsOfAProjectUpdatesTheSession(t *testing.T) {
    // 关闭项目（dropTabsOf）后快照里不再有它的标签；renameTab 换路径同样跟着走。
    // 两个断言各起一个现场，照 TestTabChangesAreWrittenBack 的构造。
}
```

第二个测试完整写出来：现场 A 开两个项目的文档各一个 → `ToggleProject(另一项目ID)` → `session.Load` 断言无该项目标签、`Active` 不指向它；现场 B `renameTab(from, to)` → 断言快照里是新路径。

- [ ] **步骤 2：运行测试验证失败**

运行：`go test ./internal/view/ -run 'TestTabChangesAreWrittenBack|TestDroppingTabsOfAProject' -v`
预期：FAIL——`session.Load` 读到的是 `New`/恢复路径之外的空快照（`Tabs` 为空）。

- [ ] **步骤 3：实现 `saveSession` 并接线**

`saveSession` 组装快照：`Tabs`/`Active` 照 `s.tabs`/`s.sel` 原样；`TreeClosed` 从 `s.treeClosed` 收集、**过滤 `s.cfg.IsOpen(k.project)`**、按 `(Project, Path)` 字典序排序（文件内容稳定）。`session.Save(s.cfgDir, …)` 的错误忽略（与 `s.save()` 同法）。`s.cfgDir == ""` 时 `Save` 自行 no-op。

`internal/view/tabs.go` 五处尾部各加一行 `s.saveSession()`：`openTab`（`activateTab` 之后）、`closeTab`（切邻/清空之后，**一次**）、`dropTab`、`dropTabsOf`（`sel` 清理之后）、`renameTab`。`activateTab` 不加（由 `openTab`/`closeTab` 覆盖，避免重复写盘）。

- [ ] **步骤 4：运行测试验证通过**

运行：`go test ./internal/view/ -v`
预期：PASS，无回归（启动剪枝路径现在也会落盘剪枝结果——无害）。

- [ ] **步骤 5：Commit**

```bash
git add internal/view/state.go internal/view/tabs.go internal/view/tabs_test.go
git commit -m "feat: 标签变化写回会话快照"
```

### 任务 4：记忆并恢复侧栏树的折叠状态

**文件：**
- 修改：`internal/view/tree.go`（`setTreeKeyOpen` 维护 `treeClosed`）
- 修改：`internal/view/scan.go`（`dropTree` 清理）
- 测试：`internal/view/tree_test.go`（与既有树测试同文件；多项目/单文件用例放 `singlefile_test.go` 或同文件均可）

恢复半边已在任务 2 的 `restoreSession` 里（closed → `treeSeen` + `treeClosed`）；本任务补「维护」与行为验证。写盘接线在任务 5，本任务的测试预置快照。

- [ ] **步骤 1：编写失败的测试**

```go
func TestRestartKeepsTheFoldedDirectoriesFolded(t *testing.T) {
    root, dir := t.TempDir(), t.TempDir()
    var cfg config.Config
    p, _ := cfg.AddProject(root, "alpha")
    cfg.ActiveID = p.ID
    writeDoc(t, root, "notes/a.md", "# A\n")
    writeDoc(t, root, "other/b.md", "# B\n")
    _ = session.Save(dir, session.Session{Version: session.Version,
        TreeClosed: []session.Ref{{Project: p.ID, Path: "notes"}},
    })
    s := New(workspace.New(cfg), cfg, dir)
    s.SetSyncScan()
    tt := ui.NewTester(s.View, 1000, 700)
    if tt.HasText("a.md") {
        t.Errorf("texts = %q, want notes folded", tt.Texts())
    }
    if !tt.HasText("b.md") {
        t.Errorf("texts = %q, want other open: unseen directories auto-open", tt.Texts())
    }
}

func TestFoldingADirectoryMarksItClosed(t *testing.T) {
    s, root := newState(t)
    writeDoc(t, root, "notes/a.md", "# A\n")
    s.Refresh()
    id := s.Trees()[0].project.ID
    s.setTreeKeyOpen(treeKeyOf(id, "notes"), false)
    if !s.treeClosed[treeKeyOf(id, "notes")] {
        t.Errorf("treeClosed = %v, want notes in it", s.treeClosed)
    }
    s.setTreeKeyOpen(treeKeyOf(id, "notes"), true)
    if s.treeClosed[treeKeyOf(id, "notes")] {
        t.Errorf("treeClosed = %v, want notes out of it", s.treeClosed)
    }
}

func TestAClosedProjectLeavesNoClosedKeysBehind(t *testing.T) {
    // dropTree（项目被拒 / 移除）随 treeSeen 一起清掉该项目的 treeClosed。
    s, root := newState(t)
    writeDoc(t, root, "notes/a.md", "# A\n")
    s.Refresh()
    id := s.Trees()[0].project.ID
    s.setTreeKeyOpen(treeKeyOf(id, "notes"), false)
    s.dropTree(id)
    for k := range s.treeClosed {
        if k.project == id {
            t.Errorf("treeClosed still holds %v after dropTree", k)
        }
    }
}
```

- [ ] **步骤 2：运行测试验证失败**

运行：`go test ./internal/view/ -run 'TestRestartKeepsTheFolded|TestFoldingADirectory|TestAClosedProjectLeavesNoClosedKeys' -v`
预期：前两个 FAIL（`treeClosed` 未维护 / 恢复后没标 `treeSeen`，`notes` 被自动展开），第三个编译期或运行期 FAIL。

- [ ] **步骤 3：实现维护与清理**

`setTreeKeyOpen`（`internal/view/tree.go`）：

```go
func (s *State) setTreeKeyOpen(k treeKey, open bool) {
    if open {
        s.treeOpen.Open.Add(k)
        delete(s.treeClosed, k)
        return
    }
    s.treeOpen.Open.Remove(k)
    s.treeClosed[k] = true
}
```

`noteNew` 不改（只对未见过的键触发，不产生 closed）。`internal/view/scan.go` 的 `dropTree`：在清 `treeSeen` 的同一循环里 `delete(s.treeClosed, k)`。`restoreSession` 若任务 2 已写好 closed 恢复，这里只核对。

- [ ] **步骤 4：运行测试验证通过**

运行：`go test ./internal/view/ -v`
预期：PASS，无回归（`TestTreeKeepsOpenStateAcrossRefresh` 等既有折叠测试不受影响——`treeClosed` 只加记忆，不改 `Open` 的行为）。

- [ ] **步骤 5：Commit**

```bash
git add internal/view/tree.go internal/view/scan.go internal/view/tree_test.go
git commit -m "feat: 记忆侧栏树折叠的目录并在启动时恢复"
```

### 任务 5：树折叠变化写回快照 + 文档回写

**文件：**
- 修改：`internal/view/tree.go`（`toggleTreeKey`、`setTreeOpen` 尾部写盘）
- 修改：`internal/view/locate.go`（crosshair 展开祖先链后写盘一次）
- 修改：`README.md`、`AGENTS.md`、`CHANGELOG.md`
- 测试：`internal/view/tree_test.go`、`internal/view/treesearch_test.go`

- [ ] **步骤 1：编写失败的测试**

```go
func TestFoldChangesAreWrittenBack(t *testing.T) {
    root, dir := t.TempDir(), t.TempDir()
    var cfg config.Config
    p, _ := cfg.AddProject(root, "alpha")
    cfg.ActiveID = p.ID
    writeDoc(t, root, "notes/a.md", "# A\n")
    writeDoc(t, root, "other/b.md", "# B\n")
    s := New(workspace.New(cfg), cfg, dir)
    s.SetSyncScan()

    s.toggleTreeKey(treeKeyOf(p.ID, "notes")) // 单行折叠
    out, _ := session.Load(dir)
    if !slices.Contains(out.TreeClosed, session.Ref{Project: p.ID, Path: "notes"}) {
        t.Errorf("closed = %v, want notes in it", out.TreeClosed)
    }

    s.setTreeOpen(p.ID, true) // 展开全部：closed 清空，只写一次盘
    out, _ = session.Load(dir)
    if len(out.TreeClosed) != 0 {
        t.Errorf("closed = %v, want none after expand-all", out.TreeClosed)
    }

    s.setTreeOpen(p.ID, false) // 折叠全部：两个目录 + 单项目无项目节点
    out, _ = session.Load(dir)
    if len(out.TreeClosed) != 2 {
        t.Errorf("closed = %v, want the two directories only", out.TreeClosed)
    }
}

func TestSearchDoesNotDisturbTheClosedSet(t *testing.T) {
    // 审查重点 3：搜索的临时展开不落盘——快照读 treeClosed，不从 Open 推导。
    root, dir := t.TempDir(), t.TempDir()
    var cfg config.Config
    p, _ := cfg.AddProject(root, "alpha")
    cfg.ActiveID = p.ID
    writeDoc(t, root, "notes/a.md", "# A\n")
    writeDoc(t, root, "other/b.md", "# B\n")
    s := New(workspace.New(cfg), cfg, dir)
    s.SetSyncScan()
    s.toggleTreeKey(treeKeyOf(p.ID, "notes")) // 用户折叠 notes

    s.openTreeSearch() // 或走既有测试打开搜索的入口
    s.closeTab(s.sel)  // 搜索开着时触发一次 saveSession
    out, _ := session.Load(dir)
    if !slices.Contains(out.TreeClosed, session.Ref{Project: p.ID, Path: "notes"}) {
        t.Errorf("closed = %v, want notes kept while the search is open", out.TreeClosed)
    }
}
```

测试里打开搜索的入口照 `treesearch_test.go` 既有的写法（`openTreeSearch` + 查询字段）；断言不变式是「搜索开着时的快照与搜索前一致」。

- [ ] **步骤 2：运行测试验证失败**

运行：`go test ./internal/view/ -run 'TestFoldChangesAreWrittenBack|TestSearchDoesNotDisturb' -v`
预期：FAIL——折叠变化尚未写盘（`TreeClosed` 为空）。

- [ ] **步骤 3：接线三处写盘**

- `toggleTreeKey` 尾部 `s.saveSession()`（单行点击；`treeMark` 与行的 `OnClick` 都走它）。
- `setTreeOpen` 尾部（`bumpTree()` 之后）`s.saveSession()` 一次——批量展开/折叠几万目录也只写一次盘（审查重点 5）。
- `internal/view/locate.go` 的 crosshair 展开祖先链的函数尾部 `s.saveSession()`（定位展开是持久的，折叠记忆随之解除）。
- `closeSearchOpen`/`openForSearch` **不加**写盘：它们直接动 `Open`、不碰 `treeClosed`，无需落盘。

- [ ] **步骤 4：运行测试验证通过**

运行：`go test ./internal/view/ -v && go test ./... && go vet ./... && gofmt -l .`
预期：全绿、vet/gofmt 干净。

- [ ] **步骤 5：文档回写**

- `README.md`：「配置文件与数据存放位置」的文件表加一行 `session.json`（内容：打开的标签页与活动标签、侧栏树里被折叠的目录）；「用法」或行为描述处补一句「重启后恢复上次的标签页与树的折叠状态（项目的开合与活动项目随 config.json 保留）」。
- `AGENTS.md`：结构表加 `internal/session` 行（会话快照的读写，原子写，不 import ui）；依赖方向加 `view → session`；「改动指引」加一条「改会话记忆」：写盘点 = 五个标签变更点 + `toggleTreeKey` / `setTreeOpen` / locate，**`setTreeKeyOpen` 里不写盘**；恢复必须在 `reloadTree` 之前（closed → `treeSeen`），否则 `noteNewDirectories` 把折叠目录全部展开；搜索的临时展开不落盘（快照读 `treeClosed`，不从 `Open` 推导）；`treeClosed` 随 `dropTree` 清理。「已知的坑」照此补两三行。
- `CHANGELOG.md`「未发布 → 新增」：

```markdown
- **重启后恢复上次的会话现场**：顶部标签页与活动标签、侧栏树里被折叠的目录都会回来；
  项目的开合与活动项目本来就随配置保留。`session.json` 丢失或写坏只会回到默认现场，不影响其他数据。
```

- [ ] **步骤 6：Commit**

```bash
git add internal/view/tree.go internal/view/locate.go README.md AGENTS.md CHANGELOG.md internal/view/tree_test.go internal/view/treesearch_test.go
git commit -m "feat: 重启恢复会话现场——树折叠写回快照并回写文档"
```

---

## 自检记录

- **规格覆盖度**：用户三项诉求——项目的开合与活动项目（既有 `config.json` 承担；任务 2 的 `TestRestartRestoresTheOpenProjects` 回归钉子）、树的折叠（任务 4 + 任务 5）、标签页与活动标签（任务 2 + 任务 3）。
- **步骤扫描**：测试给出名字与断言；实现只钉签名与关键算法（`setTreeKeyOpen` 的 closed 维护、`restoreSession` 的过滤规则、写盘点清单），函数体留给实现者。
- **类型一致性**：`session.Ref` / `session.Session` / `treeClosed map[treeKey]bool` / `saveSession` / `restoreSession` / `treeKeyOf`（既有测试助手）在各任务一致。
- **审查重点**：1→任务 1 坏文件测试；2→任务 2 剪枝测试 + 任务 4 僵尸键（`dropTree`）测试；3→任务 5 搜索测试；4→任务 4 的单文件/多项目节点（`Path: ""`）由 `TestRestartKeepsTheFoldedDirectoriesFolded` 的键形覆盖，多项目节点用例在任务 5 的 `setTreeOpen` 断言（单项目无项目节点）与 `tree_test.go` 既有折叠测试中覆盖；5→任务 5 写盘点结构 + `setTreeOpen` 测试。
- **比例**：计划约为既有计划平均长度，代码块只有测试与签名，无实现照抄。

## 执行后的修订（2026-11-27）

- **D3 的半条被推翻**：侧栏宽度与每个文档的目录宽度现在也进 `session.json`（`sidebar` 与 `toc` 两个字段，
  `Version` 升到 2）。原因：onevra 的 shell 只给 `LeftWidth()`、不持久化也不发事件，`StateKey` 只管窗口位置，
  所以「用户每次都要重新拖」成了实际抱怨。实现见
  `docs/superpowers/plans/2026-11-27-pane-widths-and-outline-follow.md`。
- D3 里其余几条仍然成立：目录窗格的展开状态、标签的正文滚动位置、搜索面板与消息面板的开合、树搜索查询
  依旧不持久化。
