# Markdown Preview 实现计划

> **面向 AI 代理的工作者：** 必需子技能：使用 subagent-driven-development（推荐）或 executing-plans 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** 一个纯 Go（`github.com/egoist/mygo`）桌面应用，多项目管理、md 文件树 + TOC + 正文预览（原生与 webview 双后端）、外部变更实时跟随，并带一个本地 MCP server 骨架。

**架构：** 三层：`workspace`（项目扫描 / 读取 / 解析缓存 / TOC，实现 `Index` 接口，不依赖 UI）、`mcp`（JSON-RPC over HTTP，消费 `Index`）、`view`（原生 UI，是 state 的纯函数，靠 `win.Invalidate()` 被外部事件唤醒）。markdown 只解析一次（`ui/markdown/parser`），原生模式喂源字符串给 `ui/markdown`，webview 模式喂 `Document.HTML()` 给自定义协议。

**技术栈：** Go 1.27+、`github.com/egoist/mygo v0.9.1`（`ui`、`ui/markdown`、`ui/markdown/parser`、`mygo.WatchFiles`、`mygo.Protocol`）、标准库 `net/http` + `encoding/json`。无第三方 markdown、无前端构建、无 bun。

**规格：** `docs/superpowers/specs/2026-11-21-markdown-preview-design.md`（执行者两份都读）

## 全局约束

- Go 模块名 `mdpreview`；`go.mod` 中 `require github.com/egoist/mygo v0.9.1`。
- 不引入任何第三方 markdown / GFM / 语法高亮 / web 框架依赖。
- 不写平台专用代码（不加 `//go:build windows` 分支），三平台可编译。
- 测试一律用 `t.TempDir()`；任何测试都不得读写真实用户目录（必要时 `mygo.App.SetPath(mygo.PathUserData, t.TempDir())`）。
- `internal/workspace` 与 `internal/mcp` 不得 import `github.com/egoist/mygo/ui`（只有 `ui/markdown/parser` 可以）。
- 配置与状态都在 `<PathUserData>` 下的 JSON 文件；不得使用 `localStorage`、不得用 `os.Getenv` 存状态。
- MCP server 默认不启动；只监听 `127.0.0.1`；无 `Authorization: Bearer <token>` 一律 401。
- 提交信息用中文 `feat:` / `test:` / `fix:` 前缀。

## 文件结构

| 文件 | 职责 |
| --- | --- |
| `main.go` | 组装：配置 → workspace → view state → 注册协议 → 绑 MCP → 开窗口 → 启动 watch |
| `mygo.json` | app 元数据（name / slug / identifier / version / icon） |
| `resources/icon.png` | 1024×1024 占位图标 |
| `internal/config/config.go` | `Config` 结构、Load/Save（原子写）、项目增删、active 切换 |
| `internal/config/config_test.go` | 上面这些的测试 |
| `internal/workspace/project.go` | `Project`、`Workspace`（项目集合、活动项目、增删） |
| `internal/workspace/scan.go` | `ListDocs`：递归扫描 + 忽略规则 + 排序 + `Tree` 结构 |
| `internal/workspace/doc.go` | `Doc`、`ReadMarkdown`（大小上限、UTF-8 规范化）、读取缓存 |
| `internal/workspace/outline.go` | `Heading`、`Slug`、`Parse`（解析缓存）、`Search` |
| `internal/workspace/index.go` | `Index` 接口 + 编译期断言 `var _ Index = (*Workspace)(nil)` |
| `internal/workspace/*_test.go` | 扫描 / slug / 搜索 / 缓存失效测试 |
| `internal/watch/watch.go` | `Watch`：把 `mygo.FileWatcher` 折叠成 `ChangeSet` 通道，可注入假源 |
| `internal/watch/watch_test.go` | 折叠与跟随重命名的测试 |
| `internal/mcp/jsonrpc.go` | JSON-RPC 2.0 请求/响应/错误码编解码 |
| `internal/mcp/server.go` | `Server`：HTTP 端点、鉴权、initialize / tools/list / tools/call 分发、启停 |
| `internal/mcp/tools.go` | 4 个只读 tool 的 schema 与实现（只依赖 `Index`） |
| `internal/mcp/server_test.go` | `httptest` 握手、401、-32601、-32602、四个 tool 的正例 |
| `internal/view/state.go` | `State`：项目、树、展开集合、当前文档、渲染模式、缓存、锁 |
| `internal/view/view.go` | `View(c)`：工具条 + 主体 Split + 状态栏 |
| `internal/view/toolbar.go` | 项目下拉、打开项目、刷新 |
| `internal/view/tree.go` | 侧栏文件树 |
| `internal/view/toc.go` | 文档目录（Heading 列表 → 跳转） |
| `internal/view/preview.go` | 预览容器：按 `renderMode` 分派 |
| `internal/view/native.go` | 原生渲染 + `markdown.Options` 装配 + 图片 / 链接 |
| `internal/view/webview.go` | webview 渲染 + `mdview://` 协议处理器 |
| `internal/view/view_test.go` | 无窗口 tester 断言树 / TOC / 分段控件 |

