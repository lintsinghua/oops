<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="brand/logo/wordmark-dark.svg">
    <img alt="oops" src="brand/logo/wordmark-light.svg" height="64">
  </picture>
</p>

<p align="center">
  <strong>Agent 闯祸，一键撤回。</strong><br>
  Claude Code、Codex、Gemini、Cursor 对文件做的任何改动都能撤销，<code>rm -rf</code> 也不例外。
</p>

<p align="center">
  <a href="https://github.com/lintsinghua/oops/actions/workflows/ci.yml"><img alt="CI" src="https://github.com/lintsinghua/oops/actions/workflows/ci.yml/badge.svg"></a>
  <a href="https://github.com/lintsinghua/oops/releases"><img alt="Release" src="https://img.shields.io/github/v/release/lintsinghua/oops?color=000000&label=release"></a>
  <a href="./LICENSE"><img alt="License: MIT" src="https://img.shields.io/badge/license-MIT-8B949E"></a>
  <a href="https://goreportcard.com/report/github.com/lintsinghua/oops"><img alt="Go Report Card" src="https://goreportcard.com/badge/github.com/lintsinghua/oops"></a>
</p>

<p align="center">
  <a href="./README.md">English</a> · 简体中文
</p>

<p align="center">
  <img alt="oops run 包住 rm -rf src &amp;&amp; git clean -fdx。oops undo 把 src 和 .env 恢复回来。" src="assets/demo.gif" width="820">
</p>

```sh
go install github.com/lintsinghua/oops/cmd/oops@latest
oops init                         # 一次接好本机所有 Agent，之后就不用管了
```

---

## 为什么需要 oops

编程 Agent 已经可以自己执行 shell 命令了。有时候那条命令是 `rm -rf`、`git clean -fdx`，或者对四十个文件跑一遍 `sed -i`，而被改的文件从来没提交过。

Agent 自带的保护网在这些场景下并不够用：

