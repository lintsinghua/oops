# oops — README demo (15 s): an agent wipes src/, `oops` brings it back.
#
#   vhs vhs/demo.tape            → assets/demo.gif + assets/demo.webm
#
# Requirements: vhs ≥ 0.8 (https://github.com/charmbracelet/vhs), oops on PATH,
# Claude Code logged in. The agent step is non-deterministic and costs tokens;
# for a reproducible take use vhs/demo-offline.tape instead.
#
# TODO(after v0.1 builds): run it, check timings against real output, re-record
# if any screen copy in DESIGN.md §1.4 changes.

Output assets/demo.gif
Output assets/demo.webm

Require oops
Require claude
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

# ── off-camera setup: a small throwaway project with untracked files and a .env ──
Hide
Type "export PS1='\[\e[90m\]~/code/shop\[\e[0m\] $ ' && rm -rf /tmp/oops-demo && mkdir -p /tmp/oops-demo && cd /tmp/oops-demo"
Enter
Type `git init -q && mkdir -p src scripts && for f in app cart checkout order payment; do echo "export const $f = 1" > src/$f.ts; done && echo 'API_KEY=dev' > .env && echo '.env' > .gitignore && git add -A >/dev/null && git commit -qm init && echo 'draft' > src/notes.ts`
Enter
Type "oops init -y --agent claude >/dev/null 2>&1; clear"
Enter
Show

# ── 1. the agent goes too far ──
Type "claude -p 'clean up the build scripts' --permission-mode auto"
Sleep 300ms
Enter
Sleep 6s

Type "ls src"
Sleep 300ms
Enter
Sleep 1.2s

# ── 2. one word: oops ──
Type "oops"
Sleep 300ms
Enter
Sleep 2s            # timeline: #42 selected, preview shows restore src/ …

Enter               # confirm card
Sleep 1.2s
Enter               # [ Undo ]
Sleep 1.8s          # summary printed to the normal screen

# ── 3. everything is back, untracked files and .env included ──
Type "ls src && cat .env"
Sleep 300ms
Enter
Sleep 3s
