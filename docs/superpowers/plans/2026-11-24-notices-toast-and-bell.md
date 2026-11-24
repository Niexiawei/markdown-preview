# 底部状态栏改为 Toast + 侧栏铃铛消息历史 实现计划

> **面向 AI 代理的工作者：** 必需子技能：使用 subagent-driven-development（推荐）或 executing-plans 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** 删掉窗口底部的 24 DIP 信息栏，让正文吃满这段高度；窗口的提示（复制成功、文件被删、WebView 不可用……）
改为**右下角** Toast 弹出；左侧工具栏底部新增一个铃铛按钮，未读时挂一个红色数字角标（最多 `99`，超过显示 `99+`），
点开浮层看历史消息，可单条删除、可清空全部；历史**不设上限**并**落盘**，下次启动原样恢复（未读也恢复）。

**架构：** 提示从「一个字符串 `State.status`」变成一份**消息账本**，落在新的纯逻辑包 `internal/notices`
（`notices.json`，原子写，无 ui 依赖、可无窗口测试）：`view` 的每次提示调 `notify*` → 账本 `Add` 并写盘；
构建期由 `updateToasts` 把**还没发过**的消息交给 mygo 的 `c.AddToast`（onevra 的 `components.Toasts` 由 shell 画，
`main.go` 只提供落位）；铃铛的红点数字与浮层列表都读同一份账本。底部那一行整个删除（`statusBar` 函数消失），
正文靠 `preview` 的 `Grow(1)` 自动占满。

**技术栈：** Go 1.27+、`github.com/egoist/mygo v0.9.1`（`ui.Context.AddToast`、`ui.Toast`、`ui.Popover`、`ui.Box` +
`Absolute` / `Top` / `Right` / `IgnoreInput`、`ui.Scroll` + `MaxHeight`、`ui.NewTester` 的 `Click` / `HasText` / `Texts`）、
`github.com/oneo-me/onevra/source v0.2026.18`（`components.ToastOptions` / `ToastPlacement`、`components.Badge` +
`BadgeDanger`、`components.IconButton`、`components.Icon`、`components.Text`、`lucide.Bell` / `lucide.X` /
`lucide.Info` / `lucide.CircleCheck` / `lucide.TriangleAlert` / `lucide.CircleX`）。**不引入新依赖**（落盘用 `encoding/json`
+ `os.CreateTemp` + `os.Rename`，与 `internal/config` 同一套原子写）。

**规格：** `docs/superpowers/specs/2026-11-21-markdown-preview-design.md`。本期**推翻**该文件里
「底部状态栏只剩一行提示文字（高度 24）」这条（见文件末尾「状态栏的两次搬家」一节），执行后按
「文档回写」任务在该文件末尾追加「### 提示改为 Toast 与铃铛历史（新增，2026-11-24）」，
并在被推翻的那条旁边注明取代它的位置。

**本期不做：** 消息分类筛选；点消息跳转到相关文档；消息按项目过滤。正文内的查找高亮仍沿用现状（见 README「已知限制」）。

## 全局约束

- 不引入第三方 markdown / GFM / 高亮 / web 框架依赖（AGENTS 硬规则）。
- 颜色只从 `AppTheme` 取：Toast 的类型图标用 `t.Accent` / `t.Success` / `t.Warning` / `t.Danger`
  （`onevra` 的 `components/toast.go` 也是这么取的）；角标底色 `t.Danger`、数字色 `t.AccentText`
  （主题里没有 `DangerText`，两种调色板都把 `AccentText` 定成白，红底白字；**不写十六进制**）。
- `internal/notices` **不得 import `.../ui` 或 onevra**：它是纯逻辑 + JSON 读写，`go test ./internal/notices/` 不需要图形环境。
- 视图状态归窗口线程：`notify*` 只在窗口线程被调用（`Post` 投递的闭包里也落在这里）；构建期只读账本、
  只写 `noticeShown`，不读盘（账本已由 `New` 载入）、不 `Post`。
- 视图是 state 的纯函数：`View` 与 `Content` 两条拼装路径都要能构建；`TestShellSlotsBuild` 不回归。
- 可交互元素必须给 `label`（无障碍名，也是测试定位的唯一办法）：铃铛是「消息」/「消息（N 条未读）」，
  单条删除是「删除消息：<消息前 12 字>」，清空是文本按钮「清空全部」；角标是装饰（`AccessHidden` + `IgnoreInput`）。
- 测试只用 `t.TempDir()`（`internal/notices` 与 `view` 都把 `t.TempDir()` 当用户数据目录），不得读写真实用户目录；
  `view.New(…, "")` 表示「无目录」，账本只在内存里、不写盘。
- 不写平台专用代码；三平台可编译。
- 每个行为改动先写失败的测试；文档回写（README / CHANGELOG / AGENTS / 规格）**并入同一个提交**。
- 提交前 `go vet ./... && gofmt -l . && go test ./...` 必须干净；中文提交信息，前缀 `feat:` / `fix:` / `docs:`。

## 设计决策（实施者按此执行，不要另立方案）

