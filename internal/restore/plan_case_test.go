//go:build windows || darwin

package restore

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lintsinghua/oops/internal/snapshot"
	"github.com/lintsinghua/oops/internal/store"
)

func TestApplyCaseRenameKeepsRestoredSpelling(t *testing.T) {
	root := t.TempDir()
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(st.Close)
	id, err := st.Put([]byte("A"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "name.txt"), []byte("B"), 0o644); err != nil {
		t.Fatal(err)
	}
	want := snapshot.Meta{Kind: snapshot.KindFile, ID: id, Mode: 0o644}
	items := []Item{
		{Path: "Name.txt", Op: "restore", Want: &want, After: &want},
		{Path: "name.txt", Op: "remove", Before: &snapshot.Meta{Kind: snapshot.KindFile}},
	}
	if err := Apply(root, st, items); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	var name string
	for _, ent := range entries {
		if strings.EqualFold(ent.Name(), "Name.txt") {
			name = ent.Name()
		}
	}
	if name != "Name.txt" {
		t.Fatalf("spelling %q", name)
	}
	b, err := os.ReadFile(filepath.Join(root, "Name.txt"))
	if err != nil || string(b) != "A" {
		t.Fatalf("bytes %q err %v", b, err)
	}
}
