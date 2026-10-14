# oops — 产品 / UX 设计与技术方案（DESIGN）

> 版本：v1.0 · 日期：2026-10-14 · 配套：[PRD.md](./PRD.md)
> 技术选型（已定）：**Go**，单二进制；TUI 用 **Bubble Tea + Lip Gloss + Bubbles**（Charm 生态）；发布用 **goreleaser**；文件监控用 **fsnotify**。
> 阅读顺序：§1 用户旅程 → §2 交互原则 → §3 技术方案 → §4 CLI 命令 → §5 UI/UX 视觉规范 → 附录。
> 本文所有屏幕示例都是**规范**。实现应尽量逐字对齐（文案、顺序、留白），偏离时需要在 PR 中说明理由。

---

## 1. 用户旅程（从发现到出事再到反悔）

一条主线：**装上 → `oops init` 一次接好 → 忘了它 → 出事时只记得敲 `oops` → 回车撤销 → 后悔就 `oops redo`。**

```
发现 ──► 安装（10s）──► oops init（≤30s）──► 日常无感（数周）──► 出事：oops（≤10s）──► 撤销后：redo 兜底
 README       brew/npm/curl    检测→一次确认→完成      零输出，异常才轻提示      时间线→预览→回车             摘要 + 反悔提示
```

示例约定：`$` 是用户输入；`⏎` 表示回车；颜色用注释标出（详见 §5.1）；时间按用户本地时区显示。

### 1.1 发现与安装

**发现**：README 首屏只有三样东西：一句话、一张 GIF、一行安装命令。

```
oops — Ctrl+Z for coding agents.
Undo anything Claude Code, Codex, Gemini or Cursor did to your files. Even rm -rf.

  brew install oops-cli/tap/oops   ·   npm i -g @oops-cli/oops   ·   curl -fsSL https://oops.sh/install | sh
```
（计划中的渠道。org、scope、域名均为占位，待注册后替换。当前 README 只提供 `go install github.com/lintsinghua/oops/cmd/oops@latest` 和 GitHub Releases。）

**安装**：三种渠道安装完成后的最后一句提示**完全一致**，只指向唯一的下一步：

```
$ brew install oops-cli/tap/oops
==> Installing oops from oops-cli/tap
==> Caveats
  Next: run `oops init` to protect your agents.
🍺  /opt/homebrew/Cellar/oops/0.1.0: 6 files, 14.2MB
```

```
$ curl -fsSL https://oops.sh/install | sh
  Downloading oops 0.1.0 (darwin/arm64)…
  Verifying checksum and signature… ok
  Installed to /usr/local/bin/oops

  ✓ oops 0.1.0 installed.
    Next: oops init
```

```
$ npm i -g @oops-cli/oops
added 2 packages in 3s

  ✓ oops 0.1.0 installed.  Next: oops init
```
- npm 的 postinstall 输出可能被静默，所以第一次运行任何 `oops` 命令、发现还没有接入任何 Agent 时，也会给出同一句引导（见 §1.4 空状态）。
- curl 脚本：检测 OS/arch，校验 sha256 与 cosign 签名，不需要 sudo 时就不提权（默认装到 `~/.local/bin`，并提示 PATH）。

### 1.2 首次接入：`oops init`（≤ 30 秒，可撤销）

**目标**：自动检测、一次确认、全部接好；用户全程只需按一次回车。

```
$ oops init

  oops keeps a snapshot before every agent turn, so you can undo it.

  Found 3 agents
    ● Claude Code   2.1.301   ~/.claude/settings.json
    ● Codex         0.142.0   ~/.codex/hooks.json
    ● Gemini CLI    0.61.0    ~/.gemini/settings.json
    ○ Cursor CLI    not found

  oops will add its hooks to these 3 files. Backups are saved next to them.
  Snapshots stay on this machine in ~/.local/share/oops. Nothing is uploaded.

  Connect all 3? [Y/n/e] ⏎

  ✓ Claude Code   connected · /oops ready
  ✓ Codex         connected · open Codex and run /hooks once to trust oops
  ✓ Gemini CLI    connected

  You're protected.
  When an agent breaks something, just type: oops

  Undo this setup any time: oops init --undo
```
颜色：`●` 和 `✓` 绿色；`○` 和路径用 dim；“just type: oops”中的 `oops` 用强调色加粗。

**`e`（edit）**：进入多选列表，空格切换，回车确认（Bubbles 的 list 组件，或者 `huh.MultiSelect`）：

```
  Choose agents to connect   (space to toggle, enter to confirm)

  > [x] Claude Code   2.1.301
    [x] Codex         0.142.0
    [ ] Gemini CLI    0.61.0
```

**再次运行（幂等）**：

```
$ oops init
  ✓ Claude Code   already connected
  ✓ Codex         already connected
  ✓ Gemini CLI    already connected
  Nothing to do.
```

**撤销接入**：

```
$ oops init --undo

  Remove oops hooks from 3 agents?
    Claude Code   ~/.claude/settings.json
    Codex         ~/.codex/hooks.json
    Gemini CLI    ~/.gemini/settings.json
  Your other settings stay untouched.

  Remove? [y/N] y
  ✓ Removed from 3 agents.

  Snapshots kept (412 MB in ~/.local/share/oops).
  Delete them too: oops gc --all
```
- 撤销接入时默认 `[y/N]`：这一步会让保护失效，所以默认值偏向“不做”。
- 只删除 oops 自己写入的条目（每个条目都带 `oops` 标识，见 §3.5），用户的其他配置一行不动。

**边界情况**

| 情况 | 输出 |
|---|---|
| 一个 Agent 都没找到 | `No agents found. oops still works with any tool:  oops run -- <command>` 并列出支持的 Agent 和安装链接 |
| 配置文件 JSON 有语法错误 | `! Claude Code  ~/.claude/settings.json is not valid JSON (line 14). Skipped — fix it and run oops init again.`，其余 Agent 照常接入 |
| 非 TTY / CI | 不提问；没有 `-y` 时只打印检测结果和 `Run oops init -y to connect.`，退出码 2 |
| 项目级 | `oops init --project` 写入 `.claude/settings.json` 等可提交文件，便于团队共享 |

### 1.3 日常：无感

原则：**oops 正常工作时，用户看不到它。** hook 对 Agent 和用户都零输出，也不往 Agent 上下文注入任何内容。

只有在“oops 没能保护你”时才提示，而且要极轻：一行，`oops ·` 前缀，dim 颜色，同一原因每个会话最多一次。

在 Claude Code 中（通过 hook 的 `systemMessage`）：
```
  oops · skipped 1 large file (data/dump.sql, 1.2 GB) — not protected. oops doctor
```
```
  oops · snapshot took too long and was partial. Run oops doctor
```

用 `oops run` 包装时，只在开始和结束各打印一行：
```
$ oops run -- aider
oops · protecting ~/code/shop
…（aider 正常运行，oops 不插话）…
oops · 6 turns saved. Type oops to review.
```

不能显示 systemMessage 的 Agent：把提示暂存起来，等用户下次运行任何 `oops` 命令时，显示在输出顶部（显示后清除）。

### 1.4 出事时刻：只记得一个词 `oops`

用户慌了，只敲了一个词：

```
$ oops
```

直接进入全屏时间线（Bubble Tea，alt-screen）。默认选中最近一个已结束的轮；右侧预览就是“如果撤销会发生什么”。

```
 oops  ~/code/shop                                               Claude Code · Codex · Gemini ✓
 ───────────────────────────────────────────────────────────────────────────────────────────────
 ▸ #42  claude   2 min ago    "clean up the build scripts"      5 files  │  Undo #42 would:
   #41  claude   9 min ago    "add retry to payment client"     3 files  │
   #40  codex    24 min ago   "fix flaky checkout test"         1 file   │    restore  src/            38 files
   ·    you      31 min ago   edited 2 files outside agents              │    restore  .env
   #39  gemini   1 hr ago     "rename Order to Purchase"       14 files  │    remove   scripts/build.mjs
   #38  claude   yesterday    "set up eslint"                   6 files  │    revert   package.json   +3 −11
                                                                         │
                                                                         │  ran: rm -rf src && git clean -fdx
                                                                         │
 ───────────────────────────────────────────────────────────────────────────────────────────────
  ↑↓ select   enter undo   d diff   r rewind to here   / filter   ? help   q quit
```

