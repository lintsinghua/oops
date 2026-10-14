package cli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestE2EPreSnapshotBeforeHookReturns(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	gitInit(t, root)
	src := filepath.Join(root, "src")
	if err := os.MkdirAll(filepath.Join(src, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "a.txt"), []byte("orig-a"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "sub", "b.txt"), []byte("orig-b"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out := runOops(t, home, root, hookBody(root, "pre", "edit"), "hook", "claude", "turn-start")
	if code != 0 {
		t.Fatalf("start: %d %s", code, out)
	}
	// No delay. The start hook must already have stored the original tree.
	if err := os.RemoveAll(src); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "src", "a.txt"), []byte("mutated"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "other.txt"), []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out = runOops(t, home, root, hookBody(root, "pre", ""), "hook", "claude", "turn-end")
	if code != 0 {
		t.Fatalf("end: %d %s", code, out)
	}
	code, out = runOops(t, home, root, "", "undo", "-y")
	if code != 0 {
		t.Fatalf("undo: %d %s", code, out)
	}
	if got := mustReadFile(t, filepath.Join(root, "src", "a.txt")); got != "orig-a" {
		t.Fatalf("a.txt %q", got)
	}
	if got := mustReadFile(t, filepath.Join(root, "src", "sub", "b.txt")); got != "orig-b" {
		t.Fatalf("b.txt %q", got)
	}
	if _, err := os.Stat(filepath.Join(root, "other.txt")); !os.IsNotExist(err) {
		t.Fatalf("other.txt still present: %v", err)
	}
}

func TestE2EGitUntouched(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	gitInit(t, root)
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("v"), 0o644); err != nil {
		t.Fatal(err)
	}
	before := dirSnapshot(t, filepath.Join(root, ".git"))
	code, out := runOops(t, home, root, hookBody(root, "s", "x"), "hook", "claude", "turn-start")
	if code != 0 {
		t.Fatal(out)
	}
	code, out = runOops(t, home, root, hookBody(root, "s", ""), "hook", "claude", "turn-end")
	if code != 0 {
		t.Fatal(out)
	}
	after := dirSnapshot(t, filepath.Join(root, ".git"))
	if before != after {
		t.Fatalf(".git changed\nbefore %s\nafter %s", before, after)
	}
}