## 审查重点（Review Focus）

1. 项目根目录被删除、移动或变成文件 → 应用不崩，该项目标记失效并切到下一个可用项目；测试：删掉 root 后 `ListDocs` 返回 `ErrProjectGone`，`view` 层切到空状态。
2. `.md` 文件超过 8 MB 或含非法 UTF-8 / 二进制内容 → 不 OOM、不 panic；测试：写入 9 MB 文件与含 `0x00` 的文件，断言 `ReadMarkdown` 返回 `ErrTooLarge`、非法字节被替换为 U+FFFD 后仍可 `Parse`。
3. 文件名含中文、空格、`#`、`%`、`?`、`+` → 树能显示、`mdview://` 能取到、TOC 能跳；测试：扫一个含 `方案 #1 50%.md` 和 `空格 与+加号.md` 的目录，断言路径往返 `url.QueryEscape` 后仍相等。
4. 符号链接环（`a/b -> a`）或超深目录 → 扫描终止、不重复列项；测试：造自引用符号链接（Windows 跳过 `os.Symlink` 失败），断言 `ListDocs` 在 5s 内返回且每个路径只出现一次。
5. 空文档、只有注释、只有 HTML、只有代码块的无标题文件 → TOC 为空时不崩、正文照常显示、`Title` 回落到文件名；测试：`Parse` 空串、单代码块文档，断言 `Outline` 长度为 0 且 `Doc.Title == base name`。

---

## 任务 1：仓库骨架与空窗口

**文件：**
- 创建：`go.mod`、`mygo.json`、`resources/icon.png`、`.gitignore`、`main.go`
- 测试：无（手动验收）

- [ ] **步骤 1：初始化仓库**

```bash
git init
printf '.mygo/\ndist/\nbuild/\n' > .gitignore
go mod init mdpreview
go get github.com/egoist/mygo@v0.9.1
```

- [ ] **步骤 2：写 `mygo.json`**

字段：`name: "Markdown Preview"`、`slug: "markdown-preview"`、`identifier: "dev.mdpreview.app"`、`version: "0.1.0"`、`icon: "resources/icon.png"`、`fileAssociations: [{ext:["md","markdown"], name:"Markdown Document", mimeType:"text/markdown"}]`。

- [ ] **步骤 3：放占位图标**

`resources/icon.png` 放一个 1024×1024 的纯色 PNG（可用 `git` 之外的任意方式生成；文件只需存在且可解码）。

- [ ] **步骤 4：写 `main.go` 最小窗口**

```go
func main() {
	mygo.App.WhenReady(func() {
		mygo.NewWindow(mygo.WindowOptions{
			Title:  "Markdown Preview",
			Width:  1280,
			Height: 820,
			Content: ui.View(func(c *ui.Context) {
				ui.Text(c, "Markdown Preview")
			}),
		})
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
```

- [ ] **步骤 5：验证编译（会触发工具链下载）**

运行：`go build ./... && go vet ./...`
预期：通过。若报 `go.mod requires go >= 1.27.3`，运行 `go env -w GOTOOLCHAIN=auto` 后重试；仍失败则停下并报告"需要升级本机 Go"。

- [ ] **步骤 6：手动启动**

运行：`go run .`
预期：出现 1280×820 空窗口，标题 "Markdown Preview"。关闭。

- [ ] **步骤 7：Commit**

```bash
git add -A && git commit -m "feat: 空窗口骨架与 mygo 配置"
```

---

## 任务 2：配置持久化

**文件：**
- 创建：`internal/config/config.go`、`internal/config/config_test.go`

**Interfaces（后续任务依赖，签名固定）：**

```go
type Project struct{ ID, Name, Root string }        // Root 为绝对路径
type RenderMode string                               // "native" | "webview"
type Config struct {
	Version  int        `json:"version"`
	Projects []Project  `json:"projects"`
	ActiveID string     `json:"active_id"`
	Render   RenderMode `json:"render"`
}
func Load(dir string) (Config, error)               // 文件不存在 → 默认值，不报错
func (c Config) Save(dir string) error              // 原子写：Config.json.tmp + rename
func ProjectID(root string) string                  // 绝对路径的稳定短哈希
func (c *Config) AddProject(root, name string) (Project, bool) // 已存在则 false
func (c *Config) RemoveProject(id string) bool
func (c *Config) SetActive(id string) bool
```