要点：
- 时间用人类可读的写法（“2 min ago”“yesterday”）；prompt 摘要只取首行、截断，用引号括起来。
- 动作用人话：**restore**（这一轮删掉的 → 找回来）、**remove**（这一轮新建的 → 删掉）、**revert**（这一轮改过的 → 改回去）。不使用 git 的 M/A/D。
- `·` 行是 gap：两轮之间用户或别的进程做的改动，只展示，不能作为撤销目标（撤销轮时也不会动它们）。
- 被忽略但受保护的文件（如 `.env`）照常显示；如果 tracked 状态有意义，在右侧以 dim 标注 `untracked` / `ignored`。

**按 `d`：全屏 diff**（`↑↓`/`space` 翻页，`n/p` 跳到下一个/上一个文件，`esc` 返回）：

```
 #42 · revert package.json                                                         3/5 files
 ───────────────────────────────────────────────────────────────────────────────────────────────
  @@ -12,14 +12,6 @@
     "scripts": {
  -    "build": "node scripts/build.mjs",
  -    "dev": "vite",
  +    "build": "vite build",
     ...
 ───────────────────────────────────────────────────────────────────────────────────────────────
  n next file   p prev file   esc back
```
注意：diff 方向是“**撤销后会变成什么**”。`−` 是当前内容（会被撤掉，红色），`+` 是撤销后的内容（绿色）。顶部标题写明动作，避免方向混淆。

**按 `enter`：确认层**（居中卡片，默认焦点在 Undo；`enter` 确认，`esc` 取消）：

```
            ┌──────────────────────────────────────────────┐
            │  Undo turn #42?                               │
            │  "clean up the build scripts" · claude        │
            │                                               │
            │  restore 39 · remove 1 · revert 1             │
            │  Your changes after this turn are kept.       │
            │                                               │
            │        [ Undo ]    Cancel                     │
            └──────────────────────────────────────────────┘
```
确认后，在原位置显示进度（spinner；超过 300 ms 才出现），完成后退出 TUI，把结果摘要打印到普通屏幕（见 §1.5），保证摘要留在终端滚动历史中。

**快捷方式：`oops undo`（无交互）**。熟练用户的肌肉记忆：

```
$ oops undo
✓ Undid turn #42 · claude · "clean up the build scripts"

  restored  39 files   src/ (38), .env
  removed    1 file    scripts/build.mjs
  reverted   1 file    package.json

  Changed your mind? oops redo
```

**会话内 `/oops`**（在 Claude Code 里输入，本地执行，不经过模型，不消耗 token）：

```
> /oops
  ⎿ oops · last turns
      #42  2 min ago   "clean up the build scripts"    5 files
      #41  9 min ago   "add retry to payment client"   3 files
    /oops undo  undo #42   ·   /oops diff  preview   ·   type oops in a terminal for the full timeline
```
```
> /oops undo
  ⎿ ✓ oops · undid #42 — restored 39, removed 1, reverted 1.  /oops redo to bring it back
```
- 在 Codex、Gemini 中：优先用 UserPromptSubmit / BeforeAgent hook 拦截以 `/oops` 开头的输入，在本地执行并阻止它进入模型（block + reason 显示结果）。如果该 Agent 的 TUI 会在提交前吞掉未知的斜杠命令，则退回到 Agent 自带的 shell 逃逸：`!oops undo`。`oops init` 结束时按 Agent 告知用户可用的写法。
- 会话内不开 TUI；需要完整时间线时，提示用户到终端里敲 `oops`。

**空状态**（还没接入或还没有轮）：
```
$ oops
  No snapshots here yet.

  oops protects this folder once an agent runs in it.
  Not set up?  oops init        Any other tool?  oops run -- <command>
```

**目标轮仍在运行**（Agent 还在写）：时间线中该轮显示 spinner 和 `running`；选中它按 enter 时：
```
  #43 is still running (claude). Stop the agent first (Esc in Claude Code), then press enter.
  Undo anyway — snapshot now and revert: shift+enter
```

### 1.5 撤销之后：说清楚，并给出反悔路径

摘要的固定结构：**结果一句话 → 按动作分组的文件 → 没动的文件及原因 → 唯一的下一步。**

```
✓ Undid turn #42 · claude · "clean up the build scripts"

  restored  39 files   src/ (38), .env
  removed    1 file    scripts/build.mjs
  reverted   1 file    package.json

  Changed your mind? oops redo
```

文件很多时按目录折叠（`src/ (38)`），完整列表用 `oops show 42`。

**redo**：
```
$ oops redo
✓ Redid turn #42 — back to how it was before your undo (41 files).
```

**Agent 侧**（v0.2，上下文同步）：用户下次在该 Agent 里提交 prompt 时，oops 以事实陈述的形式告知模型“用户已用 oops 撤销第 42 轮，以下文件已回到该轮之前的状态：…”，避免 Agent 以为改动还在。

### 1.6 冲突与错误：友好、具体、可操作

统一格式：**`✗/!` + 一句人话说明发生了什么 → 缩进的细节 → 1–2 条可以直接复制的命令。** 不出现堆栈，不出现内部术语（manifest、blob 等）；`--debug` 才显示技术细节。

**冲突（这一轮之后文件又被改过）**：默认撤销其余文件，保留冲突文件：

```
$ oops undo
! Undid turn #42, except 1 file you changed afterwards.

  restored  39 files   src/ (38), .env
  reverted   1 file    package.json
  kept       1 file    scripts/build.mjs   edited by you 1 min ago

  Undo it too (your version is saved):  oops undo 42 --file scripts/build.mjs --force
  Changed your mind?                    oops redo
```
退出码 3（部分完成），脚本可以据此判断。TUI 中，冲突文件在预览里标黄 `kept`，确认卡片多一行 `1 file changed after this turn will be kept`。

**常见错误文案**

| 场景 | 输出 |
|---|---|
| 没有可撤销的轮 | `Nothing to undo here yet. oops protects this folder once an agent runs in it.` |
| 不在受保护的工作区 | `~/Downloads isn't protected. Run oops from your project folder, or oops init to set up.` |
| 轮号不存在 | `There's no turn #99 here. The latest is #42 — see oops list.` |
| 轮已被 GC 回收 | `Turn #12 is older than your retention (30 days) and was cleaned up.` |
| 文件被占用（Windows） | `✗ Couldn't write src/app.ts — it's open in another program.` 下一行：`Close it and run oops undo again. Nothing else was changed.` |
| 磁盘满 | `✗ Not enough disk space to undo safely (need 120 MB). Nothing was changed. Free space or run oops gc.` |
| 恢复中途被中断 | 下一次运行任何 oops 命令：`! The last undo was interrupted. Finishing it…  ✓ Done.`（journal 续做；无法续做时自动回滚到撤销前，并说明） |
| hooks 失效（例如 Agent 升级后配置被覆盖） | 时间线顶部一行：`! Codex isn't connected anymore. Run oops init to fix.` |
| 非 TTY 下需要确认 | `oops rewind needs confirmation. Re-run with -y to proceed, or -n to preview.`（退出码 2） |

---

## 2. 交互原则

参考 Linear（克制、键盘优先）、Vercel CLI（一步一句话、清晰的下一步）、GitHub CLI（交互和脚本两用、`--json`）、Raycast（一个入口、即时预览）。

1. **一个入口**：`oops` 就是入口。慌张的用户不需要记住任何子命令；子命令是熟练用户和脚本的快捷方式。
2. **安全默认值**
   - 无参数永远不变更；TUI 在确认前不变更。
   - 冲突默认保留用户版本；`--force` 才覆盖，而且被覆盖的版本进入安全快照。
   - 会让保护失效的操作（`init --undo`、`gc --all`）默认 `[y/N]`；可逆的操作（单轮 undo）不设确认关卡。
