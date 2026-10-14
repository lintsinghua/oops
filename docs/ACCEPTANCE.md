# Acceptance

Checked against `docs/PRD.md` on 2026-10-13. PASS means a test or command named in the row exercised that behavior. The pre-snapshot row is the designer's correctness requirement: the turn-start hook stores the original tree before it returns.

| ID | Result | Verified by | Notes |
|---|---|---|---|
| PRE | PASS | `TestE2EPreSnapshotBeforeHookReturns` | Start hook returns, then the test immediately deletes a directory and rewrites files with no sleep. Undo restores the original bytes. Runs on every OS |
| F1 AC1 | PASS | `TestE2EUndoRestoresDeletedEnv` | `.env` restored after delete while gitignored |
| F1 AC2 | PASS | `TestE2EEditMoveModeSymlinkUnicode`, `TestE2EUndoRestoresDeletedEnv` | Write, delete, rename, mode, symlink |
| F1 AC3 | PASS | `TestE2EGitUntouched` | Workspace `.git` bytes are unchanged by a capture |
| F1 AC4 | PASS | `TestE2ELazyCloseAndRunning` | No turn-end. `oops list` closes the turn from the tree at that moment |
| F2 AC1 | PASS | `TestUndoRestoresBytes` | |
| F2 AC2 | PASS | `TestUndoRestoresBytes`, `TestE2EUndoRestoresDeletedEnv` | |
| F2 AC3 | PASS | `TestConflictKeepsUserEdit`, `TestE2EShowJSONAndConflict` | Exit 3, user bytes kept |
| F2 AC4 | PASS | `TestE2EShowJSONAndConflict` | Last turn on a non-TTY proceeds without `-y` |
| F2 AC4b | PASS | `TestE2ELazyCloseAndRunning` | Running turn exits 4. `--running` closes it and undoes |
| F2 AC5 | PASS | `TestE2EEditMoveModeSymlinkUnicode` | `undo -n --json` |
| F2 AC6 | PASS | `TestUndoRestoresBytes`, `TestE2EEditMoveModeSymlinkUnicode` | |
| F3 AC1 | PASS | `TestE2EUndoFileAndMissing` | `--file` glob |
| F3 AC2 | PASS | `TestE2EUndoFileAndMissing` | Missing path exits 2 |
| F4 AC1 | PASS | `TestE2ERewindKeepsGap` | Non-TTY without `-y` exits 2 and changes nothing |
| F4 AC2 | PASS | `TestE2ERewindKeepsGap` | A file written between turns is kept |
| F5 AC1 | PASS | `TestRedoRestoresUndo`, `TestGCKeepsSafetySnapshot` | Safety snapshot is written and survives gc |
| F5 AC2 | PASS | `TestRedoRestoresUndo`, `TestE2ERedoAndSnapDryRun` | |
| F5 AC3 | PASS | `TestRedoStack` | Undo, undo, redo, redo, undo, redo, then an explicit undo discards the redo stack |
| F6 AC1 | PASS | `TestE2EListLimitsAndColumns` | Turn number, agent, file count |
| F6 AC2 | PASS | `TestE2EListLimitsAndColumns` | Default page, `-n 2`, `--all` |
| F6 AC3 | PASS | `TestE2ERewindKeepsGap` | Gap line |
| F6 AC4 | PASS | `TestE2ESubagentStopDoesNotSplit` | `list --json` |
| F7 AC1 | PASS | `TestE2EShowLabelsAndBinaryDiff` | `untracked` and `ignored` on text and JSON |
| F7 AC2 | PASS | `TestE2EDiffPager` | 40+ lines on a TTY go to `$PAGER` |
| F7 AC3 | PASS | `TestE2EShowLabelsAndBinaryDiff` | Binary diff has no NUL bytes. `--stat` says binary |
| F8 AC1 | PASS | `TestE2ERestore` | Deleted file comes back |
| F8 AC2 | PASS | `TestE2ERestore` | A different file is kept unless `--force` |
| F9 AC1 | PASS | `TestManualSnapSameTree` | pre snapshot id equals post |
| F9 AC2 | PASS | `TestE2ESnapQuiet`, `TestE2ERedoAndSnapDryRun` | `snap -n` writes nothing. `snap -q` prints nothing |
| F10 AC1–AC8 | PASS | `TestE2EInitAgents` | Four agents, idempotent second run, backup, `--print`, `--project`, Codex `/hooks` note, Claude `/oops` file, last line is the type-oops line, `--undo` keeps other JSON, `--undo --delete-data -y` removes snapshots |
| F11 AC1–AC3 | PASS | `TestE2ERunAndSlash` | One turn, stdout and exit code passed through, idle split opens another turn |
| F12 AC1–AC7 | PASS | `TestTimelineKeys`, `TestTimelineNarrowAndEmpty`, `TestTimelineMoveAndRewind`, `TestE2ENonTTYTimelineAndInterrupt` | Select, diff, confirm-then-undo, narrow column, empty state, non-TTY status. First view under 150 ms |
| F13 AC1–AC2 | PASS | `TestE2EDoctorGCIgnorePartial` | `doctor --json` and a storage warning over the workspace cap |
| F14 AC1 | PASS | `TestMaintainKeepsRecentTurns`, `TestMaintainEnforcesByteCap` | |
| F14 AC2 | PASS | `TestE2EDoctorGCIgnorePartial` | `gc -n` prints `Would free` and leaves the turns |
| F14 AC3 | PASS | `TestGCKeepsSafetySnapshot` | Safety snapshot still loads after gc, and redo still works |
| F15 AC1 | PASS | `TestE2EUndoRestoresDeletedEnv` | |
| F15 AC2 | PASS | `TestE2EDoctorGCIgnorePartial` | `.oopsignore` `*` with `!keep.txt` |
| F15 AC3 | PASS | `TestE2ELargeFileUnprotected` | |
| F16 AC1 | PASS | `go test -race ./...` on ubuntu, macos, windows | Shared scenarios live in `internal/cli` |
| F16 AC2 | PASS | `TestE2EWindowsLongPath`, `TestE2EWindowsCaseRename`, `TestE2EWindowsLockedFile` | Skipped off Windows. Case-only rename restores the original spelling. An exclusive lock fails the undo, names the file, and leaves the bytes in place |
| F17 AC1 | PASS | `TestE2EShowLabelsAndBinaryDiff`, `TestE2EDoctorGCIgnorePartial`, `docs/schema.md` | |
| F17 AC2 | PASS | `TestE2EShowJSONAndConflict` (3), `TestE2EUndoRestoresDeletedEnv` (0), `TestUnknownCommand` (2), `TestE2ENonTTYTimelineAndInterrupt` (130) | |
| F18 AC1 | PASS | `TestE2EPartialNoticeAndHookExit` | A bad hook event still exits 0 |
| F18 AC2 | PASS | `TestPartialSnapshotMarksTurn`, `TestE2EPartialNoticeAndHookExit` | A 1ns deadline marks the snapshot partial and the hook still returns |
| F18 AC3 | PASS | `TestE2EPartialNoticeAndHookExit`, `docs/how-it-works.md` | Claude `systemMessage` is the exception |
| F19 AC1 | PASS | `TestE2EInitAgents` | Claude command file and `UserPromptExpansion` |
| F19 AC2 | PASS | `TestE2ERunAndSlash` | `/oops` and `/oops diff` answer locally |
| F19 AC3 | PASS | README agent table | |
| F19 AC4 | PASS | README “Supported agents” | |
| F20 AC1 | PASS | `TestE2ESubagentStopDoesNotSplit` | |
| F20 AC2 | PASS | `TestNoticeOncePerReason` | Same reason is stored once |
| F20 AC3 | PASS | `TestE2EPartialNoticeAndHookExit` | Claude gets one `oops ·` systemMessage. The next command does not repeat it |
| §6.2 10k turn-end | PASS | `TestHookVisibleLatency` | Agent waits 4 ms / 5 ms for the hook to return. That is not snapshot completion |
| §6.2 100k turn-end | PASS | `TestHookVisibleLatency` | Agent waits 5 ms / 6 ms for the hook to return |
| §6.2 turn-start | PASS | `TestHookVisibleLatency` | Agent waits for the pre snapshot: 92 ms / 106 ms at 10k, 0.86 s / 0.92 s at 100k. Measured 2026-10-13, 4 vCPU |
| §6.2 background completion | PASS | `TestHotCapturePercentiles` | In-process Capture after the hook has returned: 82 ms / 91 ms at 10k, 0.82 s / 3.5 s at 100k |
| §6.2 empty turn | PASS | `TestEmptyTurnAddsLittle` | Unchanged checkpoint adds at most 4 KB of objects |
| §6.2 undo 1k | PASS | `TestUndoThousandFiles` | The Windows 2 s target is measured on Windows amd64. windows-11-arm: Known gap, to be optimized in v0.1.x (non-blocking). Atomic writes stay. Under 1 s on the Linux measurement machine. Hosted windows-11-arm, without `-race`, took 2.06 s (Go 1.26.9) and 3.93 s (Go 1.27.2) on 2026-10-13 |
| §6.1 kill mid-undo | PASS | `TestKillDuringUndoRecovers` | The process is killed after the safety snapshot and pending journal are on disk and the apply has started. Each file is a complete version. The next command finishes the restore |
| §6.2 list | PASS | `TestE2EListLimitsAndColumns` | |
| §6.3 | PASS | `.github/workflows/ci.yml` | lint, vet, govulncheck, coverage gate, race on the hosted matrix except windows/arm64, cold-start and RSS bench smoke |
| §6.4 | PASS | `docs/commands.md`, `TestCommandReference` | Command help is generated from cobra. README, README.zh-CN, how-it-works, and schema.md are in the repo |
| §6.1 matrix | PASS | `.github/workflows/ci.yml`, `internal/agents/testdata`, `scripts/covergate.py` | ubuntu-latest and ubuntu-24.04-arm (ext4), ubuntu-latest tmpfs, macos-latest and macos-15-intel (case-insensitive APFS), windows-latest and windows-11-arm (NTFS). Go 1.27.2 and 1.26.9. Coverage gate is 70% overall and 85% for store, snapshot, and restore. Hook fixtures drive `oops hook` in agent event order. macos-13 is no longer a hosted runner, so the Intel macOS leg is macos-15-intel. `-race` runs on every leg except windows-11-arm, where Go reports that the race detector is not supported |
| §6.2 cold start and RSS | PASS | `TestColdStart10k`, `TestScanRSS100k` | Cold start of 10,000 files / 300,000,000 bytes was 0.69 s (limit 3 s). Peak VmRSS while scanning 100,000 files was 119.5 MiB, 125,341,696 bytes (limit 150 MB). One sample each, 2026-10-13, 4 vCPU, Linux |
| BUG-1 redo yields to a newer turn | PASS | `TestUndoAfterRedoThenNewTurn` | `undo`, `redo`, a new turn, then `undo`, with and without a user edit. The new turn is undone. The redone turn stays applied. Output does not say turn #0 |
| BUG-2 reported writes match disk | PASS | `TestUndoAfterRedoReportsRealWrites` | The same chain, plus `undo` of a redo after a user edit. The printed file count equals the files written. When nothing is written the command says so and exits 3 |
| BUG-3 Cursor workspace root | PASS | `TestCursorHookUsesWorkspaceRoot` | User-hook cwd `~/.cursor` is not the workspace. `workspace_roots` and `CURSOR_PROJECT_DIR` are. Several roots follow `CURSOR_PROJECT_DIR` |
| BUG-4 init undo restores hooks | PASS | `TestInitUndoRestoresUserHooks` | Codex and Gemini configs that already had user hooks come back byte-for-byte. `*.oops-backup-*` files are removed. Event keys are not null |
| BUG-5 agent version token | PASS | `TestVersionSkipsBanner`, `TestFindVersionOutputs` | The first semver token on stdout is the version, including `2.1.296 (Claude Code)` and `codex-cli 0.162.1`. Stderr is ignored. A line with no semver is not a version |
| DEMO-1 post snapshot stays with its turn | PASS | `TestStopThenImmediateNewTurn`, `TestSlashUndoWaitsForPendingStop` | Stop exits immediately, or never queues, and the next session starts at once. The previous post is the tree at that boundary. A later scan does not rewrite it. `/oops undo` does not report the turn as still running |
| DEMO-2 undone turns are marked | PASS | `TestTimelineUndoneMarker`, `TestPreviewUndoneSaysRedo` | An undone turn shows `undone`. The preview says `Redo #N would re-apply:` with add/remove/modify counts, not the undo verbs |
| DEMO-3 file counts ignore directories | PASS | `TestFileCountsIgnoreDirectories`, `TestFileChangesSkipDirectories` | Directories are not counted. List, show, slash, timeline preview, gap, and undo preview agree. One shared `snapshot.FileChanges` |
| DEMO-4 redo hint suggests undo | PASS | `TestRedoHintSuggestsUndo` | After redo the hint is `oops undo` |
| DEMO-5 diff skips directories | PASS | `TestTimelineDiffSkipsDirectory` | `d` opens the first file diff and skips an empty directory entry |
| DEMO-6 timeline stays on the alt screen | PASS | `TestTimelineRefreshInPlace`, `TestTimelineSuccessMark` | Undo and redo refresh the timeline in place. The program uses the alternate screen, so quit leaves the terminal clear. The footer shows `R redo`. A success toast uses ✓, not ! |
| DEMO-7 timeline columns use the width | PASS | `TestTimelinePromptUsesRemainingWidth` | At 120 columns the prompt keeps the leftover width, the file count and undone marker stay visible, and paths are middle-truncated |
| DEMO-8 /oops reads as success | PASS | `TestSlashSuccessIsShort` | The hook reply is a short `✓ oops` line. Claude still wraps it as a block decision; that wrapper is documented in both READMEs |
| DEMO-9 subcommand help | PASS | `TestInitHelpShowsFlags` | `oops init -h` shows init's flags. The root help stays the short page |
| DEMO-10 Gemini version is not a launch | PASS | `TestGeminiVersionDoesNotRun`, `TestGeminiInitLeavesSettings`, `TestNonSemverVersionIsUnknown` | Gemini's version is read from `@google/gemini-cli` `package.json`. The binary is not executed, so a hand-written settings file is unchanged and `init --undo` restores it byte for byte. Output with no semver is shown as `unknown` |
| DEMO-11 non-TTY undo | PASS | `TestUndoNonTTYLatestAndPipedNo` | A non-terminal never prompts. `oops undo` applies the latest turn and does not treat piped `n` as an answer. An older turn is refused without `-y`. On a terminal, `echo n` cancels the prompt |
| DEMO-12 undo after Stop | PASS | `TestUndoImmediatelyAfterStop`, `TestUndoCLIImmediatelyAfterStop` | Stop marks the turn ended before the hook returns. The post snapshot can still be queued. 200 immediate undos apply that turn and never say it is still running. A turn that has not received Stop exits 4 without waiting |
| DEMO-13 end hooks are synchronous | PASS | `TestClaudeEndHooksAreSynchronous`, `TestInitRewritesAsyncClaudeEndHooks` | Claude `Stop`, `StopFailure`, and `SubagentStop` are installed without `async`. Codex, Gemini, and Cursor end hooks are synchronous too. Re-running `init` rewrites an older async entry. `init --undo` puts the previous file back byte for byte. `SessionStart` stays async |

