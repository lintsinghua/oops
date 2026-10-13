# Acceptance

Checked against `docs/PRD.md` on 2026-10-13. PASS means a test or command named in the row exercised that behavior. G1–G10 are v0.2 in the PRD and were not implemented.

| ID | Result | Verified by | Notes |
|---|---|---|---|
| F1 AC1 | PASS | `TestE2EUndoRestoresDeletedEnv` | `.env` restored after delete while gitignored |
| F1 AC2 | PASS | `TestE2EEditMoveModeSymlinkUnicode`, `TestE2EUndoRestoresDeletedEnv` | Snapshot is the tree. Covered: write, delete, rename, mode, symlink |
| F1 AC3 | FAIL | — | `.git` is pruned in `snapshot.PruneDir`. No test asserts the workspace `.git` is unchanged |
| F1 AC4 | FAIL | — | Lazy-close is implemented on the next command. No test kills the agent before Stop |
| F2 AC1 | PASS | `TestUndoRestoresBytes` | |
| F2 AC2 | PASS | `TestUndoRestoresBytes`, `TestE2EUndoRestoresDeletedEnv` | |
| F2 AC3 | PASS | `TestConflictKeepsUserEdit`, `TestE2EShowJSONAndConflict` | Exit 3, user bytes kept |
| F2 AC4 | PASS | `TestE2EShowJSONAndConflict` | Last turn on a non-TTY proceeds without `-y` (exit 3, not 2) |
| F2 AC4b | FAIL | — | Running-turn exit 4 and `--running` are implemented, not covered by a test |
| F2 AC5 | PASS | `TestE2EEditMoveModeSymlinkUnicode` | `undo -n --json` |
| F2 AC6 | PASS | `TestUndoRestoresBytes`, `TestE2EEditMoveModeSymlinkUnicode` | Bytes, executable bit, symlink target |
| F3 AC1 | FAIL | — | `--file` and globs are implemented. No test |
| F3 AC2 | FAIL | — | Missing-path message is implemented. No test |
| F4 AC1 | FAIL | — | `Rewind` asks unless `-y`. No test |
| F4 AC2 | FAIL | — | Gap retention is implemented. No test |
| F5 AC1 | PASS | `TestRedoRestoresUndo` | `Engine.apply` writes the safety snapshot before restore |
| F5 AC2 | PASS | `TestRedoRestoresUndo`, `TestE2ERedoAndSnapDryRun` | |
| F5 AC3 | FAIL | — | `redo` inverts the latest applied op. A stack that undoes that redo as its own entry is not implemented |
| F6 AC1 | FAIL | — | List text columns are not asserted |
| F6 AC2 | FAIL | — | `-n` and `--all` are not asserted |
| F6 AC3 | FAIL | — | Gap line is not asserted |
| F6 AC4 | PASS | `TestE2ESubagentStopDoesNotSplit` | `list --json` |
| F7 AC1 | FAIL | `TestE2EShowJSONAndConflict` | `show --json` has path, action, and line counts. It does not label untracked or ignored |
| F7 AC2 | FAIL | — | Pager runs for a TTY and 40+ lines (`pageOrPrint`). No test drives a TTY |
| F7 AC3 | FAIL | — | Binary diffs skip unified text. No test |
| F8 AC1 | FAIL | — | `restore` is implemented. No test |
| F8 AC2 | FAIL | — | Confirm / `--force` is implemented. No test |
| F9 AC1 | FAIL | — | `ManualSnap` sets pre = post. No test reads the row |
| F9 AC2 | PASS | `TestE2ERedoAndSnapDryRun` | `snap -n` writes no turn. `-q` is not asserted |
| F10 AC1–AC8 | FAIL | — | Init copy puts “type oops” last and `--delete-data` can remove snapshots. No init test |
| F11 AC1–AC3 | FAIL | — | `oops run` is implemented. No test |
| F12 AC1–AC7 | FAIL | — | TUI has no `teatest` suite. Non-TTY fallback is implemented and not asserted |
| F13 AC1–AC2 | FAIL | — | `oops doctor` is implemented, including a storage warning over the cap. No test |
| F14 AC1 | PASS | `TestMaintainKeepsRecentTurns`, `TestMaintainEnforcesByteCap` | Automatic retention after the background snapshot. Byte trim keeps the newest turn. Objects younger than one hour are not deleted |
| F14 AC2 | FAIL | — | `oops gc -n` is implemented. No test |
| F14 AC3 | FAIL | — | Sweep keeps referenced ids, including safety snapshots. No test |
| F15 AC1 | PASS | `TestE2EUndoRestoresDeletedEnv` | gitignore respected, `.env` protected |
| F15 AC2 | FAIL | — | `.oopsignore` negation is implemented. No test |
| F15 AC3 | PASS | `TestE2ELargeFileUnprotected` | `max_file_size = 20B`, show says not protected, undo does not restore bytes |
| F16 AC1 | PASS | `go test -race ./...` on ubuntu, macos, windows | Shared scenarios live in `internal/cli/e2e_test.go` |
| F16 AC2 | FAIL | `TestE2EWindowsLongPath` | Skipped off Windows. Case-only rename and locked-file retry are not covered by a test here |
| F17 AC1 | PASS | `TestE2EEditMoveModeSymlinkUnicode`, `TestE2EShowJSONAndConflict`, `docs/schema.md` | `undo -n --json`, `show --json` file list, `schema_version` |
| F17 AC2 | PASS | `TestE2EShowJSONAndConflict` (exit 3), `TestE2EUndoRestoresDeletedEnv` (exit 0), `TestUnknownCommand` (exit 2) | 130 is not covered |
| F18 AC1 | PASS | `TestE2ESubagentStopDoesNotSplit` | Hook exit 0 |
| F18 AC2 | FAIL | — | 10s partial timeout is implemented. No test |
| F18 AC3 | PASS | `docs/how-it-works.md` | Normal hook stdout is empty. Claude `systemMessage` is the F20 exception |
| F19 AC1 | FAIL | — | Claude `/oops` command file is installed by init. No test |
| F19 AC2 | FAIL | — | `slash.Run` has no test |
| F19 AC3 | PASS | README agent table | Codex and Gemini: `/oops` when the prompt is in the start payload. Cursor: `!oops` |
| F19 AC4 | PASS | README “Supported agents” | |
| F20 AC1 | PASS | `TestE2ESubagentStopDoesNotSplit` | Empty stdout |
| F20 AC2 | FAIL | — | Notices are stored per reason. No test |
| F20 AC3 | FAIL | — | Claude `systemMessage` path is implemented. No test |
| G1–G10 | FAIL | — | PRD marks these v0.2. Not in this change |
| §6.1 matrix | FAIL | — | CI is ubuntu, macos, windows on the Go version in `go.mod`. Not the full OS/fs/agent matrix. No coverage gate |
| §6.2 10k hook | PASS | `TestHookVisibleLatency` | Post-turn hook p50 4 ms, p95 5 ms |
| §6.2 100k hook | PASS | `TestHookVisibleLatency` | Post-turn hook p50 5 ms, p95 6 ms. The in-process capture p95 is 3.6 s and runs after the hook returns |
| §6.2 other rows | FAIL | — | Cold start, empty-turn bytes, undo of 1k files, list, RSS, and binary size were not measured |
| §6.3 | PASS | `.github/workflows/ci.yml` | lint, vet, govulncheck, `go test -race` on three OS (includes e2e). Bench smoke is opt-in via `OOPS_PERF=1` |
| §6.4 | FAIL | `docs/schema.md` | No generated command reference. READMEs and how-it-works exist |
| §6.5 | FAIL | — | LICENSE, READMEs, CONTRIBUTING, CODE_OF_CONDUCT, SECURITY, issue templates, and the PR template exist. CHANGELOG, cosign, SBOM, SLSA, brew, npm, scoop, winget, and the three-person study do not. Those install channels are marked planned in `docs/PRD.md` and `docs/DESIGN.md` |

## Counts

| Result | Rows |
|---|---|
| PASS | 26 |
| FAIL | 35 |

F1–F20 and F14/F15/F16/F17/F18/F19/F20 rows above plus the §6 rows and the single G1–G10 row. G1–G10 counted once.