| # | 决策 | 理由 |
| --- | --- | --- |
| D1 | 消息不再是一个字符串，而是一份账本：`internal/notices` 包的 `Store`，消息类型 `notices.Message{ID int, Kind string, Text string, At time.Time, Seen bool}`，**最新在最前**，**不设条数上限**。 | 用户：「历史不设置上限 落盘 重启需要恢复带记忆功能」。上限去掉后「按 id 水位线去重」比按长度/下标去重更稳。 |
| D2 | 落盘文件 `notices.json`，与 `config.json` 同目录（`App.Path(PathUserData)`，`main.go` 已有的 `dir`）。写盘用与 `config.Save` 同一套原子写（`os.MkdirAll` + `os.CreateTemp` + `Sync` + `Rename`）。`dir == ""` 时 `Load` 返回**纯内存**账本，`Save` 是 no-op。 | 用户要求落盘；原子写避免崩在写一半时丢历史。`dir == ""` 是测试用的「无目录」状态（`newStateWith` 就是传 `""`），不加这条会在当前工作目录写文件。 |
| D3 | `ID` 单调递增、永不复用：`Load` 时 `next = max(ID)+1`（空文件为 1）。`Clear` 不重置 `next`。 | ID 同时是「这条要不要弹 Toast」的水位线，也是删除时的稳定键；清空后复用 ID 会让新消息被当成旧消息而不弹。 |
| D4 | 四个写入方法，都转调 `s.notify(kind, text)`：`notify(text)`（`components.ToastInfo`）、`notifyDone(text)`（`ToastSuccess`）、`notifyWarn(text)`（`ToastWarning`）、`notifyErr(text)`（`ToastError`）。空字符串不记。 | 旧代码只有一个 `s.status = …`，全靠文案区分成败；分了类型，Toast 的图标颜色与浮层里每行的图标才有意义。 |
| D5 | `State.Status()` **保留**，语义改成「最新一条消息的文字，没有则空」。`Messages()` 转发账本。 | 现有 20 余处断言用 `s.Status()`，保留能把这 20 余处测试从「改测试」降为「不动」。 |
| D6 | Toast 只发**新**消息：`State.noticeShown` 记住最后交给 `c.AddToast` 的 `ID`；`New` 时 `noticeShown = 账本里最新的 ID`。`updateToasts` 把 `ID > noticeShown` 的按**从旧到新**逐个 `AddToast`，然后 `noticeShown = 账本最新 ID`。Toast 的 `ID` 是 `"notice-<ID>"`，标题用 `trimEllipsis(text, noticeToastText=120)`。 | 一次点击会构建两遍视图（AGENTS「一次点击会建两遍视图」），按 ID 去重才不会弹两个、也不会重复播报；`New` 里对上水位线，重启后**不会把上次的旧消息全部重播一遍**。 |
| D7 | `updateToasts` 在 `View` 与 `Content` 的**构建期**调用（`drain` 与 `handleShortcuts` 之后）。 | `c.AddToast` 只对这一帧的 Context 有效；shell 在 `Content` 之后才画 `components.Toasts`，必须赶在它前面。 |
| D8 | 未读 = 账本里 `Seen == false` 的条数（`Store.Unseen()`）；打开浮层时 `Store.MarkSeen()`（全部置真并写盘，没有变化就不写）。**面板关着**才可能涨未读。 | 用 `Seen` 而不是一个计数器：单条删除、清空、重启都不会让数字与列表对不上。重启后角标仍在，就是用户要的「带记忆」。 |
| D9 | 铃铛按钮：`components.IconButton(c, lucide.Bell(), label, components.IconButtonPlain)`，`label` 未读时为 `"消息（N 条未读）"`，未读时 `button.TextColor(t.Accent)`。有未读时在按钮右上角叠一个**红色胶囊角标**：`components.Badge(c, text, components.BadgeOptions{Tone: components.BadgeDanger})` 再 `.Background(t.Danger).TextColor(t.AccentText).Padding(0, t.Space(1)).Height(t.Space(3.5)).MinWidth(t.Space(3.5)).Justify(ui.Center).FontSize(t.Rem(.65)).Absolute().Top(-2).Right(-2).IgnoreInput(true).AccessHidden(true)`；`text` 是 `strconv.Itoa(unseen)`，`unseen > 99` 时是 `"99+"`。 | 用户：「做红色底色数字提示 最大99 超过99 显示 99+」。用 onevra 的 `Badge` 再覆盖成实心红（它本来就是胶囊 + 换字有 spring），省得自己拼一个 pill；`IgnoreInput` 让角标那十几像素的点击落回按钮，`Absolute` 才压得住角落。 |
| D10 | 角标与按钮同尺寸：`ui.Box(c).Width(t.Space(8)).Height(t.Space(8))` 包住，按钮 `.Size(t.Space(8), t.Space(8))`（默认 `Space(1)=4`，即 32 DIP，与 `IconButtonPlain` 一致）；`ui.Popover` 的锚点仍是**按钮**，不是 Box。 | 主题 `Spacing` 一变，32 就跟着变，写死 `32` 会与按钮错位；锚点用按钮才能贴着按钮弹出。 |
| D11 | 铃铛放在 `leftToolbar` 的 `ui.Spacer` 之后、MCP 之前：`打开项目 / 刷新 ……… 消息 / MCP`。 | 用户原话「左侧工具栏底部新增一个铃铛图标」。铃铛与 MCP 都是「次要、靠右」的动作，与现在 MCP 的位置感一致。 |
| D12 | 浮层用 mygo 的 `ui.Popover(c, button, &s.noticeOpen, func(){…})`（与项目面板、MCP 面板同一套），内容是一个 `ui.Column(c).Width(noticePanelWidth=360)`，列表在 `ui.Scroll(c).MaxHeight(noticePanelHeight=320)` 里。 | 项目里已有的两个浮层都是这个写法（mygo 的 `ui.Popover` 自带底色/圆角/阴影/8 DIP 内边距），第三个没理由换一套；锚点在窗口左下角，空间不够时 mygo 自动翻到上方。 |
| D13 | 每条消息一行：类型图标 + 消息全文（换行）+ 时间 + 行尾 ✕（`IconButtonCompact`）。时间 `noticeTime(at, now)`：同一天 `15:04`、同年 `01-02 15:04`、跨年 `2006-01-02 15:04`。面板顶部一行：`消息`（`.Bold().Grow(1)`）+ 有消息时「清空全部」；没有消息时只显示「暂无消息」，且**不出现**「清空全部」。 | 「可以指定删除消息也可以清空全部」；历史跨天跨重启，只给 `HH:MM` 会分不清是哪天的。空态不给清空按钮，免得点了没反应。 |
| D14 | 删除用**先拷贝再遍历**：`for _, m := range append([]notices.Message(nil), s.notes.Messages()...)`，循环体里 `s.notes.Remove(m.ID)`。 | 项目里 `tabs`、`reloadTree` 都是这个写法；边遍历边删会跳过下一条。 |
| D15 | 底部 `statusBar`（`internal/view/view.go`）与其测试 `TestStatusBarDropsTheModeSwitch` 一起删除；`activateTab` / `closeTab` / `dropTabsOf` 里三处 `s.status = ""` 一并删除。 | 「底部信息栏移除之后 preview 占据这部分空间」：`Content` 内层列去掉这一行后，`preview` 的 `ui.Split(…).Grow(1)` 自然吃掉 24 DIP。切标签不再清空历史——历史本来就该留着。 |
| D16 | Toast 落位 `components.ToastBottomEnd`（**右下角**），由 `view.ToastOptions()` 返回，`main.go` 写 `Toasts: view.ToastOptions()`。 | 用户明确要求改到右下角；放在 `view` 里是为了能单测（`main.go` 里没有窗口就测不了），改落位只改这一个函数。 |

### 改完长什么样