- [ ] **步骤 1：写失败测试**

```go
func TestLoadMissingIsDefault(t *testing.T)      // Load(空目录) → Version==1, Projects 为空, Render=="native"
func TestSaveLoadRoundTrip(t *testing.T)         // 加两个项目 + 设 active → Save → Load → 相等
func TestSaveIsAtomic(t *testing.T)              // Save 后目录里没有 config.json.tmp 残留
func TestAddProjectDedupes(t *testing.T)         // 同一 root 加两次 → 第二次 ok==false，列表仍 1 项
func TestProjectIDStable(t *testing.T)           // 同一路径两次调用相等；不同路径不等
```

- [ ] **步骤 2：运行测试验证失败**

运行：`go test ./internal/config/ -v`
预期：FAIL，`undefined: Load`。

- [ ] **步骤 3：实现 `config.go`**

要点：`Load` 对 `json.Unmarshal` 的未知字段宽容（不 `DisallowUnknownFields`）；`Render` 为空时补 `"native"`；`ProjectID` 用 `sha256(filepath.Clean(abs))[:8]` 十六进制；`Save` 先 `os.MkdirAll(dir, 0o755)` 再写临时文件、`f.Sync()`、`os.Rename`。

- [ ] **步骤 4：运行测试验证通过**

运行：`go test ./internal/config/ -v`
预期：PASS（5 个用例）。

- [ ] **步骤 5：Commit**

```bash
git add internal/config && git commit -m "feat: 项目配置的加载、保存与增删"
```

---

## 任务 3：项目扫描与文件树

**文件：**
- 创建：`internal/workspace/project.go`、`internal/workspace/scan.go`、`internal/workspace/index.go`、`internal/workspace/scan_test.go`

**Interfaces：**

```go
var ErrProjectGone = errors.New("workspace: project directory is gone")

type Project = config.Project
type Doc struct {
	ProjectID string    `json:"project_id"`
	Path      string    `json:"path"`      // 相对 root，/ 分隔
	Title     string    `json:"title"`     // 见任务 4；本任务先填文件名（去后缀）
	ModTime   time.Time `json:"mod_time"`
	Size      int64     `json:"size"`
}
type Node struct {
	Name     string  `json:"name"`
	Path     string  `json:"path"`        // 目录也行，相对 root
	IsDir    bool    `json:"is_dir"`
	Children []*Node `json:"children,omitempty"`
}
func Ignored(rel string, isDir bool) bool          // 扫描与监听共用
func IsMarkdown(name string) bool                  // .md / .markdown，大小写不敏感
func ListDocs(root string) ([]Doc, error)          // 按路径字典序；root 不存在 → ErrProjectGone
func BuildTree(root string) (*Node, error)         // 目录在前、按名排序、只含 md 叶子与"含 md 的目录"
```

- [ ] **步骤 1：写失败测试**

```go
func TestIgnoredRules(t *testing.T)        // ".git"、".obsidian"、"node_modules"、"vendor"、"dist"、"build"、".mygo" → true；"docs"、"notes.md" → false
func TestIsMarkdown(t *testing.T)          // "a.md"、"A.MD"、"a.markdown" → true；"a.txt"、"a.md.bak" → false
func TestListDocsRecursive(t *testing.T)   // 造 notes/a.md、notes/b.markdown、.git/x.md、node_modules/y.md、readme.txt → 恰好 2 项，路径为 "notes/a.md"、"notes/b.markdown"
func TestListDocsSortsDirsFirst(t *testing.T) // BuildTree：根下 "docs" 目录在 "readme.md" 前
func TestListDocsProjectGone(t *testing.T) // root 删掉 → ErrProjectGone
func TestListDocsSymlinkLoop(t *testing.T) // 见审查重点 4
```

- [ ] **步骤 2：运行测试验证失败**

运行：`go test ./internal/workspace/ -v`
预期：FAIL，`undefined: ListDocs`。

- [ ] **步骤 3：实现扫描**

要点：`filepath.WalkDir`；对每个目录先问 `Ignored`，true 则 `fs.SkipDir`；对文件问 `Ignored` + `IsMarkdown`；`Path` 一律 `filepath.ToSlash(strings.TrimPrefix(p, root))` 去前导 `/`；`Root` 先 `filepath.Abs`；用 `map[string]struct{}` 记录已访问的 `EvalSymlinks` 结果防环（失败则按原路径记）；`Title` 用 `strings.TrimSuffix(name, ext)`。`BuildTree` 只保留"子树里至少有一个 md"的目录。

- [ ] **步骤 4：运行测试验证通过**