## Counts

| Result | Rows |
|---|---|
| PASS | 84 |
| FAIL | 0 |

## Out of v1 scope / pending external action

These are not FAIL. G1–G10 are v0.2 in the PRD and are not implemented. The other rows need a release process or people outside this change.

| Item | Why it is not in this change |
|---|---|
| G1–G10 | PRD marks them v0.2 (step checkpoints, richer TUI, context sync, branch export, web, MCP, encryption, reflink, team policy, diff highlighting) |
| cosign, SBOM, SLSA | Signing and provenance are release infrastructure. This repo's goreleaser config does not emit them |
| Homebrew, npm, scoop, winget, curl install | Planned install channels. The live paths are `go install` and GitHub Releases |
| Three-person user study | Needs three people on three operating systems. Not something this change can record |
| CHANGELOG generated at tag time | Keep-a-changelog output is a release step |
| Live Claude Code, Codex, Gemini CLI, and Cursor CLI smoke | Run manually by a maintainer before release; CI replays recorded hook payloads |
| btrfs and case-sensitive APFS | The PRD lists these as nightly filesystem extras. Standard GitHub-hosted runners provide ext4, tmpfs, case-insensitive APFS, and NTFS |

## Hook wait vs snapshot completion

The 5 ms and 6 ms numbers are the time the agent waits for the turn-end hook process to return. The post snapshot then finishes in the background (0.82 s p50 / 3.5 s p95 at 100k files). The turn-start hook stays synchronous: it returns only after the pre snapshot is stored (0.86 s p50 / 0.92 s p95 at 100k files on this run).