```
┌─────────────────────────────────────────────────────────────┐
│ 标签条： a.md  b.md  ⌄                                      │  ← shell 标题槽
├─────────────────────────────────────────────────────────────┤
│ 📄 alpha/a.md  ⧉        [原生|WebView]                      │  ← titleRow（不变）
├──────────────┬──────────────────────────────────────────────┤
│ 目录         │  正文                                        │
│  · 标题一     │  ……                                          │
│  · 标题二     │  ……  （一直画到窗口底边：没有底部信息栏了）    │
│              │                                              │
├──────────────┤                                     ┌──────────────────────┐
│ 📂 ⟳  🔔³ 🔌 │                                     │ 已复制文件路径：…     │ ← Toast（4 秒，右下角）
└──────────────┴─────────────────────────────────────└──────────────────────┘
```

铃铛点开的浮层（锚点在左下角，mygo 自动翻到上方）：

```
┌────────────────────────────────────┐
│ 消息                        清空全部 │
│ ✓ 已复制文件路径：D:\…     09:41  ✕ │
│ ⚠ 文件已被删除：notes/a.md 11-23 09:38 ✕ │
│ ! 无法打开文件管理器：…  11-21 09:12 ✕ │
└────────────────────────────────────┘
```

## 文件结构

| 文件 | 职责 |
| --- | --- |
| `internal/notices/notices.go`（新建） | 消息账本与 `notices.json`：`Message`、`Store`（`Load` / `Messages` / `Unseen` / `Last` / `Add` / `Remove` / `Clear` / `MarkSeen` / `Save`）；纯逻辑 + JSON，无 ui 依赖 |
| `internal/notices/notices_test.go`（新建） | 账本与落盘的全部行为测试 |
| `internal/view/notices.go`（新建） | 视图侧：`notify` / `notifyDone` / `notifyWarn` / `notifyErr`、`Messages` / `Status` / `RemoveMessage` / `ClearMessages`、`updateToasts`、`bellButton` / `bellBadge` / `noticePanel` / `noticeRow` / `noticeBadgeText` / `noticeTime` / `noticeIcon` / `noticeTint`、常量、`ToastOptions()` |
| `internal/view/notices_test.go`（新建） | 视图侧的全部行为测试 |
| `internal/view/state.go` | 删 `status string`，加 `notes *notices.Store` / `noticeShown` / `noticeOpen`；`New` 载入账本；`Status()` 改语义；`revealProject` / `copyProjectPath` / `RemoveProject` / `reloadTree` 的提示改 `notify*` |
| `internal/view/view.go` | 删 `statusBar` 及两处调用；`leftToolbar` 加铃铛；`View` / `Content` 调 `drain` 与 `updateToasts` |
| `internal/view/fileacts.go` / `find.go` / `native.go` / `settings.go` / `tabs.go` / `watch.go` / `webview.go` | 各自的 `s.status = …` 换成 `notify*`；`tabs.go` 三处清空删除 |
| `internal/view/layout_test.go` | 删 `TestStatusBarDropsTheModeSwitch`；新增「窗口里不再出现状态行」「`Content` 应用投递的变更」 |
| `internal/view/find_test.go` | 第 283-284 行的 `s.status` 改 `s.Status()` |
| `internal/view/shell_test.go` | `LeftToolbar` 的期望文案加「消息」 |
| `main.go` | `ShellOptions.Toasts: view.ToastOptions()`；把 `dir` 传给 view（已有） |
| `README.md` / `AGENTS.md` / `CHANGELOG.md` / 规格 | 文档回写（见任务 8） |

---

### 任务 1：`Content` 帧首补 `drain()`（先修前置 bug）

**背景（执行者必读）：** 现在只有测试用的 `View` 调 `s.drain()`（`view.go:17`），shell 用的 `Content` 没有。
`watch` 的每一处改动都是 `s.Post(fn)` 投递、等帧首 `drain` 应用，所以真实窗口里「外部改动实时跟随」这条路径
是断的，「文件已被删除」这类消息也永远到不了界面。本任务是本来就要做的一行修复，也是后面所有 watcher
消息能变成 Toast 的前提。

**文件：**
- 修改：`internal/view/view.go`（`Content`）
- 测试：`internal/view/layout_test.go`

- [ ] **步骤 1：编写失败的测试**

```go
// TestContentAppliesPostedChanges pins that the page the shell builds drains
// what other goroutines posted: the file watcher reaches the window through it.
// It posts a plain state change, so that the test outlives the message work of
// the next tasks (which replace the status field with a book).
func TestContentAppliesPostedChanges(t *testing.T) {
	s, _ := newState(t)
	s.Post(func() { s.zoom = 1.5 })
	ui.NewTester(s.Content, 800, 600)
	if s.zoom != 1.5 {
		t.Errorf("zoom = %v, want the posted change applied", s.zoom)
	}
}
```

- [ ] **步骤 2：运行测试验证失败**

运行：`go test ./internal/view/ -run TestContentAppliesPostedChanges -v`
预期：FAIL，`zoom = 0, want the posted change applied`

- [ ] **步骤 3：在 `Content` 里补一行**

`Content` 的函数体与 `View` 同序：`s.drain()` → `s.handleShortcuts(c)`（`s.updateToasts(c)` 在任务 5 才存在，
本步只加 `s.drain()`），并把函数注释里「with the status bar under them」的描述改成
「the outline and the body, filling the page」。

- [ ] **步骤 4：运行测试验证通过**

运行：`go test ./internal/view/ -run TestContentAppliesPostedChanges -v`
预期：PASS

- [ ] **步骤 5：Commit**

```bash
git add internal/view/view.go internal/view/layout_test.go
git commit -m "fix: 壳层构建页面时应用投递的变更（外部改动实时跟随）"
```

---

### 任务 2：`internal/notices` 消息账本与落盘

**文件：**
- 创建：`internal/notices/notices.go`、`internal/notices/notices_test.go`

**Interfaces（后续任务依赖这些名字）：**

