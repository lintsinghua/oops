// Package snapshot scans a workspace into content-addressed trees.
package snapshot

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"sort"

	"github.com/lintsinghua/oops/internal/store"
)

const (
	// KindFile is a regular file.
	KindFile uint8 = 1
	// KindDir is a directory.
	KindDir uint8 = 2
	// KindSymlink is a symlink, stored as its target string.
	KindSymlink uint8 = 3
)

const treeMagic = "OOPSTREE"
const treeVersion uint16 = 1

// Entry is one name inside a tree.
type Entry struct {
	Name    string
	Kind    uint8
	Mode    uint32
	Size    int64
	ID      store.ID
	Target  string
	Skipped string
	Mtime   int64
	Ino     uint64
}

// Meta is a flattened path inside a snapshot.
type Meta struct {
	Path    string
	Kind    uint8
	Mode    uint32
	Size    int64
	ID      store.ID
	Target  string
	Skipped string
	Mtime   int64
	Ino     uint64
}

// EncodeTree serializes entries in name order. The bytes are the object's raw content.
func EncodeTree(entries []Entry) ([]byte, error) {
	cp := append([]Entry(nil), entries...)
	sort.Slice(cp, func(i, j int) bool { return cp[i].Name < cp[j].Name })
	var buf bytes.Buffer
	buf.WriteString(treeMagic)
	if err := binary.Write(&buf, binary.LittleEndian, treeVersion); err != nil {
		return nil, err
	}
	if err := binary.Write(&buf, binary.LittleEndian, uint32(len(cp))); err != nil {
		return nil, err
	}
	for _, e := range cp {
		if err := writeStr(&buf, e.Name); err != nil {
			return nil, err
		}
		buf.WriteByte(e.Kind)
		if err := binary.Write(&buf, binary.LittleEndian, e.Mode); err != nil {
			return nil, err
		}
		if err := binary.Write(&buf, binary.LittleEndian, e.Size); err != nil {
			return nil, err
		}
		if _, err := buf.Write(e.ID[:]); err != nil {
			return nil, err
		}
		if err := writeStr(&buf, e.Target); err != nil {
			return nil, err
		}
		if err := writeStr(&buf, e.Skipped); err != nil {
			return nil, err
		}
		if err := binary.Write(&buf, binary.LittleEndian, e.Mtime); err != nil {
			return nil, err
		}
		if err := binary.Write(&buf, binary.LittleEndian, e.Ino); err != nil {
			return nil, err
		}
	}
	return buf.Bytes(), nil
}

// DecodeTree parses a tree object.
func DecodeTree(raw []byte) ([]Entry, error) {
	if len(raw) < len(treeMagic)+2+4 {
		return nil, errors.New("short tree")
	}
	if string(raw[:len(treeMagic)]) != treeMagic {
		return nil, errors.New("bad tree magic")
	}
	r := bytes.NewReader(raw[len(treeMagic):])
	var ver uint16
	if err := binary.Read(r, binary.LittleEndian, &ver); err != nil {
		return nil, err
	}
	if ver != treeVersion {
		return nil, fmt.Errorf("tree version %d", ver)
	}
	var n uint32
	if err := binary.Read(r, binary.LittleEndian, &n); err != nil {
		return nil, err
	}
	if n > 10_000_000 {
		return nil, errors.New("tree too large")
	}
	out := make([]Entry, 0, n)
	for i := uint32(0); i < n; i++ {
		var e Entry
		var err error
		e.Name, err = readStr(r)
		if err != nil {
			return nil, err
		}
		k, err := r.ReadByte()
		if err != nil {
			return nil, err
		}
		e.Kind = k
		if err := binary.Read(r, binary.LittleEndian, &e.Mode); err != nil {
			return nil, err
		}
		if err := binary.Read(r, binary.LittleEndian, &e.Size); err != nil {
			return nil, err
		}
		if _, err := r.Read(e.ID[:]); err != nil {
			return nil, err
		}
		e.Target, err = readStr(r)
		if err != nil {
			return nil, err
		}
		e.Skipped, err = readStr(r)
		if err != nil {
			return nil, err
		}
		if err := binary.Read(r, binary.LittleEndian, &e.Mtime); err != nil {
			return nil, err
		}
		if err := binary.Read(r, binary.LittleEndian, &e.Ino); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, nil
}

func writeStr(buf *bytes.Buffer, s string) error {
	if len(s) > 65535 {
		return fmt.Errorf("string too long")
	}
	if err := binary.Write(buf, binary.LittleEndian, uint16(len(s))); err != nil {
		return err
	}
	_, err := buf.WriteString(s)
	return err
}

func readStr(r *bytes.Reader) (string, error) {
	var n uint16
	if err := binary.Read(r, binary.LittleEndian, &n); err != nil {
		return "", err
	}
	b := make([]byte, n)
	if _, err := r.Read(b); err != nil {
		return "", err
	}
	return string(b), nil
}

// SameState reports whether two file states are interchangeable for undo.
func SameState(a, b *Meta) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	if a.Kind != b.Kind {
		return false
	}
	switch a.Kind {
	case KindDir:
		return true
	case KindSymlink:
		return a.Target == b.Target
	default:
		if a.Skipped != "" || b.Skipped != "" {
			return a.Skipped == b.Skipped && a.Size == b.Size && a.Mtime == b.Mtime
		}
		if a.ID != b.ID {
			return false
		}
		return (a.Mode & 0o111) == (b.Mode & 0o111)
	}
}
