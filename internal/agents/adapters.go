package agents

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Agent is one supported coding agent.
type Agent struct {
	Name      string
	Binary    []string
	Docs      string
	Config    func(home, project string, projectLevel bool) string
	Install   func(exe, path string, write bool) (Change, bool, error)
	Uninstall func(path string) (Change, bool, error)
	SlashNote string
	TrustNote string
}

// Agents is the v1 adapter list, in display order.
func Agents() []Agent {
	return []Agent{Claude(), Codex(), Gemini(), Cursor()}
}

// ByName finds an adapter.
func ByName(name string) (Agent, bool) {
	for _, a := range Agents() {
		if a.Name == name || strings.EqualFold(a.Name, name) {
			return a, true
		}
	}
	return Agent{}, false
}

// Claude returns the Claude Code adapter.
// Docs: https://code.claude.com/docs/en/hooks
func Claude() Agent {
	return Agent{
		Name:   "Claude Code",
		Binary: []string{"claude"},
		Docs:   "https://code.claude.com/docs/en/hooks",
		Config: func(home, project string, projectLevel bool) string {
			if projectLevel {
				return filepath.Join(project, ".claude", "settings.json")
			}
			return filepath.Join(home, ".claude", "settings.json")
		},
		SlashNote: "/oops ready",
		Install: func(exe, path string, write bool) (Change, bool, error) {
			doc, prev, err := ReadJSON(path)
			if err != nil {
				return Change{}, false, err
			}
			groups := map[string][]map[string]any{
				"SessionStart":        {commandAsync(exe, []string{"hook", "claude", "session-start"})},
				"UserPromptSubmit":    {commandTimeout(exe, []string{"hook", "claude", "turn-start"}, 15)},
				"UserPromptExpansion": {commandMatch(exe, []string{"hook", "claude", "slash"}, "oops")},
				"Stop":                {commandAsync(exe, []string{"hook", "claude", "turn-end"})},
				"StopFailure":         {commandAsync(exe, []string{"hook", "claude", "turn-end"})},
				"SubagentStop":        {commandAsync(exe, []string{"hook", "claude", "subagent-stop"})},
			}
			if !EnsureHooks(doc, groups) && Connected(doc) {
				return Change{Path: path, Before: prev, After: prev}, false, nil
			}
			return finishWrite(path, doc, prev, write)
		},
		Uninstall: uninstallFile,
	}
}

// Codex returns the Codex adapter.
// Docs: https://developers.openai.com/codex/hooks
func Codex() Agent {
	return Agent{
		Name:      "Codex",
		Binary:    []string{"codex"},
		Docs:      "https://developers.openai.com/codex/hooks",
		TrustNote: "open Codex and run /hooks once to trust oops",
		Config: func(home, project string, projectLevel bool) string {
			if projectLevel {
				return filepath.Join(project, ".codex", "hooks.json")
			}
			return filepath.Join(home, ".codex", "hooks.json")
		},
		Install: func(exe, path string, write bool) (Change, bool, error) {
			doc, prev, err := ReadJSON(path)
			if err != nil {
				return Change{}, false, err
			}
			if doc["description"] == nil {
				doc["description"] = "oops — undo for coding agents (managed by `oops init`)"
			}
			cmd := exe + " hook codex"
			groups := map[string][]map[string]any{
				"UserPromptSubmit": {shellHook(cmd+" turn-start", 15, "")},
				"Stop":             {shellHook(cmd+" turn-end", 15, "")},
				"Interrupt":        {shellHook(cmd+" turn-end", 3, "")},
			}
			if !EnsureHooks(doc, groups) && Connected(doc) {
				return Change{Path: path, Before: prev, After: prev}, false, nil
			}
			return finishWrite(path, doc, prev, write)
		},
		Uninstall: uninstallFile,
	}
}