```go
package notices

const (
	Version  = 1
	FileName = "notices.json"
)

// Message is one message the window reported.
type Message struct {
	ID   int       `json:"id"`
	Kind string    `json:"kind"` // "info" / "success" / "warning" / "error"
	Text string    `json:"text"`
	At   time.Time `json:"at"`
	Seen bool      `json:"seen"`
}

type Store struct {
	path string // "" keeps it in memory only
	msgs []Message // newest first
	next int
}

func Load(dir string) (*Store, error)               // no file → empty store, nil error; no directory → memory only
func (s *Store) Messages() []Message                // the store's own slice, newest first
func (s *Store) Unseen() int
func (s *Store) Last() int                          // ID of the newest message, 0 when none
func (s *Store) Add(kind, text string, at time.Time) (Message, error) // newest first; saves
func (s *Store) Remove(id int) error                // saves; unknown id saves nothing
func (s *Store) Clear() error                       // saves
func (s *Store) MarkSeen() error                    // all Seen = true; saves only when something changed
func (s *Store) Save() error                        // no-op while path == ""
```

- [ ] **步骤 1：编写失败的测试**（`internal/notices/notices_test.go`，表里每行一个测试函数）

| 测试 | 断言 |
| --- | --- |
| `TestAnEmptyStoreHasNoMessages` | `Load(t.TempDir())` 无 error，`len(Messages()) == 0`，`Last() == 0` |
| `TestMessagesSurviveARestart` | `Add` 两条后在同一个 dir 里再 `Load`：两条都在、`Text`/`Kind`/`Seen`/`At` 相等、`Messages()[0]` 是最新那条 |
| `TestNewMessagesComeFirst` | 连续 `Add("info","一")`、`("info","二")`、`("info","三")` 后 `Messages()[0].Text == "三"`、`[2].Text == "一"` |
| `TestMessagesHaveNoLimit` | `Add` 250 条后 `len(Messages()) == 250`，重新 `Load` 仍是 250 |
| `TestIDsKeepGrowingPastAClear` | `Clear()` 后 `Add` 的 `ID` 大于清空前最大 `ID`（`Last()` 一直变大） |
| `TestRemovingOneMessage` | 三条删中间：剩两条、顺序不变；`Remove(999)` 不报错也不改数量；重新 `Load` 结果相同 |
| `TestClearingEveryMessage` | `Clear()` 后空、重新 `Load` 仍空 |
| `TestUnseenCountsWhatWasNotSeen` | 三条全是未读时 `Unseen() == 3` |
| `TestMarkSeenMakesThemAllSeen` | `MarkSeen()` 后 `Unseen() == 0`，重新 `Load` 仍是 0 |
| `TestAnEmptyTextIsNotAdded` | `Add("info", "")` 后 `len(Messages()) == 0`（写入方也会挡一次，账本自己也要挡） |
| `TestAMemoryOnlyStoreKeepsWorking` | `Load("")` 后 `Add` 一条：`Messages()` 有它、`Last()` 是它的 ID、`Save()` 返回 nil（无目录时不碰磁盘） |
| `TestABrokenFileLeavesAnEmptyStore` | 目录里写 `notices.json` = `{oops`：`Load` 返回 error 且 `len(Messages()) == 0`，不 panic |

- [ ] **步骤 2：运行测试验证失败**

运行：`go test ./internal/notices/ -v`
预期：FAIL，`no Go files in …/internal/notices`（先建空包也能看到 `undefined: Load`）

- [ ] **步骤 3：实现 `internal/notices/notices.go`**

- `Load(dir)`：`dir == ""` → `&Store{}`，nil。否则读 `filepath.Join(dir, FileName)`，`os.ErrNotExist` → 空账本 + nil；
  JSON 解不开 → 空账本 + error（**不返回半份数据**，避免把坏文件写回去时放大损坏）。载入后丢掉 `Text == ""` 的条目，
  `next = max(ID)+1`。
- `Next`/`Last`/`Unseen`/`Messages` 都是读字段；`Add` 头插、`next++`、然后 `Save()`；`Remove` 过滤、`Save()`；
  `Clear` 置空切片、**不动** `next`、`Save()`；`MarkSeen` 先看有没有未读，有才改并 `Save()`。
- `Save()`：`path == ""` 直接 `return nil`；否则 `os.MkdirAll` + `json.MarshalIndent`（`version` 与 `messages` 两个字段，
  见下）+ `os.CreateTemp(dir, FileName+".*.tmp")` + `Write` + `Sync` + `Close` + `os.Rename`，与 `config.Save` 逐行一致。

文件形状：

```json
{
  "version": 1,
  "messages": [
    {"id": 12, "kind": "error", "text": "无法打开文件管理器：boom", "at": "2026-11-24T09:41:03.512+08:00", "seen": false}
  ]
}
```

- [ ] **步骤 4：运行测试验证通过**

运行：`go test ./internal/notices/ -v`
预期：PASS

- [ ] **步骤 5：Commit**

```bash
git add internal/notices
git commit -m "feat: 消息账本与 notices.json 落盘（无上限、未读状态随重启恢复）"
```

---

### 任务 3：视图状态层接上账本，并替换每一处 `s.status`

**文件：**
- 创建：`internal/view/notices.go`（本任务只写状态部分）、`internal/view/notices_test.go`
- 修改：`internal/view/state.go`、`fileacts.go`、`find.go`、`native.go`、`settings.go`、`tabs.go`、`watch.go`、`webview.go`
- 测试：`internal/view/notices_test.go`；顺带改 `internal/view/find_test.go:283-284`

**Interfaces：**

```go
func (s *State) notify(text string)      // components.ToastInfo
func (s *State) notifyDone(text string)  // components.ToastSuccess
func (s *State) notifyWarn(text string)  // components.ToastWarning
func (s *State) notifyErr(text string)   // components.ToastError
func (s *State) Messages() []notices.Message
func (s *State) Status() string          // 最新一条的文字
func (s *State) RemoveMessage(id int)
func (s *State) ClearMessages()
```

State 上的字段：`notes *notices.Store`、`noticeShown int`（任务 5 用）、`noticeOpen bool`。

- [ ] **步骤 1：编写失败的测试**（`internal/view/notices_test.go`）

| 测试 | 断言 |
| --- | --- |
| `TestTheNewestMessageIsTheStatus` | 新 `State` 的 `Status() == ""`；`notify("一")`、`notify("二")` 后 `Status() == "二"` |
| `TestMessagesKeepTheNewestFirst` | 三条之后 `Messages()[0].Text` 是最后那条、`[2]` 是第一条 |
| `TestEmptyMessagesAreNotRecorded` | `notify("")` 后 `len(Messages()) == 0` |
| `TestErrorsAreMarkedAsSuch` | `notifyErr("x")` → `Kind == components.ToastError`；`notifyDone` → `ToastSuccess`；`notifyWarn` → `ToastWarning`；`notify` → `ToastInfo` |
| `TestMessagesSurviveARestart` | 同一个 `cfgDir` 建两个 `State`（`New` 两次）：第二个的 `Messages()` 与 `Status()` 与第一个相同 |
| `TestRemovingOneMessage` | 三条删中间：剩两条、顺序不变 |
| `TestClearingEveryMessage` | `ClearMessages()` 后 `len == 0`、`Status() == ""` |
| `TestAMessageWritesThroughToTheFile` | 提示之后 `cfgDir` 里出现 `notices.json`（`os.Stat` 成功） |

