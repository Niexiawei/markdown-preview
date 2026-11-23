# 托盘图标、关闭窗口不退出、应用与托盘图标 实现计划

> **面向 AI 代理的工作者：** 必需子技能：使用 subagent-driven-development（推荐）或 executing-plans 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** 让应用常驻通知区域：**关闭窗口只是隐藏窗口，进程继续跑**（文件监听、MCP server 都不中断），
只有托盘菜单里的「退出」才真正结束进程。托盘菜单只有两项：**打开主页面**、**退出**。
同时给应用（窗口/任务栏图标、`mygo build` 出的可执行文件图标）和托盘各配一个图标。

**架构：** 新增 `internal/tray` 包（只依赖 `github.com/egoist/mygo`，不 import `.../view`）：
`go:embed` 三张 PNG（窗口图标 256、托盘图标 32、以及走 `resources/` 的可执行文件图标），
负责建托盘、装两条菜单、把 `Window.OnClose` 改成隐藏、在 `App.OnWillQuit` 里销毁托盘。
`main.go` 只做接线：先抢单实例锁 → 建 shell → 设置窗口图标 → 装托盘 → 用同一条「显示窗口」路径响应托盘菜单与第二次启动。

**技术栈：** Go 1.27+、`github.com/egoist/mygo v0.9.1`（`NewTray` / `TrayOptions{Icon, IconIsTemplate, ID, ToolTip, Menu}`、
`Tray.Destroy`、`Menu` / `MenuItem`、`Window.OnClose` / `Restore` / `Show` / `Focus` / `SetIcon`、
`Application.RequestSingleInstanceLock` / `OnSecondInstance` / `OnBeforeQuit` / `OnWillQuit` / `OnActivate`）、
`github.com/oneo-me/onevra/source v0.2026.18`（`shells/sidebar` 的 `Shell.Window()`）。
**不引入新依赖。**