// Gemini returns the Gemini CLI adapter.
// Docs: https://geminicli.com/docs/hooks/reference/
func Gemini() Agent {
	return Agent{
		Name:   "Gemini CLI",
		Binary: []string{"gemini"},
		Docs:   "https://geminicli.com/docs/hooks/reference/",
		Config: func(home, project string, projectLevel bool) string {
			if projectLevel {
				return filepath.Join(project, ".gemini", "settings.json")
			}
			return filepath.Join(home, ".gemini", "settings.json")
		},
		Install: func(exe, path string, write bool) (Change, bool, error) {
			doc, prev, err := ReadJSON(path)
			if err != nil {
				return Change{}, false, err
			}
			cmd := exe + " hook gemini"
			groups := map[string][]map[string]any{
				"BeforeAgent": {namedHook("oops-turn-start", cmd+" turn-start", 15000)},
				"AfterAgent":  {namedHook("oops-turn-end", cmd+" turn-end", 15000)},
			}
			if !EnsureHooks(doc, groups) && Connected(doc) {
				return Change{Path: path, Before: prev, After: prev}, false, nil
			}
			return finishWrite(path, doc, prev, write)
		},
		Uninstall: uninstallFile,
	}
}

// Cursor returns the Cursor CLI adapter.
// Docs: https://cursor.com/docs/hooks
func Cursor() Agent {
	return Agent{
		Name:      "Cursor CLI",
		Binary:    []string{"cursor-agent", "agent"},
		Docs:      "https://cursor.com/docs/hooks",
		SlashNote: "shell escape: !oops undo",
		Config: func(home, project string, projectLevel bool) string {
			if projectLevel {
				return filepath.Join(project, ".cursor", "hooks.json")
			}
			return filepath.Join(home, ".cursor", "hooks.json")
		},
		Install: func(exe, path string, write bool) (Change, bool, error) {
			doc, prev, err := ReadJSON(path)
			if err != nil {
				return Change{}, false, err
			}
			if doc["version"] == nil {
				doc["version"] = 1
			}
			cmd := exe + " hook cursor"
			groups := map[string][]map[string]any{
				"beforeSubmitPrompt":   {FlatCommand(cmd + " turn-start")},
				"stop":                 {FlatCommand(cmd + " turn-end")},
				"beforeShellExecution": {FlatCommand(cmd + " step")},
				"afterFileEdit":        {FlatCommand(cmd + " step")},
			}
			if !ensureCursor(doc, groups) && Connected(doc) {
				return Change{Path: path, Before: prev, After: prev}, false, nil
			}
			return finishWrite(path, doc, prev, write)
		},
		Uninstall: uninstallFile,
	}
}

func ensureCursor(doc map[string]any, groups map[string][]map[string]any) bool {
	hooks, _ := doc["hooks"].(map[string]any)
	if hooks == nil {
		hooks = map[string]any{}
		doc["hooks"] = hooks
	}
	changed := false
	for event, want := range groups {
		if eventConnected(hooks[event]) {
			continue
		}
		list, _ := hooks[event].([]any)
		for _, g := range want {
			list = append(list, g)
		}
		hooks[event] = list
		changed = true
	}
	return changed
}

func commandTimeout(exe string, args []string, timeout int) map[string]any {
	return CommandGroup(exe, args, timeout, false, "", "")
}

func commandAsync(exe string, args []string) map[string]any {
	return CommandGroup(exe, args, 0, true, "", "")
}

func commandMatch(exe string, args []string, matcher string) map[string]any {
	return CommandGroup(exe, args, 0, false, matcher, "")
}

func shellHook(command string, timeout int, name string) map[string]any {
	return CommandGroup(command, nil, timeout, false, "", name)
}

func namedHook(name, command string, timeout int) map[string]any {
	return CommandGroup(command, nil, timeout, false, "", name)
}

