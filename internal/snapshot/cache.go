package snapshot

import (
	"bytes"
	"encoding/binary"
	"os"

	"github.com/lintsinghua/oops/internal/platform"
	"github.com/lintsinghua/oops/internal/store"
)

const cacheMagic = "OSTAT1"
const cacheMagic2 = "OSTAT2"

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
// LastRoot is the tree written for this cache. A later unchanged scan reuses it.
type Cache struct {
	ScanTime int64
	LastRoot store.ID
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
	magic := string(b[:len(cacheMagic)])
	if magic != cacheMagic && magic != cacheMagic2 {
		return c, nil
	}
	p := len(cacheMagic)
	if p+8 > len(b) {
		return c, nil
	}
	c.ScanTime = int64(binary.LittleEndian.Uint64(b[p:]))
	p += 8
	if magic == cacheMagic2 {
		if p+32+4 > len(b) {
			return &Cache{Entries: map[string]CacheEntry{}}, nil
		}
		copy(c.LastRoot[:], b[p:p+32])
		p += 32
	}
	if p+4 > len(b) {
		return c, nil
	}
	n := binary.LittleEndian.Uint32(b[p:])
	p += 4
	c.Entries = make(map[string]CacheEntry, n)
	for i := uint32(0); i < n; i++ {
		if p+2 > len(b) {
			return c, nil
		}
		ln := int(binary.LittleEndian.Uint16(b[p:]))
		p += 2
		if ln < 0 || p+ln+8+8+8+4+32+1 > len(b) {
			return c, nil
		}
		key := string(b[p : p+ln])
		p += ln
		var e CacheEntry
		e.Size = int64(binary.LittleEndian.Uint64(b[p:]))
		p += 8
		e.Mtime = int64(binary.LittleEndian.Uint64(b[p:]))
		p += 8
		e.Ino = binary.LittleEndian.Uint64(b[p:])
		p += 8
		e.Mode = binary.LittleEndian.Uint32(b[p:])
		p += 4
		copy(e.ID[:], b[p:p+32])
		p += 32
		e.Kind = b[p]
		p++
		c.Entries[key] = e
	}
	return c, nil
}

// Touch updates the stored scan time without rewriting every cache entry.
func (c *Cache) Touch(path string) error {
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer f.Close()
	var buf [8]byte
	binary.LittleEndian.PutUint64(buf[:], uint64(c.ScanTime))
	_, err = f.WriteAt(buf[:], int64(len(cacheMagic2)))
	return err
}

// Save writes the cache atomically.
func (c *Cache) Save(path string) error {
	var buf bytes.Buffer
	buf.WriteString(cacheMagic2)
	_ = binary.Write(&buf, binary.LittleEndian, c.ScanTime)
	buf.Write(c.LastRoot[:])
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