**规格：** `docs/superpowers/specs/2026-11-21-markdown-preview-design.md`。该文档的「非目标」里没有托盘，
「多窗口 / 应用打包」也不是本期的目标，所以本计划**不推翻**任何既有目标，只在执行后往那份规格的
「实施后的变更（写回）」里追加一节（见「文档回写」）。需求来自用户 2026-11-23：
「给应用加上托盘图标」「关闭应用只是关闭窗口不退出」「托盘中就两个功能 打开主页面/退出」「同时给应用与托盘增加图标」，
图标文件在 `C:\Users\niexiawei\WorkBuddy\2026-11-22-23-50-38\icons\`（见任务 0）。

## 设计决策（实施者按此执行，不要另立方案）

| # | 决策 | 理由 |
| --- | --- | --- |
| D1 | **关闭窗口 = 隐藏窗口**：`win.OnClose` 里 `e.PreventDefault()` + `win.Hide()`；只有 `quitting` 为真时才放行。`quitting` 由 `mygo.App.OnBeforeQuit` 置位。 | 用户原话「关闭应用只是关闭窗口不退出」。`OnClose` 是 mygo 唯一能拦下关闭的钩子；`App.Quit()` 会走 `prepareQuit` → 逐个窗口 `close()`，所以「托盘里的退出」必须让 `OnClose` 认得这是一次真退出，否则 quit 被自己挡住，应用永远退不掉。 |
| D2 | 托盘菜单固定**两项、无分隔线**：`打开主页面`（点击 → 显示窗口）、`退出`（点击 → `mygo.App.Quit()`）。菜单挂在 `TrayOptions.Menu` 上，三平台一致。 | 用户原话「托盘中就两个功能 打开主页面/退出」。Linux 的 AppIndicator **只支持菜单**（没有直接点击回调）；macOS 一旦设了菜单，左键点击就是开菜单。统一走菜单，三个平台行为一致，也不需要为平台写分支（违反 AGENTS「不写平台专用代码」）。 |
| D3 | **托盘建不起来就退回「关闭即退出」**：`tray.Install` 先 `NewTray`，成功之后才装 `OnClose` 钩子；失败时只 `log.Printf`，窗口照旧可以关闭退出。 | 反过来（先装钩子）会在没有托盘的环境里把用户锁死：窗口一关就消失，既没有托盘可以点回来，也没有可见的退出入口——只剩任务管理器。宁可牺牲常驻，也不能出现「找不到窗口」的状态。 |
| D4 | **单实例**：`mygo.App.RequestSingleInstanceLock()` 拿不到锁就直接返回（进程退出）；拿到锁后注册 `OnSecondInstance`，把已有窗口显示出来。 | 关闭即隐藏之后，用户再次双击图标启动，若允许第二个进程就会变成「两个托盘图标、两份监听、两个 MCP 端口」。这是本功能**带出来**的必然后果，不是额外的产品需求。 |
| D5 | 「打开主页面」= `Restore()` + `Show()` + `Focus()`，抽成 `tray.Show(win)`，托盘菜单与 `OnSecondInstance` 共用。 | 窗口可能只是隐藏、也可能被最小化过；`Show` 对有 native 窗口但最小化的窗口不一定还原。一条路径同时覆盖三种「不在眼前」的状态。 |
| D6 | macOS 额外注册 `App.OnActivate`：`hasVisibleWindows == false` 时 `Show(win)`。（该回调只在 macOS 触发，其它平台注册了也不会跑。） | macOS 关窗后 Dock 图标还在，用户会去点 Dock；不响应就等于「点了没反应」。这条只补 macOS 的既有交互习惯，不引入平台分支。 |
| D7 | 图标分三层，各管一段，**不混用**：① 可执行文件/安装包的图标 = `mygo.json` 的 `icon`（`resources/icon.png`，1024 方形 PNG）由 `mygo build` 生成 `.ico`/`.icns`；② 窗口/任务栏图标 = 运行时 `win.SetIcon(png)`（256 PNG，`go:embed`，Windows/Linux 生效，macOS 窗口不带图标）；③ 托盘图标 = `tray.TrayOptions.Icon`（32 PNG，`go:embed`）。 | 用户「给应用与托盘增加图标」。`go build -o mp.exe .`（README 里的命令）**不会**读 `mygo.json`，所以任务栏图标必须运行时设；而 exe 图标只有 `mygo build` 能做——两者是不同管线，混起来会得出「图标不生效」的错误结论。 |
| D8 | 托盘图标只打包 `tray-on-light`（墨色剪影 + 透明），`IconIsTemplate: true`；**不打包 `tray-on-dark`**。 | mygo 的 template 语义正好是这套图标的设计意图：macOS 菜单栏按系统色调自动着色；Windows 任务栏深色时把不透明像素**涂白**；Linux 跟着桌面 color-scheme 涂白。自带两套再自己切是重复实现，而且 mygo 在明暗切换时**已经**会重新上色，不需要我们监听 `OnAppearanceChange`。 |
| D9 | 资源放 `internal/tray/assets/`：`window.png`（256）、`tray.png`（32），用 `//go:embed` 打进二进制。`resources/icon.png` 换成同一套的 1024 母版，只服务构建工具。 | 二进制被单独拷走（README 的 `go build -o mp.exe` 后双击）时，`resources/` 不一定在 exe 旁边；embed 与运行目录无关。1024 母版必须留在 `resources/`，因为 `mygo.json` 只认那里的 PNG。 |
| D10 | 托盘 `ID: "main"`、`ToolTip: "Markdown Preview"`。 | Windows 用 `ID` 记住图标被用户放到任务栏还是溢出区；macOS 用 `ID` 记住用户拖到的位置。固定 ID 才不会每次启动都跳回默认位置。 |
| D11 | `App.OnWillQuit` 里 `tray.Destroy()`。 | Windows 不显式销毁会留下「幽灵图标」（鼠标划过才消失）。 |
| D12 | 隐藏不改变任何视图状态：标签、滚动位置、渲染模式、搜索面板都留着，重新显示就是同一份画面。 | 隐藏复用窗口与视图，本来就不会重建；这条是**验收标准**（写进手工验收），防止有人后来改成「关闭时销毁窗口、打开时重建」。 |
| D13 | 新包叫 `internal/tray`，导出 `Install(win, toolTip) error`、`Show(win)`；`menuItems`、`keep` 等保持包内私有。**不在 `internal/view` 里做任何事**。 | 托盘是「窗口之外」的东西：`view` 的每个函数每帧都会跑，在视图里 `NewTray` 会每帧建一个托盘图标；`view` 的测试也用不了真窗口/托盘。放在自己的包里，纯逻辑（菜单项、关闭决策、图标尺寸）**不需要窗口**就能测。 |

