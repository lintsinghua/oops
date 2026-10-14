# How oops works

oops records the files in a workspace around each agent turn and can put those files back. It does not rewind the agent's conversation, and it does not undo anything outside the workspace.

## Where state lives

The data directory is `OOPS_HOME`, or `~/.local/share/oops` on Linux and macOS (`XDG_DATA_HOME` when set), or `%LOCALAPPDATA%\oops` on Windows. User settings are a separate file: `~/.config/oops/config.toml` on Unix (`XDG_CONFIG_HOME` when set) and `%APPDATA%\oops\config.toml` on Windows. A missing config file means the defaults in `internal/config`.

```
$OOPS_HOME/
  meta.db                 SQLite (modernc.org/sqlite), WAL, schema version 1
  objects/ab/<rest>       zstd bodies, named by SHA-256 of the raw bytes
  workspaces/<id>/
    statcache             binary stat cache, magic OSTAT1
    journal/current.json  in-progress undo, so a crash can finish or roll back
    pending-notices.json  warnings to show on the next command
```

`<id>` is the first 16 hex characters of SHA-256 over the workspace's canonical real path. The path is lowercased on macOS and Windows. On Unix, new data directories are mode `0700` and object files are mode `0600`. On Windows, `SecureMkdir` sets an ACL that grants the current user only.

Snapshot bytes are not written into the workspace. `oops init --project` does write the agent's own project config, for example `.claude/settings.json`. oops does not write to `.git`, create branches, or stash.

## What a snapshot contains

`snapshot.Scan` walks the workspace root. The root is the nearest parent directory that contains `.git`, or the directory you are in when there is no `.git`.

A path is stored when:

- it is not gitignored, including untracked files, or
- it matches the protect list even if gitignore would hide it.

The built-in protect patterns are `.env`, `.env.*`, `.envrc`, `*.local`, `*.local.*`, `.dev.vars`, and `local.settings.json`, plus whatever `snapshot.protect` adds. `.oopsignore` uses gitignore syntax and can exclude more paths. Nested `.gitignore` and `.oopsignore` files are picked up while walking. Global git excludes (`~/.config/git/ignore`, `~/.gitignore_global`, and `.git/info/exclude`) are read when `snapshot.respect_gitignore` is true (the default).

These directory names are never walked: `.git`, `node_modules`, `.venv`, `venv`, `__pycache__`, `.mypy_cache`, `.pytest_cache`, `.ruff_cache`, `target`, `.gradle`, `.next`, `.nuxt`, `.svelte-kit`, `.turbo`, `.parcel-cache`, `.cache`, `coverage`, `.terraform`. `dist/` and `build/` are skipped only when a gitignore rule also matches them. Names starting with `.oops-tmp-` are skipped. A gitignored path is skipped even if it was force-added to git.

Files larger than `snapshot.max_file_size` (default 50MB) are recorded as skipped with reason `too_large`. Their bytes are not stored, so undo cannot restore them. The scan stops accepting new files after 200,000 and marks the snapshot partial. The deadline is wall-clock time from the start of that scan (default 10s). A timeout also marks the snapshot partial and returns what was walked so far. Up to 8 workers hash files (`min(8, NumCPU)`).

Each regular file and symlink target is stored under its SHA-256. The compressor is zstd at the fastest level. A directory is a tree object with magic `OOPSTREE` and version 1, so two directories with the same children share one object. The stat cache reuses a previous object when size, mtime, and mode are unchanged. Inode is stored in the cache record and is not part of that comparison. If the mtime is within one second of the previous scan, the file is hashed again.

## Turns

A turn has a pre snapshot and a post snapshot. The difference between them is what undo treats as that turn's edits.

| Source | When the turn opens | When it closes |
|---|---|---|
| Claude Code | `UserPromptSubmit` | `Stop` or `StopFailure` |
| Codex | prompt hook | `Stop` or interrupt |
| Gemini CLI | `BeforeAgent` | `AfterAgent` |
| Cursor CLI | prompt | `stop` |
| `oops run` | before the wrapped command | when the command exits, and also after `run.idle_split` of silence (default 3s) |
| `oops snap` | manual checkpoint; pre and post are the same snapshot | |

Cursor user hooks run with the working directory set to `~/.cursor` and no `cwd` field. oops reads `workspace_roots` from the payload, then `CURSOR_PROJECT_DIR`. When several roots are listed, the one that matches `CURSOR_PROJECT_DIR` is the workspace; otherwise oops uses the first root.