运行：`go test ./internal/workspace/ -v`
预期：PASS。

- [ ] **步骤 5：Commit**

```bash
git add internal/workspace && git commit -m "feat: md 文件递归扫描、忽略规则与目录树"
```

---

## 任务 4：解析、TOC 与搜索

**文件：**
- 创建：`internal/workspace/doc.go`、`internal/workspace/outline.go`、`internal/workspace/doc_test.go`、`internal/workspace/outline_test.go`
- 修改：`internal/workspace/index.go`（补接口）

**Interfaces：**

```go
var ErrTooLarge = errors.New("workspace: document is too large")
const MaxDocBytes = 8 << 20

type Heading struct{ Level int; Text, Slug string }
type Hit struct {
	Path string `json:"path"`
	Line int    `json:"line"`   // 从 1 开始
	Text string `json:"text"`
}

type Index interface {
	Projects() []Project
	ListDocs(projectID string) ([]Doc, error)
	ReadMarkdown(projectID, path string) (string, error)
	Parse(projectID, path string) (*parser.Document, []Heading, error)
	Search(projectID, query string, limit int) ([]Hit, error)
}
```

> 说明：`Index` 里只出现上面 5 个方法；`Parse` 返回的 `*parser.Document` 是 `github.com/egoist/mygo/ui/markdown/parser` 的类型。

- [ ] **步骤 1：写失败测试**

```go
func TestSlug(t *testing.T)          // "Hello World" → "hello-world"；"导出流程" → "导出流程"；"a  b" → "a-b"；"C++ Notes" → "c-notes"
func TestSlugDedupes(t *testing.T)   // 三个同名 heading → slug / slug-1 / slug-2
func TestOutlineLevels(t *testing.T) // "# A\n\n### C\n\nSetext\n===" → [{1,A},{3,C},{1,Setext}]
func TestOutlineSkipsCodeFence(t *testing.T) // 代码块里的 "# not a heading" 不出现在 TOC
func TestReadMarkdownTooLarge(t *testing.T)  // 9MB 文件 → ErrTooLarge
func TestReadMarkdownInvalidUTF8(t *testing.T) // 0x00 0xff → 不报错，含 U+FFFD
func TestParseCachesByModTime(t *testing.T)  // 连调两次 Parse，第二次不重新解析（用包内计数钩子断言）
func TestSearchFindsLineNumbers(t *testing.T) // 大小写不敏感、跳过忽略目录、limit 生效、空 query → 空结果
func TestTitleFallsBackToFilename(t *testing.T) // 无 H1 时 Doc.Title == 去后缀文件名
```

- [ ] **步骤 2：运行测试验证失败**

运行：`go test ./internal/workspace/ -run 'Slug|Outline|Read|Parse|Search|Title' -v`
预期：FAIL，`undefined: Slug`。

- [ ] **步骤 3：实现**

要点：`Slug` 用 `unicode.ToLower` 逐 rune：字母/数字/CJK 保留，空白合并成 `-`，其余标点丢弃；去首尾 `-`。`Parse` 用 `parser.Parse(src)`，遍历 `doc.Children()` 找 `parser.Heading` 块，文本用 `parser.PlainText(inlines)`；缓存键 `(projectID, path)`，值 `{mtime, size, src, doc, headings}`，`.md` 变化时重算。`ReadMarkdown` 用 `os.Stat` 先查 `MaxDocBytes`，再 `os.ReadFile`，非法 UTF-8 用 `strings.ToValidUTF8(s, "\uFFFD")`。`Search` 逐文件 `ReadMarkdown` + 按行 `strings.Contains(strings.ToLower(...))`，命中数达 `limit` 即停（`limit<=0` 视为 50）。

- [ ] **步骤 4：运行测试验证通过**

运行：`go test ./internal/workspace/ -v`
预期：PASS。

- [ ] **步骤 5：Commit**

```bash
git add internal/workspace && git commit -m "feat: 文档读取、解析缓存、TOC 与搜索"
```

---

## 任务 5：界面外壳（项目 + 文件树）

**文件：**
- 创建：`internal/view/state.go`、`internal/view/view.go`、`internal/view/toolbar.go`、`internal/view/tree.go`、`internal/view/preview.go`、`internal/view/view_test.go`
- 修改：`main.go`

**Interfaces：**