### 改完长什么样（验收画面）

```
 托盘（Windows 通知区域 / macOS 菜单栏 / Linux 状态区）
 ┌────────────────┐
 │ [文档剪影图标]  │  ← tray.png + IconIsTemplate（浅色栏显示墨色，深色栏自动涂白）
 └────────────────┘
        点一下 ↓
      ┌──────────────────┐
      │ 打开主页面        │  ← Restore + Show + Focus
      │ 退出              │  ← App.Quit()
      └──────────────────┘

 主窗口（点 ✕）                      进程
 ┌──────────────────────┐          ┌──────────────────────────────┐
 │ …（窗口消失）         │  ──────► │ 继续：监听文件、MCP server、  │
 └──────────────────────┘          │ 标签与滚动位置都在内存里      │
                                    └──────────────────────────────┘
        托盘「打开主页面」 ← 同一份视图、同一个标签、同一个滚动位置
```

## 任务

### 任务 0：图标资产落地（一次性拷贝，提交进仓库）

- [ ] 从图标套件（**仓库外**，只在拷贝时用一次，构建期不引用这个路径）拷三个文件进来：
  - `C:\Users\niexiawei\WorkBuddy\2026-11-22-23-50-38\icons\icon-ink-1024.png` → `resources/icon.png`（**替换**现文件；1024×1024 RGBA 方形，`mygo.json` 已经在指它）
  - `…\icons\icon-ink-256.png` → `internal/tray/assets/window.png`（256×256）
  - `…\icons\tray\tray-on-light-32.png` → `internal/tray/assets/tray.png`（32×32，墨色剪影 + 透明）
- [ ] **不要**拷 `icon-ink.ico`（`mygo build` 从 PNG 生成）、**不要**拷 `tray-on-dark-*`（见 D8）。
- [ ] 确认三个文件都是 PNG（`file` 或 `png.DecodeConfig`），`resources/icon.png` 是方形（`mygo build` 会校验，非方形直接报错）。
- [ ] `internal/tray/assets/` 里**只放这两张图**，不要放 README 或 SVG（embed 只带需要的）。

### 任务 1：`internal/tray` 的图标（先写失败的测试）

- [ ] 写 `internal/tray/icons.go`：
  ```go
  package tray

  import _ "embed"

  // 窗口/任务栏图标（Windows、Linux；macOS 的窗口不显示图标）。
  //go:embed assets/window.png
  var windowPNG []byte

  // 托盘图标：墨色剪影 + 透明，配合 IconIsTemplate 让系统按明暗自动着色。
  //go:embed assets/tray.png
  var trayPNG []byte

  // WindowIcon 返回窗口/任务栏用的 PNG。
  func WindowIcon() []byte { return windowPNG }

  // Icon 返回托盘用的 PNG。
  func Icon() []byte { return trayPNG }
  ```
- [ ] 先写 `internal/tray/icons_test.go`（此时 `icons.go` 还不存在 → 编译失败，即为失败态）：
  ```go
  func TestEmbeddedIconsDecode(t *testing.T) // 两张图都能 png.DecodeConfig、宽高相等、非空
  func TestIconSizes(t *testing.T)           // window 256×256、tray 32×32（托盘/任务栏各自要的尺寸，见 D7）
  ```
- [ ] 跑 `go test ./internal/tray/` → 通过。

### 任务 2：关闭窗口即隐藏（先写失败的测试）

