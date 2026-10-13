package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestParseAndLoad(t *testing.T) {
	n, err := ParseSize("")
	if err != nil || n != 0 {
		t.Fatal(n, err)
	}
	n, err = ParseSize("50MB")
	if err != nil || n != 50<<20 {
		t.Fatal(n, err)
	}
	if _, err := ParseSize("nope"); err == nil {
		t.Fatal("bad size")
	}
	for _, s := range []string{"10", "10B", "2KB", "2K", "3M", "4GB", "4G"} {
		if _, err := ParseSize(s); err != nil {
			t.Fatal(s, err)
		}
	}
	d, err := ParseDuration("")
	if err != nil || d != 0 {
		t.Fatal(d, err)
	}
	d, err = ParseDuration("10s")
	if err != nil || d != 10*time.Second {
		t.Fatal(d, err)
	}
	cfg, err := Load("")
	if err != nil || cfg.Snapshot.MaxFileSize == "" {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "missing.toml")
	cfg, err = Load(path)
	if err != nil || cfg.Retention.KeepTurns == 0 {
		t.Fatal(err)
	}
	dir := t.TempDir()
	bad := filepath.Join(dir, "bad.toml")
	if err := os.WriteFile(bad, []byte("not = ["), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(bad); err == nil {
		t.Fatal("bad toml")
	}
	partial := filepath.Join(dir, "partial.toml")
	if err := os.WriteFile(partial, []byte("privacy.record_prompts = false\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err = Load(partial)
	if err != nil || cfg.Privacy.RecordPrompts || cfg.Snapshot.Timeout == "" || cfg.UI.Color == "" {
		t.Fatalf("%+v %v", cfg, err)
	}
	t.Setenv("OOPS_HOME", t.TempDir())
	home, err := DataHome()
	if err != nil || home == "" {
		t.Fatal(err)
	}
	if _, err := ConfigPath(); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" {
		t.Setenv("XDG_DATA_HOME", "")
		t.Setenv("XDG_CONFIG_HOME", "")
		t.Setenv("OOPS_HOME", "")
		if _, err := DataHome(); err != nil {
			t.Fatal(err)
		}
		if _, err := ConfigPath(); err != nil {
			t.Fatal(err)
		}
	}
}