```go
type State struct {
	ws      *workspace.Workspace   // 任务 6 起用到；本任务只需要项目与树
	cfg     config.Config
	cfgDir  string
	win     *mygo.Window
	mu      sync.Mutex

	activeID  string
	tree      *workspace.Node
	expanded  map[string]bool      // key: 项目内相对目录路径
	selected  string               // 当前文档相对路径，"" 表示未选
	sidebarW  float32              // 默认 300
	status    string
}
func New(ws *workspace.Workspace, cfg config.Config, dir string) *State
func (s *State) View(c *ui.Context)   // 整窗 view
func (s *State) OpenProject()         // Dialog.Open{Directory:true} → ws 添加 + cfg.Save + 重扫
func (s *State) SelectProject(id string)
func (s *State) Refresh()             // 重扫树
func (s *State) selectDoc(rel string)
```

- [ ] **步骤 1：写失败测试（无窗口 tester）**

```go
func TestViewShowsEmptyState(t *testing.T)     // 无项目：树上无 md 项，出现 "打开项目" 文案
func TestTreeListsMarkdown(t *testing.T)       // 造 2 个项目 + 项目内 notes/a.md：树有 "notes"、"a.md"
func TestTreeKeepsExpandedAcrossRefresh(t *testing.T) // 展开 "notes" → Refresh() → 仍展开
func TestToolbarShowsProjectNames(t *testing.T) // 两个项目 → 下拉项含两个名字
func TestRemoveProjectFallsBackToNext(t *testing.T) // 移除活动项目 → activeID 变成剩下的那个，树跟着换
func TestGoneProjectIsDropped(t *testing.T)          // root 被删 → Refresh() 把该项目标失效并切到下一个可用项目；全失效 → 空状态
```

- [ ] **步骤 2：运行测试验证失败**

运行：`go test ./internal/view/ -v`
预期：FAIL，`undefined: New`。

- [ ] **步骤 3：实现布局**

顶部：`ui.Row` 里 `ui.Select(c, &s.activeID, names)`（或 `ui.PopoverMenu` 若 Select 不满足"显示名"需求）+ `ui.Button(c, "打开项目…")` + `ui.Button(c, "刷新")`。
主体：`ui.Split(c, &s.sidebarW, s.sidebar, s.preview).Fill()`，侧栏用 `ui.Tree` 递归 `workspace.Node`，`ui.TreeItem(c, n.Name, open, children).Selected(n.Path==s.selected).Clicked()`。
底部留给任务 7。`OpenProject` 用 `mygo.Dialog.Open(mygo.OpenDialogOptions{Directory: true, Title: "打开项目目录"})`，取 `paths[0]`。
所有 state 变更都在 `s.mu` 内，然后 `s.win.Invalidate()`。项目下拉里每一项带「移除项目」（`ui.PopoverMenu` 的菜单项，调用 `s.ws.Remove(cfg)` + `cfg.Save` + 取消该项目的 watch，若移除的是活动项目则 `activeID` 切到列表里第一个，列表空则回空状态）。`Refresh()` 遇到 `workspace.ErrProjectGone` 时删除该配置项并切到下一个可用项目。

- [ ] **步骤 4：运行测试验证通过**

运行：`go test ./internal/view/ -v`
预期：PASS。

- [ ] **步骤 5：接线 `main.go` 并手动验收**

`main.go`：`cfg, _ := config.Load(dir)` → `ws := workspace.New(cfg)` → `s := view.New(ws, cfg, dir)` → `WhenReady` 里 `s.win = mygo.NewWindow(... Content: ui.View(s.View))` → `mygo.App.Run()`。
运行 `go run .`，点「打开项目…」选一个含 md 的目录：树出现、目录可折叠、点文件侧栏高亮。

- [ ] **步骤 6：Commit**

```bash
git add -A && git commit -m "feat: 项目切换、打开项目与 md 文件树"
```

---

## 任务 6：原生预览与 TOC 联动

**文件：**
- 创建：`internal/view/native.go`、`internal/view/toc.go`
- 修改：`internal/view/preview.go`、`internal/view/state.go`、`internal/view/view_test.go`

**Interfaces：**

```go
// state 新增字段
md        markdown.Options     // Theme 零值、Anchor/AnchorVersion 由 TOC 点击写入
doc       *parser.Document     // 当前文档解析结果
headings  []workspace.Heading
anchor    string
anchorVer uint64
docCache  map[string]cachedDoc // key 项目内相对路径

func (s *State) preview(c *ui.Context)  // 分派 native / webview（任务 7 补齐）
func (s *State) nativePreview(c *ui.Context)
func (s *State) toc(c *ui.Context)      // ui.Tree 列出 headings，点击 → s.gotoHeading(slug)
func (s *State) gotoHeading(slug string) // s.anchor = slug; s.anchorVer++
func (s *State) followLink(url string)   // 相对 .md 链接 → selectDoc；#slug → gotoHeading；http(s) → ui.Link
```

- [ ] **步骤 1：写失败测试**