`oops init` writes hooks whose command contains `oops hook`. It rewrites the JSON it edits and copies the previous bytes to `*.oops-backup-<pid>`. `oops init --undo` writes those bytes back and deletes the backup files, so a config that already had hooks returns to its exact pre-init contents. A config oops created is removed. Codex still needs a one-time `/hooks` trust inside Codex; init says so, and `oops doctor` keeps warning until that trust exists because oops cannot see it. Non-interactive Codex runs hooks with `codex exec --dangerously-bypass-hook-trust`. Claude Code also gets `~/.claude/commands/oops.md` so `/oops` is a local command.

The hook process always exits 0. It writes stdout only for a `/oops` reply, or a Claude Code `systemMessage` when a snapshot was partial or a file was too large. There is no daemon. A running turn is closed lazily the next time you run an oops command in that workspace, except `version` and `--help`.

Prompt text stored on the turn is the first line, trimmed, with a few secret shapes replaced by `[redacted]`, and cut at `privacy.prompt_max_chars` (default 200). Set `privacy.record_prompts = false` to store nothing. The undo headline shows at most 60 characters of that line.

`/oops` is handled locally by `internal/slash` when a start or slash event's prompt begins with `/oops`. It is not sent to the model. Claude Code is the only adapter that installs a dedicated slash hook (`UserPromptExpansion` matcher `oops`) and writes `~/.claude/commands/oops.md`. Codex and Gemini CLI rely on the turn-start payload containing the prompt. The Codex adapter does not install `!oops`. Cursor is documented as the shell escape `!oops`; its hooks are `beforeSubmitPrompt`, `stop`, `beforeShellExecution`, and `afterFileEdit`.

## Undo

Undo loads the turn's pre and post trees, diffs them, then scans the workspace again for the current bytes.

For each changed path, `restore.Plan` compares three states:

- desired: the pre snapshot (what undo wants)
- expected: the post snapshot (what the agent left)
- current: the fresh scan

| Current state | Result |
|---|---|
| Already equal to desired | skip |
| Equal to expected, or `--force` | restore, remove, or revert |
| Different, and the bytes were never stored (`too_large`) | kept, reason `unprotected` |
| Different for any other reason | kept, reason `changed_after_turn` |

Kept paths are not overwritten. The command exits 3 and prints `oops undo <n> --file <path> --force`. `--force` saves the current tree first (the safety snapshot below) and then writes the desired bytes.

Before any apply, oops writes a safety snapshot and a journal at `workspaces/<id>/journal/current.json`. If the process dies mid-apply, the next command either finishes the journal or rolls it back. `oops redo` pops the newest applied undo, rewind, or restore and puts those paths back when they still match. That operation moves to the redo stack, so another `oops redo` pops the previous one, and `oops undo` with no turn number puts the most recently redone operation back. A new undo of a specific turn discards that redo stack. `oops rewind <n>` replays the diffs from turn n through the latest turn, so edits that happened in the gaps and were not part of those turns are kept.

`oops undo` with no turn number undoes the redo stack first when one exists, otherwise the newest turn that is not already undone. That is immediate when it is also the latest finished turn, including when stdin is not a terminal. No prompt is shown, so `echo n | oops undo` still undoes that turn. An older turn, or `--force`, asks `Undo? [Y/n]` on a terminal, and a piped answer is honored (`echo n` cancels). Without a terminal that undo is refused (exit 2) unless you pass `-y`. A non-terminal never prompts. A turn that is still running is refused (exit 4) unless you pass `--running`, which closes it and then undoes it. A turn whose Stop hook has returned, with the post snapshot still queued, is not still running: undo waits up to 5 seconds for that snapshot and then applies the turn. `oops list` and the other commands close a leftover running turn lazily. `oops rewind` always asks unless you pass `-y`. The timeline (`oops` with a terminal, not `--json`, not `TERM=dumb`) asks before undo or rewind. Esc or Ctrl+C on a confirmation cancels with exit 130.

`undo`, `rewind`, `redo`, `restore`, `snap`, and `gc` accept `-n` / `--dry-run`. `undo -n --json` and `show --json` print JSON (`schema_version` 1). `show --json` includes the file list. See [schema.md](schema.md).

## Garbage collection