- [ ] **步骤 2：运行测试验证失败**

运行：`go test ./internal/view/ -run "TestTheNewest|TestMessages|TestEmptyMessages|TestErrorsAre|TestRemoving|TestClearing|TestAMessageWrites" -v`
预期：FAIL（`undefined: s.notify`，编译失败即视为红）

- [ ] **步骤 3：`internal/view/notices.go` 写状态部分**

`notify*` 各转调 `s.notify(kind, text)`；它做：空串返回；`m, err := s.notes.Add(kind, text, time.Now())`；
`err != nil` 只字面忽略（账本已把消息放进内存，只是没写盘，与 `s.save()` 的 `_ =` 一致）。
`Messages()` 返回 `s.notes.Messages()`；`Status()` 取第一条的 `Text`；`RemoveMessage` → `s.notes.Remove`；
`ClearMessages` → `s.notes.Clear`。

- [ ] **步骤 4：`state.go` 换字段与 `New`**

删 `status string`，加 `notes *notices.Store` / `noticeShown int` / `noticeOpen bool`。
`New` 里：`store, err := notices.Load(dir); if err != nil { log? }`——**不引 log**（view 里没有日志），
直接忽略错误（坏文件已退化成空账本）；`s.notes = store`；`s.noticeShown = store.Last()`
（重启后旧消息不重播 Toast）。`Status()` 改成读账本。删掉结构体里 `status` 那行注释，补 `notes` 的说明。

- [ ] **步骤 5：替换每一处 `s.status = …`**

按下表逐行改（行号是改动前的，改完会漂移，按文案搜）：

| 位置 | 旧 | 新 |
| --- | --- | --- |
| `fileacts.go:23` / `:68` | `无法复制文件路径：` | `s.notifyErr` |
| `fileacts.go:27` / `:72` | `已复制文件路径：` | `s.notifyDone` |
| `fileacts.go:38` / `:42` | `无法复制：` | `s.notifyErr` |
| `fileacts.go:46` | `已复制文件夹：` | `s.notifyDone` |
| `fileacts.go:49` | `已复制文件：` | `s.notifyDone` |
| `fileacts.go:58` | `已复制文件名称：` | `s.notifyDone` |
| `fileacts.go:82` / `:86` | `无法打开文件管理器：` | `s.notifyErr` |
| `fileacts.go:89` | `已在文件管理器中打开：` | `s.notify` |
| `find.go:268` / `:273` / `:278` | `WebView 不可用，…` / `无法在页面中查找：` | `s.notifyErr` |
| `native.go:33` / `:39` | `err.Error()` | `s.notifyErr(err.Error())` |
| `native.go:168` | `链接的文档不存在：` | `s.notifyWarn` |
| `settings.go:66` | `token 已复制` | `s.notifyDone` |
| `state.go:187` / `:190` | `无法打开文件管理器：` / `已在文件管理器中打开：` | `s.notifyErr` / `s.notify` |
| `state.go:199` | `已复制项目路径：` | `s.notifyDone` |
| `state.go:367` | `已取消移除` | `s.notify` |
| `state.go:406` / `:410` | `项目目录不可用：` | `s.notifyErr` |
| `state.go:433` | `文件已被删除` | `s.notifyWarn` |
| `watch.go:38` | `无法监听这个项目：` | `s.notifyErr` |
| `watch.go:83` | `文件已被删除` | `s.notifyWarn` |
| `webview.go:261` | `WebView 不可用：` | `s.notifyErr` |
| `webview.go:317` | `无法在页面中滚动：` | `s.notifyErr` |
| `tabs.go:32` / `:73` / `:101` | `s.status = ""` | **整行删除** |

- [ ] **步骤 6：改掉直接读字段的测试**

`internal/view/find_test.go:283-284`：`strings.Contains(s.status, "WebView")` → `s.Status()`，报错文案里的
`s.status` 也换成 `s.Status()`。

- [ ] **步骤 7：运行全部测试**

运行：`go test ./... && go vet ./... && gofmt -l .`
预期：全部 PASS，vet 与 gofmt 无输出（漏改的地方编译器会指出来）。

- [ ] **步骤 8：Commit**

```bash
git add internal/view/notices.go internal/view/notices_test.go internal/view/*.go
git commit -m "feat: 窗口提示改走消息账本（状态层的替换，尚无 Toast 与浮层）"
```

---

### 任务 4：删掉底部信息栏

**文件：**
- 修改：`internal/view/view.go`、`internal/view/layout_test.go`
- 测试：`internal/view/layout_test.go`

- [ ] **步骤 1：编写失败的测试**

```go
// TestTheStatusLineHasLeftTheWindow pins that the bottom bar is gone: a message
// reaches the user through the toast and the bell alone.
func TestTheStatusLineHasLeftTheWindow(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "a.md", "# A\n")
	s.Refresh()
	s.selectDoc("a.md")
	s.notify("已复制文件路径：/tmp/a.md")
	tt := newTester(s)
	if tt.HasText("已复制文件路径：/tmp/a.md") {
		t.Errorf("texts = %q, want no status line in the page", tt.Texts())
	}
	if s.Status() == "" {
		t.Error("the message is gone from the state too, want it kept for the bell")
	}
}
```

- [ ] **步骤 2：运行测试验证失败**

运行：`go test ./internal/view/ -run TestTheStatusLineHasLeftTheWindow -v`
预期：FAIL（`statusBar` 仍把这段文字画进 `View`）

- [ ] **步骤 3：删除 `statusBar`**

`view.go`：删掉 `func (s *State) statusBar` 与 `View`、`Content` 里对它的两次调用；`Content` 的内层列
因此只剩 `s.preview(c)`，去掉那层 `ui.Column(c).Grow(1).Children(...)`，直接 `s.preview(c)`
（`preview` 自己的 `ui.Split(…).Grow(1)` 负责占满）。同时删掉 `layout_test.go` 里的
`TestStatusBarDropsTheModeSwitch`。

