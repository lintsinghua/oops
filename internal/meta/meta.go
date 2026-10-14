// Package meta stores workspace, turn, and operation metadata in SQLite.
package meta

import (
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"

	"github.com/lintsinghua/oops/internal/platform"
)

// DB is the process-local metadata database.
type DB struct {
	sql *sql.DB
}

// Open opens or creates meta.db under home.
func Open(home string) (*DB, error) {
	if err := platform.SecureMkdir(home); err != nil {
		return nil, err
	}
	path := filepath.Join(home, "meta.db")
	dsn := sqliteDSN(path)
	s, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	s.SetMaxOpenConns(1)
	db := &DB{sql: s}
	if err := db.migrate(); err != nil {
		_ = s.Close()
		return nil, err
	}
	return db, nil
}

func sqliteDSN(path string) string {
	p := filepath.ToSlash(path)
	if len(p) >= 2 && p[1] == ':' {
		p = "/" + p
	}
	return "file:" + p + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=foreign_keys(ON)"
}

// Close closes the database.
func (db *DB) Close() error { return db.sql.Close() }

func (db *DB) migrate() error {
	_, err := db.sql.Exec(`
CREATE TABLE IF NOT EXISTS schema_version (version INTEGER NOT NULL);
CREATE TABLE IF NOT EXISTS workspaces (
  id TEXT PRIMARY KEY,
  path TEXT NOT NULL,
  created_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS snapshots (
  id TEXT PRIMARY KEY,
  ws TEXT NOT NULL,
  root TEXT NOT NULL,
  created_at TEXT NOT NULL,
  partial INTEGER NOT NULL,
  skipped TEXT NOT NULL DEFAULT '[]'
);
CREATE TABLE IF NOT EXISTS turns (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  ws TEXT NOT NULL,
  seq INTEGER NOT NULL,
  agent TEXT NOT NULL DEFAULT '',
  session_id TEXT NOT NULL DEFAULT '',
  agent_turn_id TEXT NOT NULL DEFAULT '',
  kind TEXT NOT NULL,
  prompt_summary TEXT NOT NULL DEFAULT '',
  started_at TEXT NOT NULL,
  ended_at TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL,
  pre_snap TEXT NOT NULL DEFAULT '',
  post_snap TEXT NOT NULL DEFAULT '',
  stats TEXT NOT NULL DEFAULT '{}',
  UNIQUE(ws, seq)
);
CREATE TABLE IF NOT EXISTS ops (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  ws TEXT NOT NULL,
  type TEXT NOT NULL,
  target_turns TEXT NOT NULL DEFAULT '[]',
  safety_snap TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL,
  status TEXT NOT NULL,
  detail TEXT NOT NULL DEFAULT '{}',
  applied INTEGER NOT NULL DEFAULT 1
);
CREATE INDEX IF NOT EXISTS idx_turns_ws_seq ON turns(ws, seq);
CREATE INDEX IF NOT EXISTS idx_ops_ws ON ops(ws, id);
`)
	if err != nil {
		return err
	}
	var n int
	if err := db.sql.QueryRow(`SELECT COUNT(*) FROM schema_version`).Scan(&n); err != nil {
		return err
	}
	if n == 0 {
		_, err = db.sql.Exec(`INSERT INTO schema_version(version) VALUES (1)`)
	}
	return err
}

// Workspace is a protected root.
type Workspace struct {
	ID        string
	Path      string
	CreatedAt time.Time
}

// UpsertWorkspace records a workspace path.
func (db *DB) UpsertWorkspace(id, path string, now time.Time) error {
	_, err := db.sql.Exec(`INSERT INTO workspaces(id, path, created_at) VALUES(?,?,?)
		ON CONFLICT(id) DO UPDATE SET path=excluded.path`, id, path, now.UTC().Format(time.RFC3339Nano))
	return err
}

// WorkspaceByID loads a workspace.
func (db *DB) WorkspaceByID(id string) (Workspace, error) {
	var w Workspace
	var created string
	err := db.sql.QueryRow(`SELECT id, path, created_at FROM workspaces WHERE id=?`, id).Scan(&w.ID, &w.Path, &created)
	if err != nil {
		return w, err
	}
	w.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
	return w, nil
}

// Snapshot is a stored tree root.
type Snapshot struct {
	ID        string
	WS        string
	Root      string
	CreatedAt time.Time
	Partial   bool
	Skipped   []Skipped
}

// Skipped is a file that was not fully captured.
type Skipped struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
	Size   int64  `json:"size"`
}