After a post-turn snapshot finishes, a detached process runs retention: drop turns past `retention.keep_turns` (default 500) or older than `retention.keep_days` (default 30), then drop the oldest turns until object bytes fit `retention.workspace_max` (default 2GB) and `retention.global_max` (default 10GB). One pass deletes at most 64 turns and always leaves the newest turn. Objects newer than one hour stay, so a snapshot that is still being written is not deleted; `oops doctor` warns when usage is still over the cap. `oops gc` runs the same expiry for the current workspace. `oops gc --all` deletes that workspace's turns and snapshot rows. `oops gc --all --global` does that for every workspace. Both `--all` forms ask for confirmation unless `-y` or `-n` is set.

## Commands and exit codes

| Code | Meaning |
|---|---|
| 0 | ok, or the user declined a confirmation |
| 1 | error |
| 2 | usage, or confirmation is required and there is no terminal |
| 3 | partial: some files were kept |
| 4 | nothing to act on |
| 130 | interrupted |

`NO_COLOR`, `--color=never`, and `TERM=dumb` turn color off. `OOPS_ASCII=1`, `TERM=dumb`, and a non-UTF-8 locale use ASCII glyphs. A diff of 40 lines or more is sent to `$PAGER` (or `less -FRX`) when stdout is a terminal. `--no-pager` prints it directly.

## Hooks and network

The turn-start hook stores the pre snapshot before it returns. An edit that happens after that return, including an immediate `rm -rf`, is not part of the pre snapshot, so undo can put the original tree back. The turn-end hook stores the post snapshot before it returns. Claude Code's `Stop`, `StopFailure`, and `SubagentStop` are synchronous, so the agent waits for that snapshot. The published wait is the higher of this machine's harness and a designer measurement on the same kind of tree (2026-10-14; 4 vCPU Intel Xeon on KVM, 16 GB RAM, 256 GB virtio disk, Linux 6.12.94+): turn-end 180 ms p50 / 180 ms p95 at 10,000 files and 1.4 s / 1.4 s at 100,000 files; turn-start 99 ms / 153 ms at 10,000 files and 1.2 s / 1.2 s at 100,000 files. The harness itself was lower at Stop 10k (112 ms / 127 ms), Stop 100k (1.00 s / 1.11 s), and Start 100k (0.89 s / 0.99 s). Start at 10k is the harness figure. A timeout marks the snapshot partial and the hook still returns. Codex `Stop` and `Interrupt`, Gemini `AfterAgent`, and Cursor `stop` are synchronous as well. `SessionStart` stays asynchronous. Re-running `oops init` rewrites an older async end hook. Before the scan, the hook marks the turn ended and records the wall-clock Stop time. A detached process runs retention after the snapshot is stored. If the turn-end process is killed during the scan, the mark and the job file remain, and the next command finishes the snapshot. `oops undo` waits up to 5 seconds for a snapshot Stop already queued, and finishes it in this process if the child has exited, instead of reporting the turn as still running. A turn that has not received Stop is still refused. Stop records its start time before the scan and the time the post snapshot finishes. A file whose mtime falls inside that window is kept and reported, and the undo exits 3, because the snapshot may already contain the user's bytes. Every other file is a user edit only when its content differs from the baseline: the post snapshot, or the bytes the latest undo or redo wrote for that file. Writing a file during undo or redo updates that baseline, so a later undo of the same turn does not treat oops's own write as a user edit. A file the user created after the turn is kept. The next command, and the next turn start, wait until a queued job is finished or 30 seconds have passed, and if the child has died they finish the job themselves. If the previous turn is still open when the next one starts, the pre hook closes it from the tree at that moment before storing the new pre snapshot. A finish that runs after that does not overwrite the stored post. The post snapshot is therefore not taken after the following turn has already edited files.

Claude Code's `SubagentStop` hook is installed. It is logged and does not open or close a turn. Files a subagent wrote are in the snapshot when they are on disk at the parent turn's post snapshot.

`oops version` prints the version, commit, and date compiled into the binary. `oops version --check` is the only network call: it GETs `https://api.github.com/repos/lintsinghua/oops/releases/latest`. A 404 prints `no release published`.

Release archives are tar.gz (zip on Windows) plus `checksums.txt` (sha256). `go install github.com/lintsinghua/oops/cmd/oops@latest` and [GitHub Releases](https://github.com/lintsinghua/oops/releases) are the install paths that exist today. Homebrew, npm, and a curl install script are planned.
