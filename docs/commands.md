# Command reference

Generated from the cobra commands in `internal/cli`. Regenerate with `OOPS_GEN_DOCS=1 go test ./internal/cli/ -run TestCommandReference`.

## `oops`

```
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

## `oops diff`

```
Show a turn's diff

Usage:
  oops diff [turn] [-- path...] [flags]

Examples:
  oops diff
  oops diff 12 --stat
  oops diff -- src/app.ts

Flags:
      --name-only   List paths only
      --no-pager    Print the diff without a pager
      --reverse     Show the undo direction
      --stat        Show a summary instead of a patch

Global Flags:
      --accessible     Line-oriented prompts instead of the full-screen timeline
      --color string   Color: auto, always, or never (default "auto")
      --debug          Technical details on stderr
  -C, --dir string     Run as if started in this directory
      --json           Machine-readable output
  -q, --quiet          Errors only
```

## `oops doctor`

```
Check that everything works

Usage:
  oops doctor [flags]

Examples:
  oops doctor
  oops doctor --json

Global Flags:
      --accessible     Line-oriented prompts instead of the full-screen timeline
      --color string   Color: auto, always, or never (default "auto")
      --debug          Technical details on stderr
  -C, --dir string     Run as if started in this directory
      --json           Machine-readable output
  -q, --quiet          Errors only
```

## `oops gc`

```
Free up snapshot storage

Usage:
  oops gc [flags]

Examples:
  oops gc -n
  oops gc

Flags:
      --all       Delete this workspace's snapshots
  -n, --dry-run   Show what would be freed
      --global    With --all, delete every workspace
  -y, --yes       Skip confirmation

Global Flags:
      --accessible     Line-oriented prompts instead of the full-screen timeline
      --color string   Color: auto, always, or never (default "auto")
      --debug          Technical details on stderr
  -C, --dir string     Run as if started in this directory
      --json           Machine-readable output
  -q, --quiet          Errors only
```

## `oops init`

```
Connect oops to the agents on this machine

Usage:
  oops init [flags]

Examples:
  oops init -y
  oops init --undo

Flags:
      --agent stringArray   Only this agent (repeatable)
      --delete-data         With --undo, also delete snapshot data
      --print               Print the config fragment without writing it
      --project             Write project config instead of user config
      --undo                Remove oops hooks
  -y, --yes                 Connect without asking

Global Flags:
      --accessible     Line-oriented prompts instead of the full-screen timeline
      --color string   Color: auto, always, or never (default "auto")
      --debug          Technical details on stderr
  -C, --dir string     Run as if started in this directory
      --json           Machine-readable output
  -q, --quiet          Errors only
```

## `oops list`

```
List recent turns

Usage:
  oops list [flags]

Aliases:
  list, ls

Examples:
  oops list
  oops list -n 5
  oops list --json

Flags:
      --agent string   Only this agent
      --all            Include safety snapshots
  -n, --n int          How many turns to show (default 10)
      --since string   Only turns newer than this duration (for example 24h)

Global Flags:
      --accessible     Line-oriented prompts instead of the full-screen timeline
      --color string   Color: auto, always, or never (default "auto")
      --debug          Technical details on stderr
  -C, --dir string     Run as if started in this directory
      --json           Machine-readable output
  -q, --quiet          Errors only
```

## `oops redo`

```
Reverse the last undo, rewind or restore

Usage:
  oops redo [flags]

Examples:
  oops redo
  oops redo -n

Flags:
  -n, --dry-run   Preview without changing files

Global Flags:
      --accessible     Line-oriented prompts instead of the full-screen timeline
      --color string   Color: auto, always, or never (default "auto")
      --debug          Technical details on stderr
  -C, --dir string     Run as if started in this directory
      --json           Machine-readable output
  -q, --quiet          Errors only
```

## `oops restore`

```
Bring files back as they were before a turn

Usage:
  oops restore <path>... --at <turn> [flags]

Examples:
  oops restore src/app.ts --at 12

Flags:
      --at int    Turn to restore from (the files as they were before it)
  -n, --dry-run   Preview without changing files
      --force     Overwrite a file that differs
  -y, --yes       Skip confirmation

Global Flags:
      --accessible     Line-oriented prompts instead of the full-screen timeline
      --color string   Color: auto, always, or never (default "auto")
      --debug          Technical details on stderr
  -C, --dir string     Run as if started in this directory
      --json           Machine-readable output
  -q, --quiet          Errors only
```

## `oops rewind`

```
Go back to before a turn

Usage:
  oops rewind <turn> [flags]

Examples:
  oops rewind 12
  oops rewind 12 -n

Flags:
  -n, --dry-run   Preview without changing files
      --force     Overwrite files you changed after these turns
  -y, --yes       Skip confirmation

Global Flags:
      --accessible     Line-oriented prompts instead of the full-screen timeline
      --color string   Color: auto, always, or never (default "auto")
      --debug          Technical details on stderr
  -C, --dir string     Run as if started in this directory
      --json           Machine-readable output
  -q, --quiet          Errors only
```

## `oops run`

```
Protect any tool that has no hooks

Usage:
  oops run -- <command> [args...] [flags]

Examples:
  oops run -- claude
  oops run --idle 3s -- npm test

Flags:
      --idle string   Silence before an automatic checkpoint (default 3s)
      --name string   Label stored on the turn

Global Flags:
      --accessible     Line-oriented prompts instead of the full-screen timeline
      --color string   Color: auto, always, or never (default "auto")
      --debug          Technical details on stderr
  -C, --dir string     Run as if started in this directory
      --json           Machine-readable output
  -q, --quiet          Errors only
```

## `oops show`

```
Show what a turn changed

Usage:
  oops show [turn] [flags]

Examples:
  oops show
  oops show 12

Global Flags:
      --accessible     Line-oriented prompts instead of the full-screen timeline
      --color string   Color: auto, always, or never (default "auto")
      --debug          Technical details on stderr
  -C, --dir string     Run as if started in this directory
      --json           Machine-readable output
  -q, --quiet          Errors only
```

## `oops snap`

```
Save a checkpoint now

Usage:
  oops snap [flags]

Examples:
  oops snap -m "before the experiment"

Flags:
  -n, --dry-run          Preview without writing a checkpoint
  -m, --message string   Note stored with the checkpoint

Global Flags:
      --accessible     Line-oriented prompts instead of the full-screen timeline
      --color string   Color: auto, always, or never (default "auto")
      --debug          Technical details on stderr
  -C, --dir string     Run as if started in this directory
      --json           Machine-readable output
  -q, --quiet          Errors only
```

## `oops undo`

```
Undo the last turn, or a specific one

Usage:
  oops undo [turn] [flags]

Examples:
  oops undo
  oops undo 12
  oops undo --file src/app.ts

Flags:
  -n, --dry-run            Preview without changing files
  -f, --file stringArray   Only undo these paths
      --force              Overwrite files you changed after the turn
      --running            Close a still-running turn, then undo it
  -y, --yes                Skip confirmation

Global Flags:
      --accessible     Line-oriented prompts instead of the full-screen timeline
      --color string   Color: auto, always, or never (default "auto")
      --debug          Technical details on stderr
  -C, --dir string     Run as if started in this directory
      --json           Machine-readable output
  -q, --quiet          Errors only
```