3. **危险操作前预览**：影响多轮（rewind）、强制覆盖、删除数据之前，必须展示“将发生什么”（按动作汇总的文件数和代表性路径），然后确认。所有变更性命令都支持 `-n/--dry-run`。
4. **可逆**：每一次变更都可以 `oops redo` 反悔；每条成功输出的最后一行都给出反悔路径。
5. **键盘优先**：TUI 中所有操作都有单键快捷键，底栏常驻提示；`?` 显示完整帮助；vim 键（`j/k`）和方向键都支持；鼠标只是辅助。
6. **人类可读**：时间写成“2 min ago / yesterday / Mar 3”，`--json` 才给 ISO 8601；prompt 摘要只取首行，最多 60 字符，用引号括起来；动作用 restore/remove/revert；体积用 KB/MB。
7. **一句话加一个下一步**：每次输出结尾最多给 1–2 条可复制的命令，不写长段落。
8. **脚本友好**：`--json` 输出稳定 schema（带 `schema_version`）；非 TTY 时不提问、不出 spinner、不用颜色；退出码有语义（§4.6）；stdout 放结果，stderr 放提示和进度。
9. **无色可用**：遵守 `NO_COLOR`、`--color=never`、`TERM=dumb`；无色时信息不丢失（动作词本身就有区分，不依赖颜色）；Windows 旧控制台和 `LANG` 非 UTF-8 时，符号退回 ASCII（§5.3）。
10. **不打扰**：hook 零输出；轻提示一会话一次；绝不使用表情符号、横幅或 ASCII art。
11. **快**：TUI 首屏 ≤ 150 ms；任何可能超过 300 ms 的操作才显示 spinner，超过 2 s 显示进度和数量。
12. **可预测**：同一信息在 TUI、普通输出、`/oops`、`--json` 中用词一致。

---
## 3. 技术方案

### 3.1 架构总览

```
            ┌────────────── Agent 进程（Claude Code / Codex / Gemini / Cursor / 任意）──────────────┐
            │  UserPromptSubmit / BeforeAgent / beforeSubmitPrompt   PreToolUse(v0.2)   Stop / AfterAgent │
            └───────────────┬───────────────────────────────────────┬──────────────────┬─────────────┘
                            │ stdin JSON                            │                  │
                     oops hook <agent> <event>  （短命进程，硬超时，永远 exit 0）
                            │
   oops run -- cmd ─────────┤（包装器：会话边界 + fsnotify 静默期切分）
   oops snap ───────────────┤
                            ▼
   ┌──────────────────────── internal/engine ────────────────────────┐
   │ turn.Manager（开轮/关轮/惰性关闭）  snapshot.Scanner（stat 缓存）  │
   │ restore.Planner（三方比对/冲突）    restore.Executor（journal/原子写）│
   └──────────┬──────────────────────────────┬───────────────────────┘
              ▼                              ▼
   store/objects（内容寻址，zstd）     meta.db（SQLite WAL：workspaces、turns、snapshots、ops）
              ▲
   CLI（cobra）· TUI（Bubble Tea）· --json
```

- **无常驻 daemon**（MVP）：每次 hook 都是一个短命进程，靠磁盘上的 stat 缓存做到增量。好处是没有后台进程，不会出现“oops 进程卡住”的问题，跨平台也更简单。`oops run` 只在包装期间存活。
- 所有状态都在用户数据目录，**工作区内不写任何文件**（`.oopsignore` 由用户自己创建）。

### 3.2 选型与依赖（Go）

| 用途 | 选择 | 理由 |
|---|---|---|
| 语言 | Go（≥ 1.23） | 单静态二进制、交叉编译简单、启动快（hook 冷启动 < 10 ms）、Charm 生态成熟、贡献者门槛低 |
| CLI | `spf13/cobra` | 与 gh 同源；能生成补全和文档 |
| TUI | `charmbracelet/bubbletea`、`bubbles`（list、viewport、spinner、key、help）、`lipgloss` | 已定 |
| 确认、多选 | `charmbracelet/huh`（或直接用 bubbles 组件） | 与 TUI 风格一致，支持无障碍模式 |
| 终端能力 | `charmbracelet/colorprofile`（或 `muesli/termenv`）、`mattn/go-isatty` | NO_COLOR、色深、TTY 检测 |
| 元数据 | `modernc.org/sqlite`（纯 Go，无 cgo），WAL 模式 | 多个 hook 进程并发安全，查询灵活，交叉编译不受影响 |
| 压缩 | `klauspost/compress/zstd` | 快，压缩率好 |
| 哈希 | 标准库 `crypto/sha256` | amd64 有 SHA-NI、arm64 有硬件加速，速度够用，零依赖；对象 ID 带算法前缀，便于以后换 BLAKE3 |
| ignore 匹配 | `go-git/go-git/v5/plumbing/format/gitignore` 或 `sabhiram/go-gitignore` | 支持嵌套 `.gitignore` 和 `!` 规则，**不依赖 git 二进制** |
| 文件监控 | `fsnotify/fsnotify` | 已定。只作为“提示”，正确性靠扫描（§3.7） |
| diff | `aymanbagabas/go-udiff`（Myers，unified） | 纯 Go，被 Charm 项目使用 |
| PTY（v0.2） | `creack/pty`（Unix）+ ConPTY 封装（如 `aymanbagabas/go-pty`） | `oops run` 识别轮次 |
| 配置 | `pelletier/go-toml/v2` | TOML |
| 测试 | `pgregory.net/rapid`、`charmbracelet/x/exp/teatest`、golden 文件 | property / TUI / 输出 |
| 发布 | goreleaser + cosign + syft + nfpm | §6 |

**为什么不用影子 git（不 shell out 到 git，也不用 go-git 存储）？** 两者都可行，Gemini CLI 用的就是影子 git，不过：

| 维度 | 影子 git（`GIT_DIR=~/.oops/x GIT_WORK_TREE=.`） | 自研 CAS（推荐） |
|---|---|---|
| 依赖 | 需要 git 二进制（Windows 上不一定有）；用户的 gitconfig、hooks、filter、autocrlf 可能干扰 | 零外部依赖 |
| 忽略规则 | 受 git 语义约束：被忽略但需要保护的 `.env` 要 hack | 自由组合 `.gitignore`、内置排除名单、保护名单和 `.oopsignore` |
| 性能 | `git add -A` 需要重写 index，大仓库每轮都慢（Codex ghost snapshot 的前车之鉴） | 自己的 stat 缓存，只哈希变化的文件；可以设硬超时和部分快照 |
| 字节保真 | 要关掉 autocrlf、filter 等 | 天然字节级 |
| 恢复 | `git restore` + `clean` 是整库语义 | 按文件做三方比对，冲突安全 |
| 开发量 | 小 | 中（约 1.5k 行核心代码） |

结论：用自研 CAS，数据模型借鉴 git（blob/tree、内容寻址、不可变对象）。开发方如果为了赶工先用影子 git 实现原型，必须满足 §3.3 的所有语义，并通过 §PRD 6.1 的测试矩阵。

### 3.3 快照与存储

**目录布局**（`$OOPS_HOME` 可覆盖）：

| 平台 | 数据目录 | 配置 |
|---|---|---|
| Linux | `$XDG_DATA_HOME/oops`（默认 `~/.local/share/oops`） | `$XDG_CONFIG_HOME/oops/config.toml` |
| macOS | `~/.local/share/oops`（与 Linux 一致，开发者更容易找到） | `~/.config/oops/config.toml` |
| Windows | `%LOCALAPPDATA%\oops` | `%APPDATA%\oops\config.toml` |

```
oops/
  objects/ab/cdef…            # 内容寻址对象：zstd(raw)，文件名 = sha256(raw)；全局共享，跨工作区、worktree 去重
  meta.db                     # SQLite（WAL）
  workspaces/<wsid>/
    statcache                 # 二进制：path → (size, mtime_ns, ino, mode, hash)，用于增量扫描
    journal/                  # 进行中的恢复操作日志
    pending-notices.json      # 待显示的轻提示
  logs/oops.log               # 轮转，≤ 5 MB × 3
```
`wsid` = sha256(工作区 realpath，规范化大小写和分隔符)[:16]；meta.db 中同时记录原始路径，便于 `doctor` 展示和以后重新关联。

**对象类型**
- `blob`：文件内容。
- `tree`：一个目录的条目列表（name、type file/dir/symlink、mode、size、blob/tree id、symlink 目标），按名字排序后序列化（自定义的紧凑二进制格式，带版本号）。**Merkle 结构**：目录内容不变时 tree id 不变，空轮几乎零存储。
- `snapshot` 记录在 meta.db 中：root tree id、时间、是否 partial、跳过的文件列表（路径、大小、原因）。

