# mygo：源码区间高亮（`Options.Mark` + `MarkVersion`）实现方案

> **面向实现者的工作说明：** 这份文档在 **`github.com/egoist/mygo` 仓库**执行（用户会提 PR），
> **不在本仓库**（`mdpreview`）执行。本仓库本期的计划是
> `docs/superpowers/plans/2026-11-23-find-panel-height.md`（只改搜索面板的尺寸行为）；
> 本文是那件事的下一步：正文内命中高亮。步骤使用复选框（`- [ ]`）跟踪进度。

**目标：** 让 `ui/markdown` 的视图可以把**源码里的若干字节区间**标出底色（命中高亮），
由 `Options.Mark []MarkRange` + `Options.MarkVersion uint64` 驱动，效果等价于「作者自己写了 `<mark>`」，
但**不改源码**、**不重新解析**、**不影响复制**，并且**覆盖代码块、表格单元格、原始 HTML 块与跨行内样式的命中**。

**架构：** 高亮属于**渲染层**而不是解析层——`parser` 的职责是给出结构，且它已经把
`Inline.Start/End`（内联在源码里的字节区间）与 `runMark`（段落的 rune → 源码字节映射）备好了；
`spanStyle.mark` + `Theme.Mark` 也早就是 `<mark>` 的落地点。所以实现是：把一个区间列表在**生成 span 时**
按字节切开，命中段加背景。切段发生在四个已经存在的出口，各自很小：
段落的 `spanBuilder.add`、代码块的 `codeState.highlight`、HTML 块的 `ui.Text`、以及它们的缓存键。

**技术栈：** Go 1.27+、`ui/markdown` 与 `ui/markdown/parser`（纯 Go，无新依赖）、
`ui` 的 headless `Tester` 做测试、`ui/editor` 的 tokenizer（代码块已用，无需新增依赖）。

**规格：** 本文档即规格（上游 API 提案）+ 实现计划。本仓库侧的衔接见末尾「与 mdpreview 的衔接」。

## 为什么必须在 mygo 里做（而不是在应用里）

| 应用侧的做法 | 为什么不行 |
| --- | --- |
| 在源码里插 `<mark>` 标签 | ① 源码一变，`build()` 的 `strings.HasPrefix(src, doc.Source())` 判定失败 → **整篇重新解析**（每敲一个字一次）；② 代码块里标签会被当字面量显示；③ 「复制为 Markdown」会把 `<mark>` 带进剪贴板。 |
| 用 `Options.Block` 自己画命中的块 | 等于重写 `spans.go`（emphasis、链接、行内代码、图片、脚注、数学、任务框）并丢掉 `st.note` 的**选择/复制映射**。 |
| fork / 复制 `ui/markdown` | 被 Go 的 internal 规则封死：该包 import `mygo/internal/l10n` 与 `mygo/internal/text`，复制出来编译不了；只能整模块 `replace` = 自己维护 UI 框架。 |

因此唯一的干净解是在 mygo 的渲染层加一个「按源码区间上底色」的入口。

## 全局约束（沿用 mygo 仓库的既有约定）

- **不破坏既有语义**：新字段零值（`nil` / `0`）时行为与今天**逐字节**一致；`<mark>`、`Theme.Mark` 继续可用。
- **不重新解析**：`Mark` 变化只重建 span，`parser.Document` 不动。
- **不改源码**：`Document.Source()`、`copyData` / `copyMarkdown` / 选择映射都不受影响。
- **导出 API 的注释用英文散文**，与仓库现有风格一致（`Options` 每个字段 3~6 行说明「是什么、何时重读、边界行为」）。
- 界面文案走 `internal/l10n`（本次不需要新增文案）。
- `go test ./... && go vet ./... && gofmt -l .` 干净；提交信息沿用仓库既有前缀风格。

## API（实现者按此签名落地，不要另立方案）

