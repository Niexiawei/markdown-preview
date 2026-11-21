# Markdown Preview

一个纯 Go 的本地 markdown 预览器：**多项目管理**、左侧 md 文件树、右侧「文档目录 + 正文」，
正文可以在**原生渲染**与 **WebView 渲染**之间切换；文件被 agent 或编辑器改动时，树和正文会立刻跟上。
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

## 用法

- 侧栏顶部是项目切换（显示当前项目名）→ 点开可以切换、移除项目；侧栏底部的图标按钮：打开项目、刷新、MCP。
- 侧栏（可用标题栏的按钮折叠/展开）是该项目里的 `.md` / `.markdown`（递归；跳过 `.git`、`node_modules`、`vendor`、`dist`、`build`、`.mygo` 和所有点开头的目录）。
- 右栏左半边是文档目录，点一项正文就滚到那个标题；右半边是正文。
- 正文里的相对链接可以在文档之间跳转，`#标题` 跳到本文档的标题，`http(s)` 链接交给系统浏览器。
- 底部状态栏左侧的分段控件切换当前文档的渲染后端：**原生** 或 **WebView**（每个文档各记各的，只在本次运行内有效）。
- 外部改动（新增 / 删除 / 重命名 / 写入）会自动反映；当前文档被改名时会跟着改名，被删掉时会提示并清空。

## MCP server（默认关闭）

工具栏「MCP」按钮 → 打开开关。打开后：

- 地址：`http://127.0.0.1:8765/mcp`（端口被占用时回退到系统分配的端口，界面上显示实际地址）
- 鉴权：请求头必须带 `Authorization: Bearer <token>`，token 在面板里可以复制，也写在用户数据目录的 `mcp.json`
- 传输：`POST` 单个端点，JSON-RPC 2.0，`application/json` 响应（不提供 SSE 长连接）

四个只读工具：

| tool | 入参 | 出参 |
| --- | --- | --- |
| `list_projects` | `{}` | `[{id,name,root,active}]` |
| `list_docs` | `{project?}` | `[{path,title,size,mod_time}]` |
| `read_doc` | `{path, project?}` | `{path,content}` |
| `search_docs` | `{query, project?, limit?}` | `[{path,line,text}]` |

`project` 省略时用当前活动项目。手写一次调用：

```sh
TOKEN=$(cat "$LOCALAPPDATA/markdown-preview/mcp.json" | grep token | cut -d'"' -f4)
curl -s http://127.0.0.1:8765/mcp \
  -H "Authorization: Bearer $TOKEN" -H 'content-type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"list_docs","arguments":{}}}'
```

## 已知限制

- 只读：不编辑、不保存文档。
- 没有滚动跟随：滚动正文不会高亮目录项（点目录能跳）。
- WebView 模式在文档变化或用目录跳转时会重新加载页面，滚动位置回到顶部（原生模式保留）。
- 没有代码块语法高亮（`ui/markdown` 的默认代码块是单色等宽）；WebView 模式同样只有等宽样式。
- 单个文档上限 8 MB，超过会提示而不加载；非法 UTF-8 会被替换成 `U+FFFD` 后照常显示。
- 一次只能看一个项目的一个文档（没有标签页、没有多窗口）。
- 树的 md 项超过约 3000 个时会变慢（`ui.Tree` 会构建全部节点），届时可换成 `ui.Outline`。
- 符号链接环的保护在需要管理员权限的平台上无法被测试覆盖（Windows 普通用户建不了符号链接）。

## 目录结构

```
main.go                     组装：配置 → workspace → sidebar shell → 文件监听 → MCP
internal/config/            配置的读写（项目列表、活动项目、渲染模式、MCP 设置）
internal/workspace/         扫描、读取、解析缓存、TOC/slug、搜索（Index 接口，喂 UI 和 MCP）
internal/watch/             把 mygo 的文件事件折叠成 ChangeSet
internal/mcp/               JSON-RPC 端点与四个只读工具
internal/view/              Onevra 主题 + 外壳各区域：侧栏顶栏、树、侧栏工具栏、标题、预览、状态栏、设置面板
```

设计与实现计划在 `docs/superpowers/` 下。
