# oops — 产品需求说明（PRD）

> 版本：v1.0（供开发 agent 实现用） · 日期：2026-10-14 · 状态：可开工
> 姊妹文档：[DESIGN.md](./DESIGN.md)（用户旅程、交互原则、技术方案、CLI 命令设计、UI/UX 规范）
> 技术选型已定：**Go + Bubble Tea / Lip Gloss，单二进制 CLI**。

---

## 0. TL;DR

- **oops** 是一个给任意编程 Agent（Claude Code、Codex、Gemini CLI、Cursor CLI，以及任何终端 Agent）用的**整轮撤销**工具。
- Agent 每开始一轮，oops 就给工作区拍一张增量快照。不管这一轮是用 Edit 工具、`sed -i`、`python - <<EOF`、`rm -rf` 还是 `git clean -fdx` 改的文件，也不管文件有没有进 git，都能用 `oops undo` 一键撤回。
- 不依赖 Agent 自己的检查点，不碰用户的 git 仓库。默认只在本机运行，不联网。
- **用户只需要记住一个词：`oops`**。出事时直接敲 `oops`，进入交互式时间线（Bubble Tea TUI）：选中一轮 → 预览 diff → 回车撤销。`oops undo` 是无交互的快捷方式；在 Agent 会话里也可以用 `/oops`。
- MVP（v0.1）：`oops init` 30 秒内一次接好本机所有 Agent（可一键撤销接入）；自动按轮快照；交互式时间线；`undo / rewind / redo / restore / list / show / diff`；会话内 `/oops`；通用包装器 `oops run`；支持 macOS、Linux、Windows。完整的用户旅程见 DESIGN.md §1。

---

## 1. 背景与证据（2026-10-14 核实）

### 1.1 官方需求长期悬而未决