- [ ] 写 `internal/tray/keep.go`：
  ```go
  // keep 让窗口关不掉：OnClose 把它藏起来，只有 App.Quit 才放行。
  // 全部字段只在主线程读写。
  type keep struct {
      win      *mygo.Window
      quitting bool
  }

  // install 注册两个回调：真退出时放行关闭，平时关闭改成隐藏。
  func (k *keep) install() {
      mygo.App.OnBeforeQuit(func(*mygo.QuitEvent) { k.quitting = true })
      if k.win != nil {
          k.win.OnClose(k.close)
      }
  }

  // close 是 Window.OnClose：不是退出就拦下来隐藏窗口。
  func (k *keep) close(e *mygo.CloseEvent) {
      if k.quitting {
          return
      }
      e.PreventDefault()
      if k.win != nil {
          k.win.Hide()
      }
  }
  ```
  （`k.win == nil` 的判断只为测试能构造无窗口的 `keep`；生产路径永远有窗口。）
- [ ] 先写 `internal/tray/keep_test.go`：
  ```go
  func TestClosingTheWindowHidesIt(t *testing.T)   // keep{}.close(&mygo.CloseEvent{}) → DefaultPrevented() 为 true
  func TestQuittingLetsTheWindowClose(t *testing.T) // keep{quitting:true} → DefaultPrevented() 为 false
  ```
  —— 第二条是**最重要的回归测试**：写错方向就是「托盘退出无效、应用退不掉」。
- [ ] 跑 `go test ./internal/tray/` → 通过。

### 任务 3：托盘菜单与安装（先写失败的测试）

- [ ] 写 `internal/tray/tray.go`：
  ```go
  // Show 把窗口从隐藏或最小化里带回来：还原 → 显示 → 拿焦点。
  // 主线程调用。
  func Show(win *mygo.Window) {
      if win == nil { return }
      win.Restore()
      win.Show()
      win.Focus()
  }

  // menuItems 是托盘菜单的全部内容：打开主页面 / 退出。
  // 不加分隔线、不加第三项（用户 2026-11-23：托盘中就两个功能）。
  func menuItems(win *mygo.Window) []*mygo.MenuItem {
      return []*mygo.MenuItem{
          {Label: "打开主页面", Click: func(*mygo.MenuItem, *mygo.Window) { Show(win) }},
          {Label: "退出", Click: func(*mygo.MenuItem, *mygo.Window) { mygo.App.Quit() }},
      }
  }

  // Install 建托盘图标并让窗口常驻：只有托盘里的「退出」才结束进程。
  // 必须在 App ready 之后、主线程调用。托盘建不起来时返回 error，
  // 且**不**接管窗口关闭（D3）。
  func Install(win *mygo.Window, toolTip string) error {
      t, err := mygo.NewTray(mygo.TrayOptions{
          ID:             "main",
          Icon:           Icon(),
          IconIsTemplate: true,
          ToolTip:        toolTip,
          Menu:           mygo.NewMenu(menuItems(win)),
      })
      if err != nil {
          return err
      }
      (&keep{win: win}).install()
      mygo.App.OnActivate(func(hasVisibleWindows bool) {
          if !hasVisibleWindows {
              Show(win)
          }
      })
      mygo.App.OnWillQuit(func(*mygo.QuitEvent) { t.Destroy() })
      return nil
  }
  ```
- [ ] 先写 `internal/tray/tray_test.go`（**不建托盘、不建窗口**）：
  ```go
  func TestTrayMenuOpensTheWindowOrQuits(t *testing.T) {
      items := menuItems(nil) // 菜单项是纯数据：List/Menu 都不需要 app，测试里不建托盘也不建窗口
      // 断言：恰好 2 项、无 Separator、Label 依次是「打开主页面」「退出」、两项的 Click 都非 nil
  }
  ```
- [ ] 跑 `go test ./internal/tray/` → 通过。

### 任务 4：`main.go` 接线（单实例 + 窗口图标 + 托盘）

- [ ] `main()` 开头抢锁：
  ```go
  if !mygo.App.RequestSingleInstanceLock() {
      return // 已有实例在跑：它自己会把窗口带到前面
  }
  ```
- [ ] 用包级变量保存窗口，并在 `WhenReady` 之前注册第二次启动与（macOS）激活的处理：
  ```go
  var appWindow *mygo.Window   // 只在主线程读写（WhenReady、OnSecondInstance 都在主线程）

  mygo.App.OnSecondInstance(func([]string, string) { tray.Show(appWindow) })
  ```
  （`OnSecondInstance` 可能在 `WhenReady` 之前到达；`Show(nil)` 是空操作，此时窗口本来也正要出现。）
