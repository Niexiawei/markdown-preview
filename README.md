# Markdown Preview

一个纯 Go 的本地 markdown 预览器：**多项目管理**、左侧 md 文件树，打开的文档排成**标签页**，
右侧是「文档目录 + 正文」；正文可以在**原生渲染**与 **WebView 渲染**之间切换，并支持**正文内悬浮搜索**
（`Cmd/Ctrl+F`）与正文缩放。文件被 agent 或编辑器改动时，树和正文会立刻跟上。
内置一个默认关闭的**本地 MCP server**，供 agent 检索这些文档。

界面用 [`github.com/egoist/mygo`](https://github.com/egoist/mygo) 的原生 UI（单二进制，不需要 bun、不需要 web 前端），
布局与主题来自 [`github.com/oneo-me/onevra`](https://github.com/oneo-me/onevra)：`shells/sidebar` 的可折叠侧栏外壳、
`theme.Onevra` 的调色板（主色 `#0052d9`，悬停 `#618dff`，按下 `#003cab`，警告 `#e37318`）、以及 lucide 图标。

## 运行

```sh
go run .
# 或者
go build -o mp.exe . && ./mp.exe
```

要求 Go 1.27.3 或更新（`go.mod` 已声明；`GOTOOLCHAIN=auto` 会自动取工具链）。
WebView 模式额外需要系统的 web view：Windows 上是 WebView2 Runtime（Windows 11 自带），macOS/Linux 上随系统提供。
缺它时原生模式照常可用。

图标分两段，别混：**窗口/任务栏图标和托盘图标是运行时设的**（图在二进制里，`go build` 出的 exe 也有）；
**可执行文件本身的图标只有 `mygo build` 会写**（读 `mygo.json` 的 `resources/icon.png`，生成 `.ico` / `.icns`）。

## 构建安装包

```sh
# 一次性出三平台的安装包（macOS 要 macOS 才能出 dmg）
go tool mygo build -platform darwin/universal,windows/amd64,linux/amd64
```

产物在 `dist/<os>-<arch>/`：macOS 是 `.app` + `.dmg`；Windows 是 `Markdown Preview.exe` + `markdown-preview-setup-<版本>-amd64.exe`；
Linux 是可执行文件 + 带 `install.sh` 的 `.tar.gz`，以及 `.deb` / `.rpm` / `PKGBUILD`。
`mygo` 由 `go.mod` 的 `tool` 指令钉在 v0.9.1，本地与 CI 是同一套打包器。

`.github/workflows/build.yml` 会在每次 push / PR 上跑 gofmt、`go vet`、测试，并编译三平台安装包作为 Actions artifact；
推 `v*` 标签（须与 `mygo.json` 的 `version` 一致）时再自动建 Release 并挂上这些包，
所以发版顺序是：改 `mygo.json` 的 `version` → 提交 → `git tag v<该版本> && git push --tags`。

## 用法

- 侧栏顶部是项目面板（显示当前项目名）→ 点开列出所有项目，有勾的是打开的项目（树里显示、监听中）：
  **点一行打开/关闭它**，可以同时打开多个；右键一行：在文件管理器中打开、复制项目路径、移除项目…（移除要二次确认）。
  侧栏底部的图标按钮：打开项目（对话框可一次选多个目录）、刷新、MCP。
  侧栏宽 280 DIP（可拖分隔条改，双击复位），折叠/展开按钮在标题栏左侧。
- **打开多个项目时，树最上面多一层项目名**；只打开一个项目时不加这层。树里是项目里的 `.md` / `.markdown`（递归；跳过 `.git`、`node_modules`、`vendor`、`dist`、`build`、`.mygo` 和所有点开头的目录）。
  右键树里的一项：**复制文件**（把文件本身放进剪贴板，可以直接粘到文件管理器或其他应用里；目录上是「复制文件夹」）、复制文件名称、复制文件路径；目录还多一条在文件管理器中打开。
- 预览区最上面一行是**标签栏**（在窗口标题栏那一行里）：树里点开的每个文档都是一个标签，点标签切换、点标签上的 ✕ 关闭（也可以 `Cmd/Ctrl+W`）。
  标签下面是文档路径与「原生 / WebView」分段开关。
  标签放不下时，标签条本身可以横向滚动（鼠标滚轮、触控板横扫、拖动滚动条），切换标签时会**滑动着**把它带进视野
  （系统开了「减少动态效果」时直接到位）：
  当前标签一定完整可见，滚动停下的位置尽量落在标签起点上，两端有渐隐提示还有更多标签；滚动条是一根 2 DIP 的细线，
  不会抢走点在标签上的点击；
  另外最右边还有一个 ⌄ 按钮，列出全部已打开的文档（同样可以切换与关闭）。切换回标签时正文停在原来的滚动位置。
- 右栏左半边是文档目录，点一项正文**平滑滚**过去，把那个标题顶到正文顶部（顶上留 16 DIP 的空隙，标题不会贴在边缘）；标题靠近文档末尾、顶不到顶部时，
  正文直接滚到最底部（系统开了「减少动态效果」时直接到位）。目录太长或标题太宽时上下左右都能滚。
  目录标题那一行右边是四个按钮：**展开全部**（⇱）、**折叠全部**（⇲）、**缩小正文**（－）、**放大正文**（＋），中间显示当前比例；
  放大/缩小只作用于正文（正文、代码、表格一起缩放，目录与侧栏不变），快捷键 `Cmd/Ctrl+=` / `Cmd/Ctrl+-` / `Cmd/Ctrl+0`（复位）。
- 正文可以用 `Cmd/Ctrl+F` 唤出**悬浮搜索**（不是弹窗，浮在正文右上）：输入即筛，`n/m` 是第几处/共几处。
  面板是**三块独立的浮岛**——输入框、命中行列表（行号 + 该行内容）、当前命中前后各两行的上下文，
  块与块之间留空、各自带圆角与投影，所以列表和预览不会糊成一列；上下文那块顶上写明「第 N 行上下文」，命中片段高亮。
  点列表任一行，正文平滑跳到那一处（与点目录一样：命中所在的段落顶到顶部）；`Enter` / `Cmd/Ctrl+G` 下一个，
  `Shift+Cmd/Ctrl+G` 上一个，`Esc` 先清空查询、再按一次关闭。
- 正文里的相对链接可以在文档之间跳转，`#标题` 跳到本文档的标题，`http(s)` 链接交给系统浏览器。
- 标签下面那一行是文档路径：后面的复制图标把当前文档的绝对路径放进剪贴板（`Cmd/Ctrl+C` 做同一件事），
  再后面是一组分段控件（**原生** / **WebView**）切换当前文档的渲染后端，每个文档各记各的，只在本次运行内有效；
  路径太长时先省略路径，不会把控件挤出窗口。
- 底部状态栏只显示一行提示（文件被删、链接目标不存在、MCP 状态等）。
- 外部改动（新增 / 删除 / 重命名 / 写入）会自动反映；当前文档被改名时标签跟着改名，被删掉时标签消失并切到相邻标签。
- **常驻托盘**：点窗口的 ✕ 只是**隐藏窗口**，进程继续跑（文件监听、MCP server 都不中断），标签、滚动位置、渲染模式都留着。
  托盘菜单只有两项：**打开主页面**（窗口从隐藏或最小化回来，拿回焦点）、**退出**（真正结束进程）。
  托盘图标是应用图标的剪影版，浅色任务栏/菜单栏显示墨色、深色时系统自动反白；Windows 上它出现在通知区域（可能收在 `^` 里）。
  macOS 关窗后点 Dock 图标也能把窗口叫回来。再次启动不会开第二个进程，而是把已有窗口带到前面。

## 快捷键

`Cmd` 在 macOS 是 Command，其余平台是 Control。

| 快捷键 | 作用 |
| --- | --- |
| `Cmd+F` | 唤出 / 聚焦悬浮搜索 |
| `Esc` | 搜索：先清空查询，空查询时关闭面板 |
| `Enter` / `Cmd+G` | 下一个匹配 |
| `Shift+Cmd+G` | 上一个匹配 |
| `Cmd+=` / `Cmd+-` / `Cmd+0` | 放大 / 缩小 / 复位正文 |
| `Cmd+W` | 关闭当前标签 |
| `Cmd+C` | 复制当前文档的绝对路径 |

## MCP server（默认关闭）

工具栏「MCP」按钮 → 打开开关。打开后：

- 地址：`http://127.0.0.1:8765/mcp`（端口被占用时回退到系统分配的端口，界面上显示实际地址）
- 鉴权：请求头必须带 `Authorization: Bearer <token>`，token 在面板里可以复制，也写在用户数据目录的 `mcp.json`
- 传输：`POST` 单个端点，JSON-RPC 2.0，`application/json` 响应（不提供 SSE 长连接）

四个只读工具（只列出**打开**的项目）：

| tool | 入参 | 出参 |
| --- | --- | --- |
| `list_projects` | `{}` | `[{id,name,root,active}]` |
| `list_docs` | `{project?}` | `[{path,title,size,mod_time}]` |
| `read_doc` | `{path, project?}` | `{path,content}` |
| `search_docs` | `{query, project?, limit?}` | `[{path,line,text}]` |

`project` 省略时用当前活动项目（有选中文档时是它所属的项目，否则是第一个打开的项目）。手写一次调用：

```sh
TOKEN=$(cat "$LOCALAPPDATA/markdown-preview/mcp.json" | grep token | cut -d'"' -f4)
curl -s http://127.0.0.1:8765/mcp \
  -H "Authorization: Bearer $TOKEN" -H 'content-type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"list_docs","arguments":{}}}'
```

## 已知限制

- 只读：不编辑、不保存文档。
- 没有滚动跟随：滚动正文不会高亮目录项（点目录能跳）。
- **原生渲染模式下搜索不高亮正文**：点匹配项只是把正文平滑滚到命中的那一段（段落顶到顶部），高亮只在搜索面板的上下文预览里
  （`ui/markdown` 没有「给一段文字上底色」的接口）。WebView 模式用页面内查找，整页命中都高亮。
- WebView 模式下切换文档后页面内高亮会丢（页面重新加载），点一次匹配项即恢复；缩放是就地改样式，不重载页面，所以不丢滚动位置。
- 标签只在本次运行内有效：重启后标签栏是空的，需要重新从树里打开；缩放比例也不记忆。
- 正文宽度跟随窗格：窗口拉伸、侧栏折叠、拖动目录分隔条后，markdown 会按新宽度重新排版。
  比窗格更宽的内容由正文的横向滚动容器承载（图片等）；**表格**由 mygo 的 markdown 视图自带横向滚动，
  滚动条在表格底部，而不是窗口底部。
- WebView 模式在**文档内容变化**时会重新加载页面，滚动位置回到顶部（原生模式保留）；用目录跳转不再重载页面
  （页内平滑滚动），所以不丢位置。
- 没有代码块语法高亮（`ui/markdown` 的默认代码块是单色等宽）；WebView 模式同样只有等宽样式。
- 单个文档上限 8 MB，超过会提示而不加载；非法 UTF-8 会被替换成 `U+FFFD` 后照常显示。
- 一次只显示一个文档（有标签页可切换，没有多窗口、没有分屏）。
- 项目面板是自绘的浮层（按钮行 + 右键菜单），没有系统菜单的键盘导航。
- 树的 md 项超过约 3000 个时会变慢（`ui.Tree` 会构建全部节点），届时可换成 `ui.Outline`。
- 符号链接环的保护在需要管理员权限的平台上无法被测试覆盖（Windows 普通用户建不了符号链接）。
- 托盘建不起来的环境（例如没有状态区/AppIndicator 的 Linux 桌面）会退回「关闭即退出」（日志里有一行 `tray:`），
  因为那时窗口一关就再也找不回来。
- 发布的安装包**没有代码签名**：macOS 上 Gatekeeper 会拦（需要右键打开，或在系统设置里放行），
  Windows 上 SmartScreen 会警告。要正式分发得自己加证书（见 mygo 的 `docs/github-actions.md`）。

## 目录结构

```
main.go                     组装：单实例锁 → 配置 → workspace → sidebar shell → 窗口图标 → 托盘 → 文件监听 → MCP
internal/config/            配置的读写（项目列表、打开的项目、活动项目、渲染模式、MCP 设置）
internal/workspace/         扫描、读取、解析缓存、TOC/slug、搜索（Index 接口，喂 UI 和 MCP）；find.go 是正文内查找
internal/watch/             把 mygo 的文件事件折叠成 ChangeSet
internal/mcp/               JSON-RPC 端点与四个只读工具
internal/view/              Onevra 主题（theme.go）+ 外壳各区域：侧栏顶栏（项目面板与右键菜单）、树、
                            侧栏工具栏、标签栏（tabs.go，占 shell 的标题槽位）、预览、状态栏、MCP 设置面板；
                            标签下面那行路径与渲染开关在 Content 的第一行（view.go 的 titleRow）；
                            find.go 是悬浮搜索与跳转，zoom.go 是正文缩放；
                            system.go 是桌面动作（文件管理器 / 剪贴板 / 确认）的接口
internal/tray/              托盘图标与两条菜单、关闭窗口即隐藏（退出才放行）、窗口/任务栏图标；纯逻辑无窗口可测
.github/workflows/build.yml CI：gofmt / vet / 测试（Ubuntu + Pango）→ 三平台安装包 → 打标签时建 Release
```

## 变更记录

见 [CHANGELOG.md](CHANGELOG.md)。

设计与实现计划在 `docs/superpowers/` 下。
