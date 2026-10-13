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

## `oops doctor`

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

## `oops gc`

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

## `oops init`

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

## `oops list`

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

## `oops redo`

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

## `oops restore`

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

## `oops rewind`

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

## `oops run`

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

## `oops show`

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

## `oops snap`

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

## `oops undo`

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