```go
// MarkRange is a stretch of a view's source that Options.Mark marks, as a find
// bar's hits: its text takes Theme.Mark behind it, and a current one
// Theme.MarkCurrent.
type MarkRange struct {
	// Start and End are byte offsets in the source, Start before End. A range
	// out of the source, or one that ends before it starts, is ignored.
	Start, End int
	// Current marks the one the reader is on, as the hit a find bar counts
	// "3/17": it takes Theme.MarkCurrent rather than Theme.Mark.
	Current bool
}

// 加进 Options：
	// Mark are the stretches of the source the view marks, as a find bar marks
	// what it found: the text of each takes Theme.Mark behind it, and the
	// current one Theme.MarkCurrent. They are read whenever Mark or MarkVersion
	// changes, and marking redraws what they cover without parsing the source
	// again; the source itself is untouched, so what the reader copies is what
	// the document says. Ranges may come in any order and overlap: those out of
	// the source are left out, and overlapping ones mark as one. Blocks the app
	// builds itself (Options.HTML, Options.Math, Options.CodeBlock, Options.Code)
	// are its own to mark.
	Mark []MarkRange
	// MarkVersion marks the ranges again when it changes, for the ranges of
	// another hit with the same bounds as the last, as AnchorVersion does the
	// anchor. Like Anchor, a change of Mark is read as well.
	MarkVersion uint64

// 加进 Theme：
	// MarkCurrent is the background of the stretch the reader is on, among those
	// Options.Mark marks: Theme.Mark when zero.
	MarkCurrent Color
```

设计取舍（写进 PR 描述里）：

1. **一个字段带 `Current`，而不是两个切片**：`[]MarkRange` + 每条一个 `Current` 只排序合并一次，
   也不用在「当前项到底属于哪个切片」上做一致性检查。
2. **`MarkVersion` 与 `Mark` 一起被读**：沿用 `Anchor`/`AnchorVersion` 的既有约定（同一个标题点第二次要靠版本号），
   但额外比较切片本身，避免调用方忘了加版本号就静默不更新。
3. **只做背景**：命中高亮不改变字重/颜色/间距，所以**不影响文字量宽**，`para.measured` 之类的缓存不必失效。

## 渲染语义（要一条条落成测试）

| 情形 | 行为 |
| --- | --- |
| 区间落在普通段落 / 标题 / 列表项 / 引用里 | 命中的字符带 `Theme.Mark` 背景 |
| 命中跨行内样式（`自动**定期**检测`） | 按字节求交切段，两段各自带背景，视觉连成一条 |
| 命中落在代码块里 | 按 token span 逐段染（见任务 3），跨 token 的命中切成多段 |
| 命中落在表格单元格里 | 自动成立（单元格走同一套 `para`/`spanBuilder`），加测试即可 |
| 命中落在原始 HTML 块（`Options.HTML` 未设时显示源码） | 改为 `ui.RichText` 切段染（见任务 4） |
| 命中落在 `Options.HTML` / `Math` / `CodeBlock` / `Code` 自绘的块里 | **不染**（那是应用自己画的），文档写明 |
| 区间越界 / 颠倒 / 空（`Start >= End`） | 整条忽略，不 panic、不染 |
| 区间乱序 / 重叠 | 渲染器排序合并；重叠取并集，`Current` 优先 |
| 命中落在作者自己写的 `<mark>` 里 | 并存：作者那段用 `Theme.Mark`，搜索当前项用 `MarkCurrent` |
| 复制（文本 / HTML / Markdown）与选择 | **一字不差**：源码没有被改写，span 只是被切开 |
| 流式（`Stream` / `Append`） | 区间是源码偏移，按当前源码长度钳制；变长后越界部分自动不再染 |
| `Mark` / `MarkVersion` 变化 | 只重建受影响的 span；`parser.Document` 的 `Revision()` 不变 |

## 实现要点（mygo v0.9.1 的具体落点）

### 1. 状态与准备（`view.go`）

`viewState` 加：

```go
	// marks are the ranges Options.Mark asked for, as of markRev: sorted, merged
	// and clamped to the source, so that building a span is a lookup.
	marks []MarkRange
	// markRev changes whenever the ranges or the source's length do: the
	// paragraphs and the code blocks rebuild their spans then, and only then.
	markRev uint64
	// markIn and markVersion are what Options said in the frame before, and
	// markLen is the source's length the ranges were clamped to.
	markIn      []MarkRange
	markVersion uint64
	markLen     int
```

`build()` 里在 `st.theme(c)` 之后、**`st.doc.Set(src[:shown])` 之后**（`shown` 由 `st.reveal.step` 算好，
区间要按它钳制）、构建子元素之前：

```go
	if o.MarkVersion != st.markVersion || !slices.Equal(o.Mark, st.markIn) {
		st.markVersion, st.markIn = o.MarkVersion, append(st.markIn[:0], o.Mark...)
		st.markLen = len(src[:shown])
		st.markRev++
		st.marks = mergeMarks(o.Mark, st.markLen)
	} else if st.markLen != len(src[:shown]) {
		// The source grew or shrank: the ranges are the same, the source's
		// bounds are not.
		st.markLen = len(src[:shown])
		st.markRev++
		st.marks = mergeMarks(o.Mark, st.markLen)
	}
```

