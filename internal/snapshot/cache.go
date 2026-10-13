package snapshot

import (
	"bytes"
	"encoding/binary"
	"os"

	"github.com/lintsinghua/oops/internal/platform"
	"github.com/lintsinghua/oops/internal/store"
)

const cacheMagic = "OSTAT1"

// CacheEntry is one stat-cache record.
type CacheEntry struct {
	Size  int64
	Mtime int64
	Ino   uint64
	Mode  uint32
	ID    store.ID
	Kind  uint8
}

// Cache maps a comparison key to the last stat.
type Cache struct {
	ScanTime int64
	Entries  map[string]CacheEntry
}

// LoadCache reads a cache file. A missing file is an empty cache.
func LoadCache(path string) (*Cache, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Cache{Entries: map[string]CacheEntry{}}, nil
		}
		return nil, err
	}
	c := &Cache{Entries: map[string]CacheEntry{}}
	if len(b) < len(cacheMagic)+8+4 {
		return c, nil
	}
	if string(b[:len(cacheMagic)]) != cacheMagic {
		return c, nil
	}
	r := bytes.NewReader(b[len(cacheMagic):])
	if err := binary.Read(r, binary.LittleEndian, &c.ScanTime); err != nil {
		return c, nil
	}
	var n uint32
	if err := binary.Read(r, binary.LittleEndian, &n); err != nil {
		return c, nil
	}
	for i := uint32(0); i < n; i++ {
		var ln uint16
		if err := binary.Read(r, binary.LittleEndian, &ln); err != nil {
			return c, nil
		}
		key := make([]byte, ln)
		if _, err := r.Read(key); err != nil {
			return c, nil
		}
		var e CacheEntry
		if err := binary.Read(r, binary.LittleEndian, &e.Size); err != nil {
			return c, nil
		}
		if err := binary.Read(r, binary.LittleEndian, &e.Mtime); err != nil {
			return c, nil
		}
		if err := binary.Read(r, binary.LittleEndian, &e.Ino); err != nil {
			return c, nil
		}
		if err := binary.Read(r, binary.LittleEndian, &e.Mode); err != nil {
			return c, nil
		}
		if _, err := r.Read(e.ID[:]); err != nil {
			return c, nil
		}
		k, err := r.ReadByte()
		if err != nil {
			return c, nil
		}
		e.Kind = k
		c.Entries[string(key)] = e
	}
	return c, nil
}

// Save writes the cache atomically.
func (c *Cache) Save(path string) error {
	var buf bytes.Buffer
	buf.WriteString(cacheMagic)
	_ = binary.Write(&buf, binary.LittleEndian, c.ScanTime)
	_ = binary.Write(&buf, binary.LittleEndian, uint32(len(c.Entries)))
	for k, e := range c.Entries {
		if len(k) > 65535 {
			continue
		}
		_ = binary.Write(&buf, binary.LittleEndian, uint16(len(k)))
		buf.WriteString(k)
		_ = binary.Write(&buf, binary.LittleEndian, e.Size)
		_ = binary.Write(&buf, binary.LittleEndian, e.Mtime)
		_ = binary.Write(&buf, binary.LittleEndian, e.Ino)
		_ = binary.Write(&buf, binary.LittleEndian, e.Mode)
		buf.Write(e.ID[:])
		buf.WriteByte(e.Kind)
	}
	return platform.WriteFileAtomic(path, buf.Bytes(), 0o600)
}
