package agents

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Event is a normalized hook payload.
type Event struct {
	Agent     string
	Kind      string // start, end, slash, session, step
	CWD       string
	SessionID string
	TurnID    string
	Prompt    string
	Raw       map[string]any
}

// Parse normalizes an agent hook payload. Unknown fields are ignored.
func Parse(agent, kind string, raw []byte) Event {
	ev := Event{Agent: agent, Kind: normalizeKind(agent, kind)}
	if len(raw) == 0 {
		return ev
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return ev
	}
	m, ok := v.(map[string]any)
	if !ok {
		return ev
	}
	ev.Raw = m
	ev.CWD = firstString(m, "cwd", "workspace", "workspace_dir", "project_dir", "root")
	ev.SessionID = firstString(m, "session_id", "sessionId", "session", "conversation_id")
	ev.TurnID = firstString(m, "turn_id", "turnId", "generation_id")
	ev.Prompt = firstString(m, "prompt", "user_prompt", "message", "text", "input")
	if ev.Prompt == "" {
		ev.Prompt = findPrompt(m)
	}
	if ev.CWD == "" {
		ev.CWD = findKey(m, "cwd")
	}
	return ev
}

func normalizeKind(agent, kind string) string {
	k := strings.ToLower(strings.ReplaceAll(kind, "_", "-"))
	switch k {
	case "turn-start", "userpromptsubmit", "beforeagent", "beforesubmitprompt":
		return "start"
	case "turn-end", "stop", "stopfailure", "interrupt", "afteragent":
		return "end"
	case "slash", "userpromptexpansion":
		return "slash"
	case "session-start", "sessionstart":
		return "session"
	case "step", "beforeshellexecution", "afterfileedit", "beforetool":
		return "step"
	case "subagent-stop", "subagentstop":
		return "subagent"
	default:
		return k
	}
}

func firstString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if s, ok := m[k].(string); ok && s != "" {
			return s
		}
	}
	return ""
}

func findPrompt(v any) string {
	switch t := v.(type) {
	case map[string]any:
		for _, k := range []string{"prompt", "user_prompt", "message", "text"} {
			if s, ok := t[k].(string); ok && strings.TrimSpace(s) != "" {
				return s
			}
		}
		for _, child := range t {
			if s := findPrompt(child); s != "" {
				return s
			}
		}
	case []any:
		for _, child := range t {
			if s := findPrompt(child); s != "" {
				return s
			}
		}
	}
	return ""
}

func findKey(v any, key string) string {
	switch t := v.(type) {
	case map[string]any:
		if s, ok := t[key].(string); ok && s != "" {
			return s
		}
		for _, child := range t {
			if s := findKey(child, key); s != "" {
				return s
			}
		}
	case []any:
		for _, child := range t {
			if s := findKey(child, key); s != "" {
				return s
			}
		}
	}
	return ""
}

// Slash parses "/oops ..." and reports whether the prompt is an oops command.
func Slash(prompt string) (args []string, ok bool) {
	p := strings.TrimSpace(prompt)
	if !strings.HasPrefix(strings.ToLower(p), "/oops") {
		return nil, false
	}
	fields := strings.Fields(p)
	if len(fields) <= 1 {
		return []string{"list"}, true
	}
	return fields[1:], true
}

// WorkspaceRoots reads Cursor's workspace_roots field. It is a list of
// workspace folders, normally one. A single string is accepted too.
func WorkspaceRoots(raw map[string]any) []string {
	if raw == nil {
		return nil
	}
	switch v := raw["workspace_roots"].(type) {
	case string:
		if s := strings.TrimSpace(v); s != "" {
			return []string{s}
		}
	case []any:
		var out []string
		for _, item := range v {
			s, ok := item.(string)
			if !ok {
				continue
			}
			s = strings.TrimSpace(s)
			if s == "" {
				continue
			}
			out = append(out, s)
		}
		return out
	}
	return nil
}

// ProjectDir is the workspace a hook should snapshot.
// Cursor user hooks run with the process working directory set to ~/.cursor
// and no cwd field. The payload carries workspace_roots, and the process has
// CURSOR_PROJECT_DIR. When several roots are listed, the one matching
// CURSOR_PROJECT_DIR wins; otherwise the first root is used.
func ProjectDir(ev Event) string {
	roots := WorkspaceRoots(ev.Raw)
	env := strings.TrimSpace(os.Getenv("CURSOR_PROJECT_DIR"))
	cwd := strings.TrimSpace(ev.CWD)
	if ev.Agent == "cursor" || len(roots) > 0 {
		if d := chooseRoot(roots, env, cwd); d != "" {
			return d
		}
		if (cwd == "" || isCursorConfigDir(cwd)) && env != "" {
			return env
		}
	}
	return cwd
}

func chooseRoot(roots []string, env, cwd string) string {
	if len(roots) == 0 {
		return ""
	}
	if env != "" {
		for _, r := range roots {
			if samePath(r, env) {
				return r
			}
		}
	}
	if cwd != "" && !isCursorConfigDir(cwd) {
		for _, r := range roots {
			if samePath(r, cwd) || pathUnder(cwd, r) {
				return r
			}
		}
		return ""
	}
	return roots[0]
}

func isCursorConfigDir(dir string) bool {
	return strings.EqualFold(filepath.Base(filepath.Clean(dir)), ".cursor")
}

func samePath(a, b string) bool {
	a = filepath.Clean(a)
	b = filepath.Clean(b)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}

func pathUnder(child, root string) bool {
	rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(child))
	if err != nil {
		return false
	}
	return rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
