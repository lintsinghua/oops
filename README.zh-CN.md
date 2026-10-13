<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="brand/logo/oops-wordmark-dark.svg">
    <img alt="oops" src="brand/logo/oops-wordmark.svg" height="64">
  </picture>
</p>

<p align="center">
  <strong>Agent 闯祸，一键撤回。</strong><br>
  Claude Code、Codex、Gemini、Cursor 对文件做的任何改动都能撤销，<code>rm -rf</code> 也不例外。
</p>

<p align="center">
  <a href="https://github.com/lintsinghua/oops/actions/workflows/ci.yml"><img alt="CI" src="https://github.com/lintsinghua/oops/actions/workflows/ci.yml/badge.svg"></a>
  <a href="https://github.com/lintsinghua/oops/releases"><img alt="Release" src="https://img.shields.io/github/v/release/lintsinghua/oops?color=F5A524&label=release"></a>
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

Homebrew（`oops-cli/tap`）和 npm（`@oops-cli/oops`）都还没注册。

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
| 二进制 | [GitHub Releases](https://github.com/lintsinghua/oops/releases)：macOS、Linux、Windows · amd64 / arm64 |
| Homebrew | 即将提供。`oops-cli/tap` 还没注册（`brew install oops-cli/tap/oops`）。 |
| npm | 即将提供。`@oops-cli/oops` 还没注册（`npm i -g @oops-cli/oops`）。 |

GitHub Release 的产物是 tar.gz（Windows 为 zip）和 `checksums.txt`（sha256）。当前 GoReleaser 配置不会做 cosign 签名，也不附带 SBOM 或 SLSA provenance。还没有 `curl \| sh` 安装脚本、Scoop 清单或 winget 清单。

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

在 Claude Code 里也可以直接输入 `/oops`。它在本地执行，不会发给模型，不消耗 token。

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

`undo`、`rewind`、`restore`、`gc` 支持 `-n/--dry-run`。`redo` 和 `snap` 不支持。`list`、`show`、`diff`、已经执行的 `undo`、`doctor`、`snap`、`gc` 支持 `--json`（带 `schema_version`）。`undo -n --json` 仍然打印文本预览，`show --json` 是这一轮的元数据，不是文件列表。退出码：`0` 成功 · `1` 错误 · `2` 需要确认 · `3` 部分完成（冲突文件被保留）· `4` 没有可操作的目标 · `130` 被中断。

## 支持的 Agent

| Agent | 接入方式 | 一轮的边界 | 会话内 | 说明 |
|---|---|---|---|---|
| Claude Code | `~/.claude/settings.json` 中的 hooks | 提交 prompt → `Stop` | `/oops` | 本地执行，不耗 token |
| Codex | `~/.codex/hooks.json` | 提交 prompt → `Stop` / 中断 | prompt hook 里带有这段文字时的 `/oops` | 需在 `/hooks` 中信任一次。不会安装 `!oops` |
| Gemini CLI | `~/.gemini/settings.json` | `BeforeAgent` → `AfterAgent` | `BeforeAgent` 带上 prompt 时的 `/oops` | 与 Gemini 自带检查点可共存。没有单独的 slash hook |
| Cursor CLI | `~/.cursor/hooks.json` | 提交 prompt → `stop` | `!oops` | `-p` 模式建议用 `oops run -- cursor-agent` |
| 其他任意工具 | `oops run -- <cmd>` | 整个会话，静默 3 秒自动切分 | — | Aider、opencode、自研 harness |
| 脚本 | `oops snap` | 手动检查点 | — | |

`oops init --project` 会写入项目级配置（如 `.claude/settings.json`），团队成员拉下仓库即可共享。

每个 Agent 经过测试的最低版本还没有记录。适配器写在 `internal/agents`，事件名以各家 hook 文档为准。

v0.2 计划支持：GitHub Copilot CLI、opencode、Qwen Code、Amp、Goose。[申请支持新的 Agent →](https://github.com/lintsinghua/oops/issues/new/choose)

## 工作原理

```
 Agent 一轮开始 ──► oops hook ──► 快照 "pre"
        … Agent 编辑文件、跑 sed、rm -rf、git clean …
 Agent 一轮结束 ──► oops hook ──► 快照 "post"

 oops undo ──► 对这一轮改过的每个文件：
                 还是 Agent 留下的样子？      → 还原成 "pre" 版本
                 之后被你（或别人）改过？    → 保留，并告诉你原因
```

- **按轮快照**。Agent 的 hooks 标出一轮的开始和结束，oops 在这两个时刻给工作区拍快照；两者之差就是这一轮做的全部改动，不管是哪个工具改的。
- **增量、内容寻址**。借助 stat 缓存，只读取和哈希有变化的文件。内容去重、zstd 压缩、不可变——数据模型借鉴 git（blob、tree），但不使用 git 存储。
- **你的改动是安全的**。撤销时对每个文件比较三个状态：轮前、轮后、现在。这一轮之后被你（或别的 Agent）改过的文件会被保留并列出，不会被覆盖。`--force` 可以强制覆盖，被覆盖的版本会先保存下来。
- **撤销也能撤销**。每次 undo、rewind、restore 之前都先拍一张安全快照，`oops redo` 一定能回到原样。
- **不打扰**。hook 进程永远 exit 0，没有常驻进程。它们只在回答 `/oops` 时写 stdout；Claude Code 上，快照不完整或有文件大到没存下内容时，还会输出一条 `systemMessage`。硬超时默认 10 秒，从这次扫描开始计时，超时只把快照标成 partial，不阻塞 Agent。下一次 `oops` 命令也会打印同一条提示。
- **不碰你的仓库**。快照数据不会写进工作区。不写 `.git`，不建分支，不 stash。例外是 `oops init --project`：它会写入 Agent 自己的项目配置，例如 `.claude/settings.json`。快照在 `~/.local/share/oops`（Windows 为 `%LOCALAPPDATA%\oops`）。`OOPS_HOME` 可以改数据目录。macOS 用的也是 `~/.local/share/oops`。
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
| 历史保留 | 每会话 100 个，约 30 天 | — | — | 运行 `oops gc` 时按 500 轮或 30 天 |
| 回滚对话 | ✓ | — | ✓ | ✗（不在目标内） |

<sub>✓ 支持 · ✗ 不支持 · — 不适用或文档未说明。来源：[Claude Code 检查点文档](https://code.claude.com/docs/en/checkpointing)、[openai/codex PR #19481](https://github.com/openai/codex/pull/19481)。其他工具那几列写于 2026-10-14，这次没有逐格复核，也没有补上 Gemini CLI 检查点文档链接。oops 不安装 SubagentStop hook；文件要在这一轮的 post 快照时已经在磁盘上，才会被覆盖到。2 GB 和 10 GB 的保留设置会在 `oops doctor` 里显示，但不会强制执行。</sub>

## FAQ

<details>
<summary><strong>会拖慢 Agent 吗？</strong></summary>

oops 的设计目标就是不拖慢。Codex 上一版 `/undo` 被移除，原因之一就是快照给用户带来了问题，所以这是 oops 的第一条规矩：快照是增量的（只重读 size、mtime 或 mode 变化的文件），被排除的目录直接剪掉不遍历，每次快照有硬超时（默认 10 秒）——超时后快照标记为 partial，Agent 照常继续。`oops doctor` 会报告 hook 状态、工作区规模，以及大到没存下内容的文件。它目前不会显示上一次快照的耗时。

热快照于 2026-10-13 用 `OOPS_PERF=1 go test -count=1 -timeout 45m -run TestHotCapturePercentiles ./internal/engine/` 测得（`internal/engine/perf_test.go`）。夹具是分布在 200 个目录里的 N 个互不相同的 32 字节文件。一次冷 `Capture` 填满 stat 缓存，然后对未改动的树做 21 次热 `Capture`。p50 是排序后的第 11 个样本，p95 是第 20 个（nearest-rank）。测的是进程内的 Capture（stat 遍历、缓存复用、tree 写入、SQLite 插入），不含进程启动。机器：4 vCPU Intel Xeon，Linux 6.12.94+ x86_64。

| 文件数 | p50 | p95 |
|---:|---:|---:|
| 10,000 | 98.473505ms | 108.303065ms |
| 100,000 | 946.033015ms | 4.083672579s |
</details>

<details>
<summary><strong>存了什么？存在哪里？</strong></summary>

工作区的文件内容（去重并压缩），以及一个小型 SQLite 数据库，记录每一轮的元数据：Agent、时间、改动的文件、prompt 的首行（最多存 200 字符，常见密钥会被脱敏；设置 `privacy.record_prompts = false` 可完全关闭）。数据在 `~/.local/share/oops`（Windows 是 `%LOCALAPPDATA%\oops`）。Unix 上新建的数据目录是 `0700`，对象文件是 `0600`。Windows 上数据目录会设成只给当前用户的 ACL。配置里的默认值是最近 500 轮或 30 天，单个工作区 2 GB、总计 10 GB。轮数和天数只在你运行 `oops gc` 时生效，不会自动清理。字节上限会在 `oops doctor` 里显示，但不会强制执行。`oops gc` 清理当前工作区里过期的轮。`oops gc --all` 删除当前工作区的快照。`oops gc --all --global` 删除所有工作区。一小时内新建、且已无引用的对象会先留着。
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

不会。这些命令不会主动联网。没有遥测，没有更新检查，没有账号。`oops version` 只打印编译进去的版本、提交和构建日期。`oops version --check` 没有实现。
</details>

<details>
<summary><strong>支持 Windows 吗？</strong></summary>

CI 在 Linux、macOS、Windows 上跑同一条 `go test -race ./...`。长路径、仅大小写不同的重命名、以及被其他程序占用的文件，恢复代码里有处理。这三项还没有放进三平台共用的端到端场景测试。
</details>

<details>
<summary><strong>怎么卸载？</strong></summary>

`oops init --undo` 移除 hooks（其他设置不受影响），`oops gc --all --global` 删除所有快照，然后用包管理器删除二进制即可。
</details>

## 参与贡献

欢迎提交 bug、Agent 适配器和文档修正。请先阅读 [CONTRIBUTING.md](CONTRIBUTING.md)：开发环境、`make test`、Conventional Commits、DCO 签名，以及新增 Agent 适配器的模板。提交 bug 时请附上 `oops doctor --json` 的输出。

安全问题请通过 [GitHub Security Advisories](https://github.com/lintsinghua/oops/security/advisories/new) 私下报告，详见 [SECURITY.md](SECURITY.md)。

## 许可证

[MIT](LICENSE)