`mergeMarks(ranges, n)`（包内函数，`spans.go` 或新文件 `mark.go`）：丢掉 `Start < 0 || End > n || Start >= End`，
按 `Start` 排序，合并重叠与相邻（`Current` 取或），返回升序不重叠的切片。**放在 mygo 内部**而不是要求调用方有序：
调用方每帧重算区间，排序 500 条可忽略，能少一个踩坑点。

### 2. 段落与行内（`spans.go`）

`spanStyle` 加 `markCurrent bool`。把现有 `add`（`spans.go:126`）**拆成两个**：`add` 只负责按区间切段，
`addRun` 是今天的函数体（可再多带一个 `style spanStyle`）。

```go
// add adds text from src to end of the source in style s, cut where Options.Mark
// marks the source, so that the stretches marked take Theme.Mark behind them.
// A run that is not its source's bytes (an entity, an escape) is marked whole
// when it meets a range: cutting it would need the rune map, and the run is a
// few characters at most.
func (sb *spanBuilder) add(text string, src, end int, exact bool, s spanStyle, link int) {
	if text == "" {
		return
	}
	if !exact {
		if sb.meetsMark(src, end) {
			s.mark = true
			s.markCurrent = s.markCurrent || sb.meetsCurrent(src, end)
		}
		sb.addRun(text, src, end, false, s, link)
		return
	}
	at := sb.markCursor(src) // sort.Search over sb.marks, per run
	for ; at < len(sb.marks) && sb.marks[at].Start < end; at++ {
		m := sb.marks[at]
		lo, hi := max(m.Start, src), min(m.End, end)
		if lo >= hi {
			continue
		}
		if lo > src {
			sb.addRun(text[src-sb.runStart:lo-sb.runStart], src, lo, true, s, link) // 见下「偏移」
			src = lo
		}
		hit := s
		hit.mark, hit.markCurrent = true, m.Current
		sb.addRun(text[src-sb.runStart:hi-sb.runStart], src, hi, true, hit, link)
		src = hi
	}
	if src < end {
		sb.addRun(text[src-sb.runStart:end-sb.runStart], src, end, true, s, link)
	}
}
```

实现者注意三点（PR 描述里也写一下）：

1. **偏移基准**：`add` 的 `text` 是 `src..end` 的字节副本（`exact` 成立时），所以切片下标是 `x - runStart`
   而非 `x`。函数开头记 `runStart := src`。
2. **fade（流式淡入）也在这里切段**（`spans.go:130` 起）：`addRun` 内部的 fade 切分与本次切段互不干扰——
   淡入按不透明度切，命中按源码切；两者叠加时每段各取自己的 alpha（现有代码已经这么算）。
3. **`p.marks` 仍要按 rune 递增**：每条 `addRun` 记一条 `runMark`，切段不会破坏 `sourceAt()` 的二分
   （复制与选择靠它）。这一点由任务 6 的「复制不变」测试守住。

背景选择（`spans.go` 里今天 `if s.mark { sp.Background = th.Mark }` 的那一处）：

```go
	switch {
	case s.markCurrent:
		sp.Background = th.MarkCurrent
	case s.mark:
		sp.Background = th.Mark
	}
```

`Theme.MarkCurrent` 的回落放在主题合并之后（`theme.go` 的 `*out` 合并完、返回之前）：

```go
	if th.MarkCurrent.A == 0 {
		th.MarkCurrent = th.Mark
	}
```

### 3. 段落缓存（`view.go:564` 的 `st.para`）

`paraCache`（`view.go:282`）加 `markRev uint64`；比较条件加 `&& pc.markRev == st.markRev`；
赋值处 `pc.markRev = st.markRev`。**不要**把 `markRev` 放进 `b.Revision()` 或 `pc.rev`（那是解析的版本，
`Mark` 变化时解析并没有变）。

### 4. 代码块（`code.go`）

`codeState` 加 `markRev uint64`；`st.highlight(cs, i)`（`code.go:188`）开头：

```go
	if cs.markRev != st.markRev {
		cs.markRev = st.markRev
		for k := range cs.chunks {
			cs.chunks[k].ok = false
		}
	}
```

生成 span 的内层循环（`code.go:~240`）里，把 `line[start:end]` 按 `[行首偏移+start, 行首偏移+end)` 与
`st.marks` 求交切段；行首偏移来自 `d.Line(i, k)`（`parser/html.go:214`，第二个返回值是
「该行第一个字节在源码里的偏移」）：

