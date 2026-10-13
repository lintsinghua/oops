package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Notice is a one-line warning deferred until the next oops command.
type Notice struct {
	Reason  string `json:"reason"`
	Text    string `json:"text"`
	Session string `json:"session,omitempty"`
	Shown   bool   `json:"shown,omitempty"`
}

func (e *Engine) noticePath(ws Workspace) string {
	return filepath.Join(e.wsDir(ws.ID), "pending-notices.json")
}

// AddNotice records a warning once per reason until it is shown.
func (e *Engine) AddNotice(ws Workspace, reason, text string) {
	if text == "" {
		return
	}
	cur := e.readNotices(ws)
	for _, n := range cur {
		if n.Reason == reason {
			return
		}
	}
	cur = append(cur, Notice{Reason: reason, Text: text})
	e.writeNotices(ws, cur)
}

// TakeNotices returns unseen notices and marks them shown.
func (e *Engine) TakeNotices(ws Workspace) []Notice {
	cur := e.readNotices(ws)
	var out []Notice
	for i := range cur {
		if !cur[i].Shown {
			out = append(out, cur[i])
			cur[i].Shown = true
		}
	}
	if len(out) > 0 {
		e.writeNotices(ws, cur)
	}
	return out
}

func (e *Engine) readNotices(ws Workspace) []Notice {
	b, err := os.ReadFile(e.noticePath(ws))
	if err != nil {
		return nil
	}
	var n []Notice
	_ = json.Unmarshal(b, &n)
	return n
}

func (e *Engine) writeNotices(ws Workspace, n []Notice) {
	b, err := json.MarshalIndent(n, "", "  ")
	if err != nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(e.noticePath(ws)), 0o700)
	_ = os.WriteFile(e.noticePath(ws), b, 0o600)
}
