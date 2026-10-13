// Package agents detects coding agents and edits their hook configuration.
package agents

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Hook is one command oops installs.
type Hook struct {
	Event   string
	Command string
	Args    []string
	Timeout int
	Async   bool
	Matcher string
	Name    string
}

// Installation is a detected agent.
type Installation struct {
	Name       string
	Found      bool
	Version    string
	ConfigPath string
	Connected  bool
	Invalid    bool
	InvalidMsg string
}

// Change is a config edit.
type Change struct {
	Path    string
	Before  []byte
	After   []byte
	Backup  string
	Created bool
}

// Marker is the substring that identifies an oops hook command.
const Marker = "oops hook"

// ReadJSON reads a JSON file into an ordered-enough generic value.
// Missing files yield an empty object.
func ReadJSON(path string) (map[string]any, []byte, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]any{}, nil, nil
		}
		return nil, nil, err
	}
	if len(bytes.TrimSpace(b)) == 0 {
		return map[string]any{}, b, nil
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, b, &JSONError{Path: path, Err: err}
	}
	obj, ok := v.(map[string]any)
	if !ok {
		return nil, b, &JSONError{Path: path, Err: errors.New("expected a JSON object")}
	}
	return obj, b, nil
}

// JSONError is a config file that is not valid JSON.
type JSONError struct {
	Path string
	Err  error
}

func (e *JSONError) Error() string { return e.Err.Error() }

// WriteJSON writes obj atomically and returns the new bytes.
func WriteJSON(path string, obj map[string]any, previous []byte) (Change, error) {
	raw, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		return Change{}, err
	}
	raw = append(raw, '\n')
	ch := Change{Path: path, Before: previous, After: raw, Created: len(previous) == 0}
	if bytes.Equal(previous, raw) {
		return ch, nil
	}
	if len(previous) > 0 {
		backup := fmt.Sprintf("%s.oops-backup-%d", path, os.Getpid())
		if err := os.WriteFile(backup, previous, 0o600); err != nil {
			return Change{}, err
		}
		ch.Backup = backup
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return Change{}, err
	}
	tmp := path + ".oops-tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return Change{}, err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return Change{}, err
	}
	return ch, nil
}

// Connected reports whether the document already contains an oops hook.
func Connected(v any) bool {
	if strings.Contains(flat(v), Marker) {
		return true
	}
	return walkCommands(v)
}

func walkCommands(v any) bool {
	switch t := v.(type) {
	case map[string]any:
		if _, ok := t["command"]; ok && isOopsCommand(t) {
			return true
		}
		for _, child := range t {
			if walkCommands(child) {
				return true
			}
		}
	case []any:
		for _, child := range t {
			if walkCommands(child) {
				return true
			}
		}
	}
	return false
}

func flat(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// EnsureHooks adds hook groups for each event when they are missing.
// The shape is {hooks: {event: [{hooks: [command...]}]}} which matches
// Claude, Codex, and Gemini. Cursor uses a flatter list; see AddCursorHooks.
func EnsureHooks(doc map[string]any, groups map[string][]map[string]any) bool {
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

func eventConnected(v any) bool {
	return Connected(v)
}

// RemoveOops deletes hook entries that oops installed. It returns true when
// something was removed.
func RemoveOops(v any) (any, bool) {
	switch t := v.(type) {
	case map[string]any:
		if isOopsCommand(t) {
			return nil, true
		}
		changed := false
		for k, child := range t {
			next, ch := RemoveOops(child)
			if ch {
				changed = true
			}
			if next == nil {
				delete(t, k)
				changed = true
				continue
			}
			t[k] = next
		}
		return t, changed
	case []any:
		var out []any
		changed := false
		for _, child := range t {
			next, ch := RemoveOops(child)
			if ch {
				changed = true
			}
			if dropEmpty(next) {
				changed = true
				continue
			}
			out = append(out, next)
		}
		return out, changed
	default:
		return v, false
	}
}

func dropEmpty(v any) bool {
	switch t := v.(type) {
	case nil:
		return true
	case map[string]any:
		if len(t) == 0 {
			return true
		}
		if h, ok := t["hooks"]; ok {
			if list, ok := h.([]any); ok && len(list) == 0 {
				return true
			}
		}
	case []any:
		return len(t) == 0
	}
	return false
}

func isOopsCommand(m map[string]any) bool {
	line := commandLine(m)
	if line == "" {
		return false
	}
	if strings.Contains(line, Marker) {
		return true
	}
	// The executable is not always named oops (tests, wrappers). The argv
	// shape `hook <agent> <event>` is the marker the adapters install.
	for _, agent := range []string{"claude", "codex", "gemini", "cursor"} {
		if strings.Contains(line, "hook "+agent) {
			return true
		}
	}
	return false
}

func commandLine(m map[string]any) string {
	cmd, _ := m["command"].(string)
	parts := []string{}
	if cmd != "" {
		parts = append(parts, cmd)
	}
	if args, ok := m["args"].([]any); ok {
		for _, a := range args {
			if s, ok := a.(string); ok {
				parts = append(parts, s)
			}
		}
	}
	return strings.Join(parts, " ")
}

// Command builds a nested hook group used by Claude-style configs.
func CommandGroup(command string, args []string, timeout int, async bool, matcher, name string) map[string]any {
	hook := map[string]any{"type": "command", "command": command}
	if len(args) > 0 {
		arr := make([]any, len(args))
		for i, a := range args {
			arr[i] = a
		}
		hook["args"] = arr
	}
	if timeout > 0 {
		hook["timeout"] = timeout
	}
	if async {
		hook["async"] = true
	}
	if name != "" {
		hook["name"] = name
	}
	group := map[string]any{"hooks": []any{hook}}
	if matcher != "" {
		group["matcher"] = matcher
	}
	return group
}

// FlatCommand is a Cursor-style hook entry.
func FlatCommand(command string) map[string]any {
	return map[string]any{"command": command}
}