```go
func TestTOCListsHeadings(t *testing.T)   // "# A\n## B\n### C" → TOC 三行，缩进按 level
func TestTOCClickSetAnchor(t *testing.T)  // 点第二项 → s.anchor=="b"、s.anchorVer 递增
func TestEmptyTOCDoesNotPanic(t *testing.T)// 无标题文档 → TOC 为空、正文仍显示
func TestFollowLinkOpensSiblingDoc(t *testing.T) // "other.md" → s.selected=="other.md"
func TestFollowLinkAnchor(t *testing.T)   // "#install" → anchor=="install"
```

- [ ] **步骤 2：运行测试验证失败**

运行：`go test ./internal/view/ -run 'TOC|Follow' -v`
预期：FAIL。

- [ ] **步骤 3：实现原生预览**

`md.Options` 装配：`Anchor: s.anchor`、`AnchorVersion: s.anchorVer`、`OnLink: s.followLink`、
`LoadImage: markdown.FileImages(filepath.Dir(absPath))`、`Extensions: parser.Default`、
`Theme: nil`（跟随窗口主题）。正文：`markdown.View(c, src, &s.md)`，外面套 `ui.Scroll(c, ...)`（按 `docs/ui/scroll.md` 的滚动容器写法）。
TOC 用 `ui.Tree` 的叶子项，缩进按 `(level-1)*12` DIP。`ui.Split(c, &s.tocW, s.toc, s.body).Fill()`，`tocW` 默认 240。
`doc == nil`（文件读失败/被删）时正文位置显示 `s.status` 文案而不是 `markdown.View`。

- [ ] **步骤 4：运行测试验证通过**

运行：`go test ./internal/view/ -v`
预期：PASS。

- [ ] **步骤 5：手动验收**

`go run .` 打开一个多级标题的 md：TOC 出现；点 TOC 第 3 项正文滚到该标题；点正文里指向 `other.md` 的链接会切换文档；相对路径图片能显示。

- [ ] **步骤 6：Commit**

```bash
git add -A && git commit -m "feat: 原生 markdown 预览与 TOC 跳转"
```

---

## 任务 7：webview 预览与底部切换

**文件：**
- 创建：`internal/view/webview.go`
- 修改：`internal/view/preview.go`、`internal/view/state.go`、`internal/view/view.go`、`main.go`

**Interfaces：**

```go
// state 新增
renderMode map[string]config.RenderMode // key 项目内相对路径；缺省 cfg.Render
wv         *mygo.WebView                // 懒创建，nil 表示还没建
wvURL      string                       // 上次加载的 URL，避免重复 LoadURL
webOK      bool                         // 运行环境是否支持 webview

func (s *State) webPreview(c *ui.Context)
func (s *State) ensureWebView() bool                       // 首次调用 win.NewWebView
func (s *State) loadWebDoc(absPath, slug string, rev uint64)
func docHTML(baseDir, src string, rev uint64) string       // parser.Parse(src).HTML() + <style> 模板
func handleMDView(w io.Writer, r *http.Request)            // mdview:// 的 /doc 与 /img
```

- [ ] **步骤 1：写失败测试**

```go
func TestDocHTMLEscapesAndAnchors(t *testing.T) // "# A <b>x</b>" → 含 `<h1 id="a">`、不含裸 `<b>x</b>` 逃逸问题
func TestDocHTMLTables(t *testing.T)            // GFM 表格 → `<table>`
func TestMDViewForbidsOutsideRoot(t *testing.T) // p=../../etc/passwd → 403
func TestMDViewServesImage(t *testing.T)        // 项目内 png → 200 + image/png
func TestSegmentedSwitchRemembersPerDoc(t *testing.T) // 文档 A 切 webview，切到 B 仍是 native，切回 A 是 webview
```

- [ ] **步骤 2：运行测试验证失败**

运行：`go test ./internal/view/ -run 'DocHTML|MDView|Segmented' -v`
预期：FAIL。

- [ ] **步骤 3：实现**

- 协议：`mygo.Protocol.HandleFunc("mdview", ...)`，路由 `/doc?p=&rev=`、`/img?p=`；`p` 为绝对路径，先 `filepath.Abs` + `filepath.EvalSymlinks`，再断言落在某个已打开项目 root 之内，否则 403。
- HTML 模板：`<style>` 内嵌（系统字体栈、`prefers-color-scheme` 亮暗、表格边框、代码块等宽滚动、`max-width: 46rem`）。
- 懒创建：`s.wv, err = s.win.NewWebView(mygo.WebViewOptions{URL: url})`；`webOK` 由 `err != nil` 决定。创建后视图只 `ui.WebView(c, s.wv).Grow(1)`。
- 切换：底部 `ui.Segmented(c, &idx, "原生", "WebView")`，`idx` 由 `s.renderMode[selected]` 映射；`!s.webOK` 时把第二段文本改为 `"WebView（不可用）"` 并在 `s.status` 说明。
- 文档变化：`loadWebDoc` 只有 `url` 与 `s.wvURL` 不同才 `LoadURL`（rev 递增触发重载）。

