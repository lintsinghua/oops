package snapshot

import "testing"

func TestAnnotateBinaryAndText(t *testing.T) {
	changes := []Change{{Path: "a", Pre: &Meta{Kind: KindFile}, Post: &Meta{Kind: KindFile}}}
	if AnnotateLines(changes, nil)[0].Binary {
		t.Fatal("nil reader")
	}
	bin := AnnotateLines(changes, func(*Meta) []byte { return []byte{0, 1, 2} })
	if !bin[0].Binary {
		t.Fatal("binary")
	}
	dirs := []Change{{Path: "d", Pre: &Meta{Kind: KindDir}, Post: &Meta{Kind: KindDir}}}
	if AnnotateLines(dirs, func(*Meta) []byte { return []byte("a\n") })[0].Binary {
		t.Fatal("dir")
	}
	text := AnnotateLines([]Change{{Path: "t", Pre: &Meta{Kind: KindFile}, Post: &Meta{Kind: KindFile}}}, func(m *Meta) []byte {
		if m == nil {
			return nil
		}
		return []byte("a\nb\n")
	})
	if text[0].Binary || text[0].Insertions+text[0].Deletions < 0 {
		t.Fatal(text[0])
	}
	if _, err := Unified("a", "b", "a\n", "b\n", 1); err != nil {
		t.Fatal(err)
	}
}

func TestFileChangesSkipDirectories(t *testing.T) {
	changes := []Change{
		{Path: "onlydir", Action: "added", Post: &Meta{Kind: KindDir}},
		{Path: "nested", Action: "added", Post: &Meta{Kind: KindDir}},
		{Path: "nested/a.txt", Action: "added", Post: &Meta{Kind: KindFile}},
		{Path: "b.txt", Action: "added", Post: &Meta{Kind: KindFile}},
		{Path: "link", Action: "added", Post: &Meta{Kind: KindSymlink}},
	}
	got := FileChanges(changes)
	if CountFileChanges(changes) != 3 || len(got) != 3 {
		t.Fatalf("count %d %+v", CountFileChanges(changes), got)
	}
	if got[0].Path != "nested/a.txt" && got[0].Path != "b.txt" && got[0].Path != "link" {
		t.Fatal(got[0].Path)
	}
	for _, c := range got {
		if IsDirChange(c) {
			t.Fatalf("directory %s", c.Path)
		}
	}
}
