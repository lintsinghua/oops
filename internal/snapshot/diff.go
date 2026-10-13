package snapshot

import (
	"bytes"
	"sort"
	"unicode/utf8"

	"github.com/aymanbagabas/go-udiff"
)

// Change is one path that differs between two snapshots.
type Change struct {
	Path       string
	Action     string // added, deleted, modified — what the turn did
	UndoAction string // remove, restore, revert
	Pre        *Meta
	Post       *Meta
	Insertions int
	Deletions  int
	Binary     bool
	Untracked  bool
	Ignored    bool
}

// Diff compares pre and post flattened trees.
func Diff(pre, post map[string]Meta) []Change {
	keys := map[string]struct{}{}
	for k := range pre {
		keys[k] = struct{}{}
	}
	for k := range post {
		keys[k] = struct{}{}
	}
	var out []Change
	for k := range keys {
		a, aok := pre[k]
		b, bok := post[k]
		var ap, bp *Meta
		if aok {
			cp := a
			ap = &cp
		}
		if bok {
			cp := b
			bp = &cp
		}
		if SameState(ap, bp) {
			continue
		}
		ch := Change{Path: k, Pre: ap, Post: bp}
		switch {
		case ap == nil:
			ch.Action = "added"
			ch.UndoAction = "remove"
		case bp == nil:
			ch.Action = "deleted"
			ch.UndoAction = "restore"
		default:
			ch.Action = "modified"
			ch.UndoAction = "revert"
		}
		out = append(out, ch)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

// AnnotateLines fills insertion and deletion counts using blob bytes.
func AnnotateLines(changes []Change, read func(m *Meta) []byte) []Change {
	if read == nil {
		return changes
	}
	for i := range changes {
		if (changes[i].Pre != nil && changes[i].Pre.Kind == KindDir) || (changes[i].Post != nil && changes[i].Post.Kind == KindDir) {
			continue
		}
		oldB := read(changes[i].Pre)
		newB := read(changes[i].Post)
		if isBinary(oldB) || isBinary(newB) {
			changes[i].Binary = true
			continue
		}
		changes[i].Insertions, changes[i].Deletions = lineDelta(string(oldB), string(newB))
	}
	return changes
}

func lineDelta(old, newS string) (int, int) {
	edits := udiff.Strings(old, newS)
	uni, err := udiff.ToUnified("a", "b", old, edits, 0)
	if err != nil {
		return 0, 0
	}
	add, del := 0, 0
	for _, line := range bytes.Split([]byte(uni), []byte("\n")) {
		if len(line) == 0 {
			continue
		}
		switch line[0] {
		case '+':
			if !bytes.HasPrefix(line, []byte("+++")) {
				add++
			}
		case '-':
			if !bytes.HasPrefix(line, []byte("---")) {
				del++
			}
		}
	}
	return add, del
}

func isBinary(b []byte) bool {
	if len(b) == 0 {
		return false
	}
	n := len(b)
	if n > 8000 {
		n = 8000
	}
	if bytes.IndexByte(b[:n], 0) >= 0 {
		return true
	}
	return !utf8.Valid(b[:n])
}

// Unified builds a unified diff of two texts.
func Unified(oldName, newName, old, newS string, context int) (string, error) {
	edits := udiff.Strings(old, newS)
	return udiff.ToUnified(oldName, newName, old, edits, context)
}
