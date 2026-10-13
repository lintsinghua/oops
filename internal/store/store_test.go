package store

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestStoreRoundTripAndErrors(t *testing.T) {
	root := t.TempDir()
	st, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	id, err := st.Put([]byte("hello"))
	if err != nil {
		t.Fatal(err)
	}
	if !st.Has(id) {
		t.Fatal("missing object")
	}
	got, err := st.Get(id)
	if err != nil || !EqualBytes(got, []byte("hello")) {
		t.Fatalf("get %q err %v", got, err)
	}
	again, err := st.Put([]byte("hello"))
	if err != nil || again != id {
		t.Fatalf("second put %s err %v", again, err)
	}
	if _, err := ParseID("nope"); err == nil {
		t.Fatal("short id accepted")
	}
	if _, err := ParseID("zz" + id.String()[2:]); err == nil {
		t.Fatal("bad hex accepted")
	}
	parsed, err := ParseID(id.String())
	if err != nil || parsed != id {
		t.Fatal(err)
	}
	if _, err := st.Get(ID{1}); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing get: %v", err)
	}
	other, err := st.Put([]byte("other-bytes"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(objectPath(st, id), mustRead(t, objectPath(st, other)), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Get(id); err == nil {
		t.Fatal("corrupt object was accepted")
	}
	n, err := st.Size()
	if err != nil || n <= 0 {
		t.Fatalf("size %d err %v", n, err)
	}
}

func objectPath(st *Store, id ID) string {
	h := id.String()
	return filepath.Join(st.root, h[:2], h[2:])
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