**扫描算法（增量）**
1. 从工作区根开始并发遍历（`filepath.WalkDir` + worker pool，并发数 = min(8, NumCPU)），在遍历中应用忽略规则；被排除的目录**整棵剪掉，不下钻**（例如 `node_modules`）。
2. 对每个文件做 lstat，与 statcache 比较 (size, mtime_ns, ino, mode)。完全一致就复用旧 hash；否则读文件、算 hash，对象不存在时压缩写入（先写临时文件，再 fsync 并 rename，保证原子性；对象不可变，所以多进程并发写安全）。
3. **racy-clean 处理**（借鉴 git）：文件 mtime 与上次扫描时间处在同一时间粒度窗口内时，下次强制重新哈希。
4. 读取过程中如果文件的 size 或 mtime 发生变化，最多重试 2 次，仍不稳定就标记为 `unstable`（记录下来，但这一轮不保证它）。
5. 由下往上构建 tree 对象，得到 root id，写入 snapshot 记录，更新 statcache（写临时文件再 rename）。
6. 硬超时（默认 10 s，`snapshot.timeout`）：超时后已经扫描的部分照常提交，未扫描的部分沿用上一个快照的条目，并标记 partial，同时登记轻提示。

**收录规则（按优先级从高到低）**
1. `.oopsignore` 中的 `!pattern`：强制收录。
2. `.oopsignore` 中的 pattern：排除。
3. 内置**保护名单**（即使被 gitignore 也收录）：`.env`、`.env.*`、`.envrc`、`*.local`、`*.local.*`、`.dev.vars`、`local.settings.json`。
4. 内置**排除名单**（即使没有被 gitignore 也排除）：`.git/`、`node_modules/`、`.venv/`、`venv/`、`__pycache__/`、`.mypy_cache/`、`.pytest_cache/`、`.ruff_cache/`、`target/`、`.gradle/`、`.next/`、`.nuxt/`、`.svelte-kit/`、`.turbo/`、`.parcel-cache/`、`.cache/`、`dist/`、`build/`、`out/`、`coverage/`、`.terraform/`、`.DS_Store`。（`dist/` 和 `build/` 只在被 gitignore 时才排除，避免误伤源码目录。）
5. `.gitignore` 规则（包括嵌套的 `.gitignore`、`.git/info/exclude` 和全局 excludesfile）：排除。
6. 其余全部收录：tracked、untracked、非 git 目录中的文件。

**大文件**：超过 `max_file_size`（默认 50 MB）的文件只记录元数据（size、mtime、hash 可选，默认不算），标记为 `skipped:large`。如果这个文件在一轮中被删除或修改，撤销时明确说明“无法恢复：超过大小上限”。
**工作区规模守卫**：首次扫描发现文件超过 200k 个或可收录内容超过 5 GB 时，`init` 和 `doctor` 会提示并建议 `.oopsignore`；在此之前，hook 用 partial 模式运行，不阻塞 Agent。
**工作区根的判定**：hook 的 `cwd` / `CLAUDE_PROJECT_DIR` / `GEMINI_PROJECT_DIR` → 向上找 `.git`，找到就以 git toplevel 为根，否则以 cwd 为根。**拒绝**以 `/`、`$HOME`、系统目录作为根，除非用户在配置中显式允许（避免扫描整个 home）。git worktree 各自是独立的工作区，对象全局去重。

**特殊文件**
- 符号链接：保存为 symlink 条目（保存目标字符串，不跟随）；恢复时重建链接本身。
- 硬链接：按普通文件保存内容；恢复时写新文件（会断开硬链接），并在输出中注明。
- 可执行位：保存 mode 的 0o111 位；Windows 上忽略。
- 空目录：记录在 tree 中，恢复时重建。
- 特殊文件（FIFO、socket、设备文件）：跳过。

### 3.4 “轮”模型

```sql
turns(id INTEGER, ws TEXT, seq INTEGER,            -- seq：工作区内递增的轮号（#42）
      agent TEXT, session_id TEXT, agent_turn_id TEXT,
      kind TEXT,                                    -- agent | auto | manual | safety
      prompt_summary TEXT,                          -- 首行，≤ 200 字符，已脱敏；可关闭
      started_at, ended_at, status TEXT,            -- running | done | undone | partial
      pre_snap TEXT, post_snap TEXT,
      stats JSON)                                   -- files、+/−、动作计数
ops(id, ws, type, target_turns, safety_snap, created_at, status)   -- undo/rewind/restore/redo 操作栈
steps(turn_id, idx, tool, summary, pre_snap)        -- v0.2
```

**生命周期**
1. **开轮**：在轮开始事件（CC `UserPromptSubmit`、Codex `UserPromptSubmit`、Gemini `BeforeAgent`、Cursor `beforeSubmitPrompt`）时拍 pre 快照，同步完成。如果同一个会话有未关闭的轮，先用当前状态关闭它。
2. **关轮**：在轮结束事件（CC `Stop`/`StopFailure`、Codex `Stop`/`Interrupt`、Gemini `AfterAgent`、Cursor `stop`）时拍 post 快照。
3. **惰性关闭**：如果没有收到结束事件（崩溃、kill），下一次开轮或任何 `oops` 命令执行时，把 post 设为当时的状态，status 记为 `done`，并在 stats 中标记 `closed_late`。
4. **gap**：`pre(N+1)` 与 `post(N)` 之间的差异就是轮外改动（用户编辑、后台 subagent 等），作为只读的时间线条目展示，不分配轮号。
5. **并发会话**：同一个工作区可能有多个 running 轮（多个会话）。每次快照都取全工作区状态，因此重叠的轮可能把对方的改动算进自己的 diff。处理方式：时间线标注 `overlaps #41`；撤销时依靠三方比对的冲突检测，保证不会误伤。MVP 接受这种归因不精确，v0.2 结合 PostToolUse 的文件路径做归因细化。
6. **安全快照**：每次变更性操作（undo/rewind/restore/redo）执行前，拍一个 kind=safety 的快照，挂在 ops 记录上，不出现在默认时间线里（`oops list --all` 可见）。

**轮号与引用语法**：`42` 或 `#42`；`last`（默认）；v0.2 增加 `42.3`（步骤）。不引入 `@~1` 这类 git 风格的语法，保持简单。

### 3.5 各 Agent 的接入方式

统一入口：`oops hook <agent> <event>`。它从 stdin 读取 JSON，解析时宽容（忽略未知字段），**任何错误都 exit 0、stdout 不输出**（除非需要返回 systemMessage 或拦截 `/oops`），日志写到 `logs/`。命令路径写成 `os.Executable()` 的结果，**不做 EvalSymlinks**，从而保留 `/opt/homebrew/bin/oops` 这类稳定的 shim 路径，避免升级后路径失效。每个写入的条目都带 `oops` 标识（命令行里包含 `oops hook`，便于识别、去重和卸载）。