```go
		line, lineAt := d.Line(i, l)
		// …按 toks 迭代，每个 token 的 [start,end) 变成若干 span：
		for _, seg := range segsOf(st.marks, lineAt+start, lineAt+end) {
			sp := ui.Span{Text: line[start:end][seg.a-(lineAt+start) : seg.b-(lineAt+start)], Color: ct.TokenColor(t.Kind)}
			// 粗体/斜体照旧
			if seg.mark {
				sp.Background = st.markColor(seg.current)
			}
			ch.spans = append(ch.spans, sp)
		}
```

注意：缩进代码块的行有 `pad`（前导空格）——`Line` 返回的字符串已含 pad，但**偏移仍是 `s.start`**，
所以 pad 那几个空格在源码里没有对应字节，切段时按「无命中」处理即可（不要为它们查区间）。

### 5. 原始 HTML 块（`view.go:491`）

```go
	case parser.HTMLBlock:
		html := strings.TrimRight(d.Lines(i), "\n")
		if st.o.HTML != nil { /* 不变：应用自绘 */ }
		// 未设 Options.HTML 时显示源码：用 RichText 切段，才能在命中上加底色。
		el := ui.RichText(c, st.markedSpans(html, b.Start, th.CodeFont, th.em(th.CodeSize), th.Muted)...)
		st.note(el.ID(), i, 0)
		return el
```

`st.markedSpans` 是包内小工具：按 `st.marks` 与 `[start, start+len(text))` 求交，产出若干 `ui.Span`（同字体、
同颜色，命中段加背景）。数学块（`parser.MathBlock`）**本期不动**（它很少被搜索命中，改了还要考虑 `Options.Math`）。

### 6. 表格

不改代码：`case parser.Table` 的单元格走 `st.para`，自动被第 1~3 点覆盖。任务 5 加测试证明。

## 任务

### 任务 1：API 与「段落里的命中」

**文件：**
- 创建：`ui/markdown/mark.go`（`MarkRange`、`mergeMarks`、`segOf` 之类的纯函数）
- 修改：`ui/markdown/view.go`（`Options.Mark` / `Options.MarkVersion`、`viewState` 的 `marks/markRev`、`build` 的准备、`paraCache.markRev`、`st.para`）、
  `ui/markdown/theme.go`（`Theme.MarkCurrent` + 回落）、`ui/markdown/spans.go`（`spanStyle.markCurrent`、`add` 切段、背景选择）
- 测试：`ui/markdown/mark_test.go`（新）

- [ ] 写失败的测试：段落命中着色、乱序/重叠/越界、`Current` 用 `MarkCurrent` 色、`MarkVersion` 换代。
- [ ] 跑 `go test ./ui/markdown/` → FAIL（编译：`undefined: MarkRange`）。
- [ ] 实现 `MarkRange` + `mergeMarks` + `Options`/`Theme` 字段 + `spanBuilder.add` 切段 + 段落缓存失效。
- [ ] 跑 `go test ./ui/markdown/` → PASS；`go test ./ui/...` 无回归（`<mark>` 的行为不变）。
- [ ] Commit：`feat: mark stretches of a source in paragraphs (Options.Mark)`。

### 任务 2：跨行内样式的命中

- [ ] 写失败测试：`自动**定期**检测` 里搜 `自动定期检测`，段落的两段（普通 + 强调内）都带背景；
      再搜一个落在行内代码 `` `code` `` 里的片段（**应该染**：行内代码也是文字，只是字体不同）。
- [ ] 实现：确认第 1 点的切段对 `Inlines` 的每一层都生效（`spanBuilder.inlines` 递归时 `add` 是同一条路径），
      不需要额外改动；若有遗漏，修 `add` 而非在调用处补。
- [ ] 跑 → PASS。

### 任务 3：代码块

- [ ] 写失败测试：Go 代码块里搜一个标识符（跨 token 的命中：搜 `func main` 之类跨关键字与函数名的片段）。
- [ ] 实现第 4 点（`codeState.markRev` + token span 切段）。
- [ ] 跑 → PASS；`go test ./ui/markdown/ -run Code -v` 无回归。

### 任务 4：原始 HTML 块

- [ ] 写失败测试：`<div class="x">自动定期检测</div>` 的 HTML 块里搜 `自动定期检测`。
- [ ] 实现第 5 点（`ui.RichText` 切段，保留 `st.note`）。
- [ ] 跑 → PASS。

### 任务 5：表格单元格与「不染自绘块」