// InsertSnapshot stores a snapshot row.
func (db *DB) InsertSnapshot(s Snapshot) error {
	b, err := json.Marshal(s.Skipped)
	if err != nil {
		return err
	}
	partial := 0
	if s.Partial {
		partial = 1
	}
	_, err = db.sql.Exec(`INSERT INTO snapshots(id, ws, root, created_at, partial, skipped) VALUES(?,?,?,?,?,?)`,
		s.ID, s.WS, s.Root, s.CreatedAt.UTC().Format(time.RFC3339Nano), partial, string(b))
	return err
}

// GetSnapshot loads a snapshot.
func (db *DB) GetSnapshot(id string) (Snapshot, error) {
	var s Snapshot
	var created, skipped string
	var partial int
	err := db.sql.QueryRow(`SELECT id, ws, root, created_at, partial, skipped FROM snapshots WHERE id=?`, id).
		Scan(&s.ID, &s.WS, &s.Root, &created, &partial, &skipped)
	if err != nil {
		return s, err
	}
	s.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
	s.Partial = partial == 1
	_ = json.Unmarshal([]byte(skipped), &s.Skipped)
	return s, nil
}

// StatusPending means Stop has ended the turn and the post snapshot is not stored yet.
const StatusPending = "pending"

// Turn is one agent or manual turn.
type Turn struct {
	ID          int64
	WS          string
	Seq         int
	Agent       string
	SessionID   string
	AgentTurnID string
	Kind        string
	Prompt      string
	StartedAt   time.Time
	EndedAt     time.Time
	Status      string
	PreSnap     string
	PostSnap    string
	Stats       Stats
}

// Stats is the stored change summary.
type Stats struct {
	Files      int   `json:"files"`
	Added      int   `json:"added"`
	Modified   int   `json:"modified"`
	Deleted    int   `json:"deleted"`
	Insertions int   `json:"insertions"`
	Deletions  int   `json:"deletions"`
	ClosedLate bool  `json:"closed_late,omitempty"`
	Overlaps   []int `json:"overlaps,omitempty"`
}

