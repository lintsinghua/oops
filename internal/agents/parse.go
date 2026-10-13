package agents

import (
	"encoding/json"
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
	ev.SessionID = firstString(m, "session_id", "sessionId", "session")
	ev.TurnID = firstString(m, "turn_id", "turnId")
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
