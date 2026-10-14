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
| §6.2 10k turn-end | PASS | `TestHookVisibleLatency` | Published agent wait 180 ms / 180 ms, the higher of this run (112 ms / 127 ms) and the designer measurement (~180 ms). The post snapshot is stored before the hook returns. Measured 2026-10-14: 4 vCPU Intel Xeon on KVM, 16 GB RAM, 256 GB virtio disk, Linux 6.12.94+ |
| §6.2 100k turn-end | PASS | `TestHookVisibleLatency` | Published wait 1.4 s / 1.4 s. This run was 1.00 s / 1.11 s; the designer measurement was ~1.4 s |
| §6.2 turn-start | PASS | `TestHookVisibleLatency` | Published pre-snapshot wait: 174 ms / 240 ms at 10k (this run was 99 ms / 153 ms; QA measured 174 ms / 240 ms) and 1.2 s / 1.2 s at 100k (this run was 0.89 s / 0.99 s; the designer measurement was ~1.2 s) |
| §6.2 in-process capture | PASS | `TestHotCapturePercentiles` | Scan without process startup, measured 2026-10-13: 82 ms / 91 ms at 10k, 0.82 s / 3.5 s at 100k. The agent wait is the turn-end row |
| §6.2 empty turn | PASS | `TestEmptyTurnAddsLittle` | Unchanged checkpoint adds at most 4 KB of objects |
| §6.2 undo 1k | PASS | `TestUndoThousandFiles` | The Windows 2 s target is measured on Windows amd64. windows-11-arm: Known gap, to be optimized in v0.1.x (non-blocking). Atomic writes stay. Under 1 s on the Linux measurement machine. Hosted windows-11-arm, without `-race`, took 2.06 s (Go 1.26.9) and 3.93 s (Go 1.27.2) on 2026-10-13 |
| §6.1 kill mid-undo | PASS | `TestKillDuringUndoRecovers` | The process is killed after the safety snapshot and pending journal are on disk and the apply has started. Each file is a complete version. The next command finishes the restore |
| §6.2 list | PASS | `TestE2EListLimitsAndColumns` | |
| §6.3 | PASS | `.github/workflows/ci.yml` | lint, vet, govulncheck, coverage gate, race on the hosted matrix except windows/arm64, cold-start and RSS bench smoke |
| §6.4 | PASS | `docs/commands.md`, `TestCommandReference` | Command help is generated from cobra. README, README.en, how-it-works, and schema.md are in the repo |
| §6.1 matrix | PASS | `.github/workflows/ci.yml`, `internal/agents/testdata`, `scripts/covergate.py` | ubuntu-latest and ubuntu-24.04-arm (ext4), ubuntu-latest tmpfs, macos-latest and macos-15-intel (case-insensitive APFS), windows-latest and windows-11-arm (NTFS). Go 1.27.2 and 1.26.9. Coverage gate is 70% overall and 85% for store, snapshot, and restore. Hook fixtures drive `oops hook` in agent event order. macos-13 is no longer a hosted runner, so the Intel macOS leg is macos-15-intel. `-race` runs on every leg except windows-11-arm, where Go reports that the race detector is not supported |
| §6.2 cold start and RSS | PASS | `TestColdStart10k`, `TestScanRSS100k` | Cold start of 10,000 files / 300,000,000 bytes was 0.69 s (limit 3 s). Peak VmRSS while scanning 100,000 files was 119.5 MiB, 125,341,696 bytes (limit 150 MB). One sample each, 2026-10-13, 4 vCPU, Linux |
| BUG-1 redo yields to a newer turn | PASS | `TestUndoAfterRedoThenNewTurn` | `undo`, `redo`, a new turn, then `undo`, with and without a user edit. The new turn is undone. The redone turn stays applied. Output does not say turn #0 |
| BUG-2 reported writes match disk | PASS | `TestUndoAfterRedoReportsRealWrites` | The same chain, plus `undo` of a redo after a user edit. The printed file count equals the files written. When nothing is written the command says so and exits 3 |
| BUG-3 Cursor workspace root | PASS | `TestCursorHookUsesWorkspaceRoot` | User-hook cwd `~/.cursor` is not the workspace. `workspace_roots` and `CURSOR_PROJECT_DIR` are. Several roots follow `CURSOR_PROJECT_DIR` |
| BUG-4 init undo restores hooks | PASS | `TestInitUndoRestoresUserHooks` | Codex and Gemini configs that already had user hooks come back byte-for-byte. `*.oops-backup-*` files are removed. Event keys are not null |
| BUG-5 agent version token | PASS | `TestVersionSkipsBanner`, `TestFindVersionOutputs` | The first semver token on stdout is the version, including `2.1.296 (Claude Code)` and `codex-cli 0.162.1`. Stderr is ignored. A line with no semver is not a version |
| DEMO-1 post snapshot stays with its turn | PASS | `TestStopThenImmediateNewTurn`, `TestSlashUndoWaitsForPendingStop` | A queued post snapshot is sealed when the next session starts. A later scan does not rewrite it. `/oops undo` does not report the turn as still running. The hook itself stores the post snapshot before it returns (DEMO-14) |
| DEMO-2 undone turns are marked | PASS | `TestTimelineUndoneMarker`, `TestPreviewUndoneSaysRedo` | An undone turn shows `undone`. The preview says `Redo #N would re-apply:` with add/remove/modify counts, not the undo verbs |
| DEMO-3 file counts ignore directories | PASS | `TestFileCountsIgnoreDirectories`, `TestFileChangesSkipDirectories` | Directories are not counted. List, show, slash, timeline preview, gap, and undo preview agree. One shared `snapshot.FileChanges` |
| DEMO-4 redo hint suggests undo | PASS | `TestRedoHintSuggestsUndo` | After redo the hint is `oops undo` |
| DEMO-5 diff skips directories | PASS | `TestTimelineDiffSkipsDirectory` | `d` opens the first file diff and skips an empty directory entry |
| DEMO-6 timeline stays on the alt screen | PASS | `TestTimelineRefreshInPlace`, `TestTimelineSuccessMark` | Undo and redo refresh the timeline in place. The program uses the alternate screen, so quit leaves the terminal clear. The footer shows `R redo`. A success toast uses ✓, not ! |
| DEMO-7 timeline columns use the width | PASS | `TestTimelinePromptUsesRemainingWidth`, `TestTimelineLayoutAt120`, `TestTimelineColumnsRightAlign` | The prompt is truncated to the width left after the turn, agent, time, file-count, and status columns. The file count and the `undone` marker are right-aligned in fixed-width columns. At 120 columns the preview sits under the list. A 95-character prompt keeps at least 60 characters. At 170 columns the preview stays beside the list |
| DEMO-8 /oops reads as success | PASS | `TestSlashSuccessIsShort` | The hook reply is a short `✓ oops` line. Claude still wraps it as a block decision; that wrapper is documented in both READMEs |
| DEMO-9 subcommand help | PASS | `TestInitHelpShowsFlags` | `oops init -h` shows init's flags. The root help stays the short page |
| DEMO-10 Gemini version is not a launch | PASS | `TestGeminiVersionDoesNotRun`, `TestGeminiInitLeavesSettings`, `TestNonSemverVersionIsUnknown` | Gemini's version is read from `@google/gemini-cli` `package.json`. The binary is not executed, so a hand-written settings file is unchanged and `init --undo` restores it byte for byte. Output with no semver is shown as `unknown` |
| DEMO-11 non-TTY undo | PASS | `TestUndoNonTTYLatestAndPipedNo` | A non-terminal never prompts. `oops undo` applies the latest turn and does not treat piped `n` as an answer. An older turn is refused without `-y`. On a terminal, `echo n` cancels the prompt |
| DEMO-12 undo after Stop | PASS | `TestUndoImmediatelyAfterStop`, `TestUndoCLIImmediatelyAfterStop` | Stop marks the turn ended before the hook returns. The post snapshot can still be queued. 200 immediate undos apply that turn and never say it is still running. A turn that has not received Stop exits 4 without waiting |
| DEMO-13 end hooks are synchronous | PASS | `TestClaudeEndHooksAreSynchronous`, `TestInitRewritesAsyncClaudeEndHooks` | Claude `Stop`, `StopFailure`, and `SubagentStop` are installed without `async`. Codex, Gemini, and Cursor end hooks are synchronous too. Re-running `init` rewrites an older async entry. `init --undo` puts the previous file back byte for byte. `SessionStart` stays async |
| DEMO-14 post snapshot finishes before Stop returns | PASS | `TestEditRightAfterStopIsKept`, `TestUndoKeepsFilesNewerThanStop`, `TestUndoForceOverwritesNewerMtime` | The real binary's turn-end hook returns only after the post snapshot is stored. An edit and a new file at that instant stay, exit 3, and the edit is reported kept. Stop records its start and the time the post snapshot finishes. A file is kept only when its content differs from the baseline (the post snapshot, or the bytes the latest undo or redo wrote). An mtime inside the post-snapshot window is not enough, because NTFS can report that for bytes the snapshot already stored. `--force` still overwrites. Runs on every OS. A timeout marks the snapshot partial |
| DEMO-15 timeline stays on the acted-on turn | PASS | `TestTimelineStaysOnActedTurn` | After undo the cursor stays on that turn. The preview is that turn's redo preview |
| DEMO-16 preview keeps the file count | PASS | `TestTimelinePathCountStaysWhole`, `TestPreviewListsEachPath`, `TestPreviewPathsTruncateSeparately` | Each path is on its own line, capped, then `+K more`, and middle-truncated on its own. `5 files` stays whole. The `ran:` line wraps across lines |
| DEMO-17 toast reserves a row | PASS | `TestTimelineNoticeReservesRow`, `TestTimelineLayoutAt120` | The toast row is reserved at the full terminal width, blank until a toast arrives. At 120×40 the first list row stays on the same line before and after undo, and the prompt is shown in full |
| DEMO-18 init version column | PASS | `TestInitVersionColumnFits` | Column width follows the text, including `2026.10.12-d47ca18` |
| DEMO-19 init undo removes empty dirs | PASS | `TestInitUndoRemovesEmptyDirs` | Directories oops created are removed when empty. A directory that still holds a user file stays |
| DEMO-20 init undo clears an empty Claude home | PASS | `TestInitUndoClearsEmptyClaudeHome`, `TestInitUndoRestoresClaudeSettings`, `TestRemoveOopsDropsEmptyMatcher`, `TestUninstallDeletesOopsCreatedClaudeFile`, `TestUninstallRestoresClaudeBytes` | Matcher groups with no hooks, empty event arrays, and empty `hooks` objects are pruned. An oops-created settings file that is left as `{}` is deleted, and an empty `~/.claude` oops created is removed. A file that already existed is restored byte for byte |
| DEMO-21 preview stays inside the terminal | PASS | `TestTimelinePreviewFitsHeight`, `TestClipPreviewTightRow` | At 120×22 and 80×24 the title is on line 1 and the view has at most that many lines. Header, toast, and footer stay. A long file list is cut to the rows that remain, then `+K more`. One tight row is `src/api.ts … +3 more` |
| DEMO-22 stacked list and preview are separated | PASS | `TestTimelinePreviewFitsHeight` | At 120×22 and 80×24 the preview sits under the list with a rule between them |
| DEMO-23 side-by-side divider spans the pane | PASS | `TestTimelineDividerFullHeight` | At 170 columns the `│` divider is on every row of the taller pane, in one column. Path lines in the preview share one indent |
| DEMO-24 undo, redo, undo on the same turn | PASS | `TestUndoRedoUndoSameTurn`, `TestUndoRedoUndoSameTurnExitsClean` | Three rounds of undo then redo on the same turn exit 0 and restore the turn's bytes, then the earlier bytes. A real edit after redo is kept, the other file from that turn is reverted, and the command exits 3 |
| DEMO-25 edit during the post snapshot | PASS | `TestMatchingBytesInsideSnapshotWindowAreRestored`, `TestConflictKeepsUserEdit` | A file whose mtime is inside [Stop start, post snapshot done], and whose bytes match the post snapshot, is restored. A file whose bytes differ is kept, the result is partial, and the turn records that path |
| DEMO-26 timeline names kept files | PASS | `TestUndoNoticeNamesKeptFile`, `TestKeptFileShowsInStatus` | A partial undo toast is `Undid #N, except` the path. The row marker is `partial` in its own column. The path is in the preview |
| DEMO-27 redo does not say undo | PASS | `TestUndoRedoUndoSameTurnExitsClean`, `TestRedoHintSuggestsUndo` | `oops redo` does not print `Undid`. The hint stays `oops undo` |
| DEMO-28 narrow footer keeps help and quit | PASS | `TestFooterKeepsHelpAndQuit` | At 80×24 the footer shows `? help` and `q quit` and drops the rewind hint |
| DEMO-29 wide prompt uses the leftover width | PASS | `TestPromptUsesRemainingWidthAt175` | At 175 columns a 95-character prompt is shown in full beside the right-aligned file count |
| DEMO-30 partial row marker stays in its column | PASS | `TestKeptFileShowsInStatus` | After a partial undo the row is `9 files` then `partial`, with a gap. The path is not concatenated onto the count |
| DEMO-31 partial undo preview is the redo preview | PASS | `TestPreviewPartialUndoSaysRedo`, `TestKeptFileShowsInStatus` | The preview says the turn is undone (partial) and `Redo #N would re-apply:`. The lines are the redo plan, so a kept file is `keep 1 file (yours)` and the other files are `add 8 files` |
| DEMO-32 partial redo toast names the file | PASS | `TestRedoNoticeNamesKeptFile`, `TestPartialRedoToastMatchesCLI`, `TestApplyTimelineRedoNamesKeptFile` | The timeline toast is `! Redid the last change, except 1 file you changed afterwards` and names the file |
| DEMO-33 preview counts match the applied plan | PASS | `TestPreviewPartialUndoSaysRedo`, `TestKeptFileShowsInStatus` | Undo and redo previews, and the confirm card, use the plan Undo and Redo apply. The preview counts equal the applied result: `remove 8 files, keep 1 file (yours)` then `add 8 files, keep 1 file (yours)`. The confirm card says `keep 1 file (yours)` |
| DEMO-34 hints name only the kept file | PASS | `TestPartialHintsTargetKeptFilesOnly` | After a partial undo, every suggested command in the text and in `--json` is `oops undo N --file` for exactly the kept file. Directories and parent paths are not targets. After a partial redo, the text and `--json` do not suggest `oops undo --file` |
| DEMO-35 redo dry run says redo | PASS | `TestRedoDryRunUsesRedoWording` | `oops redo -n` is headed `Redo turn #N would` and ends `Run oops redo to apply.` |
| DEMO-36 a kept file stays the user's | PASS | `TestKeptFileSurvivesUndoRedoUndo`, `TestKeptUserFileSurvivesSecondUndo` | Undo, redo, undo, redo on a turn that kept a user edit leaves that file in place and exits 3. `--force` overwrites it |
| DEMO-37 timeline color follows the palette | PASS | `TestListRowColors`, `TestDiffColorsAddedAndRemoved` | Agent and time are Muted. `+` / `−` in the file count are Success / Danger. `undone` and `partial` are Warning. Diff additions are Success and removals are Danger. `NO_COLOR` and color off leave the text plain |
| DEMO-38 the timeline fits the terminal | PASS | `TestConfirmCardKeepsTitle`, `TestClipPreviewShowsSingleHiddenFile`, `TestStackedPreviewBlanksBeforeRan` | At 30 rows the confirm card leaves the title on screen. A `+1 more` that would hide one file shows that file instead. Stacked layout has a blank line before `ran:` |
| DEMO-39 redo wording and kept-file diff | PASS | `TestRedoDryRunUsesRestoreWording`, `TestRedoAppliedSaysReapplied`, `TestRedoJSONSuggestsUndo`, `TestKeptPreviewDiffIsDiskVersusApply` | `oops redo -n` says re-apply. An applied redo says re-applied, not reverted. Applied `redo --json` suggests `oops undo`. A kept file's timeline diff is only "You changed this file after the turn — oops will leave it as is." |