// InsertTurn allocates the next sequence number and inserts a running turn.
func (db *DB) InsertTurn(t Turn) (Turn, error) {
	tx, err := db.sql.Begin()
	if err != nil {
		return t, err
	}
	defer func() { _ = tx.Rollback() }()
	var seq int
	if err := tx.QueryRow(`SELECT COALESCE(MAX(seq),0)+1 FROM turns WHERE ws=? AND kind!='safety'`, t.WS).Scan(&seq); err != nil {
		return t, err
	}
	t.Seq = seq
	stats, _ := json.Marshal(t.Stats)
	res, err := tx.Exec(`INSERT INTO turns(ws, seq, agent, session_id, agent_turn_id, kind, prompt_summary, started_at, ended_at, status, pre_snap, post_snap, stats)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		t.WS, t.Seq, t.Agent, t.SessionID, t.AgentTurnID, t.Kind, t.Prompt,
		fmtTime(t.StartedAt), fmtTime(t.EndedAt), t.Status, t.PreSnap, t.PostSnap, string(stats))
	if err != nil {
		return t, err
	}
	t.ID, _ = res.LastInsertId()
	if err := tx.Commit(); err != nil {
		return t, err
	}
	return t, nil
}

// UpdateTurn writes status, post snapshot, and stats.
func (db *DB) UpdateTurn(t Turn) error {
	stats, err := json.Marshal(t.Stats)
	if err != nil {
		return err
	}
	_, err = db.sql.Exec(`UPDATE turns SET ended_at=?, status=?, post_snap=?, stats=?, prompt_summary=? WHERE id=?`,
		fmtTime(t.EndedAt), t.Status, t.PostSnap, string(stats), t.Prompt, t.ID)
	return err
}

// SetStatus updates a turn status.
func (db *DB) SetStatus(id int64, status string) error {
	_, err := db.sql.Exec(`UPDATE turns SET status=? WHERE id=?`, status, id)
	return err
}

// MarkTurnPending records that Stop has ended the turn. The post snapshot
// is still outstanding. The write does not walk the tree.
func (db *DB) MarkTurnPending(id int64, ended time.Time) error {
	_, err := db.sql.Exec(`UPDATE turns SET status=?, ended_at=? WHERE id=? AND status='running'`,
		StatusPending, fmtTime(ended), id)
	return err
}

// LatestSnapshot returns the newest snapshot row for a workspace.
func (db *DB) LatestSnapshot(ws string) (Snapshot, error) {
	var s Snapshot
	var created, skipped string
	var partial int
	err := db.sql.QueryRow(`SELECT id, ws, root, created_at, partial, skipped FROM snapshots WHERE ws=? ORDER BY created_at DESC LIMIT 1`, ws).
		Scan(&s.ID, &s.WS, &s.Root, &created, &partial, &skipped)
	if err != nil {
		return s, err
	}
	s.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
	s.Partial = partial == 1
	_ = json.Unmarshal([]byte(skipped), &s.Skipped)
	return s, nil
}

// GetTurn loads one turn by workspace and sequence.
func (db *DB) GetTurn(ws string, seq int) (Turn, error) {
	row := db.sql.QueryRow(`SELECT id, ws, seq, agent, session_id, agent_turn_id, kind, prompt_summary, started_at, ended_at, status, pre_snap, post_snap, stats
		FROM turns WHERE ws=? AND seq=? AND kind!='safety'`, ws, seq)
	return scanTurn(row)
}

// LatestFinished returns the highest done or undone non-safety turn.
func (db *DB) LatestFinished(ws string) (Turn, error) {
	row := db.sql.QueryRow(`SELECT id, ws, seq, agent, session_id, agent_turn_id, kind, prompt_summary, started_at, ended_at, status, pre_snap, post_snap, stats
		FROM turns WHERE ws=? AND kind!='safety' AND status!='running' ORDER BY seq DESC LIMIT 1`, ws)
	return scanTurn(row)
}

// LatestUndoable returns the newest finished turn that is not already undone.
func (db *DB) LatestUndoable(ws string) (Turn, error) {
	row := db.sql.QueryRow(`SELECT id, ws, seq, agent, session_id, agent_turn_id, kind, prompt_summary, started_at, ended_at, status, pre_snap, post_snap, stats
		FROM turns WHERE ws=? AND kind!='safety' AND status NOT IN ('running','undone') ORDER BY seq DESC LIMIT 1`, ws)
	return scanTurn(row)
}

// ListTurns returns turns newest first.
func (db *DB) ListTurns(ws string, limit int, all bool) ([]Turn, error) {
	q := `SELECT id, ws, seq, agent, session_id, agent_turn_id, kind, prompt_summary, started_at, ended_at, status, pre_snap, post_snap, stats
		FROM turns WHERE ws=?`
	if !all {
		q += ` AND kind!='safety'`
	}
	q += ` ORDER BY seq DESC`
	args := []any{ws}
	if limit > 0 {
		q += ` LIMIT ?`
		args = append(args, limit)
	}
	rows, err := db.sql.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Turn
	for rows.Next() {
		t, err := scanTurn(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// RunningTurns returns turns that have not been closed.
func (db *DB) RunningTurns(ws string) ([]Turn, error) {
	rows, err := db.sql.Query(`SELECT id, ws, seq, agent, session_id, agent_turn_id, kind, prompt_summary, started_at, ended_at, status, pre_snap, post_snap, stats
		FROM turns WHERE ws=? AND status='running' ORDER BY seq`, ws)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Turn
	for rows.Next() {
		t, err := scanTurn(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// PendingTurns returns turns Stop has ended whose post snapshot is not stored.
func (db *DB) PendingTurns(ws string) ([]Turn, error) {
	rows, err := db.sql.Query(`SELECT id, ws, seq, agent, session_id, agent_turn_id, kind, prompt_summary, started_at, ended_at, status, pre_snap, post_snap, stats
		FROM turns WHERE ws=? AND status=? ORDER BY seq`, ws, StatusPending)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Turn
	for rows.Next() {
		t, err := scanTurn(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// RunningBySession finds an open turn for a session.
func (db *DB) RunningBySession(ws, session string) (Turn, error) {
	row := db.sql.QueryRow(`SELECT id, ws, seq, agent, session_id, agent_turn_id, kind, prompt_summary, started_at, ended_at, status, pre_snap, post_snap, stats
		FROM turns WHERE ws=? AND session_id=? AND status='running' ORDER BY seq DESC LIMIT 1`, ws, session)
	return scanTurn(row)
}

type scanner interface {
	Scan(dest ...any) error
}

func scanTurn(s scanner) (Turn, error) {
	var t Turn
	var started, ended, stats string
	err := s.Scan(&t.ID, &t.WS, &t.Seq, &t.Agent, &t.SessionID, &t.AgentTurnID, &t.Kind, &t.Prompt, &started, &ended, &t.Status, &t.PreSnap, &t.PostSnap, &stats)
	if err != nil {
		return t, err
	}
	t.StartedAt, _ = time.Parse(time.RFC3339Nano, started)
	if ended != "" {
		t.EndedAt, _ = time.Parse(time.RFC3339Nano, ended)
	}
	_ = json.Unmarshal([]byte(stats), &t.Stats)
	return t, nil
}

// Op is one undo, rewind, restore, or redo.
type Op struct {
	ID         int64
	WS         string
	Type       string
	Targets    []int
	SafetySnap string
	CreatedAt  time.Time
	Status     string
	Detail     string
	Applied    bool
}

// InsertOp appends an operation.
func (db *DB) InsertOp(op Op) (Op, error) {
	b, _ := json.Marshal(op.Targets)
	applied := 0
	if op.Applied {
		applied = 1
	}
	res, err := db.sql.Exec(`INSERT INTO ops(ws, type, target_turns, safety_snap, created_at, status, detail, applied) VALUES(?,?,?,?,?,?,?,?)`,
		op.WS, op.Type, string(b), op.SafetySnap, fmtTime(op.CreatedAt), op.Status, op.Detail, applied)
	if err != nil {
		return op, err
	}
	op.ID, _ = res.LastInsertId()
	return op, nil
}

// LatestAppliedOp returns the newest applied op.
func (db *DB) LatestAppliedOp(ws string) (Op, error) {
	row := db.sql.QueryRow(`SELECT id, ws, type, target_turns, safety_snap, created_at, status, detail, applied
		FROM ops WHERE ws=? AND applied=1 ORDER BY id DESC LIMIT 1`, ws)
	return scanOp(row)
}

// LatestStackOp returns the newest undo, rewind, or restore that has not been discarded.
func (db *DB) LatestStackOp(ws string) (Op, error) {
	row := db.sql.QueryRow(`SELECT id, ws, type, target_turns, safety_snap, created_at, status, detail, applied
		FROM ops WHERE ws=? AND type IN ('undo','rewind','restore') AND status!='discarded' ORDER BY id DESC LIMIT 1`, ws)
	return scanOp(row)
}

// LatestAppliedStackOp returns the newest applied undo, rewind, or restore.
func (db *DB) LatestAppliedStackOp(ws string) (Op, error) {
	row := db.sql.QueryRow(`SELECT id, ws, type, target_turns, safety_snap, created_at, status, detail, applied
		FROM ops WHERE ws=? AND applied=1 AND type IN ('undo','rewind','restore') AND status!='discarded' ORDER BY id DESC LIMIT 1`, ws)
	return scanOp(row)
}

// DiscardUnapplied drops the redo stack so a new undo cannot revive it.
func (db *DB) DiscardUnapplied(ws string) error {
	_, err := db.sql.Exec(`UPDATE ops SET status='discarded' WHERE ws=? AND applied=0 AND type IN ('undo','rewind','restore') AND status!='discarded'`, ws)
	return err
}

// MoveToRedoStack marks op unapplied and reinserts it so it is the newest
// stack row. Undo with no turn number reapplies that newest unapplied row.
func (db *DB) MoveToRedoStack(op Op) error {
	if _, err := db.sql.Exec(`DELETE FROM ops WHERE id=?`, op.ID); err != nil {
		return err
	}
	op.ID = 0
	op.Applied = false
	_, err := db.InsertOp(op)
	return err
}

// SetOpApplied updates the applied bit.
func (db *DB) SetOpApplied(id int64, applied bool) error {
	v := 0
	if applied {
		v = 1
	}
	_, err := db.sql.Exec(`UPDATE ops SET applied=? WHERE id=?`, v, id)
	return err
}

// ListOps returns operations newest first.
func (db *DB) ListOps(ws string, limit int) ([]Op, error) {
	q := `SELECT id, ws, type, target_turns, safety_snap, created_at, status, detail, applied FROM ops WHERE ws=? ORDER BY id DESC`
	args := []any{ws}
	if limit > 0 {
		q += ` LIMIT ?`
		args = append(args, limit)
	}
	rows, err := db.sql.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Op
	for rows.Next() {
		op, err := scanOp(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, op)
	}
	return out, rows.Err()
}

func scanOp(s scanner) (Op, error) {
	var op Op
	var targets, created string
	var applied int
	err := s.Scan(&op.ID, &op.WS, &op.Type, &targets, &op.SafetySnap, &created, &op.Status, &op.Detail, &applied)
	if err != nil {
		return op, err
	}
	_ = json.Unmarshal([]byte(targets), &op.Targets)
	op.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
	op.Applied = applied == 1
	return op, nil
}

// CountTurns counts non-safety turns.
func (db *DB) CountTurns(ws string) (int, error) {
	var n int
	err := db.sql.QueryRow(`SELECT COUNT(*) FROM turns WHERE ws=? AND kind!='safety'`, ws).Scan(&n)
	return n, err
}

// OldestTurns returns the oldest non-safety turns.
func (db *DB) OldestTurns(ws string, limit int) ([]Turn, error) {
	rows, err := db.sql.Query(`SELECT id, ws, seq, agent, session_id, agent_turn_id, kind, prompt_summary, started_at, ended_at, status, pre_snap, post_snap, stats
		FROM turns WHERE ws=? AND kind!='safety' ORDER BY seq ASC LIMIT ?`, ws, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Turn
	for rows.Next() {
		t, err := scanTurn(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// DeleteTurn removes a turn row.
func (db *DB) DeleteTurn(id int64) error {
	_, err := db.sql.Exec(`DELETE FROM turns WHERE id=?`, id)
	return err
}

// SnapshotIDs returns every snapshot id still referenced.
func (db *DB) ReferencedSnapshots(ws string) (map[string]struct{}, error) {
	out := map[string]struct{}{}
	rows, err := db.sql.Query(`SELECT pre_snap, post_snap FROM turns WHERE ws=?`, ws)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var a, b string
		if err := rows.Scan(&a, &b); err != nil {
			return nil, err
		}
		if a != "" {
			out[a] = struct{}{}
		}
		if b != "" {
			out[b] = struct{}{}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	orows, err := db.sql.Query(`SELECT safety_snap FROM ops WHERE ws=?`, ws)
	if err != nil {
		return nil, err
	}
	defer orows.Close()
	for orows.Next() {
		var a string
		if err := orows.Scan(&a); err != nil {
			return nil, err
		}
		if a != "" {
			out[a] = struct{}{}
		}
	}
	return out, orows.Err()
}

// DeleteSnapshot removes a snapshot row.
func (db *DB) DeleteSnapshot(id string) error {
	_, err := db.sql.Exec(`DELETE FROM snapshots WHERE id=?`, id)
	return err
}

// AllSnapshotRoots lists snapshot roots for a workspace, or all workspaces when ws is empty.
func (db *DB) AllSnapshotRoots(ws string) ([]string, error) {
	q := `SELECT root FROM snapshots`
	var args []any
	if ws != "" {
		q += ` WHERE ws=?`
		args = append(args, ws)
	}
	rows, err := db.sql.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var r string
		if err := rows.Scan(&r); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ListSnapshotMeta returns snapshots older than a cutoff for GC of rows.
func (db *DB) UnreferencedSnapshotRows(ws string, keep map[string]struct{}) ([]string, error) {
	rows, err := db.sql.Query(`SELECT id FROM snapshots WHERE ws=?`, ws)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		if _, ok := keep[id]; !ok {
			out = append(out, id)
		}
	}
	return out, rows.Err()
}

// ErrNotFound is returned when a row is missing.
var ErrNotFound = os.ErrNotExist

// IsNotFound reports a missing row.
func IsNotFound(err error) bool {
	return errors.Is(err, sql.ErrNoRows) || errors.Is(err, os.ErrNotExist)
}

func fmtTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339Nano)
}

// DeleteWorkspaceData removes turns, ops, snapshots, and the workspace row.
func (db *DB) DeleteWorkspaceData(ws string) error {
	tx, err := db.sql.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for _, q := range []string{
		`DELETE FROM ops WHERE ws=?`,
		`DELETE FROM turns WHERE ws=?`,
		`DELETE FROM snapshots WHERE ws=?`,
		`DELETE FROM workspaces WHERE id=?`,
	} {
		if _, err := tx.Exec(q, ws); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// Workspaces lists every workspace.
func (db *DB) Workspaces() ([]Workspace, error) {
	rows, err := db.sql.Query(`SELECT id, path, created_at FROM workspaces`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Workspace
	for rows.Next() {
		var w Workspace
		var created string
		if err := rows.Scan(&w.ID, &w.Path, &created); err != nil {
			return nil, err
		}
		w.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
		out = append(out, w)
	}
	return out, rows.Err()
}
