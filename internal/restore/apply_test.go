package restore

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/lintsinghua/oops/internal/snapshot"
	"github.com/lintsinghua/oops/internal/store"
)

func TestApplyWritesAndJournal(t *testing.T) {
	root := t.TempDir()
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	id, err := st.Put([]byte("body"))
	if err != nil {
		t.Fatal(err)
	}
	file := snapshot.Meta{Kind: snapshot.KindFile, ID: id, Mode: 0o644}
	dir := snapshot.Meta{Kind: snapshot.KindDir, Mode: 0o755}
	link := snapshot.Meta{Kind: snapshot.KindSymlink, Target: "body.txt"}
	if err := os.WriteFile(filepath.Join(root, "gone.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	items := []Item{
		{Path: "sub/body.txt", Op: "restore", Want: &file},
		{Path: "sub", Op: "restore", Want: &dir},
		{Path: "sub/link.txt", Op: "restore", Want: &link},
		{Path: "gone.txt", Op: "remove", Before: &file},
		{Path: "missing", Op: "restore", Want: &snapshot.Meta{Kind: snapshot.KindFile}},
	}
	if err := Apply(root, st, items); err == nil {
		t.Fatal("missing blob should fail")
	}
	items = items[:4]
	if err := Apply(root, st, items); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(root, "sub", "body.txt"))
	if err != nil || string(b) != "body" {
		t.Fatalf("body %q %v", b, err)
	}
	target, err := os.Readlink(filepath.Join(root, "sub", "link.txt"))
	if err != nil || target != "body.txt" {
		t.Fatalf("link %q %v", target, err)
	}
	if _, err := os.Stat(filepath.Join(root, "gone.txt")); !os.IsNotExist(err) {
		t.Fatal("remove")
	}
	jpath := filepath.Join(t.TempDir(), "journal.json")
	if err := WriteJournal(jpath, Journal{Status: "pending", WS: "ws"}); err != nil {
		t.Fatal(err)
	}
	j, err := ReadJournal(jpath)
	if err != nil || j.Status != "pending" {
		t.Fatal(err)
	}
	if _, err := ReadJournal(filepath.Join(t.TempDir(), "nope.json")); !os.IsNotExist(err) {
		t.Fatal(err)
	}
	busy := &BusyError{Path: "a.txt", Err: errors.New("busy")}
	if busy.Error() != "busy" || busy.Unwrap().Error() != "busy" {
		t.Fatal("busy")
	}
	nfc := "caf\u00e9.txt"
	nfd := "cafe\u0301.txt"
	if err := os.WriteFile(filepath.Join(root, nfd), []byte("B"), 0o644); err != nil {
		t.Fatal(err)
	}
	want := file
	if err := Apply(root, st, []Item{
		{Path: nfc, Op: "restore", Want: &want},
		{Path: nfd, Op: "remove", Before: &snapshot.Meta{Kind: snapshot.KindFile}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := Apply(root, st, []Item{{Path: "../escape", Op: "restore", Want: &want}}); err == nil {
		t.Fatal("escape")
	}
	nested := snapshot.Meta{Kind: snapshot.KindDir}
	if err := os.MkdirAll(filepath.Join(root, "rmdir", "child"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := Apply(root, st, []Item{{Path: "rmdir", Op: "remove", Before: &nested}}); err != nil {
		t.Fatal(err)
	}
	if err := writeOne(root, st, Item{Path: "skip", Want: nil}); err != nil {
		t.Fatal(err)
	}
	made := snapshot.Meta{Kind: snapshot.KindDir, Mode: 0o755}
	if err := writeOne(root, st, Item{Path: "made-dir", Want: &made}); err != nil {
		t.Fatal(err)
	}
	if _, err := caseRenames(root, []Item{{Path: "ok.txt"}}, []Item{{Path: "foo/../ok.txt"}}); err == nil {
		t.Fatal("case rename escape")
	}
}