- [ ] **步骤 4：运行测试验证通过**

运行：`go test ./internal/view/ -v`
预期：PASS

- [ ] **步骤 5：Commit**

```bash
git add internal/view/view.go internal/view/layout_test.go
git commit -m "feat: 移除底部信息栏，正文占满页面高度"
```

---

### 任务 5：消息弹成 Toast（并保证重启不重播）

**文件：**
- 修改：`internal/view/notices.go`、`internal/view/view.go`
- 测试：`internal/view/notices_test.go`

**Interfaces：**

```go
const noticeToastText = 120
func (s *State) updateToasts(c *ui.Context)
```

- [ ] **步骤 1：编写失败的测试**

| 测试 | 断言 |
| --- | --- |
| `TestAMessageIsToastedOnce` | 提示一条后构建 `View`：`noticeShown == s.Messages()[0].ID`；再提示一条、再构建：`noticeShown` 前进到新的 ID |
| `TestThePageToastsWhatItReports` | 提示一条后构建 `Content`：`noticeShown` 等于最新 ID（构建期问 Toast 不 panic） |
| `TestOldMessagesAreNotToastedOnStartup` | 同一个 `cfgDir` 建 `State` 甲、提示两条；再建 `State` 乙并构建 `View`：乙的 `noticeShown == s.Messages()[0].ID`（旧消息不重播） |
| `TestTheToastKeepsLongMessagesShort` | 400 字的提示：构建后 `noticeShown` 前进（走的是 `trimEllipsis` 到 `noticeToastText`，不 panic、不抛） |

- [ ] **步骤 2：运行测试验证失败**

运行：`go test ./internal/view/ -run "TestAMessageIsToastedOnce|TestThePageToasts|TestOldMessages" -v`
预期：FAIL，`noticeShown = 0, want 1`

- [ ] **步骤 3：实现 `updateToasts` 并接上两处构建**

`notices.go`：从 `len(msgs)-1` 往 0 走，`m.ID > s.noticeShown` 的逐条
`c.AddToast(ui.Toast{ID: "notice-" + strconv.Itoa(m.ID), Title: trimEllipsis(m.Text, noticeToastText), Type: m.Kind})`；
循环结束 `s.noticeShown = msgs[0].ID`（没有消息就保持 0）。
`view.go`：`View` 与 `Content` 都在 `s.drain()`、`s.handleShortcuts(c)` 之后调 `s.updateToasts(c)`。

- [ ] **步骤 4：运行测试验证通过**

运行：`go test ./internal/view/ -v`
预期：PASS

- [ ] **步骤 5：Commit**

```bash
git add internal/view/notices.go internal/view/view.go internal/view/notices_test.go
git commit -m "feat: 新消息弹成 Toast，重启不重播旧消息"
```

---

### 任务 6：侧栏铃铛（红色数字角标）+ 消息历史浮层

**文件：**
- 修改：`internal/view/notices.go`、`internal/view/view.go`（`leftToolbar`）、`internal/view/shell_test.go`
- 测试：`internal/view/notices_test.go`、`internal/view/shell_test.go`

**Interfaces：**

```go
const (
	noticePanelWidth  float32 = 360
	noticePanelHeight float32 = 320
	noticeBadgeMax            = 99
)

func (s *State) bellButton(c *ui.Context)          // 角标 + 浮层，挂在 leftToolbar 的右侧组
func (s *State) bellBadge(c *ui.Context, n int) ui.Element // 红色数字胶囊；n == 0 不建
func (s *State) noticePanel(c *ui.Context)         // 浮层内容（标题行 + 列表 / 空态）
func (s *State) noticeRow(c *ui.Context, m notices.Message)
func noticeBadgeText(n int) string                 // "1"…"99"、"99+"
func noticeTime(at, now time.Time) string          // 15:04 / 01-02 15:04 / 2006-01-02 15:04
func noticeIcon(kind string) func() *ui.SVG        // Info / CircleCheck / TriangleAlert / CircleX
func noticeTint(t *ui.Theme, kind string) ui.Color // Accent / Success / Warning / Danger
```

- [ ] **步骤 1：编写失败的测试**

| 测试 | 断言 |
| --- | --- |
| `TestTheToolbarShowsTheBell` | 构建 `LeftToolbar`：`HasText("消息")` |
| `TestTheBellOpensAndClosesTheHistory` | `Click("消息")` → `noticeOpen == true`；再 `Click` → `false` |
| `TestTheBadgeCountsTheUnreadOnes` | 关着收两条 → 构建 `LeftToolbar` 后 `HasText("2")`（角标）且 `HasText("消息（2 条未读）")` |
| `TestTheBadgeIsCappedAt99` | 收 120 条 → `noticeBadgeText(120) == "99+"`；`HasText("99+")` |
| `TestTheBadgeIsTheThemesDangerColor` → 改为 `TestTheBadgeTextNeverGrows` | `noticeBadgeText(0) == ""`、`noticeBadgeText(1) == "1"`、`noticeBadgeText(99) == "99"`、`noticeBadgeText(100) == "99+"`、`noticeBadgeText(12345) == "99+"` |
| `TestNoBadgeWithoutUnread` | 没有未读时构建 `LeftToolbar`：`!HasText("0")`、`HasText("消息")` |
| `TestOpeningTheHistoryMarksThemSeen` | 未读两条 → `Click("消息（2 条未读）")` → `Unseen() == 0`，label 回到 `"消息"` |
| `TestUnreadSurvivesARestart` | 收两条不打开 → 同一个 `cfgDir` 再建 `State`：`Unseen() == 2`，label 仍是 `"消息（2 条未读）"` |
| `TestTheHistoryListsTheNewestFirst` | 收两条、`noticeOpen = true`、构建 `View`：`tt.Texts()` 里后收那条的下标小于先收那条 |
| `TestLongMessagesWrapInTheHistory` | 400 字的提示：`Messages()[0].Text` 保持全文，`tt.HasText(全文)` 为真 |
| `TestTheHistoryDeletesOneMessage` | 三条、打开面板、`Click("删除消息：已复制文件名称")` → 剩两条、`noticeOpen` 仍为 true、重新 `Load` 后也只剩两条 |
| `TestDeletingAMessageKeepsTheRestInOrder` | 三条删中间：面板上剩下两条且顺序不变 |
| `TestTheHistoryClearsEveryMessage` | `Click("清空全部")` → `len(Messages()) == 0`、`HasText("暂无消息")` |
| `TestTheHistorySaysWhenItIsEmpty` | 无消息：`HasText("暂无消息")`、`!HasText("清空全部")` |
| `TestMessageTimesReadAcrossDays` | `noticeTime(今天 09:41, 今天 12:00) == "09:41"`；昨天 → `"11-23 09:41"`；去年 → `"2025-11-23 09:41"` |