| Agent | 配置位置 | 开轮 | 关轮 | 步骤（v0.2） | `/oops` | 备注 |
|---|---|---|---|---|---|---|
| **Claude Code** | `~/.claude/settings.json`（`--project`：`.claude/settings.json`） | `UserPromptSubmit`（同步；CC 对该事件的默认超时是 30 s，oops 显式设 `timeout: 15`） | `Stop`、`StopFailure`、`SubagentStop`（同步；hook 在毫秒级返回，post 快照仍在后台。`oops init` 会把已安装的 `async: true` 改写掉） | `PreToolUse` matcher `Bash\|PowerShell\|Edit\|Write\|MultiEdit\|NotebookEdit` | 安装 `~/.claude/commands/oops.md`，再用 `UserPromptExpansion`（matcher `oops`）拦截，在本地执行并 block，用 reason 显示结果 | `SessionStart` 时异步预热基线；`SubagentStop` 只记录不切轮；Windows 上用 exec 形式（`command`+`args`）避免 shell 引号问题；之后可以作为 plugin 分发 |
| **Codex** | `~/.codex/hooks.json`（`--project`：`.codex/hooks.json`） | `UserPromptSubmit`（入参带 `turn_id`） | `Stop`、`Interrupt` | `PreToolUse` matcher `Bash\|apply_patch` | `UserPromptSubmit` 检测到 `/oops` 前缀 → 本地执行，`{"decision":"block","reason":…}` 阻止进入模型；被 TUI 吞掉时退回 `!oops` | hooks 默认开启（`[features] hooks`），非托管 hooks **需要用户在 `/hooks` 中信任**，init 会提示；timeout 单位是秒 |
| **Gemini CLI** | `~/.gemini/settings.json` | `BeforeAgent` | `AfterAgent` | `BeforeTool` matcher `run_shell_command\|write_file\|replace` | `BeforeAgent` 拦截 `/oops` 前缀 → `decision: deny` 丢弃 prompt，用 reason 显示结果 | timeout 单位是毫秒（默认 60000）；每个 hook 需要 `name` 字段；用户开启了 Gemini 自带 checkpointing 也不冲突 |
| **Cursor CLI** | `~/.cursor/hooks.json`（`version: 1`） | `beforeSubmitPrompt`（**只有交互模式才触发，`-p` 模式不触发**） | `stop` | `beforeShellExecution`、`afterFileEdit` | 不支持拦截 → 提示用 shell 逃逸 | `-p` 模式退化为：在第一次 `beforeShellExecution` / `afterFileEdit` 前补拍 pre 快照，在 `stop` 时关轮；同时建议使用 `oops run -- cursor-agent` |
| **任意 Agent** | 无 | `oops run` 启动时 | 退出时 | — | — | 运行期间用 fsnotify 观察：“有活动 → 静默 ≥ 3 s”就切出一个 auto 轮；v0.2 通过 PTY 识别回车提交来切轮 |
| **脚本 / 自研 harness** | 无 | `oops snap -q` | `oops snap -q` | — | — | 也可以直接调用 `oops hook generic start|stop`，入参是最小的 JSON（cwd、session_id、prompt） |

**写入 Claude Code 的片段示例**（用户级；init 写入时会与已有的 hooks 合并，不覆盖）：

```json
{
  "hooks": {
    "SessionStart":      [{ "hooks": [{ "type": "command", "command": "/opt/homebrew/bin/oops", "args": ["hook","claude","session-start"], "async": true }] }],
    "UserPromptSubmit":  [{ "hooks": [{ "type": "command", "command": "/opt/homebrew/bin/oops", "args": ["hook","claude","turn-start"], "timeout": 15 }] }],
    "UserPromptExpansion": [{ "matcher": "oops", "hooks": [{ "type": "command", "command": "/opt/homebrew/bin/oops", "args": ["hook","claude","slash"] }] }],
    "Stop":              [{ "hooks": [{ "type": "command", "command": "/opt/homebrew/bin/oops", "args": ["hook","claude","turn-end"] }] }],
    "StopFailure":       [{ "hooks": [{ "type": "command", "command": "/opt/homebrew/bin/oops", "args": ["hook","claude","turn-end"] }] }],
    "SubagentStop":      [{ "hooks": [{ "type": "command", "command": "/opt/homebrew/bin/oops", "args": ["hook","claude","subagent-stop"] }] }]
  }
}
```

**Codex `hooks.json` 片段**：

```json
{
  "description": "oops — undo for coding agents (managed by `oops init`)",
  "hooks": {
    "UserPromptSubmit": [{ "hooks": [{ "type": "command", "command": "/opt/homebrew/bin/oops hook codex turn-start", "timeout": 15 }] }],
    "Stop":             [{ "hooks": [{ "type": "command", "command": "/opt/homebrew/bin/oops hook codex turn-end", "timeout": 15 }] }],
    "Interrupt":        [{ "hooks": [{ "type": "command", "command": "/opt/homebrew/bin/oops hook codex turn-end", "timeout": 3 }] }]
  }
}
```
（Gemini 和 Cursor 片段在实现时由 `oops init --print` 生成，同一份模板也放进文档。）

**适配器的代码约束**：`internal/agents/<name>/`，实现统一接口：

```go
type Adapter interface {
    Name() string                        // "claude"
    Detect(ctx) (Installation, bool)     // 二进制、版本、配置路径
    Install(opts) (Change, error)        // 返回要写入的 diff；dry-run 时只返回
    Uninstall(opts) (Change, error)
    Parse(event string, stdin []byte) (Event, error) // → TurnStart/TurnEnd/Step/Slash/SessionStart
    Respond(Event, Result) []byte        // 生成该 Agent 期望的 stdout（block、systemMessage…）
}
```
每个适配器都要附带：真实 payload fixture（`testdata/<agent>/<version>/*.json`）、Install/Uninstall 的 golden 测试（对各种已有配置做合并，验证无损）。

### 3.6 恢复算法（undo / rewind / restore / redo）

记 `pre = pre(N)`、`post = post(N)`、`cur` = 执行前对当前工作区扫描的结果。

1. **变更集合**：`C = diff(pre, post)` 中的路径（新增、删除、修改、类型变化、mode 变化）。`--file` 进一步过滤。
2. **逐路径判定**
   - `cur[p] == post[p]`（包括两者都不存在）→ 可以撤销：目标是 `pre[p]`（不存在则删除）。
   - `cur[p] == pre[p]` → 已经是目标状态，跳过（幂等）。
   - 其他情况 → **冲突**：默认 `kept`；`--force` 时按目标覆盖。
3. **目录**：删除文件后，如果它的父目录是本轮创建的（不在 pre 中）而且已经为空，就一并删除；从不删除 pre 中已经存在的目录。
4. **执行**
   1. 拍安全快照（只针对受影响的路径时可以走快速路径）。
   2. 写 journal：计划（路径、动作、目标对象 id）和状态。
   3. 按“先写入/恢复，后删除”的顺序执行。每个文件都先写到同目录的临时文件，然后 fsync、设置 mode，再 rename 覆盖；Windows 上用 `MoveFileEx(REPLACE_EXISTING)`，遇到共享冲突时重试 3 次（50/150/400 ms）。
   4. 逐条标记完成；全部完成后 journal 标为 committed，写入 ops 记录，把轮 status 设为 `undone`。
   5. 中断恢复：启动任何命令时先检查未完成的 journal，能续做就续做，否则用安全快照回滚，并告诉用户。
5. **路径安全**：所有路径都是相对路径，拒绝 `..`、绝对路径和 Windows 设备名；写入前逐级检查父目录，**不跟随指向工作区外的符号链接**（Unix 用 `openat` + `O_NOFOLLOW` 语义，Windows 检查 reparse point）。
6. **rewind N**：等价于把目标设为 `pre(N)`，变更集合是从 N 到最新一轮所有轮的变更并集；gap 中用户的改动按冲突规则保留。
7. **restore p --at N**：目标取 `pre(N)[p]`；当前文件存在且内容不同时，需要确认或 `--force`。
8. **redo**：取 ops 栈顶的安全快照，对该操作涉及的路径恢复到安全快照的状态（同样走三方比对：当前等于操作后的状态才回退，否则判为冲突）。

### 3.7 跨平台与文件监控

