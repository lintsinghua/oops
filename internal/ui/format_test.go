package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
)

func TestFormatHelpers(t *testing.T) {
	now := time.Date(2026, 10, 13, 15, 0, 0, 0, time.UTC)
	if RelTime(now, time.Time{}) != "" {
		t.Fatal("zero")
	}
	if RelTime(now, now.Add(-10*time.Second)) != "just now" {
		t.Fatal("just now")
	}
	if RelTime(now, now.Add(-5*time.Minute)) != "5 min ago" {
		t.Fatal("min")
	}
	if RelTime(now, now.Add(-3*time.Hour)) != "3 hr ago" {
		t.Fatal("hr")
	}
	if RelTime(now, now.AddDate(0, 0, -1)) != "yesterday" {
		t.Fatal("yesterday")
	}
	if RelTime(now, now.AddDate(0, 0, -3)) != "Sat" {
		t.Fatalf("weekday %q", RelTime(now, now.AddDate(0, 0, -3)))
	}
	if RelTime(now, time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC)) != "Mar 2" {
		t.Fatal("month")
	}
	if RelTime(now, time.Date(2024, 3, 2, 0, 0, 0, 0, time.UTC)) != "Mar 2, 2024" {
		t.Fatal("year")
	}
	if Plural(1, "file", "files") != "1 file" || Plural(2, "file", "files") != "2 files" {
		t.Fatal("plural")
	}
	if Size(12) != "12 B" || Size(1500) != "1.5 KB" || Size(2_000_000) != "2 MB" || Size(3_000_000_000) != "3 GB" {
		t.Fatalf("size %s %s %s %s", Size(12), Size(1500), Size(2_000_000), Size(3_000_000_000))
	}
	if Quote("  hi  ", 10) != `"hi"` || Quote("", 10) != "" {
		t.Fatal("quote")
	}
	if Quote("abcdefghij", 4) == "" || Truncate("abcdef", 1) != "…" || Truncate("ab", 10) != "ab" {
		t.Fatal("truncate")
	}
	if Pad("a", 3) != "a  " || Pad("abcd", 2) != "abcd" {
		t.Fatal("pad")
	}
	long := "internal/very/long/path/handler.go"
	mid := MiddleTruncate(long, 24)
	if !strings.Contains(mid, "…") || !strings.HasSuffix(mid, "handler.go") || strings.Contains(mid, long) {
		t.Fatalf("middle %q", mid)
	}
	if MiddleTruncate("short", 20) != "short" {
		t.Fatal("short path")
	}
	if DisplayWidth("ab") != 2 || RuneLen("é") != 1 {
		t.Fatal("width")
	}
	if Paint(lipgloss.NewStyle(), "x") != "x" {
		t.Fatal("paint")
	}
}