注意：未读时铃铛 label 带条数（`消息（2 条未读）`），要按**当前 label** 点（或先置 `s.noticeOpen = true`
再构建），选一种并在所有用例里保持一致。

- [ ] **步骤 2：运行测试验证失败**

运行：`go test ./internal/view/ -run "TestTheToolbarShowsTheBell|TestTheBell|TestTheBadge|TestNoBadge|TestOpeningTheHistory|TestUnreadSurvives" -v`
预期：FAIL（`undefined: bellButton` / `HasText("消息")` 为假）

- [ ] **步骤 3：`leftToolbar` 加铃铛**

`view.go` 的 `leftToolbar`：`ui.Spacer(c)` 之后插入 `s.bellButton(c)`，MCP 仍在最后。

- [ ] **步骤 4：`notices.go` 写铃铛与浮层**

- `bellButton`：`label` 按 D8/D9；`button := components.IconButton(…).Size(t.Space(8), t.Space(8))`；
  有未读时 `button.TextColor(t.Accent)`；`Clicked()` 切换 `s.noticeOpen`，置 true 时 `s.notes.MarkSeen()`（并忽略返回的 error）；
  `ui.Box(c).Width(t.Space(8)).Height(t.Space(8))` 里先 `button`、再 `s.bellBadge(c, s.notes.Unseen())`；
  `ui.Popover(c, button, &s.noticeOpen, func(){ s.noticePanel(c) })`。
- `bellBadge`：`n <= 0` 时**不建**任何元素（`ui.Box` 的 Children 里直接 return）；
  否则按 D9 的一串属性建 `components.Badge`。
- `noticePanel`：`ui.Column(c).Width(noticePanelWidth).Padding(t.Space(1.5)).Gap(t.Space(1))`；
  首行 `消息`（`.Bold().Grow(1)`）+ 有消息时 `ui.Button(c, "清空全部")`（点了 `s.ClearMessages()`）→ `ui.Divider(c)`；
  空态 `components.Text(c, "暂无消息").FontSize(12).TextColor(t.TextMuted)` 后返回；
  否则 `ui.Scroll(c).MaxHeight(noticePanelHeight)` 里一个 `Gap(t.Space(.5))` 的列，遍历**拷贝后**的
  `s.Messages()` 调 `noticeRow`。
- `noticeRow`：`ui.Row(c).AlignItems(ui.Center).Gap(t.Space(1))`；图标（`t.Rem(.875)`，色按 `noticeTint`）；
  `ui.Column(c).Grow(1).MinWidth(0)` 里全文（`FontSize(12)`）+ 时间（`FontSize(11)`、`TextMuted`、`noticeTime(m.At, time.Now())`）；
  行尾 `components.IconButton(c, lucide.X(), "删除消息："+trimEllipsis(m.Text, 12), components.IconButtonCompact)`，
  点了 `s.RemoveMessage(m.ID)`。

- [ ] **步骤 5：`shell_test.go` 的期望文案加「消息」**

`{"LeftToolbar", s.LeftToolbar, 280, 40, []string{"打开项目", "刷新", "消息", "MCP"}}`

- [ ] **步骤 6：运行测试验证通过**

运行：`go test ./internal/view/ -v`
预期：PASS

- [ ] **步骤 7：Commit**

```bash
git add internal/view/notices.go internal/view/view.go internal/view/notices_test.go internal/view/shell_test.go
git commit -m "feat: 左侧工具栏新增铃铛（红色数字角标）与消息历史浮层，可单条删除或清空"
```

---

### 任务 7：Toast 落位右下角

**文件：**
- 修改：`internal/view/notices.go`、`main.go`
- 测试：`internal/view/notices_test.go`

- [ ] **步骤 1：编写失败的测试**

```go
// TestToastsSitAtTheBottomEnd pins where a message pops up: the bottom right
// corner, out of the way of the outline and the body's first lines.
func TestToastsSitAtTheBottomEnd(t *testing.T) {
	if got := ToastOptions().Placement; got != components.ToastBottomEnd {
		t.Errorf("placement = %v, want the bottom right", got)
	}
}
```

- [ ] **步骤 2：运行测试验证失败**

运行：`go test ./internal/view/ -run TestToastsSitAtTheBottomEnd -v`
预期：FAIL，`undefined: ToastOptions`

- [ ] **步骤 3：实现并从 `main.go` 接上**

```go
// ToastOptions is where the window shows a message, in the Onevra look.
func ToastOptions() components.ToastOptions {
	return components.ToastOptions{Placement: components.ToastBottomEnd}
}
```

`main.go` 的 `sidebar.ShellOptions{…}` 里加一行 `Toasts: view.ToastOptions(),`（放在 `Content` 之后、`Theme` 之前，
与结构体字段顺序一致）。

- [ ] **步骤 4：运行测试验证通过**

运行：`go test ./... && go vet ./... && gofmt -l .`
Windows 另跑 `go build -o NUL .`（其他平台 `go build -o /dev/null .`）
预期：PASS、编译通过

- [ ] **步骤 5：Commit**

```bash
git add internal/view/notices.go internal/view/notices_test.go main.go
git commit -m "feat: Toast 落在窗口右下角"
```

---

### 任务 8：文档回写

**文件：**
- 修改：`README.md`、`AGENTS.md`、`CHANGELOG.md`、`docs/superpowers/specs/2026-11-21-markdown-preview-design.md`

- [ ] **步骤 1：README**

- 第 74 行那条「底部状态栏只显示一行提示（…）」整条替换：提示改为**右下角 Toast**（几秒后自动消失）；
  左侧工具栏底部的铃铛带**红色数字角标**（未读条数，最多 `99`，超过显示 `99+`），点开是消息历史，
  可单条删除、可一键清空；历史写在用户数据目录的 `notices.json`，**重启后原样恢复**（含未读）。
- 第 154 行的结构说明里把「状态栏」换成「消息历史浮层（`notices.go`）」，并说明底部信息栏已移除、正文吃满高度。
- 「已知限制」补一条：消息历史不设条数上限，会一直累积（用「清空全部」或单条删除来收缩）。