- [ ] **步骤 4：运行测试验证通过**

运行：`go test ./internal/view/ -v`
预期：PASS。

- [ ] **步骤 5：手动验收**

同一文档在两种模式下切换：表格 / 代码块 / 图片都正常；webview 模式下点 TOC 能滚到对应标题（`#slug`）；切回原生滚动位置不必保留（记录在案）。

- [ ] **步骤 6：Commit**

```bash
git add -A && git commit -m "feat: webview 渲染后端与底部模式切换"
```

---

## 任务 8：文件监听与自动重载

**文件：**
- 创建：`internal/watch/watch.go`、`internal/watch/watch_test.go`
- 修改：`internal/view/state.go`、`main.go`

**Interfaces：**

```go
type Change struct {
	Op   mygo.FileOp // 复用 mygo 的操作枚举
	Path string      // 相对 root，/ 分隔
	Old  string      // 重命名的来源
}
type ChangeSet struct {
	Docs []Change  // 与 md 相关（或无法判断时的全部事件）
	TreeDirty bool // 有目录增删/重命名
}
type Source interface { Next() (ChangeSet, error); Close() error }
func Watch(ctx context.Context, root string) (Source, error)   // 包 mygo.WatchFiles，Ignore=workspace.Ignored, Recursive=true
func Fold(events []mygo.FileEvent) ChangeSet                    // 纯函数，便于测试
```

- [ ] **步骤 1：写失败测试**

```go
func TestFoldWriteMarksDoc(t *testing.T)      // {FileWrite,"notes/a.md"} → Docs 1 项，TreeDirty false
func TestFoldCreateMDMarksTreeDirty(t *testing.T) // {FileCreate,"notes/new.md"} → TreeDirty true
func TestFoldRemoveDirMarksTreeDirty(t *testing.T) // {FileRemove,"notes",IsDir:true} → TreeDirty true
func TestFoldRenameCarriesOld(t *testing.T)   // {FileRename,"notes/b.md",OldPath:"notes/a.md"} → Old=="notes/a.md"
func TestFoldIgnoresNonMD(t *testing.T)       // {FileWrite,"readme.txt"} → 空 ChangeSet
func TestWatchSeesExternalWrite(t *testing.T) // t.TempDir 起 Watch，写入 a.md，2s 内收到含 a.md 的 ChangeSet
```

- [ ] **步骤 2：运行测试验证失败**

运行：`go test ./internal/watch/ -v`
预期：FAIL，`undefined: Fold`。

- [ ] **步骤 3：实现**

`Watch` 在 `mygo.App.WhenReady` 之后才可调用（`WatchFiles` 需要 app 运行）；测试里用 `mygo.App.Run` 或跳过需要 app 的用例并标注。`view` 侧：每个已打开项目一个 goroutine，循环 `Next()` → `s.applyChanges(id, set)`：`TreeDirty` 则 `Refresh()`；`Docs` 里若 `Path == s.selected` → 重读并重建缓存 + 重载 webview；`Old == s.selected` → `s.selected = Path`（跟随重命名）。结束后 `win.Invalidate()`。`ctx` 由项目移除或应用退出取消。

- [ ] **步骤 4：运行测试验证通过**

运行：`go test ./internal/watch/ -v`
预期：PASS（`TestWatchSeesExternalWrite` 若无 app 运行环境则在该机跳过并保留 `t.Skip` 说明）。

- [ ] **步骤 5：手动验收（关键场景）**

1. 应用开着，用外部命令 `printf '\n## 新章节\n' >> notes/a.md`：正文几秒内出现新章节，TOC 多一行。
2. `mv notes/a.md notes/b.md`：树显示 `b.md`，正文仍打开该文档（跟随）。
3. `rm notes/b.md`：树移除该项，正文显示"文件已被删除"。
4. 新增 `notes/new.md`：树立刻出现该文件。

- [ ] **步骤 6：Commit**

```bash
git add -A && git commit -m "feat: 文件监听与外部变更自动重载"
```

---

## 任务 9：MCP server 骨架

**文件：**
- 创建：`internal/mcp/jsonrpc.go`、`internal/mcp/server.go`、`internal/mcp/tools.go`、`internal/mcp/server_test.go`、`internal/view/settings.go`
- 修改：`main.go`、`internal/view/view.go`、`internal/config/config.go`、`internal/config/config_test.go`

**Interfaces：**