- [ ] shell 建好之后：
  ```go
  win := shell.Window()
  appWindow = win
  if err := win.SetIcon(tray.WindowIcon()); err != nil {
      log.Printf("window icon: %v", err)
  }
  if err := tray.Install(win, "Markdown Preview"); err != nil {
      // 没有托盘就退回到「关闭即退出」，否则窗口一关就再也找不回来（D3）。
      log.Printf("tray: %v", err)
  }
  ```
  —— `tray.Install` 的成功路径已经包含 `Show`/`Quit` 菜单、关闭即隐藏、`OnActivate`、`OnWillQuit` 销毁托盘。
- [ ] **不要**在这里调 `mygo.App.SetName(...)`：`App.Path(PathUserData)` 是 `UserConfigDir/<App.Name()>`，
  改名会把用户的 `config.json` / `mcp.json` 搬到新目录，等于丢掉配置（见「已知的坑」）。
- [ ] `go build ./...` 通过；`go vet ./...` 干净。

### 任务 5：文档回写（**并入同一个提交**，不单开一次「补文档」）

- [ ] `README.md`
  - 「用法」末尾加一条托盘条目：关闭窗口只是隐藏（进程继续、监听与 MCP 不中断），
    托盘点「打开主页面」回来（标签与滚动位置都还在），只有「退出」结束进程；
    macOS 关窗后点 Dock 图标也能把窗口叫回来；再次启动不会开第二个实例。
  - 「运行」小节补一句图标的分工：可执行文件图标来自 `mygo.json` 的 `resources/icon.png`，
    **只有 `mygo build` 会把它写进 exe**；窗口/任务栏与托盘图标是运行时从二进制里的 PNG 设的，`go build` 也有。
  - 「已知限制」：托盘建不起来（缺少状态区支持的环境）时会退回「关闭即退出」，日志里有 `tray:` 提示。
  - 「目录结构」加 `internal/tray/` 一行（图标、托盘菜单、关闭即隐藏）。
- [ ] `CHANGELOG.md` 的「未发布」
  - **新增**：托盘常驻（关闭窗口不退出、托盘两项：打开主页面/退出、macOS 点 Dock 召回窗口）、单实例、
    应用图标（窗口/任务栏 + `mygo build` 的 exe 图标）与托盘图标（跟随系统明暗自动着色）。
  - **变更**：点窗口关闭按钮不再结束进程（以前会退出），要退出请用托盘菜单。
- [ ] `AGENTS.md`
  - 「结构」表加 `internal/tray`；`main.go` 的职责里补「单实例锁、窗口图标、托盘」。
  - 「硬规则」加：
    - 托盘与窗口图标只经 `internal/tray`（`Install` / `WindowIcon` / `Show`），**不在 `internal/view` 里建**：
      view 每帧都会跑。
    - 关闭窗口 = 隐藏，**必须**保留 `quitting`（`App.OnBeforeQuit` 置位）这一条放行路径，否则「退出」退不掉。
    - 托盘失败要退回到关闭即退出（先 `NewTray` 后装 `OnClose`），不要反过来。
    - 图标三处分工（exe/窗口/托盘）与 `IconIsTemplate` 的用法：不要自己再按明暗换图。
  - 「已知的坑」加：`App.SetName` 会改 `PathUserData`（丢配置）；`go build` 不带 exe 图标；`OnWillQuit` 必须 `Tray.Destroy`。
  - 「改动指引」加：改托盘菜单只改 `internal/tray/tray.go` 的 `menuItems` 并补 `TestTrayMenuOpensTheWindowOrQuits`；
    换图标只换 `internal/tray/assets/` 两张图（窗口 256、托盘 32；托盘留墨色剪影 + 透明）与 `resources/icon.png` 母版。
- [ ] `docs/superpowers/specs/2026-11-21-markdown-preview-design.md`：在「实施后的变更（写回）」末尾追加
  「### 托盘与窗口生命周期（新增，2026-11-23）」：关闭即隐藏、托盘两项、单实例、图标三层、托盘失败回退。