| 主题 | macOS | Linux | Windows |
|---|---|---|---|
| 大小写 | APFS 默认大小写不敏感：路径比较用规范化后的 key；支持仅大小写不同的重命名（先改成临时名，再改成目标名） | 敏感 | 不敏感，处理方式同 macOS |
| Unicode | NFD/NFC：存储时保留原始字节，比较时用 NFC key | — | — |
| 长路径 | — | — | 使用 `\\?\` 前缀；manifest 中统一用 `/` 作分隔符 |
| 权限 | mode | mode | 只记录只读属性；不处理 ACL |
| 符号链接 | 支持 | 支持 | 需要开发者模式或管理员权限；没有权限时跳过并明确报告 |
| 文件锁 | — | — | 共享冲突时重试；仍失败就整体中止（还没写的不写），并提示是哪个程序占用 |
| fsnotify | 后端是 kqueue，**每个文件占一个 fd，大仓库会耗尽**：只监控目录，超过阈值（5k 个目录）就退化成 2 s 轮询加 stat 缓存 | inotify 不递归：逐目录添加；`max_user_watches` 不足时由 doctor 提示，并退化成轮询 | ReadDirectoryChangesW（递归）表现良好 |
| hook 执行 | `sh -c` / exec | 同左 | CC 默认用 Git Bash 或 PowerShell：写成 exec 形式，`command` 是 `oops.exe` 的绝对路径 |
| 原生快照（未来） | APFS `clonefile` 加速恢复 | btrfs/XFS `FICLONE` | ReFS block clone |

**fsnotify 的定位**：**只是提示，不保证正确**（事件可能丢失或合并）。正确性永远由“开轮/关轮时的全量增量扫描”保证；fsnotify 只用于 `oops run` 的静默期切轮，以及（未来的）daemon 预热 statcache。

### 3.8 性能与存储预算

- 目标见 PRD §6.2。实现上的关键点：
  - hook 进程冷启动：避免在 `init()` 中做重活；SQLite 懒加载；`oops hook` 走最短路径（不初始化 cobra 的全部命令树，在 `main` 中快速分流）。
  - 扫描：被排除的目录整棵剪掉；并发 lstat；statcache 用 mmap 友好的定长记录，加载 10 万条 < 20 ms。
  - 哈希只针对变化的文件；超过 8 MB 的文件流式哈希，同时压缩。
  - 增量快照的写入量 = 变化文件的压缩体积 + 变化路径上的 tree 节点。
- **存储预算**：每个工作区默认 2 GB，全局 10 GB，最近 500 轮或 30 天。GC：从所有保留的快照（含安全快照和最近 20 个 ops）出发标记可达对象，清除不可达对象；新对象有 1 小时宽限期，避免和并发写入竞争；GC 期间持有全局锁文件，hook 不需要等待 GC（hook 只新增对象）。
- **基准测试**（`go test -bench`，放在 `bench/` 下）：合成仓库（1k / 10k / 100k 文件，混合大小），以及 linux kernel 子集等真实仓库；指标包括 turn-start p50/p95、undo 时长、存储增量；CI 中用 benchstat 与 main 分支对比。

### 3.9 安全与隐私

- **威胁模型**：本机单用户工具。保护目标是“数据不外泄、不被同机其他用户读取、恢复时不越界写入”。
- 数据目录 0700，文件 0600（Windows 上设置仅当前用户可访问的 ACL）。
- **零网络**：核心路径不发起任何网络请求；没有遥测，也没有自动更新检查（`oops version --check` 由用户手动触发，是唯一的联网点）。
- prompt 摘要：默认记录首行，最多 200 字符，并用规则（gitleaks 风格的常见 key 正则、长 base64 串、`password=` 等）替换成 `[redacted]`；`privacy.record_prompts = false` 可以完全关闭。
- 快照包含 `.env` 等敏感文件：README 和 SECURITY.md 中明确说明；`oops forget <path>`（v0.2）可以从所有快照中清除指定路径（重写 tree 后 GC）；`oops gc --all` 删除全部数据。
- 配置写入：修改 Agent 配置前先备份（`settings.json.oops-backup-<ts>`），通过“临时文件 + rename”原子写；保留原有的 JSON 格式和键顺序（用保序 JSON 处理），只增删 oops 自己的条目。
- 恢复路径安全：见 §3.6 第 5 条。
- hook 输入不可信：把 payload 当作数据处理，不执行其中的内容；不用 shell 拼接命令。
- 供应链：Go module 最小依赖；`govulncheck`；release 产物用 cosign keyless 签名，附 SBOM 和 SLSA provenance；安装脚本校验签名。

### 3.10 代码组织

```
cmd/oops/main.go                 # 快速分流：hook 路径不加载 TUI
internal/
  cli/        cobra 命令定义、输出渲染（human/json）
  tui/        Bubble Tea 模型：timeline、diffview、confirm、help
  ui/         Lip Gloss 主题、符号、颜色降级、人类时间、路径折叠
  engine/
    store/    对象存储（put/get/has、zstd、原子写）
    snapshot/ 扫描器、statcache、ignore 规则、tree 构建
    turn/     轮生命周期、gap、惰性关闭
    restore/  Planner（三方比对）、Executor（journal）
    gc/
  meta/       SQLite schema 与迁移
  agents/     claude/ codex/ gemini/ cursor/ generic/
  run/        包装器、fsnotify 静默期检测、PTY（v0.2）
  config/
  platform/   各 OS 的差异（长路径、ACL、文件锁、clonefile）
docs/  bench/  scripts/install.sh  .goreleaser.yaml  vhs/demo.tape
```

### 3.11 配置（`config.toml`，所有键都有合理的默认值）

```toml
[snapshot]
max_file_size = "50MB"
timeout = "10s"
protect = [".env", ".env.*", ".envrc", "*.local", "*.local.*"]   # 追加到内置名单
exclude = []                                                      # 追加到内置名单
respect_gitignore = true

[retention]
keep_turns = 500
keep_days = 30
workspace_max = "2GB"
global_max = "10GB"

[privacy]
record_prompts = true
prompt_max_chars = 200

[ui]
color = "auto"          # auto | always | never
pager = "auto"          # 默认 $PAGER，否则 less -FRX
time = "relative"       # relative | absolute

[run]
idle_split = "3s"
```

---
## 4. CLI 命令设计

风格参考 git（动词短、语义稳定）、gh（交互与 `--json` 两用）、Vercel（每一步一句话）、Linear（克制）、Stripe CLI（清晰的错误和下一步）。

### 4.1 命令总览

```
$ oops --help
Ctrl+Z for coding agents.

Usage
  oops                     Open the timeline (pick a turn, preview, undo)
  oops <command> [flags]

Undo
  undo [turn]              Undo the last turn, or a specific one
  rewind <turn>            Go back to before a turn (undoes it and everything after)
  redo                     Reverse the last undo, rewind or restore
  restore <path>... --at <turn>
                           Bring back files as they were before a turn

Inspect
  list                     List recent turns
  show [turn]              Show what a turn changed
  diff [turn]              Show a turn's diff

Setup
  init                     Connect oops to the agents on this machine
  run -- <command>         Protect any tool that has no hooks
  snap                     Save a checkpoint now
  doctor                   Check that everything works
  gc                       Free up snapshot storage

Flags
  -h, --help     --version     --json     --color <auto|always|never>     -C <dir>

Learn more: https://oops.sh/docs
```
隐藏命令：`hook`（Agent 调用）、`completion`（cobra 生成）、`version`。

### 4.2 全局 flag
- `-C <dir>`：在指定目录运行，对应的工作区（同 git）。
- `--json`：机器可读输出，隐含 `--color=never`，不提问、不进 TUI。
- `--color auto|always|never`；遵守 `NO_COLOR`、`CLICOLOR_FORCE`。
- `-q/--quiet`：只输出错误。
- `--debug`：在 stderr 输出技术细节。

### 4.3 各命令

**`oops`**：TTY 下进入 TUI（§1.4）。非 TTY 下打印：
```
$ oops | cat
~/code/shop · protected (claude, codex, gemini)
#42  claude  2 min ago   "clean up the build scripts"   5 files
#41  claude  9 min ago   "add retry to payment client"  3 files
#40  codex   24 min ago  "fix flaky checkout test"      1 file
```

**`oops undo [turn]`**

| flag | 说明 |
|---|---|
| `-f, --file <path>...` | 只撤销这些文件（支持 glob） |
| `-n, --dry-run` | 预览，不执行 |
| `--force` | 覆盖冲突文件（被覆盖的版本保存在安全快照中） |
| `-y, --yes` | 跳过确认（只在需要确认的情况下有意义） |
| `--running` | 目标轮仍在运行时，立即关闭该轮再撤销 |

行为：`oops undo`（最后一轮）直接执行并打印摘要（§1.4、§1.5），因为可以 `oops redo` 反悔；指定的不是最后一轮，或者带了 `--force` 时，TTY 下先预览再确认（`[Y/n]`），非 TTY 下需要 `-y`。

```
$ oops undo -n
Undo turn #42 · claude · 2 min ago · "clean up the build scripts"

  restore  src/              38 files
  restore  .env
  remove   scripts/build.mjs
  revert   package.json      +3 −11

