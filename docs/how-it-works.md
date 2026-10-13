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

`oops init` writes hooks whose command contains `oops hook`. `oops init --undo` removes only those entries and leaves the rest of the JSON config. Codex still needs a one-time `/hooks` trust inside Codex; init says so, and `oops doctor` keeps warning until that trust exists because oops cannot see it. Claude Code also gets `~/.claude/commands/oops.md` so `/oops` is a local command. JSON configs are re-serialized, so key order is not preserved. The previous file is copied to `*.oops-backup-<pid>`.

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

Before any apply, oops writes a safety snapshot and a journal at `workspaces/<id>/journal/current.json`. If the process dies mid-apply, the next command either finishes the journal or rolls it back. `oops redo` reads the latest applied operation and puts those paths back to the pre-undo state when they still match what undo wrote. `oops rewind <n>` replays the diffs from turn n through the latest turn, so edits that happened in the gaps and were not part of those turns are kept.

The last finished turn is undone immediately by `oops undo`. An older turn, or `--force`, prints a preview and asks on a terminal; without a terminal it exits 2 unless you pass `-y`. `oops rewind` always asks unless you pass `-y`. The timeline (`oops` with a terminal, not `--json`, not `TERM=dumb`) asks before undo, rewind, and redo.

`undo`, `rewind`, `restore`, and `gc` accept `-n` / `--dry-run`. `redo` and `snap` do not. `undo -n --json` still prints the text preview.

## Garbage collection

`oops gc` deletes turns in the current workspace that are past `retention.keep_turns` (default 500) or older than `retention.keep_days` (default 30), then deletes objects no remaining snapshot still references. Objects newer than one hour are kept even when nothing references them. `oops gc --all` deletes that workspace's turns and snapshot rows. `oops gc --all --global` does that for every workspace. Both `--all` forms ask for confirmation unless `-y` or `-n` is set.

`retention.workspace_max` (default 2GB) and `retention.global_max` (default 10GB) are stored in config. `oops doctor` prints the workspace cap next to the current object size. Nothing in this build refuses a snapshot or runs gc because those caps were exceeded.

## Commands and exit codes

| Code | Meaning |
|---|---|
| 0 | ok, or the user declined a confirmation |
| 1 | error |
| 2 | usage, or confirmation is required and there is no terminal |
| 3 | partial: some files were kept |
| 4 | nothing to act on |
| 130 | interrupted |

`NO_COLOR`, `--color=never`, and `TERM=dumb` turn color off. `OOPS_ASCII=1`, `TERM=dumb`, and a non-UTF-8 locale use ASCII glyphs. Diffs are printed directly. `--no-pager` is accepted and does not launch a pager.

## What this build does not do

- It does not install a `SubagentStop` hook. A file written by a background task is covered only if it is on disk when the turn's post snapshot is taken.
- It does not phone home. `oops version` prints the version, commit, and date compiled into the binary. `oops version --check` is not implemented.
- It does not cosign release artifacts or attach an SBOM or SLSA provenance. Release archives are tar.gz (zip on Windows) plus `checksums.txt` (sha256).
- It does not enforce the 2GB / 10GB caps, and it does not expire turns until you run `oops gc`.
- Minimum tested versions of Claude Code, Codex, Gemini CLI, and Cursor CLI are not recorded. The adapters are in `internal/agents`.
- Homebrew (`oops-cli/tap`) and the npm package (`@oops-cli/oops`) are not published. `go install github.com/lintsinghua/oops/cmd/oops@latest` and [GitHub Releases](https://github.com/lintsinghua/oops/releases) are the install paths that exist today.
