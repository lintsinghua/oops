package ui

import (
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
)

var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]`)

func stripANSI(s string) string { return ansiPattern.ReplaceAllString(s, "") }

// RelTime formats a timestamp the way the timeline does.
func RelTime(now, t time.Time) string {
	if t.IsZero() {
		return ""
	}
	d := now.Sub(t)
	if d < time.Minute {
		return "just now"
	}
	if d < time.Hour {
		n := int(d.Minutes())
		if n < 1 {
			n = 1
		}
		return fmt.Sprintf("%d min ago", n)
	}
	if sameDay(now, t) {
		n := int(d.Hours())
		if n < 1 {
			n = 1
		}
		return fmt.Sprintf("%d hr ago", n)
	}
	if sameDay(now.AddDate(0, 0, -1), t) {
		return "yesterday"
	}
	if d < 7*24*time.Hour {
		return t.Format("Mon")
	}
	if t.Year() == now.Year() {
		return t.Format("Jan 2")
	}
	return t.Format("Jan 2, 2006")
}

func sameDay(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}

// Plural returns "1 file" or "3 files".
func Plural(n int, one, many string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, one)
	}
	return fmt.Sprintf("%d %s", n, many)
}

// Size formats a byte count.
func Size(n int64) string {
	switch {
	case n < 1000:
		return fmt.Sprintf("%d B", n)
	case n < 1000*1000:
		return trimFloat(float64(n)/1000) + " KB"
	case n < 1000*1000*1000:
		return trimFloat(float64(n)/(1000*1000)) + " MB"
	default:
		return trimFloat(float64(n)/(1000*1000*1000)) + " GB"
	}
}

func trimFloat(f float64) string {
	s := fmt.Sprintf("%.1f", f)
	return strings.TrimSuffix(s, ".0")
}

// Quote truncates a prompt summary and wraps it in quotes.
func Quote(s string, width int) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if width > 0 && runewidth.StringWidth(s) > width {
		s = runewidth.Truncate(s, width, "")
		s = strings.TrimRight(s, " ") + "…"
	}
	return `"` + s + `"`
}

// Truncate shortens s to width display columns.
func Truncate(s string, width int) string {
	if width <= 0 || runewidth.StringWidth(s) <= width {
		return s
	}
	if width == 1 {
		return "…"
	}
	return runewidth.Truncate(s, width, "…")
}

// MiddleTruncate keeps the head and the tail of s, so a long path still shows its filename.
func MiddleTruncate(s string, width int) string {
	if width <= 0 || runewidth.StringWidth(s) <= width {
		return s
	}
	if width <= 1 {
		return "…"
	}
	tailW := (width - 1) / 2
	if tailW < 1 {
		tailW = 1
	}
	headW := width - 1 - tailW
	if headW < 1 {
		headW = 1
		tailW = width - 2
		if tailW < 1 {
			tailW = 1
		}
	}
	head := runewidth.Truncate(s, headW, "")
	tail := s
	for runewidth.StringWidth(tail) > tailW && len(tail) > 0 {
		_, size := utf8.DecodeRuneInString(tail)
		if size <= 0 {
			break
		}
		tail = tail[size:]
	}
	return head + "…" + tail
}

// Pad pads s to width display columns.
func Pad(s string, width int) string {
	w := runewidth.StringWidth(s)
	if w >= width {
		return s
	}
	return s + strings.Repeat(" ", width-w)
}

// DisplayWidth is the terminal width of s. ANSI color codes do not count.
func DisplayWidth(s string) int { return runewidth.StringWidth(stripANSI(s)) }

// RuneLen counts runes, used where byte length would split a character.
func RuneLen(s string) int { return utf8.RuneCountInString(s) }

// Paint applies style when color is on. The style itself already no-ops
// under an ASCII color profile.
func Paint(style lipgloss.Style, s string) string { return style.Render(s) }