Dry run — nothing changed. Run oops undo to apply.
```

**`oops rewind <turn>`**（TTY 下需要确认）：
```
$ oops rewind 39
Go back to before turn #39? This undoes 4 turns:

  #42  claude   "clean up the build scripts"     5 files
  #41  claude   "add retry to payment client"    3 files
  #40  codex    "fix flaky checkout test"        1 file
  #39  gemini   "rename Order to Purchase"      14 files

  21 files will change. Your own edits in between are kept.

Rewind? [Y/n] ⏎
✓ Rewound to before #39 — 21 files changed.

  Changed your mind? oops redo
```

**`oops redo`**：无参数；没有可以反悔的操作时输出 `Nothing to redo.`。

**`oops restore <path>... --at <turn>`**：
```
$ oops restore config/local.yml --at 31
✓ Restored config/local.yml as it was before turn #31 (2.1 KB).
  Changed your mind? oops redo
```
目标文件已存在且内容不同时：TTY 下询问 `Overwrite config/local.yml? [y/N]`；非 TTY 下需要 `--force`。

**`oops list`**（别名 `ls`）：`-n <N>`（默认 10）、`--all`（含 safety、undone）、`--agent <name>`、`--since <dur>`、`--json`。
```
$ oops list -n 4
  #42  claude   2 min ago    "clean up the build scripts"     5 files  +12 −340
  #41  claude   9 min ago    "add retry to payment client"    3 files  +88 −4
  #40  codex    24 min ago   "fix flaky checkout test"        1 file   +2 −2     undone
  ·    you      31 min ago   2 files changed outside agents
```

**`oops show [turn]`**：
```
$ oops show 42
Turn #42 · claude · today 14:03–14:05 (2m 11s) · session 7f3a…
"clean up the build scripts"

  deleted   src/              38 files            −312
  deleted   .env                                  ignored · protected
  added     scripts/build.mjs                     +9
  modified  package.json                          +3 −11
  ! skipped data/dump.sql — 1.2 GB, over the 50 MB limit (not protected)

Undo: oops undo 42      Diff: oops diff 42
```
注意：`show` 和 `diff` 描述的是**这一轮做了什么**（deleted/added/modified）；`undo` 预览和 TUI 描述的是**撤销会做什么**（restore/remove/revert）。两套用词分开，各自一致。

**`oops diff [turn] [-- path...]`**：`--stat`、`--name-only`、`--reverse`（显示撤销方向的 diff）、`--no-pager`。默认显示这一轮的正向 diff（与 git 习惯一致）。

**`oops init`**：`-y`、`--project`、`--print`、`--undo`、`--agent <name>...`（只处理指定的 Agent）。输出见 §1.2。

**`oops run [--name <label>] -- <command> [args...]`**：透传 stdio、信号和退出码；`--idle <dur>` 覆盖静默切分阈值。

**`oops snap [-m <message>] [-q]`**：
```
$ oops snap -m "before db migration"
✓ Saved checkpoint #43 "before db migration".  Go back: oops rewind 43
```

**`oops doctor`**：
```
$ oops doctor
  ✓ oops 0.1.0 (darwin/arm64) at /opt/homebrew/bin/oops
  ✓ Claude Code   hooks installed, last fired 2 min ago
  ! Codex         hooks installed but not trusted yet — open Codex and run /hooks
  ✓ Gemini CLI    hooks installed
  ✓ Workspace     ~/code/shop · 8,412 files · 212 MB protected
  ! 1 file too large to protect: data/dump.sql (1.2 GB)   → add it to .oopsignore, or raise snapshot.max_file_size
  ✓ Storage       412 MB of 2 GB · 186 turns
  ✓ Last snapshot 48 ms

  2 warnings.
```

**`oops gc`**：`-n`（显示可以释放多少）、`--all`（删除本工作区全部数据，需要确认；`--all --global` 删除全部）。

### 4.4 输出流
- stdout：结果（列表、diff、摘要、JSON）。
- stderr：提示、警告、进度、确认问题。
- 管道中自动去掉颜色、spinner 和对齐填充以外的装饰。

### 4.5 `--json` 示例

```json
{
  "schema_version": 1,
  "workspace": "/Users/lin/code/shop",
  "turns": [
    {
      "id": 42, "agent": "claude", "session_id": "7f3a…", "kind": "agent",
      "status": "done", "started_at": "2026-10-14T14:03:12+08:00", "ended_at": "2026-10-14T14:05:23+08:00",
      "prompt": "clean up the build scripts",
      "stats": { "files": 5, "added": 1, "modified": 1, "deleted": 39, "insertions": 12, "deletions": 340 },
      "skipped": [{ "path": "data/dump.sql", "reason": "too_large", "size": 1288490188 }]
    }
  ]
}
```
`undo --json` 返回 `{ "result": "ok|partial|refused", "restored": [...], "removed": [...], "reverted": [...], "kept": [{"path":…, "reason":"changed_after_turn"}], "redo": "oops redo" }`。

### 4.6 退出码

| 码 | 含义 |
|---|---|
| 0 | 成功 |
| 1 | 一般错误（I/O、内部错误） |
| 2 | 用法错误或需要确认但无法交互 |
| 3 | 部分完成（有冲突文件被保留） |
| 4 | 目标不可操作（轮仍在运行、轮不存在、已被回收、没有可撤销的轮） |
| 130 | 用户中断（Ctrl+C，此时保证没有半写入的文件） |

---

## 5. UI/UX 视觉规范

总原则：**优雅但不花哨**。一个强调色；靠留白和对齐建立层级，不用框线堆砌；不用表情符号；不画横幅。

### 5.1 色板（Lip Gloss `AdaptiveColor`，自动适配深色和浅色终端）

| 语义 | 用途 | 深色终端 | 浅色终端 | 256 色降级 |
|---|---|---|---|---|
| accent | 品牌、选中行指示 `▸`、关键命令 | `#F5A524`（琥珀） | `#B76E00` | 214 / 130 |
| success | `✓`、restore、`+` 行 | `#3FB950` | `#1A7F37` | 71 / 28 |
| danger | `✗`、remove、`−` 行 | `#F85149` | `#CF222E` | 203 / 160 |
| warning | `!`、kept、冲突 | `#D29922` | `#9A6700` | 178 / 136 |
| info | revert、修改 | `#58A6FF` | `#0969DA` | 75 / 25 |
| muted | 时间、路径前缀、提示、gap | `#8B949E` | `#57606A` | 245 / 242 |
| fg | 正文 | 终端默认前景色 | 终端默认前景色 | — |

- 不设置背景色（尊重用户的终端主题）。唯一的例外是 TUI 选中行用很浅的反色背景（`lipgloss.AdaptiveColor{Light:"#EAEEF2", Dark:"#1F2428"}`）。
- 颜色只是强化手段：所有信息都有文字或符号承载，无色时不丢失信息。
- 色深降级链：TrueColor → 256 → 16 → 无色（`colorprofile` 自动检测）。

### 5.2 排版
- 左缩进 2 个空格作为内容区；标题行（结果句）顶格。
- 列对齐：轮号右对齐，Agent 列固定 7 字符，时间列固定 11 字符，prompt 列自适应宽度后截断，用 `…` 结尾（按显示宽度计算，正确处理 CJK 宽字符，使用 `lipgloss.Width` / `go-runewidth`）。
- 段落之间空一行，输出结尾不留多余空行。
- 路径：工作区内显示相对路径；工作区本身用 `~` 缩写；大量文件按目录折叠为 `src/ (38)`。
- 数字：`1 file` / `3 files` 单复数正确；行数用 `+12 −340`（使用 U+2212 减号；ASCII 模式下用 `-`）。
- 时间：< 1 min “just now”；< 1 h “N min ago”；当天 “N hr ago”；昨天 “yesterday”；一周内 “Mon”；更早 “Mar 3”；跨年 “Mar 3, 2025”。`--json` 一律输出 ISO 8601 带时区。
- 文案语气：主动语态、现在时、简短；先说结果后说细节；不道歉，不说 “Oops!”（品牌名已经够了）。

### 5.3 符号

| 用途 | Unicode | ASCII 回退 |
|---|---|---|
| 成功 | `✓` | `ok` |
| 失败 | `✗` | `x` |
| 警告 | `!` | `!` |
| 选中 | `▸` | `>` |
| 已检测或已接入 | `●` | `*` |
| 未检测到 | `○` | `-` |
| gap | `·` | `.` |
| 分隔 | `·` | `-` |