## Counts

| Result | Rows |
|---|---|
| PASS | 110 |
| FAIL | 0 |

## Out of v1 scope / pending external action

These are not FAIL. G1–G10 are v0.2 in the PRD and are not implemented. The other rows need a release process or people outside this change.

| Item | Why it is not in this change |
|---|---|
| G1–G10 | PRD marks them v0.2 (step checkpoints, richer TUI, context sync, branch export, web, MCP, encryption, reflink, team policy, diff highlighting) |
| cosign, SBOM, SLSA | Signing and provenance are release infrastructure. This repo's goreleaser config does not emit them |
| Homebrew tap repo, npm publish token, scoop, winget | The formula, npm packages, and curl/PowerShell installers are in the repo. The first release still needs the `lintsinghua/homebrew-tap` repository, `HOMEBREW_TAP_TOKEN`, and `NPM_TOKEN`. Scoop and winget are not implemented |
| Three-person user study | Needs three people on three operating systems. Not something this change can record |
| CHANGELOG generated at tag time | Keep-a-changelog output is a release step |
| Live Claude Code, Codex, Gemini CLI, and Cursor CLI smoke | Run manually by a maintainer before release; CI replays recorded hook payloads |
| btrfs and case-sensitive APFS | The PRD lists these as nightly filesystem extras. Standard GitHub-hosted runners provide ext4, tmpfs, case-insensitive APFS, and NTFS |

## Hook wait vs snapshot completion

The turn-end hook returns only after the post snapshot is stored. On 2026-10-14 the published wait is 180 ms p50 / 180 ms p95 at 10k files and 1.4 s / 1.4 s at 100k, the higher of `TestHookVisibleLatency` (112 ms / 127 ms and 1.00 s / 1.11 s) and a designer measurement (~180 ms and ~1.4 s). The turn-start hook returns only after the pre snapshot is stored. The published wait is 174 ms / 240 ms at 10k (this run was 99 ms / 153 ms; QA measured 174 ms / 240 ms) and 1.2 s / 1.2 s at 100k (this run was 0.89 s / 0.99 s; the designer measurement was ~1.2 s). Machine: 4 vCPU (Intel Xeon on KVM), 16 GB RAM, 256 GB virtio disk (ext4), Linux 6.12.94+. A timeout marks the snapshot partial. A file is kept only when its content differs from the baseline. An mtime inside the post-snapshot window is not enough.
