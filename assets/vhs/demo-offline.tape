# oops — reproducible demo without a real agent (no network, no tokens).
# The "agent" is a shell command wrapped by `oops run`, which records it as a turn.
#
#   vhs assets/vhs/demo-offline.tape    → assets/demo.gif
#
# Recorded flow: `oops run` wraps the delete, then `oops undo` restores files.
# This tape does not open the timeline TUI.

Output assets/demo.gif

Require oops
Require git

Set Shell "bash"
Set FontFamily "Geist Mono"
Set FontSize 16
Set LineHeight 1.35
Set Width 1100
Set Height 600
Set Padding 28
Set TypingSpeed 45ms
Set CursorBlink false
Set Theme { "name": "oops", "background": "#0A0A0A", "foreground": "#EDEDED", "cursor": "#F5A524", "selection": "#1F2428", "black": "#0A0A0A", "red": "#F85149", "green": "#3FB950", "yellow": "#D29922", "blue": "#58A6FF", "magenta": "#BC8CFF", "cyan": "#39C5CF", "white": "#C9D1D9", "brightBlack": "#8B949E", "brightRed": "#FF7B72", "brightGreen": "#56D364", "brightYellow": "#F5A524", "brightBlue": "#79C0FF", "brightMagenta": "#D2A8FF", "brightCyan": "#56D4DD", "brightWhite": "#FFFFFF" }

Hide
Type "export PS1='\[\e[90m\]~/code/shop\[\e[0m\] $ ' && rm -rf /tmp/oops-demo && mkdir -p /tmp/oops-demo && cd /tmp/oops-demo"
Enter
Type `git init -q && mkdir -p src && for f in app cart checkout order payment; do echo "export const $f = 1" > src/$f.ts; done && echo 'API_KEY=dev' > .env && echo '.env' > .gitignore && git add -A >/dev/null && git commit -qm init && echo 'draft' > src/notes.ts && clear`
Enter
Show

Type "ls src"
Enter
Sleep 1s

# the "agent": deletes tracked, untracked and ignored files in one go
Type "oops run -- sh -c 'rm -rf src && git clean -fdx -q'"
Sleep 300ms
Enter
Sleep 2s

Type "ls src .env"
Enter
Sleep 1.5s

Type "oops undo"
Sleep 300ms
Enter
Sleep 2.5s

Type "ls src && cat .env"
Enter
Sleep 3s