func TestE2ELazyCloseAndRunning(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	gitInit(t, root)
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("v1"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out := runOops(t, home, root, hookBody(root, "s", "run"), "hook", "claude", "turn-start")
	if code != 0 {
		t.Fatal(out)
	}
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("v2"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out = runOops(t, home, root, "", "undo", "-y")
	if code != 4 || !strings.Contains(out, "still running") {
		t.Fatalf("running undo: %d %s", code, out)
	}
	if mustReadFile(t, filepath.Join(root, "a.txt")) != "v2" {
		t.Fatal("undo changed a running turn")
	}
	code, out = runOops(t, home, root, "", "undo", "--running", "-y")
	if code != 0 {
		t.Fatalf("undo --running: %d %s", code, out)
	}
	if mustReadFile(t, filepath.Join(root, "a.txt")) != "v1" {
		t.Fatalf("running undo got %q", mustReadFile(t, filepath.Join(root, "a.txt")))
	}

	code, out = runOops(t, home, root, hookBody(root, "late", "x"), "hook", "claude", "turn-start")
	if code != 0 {
		t.Fatal(out)
	}
	if err := os.WriteFile(filepath.Join(root, "late.txt"), []byte("after"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out = runOops(t, home, root, "", "list")
	if code != 0 {
		t.Fatal(out)
	}
	if strings.Contains(out, "running") {
		t.Fatalf("lazy close left the turn running:\n%s", out)
	}
	code, out = runOops(t, home, root, "", "undo", "-y")
	if code != 0 {
		t.Fatal(out)
	}
	if _, err := os.Stat(filepath.Join(root, "late.txt")); !os.IsNotExist(err) {
		t.Fatalf("lazy-closed turn did not undo: %v", err)
	}
}

func TestE2EUndoFileAndMissing(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	gitInit(t, root)
	if err := os.MkdirAll(filepath.Join(root, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "src", "a.txt"), []byte("a1"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "src", "b.txt"), []byte("b1"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, _ = runOops(t, home, root, hookBody(root, "s", "both"), "hook", "claude", "turn-start")
	if err := os.WriteFile(filepath.Join(root, "src", "a.txt"), []byte("a2"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "src", "b.txt"), []byte("b2"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, _ = runOops(t, home, root, hookBody(root, "s", ""), "hook", "claude", "turn-end")
	code, out := runOops(t, home, root, "", "undo", "--file", "missing.txt")
	if code != 2 || !strings.Contains(out, "wasn't part of turn") {
		t.Fatalf("missing: %d %s", code, out)
	}
	code, out = runOops(t, home, root, "", "undo", "-y", "--file", "src/*.txt")
	if code != 0 {
		t.Fatalf("glob: %d %s", code, out)
	}
	if mustReadFile(t, filepath.Join(root, "src", "a.txt")) != "a1" || mustReadFile(t, filepath.Join(root, "src", "b.txt")) != "b1" {
		t.Fatal("glob undo missed a file")
	}
}

func TestE2ERewindKeepsGap(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	gitInit(t, root)
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("v1"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, _ = runOops(t, home, root, hookBody(root, "t1", "one"), "hook", "claude", "turn-start")
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("v2"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, _ = runOops(t, home, root, hookBody(root, "t1", ""), "hook", "claude", "turn-end")
	if err := os.WriteFile(filepath.Join(root, "mine.txt"), []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, _ = runOops(t, home, root, hookBody(root, "t2", "two"), "hook", "claude", "turn-start")
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("v3"), 0o644); err != nil {
		t.Fatal(err)
	}
	// A one-second mtime can land inside the turn-end snapshot window and look
	// like a user edit. Pin it before that window so rewind still sees the agent's bytes.
	past := time.Now().Add(-2 * time.Second)
	if err := os.Chtimes(filepath.Join(root, "a.txt"), past, past); err != nil {
		t.Fatal(err)
	}
	_, _ = runOops(t, home, root, hookBody(root, "t2", ""), "hook", "claude", "turn-end")
	code, out := runOops(t, home, root, "", "rewind", "1")
	if code != 2 || !strings.Contains(out, "needs confirmation") {
		t.Fatalf("rewind confirm: %d %s", code, out)
	}
	if mustReadFile(t, filepath.Join(root, "a.txt")) != "v3" {
		t.Fatal("rewind without -y changed files")
	}
	code, out = runOops(t, home, root, "", "rewind", "1", "-y")
	if code != 0 {
		t.Fatalf("rewind: %d %s", code, out)
	}
	if mustReadFile(t, filepath.Join(root, "a.txt")) != "v1" {
		t.Fatalf("a.txt %q", mustReadFile(t, filepath.Join(root, "a.txt")))
	}
	if mustReadFile(t, filepath.Join(root, "mine.txt")) != "keep" {
		t.Fatal("gap file was not kept")
	}
	code, out = runOops(t, home, root, "", "list")
	if code != 0 || !strings.Contains(out, "changed outside agents") {
		t.Fatalf("gap line: %d %s", code, out)
	}
}

func TestE2EListLimitsAndColumns(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	gitInit(t, root)
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("v"), 0o644); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 12; i++ {
		code, out := runOops(t, home, root, "", "snap", "-q", "-m", "m")
		if code != 0 {
			t.Fatalf("snap: %d %s", code, out)
		}
	}
	code, out := runOops(t, home, root, "", "list")
	if code != 0 {
		t.Fatal(out)
	}
	if strings.Count(out, "\n#")+strings.Count(out, "\n  #") < 1 {
		t.Fatalf("list columns:\n%s", out)
	}
	if !strings.Contains(out, "you") || !strings.Contains(out, "file") {
		t.Fatalf("list missing columns:\n%s", out)
	}
	n := strings.Count(out, "#")
	code, out = runOops(t, home, root, "", "list", "-n", "2")
	if code != 0 || strings.Count(out, "#") != 2 {
		t.Fatalf("list -n: %d count %d\n%s", code, strings.Count(out, "#"), out)
	}
	code, out = runOops(t, home, root, "", "list", "--all")
	if code != 0 || strings.Count(out, "#") < n {
		t.Fatalf("--all showed fewer turns than the default page: %d vs %d\n%s", strings.Count(out, "#"), n, out)
	}
	start := time.Now()
	code, out = runOops(t, home, root, "", "list", "--json")
	if code != 0 || !strings.Contains(out, `"schema_version"`) {
		t.Fatalf("list json: %d %s", code, out)
	}
	if time.Since(start) > 100*time.Millisecond && runtime.GOOS != "windows" {
		t.Fatalf("list took %s", time.Since(start))
	}
	if runtime.GOOS == "windows" && time.Since(start) > 200*time.Millisecond {
		t.Fatalf("list took %s", time.Since(start))
	}
}

func TestE2EShowLabelsAndBinaryDiff(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	gitInit(t, root)
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(".env\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "tracked.txt"), []byte("t"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitAdd(t, root, "tracked.txt")
	if err := os.WriteFile(filepath.Join(root, ".env"), []byte("K=1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, _ = runOops(t, home, root, hookBody(root, "s", "x"), "hook", "claude", "turn-start")
	if err := os.WriteFile(filepath.Join(root, "notes.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".env"), []byte("K=2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "bin.dat"), []byte{0, 1, 2, 3}, 0o644); err != nil {
		t.Fatal(err)
	}
	_, _ = runOops(t, home, root, hookBody(root, "s", ""), "hook", "claude", "turn-end")
	code, out := runOops(t, home, root, "", "show", "--json")
	if code != 0 || !strings.Contains(out, `"untracked"`) || !strings.Contains(out, `"ignored"`) {
		t.Fatalf("show json: %d %s", code, out)
	}
	code, out = runOops(t, home, root, "", "show")
	if code != 0 || !strings.Contains(out, "untracked") || !strings.Contains(out, "ignored") {
		t.Fatalf("show: %d %s", code, out)
	}
	code, out = runOops(t, home, root, "", "diff", "--no-pager")
	if code != 0 || !strings.Contains(out, "bin.dat") || strings.Contains(out, "\x00") {
		t.Fatalf("binary diff: %d %q", code, out)
	}
	code, out = runOops(t, home, root, "", "diff", "--stat")
	if code != 0 || !strings.Contains(out, "binary") {
		t.Fatalf("stat: %d %s", code, out)
	}
}

func TestE2EDiffPager(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	gitInit(t, root)
	var body strings.Builder
	for i := 0; i < 80; i++ {
		body.WriteString("line\n")
	}
	if err := os.WriteFile(filepath.Join(root, "long.txt"), []byte(body.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	_, _ = runOops(t, home, root, hookBody(root, "s", "x"), "hook", "claude", "turn-start")
	body.Reset()
	for i := 0; i < 80; i++ {
		body.WriteString("changed\n")
	}
	if err := os.WriteFile(filepath.Join(root, "long.txt"), []byte(body.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	_, _ = runOops(t, home, root, hookBody(root, "s", ""), "hook", "claude", "turn-end")

	pagerDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(pagerDir, "go.mod"), []byte("module pager\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	src := "package main\nimport (\"io\";\"os\")\nfunc main(){b,_:=io.ReadAll(os.Stdin);os.WriteFile(os.Getenv(\"OOPS_PAGER_OUT\"),b,0644)}\n"
	if err := os.WriteFile(filepath.Join(pagerDir, "main.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(pagerDir, "pager")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	cmd := exec.Command("go", "build", "-o", bin, ".")
	cmd.Dir = pagerDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("pager build: %v %s", err, out)
	}
	marker := filepath.Join(t.TempDir(), "paged.txt")
	t.Setenv("PAGER", bin)
	t.Setenv("OOPS_PAGER_OUT", marker)
	testingTTY = func(any) bool { return true }
	t.Cleanup(func() { testingTTY = nil })
	code, out := runOops(t, home, root, "", "diff")
	if code != 0 {
		t.Fatal(out)
	}
	paged, err := os.ReadFile(marker)
	if err != nil || !bytes.Contains(paged, []byte("long.txt")) {
		t.Fatalf("pager did not run: %v %q stdout %s", err, paged, out)
	}
}

func TestE2ERestore(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	gitInit(t, root)
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("v1"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, _ = runOops(t, home, root, hookBody(root, "s", "x"), "hook", "claude", "turn-start")
	if err := os.Remove(filepath.Join(root, "a.txt")); err != nil {
		t.Fatal(err)
	}
	_, _ = runOops(t, home, root, hookBody(root, "s", ""), "hook", "claude", "turn-end")
	code, out := runOops(t, home, root, "", "restore", "a.txt", "--at", "1")
	if code != 0 {
		t.Fatalf("restore deleted: %d %s", code, out)
	}
	if mustReadFile(t, filepath.Join(root, "a.txt")) != "v1" {
		t.Fatalf("restored %q", mustReadFile(t, filepath.Join(root, "a.txt")))
	}
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("user"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out = runOops(t, home, root, "", "restore", "a.txt", "--at", "1")
	if code != 2 {
		t.Fatalf("restore confirm: %d %s", code, out)
	}
	if mustReadFile(t, filepath.Join(root, "a.txt")) != "user" {
		t.Fatal("restore overwrote without confirmation")
	}
	code, out = runOops(t, home, root, "", "restore", "a.txt", "--at", "1", "--force")
	if code != 0 || mustReadFile(t, filepath.Join(root, "a.txt")) != "v1" {
		t.Fatalf("force: %d %s body %q", code, out, mustReadFile(t, filepath.Join(root, "a.txt")))
	}
}

func TestE2ESnapQuiet(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	gitInit(t, root)
	code, out := runOops(t, home, root, "", "snap", "-n")
	if code != 0 || !strings.Contains(out, "Nothing changed") {
		t.Fatalf("dry: %d %s", code, out)
	}
	code, out = runOops(t, home, root, "", "list")
	if code != 0 || strings.Contains(out, "#1") {
		t.Fatalf("dry snap wrote a turn: %d\n%s", code, out)
	}
	code, out = runOops(t, home, root, "", "snap", "-q", "-m", "note")
	if code != 0 || strings.TrimSpace(out) != "" {
		t.Fatalf("quiet snap: %d %q", code, out)
	}
	code, out = runOops(t, home, root, "", "list", "--json")
	if code != 0 || !strings.Contains(out, `"id": 1`) {
		t.Fatalf("snap missing: %s", out)
	}
}

func TestE2EInitAgents(t *testing.T) {
	home := t.TempDir()
	user := t.TempDir()
	root := t.TempDir()
	gitInit(t, root)
	bin := t.TempDir()
	for _, name := range []string{"claude", "codex", "gemini", "cursor-agent"} {
		writeFakeAgent(t, bin, name)
	}
	if err := os.MkdirAll(filepath.Join(user, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(user, ".claude", "settings.json"), []byte("{\"theme\":\"dark\"}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out := runOops(t, home, root, "", "--user-home", user, "init", "--print", "--path", bin, "-y")
	if code != 0 {
		t.Fatalf("print: %d %s", code, out)
	}
	if _, err := os.Stat(filepath.Join(user, ".codex", "hooks.json")); !os.IsNotExist(err) {
		t.Fatal("--print wrote codex config")
	}
	code, out = runOops(t, home, root, "", "--user-home", user, "init", "--path", bin, "-y")
	if code != 0 {
		t.Fatalf("init: %d %s", code, out)
	}
	if !strings.Contains(out, "already connected") && !strings.Contains(out, "connected") {
		t.Fatalf("init output:\n%s", out)
	}
	if !strings.Contains(out, "/hooks") {
		t.Fatalf("codex trust missing:\n%s", out)
	}
	if !strings.HasSuffix(strings.TrimSpace(out), "oops") && !strings.Contains(out, "just type: oops") {
		t.Fatalf("last line:\n%s", out)
	}
	claude := mustReadFile(t, filepath.Join(user, ".claude", "settings.json"))
	if !strings.Contains(claude, "turn-start") || !strings.Contains(claude, "theme") || !strings.Contains(claude, "UserPromptExpansion") {
		t.Fatalf("claude settings %s", claude)
	}
	matches, _ := filepath.Glob(filepath.Join(user, ".claude", "settings.json.oops-backup-*"))
	if len(matches) == 0 {
		t.Fatal("no backup")
	}
	cmdPath := filepath.Join(user, ".claude", "commands", "oops.md")
	if !strings.Contains(mustReadFile(t, cmdPath), "oops") {
		t.Fatal("claude command file")
	}
	for _, p := range []string{
		filepath.Join(user, ".codex", "hooks.json"),
		filepath.Join(user, ".gemini", "settings.json"),
		filepath.Join(user, ".cursor", "hooks.json"),
	} {
		body := mustReadFile(t, p)
		if !strings.Contains(body, "hook") {
			t.Fatalf("%s missing hook: %s", p, body)
		}
	}
	code, out = runOops(t, home, root, "", "--user-home", user, "init", "--path", bin, "-y")
	if code != 0 || !strings.Contains(out, "already connected") {
		t.Fatalf("second init: %d %s", code, out)
	}
	code, out = runOops(t, home, root, "", "--user-home", user, "init", "--project", "--path", bin, "-y")
	if code != 0 {
		t.Fatalf("project: %d %s", code, out)
	}
	if _, err := os.Stat(filepath.Join(root, ".claude", "settings.json")); err != nil {
		t.Fatal(err)
	}
	_, _ = runOops(t, home, root, hookBody(root, "s", "x"), "hook", "claude", "turn-start")
	_, _ = runOops(t, home, root, hookBody(root, "s", ""), "hook", "claude", "turn-end")
	code, out = runOops(t, home, root, "", "--user-home", user, "init", "--undo", "--delete-data", "-y")
	if code != 0 {
		t.Fatalf("undo init: %d %s", code, out)
	}
	left := mustReadFile(t, filepath.Join(user, ".claude", "settings.json"))
	if strings.Contains(left, "turn-start") {
		t.Fatalf("hooks remain: %s", left)
	}
	if _, err := os.Stat(cmdPath); !os.IsNotExist(err) {
		t.Fatal("command file remains")
	}
	code, out = runOops(t, home, root, "", "list")
	if !strings.Contains(out, "No snapshots here yet.") {
		t.Fatalf("snapshots remain: %d %s", code, out)
	}
}

func TestE2ERunAndSlash(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	gitInit(t, root)
	var args []string
	if runtime.GOOS == "windows" {
		args = []string{"run", "--", "cmd", "/c", "echo hello& exit /b 3"}
	} else {
		args = []string{"run", "--", "sh", "-c", "echo hello; exit 3"}
	}
	code, out := runOops(t, home, root, "", args...)
	if code != 3 || !strings.Contains(out, "hello") || !strings.Contains(out, "protecting") || !strings.Contains(out, "saved") {
		t.Fatalf("run: %d %s", code, out)
	}
	code, out = runOops(t, home, root, "", "list", "--json")
	if code != 0 || !strings.Contains(out, `"id": 1`) {
		t.Fatalf("run turn: %s", out)
	}

	var idle []string
	if runtime.GOOS == "windows" {
		idle = []string{"run", "--idle", "200ms", "--", "powershell", "-NoProfile", "-Command", "Start-Sleep -Milliseconds 300; Set-Content -Path a.txt -Value x; Start-Sleep -Seconds 1; Add-Content -Path a.txt -Value y"}
	} else {
		idle = []string{"run", "--idle", "200ms", "--", "sh", "-c", "sleep 0.3; echo x > a.txt; sleep 1; echo y >> a.txt"}
	}
	code, out = runOops(t, home, root, "", idle...)
	if code != 0 {
		t.Fatalf("idle run: %d %s", code, out)
	}
	code, out = runOops(t, home, root, "", "list", "--json")
	if code != 0 || strings.Count(out, `"id"`) < 3 {
		t.Fatalf("idle split turns: %d %s", code, out)
	}

	code, out = runOops(t, home, root, hookBody(root, "s", "/oops"), "hook", "claude", "turn-start")
	if code != 0 || !strings.Contains(out, "decision") {
		t.Fatalf("slash: %d %s", code, out)
	}
	code, out = runOops(t, home, root, hookBody(root, "s", "/oops diff"), "hook", "claude", "slash")
	if code != 0 || !strings.Contains(out, "diff") {
		t.Fatalf("slash diff: %d %s", code, out)
	}
}

func TestE2EDoctorGCIgnorePartial(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	gitInit(t, root)
	cfg := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(cfg, []byte("[retention]\nkeep_turns = 1\nworkspace_max = \"1B\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("v"), 0o644); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		code, out := runOops(t, home, root, "", "--config", cfg, "snap", "-q")
		if code != 0 {
			t.Fatalf("snap %d: %s", i, out)
		}
	}
	code, out := runOops(t, home, root, "", "--config", cfg, "doctor", "--json")
	if code != 0 || !strings.Contains(out, "schema_version") || !strings.Contains(out, "over the workspace cap") {
		t.Fatalf("doctor: %d %s", code, out)
	}
	code, out = runOops(t, home, root, "", "--config", cfg, "gc", "-n")
	if code != 0 || !strings.Contains(out, "Would free") {
		t.Fatalf("gc dry: %d %s", code, out)
	}
	code, out = runOops(t, home, root, "", "--config", cfg, "list", "--json")
	if code != 0 {
		t.Fatalf("list before gc: %d %s", code, out)
	}
	before := strings.Count(out, `"id"`)
	code, out = runOops(t, home, root, "", "--config", cfg, "gc")
	if code != 0 {
		t.Fatalf("gc: %d %s", code, out)
	}
	code, out = runOops(t, home, root, "", "--config", cfg, "list", "--json")
	if code != 0 || strings.Count(out, `"id"`) >= before {
		t.Fatalf("gc did not drop turns: %d\n%s", code, out)
	}

	if err := os.WriteFile(filepath.Join(root, ".oopsignore"), []byte("*\n!keep.txt\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "keep.txt"), []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "skip.txt"), []byte("skip"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, _ = runOops(t, home, root, hookBody(root, "ig", "x"), "--config", cfg, "hook", "claude", "turn-start")
	if err := os.WriteFile(filepath.Join(root, "keep.txt"), []byte("gone"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "skip.txt"), []byte("changed"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, _ = runOops(t, home, root, hookBody(root, "ig", ""), "--config", cfg, "hook", "claude", "turn-end")
	_, _ = runOops(t, home, root, "", "--config", cfg, "undo", "-y")
	if mustReadFile(t, filepath.Join(root, "keep.txt")) != "keep" {
		t.Fatalf("keep.txt %q", mustReadFile(t, filepath.Join(root, "keep.txt")))
	}
	if mustReadFile(t, filepath.Join(root, "skip.txt")) != "changed" {
		t.Fatal("ignored file was restored")
	}
}

func TestE2EPartialNoticeAndHookExit(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	gitInit(t, root)
	cfg := filepath.Join(t.TempDir(), "oops.toml")
	if err := os.WriteFile(cfg, []byte("[snapshot]\ntimeout = \"1ns\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 30; i++ {
		if err := os.WriteFile(filepath.Join(root, "f"+strings.Repeat("x", 0)+string(rune('a'+i%26))+".txt"), []byte("z"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	code, out := runOops(t, home, root, hookBody(root, "s", "slow"), "--config", cfg, "hook", "claude", "turn-start")
	if code != 0 {
		t.Fatalf("partial start: %d %s", code, out)
	}
	if !strings.Contains(out, "systemMessage") || !strings.Contains(out, "oops ·") {
		t.Fatalf("claude notice: %s", out)
	}
	code, out = runOops(t, home, root, hookBody(root, "s", ""), "--config", cfg, "hook", "nope", "not-an-event")
	if code != 0 {
		t.Fatalf("bad hook: %d %s", code, out)
	}
	code, out = runOops(t, home, root, "", "--config", cfg, "list")
	if code != 0 || strings.Contains(out, "systemMessage") {
		t.Fatalf("notice repeated on the next command: %d\n%s", code, out)
	}
}

func TestE2ENonTTYTimelineAndInterrupt(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	gitInit(t, root)
	code, out := runOops(t, home, root, "", "status")
	if code != 2 {
		t.Fatalf("unknown command: %d %s", code, out)
	}
	code, out = runOops(t, home, root, "")
	if code != 0 || !strings.Contains(out, "No snapshots here yet.") || !strings.Contains(out, "oops init") {
		t.Fatalf("empty timeline: %d %s", code, out)
	}
	t.Setenv("TERM", "dumb")
	code, out = runOops(t, home, root, "", "snap", "-q")
	if code != 0 {
		t.Fatalf("snap: %d %s", code, out)
	}
	code, out = runOops(t, home, root, "")
	if code != 0 || !strings.Contains(out, "#1") {
		t.Fatalf("dumb timeline: %d %s", code, out)
	}

	testingTTY = func(any) bool { return true }
	t.Cleanup(func() { testingTTY = nil })
	code, out = runOops(t, home, root, "\x1b\n", "rewind", "1")
	if code != 130 || !strings.Contains(out, "Cancelled") {
		t.Fatalf("interrupt: %d %s", code, out)
	}
}

func TestCommandHelpExamples(t *testing.T) {
	opt := &options{color: "never", stdout: &bytes.Buffer{}, stderr: &bytes.Buffer{}, stdin: strings.NewReader("")}
	root := newRoot(opt)
	for _, c := range root.Commands() {
		if c.Hidden || c.Name() == "help" {
			continue
		}
		if c.Short == "" {
			t.Fatalf("%s has no short", c.Name())
		}
		if c.Example == "" && c.Name() != "version" && c.Name() != "hook" && c.Name() != "maintain" && c.Name() != "finish-pending" {
			t.Fatalf("%s has no example", c.Name())
		}
	}
}

func mustReadFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func dirSnapshot(t *testing.T, root string) string {
	t.Helper()
	var b strings.Builder
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		info, err := d.Info()
		if err != nil {
			return err
		}
		b.WriteString(rel)
		b.WriteByte(' ')
		b.WriteString(info.Mode().String())
		b.WriteByte('\n')
		if d.IsDir() {
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		b.Write(body)
		b.WriteByte('\n')
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return b.String()
}

func gitAdd(t *testing.T, dir, name string) {
	t.Helper()
	cmd := exec.Command("git", "add", name)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v %s", err, out)
	}
}

func writeFakeAgent(t *testing.T, dir, name string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		path := filepath.Join(dir, name+".cmd")
		if err := os.WriteFile(path, []byte("@echo 1.2.3\r\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\necho 1.2.3\n"), 0o755); err != nil {
		t.Fatal(err)
	}
}