```go
type Options struct {
	Index  workspace.Index
	Addr   string        // 默认 "127.0.0.1:8765"
	Token  string        // 空则 New 生成随机 token
	Log    io.Writer     // nil 则 io.Discard
}
type Server struct{ /* ... */ }
func New(o Options) (*Server, error)
func (s *Server) Start() (addr string, err error)   // 监听失败由调用方回退到 ":0"
func (s *Server) Close() error
func (s *Server) Handler() http.Handler             // 便于 httptest

// 配置新增字段（config.Config）
type MCPConfig struct {
	Enabled bool   `json:"enabled"`
	Port    int    `json:"port"`
	Token   string `json:"token"`
}
```

- [ ] **步骤 1：写失败测试**

```go
func TestInitializeHandshake(t *testing.T)     // POST /mcp {"method":"initialize"} → result.protocolVersion 非空、capabilities.tools 存在、serverInfo.name=="markdown-preview"
func TestToolsListHasFourReadOnlyTools(t *testing.T) // 恰好 list_projects/list_docs/read_doc/search_docs，每个 inputSchema.type=="object" 且无 destructiveHint
func TestToolsCallListDocs(t *testing.T)       // 假 Index 返回 1 个 Doc → content[0].text 里含该路径
func TestUnknownMethodIsMethodNotFound(t *testing.T) // -32601
func TestUnknownToolIsInvalidParams(t *testing.T)    // -32602
func TestMissingTokenIs401(t *testing.T)       // 无 Authorization → 401
func TestGetOnEndpointIs405(t *testing.T)
```

- [ ] **步骤 2：运行测试验证失败**

运行：`go test ./internal/mcp/ -v`
预期：FAIL，`undefined: New`。

- [ ] **步骤 3：实现**

JSON-RPC：请求 `{jsonrpc,id,method,params}`；响应 `{jsonrpc:"2.0",id,result}` 或 `{jsonrpc:"2.0",id,error:{code,message}}`；通知（无 `id`）回 202 且空体。
`tools/call` 的 result 为 `{content:[{type:"text",text:"<json>"}], isError:bool}`。
鉴权中间件：`Authorization` 必须等于 `"Bearer "+token`。日志：每行 `method`, `id`, `duration`, `status` 写 `Log`。
启动：`main.go` 读 `cfg.MCP`，`Enabled` 才 `Start()`；启动失败（端口占用）→ 用 `127.0.0.1:0` 重试并把实际端口写回 `<PathUserData>/mcp.json`。
设置入口：`internal/view/settings.go` 在工具条加一个「MCP」按钮，弹出 `ui.AlertDialog`：开关（`ui.Switch`）、当前地址、token（可复制）、"在 agent 里怎么配"的说明文字。

- [ ] **步骤 4：运行测试验证通过**

运行：`go test ./internal/mcp/ -v`
预期：PASS。

- [ ] **步骤 5：手动验收**

开启 MCP，`curl -s http://127.0.0.1:8765/mcp -H "Authorization: Bearer <token>" -H 'content-type: application/json' -d '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"list_docs","arguments":{}}}'`：返回当前项目的 md 列表。关掉开关后同一命令连接失败。

- [ ] **步骤 6：Commit**

```bash
git add -A && git commit -m "feat: 本地 MCP server 骨架与只读文档工具"
```

---

## 任务 10：收尾、文档与全量验收

**文件：**
- 创建：`README.md`
- 修改：各处注释；`docs/superpowers/specs/2026-11-21-markdown-preview-design.md` 若与实现不符则回改

- [ ] **步骤 1：README**

写清：怎么 `go run .`、怎么加项目、两种渲染模式的区别与限制、MCP 配置示例（JSON 片段，指向 `127.0.0.1:8765/mcp` + Bearer token）、已知限制（无 scroll spy、无代码高亮、只读、8 MB 上限）。

- [ ] **步骤 2：格式与静态检查**

```bash
gofmt -l .    # 预期无输出
go vet ./...  # 预期无输出
go test ./... # 预期全部 PASS
```

- [ ] **步骤 3：跑一遍审查重点里的 5 条**

逐条按 `## 审查重点` 的描述手工复现一次（删除 root、9 MB 文件、中文带 `#%?+` 文件名、符号链接环、空文档），确认行为与写下的期望一致。有不一致的，回到对应任务修正并补测试。

- [ ] **步骤 4：Commit**

```bash
git add -A && git commit -m "docs: README 与限制说明"
```

---

## 执行交接

任务之间有接口依赖（`Index`、`ChangeSet`、`State` 字段），但每个任务的测试都是自洽的。建议按任务顺序实现；任务 7（webview）与任务 9（MCP）互不依赖，可并行。