- [ ] 本计划末尾追加「执行后的修订」（只追加，不改写上面的任务步骤）：实际做了什么、
  与计划的偏离（例如 D8 发现某平台不涂白、或 `OnActivate` 在某平台也触发）。

## 验收

自动化：

```sh
go vet ./... && gofmt -l . && go test ./...
```

手工（三平台至少 Windows）：

- [ ] 启动后任务栏/窗口看到新图标；托盘里有墨色文档剪影（深色任务栏时是白色剪影）。
- [ ] 点窗口 ✕ → 窗口消失、**进程还在**（任务管理器/`ps` 里仍有），托盘图标还在。
- [ ] 托盘 → 打开主页面 → 窗口回来，**同一个标签、同一个滚动位置、渲染模式没变**。
- [ ] 窗口最小化后走同一条路径 → 窗口还原并拿到焦点。
- [ ] 托盘 → 退出 → 进程结束、托盘图标消失（Windows 没有幽灵图标）。
- [ ] 隐藏状态下再次双击 exe → 不开第二个进程/第二个托盘图标，已有窗口被带到前面。
- [ ] macOS：关窗后点 Dock 图标 → 窗口回来；`Cmd+Q` → 正常退出。
- [ ] 断开/失败场景：日志里出现 `tray:` 时，点 ✕ 应能正常关闭并退出（D3 的回退）。

## 未决问题（本计划按括号里的选择执行，用户可推翻）

1. 托盘点一下是**直接开菜单**（本计划 D2）还是「左键打开主页面、右键菜单」？后者在 Linux AppIndicator 上做不到，
   且要写平台分支。
2. 要不要「最小化也收进托盘」（本计划只改关闭按钮，最小化仍是最小化）？
3. 要不要记住「上次是被关闭到托盘而不是退出」，以便开机自启时静默启动（本期不做，无开机自启）。

## 执行后的修订（2026-11-23）

1. **任务 0 与任务 1 合成一个提交**（`feat: 应用与托盘图标资产，并嵌入二进制`）：资产拷进来就必须有 embed 才能编译，
   分开提交会留下一个「assets 在、但没人用」的中间提交。任务 2 / 3 / 4 各一个提交，共四个功能提交。
2. **实现与计划一致**，没有接口层面的偏离：`internal/tray` 导出 `Install(win, toolTip)`、`Show(win)`、
   `WindowIcon()`、`Icon()`；`keep` / `menuItems` 保持包内私有。
   `TestIconSizes`（256 / 32）、`TestClosingTheWindowHidesIt`、`TestQuittingLetsTheWindowClose`、
   `TestTrayMenuOpensTheWindowOrQuits` 全部先跑出红再转绿。
3. **确认了 D8 的判断**：读 mygo v0.9.1 的 `internal/{windows,linux,darwin}`，`IconIsTemplate` 在三个平台都由
   后端自己处理明暗（Windows/Linux 在任务栏/面板变暗时把不透明像素涂白、macOS 交给 AppKit 着色），
   所以只打包 `tray-on-light-32.png`，没有 `OnAppearanceChange` 回调、也没有 `tray-on-dark`。
4. **`Show` 里保留 `Restore()`**：对没最小化的窗口它是空操作；保留是为了「隐藏 + 最小化过」这一种状态也能回来，
   与 D5 一致。
5. **自动化跑过的一部分验收（Windows）**：构建 `mp.exe` 后启动，日志没有 `tray:`（托盘建成功）；
   用 `CloseMainWindow()` 发 `WM_CLOSE` 后进程仍在（`HasExited=False`，窗口句柄变 0）→ 关闭即隐藏成立；
   接着再启动一份 `mp.exe`，第二份立即以 0 退出、第一份的窗口句柄又回来 → 单实例与第二次启动召回窗口成立。
   **仍需人手确认的**：托盘菜单两项的实际点击（打开主页面 / 退出）、三平台的图标外观（含深色任务栏反白）。
6. **工作方式**：在 `feat/tray-and-icons` 分支上原地执行（没有另开 git worktree），与该仓库既有的
   `feat/*` → 合并的流程一致。