- [ ] 写失败测试：表格单元格里的命中带背景；同时断言设了 `Options.CodeBlock` 的块**不**带背景（应用自绘，不在范围内）。
- [ ] 实现：无需改代码（只为把「表格自动覆盖」与「自绘块不覆盖」钉成契约）。
- [ ] 跑 → PASS。

### 任务 6：不重解析、复制不变、流式安全

- [ ] 写失败测试：
  - `markRev` 变化后 `doc.Revision()` 不变、块数不变、`Document.Source()` 不变；
  - `copyData` 的纯文本与 `copyMarkdown` 的输出在挂 `Mark` 前后**完全相同**；
  - `Stream` 场景：源码追加后，超出新源码长度的区间自动不再染，不 panic。
- [ ] 跑 → PASS（若第一条不成立，说明实现把 `Mark` 混进了解析路径，必须改实现而不是改测试）。
- [ ] 补基准 `BenchmarkMarkScrub`（583 块文档、500 区间、每帧换 `Current`），
      用 benchstat 对比不挂 `Mark` 的同一构建：目标 ≤ 1.3×，超过就按需优化（切段用光标而不是每段二分）。

### 任务 7：文档与导出注释

- [ ] `Options.Mark` / `Options.MarkVersion` / `MarkRange` / `Theme.MarkCurrent` 的 godoc 写完（英文，散文体，
      含「何时重读」「越界忽略」「自绘块不在范围内」「复制不受影响」四条边界）。
- [ ] 仓库里若有 markdown 视图的说明文档（`docs/` 或 README 的章节），补一段带例子的用法。
- [ ] CHANGELOG / release note 一条：`Options.Mark` 按源码区间给文字上底色，不重新解析、不改源码。

## 验收

- [ ] `go test ./... && go vet ./... && gofmt -l .` 干净；`go test -race ./ui/markdown/` 通过。
- [ ] 在 `mdpreview` 里做一次端到端验证（不改那个仓库的代码，只用 `replace`）：

```sh
# 在 mdpreview 里临时指向本地 mygo
go mod edit -replace github.com/egoist/mygo=../mygo
# 在 internal/view/find.go 的 refreshMatches/gotoMatch 里临时设：
#   s.md.Mark = rangesOf(s.find.matches, s.find.current)
#   s.md.MarkVersion++
go run .
```

  期望：原生后端搜一个词 → 正文里所有命中的文字带底色、当前项是另一种颜色、点 `↑`/`↓` 当前项换人、
  代码块里也高亮；同时**复制为 Markdown 的输出里没有多余字符**。
- [ ] 验证完 `go mod edit -dropreplace github.com/egoist/mygo`，别把 replace 提进 mdpreview。

## 与 mdpreview 的衔接（PR 合并发版后，另写一份计划）

1. `internal/view/find.go`：`refreshMatches()` / `gotoMatch()` 里把 `s.find.matches` 变成
   `[]markdown.MarkRange` 交给 `s.md.Mark`，`s.md.MarkVersion++`；跳转逻辑（`jump` / `jumpMarker`）**不动**。
2. 当前匹配用 `Current: true` + `s.md.Theme.MarkCurrent = c.Theme().Accent.Alpha(0.22)`，
   其余用 `s.md.Theme.Mark = c.Theme().Accent.Alpha(0.12)`（颜色从 `AppTheme` 取，符合本仓库硬规则）。
3. WebView 后端保持 `FindInPage`（浏览器自带高亮）；两边观感不同这件事写进 README 的「已知限制」。
4. 删掉 README「已知限制」里「原生渲染模式下搜索不高亮正文」一条，CHANGELOG 记一条**新增**。
5. `find.go` 的 `markedLine`（面板上下文预览里的着色）保留——它染的是面板自己的文本，与正文无关。

## 未决问题（PR 前先定，按括号里的默认推进）

1. 字段名用 `Mark`（与 `Theme.Mark`、`<mark>` 一致）还是 `Highlight`（本方案用 `Mark`）。
2. `Current` 放在区间里（本方案）还是单独的 `MarkCurrent int` 索引（前者少一个一致性检查）。
3. 是否也要覆盖 `Options.HTML` / `Math` / `CodeBlock` 自绘块（本方案：不覆盖，文档写明；真要覆盖应给这些钩子
   额外传 `Mark` 区间，属于另一件事）。
4. 排序合并放在渲染器（本方案，调用方省事）还是要求调用方保证有序。
5. 行内代码里的命中要不要染（本方案：**要**，它是文字，只是字体不同；若想与作者 `<mark>` 区分，用 `MarkCurrent` 之外
   再给一个 `Theme.MarkCode`，本期不做）。