func uninstallFile(path string) (Change, bool, error) {
	doc, prev, err := ReadJSON(path)
	if err != nil {
		return Change{}, false, err
	}
	if len(prev) == 0 {
		return Change{Path: path}, false, nil
	}
	next, changed := RemoveOops(doc)
	obj, _ := next.(map[string]any)
	if obj == nil {
		obj = map[string]any{}
	}
	if !changed {
		return Change{Path: path, Before: prev, After: prev}, false, nil
	}
	ch, err := WriteJSON(path, obj, prev)
	return ch, true, err
}

func finishWrite(path string, doc map[string]any, prev []byte, write bool) (Change, bool, error) {
	if !write {
		raw, err := json.MarshalIndent(doc, "", "  ")
		if err != nil {
			return Change{}, false, err
		}
		raw = append(raw, '\n')
		return Change{Path: path, Before: prev, After: raw}, !bytesEqual(prev, raw), nil
	}
	ch, err := WriteJSON(path, doc, prev)
	if err != nil {
		return ch, false, err
	}
	return ch, !bytesEqual(ch.Before, ch.After), nil
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

var verRe = regexp.MustCompile(`\d+\.\d+(?:\.\d+)?`)

// Detect finds binaries and whether hooks are already installed.
func Detect(a Agent, home, project string, projectLevel bool, pathExtra []string) Installation {
	inst := Installation{Name: a.Name, ConfigPath: a.Config(home, project, projectLevel)}
	bin, ok := look(a.Binary, pathExtra)
	inst.Found = ok
	if ok {
		inst.Version = versionOf(bin)
		if inst.Version == "" {
			inst.Version = "unknown"
		}
	}
	doc, _, err := ReadJSON(inst.ConfigPath)
	if err != nil {
		inst.Invalid = true
		inst.InvalidMsg = err.Error()
	} else {
		inst.Connected = Connected(doc)
	}
	return inst
}

func look(names, extra []string) (string, bool) {
	path := os.Getenv("PATH")
	if len(extra) > 0 {
		path = strings.Join(extra, string(os.PathListSeparator)) + string(os.PathListSeparator) + path
	}
	for _, name := range names {
		for _, dir := range filepath.SplitList(path) {
			candidate := filepath.Join(dir, name)
			if st, err := os.Stat(candidate); err == nil && !st.IsDir() {
				return candidate, true
			}
			if os.PathSeparator == '\\' {
				for _, ext := range []string{".exe", ".cmd", ".bat"} {
					p := candidate + ext
					if st, err := os.Stat(p); err == nil && !st.IsDir() {
						return p, true
					}
				}
			}
		}
	}
	return "", false
}

func versionOf(bin string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 800*time.Millisecond)
	defer cancel()
	out, err := exec.CommandContext(ctx, bin, "--version").CombinedOutput()
	if err != nil {
		ctx2, cancel2 := context.WithTimeout(context.Background(), 800*time.Millisecond)
		defer cancel2()
		out, err = exec.CommandContext(ctx2, bin, "version").CombinedOutput()
		if err != nil && len(out) == 0 {
			return ""
		}
	}
	line := strings.TrimSpace(string(out))
	if i := strings.IndexAny(line, "\r\n"); i >= 0 {
		line = line[:i]
	}
	if m := verRe.FindString(line); m != "" {
		return m
	}
	return strings.TrimSpace(line)
}

// ClaudeCommandPath is the slash-command file Claude Code discovers.
func ClaudeCommandPath(home, project string, projectLevel bool) string {
	if projectLevel {
		return filepath.Join(project, ".claude", "commands", "oops.md")
	}
	return filepath.Join(home, ".claude", "commands", "oops.md")
}

// ClaudeCommandBody is the local /oops command description.
const ClaudeCommandBody = `---
description: Undo the last agent turn with oops
---

<!-- oops -->
Show recent oops turns, or undo one. The oops hook runs this locally; it is not sent to the model.

Usage: /oops [undo [N] | redo | diff [N]]
`