| 证据 | 状态（2026-10-14 经 GitHub API 核实） | 说明 |
|---|---|---|
| [openai/codex#9203](https://github.com/openai/codex/issues/9203) “Please make /undo back” | **open**，👍 480，93 条评论，2026-01-14 创建，标签 enhancement / TUI / session | 原帖的场景是 Codex 误删未进 git 的文件、误改未提交的内容 |
| [openai/codex#11626](https://github.com/openai/codex/issues/11626) `/rewind` 同时回滚对话和代码 | **open**，👍 227，48 条评论 | 提出的需求包括：列出检查点（时间、prompt 摘要、改动文件）、预览、只回滚 Agent 的改动、冲突时要求确认 |
| Codex 移除 /undo 的原因（[#8804](https://github.com/openai/codex/issues/8804)、[PR #19481](https://github.com/openai/codex/pull/19481)，2026-04 合入） | 已移除 ghost snapshot | 官方解释：使用量低，而且实现给很多用户（包括没用这个功能的人）带来严重问题；还说“如果回归，需要新设计”。**这是 oops 必须吸取的教训：快照绝不能拖慢 Agent，也不能干扰用户环境。** |
| [Claude Code 检查点文档](https://code.claude.com/docs/en/checkpointing) | 官方写明的限制 | 不追踪 Bash 修改（rm/mv/cp）；后台 subagent 的编辑不恢复；外部或并发会话的修改不追踪；跳过符号链接和硬链接；每个会话只保留最近 100 个检查点，快照约 30 天后被清理 |
| [anthropics/claude-code#87575](https://github.com/anthropics/claude-code/issues/87575) | **open**，👍 42，15 条评论，bug / has repro | auto mode 的系统提示让模型用 Bash 改文件，导致 `/rewind` 报成功、实际什么都没恢复 |
| [anthropics/claude-code#87971](https://github.com/anthropics/claude-code/issues/87971) | **open**，👍 91 | auto mode 下 Claude 用 bash 做读、写、编辑 |
| Gemini CLI checkpointing | 默认关闭，需在 settings 中开启 | 用影子 git（`~/.gemini/history/<hash>`），只在 write_file/replace 前快照；`/restore` 会执行 `git restore` 加 `git clean`，即整库回滚，会冲掉用户自己的改动 |

### 1.2 竞品（2026-10-14 快照）

| 项目 | ★ | 最近推送 | 范围与不足 |
|---|---|---|---|
| [RonitSachdev/ccundo](https://github.com/RonitSachdev/ccundo) | 1,401 | 2025-07-27 | 只支持 Claude Code；只解析 Edit 类工具调用，不覆盖 bash；简介自称“claude now natively supports this feature”，**事实上已停更** |
| [arpagon/pi-rewind](https://github.com/arpagon/pi-rewind) | 115 | 2026-03-31 | 只是 Pi Agent 的扩展：每轮一个检查点，带 diff 预览和 redo |
| [extracurricular-ai/codex-rewind](https://github.com/extracurricular-ai/codex-rewind) | 54 | 2026-09-27 | Codex 的非官方 fork（`codexr`），要跟着上游发版，只服务 Codex |
| trouties/bashback | 2 | 2026-07 | 只做 bash 的 /rewind |
| Archerkattri/stepback、hichipli/takeback、Abelo9996/snap-back、MacHatter1/bb-plugin-rewind | 0–1 | 2026-09～10 | 同类雏形，都刚起步。说明**赛道正在升温，窗口期有限** |
| Agent 自带（CC `/rewind`、Gemini `/restore`、opencode undo 等） | — | — | 各管各的，覆盖面不全（见上表），不能跨 Agent，也没有统一的时间线 |

**结论**：需求硬、长期未满足；现有方案要么绑死单个 Agent，要么只是雏形。oops 的机会在于“**跨 Agent、文件系统级、对已有改动安全**”的成熟实现，并且必须在官方补齐之前抢到心智。

---

## 2. 产品定位

### 2.1 一句话定位
**oops 是所有编程 Agent 共用的撤销键**：工作在文件系统层，Agent 用什么方式改的文件、文件有没有进 git，都能撤回。

### 2.2 Slogan
- 主：**`oops undo` — Ctrl+Z for coding agents.**
- 中文：**Agent 闯祸，一键撤回。**
- 备选：“Undo anything your agent did. Even `rm -rf`.”

### 2.3 设计信条（所有决策的裁判）
1. **先保命，再好用**：oops 自己绝不能丢用户数据。每次撤销前都先拍一张安全快照，撤销本身也能撤销。
2. **隐形**：正常使用时用户感觉不到 oops 存在。hook 很快，不污染 Agent 的上下文，不打印噪音。
3. **只撤 Agent 做的事**：按轮撤销，不冲掉用户自己的改动和别的 Agent 的改动；有冲突就停下来说清楚。
4. **不碰用户的 git**：不写 `.git`，不建分支，不 stash；仓库外的存储只属于 oops。
5. **跨 Agent，零锁定**：同一工作区里多个 Agent 共用一条时间线。
6. **克制**：命令少，输出短，默认值正确。
7. **一个词就够**：慌张的用户只需要记住 `oops`。其余命令都是给熟练用户和脚本用的快捷方式。
8. **可逆即可快**：所有操作都能 `oops redo` 反悔，所以单轮撤销不设确认关卡；影响多轮或者要强制覆盖时，先预览再确认。

### 2.4 非目标（明确不做）
- 不替代 git，也不做版本管理或协作。
- 不回滚工作区之外的副作用：数据库、网络请求、全局安装的包、工作区外的文件（v0.2 起会给出警告）。
- 不回滚 Agent 的对话上下文（那是 Agent 自己的事）。v0.2 会在撤销后把“文件已被撤回”这一事实告知 Agent。
- 不做云同步，没有账号体系，默认不收集遥测。

---

## 3. 目标用户

| 画像 | 描述 | 核心诉求 |
|---|---|---|
| P1 重度 Agent 用户（主） | 每天用 Claude Code / Codex 跑几十轮，经常开 auto mode / `--yolo` / full-auto | 放手让 Agent 跑，跑坏了 3 秒内回到上一轮 |
| P2 多 Agent 切换者 | 同一个仓库里 CC、Codex、Gemini 交替使用，或几个会话并行 | 统一的时间线，知道哪一轮是谁改的 |
| P3 非 git / 半 git 场景 | 脚本目录、数据分析目录、刚 `init` 还没提交的项目、`.env` 和本地配置 | 没进 git 的文件被删了也能找回 |
| P4 团队与平台工程师 | 给团队推 Agent 工具链、写 devcontainer 和规范 | 可脚本化（`--json`）、可预装（project 级 hooks）、安全可审计 |
| P5 Agent 工具作者（次要） | 自研 harness 或 Agent | 一个可以嵌入的撤销层（`oops snap` / hook 协议） |

---

## 4. 核心场景与用户故事

### 4.1 核心场景
- **S1 灾难恢复**：Agent 执行了 `rm -rf src && git clean -fdx`，或者用 `> file` 清空了文件 → `oops undo` → 全部回来，包括没进 git 的文件。
- **S2 方向错了**：Agent 这一轮重构了 20 个文件，方向不对 → `oops diff` 看一眼 → `oops undo`。
- **S3 只要部分**：这一轮大部分可以留下，只有一个文件改坏了 → `oops undo --file src/auth.ts`。
- **S4 回到很早以前**：最近 5 轮都走偏了 → `oops rewind 37`，回到第 37 轮开始之前。
- **S5 撤销后悔了** → `oops redo`。
- **S6 找回某个历史版本**：前面某一轮删掉的文件，现在想要回来 → `oops restore config/local.yml --at 31`。
- **S7 我自己的改动别动**：Agent 这一轮结束后我手动改了文件，再撤销这一轮时，oops 发现冲突，默认跳过并解释原因。
- **S8 任意 Agent**：用 Aider / opencode / 自研 Agent → `oops run -- aider`，同样能撤销。

### 4.2 用户故事（编号供验收引用）
- **US-1** 作为 P1，我在 Claude Code 的 auto mode 下让 Agent 用 bash 批量改了文件，我希望 `oops undo` 能把这一轮的改动全部撤回，即使官方 `/rewind` 做不到。
- **US-2** 作为 P1，我希望安装后执行一条 `oops init`，它自动检测本机装了哪些 Agent，我确认一次就全部接上，全程 30 秒内完成；不想用了，`oops init --undo` 就能干净地撤掉接入。
- **US-3** 作为 P1，撤销前我想先看到会发生什么：哪些文件恢复、哪些删除、哪些还原，再确认执行。
- **US-4** 作为 P3，Agent 删除了被 `.gitignore` 忽略的 `.env`，我希望能找回来。
- **US-5** 作为 P2，我希望 `oops list` 能显示每一轮是哪个 Agent、什么时间、prompt 摘要、改了几个文件。
- **US-6** 作为 P1，我希望 oops 永远不拖慢 Agent。开不开 oops，我感觉不到区别。
- **US-7** 作为 P1，我在撤销时如果某些文件在这一轮之后被我改过，我希望 oops 不要覆盖它们，并告诉我怎么处理。
- **US-8** 作为 P1，撤销本身出错或者我后悔了，我希望 `oops redo` 一定能回到撤销前的状态。
- **US-9** 作为 P4，我希望所有命令都支持 `--json`，非交互环境下行为确定（不会卡在确认提示上）。
- **US-10** 作为 P5 / P8 场景用户，没有 hooks 的 Agent 我也能用 `oops run -- <cmd>` 或 `oops snap` 获得保护。
- **US-11** 作为注重安全的用户，我希望 oops 的数据只在本机、权限严格、可以一键清除，并且清楚知道它存了什么。
- **US-12** 作为 Windows 用户，我希望和 macOS、Linux 用户有同样的体验。
- **US-13** 作为慌张的 P1，我只记得一个词。敲 `oops` 就能看到最近几轮的时间线，用方向键选中、看 diff、回车撤销，不需要查文档。
- **US-14** 作为 P1，我正在 Claude Code / Codex / Gemini 会话里，不想切出终端，希望输入 `/oops` 就能查看和撤销上一轮。
- **US-15** 作为 P1，平时我希望完全感觉不到 oops；只有它没能保护我（比如跳过了大文件、hooks 失效）时，才用一行很轻的提示告诉我。
- **US-16** 作为 P1，撤销完成后我想清楚知道恢复了什么、哪些没动、为什么没动，以及怎么反悔（`oops redo`）。
- **US-17** 作为脚本作者，我希望在没有 TTY、设置了 `NO_COLOR` 或使用 `--json` 时，oops 的输出稳定、无颜色、无交互。

---

## 5. 功能范围与验收标准

> 约定：“工作区”指 oops 保护的根目录（默认取 git toplevel，否则取 Agent 的 cwd）。“轮”（turn）指用户提交一次 prompt 到 Agent 停止之间的全部文件系统变化。验收标准（AC）全部可以自动化测试，测试矩阵见第 6 节。

### 5.1 MVP（v0.1）— 目标：发布即可演示“rm -rf 后一键复原”，四家 Agent 开箱即用

| ID | 功能 | 验收标准 |
|---|---|---|
| **F1** | **按轮自动快照**：轮开始前拍 pre 快照，轮结束时拍 post 快照（增量、内容寻址、去重） | AC1 覆盖 tracked、untracked、被忽略但在 `protect` 名单内的文件（默认含 `.env*`）。AC2 bash（`sed -i`、`>`、`rm`、`mv`、`cp`、`chmod`、heredoc）、Edit 工具、`git checkout/reset/clean` 造成的改动都能被 post−pre 差异捕获。AC3 不写入用户的 `.git`，不改动工作区中任何文件。AC4 Agent 异常退出（没触发 Stop）时，该轮在下一次 prompt 或下一次执行 `oops` 命令时被惰性关闭，post 快照取当时的状态 |
| **F2** | **`oops undo [TURN]`**：撤销最近一轮或指定一轮 | AC1 默认撤销最后一个已结束的轮。AC2 对该轮改动的每个文件：当前内容等于 post 时还原为 pre（修改的还原，新建的删除，删除的恢复）。AC3 当前内容不等于 post（这一轮之后又被改过）时判为冲突，默认跳过并列出原因，退出码 3；`--force` 覆盖（被覆盖的版本进入安全快照）。AC4 **无交互快捷方式**：`oops undo`（单轮）直接执行，执行完打印恢复摘要和 `oops redo` 提示（可逆，所以不设确认）；带 `--force` 或者目标轮不是最后一轮时，在 TTY 下先预览再确认，`-y` 跳过确认，非 TTY 且没有 `-y` 时拒绝执行，退出码 2。AC4b 目标轮仍在运行（Agent 没停）时拒绝执行，提示先停下 Agent，退出码 4；`--running` 立即关闭该轮并撤销。AC5 `-n/--dry-run` 只展示不执行。AC6 恢复文件的内容、权限位、符号链接目标与 pre 快照逐字节一致 |
| **F3** | **`oops undo --file <path>...`**：只撤销指定文件 | AC1 支持多个路径和 glob。AC2 不在该轮改动集合里的路径给出明确提示 |
| **F4** | **`oops rewind <TURN>`**：把工作区回到第 TURN 轮开始之前（撤销该轮及之后的所有轮） | AC1 等价于从最新一轮到 TURN 依次 undo，冲突处理规则相同；因为影响多轮，TTY 下先预览（按轮汇总）再确认，`-y` 跳过。AC2 轮与轮之间用户自己的改动（gap）默认保留；冲突时提示 |
| **F5** | **`oops redo`**：撤回最近一次 undo / rewind / restore | AC1 每次变更性操作执行前都自动拍安全快照。AC2 redo 之后工作区与 undo 之前逐字节一致（property 测试）。AC3 redo 本身也可以再次被 undo / redo（栈结构） |
| **F6** | **`oops list`**：时间线 | AC1 每行显示：轮号、Agent、相对时间、prompt 摘要（截断）、改动统计（文件数与 +/− 行数）、状态（running / done / undone）。AC2 默认显示最近 10 轮，`-n` 指定数量，`--all` 显示全部。AC3 轮与轮之间有外部改动时显示一行灰色的 gap 提示。AC4 支持 `--json` |
| **F7** | **`oops show [TURN]`**、**`oops diff [TURN]`** | AC1 show 显示元数据和文件列表（动作、路径、+/−、是否 untracked 或 ignored）。AC2 diff 输出 unified diff，带颜色，内容超过一屏时走 pager。`--stat`、`--name-only`、`-- <path>` 可用。AC3 二进制文件显示大小变化，不输出乱码 |
| **F8** | **`oops restore <path>... --at <TURN>`**：从某一轮开始之前的快照取回文件 | AC1 能取回任意一轮中已删除的文件。AC2 默认不覆盖已有文件，需要确认或 `--force` |
| **F9** | **`oops snap [-m msg]`**：手动检查点 | AC1 生成一个 kind=manual 的轮（pre=post=当前状态），可以作为 rewind 的目标。AC2 可供脚本调用，静默模式 `-q` |
| **F10** | **`oops init`**：一键接入 | AC1 自动检测已安装的 claude / codex / gemini / cursor-agent（显示版本和配置文件路径），**一次确认全部接上**；按 `e` 可以逐个勾选。AC2 从启动到完成，交互耗时 ≤ 30 秒，命令自身耗时 ≤ 2 秒（不含用户思考时间）。AC3 幂等，重复执行不产生重复条目，已接入的显示为“already connected”。AC4 修改前备份原配置文件；`oops init --undo` 完整移除 oops 写入的条目（不动用户其他配置），并可以选择是否删除快照数据。AC5 `--project` 写入项目级配置；`--print` 只打印将要写入的片段；`-y` 非交互。AC6 Codex 需要信任 hooks，init 结束时给出明确的下一步提示。AC7 同时安装会话内 `/oops` 命令（F19）。AC8 完成后最后一行提示“下次出事，敲 `oops`” |
| **F11** | **`oops run -- <cmd...>`**：通用包装器 | AC1 启动前拍快照，退出后拍快照，整个会话至少形成一轮。AC2 运行期间用 fsnotify 检测活动，“活动 → 静默 ≥ N 秒”就自动切出一个检查点（auto turn，N 默认 3s）。AC3 透传 stdin/stdout/stderr、信号和退出码，不改变 Agent 的交互体验 |
| **F12** | **`oops`（无参数）= 交互式时间线 TUI**（Bubble Tea） | AC1 在 TTY 中打开全屏时间线：左侧是轮列表（轮号、Agent、人类可读的时间、prompt 摘要、改动统计），右侧是选中轮的文件列表和 diff 预览。AC2 键盘优先：`↑↓/jk` 选择，`enter` 撤销所选轮（弹出确认层，显示恢复、删除、还原的文件数），`tab` 切换焦点，`d` 全屏 diff，`r` rewind 到所选轮之前，`/` 过滤，`?` 帮助，`q/esc` 退出；鼠标可选。AC3 **在用户确认之前不执行任何变更**；确认层默认焦点在“Undo”，`esc` 取消。AC4 启动到首屏 ≤ 150 ms。AC5 非 TTY、`--json` 或 `TERM=dumb` 时退化为打印状态与最近轮（等同 `oops list -n 5` 加保护状态），不进入 TUI。AC6 宽度小于 80 列时单栏显示，diff 用 `enter→d` 进入。AC7 没有任何轮时显示空状态引导（“还没有快照，运行 `oops init`”） |
| **F13** | **`oops doctor`** | AC1 检查：二进制路径、hooks 是否安装且可执行、数据目录权限、工作区规模（文件数和体积超阈值时给出建议）、被跳过的大文件、fsnotify 限额（Linux 的 inotify watches）。AC2 每个问题都附带修复命令 |
| **F14** | **存储与清理**：保留策略和 `oops gc` | AC1 默认保留最近 500 轮或 30 天（先到者为准），单工作区上限 2 GB、全局上限 10 GB，超限时按 LRU 回收最旧的轮，并在 `oops` 时间线顶部提示。AC2 `oops gc` 手动回收，`--dry-run` 显示可释放的空间。AC3 GC 不会删除任何仍被引用的对象（含安全快照） |
| **F15** | **忽略与大文件** | AC1 默认尊重 `.gitignore`，同时内置排除名单（`node_modules/`、`.venv/`、`target/` 等，见 DESIGN）和保护名单（`.env*` 等）。AC2 支持项目级 `.oopsignore`（gitignore 语法，`!` 表示强制包含）。AC3 单文件超过 `max_file_size`（默认 50 MB）时只记录元数据，不存内容，`show` 和 `doctor` 中标记为“未受保护” |
| **F16** | **跨平台** | AC1 macOS（arm64/amd64）、Linux（amd64/arm64）、Windows（amd64/arm64）三平台通过同一套 e2e 场景测试。AC2 Windows 支持长路径、大小写不敏感、文件被占用时重试并报告 |
| **F17** | **机器可读与脚本化** | AC1 `list/show/diff --stat/undo -n/doctor` 支持 `--json`，schema 写入文档并带 `"schema_version"`。AC2 退出码约定见 DESIGN §4.6 |
| **F19** | **会话内 `/oops`** | AC1 `oops init` 为 CC 安装 `/oops` 命令，并通过 UserPromptExpansion hook 在本地直接执行，结果作为提示显示给用户，**不发送给模型、不消耗 token**。AC2 `/oops` 显示最近 5 轮和上一轮的改动摘要；`/oops undo [N]`、`/oops redo`、`/oops diff [N]` 可用（在会话内不能开 TUI）。AC3 对不能拦截斜杠命令的 Agent（Codex、Gemini 等），尽量通过 UserPromptSubmit / BeforeAgent 拦截以 `/oops` 开头的 prompt 并阻止它进入模型；做不到的，文档和 init 提示使用 Agent 自带的 shell 逃逸（`!oops undo`）。AC4 每个 Agent 的支持方式写进 README 的支持矩阵 |
| **F20** | **日常无感与轻提示** | AC1 正常情况下 hook 零输出。AC2 只在“oops 没能保护你”时提示：hooks 失效、快照超时（partial）、跳过了大文件、存储接近上限；同一原因每个会话最多提示一次。AC3 提示形式最轻：CC 用 `systemMessage`（一行，`oops ·` 前缀），其他 Agent 在下一次执行 `oops` 命令时显示；`oops run` 只在开始和结束时各打印一行灰色字 |
| **F18** | **hook 永不伤害 Agent** | AC1 `oops hook` 内部出现任何错误都 exit 0（不阻断 Agent），错误写入日志，下一次执行 `oops` 时提示。AC2 有硬超时（默认 10s，可配），超时后标记该轮快照为 partial 并继续。AC3 hook 不向 Agent 上下文输出任何内容（v0.1） |

**MVP 不包含**：步骤级（单个工具调用级）撤销、Agent 上下文同步、Web 视图、MCP。（交互式时间线 TUI 已纳入 MVP，见 F12。）

### 5.2 v0.2 — 目标：更细的粒度，更好的体验，口碑传播

| ID | 功能 | 验收标准 |
|---|---|---|
| **G1** | **步骤级检查点**：在 PreToolUse（Bash / 写文件工具）前做增量快照，`oops show 42` 列出步骤（含命令文本） | AC1 支持 `oops undo 42.3`（撤销第 42 轮从第 3 步起的改动）。AC2 单步快照开销 p95 < 80 ms（10k 文件仓库）。AC3 Edit/Write 类工具只对目标文件做定向快照 |
| **G2** | **TUI 增强** | AC1 在 TUI 中按文件逐个勾选撤销（空格键选择）。AC2 在 TUI 中 restore 历史版本。AC3 按 Agent、会话、日期分组与过滤。AC4 diff 并排模式 |
| **G3** | **Agent 上下文同步**：撤销后，下一次 UserPromptSubmit / BeforeAgent 时向 Agent 注入一句事实性说明（“文件 X、Y 已由用户用 oops 撤回到第 42 轮之前的状态”） | AC1 在 CC、Codex、Gemini 上可用。AC2 可以关闭。AC3 文本不超过 500 字符，不使用命令式措辞 |
| **G4** | **Git 引用保护**：记录每轮前后的 HEAD、分支和 refs，检测 Agent 执行的 `reset --hard`、`commit --amend`、`branch -D`、`checkout` 等 | AC1 `show` 中提示“本轮改动了 git 引用”。AC2 `oops undo --git` 可以把 refs 还原到轮前状态（依据对象仍存在和 reflog），默认不动 |
| **G5** | **工作区外写入警告** | AC1 对 bash 命令做静态解析（PreToolUse），发现写入工作区外路径时，在 `show` 中记录并提示“这部分无法撤销” |
| **G6** | **`oops run` 的 PTY 轮次识别** | AC1 在 PTY 中识别用户按回车提交，以此作为轮边界（适配 aider / opencode 等），Windows 使用 ConPTY |
| **G7** | **Claude Code plugin 与 Codex plugin 分发** | AC1 能从官方插件市场一键安装 hooks 和 `/oops`。AC2 与 `oops init` 不重复安装 |
| **G8** | **`oops config`** | AC1 `get/set/list/edit` 子命令，校验配置键和取值 |
| **G9** | **更多 Agent 适配器**：GitHub Copilot CLI、opencode、Qwen Code、Amp、Goose 等有 hooks 的 Agent | AC1 每个适配器都有录制的 hook payload fixture 和 e2e |
| **G10** | **diff 增强**：词级高亮、语法高亮（chroma，可关）、并排模式 `--side-by-side` | AC1 在 1000 行 diff 上渲染 < 100 ms |

### 5.3 未来（Backlog，按价值排序）
1. **`oops mcp`**：以 MCP server 暴露 list/diff/undo，让 Agent 可以自我回滚（默认只读，写操作需要用户确认）。
2. **本地 Web 时间线** `oops --web`：只监听 127.0.0.1，使用随机 token，供分享截图和长 diff 阅读。
3. **VS Code / JetBrains 扩展**：在编辑器侧边栏显示 oops 时间线。
4. **文件系统原生加速**：APFS clonefile、btrfs/XFS reflink、ZFS snapshot，用于大仓库的秒级快照和恢复。
5. **快照加密**（age），用于存放敏感文件的工作区。
6. **devcontainer / 远程 / CI 模式**：在 Agent 沙箱中运行，把时间线导出成 artifact。
7. **团队策略**：project 级 `oops.toml` 统一保护名单和保留策略。
8. **“回滚到这里并分叉”**：与 git worktree 联动，把某一轮的状态导出成分支，方便对比不同方案。

---

## 6. 质量与验收标准

### 6.1 测试矩阵

| 维度 | 取值 |
|---|---|
| OS / 架构 | ubuntu-latest（amd64）、ubuntu-24.04-arm、macos-latest（arm64）、macos-13（amd64）、windows-latest（amd64）、windows-11-arm（有 runner 时） |
| Go | stable、oldstable |
| 文件系统 | ext4、tmpfs、APFS（默认大小写不敏感）、NTFS；nightly 加跑 btrfs、大小写敏感 APFS 卷 |
| Agent 适配器 | Claude Code、Codex、Gemini CLI、Cursor CLI：①录制的真实 hook stdin payload fixture（每次上游版本变化时更新）；②fake-agent harness 按真实事件顺序调用 `oops hook`；③nightly 对已安装的真实 CLI 跑冒烟测试（有凭据时） |
| 场景语料（e2e，每个都要做到“undo 后逐字节等于 pre”） | `rm -rf src && git clean -fdx`；`sed -i`；`mv` 重命名与跨目录移动；仅大小写不同的重命名（macOS/Windows）；`chmod +x`；符号链接新建、删除、改指向；Unicode / NFD 文件名；CRLF 保持；空目录；Windows 260+ 长路径；文件被占用；超过 max_file_size 的大文件（应提示未受保护）；`.env` 被删；`git reset --hard`；轮内并发写入；两个会话并发出轮；undo 中途进程被 kill（重启后可恢复或回滚） |
| 测试类型 | 单元测试；property-based 测试（`pgregory.net/rapid`：随机 FS 操作序列 → undo → 等于 pre；undo → redo → 等于 undo 前）；fuzz（manifest 解码、ignore 匹配、hook payload 解析）；golden 测试（CLI 输出，分 color/no-color、宽窄终端）；TUI 用 `teatest`；`-race` 全量开启；故障注入（磁盘满、权限拒绝、rename 失败） |
| 覆盖率 | 核心包（store / snapshot / restore）≥ 85%，整体 ≥ 70%，CI 中设门槛 |

### 6.2 性能指标（CI 基准测试，回归超过 10% 即失败）

| 指标 | 目标（Linux/macOS，NVMe） | Windows |
|---|---|---|
| hook 开销（热缓存增量快照），10k 文件 / 300 MB 仓库 | p50 ≤ 60 ms，p95 ≤ 150 ms | ×2 |
| 同上，100k 文件 / 3 GB | p50 ≤ 400 ms，p95 ≤ 1 s | ×2 |
| 冷启动首次基线，10k 文件 / 300 MB | ≤ 3 s（在 SessionStart / init 时后台完成） | ≤ 6 s |
| 无改动的空轮 | 新增存储 ≤ 4 KB | 同 |
| `oops undo`，改动 1k 个文件 | ≤ 1 s | ≤ 2 s |
| `oops list` | ≤ 50 ms | ≤ 100 ms |
| 进程常驻内存（扫描 100k 文件） | ≤ 150 MB | 同 |
| 二进制体积 | ≤ 25 MB（strip 后） | 同 |
| 典型存储开销（一天 200 轮的中型仓库） | ≤ 仓库体积的 30%（去重 + zstd） | 同 |

### 6.3 跨平台 CI
- PR 必跑：lint（golangci-lint）、`go vet`、`govulncheck`、单元、property、golden、e2e（三平台）、基准测试冒烟。
- nightly：完整基准测试和趋势图、fuzz 30 分钟、真实 Agent 冒烟、额外文件系统。
- release：goreleaser 产出全平台产物，安装方式冒烟（brew / scoop / npm / curl / go install 各安装一次并执行 `oops version`）。

### 6.4 文档完整性（发布门槛）
- README：一句话定位、GIF 演示（用 charmbracelet/vhs 生成，脚本放在仓库里）、30 秒上手、支持的 Agent 矩阵、与 Agent 自带功能的对比、FAQ（“它会拖慢 Agent 吗？”“存了什么？”“和 git 什么关系？”）。
- docs/：安装、各 Agent 接入、命令参考（由 cobra 自动生成，与 `--help` 同源）、配置参考、`--json` schema、工作原理（存储格式）、隐私与安全、故障排查、卸载。
- 每个命令的 `--help` 都有 1–3 个例子。
- 中英双语 README（英文为主，中文 README.zh-CN.md）。

### 6.5 “可开源”发布检查清单（v0.1 发布前必须全部打勾）

- [ ] `LICENSE`：**MIT**（建议，与 gh、lazygit、Charm 生态一致，摩擦最小；如果担心专利问题可改 Apache-2.0，二选一，不要双许可）
- [ ] `README.md` 和 `README.zh-CN.md`，包含 GIF、徽章（CI、release、license、Go Report Card）
- [ ] `CONTRIBUTING.md`：开发环境、`make test`、提交规范（Conventional Commits）、如何新增 Agent 适配器（附模板）、DCO 或 CLA 二选一（建议 DCO）
- [ ] `CODE_OF_CONDUCT.md`：Contributor Covenant 2.1
- [ ] `SECURITY.md`：开启 GitHub Private Vulnerability Reporting，承诺 72 小时内响应；写明支持的版本
- [ ] `.github/ISSUE_TEMPLATE/`：bug（YAML 表单，必填 `oops doctor --json` 输出、OS、Agent 及版本）、feature、agent-integration request；`config.yml` 把问答引导到 Discussions
- [ ] `.github/pull_request_template.md`：变更说明、测试、是否影响存储格式、截图
- [ ] `CHANGELOG.md`：Keep a Changelog 格式，release 时由 goreleaser changelog（按 conventional commit 分组）生成后人工润色
- [ ] `.goreleaser.yaml`：darwin/linux/windows × amd64/arm64；tar.gz/zip；`checksums.txt`；cosign keyless 签名；syft SBOM；GitHub artifact attestation（SLSA provenance）；nfpm 打 deb/rpm/apk
- [ ] Release 工作流：push tag `v*` → 测试 → goreleaser → 发布到 GitHub Releases → 更新 Homebrew tap、Scoop bucket、winget manifest（PR）、npm 包
- [ ] 安装方式（scoop、winget 仍是计划渠道；脚本、Homebrew formula、npm 包已在仓库里，tap 仓库和 registry 发布还要等第一次 tag）：
  - `brew install lintsinghua/tap/oops`（formula 由 goreleaser 推到 `lintsinghua/homebrew-tap`；达到 homebrew-core 的知名度门槛后再提交到 core）
  - `curl -fsSL https://raw.githubusercontent.com/lintsinghua/oops/main/install.sh | sh`（校验 `checksums.txt` 的 sha256，支持 `OOPS_VERSION` 和 `OOPS_INSTALL_DIR`；Windows 用 `install.ps1`）
  - `npm i -g @lintsinghua/oops` 或 `npx @lintsinghua/oops`（**npm 上的 `oops` 已被占用**，registry 现有 `oops@0.0.1`；采用 esbuild 式的平台 optionalDependencies 分发二进制）
  - `scoop install oops`、`winget install oops`
  - `go install github.com/<org>/oops/cmd/oops@latest`
  - AUR `oops-bin`（社区维护）
- [ ] 版本与兼容：SemVer；存储格式带版本号，提供升级迁移；1.0 之前允许破坏性变更，但必须写进 CHANGELOG 并提供迁移
- [ ] 仓库设置：Discussions、topics（ai-agents, claude-code, codex, undo, cli, go）、social preview 图、branch protection、CODEOWNERS、Dependabot、OpenSSF Scorecard 工作流
- [ ] 交互验收：按 DESIGN §1 的用户旅程，由 3 位从未用过的人在 macOS / Linux / Windows 各走一遍（安装 → `oops init` → 制造一次 `rm -rf` → `oops` 撤销 → `oops redo`），不看文档完成，且 init 用时 ≤ 30 s
- [ ] `oops --version` 输出 commit 和构建时间；`oops doctor` 能输出可粘贴到 issue 的诊断信息

---

## 7. 成功指标（10k★ 路径）

| 阶段 | 指标 |
|---|---|
| 发布首周 | HN Show 首页；★ ≥ 1,500；真实撤销成功案例 ≥ 20 条（X / issue） |
| 3 个月 | ★ ≥ 5,000；GitHub release + brew + npm 累计安装 ≥ 30k；外部贡献者 ≥ 30；支持 Agent ≥ 6 |
| 12 个月 | ★ ≥ 10,000；进入 Claude Code / Codex 插件市场推荐位；homebrew-core |
| 健康度 | issue 首次响应 < 48h；P0 数据安全 bug 修复 < 24h；每月至少一个 release |

**传播策略**
1. 一个 15 秒 GIF：`rm -rf src && git clean -fdx` → `oops undo` → 全部回来。
2. 对比表：“CC `/rewind` / Codex / Gemini `/restore` / oops”，逐项列出 bash、untracked、`.env`、跨 Agent、冲突保护。
3. 在相关 issue（#9203、#11626、#87575）中以“社区 workaround”的形式**一次性、克制地**说明用法，遵守各仓库规范，不刷屏。
4. 提交到 awesome-claude-code、awesome-codex 等列表；发布 Claude Code plugin。
5. 不收集遥测，用 README 的 “Used by” 和 Discussions 的 Show & Tell 收集案例。

---

## 8. 风险与应对

| 风险 | 概率 / 影响 | 应对 |
|---|---|---|
| **R1 官方恢复或增强撤销**（Codex 重做 /undo，CC 检查点覆盖 bash） | 中高 / 高 | 见 §8.1 的差异化。官方做得越好，“跨 Agent 统一时间线 + 安全网”的定位越清晰；oops 应定位为**底层安全网**，与官方功能共存，不对抗 |
| **R2 oops 自身丢数据**（恢复 bug 覆盖了用户文件） | 低 / 致命 | 变更前必拍安全快照；恢复过程写 journal，可续做或回滚；原子写（临时文件 + rename）；property 测试与故障注入；存储对象不可变；GC 采用标记-清除，加宽限期 |
| **R3 拖慢 Agent**（重蹈 Codex ghost snapshot 的覆辙） | 中 / 高 | 增量扫描（stat 缓存），只哈希变化的文件；硬超时；冷基线放在后台；超大仓库自动降级并提示 `.oopsignore`；基准测试进 CI |
| **R4 上游 hooks API 变化**（字段、事件名、信任机制） | 高 / 中 | 每个 Agent 一个独立的 adapter 包；payload 解析宽容（未知字段忽略，缺字段降级）；录制 fixture 并在 nightly 对真实 CLI 冒烟；`oops doctor` 检测版本兼容性 |
| **R5 Windows 复杂度**（文件锁、长路径、符号链接权限、ConPTY） | 高 / 中 | 从第一天起三平台 CI；Windows 专项场景；遇到无法处理的情况明确报告，绝不静默跳过 |
| **R6 存储膨胀**（大二进制、生成文件） | 中 / 中 | 内置排除名单；`max_file_size`；配额加 LRU；时间线顶部和轻提示中提醒；`oops gc` |
| **R7 隐私与安全审视**（快照里有 `.env`，prompt 被落盘） | 中 / 中 | 目录权限 0700、文件 0600；不联网；prompt 只存截断后的摘要，并用正则脱敏常见 key；`oops forget`/`purge`（v0.2）；`SECURITY.md` 写清楚威胁模型 |
| **R8 名称冲突**：Go 生态有 samber/oops（995★，错误处理库），npm 上 `oops` 已被占用 | 确定 / 低 | 二进制名和品牌保持 `oops`；npm 用 scope 包；仓库名 `oops` 放在专属 org 下；SEO 用 “oops undo” / “oops for agents” |
| **R9 同类雏形快速增长**（stepback、takeback 等） | 中 / 中 | 抢发 MVP（4–6 周），以“跨平台单二进制 + 四家 Agent 开箱即用 + 冲突安全”拉开差距；尽早建立社区和适配器生态 |
| **R10 Agent 信任与权限门槛**（Codex 要求信任 hooks；企业禁用用户 hooks） | 中 / 中 | init 给出逐步引导；`oops run` 与 fsnotify 作为兜底；文档提供管理员部署方式 |

### 8.1 对“官方恢复 /undo”的应对与差异化

官方即使恢复，大概率仍然是**单 Agent、会话内、工具级**的实现（CC 文档写明不追踪 bash、不恢复后台 subagent、100 个检查点上限、约 30 天清理；Codex 上一版因为副作用被整体移除）。oops 的长期差异化：

1. **跨 Agent 统一时间线**：同一仓库里 CC、Codex、Gemini、Cursor 和人工改动在同一条时间线上，用同一套撤销语义。
2. **文件系统级完整覆盖**：bash、subagent、后台任务、外部进程，只要改动落在工作区内都会被覆盖；`.env` 这类被忽略的重要文件也受保护。
3. **对人类改动安全**：基于 post 快照做三方比对和冲突检测，只撤销 Agent 的改动，不像整库 restore 加 clean 那样冲掉用户的工作。
4. **独立于会话**：Agent 崩溃、会话丢失、`/clear` 之后，甚至换了一个 Agent，都还能撤销；保留策略由用户决定。
5. **可脚本化、可审计**：`--json`、退出码、`oops snap`，可以嵌入 CI、devcontainer 和自研 harness。
6. **Git 引用保护**（v0.2）和 MCP（未来）：从“撤销文件”扩展到“Agent 安全网”。
7. **兼容与共存**：官方功能可用时，oops 不替代它，只补盖官方覆盖不到的地方；文档提供“官方 `/rewind` 恢复对话 + `oops undo` 恢复文件”的组合用法。

如果官方的实现足够完整（跨 bash、未跟踪文件、冲突安全），oops 退守“多 Agent 用户与非 git 场景”的细分市场，同时把核心存储引擎作为库（`pkg/oops`）开放给其他 harness 嵌入。

---

## 9. 里程碑（建议）

| 周 | 交付 |
|---|---|
| W1 | 存储引擎（CAS + manifest + stat 缓存）、snapshot/restore 核心逻辑与 property 测试 |
| W2 | 轮模型、SQLite 元数据、`undo/redo/list/show/diff`、CC 适配器、`oops init` |
| W3 | Codex / Gemini / Cursor 适配器、会话内 `/oops`、`run`、`snap`、`restore`、`rewind`；Bubble Tea 时间线 TUI |
| W4 | Windows 打磨、doctor/gc/保留策略、golden 测试、性能调优 |
| W5 | 文档、GIF、goreleaser、安装渠道、发布检查清单 → **v0.1.0** |
| W6–W10 | v0.2：步骤级、TUI 增强、上下文同步、git 引用保护、插件分发 |
