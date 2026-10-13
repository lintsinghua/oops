package snapshot

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lintsinghua/oops/internal/store"
)

func TestCacheRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "statcache")
	empty, err := LoadCache(filepath.Join(dir, "missing"))
	if err != nil || empty == nil || len(empty.Entries) != 0 {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("nope"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadCache(path); err != nil {
		t.Fatal(err)
	}
	c := &Cache{
		ScanTime: 42,
		LastRoot: store.ID{1, 2, 3},
		Entries: map[string]CacheEntry{
			"a.txt": {Size: 3, Mtime: 4, Mode: 0o644, Kind: KindFile, ID: store.ID{9}},
		},
	}
	long := strings.Repeat("k", 70000)
	c.Entries[long] = CacheEntry{Size: 1}
	if err := c.Save(path); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadCache(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.ScanTime != 42 || loaded.Entries["a.txt"].Size != 3 || loaded.LastRoot != c.LastRoot {
		t.Fatalf("loaded %+v", loaded)
	}
	if _, ok := loaded.Entries[long]; ok {
		t.Fatal("long key was stored")
	}
	loaded.ScanTime = 99
	if err := loaded.Touch(path); err != nil {
		t.Fatal(err)
	}
	loaded, err = LoadCache(path)
	if err != nil || loaded.ScanTime != 99 {
		t.Fatalf("touch %v %+v", err, loaded)
	}
}