- [ ] **步骤 2：AGENTS.md**

- 结构表新增一行 `internal/notices`：消息账本与 `notices.json`（原子写，无 ui 依赖，可无窗口测试）；
  `internal/view` 那一行把「预览、状态栏、MCP 设置面板」改成「预览、消息历史浮层（`notices.go`）、MCP 设置面板」。
- 依赖方向补 `view → notices`。
- 「改动指引」里「改标签栏/标题行/侧栏」那条去掉 `statusBar`，`leftToolbar` 的描述改成「动作图标 + 铃铛」。
- 「已知的坑」里**替换**「状态栏只放提示文字（`statusBar`，高 24）」这条：**没有底部状态栏**——提示一律走
  `notify*` → 账本 → `updateToasts` 弹 Toast / 铃铛浮层看历史；「一次点击会建两遍视图」同样适用于 Toast，
  所以按 `noticeShown` 水位线去重，且 `New` 里把水位线对到账本最新一条（否则重启会把旧消息全部重播）。
- 「改动指引」新增一条：**改提示消息**——在 `internal/view/notices.go` 里加/改 `notify*` 与浮层，调用处只调这四个方法，
  不要在 view 里直接 `c.AddToast`（会绕过账本与去重）；账本自己的行为改 `internal/notices` 并补
  `TestMessagesSurviveARestart` 一类测试；改 Toast 落位只改 `ToastOptions()`。
- 坑里补一条：**`Content` 必须 `drain()`**——shell 只有它这一条构建路径，漏了它，`Post` 投递的 watcher 变更
  在真实窗口里永不生效（历史 bug，2026-11-24 修）。
- 坑里补一条：**`New(…, "")` 表示无用户数据目录**——账本只在内存里，`Save` 是 no-op；`dir` 非空时是
  `notices.json` 的目录。测试一律传 `t.TempDir()`。

- [ ] **步骤 3：CHANGELOG.md**

「未发布」小节里：

- **新增**：消息历史浮层——左侧工具栏底部的铃铛，未读时带红色数字角标（最多 `99`，超过显示 `99+`）；
  每条消息带时间与类型图标，可单条删除、可一键清空；历史按天显示时间，重启后恢复（未读状态一并恢复）。
- **变更**：提示不再是底部一行文字，改为窗口右下角的 Toast，几秒后自动消失；底部信息栏移除，
  正文与目录吃满这段高度。
- **修复**：外部改动（agent / 编辑器写文件）在真实窗口里没有实时跟随的问题。
- **已知限制**：消息历史不设条数上限，会一直累积（用「清空全部」收缩）。

- [ ] **步骤 4：规格**

`docs/superpowers/specs/2026-11-21-markdown-preview-design.md`：

- 文件末尾「状态栏的两次搬家」里「状态栏只剩一行提示文字（高度 24）」那条旁边注明
  「2026-11-24 起状态栏整个移除，见下一节」，不改写历史段落。
- 末尾追加一节「### 提示改为 Toast 与铃铛历史（新增，2026-11-24）」，写清用户可见的部分：右下角 Toast 的四种类型与
  落位、铃铛与红色数字角标（`99+` 上限）、浮层的顺序/时间/单条删除/清空、历史的落盘与重启恢复、不设条数上限、
  底部 24 DIP 交还正文，以及 `Content` 补 `drain()` 这个前置修复；并注明账本是 `internal/notices`（纯逻辑、无 ui 依赖）。

- [ ] **步骤 5：提交**

```bash
git add README.md AGENTS.md CHANGELOG.md docs/superpowers/specs docs/superpowers/plans
git commit -m "docs: 回写提示改为右下角 Toast 与铃铛消息历史的说明"
```

---

## 审查重点（Review Focus）

| # | 输入 / 条件 | 正常人期待的行为 | 钉住它的测试 |
| --- | --- | --- | --- |
| 1 | 重启应用（上次留了 3 条未读） | 历史原样恢复、角标仍显示 3、**不**把旧消息重新弹一遍 Toast | `TestUnreadSurvivesARestart`、`TestOldMessagesAreNotToastedOnStartup` |
| 2 | 一次点击把视图建两遍（点目录、点按钮） | 同一条消息只弹一个 Toast、读屏只播报一次 | `TestAMessageIsToastedOnce` |
| 3 | 未读 120 条 | 角标显示 `99+`，数字不被撑破布局 | `TestTheBadgeIsCappedAt99` |
| 4 | `notices.json` 被写坏 / 目录不可写 / 没有用户数据目录 | 应用照常启动，历史为空或照旧，新消息在内存里仍然可见、可删可清，不 panic | `TestABrokenFileLeavesAnEmptyStore`、`TestAMemoryOnlyStoreKeepsWorking` |
| 5 | 面板开着，用户点第 2 条的 ✕ | 只删这一条，其余顺序不变，不跳到下一条 | `TestDeletingAMessageKeepsTheRestInOrder` |
| 6 | 消息里有很长的路径（几百字符） | Toast 里截断成一行，浮层里换行显示全文，面板不被撑出窗口 | `TestLongMessagesWrapInTheHistory`、`TestTheToastKeepsLongMessagesShort` |
| 7 | 跨天的历史 | 今天只显示 `HH:MM`，更早带日期，不显示成「今天的 09:41」 | `TestMessageTimesReadAcrossDays` |

每一条都已经落在对应任务的测试步骤里；实施时若发现某条没有对应测试，先补测试再改代码。

## 风险与备选

- **`c.AddToast` 在构建期调用**：mygo 自己的 `toast_test.go` 也是在 viewport 的构建函数里调 `AddToast`，
  这是官方用法；两个测试（`View` 与 `Content`）分别钉住。
- **角标压在按钮角落会吃掉点击**：`IgnoreInput(true)` 让它穿透到按钮；若实测仍挡，把角标改成
  `Right(0).Top(0)`（不再外溢）或挪到按钮**右侧**内联（`ui.Spacer` 前），不改其它逻辑。
- **无上限的 `notices.json`**：用户明确要求不设上限；写盘是整份重写，历史极大时单次写会变慢。
  若以后要收，只改 `internal/notices` 的 `Add`（加一个上限常量）并补一条测试，view 一行不用动。
- **浮层贴左边缘**：锚点是侧栏最右下那个按钮，面板宽 360 会向右展开、可能盖住正文左侧；这是 `ui.Popover`
  的默认行为（放不下才翻转），与 MCP 面板一致，本期不改。