满足以下任一条件时回退 ASCII：`TERM=dumb`、Windows 旧控制台（conhost 且没有开启 VT）、语言环境不是 UTF-8、`OOPS_ASCII=1`。

### 5.4 diff 预览
- unified 格式，3 行上下文；hunk 头为 muted 色；`+` 行 success 色，`−` 行 danger 色；v0.2 增加词级高亮（变化部分加粗或下划线，不用背景色块）。
- 每个文件前一行标题：`动作 path  +a −b`（例如 `revert package.json +3 −11`）。
- 二进制文件：`binary · 1.2 MB → 0 B`。新增或删除的大文件（> 2000 行）：只显示前 50 行，再加 `… 1,950 more lines · oops diff 42 -- path`。
- 超过一屏：普通输出走 pager（`$PAGER`，默认 `less -FRX`），TUI 中用 viewport 滚动。
- 方向：`diff` 命令默认是这一轮的正向 diff；TUI 预览和 `undo -n` 用撤销方向，并在标题中写明动作，避免歧义。

### 5.5 确认交互
- 命令行确认：`Question? [Y/n]`，大写字母是默认值，回车即默认；Ctrl+C 或 Esc 取消并输出 `Cancelled. Nothing changed.`（退出码 130）。
- 默认值规则：可逆操作默认 Y；让保护失效或删除数据的操作默认 N。
- 确认前一定有预览：确认问题之前总有按动作汇总的变更列表。
- 非交互：不提问；需要确认的操作没有 `-y` 时以退出码 2 失败，并给出带 `-y` 的完整命令。
- TUI 确认卡片：居中，圆角边框（`lipgloss.RoundedBorder()`，muted 色），这是 TUI 中唯一使用边框的地方；按钮 `[ Undo ]` 为 accent 色，`Cancel` 为 muted 色；`←→/tab` 切换焦点，`enter` 确认，`esc` 取消，`y/n` 直接选择。

### 5.6 TUI 规范（Bubble Tea）
- **布局**：顶栏（工作区和 Agent 接入状态）+ 主体（左边时间线占 55%，右边预览占 45%，中间用 muted 色的 `│` 分隔）+ 底栏（按键提示，使用 `bubbles/help` 的短格式）。宽度 < 80 列时单栏，`enter` 进入详情。高度 < 15 行时隐藏顶栏。
- **键位**

| 键 | 动作 |
|---|---|
| `↑/↓`、`k/j` | 选择轮 |
| `g/G` | 第一轮 / 最后一轮 |
| `tab` | 焦点在时间线和文件列表之间切换 |
| `enter` | 在时间线上：撤销所选轮（确认卡片）；在文件列表上：打开该文件的 diff |
| `d` | 全屏 diff |
| `r` | rewind 到所选轮之前（确认卡片，列出受影响的轮） |
| `u` | 等同于 enter |
| `R` | redo（底栏在有可反悔的操作时显示） |
| `/` | 过滤（匹配 prompt、路径、Agent） |
| `a` | 显示或隐藏 gap 和 safety 条目 |
| `?` | 完整帮助 |
| `q`、`esc`、`ctrl+c` | 退出（在弹层中时先关闭弹层） |

- **状态**：running 轮用 `spinner.MiniDot`（muted 色）；undone 轮整行 muted，并带删除线或 `undone` 标签；冲突文件在预览中标黄 `kept`。
- **性能**：首屏只加载最近 50 轮（滚动时分页）；diff 在选中后懒计算，并做 LRU 缓存；渲染不阻塞，计算放在 `tea.Cmd` 中完成。
- **无障碍**：`OOPS_ACCESSIBLE=1` 或 `--accessible` 时不进 alt-screen，改用逐行提问（`huh` 的 accessible 模式），方便屏幕阅读器。
- **退出后**：执行了操作的，摘要打印在普通屏幕上（不随 alt-screen 消失）；没有执行操作的，不留任何输出。

### 5.7 错误提示规范
- 结构：`✗ 发生了什么（一句话）` → 缩进的原因或细节（可选）→ 可复制的修复命令（1–2 条）→ 必要时说明“Nothing was changed.”以安抚用户。
- 不暴露堆栈；`--debug` 才输出技术细节和日志路径。
- 涉及数据安全时，一定说明数据的状态（“Nothing was changed.” / “Your version is saved.”）。
- 示例集中维护在 `internal/ui/messages.go`，并做 golden 测试，保证文案一致。

### 5.8 可选：Web 时间线（未来）
- `oops --web`：在 127.0.0.1 的随机端口启动只读服务，URL 带一次性 token，自动打开浏览器；写操作需要在终端确认。
- 视觉延续 CLI：单一 accent 色、等宽字体显示 diff、时间线左对齐；不使用图表和仪表盘。
- 用途：阅读长 diff、截图分享、演示。

---

## 附录 A：发布工作流要点（goreleaser）

```yaml
# .goreleaser.yaml（节选）
version: 2
builds:
  - main: ./cmd/oops
    env: [CGO_ENABLED=0]
    goos: [darwin, linux, windows]
    goarch: [amd64, arm64]
    flags: [-trimpath]
    ldflags: ["-s -w -X main.version={{.Version}} -X main.commit={{.Commit}} -X main.date={{.Date}}"]
archives:
  - formats: [tar.gz]
    format_overrides: [{ goos: windows, formats: [zip] }]
checksum: { name_template: checksums.txt }
sboms: [{ artifacts: archive }]
signs: [{ cmd: cosign, artifacts: checksum, args: ["sign-blob", "--output-signature=${signature}", "${artifact}", "--yes"] }]
nfpms: [{ formats: [deb, rpm, apk], bindir: /usr/bin }]
brews: [{ repository: { owner: oops-cli, name: homebrew-tap }, caveats: "Next: run `oops init` to protect your agents." }]
scoops: [{ repository: { owner: oops-cli, name: scoop-bucket } }]
winget: [{ publisher: oops-cli, repository: { owner: oops-cli, name: winget-pkgs } }]
changelog: { use: github, groups: [{ title: Features, regexp: '^feat' }, { title: Fixes, regexp: '^fix' }] }
```
- npm 分发：`@oops-cli/oops`（主包，含 JS shim 和 postinstall 提示）加上 `@oops-cli/oops-<os>-<arch>` 平台包（optionalDependencies，esbuild 模式），由 release 工作流在 goreleaser 之后发布。
- GitHub Actions：`ci.yml`（PR：lint/test/e2e，三平台）、`bench.yml`（nightly）、`release.yml`（tag 触发：goreleaser，加 `actions/attest-build-provenance`，再发布 npm），`scorecard.yml`。

## 附录 B：演示脚本（vhs）

```
# vhs/demo.tape
Output demo.gif
Set FontSize 16
Set Width 1100
Set Height 600
Type "claude -p 'clean up the build' --permission-mode auto"  Enter  Sleep 4s
Type "ls src"  Enter  Sleep 1s          # ls: cannot access 'src'
Type "oops"  Enter  Sleep 1.5s          # 时间线，选中 #42，预览 restore src/ (38)
Enter  Sleep 0.5s  Enter  Sleep 1s      # 确认卡片 → Undo
Type "ls src"  Enter  Sleep 2s          # 全部回来
```

## 附录 C：实现检查点（给开发 agent）
1. 先实现 `engine/store` + `engine/snapshot` + `engine/restore`，并让 property 测试通过，再接 CLI。数据安全是第一优先级。
2. 所有屏幕文案按 §1 与 §4 实现，并纳入 golden 测试（color / no-color / ascii、80 列 / 120 列）。
3. 每个 Agent 适配器都要先有 fixture，再写代码；上游文档链接写在适配器的包注释里：
   - Claude Code hooks：https://code.claude.com/docs/en/hooks · checkpointing：https://code.claude.com/docs/en/checkpointing
   - Codex hooks：https://developers.openai.com/codex/hooks
   - Gemini CLI hooks：https://geminicli.com/docs/hooks/reference/
   - Cursor hooks：https://cursor.com/docs/hooks（CLI 支持的事件比 IDE 少，以论坛的官方回复和实测为准）
4. 性能预算不达标的 PR 不合并（CI benchstat 门槛）。
