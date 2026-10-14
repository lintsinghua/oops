// Package store is a content-addressed object store.
// Object IDs are SHA-256 of the raw bytes. Bodies are zstd-compressed.
package store

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/klauspost/compress/zstd"

	"github.com/lintsinghua/oops/internal/platform"
)

// ID is a SHA-256 content address.
type ID [32]byte

// Zero is the empty ID.
var Zero ID

// ParseID decodes a hex object ID.
func ParseID(s string) (ID, error) {
	var id ID
	if len(s) != 64 {
		return id, fmt.Errorf("object id %q", s)
	}
	b, err := hex.DecodeString(s)
	if err != nil {
		return id, err
	}
	copy(id[:], b)
	return id, nil
}

func (id ID) String() string { return hex.EncodeToString(id[:]) }

// IsZero reports whether id is unset.
func (id ID) IsZero() bool { return id == Zero }

// Hash returns the content ID of raw.
func Hash(raw []byte) ID {
	return sha256.Sum256(raw)
}

// Store writes immutable objects under root/objects.
type Store struct {
	root string
	enc  *zstd.Encoder
	dec  *zstd.Decoder
	mu   sync.Mutex
}

// Open creates the object directory.
func Open(root string) (*Store, error) {
	dir := filepath.Join(root, "objects")
	if err := platform.SecureMkdir(dir); err != nil {
		return nil, err
	}
	enc, err := zstd.NewWriter(nil, zstd.WithEncoderLevel(zstd.SpeedFastest))
	if err != nil {
		return nil, err
	}
	dec, err := zstd.NewReader(nil)
	if err != nil {
		enc.Close()
		return nil, err
	}
	return &Store{root: dir, enc: enc, dec: dec}, nil
}

// Close releases compressor resources.
func (s *Store) Close() {
	if s.enc != nil {
		s.enc.Close()
	}
	if s.dec != nil {
		s.dec.Close()
	}
}

func (s *Store) path(id ID) string {
	h := id.String()
	return filepath.Join(s.root, h[:2], h[2:])
}

// Has reports whether id is already stored.
func (s *Store) Has(id ID) bool {
	_, err := os.Lstat(s.path(id))
	return err == nil
}

// Put stores raw if needed and returns its ID.
func (s *Store) Put(raw []byte) (ID, error) {
	id := Hash(raw)
	if s.Has(id) {
		return id, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Has(id) {
		return id, nil
	}
	compressed := s.enc.EncodeAll(raw, nil)
	if err := platform.WriteFileAtomicUnsynced(s.path(id), compressed, 0o600); err != nil {
		return ID{}, err
	}
	return id, nil
}

// Sync flushes the object directory after a batch of puts.
func (s *Store) Sync() error {
	return platform.SyncDir(s.root)
}

// Get reads and decompresses an object.
func (s *Store) Get(id ID) ([]byte, error) {
	f, err := os.Open(s.path(id))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	body, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}
	raw, err := s.dec.DecodeAll(body, nil)
	if err != nil {
		return nil, fmt.Errorf("decode object %s: %w", id, err)
	}
	if Hash(raw) != id {
		return nil, fmt.Errorf("object %s failed checksum", id)
	}
	return raw, nil
}

// Size returns compressed bytes used by objects.
func (s *Store) Size() (int64, error) {
	var n int64
	err := filepath.WalkDir(s.root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		n += info.Size()
		return nil
	})
	return n, err
}

// ErrMiss is returned when an object does not exist.
var ErrMiss = errors.New("object not found")

// EqualBytes reports whether a and b are identical.
func EqualBytes(a, b []byte) bool { return bytes.Equal(a, b) }
