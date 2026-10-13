# JSON output

Commands that accept `--json` print a single JSON document with `"schema_version": 1`.

## `oops list --json` and `oops snap --json`

```json
{
  "schema_version": 1,
  "workspace": "/path",
  "turns": [
    {
      "id": 1,
      "agent": "claude",
      "session_id": "s",
      "kind": "agent",
      "status": "done",
      "started_at": "2026-10-13T00:00:00Z",
      "ended_at": "2026-10-13T00:00:01Z",
      "prompt": "first line",
      "stats": {"files": 1, "added": 0, "deleted": 0, "modified": 1, "insertions": 1, "deletions": 1}
    }
  ]
}
```

## `oops show --json`

`files` is the turn's file list. Directory-only rows are omitted.

```json
{
  "schema_version": 1,
  "turn": 1,
  "agent": "claude",
  "kind": "agent",
  "status": "done",
  "started_at": "2026-10-13T00:00:00Z",
  "ended_at": "2026-10-13T00:00:01Z",
  "prompt": "first line",
  "stats": {"files": 1},
  "files": [
    {"path": "a.txt", "action": "modified", "insertions": 1, "deletions": 1, "untracked": true}
  ]
}
```

`untracked` and `ignored` are omitted when false. `ignored` is set when `git check-ignore` matches the path.

## `oops diff --json`

```json
{
  "schema_version": 1,
  "turn": 1,
  "files": [
    {"path": "a.txt", "action": "modified", "insertions": 1, "deletions": 1}
  ]
}
```

## `oops undo -n --json` and `oops redo -n --json`

```json
{
  "schema_version": 1,
  "result": "ok",
  "restored": [],
  "removed": [],
  "reverted": ["a.txt"],
  "kept": [],
  "redo": "oops redo",
  "dry_run": true
}
```

## `oops doctor --json`

```json
{
  "schema_version": 1,
  "checks": [
    {"name": "storage", "status": "ok", "detail": "1KB of 2GB · 1 turn"}
  ],
  "warnings": 0
}
```

## `oops snap -n --json`

```json
{"schema_version": 1, "dry_run": true, "action": "snap"}
```

## `oops version --check`

This command prints plain text. It is the only network call: `GET https://api.github.com/repos/lintsinghua/oops/releases/latest`.