- **Codex** 原本有 `/undo`，2026 年 4 月被移除（[#8804](https://github.com/openai/codex/issues/8804)、[PR #19481](https://github.com/openai/codex/pull/19481)）。要求恢复它的 [openai/codex#9203](https://github.com/openai/codex/issues/9203)“Please make /undo back”至今仍是 open 状态，有 480 个赞、93 条评论。原帖的场景是：Codex 误删了没进 git 的文件，误改了还没提交的内容。相关的 `/rewind` 需求（[#11626](https://github.com/openai/codex/issues/11626)）也有 227 个赞。
- **Claude Code** 的检查点[不追踪通过 Bash 做的修改](https://code.claude.com/docs/en/checkpointing)（`rm`、`mv`、`cp`），不恢复后台 subagent 的编辑，每个会话只保留最近 100 个检查点。auto mode 下 Agent 经常用 Bash 改文件，于是 `/rewind` 可能报告成功、实际什么都没恢复（[anthropics/claude-code#87575](https://github.com/anthropics/claude-code/issues/87575)）。
- **Gemini CLI** 的检查点默认关闭，只在 `write_file` / `replace` 之前快照；`/restore` 会对整个影子仓库执行 `git restore` 加 `git clean`，连你自己的改动也一起冲掉。

<sub>issue 数据截至 2026-10-14。</sub>

oops 工作在更底层的文件系统。不管 Agent 用的是自带的 Edit 工具、`sed`、Python heredoc 还是 `rm -rf`，也不管文件有没有进 git，这一轮都能撤回。

## 安装

| | |
|---|---|
| Go | `go install github.com/lintsinghua/oops/cmd/oops@latest` |
| Releases | [GitHub Releases](https://github.com/lintsinghua/oops/releases)：macOS、Linux、Windows · amd64 / arm64 |

Homebrew、npm 和安装脚本即将提供。

## 30 秒上手

```console
$ oops init

  oops keeps a snapshot before every agent turn, so you can undo it.

  Found 3 agents
    ● Claude Code   ~/.claude/settings.json
    ● Codex         ~/.codex/hooks.json
    ● Gemini CLI    ~/.gemini/settings.json
    ○ Cursor CLI    not found

  Connect all 3? [Y/n/e] ⏎

  ✓ Claude Code   connected · /oops ready
  ✓ Codex         connected · open Codex and run /hooks once to trust oops
  ✓ Gemini CLI    connected

  You're protected.
  When an agent breaks something, just type: oops
```

就这样。之后照常工作，oops 运行时不会输出任何东西。

出事的时候，只需要敲一个词：

```console
$ oops
```

时间线随即打开，默认选中最近一轮，右侧预览撤销后会发生什么。按 <kbd>Enter</kbd>，确认，完成。

```console
$ oops undo
✓ Undid turn #42 · claude · "clean up the build scripts"

  restored  39 files   src/ (38), .env
  removed    1 file    scripts/build.mjs
  reverted   1 file    package.json

  Changed your mind? oops redo
```

在 Claude Code 里也可以直接输入 `/oops`。它在本地执行，不会发给模型，不消耗 token。Claude Code 会把这段回复显示成 hook 拦截（“operation blocked by hook”）。以 `✓ oops` 开头的那一行是结果，不是报错。

不想用了？`oops init --undo` 只移除 oops 自己写入的配置，其他设置一行不动。

## 命令

你只需要记住 `oops`。其余命令是给熟练用户和脚本用的快捷方式。

| 命令 | 作用 |
|---|---|
| `oops` | 打开时间线：选中一轮 → 预览 → 撤销 |
| `oops undo [turn]` | 撤销最近一轮或指定的一轮；`--file <path>` 只撤销部分文件 |
| `oops rewind <turn>` | 回到某一轮开始之前（撤销该轮及之后的所有轮） |
| `oops redo` | 撤回最近一次 undo、rewind 或 restore |
| `oops restore <path>… --at <turn>` | 取回某一轮开始之前的文件版本 |
| `oops list` | 最近的轮：Agent、时间、prompt 摘要、改动文件数 |
| `oops show [turn]` | 查看某一轮改了什么 |
| `oops diff [turn]` | 查看某一轮的 diff（`--stat`、`--name-only`、`--reverse`） |
| `oops init` | 接入本机的 Agent（`--project`、`--print`、`--undo`） |
| `oops run -- <cmd>` | 保护任何没有 hooks 的工具 |
| `oops snap [-m msg]` | 立即保存一个检查点 |
| `oops doctor` | 检查一切是否正常 |
| `oops gc` | 回收快照占用的空间 |

`undo`、`rewind`、`redo`、`restore`、`snap`、`gc` 支持 `-n/--dry-run`。`list`、`show`、`diff`、`undo -n`、`doctor`、`snap`、`gc` 支持 `--json`（带 `schema_version`）。`show --json` 包含文件列表。退出码：`0` 成功 · `1` 错误 · `2` 需要确认 · `3` 部分完成（冲突文件被保留）· `4` 没有可操作的目标 · `130` 被中断。字段见 [docs/schema.md](docs/schema.md)。

## 支持的 Agent

| Agent | 接入方式 | 一轮的边界 | 会话内 | 说明 |
|---|---|---|---|---|
| Claude Code | `~/.claude/settings.json` 中的 hooks | 提交 prompt → `Stop` | `/oops` | 本地执行，不耗 token。Claude 显示为 “operation blocked by hook”；`✓ oops` 那一行是结果 |
| Codex | `~/.codex/hooks.json` | 提交 prompt → `Stop` / 中断 | prompt hook 里带有这段文字时的 `/oops` | 需在 `/hooks` 中信任一次。非交互运行用 `codex exec --dangerously-bypass-hook-trust`。不会安装 `!oops` |
| Gemini CLI | `~/.gemini/settings.json` | `BeforeAgent` → `AfterAgent` | `BeforeAgent` 带上 prompt 时的 `/oops` | 与 Gemini 自带检查点可共存。没有单独的 slash hook |
| Cursor CLI | `~/.cursor/hooks.json` | 提交 prompt → `stop` | `!oops` | `-p` 模式建议用 `oops run -- cursor-agent` |
| 其他任意工具 | `oops run -- <cmd>` | 整个会话，静默 3 秒自动切分 | — | Aider、opencode、自研 harness |
| 脚本 | `oops snap` | 手动检查点 | — | |

`oops init --project` 会写入项目级配置（如 `.claude/settings.json`），团队成员拉下仓库即可共享。

v0.2 计划支持：GitHub Copilot CLI、opencode、Qwen Code、Amp、Goose。[申请支持新的 Agent →](https://github.com/lintsinghua/oops/issues/new/choose)

## 工作原理

```
 Agent 一轮开始 ──► oops hook 要等快照 "pre" 落盘才返回
        … Agent 编辑文件、跑 sed、rm -rf、git clean …
 Agent 一轮结束 ──► oops hook 立刻返回 ──► 快照 "post" 在后台完成

 oops undo ──► 对这一轮改过的每个文件：
                 还是 Agent 留下的样子？      → 还原成 "pre" 版本
                 之后被你（或别人）改过？    → 保留，并告诉你原因
```

- **按轮快照**。Agent 的 hooks 标出一轮的开始和结束，oops 在这两个时刻给工作区拍快照；两者之差就是这一轮做的全部改动，不管是哪个工具改的。
- **增量、内容寻址**。借助 stat 缓存，只读取和哈希有变化的文件。内容去重、zstd 压缩、不可变——数据模型借鉴 git（blob、tree），但不使用 git 存储。
- **你的改动是安全的**。撤销时对每个文件比较三个状态：轮前、轮后、现在。这一轮之后被你（或别的 Agent）改过的文件会被保留并列出，不会被覆盖。`--force` 可以强制覆盖，被覆盖的版本会先保存下来。
- **撤销也能撤销**。每次 undo、rewind、restore 之前都先拍一张安全快照，`oops redo` 一定能回到原样。
- **不打扰**。hook 进程永远 exit 0，没有常驻进程。轮次开始的 hook 会等 pre 快照落盘才返回，后面的修改不会被当成撤销要恢复的原件。轮次结束的 hook 先在数据库里把这一轮标成已结束，记下任务后立刻返回，由脱离的进程写入 post 快照并做保留清理。`oops undo` 会等这张已排队的快照最多 5 秒，而不会把这一轮当成仍在运行。下一轮会等这张 post 快照（最多 30 秒，超时后由等待的进程自己写完），轮次边界不会错。如果上一轮还开着，pre hook 会先按当时的目录树把它收完，再存新的 pre 快照；Agent 进程在 Stop 之后立刻退出也一样。之后才跑完的后台扫描不会覆盖这张 post。后台进程如果中途退出，任务文件还在，下一条命令会把这一轮写完。hook 只在回答 `/oops` 时写 stdout；Claude Code 上，快照不完整或有文件大到没存下内容时，还会输出一条 `systemMessage`。其他 Agent 在下一次执行 `oops` 时看到同一条提示。同一个原因只提示一次。硬超时默认 10 秒，从这次扫描开始计时，超时只把快照标成 partial，不阻塞 Agent。
- **不碰你的仓库**。快照数据不会写进工作区。不写 `.git`，不建分支，不 stash。例外是 `oops init --project`：它会写入 Agent 自己的项目配置，例如 `.claude/settings.json`。快照在 `~/.local/share/oops`（Windows 为 `%LOCALAPPDATA%\oops`）。`OOPS_HOME` 可以改数据目录。
- **收录范围**。没被 gitignore 的文件（含未跟踪文件），再加上即使被 gitignore 也受保护的文件（`.env`、`.env.*`、`.envrc`、`*.local`、`*.local.*`、`.dev.vars`、`local.settings.json`）。被 gitignore 的路径即使曾经 `git add -f` 也会跳过。`node_modules/`、`.venv/`、`target/` 等重目录自动跳过。`dist/` 和 `build/` 只在同时命中 gitignore 规则时跳过。可用 `.oopsignore`（gitignore 语法）调整。超过 50 MB 的文件只记录元数据不存内容。`oops doctor`、`oops show` 和下一条命令会说明它们未受保护。

详见 [docs/how-it-works.md](docs/how-it-works.md)

## 与 Agent 自带检查点的对比

oops 不取代 Agent 自带的检查点，而是补上它们覆盖不到的地方。推荐组合使用：对话用 Agent 的 `/rewind`，文件用 `oops undo`。

| | Claude Code `/rewind` | Codex | Gemini CLI `/restore` | **oops** |
|---|:-:|:-:|:-:|:-:|
| 有内置撤销 | ✓ | 已移除（2026-04） | 需手动开启 | ✓ |
| 通过 shell 做的修改（`rm`、`mv`、`sed`） | ✗ | — | 只在 `write_file` / `replace` 前快照 | ✓ |
| 后台 subagent 的编辑 | ✗ | — | — | 本轮结束前已写到磁盘的文件 |
| 未跟踪和被 gitignore 的文件（`.env`） | — | — | — | ✓ |
| 保留你在这一轮之后的改动 | — | — | ✗（整库 restore + clean） | ✓ |
| 跨 Agent 的统一时间线 | ✗ | ✗ | ✗ | ✓ |
| 独立于 Agent 会话 | ✗（按会话） | — | — | ✓ |
| 历史保留 | 每会话 100 个，约 30 天 | — | — | 500 轮或 30 天 |
| 回滚对话 | ✓ | — | ✓ | ✗（不在目标内） |

<sub>✓ 支持 · ✗ 不支持 · — 不适用或文档未说明。来源：[Claude Code 检查点文档](https://code.claude.com/docs/en/checkpointing)、[openai/codex PR #19481](https://github.com/openai/codex/pull/19481)、[Gemini CLI 检查点](https://geminicli.com/docs/cli/checkpointing/)。Claude Code 的 `SubagentStop` 会被记录，且不会把这一轮拆开。文件要在父轮的 post 快照时已经在磁盘上，才会被覆盖到。</sub>

## FAQ

<details>
<summary><strong>会拖慢 Agent 吗？</strong></summary>

oops 的设计目标就是不拖慢。Codex 上一版 `/undo` 被移除，原因之一就是快照给用户带来了问题，所以这是 oops 的第一条规矩：快照是增量的（只重读 size、mtime 或 mode 变化的文件），被排除的目录直接剪掉不遍历，每次快照有硬超时（默认 10 秒）——超时后快照标记为 partial，Agent 照常继续。轮次结束的 hook 在 post 快照入队后立刻返回。脱离的进程负责写入。轮次开始的 hook 要等 pre 快照落盘才返回。

2026-10-13 用 `OOPS_PERF=1 go test -count=1 -timeout 45m -run 'TestHotCapturePercentiles|TestHookVisibleLatency' ./internal/engine/` 测得（`internal/engine/perf_test.go`）。夹具是分布在 200 个目录里的 N 个互不相同的 32 字节文件。p50 是 21 个样本里排序后的第 11 个，p95 是第 20 个（nearest-rank）。机器：4 vCPU，Linux 6.12.94+ x86_64。

**Agent 等待轮次结束 hook 返回的时间。** 这不是快照完成时间。`TestHookVisibleLatency` 编译 `./cmd/oops`，先走一轮冷启动，等后台任务清空，再计时 21 次 `oops hook claude turn-end` 从进程启动到退出。该进程只写入任务后退出。

| 文件数 | Agent 等待 turn-end hook 返回，p50 | Agent 等待 turn-end hook 返回，p95 |
|---:|---:|---:|
| 10,000 | 4 ms | 5 ms |
| 100,000 | 5 ms | 6 ms |

**后台 post 快照的完成时间**，由进程内热 `Capture` 另测（`TestHotCapturePercentiles`：一次冷 Capture，然后 21 次未改动的 Capture，含 stat 遍历、缓存复用、tree 写入、SQLite 插入）。这是 turn-end hook 已经返回之后，快照本身还要花的时间：

| 文件数 | 后台快照完成，p50 | 后台快照完成，p95 |
|---:|---:|---:|
| 10,000 | 82 ms | 91 ms |
| 100,000 | 0.82 s | 3.5 s |

**Agent 等待轮次开始 hook 返回的时间。** pre 快照在这个进程里完成，hook 要等快照落盘才返回。方法和上面的 turn-end 表相同，计时的是上一张后台快照已经结束后的 `oops hook claude turn-start`，所以这段等待是 pre 快照加上进程启动，不是上面的 5 ms / 6 ms。

| 文件数 | Agent 等待 turn-start hook 返回，p50 | Agent 等待 turn-start hook 返回，p95 |
|---:|---:|---:|
| 10,000 | 92 ms | 106 ms |
| 100,000 | 0.86 s | 0.92 s |

同一次测量里，10 万文件的进程内 Capture p95 是 3.5 s，turn-start hook 的 p95 是 0.92 s。两组都是同一台机器、同一天、21 个样本的 nearest-rank。如果上一张 post 快照还在写，turn-start hook 会先等它，上限 30 秒（`TestWaitPendingBounded`）。任务已经完成时，这段额外等待不到 50 ms。后台进程死掉不会丢掉这一轮：任务文件会留到后面的命令把 post 快照写完（`TestCrashedChildFinishesTurn`）。

**冷启动和扫描时的内存。** `TestColdStart10k` 计时的是对 10,000 个文件、合计 300,000,000 字节做的第一次进程内 Capture。这台机器上是 0.69 s（PRD 在 Linux 上的上限是 3 s）。`TestScanRSS100k` 在扫描 100,000 个文件时采样 VmRSS，峰值 119.5 MiB，即 125,341,696 字节（PRD 上限是 150 MB）。同一台机器、同一天。这两个数各是一次采样，不是百分位。
</details>

<details>
<summary><strong>存了什么？存在哪里？</strong></summary>

工作区的文件内容（去重并压缩），以及一个小型 SQLite 数据库，记录每一轮的元数据：Agent、时间、改动的文件、prompt 的首行（最多存 200 字符，常见密钥会被脱敏；设置 `privacy.record_prompts = false` 可完全关闭）。数据在 `~/.local/share/oops`（Windows 是 `%LOCALAPPDATA%\oops`）。Unix 上新建的数据目录是 `0700`，对象文件是 `0600`。Windows 上数据目录会设成只给当前用户的 ACL。每张 post 快照之后会自动保留最近 500 轮或 30 天，并在单个工作区超过 2 GB 或全局超过 10 GB 时删掉最旧的轮。`oops gc` 可以手动做同样的清理。`oops gc --all` 删除当前工作区的快照。`oops gc --all --global` 删除所有工作区。一小时内新建、且已无引用的对象会先留着。
</details>

<details>
<summary><strong>快照里有我的 <code>.env</code>，这样安全吗？</strong></summary>

这是有意为之——`.env` 被删是最让人头疼的损失之一。数据永远不离开你的机器，只有你的用户可以读取。如果不想保存某个文件，把它写进 `.oopsignore`。威胁模型见 [SECURITY.md](SECURITY.md)。
</details>

<details>
<summary><strong>和 git 是什么关系？</strong></summary>

git 管的是你决定保留的历史，oops 管的是两次提交之间的那几分钟。它覆盖未跟踪和被忽略的文件，不需要提交，也从不碰你的仓库：不写 `.git`，不建分支，不 stash。在不是 git 仓库的目录里也能用。
</details>

<details>
<summary><strong>能撤销数据库改动、网络请求或安装的包吗？</strong></summary>

不能。oops 只恢复工作区内的文件。工作区之外的副作用（数据库、网络请求、全局安装、磁盘其他位置的文件）oops 无法撤销。
</details>

<details>
<summary><strong>会回滚 Agent 的对话吗？</strong></summary>

不会，那是 Agent 自己的事（Claude Code 的 `/rewind` 就做得很好）。对话交给 Agent，文件交给 oops。
</details>

<details>
<summary><strong>Agent 这一轮之后我又改了文件怎么办？</strong></summary>

oops 会保留你的版本，并告诉你跳过了哪些文件、为什么（退出码 `3`）。如果你确实想把 Agent 的改动也撤掉，输出里会给出完整命令（带 `--force`），你的版本会先被保存。
</details>

<details>
<summary><strong>会联网吗？</strong></summary>

这些命令不会主动联网。没有遥测，也没有账号。`oops version` 只打印编译进去的版本、提交和构建日期。唯一的网络请求是 `oops version --check`，它读取 GitHub 上的最新 release。
</details>

<details>
<summary><strong>支持 Windows 吗？</strong></summary>

CI 在 Linux、macOS、Windows 上跑同一条 `go test -race ./...`，其中包含共用的端到端场景（删除、修改、重命名、权限、符号链接、Unicode 文件名、空目录、`.env`、大文件、dry-run JSON）。
</details>

<details>
<summary><strong>怎么卸载？</strong></summary>

`oops init --undo` 移除 hooks（其他设置不受影响），`oops gc --all --global` 删除所有快照，然后删除 `$(go env GOPATH)/bin/oops`。
</details>

## 参与贡献

欢迎提交 bug、Agent 适配器和文档修正。请先阅读 [CONTRIBUTING.md](CONTRIBUTING.md)：开发环境、`make test`、Conventional Commits、DCO 签名，以及新增 Agent 适配器的模板。提交 bug 时请附上 `oops doctor --json` 的输出。

安全问题请通过 [GitHub Security Advisories](https://github.com/lintsinghua/oops/security/advisories/new) 私下报告，详见 [SECURITY.md](SECURITY.md)。

## 许可证

[MIT](LICENSE)
