package engine

import (
	"context"
	"regexp"
	"strings"
	"time"

	"github.com/lintsinghua/oops/internal/meta"
	"github.com/lintsinghua/oops/internal/snapshot"
)

// TurnInput opens or closes a turn.
type TurnInput struct {
	Workspace   Workspace
	Agent       string
	SessionID   string
	AgentTurnID string
	Kind        string
	Prompt      string
	Timeout     time.Duration
}

// StartTurn captures a pre snapshot and records a running turn.
// An unfinished turn in the same session is closed first.
func (e *Engine) StartTurn(ctx context.Context, in TurnInput) (meta.Turn, error) {
	if err := e.WaitPending(ctx, in.Workspace); err != nil {
		return meta.Turn{}, err
	}
	// The previous turn is over. Seal its post from the tree as it is now,
	// before this turn's pre snapshot and before the agent edits again.
	if err := e.SealRunning(ctx, in.Workspace); err != nil {
		return meta.Turn{}, err
	}
	if in.Kind == "" {
		in.Kind = "agent"
	}
	if err := e.LazyCloseSession(ctx, in.Workspace, in.SessionID); err != nil {
		return meta.Turn{}, err
	}
	others, err := e.DB.RunningTurns(in.Workspace.ID)
	if err != nil {
		return meta.Turn{}, err
	}
	snap, _, err := e.Capture(ctx, in.Workspace, SnapOptions{Timeout: in.Timeout})
	if err != nil {
		return meta.Turn{}, err
	}
	if snap.Partial {
		e.AddNotice(in.Workspace, "partial", "snapshot took too long and was partial. Run oops doctor")
	}
	var overlaps []int
	for _, o := range others {
		overlaps = append(overlaps, o.Seq)
	}
	t := meta.Turn{
		WS: in.Workspace.ID, Agent: in.Agent, SessionID: in.SessionID, AgentTurnID: in.AgentTurnID,
		Kind: in.Kind, Prompt: SummarizePrompt(in.Prompt, e.Config.Privacy.RecordPrompts, e.Config.Privacy.PromptMaxChars),
		StartedAt: e.Now, Status: "running", PreSnap: snap.ID,
		Stats: meta.Stats{Overlaps: overlaps},
	}
	inserted, err := e.DB.InsertTurn(t)
	if err != nil {
		return inserted, err
	}
	// A new turn makes an earlier redo stale. Drop it so plain undo targets this turn.
	if err := e.DB.DiscardUnapplied(in.Workspace.ID); err != nil {
		return inserted, err
	}
	return inserted, nil
}

// CloseTurn captures the post snapshot for the running session turn.
func (e *Engine) CloseTurn(ctx context.Context, in TurnInput, late bool) (meta.Turn, error) {
	if err := e.WaitPending(ctx, in.Workspace); err != nil {
		return meta.Turn{}, err
	}
	t, err := e.DB.RunningBySession(in.Workspace.ID, in.SessionID)
	if err != nil {
		if meta.IsNotFound(err) && in.SessionID == "" {
			running, rerr := e.DB.RunningTurns(in.Workspace.ID)
			if rerr != nil {
				return meta.Turn{}, rerr
			}
			if len(running) == 1 {
				t = running[0]
			} else {
				return meta.Turn{}, err
			}
		} else {
			return meta.Turn{}, err
		}
	}
	return e.finish(ctx, in.Workspace, t, late)
}

// LazyClose finishes every running turn using the current tree.
func (e *Engine) LazyClose(ctx context.Context, ws Workspace) error {
	running, err := e.DB.RunningTurns(ws.ID)
	if err != nil || len(running) == 0 {
		return err
	}
	for _, t := range running {
		if _, err := e.finish(ctx, ws, t, true); err != nil {
			return err
		}
	}
	return nil
}

// LazyCloseSession closes a single session's running turn.
func (e *Engine) LazyCloseSession(ctx context.Context, ws Workspace, session string) error {
	if session == "" {
		return nil
	}
	t, err := e.DB.RunningBySession(ws.ID, session)
	if err != nil {
		if meta.IsNotFound(err) {
			return nil
		}
		return err
	}
	_, err = e.finish(ctx, ws, t, true)
	return err
}

func (e *Engine) finish(ctx context.Context, ws Workspace, t meta.Turn, late bool) (meta.Turn, error) {
	snap, res, err := e.Capture(ctx, ws, SnapOptions{})
	if err != nil {
		return t, err
	}
	pre, _, err := e.Files(t.PreSnap)
	if err != nil {
		return t, err
	}
	cur, gerr := e.DB.GetTurn(ws.ID, t.Seq)
	// Stop marks the turn pending before this snapshot exists. A later finish
	// stores the post once. A turn that already has one is left as stored.
	if gerr == nil && cur.Status != "running" && cur.PostSnap != "" {
		return cur, nil
	}
	changes := snapshot.Diff(pre, res.Files)
	changes = snapshot.AnnotateLines(changes, e.ReadBlob)
	stats := statsFrom(changes)
	stats.Overlaps = t.Stats.Overlaps
	stats.ClosedLate = late
	if snap.Partial {
		t.Status = "partial"
	} else {
		t.Status = "done"
	}
	// Stop already recorded a wall-clock EndedAt. Keep it. A logical clock
	// (e.Now) is not comparable with filesystem mtimes.
	if t.EndedAt.IsZero() && gerr == nil {
		t.EndedAt = cur.EndedAt
	}
	if t.EndedAt.IsZero() {
		t.EndedAt = e.Now
	}
	t.PostSnap = snap.ID
	t.Stats = stats
	if err := e.DB.UpdateTurn(t); err != nil {
		return t, err
	}
	if len(snap.Skipped) > 0 {
		e.AddNotice(ws, "large", noticeLarge(snap.Skipped))
	}
	if snap.Partial {
		e.AddNotice(ws, "partial", "snapshot took too long and was partial. Run oops doctor")
	}
	return t, nil
}

func statsFrom(changes []snapshot.Change) meta.Stats {
	var s meta.Stats
	for _, c := range snapshot.FileChanges(changes) {
		s.Files++
		switch c.Action {
		case "added":
			s.Added++
		case "deleted":
			s.Deleted++
		default:
			s.Modified++
		}
		s.Insertions += c.Insertions
		s.Deletions += c.Deletions
	}
	return s
}

func noticeLarge(skipped []meta.Skipped) string {
	if len(skipped) == 0 {
		return ""
	}
	return "skipped " + itoa(len(skipped)) + " large file (" + skipped[0].Path + ") — not protected. oops doctor"
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [16]byte
	i := len(b)
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

var (
	reAssign = regexp.MustCompile(`(?i)\b(api[_-]?key|secret|password|token|authorization)\b\s*[=:]\s*\S+`)
	reToken  = regexp.MustCompile(`\b(?:sk|ghp|gho|github_pat|xox[baprs]|AKIA)[A-Za-z0-9_\-]{8,}\b`)
)

// SummarizePrompt keeps the first line, redacts secrets, and truncates.
func SummarizePrompt(prompt string, record bool, max int) string {
	if !record {
		return ""
	}
	line := prompt
	if i := strings.IndexAny(line, "\r\n"); i >= 0 {
		line = line[:i]
	}
	line = strings.TrimSpace(line)
	line = reAssign.ReplaceAllString(line, "$1=[redacted]")
	line = reToken.ReplaceAllString(line, "[redacted]")
	if max <= 0 {
		max = 200
	}
	if len(line) > max {
		line = line[:max]
	}
	return line
}
