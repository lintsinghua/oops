package snapshot

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lintsinghua/oops/internal/store"
)

func TestRulesAndScanEdges(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	ignore := "*.log\ndist/\nbuild/\n!keep.log\nfoo/**/bar\nsub/?\n"
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(ignore), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".oopsignore"), []byte("secret\n!secret/keep.txt\n*.tmp\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{"node_modules", "dist", "build", "secret", "foo/a", "sub", "nested"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "nested", ".gitignore"), []byte("hidden.txt\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"keep.txt": "k", "a.log": "l", "keep.log": "kl", ".env": "secret",
		"notes.pem": "p", "skipme": "s", "secret/no.txt": "n", "secret/keep.txt": "y",
		"foo/a/bar": "b", "sub/z": "z", "nested/hidden.txt": "h", "nested/ok.txt": "o",
		".oops-tmp-x": "t", "café.txt": "c",
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink("keep.txt", filepath.Join(root, "link.txt")); err != nil {
		t.Fatal(err)
	}
	rules, err := LoadRules(root, []string{"*.pem"}, []string{"skipme"}, true)
	if err != nil {
		t.Fatal(err)
	}
	if rules.Include("node_modules", true) || !rules.PruneDir("node_modules") {
		t.Fatal("node_modules")
	}
	if rules.Include("a.log", false) {
		t.Fatal("log included")
	}
	if !rules.Include(".env", false) {
		t.Fatal(".env should be protected")
	}
	if rules.Include("skipme", false) || rules.Include(".oops-tmp-x", false) {
		t.Fatal("extra or tmp included")
	}
	if !rules.Include("keep.txt", false) || rules.Include(".DS_Store", false) {
		t.Fatal("keep or ds_store")
	}
	if rules.Include("dist", true) {
		t.Fatal("dist included")
	}
	if err := rules.NoteDir(root, "nested"); err != nil {
		t.Fatal(err)
	}
	if rules.Include("nested/hidden.txt", false) {
		t.Fatal("nested gitignore")
	}
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	res, err := Scan(t.Context(), st, root, Options{RespectGit: true, Protect: []string{"*.pem"}, Exclude: []string{"skipme"}, MaxFileSize: 4})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Files) == 0 {
		t.Fatal("empty scan")
	}
	flat, err := Flatten(st, res.Root)
	if err != nil || len(flat) == 0 {
		t.Fatalf("flatten %v %d", err, len(flat))
	}
	if _, err := DecodeTree(nil); err == nil {
		t.Fatal("nil tree")
	}
	if _, err := DecodeTree([]byte("not-a-tree-object!!")); err == nil {
		t.Fatal("bad magic")
	}
	raw, err := EncodeTree([]Entry{{Name: "a", Kind: KindFile, Mode: 0o644}})
	if err != nil {
		t.Fatal(err)
	}
	raw[len(treeMagic)] = 9
	if _, err := DecodeTree(raw); err == nil {
		t.Fatal("bad version")
	}
	long := make([]byte, 70000)
	if _, err := EncodeTree([]Entry{{Name: string(long), Kind: KindFile}}); err == nil {
		t.Fatal("long name")
	}
	if SameState(nil, nil) != true || SameState(nil, &Meta{}) != false {
		t.Fatal("same state")
	}
	if _, err := Flatten(st, store.ID{}); err != nil {
		t.Fatal(err)
	}
	again, err := Scan(t.Context(), st, root, Options{RespectGit: true, Prev: res.Cache, MaxFileSize: 1 << 20})
	if err != nil {
		t.Fatal(err)
	}
	if again.Root.IsZero() && res.Root.IsZero() {
		t.Fatal("second scan")
	}
	partial, err := Scan(t.Context(), st, root, Options{Timeout: 1, MaxFileSize: 1 << 20})
	if err != nil {
		t.Fatal(err)
	}
	_ = partial
	if _, err := DecodeTree(raw[:len(treeMagic)+3]); err == nil {
		t.Fatal("truncated tree")
	}
}
