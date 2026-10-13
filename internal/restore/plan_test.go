package restore

import (
	"testing"

	"github.com/lintsinghua/oops/internal/snapshot"
	"github.com/lintsinghua/oops/internal/store"
)

func TestPlanKeepsCasePairDistinct(t *testing.T) {
	// NFC and NFD are the same key on every OS, and different strings.
	nfc := "caf\u00e9.txt"
	nfd := "cafe\u0301.txt"
	preID := store.ID{1}
	postID := store.ID{2}
	pre := map[string]snapshot.Meta{
		nfc: {Kind: snapshot.KindFile, ID: preID, Mode: 0o644},
	}
	post := map[string]snapshot.Meta{
		nfd: {Kind: snapshot.KindFile, ID: postID, Mode: 0o644},
	}
	items := Plan([]string{nfc, nfd}, post, pre, post, false)
	got := map[string]string{}
	for _, it := range items {
		got[it.Path] = it.Op
	}
	if got[nfc] != "restore" {
		t.Fatalf("original spelling op %q", got[nfc])
	}
	if got[nfd] != "remove" {
		t.Fatalf("new spelling op %q", got[nfd])
	}
}

func TestPlanFoldsUnplannedSpelling(t *testing.T) {
	nfc := "caf\u00e9.txt"
	nfd := "cafe\u0301.txt"
	id := store.ID{3}
	file := snapshot.Meta{Kind: snapshot.KindFile, ID: id, Mode: 0o644}
	// The plan names only the NFC path. The live tree uses NFD. That is the same file.
	items := Plan([]string{nfc}, map[string]snapshot.Meta{nfc: file}, map[string]snapshot.Meta{nfc: file}, map[string]snapshot.Meta{nfd: file}, false)
	if len(items) != 1 || items[0].Op != "skip" {
		t.Fatalf("got %+v", items)
	}
}
